# 0002. Monorepo com go.work para Desenvolvimento Local

- Status: accepted
- Date: 2026-05-19

## Context

A plataforma é composta por 7+ serviços Go independentes (`kd-gateway`, `kd-agent`, `kd-analyzer`, `kd-executor`, `kd-store`, `kd-mcp`, `kdctl`) e uma biblioteca compartilhada (`kd-libs`). Durante o desenvolvimento, mudanças em `kd-libs` precisam ser visíveis imediatamente em todos os serviços sem publicar versões intermediárias. Cada serviço deve manter ciclo de release independente em produção.

## Considered Options

- **`go.work` na raiz do monorepo:** mecanismo oficial do Go para workspaces multi-módulo; sem `replace` directives; funciona nativamente com `go build`, `go test` e gopls; `go.work` não é commitado em CI/CD.
- **`replace` directives em cada `go.mod`:** funciona mas polui os `go.mod` com paths locais que precisam ser removidos antes de cada release; propenso a erros humanos.
- **Único `go.mod` na raiz:** acopla todos os serviços num único ciclo de release; impede deploys independentes; não escala para times trabalhando em serviços diferentes.

## Decision

Monorepo com `go.work` na raiz conectando todos os módulos durante o desenvolvimento. Cada serviço mantém seu próprio `go.mod` com module path `github.com/kubediscovery/<service>`. O arquivo `go.work` não é commitado em CI/CD — cada pipeline de CI opera no diretório do serviço com seu `go.mod` independente.

## Consequences

- **Positivo:** DX local fluida — mudanças em `kd-libs` são imediatamente visíveis em todos os serviços sem publicação.
- **Positivo:** `go build ./...` na raiz compila tudo; `go test ./...` testa tudo.
- **Negativo:** `go.work` deve ser documentado claramente para novos desenvolvedores — a ausência do arquivo em CI pode causar confusão.
- **Negativo:** Versioning de `kd-libs` em produção requer disciplina — cada serviço deve fixar a versão da lib no `go.mod`.
- **Follow-up:** Adicionar `go.work` ao `.gitignore` e documentar o setup de desenvolvimento no README.
