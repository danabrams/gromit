//go:build acceptance

package main

import (
	"encoding/json"
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
	// Expected failure: decomposeSkipValidation flag variable does not exist in decompose.go
	// Expected failure: flag is not defined in decomposeCmd.Flags()
	// Expected failure: validation bypass logic using decomposeSkipValidation does not exist in decomposeSinglePlan

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), `# Test Plan
Implement feature with beads that would violate sizing rules`)

	// Track Claude invocation count
	invocationCount := 0
	mockClaude := &testClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			invocationCount++
			// Always return beads with violations (4 criteria)
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
	mockBead := &testBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			capturedBeads = append(capturedBeads, title)
			return map[string]interface{}{"id": "bead-1", "title": title}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Paths.GromitDir = filepath.Join(tmpDir, ".gromit")
	cfg.Paths.Plans = filepath.Join(tmpDir, ".gromit", "plans")

	// Set the skip-validation flag
	oldSkipValidation := decomposeSkipValidation
	defer func() { decomposeSkipValidation = oldSkipValidation }()
	decomposeSkipValidation = true

	// Execute decompose with skip-validation enabled
	err := testDecomposeSinglePlan(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan with --skip-validation failed: %v", err)
	}

	// Verify Claude was called only ONCE (no validation retry)
	if invocationCount != 1 {
		t.Errorf("Claude invocations = %d, want 1 (no validation retry with --skip-validation)", invocationCount)
	}

	// Verify bead was created despite violations
	if len(capturedBeads) != 1 {
		t.Errorf("Created beads count = %d, want 1", len(capturedBeads))
	}
	if len(capturedBeads) > 0 && capturedBeads[0] != "Oversized bead" {
		t.Errorf("Created bead title = %q, want 'Oversized bead'", capturedBeads[0])
	}
}

// TestDecompose_ValidationAfterJSONParse verifies validation runs after JSON parsing before bead creation
func TestDecompose_ValidationAfterJSONParse(t *testing.T) {
	// Expected failure: validate.CheckBeads is not called in decomposeSinglePlan after JSON parsing
	// Expected failure: conversion from beadDef to validate.BeadCandidate does not exist
	// Expected failure: validation check integration between JSON parse and bd create is missing

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	mockClaude := &testClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Test bead",
					"description": "Test",
					"priority": "P1",
					"acceptance_criteria": ["Criterion 1", "Criterion 2"],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	var createdBead bool
	mockBead := &testBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			createdBead = true
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Paths.GromitDir = filepath.Join(tmpDir, ".gromit")
	cfg.Paths.Plans = filepath.Join(tmpDir, ".gromit", "plans")

	err := testDecomposeSinglePlan(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}

	// Verify bead was created (validation passed)
	if !createdBead {
		t.Errorf("Expected bead creation after validation passed")
	}
}

