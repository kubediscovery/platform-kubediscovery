## ADDED Requirements

### Requirement: Geração local de certificados mTLS via kdctl

Feature: kdctl gera certificados mTLS autoassinados para comunicação segura
Rule: Todos os certificados são gerados localmente pelo kdctl e armazenados em `~/.kubediscovery/certs/<environment>/`.

#### Scenario: CA gerada com validade configurável
- **GIVEN** `kdctl init --environment production`
- **WHEN** a CA é gerada
- **THEN** `ca.crt` tem validade padrão de 10 anos e `ca.key` é armazenada com permissão `600`

#### Scenario: Certificado cliente gerado e assinado pela CA local
- **GIVEN** CA existente para ambiente `production`
- **WHEN** `kdctl certificate --create --name client-agent-srv001 --environment production`
- **THEN** certificado gerado com CN=`client-agent-srv001`, SAN incluindo o nome, assinado pela CA local

### Requirement: Publicação de certificados no HashiCorp Vault pelo usuário

Feature: O usuário publica os certificados gerados no HashiCorp Vault
Rule: A plataforma não acessa o Vault para escrita — apenas para leitura no bootstrap do Operator.

#### Scenario: Usuário publica certificado no Vault
- **GIVEN** certificado `client-agent-srv001.crt` e `client-agent-srv001.key` gerados localmente
- **WHEN** o usuário executa `vault kv put kubediscovery/production/agent-srv001/certs client.crt=@... client.key=@... ca.crt=@...`
- **THEN** certificados disponíveis no path `kubediscovery/production/agent-srv001/certs` para o Operator

#### Scenario: Path do Vault segue convenção da plataforma
- **GIVEN** ambiente `production` e agente `agent-srv001`
- **WHEN** o Operator busca certificados
- **THEN** lê do path `kubediscovery/<environment>/<agent-name>/certs`

### Requirement: Download de certificados do Vault pelo Operator no bootstrap

Feature: Kubernetes Operator baixa certificados do Vault e cria Kubernetes Secrets
Rule: O Operator usa AppRole ou token Vault configurado via Secret do Kubernetes para autenticar no Vault.

#### Scenario: Operator autentica no Vault via AppRole
- **GIVEN** Secret `vault-credentials` com `role_id` e `secret_id` no namespace do Operator
- **WHEN** o Operator inicializa
- **THEN** autentica no Vault via AppRole e obtém token para leitura dos certificados

#### Scenario: Certificados baixados e criados como Kubernetes Secret
- **GIVEN** Operator autenticado no Vault com certificados disponíveis
- **WHEN** reconcilia novo CRD Agent
- **THEN** cria Secret `kubediscovery-agent-certs` com keys `ca.crt`, `client.crt`, `client.key`

### Requirement: Rotação de certificados

Feature: Certificados podem ser rotacionados sem downtime
Rule: A rotação é iniciada pelo usuário via kdctl e propagada pelo Operator.

#### Scenario: Rotação de certificado de agente
- **GIVEN** certificado de agente próximo da expiração
- **WHEN** usuário gera novo certificado, publica no Vault e aplica `forceRotate: true` no CRD Agent
- **THEN** Operator baixa novo certificado do Vault, atualiza o Secret e reinicia o kd-agent graciosamente

## MODIFIED Requirements

## REMOVED Requirements
