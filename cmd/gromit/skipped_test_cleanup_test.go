package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

		projectRoot, err := findProjectRoot()
		if err != nil {
			t.Fatalf("could not find project root: %v", err)
		}

		backlogPath := filepath.Join(projectRoot, ".gromit", "backlog.jsonl")
		data, err := os.ReadFile(backlogPath)
		if err != nil {
			t.Fatalf("could not read backlog.jsonl: %v", err)
		}

		// Parse all backlog entries
		var ideas []backlogIdea
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var idea backlogIdea
			if err := json.Unmarshal([]byte(line), &idea); err != nil {
				t.Fatalf("failed to parse backlog line: %v", err)
			}
			ideas = append(ideas, idea)
		}

		// Each skipped behavior source must have at least one matching backlog entry.
		// We use specific keyword combinations to avoid false positives from
		// pre-existing backlog entries about related features.
		skippedBehaviors := []struct {
			description string
			// matchFn returns true if the idea text+context matches this behavior.
			matchFn func(text string) bool
		}{
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
		}

		for _, behavior := range skippedBehaviors {
			found := false
			for _, idea := range ideas {
				textLower := strings.ToLower(idea.Text + " " + idea.Context)
				if behavior.matchFn(textLower) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no backlog idea found for: %s", behavior.description)
			}
		}
	})

	t.Run("go test compiles after file deletion", func(t *testing.T) {
		// This test's existence and successful compilation proves that the package
		// compiles without run_scope_acceptance_test.go. If that file still existed
		// with stale imports or broken references, compilation would fail.
		//
		// The "run_scope_acceptance_test.go is deleted" subtest above verifies
		// the file is gone. This subtest is a compile-time proof that the package
		// remains valid after the deletion.
	})
}

// backlogIdea is a minimal struct for reading backlog entries in tests.
type backlogIdea struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Type    string `json:"type"`
	Context string `json:"context"`
}

// containsAny returns true if text contains any of the given substrings (case-insensitive).
func containsAny(text string, substrs ...string) bool {
	for _, s := range substrs {
		if strings.Contains(text, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// findProjectRoot walks up from the current directory to find the project root
// (identified by the presence of gromit.yaml or .gromit/ directory).
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "gromit.yaml")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".gromit")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
