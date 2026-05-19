# 0003. UberFX como Framework de Dependency Injection

- Status: accepted
- Date: 2026-05-19

## Context

Todos os serviços Go da plataforma precisam de um padrão consistente de dependency injection e lifecycle management (inicialização ordenada, graceful shutdown). A consistência entre serviços reduz a curva de aprendizado e facilita onboarding de novos desenvolvedores. O framework escolhido deve suportar modularização por domínio e integrar-se bem com o padrão de layout de serviço adotado.

## Considered Options

- **`go.uber.org/fx`:** lifecycle hooks (`OnStart`/`OnStop`) para graceful shutdown; wiring declarativo; `fx.Module` mapeia diretamente para domínios; amplamente adotado em produção; suporte ativo da Uber.
- **Google Wire:** geração de código em tempo de compilação; sem overhead de runtime; menos flexível para módulos dinâmicos; requer regeneração ao mudar dependências.
- **Injeção manual:** zero dependências externas; máximo controle; não escala para 7+ serviços com dezenas de dependências cada; `main.go` se torna difícil de manter.

## Decision

Todos os serviços usam `go.uber.org/fx` para dependency injection e lifecycle management. O entry point de cada serviço chama `fx.New(configs.Module, observability.Module, infrastructure.Module, core.Module).Run()`. Cada domínio expõe `var Module = fx.Module(...)` em `module.go`.

## Consequences

- **Positivo:** Graceful shutdown garantido via hooks `OnStop` — conexões gRPC, pools de DB e streams são fechados ordenadamente.
- **Positivo:** Estrutura de módulos consistente entre todos os serviços facilita code review e onboarding.
- **Negativo:** Erros de wiring são detectados apenas em runtime (não em compilação como Wire).
- **Negativo:** Stack traces de erros de DI podem ser verbosos e difíceis de depurar inicialmente.
- **Follow-up:** Criar template de serviço com estrutura FX padrão para acelerar criação de novos serviços.
