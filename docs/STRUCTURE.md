# ESTRUTURA DO PROJETO: platform-kubediscovery

## Estratégia: Monorepo com Go Workspace

Cada serviço é um **módulo Go independente** com seu próprio `go.mod`, empacotado e deployado separadamente. O `go.work` na raiz conecta todos os módulos para desenvolvimento local, eliminando a necessidade de publicar libs entre PRs.

```
go.work resolve: libs/ ← services/kd-gateway, services/kd-agent, etc.
```

---

## Visão Geral da Estrutura

```
platform-kubediscovery/
│
├── go.work                          # Go workspace — une todos os módulos
├── go.work.sum
├── Makefile                         # targets globais: build-all, test-all, proto-gen
│
├── libs/                            # Bibliotecas compartilhadas entre todos os serviços
├── services/                        # Serviços Go (cada um = módulo + app independente)
│   ├── kd-gateway/                  # Control Plane: roteador gRPC central
│   ├── kd-analyzer/                 # Control Plane: pipeline LLM / AIOps
│   ├── kd-executor/                 # Data Plane: watcher + executor de comandos K8s
│   ├── kd-agent/                    # Data Plane: cliente gRPC persistente
│   └── kd-store/                    # Control Plane: API de persistência (PostgreSQL + Redis)
│
├── cli/
│   └── kdctl/                       # CLI de administração (cobra + huh TUI)
│
├── frontend/
│   └── kd-portal/                   # Dashboard web (React/Vite)
│
├── proto/                           # Fonte canônica dos arquivos .proto
├── infra/                           # Infraestrutura: K8s manifests, Helm, Terraform, Docker
├── scripts/                         # Scripts de automação
└── docs/                            # Documentação técnica
```

---

## `go.work` — Go Workspace

```
// go.work
go 1.26

use (
    ./libs
    ./services/kd-gateway
    ./services/kd-analyzer
    ./services/kd-executor
    ./services/kd-agent
    ./services/kd-store
    ./cli/kdctl
)
```

---

## `libs/` — Bibliotecas Compartilhadas

```
libs/
├── go.mod                           # module github.com/kubediscovery/kd-libs
│
├── core/
│   └── v1/
│       ├── proto/
│       │   ├── health.proto         # HealthService (bidirecional stream)
│       │   ├── executor.proto       # ExecutorAction / ExecutorResult
│       │   └── analyzer.proto       # AnalysisRequest / AnalysisResult
│       ├── health/
│       │   ├── health.pb.go         # gerado — NÃO editar manualmente
│       │   └── health_grpc.pb.go    # gerado — NÃO editar manualmente
│       ├── executor/
│       │   ├── executor.pb.go
│       │   └── executor_grpc.pb.go
│       └── configPath/
│           └── configPath.go        # ~/.kubediscovery path helpers
│
├── sslGenerate/                     # Geração de CA e certificados TLS (x509)
│   ├── ca.go
│   ├── server.go
│   ├── client.go
│   ├── storage.go
│   └── types.go
│
└── forms/                           # TUI components (huh/lipgloss)
    ├── table.go
    ├── notification.go
    ├── layout.go
    └── types.go
```

> Toda mudança em `.proto` exige rodar `make proto-gen` na raiz para regenerar o código Go em `libs/core/v1/`.

---

## Template de Serviço Go

Todos os serviços seguem o mesmo layout interno. Use como referência ao criar um novo serviço:

