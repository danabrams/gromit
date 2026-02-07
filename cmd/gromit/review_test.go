package main

import (
	"testing"
)

// TestReviewPassesClaudeFlags verifies that buildReviewArgs correctly combines
// flags and prompt into an args array for the claude CLI.
func TestReviewPassesClaudeFlags(t *testing.T) {
	flags := []string{"--dangerously-skip-permissions", "--some-other-flag"}
	prompt := "Review this code"

	args := buildReviewArgs(flags, prompt)

	// Verify all flags are included in order
	if len(args) != 3 {
		t.Fatalf("Expected 3 args, got %d", len(args))
	}

	if args[0] != "--dangerously-skip-permissions" {
		t.Errorf("Expected first arg to be --dangerously-skip-permissions, got %s", args[0])
	}

	if args[1] != "--some-other-flag" {
		t.Errorf("Expected second arg to be --some-other-flag, got %s", args[1])
	}

	if args[2] != prompt {
		t.Errorf("Expected third arg to be %q, got %s", prompt, args[2])
	}
}

// TestReviewWithoutFlags verifies that buildReviewArgs works when no flags are provided
func TestReviewWithoutFlags(t *testing.T) {
	flags := []string{} // No flags configured
	prompt := "Review this code"

	args := buildReviewArgs(flags, prompt)

	// With no flags, should only have the prompt
	if len(args) != 1 {
		t.Fatalf("Expected 1 arg, got %d", len(args))
	}

	if args[0] != prompt {
		t.Errorf("Expected arg to be %q, got %s", prompt, args[0])
	}
}
