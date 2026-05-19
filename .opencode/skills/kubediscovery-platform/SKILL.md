---
name: kubediscovery-platform
description: >
  Conhecimento completo da plataforma Kubediscovery — arquitetura, serviços, ferramentas obrigatórias e proibidas,
  convenções de código e padrões de desenvolvimento. Use quando: implementar qualquer serviço do ecossistema
  (kd-gateway, kd-agent, kd-analyzer, kd-executor, kd-store, kdctl); criar novos módulos ou libs compartilhadas;
  tomar decisões de design de comunicação gRPC/mTLS; integrar LLM via Google ADK e Databricks; configurar
  UberFX, viper, cobra, observabilidade; revisar ou criar proto files; implementar o Kubernetes Operator (CRD Agent).
  Triggers: kubediscovery, kd-gateway, kd-agent, kd-analyzer, kd-executor, kd-store, kdctl, platform_kd, kd-libs.
user-invocable: true
---

# Kubediscovery Platform

Plataforma de gerenciamento e orquestração distribuída para ambientes Kubernetes.
Arquitetura **Control Plane / Data Plane** com comunicação exclusivamente via **gRPC bidirecional com mTLS**.

> Para decisões de design, comece sempre pelos docs canônicos em `docs/` antes de assumir comportamentos.
> Os arquivos `.proto` vivem em `proto/` (ou `libs/core/v1/proto/`). **Nunca edite arquivos `*.pb.go` gerados manualmente.**

---

## Planos e Referências

| Tópico | Arquivo |
|--------|---------|
| Arquitetura geral, fluxos e serviços | [./references/architecture.md](./references/architecture.md) |
| Ferramentas obrigatórias e proibidas | [./references/tools-stack.md](./references/tools-stack.md) |
| Convenções de código e estrutura | [./references/conventions.md](./references/conventions.md) |

---

## Resumo dos Serviços

| Serviço | Plano | Responsabilidade |
|---------|-------|-----------------|
| `kd-gateway` | Control Plane | Ponto central gRPC — roteamento, identidade mTLS, orquestração |
| `kd-analyzer` | Control Plane | Pipeline LLM (Google ADK + Databricks) — análise de causa raiz |
| `kd-store` | Control Plane | Persistência (PostgreSQL) + cache (Redis) + histórico de análises |
| `kd-agent` | Data Plane | Cliente gRPC persistente — conecta o cluster remoto ao gateway |
| `kd-executor` | Data Plane | Watcher (client-go informers) + Executor de ações Kubernetes |
| `kdctl` | CLI | Gerenciamento: init CA, certificados, clusters, kdctl certificate |
| `kd-libs` | Shared lib | Proto gerado, sslGenerate, forms TUI — `github.com/kubediscovery/kd-libs` |

---

## Regras Invioláveis

1. **Toda comunicação Control Plane ↔ Data Plane é gRPC bidirecional com mTLS.** Sem exceções.
2. **Nunca editar `*.pb.go` manualmente** — rodar `make proto-gen` após mudar `.proto`.
3. **`kd-analyzer` não é chamado diretamente** — apenas via `kd-gateway` (origin: `kd-executor` watcher ou request externa).
4. **`AGENT_ID` deve ser único** por instância de `kd-agent` — torna-se o `caller_id` em toda mensagem.
5. **Cada serviço é um módulo Go independente** com seu próprio `go.mod`. O `go.work` na raiz une tudo localmente.
6. **Observabilidade obrigatória**: Prometheus `/metrics` + OpenTelemetry OTLP em **todos** os serviços.
7. **DI via UberFX**: todo serviço usa `go.uber.org/fx`. Nunca instanciar dependências em `init()` ou variáveis globais.

---

## Checklist para Novos Serviços

Antes de implementar, verifique:
- [ ] `go.mod` com module path `github.com/kubediscovery/<nome>`
- [ ] Entrada no `go.work` da raiz
- [ ] Layout `cmd/grpc/` → `main.go / setup.go / providers.go / wire.go`
- [ ] `configs/` com structs viper (config.go, grpc.go, observability.go…)
- [ ] `internal/core/<domain>/` com entity / service / repository / handler / module.go
- [ ] `internal/infrastructure/grpc/server.go` com mTLS e interceptors
- [ ] `fx.Module` em cada `module.go`
- [ ] Prometheus + OTEL inicializados em `internal/infrastructure/observability/`
- [ ] `.env.example` com todas as variáveis necessárias
