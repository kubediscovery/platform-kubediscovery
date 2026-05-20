package handler_test

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

	"github.com/kubediscovery/kd-gateway/configs"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/entity"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/handler"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// eofMockStream is a minimal bidirectional server-side stream that returns
// io.EOF once all pre-loaded messages have been consumed.  Suitable for tests
// that only care about a finite sequence of frames.
type eofMockStream struct {
	grpc.ServerStream

	ctx       context.Context
	recvQueue []*gatewayv1.AgentStreamMessage
	recvIdx   int
	mu        sync.Mutex
	sendQueue []*gatewayv1.AgentStreamMessage
}

func newMockStream(msgs ...*gatewayv1.AgentStreamMessage) *eofMockStream {
	return &eofMockStream{
		ctx:       context.Background(),
		recvQueue: msgs,
	}
}

func (m *eofMockStream) Context() context.Context { return m.ctx }

func (m *eofMockStream) Recv() (*gatewayv1.AgentStreamMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recvIdx >= len(m.recvQueue) {
		return nil, io.EOF
	}
	msg := m.recvQueue[m.recvIdx]
	m.recvIdx++
	return msg, nil
}

func (m *eofMockStream) Send(msg *gatewayv1.AgentStreamMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendQueue = append(m.sendQueue, msg)
	return nil
}

func (m *eofMockStream) SetHeader(metadata.MD) error  { return nil }
func (m *eofMockStream) SendHeader(metadata.MD) error { return nil }
func (m *eofMockStream) SetTrailer(metadata.MD)       {}
func (m *eofMockStream) RecvMsg(any) error            { return nil }
func (m *eofMockStream) SendMsg(any) error            { return nil }

// blockingMockStream delivers pre-loaded messages then blocks in Recv() until
// its context is cancelled.  Used for eviction tests where the old stream must
// remain "live" while the new connection arrives.
type blockingMockStream struct {
	grpc.ServerStream

	ctx       context.Context
	cancel    context.CancelFunc
	recvQueue []*gatewayv1.AgentStreamMessage
	mu        sync.Mutex
	recvIdx   int
	sendQueue []*gatewayv1.AgentStreamMessage
}

func newBlockingMockStream(msgs ...*gatewayv1.AgentStreamMessage) *blockingMockStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &blockingMockStream{
		ctx:       ctx,
		cancel:    cancel,
		recvQueue: msgs,
	}
}

func (m *blockingMockStream) Context() context.Context { return m.ctx }

func (m *blockingMockStream) Recv() (*gatewayv1.AgentStreamMessage, error) {
	m.mu.Lock()
	if m.recvIdx < len(m.recvQueue) {
		msg := m.recvQueue[m.recvIdx]
		m.recvIdx++
		m.mu.Unlock()
		return msg, nil
	}
	m.mu.Unlock()
	// Block until context is cancelled (simulates a live stream with no pending messages).
	<-m.ctx.Done()
	return nil, io.EOF
}

func (m *blockingMockStream) Send(msg *gatewayv1.AgentStreamMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendQueue = append(m.sendQueue, msg)
	return nil
}

func (m *blockingMockStream) SetHeader(metadata.MD) error  { return nil }
func (m *blockingMockStream) SendHeader(metadata.MD) error { return nil }
func (m *blockingMockStream) SetTrailer(metadata.MD)       {}
func (m *blockingMockStream) RecvMsg(any) error            { return nil }
func (m *blockingMockStream) SendMsg(any) error            { return nil }

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

func defaultCfg() *configs.Config {
	return &configs.Config{
		Agent: configs.AgentConfig{
			DuplicatePolicy: configs.DuplicatePolicyRejectNew,
		},
	}
}

func evictCfg() *configs.Config {
	return &configs.Config{
		Agent: configs.AgentConfig{
			DuplicatePolicy: configs.DuplicatePolicyEvictPrevious,
		},
	}
}

func newHandler() (*handler.Handler, *registry.Registry) {
	reg := registry.New()
	h := handler.New(reg, slog.Default(), nil, defaultCfg())
	return h, reg
}

func newHandlerWithCfg(cfg *configs.Config) (*handler.Handler, *registry.Registry) {
	reg := registry.New()
	h := handler.New(reg, slog.Default(), nil, cfg)
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

func TestAgentStream_RejectNew_DuplicateCallerIDRejected(t *testing.T) {
	h, reg := newHandler() // default: reject_new
	// Pre-register agent-1 so it appears already connected.
	if _, err := reg.Register("agent-1", nil, nil); err != nil {
		t.Fatalf("pre-register: %v", err)
	}

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

// captureResultSink records every Deliver call for test assertions.
type captureResultSink struct {
	mu      sync.Mutex
	results []*gatewayv1.AgentCommandResult
}

func (s *captureResultSink) Deliver(r *gatewayv1.AgentCommandResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = append(s.results, r)
}

func (s *captureResultSink) all() []*gatewayv1.AgentCommandResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*gatewayv1.AgentCommandResult, len(s.results))
	copy(out, s.results)
	return out
}

