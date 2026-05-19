# KUBEDISCOVERY ANALYZER (KD-ANALYZER)

## Overview

O `kd-analyzer` é o módulo de inteligência artificial do ecossistema Kubediscovery. Ele recebe dados operacionais dos clusters Kubernetes (logs, eventos, describes de recursos) e produz análises de causa raiz e planos de remediação via **LLM (Large Language Models)**.

A arquitetura usa o **Google ADK** (`google.golang.org/adk`) como orquestrador de múltiplos agentes e o **Databricks** como provedor LLM via API compatível com OpenAI.

> **Regra de entrada**: o `kd-analyzer` **não é chamado diretamente** por nenhum cliente externo. Toda request chega exclusivamente via **gRPC bidirecional** a partir de dois originadores:
> - **`kd-gateway`** — roteador central; decide encaminhar ao analyzer quando recebe uma request externa (usuário, MCP, ferramenta de automação) ou um evento detectado pelo `kd-executor` watcher.
> - **`kd-executor`** — envia dados de observabilidade (logs, eventos, describe) coletados pelo watcher no cluster remoto.
>
> O `kd-gateway` é o único ponto que conhece o `kd-analyzer` — ele avalia o tipo de payload e decide se deve acionar o módulo de análise.

---

## Arquitetura

```
                    ┌─────────────────────────────────┐
                    │  Originadores (únicos permitidos)│
                    └──────────────┬──────────────────┘
                                   │
          ┌────────────────────────┴──────────────────────┐
          │                                               │
  [kd-gateway]                                   [kd-executor]
  Request externa:                                Watcher interno:
  - usuário via MCP                               - pod Pending/CrashLoop
  - ferramenta de automação                       - evento anômalo detectado
  - alert externo                                 - describe coletado
          │                                               │
          └────────────────────┬──────────────────────────┘
                               │  gRPC bidirecional (mTLS)
                               ▼
                         [kd-analyzer]
                               │
                               ├─ DatabricksModel  ←  implementa model.LLM do ADK
                               │       ├─ TokenProvider (static ou M2M OAuth)
                               │       └─ OpenAI-compatible client → https://<workspace>/serving-endpoints/
                               │
                               ├─ Agentes (llmagent.New)
                               │   ├─ log_analyst          ← analisa logs, identifica causa raiz
                               │   ├─ event_analyst        ← analisa events do Kubernetes
                               │   ├─ describe_analyst     ← analisa describe de pods/nodes/ingresses
                               │   ├─ karpenter_analyst    ← analisa logs do Karpenter (pods Pending/taint)
                               │   └─ remedy_advisor       ← gera plano de remediação
                               │
                               ├─ Tools (functiontool.New — tipadas em Go)
                               │   ├─ analyze_k8s_logs     ← input: cluster/ns/logs/histórico
                               │   ├─ get_k8s_events       ← busca eventos no kd-store
                               │   ├─ describe_resource    ← describe de pod/node/ingress
                               │   └─ get_karpenter_logs   ← logs do Karpenter deployment
                               │
                               ├─ Runner (runner.New + session.InMemoryService)
                               │       └─ Cada agente roda em sessão isolada
                               │
                               └─ Retorna sempre: AnalysisResult
                                       ├─ Request:  string
                                       ├─ Analyze:  string
                                       └─ Resolver: string
                                               │
                                               ▼
                                        kd-gateway → notifica usuário
                                        kd-store   → persiste no histórico
```

### Decisão de roteamento no `kd-gateway`

O `kd-gateway` decide acionar o `kd-analyzer` com base no tipo de payload recebido:

| Originador | Trigger | Ação do gateway |
|---|---|---|
| `kd-executor` (watcher) | Pod em `Pending`, `CrashLoopBackOff`, `OOMKilled` | Roteia para `kd-analyzer` automaticamente |
| `kd-executor` (watcher) | Evento `Warning` com alta frequência | Roteia para `kd-analyzer` automaticamente |
| Usuário / MCP | Request explícita de análise | Roteia para `kd-analyzer` sob demanda |
| Ferramenta de automação | Webhook / alert externo | Roteia para `kd-analyzer` conforme regra configurada |

---

## Provedor LLM: Databricks

O `kd-analyzer` usa a API Databricks via endpoint OpenAI-compatible.

### Exemplo de chamada direta

```bash
curl --location 'https://<DATABRICKS_HOST>/serving-endpoints/chat/completions' \
  --header 'Authorization: Bearer <DATABRICKS_TOKEN>' \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "databricks-claude-sonnet-4-6",
    "messages": [
      { "role": "user", "content": "What is an LLM agent?" }
    ]
  }'
```

