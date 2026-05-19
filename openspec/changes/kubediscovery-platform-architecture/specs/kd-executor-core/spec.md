## ADDED Requirements

### Requirement: Execução remota de comandos Kubernetes

Feature: kd-executor executa comandos Kubernetes no cluster local em nome do Control Plane
Rule: O executor traduz comandos recebidos do kd-agent em operações nativas da Kubernetes API.

#### Scenario: Executor executa listagem de pods
- **GIVEN** comando `list pods` recebido do kd-agent com namespace `production`
- **WHEN** o executor processa o comando
- **THEN** consulta a Kubernetes API local e retorna lista de pods com status, imagem e idade

#### Scenario: Executor executa comando em namespace inexistente
- **GIVEN** comando `list pods` com namespace `inexistente`
- **WHEN** o executor consulta a Kubernetes API
- **THEN** retorna erro `NOT_FOUND` com mensagem `namespace "inexistente" not found`

#### Scenario: Executor sem permissão para executar comando
- **GIVEN** ServiceAccount do executor sem RBAC para `delete pods`
- **WHEN** recebe comando `delete pod nginx-abc -n production`
- **THEN** retorna erro `PERMISSION_DENIED` com mensagem descritiva do recurso bloqueado

### Requirement: Watcher de eventos Kubernetes

Feature: kd-executor monitora eventos do cluster e reporta problemas ao kd-agent
Rule: O watcher deve monitorar eventos de Pods e Events do Kubernetes e reportar condições anômalas.

#### Scenario: Watcher detecta Pod em estado Pending
- **GIVEN** watcher ativo monitorando eventos do cluster
- **WHEN** um Pod entra em estado `Pending` por mais de 60 segundos
- **THEN** reporta evento ao kd-agent com tipo `POD_PENDING`, namespace, nome do pod e mensagem do evento

#### Scenario: Watcher detecta FailedScheduling
- **GIVEN** watcher monitorando Events do Kubernetes
- **WHEN** evento `FailedScheduling` é emitido para um Pod
- **THEN** reporta ao kd-agent com tipo `SCHEDULING_FAILURE`, incluindo a mensagem completa do evento

#### Scenario: Watcher reinicia após erro de conexão com Kubernetes API
- **GIVEN** watcher ativo que perde conexão com a Kubernetes API
- **WHEN** a conexão é interrompida
- **THEN** o watcher tenta reconectar com backoff e retoma o watch do ponto de interrupção usando `resourceVersion`

### Requirement: Acesso à Kubernetes API via ServiceAccount

Feature: kd-executor usa ServiceAccount com RBAC mínimo necessário
Rule: O executor deve operar com princípio de least privilege — apenas os verbos e recursos necessários para as operações suportadas.

#### Scenario: Executor usa in-cluster config por padrão
- **GIVEN** kd-executor rodando como Pod no cluster remoto
- **WHEN** inicializa o cliente Kubernetes
- **THEN** usa in-cluster config (ServiceAccount token montado automaticamente)

#### Scenario: Executor usa kubeconfig externo em modo de desenvolvimento
- **GIVEN** variável `KUBECONFIG` apontando para arquivo de configuração
- **WHEN** kd-executor inicializa fora de um cluster
- **THEN** usa o kubeconfig especificado para conectar à Kubernetes API

## MODIFIED Requirements

## REMOVED Requirements
