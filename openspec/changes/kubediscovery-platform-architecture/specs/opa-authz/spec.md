## ADDED Requirements

### Requirement: Autorização fine-grained embutida no kd-gateway via OPA

Feature: kd-gateway avalia políticas OPA em cada chamada gRPC e HTTP
Rule: OPA é embutido como biblioteca Go — nenhuma chamada de rede é feita para avaliação de política.

#### Scenario: Chamada gRPC autorizada por política OPA
- **GIVEN** usuário com permissão `pods:list` no namespace `production` do cluster `prod-us-east`
- **WHEN** realiza chamada gRPC `ListPods` com namespace `production`
- **THEN** OPA avalia `allow = true` e o gateway processa a chamada

#### Scenario: Chamada gRPC negada por política OPA
- **GIVEN** usuário sem permissão `pods:delete` no namespace `production`
- **WHEN** realiza chamada gRPC `DeletePod`
- **THEN** OPA avalia `allow = false`, gateway retorna `PERMISSION_DENIED` sem encaminhar ao agente

#### Scenario: Políticas carregadas na inicialização do gateway
- **GIVEN** arquivos Rego em `configs/policies/`
- **WHEN** o kd-gateway inicia
- **THEN** políticas são compiladas e cacheadas em memória pelo OPA embutido

### Requirement: Escopo Kubernetes — verbos, namespaces e kinds

Feature: Políticas OPA controlam acesso a recursos Kubernetes por verbo, namespace e kind
Rule: O modelo de autorização espelha o RBAC do Kubernetes estendido para recursos da plataforma.

#### Scenario: Permissão por verbo e kind
- **GIVEN** política concedendo `get` e `list` em `pods` para usuário `devops-user`
- **WHEN** `devops-user` tenta `delete` em `pods`
- **THEN** OPA nega com `allow = false`

#### Scenario: Permissão restrita por namespace
- **GIVEN** política concedendo acesso apenas ao namespace `staging`
- **WHEN** usuário tenta listar pods no namespace `production`
- **THEN** OPA nega acesso ao namespace `production`

### Requirement: Escopo IA — controle de uso de LLM

Feature: Políticas OPA controlam quais usuários ou tenants podem usar análise LLM
Rule: Uso de LLM requer permissão explícita `llm:analyze` ou `llm:query`.

#### Scenario: Usuário com permissão LLM executa análise
- **GIVEN** usuário com permissão `llm:analyze`
- **WHEN** solicita análise de cluster via kd-mcp ou API
- **THEN** OPA autoriza e kd-gateway encaminha para kd-analyzer

#### Scenario: Usuário sem permissão LLM tenta análise
- **GIVEN** usuário sem permissão `llm:analyze`
- **WHEN** tenta solicitar análise
- **THEN** OPA nega com `PERMISSION_DENIED` e mensagem `LLM analysis not authorized for this user`

### Requirement: Escopo Plataforma — gestão de clusters

Feature: Políticas OPA controlam operações administrativas da plataforma
Rule: Operações como `cluster:pause`, `cluster:register` e `cluster:delete` requerem permissões explícitas de plataforma.

#### Scenario: Admin pausa cluster com permissão de plataforma
- **GIVEN** usuário com permissão `cluster:pause`
- **WHEN** executa `kdctl cluster pause --uid abc-123`
- **THEN** OPA autoriza e gateway executa a operação de pause

#### Scenario: Usuário sem permissão administrativa tenta deletar cluster
- **GIVEN** usuário com apenas permissões Kubernetes (sem `cluster:delete`)
- **WHEN** tenta `kdctl cluster delete --uid abc-123`
- **THEN** OPA nega com `PERMISSION_DENIED`

## MODIFIED Requirements

## REMOVED Requirements
