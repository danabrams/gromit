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

func TestVerifySpecCmd_ArgParsing(t *testing.T) {
	_, stderr, exitCode := runGromitCobra(t, "verify-spec")
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr, "accepts 1 arg(s), received 0") {
		t.Fatalf("stderr = %q, want argument count error", stderr)
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

func TestVerifySpecCmd_OutputTableFormat(t *testing.T) {
	specContent := `---
id: my-spec
---

# My Spec

## Acceptance Criteria

- First criterion
- Second criterion
`
	setupVerifySpecTest(t, "my-spec", specContent)

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed: true,
			Results: []specgate.CriterionResult{
				{Criterion: "First criterion", Passed: true, Evidence: "covered by unit tests"},
				{Criterion: "Second criterion", Passed: false, Evidence: "missing assertion"},
			},
		}, nil
	}
	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
	})

	stdout, stderr, exitCode := runGromitCobra(t, "verify-spec", "my-spec")
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", exitCode, stderr)
	}
	if !strings.Contains(stdout, "CRITERION") || !strings.Contains(stdout, "STATUS") || !strings.Contains(stdout, "EVIDENCE") {
		t.Fatalf("stdout missing table header columns, got: %q", stdout)
	}
	if !strings.Contains(stdout, "First criterion") || !strings.Contains(stdout, "PASS") || !strings.Contains(stdout, "covered by unit tests") {
		t.Fatalf("stdout missing PASS row details, got: %q", stdout)
	}
	if !strings.Contains(stdout, "Second criterion") || !strings.Contains(stdout, "FAIL") || !strings.Contains(stdout, "missing assertion") {
		t.Fatalf("stdout missing FAIL row details, got: %q", stdout)
	}
}

func TestRunVerifySpec_PassReturnsNil(t *testing.T) {
	specContent := `---
id: my-spec
---

# My Spec

## Acceptance Criteria

- First criterion
`
	setupVerifySpecTest(t, "my-spec", specContent)

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		if specName != "my-spec" {
			t.Fatalf("specName = %q, want %q", specName, "my-spec")
		}
		if len(criteria) != 1 || criteria[0] != "First criterion" {
			t.Fatalf("criteria = %v, want [First criterion]", criteria)
		}
		return &specgate.GateVerdict{
			Passed:  true,
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

func TestVerifySpecCmd_ExitCodePassAndFail(t *testing.T) {
	specContent := `---
id: my-spec
---

# My Spec

## Acceptance Criteria

- First criterion
`
	setupVerifySpecTest(t, "my-spec", specContent)

	prevRunner := verifySpecGateRunner
	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
	})

	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed:  true,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: true, Evidence: "ok"}},
		}, nil
	}
	_, stderr, exitCode := runGromitCobra(t, "verify-spec", "my-spec")
	if exitCode != 0 {
		t.Fatalf("pass case exit code = %d, want 0 (stderr: %s)", exitCode, stderr)
	}

	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed:  false,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: false, Evidence: "missing"}},
		}, nil
	}
	_, stderr, exitCode = runGromitCobra(t, "verify-spec", "my-spec")
	if exitCode != 1 {
		t.Fatalf("fail case exit code = %d, want 1 (stderr: %s)", exitCode, stderr)
	}
	if !strings.Contains(stderr, "spec gate failed") {
		t.Fatalf("stderr = %q, want spec gate failed", stderr)
	}
}

func TestRunVerifySpec_CreateBeadsOnFailure(t *testing.T) {
	specContent := `---
id: my-spec
---

# My Spec

## Acceptance Criteria

- First criterion
`
	setupVerifySpecTest(t, "my-spec", specContent)

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed:  false,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: false, Evidence: "missing"}},
		}, nil
	}
	prevCreate := verifySpecFixBeadsFn
	called := false
	verifySpecFixBeadsFn = func(ctx context.Context, specName string, verdict *specgate.GateVerdict) ([]string, error) {
		called = true
		return []string{"gromit-123"}, nil
	}
	verifySpecCreateBeads = true

	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
		verifySpecFixBeadsFn = prevCreate
		verifySpecCreateBeads = false
	})

	if err := runVerifySpec(verifySpecCmd, []string{"my-spec"}); err == nil {
		t.Fatal("expected error on gate failure")
	}
	if !called {
		t.Fatal("expected fix beads creation on failure")
	}
}

func TestVerifySpecCmd_CreateBeadsPrintsIDs(t *testing.T) {
	specContent := `---
id: my-spec
---

# My Spec

## Acceptance Criteria

- First criterion
`
	setupVerifySpecTest(t, "my-spec", specContent)

	prevRunner := verifySpecGateRunner
	verifySpecGateRunner = func(ctx context.Context, cfg *config.Config, specName string, criteria []string, block string, body string) (*specgate.GateVerdict, error) {
		return &specgate.GateVerdict{
			Passed:  false,
			Results: []specgate.CriterionResult{{Criterion: "First criterion", Passed: false, Evidence: "missing"}},
		}, nil
	}
	prevCreate := verifySpecFixBeadsFn
	verifySpecFixBeadsFn = func(ctx context.Context, specName string, verdict *specgate.GateVerdict) ([]string, error) {
		return []string{"gromit-a", "gromit-b"}, nil
	}

	t.Cleanup(func() {
		verifySpecGateRunner = prevRunner
		verifySpecFixBeadsFn = prevCreate
		verifySpecCreateBeads = false
	})

	stdout, _, exitCode := runGromitCobra(t, "verify-spec", "my-spec", "--create-beads")
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stdout, "Created fix beads: gromit-a, gromit-b") {
		t.Fatalf("stdout = %q, want created bead IDs", stdout)
	}
}

func setupVerifySpecTest(t *testing.T, specName string, specContent string) {
	t.Helper()

	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, specName+".md"), []byte(specContent), 0644); err != nil {
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
}