```
services/<nome-do-servico>/
│
├── go.mod                           # module github.com/kubediscovery/<nome>
├── go.sum
├── Makefile                         # build, test, docker, run-local
├── Dockerfile
├── .env.example
│
├── cmd/
│   └── grpc/                        # ou http/, ou app/ dependendo do tipo
│       ├── main.go                  # entry point — wires UberFX providers
│       ├── setup.go                 # bootstrap FX + graceful shutdown
│       ├── providers.go             # providers globais registrados no FX
│       └── wire.go                  # composição dos módulos FX
│
├── configs/
│   ├── config.go                    # Config raiz + carregamento via spf13/viper
│   ├── grpc.go                      # GRPCConfig
│   ├── database.go                  # DatabaseConfig (se aplicável)
│   ├── cache.go                     # RedisConfig (se aplicável)
│   ├── llm.go                       # LLMConfig (se aplicável)
│   └── observability.go             # OTelConfig + PrometheusConfig
│
├── internal/
│   ├── core/                        # Domínio de negócio — sem dependências de infra
│   │   └── <dominio>/
│   │       ├── entity/
│   │       │   └── <dominio>.go     # structs puras, sem tags json/db
│   │       ├── service/
│   │       │   └── service.go       # lógica de negócio + validação
│   │       ├── repository/
│   │       │   ├── interface.go     # contrato (interface Go)
│   │       │   └── postgres.go      # implementação PostgreSQL
│   │       ├── handler/
│   │       │   ├── grpc_handler.go  # handler gRPC
│   │       │   └── http_handler.go  # handler HTTP (se aplicável)
│   │       └── module.go            # FX Module: wire repo → svc → handler
│   │
│   └── infrastructure/
│       ├── grpc/
│       │   ├── server.go            # configuração do servidor gRPC (TLS/mTLS)
│       │   └── middleware/
│       │       └── interceptor.go   # UnaryInterceptor + StreamInterceptor
│       ├── http/
│       │   ├── server.go
│       │   └── middleware/
│       │       ├── auth.go
│       │       └── logger.go
│       ├── database/
│       │   ├── postgres.go          # pool de conexões pgx
│       │   └── migrations.go        # golang-migrate
│       ├── cache/
│       │   └── redis.go             # go-redis
│       └── observability/
│           ├── otel.go              # OpenTelemetry tracer + exporter OTLP
│           ├── prometheus.go        # métricas Prometheus
│           └── logger.go            # structured logger (slog)
│
└── pkg/                             # Utilitários exportáveis do serviço
    ├── errors/
    ├── validator/
    └── response/
```

---

## Serviços — Detalhamento

### `services/kd-gateway/` — Control Plane: Roteador gRPC

```
kd-gateway/
├── cmd/grpc/main.go
├── configs/
│   ├── config.go
│   └── grpc.go
├── internal/
│   ├── core/
│   │   ├── cluster/                 # entity, service, repository, handler gRPC, module
│   │   ├── health/                  # HealthService bidirecional (clientes conectados)
│   │   └── routing/                 # lógica de decisão: qual serviço acionar
│   └── infrastructure/
│       ├── grpc/server.go           # mTLS + interceptors
│       ├── middleware/grpc/
│       │   └── interceptor.go
│       └── datastore/               # kd-store client
└── pkg/grpcserver/
    └── grpc.go                      # NewServer, Run, mTLS, interceptor chain
```

---

### `services/kd-analyzer/` — Control Plane: LLM / AIOps

```
kd-analyzer/
├── cmd/grpc/main.go
├── configs/
│   ├── config.go
│   └── llm.go                       # DatabricksConfig, modelos por agente
├── internal/
│   ├── core/analyzer/
│   │   ├── agent/
│   │   │   ├── log_analyst.go
│   │   │   ├── event_analyst.go
│   │   │   ├── describe_analyst.go
│   │   │   ├── karpenter_analyst.go  # condicional: pod Pending + taint
│   │   │   └── remedy_advisor.go
│   │   ├── tool/
│   │   │   ├── analyze_k8s_logs.go
│   │   │   ├── get_k8s_events.go
│   │   │   ├── describe_resource.go
│   │   │   ├── get_karpenter_logs.go
│   │   │   └── get_incident_history.go
│   │   ├── result.go                 # AnalysisResult{Request, Analyze, Resolver, ...}
│   │   ├── pipeline.go               # orquestra agentes + lógica condicional Karpenter
│   │   └── service.go                # interface pública + FX Module
│   └── infrastructure/llm/
│       └── databricks/
│           ├── model.go              # DatabricksModel (impl model.LLM do ADK)
│           ├── token_provider.go     # static + M2M OAuth (cache + renovação automática)
│           ├── converters.go         # ADK ↔ OpenAI message format
│           └── tools.go              # ADK tools → OpenAI tools format
└── pkg/runner/
    └── runner.go                     # runAgent helper (sessão isolada por execução)
```

