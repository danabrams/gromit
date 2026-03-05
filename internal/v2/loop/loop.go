package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
)

// AdapterSet alias exposes the adapter basket consumed by the run loop.
type AdapterSet = adapter.AdapterSet

// DependencyGate enforces spec-level dependency checks before a run executes.
type DependencyGate interface {
	EnsureSpecReady(ctx context.Context, specID string) error
}

// SpecLoop orchestrates the adapters that drive a single spec iteration.
type SpecLoop struct {
	adapters adapter.AdapterSet
	cfg      *config.Config
	gate     DependencyGate
}

// NewSpecLoop constructs a spec loop backed by the provided adapters and configuration.
func NewSpecLoop(adapters adapter.AdapterSet, cfg *config.Config, gate DependencyGate) (*SpecLoop, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if adapters.Git == nil {
		return nil, fmt.Errorf("git adapter required")
	}
	if adapters.LLM == nil {
		return nil, fmt.Errorf("llm adapter required")
	}
	if adapters.TaskTracker == nil {
		return nil, fmt.Errorf("task tracker adapter required")
	}
	if adapters.Presenter == nil {
		return nil, fmt.Errorf("presenter adapter required")
	}
	if gate == nil {
		return nil, fmt.Errorf("dependency gate required")
	}
	return &SpecLoop{adapters: adapters, cfg: cfg, gate: gate}, nil
}

// Run executes the configured adapters for the requested spec.
func (s *SpecLoop) Run(ctx context.Context, specID string) error {
	if specID == "" {
		return fmt.Errorf("spec ID required")
	}

	if err := s.gate.EnsureSpecReady(ctx, specID); err != nil {
		return fmt.Errorf("dependency gate: %w", err)
	}

	worktree, err := s.adapters.Git.Checkout(ctx, specID)
	if err != nil {
		return fmt.Errorf("checkout: %w", err)
	}

	plan, err := s.adapters.LLM.GeneratePlan(ctx, specID)
	if err != nil {
		return fmt.Errorf("generate plan: %w", err)
	}

	if err := s.adapters.TaskTracker.RecordPlan(ctx, specID, plan); err != nil {
		return fmt.Errorf("record plan: %w", err)
	}

	specBranch := presentation.SpecBranchName(specID)
	integrationBranch := strings.TrimSpace(s.cfg.Git.BaseBranch)
	if integrationBranch == "" {
		integrationBranch = presentation.DefaultIntegrationBranch()
	}
	summary := presentation.PresentationSummary{
		SpecName:          specID,
		SpecBranch:        specBranch,
		IntegrationBranch: integrationBranch,
		Plan:              plan,
		Worktree:          worktree,
		Success:           true,
	}
	if err := s.adapters.Presenter.PresentSummary(ctx, specID, summary); err != nil {
		return fmt.Errorf("present summary: %w", err)
	}

	return nil
}
