## 1. Monorepo e Workspace

- [ ] 1.1 Criar `go.work` na raiz do monorepo listando todos os módulos planejados
- [ ] 1.2 Adicionar `go.work` e `go.work.sum` ao `.gitignore`
- [ ] 1.3 Criar módulo `libs/` com `go.mod` (`github.com/kubediscovery/kd-libs`) e estrutura `pkg/errors`, `pkg/logger`, `pkg/validator`, `pkg/response`
- [ ] 1.4 Documentar setup de desenvolvimento local (go.work, env vars, dependências) no `README.md` raiz
- [ ] 1.5 Criar template de serviço Go com layout padrão (`cmd/`, `configs/`, `internal/core/`, `internal/infrastructure/`, `pkg/`)

## 2. Proto e Contratos gRPC

- [ ] 2.1 Criar diretório `proto/kubediscovery/v1/` com `gateway.proto` definindo o serviço gRPC bidirecional entre gateway e agentes
- [ ] 2.2 Criar `proto/kubediscovery/v1/executor.proto` com mensagens de comandos Kubernetes e respostas
- [ ] 2.3 Criar `proto/kubediscovery/v1/analyzer.proto` com `AnalysisRequest`, `AnalysisResult` e `MemoryKey`
- [ ] 2.4 Criar `proto/kubediscovery/v1/mcp.proto` com contrato entre `kd-mcp` e `kd-gateway`
- [ ] 2.5 Criar `scripts/proto-gen.sh` e `Makefile` target `proto-gen` para geração de código Go em `libs/core/v1/`
- [x] 2.6 Executar `make proto-gen` e verificar que os arquivos `*.pb.go` são gerados corretamente em `libs/`

## 3. Phase 1 — kd-store (Infraestrutura de Dados)

- [ ] 3.1 Criar módulo `services/kd-store/` com `go.mod` (`github.com/kubediscovery/kd-store`)
- [ ] 3.2 Implementar `internal/infrastructure/database/postgres.go` com pool pgx e configuração via Viper
- [ ] 3.3 Implementar `internal/infrastructure/database/migrations.go` com golang-migrate aplicando migrations na inicialização
- [x] 3.4 Criar migration `000001_create_clusters_table.sql` com campos: `uid`, `name`, `environment`, `status`, `created_at`, `updated_at`
- [ ] 3.5 Criar migration `000002_create_events_table.sql` com campos: `id`, `cluster_uid`, `memory_key`, `severity`, `diagnosis`, `recommendations`, `created_at`
- [ ] 3.6 Criar migration `000003_create_analysis_memory_table.sql` com coluna `embedding vector(1536)` e índice HNSW para pgvector
- [ ] 3.7 Implementar `internal/infrastructure/cache/redis.go` com go-redis e configuração via Viper
- [ ] 3.8 Implementar `cmd/grpc/main.go` com `fx.New(configs.Module, infrastructure.Module).Run()` e graceful shutdown
- [ ] 3.9 Escrever testes de integração para migrations e operações CRUD básicas no PostgreSQL
- [x] 3.10 Verificar que `go test ./...` passa no módulo `kd-store`

## 4. Phase 1 — kd-gateway (Control Plane Core)

- [ ] 4.1 Criar módulo `services/kd-gateway/` com `go.mod` (`github.com/kubediscovery/kd-gateway`)
- [x] 4.2 Implementar `internal/infrastructure/grpc/server.go` com mTLS — carregar CA, certificado servidor e exigir certificado cliente
- [ ] 4.3 Implementar handler gRPC bidirecional `internal/core/agent/handler/grpc_handler.go` — aceitar stream, registrar agente por `caller_id`, indexar em mapa em memória
- [x] 4.4 Implementar detecção de desconexão via heartbeat — marcar agente como `disconnected` ao expirar TTL
- [ ] 4.5 Implementar lógica de roteamento de comandos para agente por `caller_id` — retornar `UNAVAILABLE` se agente offline
- [x] 4.6 Implementar rejeição de `caller_id` duplicado com política configurável (rejeitar novo ou encerrar anterior)
- [ ] 4.7 Implementar `internal/infrastructure/http/server.go` com Gin-Gonic em porta separada da gRPC
- [ ] 4.8 Implementar `GET /api/v1/agents` retornando lista de agentes com status, `caller_id`, ambiente e última atividade
- [ ] 4.9 Implementar middleware de erro estruturado HTTP (`{"error": "...", "code": "..."}`)
- [ ] 4.10 Implementar `internal/infrastructure/observability/` com Prometheus `/metrics` e OpenTelemetry OTLP exporter
- [ ] 4.11 Expor métricas `grpc_requests_total`, `grpc_request_duration_seconds`, `active_agents_total`
- [ ] 4.12 Implementar `cmd/grpc/main.go` com `fx.New(configs.Module, observability.Module, infrastructure.Module, core.Module).Run()`
- [ ] 4.13 Escrever testes unitários para handler gRPC (mock de stream), roteamento e middleware HTTP
- [ ] 4.14 Verificar que `go test ./...` passa no módulo `kd-gateway`

