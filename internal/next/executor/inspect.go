package executor

import (
	"context"

	"github.com/danabrams/gromit/internal/next/validator"
)

// GitClient abstracts git operations for testability.
type GitClient interface {
	DiffFiles(workDir string) ([]string, error)
}

// CheckRunner abstracts the validator runner for testability.
type CheckRunner interface {
	RunTargeted(ctx context.Context, proofChecks []string, workDir string) (validator.CheckResults, error)
	RunAlwaysRun(ctx context.Context, checks []validator.Check, workDir string) (validator.CheckResults, error)
}

// InspectInput holds the parameters for InspectChanges.
type InspectInput struct {
	GitClient      GitClient
	WorkDir        string
	CheckRunner    CheckRunner
	TargetedChecks []string
	AlwaysRun      []validator.Check
}

// InspectResult holds the outcome of inspecting changes after task execution.
type InspectResult struct {
	FilesChanged    []string               `json:"files_changed"`
	TargetedResult  validator.CheckResults `json:"targeted_result"`
	AlwaysRunResult validator.CheckResults `json:"always_run_result"`
}

// InspectChanges returns the list of files modified in the given worktree
// and runs targeted (proof) checks and always-run checks.
func InspectChanges(ctx context.Context, input InspectInput) (InspectResult, error) {
	files, err := input.GitClient.DiffFiles(input.WorkDir)
	if err != nil {
		return InspectResult{}, err
	}
	if files == nil {
		files = []string{}
	}

	result := InspectResult{FilesChanged: files}

	if input.CheckRunner != nil {
		if len(input.TargetedChecks) > 0 {
			targeted, err := input.CheckRunner.RunTargeted(ctx, input.TargetedChecks, input.WorkDir)
			if err != nil {
				return InspectResult{}, err
			}
			result.TargetedResult = targeted
		}
		if len(input.AlwaysRun) > 0 {
			ar, err := input.CheckRunner.RunAlwaysRun(ctx, input.AlwaysRun, input.WorkDir)
			if err != nil {
				return InspectResult{}, err
			}
			result.AlwaysRunResult = ar
		}
	}

	return result, nil
}
