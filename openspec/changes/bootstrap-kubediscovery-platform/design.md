## Context

Kubediscovery is currently a bootstrap Go module with only `main.go`, while the target product requires a distributed control-plane/data-plane architecture with secure bidirectional communication, policy enforcement, and operator-managed edge components. The proposal defines seven new capabilities spanning cluster lifecycle, secure connectivity, remote execution routing, AI orchestration, authorization, and observability. The design must enable incremental delivery, preserve clear service boundaries, and avoid locking implementation to a single deployment topology too early.

Primary constraints:
- All control-plane <-> data-plane traffic uses bidirectional gRPC over mTLS.
- Data-plane behavior is declarative through a Kubernetes `Agent` CR and controller reconciliation.
- Authorization must support Kubernetes-style resource scoping plus platform/LLM scopes.
- The repository will evolve from single-module bootstrap to multi-service structure without breaking early iteration speed.

Stakeholders:
- Platform engineering (service architecture, release model).
- SRE/DevOps (operability, observability, day-2 lifecycle actions).
- Security/compliance (identity, authorization, auditability).
- Users/operators consuming `kdctl` and dashboard workflows.

## Goals / Non-Goals

**Goals:**
- Define a reference architecture for control plane (`kd-gateway`, `kd-executor`, `kd-analyzer`, `kd-store`, `kd-cache`) and data plane (`kd-agent`, agent executor/analyzer workers) with explicit responsibility boundaries.
- Define protocol and identity patterns for cluster registration, healthcheck, command dispatch, result correlation, and event ingestion over streaming gRPC.
- Define an operator-first lifecycle where `Agent` CRs declaratively manage data-plane component enablement.
- Define an authorization integration point (OPA) in the command path before remote action dispatch.
- Define baseline observability requirements (metrics, traces, logs) that connect user request -> gateway routing -> agent execution -> response.
- Enable phased implementation from MVP to advanced capabilities without redesigning core contracts.

**Non-Goals:**
- Full protobuf field-level schemas for every RPC in this artifact.
- Production HA topology details for every backend dependency (Postgres/Redis sizing, sharding).
- Detailed UI design for `kd-portal`.
- Final model/provider strategy for all LLM backends.

## Decisions

1) Control-plane centric command orchestration
- Decision: `kd-gateway` is the authoritative ingress and orchestration boundary for all user/system requests targeting clusters.
- Rationale: keeps policy checks, routing, auditing, and correlation IDs centralized.
- Alternative considered: direct client-to-agent command channels from CLI/portal; rejected due to fragmented auth/audit and higher trust surface.

2) Long-lived bidirectional gRPC streams per cluster
- Decision: each `kd-agent` establishes and maintains a long-lived mTLS-authenticated stream to `kd-gateway` for heartbeats, command delivery, results, and event push.
- Rationale: efficient NAT traversal pattern, lower connection churn, and natural fit for async execution.
- Alternative considered: request/response polling from agents; rejected due to latency and weaker real-time control.

3) UID-bound cluster identity with certificate lifecycle managed by `kdctl`
- Decision: provisioning issues a stable cluster UID and per-cluster cert chain; gateway validates cert subject/SAN against registered UID and status.
- Rationale: deterministic identity mapping and straightforward revocation/pause semantics.
- Alternative considered: token-only authentication; rejected as insufficient for required mTLS mutual auth guarantees.

4) Operator-managed data plane via `Agent` CRD
- Decision: a Kubernetes controller reconciles `Agent` specs to desired runtime state for `kd-agent`, executor, analyzer, and optional troubleshooting image support.
- Rationale: native Kubernetes control loop, declarative enable/disable, and predictable drift correction.
- Alternative considered: ad-hoc Helm values without controller; rejected due to weaker lifecycle semantics and poorer runtime toggling.

5) Policy decision point at gateway using OPA
- Decision: gateway performs OPA authorization before dispatching remote actions and before granting LLM-scoped operations.
- Rationale: central enforcement across all entry paths with Kubernetes-like verbs/kinds/namespaces and platform scopes.
- Alternative considered: policy checks only in agent; rejected because requests must be denied before crossing trust boundary.

6) Event and execution memory model
- Decision: persist canonical lifecycle and execution history in `kd-store`; use `kd-cache` for short-lived routing/session state and fast coordination.
- Rationale: separates durability concerns from low-latency operational state.
- Alternative considered: cache-only model; rejected due to audit/history requirements.

7) Observability as contract, not optional add-on
- Decision: every cross-service request path carries trace context and correlation IDs; services expose Prometheus metrics and structured logs.
- Rationale: troubleshooting distributed flows is impossible without end-to-end telemetry.
- Alternative considered: defer observability until later phases; rejected due to high debugging cost and migration friction.

## Risks / Trade-offs

- [Risk] Stream management complexity (disconnect storms, backoff, duplicate deliveries) -> Mitigation: explicit connection state machine, idempotent command IDs, bounded retry/backoff policies.
- [Risk] Early over-design from bootstrap state -> Mitigation: phase delivery (MVP first: registration + health + basic remote exec), keep interfaces versioned and minimal.
- [Risk] Policy latency or policy drift -> Mitigation: bundle/version Rego policies with rollout controls, add policy decision metrics and denial audit logs.
- [Risk] LLM token and data exposure across planes -> Mitigation: server-owned secret management by default, scoped delegation, strict redaction and audit trails.
- [Risk] Operator CRD growth and compatibility burden -> Mitigation: versioned CRD (`v1beta1` -> `v1`), defaulting/validation webhooks, backwards-compatible field evolution.
- [Risk] Multi-service repo restructuring churn -> Mitigation: incremental folder migration with stable module boundaries and CI checks per service.

## Migration Plan

1. Establish repository skeleton and shared libraries (config, logging, tracing, errors, auth context) while preserving a runnable baseline.
2. Deliver MVP contracts: cluster provisioning/registration, mTLS validation, healthcheck stream, and minimal remote command execution.
3. Introduce `Agent` CRD + controller to manage data-plane component toggles and rollout behavior.
4. Integrate OPA decision point in gateway request flow and ship initial policy packs.
5. Add analyzer/event pipelines (server-first), then optional client-side analyzer mode based on policy/config.
6. Expand observability dashboards and SLO-oriented alerts for connection health, command latency, and error budgets.
7. Rollback strategy: disable new routing paths via feature flags, pause affected cluster UIDs, and revert to prior service versions with compatible proto contracts.

## Open Questions

- What is the exact protobuf service decomposition (single gateway service vs multiple domain services) for versioning and ownership?
- Which command classes must be strictly synchronous vs asynchronous at MVP?
- Should OPA run embedded in gateway process or as sidecar/service in initial deployment?
- What minimal audit log schema is required for compliance from day one?
- What is the default policy for server-side vs client-side LLM execution per cluster environment?
