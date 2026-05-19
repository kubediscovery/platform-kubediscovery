## ADDED Requirements

### Requirement: Conexão gRPC bidirecional obrigatória e persistente

Feature: kd-agent mantém stream gRPC bidirecional ativo com o kd-gateway
Rule: O kd-agent é obrigatório e deve estar sempre ativo — é o único canal de comunicação entre o Data Plane e o Control Plane.

#### Scenario: Agente inicia conexão com sucesso
- **GIVEN** o kd-agent configurado com endereço do gateway, certificado cliente e `AGENT_ID` único
- **WHEN** o agente é iniciado
- **THEN** estabelece stream gRPC bidirecional com o gateway e envia frame inicial com `caller_id`

#### Scenario: Agente reconecta após falha de rede
- **GIVEN** um kd-agent com stream ativo que perde conectividade
- **WHEN** a conexão é interrompida
- **THEN** o agente tenta reconectar com backoff exponencial: 1s, 3s, 9s, 27s, 81s

#### Scenario: Agente esgota tentativas de reconexão
- **GIVEN** um kd-agent que falhou em todas as 5 tentativas de reconexão
- **WHEN** a última tentativa (81s) falha
- **THEN** o agente encerra com status fatal e registra erro crítico no log

### Requirement: Identificação única por instância

Feature: Cada instância de kd-agent possui identidade única na plataforma
Rule: `AGENT_ID` deve ser único por instância — é usado como `caller_id` no gateway.

#### Scenario: Agente usa AGENT_ID como caller_id
- **GIVEN** variável de ambiente `AGENT_ID=agent-cluster-prod-01`
- **WHEN** o agente inicia a conexão
- **THEN** envia `caller_id: "agent-cluster-prod-01"` no primeiro frame do stream

#### Scenario: AGENT_ID não configurado
- **GIVEN** variável de ambiente `AGENT_ID` ausente
- **WHEN** o agente tenta iniciar
- **THEN** falha na inicialização com erro explícito: `AGENT_ID is required`

### Requirement: Delegação de comandos ao kd-executor

Feature: kd-agent delega execução de comandos Kubernetes ao kd-executor
Rule: O kd-agent não executa comandos Kubernetes diretamente — delega ao kd-executor local.

#### Scenario: Agente recebe comando do gateway e delega ao executor
- **GIVEN** um comando `kubectl get pods -n production` recebido via stream
- **WHEN** o agente processa o comando
- **THEN** encaminha ao kd-executor local e aguarda resposta antes de retornar ao gateway

#### Scenario: Executor indisponível ao receber comando
- **GIVEN** kd-executor não disponível no Data Plane
- **WHEN** o agente recebe um comando que requer execução
- **THEN** retorna erro `UNAVAILABLE` ao gateway com mensagem `executor not available`

### Requirement: Autenticação mTLS com certificado cliente

Feature: kd-agent autentica no gateway via certificado mTLS
Rule: O certificado cliente deve ser montado a partir do Kubernetes Secret criado pelo Operator.

#### Scenario: Agente carrega certificado do Secret montado
- **GIVEN** Secret `kubediscovery-agent-certs` montado em `/etc/kubediscovery/certs/`
- **WHEN** o agente inicializa a conexão TLS
- **THEN** usa `client.crt` e `client.key` do Secret para autenticar no gateway

## MODIFIED Requirements

## REMOVED Requirements
