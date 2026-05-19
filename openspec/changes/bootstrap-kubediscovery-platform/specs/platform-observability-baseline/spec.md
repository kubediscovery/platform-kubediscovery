## ADDED Requirements

### Requirement: Metrics coverage for core services
Core services (`kd-gateway`, `kd-agent`, executor, analyzer) MUST expose Prometheus-compatible metrics for request throughput, error rates, latency, and connection health.

#### Scenario: Export gateway request metrics
- **WHEN** gateway processes routed commands
- **THEN** it SHALL expose labeled metrics for total requests, failures, and duration histograms

### Requirement: End-to-end trace propagation
All cross-service operations MUST propagate trace context and correlation identifiers from ingress through execution and response.

#### Scenario: Trace spans linked across gateway and agent
- **WHEN** a routed command traverses gateway and data-plane agent
- **THEN** emitted spans SHALL share trace lineage that allows reconstructing the full request path

### Requirement: Structured operational logging
Services MUST emit structured logs containing timestamp, severity, service name, cluster UID (when applicable), and correlation ID.

#### Scenario: Log failed remote execution with context
- **WHEN** remote execution fails in the data plane
- **THEN** the system SHALL record structured error logs that include cluster UID, command ID, failure classification, and correlation ID
