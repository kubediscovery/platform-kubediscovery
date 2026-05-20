package executor_test

import (
	"context"
	"errors"
	"testing"

	executorv1 "github.com/kubediscovery/kd-libs/core/v1/executor"

	"github.com/kubediscovery/kd-agent/internal/executor"
)

// TestGRPCClient_Execute_UnreachableAddress verifies that dialling an address
// where nothing is listening produces ErrUnavailable.
func TestGRPCClient_Execute_UnreachableAddress(t *testing.T) {
	// Port 1 is reserved and should always refuse connections.
	client := executor.NewGRPCClient("localhost:1")

	_, err := client.Execute(context.Background(), &executorv1.ExecutorCommand{
		RequestId:  "test-req",
		ActionType: "list_pods",
	})

	if err == nil {
		t.Fatal("expected error when executor address is unreachable, got nil")
	}
	if !errors.Is(err, executor.ErrUnavailable) {
		t.Fatalf("expected errors.Is(err, ErrUnavailable)=true, got: %v", err)
	}
}

// TestErrUnavailable_Wrapping verifies that ErrUnavailable can be wrapped and
// unwrapped correctly with errors.Is, which is the pattern used by the stream
// handler to detect the unavailable condition.
func TestErrUnavailable_Wrapping(t *testing.T) {
	wrapped := errors.Join(errors.New("connection refused"), executor.ErrUnavailable)
	if !errors.Is(wrapped, executor.ErrUnavailable) {
		t.Error("errors.Is must detect ErrUnavailable through errors.Join wrapping")
	}
}
