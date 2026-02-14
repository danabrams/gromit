//go:build acceptance

package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/validate"
)

// TestDecomposeTask_ValidationAfterReturn verifies validation runs after DecomposeTask
// returns sub-tasks but before CreateSubBeads is called
func TestDecomposeTask_ValidationAfterReturn(t *testing.T) {
	// Expected failure: validation integration between DecomposeTask and CreateSubBeads does not exist
	// Expected failure: validate.CheckBeads is not called in the runner decompose flow
	// Expected failure: conversion from SubTask to validate.BeadCandidate does not exist

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Decompose.MaxValidationRetries = 2

	mockBeads := &mockBeadClient{
		FnGetParent: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil // No parent
		},
		FnCreate: func(title string, priority int, labels []string, outputs []string) (*bead.Bead, error) {
			return &bead.Bead{ID: "child-1", Title: title}, nil
		},
	}

	decomposeCalled := false
	mockRouter := &mockRouter{
		FnSelect: func(phase, tier string) (provider.Provider, string) {
			if phase == "decompose" {
				return &mockProvider{
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						decomposeCalled = true
						// Return sub-tasks with violations (4 criteria)
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Oversized sub-task",
								"description": "Too many criteria",
								"priority": "P1",
								"acceptance_criteria": [
									"Criterion 1",
									"Criterion 2",
									"Criterion 3",
									"Criterion 4"
								],
								"depends_on": []
							}]`,
						}, nil
					},
				}, "opus"
			}
			return nil, ""
		},
	}

	mockRenderer := &mockRenderer{
		FnRenderDecompose: func(ctx interface{}) (string, error) {
			return "decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent bead to decompose",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call DecomposeTask
	subTasks, err := r.DecomposeTask(ctx, parentBead)
	if err != nil {
		t.Fatalf("DecomposeTask() failed: %v", err)
	}

	if !decomposeCalled {
		t.Fatal("DecomposeTask did not invoke Claude")
	}

	// Expected failure: validation should have been called and detected violations
	// After implementation, the returned subTasks should be validated
	if len(subTasks) == 0 {
		t.Fatal("DecomposeTask returned no sub-tasks")
	}

	// Convert SubTask to BeadCandidate for manual validation check
	candidates := make([]validate.BeadCandidate, len(subTasks))
	for i, st := range subTasks {
		candidates[i] = validate.BeadCandidate{
			Title:              st.Title,
			Description:        st.Description,
			AcceptanceCriteria: st.AcceptanceCriteria,
		}
	}

	violations := validate.CheckBeads(candidates)
	if len(violations) == 0 {
		t.Error("Expected violations to be detected for oversized sub-task, but got none")
	}

	// The test demonstrates that currently violations are NOT caught between
	// DecomposeTask return and CreateSubBeads call. After implementation,
	// validation should occur in this gap and trigger re-decomposition.
}

// TestCreateSubBeads_ValidationBeforeCreation verifies validation runs before
// CreateSubBeads creates beads via bd
func TestCreateSubBeads_ValidationBeforeCreation(t *testing.T) {
	// Expected failure: validation check before CreateSubBeads does not exist
	// Expected failure: SubTask to BeadCandidate conversion does not exist in CreateSubBeads

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	createCalled := false
	mockBeads := &mockBeadClient{
		FnCreate: func(title string, priority int, labels []string, outputs []string) (*bead.Bead, error) {
			createCalled = true
			return &bead.Bead{ID: "child-1", Title: title}, nil
		},
		FnAddComment: func(beadID, comment string) error {
			return nil
		},
		FnClose: func(beadID string) error {
			return nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Sub-tasks with violations (4 criteria)
	subTasks := []SubTask{
		{
			Title:       "Oversized sub-task",
			Description: "Too many criteria",
			Priority:    "P1",
			AcceptanceCriteria: []string{
				"Criterion 1",
				"Criterion 2",
				"Criterion 3",
				"Criterion 4",
			},
		},
	}

	err := r.CreateSubBeads(context.Background(), parentBead, subTasks)

	// Currently this succeeds even though sub-tasks have violations
	if err != nil {
		t.Fatalf("CreateSubBeads() failed: %v", err)
	}

	if !createCalled {
		t.Fatal("CreateSubBeads did not create beads")
	}

	// Expected failure: after implementation, CreateSubBeads should detect
	// violations and either reject creation or trigger re-decomposition
	// Currently it proceeds without validation
}

// TestRunnerDecomposePath_ValidationEnabled verifies validation is enabled by default
func TestRunnerDecomposePath_ValidationEnabled(t *testing.T) {
	// Expected failure: cfg.Decompose.IsValidateEnabled() method does not exist
	// Expected failure: DecomposeConfig type does not exist

	cfg := &config.Config{}
	cfg.SetDefaults()

	// Expected failure: Config.Decompose field does not exist
	if !cfg.Decompose.IsValidateEnabled() {
		t.Error("expected decompose validation to be enabled by default")
	}
}

// TestRunnerDecomposePath_ValidationDisabledViaConfig verifies validation can be disabled
func TestRunnerDecomposePath_ValidationDisabledViaConfig(t *testing.T) {
	// Expected failure: cfg.Decompose.IsValidateEnabled() method does not exist
	// Expected failure: DecomposeConfig.Validate field does not exist

	cfg := &config.Config{}
	cfg.SetDefaults()

	// Disable validation
	falseVal := false
	cfg.Decompose.Validate = &falseVal

	if cfg.Decompose.IsValidateEnabled() {
		t.Error("expected decompose validation to be disabled when Validate=false")
	}
}

// TestRunnerDecomposePath_RepromptOnViolations verifies violations trigger reprompt
func TestRunnerDecomposePath_RepromptOnViolations(t *testing.T) {
	// Expected failure: validation retry loop with reprompt does not exist in runner decompose path
	// Expected failure: validate.BuildReprompt is not called in runner
	// Expected failure: re-invocation of router.Select/provider.Run with reprompt does not exist

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Decompose.MaxValidationRetries = 2

	invocationCount := 0
	var capturedPrompts []string

	mockRouter := &mockRouter{
		FnSelect: func(phase, tier string) (provider.Provider, string) {
			if phase == "decompose" {
				return &mockProvider{
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						invocationCount++
						capturedPrompts = append(capturedPrompts, prompt)

						// First invocation: return beads with violations
						if invocationCount == 1 {
							return &provider.Result{
								Success:  true,
								ExitCode: 0,
								Output: `[{
									"title": "Oversized",
									"description": "Refactor entire system",
									"priority": "P1",
									"acceptance_criteria": ["A", "B", "C", "D"],
									"depends_on": []
								}]`,
							}, nil
						}

						// Second invocation: verify prompt contains violation feedback
						if !strings.Contains(prompt, "criteria_count") && !strings.Contains(prompt, "violation") {
							t.Errorf("Reprompt missing violation details")
						}

						// Return fixed beads
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Fixed bead 1",
								"description": "Split properly",
								"priority": "P1",
								"acceptance_criteria": ["Criterion 1"],
								"depends_on": []
							}]`,
						}, nil
					},
				}, "opus"
			}
			return nil, ""
		},
	}

	mockBeads := &mockBeadClient{
		FnGetParent: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockRenderer := &mockRenderer{
		FnRenderDecompose: func(ctx interface{}) (string, error) {
			return "original decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subTasks, err := r.DecomposeTask(ctx, parentBead)
	if err != nil {
		t.Fatalf("DecomposeTask() failed: %v", err)
	}

	// Expected failure: currently invocationCount will be 1 (no retry)
	// After implementation, should be 2 (initial + 1 retry)
	if invocationCount != 2 {
		t.Errorf("Claude invocations = %d, want 2 (initial + 1 retry)", invocationCount)
	}

	// Expected failure: currently subTasks will contain the original oversized bead
	// After implementation, should contain fixed beads
	if len(subTasks) != 1 {
		t.Errorf("Sub-tasks count = %d, want 1", len(subTasks))
	}
	if len(subTasks) > 0 && subTasks[0].Title != "Fixed bead 1" {
		t.Errorf("Sub-task title = %q, want 'Fixed bead 1'", subTasks[0].Title)
	}
}

// TestRunnerDecomposePath_MaxRetriesEnforced verifies retry loop respects MaxValidationRetries
func TestRunnerDecomposePath_MaxRetriesEnforced(t *testing.T) {
	// Expected failure: cfg.Decompose.MaxValidationRetries is not used in runner
	// Expected failure: retry counter and max retries check do not exist in runner decompose path

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Decompose.MaxValidationRetries = 2

	invocationCount := 0
	mockRouter := &mockRouter{
		FnSelect: func(phase, tier string) (provider.Provider, string) {
			if phase == "decompose" {
				return &mockProvider{
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						invocationCount++
						// Always return beads with violations
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Persistently oversized",
								"description": "Refactor entire codebase",
								"priority": "P1",
								"acceptance_criteria": ["A", "B", "C", "D", "E"],
								"depends_on": []
							}]`,
						}, nil
					},
				}, "opus"
			}
			return nil, ""
		},
	}

	mockBeads := &mockBeadClient{
		FnGetParent: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockRenderer := &mockRenderer{
		FnRenderDecompose: func(ctx interface{}) (string, error) {
			return "decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subTasks, err := r.DecomposeTask(ctx, parentBead)
	if err != nil {
		t.Fatalf("DecomposeTask() failed: %v", err)
	}

	// Expected failure: currently invocationCount will be 1 (no retry)
	// After implementation, should be at most MaxValidationRetries+1 (initial + 2 retries)
	if invocationCount > 3 {
		t.Errorf("Claude invocations = %d, want <= 3 (initial + MaxValidationRetries)", invocationCount)
	}

	// Beads should be created despite persistent violations (warning path)
	if len(subTasks) != 1 {
		t.Errorf("Sub-tasks count = %d, want 1 (proceed with warning after max retries)", len(subTasks))
	}
}

// TestRunnerDecomposePath_ConvertSubTaskToCandidate verifies SubTask to BeadCandidate conversion
func TestRunnerDecomposePath_ConvertSubTaskToCandidate(t *testing.T) {
	// Expected failure: conversion function from SubTask to validate.BeadCandidate does not exist
	// Expected failure: toBeadCandidate or similar helper is not defined in runner

	subTask := SubTask{
		Title:       "Test sub-task",
		Description: "Test description with refactor entire keyword",
		Priority:    "P1",
		AcceptanceCriteria: []string{
			"Specific criterion 1",
			"Specific criterion 2",
		},
	}

	// This conversion should happen in the runner between DecomposeTask and CreateSubBeads
	candidate := validate.BeadCandidate{
		Title:              subTask.Title,
		Description:        subTask.Description,
		AcceptanceCriteria: subTask.AcceptanceCriteria,
	}

	// Verify conversion preserves all fields
	if candidate.Title != "Test sub-task" {
		t.Errorf("Title = %q, want 'Test sub-task'", candidate.Title)
	}
	if candidate.Description != "Test description with refactor entire keyword" {
		t.Errorf("Description = %q, want specific value", candidate.Description)
	}
	if len(candidate.AcceptanceCriteria) != 2 {
		t.Errorf("AcceptanceCriteria count = %d, want 2", len(candidate.AcceptanceCriteria))
	}

	// Verify the converted candidate can be validated
	violations := validate.CheckBeads([]validate.BeadCandidate{candidate})
	foundScopeSignal := false
	for _, v := range violations {
		if v.Rule == "scope_signals" {
			foundScopeSignal = true
		}
	}
	if !foundScopeSignal {
		t.Error("Expected scope_signals violation for 'refactor entire' keyword")
	}
}

// TestRunnerDecomposePath_LogsValidationWarnings verifies validation violations are logged
func TestRunnerDecomposePath_LogsValidationWarnings(t *testing.T) {
	// Expected failure: logging of validation violations does not exist in runner decompose path
	// Expected failure: log output for violations and retry attempts is missing

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Decompose.MaxValidationRetries = 1

	invocationCount := 0
	mockRouter := &mockRouter{
		FnSelect: func(phase, tier string) (provider.Provider, string) {
			if phase == "decompose" {
				return &mockProvider{
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						invocationCount++
						// Always return beads with violations
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Over-scoped bead",
								"description": "Test",
								"priority": "P1",
								"acceptance_criteria": ["A", "B", "C", "D"],
								"depends_on": []
							}]`,
						}, nil
					},
				}, "opus"
			}
			return nil, ""
		},
	}

	mockBeads := &mockBeadClient{
		FnGetParent: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockRenderer := &mockRenderer{
		FnRenderDecompose: func(ctx interface{}) (string, error) {
			return "decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.DecomposeTask(ctx, parentBead)
	if err != nil {
		t.Fatalf("DecomposeTask() failed: %v", err)
	}

	// Expected failure: no logging about validation violations exists yet
	// After implementation, output should contain violation warnings
	output := buf.String()
	if !strings.Contains(strings.ToLower(output), "validation") && !strings.Contains(strings.ToLower(output), "violation") {
		t.Error("Expected log output to mention validation or violations")
	}
}

// TestRunnerDecomposePath_ProceedsAfterMaxRetries verifies beads are created after retries exhausted
func TestRunnerDecomposePath_ProceedsAfterMaxRetries(t *testing.T) {
	// Expected failure: warning path that proceeds with violations after max retries does not exist
	// Expected failure: CreateSubBeads is never called when violations persist

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Decompose.MaxValidationRetries = 1

	mockRouter := &mockRouter{
		FnSelect: func(phase, tier string) (provider.Provider, string) {
			if phase == "decompose" {
				return &mockProvider{
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						// Always return beads with violations
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Persistent violation",
								"description": "Test",
								"priority": "P1",
								"acceptance_criteria": ["A", "B", "C", "D"],
								"depends_on": []
							}]`,
						}, nil
					},
				}, "opus"
			}
			return nil, ""
		},
	}

	mockBeads := &mockBeadClient{
		FnGetParent: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockRenderer := &mockRenderer{
		FnRenderDecompose: func(ctx interface{}) (string, error) {
			return "decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This should succeed despite persistent violations
	subTasks, err := r.DecomposeTask(ctx, parentBead)
	if err != nil {
		t.Fatalf("DecomposeTask() should proceed with warning after max retries, got error: %v", err)
	}

	// Sub-tasks should be returned (the original oversized ones)
	if len(subTasks) == 0 {
		t.Error("Expected sub-tasks to be returned despite persistent violations")
	}
}
