//go:build acceptance

package acceptance_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	pipelineepilog "github.com/danabrams/gromit/internal/pipeline/epilogue"
	"github.com/danabrams/gromit/internal/runner"
)

// smoke-matrix: keep | rationale: Covers critical end-to-end merge-failure warning path to ensure run loop continues under configured warn mode. | destination: internal/runner/acceptance/worktree_merge_acceptance_test.go:TestRunnerSmoke_WorktreeMergeModesEndToEnd
func TestRunnerSmoke_WorktreeMergeModesEndToEnd(t *testing.T) {
	cfg := baseWorktreeMergeConfig()
	configureWorktreeMerge(cfg, true, "warn")
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	branches := []string{"gromit/review-123", "gromit/retro-456"}
	mergeCalls := []string{}
	mockWorktree := &mockWorktreeManager{
		PendingBranchesFn: func() ([]string, error) {
			return branches, nil
		},
		MergeBackFn: func(branch string) error {
			mergeCalls = append(mergeCalls, branch)
			return errors.New("merge conflict")
		},
	}

	beadReady := false
	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadReady {
				return nil, nil
			}
			beadReady = true
			return &bead.Bead{
				ID:       "worktree-test-1",
				Title:    "Worktree test bead",
				Priority: 1,
			}, nil
		},
	}

	epilogueStage := pipelineepilog.New(mockBeads, &mockEpilogueStatusWriter{}, io.Discard).
		WithWorktree(mockWorktree)

	orch := runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate:     &noopStage{},
		Build:    &noopStage{},
		Validate: &noopStage{},
		Epilogue: epilogueStage,
		GetBead: func(ctx context.Context) (*bead.Bead, error) {
			return mockBeads.Ready()
		},
		Config: cfg,
		Output: io.Discard,
	})

	err := orch.Run(context.Background(), 1, time.Now().Add(time.Minute), nil)
	if err != nil {
		t.Fatalf("expected merge failure to warn and continue, got error: %v", err)
	}

	if len(mergeCalls) != len(branches) {
		t.Fatalf("expected MergeBack called for %d branches, got %d", len(branches), len(mergeCalls))
	}
	for i, branch := range branches {
		if mergeCalls[i] != branch {
			t.Errorf("MergeBack call %d = %q, want %q", i, mergeCalls[i], branch)
		}
	}
}

// mockEpilogueStatusWriter is a minimal epilogue.StatusWriter for testing.
type mockEpilogueStatusWriter struct{}

func (m *mockEpilogueStatusWriter) Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error {
	return nil
}
