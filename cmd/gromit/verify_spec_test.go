package main

import (
	"strings"
	"testing"
)

func TestVerifySpecCmd_Registration(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "verify-spec <spec>" {
			found = true
			break
		}
	}

	if !found {
		t.Error("verify-spec command not registered with rootCmd")
	}
}

func TestVerifySpecCmd_Flags(t *testing.T) {
	flag := verifySpecCmd.Flags().Lookup("create-beads")
	if flag == nil {
		t.Error("verify-spec should have --create-beads flag")
		return
	}
	if flag.Value.Type() != "bool" {
		t.Errorf("--create-beads flag should be bool, got %s", flag.Value.Type())
	}
}

func TestExtractAcceptanceCriteria(t *testing.T) {
	body := `# Title

## Acceptance Criteria

- First criterion
- Second criterion

## Decisions

- Not part of criteria
`

	criteria, block := extractAcceptanceCriteria(body)
	if len(criteria) != 2 {
		t.Fatalf("criteria count = %d, want 2", len(criteria))
	}
	if criteria[0] != "First criterion" {
		t.Errorf("criteria[0] = %q, want %q", criteria[0], "First criterion")
	}
	if criteria[1] != "Second criterion" {
		t.Errorf("criteria[1] = %q, want %q", criteria[1], "Second criterion")
	}
	if !strings.Contains(block, "- First criterion") {
		t.Errorf("block missing first criterion: %q", block)
	}
	if strings.Contains(block, "Not part of criteria") {
		t.Errorf("block should not include subsequent sections: %q", block)
	}
}
