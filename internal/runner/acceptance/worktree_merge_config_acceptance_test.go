//go:build acceptance

package acceptance_test

import (
	"testing"
)

func TestWorktreeMergeConfigHelpersSuffice(t *testing.T) {
	// Verify worktree merge test setup works via helpers only,
	// without a direct config import in the test file.
	// worktree_merge_acceptance_test.go has an unused config import
	// that prevents this package from compiling.
	cfg := baseWorktreeMergeConfig()
	configureWorktreeMerge(cfg, true, "warn")
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	if cfg.Worktree.MergeFailure != "warn" {
		t.Fatalf("MergeFailure = %q, want %q", cfg.Worktree.MergeFailure, "warn")
	}
}
