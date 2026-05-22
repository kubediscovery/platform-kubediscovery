package stream_test

import (
	"errors"
	"testing"

	"github.com/kubediscovery/kd-agent/internal/core/stream"
)

// ── ErrEmptyCallerID ─────────────────────────────────────────────────────────

func TestNewHelloMessage_EmptyCallerIDErrors(t *testing.T) {
	_, err := stream.NewHelloMessage("", nil)
	if err == nil {
		t.Fatal("expected error for empty caller_id, got nil")
	}
	if !errors.Is(err, stream.ErrEmptyCallerID) {
		t.Errorf("expected ErrEmptyCallerID, got %v", err)
	}
}

// ── caller_id is propagated correctly ────────────────────────────────────────

func TestNewHelloMessage_CallerIDPopulated(t *testing.T) {
	msg, err := stream.NewHelloMessage("my-agent-01", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hello := msg.GetHello()
	if hello == nil {
		t.Fatal("AgentHello must not be nil")
	}
	if got := hello.GetCallerId(); got != "my-agent-01" {
		t.Errorf("CallerId = %q, want %q", got, "my-agent-01")
	}
}

// ── Message carries an AgentHello payload (not another variant) ──────────────

func TestNewHelloMessage_PayloadIsAgentHello(t *testing.T) {
	msg, err := stream.NewHelloMessage("agent-x", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.GetHello() == nil {
		t.Error("payload must be AgentHello; got a different oneof variant")
	}
	if msg.GetHeartbeat() != nil {
		t.Error("payload must be AgentHello, not AgentHeartbeat")
	}
	if msg.GetCommandResult() != nil {
		t.Error("payload must be AgentHello, not AgentCommandResult")
	}
}

// ── SentAt timestamp is set ───────────────────────────────────────────────────

func TestNewHelloMessage_SentAtIsSet(t *testing.T) {
	msg, err := stream.NewHelloMessage("agent-ts", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.GetHello().GetSentAt() == nil {
		t.Error("SentAt must be populated in AgentHello")
	}
}

// ── metadata is serialised into the Struct field ─────────────────────────────

func TestNewHelloMessage_MetadataPopulated(t *testing.T) {
	md := map[string]any{"kind": "agent->server", "version": "v1.0"}
	msg, err := stream.NewHelloMessage("agent-md", md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := msg.GetHello().GetMetadata().AsMap()
	if got["kind"] != "agent->server" {
		t.Errorf("metadata[kind] = %v, want agent->server", got["kind"])
	}
	if got["version"] != "v1.0" {
		t.Errorf("metadata[version] = %v, want v1.0", got["version"])
	}
}

// ── nil metadata does not cause a panic ───────────────────────────────────────

func TestNewHelloMessage_NilMetadataOK(t *testing.T) {
	msg, err := stream.NewHelloMessage("agent-nil-md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Metadata may be nil or an empty struct; both are acceptable.
	_ = msg.GetHello().GetMetadata()
}

// ── DefaultMetadata includes the required "kind" key ─────────────────────────

func TestDefaultMetadata_ContainsKind(t *testing.T) {
	md := stream.DefaultMetadata()
	if md["kind"] != "agent->server" {
		t.Errorf("DefaultMetadata[kind] = %v, want agent->server", md["kind"])
	}
}

// ── DefaultMetadata integration: hello built with DefaultMetadata ─────────────

func TestNewHelloMessage_WithDefaultMetadata(t *testing.T) {
	msg, err := stream.NewHelloMessage("agent-default-md", stream.DefaultMetadata())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kind := msg.GetHello().GetMetadata().AsMap()["kind"]
	if kind != "agent->server" {
		t.Errorf("metadata[kind] = %v, want agent->server", kind)
	}
}

// ── Table-driven: variety of valid caller_id values ──────────────────────────

func TestNewHelloMessage_ValidCallerIDs(t *testing.T) {
	cases := []string{
		"kd-agent",
		"kd-agent-prod-us-east-1",
		"agent-001",
		"a",
		"very-long-agent-identifier-that-is-still-valid-12345678",
	}

	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			msg, err := stream.NewHelloMessage(id, nil)
			if err != nil {
				t.Fatalf("unexpected error for caller_id %q: %v", id, err)
			}
			if msg.GetHello().GetCallerId() != id {
				t.Errorf("CallerId = %q, want %q", msg.GetHello().GetCallerId(), id)
			}
		})
	}
}
