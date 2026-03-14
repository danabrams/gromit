package specloop

import (
	"context"
	"time"

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

// CheckSummary tracks pass/fail counts for a set of checks.
type CheckSummary struct {
	Pass int
	Fail int
}

// TaskResult is the outcome of running or repairing a task.
type TaskResult struct {
	TaskID          string // set by RunTaskLoop to the originating task's ID
	Status          string // done, failed, needs_split
	Attempts        int
	TokensUsed      int
	Cost            float64
	DurationMs      int64
	FilesChanged    []string
	Model           string
	Tier            string
	TargetedChecks  CheckSummary
	AlwaysRunChecks CheckSummary
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for JSON consistency.
func (tr *TaskResult) NormalizeNilFields() {
	if tr.FilesChanged == nil {
		tr.FilesChanged = []string{}
	}
}

// FilesChangedFunc detects files changed in a working directory.
// It returns the list of changed file paths (relative to workDir), or an error.
// Implementations should handle non-git directories gracefully (return empty list, no error).
type FilesChangedFunc func(workDir string) ([]string, error)

// TaskLoopConfig configures the task loop.
type TaskLoopConfig struct {
	MaxRetries             int
	Inspector              TaskInspector
	MaxRedecompositions    int
	Decomposer             TaskDecomposer
	GitOps                 GitOps
	Budget                 *Budget
	WorkDir                string
	MaxTaskDurationSeconds int
	EventLog               *runstore.EventLog
	Cycle                  int
	DetectFilesChanged     FilesChangedFunc // optional; if nil, FilesChanged stays as returned by runner
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
			results = append(results, TaskResult{TaskID: entry.task.TaskID, Status: "blocked", Attempts: 0})
			continue
		}

		// Emit task_started
		emitTaskEvent(cfg.EventLog, runstore.TaskStartedEvent{
			BaseEvent: runstore.BaseEvent{Type: "task_started", Timestamp: time.Now()},
			TaskID:    entry.task.TaskID,
			Cycle:     cfg.Cycle,
		})

		// Create per-task context with timeout if configured
		taskCtx := ctx
		var taskCancel context.CancelFunc
		if cfg.MaxTaskDurationSeconds > 0 {
			taskCtx, taskCancel = context.WithTimeout(ctx, time.Duration(cfg.MaxTaskDurationSeconds)*time.Second)
		}

		result, err := runner.RunTask(taskCtx, entry.task)
		if taskCancel != nil {
			taskCancel()
		}
		// Track cumulative cost across all attempts for accurate reporting
		cumulativeCost := result.Cost
		// Update budget with cost from this invocation
		if cfg.Budget != nil {
			cfg.Budget.AddCost(result.Cost)
		}
		if err != nil {
			result.TaskID = entry.task.TaskID
			result.Status = "failed"
			result.Attempts = 1
			emitTaskEvent(cfg.EventLog, runstore.TaskFailedEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_failed", Timestamp: time.Now()},
				TaskID:    entry.task.TaskID,
				Reason:    err.Error(),
			})
			results = append(results, result)
			continue
		}
		result.TaskID = entry.task.TaskID
		attempts := 1

		// Handle needs_split
		if result.Status == "needs_split" {
			emitTaskEvent(cfg.EventLog, runstore.TaskNeedsSplitEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_needs_split", Timestamp: time.Now()},
				TaskID:    entry.task.TaskID,
			})

			if entry.canDecompose && cfg.Decomposer != nil && decompositionsUsed < cfg.MaxRedecompositions {
				// Revert touched files
				if cfg.GitOps != nil && len(result.FilesChanged) > 0 {
					cfg.GitOps.CheckoutFiles(cfg.WorkDir, result.FilesChanged)
				}
				subTasks, dErr := cfg.Decomposer.Decompose(ctx, entry.task)
				if dErr == nil {
					decompositionsUsed++
					emitTaskEvent(cfg.EventLog, runstore.RedecompositionTriggeredEvent{
						BaseEvent: runstore.BaseEvent{Type: "redecomposition_triggered", Timestamp: time.Now()},
						Reason:    "task " + entry.task.TaskID + " needs split",
					})
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
			emitTaskEvent(cfg.EventLog, runstore.TaskValidationResultEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_validation_result", Timestamp: time.Now()},
				TaskID:    entry.task.TaskID,
				Passed:    ir.Pass,
			})
			if !ir.Pass {
				// Retry loop
				for retry := 0; retry < cfg.MaxRetries; retry++ {
					repairCtx := ctx
					var repairCancel context.CancelFunc
					if cfg.MaxTaskDurationSeconds > 0 {
						repairCtx, repairCancel = context.WithTimeout(ctx, time.Duration(cfg.MaxTaskDurationSeconds)*time.Second)
					}
					repairResult, rErr := runner.RepairTask(repairCtx, entry.task, ir.Failures)
					if repairCancel != nil {
						repairCancel()
					}
					// Update budget and cumulative cost from repair invocation
					cumulativeCost += repairResult.Cost
					if cfg.Budget != nil {
						cfg.Budget.AddCost(repairResult.Cost)
					}
					attempts++
					if rErr != nil {
						result.Status = "failed"
						break
					}
					result = repairResult
					result.TaskID = entry.task.TaskID
					ir = cfg.Inspector.Inspect(ctx, entry.task)
					emitTaskEvent(cfg.EventLog, runstore.TaskValidationResultEvent{
						BaseEvent: runstore.BaseEvent{Type: "task_validation_result", Timestamp: time.Now()},
						TaskID:    entry.task.TaskID,
						Passed:    ir.Pass,
					})
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
		result.Cost = cumulativeCost

		// Detect files changed by this task (if detector is configured).
		if cfg.DetectFilesChanged != nil && cfg.WorkDir != "" {
			if changed, err := cfg.DetectFilesChanged(cfg.WorkDir); err == nil {
				result.FilesChanged = changed
			}
		}

		// Emit task_completed or task_failed
		if result.Status == "done" {
			emitTaskEvent(cfg.EventLog, runstore.TaskCompletedEvent{
				BaseEvent:  runstore.BaseEvent{Type: "task_completed", Timestamp: time.Now()},
				TaskID:     entry.task.TaskID,
				TokensUsed: result.TokensUsed,
				DurationMs: result.DurationMs,
			})
		} else if result.Status == "failed" {
			emitTaskEvent(cfg.EventLog, runstore.TaskFailedEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_failed", Timestamp: time.Now()},
				TaskID:    entry.task.TaskID,
				Reason:    "task execution failed",
			})
		}

		results = append(results, result)
	}

	return results, nil
}

// emitTaskEvent appends an event to the log if the log is non-nil.
func emitTaskEvent(el *runstore.EventLog, ev runstore.TypedEvent) {
	if el != nil {
		el.Append(ev)
	}
}

// taskEntry tracks a task in the queue with metadata about decomposition eligibility.
type taskEntry struct {
	task         runstore.Task
	canDecompose bool
}