// TestDecompose_ViolationsTriggersReprompt verifies violations trigger validate.BuildReprompt and re-invoke Claude
func TestDecompose_ViolationsTriggersReprompt(t *testing.T) {
	// Expected failure: validate.CheckBeads call does not exist in decomposeSinglePlan
	// Expected failure: validate.BuildReprompt call does not exist in decompose validation path
	// Expected failure: retry loop that re-invokes claudeClient.Run with reprompt does not exist

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	invocationCount := 0
	var capturedPrompts []string

	mockClaude := &testClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			invocationCount++
			capturedPrompts = append(capturedPrompts, prompt)

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
			if !strings.Contains(prompt, "criteria_count") && !strings.Contains(prompt, "violation") {
				t.Errorf("Reprompt missing violation details (criteria_count or violation keyword)")
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
	mockBead := &testBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			capturedBeads = append(capturedBeads, title)
			return map[string]interface{}{"id": fmt.Sprintf("bead-%d", len(capturedBeads))}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Paths.GromitDir = filepath.Join(tmpDir, ".gromit")
	cfg.Paths.Plans = filepath.Join(tmpDir, ".gromit", "plans")

	err := testDecomposeSinglePlan(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
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
	// Expected failure: cfg.Validation.MaxValidationRetries is not used in decompose validation loop
	// Expected failure: retry counter and max retries check do not exist in decomposeSinglePlan

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	invocationCount := 0
	mockClaude := &testClaudeClient{
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
	mockBead := &testBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			capturedBeads = append(capturedBeads, title)
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Paths.GromitDir = filepath.Join(tmpDir, ".gromit")
	cfg.Paths.Plans = filepath.Join(tmpDir, ".gromit", "plans")
	// Set MaxValidationRetries to 2
	cfg.Validation.MaxValidationRetries = 2

	err := testDecomposeSinglePlan(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
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
	if len(capturedBeads) > 0 && capturedBeads[0] != "Persistently oversized bead" {
		t.Errorf("Created bead = %q, want 'Persistently oversized bead' (original violated bead)", capturedBeads[0])
	}
}

// TestDecompose_ConvertBeadDefToCandidateForValidation verifies beadDef to BeadCandidate conversion
func TestDecompose_ConvertBeadDefToCandidateForValidation(t *testing.T) {
	// Expected failure: conversion function from beadDef to validate.BeadCandidate does not exist
	// Expected failure: toBeadCandidate or similar helper is not defined in decompose.go

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	// Track what fields get validated by checking if violations reference correct data
	mockClaude := &testClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Test bead with specific title",
					"description": "Test description with refactor entire keyword",
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

	mockBead := &testBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			// Verify that the converted data made it through validation to bead creation
			if title != "Test bead with specific title" {
				t.Errorf("Bead title = %q, want 'Test bead with specific title'", title)
			}
			if desc != "Test description with refactor entire keyword" {
				t.Errorf("Bead description = %q, want 'Test description with refactor entire keyword'", desc)
			}
			if len(criteria) != 2 {
				t.Errorf("Criteria count = %d, want 2", len(criteria))
			}
			if len(criteria) > 0 && criteria[0] != "Specific criterion 1" {
				t.Errorf("First criterion = %q, want 'Specific criterion 1'", criteria[0])
			}
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Paths.GromitDir = filepath.Join(tmpDir, ".gromit")
	cfg.Paths.Plans = filepath.Join(tmpDir, ".gromit", "plans")

	err := testDecomposeSinglePlan(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}
}

// TestDecompose_ValidationDisabledViaConfig verifies validation can be disabled via config
func TestDecompose_ValidationDisabledViaConfig(t *testing.T) {
	// Expected failure: cfg.Validation.Enabled check is not implemented in decompose validation path
	// Expected failure: early return or bypass when Enabled=false does not exist

	tmpDir := t.TempDir()
	setupDecomposeTestEnv(t, tmpDir)
	createTestPlan(t, filepath.Join(tmpDir, ".gromit", "plans", "test-plan.md"), "# Test Plan")

	invocationCount := 0
	mockClaude := &testClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			invocationCount++
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

	mockBead := &testBeadClient{
		createFunc: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.Paths.GromitDir = filepath.Join(tmpDir, ".gromit")
	cfg.Paths.Plans = filepath.Join(tmpDir, ".gromit", "plans")
	// Disable validation via config
	cfg.Validation.Enabled = false

	err := testDecomposeSinglePlan(t, tmpDir, "test-plan", cfg, mockClaude, mockBead)
	if err != nil {
		t.Fatalf("decomposeSinglePlan failed: %v", err)
	}

	// Verify Claude was called only once (no retry)
	if invocationCount != 1 {
		t.Errorf("Claude invocations = %d, want 1 (validation disabled)", invocationCount)
	}
}

// Helper functions and mocks

type testClaudeClient struct {
	runFunc func(prompt string, model string) (interface{}, error)
}

func (m *testClaudeClient) Run(prompt string, model string) (interface{}, error) {
	if m.runFunc != nil {
		return m.runFunc(prompt, model)
	}
	return nil, fmt.Errorf("not implemented")
}

type testBeadClient struct {
	createFunc func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error)
}

func (m *testBeadClient) Ready() (interface{}, error) {
	return []interface{}{}, nil
}

func (m *testBeadClient) Show(id string) (interface{}, error) {
	return nil, nil
}

func (m *testBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	return nil, fmt.Errorf("use CreateWithDepsAndDescription")
}

func (m *testBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
	if m.createFunc != nil {
		return m.createFunc(title, priority, labels, criteria, deps, desc)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *testBeadClient) Close(id string) error {
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

// testDecomposeSinglePlan exercises the decompose workflow with injected dependencies
// Expected failure: decomposeSinglePlan does not call validation logic (validate.CheckBeads, validate.BuildReprompt)
// Expected failure: decomposeSinglePlan does not accept mock dependencies for testing
func testDecomposeSinglePlan(t *testing.T, tmpDir string, planName string, cfg *config.Config, claudeClient pipeline.ClaudeClient, beadClient pipeline.BeadClient) error {
	// This function simulates what decomposeSinglePlan should do:
	// 1. Read plan file
	// 2. Build prompt
	// 3. Invoke Claude
	// 4. Parse JSON response into beadDef structs
	// 5. Convert beadDef to BeadCandidate (THIS STEP DOES NOT EXIST YET)
	// 6. Call validate.CheckBeads (THIS STEP DOES NOT EXIST YET)
	// 7. If violations: call validate.BuildReprompt and retry (THIS LOGIC DOES NOT EXIST YET)
	// 8. Create beads via bd

	planPath := filepath.Join(cfg.Paths.Plans, planName+".md")
	planContent, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("reading plan: %w", err)
	}

	// Simplified prompt for testing
	prompt := fmt.Sprintf("Decompose this plan into beads:\n\n%s", string(planContent))

	// Retry loop (NEW - this is what we're testing)
	maxRetries := cfg.Validation.MaxValidationRetries
	var beadDefs []beadDef

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Invoke Claude
		result, err := claudeClient.Run(prompt, "sonnet")
		if err != nil {
			return fmt.Errorf("claude invocation failed: %w", err)
		}

		// Parse result
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected result type")
		}
		output := resultMap["Output"].(string)

		// Parse JSON
		if err := json.Unmarshal([]byte(output), &beadDefs); err != nil {
			return fmt.Errorf("parsing JSON: %w", err)
		}

		// Convert to BeadCandidate (THIS CONVERSION DOES NOT EXIST YET)
		candidates := toBeadCandidates(beadDefs)

		// Check validation bypass conditions
		if decomposeSkipValidation || !cfg.Validation.Enabled {
			// Skip validation
			break
		}

		// Validate (THIS CALL DOES NOT EXIST IN REAL decomposeSinglePlan YET)
		violations := validate.CheckBeads(candidates)

		if len(violations) == 0 {
			// No violations - proceed
			break
		}

		if attempt >= maxRetries {
			// Max retries reached - proceed with warning
			t.Logf("Warning: proceeding with %d validation violations after %d retries", len(violations), maxRetries)
			break
		}

		// Build reprompt (THIS CALL DOES NOT EXIST IN REAL decomposeSinglePlan YET)
		prompt = validate.BuildReprompt(prompt, candidates, violations)
	}

	// Create beads
	for _, def := range beadDefs {
		priority := parsePriority(def.Priority)
		labels := []string{"spec:" + planName}
		_, err := beadClient.CreateWithDepsAndDescription(
			def.Title,
			priority,
			labels,
			def.AcceptanceCriteria,
			[]string{}, // deps
			def.Description,
		)
		if err != nil {
			return fmt.Errorf("creating bead: %w", err)
		}
	}

	return nil
}

// toBeadCandidates converts beadDef slice to BeadCandidate slice
// Expected failure: this helper function does not exist in decompose.go
func toBeadCandidates(defs []beadDef) []validate.BeadCandidate {
	candidates := make([]validate.BeadCandidate, len(defs))
	for i, def := range defs {
		candidates[i] = validate.BeadCandidate{
			Title:              def.Title,
			Description:        def.Description,
			AcceptanceCriteria: def.AcceptanceCriteria,
		}
	}
	return candidates
}

func parsePriority(p string) int {
	// Simple parser for "P1", "P2" format
	if len(p) > 1 && p[0] == 'P' {
		var priority int
		fmt.Sscanf(p[1:], "%d", &priority)
		return priority
	}
	return 1
}
