//go:build acceptance

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/validate"
)

// TestDecompose_SkipValidationFlag verifies --skip-validation flag bypasses all validation checks
func TestDecompose_SkipValidationFlag(t *testing.T) {
	// Expected failure: --skip-validation flag does not exist in decompose command
	// Expected failure: validation integration in decomposeSinglePlan does not exist

	// Setup test environment
	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)

	// Create a plan
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), `# Test Plan
Implement feature with beads that would violate sizing rules`)

	// Mock Claude returns beads with violations (4 criteria)
	mockClaude := &mockDecomposeClaudeClient{
		runCount: 0,
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Oversized bead",
					"description": "Too many criteria",
					"priority": "P1",
					"acceptance_criteria": [
						"Criterion 1",
						"Criterion 2",
						"Criterion 3",
						"Criterion 4"
					],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	var capturedBeads []string
	mockBead := &mockDecomposeBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			capturedBeads = append(capturedBeads, title)
			return map[string]interface{}{"id": "bead-1", "title": title}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	// Override global variables for test
	oldSkipValidation := decomposeSkipValidation
	defer func() { decomposeSkipValidation = oldSkipValidation }()
	decomposeSkipValidation = true // Simulates --skip-validation flag

	// Execute decompose with skip-validation
	err := decomposeSinglePlanWithMocks(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan with --skip-validation failed: %v", err)
	}

	// Verify Claude was called only ONCE (no reprompt/retry)
	if mockClaude.runCount != 1 {
		t.Errorf("Claude invocations = %d, want 1 (no validation retry with --skip-validation)", mockClaude.runCount)
	}

	// Verify bead was created despite violations
	if len(capturedBeads) != 1 {
		t.Errorf("Created beads count = %d, want 1", len(capturedBeads))
	}
	if capturedBeads[0] != "Oversized bead" {
		t.Errorf("Created bead title = %q, want 'Oversized bead'", capturedBeads[0])
	}
}

// TestDecompose_ValidationLoopAfterJSONParse verifies validation runs after JSON parsing before bead creation
func TestDecompose_ValidationLoopAfterJSONParse(t *testing.T) {
	// Expected failure: validate.CheckBeads is not called in decomposeSinglePlan after JSON parsing
	// Expected failure: validation loop with retry logic does not exist

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	// Track execution order
	var executionOrder []string

	mockClaude := &mockDecomposeClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			executionOrder = append(executionOrder, "claude_invocation")
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Valid bead",
					"description": "Test",
					"priority": "P1",
					"acceptance_criteria": ["Criterion 1"],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	mockBead := &mockDecomposeBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			executionOrder = append(executionOrder, "bead_creation")
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	err := decomposeSinglePlanWithMocks(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}

	// Verify execution order: claude -> validation (implicit) -> bead creation
	// Since validation is implicit between JSON parse and bead creation, we verify
	// that bead creation happens after claude invocation (validation in between)
	if len(executionOrder) < 2 {
		t.Fatalf("Execution order = %v, want at least 2 steps", executionOrder)
	}
	if executionOrder[0] != "claude_invocation" {
		t.Errorf("First step = %q, want 'claude_invocation'", executionOrder[0])
	}
	if executionOrder[len(executionOrder)-1] != "bead_creation" {
		t.Errorf("Last step = %q, want 'bead_creation'", executionOrder[len(executionOrder)-1])
	}
}