func TestAgentStream_CommandResult_DelegatesToResultSink(t *testing.T) {
	reg := registry.New()
	sink := &captureResultSink{}
	h := handler.New(reg, slog.Default(), sink, defaultCfg())

	cmdResult := &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_CommandResult{
			CommandResult: &gatewayv1.AgentCommandResult{
				RequestId: "req-sink-test",
				CallerId:  "agent-sink",
				Success:   true,
				Message:   "delivered",
			},
		},
	}
	stream := newMockStream(helloMsg("agent-sink"), cmdResult)

	err := h.AgentStream(stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := sink.all()
	if len(got) != 1 {
		t.Fatalf("ResultSink.Deliver called %d times, want 1", len(got))
	}
	if got[0].GetRequestId() != "req-sink-test" {
		t.Errorf("delivered request_id = %q, want %q", got[0].GetRequestId(), "req-sink-test")
	}
}

// --- Eviction policy tests ---

// TestAgentStream_EvictPrevious_AcceptsNewAndEvictsOld verifies that when
// duplicate_policy=evict_previous, the new connection succeeds and the old
// stream handler receives codes.Aborted.
func TestAgentStream_EvictPrevious_AcceptsNewAndEvictsOld(t *testing.T) {
	h, reg := newHandlerWithCfg(evictCfg())

	// Old stream: hello then blocks waiting for messages (simulates a live agent).
	oldStream := newBlockingMockStream(helloMsg("agent-dup"))

	oldErrCh := make(chan error, 1)
	go func() {
		oldErrCh <- h.AgentStream(oldStream)
	}()

	// Wait until old agent is registered.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if a, ok := reg.Get("agent-dup"); ok && a.Status == entity.StatusConnected {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	a, ok := reg.Get("agent-dup")
	if !ok || a.Status != entity.StatusConnected {
		t.Fatal("old agent should be registered and connected before new connection arrives")
	}

	// New stream: hello then EOF — should succeed under evict_previous policy.
	newStream := newMockStream(helloMsg("agent-dup"))
	newErr := h.AgentStream(newStream)
	if newErr != nil {
		t.Fatalf("new stream AgentStream returned error: %v", newErr)
	}

	// Old stream handler must have returned codes.Aborted.
	select {
	case err := <-oldErrCh:
		if status.Code(err) != codes.Aborted {
			t.Errorf("old stream error code = %v, want Aborted", status.Code(err))
		}
	case <-time.After(500 * time.Millisecond):
		oldStream.cancel()
		t.Error("old stream handler did not return within timeout")
	}
}

// TestAgentStream_EvictPrevious_NewAgentRemainsConnectedAfterOldExits verifies
// that the new agent's registry entry is not clobbered when the evicted stream
// goroutine completes (it must skip Deregister when evicted=true).
func TestAgentStream_EvictPrevious_NewAgentRemainsConnectedAfterOldExits(t *testing.T) {
	h, reg := newHandlerWithCfg(evictCfg())

	oldStream := newBlockingMockStream(helloMsg("agent-dup2"))
	oldErrCh := make(chan error, 1)
	go func() { oldErrCh <- h.AgentStream(oldStream) }()

	// Wait for old agent to connect.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if a, ok := reg.Get("agent-dup2"); ok && a.Status == entity.StatusConnected {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	// New connection: evicts old, registers itself, then stays open briefly.
	newStream := newBlockingMockStream(helloMsg("agent-dup2"))
	newErrCh := make(chan error, 1)
	go func() { newErrCh <- h.AgentStream(newStream) }()

	// Wait for old handler to be evicted.
	select {
	case err := <-oldErrCh:
		if status.Code(err) != codes.Aborted {
			t.Errorf("old stream error code = %v, want Aborted", status.Code(err))
		}
	case <-time.After(500 * time.Millisecond):
		oldStream.cancel()
		t.Fatal("old stream handler did not return in time")
	}

	// Small pause to let goroutine cleanup settle.
	time.Sleep(10 * time.Millisecond)

	// The new agent must still be connected (evicted goroutine must not have
	// called Deregister on the new registration).
	a, ok := reg.Get("agent-dup2")
	if !ok {
		t.Fatal("agent entry should exist after eviction")
	}
	if a.Status != entity.StatusConnected {
		t.Errorf("new agent Status = %q after old handler exits, want StatusConnected", a.Status)
	}

	// Clean up new stream.
	newStream.cancel()
	select {
	case <-newErrCh:
	case <-time.After(200 * time.Millisecond):
		t.Error("new stream handler did not return after context cancel")
	}
}

// TestAgentStream_RejectNew_IsDefault verifies that an unknown/empty policy
// falls back to reject_new behaviour.
func TestAgentStream_RejectNew_IsDefault(t *testing.T) {
	// Use a config with an empty (unrecognised) policy — should behave as reject_new.
	cfg := &configs.Config{
		Agent: configs.AgentConfig{DuplicatePolicy: ""},
	}
	h, reg := newHandlerWithCfg(cfg)

	if _, err := reg.Register("agent-def", nil, nil); err != nil {
		t.Fatalf("pre-register: %v", err)
	}

	stream := newMockStream(helloMsg("agent-def"))
	err := h.AgentStream(stream)
	if status.Code(err) != codes.AlreadyExists {
		t.Errorf("default policy: code = %v, want AlreadyExists", status.Code(err))
	}
}
