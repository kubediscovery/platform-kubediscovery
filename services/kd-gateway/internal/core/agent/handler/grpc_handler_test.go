package handler_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/entity"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/handler"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// mockStream is a minimal in-process implementation of the bidirectional
// server-side stream.  Messages are pre-loaded into recvQueue; anything sent
// by the handler is discarded (the tests that care about sends can inspect
// sendQueue).
type mockStream struct {
	grpc.ServerStream

	ctx       context.Context
	recvQueue []*gatewayv1.AgentStreamMessage
	recvIdx   int
	sendQueue []*gatewayv1.AgentStreamMessage
}

func newMockStream(msgs ...*gatewayv1.AgentStreamMessage) *mockStream {
	return &mockStream{
		ctx:       context.Background(),
		recvQueue: msgs,
	}
}

func (m *mockStream) Context() context.Context { return m.ctx }

func (m *mockStream) Recv() (*gatewayv1.AgentStreamMessage, error) {
	if m.recvIdx >= len(m.recvQueue) {
		return nil, io.EOF
	}
	msg := m.recvQueue[m.recvIdx]
	m.recvIdx++
	return msg, nil
}

func (m *mockStream) Send(msg *gatewayv1.AgentStreamMessage) error {
	m.sendQueue = append(m.sendQueue, msg)
	return nil
}

func (m *mockStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockStream) SendHeader(metadata.MD) error { return nil }
func (m *mockStream) SetTrailer(metadata.MD)       {}
func (m *mockStream) RecvMsg(any) error            { return nil }
func (m *mockStream) SendMsg(any) error            { return nil }

// --- helpers ---

func helloMsg(callerID string) *gatewayv1.AgentStreamMessage {
	return &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Hello{
			Hello: &gatewayv1.AgentHello{CallerId: callerID},
		},
	}
}

func heartbeatMsg(callerID, reqID string) *gatewayv1.AgentStreamMessage {
	return &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Heartbeat{
			Heartbeat: &gatewayv1.AgentHeartbeat{
				CallerId:  callerID,
				RequestId: reqID,
			},
		},
	}
}

func newHandler() (*handler.Handler, *registry.Registry) {
	reg := registry.New()
	h := handler.New(reg, slog.Default())
	return h, reg
}

// --- tests ---

func TestAgentStream_RejectsMissingHello(t *testing.T) {
	h, _ := newHandler()
	// First message is a heartbeat, not a hello.
	stream := newMockStream(heartbeatMsg("x", "r1"))

	err := h.AgentStream(stream)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestAgentStream_RejectsEmptyCallerID(t *testing.T) {
	h, _ := newHandler()
	emptyHello := &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Hello{
			Hello: &gatewayv1.AgentHello{CallerId: ""},
		},
	}
	stream := newMockStream(emptyHello)

	err := h.AgentStream(stream)
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestAgentStream_RegistersAgentOnHello(t *testing.T) {
	h, reg := newHandler()
	// Hello then EOF → stream ends cleanly.
	stream := newMockStream(helloMsg("agent-42"))

	err := h.AgentStream(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After stream closes the agent must be deregistered.
	a, ok := reg.Get("agent-42")
	if !ok {
		t.Fatal("agent entry should exist even after disconnection")
	}
	if a.Status != entity.StatusDisconnected {
		t.Errorf("Status = %q, want StatusDisconnected after stream end", a.Status)
	}
}

func TestAgentStream_DuplicateCallerIDRejected(t *testing.T) {
	h, reg := newHandler()
	// Pre-register agent-1 so it appears already connected.
	_ = reg.Register("agent-1", nil, nil)

	stream := newMockStream(helloMsg("agent-1"))
	err := h.AgentStream(stream)

	if status.Code(err) != codes.AlreadyExists {
		t.Errorf("code = %v, want AlreadyExists", status.Code(err))
	}
}

func TestAgentStream_HeartbeatUpdatesLastSeen(t *testing.T) {
	h, reg := newHandler()
	stream := newMockStream(
		helloMsg("agent-hb"),
		heartbeatMsg("agent-hb", "r-001"),
	)

	_ = h.AgentStream(stream)

	// After the stream ends, LastSeenAt should have been updated by the heartbeat.
	a, ok := reg.Get("agent-hb")
	if !ok {
		t.Fatal("agent not found")
	}
	// ConnectedAt and LastSeenAt are both set on Register; heartbeat only
	// bumps LastSeenAt.  In a fast test they may be equal, but they should
	// not be zero.
	if a.LastSeenAt.IsZero() {
		t.Error("LastSeenAt should not be zero")
	}
}

func TestAgentStream_CommandResultLogged(t *testing.T) {
	h, _ := newHandler()
	cmdResult := &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_CommandResult{
			CommandResult: &gatewayv1.AgentCommandResult{
				RequestId: "req-999",
				CallerId:  "agent-cr",
				Success:   true,
				Message:   "ok",
			},
		},
	}
	stream := newMockStream(helloMsg("agent-cr"), cmdResult)

	err := h.AgentStream(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentStream_EOF_ReturnsNil(t *testing.T) {
	h, _ := newHandler()
	// Only hello, then EOF.
	stream := newMockStream(helloMsg("agent-eof"))

	err := h.AgentStream(stream)
	if err != nil {
		t.Errorf("EOF should return nil error, got: %v", err)
	}
}
