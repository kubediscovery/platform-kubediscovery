package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/dto"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/handler"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/service"
)

func TestHTTPHandler_ListAgents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reg := registry.New()
	lastSeen := time.Date(2026, 5, 22, 15, 30, 0, 0, time.UTC)
	if _, err := reg.Register("srv-001", nil, map[string]any{"environment": "staging"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if a, ok := reg.Get("srv-001"); ok {
		a.LastSeenAt = lastSeen
	}

	h := handler.NewHTTP(service.New(reg))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	c.Request = req

	h.ListAgents(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp dto.AgentListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(resp.Agents))
	}

	agent := resp.Agents[0]
	if agent.CallerID != "srv-001" {
		t.Errorf("caller_id = %q, want srv-001", agent.CallerID)
	}
	if agent.Status != "connected" {
		t.Errorf("status = %q, want connected", agent.Status)
	}
	if agent.Environment != "staging" {
		t.Errorf("environment = %q, want staging", agent.Environment)
	}
	if agent.LastActivity != lastSeen.UTC().Format(time.RFC3339) {
		t.Errorf("last_activity = %q, want %s", agent.LastActivity, lastSeen.UTC().Format(time.RFC3339))
	}
}

func TestHTTPHandler_ListAgents_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := handler.NewHTTP(service.New(registry.New()))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)

	h.ListAgents(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp dto.AgentListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Agents == nil {
		t.Fatal("expected non-nil agents slice")
	}
	if len(resp.Agents) != 0 {
		t.Fatalf("expected empty agents, got %d", len(resp.Agents))
	}
}