## 5. Phase 1 — kd-agent (Data Plane Core)

- [ ] 5.1 Criar módulo `services/kd-agent/` com `go.mod` (`github.com/kubediscovery/kd-agent`)
- [ ] 5.2 Implementar cliente gRPC com mTLS — carregar certificado cliente do path configurado via `GRPC_CLIENT_CERT_FILE` e `GRPC_CLIENT_KEY_FILE`
- [ ] 5.3 Implementar loop de conexão com backoff exponencial: base 1s, multiplicador 3, tentativas máximas 5 (1s → 3s → 9s → 27s → 81s → fatal)
- [ ] 5.4 Implementar envio de frame inicial com `caller_id` = `AGENT_ID` ao estabelecer stream
- [ ] 5.5 Implementar validação de `AGENT_ID` na inicialização — falhar com erro explícito se ausente
- [ ] 5.6 Implementar recebimento e despacho de comandos do gateway para o `kd-executor` local
- [x] 5.7 Implementar retorno de erro `UNAVAILABLE` quando `kd-executor` não estiver disponível
- [ ] 5.8 Implementar `internal/infrastructure/observability/` com Prometheus e OpenTelemetry — propagar trace context via metadata gRPC
- [ ] 5.9 Implementar `cmd/grpc/main.go` com UberFX e graceful shutdown do stream gRPC
- [ ] 5.10 Escrever testes unitários para lógica de retry, envio de `caller_id` e despacho de comandos
- [ ] 5.11 Verificar que `go test ./...` passa no módulo `kd-agent`

## 6. Phase 1 — kd-executor (Execução Remota e Watcher)

- [ ] 6.1 Criar módulo `services/kd-executor/` com `go.mod` (`github.com/kubediscovery/kd-executor`)
- [ ] 6.2 Implementar cliente Kubernetes com in-cluster config por padrão e fallback para `KUBECONFIG` env var
- [ ] 6.3 Implementar handlers de comandos Kubernetes: `list pods`, `get pod logs`, `list deployments`, `list events` com suporte a filtro por namespace
- [ ] 6.4 Implementar retorno de erro `NOT_FOUND` para namespace inexistente e `PERMISSION_DENIED` para RBAC insuficiente
- [ ] 6.5 Implementar watcher de eventos Kubernetes monitorando Pods em estado `Pending` por mais de 60 segundos
- [ ] 6.6 Implementar watcher de Events monitorando `FailedScheduling` e reportar ao `kd-agent` com tipo, namespace, nome e mensagem
- [ ] 6.7 Implementar reconexão do watcher com backoff ao perder conexão com a Kubernetes API, retomando pelo `resourceVersion`
- [ ] 6.8 Criar `ClusterRole` e `ClusterRoleBinding` YAML com permissões mínimas necessárias para o executor
- [ ] 6.9 Implementar `internal/infrastructure/observability/` com Prometheus e OpenTelemetry
- [ ] 6.10 Implementar `cmd/grpc/main.go` com UberFX
- [ ] 6.11 Escrever testes unitários para handlers de comandos (mock da Kubernetes API) e lógica do watcher
- [ ] 6.12 Verificar que `go test ./...` passa no módulo `kd-executor`

## 7. Phase 1 — kdctl (CLI)

- [ ] 7.1 Criar módulo `cli/kdctl/` com `go.mod` (`github.com/kubediscovery/kdctl`) e estrutura Cobra/Viper
- [ ] 7.2 Implementar comando `kdctl init` — gerar CA e certificado servidor em `~/.kubediscovery/certs/<environment>/` com validação de ambiente já existente
- [ ] 7.3 Implementar comando `kdctl certificate --create` — gerar certificado cliente assinado pela CA, inferindo flag `--client` pelo prefixo `client-`
- [ ] 7.4 Implementar comando `kdctl certificate --list` — exibir tabela com nome, tipo, ambiente, expiração e status
- [ ] 7.5 Implementar comando `kdctl cluster register` — criar registro no kd-store via API HTTP do gateway, retornar UID gerado
- [ ] 7.6 Implementar comando `kdctl cluster list` — exibir tabela de clusters com UID, ambiente, status e última atividade
- [ ] 7.7 Implementar comando `kdctl cluster pause --uid` — enviar ação de pause ao gateway via API HTTP
- [ ] 7.8 Implementar comando `kdctl cluster delete --uid` — remover cluster do kd-store e encerrar stream do agente
- [ ] 7.9 Implementar comando `kdctl history` — consultar histórico de análises do kd-store com filtros por cluster, namespace e limit
- [ ] 7.10 Implementar configuração via `~/.kubediscovery/config.yaml` com Viper e fallback para variáveis de ambiente
- [ ] 7.11 Escrever testes unitários para cada subcomando usando `cobra.Command.SetArgs` e `SetOut`
- [ ] 7.12 Verificar que `go test ./...` passa no módulo `kdctl`

