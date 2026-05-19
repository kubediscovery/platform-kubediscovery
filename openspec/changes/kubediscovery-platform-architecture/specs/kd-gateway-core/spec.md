## ADDED Requirements

### Requirement: Servidor gRPC com mTLS

Feature: kd-gateway aceita conexões gRPC de agentes autenticados via mTLS
Rule: Toda conexão de `kd-agent` deve apresentar certificado cliente válido assinado pela CA da plataforma.

#### Scenario: Agente conecta com certificado válido
- **GIVEN** um `kd-agent` com certificado cliente válido assinado pela CA
- **WHEN** o agente inicia a conexão gRPC bidirecional
- **THEN** o gateway aceita a conexão e registra o agente como ativo usando `caller_id` como chave

#### Scenario: Agente conecta com certificado inválido
- **GIVEN** um `kd-agent` com certificado expirado ou não assinado pela CA
- **WHEN** o agente tenta iniciar a conexão gRPC
- **THEN** o gateway rejeita a conexão com erro `UNAUTHENTICATED` e registra o evento

#### Scenario: Agente desconecta inesperadamente
- **GIVEN** um `kd-agent` com stream ativo
- **WHEN** a conexão cai sem fechamento gracioso
- **THEN** o gateway detecta via heartbeat, marca o agente como `disconnected` e libera os recursos do stream

### Requirement: Identificação de agentes por três fontes

Feature: Gateway identifica agentes a partir de caller_id, metadata e CN/SAN do certificado
Rule: `caller_id` é a chave lógica de mapeamento; CN/SAN do certificado é a fonte confiável para autenticação.

#### Scenario: Agente identificado por caller_id
- **GIVEN** um agente enviando `caller_id: "agent-srv001"` no primeiro frame do stream
- **WHEN** o gateway processa a conexão
- **THEN** o agente é indexado em memória usando `caller_id` como chave primária

#### Scenario: Conflito de caller_id
- **GIVEN** dois agentes tentando conectar com o mesmo `caller_id`
- **WHEN** o segundo agente inicia a conexão
- **THEN** o gateway rejeita o segundo com erro `ALREADY_EXISTS` ou encerra a conexão anterior conforme política configurada

### Requirement: API HTTP REST com Gin-Gonic

Feature: kd-gateway expõe API HTTP para portal e kdctl
Rule: A API HTTP deve estar disponível na mesma instância do gateway, em porta separada da gRPC.

#### Scenario: Portal consulta lista de agentes conectados
- **GIVEN** o kd-portal autenticado na API HTTP
- **WHEN** realiza `GET /api/v1/agents`
- **THEN** recebe lista de agentes com status, `caller_id`, ambiente e última atividade

#### Scenario: API HTTP retorna erro estruturado
- **GIVEN** uma requisição inválida para a API HTTP
- **WHEN** o handler detecta o erro de validação
- **THEN** retorna HTTP 400 com body JSON `{"error": "<mensagem>", "code": "<código>"}`

### Requirement: Roteamento de comandos para agentes

Feature: Gateway roteia comandos do Control Plane para o agente correto
Rule: O roteamento é baseado no `caller_id` do agente alvo.

#### Scenario: Comando roteado para agente ativo
- **GIVEN** um agente `agent-srv001` com stream ativo
- **WHEN** o gateway recebe um comando destinado a `agent-srv001`
- **THEN** o comando é enviado pelo stream bidirecional do agente e a resposta é aguardada

#### Scenario: Comando para agente desconectado
- **GIVEN** um agente `agent-srv001` sem stream ativo
- **WHEN** o gateway recebe um comando destinado a `agent-srv001`
- **THEN** retorna erro `UNAVAILABLE` com mensagem indicando que o agente está offline

### Requirement: Notificação Slack ao detectar problema

Feature: Gateway dispara notificação Slack após análise de problema
Rule: Após receber `AnalysisResult` do `kd-analyzer`, o gateway envia notificação Slack configurada por cluster/ambiente.

#### Scenario: Problema detectado dispara notificação Slack
- **GIVEN** um `AnalysisResult` recebido do `kd-analyzer` com severidade `critical`
- **WHEN** o gateway processa o resultado
- **THEN** envia mensagem Slack ao canal configurado para o cluster/ambiente com resumo do problema e análise LLM

#### Scenario: Slack webhook não configurado
- **GIVEN** um cluster sem webhook Slack configurado
- **WHEN** um problema é detectado nesse cluster
- **THEN** o gateway persiste o evento no `kd-store` e registra log de aviso, sem falhar o fluxo principal

## MODIFIED Requirements

## REMOVED Requirements