---

### `services/kd-executor/` — Data Plane: Watcher + Executor

```
kd-executor/
├── cmd/grpc/main.go
├── configs/
│   ├── config.go
│   └── watcher.go                   # thresholds, namespaces, resync period
├── internal/
│   ├── core/
│   │   ├── executor/
│   │   │   ├── interface.go         # Executor interface + ExecutorAction + ExecutorResult
│   │   │   ├── scale.go
│   │   │   ├── restart.go
│   │   │   ├── patch_resources.go
│   │   │   ├── cordon_node.go
│   │   │   ├── drain_node.go
│   │   │   ├── exec.go
│   │   │   ├── logs.go
│   │   │   ├── describe.go
│   │   │   ├── delete_pod.go
│   │   │   ├── apply_manifest.go
│   │   │   ├── karpenter_logs.go
│   │   │   └── registry.go          # map[ActionType]Executor — dispatcher
│   │   └── watcher/
│   │       ├── watcher.go           # start/stop SharedInformerFactory
│   │       ├── pod_handler.go
│   │       ├── node_handler.go
│   │       ├── ingress_handler.go
│   │       ├── event_handler.go
│   │       ├── trigger.go           # condição → WatcherPayload
│   │       └── collector.go         # logs, describe, events via client-go
│   └── infrastructure/
│       ├── k8s/
│       │   ├── client.go            # *kubernetes.Clientset (in-cluster ou kubeconfig)
│       │   └── informer_factory.go
│       └── grpc/
│           └── client.go            # cliente gRPC → kd-agent
```

---

### `services/kd-agent/` — Data Plane: Cliente gRPC Persistente

```
kd-agent/
├── cmd/grpc/main.go                 # retry loop, mTLS, HealthStream bidirecional
└── configs/
    └── config.go                    # AGENT_ID, GRPC_ADDR, certs
```

---

### `services/kd-store/` — Control Plane: Persistência

```
kd-store/
├── cmd/
│   ├── grpc/main.go                 # API gRPC interna (para kd-gateway, kd-analyzer)
│   └── http/main.go                 # API HTTP (para kd-portal dashboard)
├── configs/
│   ├── database.go
│   └── cache.go
├── internal/
│   ├── core/
│   │   ├── cluster/                 # CRUD de clusters registrados
│   │   ├── incident/                # histórico de AnalysisResult (memória do analyzer)
│   │   └── certificate/             # registro de certificados emitidos
│   └── infrastructure/
│       ├── database/
│       │   ├── postgres.go
│       │   └── migrations/
│       │       ├── 001_clusters.sql
│       │       ├── 002_incidents.sql
│       │       └── 003_certificates.sql
│       └── cache/
│           └── redis.go
```

---

### `cli/kdctl/` — CLI de Administração

```
cli/kdctl/
├── go.mod                           # module github.com/kubediscovery/kdctl
├── main.go
├── cmd/
│   ├── root.go                      # rootCmd (cobra)
│   ├── initconfig.go                # kdctl init
│   ├── certificate.go               # kdctl certificate
│   ├── server.go                    # kdctl server
│   ├── client.go                    # kdctl client
│   ├── init/                        # lógica do init (TUI huh, sslGenerate)
│   │   ├── init.go
│   │   ├── form.go
│   │   └── types.go
│   └── server/
│       ├── certificate.go           # kdctl server certificate
│       └── client.go                # kdctl server client
└── internal/
    ├── service/server/
    │   └── certificate.go           # ListAllCertificates()
    └── infrastructure/certgen/      # certgen helpers (client certs)
```

---

## `proto/` — Fonte Canônica dos Contratos gRPC

```
proto/
└── kubediscovery/
    └── v1/
        ├── health.proto             # HealthService (kd-agent ↔ kd-gateway)
        ├── executor.proto           # ExecutorAction / ExecutorResult (kd-gateway → kd-executor)
        ├── analyzer.proto           # AnalysisRequest / AnalysisResult (kd-gateway → kd-analyzer)
        └── store.proto              # CRUD de clusters / incidents (kd-gateway → kd-store)
```

