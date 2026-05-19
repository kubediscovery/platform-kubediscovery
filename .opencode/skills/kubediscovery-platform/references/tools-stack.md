# Ferramentas e Stack Tecnológica — Kubediscovery

## Versões Base

| Item | Versão |
|------|--------|
| Go | `1.26.1` |
| gRPC | `google.golang.org/grpc v1.80.0` |
| Google ADK | `google.golang.org/adk v1.2.0` |
| Cobra | `github.com/spf13/cobra v1.10.2` |
| Huh TUI | `charm.land/huh/v2 v2.0.3` |
| UberFX | `go.uber.org/fx` (latest stable) |
| Viper | `github.com/spf13/viper` |

---

## ✅ Ferramentas OBRIGATÓRIAS

### Comunicação

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| `google.golang.org/grpc` | Toda comunicação entre serviços | Exclusivamente bidirecional (`stream`) |
| mTLS (crypto/tls + x509) | Autenticação de serviços | Certificados gerados pelo `kdctl init` / `kdctl certificate` |
| `google.golang.org/protobuf` | Serialização de mensagens | Sempre via proto definido em `proto/` ou `libs/core/v1/proto/` |

### MCP (Model Context Protocol)

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| `github.com/mark3labs/mcp-go v0.54.0` | Servidor MCP no `kd-gateway` | Expõe tools Kubernetes ao LLM via protocolo MCP |
| `mcp.NewTool` | Definição de tools MCP | Cada tool tem nome, description e parâmetros tipados |
| `server.NewMCPServer` | Instância do servidor MCP | Inicializado com `Name`, `Version` e `WithInstructions` (system prompt das tools) |
| `server.ServeStdio` | Transport stdio | Integração com agentes locais (Claude Desktop, VS Code Copilot como processo filho) |
| `server.NewSSEServer` | Transport HTTP/SSE | Integração remota via HTTP — clientes se conectam por URL (ex: `http://kd-gateway:8080/sse`) |

### Quando usar cada transport

| Transport | Quando usar | Exemplo de cliente |
|-----------|-------------|-------------------|
| `ServeStdio` | MCP rodando como processo local; cliente inicia o processo | Claude Desktop, VS Code Copilot (extensão MCP local) |
| `NewSSEServer` (HTTP) | MCP exposto como serviço HTTP na rede; múltiplos clientes simultâneos | Agentes remotos, dashboards, integrações via URL |

> **No Kubediscovery**: o `kd-gateway` deve suportar **ambos os transports**. `ServeStdio` para uso local/dev; `NewSSEServer` para o serviço deployado no Control Plane.

**Transport stdio:**
```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

s := server.NewMCPServer("Kubediscovery", "0.1.0",
    server.WithInstructions("<system prompt das tools>"),
)
s.AddTool(mcp.NewTool("get_pods",
    mcp.WithDescription("List pods in the specified namespace"),
    mcp.WithString("namespace", mcp.Required()),
), handleGetPods)

// Modo stdio — processo filho iniciado pelo cliente MCP
if err := server.ServeStdio(s); err != nil {
    log.Fatal(err)
}
```

**Transport HTTP/SSE:**
```go
// Modo HTTP — serviço exposto na rede
sseServer := server.NewSSEServer(s,
    server.WithBaseURL("http://kd-gateway:8080"),
)
// Clientes conectam em: http://kd-gateway:8080/sse
if err := sseServer.Start(":8080"); err != nil {
    log.Fatal(err)
}
```

### Dependency Injection e Ciclo de Vida

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| `go.uber.org/fx` | DI em todos os serviços | `fx.Module` por domínio, lifecycle via `OnStart`/`OnStop` |
| Nunca usar `init()` para dependências | Anti-pattern | Usar `fx.Provide` e `fx.Lifecycle` |
| Nunca usar variáveis globais para deps | Anti-pattern | Injetar via construtor |

### Configuração

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| `github.com/spf13/viper` | Config em todos os serviços | CLI: `~/.kubediscovery/config.yaml`; serviços: env vars; dev: `.env` via `godotenv` |
| `github.com/joho/godotenv` | Dev local | Carrega `.env` / `.env.dev` automaticamente |

### CLI

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| `github.com/spf13/cobra` | Árvore de comandos do `kdctl` | Exclusivo para o CLI; serviços não usam cobra |
| `charm.land/huh/v2` | TUI interativa no `kdctl init` | Forms e prompts interativos |
| `github.com/charmbracelet/lipgloss` | Estilização de output TUI | Junto com huh |

### LLM / IA

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| `google.golang.org/adk` | Orquestrador de agentes LLM | `llmagent.New`, `runner.New`, `session.InMemoryService`, `functiontool.New` |
| Databricks API | Provedor LLM | Endpoint OpenAI-compatible: `https://<host>/serving-endpoints/chat/completions` |
| `DatabricksModel` (custom) | Implementa `model.LLM` do ADK | Bridge ADK ↔ Databricks |
| Auth estático: `DATABRICKS_TOKEN` | Dev/staging | PAT token |
| Auth M2M OAuth: `DATABRICKS_SP_CLIENT_ID` + `DATABRICKS_SP_CLIENT_SECRET` | Produção | M2M tem prioridade se ambos definidos |

### Kubernetes

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| `k8s.io/client-go` | kd-executor (WATCHER + EXECUTOR) | Informers para Pod, Node, Ingress, Events |
| `sigs.k8s.io/controller-runtime` | Agent CRD Operator | Gerencia instâncias kd-agent/executor/analyzer no cluster |