## 8. Phase 1 — Validação End-to-End do MVP

- [ ] 8.1 Criar `build/docker-compose.yaml` com PostgreSQL, Redis e serviços do MVP para desenvolvimento local
- [ ] 8.2 Executar fluxo completo: `kdctl init` → `kdctl certificate --create` → iniciar `kd-gateway` → iniciar `kd-agent` → `kdctl cluster register` → verificar agente conectado via `kdctl cluster list`
- [ ] 8.3 Executar fluxo de comando remoto: `kdctl` → gateway → agente → executor → retorno
- [ ] 8.4 Executar fluxo de watcher: simular Pod Pending → executor detecta → agente reporta → gateway persiste no kd-store
- [ ] 8.5 Verificar métricas Prometheus em `/metrics` do gateway e do agente
- [ ] 8.6 Verificar traces OpenTelemetry propagados entre gateway e agente

## 9. Phase 2 — kd-analyzer (Pipeline LLM)

- [ ] 9.1 Criar módulo `services/kd-analyzer/` com `go.mod` (`github.com/kubediscovery/kd-analyzer`)
- [ ] 9.2 Implementar pipeline LLM com Google ADK v1.2.0 e Databricks API — agentes `log_analyst` e `event_analyst` em sessões isoladas
- [ ] 9.3 Implementar lógica condicional de ativação do `karpenter_analyst` — verificar estado `Pending` E presença de `"untolerated taint"`, `"0/N nodes available"` ou `"FailedScheduling"` nos eventos
- [ ] 9.4 Implementar passagem manual de output entre agentes (Go pipeline, não ADK multi-agent)
- [ ] 9.5 Implementar retorno de `AnalysisResult` com `MemoryKey = clusterName+environment+namespace`
- [ ] 9.6 Implementar busca de contexto histórico no kd-store via pgvector antes de cada análise
- [ ] 9.7 Implementar persistência de `AnalysisResult` com embedding no kd-store após cada análise
- [ ] 9.8 Implementar suporte a `analyzer.mode: local` — executar pipeline localmente no Data Plane quando configurado
- [ ] 9.9 Escrever testes unitários para lógica condicional do `karpenter_analyst` e construção do `MemoryKey`
- [ ] 9.10 Verificar que `go test ./...` passa no módulo `kd-analyzer`

## 10. Phase 2 — Notificações Slack

- [ ] 10.1 Implementar cliente Slack (`github.com/slack-go/slack`) no `kd-gateway` com webhook URL configurável por cluster/ambiente via Viper
- [ ] 10.2 Implementar envio de notificação Slack após receber `AnalysisResult` com severidade `critical` ou `warning`
- [ ] 10.3 Implementar fallback gracioso quando webhook Slack não estiver configurado — persistir evento e logar aviso sem falhar o fluxo
- [ ] 10.4 Escrever testes unitários para lógica de notificação com mock do cliente Slack

## 11. Phase 2 — OPA Autorização

- [ ] 11.1 Adicionar dependência `github.com/open-policy-agent/opa` ao `kd-gateway`
- [ ] 11.2 Criar políticas Rego iniciais em `configs/policies/` cobrindo escopos Kubernetes, IA e Plataforma
- [ ] 11.3 Implementar interceptor gRPC que avalia política OPA antes de cada handler — retornar `PERMISSION_DENIED` se `allow = false`
- [ ] 11.4 Implementar middleware HTTP Gin que avalia política OPA para endpoints da API REST
- [ ] 11.5 Implementar carregamento e compilação de políticas Rego em cache na inicialização do gateway
- [ ] 11.6 Expor métrica Prometheus `opa_policy_evaluations_total` com labels `allowed` e `denied`
- [ ] 11.7 Escrever testes unitários para interceptor OPA com políticas de fixture e casos allow/deny

## 12. Phase 2 — kd-mcp (MCP Server)

- [ ] 12.1 Criar módulo `services/kd-mcp/` com `go.mod` (`github.com/kubediscovery/kd-mcp`)
- [ ] 12.2 Implementar servidor MCP expondo catálogo de tools: `list_pods`, `get_pod_logs`, `list_deployments`, `list_events`, `analyze_cluster`
- [ ] 12.3 Implementar tradução de tool calls MCP para chamadas gRPC ao `kd-gateway`
- [ ] 12.4 Implementar validação de parâmetros obrigatórios — retornar erro MCP `INVALID_PARAMS` com campo ausente identificado
- [ ] 12.5 Implementar retorno de erro MCP estruturado para agente desconectado
- [ ] 12.6 Implementar autenticação do `kd-mcp` no `kd-gateway` via certificado de serviço
- [ ] 12.7 Implementar `internal/infrastructure/observability/` com Prometheus e OpenTelemetry
- [ ] 12.8 Escrever testes unitários para tradução MCP → gRPC e validação de parâmetros
- [ ] 12.9 Verificar que `go test ./...` passa no módulo `kd-mcp`

