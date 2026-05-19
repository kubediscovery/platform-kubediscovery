## 1. Repository and Service Bootstrap

- [ ] 1.1 Create base project structure for services, shared libs, API, infra, and build assets aligned with the design.
- [ ] 1.2 Add shared configuration, logging, tracing, and error packages for reuse across `kd-gateway`, `kd-agent`, `kdctl`, executor, and analyzer services.
- [ ] 1.3 Introduce dependency injection wiring with Uber FX bootstrap for service startup and graceful shutdown.

## 2. Identity, Provisioning, and Secure Connectivity

- [ ] 2.1 Implement `kdctl` cluster provisioning flow that creates UID-bound cluster records and mTLS certificate materials.
- [ ] 2.2 Implement gateway cluster registry persistence with lifecycle states (`unregistered`, `registered`, `paused`, `deleted`).
- [ ] 2.3 Implement bidirectional gRPC stream authentication and certificate-to-UID validation in `kd-gateway` and `kd-agent`.
- [ ] 2.4 Implement heartbeat/health signaling and degraded-connection detection for registered clusters.

## 3. Data Plane Operator and Runtime Reconciliation

- [ ] 3.1 Define `kubediscovery.io/v1beta1` `Agent` CRD schema with flags for `agent`, `executor`, `analyzer`, and troubleshooting image.
- [ ] 3.2 Implement controller reconciliation to create/update/delete data-plane runtime workloads from `Agent` desired state.
- [ ] 3.3 Implement runtime toggling behavior validation so disabled components are removed and enabled components converge.

## 4. Remote Execution Routing and Results

- [ ] 4.1 Implement gateway command ingress with correlation IDs and routing to target cluster streams.
- [ ] 4.2 Implement agent-side command dispatch to executor with synchronous response flow.
- [ ] 4.3 Implement asynchronous execution lifecycle with durable command IDs and completion reporting.
- [ ] 4.4 Persist execution records and status transitions in `kd-store`, with transient coordination in `kd-cache`.

## 5. Authorization and Policy Enforcement

- [ ] 5.1 Integrate OPA decision point in gateway pre-dispatch path for Kubernetes-scoped, platform-scoped, and LLM-scoped actions.
- [ ] 5.2 Implement policy input model containing requester identity, cluster UID, action scope, and target resource context.
- [ ] 5.3 Implement auditable allow/deny decision logging with correlation IDs and reason metadata.

## 6. AI Analysis Orchestration

- [ ] 6.1 Implement control-plane analyzer flow for server-side LLM execution.
- [ ] 6.2 Implement event-driven analyzer pipeline from executor watcher events via agent to gateway.
- [ ] 6.3 Implement policy/config-gated client-side analyzer mode and token delegation controls.

## 7. Observability and Operational Readiness

- [ ] 7.1 Expose Prometheus metrics for request throughput, errors, latency, and stream health in core services.
- [ ] 7.2 Implement OpenTelemetry trace propagation across requester, gateway, agent, executor, and analyzer flows.
- [ ] 7.3 Standardize structured logging fields (service, cluster UID, correlation ID, severity) across services.
- [ ] 7.4 Define initial SLO dashboards and alerts for connectivity health and command execution reliability.

## 8. Verification and Incremental Rollout

- [ ] 8.1 Validate MVP end-to-end flow: provision -> register -> health -> authorized remote execution -> response.
- [ ] 8.2 Validate lifecycle controls (`pause` and `delete`) and enforcement in dispatch and connectivity paths.
- [ ] 8.3 Validate OPA policy outcomes and audit trails for allow/deny scenarios including LLM scopes.
- [ ] 8.4 Roll out by feature flags and document rollback steps for routing, analyzer, and operator features.
