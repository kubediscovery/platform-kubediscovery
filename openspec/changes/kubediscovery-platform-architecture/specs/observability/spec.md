## ADDED Requirements

### Requirement: Métricas Prometheus em todos os serviços

Feature: Todos os serviços expõem endpoint /metrics compatível com Prometheus
Rule: Cada serviço com interface HTTP deve expor `/metrics` com métricas de RED (Rate, Errors, Duration) e métricas de negócio relevantes.

#### Scenario: Endpoint /metrics disponível no kd-gateway
- **GIVEN** kd-gateway em execução
- **WHEN** Prometheus faz scrape em `GET /metrics`
- **THEN** retorna métricas incluindo `grpc_requests_total`, `grpc_request_duration_seconds`, `active_agents_total`

#### Scenario: Métricas de agentes conectados expostas
- **GIVEN** 5 agentes conectados ao kd-gateway
- **WHEN** Prometheus faz scrape
- **THEN** métrica `active_agents_total` retorna valor `5`

#### Scenario: Métricas de erro LLM expostas no kd-analyzer
- **GIVEN** kd-analyzer com falhas de chamada ao Databricks API
- **WHEN** Prometheus faz scrape
- **THEN** métrica `llm_requests_errors_total` reflete o número de falhas

### Requirement: Traces OpenTelemetry em todos os serviços

Feature: Todos os serviços emitem traces OpenTelemetry via OTLP HTTP exporter
Rule: Traces devem iniciar em cada handler gRPC/HTTP e propagar via metadata gRPC entre serviços.

#### Scenario: Trace inicia no handler gRPC do kd-gateway
- **GIVEN** chamada gRPC `ListPods` recebida pelo kd-gateway
- **WHEN** o handler processa a chamada
- **THEN** span é criado com atributos `rpc.service`, `rpc.method`, `agent.caller_id` e `cluster.name`

#### Scenario: Trace propagado do gateway para o agente via metadata gRPC
- **GIVEN** span ativo no kd-gateway ao rotear comando para kd-agent
- **WHEN** o gateway envia mensagem pelo stream bidirecional
- **THEN** trace context é propagado via metadata gRPC e o kd-agent cria span filho

#### Scenario: Trace do pipeline LLM captura cada agente
- **GIVEN** pipeline LLM executando `log_analyst` → `event_analyst`
- **WHEN** cada agente ADK é invocado
- **THEN** span filho é criado para cada agente com atributos `llm.agent_name` e `llm.model`

### Requirement: Logging estruturado com slog

Feature: Todos os serviços usam slog para logging estruturado
Rule: Logs devem ser emitidos em formato JSON com campos padronizados: `level`, `time`, `service`, `trace_id`, `span_id`, `msg`.

#### Scenario: Log de conexão de agente inclui trace_id
- **GIVEN** agente conectando ao kd-gateway com trace ativo
- **WHEN** o gateway registra o evento de conexão
- **THEN** log JSON inclui `trace_id` e `span_id` do trace ativo para correlação

#### Scenario: Log de erro inclui contexto suficiente para diagnóstico
- **GIVEN** falha ao rotear comando para agente desconectado
- **WHEN** o gateway registra o erro
- **THEN** log JSON inclui `level: "error"`, `agent_id`, `command_type`, `error` e `trace_id`

### Requirement: Configuração de observabilidade via variáveis de ambiente

Feature: Endpoints de exportação de traces e métricas são configuráveis por ambiente
Rule: Serviços devem ler configurações de observabilidade de variáveis de ambiente sem necessidade de recompilação.

#### Scenario: OTLP exporter configurado via env var
- **GIVEN** variável `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318`
- **WHEN** o serviço inicializa
- **THEN** traces são exportados para o endpoint configurado

#### Scenario: Observabilidade desabilitada em testes
- **GIVEN** variável `OTEL_SDK_DISABLED=true` em ambiente de teste
- **WHEN** o serviço inicializa
- **THEN** nenhum trace é exportado e o overhead de instrumentação é mínimo

## MODIFIED Requirements

## REMOVED Requirements