// TestDecompose_ViolationsTriggersRepromptAndRetry verifies violations trigger validate.BuildReprompt and re-invoke Claude
func TestDecompose_ViolationsTriggersRepromptAndRetry(t *testing.T) {
	// Expected failure: validate.CheckBeads call does not exist in decomposeSinglePlan
	// Expected failure: validate.BuildReprompt call and retry loop do not exist

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	invocationCount := 0
	mockClaude := &mockDecomposeClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			invocationCount++

			// First invocation: return beads with violations
			if invocationCount == 1 {
				return map[string]interface{}{
					"Success":  true,
					"ExitCode": 0,
					"Output": `[{
						"title": "Oversized bead",
						"description": "Refactor entire system",
						"priority": "P1",
						"acceptance_criteria": [
							"Criterion 1",
							"Criterion 2",
							"Criterion 3",
							"Criterion 4"
						],
						"depends_on_index": []
					}]`,
				}, nil
			}

			// Second invocation: verify prompt contains violation feedback
			if !strings.Contains(prompt, "criteria_count") && !strings.Contains(prompt, "scope_signals") {
				t.Errorf("Reprompt missing violation details (criteria_count or scope_signals)")
			}
			if !strings.Contains(prompt, "Oversized bead") {
				t.Errorf("Reprompt missing flagged bead title")
			}

			// Return fixed beads
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Fixed bead 1",
					"description": "Split properly",
					"priority": "P1",
					"acceptance_criteria": ["Criterion 1"],
					"depends_on_index": []
				},
				{
					"title": "Fixed bead 2",
					"description": "Split properly",
					"priority": "P1",
					"acceptance_criteria": ["Criterion 2"],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	var capturedBeads []string
	mockBead := &mockDecomposeBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			capturedBeads = append(capturedBeads, title)
			return map[string]interface{}{"id": fmt.Sprintf("bead-%d", len(capturedBeads))}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	err := decomposeSinglePlanWithMocks(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}

	// Verify retry happened (2 invocations total)
	if invocationCount != 2 {
		t.Errorf("Claude invocations = %d, want 2 (initial + 1 retry)", invocationCount)
	}

	// Verify fixed beads were created (not the original violating bead)
	if len(capturedBeads) != 2 {
		t.Errorf("Created beads count = %d, want 2", len(capturedBeads))
	}
	hasFixedBead1 := false
	hasFixedBead2 := false
	for _, bead := range capturedBeads {
		if bead == "Fixed bead 1" {
			hasFixedBead1 = true
		}
		if bead == "Fixed bead 2" {
			hasFixedBead2 = true
		}
	}
	if !hasFixedBead1 || !hasFixedBead2 {
		t.Errorf("Created beads = %v, want fixed beads not original oversized bead", capturedBeads)
	}
}

// TestDecompose_MaxValidationRetriesEnforced verifies retry loop respects MaxValidationRetries limit
func TestDecompose_MaxValidationRetriesEnforced(t *testing.T) {
	// Expected failure: MaxValidationRetries config is not used in decompose validation loop
	// Expected failure: retry counter and limit check do not exist

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	invocationCount := 0
	mockClaude := &mockDecomposeClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			invocationCount++
			// Always return beads with violations
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Persistently oversized bead",
					"description": "Refactor entire codebase",
					"priority": "P1",
					"acceptance_criteria": [
						"Criterion 1",
						"Criterion 2",
						"Criterion 3",
						"Criterion 4",
						"Criterion 5"
					],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	var capturedBeads []string
	mockBead := &mockDecomposeBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			capturedBeads = append(capturedBeads, title)
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	// Set MaxValidationRetries to 2
	cfg.Validation.MaxValidationRetries = 2

	err := decomposeSinglePlanWithMocks(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}

	// Verify Claude was called at most MaxValidationRetries+1 times (initial + 2 retries)
	if invocationCount > 3 {
		t.Errorf("Claude invocations = %d, want <= 3 (initial + MaxValidationRetries)", invocationCount)
	}

	// Verify beads were created despite persistent violations (warning path)
	if len(capturedBeads) != 1 {
		t.Errorf("Created beads count = %d, want 1 (created with warning after max retries)", len(capturedBeads))
	}
	if capturedBeads[0] != "Persistently oversized bead" {
		t.Errorf("Created bead = %q, want 'Persistently oversized bead' (original violated bead)", capturedBeads[0])
	}
}

