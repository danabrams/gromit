//go:build acceptance

package runner

import (
	"context"
	"errors"
	"testing"
	"time"
)

// smoke-matrix: keep | rationale: Covers critical end-to-end merge-failure warning path to ensure run loop continues under configured warn mode. | destination: internal/runner/worktree_merge_acceptance_test.go:TestRunnerSmoke_WorktreeMergeModesEndToEnd
func TestRunnerSmoke_WorktreeMergeModesEndToEnd(t *testing.T) {
	cfg := baseWorktreeMergeConfig()
	configureWorktreeMerge(cfg, true, "warn")

	branches := []string{"gromit/review-123", "gromit/retro-456"}
	mergeCalls := []string{}
	mockWorktrees := &mockWorktreeManager{
		PendingBranchesFn: func() ([]string, error) {
			return branches, nil
		},
		MergeBackFn: func(branch string) error {
			mergeCalls = append(mergeCalls, branch)
			return errors.New("merge conflict")
		},
	}

	r := setupRunnerForWorktreeMerge(t, cfg)
	r.worktreeManager = mockWorktrees

	err := r.Run(context.Background(), 1, time.Now().Add(time.Minute), nil, false)
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
