# Convenções de Código e Estrutura — Kubediscovery

## Layout de Serviço (template obrigatório)

Todos os serviços seguem este layout. Desvios precisam de justificativa explícita.

```
services/<nome>/
├── go.mod                   # module github.com/kubediscovery/<nome>
├── go.sum
├── Makefile                 # targets: build, test, docker, run-local
├── Dockerfile
├── .env.example             # todas as variáveis necessárias documentadas
│
├── cmd/
│   └── grpc/                # ponto de entrada (ou http/ se for HTTP)
│       ├── main.go          # fx.New(...modules).Run()
│       ├── setup.go         # bootstrap FX + graceful shutdown
│       ├── providers.go     # providers globais registrados no FX
│       └── wire.go          # composição dos módulos FX
│
├── configs/
│   ├── config.go            # config raiz, leitura via viper
│   ├── grpc.go
│   ├── database.go          # se aplicável
│   ├── cache.go             # se aplicável
│   ├── llm.go               # se aplicável
│   └── observability.go
│
├── internal/
│   ├── core/
│   │   └── <domain>/
│   │       ├── entity/      # structs puras — SEM json/db tags
│   │       ├── service/     # lógica de negócio
│   │       ├── repository/  # interface.go + postgres.go (ou outra impl)
│   │       ├── handler/     # grpc_handler.go e/ou http_handler.go
│   │       └── module.go    # fx.Module — wires repo → svc → handler
│   │
│   └── infrastructure/
│       ├── grpc/
│       │   └── server.go    # mTLS + chain de interceptors
│       ├── database/
│       │   ├── postgres.go  # pgx pool
│       │   └── migrations.go
│       ├── cache/
│       │   └── redis.go
│       └── observability/
│           ├── otel.go
│           ├── prometheus.go
│           └── logger.go    # slog setup
│
└── pkg/                     # utilidades exportadas (errors, validator, response)
```

---

## Padrão UberFX por Domínio

Cada domínio expõe `var Module = fx.Module(...)` em `module.go`:

```go
// internal/core/cluster/module.go
var Module = fx.Module("cluster",
    fx.Provide(
        repository.NewPostgresRepository,
        service.NewClusterService,
        handler.NewGRPCHandler,
    ),
)
```

Composição no `cmd/grpc/wire.go`:

```go
func BuildApp() *fx.App {
    return fx.New(
        configs.Module,
        observability.Module,
        infrastructure.Module,
        cluster.Module,
        discovery.Module,
    )
}
```

---

## Entidades: Sem Tags de Framework

Entidades em `entity/` são structs puras — **sem json/db/proto tags**:

```go
// ✅ correto
type Cluster struct {
    ID          uuid.UUID
    Name        string
    Environment string
    Status      ClusterStatus
    CreatedAt   time.Time
}

// ❌ errado — entidade com tag de infraestrutura
type Cluster struct {
    ID   uuid.UUID `json:"id" db:"id"`
    Name string    `json:"name" db:"name"`
}
```

Tags pertencem a DTOs (`dto/`) e ao código gerado pelo sqlc/proto.

---

## Repositório: Sempre Interface + Implementação

```go
// repository/interface.go
type ClusterRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*entity.Cluster, error)
    Save(ctx context.Context, cluster *entity.Cluster) error
}

// repository/postgres.go
type postgresRepository struct { db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) ClusterRepository {
    return &postgresRepository{db: db}
}
```

---

## Configuração via Viper

```go
// configs/config.go
type Config struct {
    App           AppConfig
    GRPC          GRPCConfig
    Observability ObservabilityConfig
}

func New() (*Config, error) {
    viper.AutomaticEnv()
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    // ...
    var cfg Config
    return &cfg, viper.Unmarshal(&cfg)
}
```

- CLI: lê `~/.kubediscovery/config.yaml`
- Serviços deployados: apenas variáveis de ambiente
- Dev local: `.env` via `godotenv` (carregado em `setup.go`)

---

