package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner"
)

// HydrationProvider exposes the data sources used to hydrate the TUI state.
type HydrationProvider interface {
	RunnerStatus(ctx context.Context, gromitDir string) (*runner.Status, error)
	PipelineStatus(ctx context.Context, gromitDir, specsDir, plansDir string, startedAt *time.Time) (*pipeline.PipelineStatus, error)
	PipelineItems(ctx context.Context, gromitDir, specsDir, plansDir string) (PipelineItems, error)
}

// HydrateStore loads the dashboard and queue fields from the provided sources.
func HydrateStore(ctx context.Context, cfg *config.Config, gromitDir, specsDir, plansDir string, provider HydrationProvider) *Store {
	now := time.Now()
	store := &Store{}
	if provider == nil {
		store.Dashboard.LastHydration = now
		store.Dashboard.Warnings = append(store.Dashboard.Warnings, "hydration provider missing")
		return store
	}

	var dashboardWarnings []string

	runnerStatus, err := provider.RunnerStatus(ctx, gromitDir)
	if err != nil {
		dashboardWarnings = append(dashboardWarnings, fmt.Sprintf("runner status: %v", err))
	}
	store.Dashboard.RunnerStatus = runnerStatus

	var startedAt *time.Time
	if runnerStatus != nil && !runnerStatus.StartedAt.IsZero() {
		startedAt = &runnerStatus.StartedAt
	}

	pipelineStatus, err := provider.PipelineStatus(ctx, gromitDir, specsDir, plansDir, startedAt)
	if err != nil {
		dashboardWarnings = append(dashboardWarnings, fmt.Sprintf("pipeline status: %v", err))
	}
	store.Dashboard.PipelineStatus = pipelineStatus

	pipelineItems, err := provider.PipelineItems(ctx, gromitDir, specsDir, plansDir)
	if err != nil {
		dashboardWarnings = append(dashboardWarnings, fmt.Sprintf("pipeline items: %v", err))
	} else {
		store.SetPipelineItems(pipelineItems)
	}

	store.Dashboard.LastHydration = now
	store.Dashboard.Warnings = dashboardWarnings

	return store
}
