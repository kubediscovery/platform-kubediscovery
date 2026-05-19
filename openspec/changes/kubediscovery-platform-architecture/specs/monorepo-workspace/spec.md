## ADDED Requirements

### Requirement: Monorepo layout com go.work

Feature: Monorepo multi-módulo Go com workspace unificado
Rule: Todos os módulos Go do projeto devem ser conectados via `go.work` na raiz para desenvolvimento local, mantendo `go.mod` independentes para publicação.

#### Scenario: Workspace conecta todos os módulos localmente
- **GIVEN** o repositório clonado localmente com `go.work` na raiz
- **WHEN** o desenvolvedor executa `go build ./...` na raiz
- **THEN** todos os serviços compilam usando as versões locais de `kd-libs` sem necessidade de `replace` directives

#### Scenario: Módulos publicam independentemente
- **GIVEN** cada serviço com seu próprio `go.mod` (ex: `github.com/kubediscovery/kd-gateway`)
- **WHEN** um serviço é publicado via tag Git
- **THEN** ele pode ser importado por outros projetos sem depender do monorepo

#### Scenario: go.work não é commitado em CI/CD
- **GIVEN** um pipeline de CI executando build de um serviço específico
- **WHEN** o pipeline executa `go build` no diretório do serviço
- **THEN** o build usa apenas o `go.mod` local, sem depender do `go.work` da raiz

### Requirement: Estrutura de diretórios padronizada por serviço

Feature: Layout interno de cada serviço Go
Rule: Cada serviço deve seguir o layout padrão definido: `cmd/`, `configs/`, `internal/core/`, `internal/infrastructure/`, `pkg/`.

#### Scenario: Novo serviço segue o layout padrão
- **GIVEN** um novo serviço sendo criado (ex: `kd-gateway`)
- **WHEN** o desenvolvedor inicializa o serviço
- **THEN** a estrutura contém `cmd/grpc/main.go`, `configs/`, `internal/core/`, `internal/infrastructure/`, `pkg/`

#### Scenario: Entry point usa UberFX
- **GIVEN** o arquivo `cmd/grpc/main.go` de qualquer serviço
- **WHEN** o serviço é iniciado
- **THEN** o entry point chama `fx.New(...)` com os módulos de configs, observability, infrastructure e core

### Requirement: Biblioteca compartilhada kd-libs

Feature: Libs compartilhadas entre todos os serviços
Rule: Código reutilizável entre serviços deve residir em `libs/` com módulo `github.com/kubediscovery/kd-libs`.

#### Scenario: Serviço importa utilitário de kd-libs
- **GIVEN** um serviço que precisa de um utilitário de erros ou logging
- **WHEN** o desenvolvedor importa `github.com/kubediscovery/kd-libs/pkg/errors`
- **THEN** o código compila usando a versão local via `go.work` em desenvolvimento

## MODIFIED Requirements

## REMOVED Requirements
