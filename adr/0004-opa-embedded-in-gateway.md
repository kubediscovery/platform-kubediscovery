# 0004. OPA Embutido como Biblioteca Go no kd-gateway

- Status: accepted
- Date: 2026-05-19

## Context

O `kd-gateway` precisa avaliar políticas de autorização fine-grained em cada chamada gRPC e HTTP — escopos Kubernetes (verbs/namespaces/kinds), IA (`llm:analyze`) e Plataforma (`cluster:pause`). A avaliação ocorre no hot path de cada request, tornando latência um fator crítico. O modelo de políticas deve ser expressivo o suficiente para suportar ABAC (Attribute-Based Access Control) no estilo Kubernetes.

## Considered Options

- **OPA embutido (`github.com/open-policy-agent/opa/rego`):** avaliação in-process em microsegundos; sem dependência de rede no hot path; políticas Rego carregadas e compiladas em cache na inicialização; deploy simplificado.
- **OPA como sidecar:** avaliação via HTTP local (~1ms de latência); gestão de políticas centralizada independente do gateway; adiciona complexidade de deploy (sidecar em cada pod do gateway).
- **OPA como serviço externo:** gestão centralizada de políticas para múltiplas instâncias; latência de rede inaceitável no hot path de autorização; ponto único de falha externo.

## Decision

OPA é embutido como biblioteca Go (`github.com/open-policy-agent/opa/rego`) no `kd-gateway`. Políticas Rego são carregadas de arquivos em `configs/policies/` e compiladas em cache na inicialização do serviço. A avaliação ocorre como interceptor gRPC antes de qualquer handler de negócio.

## Consequences

- **Positivo:** Latência de autorização em microsegundos — sem overhead de rede no hot path.
- **Positivo:** Deploy simplificado — sem sidecar ou serviço externo para operar.
- **Negativo:** Atualização de políticas requer restart do gateway (sem hot reload no MVP).
- **Negativo:** Footprint de memória do gateway aumenta com as políticas compiladas em cache.
- **Follow-up:** Avaliar hot reload de políticas via `WatchConfig` do Viper ou polling de arquivo em fase posterior. Monitorar uso de memória via Prometheus.
