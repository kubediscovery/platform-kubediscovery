// Package dto defines HTTP response shapes for the agent API.
package dto

// AgentListResponse is the JSON body for GET /api/v1/agents.
type AgentListResponse struct {
	Agents []AgentItem `json:"agents"`
}

// AgentItem is a single agent entry in the list response.
type AgentItem struct {
	CallerID     string `json:"caller_id"`
	Status       string `json:"status"`
	Environment  string `json:"environment"`
	LastActivity string `json:"last_activity"`
}
