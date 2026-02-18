package engine

import (
	"context"
	"time"

	"github.com/filanov/maestro/internal/db"
	"github.com/filanov/maestro/internal/models"
)

type Scheduler struct {
	db db.DB
}

func NewScheduler(database db.DB) *Scheduler {
	return &Scheduler{db: database}
}

func (s *Scheduler) GetTasksForAgent(ctx context.Context, agentID string) ([]*models.Task, error) {
	agent, err := s.db.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	tasks, err := s.db.GetTasksForCluster(ctx, agent.ClusterID)
	if err != nil {
		return nil, err
	}

	executions, err := s.db.GetExecutionsForAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	executionMap := make(map[string]*models.TaskExecution)
	for _, exec := range executions {
		executionMap[exec.TaskID] = exec
	}

	pendingTasks := make([]*models.Task, 0)
	blockingFailed := false

	for _, task := range tasks {
		exec, exists := executionMap[task.ID]

		// Handle scheduled tasks (periodic execution)
		if task.ScheduleEnabled {
			if !exists {
				// First run - include it
				pendingTasks = append(pendingTasks, task)
				continue
			}

			// Check if task is currently running
			if exec.Status == models.ExecutionStatusRunning {
				continue
			}

			// Check if enough time has elapsed since last execution
			elapsed := time.Since(exec.StartedAt)
			if elapsed >= task.ScheduleInterval {
				// Time to re-run
				pendingTasks = append(pendingTasks, task)
			}
			continue
		}

		// Handle regular (non-scheduled) tasks
		if exists {
			switch exec.Status {
			case models.ExecutionStatusSuccess:
				continue
			case models.ExecutionStatusRunning:
				continue
			case models.ExecutionStatusSkipped:
				continue
			case models.ExecutionStatusFailed:
				if task.Blocking {
					blockingFailed = true
					continue
				}
				continue
			}
		}

		if blockingFailed {
			continue
		}

		pendingTasks = append(pendingTasks, task)
	}

	return pendingTasks, nil
}
