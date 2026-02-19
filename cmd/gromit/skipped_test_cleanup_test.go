package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSkippedTestCleanup verifies the acceptance criteria for the
// "Capture skipped test ideas in backlog and delete entirely-skipped files" task.
func TestSkippedTestCleanup(t *testing.T) {
	t.Run("run_scope_acceptance_test.go is deleted", func(t *testing.T) {
		// The file cmd/gromit/run_scope_acceptance_test.go should not exist
		// because all 11 tests in it were t.Skip — the file should be deleted entirely.
		_, err := os.Stat("run_scope_acceptance_test.go")
		if err == nil {
			t.Fatal("run_scope_acceptance_test.go should be deleted (all 11 tests were t.Skip)")
		}
		if !os.IsNotExist(err) {
			t.Fatalf("unexpected error checking for run_scope_acceptance_test.go: %v", err)
		}
	})

	t.Run("backlog contains ideas for skipped test behaviors", func(t *testing.T) {
		// The implementation should have run `gromit add` for each category of
		// skipped test behavior. Verify the backlog has entries covering these areas.
		//
		// Skipped test sources:
		// 1. run_scope_acceptance_test.go: scope flags (--spec, --epic) integration with runLoop()
		// 2. review_scope_acceptance_test.go: 2 skipped tests about --spec flag priority
		//    and no-matching-beads handling
		// 3. filtered_hash_eviction_acceptance_test.go: 2 skipped tests about single-save
		//    optimization and archived learnings

		ideas := loadBacklogIdeas(t)

		// Each skipped behavior source must have at least one matching backlog entry.
		// We use specific keyword combinations to avoid false positives from
		// pre-existing backlog entries about related features.
		assertBacklogContainsBehaviors(t, ideas, []backlogBehaviorCheck{
			{
				description: "run command scope flags (--spec, --epic) integration with runLoop",
				matchFn: func(text string) bool {
					// Must mention both scope/flag AND run/runLoop concepts
					hasScope := containsAny(text, "scope flag", "--spec", "--epic", "spec flag", "epic flag")
					hasRun := containsAny(text, "runloop", "run loop", "run command", "gromit run")
					return hasScope && hasRun
				},
			},
			{
				description: "review --spec flag priority and no-matching-beads handling",
				matchFn: func(text string) bool {
					hasReview := containsAny(text, "review")
					hasSpec := containsAny(text, "spec flag", "--spec", "flag priority", "no matching bead", "no-matching-bead", "no beads")
					return hasReview && hasSpec
				},
			},
			{
				description: "filtered hash eviction: single-save optimization and archived learnings",
				matchFn: func(text string) bool {
					hasEviction := containsAny(text, "hash eviction", "filtered hash", "eviction")
					hasBehavior := containsAny(text, "single save", "single-save", "archived learning", "archived")
					return hasEviction && hasBehavior
				},
			},
		})
	})

}

// TestSkippedTestCleanup_GromitJ99sy verifies acceptance criteria for
// "Delete entirely-skipped test files violating t.Skip rules".
func TestSkippedTestCleanup_GromitJ99sy(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	t.Run("entirely-skipped test files are deleted", func(t *testing.T) {
		paths := []string{
			filepath.Join(projectRoot, "cmd", "gromit", "explore_pipeline_adapter_test.go"),
			filepath.Join(projectRoot, "cmd", "gromit", "retro_launch_in_dir_integration_test.go"),
			filepath.Join(projectRoot, "internal", "runner", "integration_test.go"),
			filepath.Join(projectRoot, "internal", "runner", "legacy_mocks_test.go"),
		}

		for _, path := range paths {
			_, err := os.Stat(path)
			if err == nil {
				t.Fatalf("%s should be deleted (entirely skipped)", path)
			}
			if !os.IsNotExist(err) {
				t.Fatalf("unexpected error checking for %s: %v", path, err)
			}
		}
	})

	t.Run("backlog contains ideas for skipped test behaviors (gromit-j99sy)", func(t *testing.T) {
		ideas := loadBacklogIdeas(t)

		assertBacklogContainsBehaviors(t, ideas, []backlogBehaviorCheck{
			{
				description: "explore CLI delegates to Pipeline.Explore and reports created artifacts",
				matchFn: func(text string) bool {
					hasExplore := containsAny(text, "explore")
					hasPipeline := containsAny(text, "pipeline", "pipeline.explore")
					hasArtifacts := containsAny(text, "created", "epic", "spec", "backlog")
					return hasExplore && hasPipeline && hasArtifacts
				},
			},
			{
				description: "explore prompt renderer and helper reuse",
				matchFn: func(text string) bool {
					hasRenderer := containsAny(text, "renderer", "renderexplore", "prompt")
					hasExplore := containsAny(text, "explore")
					hasHelpers := containsAny(text, "helpers", "artifact", "listmarkdownfiles")
					return hasRenderer && hasExplore && hasHelpers
				},
			},
			{
				description: "retro worktree dir passed to LaunchClaudeCode",
				matchFn: func(text string) bool {
					hasRetro := containsAny(text, "retro")
					hasDir := containsAny(text, "dir", "directory", "worktree")
					hasLaunch := containsAny(text, "launchclaudecode")
					return hasRetro && hasDir && hasLaunch
				},
			},
			{
				description: "runner integration coverage updated for router-based flows",
				matchFn: func(text string) bool {
					hasRunner := containsAny(text, "runner", "run loop", "runloop")
					hasRouter := containsAny(text, "router")
					hasIntegration := containsAny(text, "integration")
					return hasRunner && hasRouter && hasIntegration
				},
			},
		})
	})
}
