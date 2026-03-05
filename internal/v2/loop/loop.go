package loop

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
)

// AdapterSet alias exposes the adapter basket consumed by the run loop.
type AdapterSet = adapter.AdapterSet

// SpecLoop orchestrates the adapters that drive a single spec iteration.
type SpecLoop struct {
	adapters adapter.AdapterSet
	cfg      *config.Config
}

// NewSpecLoop constructs a spec loop backed by the provided adapters and configuration.
func NewSpecLoop(adapters adapter.AdapterSet, cfg *config.Config) (*SpecLoop, error) {
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
	return &SpecLoop{adapters: adapters, cfg: cfg}, nil
}

// Run executes the configured adapters for the requested spec.
func (s *SpecLoop) Run(ctx context.Context, specID string) error {
	if specID == "" {
		return fmt.Errorf("spec ID required")
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

	summary := fmt.Sprintf("spec=%s profile=%s plan=%q worktree=%s", specID, s.cfg.Project.Profile, plan, worktree)
	if err := s.adapters.Presenter.PresentSummary(ctx, specID, summary); err != nil {
		return fmt.Errorf("present summary: %w", err)
	}

	return nil
}
