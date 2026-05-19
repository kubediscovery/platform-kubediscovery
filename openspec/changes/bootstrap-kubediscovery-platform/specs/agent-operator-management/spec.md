## ADDED Requirements

### Requirement: Agent custom resource reconciliation
The data-plane operator MUST reconcile `kubediscovery.io/v1beta1` `Agent` resources into runtime component state for `kd-agent`, `executor`, and `analyzer`.

#### Scenario: Enable runtime components from Agent spec
- **WHEN** an `Agent` custom resource sets `agent.enabled=true` and `executor.enabled=true`
- **THEN** the operator SHALL ensure corresponding runtime workloads are created and kept running

### Requirement: Declarative component toggling
Component enablement flags in the `Agent` resource MUST be authoritative, and reconciliation MUST create, update, or remove runtime components to match desired state.

#### Scenario: Disable analyzer component
- **WHEN** an existing `Agent` resource is updated with `analyzer.enabled=false`
- **THEN** the operator SHALL scale down or remove analyzer runtime resources while leaving other enabled components intact

### Requirement: Troubleshooting image policy handling
The operator MUST support optional troubleshooting image controls defined in the `Agent` resource and enforce disabled-by-default behavior when unset.

#### Scenario: Troubleshooting image omitted
- **WHEN** an `Agent` resource does not enable troubleshooting image support
- **THEN** the operator SHALL not deploy troubleshooting workloads for that cluster
