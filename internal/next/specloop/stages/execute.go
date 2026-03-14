package stages

import (
	"context"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// ExecuteStageConfig configures the ExecuteStage.
type ExecuteStageConfig struct {
	MaxRetries             int
	MaxRedecompositions    int
	Inspector              specloop.TaskInspector
	Decomposer             specloop.TaskDecomposer
	GitOps                 specloop.GitOps
	Budget                 *specloop.Budget
	WorkDir                string
	MaxTaskDurationSeconds int
	EventLog               *runstore.EventLog
	DetectFilesChanged     specloop.FilesChangedFunc // optional; populates TaskResult.FilesChanged
}

// ExecuteStage runs the task loop on all tasks in the run state.
type ExecuteStage struct {
	runner specloop.TaskRunner
	cfg    ExecuteStageConfig
}

// NewExecuteStage creates a new ExecuteStage.
func NewExecuteStage(runner specloop.TaskRunner, cfg ExecuteStageConfig) *ExecuteStage {
	return &ExecuteStage{runner: runner, cfg: cfg}
}

// Name returns the stage name.
func (s *ExecuteStage) Name() string { return "execute" }

// Decomposer returns the configured TaskDecomposer (nil if not set).
// Exposed for testing wiring in BuildStages.
func (s *ExecuteStage) Decomposer() specloop.TaskDecomposer { return s.cfg.Decomposer }

// TaskGitOps returns the configured GitOps (nil if not set).
// Exposed for testing wiring in BuildStages.
func (s *ExecuteStage) TaskGitOps() specloop.GitOps { return s.cfg.GitOps }

// pendingTasks returns only tasks that have not yet been executed (status "pending").
func pendingTasks(tasks []runstore.Task) []runstore.Task {
	var pending []runstore.Task
	for _, t := range tasks {
		if t.Status == "pending" {
			pending = append(pending, t)
		}
	}
	return pending
}

// Run executes all pending tasks via the task loop.
func (s *ExecuteStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	results, err := specloop.RunTaskLoop(ctx, pendingTasks(rs.Tasks), s.runner, specloop.TaskLoopConfig{
		MaxRetries:             s.cfg.MaxRetries,
		Inspector:              s.cfg.Inspector,
		MaxRedecompositions:    s.cfg.MaxRedecompositions,
		Decomposer:             s.cfg.Decomposer,
		GitOps:                 s.cfg.GitOps,
		Budget:                 s.cfg.Budget,
		WorkDir:                s.cfg.WorkDir,
		MaxTaskDurationSeconds: s.cfg.MaxTaskDurationSeconds,
		EventLog:               s.cfg.EventLog,
		Cycle:                  rs.Cycle,
		DetectFilesChanged:     s.cfg.DetectFilesChanged,
	})
	if err != nil {
		return specloop.NextAction{}, err
	}

	// Build a map of existing task indices by TaskID for fast lookup.
	taskIndex := make(map[string]int, len(rs.Tasks))
	for i, t := range rs.Tasks {
		taskIndex[t.TaskID] = i
	}

	// Update task statuses from results.
	// Results are mapped by TaskID rather than by index to handle decomposition correctly.
	allFailed := true
	for _, r := range results {
		if idx, ok := taskIndex[r.TaskID]; ok {
			rs.Tasks[idx].Status = r.Status
			rs.Tasks[idx].Attempts = r.Attempts
			rs.Tasks[idx].TokensUsed = r.TokensUsed
			rs.Tasks[idx].DurationMs = r.DurationMs
			rs.Tasks[idx].FilesChanged = r.FilesChanged
			rs.Tasks[idx].ModelTier = r.Tier
			rs.Tasks[idx].NormalizeNilFields()
		} else {
			// Decomposed sub-task result — append as new task entry
			newTask := runstore.Task{
				TaskID:       r.TaskID,
				Status:       r.Status,
				Attempts:     r.Attempts,
				TokensUsed:   r.TokensUsed,
				DurationMs:   r.DurationMs,
				FilesChanged: r.FilesChanged,
				ModelTier:    r.Tier,
				Cycle:        rs.Cycle,
				Kind:         "decomposed",
			}
			newTask.NormalizeNilFields()
			rs.Tasks = append(rs.Tasks, newTask)
			taskIndex[r.TaskID] = len(rs.Tasks) - 1
		}
		if r.Status != "failed" {
			allFailed = false
		}
	}

	// Accumulate cost
	for _, r := range results {
		rs.AccumulatedCost += r.Cost
	}

	if allFailed && len(results) > 0 {
		return specloop.NextAction{
			Kind: specloop.ReplanFrom,
			Context: &specloop.FailureContext{
				Failures: []string{"all tasks failed"},
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}
