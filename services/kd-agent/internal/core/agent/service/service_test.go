package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/grpc/metadata"

	"github.com/kubediscovery/kd-agent/internal/core/agent/executor"
	"github.com/kubediscovery/kd-agent/internal/core/agent/service"
)

// --- mock stream -----------------------------------------------------------

// mockStream is a minimal fake of the client-side bidirectional stream.
// Messages placed in recvQueue are returned sequentially by Recv; anything
// passed to Send is stored in SentMsgs.
//
// Note: Send is only called from the single sender goroutine inside recvLoop,
// so no mutex is needed when the service cleanly waits for goroutines before
// returning (which it does).
type mockStream struct {
	recvQueue []*gatewayv1.AgentStreamMessage
	recvIdx   int
	SentMsgs  []*gatewayv1.AgentStreamMessage
	sendErr   error
	ctx       context.Context
}

func newMockStream(msgs ...*gatewayv1.AgentStreamMessage) *mockStream {
	return &mockStream{
		recvQueue: msgs,
		ctx:       context.Background(),
	}
}

func (m *mockStream) Send(msg *gatewayv1.AgentStreamMessage) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.SentMsgs = append(m.SentMsgs, msg)
	return nil
}

func (m *mockStream) Recv() (*gatewayv1.AgentStreamMessage, error) {
	if m.recvIdx >= len(m.recvQueue) {
		return nil, io.EOF
	}
	msg := m.recvQueue[m.recvIdx]
	m.recvIdx++
	return msg, nil
}

func (m *mockStream) CloseSend() error             { return nil }
func (m *mockStream) Context() context.Context     { return m.ctx }
func (m *mockStream) Header() (metadata.MD, error) { return nil, nil }
func (m *mockStream) Trailer() metadata.MD         { return nil }
func (m *mockStream) RecvMsg(v any) error          { return nil }
func (m *mockStream) SendMsg(v any) error          { return nil }

// --- mock opener -----------------------------------------------------------

// singleStreamOpener returns the same pre-built stream on each call.
type singleStreamOpener struct {
	stream *mockStream
	err    error
}

func (o *singleStreamOpener) OpenStream(_ context.Context) (gatewayv1.GatewayService_AgentStreamClient, error) {
	return o.stream, o.err
}

// countingOpener counts how many times OpenStream is called and always fails.
type countingOpener struct {
	calls int
	err   error
}

func (o *countingOpener) OpenStream(_ context.Context) (gatewayv1.GatewayService_AgentStreamClient, error) {
	o.calls++
	return nil, o.err
}

// --- mock executor ---------------------------------------------------------

type mockExecutor struct {
	result *gatewayv1.AgentCommandResult
	err    error
}

func (e *mockExecutor) Dispatch(_ context.Context, cmd *gatewayv1.GatewayCommand) (*gatewayv1.AgentCommandResult, error) {
	if e.err != nil {
		return nil, e.err
	}
	return &gatewayv1.AgentCommandResult{
		RequestId: cmd.GetRequestId(),
		Success:   true,
		Message:   "ok",
	}, nil
}

// --- helpers ---------------------------------------------------------------

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// noRetryConfig returns a RetryConfig that opens the stream exactly once and
// never delays. Suitable for tests that focus on stream behaviour, not retries.
func noRetryConfig() service.RetryConfig {
	return service.RetryConfig{BaseDelay: 0, Multiplier: 1, MaxAttempts: 1}
}

func newService(t *testing.T, agentID string, retry service.RetryConfig, opener service.StreamOpener, exec executor.Dispatcher) *service.Service {
	t.Helper()
	svc, err := service.New(agentID, retry, opener, exec, noopLogger())
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	return svc
}

// ==========================================================================
// RetryConfig.DelayForAttempt tests
// ==========================================================================

