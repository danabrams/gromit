package specloop

import (
	"context"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// TaskRunner executes a single task or repairs it after failure.
type TaskRunner interface {
	RunTask(ctx context.Context, task runstore.Task) (TaskResult, error)
	RepairTask(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error)
}

// TaskInspector validates a task after execution.
type TaskInspector interface {
	Inspect(ctx context.Context, task runstore.Task) InspectResult
}

// TaskDecomposer splits a task into smaller sub-tasks.
type TaskDecomposer interface {
	Decompose(ctx context.Context, task runstore.Task) ([]runstore.Task, error)
}

// GitOps abstracts git operations needed by the task loop.
type GitOps interface {
	CheckoutFiles(workDir string, files []string) error
}

// InspectResult is the outcome of task inspection.
type InspectResult struct {
	Pass     bool
	Failures []string
}

// TaskResult is the outcome of running or repairing a task.
type TaskResult struct {
	Status       string // done, failed, needs_split
	Attempts     int
	TokensUsed   int
	Cost         float64
	DurationMs   int64
	FilesChanged []string
	Model        string
	Tier         string
}

// TaskLoopConfig configures the task loop.
type TaskLoopConfig struct {
	MaxRetries          int
	Inspector           TaskInspector
	MaxRedecompositions int
	Decomposer          TaskDecomposer
	GitOps              GitOps
	Budget              *Budget
	WorkDir             string
}

// RunTaskLoop executes all tasks, with inspection and retry support.
func RunTaskLoop(ctx context.Context, tasks []runstore.Task, runner TaskRunner, cfg TaskLoopConfig) ([]TaskResult, error) {
	var results []TaskResult

	decompositionsUsed := 0

	queue := make([]taskEntry, len(tasks))
	for i, t := range tasks {
		queue[i] = taskEntry{task: t, canDecompose: true}
	}

	for i := 0; i < len(queue); i++ {
		entry := queue[i]

		// Check budget before each task
		if cfg.Budget != nil && cfg.Budget.HardBudgetExceeded() {
			results = append(results, TaskResult{Status: "blocked", Attempts: 0})
			continue
		}

		result, err := runner.RunTask(ctx, entry.task)
		if err != nil {
			results = append(results, TaskResult{Status: "failed", Attempts: 1})
			continue
		}
		attempts := 1

		// Handle needs_split
		if result.Status == "needs_split" {
			if entry.canDecompose && cfg.Decomposer != nil && decompositionsUsed < cfg.MaxRedecompositions {
				// Revert touched files
				if cfg.GitOps != nil && len(result.FilesChanged) > 0 {
					cfg.GitOps.CheckoutFiles(cfg.WorkDir, result.FilesChanged)
				}
				subTasks, dErr := cfg.Decomposer.Decompose(ctx, entry.task)
				if dErr == nil {
					decompositionsUsed++
					for _, st := range subTasks {
						queue = append(queue, taskEntry{task: st, canDecompose: false})
					}
					continue // skip adding a result for the parent
				}
			}
			// Decomposition not possible or failed — treat as failed
			result.Status = "failed"
		}

		// Inspect
		if cfg.Inspector != nil && result.Status == "done" {
			ir := cfg.Inspector.Inspect(ctx, entry.task)
			if !ir.Pass {
				// Retry loop
				for retry := 0; retry < cfg.MaxRetries; retry++ {
					repairResult, rErr := runner.RepairTask(ctx, entry.task, ir.Failures)
					attempts++
					if rErr != nil {
						result.Status = "failed"
						break
					}
					result = repairResult
					ir = cfg.Inspector.Inspect(ctx, entry.task)
					if ir.Pass {
						break
					}
				}
				// If inspection still fails after all retries, mark as failed
				if !ir.Pass {
					result.Status = "failed"
				}
			}
		}

		result.Attempts = attempts
		results = append(results, result)
	}

	return results, nil
}

// taskEntry tracks a task in the queue with metadata about decomposition eligibility.
type taskEntry struct {
	task         runstore.Task
	canDecompose bool
}