### Observabilidade

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| `github.com/prometheus/client_golang` | Métricas em todos os serviços | Endpoint `/metrics` obrigatório |
| `go.opentelemetry.io/otel` | Tracing em todos os serviços | OTLP HTTP exporter, traces iniciam no handler |
| `log/slog` (stdlib) | Logging estruturado | Padrão Go, sem biblioteca externa |

### Autorização

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| OPA (Open Policy Agent) | RBAC/ABAC no `kd-gateway` | Recomendado; políticas em Rego |
| Rego | Linguagem de políticas OPA | Avalia: verbo + kind + namespace + cluster |

### Banco de Dados e Cache

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| PostgreSQL | `kd-store` (persistência) | pgx pool |
| Redis | `kd-store` (cache) | go-redis |
| `golang-migrate` | Migrations do kd-store | Aplicadas via `database/migrations.go` |

### Certificados TLS

| Ferramenta | Uso | Notas |
|-----------|-----|-------|
| `crypto/x509` + `crypto/tls` (stdlib) | Geração de CA e certs | Via `libs/sslGenerate/` |
| `sslGenerate.NewClientCertificate` | Cert de cliente (kd-agent) | Auto-detectado se nome começa com `client-` |
| `sslGenerate.NewServerCertificate` | Cert de servidor (kd-gateway) | Gerado por `kdctl server certificate` |

---

## ❌ Ferramentas PROIBIDAS / Anti-patterns

| Proibição | Motivo |
|-----------|--------|
| **Editar `*.pb.go` manualmente** | Código gerado — será sobrescrito por `make proto-gen`. Edite sempre o `.proto` |
| **HTTP/REST entre Control Plane e Data Plane** | Toda comunicação deve ser gRPC bidirecional com mTLS |
| **gRPC sem mTLS em produção** | Inseguro — toda conexão requer certificado validado pelo CA |
| **Inicializar dependências em `init()`** | Viola o padrão UberFX — use `fx.Provide` |
| **Variáveis globais para serviços/repositórios** | Viola DI — injetar via construtor |
| **Chamar `kd-analyzer` diretamente** | Deve sempre passar pelo `kd-gateway` |
| **OpenFGA** | Para ABAC baseado em K8s verbs/kinds, OPA + Rego é o fit correto. OpenFGA (ReBAC) é para hierarquias relacionais, não o caso aqui |
| **Compartilhar histórico de sessão entre agentes LLM** | Cada agente ADK roda em sessão isolada — output passa manualmente como input do próximo |
| **Usar o mesmo `AGENT_ID` em múltiplas instâncias** | Colide no mapa em memória do gateway (`lastByCaller`, `clientConnected`) |
| **Arquivos `.proto` fora de `proto/` ou `libs/core/v1/proto/`** | Fonte canônica é única — protos dispersos causam dessincronia |
| **Ativar `watcher` sem `executor.enabled: true` no CRD** | O watcher é funcionalidade do executor; não funciona independentemente |
| **Tokens de autenticação em código fonte** | Usar variáveis de ambiente ou secrets manager |

---

## Variáveis de Ambiente por Serviço

### kd-gateway

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `GRPC_ADDR` | `:50051` | Endereço de escuta |
| `GRPC_MTLS` | `1` | Habilita mTLS (obrigatório em prod) |
| `GRPC_CLIENT_CA_FILE` | — | Caminho do CA para validar clientes |
| `GRPC_DEBUG` | `0` | Habilita interceptors de debug por chamada |

### kd-agent

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `AGENT_ID` | `kd-agent` | **Deve ser único por instância** → vira `caller_id` |
| `GRPC_ADDR` | `localhost:50051` | Endereço do kd-gateway |
| `GRPC_CA_FILE` | `~/.kubediscovery/certs/staging/ca.crt` | CA para validar o servidor |
| `GRPC_CLIENT_CERT_FILE` | `~/.kubediscovery/certs/staging/srv004.crt` | Cert mTLS do agente |
| `GRPC_CLIENT_KEY_FILE` | `~/.kubediscovery/certs/staging/srv004.key` | Chave mTLS do agente |

### kd-analyzer

| Variável | Descrição |
|----------|-----------|
| `DATABRICKS_HOST` | Workspace Databricks (`sem https://`) |
| `DATABRICKS_ENDPOINT_NAME` | Nome do endpoint (ex: `databricks-claude-sonnet-4-6`) |
| `DATABRICKS_TOKEN` | Token estático (dev) |
| `DATABRICKS_SP_CLIENT_ID` | Client ID para M2M OAuth (prod) |
| `DATABRICKS_SP_CLIENT_SECRET` | Client Secret para M2M OAuth (prod) |
| `MODEL_LOG_ANALYST` | Modelo para o log_analyst |
| `MODEL_EVENT_ANALYST` | Modelo para o event_analyst |
| `MODEL_DESCRIBE_ANALYST` | Modelo para o describe_analyst |
| `MODEL_KARPENTER_ANALYST` | Modelo para o karpenter_analyst |
| `MODEL_REMEDY_ADVISOR` | Modelo para o remedy_advisor |

---

## Módulos Go — Nomes Canônicos

| Caminho | Módulo Go |
|---------|-----------|
| `libs/` | `github.com/kubediscovery/kd-libs` |
| `services/kd-gateway/` | `github.com/kubediscovery/kd-gateway` |
| `services/kd-agent/` | `github.com/kubediscovery/kd-agent` |
| `services/kd-analyzer/` | `github.com/kubediscovery/kd-analyzer` |
| `services/kd-executor/` | `github.com/kubediscovery/kd-executor` |
| `services/kd-store/` | `github.com/kubediscovery/kd-store` |
| `cli/kdctl/` | `github.com/kubediscovery/kdctl` |
