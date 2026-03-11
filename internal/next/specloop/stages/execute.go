package stages

import (
	"context"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// ExecuteStageConfig configures the ExecuteStage.
type ExecuteStageConfig struct {
	MaxRetries          int
	MaxRedecompositions int
	Inspector           specloop.TaskInspector
	Decomposer          specloop.TaskDecomposer
	GitOps              specloop.GitOps
	Budget              *specloop.Budget
	WorkDir             string
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

// Run executes all tasks via the task loop.
func (s *ExecuteStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	results, err := specloop.RunTaskLoop(ctx, rs.Tasks, s.runner, specloop.TaskLoopConfig{
		MaxRetries:          s.cfg.MaxRetries,
		Inspector:           s.cfg.Inspector,
		MaxRedecompositions: s.cfg.MaxRedecompositions,
		Decomposer:          s.cfg.Decomposer,
		GitOps:              s.cfg.GitOps,
		Budget:              s.cfg.Budget,
		WorkDir:             s.cfg.WorkDir,
	})
	if err != nil {
		return specloop.NextAction{}, err
	}

	// Update task statuses from results
	allFailed := true
	for i, r := range results {
		if i < len(rs.Tasks) {
			rs.Tasks[i].Status = r.Status
			rs.Tasks[i].Attempts = r.Attempts
			rs.Tasks[i].TokensUsed = r.TokensUsed
			rs.Tasks[i].DurationMs = r.DurationMs
			rs.Tasks[i].FilesChanged = r.FilesChanged
			rs.Tasks[i].ModelTier = r.Tier
			rs.Tasks[i].NormalizeNilFields()
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
			Kind: specloop.NeedsHuman,
			Context: &specloop.FailureContext{
				Failures: []string{"all tasks failed"},
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}
