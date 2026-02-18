package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/specgate"
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

func TestRunVerifySpec_PassReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	specContent := `---
id: my-spec
---

# My Spec

## Acceptance Criteria

- First criterion
`
	if err := os.WriteFile(filepath.Join(specsDir, "my-spec.md"), []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := "paths:\n  specs: " + specsDir + "\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		if specName != "my-spec" {
			t.Fatalf("specName = %q, want %q", specName, "my-spec")
		}
		if len(criteria) != 1 || criteria[0] != "First criterion" {
			t.Fatalf("criteria = %v, want [First criterion]", criteria)
		}
		return &specgate.GateVerdict{
			Passed: true,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: true, Evidence: "ok"}},
		}, nil
	}
	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
	})

	verifySpecCreateBeads = false
	if err := runVerifySpec(verifySpecCmd, []string{"my-spec"}); err != nil {
		t.Fatalf("runVerifySpec returned error: %v", err)
	}
}