## 13. Phase 3 — Kubernetes Operator (CRD Agent)

- [ ] 13.1 Criar módulo `services/kd-operator/` com `go.mod` (`github.com/kubediscovery/kd-operator`) usando controller-runtime
- [ ] 13.2 Definir CRD `Agent` (`apiVersion: kubediscovery.io/v1beta1`) com campos `agent.enabled`, `executor.enabled`, `analyzer.enabled`, `analyzer.mode`, `troubleshootingImage.enabled`, `forceRotate`
- [ ] 13.3 Implementar reconciler que cria Deployments para `kd-agent` (sempre), `kd-executor` e `kd-analyzer` (condicionais)
- [ ] 13.4 Implementar lógica que ignora `agent.enabled: false` e mantém `kd-agent` ativo com log de aviso
- [ ] 13.5 Implementar remoção de Deployment ao desabilitar componente opcional no CRD
- [ ] 13.6 Implementar bootstrap de certificados — autenticar no Vault via AppRole e baixar `ca.crt`, `client.crt`, `client.key`
- [ ] 13.7 Implementar criação do Secret `kubediscovery-agent-certs` com os certificados baixados do Vault
- [ ] 13.8 Implementar lógica de `forceRotate: true` — baixar novos certs do Vault, atualizar Secret e reiniciar `kd-agent` graciosamente
- [ ] 13.9 Implementar atualização de `.status.conditions` após cada reconciliação (Ready, ExecutorReady, AnalyzerReady, CertificatesReady)
- [ ] 13.10 Criar manifests YAML para instalação do Operator: `ServiceAccount`, `ClusterRole`, `ClusterRoleBinding`, `Deployment`
- [ ] 13.11 Escrever testes unitários para o reconciler com envtest (controller-runtime)
- [ ] 13.12 Verificar que `go test ./...` passa no módulo `kd-operator`

## 14. Phase 3 — Gestão de Certificados com HashiCorp Vault

- [ ] 14.1 Documentar path convention do Vault: `kubediscovery/<environment>/<agent-name>/certs`
- [ ] 14.2 Criar exemplo de configuração AppRole Vault para o Operator com política de leitura mínima
- [ ] 14.3 Implementar cliente Vault no Operator usando `github.com/hashicorp/vault/api`
- [ ] 14.4 Implementar tratamento de Vault indisponível — entrar em estado `Pending` com condição `CertificatesNotReady` e retry com backoff
- [ ] 14.5 Implementar reutilização de Secret existente sem sobrescrever quando `forceRotate` não estiver definido

## 15. Phase 3 — kd-portal (Dashboard Web)

- [ ] 15.1 Criar projeto frontend `frontend/kd-portal/` com framework a definir (React/Vue)
- [ ] 15.2 Implementar tela de listagem de clusters com status, ambiente e última atividade consumindo `GET /api/v1/agents`
- [ ] 15.3 Implementar tela de histórico de análises com filtros por cluster e namespace
- [ ] 15.4 Implementar tela de detalhes de análise com diagnóstico, severidade e recomendações LLM
- [ ] 15.5 Implementar autenticação no portal (mecanismo a definir — ver Open Question do design.md)

## 16. Observabilidade Transversal

- [ ] 16.1 Criar pacote `libs/pkg/observability/` com helpers para inicialização de OpenTelemetry e Prometheus reutilizáveis por todos os serviços
- [ ] 16.2 Implementar propagação de trace context via metadata gRPC em todos os serviços (interceptor de cliente e servidor)
- [ ] 16.3 Implementar correlação de `trace_id` e `span_id` nos logs slog de todos os serviços
- [ ] 16.4 Criar `build/observability/docker-compose.yaml` com Prometheus, Grafana e Jaeger/Tempo para desenvolvimento local
- [ ] 16.5 Criar dashboard Grafana básico com métricas de agentes conectados, latência gRPC e erros LLM

## 17. Validação Final

- [ ] 17.1 Executar `openspec validate kubediscovery-platform-architecture --type change --strict` e corrigir divergências
- [ ] 17.2 Executar `go test ./...` em todos os módulos e garantir 0 falhas
- [ ] 17.3 Executar fluxo completo Phase 2 end-to-end: MCP tool call → gateway → agente → executor → retorno + análise LLM + notificação Slack
- [ ] 17.4 Verificar que todos os ADRs (0001–0007) estão sendo honrados na implementação
