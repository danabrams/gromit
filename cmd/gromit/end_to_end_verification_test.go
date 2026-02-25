package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/experiment"
)

// TestEndToEndVerification_LoaderParsesSampleExperimentYAML verifies that the loader
// can parse a sample experiment YAML file in a temporary directory.
func TestEndToEndVerification_LoaderParsesSampleExperimentYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a sample experiment YAML file
	expYAML := `id: sample-exp-1
phase: build
description: Sample experiment for verification
created: 2026-02-25T00:00:00Z
control:
  id: control-prompt
  template: PROMPT_build.md
variants:
  - id: variant-improved
    template: PROMPT_build_improved.md
  - id: variant-experimental
    template: PROMPT_build_experimental.md
`

	expPath := filepath.Join(tmpDir, "sample-experiment.yaml")
	if err := os.WriteFile(expPath, []byte(expYAML), 0o644); err != nil {
		t.Fatalf("failed to write sample experiment file: %v", err)
	}

	// Load experiments from the directory
	exps, err := experiment.LoadExperiments(tmpDir)
	if err != nil {
		t.Fatalf("LoadExperiments failed: %v", err)
	}

	if len(exps) == 0 {
		t.Fatal("expected to load one experiment, got none")
	}

	exp := exps[0]
	if exp.ID != "sample-exp-1" {
		t.Errorf("expected experiment ID 'sample-exp-1', got %q", exp.ID)
	}

	if exp.Phase != "build" {
		t.Errorf("expected phase 'build', got %q", exp.Phase)
	}

	if exp.Control == nil || exp.Control.ID != "control-prompt" {
		t.Error("control variant not properly loaded")
	}

	if len(exp.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(exp.Variants))
	}
}

// TestEndToEndVerification_BanditConvergesOver100Iterations verifies that the bandit
// converges correctly over 100 iterations with a rigged success rate (control has 80% success,
// variant has 60% success).
func TestEndToEndVerification_BanditConvergesOver100Iterations(t *testing.T) {
	// Create a bandit with control and variant arms
	bs := &experiment.BanditState{
		Arms: []experiment.ArmState{
			{ID: "control", Successes: 0, Failures: 0},
			{ID: "variant", Successes: 0, Failures: 0},
		},
	}

	// Simulate 100 iterations with rigged success rates:
	// - Control: 80% success (80 successes, 20 failures)
	// - Variant: 60% success (60 successes, 40 failures)
	for i := 0; i < 100; i++ {
		// Control arm: 80% success
		if i < 80 {
			bs.RecordOutcome("control", true)
		} else {
			bs.RecordOutcome("control", false)
		}

		// Variant arm: 60% success
		if i < 60 {
			bs.RecordOutcome("variant", true)
		} else {
			bs.RecordOutcome("variant", false)
		}
	}

	// Verify the bandit state reflects the recorded outcomes
	if bs.Arms[0].Successes != 80 {
		t.Errorf("control expected 80 successes, got %d", bs.Arms[0].Successes)
	}
	if bs.Arms[0].Failures != 20 {
		t.Errorf("control expected 20 failures, got %d", bs.Arms[0].Failures)
	}

	if bs.Arms[1].Successes != 60 {
		t.Errorf("variant expected 60 successes, got %d", bs.Arms[1].Successes)
	}
	if bs.Arms[1].Failures != 40 {
		t.Errorf("variant expected 40 failures, got %d", bs.Arms[1].Failures)
	}

	// Verify convergence: with this stark difference, bandit should converge with high confidence
	// that control is better
	isConverged := bs.IsConverged(0.95) // 95% confidence threshold
	if !isConverged {
		t.Error("bandit should have converged with 95% confidence after 100 iterations with clear winner")
	}

	// Verify the bandit consistently selects the control (better) arm
	selected := bs.SelectVariant("")
	if selected != "control" {
		t.Errorf("bandit should select control (better arm), got %q", selected)
	}
}

// TestEndToEndVerification_ExperimentsCommandExecutesAndOutputsTextReport verifies that
// the experiments command executes and outputs a formatted text report with experiment
// ID and variant details when given experiment definitions and state files.
func TestEndToEndVerification_ExperimentsCommandExecutesAndOutputsTextReport(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	experimentsDir := filepath.Join(gromitDir, "experiments")
	stateDir := filepath.Join(gromitDir, "experiment-state")

	if err := os.MkdirAll(experimentsDir, 0o755); err != nil {
		t.Fatalf("failed to create experiments directory: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("failed to create state directory: %v", err)
	}

	// Create experiment YAML
	expYAML := `id: cli-test-exp
phase: build
description: CLI test experiment
created: 2026-02-25T00:00:00Z
control:
  id: control
  template: PROMPT_build.md
variants:
  - id: variant-v1
    template: PROMPT_build_v1.md
`
	expPath := filepath.Join(experimentsDir, "cli-test-exp.yaml")
	if err := os.WriteFile(expPath, []byte(expYAML), 0o644); err != nil {
		t.Fatalf("failed to write experiment YAML: %v", err)
	}

	// Create state file with bandit data
	stateData := map[string]interface{}{
		"arms": []interface{}{
			map[string]interface{}{
				"id":        "control",
				"successes": 40,
				"failures":  10,
				"samples":   []interface{}{},
			},
			map[string]interface{}{
				"id":        "variant-v1",
				"successes": 30,
				"failures":  20,
				"samples":   []interface{}{},
			},
		},
	}
	stateFile := filepath.Join(stateDir, "cli-test-exp.json")
	stateBytes, err := json.MarshalIndent(stateData, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal state data: %v", err)
	}
	if err := os.WriteFile(stateFile, stateBytes, 0o644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	// Create config file
	configFile := filepath.Join(tmpDir, "gromit.yaml")
	configContent := fmt.Sprintf(`paths:
  gromit_dir: %s
experiment:
  experiments_dir: %s
`, gromitDir, experimentsDir)
	if err := os.WriteFile(configFile, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Change to tmpDir and set config path
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(origDir)
	})

	restore := configPath
	defer func() { configPath = restore }()
	configPath = configFile

	// Execute experiments command and capture output
	output := captureExperimentsStdout(t, func() {
		rootCmd.SetArgs([]string{"experiments"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("experiments command failed: %v", err)
		}
	})

	// Verify output contains experiment ID
	if !strings.Contains(output, "cli-test-exp") {
		t.Errorf("expected output to contain 'cli-test-exp', got: %q", output)
	}

	// Verify output contains variant IDs
	if !strings.Contains(output, "control") {
		t.Errorf("expected output to contain 'control', got: %q", output)
	}
	if !strings.Contains(output, "variant-v1") {
		t.Errorf("expected output to contain 'variant-v1', got: %q", output)
	}
}