> O código gerado vai para `libs/core/v1/<serviço>/`. Nunca editar manualmente.

---

## `infra/` — Infraestrutura

```
infra/
├── k8s/
│   ├── crds/
│   │   └── agent.kubediscovery.io_v1beta1.yaml   # Agent CRD (habilita kd-executor, kd-agent)
│   ├── control-plane/
│   │   ├── kd-gateway-deployment.yaml
│   │   ├── kd-analyzer-deployment.yaml
│   │   └── kd-store-deployment.yaml
│   └── data-plane/
│       └── operator/                              # Kubernetes Operator (controller do Agent CRD)
│
├── helm/
│   ├── kd-control-plane/            # chart: gateway + analyzer + store + cache
│   └── kd-data-plane/               # chart: operator + agent + executor
│
├── docker/
│   ├── kd-gateway.Dockerfile
│   ├── kd-analyzer.Dockerfile
│   ├── kd-executor.Dockerfile
│   ├── kd-agent.Dockerfile
│   └── docker-compose.yml           # dev local: gateway + store + redis + postgres
│
└── terraform/
    └── aws/                         # EKS, RDS, ElastiCache, IAM (Service Account)
```

---

## `scripts/` — Automação

```
scripts/
├── proto-gen.sh           # protoc → gera código em libs/core/v1/
├── build-all.sh           # go build para todos os serviços
├── test-all.sh            # go test ./... em todos os módulos
├── lint-all.sh            # golangci-lint em todos os módulos
└── dev-setup.sh           # instala ferramentas: protoc, buf, air, golangci-lint
```

---

## Padrões Transversais

### Dependency Injection: UberFX

Cada serviço usa `go.uber.org/fx` para compor módulos. O `main.go` apenas declara o `fx.App`:

```go
// cmd/grpc/main.go
func main() {
    fx.New(
        configs.Module,
        observability.Module,
        infrastructure.Module,
        core.Module,
    ).Run()
}
```

Cada domínio expõe um `fx.Module` em `module.go`:

```go
// internal/core/cluster/module.go
var Module = fx.Module("cluster",
    fx.Provide(repository.NewPostgresRepository),
    fx.Provide(service.NewClusterService),
    fx.Provide(handler.NewGRPCHandler),
)
```

### Configuração: spf13/viper

| Contexto | Fonte de configuração |
|---|---|
| CLI (`kdctl`) | `~/.kubediscovery/config.yaml` |
| Serviços (deploy) | Variáveis de ambiente |
| Dev local | `.env` (carregado via `godotenv`) |

### Observabilidade: OpenTelemetry + Prometheus

Todo serviço inclui:

```go
// infrastructure/observability/otel.go
// - Tracer OTLP HTTP exporter
// - Spans iniciados em cada handler gRPC/HTTP
// - Propagação de trace via gRPC metadata

// infrastructure/observability/prometheus.go
// - /metrics endpoint
// - Métricas: request_total, request_duration_seconds, active_connections
```

### Módulo Go por serviço

| Serviço | Módulo Go |
|---|---|
| `libs` | `github.com/kubediscovery/kd-libs` |
| `kd-gateway` | `github.com/kubediscovery/kd-gateway` |
| `kd-analyzer` | `github.com/kubediscovery/kd-analyzer` |
| `kd-executor` | `github.com/kubediscovery/kd-executor` |
| `kd-agent` | `github.com/kubediscovery/kd-agent` |
| `kd-store` | `github.com/kubediscovery/kd-store` |
| `kdctl` | `github.com/kubediscovery/kdctl` |

---

## Documentação dos Módulos

| Serviço | Documento |
|---|---|
| CLI | [KDCTL.md](KDCTL.md) |
| Gateway gRPC | [KD-GATEWAY.md](KD-GATEWAY.md) |
| Agente Data Plane | [KD-AGENT.md](KD-AGENT.md) |
| Executor + Watcher | [KD-EXECUTOR.md](KD-EXECUTOR.md) |
| Analyzer LLM | [KD-ANALYZER.md](KD-ANALYZER.md) |
