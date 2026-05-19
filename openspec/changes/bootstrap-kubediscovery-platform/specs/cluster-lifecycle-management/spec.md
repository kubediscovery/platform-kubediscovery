## ADDED Requirements

### Requirement: Cluster provisioning and identity lifecycle
The control plane MUST support provisioning managed clusters with a unique immutable UID, human-readable cluster name, and environment metadata. The system MUST persist lifecycle state transitions including `unregistered`, `registered`, `paused`, and `deleted`.

#### Scenario: Provision new cluster
- **WHEN** an operator provisions a cluster using `kdctl` with cluster name and environment
- **THEN** the system SHALL create a cluster record with immutable UID, initial `unregistered` status, and issued bootstrap identity materials

### Requirement: Cluster registration handshake
The control plane MUST register a cluster only after validating presented cluster identity against a provisioned UID and current lifecycle status.

#### Scenario: Successful first registration
- **WHEN** a provisioned cluster in `unregistered` state connects with valid UID-bound identity
- **THEN** the system SHALL transition the cluster to `registered` and mark it eligible for command dispatch

### Requirement: Administrative pause and delete controls
The platform MUST allow administrators to pause and delete clusters by UID, and MUST enforce those states in command routing and connectivity acceptance.

#### Scenario: Pause blocks operations
- **WHEN** an administrator sets a cluster state to `paused`
- **THEN** the system SHALL reject new command dispatches for that UID until resumed or reactivated
