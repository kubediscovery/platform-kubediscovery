# 0007. HashiCorp Vault para Distribuição de Certificados mTLS

- Status: accepted
- Date: 2026-05-19

## Context

Certificados mTLS de cliente precisam ser distribuídos para clusters remotos (Data Plane) de forma segura. Os clusters remotos podem estar em redes privadas sem acesso direto do administrador via `kubectl`. A plataforma não deve gerenciar o acesso ao Vault — cada organização tem sua própria política de Vault. O usuário é responsável por publicar os certificados; a plataforma é responsável por consumi-los.

## Considered Options

- **HashiCorp Vault + Kubernetes Operator:** usuário publica certs no Vault; Operator baixa no bootstrap e cria Kubernetes Secrets; padrão de mercado para gestão de secrets em Kubernetes; separa responsabilidades claramente.
- **`kdctl` fazendo push direto via kubeconfig:** simples no MVP; não escala para ambientes com restrições de acesso direto ao cluster remoto; requer que o administrador tenha kubeconfig de todos os clusters.
- **Cert-Manager com CA embutida:** automatiza renovação; requer instalação de Cert-Manager em cada cluster remoto; acoplamento a infraestrutura específica.

## Decision

`kdctl` gera certificados mTLS localmente. O usuário publica no HashiCorp Vault no path `kubediscovery/<environment>/<agent-name>/certs`. O Kubernetes Operator autentica no Vault via AppRole, baixa os certificados no bootstrap e cria o Secret `kubediscovery-agent-certs` no cluster remoto. Os componentes do Data Plane montam o Secret.

## Consequences

- **Positivo:** Padrão de mercado para gestão de secrets — equipes já conhecem e operam Vault.
- **Positivo:** Separação clara de responsabilidades — a plataforma não gerencia acesso de escrita ao Vault.
- **Positivo:** Funciona em clusters remotos sem acesso direto do administrador.
- **Negativo:** HashiCorp Vault é uma dependência de infraestrutura adicional que o usuário deve operar.
- **Negativo:** Se o Vault estiver indisponível, novos agentes não conseguem fazer bootstrap (Secrets já existentes não são afetados).
- **Follow-up:** Documentar path convention do Vault e exemplo de configuração AppRole. Avaliar suporte a outros backends de secrets (AWS Secrets Manager, GCP Secret Manager) em fases posteriores.