func TestRetryConfig_DelayForAttempt(t *testing.T) {
	cfg := service.RetryConfig{BaseDelay: 1 * time.Second, Multiplier: 3, MaxAttempts: 5}

	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 0},
		{1, 1 * time.Second},
		{2, 3 * time.Second},
		{3, 9 * time.Second},
		{4, 27 * time.Second},
		{5, 81 * time.Second},
	}
	for _, tc := range cases {
		got := cfg.DelayForAttempt(tc.attempt)
		if got != tc.want {
			t.Errorf("DelayForAttempt(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

// ==========================================================================
// Retry loop tests
// ==========================================================================

func TestRun_ReturnsErrMaxRetriesExceeded_WhenOpenerAlwaysFails(t *testing.T) {
	opener := &countingOpener{err: errors.New("dial error")}
	// BaseDelay=0 so all delays are 0 and the test completes instantly.
	retry := service.RetryConfig{BaseDelay: 0, Multiplier: 1, MaxAttempts: 3}
	svc := newService(t, "agent-retry", retry, opener, executor.UnavailableDispatcher{})

	runErr := svc.Run(context.Background())

	if !errors.Is(runErr, service.ErrMaxRetriesExceeded) {
		t.Errorf("Run error = %v, want ErrMaxRetriesExceeded", runErr)
	}
	if opener.calls != 3 {
		t.Errorf("OpenStream called %d times, want 3", opener.calls)
	}
}

func TestRun_ReturnsContextError_WhenContextCancelled(t *testing.T) {
	opener := &countingOpener{err: errors.New("dial error")}
	// Short delay so context cancellation fires before MaxAttempts is reached.
	retry := service.RetryConfig{BaseDelay: 100 * time.Millisecond, Multiplier: 1, MaxAttempts: 100}
	svc := newService(t, "agent-ctx", retry, opener, executor.UnavailableDispatcher{})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	runErr := svc.Run(ctx)
	if !errors.Is(runErr, context.Canceled) {
		t.Errorf("Run error = %v, want context.Canceled", runErr)
	}
}

// ==========================================================================
// AgentID / caller_id validation tests
// ==========================================================================

func TestNew_ReturnsErrEmptyAgentID_WhenAgentIDIsBlank(t *testing.T) {
	_, err := service.New("", service.DefaultRetryConfig(), &singleStreamOpener{}, executor.UnavailableDispatcher{}, noopLogger())
	if !errors.Is(err, service.ErrEmptyAgentID) {
		t.Errorf("New error = %v, want ErrEmptyAgentID", err)
	}
}

func TestNew_Succeeds_WhenAgentIDIsNonEmpty(t *testing.T) {
	svc, err := service.New("my-agent", service.DefaultRetryConfig(), &singleStreamOpener{}, executor.UnavailableDispatcher{}, noopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.AgentID() != "my-agent" {
		t.Errorf("AgentID = %q, want %q", svc.AgentID(), "my-agent")
	}
}

func TestRun_SendsHelloWithCallerID(t *testing.T) {
	// Stream returns EOF immediately after hello is sent (empty recv queue).
	stream := newMockStream()
	opener := &singleStreamOpener{stream: stream}
	svc := newService(t, "my-agent-id", noRetryConfig(), opener, executor.UnavailableDispatcher{})

	_ = svc.Run(context.Background())

	// recvLoop waits for the sender goroutine before returning, so SentMsgs
	// is fully populated when Run returns — no sleep or extra sync needed.
	if len(stream.SentMsgs) == 0 {
		t.Fatal("no messages sent on stream")
	}
	firstMsg := stream.SentMsgs[0]
	hello := firstMsg.GetHello()
	if hello == nil {
		t.Fatal("first sent message is not AgentHello")
	}
	if hello.GetCallerId() != "my-agent-id" {
		t.Errorf("caller_id = %q, want %q", hello.GetCallerId(), "my-agent-id")
	}
	if hello.GetSentAt() == nil {
		t.Error("sent_at must not be nil in AgentHello")
	}
}

// ==========================================================================
// Command dispatch tests
// ==========================================================================

func TestRun_DispatchesCommandToExecutor(t *testing.T) {
	cmdMsg := &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Command{
			Command: &gatewayv1.GatewayCommand{
				RequestId: "req-001",
				Operation: "list_pods",
			},
		},
	}

	stream := newMockStream(cmdMsg)
	opener := &singleStreamOpener{stream: stream}
	exec := &mockExecutor{}
	svc := newService(t, "dispatch-agent", noRetryConfig(), opener, exec)

	// Run completes only after the sender goroutine has drained sendCh and
	// the sender WaitGroup signals done. SentMsgs is safe to read here.
	_ = svc.Run(context.Background())

	var resultMsg *gatewayv1.AgentStreamMessage
	for _, m := range stream.SentMsgs {
		if m.GetCommandResult() != nil {
			resultMsg = m
			break
		}
	}
	if resultMsg == nil {
		t.Fatal("no AgentCommandResult was sent back")
	}
	result := resultMsg.GetCommandResult()
	if result.GetRequestId() != "req-001" {
		t.Errorf("request_id = %q, want %q", result.GetRequestId(), "req-001")
	}
	if !result.GetSuccess() {
		t.Errorf("expected success=true, got false (message: %s)", result.GetMessage())
	}
	if result.GetCallerId() != "dispatch-agent" {
		t.Errorf("caller_id = %q, want %q", result.GetCallerId(), "dispatch-agent")
	}
}

func TestRun_ReportsUnavailable_WhenExecutorIsUnavailable(t *testing.T) {
	cmdMsg := &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Command{
			Command: &gatewayv1.GatewayCommand{
				RequestId: "req-002",
				Operation: "list_pods",
			},
		},
	}

	stream := newMockStream(cmdMsg)
	opener := &singleStreamOpener{stream: stream}
	svc := newService(t, "unavail-agent", noRetryConfig(), opener, executor.UnavailableDispatcher{})

	_ = svc.Run(context.Background())

	var resultMsg *gatewayv1.AgentStreamMessage
	for _, m := range stream.SentMsgs {
		if m.GetCommandResult() != nil {
			resultMsg = m
			break
		}
	}
	if resultMsg == nil {
		t.Fatal("no AgentCommandResult was sent back")
	}
	result := resultMsg.GetCommandResult()
	if result.GetSuccess() {
		t.Error("expected success=false when executor is unavailable")
	}
	if result.GetMessage() != "kd-executor unavailable" {
		t.Errorf("message = %q, want %q", result.GetMessage(), "kd-executor unavailable")
	}
}

func TestRun_ReportsError_WhenExecutorFails(t *testing.T) {
	cmdMsg := &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Command{
			Command: &gatewayv1.GatewayCommand{
				RequestId: "req-003",
				Operation: "list_pods",
			},
		},
	}

	stream := newMockStream(cmdMsg)
	opener := &singleStreamOpener{stream: stream}
	exec := &mockExecutor{err: errors.New("connection refused")}
	svc := newService(t, "fail-agent", noRetryConfig(), opener, exec)

	_ = svc.Run(context.Background())

	var resultMsg *gatewayv1.AgentStreamMessage
	for _, m := range stream.SentMsgs {
		if m.GetCommandResult() != nil {
			resultMsg = m
			break
		}
	}
	if resultMsg == nil {
		t.Fatal("no AgentCommandResult was sent back")
	}
	if resultMsg.GetCommandResult().GetSuccess() {
		t.Error("expected success=false when executor returns error")
	}
}
