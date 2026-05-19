## ADDED Requirements

### Requirement: CRD Agent gerencia ciclo de vida dos componentes do Data Plane

Feature: Kubernetes Operator com CRD Agent controla quais componentes rodam no cluster remoto
Rule: kd-agent é obrigatório e sempre ativo; kd-executor e kd-analyzer são opcionais e controlados pelo CRD.

#### Scenario: CRD Agent criado com configuração padrão
- **GIVEN** CRD Agent aplicado com `agent.enabled: true`, `executor.enabled: true`, `analyzer.enabled: false`
- **WHEN** o Operator reconcilia o recurso
- **THEN** cria Deployments para `kd-agent` e `kd-executor`, não cria Deployment para `kd-analyzer`

#### Scenario: kd-agent não pode ser desabilitado
- **GIVEN** CRD Agent com `agent.enabled: false`
- **WHEN** o Operator processa o recurso
- **THEN** ignora o campo e mantém `kd-agent` ativo, registrando aviso: `kd-agent is mandatory and cannot be disabled`

#### Scenario: Analyzer habilitado via CRD
- **GIVEN** CRD Agent atualizado com `analyzer.enabled: true`
- **WHEN** o Operator reconcilia a mudança
- **THEN** cria Deployment do `kd-analyzer` local com `analyzer.mode` configurado

#### Scenario: Componente desabilitado tem Deployment removido
- **GIVEN** CRD Agent com `executor.enabled: true` atualizado para `executor.enabled: false`
- **WHEN** o Operator reconcilia a mudança
- **THEN** remove o Deployment do `kd-executor` graciosamente

### Requirement: Bootstrap de certificados via HashiCorp Vault

Feature: Operator baixa certificados mTLS do Vault e cria Kubernetes Secrets no cluster remoto
Rule: O usuário é responsável por publicar os certificados no Vault; o Operator faz o download no bootstrap.

#### Scenario: Operator baixa certificados do Vault no bootstrap
- **GIVEN** Vault acessível com path `kubediscovery/<environment>/<agent-name>/certs`
- **WHEN** o Operator inicializa para um novo Agent CRD
- **THEN** baixa `ca.crt`, `client.crt` e `client.key` do Vault e cria Secret `kubediscovery-agent-certs`

#### Scenario: Vault indisponível no bootstrap
- **GIVEN** Vault inacessível durante o bootstrap do Operator
- **WHEN** o Operator tenta baixar os certificados
- **THEN** entra em estado `Pending` com condição `CertificatesNotReady` e tenta novamente com backoff

#### Scenario: Secret de certificados já existe
- **GIVEN** Secret `kubediscovery-agent-certs` já presente no cluster
- **WHEN** o Operator reconcilia o CRD Agent
- **THEN** reutiliza o Secret existente sem sobrescrever, a menos que `forceRotate: true` esteja definido no CRD

### Requirement: Status do CRD Agent reflete estado dos componentes

Feature: CRD Agent expõe status observável dos componentes gerenciados
Rule: O campo `.status` do CRD deve ser atualizado pelo Operator a cada reconciliação.

#### Scenario: Status atualizado após reconciliação bem-sucedida
- **GIVEN** todos os Deployments gerenciados em estado `Ready`
- **WHEN** o Operator completa a reconciliação
- **THEN** `.status.conditions` inclui `Ready: True` com timestamp e mensagem descritiva

#### Scenario: Status reflete componente com falha
- **GIVEN** Deployment do `kd-executor` em `CrashLoopBackOff`
- **WHEN** o Operator detecta a falha durante reconciliação
- **THEN** `.status.conditions` inclui `ExecutorReady: False` com motivo da falha

## MODIFIED Requirements

## REMOVED Requirements
