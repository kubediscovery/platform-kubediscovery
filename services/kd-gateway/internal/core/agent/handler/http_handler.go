// Package handler implements HTTP handlers for the agent domain.
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/dto"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/service"
)

// HTTPHandler exposes REST endpoints for agent registry inspection.
type HTTPHandler struct {
	svc *service.Service
}

// NewHTTP constructs an HTTPHandler backed by the agent list service.
func NewHTTP(svc *service.Service) *HTTPHandler {
	return &HTTPHandler{svc: svc}
}

// ListAgents handles GET /api/v1/agents.
func (h *HTTPHandler) ListAgents(c *gin.Context) {
	summaries := h.svc.List()
	items := make([]dto.AgentItem, 0, len(summaries))
	for _, s := range summaries {
		items = append(items, dto.AgentItem{
			CallerID:     s.CallerID,
			Status:       string(s.Status),
			Environment:  s.Environment,
			LastActivity: s.LastActivity.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, dto.AgentListResponse{Agents: items})
}
