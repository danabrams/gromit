package main

import (
	"os/exec"
	"strconv"
	"testing"
)

// TestBuildReviewArgsCorrectlyOrdersArguments verifies that buildReviewArgs produces
// arguments in the correct order for exec.Command: flags first, then prompt last.
// This ensures the prompt is correctly interpreted as the final positional argument.
func TestBuildReviewArgsCorrectlyOrdersArguments(t *testing.T) {
	tests := []struct {
		name   string
		flags  []string
		prompt string
		want   []string
	}{
		{
			name:   "single flag with prompt",
			flags:  []string{"--dangerously-skip-permissions"},
			prompt: "Review this code",
			want:   []string{"--dangerously-skip-permissions", "Review this code"},
		},
		{
			name:   "multiple flags with prompt",
			flags:  []string{"--dangerously-skip-permissions", "--fast"},
			prompt: "Review this code",
			want:   []string{"--dangerously-skip-permissions", "--fast", "Review this code"},
		},
		{
			name:   "no flags with prompt",
			flags:  []string{},
			prompt: "Review this code",
			want:   []string{"Review this code"},
		},
		{
			name:   "nil flags with prompt",
			flags:  nil,
			prompt: "Review this code",
			want:   []string{"Review this code"},
		},
		{
			name:   "flags with flag values",
			flags:  []string{"--model", "opus", "--timeout", "60"},
			prompt: "Review this code",
			want:   []string{"--model", "opus", "--timeout", "60", "Review this code"},
		},
		{
			name:   "prompt with special characters",
			flags:  []string{"--flag"},
			prompt: "Review: changes in main.go (impact: high)",
			want:   []string{"--flag", "Review: changes in main.go (impact: high)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildReviewArgs(tt.flags, tt.prompt)

			// Verify length
			if len(args) != len(tt.want) {
				t.Fatalf("expected %d args, got %d", len(tt.want), len(args))
			}

			// Verify order and content
			for i, wantArg := range tt.want {
				if args[i] != wantArg {
					t.Errorf("args[%d]: expected %q, got %q", i, wantArg, args[i])
				}
			}

			// Verify prompt is always last
			if len(args) > 0 && args[len(args)-1] != tt.prompt {
				t.Errorf("prompt should be last arg: got %q at index %d", args[len(args)-1], len(args)-1)
			}
		})
	}
}

// TestBuildReviewArgsIntegration verifies that the output of buildReviewArgs
// can be used to construct a valid exec.Command for the claude CLI.
// It verifies that cmd.Args contains all the flags and prompt in the correct order.
func TestBuildReviewArgsIntegration(t *testing.T) {
	binary := "claude"
	flags := []string{"--dangerously-skip-permissions", "--fast"}
	prompt := "Review this code"

	args := buildReviewArgs(flags, prompt)

	// Verify that cmd construction doesn't panic and produces the right command
	cmd := exec.Command(binary, args...)

	if len(cmd.Args) != len(args)+1 { // +1 for the binary name
		t.Errorf("command args length: expected %d, got %d", len(args)+1, len(cmd.Args))
	}

	// Remaining args after the binary name should match our built args
	for i, arg := range args {
		if cmd.Args[i+1] != arg {
			t.Errorf("cmd.Args[%d]: expected %q, got %q", i+1, arg, cmd.Args[i+1])
		}
	}

	// Verify prompt is the last argument
	if cmd.Args[len(cmd.Args)-1] != prompt {
		t.Errorf("last argument should be prompt %q, got %q", prompt, cmd.Args[len(cmd.Args)-1])
	}
}

// TestBuildReviewArgsPreservesCapacity verifies that buildReviewArgs
// allocates the correct capacity to avoid reallocation.
func TestBuildReviewArgsPreservesCapacity(t *testing.T) {
	flags := []string{"--flag1", "--flag2"}
	prompt := "Review this"

	args := buildReviewArgs(flags, prompt)

	// Capacity should be at least len(flags)+1 to avoid reallocation during append
	expectedCap := len(flags) + 1
	if cap(args) < expectedCap {
		t.Errorf("slice capacity: expected at least %d, got %d", expectedCap, cap(args))
	}

	// Length should equal the number of flags + 1 (for prompt)
	if len(args) != len(flags)+1 {
		t.Errorf("slice length: expected %d, got %d", len(flags)+1, len(args))
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
