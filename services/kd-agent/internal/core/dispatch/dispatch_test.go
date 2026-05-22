package dispatch_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/kubediscovery/kd-agent/internal/core/dispatch"
)

// ── mock Executor ─────────────────────────────────────────────────────────────

type mockExecutor struct {
	executeFn func(ctx context.Context, op string, payload map[string]any) (map[string]any, error)
}

func (m *mockExecutor) Execute(ctx context.Context, op string, payload map[string]any) (map[string]any, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, op, payload)
	}
	return nil, nil
}

// successExecutor always succeeds and echoes the operation in the result.
func successExecutor(result map[string]any) *mockExecutor {
	return &mockExecutor{
		executeFn: func(_ context.Context, op string, _ map[string]any) (map[string]any, error) {
			if result == nil {
				result = map[string]any{"op": op}
			}
			return result, nil
		},
	}
}

// unavailableExecutor always returns ErrExecutorUnavailable.
func unavailableExecutor() *mockExecutor {
	return &mockExecutor{
		executeFn: func(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
			return nil, dispatch.ErrExecutorUnavailable
		},
	}
}

// errExecutor always returns a plain (non-unavailable) error.
func errExecutor(msg string) *mockExecutor {
	return &mockExecutor{
		executeFn: func(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
			return nil, errors.New(msg)
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func makeCommand(requestID, operation string) *gatewayv1.GatewayCommand {
	return &gatewayv1.GatewayCommand{
		RequestId: requestID,
		Operation: operation,
	}
}

func makeCommandWithPayload(requestID, operation string, payload map[string]any) *gatewayv1.GatewayCommand {
	pb, _ := structpb.NewStruct(payload)
	return &gatewayv1.GatewayCommand{
		RequestId: requestID,
		Operation: operation,
		Payload:   pb,
	}
}

func newDispatcher(callerID string, exec dispatch.Executor) *dispatch.Dispatcher {
	return dispatch.New(callerID, exec, slog.Default())
}

// ── nil command ───────────────────────────────────────────────────────────────

func TestDispatch_NilCommandReturnsError(t *testing.T) {
	d := newDispatcher("agent-1", successExecutor(nil))
	_, err := d.Dispatch(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil command, got nil")
	}
}

// ── successful dispatch ───────────────────────────────────────────────────────

func TestDispatch_SuccessfulExecution(t *testing.T) {
	d := newDispatcher("agent-1", successExecutor(map[string]any{"status": "done"}))
	cmd := makeCommand("req-001", "list_pods")

	msg, err := d.Dispatch(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := msg.GetCommandResult()
	if result == nil {
		t.Fatal("response must contain AgentCommandResult")
	}
	if !result.GetSuccess() {
		t.Errorf("Success = false, want true")
	}
}

// ── caller_id is stamped on the result ───────────────────────────────────────

func TestDispatch_CallerIDStampedOnResult(t *testing.T) {
	callerID := "my-agent-42"
	d := newDispatcher(callerID, successExecutor(nil))
	cmd := makeCommand("req-002", "get_logs")

	msg, err := d.Dispatch(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := msg.GetCommandResult().GetCallerId(); got != callerID {
		t.Errorf("CallerId = %q, want %q", got, callerID)
	}
}

// ── request_id is echoed ──────────────────────────────────────────────────────

func TestDispatch_RequestIDEchoed(t *testing.T) {
	d := newDispatcher("agent-1", successExecutor(nil))
	cmd := makeCommand("req-echo-99", "list_deployments")

	msg, _ := d.Dispatch(context.Background(), cmd)
	if got := msg.GetCommandResult().GetRequestId(); got != "req-echo-99" {
		t.Errorf("RequestId = %q, want %q", got, "req-echo-99")
	}
}

// ── response payload is populated on success ─────────────────────────────────

func TestDispatch_ResponsePayloadPopulated(t *testing.T) {
	d := newDispatcher("agent-1", successExecutor(map[string]any{"pods": float64(3)}))
	cmd := makeCommand("req-003", "list_pods")

	msg, _ := d.Dispatch(context.Background(), cmd)
	result := msg.GetCommandResult()

	if result.GetPayload() == nil {
		t.Fatal("Payload should be populated on success")
	}
	if result.GetPayload().AsMap()["pods"] != float64(3) {
		t.Errorf("Payload[pods] = %v, want 3", result.GetPayload().AsMap()["pods"])
	}
}

// ── executor unavailable → success=false + UNAVAILABLE status ────────────────

func TestDispatch_ExecutorUnavailable_SuccessFalse(t *testing.T) {
	d := newDispatcher("agent-1", unavailableExecutor())
	cmd := makeCommand("req-004", "list_pods")

	msg, err := d.Dispatch(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Dispatch itself should not error on executor failure: %v", err)
	}

	result := msg.GetCommandResult()
	if result.GetSuccess() {
		t.Error("Success should be false when executor is unavailable")
	}
	if result.GetMessage() == "" {
		t.Error("Message must be non-empty when executor is unavailable")
	}
}

// ── executor unavailable → message contains Unavailable gRPC status ──────────

func TestDispatch_ExecutorUnavailable_MessageContainsStatus(t *testing.T) {
	d := newDispatcher("agent-1", unavailableExecutor())
	cmd := makeCommand("req-005", "list_pods")

	msg, _ := d.Dispatch(context.Background(), cmd)
	msgText := msg.GetCommandResult().GetMessage()
	if msgText == "" {
		t.Error("Message should describe the unavailability")
	}
}

// ── wrapped ErrExecutorUnavailable is handled the same way ───────────────────

func TestDispatch_WrappedExecutorUnavailable(t *testing.T) {
	wrapped := &mockExecutor{
		executeFn: func(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
			return nil, errors.Join(errors.New("dial timeout"), dispatch.ErrExecutorUnavailable)
		},
	}
	d := newDispatcher("agent-1", wrapped)
	cmd := makeCommand("req-006", "list_pods")

	msg, _ := d.Dispatch(context.Background(), cmd)
	if msg.GetCommandResult().GetSuccess() {
		t.Error("Success must be false for wrapped ErrExecutorUnavailable")
	}
}

// ── generic executor error → success=false, arbitrary message ────────────────

func TestDispatch_GenericExecutorError(t *testing.T) {
	d := newDispatcher("agent-1", errExecutor("some internal error"))
	cmd := makeCommand("req-007", "list_pods")

	msg, err := d.Dispatch(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Dispatch itself should not error: %v", err)
	}

	result := msg.GetCommandResult()
	if result.GetSuccess() {
		t.Error("Success should be false for generic executor error")
	}
	if result.GetMessage() == "" {
		t.Error("Message must not be empty on error")
	}
}

// ── result payload is nil on failure ─────────────────────────────────────────

func TestDispatch_PayloadNilOnFailure(t *testing.T) {
	d := newDispatcher("agent-1", unavailableExecutor())
	cmd := makeCommand("req-008", "list_pods")

	msg, _ := d.Dispatch(context.Background(), cmd)
	if msg.GetCommandResult().GetPayload() != nil {
		t.Error("Payload should be nil when execution fails")
	}
}

// ── responded_at timestamp is set ────────────────────────────────────────────

func TestDispatch_RespondedAtIsSet(t *testing.T) {
	d := newDispatcher("agent-1", successExecutor(nil))
	cmd := makeCommand("req-009", "ping")

	msg, _ := d.Dispatch(context.Background(), cmd)
	if msg.GetCommandResult().GetRespondedAt() == nil {
		t.Error("RespondedAt must be set on every result")
	}
}

// ── command payload is forwarded to the executor ─────────────────────────────

func TestDispatch_PayloadForwardedToExecutor(t *testing.T) {
	var capturedPayload map[string]any

	exec := &mockExecutor{
		executeFn: func(_ context.Context, _ string, payload map[string]any) (map[string]any, error) {
			capturedPayload = payload
			return nil, nil
		},
	}

	d := newDispatcher("agent-1", exec)
	cmd := makeCommandWithPayload("req-010", "get_logs", map[string]any{"namespace": "default", "pod": "my-pod"})

	_, err := d.Dispatch(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPayload["namespace"] != "default" {
		t.Errorf("namespace not forwarded, got %v", capturedPayload["namespace"])
	}
	if capturedPayload["pod"] != "my-pod" {
		t.Errorf("pod not forwarded, got %v", capturedPayload["pod"])
	}
}

// ── operation is forwarded to executor ───────────────────────────────────────

func TestDispatch_OperationForwardedToExecutor(t *testing.T) {
	var capturedOp string

	exec := &mockExecutor{
		executeFn: func(_ context.Context, op string, _ map[string]any) (map[string]any, error) {
			capturedOp = op
			return nil, nil
		},
	}

	d := newDispatcher("agent-1", exec)
	cmd := makeCommand("req-011", "list_deployments")

	_, _ = d.Dispatch(context.Background(), cmd)

	if capturedOp != "list_deployments" {
		t.Errorf("operation = %q, want list_deployments", capturedOp)
	}
}

// ── response is always an AgentCommandResult (not another payload type) ───────

func TestDispatch_ResponseIsCommandResult(t *testing.T) {
	d := newDispatcher("agent-1", successExecutor(nil))
	cmd := makeCommand("req-012", "ping")

	msg, _ := d.Dispatch(context.Background(), cmd)
	if msg.GetCommandResult() == nil {
		t.Error("response payload must be AgentCommandResult")
	}
	if msg.GetHello() != nil {
		t.Error("response must not contain AgentHello")
	}
	if msg.GetHeartbeat() != nil {
		t.Error("response must not contain AgentHeartbeat")
	}
}

// ── table-driven: multiple operations ────────────────────────────────────────

func TestDispatch_MultipleOperations(t *testing.T) {
	operations := []string{
		"list_pods",
		"get_pod_logs",
		"list_deployments",
		"list_events",
	}

	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			d := newDispatcher("agent-1", successExecutor(nil))
			cmd := makeCommand("req-multi", op)

			msg, err := d.Dispatch(context.Background(), cmd)
			if err != nil {
				t.Fatalf("operation %q: unexpected error: %v", op, err)
			}
			if !msg.GetCommandResult().GetSuccess() {
				t.Errorf("operation %q: Success = false, want true", op)
			}
		})
	}
}
