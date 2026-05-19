## ADDED Requirements

### Requirement: Persistência estruturada com PostgreSQL

Feature: kd-store persiste estado de clusters, agentes, eventos e configurações
Rule: Toda persistência estruturada usa PostgreSQL com migrations versionadas via golang-migrate.

#### Scenario: Registro de novo cluster persistido
- **GIVEN** um novo cluster registrado via `kdctl`
- **WHEN** o gateway persiste o registro
- **THEN** o cluster é salvo no PostgreSQL com UID único, nome, ambiente, status `unregistered` e timestamp

#### Scenario: Evento de problema persistido com análise
- **GIVEN** `AnalysisResult` recebido pelo gateway após análise LLM
- **WHEN** o gateway persiste o resultado
- **THEN** evento é salvo com `MemoryKey`, severidade, diagnóstico, recomendações e timestamp

#### Scenario: Migration executada na inicialização
- **GIVEN** nova versão do kd-store com migration pendente
- **WHEN** o serviço inicia
- **THEN** golang-migrate aplica as migrations pendentes antes de aceitar conexões

### Requirement: Memória semântica LLM com pgvector

Feature: kd-store indexa histórico de análises para busca semântica pelo kd-analyzer
Rule: Embeddings de análises são armazenados com pgvector e indexados por `MemoryKey = clusterName+environment+namespace`.

#### Scenario: Embedding de análise indexado no pgvector
- **GIVEN** `AnalysisResult` com `MemoryKey = "prod-us-eastproductionpayments"`
- **WHEN** o kd-analyzer persiste o resultado
- **THEN** o embedding é armazenado na tabela `analysis_memory` com a `MemoryKey` como partição

#### Scenario: Busca semântica retorna histórico relevante
- **GIVEN** nova análise para `MemoryKey = "prod-us-eastproductionpayments"`
- **WHEN** o kd-analyzer busca contexto histórico
- **THEN** retorna os N análises mais similares semanticamente para aquela chave

#### Scenario: Sem histórico para nova MemoryKey
- **GIVEN** análise para cluster/namespace nunca antes analisado
- **WHEN** o kd-analyzer busca contexto histórico
- **THEN** retorna lista vazia sem erro — análise prossegue sem contexto histórico

### Requirement: Cache Redis para estado efêmero

Feature: kd-store usa Redis para estado de alta frequência e dados temporários
Rule: Estado de agentes conectados, sessões ativas e dados de cache devem usar Redis.

#### Scenario: Status de agente cacheado no Redis
- **GIVEN** agente `agent-srv001` conectado ao gateway
- **WHEN** o gateway atualiza o status do agente
- **THEN** o status é escrito no Redis com TTL de 30 segundos, renovado a cada heartbeat

#### Scenario: Cache expirado retorna ao PostgreSQL
- **GIVEN** consulta de status de agente com cache expirado no Redis
- **WHEN** a consulta é realizada
- **THEN** o sistema lê do PostgreSQL e repovoа o cache Redis

## MODIFIED Requirements

## REMOVED Requirements
