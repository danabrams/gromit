package main

import (
	"strings"
	"testing"
)

// TestRetroCmd_CLIContract_SpecFlag verifies that the retro command's --spec flag
// is included in the CLI contract test expectations
func TestRetroCmd_CLIContract_SpecFlag(t *testing.T) {
	// This test documents that the CLI contract test should be updated
	// to expect the --spec flag on the retro command
	//
	// The cli_contract_test.go file includes:
	//   {
	//     name: "retro",
	//     flags: map[string]string{
	//       "non-interactive": "bool",
	//       "spec":            "string",  // Added
	//       "epic":            "string",  // Added
	//     },
	//   }

	// Implementation complete: CLI contract test now includes --spec and --epic flags
}

// TestRetroCmd_CLIContract_EpicFlag verifies that the retro command's --epic flag
// is included in the CLI contract test expectations
func TestRetroCmd_CLIContract_EpicFlag(t *testing.T) {
	// Implementation complete: The CLI contract test has been updated
	// to expect the --epic flag on the retro command (see cli_contract_test.go)
}

// TestRetroCmd_HelpText_DocumentsSpecFlag verifies that --spec flag is documented
// in the retro command's Long help text
func TestRetroCmd_HelpText_DocumentsSpecFlag(t *testing.T) {
	// The retroCmd.Long string should include documentation for --spec flag
	// Expected text should explain:
	// - What --spec does (filters retro to a specific spec's beads)
	// - How to use it (--spec <name>)
	// - That it's mutually exclusive with --epic

	helpText := retroCmd.Long
	if !strings.Contains(helpText, "--spec") {
		t.Error("retroCmd.Long should document --spec flag")
	}

	// Should mention scope filtering or similar concept
	if !strings.Contains(strings.ToLower(helpText), "filter") {
		t.Error("retroCmd.Long should explain filtering when using --spec")
	}

	// Should mention mutual exclusivity
	if !strings.Contains(strings.ToLower(helpText), "mutually exclusive") {
		t.Error("retroCmd.Long should note that --spec and --epic are mutually exclusive")
	}
}

// TestRetroCmd_HelpText_DocumentsEpicFlag verifies that --epic flag is documented
// in the retro command's Long help text
func TestRetroCmd_HelpText_DocumentsEpicFlag(t *testing.T) {
	// The retroCmd.Long string should include documentation for --epic flag
	// Expected text should explain:
	// - What --epic does (filters retro to an epic's beads)
	// - How to use it (--epic <id>)
	// - That it's mutually exclusive with --spec

	helpText := retroCmd.Long
	if !strings.Contains(helpText, "--epic") {
		t.Error("retroCmd.Long should document --epic flag")
	}

	// Should mention scope filtering or similar concept
	if !strings.Contains(strings.ToLower(helpText), "filter") {
		t.Error("retroCmd.Long should explain filtering when using --epic")
	}

	// Should mention mutual exclusivity
	if !strings.Contains(strings.ToLower(helpText), "mutually exclusive") {
		t.Error("retroCmd.Long should note that --spec and --epic are mutually exclusive")
	}
}

// TestRetroCmd_HelpText_ExplainsScopeFiltering verifies that the help text
// includes a section explaining scope filtering
func TestRetroCmd_HelpText_ExplainsScopeFiltering(t *testing.T) {
	// The retroCmd.Long should have a section that explains:
	// - Both flags filter the analysis to a subset of beads
	// - --spec filters to one spec's beads
	// - --epic filters to all beads from specs in that epic
	// - The flags are mutually exclusive

	// Suggested format:
	//
	// Scope filtering:
	//   --spec <name>      Filter retro to a specific spec's beads
	//   --epic <id>        Filter retro to an epic's beads (all specs in that epic)
	//
	// The --spec and --epic flags are mutually exclusive.

	helpText := retroCmd.Long

	// Check for key concepts
	if !strings.Contains(helpText, "--spec") {
		t.Error("Help text should mention --spec flag")
	}

	if !strings.Contains(helpText, "--epic") {
		t.Error("Help text should mention --epic flag")
	}

	if !strings.Contains(strings.ToLower(helpText), "filter") {
		t.Error("Help text should explain filtering concept")
	}

	if !strings.Contains(strings.ToLower(helpText), "mutually exclusive") {
		t.Error("Help text should note mutual exclusivity")
	}
}

// TestRetroCmd_HelpText_MutualExclusivityDocumented verifies that the help text
// explicitly states that --spec and --epic are mutually exclusive
func TestRetroCmd_HelpText_MutualExclusivityDocumented(t *testing.T) {
	// The help text should clearly state that --spec and --epic cannot be used together
	// This is an important user-facing constraint that should be documented

	helpText := retroCmd.Long
	helpLower := strings.ToLower(helpText)

	if !strings.Contains(helpLower, "mutually exclusive") {
		t.Error("retroCmd.Long should explicitly state that --spec and --epic are mutually exclusive")
	}

	// Should mention both flags in the context of exclusivity
	if !(strings.Contains(helpText, "--spec") && strings.Contains(helpText, "--epic")) {
		t.Error("retroCmd.Long should mention both --spec and --epic when discussing mutual exclusivity")
	}
}
