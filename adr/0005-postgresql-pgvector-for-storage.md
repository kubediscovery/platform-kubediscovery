# 0005. PostgreSQL com pgvector para Persistência e Memória LLM

- Status: accepted
- Date: 2026-05-19

## Context

O `kd-store` precisa atender dois casos de uso distintos: (1) persistência estruturada relacional — clusters, agentes, eventos, configurações com queries SQL e joins; (2) memória semântica para o pipeline LLM — embeddings de análises indexados por `clusterName+environment+namespace` para busca por similaridade. Manter infraestrutura mínima é prioritário no MVP.

## Considered Options

- **PostgreSQL + pgvector:** um único banco para ambos os casos de uso; pgvector suporta similarity search com índices HNSW/IVFFlat; operacionalmente familiar para a maioria das equipes; extensão madura e mantida ativamente.
- **PostgreSQL + Qdrant/Weaviate:** separação clara de responsabilidades; melhor performance e features para vector search em escala; adiciona um componente de infraestrutura extra para operar no MVP.
- **MongoDB + Atlas Vector Search:** schema flexível; vector search integrado; menos familiar para equipes acostumadas com SQL; sem garantias de consistência relacional.

## Decision

`kd-store` usa PostgreSQL com extensão `pgvector` para ambos os casos de uso. Redis complementa como cache para estado efêmero e alta frequência. Migrations são gerenciadas via `golang-migrate` com arquivos versionados e reversíveis.

## Consequences

- **Positivo:** Infraestrutura mínima no MVP — um banco relacional para dois casos de uso.
- **Positivo:** Operacionalmente familiar; equipes DevOps já conhecem PostgreSQL.
- **Negativo:** pgvector tem limitações de escala comparado a vector stores dedicados — índices HNSW requerem tuning para grandes volumes.
- **Negativo:** Queries de similarity search e queries relacionais no mesmo banco podem competir por recursos em carga alta.
- **Follow-up:** Monitorar performance de similarity search via Prometheus. Planejar migração para Qdrant/Weaviate se volume de embeddings ou latência de retrieval exigir.
