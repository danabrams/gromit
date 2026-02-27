package tui

import (
    "context"
    "testing"
    "time"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/pipeline"
    "github.com/danabrams/gromit/internal/runner"
)

func TestHydrateStore_PopulatesDashboardFromStatuses(t *testing.T) {
    t.Parallel()

    runnerStatus := &runner.Status{Running: true, BeadID: "bead-123", StartedAt: time.Unix(1, 0)}
    pipelineStatus := &pipeline.PipelineStatus{ReadyBeadCount: 5}

    provider := &mockHydrationProvider{
        runnerStatus:   runnerStatus,
        pipelineStatus: pipelineStatus,
    }

    cfg := &config.Config{}
    store := HydrateStore(context.Background(), cfg, ".gromit", ".gromit/specs", ".gromit/plans", provider)

    if store.Dashboard.RunnerStatus != runnerStatus {
        t.Fatalf("runner status = %+v, want %+v", store.Dashboard.RunnerStatus, runnerStatus)
    }

    if store.Dashboard.PipelineStatus != pipelineStatus {
        t.Fatalf("pipeline status = %+v, want %+v", store.Dashboard.PipelineStatus, pipelineStatus)
    }

    if store.Dashboard.LastHydration.IsZero() {
        t.Fatalf("expected dashboard last hydration to be set")
    }

    if len(store.Dashboard.Warnings) != 0 {
        t.Fatalf("unexpected dashboard warnings: %v", store.Dashboard.Warnings)
    }
}

var _ HydrationProvider = (*mockHydrationProvider)(nil)

type mockHydrationProvider struct {
    runnerStatus   *runner.Status
    pipelineStatus *pipeline.PipelineStatus
    runnerErr      error
    pipelineErr    error
}

func (m *mockHydrationProvider) RunnerStatus(ctx context.Context, gromitDir string) (*runner.Status, error) {
    return m.runnerStatus, m.runnerErr
}

func (m *mockHydrationProvider) PipelineStatus(ctx context.Context, gromitDir, specsDir, plansDir string, startedAt *time.Time) (*pipeline.PipelineStatus, error) {
    return m.pipelineStatus, m.pipelineErr
}
