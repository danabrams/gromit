//go:build acceptance

package main

import (
	"os"
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

	t.Run("backlog contains ideas for skipped test behaviors", func(t *testing.T) {
		// The implementation should have run `gromit add` for the skipped test behaviors.
		// Verify the backlog has entries covering these areas.
		//
		// Skipped test sources:
		// 1. internal/retro/retro_filter_test.go: 14 tests about bead filtering in retro Run method
		// 2. cmd/gromit/stdin_helper_example_test.go: 2 tests about test helper for stdin/picker interaction

		ideas := loadBacklogIdeas(t)

		// Each skipped behavior source must have at least one matching backlog entry.
		skippedBehaviors := []struct {
			description string
			matchFn     func(text string) bool
		}{
			{
				description: "retro Run method bead filtering (all 14 skipped tests)",
				matchFn: func(text string) bool {
					hasRetro := containsAny(text, "retro", "retrospective")
					hasFilter := containsAny(text, "filter", "filtering", "bead id", "scope")
					return hasRetro && hasFilter
				},
			},
			{
				description: "runGromitWithStdin test helper for interactive commands (2 skipped example tests)",
				matchFn: func(text string) bool {
					// Must mention both the helper concept AND gromit command execution with stdin
					hasHelper := containsAny(text, "helper function", "test helper", "helper for test", "test utility", "testing util")
					hasGromitStdin := containsAny(text, "rungromit", "gromit command", "gromit with stdin", "execute gromit")
					return hasHelper && hasGromitStdin
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
}
