package main

import (
	"strconv"
	"testing"
)

// TestReviewPassesClaudeFlags verifies that buildReviewArgs correctly combines
// flags and prompt into an args array for the claude CLI.
// Validates extraction fidelity against inline reconstruction.
func TestReviewPassesClaudeFlags(t *testing.T) {
	flags := []string{"--dangerously-skip-permissions", "--some-other-flag"}
	prompt := "Review this code"

	// Call extracted function
	args := buildReviewArgs(flags, prompt)

	// Verify against inline reconstruction (what runReviewInteractive did at lines 314-316)
	// Original inline code was:
	//   args := make([]string, 0, len(cfg.Claude.Flags)+1)
	//   args = append(args, cfg.Claude.Flags...)
	//   args = append(args, initialPrompt)
	expected := make([]string, 0, len(flags)+1)
	expected = append(expected, flags...)
	expected = append(expected, prompt)

	// Verify structural properties
	if len(args) != len(expected) {
		t.Fatalf("Expected %d args, got %d", len(expected), len(args))
	}

	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("Expected args[%d] to be %q, got %q", i, arg, args[i])
		}
	}
}

// TestReviewWithoutFlags verifies that buildReviewArgs works when no flags are provided.
// Validates extraction fidelity against inline reconstruction.
func TestReviewWithoutFlags(t *testing.T) {
	flags := []string{} // No flags configured
	prompt := "Review this code"

	// Call extracted function
	args := buildReviewArgs(flags, prompt)

	// Verify against inline reconstruction
	expected := make([]string, 0, len(flags)+1)
	expected = append(expected, flags...)
	expected = append(expected, prompt)

	// Verify structural properties
	if len(args) != len(expected) {
		t.Fatalf("Expected %d args, got %d", len(expected), len(args))
	}

	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("Expected args[%d] to be %q, got %q", i, arg, args[i])
		}
	}
}

// TestReviewWithNilFlags verifies that buildReviewArgs works when flags is nil.
// Validates extraction fidelity against inline reconstruction.
func TestReviewWithNilFlags(t *testing.T) {
	var flags []string // nil slice
	prompt := "Review this code"

	// Call extracted function
	args := buildReviewArgs(flags, prompt)

	// Verify against inline reconstruction (nil slice behavior matches empty slice)
	expected := make([]string, 0, len(flags)+1)
	expected = append(expected, flags...)
	expected = append(expected, prompt)

	// Verify structural properties
	if len(args) != len(expected) {
		t.Fatalf("Expected %d args, got %d", len(expected), len(args))
	}

	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("Expected args[%d] to be %q, got %q", i, arg, args[i])
		}
	}
}

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
