package router_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/router"
)

// mockStream is a minimal in-process bidirectional server stream.
// Received messages are stored in sendQueue; callers can signal when a message
// has arrived via sentNotify so that tests avoid arbitrary sleeps.
type mockStream struct {
	grpc.ServerStream

	ctx context.Context

	mu         sync.Mutex
	sendQueue  []*gatewayv1.AgentStreamMessage
	sentNotify chan struct{}
	sendErr    error // if non-nil, Send returns this error
}

func newMockStream() *mockStream {
	return &mockStream{
		ctx:        context.Background(),
		sentNotify: make(chan struct{}, 16),
	}
}

func (m *mockStream) Context() context.Context { return m.ctx }

func (m *mockStream) Recv() (*gatewayv1.AgentStreamMessage, error) {
	return nil, io.EOF
}

func (m *mockStream) Send(msg *gatewayv1.AgentStreamMessage) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.mu.Lock()
	m.sendQueue = append(m.sendQueue, msg)
	m.mu.Unlock()
	select {
	case m.sentNotify <- struct{}{}:
	default:
	}
	return nil
}

func (m *mockStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockStream) SendHeader(metadata.MD) error { return nil }
func (m *mockStream) SetTrailer(metadata.MD)       {}
func (m *mockStream) RecvMsg(any) error            { return nil }
func (m *mockStream) SendMsg(any) error            { return nil }

// waitSent blocks until at least one Send call has been made or the deadline
// is reached, returning false on timeout.
func (m *mockStream) waitSent(d time.Duration) bool {
	select {
	case <-m.sentNotify:
		return true
	case <-time.After(d):
		return false
	}
}

// lastSent returns the most recently sent message, or nil if the queue is empty.
func (m *mockStream) lastSent() *gatewayv1.AgentStreamMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sendQueue) == 0 {
		return nil
	}
	return m.sendQueue[len(m.sendQueue)-1]
}

// --- helpers ---

func newRouter(ms *mockStream, callerID string) (*router.Router, *registry.Registry) {
	reg := registry.New()
	if ms != nil {
		_ = reg.Register(callerID, ms, nil)
	}
	r := router.New(reg, slog.Default())
	return r, reg
}

func gatewayCmd(requestID, targetCallerID, operation string) *gatewayv1.GatewayCommand {
	return &gatewayv1.GatewayCommand{
		RequestId:      requestID,
		TargetCallerId: targetCallerID,
		Operation:      operation,
	}
}

func commandResult(requestID, callerID string, success bool) *gatewayv1.AgentCommandResult {
	return &gatewayv1.AgentCommandResult{
		RequestId: requestID,
		CallerId:  callerID,
		Success:   success,
		Message:   "ok",
	}
}

// --- tests: Send ---

func TestRouter_Send_AgentNotRegistered(t *testing.T) {
	r, _ := newRouter(nil, "")

	ctx := context.Background()
	_, err := r.Send(ctx, "ghost-agent", gatewayCmd("req-1", "ghost-agent", "list_pods"))

	if err == nil {
		t.Fatal("expected Unavailable error, got nil")
	}
	if got := status.Code(err); got != codes.Unavailable {
		t.Errorf("code = %v, want Unavailable", got)
	}
}

func TestRouter_Send_AgentDisconnected(t *testing.T) {
	ms := newMockStream()
	r, reg := newRouter(ms, "agent-offline")

	// Deregister the agent so it appears offline.
	reg.Deregister("agent-offline")

	ctx := context.Background()
	_, err := r.Send(ctx, "agent-offline", gatewayCmd("req-2", "agent-offline", "list_pods"))

	if err == nil {
		t.Fatal("expected Unavailable error for disconnected agent, got nil")
	}
	if got := status.Code(err); got != codes.Unavailable {
		t.Errorf("code = %v, want Unavailable", got)
	}
}

func TestRouter_Send_StreamSendError(t *testing.T) {
	ms := newMockStream()
	ms.sendErr = status.Error(codes.Internal, "transport closed")

	r, _ := newRouter(ms, "agent-err")

	ctx := context.Background()
	_, err := r.Send(ctx, "agent-err", gatewayCmd("req-3", "agent-err", "list_pods"))

	if err == nil {
		t.Fatal("expected Unavailable error when stream.Send fails, got nil")
	}
	if got := status.Code(err); got != codes.Unavailable {
		t.Errorf("code = %v, want Unavailable", got)
	}
}