### `DatabricksModel` — implementação da interface `model.LLM`

```go
type Config struct {
    WorkspaceHost string   // ex: stone-dataplatform-production.cloud.databricks.com
    EndpointName  string   // ex: databricks-claude-sonnet-4-6
    ModelName     string   // nome lógico do modelo (padrão: EndpointName)
    TokenProvider *TokenProvider
}
```

O `DatabricksModel` implementa `model.LLM` do ADK, fazendo a ponte entre o formato de request do ADK (genai) e a API OpenAI-compatible do Databricks. Suporta **streaming** e **não-streaming**.

---

## Autenticação: `TokenProvider`

Dois modos de autenticação suportados:

| Modo | Quando usar | Como criar |
|---|---|---|
| **Token estático** | Dev / teste | `NewStaticTokenProvider(token)` |
| **M2M OAuth** (Service Principal) | Produção | `NewTokenProvider(host, clientID, clientSecret)` |

### Token estático (dev/test)
```go
provider := databricksmodel.NewStaticTokenProvider(os.Getenv("DATABRICKS_TOKEN"))
```

### M2M OAuth (produção)
```go
provider, err := databricksmodel.NewTokenProvider(
    os.Getenv("DATABRICKS_HOST"),
    os.Getenv("DATABRICKS_SP_CLIENT_ID"),
    os.Getenv("DATABRICKS_SP_CLIENT_SECRET"),
)
```

O `TokenProvider` M2M usa `sync.Mutex` + cache interno, renovando automaticamente o token 30 segundos antes do vencimento (tokens Databricks duram 1h).

---

## Variáveis de Ambiente

| Variável | Obrigatória | Descrição |
|---|---|---|
| `DATABRICKS_HOST` | Sim | Host do workspace Databricks (sem `https://`) |
| `DATABRICKS_ENDPOINT_NAME` | Sim | Nome do serving endpoint (ex: `databricks-claude-sonnet-4-6`) |
| `DATABRICKS_TOKEN` | Dev | Token estático PAT (Personal Access Token) |
| `DATABRICKS_SP_CLIENT_ID` | Prod | Client ID do Service Principal (M2M OAuth) |
| `DATABRICKS_SP_CLIENT_SECRET` | Prod | Client Secret do Service Principal (M2M OAuth) |

> **Prioridade de autenticação**: se `DATABRICKS_SP_CLIENT_ID` + `DATABRICKS_SP_CLIENT_SECRET` estiverem definidos, M2M é usado. Caso contrário, `DATABRICKS_TOKEN` estático é usado.

> **Modelo por agente**: o modelo a ser usado é passado como parâmetro ao criar o `DatabricksModel`. Cada agente pode usar um modelo diferente — ex: agentes de triagem podem usar um modelo menor/mais barato e agentes de análise profunda um modelo mais capaz.

---

## Orquestrador de Agentes: Google ADK

O **Google ADK** (`google.golang.org/adk`) gerencia o ciclo de vida de cada agente LLM — sessão, histórico, tools, streaming e execução.

### Estrutura de criação de um agente

```go
agent, err := llmagent.New(llmagent.Config{
    Name:        "log_analyst",
    Model:       dbModel,                  // DatabricksModel (model.LLM)
    Description: "Analisa logs de pods",
    Instruction: `<system prompt do agente>`,
    Tools:       []tool.Tool{k8sTool},     // tools tipadas (optional)
    GenerateContentConfig: &genai.GenerateContentConfig{
        Temperature:     genai.Ptr[float32](0.1),
        MaxOutputTokens: 1024,
    },
})
```

### Execução isolada por agente

Cada agente roda em sua própria sessão — sem compartilhar histórico. O output de um agente é passado manualmente como input do próximo (pipeline em Go).

```go
func runAgent(ctx context.Context, a agent.Agent, prompt string) (string, error) {
    sessionService := session.InMemoryService()
    sess, _ := sessionService.Create(ctx, &session.CreateRequest{
        AppName: "k8s_incident_app",
        UserID:  "ops_user",
    })
    r, _ := runner.New(runner.Config{
        AppName:        "k8s_incident_app",
        Agent:          a,
        SessionService: sessionService,
    })
    // Itera sobre eventos — coleta apenas FinalResponse
    for event, err := range r.Run(ctx, "ops_user", sess.Session.ID(), userMsg, agent.RunConfig{
        StreamingMode: agent.StreamingModeNone,
    }) { ... }
}
```

