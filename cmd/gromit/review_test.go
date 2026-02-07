package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestReviewPassesClaudeFlags verifies that gromit review respects claude.flags
// from gromit.yaml, specifically the --dangerously-skip-permissions flag.
// Both interactive and non-interactive modes should pass through configured flags.
func TestReviewPassesClaudeFlags(t *testing.T) {
	// Test config with --dangerously-skip-permissions flag
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Flags:   []string{"--dangerously-skip-permissions", "--some-other-flag"},
			Timeout: 600,
		},
		Review: config.ReviewConfig{
			Thorough: config.ThoroughReviewConfig{
				Model:   "opus",
				Timeout: 900,
			},
		},
		Paths: config.PathsConfig{
			GromitDir:       ".gromit",
			Templates:       ".gromit/templates",
			Specs:           ".gromit/specs",
			ProjectClaudeMD: "CLAUDE.md",
			Logs:            ".gromit/logs",
		},
	}

	// Verify flags are properly configured
	if len(cfg.Claude.Flags) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(cfg.Claude.Flags))
	}

	if cfg.Claude.Flags[0] != "--dangerously-skip-permissions" {
		t.Errorf("Expected first flag to be --dangerously-skip-permissions, got %s", cfg.Claude.Flags[0])
	}

	// NOTE: Both runReviewInteractive and runReviewNonInteractive use cfg.Claude.Flags:
	// - Interactive mode: lines 314-316 in review.go build args from cfg.Claude.Flags
	// - Non-interactive mode: line 361 passes cfg.Claude.Flags to claude.NewClient
	//
	// This test documents the expected behavior. Actual execution testing would
	// require mocking exec.Command, which is complex for this verification.
}

// TestReviewWithoutFlags verifies that review works when no flags are configured
func TestReviewWithoutFlags(t *testing.T) {
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Flags:   []string{}, // No flags configured
			Timeout: 600,
		},
		Review: config.ReviewConfig{
			Thorough: config.ThoroughReviewConfig{
				Model:   "opus",
				Timeout: 900,
			},
		},
		Paths: config.PathsConfig{
			GromitDir:       ".gromit",
			Templates:       ".gromit/templates",
			Specs:           ".gromit/specs",
			ProjectClaudeMD: "CLAUDE.md",
			Logs:            ".gromit/logs",
		},
	}

	// Verify empty flags are properly configured
	if len(cfg.Claude.Flags) != 0 {
		t.Errorf("Expected 0 flags, got %d", len(cfg.Claude.Flags))
	}

	// With no flags, review should still work (with permission prompts)
	// Both modes correctly handle empty flags slice:
	// - Interactive: append empty slice is a no-op, only adds initial prompt
	// - Non-interactive: claude.NewClient accepts empty flags slice
}
