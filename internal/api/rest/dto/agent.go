package dto

import (
	"time"

	"github.com/filanov/maestro/internal/models"
)

type AgentResponse struct {
	ID            string    `json:"id"`
	ClusterID     string    `json:"cluster_id"`
	Hostname      string    `json:"hostname"`
	Status        string    `json:"status"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	RegisteredAt  time.Time `json:"registered_at"`
	LastResetAt   time.Time `json:"last_reset_at"`
}

func AgentToResponse(agent *models.Agent) AgentResponse {
	return AgentResponse{
		ID:            agent.ID,
		ClusterID:     agent.ClusterID,
		Hostname:      agent.Hostname,
		Status:        string(agent.Status),
		LastHeartbeat: agent.LastHeartbeat,
		RegisteredAt:  agent.RegisteredAt,
		LastResetAt:   agent.LastResetAt,
	}
}

func AgentsToResponse(agents []*models.Agent) []AgentResponse {
	result := make([]AgentResponse, len(agents))
	for i, agent := range agents {
		result[i] = AgentToResponse(agent)
	}
	return result
}