// TestDecompose_ViolationsLoggedDuringRetry verifies violations are logged during retry attempts
func TestDecompose_ViolationsLoggedDuringRetry(t *testing.T) {
	// Expected failure: violation logging does not exist in decompose validation loop
	// Expected failure: log output with violation details is not generated

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	// Capture log output (in practice, this would check actual log files or stderr)
	var logOutput strings.Builder

	mockClaude := &mockDecomposeClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Oversized",
					"description": "Refactor entire",
					"priority": "P1",
					"acceptance_criteria": ["A", "B", "C", "D"],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	mockBead := &mockDecomposeBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Validation.MaxValidationRetries = 1

	// Execute with log capture
	err := decomposeSinglePlanWithMocksAndLogCapture(t, tmpDir, "test-plan", cfg, mockClaude, mockBead, &logOutput)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}

	// Verify log contains violation details
	logs := logOutput.String()
	if !strings.Contains(logs, "violation") && !strings.Contains(logs, "criteria_count") && !strings.Contains(logs, "scope_signals") {
		t.Errorf("Log output missing violation details:\n%s", logs)
	}
	if !strings.Contains(logs, "retry") && !strings.Contains(logs, "attempt") {
		t.Errorf("Log output missing retry indication:\n%s", logs)
	}
}

// TestDecompose_PersistentViolationsProceedWithWarning verifies that after max retries, beads are created with warning
func TestDecompose_PersistentViolationsProceedWithWarning(t *testing.T) {
	// Expected failure: warning log for persistent violations does not exist
	// Expected failure: proceed-with-warning path after max retries is not implemented

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	var logOutput strings.Builder

	mockClaude := &mockDecomposeClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			// Always return violations
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Stubborn oversized bead",
					"description": "Update all packages",
					"priority": "P1",
					"acceptance_criteria": ["A", "B", "C", "D", "E"],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	var capturedBeads []string
	mockBead := &mockDecomposeBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			capturedBeads = append(capturedBeads, title)
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Validation.MaxValidationRetries = 1

	err := decomposeSinglePlanWithMocksAndLogCapture(t, tmpDir, "test-plan", cfg, mockClaude, mockBead, &logOutput)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}

	// Verify beads were created despite violations
	if len(capturedBeads) != 1 {
		t.Errorf("Created beads count = %d, want 1 (proceed with warning)", len(capturedBeads))
	}

	// Verify warning was logged
	logs := logOutput.String()
	if !strings.Contains(logs, "warning") && !strings.Contains(logs, "proceeding") {
		t.Errorf("Log output missing warning about proceeding with violations:\n%s", logs)
	}
}

// TestDecompose_ValidationConvertBeadDefToCandidate verifies beadDef to BeadCandidate conversion
func TestDecompose_ValidationConvertBeadDefToCandidate(t *testing.T) {
	// Expected failure: conversion function from beadDef to validate.BeadCandidate does not exist
	// Expected failure: mapping logic between CLI beadDef struct and validate.BeadCandidate is missing

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	// Track what gets validated
	var validatedCandidates []validate.BeadCandidate

	mockClaude := &mockDecomposeClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Test bead with specific title",
					"description": "Test description with specific content",
					"priority": "P1",
					"acceptance_criteria": [
						"Specific criterion 1",
						"Specific criterion 2"
					],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	// Note: In actual implementation, we would inject a validator that captures candidates
	// This test verifies the conversion happens by checking the validation was called correctly
	// Expected failure: injection mechanism for validate.CheckBeads does not exist
	_ = validatedCandidates // Will be populated by validation call

	mockBead := &mockDecomposeBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	err := decomposeSinglePlanWithMocks(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}

	// Verify conversion produced correct BeadCandidate
	if len(validatedCandidates) != 1 {
		t.Fatalf("Validated candidates count = %d, want 1", len(validatedCandidates))
	}

	candidate := validatedCandidates[0]
	if candidate.Title != "Test bead with specific title" {
		t.Errorf("Candidate.Title = %q, want 'Test bead with specific title'", candidate.Title)
	}
	if candidate.Description != "Test description with specific content" {
		t.Errorf("Candidate.Description = %q, want 'Test description with specific content'", candidate.Description)
	}
	if len(candidate.AcceptanceCriteria) != 2 {
		t.Errorf("Candidate.AcceptanceCriteria length = %d, want 2", len(candidate.AcceptanceCriteria))
	}
	if candidate.AcceptanceCriteria[0] != "Specific criterion 1" {
		t.Errorf("Candidate.AcceptanceCriteria[0] = %q, want 'Specific criterion 1'", candidate.AcceptanceCriteria[0])
	}
}

