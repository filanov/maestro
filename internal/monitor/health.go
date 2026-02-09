package monitor

import (
	"context"
	"log/slog"
	"time"

	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/models"
)

type HealthMonitor struct {
	db              db.DB
	interval        time.Duration
	missedThreshold int
}

func NewHealthMonitor(database db.DB, interval time.Duration, missedThreshold int) *HealthMonitor {
	return &HealthMonitor{
		db:              database,
		interval:        interval,
		missedThreshold: missedThreshold,
	}
}

func (m *HealthMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	slog.Info("health monitor started", "interval", m.interval, "threshold", m.missedThreshold)

	for {
		select {
		case <-ticker.C:
			if err := m.checkHealth(ctx); err != nil {
				slog.Error("health check failed", "error", err)
			}
		case <-ctx.Done():
			slog.Info("health monitor stopped")
			return
		}
	}
}

func (m *HealthMonitor) checkHealth(ctx context.Context) error {
	threshold := time.Now().Add(-m.interval * time.Duration(m.missedThreshold))
	agents, err := m.db.FindAgentsWithHeartbeatBefore(ctx, threshold)
	if err != nil {
		return err
	}

	for _, agent := range agents {
		slog.Info("agent became inactive", "agent_id", agent.ID, "last_heartbeat", agent.LastHeartbeat)

		if err := m.db.UpdateAgentStatus(ctx, agent.ID, models.AgentStatusInactive); err != nil {
			slog.Error("failed to mark agent inactive", "agent_id", agent.ID, "error", err)
			continue
		}

		if err := m.db.FailRunningTasksForAgent(ctx, agent.ID); err != nil {
			slog.Error("failed to fail running tasks", "agent_id", agent.ID, "error", err)
		}
	}

	if len(agents) > 0 {
		slog.Info("marked agents inactive", "count", len(agents))
	}

	return nil
}
