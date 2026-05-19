## ADDED Requirements

### Requirement: Bidirectional gRPC transport with mTLS
All control-plane to data-plane communication MUST occur over bidirectional gRPC streams authenticated with mutual TLS.

#### Scenario: Reject non-mTLS connection
- **WHEN** a data-plane client attempts to connect without valid client certificate authentication
- **THEN** the gateway SHALL deny stream establishment and record an authentication failure event

### Requirement: Certificate-to-UID validation
The gateway MUST validate certificate identity attributes against the provisioned cluster UID before accepting an operational stream.

#### Scenario: Certificate UID mismatch
- **WHEN** a connecting agent presents a certificate that does not map to the requested cluster UID
- **THEN** the gateway SHALL reject the connection and keep the cluster state unchanged

### Requirement: Health stream continuity
Registered clusters MUST maintain periodic health signaling over the active stream, and the control plane MUST detect missed heartbeats.

#### Scenario: Heartbeat timeout marks degraded connectivity
- **WHEN** expected heartbeats are not received within the configured timeout window
- **THEN** the control plane SHALL mark the cluster connectivity as degraded and emit a health event