// TestDecompose_ValidationDisabledByConfig verifies validation can be disabled via config
func TestDecompose_ValidationDisabledByConfig(t *testing.T) {
	// Expected failure: config-based validation disable does not exist in decompose
	// Expected failure: cfg.Validation.Enabled check is not implemented in decompose path

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	mockClaude := &mockDecomposeClaudeClient{
		runCount: 0,
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Oversized",
					"description": "Refactor entire",
					"priority": "P1",
					"acceptance_criteria": ["A", "B", "C", "D"],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	mockBead := &mockDecomposeBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	// Disable validation via config
	cfg.Validation.Enabled = false

	err := decomposeSinglePlanWithMocks(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}

	// Verify Claude was called only once (no retry)
	if mockClaude.runCount != 1 {
		t.Errorf("Claude invocations = %d, want 1 (validation disabled)", mockClaude.runCount)
	}
}

// Helper functions and mocks

type mockDecomposeClaudeClient struct {
	runFunc  func(prompt string, model string) (interface{}, error)
	runCount int
}

func (m *mockDecomposeClaudeClient) Run(prompt string, model string) (interface{}, error) {
	m.runCount++
	if m.runFunc != nil {
		return m.runFunc(prompt, model)
	}
	return nil, fmt.Errorf("not implemented")
}

type mockDecomposeBeadClient struct {
	createFunc func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error)
}

func (m *mockDecomposeBeadClient) Ready() (interface{}, error) {
	return nil, nil
}

func (m *mockDecomposeBeadClient) Show(id string) (interface{}, error) {
	return nil, nil
}

func (m *mockDecomposeBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	return nil, fmt.Errorf("use CreateWithDepsAndDescription")
}

func (m *mockDecomposeBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
	if m.createFunc != nil {
		return m.createFunc(title, priority, labels, criteria, deps, desc)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockDecomposeBeadClient) Close(id string) error {
	return nil
}

func setupDecomposeTestEnv(t *testing.T, tmpDir string) {
	gromitDir := filepath.Join(tmpDir, ".gromit")
	dirs := []string{
		filepath.Join(gromitDir, "plans"),
		filepath.Join(gromitDir, "templates"),
		filepath.Join(gromitDir, "specs"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("creating directory %s: %v", dir, err)
		}
	}

	// Create minimal config
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	cfgContent := `paths:
  gromit_dir: .gromit
  plans: .gromit/plans
validation:
  enabled: true
  max_validation_retries: 2
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("creating config: %v", err)
	}
}

func createTestPlan(t *testing.T, path string, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("creating plan file: %v", err)
	}
}

// decomposeSinglePlanWithMocks is a test helper that calls decomposeSinglePlan with injected dependencies
// Expected failure: decomposeSinglePlan does not accept injected dependencies for testing
func decomposeSinglePlanWithMocks(t *testing.T, tmpDir string, planName string, cfg *config.Config, claudeClient pipeline.ClaudeClient, beadClient pipeline.BeadClient) error {
	// In the actual implementation, decomposeSinglePlan would need to accept injected deps
	// For now, this signature demonstrates what the test expects
	return fmt.Errorf("decomposeSinglePlanWithMocks not yet implemented - requires dependency injection in decomposeSinglePlan")
}

// decomposeSinglePlanWithMocksAndLogCapture is like decomposeSinglePlanWithMocks but also captures log output
// Expected failure: log capture mechanism does not exist in decomposeSinglePlan
func decomposeSinglePlanWithMocksAndLogCapture(t *testing.T, tmpDir string, planName string, cfg *config.Config, claudeClient pipeline.ClaudeClient, beadClient pipeline.BeadClient, logOutput *strings.Builder) error {
	return fmt.Errorf("decomposeSinglePlanWithMocksAndLogCapture not yet implemented")
}

// Global variable for --skip-validation flag (referenced in tests)
// Expected failure: decomposeSkipValidation variable does not exist
var decomposeSkipValidation bool
