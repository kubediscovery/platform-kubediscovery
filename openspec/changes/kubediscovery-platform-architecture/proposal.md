## Why

Equipes de DevOps e SRE que operam múltiplos clusters Kubernetes não possuem uma plataforma unificada para gerenciamento centralizado, execução remota de comandos e análise inteligente via LLM — forçando o uso de ferramentas fragmentadas sem visibilidade consolidada. O Kubediscovery resolve isso com uma arquitetura Control Plane / Data Plane conectada via gRPC bidirecional com mTLS, entregando gestão centralizada, AIOps e segurança nativa desde o MVP.

## What Changes

- Criação do monorepo com `go.work` conectando todos os módulos Go localmente
- Novo componente `kd-gateway`: ponto focal gRPC do Control Plane, expõe também API HTTP (Gin-Gonic) para o portal
- Novo componente `kd-agent`: cliente gRPC obrigatório e sempre ativo no Data Plane, mantém conexão bidirecional com o gateway
- Novo componente `kd-executor`: executa comandos Kubernetes remotos e atua como watcher de eventos no cluster remoto
- Novo componente `kd-analyzer`: pipeline LLM no Control Plane (Google ADK + Databricks); execução no Data Plane é opt-in via `analyzer.mode`
- Novo componente `kd-store`: persistência PostgreSQL + pgvector (estado estruturado + memória LLM indexada por `clusterName+Environment+Namespace`)
- Novo componente `kd-mcp`: servidor MCP dedicado, traduz chamadas de clientes externos (Claude Desktop, Cursor, IDEs) para gRPC → `kd-gateway`
- Novo componente `kd-portal`: dashboard web consumindo API HTTP Gin-Gonic do `kd-gateway`
- Novo Kubernetes Operator com CRD `Agent`: gerencia instâncias de `kd-agent` (obrigatório), `kd-executor` e `kd-analyzer` (opcionais) no Data Plane
- CLI `kdctl`: gestão de certificados mTLS (gerados localmente, publicados no HashiCorp Vault pelo usuário, baixados pelo Operator como Kubernetes Secrets), registro e ciclo de vida de clusters
- OPA embutido como biblioteca Go no `kd-gateway` para autorização fine-grained (escopos K8s, IA e Plataforma)
- Notificações Slack disparadas pelo `kd-gateway` ao detectar problemas via pipeline `kd-executor (watcher) → kd-agent → kd-gateway → kd-analyzer → Slack`
- Toda comunicação Control Plane ↔ Data Plane via gRPC bidirecional com mTLS

## Capabilities

### New Capabilities

- `monorepo-workspace`: Estrutura do monorepo com `go.work`, layout de diretórios, módulos Go e libs compartilhadas (`kd-libs`)
- `kd-gateway-core`: Servidor gRPC central com mTLS, roteamento de agentes, API HTTP Gin-Gonic, OPA embutido e integração Slack
- `kd-agent-core`: Cliente gRPC bidirecional obrigatório no Data Plane, retry com backoff exponencial, identificação via `caller_id` + mTLS CN/SAN
- `kd-executor-core`: Execução remota de comandos Kubernetes e watcher de eventos (Pods, Events) no cluster remoto
- `kd-analyzer-core`: Pipeline LLM multi-agente (Google ADK + Databricks) no Control Plane; modo local opt-in via `analyzer.mode` no CRD
- `kd-store-core`: PostgreSQL + pgvector para estado estruturado e memória LLM; Redis para cache e estado em memória
- `kd-mcp-server`: Servidor MCP dedicado traduzindo protocolo MCP → gRPC para integração com clientes LLM externos
- `kubernetes-operator`: CRD `Agent` (apiVersion: `kubediscovery.io/v1beta1`) gerenciando o ciclo de vida dos componentes do Data Plane
- `kdctl-cli`: CLI com Cobra/Viper para gestão de certificados mTLS, registro de clusters, emissão de comandos e consulta de histórico
- `certificate-management`: Geração de certs mTLS via `kdctl`, publicação no HashiCorp Vault pelo usuário, download pelo Operator como Kubernetes Secrets
- `opa-authz`: Autorização fine-grained embutida no `kd-gateway` com escopos K8s (verbs/namespaces/kinds), IA (`llm:analyze`) e Plataforma (`cluster:pause`)
- `observability`: Prometheus `/metrics` + OpenTelemetry traces (OTLP HTTP) em todos os serviços; traces iniciando em cada handler gRPC/HTTP

### Modified Capabilities

_(nenhuma — projeto novo, sem specs existentes)_

## Impact

**Código:**
- Novo monorepo com 7+ módulos Go independentes conectados via `go.work`
- `services/validate/` existente permanece inalterado (protótipo de referência para o pipeline LLM)

**Infraestrutura:**
- PostgreSQL com extensão `pgvector`
- Redis
- HashiCorp Vault (gerenciado pelo usuário para distribuição de certificados)
- Kubernetes (Control Plane cluster + N clusters remotos com Operator instalado)

**Dependências externas:**
- `go.uber.org/fx` (DI em todos os serviços)
- `github.com/spf13/cobra` + `github.com/spf13/viper` (CLI e config)
- `google.golang.org/grpc` com mTLS
- `google.golang.org/adk` v1.2.0 (pipeline LLM)
- `github.com/gin-gonic/gin` (API HTTP)
- `github.com/open-policy-agent/opa` (autorização embutida)
- `github.com/slack-go/slack` (notificações)
- Databricks OpenAI-compatible API (modelo LLM)

**Fases de entrega:**
- **MVP (Phase 1):** `kdctl` + `kd-gateway` + `kd-agent` + `kd-executor` + `kd-store` — fluxo básico de conexão, registro e execução remota
- **Phase 2:** `kd-analyzer` + `kd-mcp` + Slack notifications + OPA authz
- **Phase 3:** Kubernetes Operator (CRD `Agent`) + `kd-portal` + HashiCorp Vault + pgvector LLM memory
