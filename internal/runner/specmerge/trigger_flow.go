package specmerge

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/review"
)

// FlowStageRunner executes a review stage for a given spec and diff.
type FlowStageRunner func(ctx context.Context, specName, diff string) (*review.ReviewResult, *provider.Result, error)

// FlowStage bundles runtime metadata with the runner func.
type FlowStage struct {
	Name   string
	Tier   string
	Runner FlowStageRunner
}

// FlowResult summarizes the progress of a TriggerFlow run.
type FlowResult struct {
	StageResults []StageResult
}

// StageResult captures the outcome of a single review stage execution.
type StageResult struct {
	StageName      string
	Tier           string
	Passed         bool
	ReviewResult   *review.ReviewResult
	ProviderResult *provider.Result
}

// StageFailureError is returned when a stage fails so callers can inspect the stage result.
type StageFailureError struct {
	Result StageResult
}

func (e StageFailureError) Error() string {
	if e.Result.StageName == "" {
		return "review stage failed"
	}
	return fmt.Sprintf("review stage %q failed", e.Result.StageName)
}

// TriggerFlow orchestrates sequential review stages for a spec merge.
type TriggerFlow struct {
	Stages       []FlowStage
	DiffProvider DiffProvider
}

// DiffProvider supplies the diff that stages should evaluate.
type DiffProvider interface {
	GetDiff(ctx context.Context, specName string) (string, error)
}

// DiffProviderFunc adapts a function to the DiffProvider interface.
type DiffProviderFunc func(ctx context.Context, specName string) (string, error)

func (fn DiffProviderFunc) GetDiff(ctx context.Context, specName string) (string, error) {
	if fn == nil {
		return "", fmt.Errorf("diff provider func is nil")
	}
	return fn(ctx, specName)
}

// Run executes each configured stage in order, stopping when a stage fails.
func (f *TriggerFlow) Run(ctx context.Context, specName string) (*FlowResult, error) {
	if f == nil {
		return nil, fmt.Errorf("trigger flow is nil")
	}
	if f.DiffProvider == nil {
		return nil, fmt.Errorf("diff provider is required")
	}
	specName = strings.TrimSpace(specName)
	if specName == "" {
		return nil, fmt.Errorf("spec name is required")
	}
	if len(f.Stages) == 0 {
		return nil, fmt.Errorf("no stages configured")
	}

	diff, err := f.DiffProvider.GetDiff(ctx, specName)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}

	result := &FlowResult{}
	for _, stage := range f.Stages {
		if stage.Runner == nil {
			return result, fmt.Errorf("stage %q runner is nil", stage.Name)
		}

		reviewResult, providerResult, err := stage.Runner(ctx, specName, diff)
		if err != nil {
			return result, fmt.Errorf("stage %q runner error: %w", stage.Name, err)
		}
		if reviewResult == nil {
			return result, fmt.Errorf("stage %q returned nil review result", stage.Name)
		}

		stageResult := StageResult{
			StageName:      stage.Name,
			Tier:           stage.Tier,
			Passed:         reviewResult.Passed,
			ReviewResult:   reviewResult,
			ProviderResult: providerResult,
		}
		result.StageResults = append(result.StageResults, stageResult)

		if !reviewResult.Passed {
			return result, StageFailureError{Result: stageResult}
		}
	}

	return result, nil
}
