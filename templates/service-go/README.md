# Go Service Template

Template base para novos serviços Kubediscovery com layout padrão.

## Estrutura

- `cmd/grpc/`: bootstrap da aplicação com UberFX.
- `configs/`: configuração via Viper.
- `internal/core/`: domínio (entidades, serviços, repositórios e handlers).
- `internal/infrastructure/`: adaptadores técnicos (gRPC, banco, cache, observabilidade).
- `pkg/`: utilitários exportáveis do serviço.

## Como usar

1. Copie este diretório para `services/<nome-servico>/`.
2. Ajuste `go.mod` para o módulo do serviço.
3. Renomeie `internal/core/example` para o domínio real.
4. Complete providers e módulos UberFX.
