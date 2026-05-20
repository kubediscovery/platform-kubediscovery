package stream_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	executorv1 "github.com/kubediscovery/kd-libs/core/v1/executor"
	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"

	"github.com/kubediscovery/kd-agent/internal/executor"
	"github.com/kubediscovery/kd-agent/internal/stream"
)

// stubExecutor is a test double for executor.Client.
type stubExecutor struct {
	resp *executorv1.ExecutorResponse
	err  error
}

func (s *stubExecutor) Execute(_ context.Context, _ *executorv1.ExecutorCommand) (*executorv1.ExecutorResponse, error) {
	return s.resp, s.err
}

func newHandler(execClient executor.Client) *stream.Handler {
	return stream.New("test-agent", execClient, slog.Default())
}

func gatewayCmd(requestID, operation string) *gatewayv1.GatewayCommand {
	return &gatewayv1.GatewayCommand{
		RequestId: requestID,
		Operation: operation,
	}
}

// --- UNAVAILABLE scenarios ---

func TestHandleCommand_ExecutorNil_ReturnsUnavailable(t *testing.T) {
	h := stream.New("test-agent", nil, slog.Default())

	result := h.HandleCommand(context.Background(), gatewayCmd("req-1", "list pods"))

	if result.GetSuccess() {
		t.Fatal("expected success=false when executor client is nil")
	}
	if result.GetMessage() != "executor not available" {
		t.Fatalf("expected message %q, got %q", "executor not available", result.GetMessage())
	}
	if result.GetRequestId() != "req-1" {
		t.Fatalf("expected request_id %q, got %q", "req-1", result.GetRequestId())
	}
	if result.GetCallerId() != "test-agent" {
		t.Fatalf("expected caller_id %q, got %q", "test-agent", result.GetCallerId())
	}
}

func TestHandleCommand_ExecutorUnavailable_ReturnsUnavailable(t *testing.T) {
	h := newHandler(&stubExecutor{err: executor.ErrUnavailable})

	result := h.HandleCommand(context.Background(), gatewayCmd("req-2", "get logs"))

	if result.GetSuccess() {
		t.Fatal("expected success=false when executor returns ErrUnavailable")
	}
	if result.GetMessage() != "executor not available" {
		t.Fatalf("expected message %q, got %q", "executor not available", result.GetMessage())
	}
	if result.GetRequestId() != "req-2" {
		t.Fatalf("expected request_id %q, got %q", "req-2", result.GetRequestId())
	}
}

func TestHandleCommand_WrappedUnavailable_ReturnsUnavailable(t *testing.T) {
	wrappedErr := fmt.Errorf("transport: %w", executor.ErrUnavailable)
	h := newHandler(&stubExecutor{err: wrappedErr})

	result := h.HandleCommand(context.Background(), gatewayCmd("req-3", "scale"))

	if result.GetSuccess() {
		t.Fatal("expected success=false for wrapped ErrUnavailable")
	}
	if result.GetMessage() != "executor not available" {
		t.Fatalf("expected message %q, got %q", "executor not available", result.GetMessage())
	}
}

// --- Other error scenarios ---

func TestHandleCommand_ExecutorOtherError_ReturnsFailure(t *testing.T) {
	otherErr := errors.New("internal executor error")
	h := newHandler(&stubExecutor{err: otherErr})

	result := h.HandleCommand(context.Background(), gatewayCmd("req-4", "restart"))

	if result.GetSuccess() {
		t.Fatal("expected success=false for executor error")
	}
	if result.GetMessage() == "executor not available" {
		t.Fatal("non-unavailable error should not produce 'executor not available' message")
	}
	if result.GetMessage() == "" {
		t.Fatal("expected non-empty error message")
	}
}

// --- Success scenario ---

func TestHandleCommand_ExecutorSuccess_ReturnsSuccess(t *testing.T) {
	h := newHandler(&stubExecutor{
		resp: &executorv1.ExecutorResponse{
			RequestId: "req-5",
			Success:   true,
			Output:    "pod/nginx deleted",
		},
	})

	result := h.HandleCommand(context.Background(), gatewayCmd("req-5", "delete_pod"))

	if !result.GetSuccess() {
		t.Fatalf("expected success=true, got message: %q", result.GetMessage())
	}
	if result.GetMessage() != "pod/nginx deleted" {
		t.Fatalf("expected output %q, got %q", "pod/nginx deleted", result.GetMessage())
	}
}

func TestHandleCommand_ExecutorFailure_ReturnsFailureMessage(t *testing.T) {
	h := newHandler(&stubExecutor{
		resp: &executorv1.ExecutorResponse{
			RequestId: "req-6",
			Success:   false,
			Error:     "namespace \"staging\" not found",
		},
	})

	result := h.HandleCommand(context.Background(), gatewayCmd("req-6", "list_pods"))

	if result.GetSuccess() {
		t.Fatal("expected success=false for executor failure response")
	}
	if result.GetMessage() != `namespace "staging" not found` {
		t.Fatalf("expected error message %q, got %q", `namespace "staging" not found`, result.GetMessage())
	}
}

// --- Result metadata ---

func TestHandleCommand_ResultAlwaysContainsCallerID(t *testing.T) {
	cases := []struct {
		name        string
		execClient  executor.Client
		expectMsg   string
	}{
		{
			name:       "nil executor",
			execClient: nil,
			expectMsg:  "executor not available",
		},
		{
			name:       "unavailable executor",
			execClient: &stubExecutor{err: executor.ErrUnavailable},
			expectMsg:  "executor not available",
		},
		{
			name: "successful executor",
			execClient: &stubExecutor{
				resp: &executorv1.ExecutorResponse{Success: true, Output: "ok"},
			},
			expectMsg: "ok",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := stream.New("my-agent-id", tc.execClient, slog.Default())
			result := h.HandleCommand(context.Background(), gatewayCmd("r", "op"))

			if result.GetCallerId() != "my-agent-id" {
				t.Errorf("expected caller_id %q, got %q", "my-agent-id", result.GetCallerId())
			}
			if result.GetRespondedAt() == nil {
				t.Error("responded_at must not be nil")
			}
			if result.GetMessage() != tc.expectMsg {
				t.Errorf("expected message %q, got %q", tc.expectMsg, result.GetMessage())
			}
		})
	}
}
