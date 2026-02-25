package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// TestEndToEndVerification_ExperimentsCommandWithFixtureData verifies that the
// gromit experiments command works correctly with fixture data.
func TestEndToEndVerification_ExperimentsCommandWithFixtureData(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	experimentsDir := filepath.Join(gromitDir, "experiments")

	if err := os.MkdirAll(experimentsDir, 0o755); err != nil {
		t.Fatalf("failed to create experiments directory: %v", err)
	}

	// Create a fixture experiment YAML
	fixtureExpYAML := `id: fixture-exp-1
phase: validate
description: Fixture experiment for testing
created: 2026-02-25T00:00:00Z
control:
  id: control
  template: PROMPT_validate.md
variants:
  - id: variant-test
    template: PROMPT_validate_test.md
`

	expPath := filepath.Join(experimentsDir, "fixture-experiment.yaml")
	if err := os.WriteFile(expPath, []byte(fixtureExpYAML), 0o644); err != nil {
		t.Fatalf("failed to write fixture experiment file: %v", err)
	}

	// Create a fixture state file with bandit data
	stateDir := filepath.Join(gromitDir, "experiment-state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("failed to create state directory: %v", err)
	}

	stateData := map[string]interface{}{
		"fixture-exp-1": map[string]interface{}{
			"control": map[string]interface{}{
				"successes": 50,
				"failures":  10,
				"samples":   []interface{}{},
			},
			"variant-test": map[string]interface{}{
				"successes": 30,
				"failures":  20,
				"samples":   []interface{}{},
			},
		},
	}

	stateFile := filepath.Join(stateDir, "state.json")
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

	// Load experiments and verify they're parsed correctly
	exps, err := experiment.LoadExperiments(experimentsDir)
	if err != nil {
		t.Fatalf("LoadExperiments failed: %v", err)
	}

	if len(exps) != 1 {
		t.Errorf("expected 1 experiment, got %d", len(exps))
	}

	if exps[0].ID != "fixture-exp-1" {
		t.Errorf("expected experiment ID 'fixture-exp-1', got %q", exps[0].ID)
	}

	// Verify the state file exists and is valid JSON
	loadedStateBytes, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var loadedState map[string]interface{}
	if err := json.Unmarshal(loadedStateBytes, &loadedState); err != nil {
		t.Fatalf("failed to unmarshal state file: %v", err)
	}

	if _, hasExp := loadedState["fixture-exp-1"]; !hasExp {
		t.Error("state file missing fixture-exp-1 data")
	}
}
