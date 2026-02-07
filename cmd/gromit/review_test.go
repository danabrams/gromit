package main

import (
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
