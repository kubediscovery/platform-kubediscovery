## ADDED Requirements

### Requirement: Central authorization decision point
The gateway MUST evaluate authorization policy decisions before allowing platform, Kubernetes, or LLM-scoped operations.

#### Scenario: Allow authorized Kubernetes action
- **WHEN** a requester asks to perform an allowed Kubernetes verb on a permitted kind and namespace
- **THEN** the gateway SHALL allow dispatch only after receiving an allow decision from policy evaluation

### Requirement: Fine-grained policy scopes
Authorization policy MUST support Kubernetes scopes (`verbs`, `kinds`, `namespaces`), platform scopes (`cluster:pause`, `cluster:register`, `cluster:delete`), and AI scopes (`llm:analyze`, `llm:query`).

#### Scenario: Deny unauthorized LLM usage
- **WHEN** a requester without `llm:analyze` permission submits an analysis request
- **THEN** the system SHALL deny the request and record the denied scope in audit output

### Requirement: Auditable policy outcomes
Each policy decision MUST produce auditable metadata including requester identity, target scope, decision result, and correlation ID.

#### Scenario: Record deny audit entry
- **WHEN** a policy decision denies a request
- **THEN** the gateway SHALL emit an audit record containing identity, requested action, scope, reason code, and correlation ID