---

## Tools Kubernetes (functiontool)

As tools são funções Go tipadas, expostas ao LLM via `functiontool.New`. O ADK gera automaticamente o JSON Schema a partir dos tipos Go.

### Tool atual: `analyze_k8s_logs`

```go
type AnalyzeK8sLogsInput struct {
    ClusterName string `json:"cluster_name"  jsonschema:"Nome do cluster Kubernetes"`
    Environment string `json:"environment"   jsonschema:"Ambiente: production, staging, dev"`
    Namespace   string `json:"namespace"     jsonschema:"Namespace da aplicação"`
    Logs        string `json:"logs"          jsonschema:"Logs brutos do pod em CrashLoopBackOff"`
    History     string `json:"history"       jsonschema:"Histórico dos 2 incidentes anteriores (JSON)"`
}

type AnalyzeK8sLogsOutput struct {
    RootCause   string   `json:"root_cause"`
    Severity    string   `json:"severity"`    // high | medium | low
    Suggestions []string `json:"suggestions"`
}
```

### Tools planejadas para `kd-analyzer`

| Tool | Input | Output | Fonte dos dados |
|---|---|---|---|
| `analyze_k8s_logs` | cluster, namespace, logs, histórico | rootCause, severity, suggestions | kd-executor → kd-gateway |
| `get_k8s_events` | cluster, namespace, kind, name | lista de eventos | kd-store (PostgreSQL) |
| `describe_pod` | cluster, namespace, pod name | describe completo | kd-executor (kubectl describe) |
| `describe_node` | cluster, node name | describe completo | kd-executor (kubectl describe) |
| `describe_ingress` | cluster, namespace, name | describe completo | kd-executor (kubectl describe) |
| `get_incident_history` | cluster, namespace, chave índice | histórico de incidentes | kd-store (índice: cluster+env+namespace) |
| `get_karpenter_logs` | cluster, environment, lines | logs, version, nodePools | kd-executor (`kubectl logs -n karpenter deployment/karpenter`) |

---

## Agentes Planejados

A estrutura de agentes segue o padrão `llmagent.Config` — o modelo é injetado como parâmetro, permitindo usar modelos diferentes por agente.

### Pipeline de análise de incidente

```
Incidente recebido (log / evento / describe)
    │  Source: "kd-gateway" (externo) ou "kd-executor" (watcher)
    │
    ├─ [Agente 1] log_analyst
    │   Model:       <MODEL_LOG_ANALYST>     ← variável de ambiente
    │   Tools:       analyze_k8s_logs, get_incident_history
    │   Temperature: 0.1 (determinístico)
    │   Output:      causa raiz + severidade
    │
    ├─ [Agente 2] event_analyst
    │   Model:       <MODEL_EVENT_ANALYST>
    │   Tools:       get_k8s_events
    │   Temperature: 0.1
    │   Output:      padrão de eventos anômalos
    │
    ├─ [Agente 3] describe_analyst
    │   Model:       <MODEL_DESCRIBE_ANALYST>
    │   Tools:       describe_pod, describe_node, describe_ingress
    │   Temperature: 0.1
    │   Output:      estado do recurso + configurações problemáticas
    │
    ├─ [Agente 3] describe_analyst
    │   Model:       <MODEL_DESCRIBE_ANALYST>
    │   Tools:       describe_pod, describe_node, describe_ingress
    │   Temperature: 0.1
    │   Output:      estado do recurso + configurações problemáticas
    │
    ├─ [Agente 4] karpenter_analyst  ← acionado condicionalmente
    │   Model:       <MODEL_KARPENTER_ANALYST>
    │   Condição:    pod em Pending OU eventos com "untolerated taint" / "node(s) had untolerated taint"
    │   Tools:       get_karpenter_logs, describe_node
    │   Temperature: 0.1
    │   Instruction: Você é especialista em Karpenter (AWS Node Autoscaler).
    │                Analise os logs do deployment karpenter no namespace karpenter.
    │                Identifique: nodepool sem capacidade, taints não toleradas,
    │                consolidation bloqueando scheduling, limites de instância EC2.
    │   Output:      diagnóstico do Karpenter + ajuste de NodePool/EC2NodeClass recomendado
    │
    └─ [Agente 5] remedy_advisor
        Model:       <MODEL_REMEDY_ADVISOR>  ← pode ser modelo mais capaz
        Tools:       (nenhuma — raciocínio puro)
        Temperature: 0.2
        Input:       outputs dos agentes anteriores (condicionais)
        Output:      plano de remediação priorizado com kubectl commands
```

