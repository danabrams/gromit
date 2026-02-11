//go:build acceptance

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDeletePermanentlySkippedTests verifies the acceptance criteria for the
// "Delete permanently-skipped tests in retro_filter_test.go and stdin_helper_example_test.go" task.
func TestDeletePermanentlySkippedTests(t *testing.T) {
	t.Run("retro_filter_test.go is deleted", func(t *testing.T) {
		// Expected failure: retro_filter_test.go currently exists with 14 t.Skip tests
		// The file internal/retro/retro_filter_test.go should not exist
		// because all 14 tests in it were t.Skip — the file should be deleted entirely.
		_, err := os.Stat("../../internal/retro/retro_filter_test.go")
		if err == nil {
			t.Fatal("internal/retro/retro_filter_test.go should be deleted (all 14 tests were t.Skip)")
		}
		if !os.IsNotExist(err) {
			t.Fatalf("unexpected error checking for retro_filter_test.go: %v", err)
		}
	})

	t.Run("stdin_helper_example_test.go is deleted", func(t *testing.T) {
		// Expected failure: stdin_helper_example_test.go currently exists with 2 t.Skip tests
		// The file cmd/gromit/stdin_helper_example_test.go should not exist
		// because both tests in it were t.Skip — the file should be deleted entirely.
		_, err := os.Stat("stdin_helper_example_test.go")
		if err == nil {
			t.Fatal("stdin_helper_example_test.go should be deleted (both tests were t.Skip)")
		}
		if !os.IsNotExist(err) {
			t.Fatalf("unexpected error checking for stdin_helper_example_test.go: %v", err)
		}
	})

	t.Run("backlog contains ideas for retro filter skipped behaviors", func(t *testing.T) {
		// Expected failure: backlog entries for retro filter behaviors do not exist yet
		// The implementation should have run `gromit add` for the retro filter behaviors.
		// Verify the backlog has entries covering the filter-related functionality.
		//
		// Skipped test source: internal/retro/retro_filter_test.go
		// Main theme: Add bead filtering to retro Run method (filter by bead ID)
		// Sub-behaviors:
		// - Run method accepts optional beadFilter parameter (map[string]bool)
		// - Nil or empty filter includes all beads (default behavior)
		// - Non-empty filter excludes non-matching beads
		// - Filtering applied before stats computation (BeadStats, RunStats, EfficiencyReport)
		// - Integration with --spec and --epic flags

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

		// The retro filter behavior must have at least one matching backlog entry.
		// We look for entries that mention both "retro" and "filter" or "bead" filtering concepts.
		found := false
		for _, idea := range ideas {
			textLower := strings.ToLower(idea.Text + " " + idea.Context)
			hasRetro := containsAny(textLower, "retro", "retrospective")
			hasFilter := containsAny(textLower, "filter", "filtering", "bead id", "scope")
			if hasRetro && hasFilter {
				found = true
				break
			}
		}
		if !found {
			t.Error("no backlog idea found for: retro Run method bead filtering (all 14 skipped tests)")
		}
	})

	t.Run("backlog contains ideas for stdin helper example behaviors", func(t *testing.T) {
		// Expected failure: backlog entries for stdin helper behaviors do not exist yet
		// The implementation should have run `gromit add` for the stdin helper behaviors.
		// Verify the backlog has entries covering the stdin test helper functionality.
		//
		// Skipped test source: cmd/gromit/stdin_helper_example_test.go
		// Main theme: Test helper for commands with stdin/picker interaction
		// Sub-behaviors:
		// - runGromitWithStdin helper for simulating user input
		// - Support for picker selections (testutil.PickerStdin)
		// - Support for multiple stdin inputs

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

		// The stdin helper behavior must have at least one matching backlog entry.
		// We look for entries that specifically mention the test helper function
		// pattern (runGromitWithStdin or similar), not just tests that use stdin.
		found := false
		for _, idea := range ideas {
			textLower := strings.ToLower(idea.Text + " " + idea.Context)
			// Must mention both the helper concept AND gromit command execution with stdin
			hasHelper := containsAny(textLower, "helper function", "test helper", "helper for test", "test utility", "testing util")
			hasGromitStdin := containsAny(textLower, "rungromit", "gromit command", "gromit with stdin", "execute gromit")
			if hasHelper && hasGromitStdin {
				found = true
				break
			}
		}
		if !found {
			t.Error("no backlog idea found for: runGromitWithStdin test helper for interactive commands (2 skipped example tests)")
		}
	})
}
