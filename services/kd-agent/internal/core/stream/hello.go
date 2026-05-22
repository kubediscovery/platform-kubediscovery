package stream

import (
	"errors"
	"fmt"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ErrEmptyCallerID is returned by NewHelloMessage when the caller_id is blank.
// The caller_id maps directly to the AGENT_ID environment variable and must
// be unique per kd-agent instance.
var ErrEmptyCallerID = errors.New("caller_id must not be empty")

// NewHelloMessage constructs the initial AgentStreamMessage containing an
// AgentHello frame.  This is the first message the agent sends after the
// bidirectional stream is established.
//
// callerID is the value of the AGENT_ID environment variable.
// metadata is a free-form diagnostics map (e.g. {"kind": "agent->server"}).
//
// An error is returned when callerID is empty or metadata cannot be
// serialised as a protobuf Struct.
func NewHelloMessage(callerID string, metadata map[string]any) (*gatewayv1.AgentStreamMessage, error) {
	if callerID == "" {
		return nil, ErrEmptyCallerID
	}

	md, err := structpb.NewStruct(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	return &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Hello{
			Hello: &gatewayv1.AgentHello{
				CallerId: callerID,
				Metadata: md,
				SentAt:   timestamppb.Now(),
			},
		},
	}, nil
}

// DefaultMetadata returns the standard diagnostics map included in every
// AgentHello frame.  Callers may extend the returned map before passing it
// to NewHelloMessage.
func DefaultMetadata() map[string]any {
	return map[string]any{"kind": "agent->server"}
}
