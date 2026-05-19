## ADDED Requirements

### Requirement: Inicialização da plataforma com kdctl init

Feature: kdctl init configura o Control Plane e gera certificados CA e servidor
Rule: `kdctl init` é o ponto de entrada para configurar a plataforma pela primeira vez.

#### Scenario: Init cria CA e certificado de servidor
- **GIVEN** `kdctl init --name kubediscovery --address gateway.example.com:50051 --environment production`
- **WHEN** o comando é executado
- **THEN** gera `ca.crt`, `ca.key`, `server.crt`, `server.key` em `~/.kubediscovery/certs/production/`

#### Scenario: Init falha se ambiente já inicializado
- **GIVEN** ambiente `production` já inicializado com CA existente
- **WHEN** `kdctl init --environment production` é executado sem `--force`
- **THEN** retorna erro: `environment "production" already initialized. Use --force to reinitialize.`

### Requirement: Gestão de certificados de agentes

Feature: kdctl certificate cria certificados cliente para cada agente
Rule: Certificados cliente são assinados pela CA gerada no `kdctl init`.

#### Scenario: Criação de certificado para novo agente
- **GIVEN** CA existente para ambiente `production`
- **WHEN** `kdctl certificate --create --name client-agent-srv001 --environment production`
- **THEN** gera `client-agent-srv001.crt` e `client-agent-srv001.key` em `~/.kubediscovery/certs/production/`

#### Scenario: Nome começando com "client-" detectado automaticamente como cliente
- **GIVEN** `kdctl certificate --create --name client-agent-srv001 --environment production`
- **WHEN** o comando é processado
- **THEN** flag `--client` é inferida automaticamente pelo prefixo `client-`

#### Scenario: Listagem de certificados existentes
- **GIVEN** múltiplos certificados gerados para ambiente `production`
- **WHEN** `kdctl certificate --list`
- **THEN** exibe tabela com nome, tipo (CA/server/client), ambiente, data de expiração e status

### Requirement: Gestão de clusters registrados

Feature: kdctl gerencia o ciclo de vida de clusters clientes
Rule: Cada cluster cliente tem UID único gerado no registro e status gerenciado pelo Control Plane.

#### Scenario: Registro de novo cluster
- **GIVEN** `kdctl cluster register --name cluster-prod-us-east --environment production`
- **WHEN** o comando é executado
- **THEN** cria registro no kd-store com UID único, status `unregistered` e exibe UID gerado

#### Scenario: Listagem de clusters conectados
- **GIVEN** múltiplos agentes conectados ao gateway
- **WHEN** `kdctl cluster list`
- **THEN** exibe tabela com nome, UID, ambiente, status (`connected`/`disconnected`/`paused`) e última atividade

#### Scenario: Pause de cluster por UID
- **GIVEN** cluster com UID `abc-123` em estado `connected`
- **WHEN** `kdctl cluster pause --uid abc-123`
- **THEN** gateway bloqueia novos comandos para o agente e status muda para `paused`

#### Scenario: Remoção de cluster por UID
- **GIVEN** cluster com UID `abc-123`
- **WHEN** `kdctl cluster delete --uid abc-123`
- **THEN** gateway encerra stream do agente, remove registro do kd-store e revoga certificado

### Requirement: Consulta de histórico de análises

Feature: kdctl permite consultar histórico de validações e problemas detectados
Rule: O histórico é consultado do kd-store filtrado por cluster, ambiente e namespace.

#### Scenario: Consulta de histórico por cluster e namespace
- **GIVEN** histórico de análises persistido no kd-store
- **WHEN** `kdctl history --cluster prod-us-east --namespace payments --limit 10`
- **THEN** exibe lista das 10 análises mais recentes com timestamp, severidade e resumo do diagnóstico

## MODIFIED Requirements

## REMOVED Requirements
