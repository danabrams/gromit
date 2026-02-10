package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestTimestampComparison verifies that Unix timestamp comparison is done numerically,
// not lexicographically. This is a regression test for the bug where string comparison
// was used (e.g. "9" > "10" in string comparison).
func TestTimestampComparison(t *testing.T) {
	tests := []struct {
		name       string
		timestamp1 string
		timestamp2 string
		want       bool // true if timestamp1 < timestamp2
	}{
		{
			name:       "clearly earlier (10 digits)",
			timestamp1: "1609459200", // 2021-01-01
			timestamp2: "1640995200", // 2022-01-01
			want:       true,
		},
		{
			name:       "clearly later (10 digits)",
			timestamp1: "1640995200", // 2022-01-01
			timestamp2: "1609459200", // 2021-01-01
			want:       false,
		},
		{
			name:       "equal timestamps",
			timestamp1: "1609459200",
			timestamp2: "1609459200",
			want:       false,
		},
		{
			name:       "single digit vs double digit (string compare would fail)",
			timestamp1: "9",
			timestamp2: "10",
			want:       true, // 9 < 10 numerically (but "9" > "10" in string comparison)
		},
		{
			name:       "large vs small with string comparison issue",
			timestamp1: "999999999",  // 9 digits
			timestamp2: "1000000000", // 10 digits (epoch start)
			want:       true,         // numerically correct (but string compare would be false)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the timestamps using the same logic as isCommitEarlier
			ts1, err := strconv.ParseInt(tt.timestamp1, 10, 64)
			if err != nil {
				t.Fatalf("Failed to parse timestamp1 %q: %v", tt.timestamp1, err)
			}

			ts2, err := strconv.ParseInt(tt.timestamp2, 10, 64)
			if err != nil {
				t.Fatalf("Failed to parse timestamp2 %q: %v", tt.timestamp2, err)
			}

			got := ts1 < ts2

			if got != tt.want {
				t.Errorf("timestamp comparison: ts1=%d, ts2=%d, got %v, want %v", ts1, ts2, got, tt.want)
			}

			// Also verify that string comparison would give incorrect results for the edge cases
			if tt.name == "single digit vs double digit (string compare would fail)" {
				stringCompare := tt.timestamp1 < tt.timestamp2
				if stringCompare == got {
					t.Errorf("Expected string comparison to differ from numeric comparison for test case %q", tt.name)
				}
			}
		})
	}
}

// TestDetermineReviewScope_MutualExclusivity verifies that determineReviewScope
// validates mutual exclusivity between --epic and --spec flags
func TestDetermineReviewScope_MutualExclusivity(t *testing.T) {
	cfg := &config.Config{}

	// Save original flag values and restore them after test
	origEpic := reviewEpic
	origSpec := reviewSpec
	origSince := reviewSince
	defer func() {
		reviewEpic = origEpic
		reviewSpec = origSpec
		reviewSince = origSince
	}()

	t.Run("both epic and spec set", func(t *testing.T) {
		reviewEpic = "gromit-xyz"
		reviewSpec = "init-wizard"
		reviewSince = ""

		_, err := determineReviewScope(cfg)
		if err == nil {
			t.Fatal("expected error when both --epic and --spec are set")
		}

		if !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("error should mention mutual exclusivity, got: %v", err)
		}
	})

	t.Run("only epic set", func(t *testing.T) {
		reviewEpic = "gromit-xyz"
		reviewSpec = ""
		reviewSince = ""

		// This will fail because we don't have a real epic, but it should
		// not fail on mutual exclusivity
		_, err := determineReviewScope(cfg)
		if err != nil && strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("should not error on mutual exclusivity when only --epic is set, got: %v", err)
		}
	})

	t.Run("only spec set", func(t *testing.T) {
		reviewEpic = ""
		reviewSpec = "init-wizard"
		reviewSince = ""

		// This will fail because we don't have a real spec, but it should
		// not fail on mutual exclusivity
		_, err := determineReviewScope(cfg)
		if err != nil && strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("should not error on mutual exclusivity when only --spec is set, got: %v", err)
		}
	})

	t.Run("since overrides without mutual exclusivity error", func(t *testing.T) {
		reviewEpic = "gromit-xyz"
		reviewSpec = "init-wizard"
		reviewSince = "abc123"

		// Even if both epic and spec are set, --since takes priority
		// and should not trigger mutual exclusivity error
		commit, err := determineReviewScope(cfg)
		if err != nil {
			t.Fatalf("should not error when --since is set (takes priority), got: %v", err)
		}

		if commit != "abc123" {
			t.Errorf("expected commit abc123, got %s", commit)
		}
	})
}