func TestRouter_Send_ContextTimeout_BeforeDeliver(t *testing.T) {
	ms := newMockStream()
	r, _ := newRouter(ms, "agent-slow")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	// Do NOT call Deliver — the context should expire first.
	_, err := r.Send(ctx, "agent-slow", gatewayCmd("req-timeout", "agent-slow", "list_pods"))

	if err == nil {
		t.Fatal("expected DeadlineExceeded or Canceled, got nil")
	}
	code := status.Code(err)
	if code != codes.DeadlineExceeded && code != codes.Canceled {
		t.Errorf("code = %v, want DeadlineExceeded or Canceled", code)
	}
}

func TestRouter_Send_SuccessfulRoutingAndDeliver(t *testing.T) {
	ms := newMockStream()
	r, _ := newRouter(ms, "agent-ok")

	cmd := gatewayCmd("req-success", "agent-ok", "list_pods")
	expected := commandResult("req-success", "agent-ok", true)

	// Start Send in a goroutine because it blocks waiting for the result.
	type sendResult struct {
		result *gatewayv1.AgentCommandResult
		err    error
	}
	ch := make(chan sendResult, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		res, err := r.Send(ctx, "agent-ok", cmd)
		ch <- sendResult{res, err}
	}()

	// Wait for the mock stream to receive the forwarded command, then deliver the result.
	if !ms.waitSent(time.Second) {
		t.Fatal("timed out waiting for router to send command to mock stream")
	}

	// Verify the command was forwarded with the correct payload.
	sent := ms.lastSent()
	if sent == nil {
		t.Fatal("no message on mock stream send queue")
	}
	fwdCmd := sent.GetCommand()
	if fwdCmd == nil {
		t.Fatal("forwarded message is not a GatewayCommand")
	}
	if fwdCmd.GetRequestId() != "req-success" {
		t.Errorf("forwarded request_id = %q, want %q", fwdCmd.GetRequestId(), "req-success")
	}

	// Deliver the result to unblock Send.
	r.Deliver(expected)

	result := <-ch
	if result.err != nil {
		t.Fatalf("Send() returned unexpected error: %v", result.err)
	}
	if result.result == nil {
		t.Fatal("Send() returned nil result")
	}
	if result.result.GetRequestId() != expected.GetRequestId() {
		t.Errorf("result request_id = %q, want %q", result.result.GetRequestId(), expected.GetRequestId())
	}
	if !result.result.GetSuccess() {
		t.Error("result.success = false, want true")
	}
}

// --- tests: Deliver ---

func TestRouter_Deliver_NoWaiter_DoesNotPanic(t *testing.T) {
	r, _ := newRouter(nil, "")

	// Delivering when no Send is pending should be a no-op, not a panic.
	r.Deliver(commandResult("orphan-req", "agent-x", true))
}

func TestRouter_Deliver_AfterContextTimeout_DropsResult(t *testing.T) {
	ms := newMockStream()
	r, _ := newRouter(ms, "agent-late")

	cmd := gatewayCmd("req-late", "agent-late", "list_pods")

	// Run Send with a very short timeout so it returns before we deliver.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = r.Send(ctx, "agent-late", cmd)
	}()

	// Wait for Send to finish (timeout path).
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Send did not return after context timeout")
	}

	// Deliver after the waiter has been cleaned up — must not panic.
	r.Deliver(commandResult("req-late", "agent-late", true))
}

func TestRouter_Send_CommandForwardsToCorrectAgent(t *testing.T) {
	// Register two agents; only one should receive the command.
	msA := newMockStream()
	msB := newMockStream()

	reg := registry.New()
	_ = reg.Register("agent-A", msA, nil)
	_ = reg.Register("agent-B", msB, nil)
	r := router.New(reg, slog.Default())

	cmd := gatewayCmd("req-fwd", "agent-A", "get_logs")

	ch := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := r.Send(ctx, "agent-A", cmd)
		ch <- err
	}()

	if !msA.waitSent(time.Second) {
		t.Fatal("timed out waiting for agent-A to receive command")
	}

	// agent-B should not have received anything.
	msB.mu.Lock()
	bCount := len(msB.sendQueue)
	msB.mu.Unlock()
	if bCount != 0 {
		t.Errorf("agent-B received %d messages, want 0", bCount)
	}

	r.Deliver(commandResult("req-fwd", "agent-A", true))

	if err := <-ch; err != nil {
		t.Errorf("Send() unexpected error: %v", err)
	}
}
