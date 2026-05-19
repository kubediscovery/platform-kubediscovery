# Arquitetura da Plataforma Kubediscovery

## Topologia Geral

```
┌──────────────────────────────────────────────────────────────────────┐
│                        CONTROL PLANE                                 │
│                                                                      │
│   [kdctl CLI]                                                        │
│       │  administração, certificados, UIDs                           │
│       ▼                                                              │
│   [kd-gateway]  ←── MCP / Usuário / Automação                       │
│       │  gRPC bidirecional (mTLS) ─────────────────────┐            │
│       │                                                 │            │
│       ├──→ [kd-analyzer]  (LLM pipeline)               │            │
│       │         └──→ [kd-store]  (histórico + memória) │            │
│       │                                                 │            │
│       └──→ [kd-store]  (state + cache Redis)            │            │
└─────────────────────────────────────────────────────────┼────────────┘
                                                          │ gRPC mTLS
┌─────────────────────────────────────────────────────────┼────────────┐
│  Cluster Remoto A                   DATA PLANE          │            │
│                                                         ▼            │
│   [Agent CRD] ──→ [kd-agent]  ←────────────────────────┘            │
│                       │                                              │
│                       ├──→ [kd-executor / WATCHER]  (client-go)     │
│                       │           │ detecta: OOMKilled, Pending...   │
│                       │           └──→ envia dados ao kd-gateway     │
│                       │                                              │
│                       └──→ [kd-executor / EXECUTOR]                  │
│                                   │ executa ações: scale, restart... │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Fluxos Principais

### Fluxo de Request do Usuário

```
USER / MCP → kd-gateway
               │ decide servidor destino
               ▼
          kd-agent (cluster alvo)
               │ decide a action
               ▼
          kd-executor (executa)
               │ retorna resultado
               ▼
          kd-agent → kd-gateway → usuário
```

### Fluxo de Identificação de Problemas (automático)

```
kd-executor (watcher detecta anomalia)
    → kd-agent
        → kd-gateway
            → kd-analyzer (LLM pipeline)
                → AnalysisResult
                    → kd-gateway notifica usuário
                    → kd-store persiste histórico
```

---

## Kubernetes Operator (Agent CRD)

O Data Plane é gerenciado por um **Kubernetes Controller** que controla instâncias de cada componente:

```yaml
apiVersion: kubediscovery.io/v1beta1
kind: Agent
metadata:
  name: agent-srv001
spec:
  agent:
    enabled: true
  executor:
    enabled: true      # habilita EXECUTOR + WATCHER
    watcher: true      # watcher só funciona se executor.enabled=true
  analyzer:
    enabled: false     # kd-analyzer local (edge LLM) — opcional
  troubleshootingImage:
    enabled: false
```

> Se `executor.enabled: false`, o `watcher` não funciona (é funcionalidade do executor).

---

## kd-gateway — Identidade dos Clientes

Três fontes de identidade, em ordem de prioridade:

| Prioridade | Fonte | Confiabilidade |
|------------|-------|----------------|
| 1 | `caller_id` (campo proto) | Auto-reportado — chave do mapa em memória |
| 2 | `metadata` (google.protobuf.Struct) | Logado, não confiável para auth |
| 3 | Certificado mTLS (CN/SAN) | **Confiável** — emitido pelo CA do `kdctl init` |

Resolução do `cert_name`: SAN URI > SAN DNS > CommonName (CN).

### Estado em memória do healthService

```go
type healthService struct {
    mu              sync.RWMutex
    lastByCaller    map[string]*HealthCheckRequest   // keyed by caller_id
    clientConnected map[string]*HealthCheckRequest   // keyed by caller_id
}
```

---

## kd-agent — Goroutines e Retry

O agente mantém **3 goroutines** simultâneas:

| Goroutine | Função |
|-----------|--------|
| **Sender** | Envia `HealthCheckRequest` com `caller_id`, `metadata`, `sent_at` |
| **Ticker** | Controla intervalo de envio |
| **Receiver** | Recebe `HealthCheckResponse` do gateway |

**Retry exponencial**: base 1s, peso 3 → delays: 1s → 3s → 9s → 27s → 81s → **fatal**.

`AGENT_ID` (env var) → `caller_id` em **todas** as mensagens. Deve ser único por instância.

---

## kd-analyzer — Pipeline LLM

Cinco agentes em sequência, cada um com sessão isolada (sem histórico compartilhado):

| Agente | Trigger | Env var modelo |
|--------|---------|----------------|
| `log_analyst` | sempre | `MODEL_LOG_ANALYST` |
| `event_analyst` | sempre | `MODEL_EVENT_ANALYST` |
| `describe_analyst` | sempre | `MODEL_DESCRIBE_ANALYST` |
| `karpenter_analyst` | Pod `Pending` + eventos com "untolerated taint" / "0/N nodes available" / "FailedScheduling" | `MODEL_KARPENTER_ANALYST` |
| `remedy_advisor` | sempre (usa saída dos anteriores) | `MODEL_REMEDY_ADVISOR` |

**Output sempre retornado**:
```go
type AnalysisResult struct {
    Request      string
    Analyze      string
    Resolver     string
    ClusterName  string
    Environment  string
    Namespace    string
    ResourceKind string
    ResourceName string
    Severity     string
    Source       string
    AnalyzedAt   time.Time
    MemoryKey    string  // = clusterName + environment + namespace
}
```

---

## kd-executor — Ações Disponíveis

| ActionType | Descrição |
|------------|-----------|
| `scale` | Escala Deployment/StatefulSet |
| `restart` | Rollout restart |
| `patch_resources` | Patch de resources (CPU/memory limits) |
| `cordon_node` | Cordona node |
| `drain_node` | Drena node |
| `exec` | Executa comando em container |
| `logs` | Coleta logs |
| `describe` | Describe de recurso |
| `delete_pod` | Delete de pod |
| `apply_manifest` | Aplica manifest YAML |
| `karpenter_logs` | Logs do Karpenter |

**Dispatcher**: `Registry map[ActionType]Executor` — cada ação implementa `Executor` interface:
```go
type Executor interface {
    Execute(ctx context.Context, action ExecutorAction) (ExecutorResult, error)
    ActionType() string
}
```

---

## kd-store — Indexação de Histórico

Chave de memória/sessão para buscas históricas:
```
MemoryKey = clusterName + environment + namespace
```

O `kd-analyzer` sempre retorna `MemoryKey` no `AnalysisResult` para que `kd-store` possa indexar e consultas futuras recuperem o histórico relevante do contexto.

---

## Gestão de Certificados (kdctl)

```bash
# 1. Inicializa CA + cert do servidor
kdctl init --name <name> --address <host:port> --environment <env>
# Gera: ~/.kubediscovery/certs/<environment>/{ca.crt, ca.key, server.crt, server.key}

# 2. Gera cert de cliente para kd-agent
kdctl certificate --create --name client-<agent> --environment <env> --client
# --client é auto-detectado se o nome começa com "client-"

# 3. Lista todos os certs
kdctl certificate --list
```

---

## Observabilidade (obrigatória em todos os serviços)

- **Prometheus**: endpoint `/metrics` exposto no server mux
- **OpenTelemetry**: OTLP HTTP exporter — traces iniciam em cada handler gRPC/HTTP e propagam via metadata
- **Logger**: `slog` (Go standard library)
- **Variável**: `OTEL_EXPORTER_OTLP_ENDPOINT`
