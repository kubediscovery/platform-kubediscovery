## ADDED Requirements

### Requirement: Routed command dispatch pipeline
The platform MUST route remote operations through the control path `requester -> gateway -> kd-agent -> executor` and return responses through the reverse path.

#### Scenario: Execute synchronous command
- **WHEN** an authorized requester submits a synchronous execution request for a registered cluster
- **THEN** the gateway SHALL dispatch the command to the target agent and return execution output correlated to the original request ID

### Requirement: Asynchronous execution correlation
The system MUST support asynchronous command execution with durable command identifiers and eventual result reporting.

#### Scenario: Receive async completion result
- **WHEN** an asynchronous command completes in the data plane
- **THEN** the agent SHALL publish the result with the original command ID and the gateway SHALL persist and expose completion state

### Requirement: Policy-aware routing gate
The gateway MUST perform authorization checks before dispatching any remote execution request.

#### Scenario: Unauthorized command denied pre-dispatch
- **WHEN** a requester lacks required permission for target verb/resource/namespace scope
- **THEN** the gateway SHALL deny the request and MUST NOT forward it to the data-plane agent