### Lógica condicional: quando acionar `karpenter_analyst`

```
describe_analyst retorna
    │
    ├─ pod.Status == "Pending"?
    │       └─ Sim → verifica eventos do pod
    │               └─ evento contém qualquer um de:
    │                   - "untolerated taint"
    │                   - "node(s) had untolerated taint"
    │                   - "no new claims to deallocate"
    │                   - "Preemption is not helpful"
    │                   - "0/N nodes are available"
    │                       └─ Sim → aciona karpenter_analyst
    │
    └─ eventos com Warning de FailedScheduling?
            └─ Sim → aciona karpenter_analyst
```

### Tool: `get_karpenter_logs`

```go
type GetKarpenterLogsInput struct {
    ClusterName string `json:"cluster_name"`
    Environment string `json:"environment"`
    // Karpenter roda no namespace karpenter por padrão (AWS EKS)
    // Lines limita o volume de logs enviados ao LLM
    Lines       int    `json:"lines"`  // padrão: 200
}

type GetKarpenterLogsOutput struct {
    Logs            string   `json:"logs"`
    KarpenterVersion string  `json:"karpenter_version"`
    NodePoolNames   []string `json:"node_pool_names"`
}
```

> **Fonte dos logs**: o `kd-executor` coleta via `kubectl logs -n karpenter deployment/karpenter --tail=200` e envia ao `kd-gateway`, que repassa ao `kd-analyzer` como parte do payload.

### Variáveis de ambiente por modelo de agente

```bash
MODEL_LOG_ANALYST=databricks-claude-sonnet-4-6
MODEL_EVENT_ANALYST=databricks-claude-sonnet-4-6
MODEL_DESCRIBE_ANALYST=databricks-claude-sonnet-4-6
MODEL_KARPENTER_ANALYST=databricks-claude-sonnet-4-6
MODEL_REMEDY_ADVISOR=databricks-claude-sonnet-4-6
```

> Cada agente lê sua própria variável de modelo — permite usar modelos menores (mais baratos) para triagem e modelos maiores para análise profunda e remediação.

---

## Fluxo de Dados completo

### Fluxo proativo (watcher do kd-executor)

```
kd-executor detecta pod Pending / CrashLoopBackOff / OOMKilled
    │
    └─ envia via gRPC → kd-gateway
            │
            └─ kd-gateway decide: acionar kd-analyzer
                    │
                    └─ kd-analyzer.Analyze(AnalyzeRequest)
                            │
                            ├─ [Agente 1] log_analyst
                            │       → causa raiz + severidade
                            │
                            ├─ [Agente 2] event_analyst
                            │       → padrão de eventos
                            │
                            ├─ [Agente 3] describe_analyst
                            │       → estado do recurso
                            │       └─ pod Pending + taint events?
                            │               └─ Sim → [Agente 4] karpenter_analyst
                            │                           → logs karpenter
                            │                           → diagnóstico NodePool/EC2NodeClass
                            │
                            └─ [Agente 5] remedy_advisor
                                    → plano de remediação
                                    │
                                    └─ retorna AnalysisResult{
                                            Request:  <descrição do incidente>
                                            Analyze:  <causa raiz consolidada>
                                            Resolver: <plano de ação com kubectl>
                                            MemoryKey: cluster+env+namespace
                                       }
                                            │
                                            ├─ kd-gateway → notificação ao usuário
                                            └─ kd-store   → persiste histórico
```

### Fluxo sob demanda (usuário via kd-gateway / MCP)

```
Usuário / ferramenta → MCP → kd-gateway
    │
    └─ kd-gateway avalia payload → aciona kd-analyzer
            │
            └─ mesmo pipeline acima
                    └─ retorna AnalysisResult ao kd-gateway
                            └─ kd-gateway → responde ao usuário / MCP
```

---

## Estrutura de Resposta Padrão: `AnalysisResult`

O `kd-analyzer` **sempre** retorna a mesma estrutura de resposta, independente do agente ou pipeline executado. Isso garante consistência para o histórico de memória no `kd-store`.