## Proto: Fluxo de Alteração

```
1. Editar  proto/<service>.proto  (ou libs/core/v1/proto/<service>.proto)
2. Rodar   make proto-gen
3. Código gerado vai para  libs/core/v1/<service>/*.pb.go
4. NUNCA editar *.pb.go diretamente
```

Regra de nomenclatura proto:
- Package: `kubediscovery.v1`
- Go package: `github.com/kubediscovery/kd-libs/core/v1/<service>`

---

## gRPC Handler: Interceptors Obrigatórios

```go
// internal/infrastructure/grpc/server.go
grpc.NewServer(
    grpc.Creds(tlsCreds),   // mTLS — obrigatório
    grpc.ChainUnaryInterceptor(
        loggingUnaryInterceptor,
        tracingUnaryInterceptor,  // OTEL
    ),
    grpc.ChainStreamInterceptor(
        loggingStreamInterceptor,
        tracingStreamInterceptor, // OTEL
    ),
)
```

`GRPC_DEBUG=1` adiciona interceptors de duração por chamada.

---

## Observabilidade: Inicialização Padrão

```go
// internal/infrastructure/observability/otel.go
func InitTracer(serviceName, endpoint string) (func(), error) {
    exporter, _ := otlptracehttp.New(ctx,
        otlptracehttp.WithEndpoint(endpoint),
    )
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.ServiceNameKey.String(serviceName),
        )),
    )
    otel.SetTracerProvider(tp)
    return func() { tp.Shutdown(ctx) }, nil
}
```

Prometheus: registrar métricas no `prometheus.DefaultRegisterer` e expor via handler no mux.

---

## Nomenclatura

| Conceito | Padrão |
|----------|--------|
| Módulo Go | `github.com/kubediscovery/<nome>` |
| Pacote de domínio | `internal/core/<domain>/` |
| Interface de repositório | `<Domain>Repository` |
| Implementação PostgreSQL | `postgres<Domain>Repository` |
| Handler gRPC | `<Domain>GRPCHandler` |
| Handler HTTP | `<Domain>HTTPHandler` |
| FX Module var | `var Module = fx.Module(...)` |
| Env var de serviço | `UPPER_SNAKE_CASE` |
| Config struct field | `PascalCase` com tag `mapstructure:"snake_case"` |

---

## Erros de Domínio

```go
// internal/core/<domain>/errors/errors.go
type DomainError struct {
    Code    string
    Message string
}

var (
    ErrClusterNotFound = &DomainError{Code: "CLUSTER_NOT_FOUND", Message: "cluster not found"}
    ErrInvalidStatus   = &DomainError{Code: "INVALID_STATUS", Message: "invalid cluster status"}
)
```

Mapear para status gRPC no handler (não no service):
```go
// grpc_handler.go
if errors.Is(err, domainerrors.ErrClusterNotFound) {
    return nil, status.Error(codes.NotFound, err.Error())
}
```

---

## Testes

| Tipo | Onde | Tag de build |
|------|------|--------------|
| Unitário | `*_test.go` junto ao código | nenhuma |
| Integração | `tests/integration/` | `//go:build integration` |
| E2E | `tests/e2e/` | `//go:build e2e` |

- Integração/E2E usam `testcontainers-go` com `postgres:15-alpine`
- Mocks escritos à mão em `tests/mocks/` — cada campo de função deve ser configurado por teste
- Handler tests: `gin.SetMode(gin.TestMode)` antes de `gin.New()` (se serviço tiver HTTP)
- Coverage: `go test -coverprofile=coverage.out ./internal/... && go tool cover -func=coverage.out`

---

## Makefile Targets Globais (raiz do monorepo)

| Target | Descrição |
|--------|-----------|
| `make proto-gen` | Regenera todo código Go a partir dos `.proto` |
| `make build-all` | Build de todos os serviços |
| `make test-all` | Testes unitários em todos os módulos |
| `make lint` | golangci-lint em todos os módulos |
