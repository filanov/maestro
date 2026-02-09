package cleaner

import (
	"context"
	"log/slog"
	"time"

	"github.com/filanov/maestro/internal/db"
)

type Cleaner struct {
	db                        db.DB
	interval                  time.Duration
	retentionDays             int
	maxExecutionsPerAgentTask int
}

func NewCleaner(database db.DB, interval time.Duration, retentionDays, maxExecutionsPerAgentTask int) *Cleaner {
	return &Cleaner{
		db:                        database,
		interval:                  interval,
		retentionDays:             retentionDays,
		maxExecutionsPerAgentTask: maxExecutionsPerAgentTask,
	}
}

func (c *Cleaner) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	slog.Info("cleaner started", "interval", c.interval, "retention_days", c.retentionDays)

	for {
		select {
		case <-ticker.C:
			if err := c.cleanup(ctx); err != nil {
				slog.Error("cleanup failed", "error", err)
			}
		case <-ctx.Done():
			slog.Info("cleaner stopped")
			return
		}
	}
}

func (c *Cleaner) cleanup(ctx context.Context) error {
	threshold := time.Now().AddDate(0, 0, -c.retentionDays)
	if err := c.db.DeleteExecutionsForDeletedTasksOlderThan(ctx, threshold); err != nil {
		slog.Error("failed to delete executions for deleted tasks", "error", err)
	}

	if err := c.db.DeleteOldExecutionsKeepLastN(ctx, c.maxExecutionsPerAgentTask); err != nil {
		slog.Error("failed to delete old executions", "error", err)
	}

	debugThreshold := time.Now().Add(-24 * time.Hour)
	if err := c.db.DeleteCompletedDebugTasksOlderThan(ctx, debugThreshold); err != nil {
		slog.Error("failed to delete old debug tasks", "error", err)
	}

	timeoutThreshold := time.Now().Add(-1 * time.Hour)
	if err := c.db.TimeoutPendingDebugTasks(ctx, timeoutThreshold); err != nil {
		slog.Error("failed to timeout pending debug tasks", "error", err)
	}

	return nil
}
