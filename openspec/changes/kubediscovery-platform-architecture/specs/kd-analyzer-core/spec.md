## ADDED Requirements

### Requirement: Pipeline LLM multi-agente no Control Plane

Feature: kd-analyzer processa eventos Kubernetes via pipeline LLM usando Google ADK e Databricks
Rule: Por padrão, toda análise LLM é executada no Control Plane. Execução no Data Plane é opt-in.

#### Scenario: Gateway solicita análise de evento ao analyzer
- **GIVEN** evento `POD_PENDING` recebido pelo gateway com logs e eventos do pod
- **WHEN** o gateway invoca o kd-analyzer
- **THEN** o analyzer executa o pipeline LLM e retorna `AnalysisResult` com diagnóstico, severidade e recomendações

#### Scenario: Analyzer executa agentes em sessões isoladas
- **GIVEN** pipeline com agentes `log_analyst`, `event_analyst` e `karpenter_analyst`
- **WHEN** o pipeline é executado
- **THEN** cada agente roda em sessão ADK isolada, sem histórico compartilhado entre agentes

#### Scenario: Output de um agente é input do próximo
- **GIVEN** `log_analyst` completou análise de logs
- **WHEN** `event_analyst` é invocado
- **THEN** recebe o output do `log_analyst` como input manual (Go pipeline, não ADK multi-agent)

### Requirement: Ativação condicional do karpenter_analyst

Feature: karpenter_analyst é ativado apenas para falhas de scheduling específicas
Rule: O agente `karpenter_analyst` só é invocado quando o pod está `Pending` E os eventos contêm indicadores de scheduling failure.

#### Scenario: karpenter_analyst ativado para untolerated taint
- **GIVEN** pod em estado `Pending` com evento contendo `"untolerated taint"`
- **WHEN** o pipeline avalia a condição de ativação
- **THEN** `karpenter_analyst` é incluído no pipeline de análise

#### Scenario: karpenter_analyst não ativado para CrashLoopBackOff
- **GIVEN** pod em estado `CrashLoopBackOff`
- **WHEN** o pipeline avalia a condição de ativação
- **THEN** `karpenter_analyst` não é incluído no pipeline

### Requirement: AnalysisResult com chave de memória para kd-store

Feature: Resultado da análise inclui chave para indexação semântica no kd-store
Rule: Todo `AnalysisResult` deve incluir `MemoryKey = clusterName+environment+namespace` para indexação no pgvector.

#### Scenario: AnalysisResult retornado com MemoryKey correta
- **GIVEN** análise de pod no cluster `prod-us-east`, ambiente `production`, namespace `payments`
- **WHEN** o analyzer conclui o pipeline
- **THEN** `AnalysisResult.MemoryKey = "prod-us-eastproductionpayments"`

### Requirement: Modo local opt-in via analyzer.mode no CRD

Feature: kd-analyzer pode ser executado localmente no Data Plane quando configurado
Rule: O campo `analyzer.mode` no CRD `Agent` controla onde a análise é executada; o default é `remote` (Control Plane).

#### Scenario: Análise executada no Control Plane por padrão
- **GIVEN** CRD Agent sem campo `analyzer.mode` definido
- **WHEN** um evento é reportado pelo kd-agent
- **THEN** a análise é delegada ao `kd-analyzer` no Control Plane via gateway

#### Scenario: Análise executada localmente quando mode=local
- **GIVEN** CRD Agent com `analyzer.mode: local`
- **WHEN** um evento é reportado pelo kd-agent
- **THEN** a análise é executada pelo `kd-analyzer` local no Data Plane, sem enviar dados ao Control Plane

## MODIFIED Requirements

## REMOVED Requirements