```go
type AnalysisResult struct {
    // Campos obrigatórios — base do histórico de memória
    Request  string `json:"request"`  // descrição do que foi solicitado / incidente recebido
    Analyze  string `json:"analyze"`  // análise de causa raiz produzida pelos agentes
    Resolver string `json:"resolver"` // plano de remediação / ações recomendadas

    // Campos de contexto — enriquecem o histórico
    ClusterName  string    `json:"cluster_name"`
    Environment  string    `json:"environment"`
    Namespace    string    `json:"namespace"`
    ResourceKind string    `json:"resource_kind"` // Pod, Node, Ingress, ...
    ResourceName string    `json:"resource_name"`
    Severity     string    `json:"severity"`      // high | medium | low
    Source       string    `json:"source"`        // "kd-gateway" | "kd-executor"
    AnalyzedAt   time.Time `json:"analyzed_at"`

    // Índice de memória no kd-store
    MemoryKey    string    `json:"memory_key"`    // clusterName+environment+namespace
}
```

### Por que essa estrutura

| Campo | Papel no histórico |
|---|---|
| `Request` | Preserva o contexto original do incidente — permite reproduzir a análise |
| `Analyze` | Causa raiz identificada — comparada com incidentes anteriores para detectar padrões |
| `Resolver` | Ações tomadas / recomendadas — evita resugerir ações já tentadas |
| `MemoryKey` | Índice `clusterName+environment+namespace` — chave de busca no kd-store |
| `Source` | Permite distinguir análises proativas (executor) de análises sob demanda (gateway) |

---

## Índice de Histórico no kd-store

O histórico de incidentes é indexado por:

```
clusterName + Environment + Namespace
```

Exemplo: `runtime-testes:staging:envoy-gateway-system`

Isso permite que o agente `log_analyst` consulte os **2 incidentes anteriores** do mesmo contexto, enriquecendo a análise com padrões históricos.

---

## Estrutura de Diretórios Planejada

```
kd-analyzer/
├── cmd/
│   └── grpc/
│       └── main.go               # entry point gRPC
│
├── internal/
│   ├── core/
│   │   └── analyzer/
│   │       ├── agent/
│   │       │   ├── log_analyst.go
│   │       │   ├── event_analyst.go
│   │       │   ├── describe_analyst.go
│   │       │   ├── karpenter_analyst.go  # condicional: pod Pending + taint events
│   │       │   └── remedy_advisor.go
│   │       ├── tool/
│   │       │   ├── analyze_k8s_logs.go
│   │       │   ├── get_k8s_events.go
│   │       │   ├── describe_resource.go
│   │       │   ├── get_karpenter_logs.go # kubectl logs -n karpenter deployment/karpenter
│   │       │   └── get_incident_history.go
│   │       ├── result.go         # AnalysisResult struct (Request, Analyze, Resolver, ...)
│   │       ├── pipeline.go       # orquestra agentes + lógica condicional Karpenter
│   │       └── service.go        # interface pública do analyzer
│   │
│   └── infrastructure/
│       └── llm/
│           ├── databricks/
│           │   ├── model.go          # DatabricksModel (impl model.LLM)
│           │   ├── token_provider.go # static + M2M OAuth
│           │   ├── converters.go     # ADK ↔ OpenAI format
│           │   └── tools.go          # ADK tools → OpenAI tools
│           └── factory.go            # newModel(modelName) → model.LLM
│
├── pkg/
│   └── runner/
│       └── runner.go             # runAgent helper reutilizável
│
└── go.mod
```

---

## Bibliotecas Utilizadas

| Biblioteca | Versão | Propósito |
|---|---|---|
| `google.golang.org/adk` | v1.2.0 | Orquestrador de agentes LLM (runner, session, llmagent, functiontool) |
| `google.golang.org/genai` | v1.57.0 | Tipos genai (Content, Schema, Tool, GenerateContentConfig) |
| `github.com/openai/openai-go/v3` | v3.36.0 | Cliente HTTP OpenAI-compatible → API Databricks |
| `github.com/databricks/databricks-sdk-go` | v0.133.0 | SDK Databricks para autenticação M2M OAuth |

---

## Arquivos do Módulo (validate — protótipo atual)

| Arquivo | Responsabilidade |
|---|---|
| `services/validate/main.go` | Pipeline multi-agente: `log_analyst` + `remedy_advisor`, tool `analyze_k8s_logs`, `runAgent` helper |
| `databricksmodel/model.go` | `DatabricksModel`: implementa `model.LLM`, constrói client OpenAI, converte requests ADK→OpenAI |
| `databricksmodel/token_provider.go` | `TokenProvider`: cache de token com renovação automática M2M OAuth |
| `databricksmodel/converters.go` | Conversão de mensagens ADK (`genai.Content`) → OpenAI (`ChatCompletionMessageParamUnion`) |
| `databricksmodel/tools.go` | Conversão de tools ADK (`genai.Tool`) → OpenAI (`ChatCompletionToolUnionParam`) |
