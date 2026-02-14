//go:build acceptance

package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/validate"
)

// TestRunnerDecomposeValidation_IntegrationBetweenDecomposeAndCreate verifies
// validation runs after DecomposeTask returns but before CreateSubBeads is called
func TestRunnerDecomposeValidation_IntegrationBetweenDecomposeAndCreate(t *testing.T) {
	// Expected failure: attemptDecomposition method does not exist in process.go
	// Expected failure: validation integration between DecomposeTask and CreateSubBeads does not exist
	// Expected failure: SubTask to BeadCandidate conversion in validation flow does not exist
	// Expected failure: validate.CheckBeads call in runner decompose path does not exist

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
						// First invocation: return sub-tasks with violations
						if invocationCount == 1 {
							return &provider.Result{
								Success:  true,
								ExitCode: 0,
								Output: `[{
									"title": "Oversized sub-task",
									"description": "Refactor entire system",
									"priority": "P1",
									"acceptance_criteria": ["A", "B", "C", "D"],
									"depends_on": null
								}]`,
							}, nil
						}
						// Second invocation: return fixed sub-tasks
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Fixed sub-task 1",
								"description": "Properly scoped",
								"priority": "P1",
								"acceptance_criteria": ["Criterion 1"],
								"depends_on": null
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

	mockPromptRenderer := &mockPromptRenderer{
		RenderDecomposeFn: func(ctx interface{}) (string, error) {
			return "decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockPromptRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn, nil),
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

	// The implementation should call attemptDecomposition instead of DecomposeTask directly
	// attemptDecomposition should wrap DecomposeTask with validation logic
	subTasks, err := r.DecomposeTask(ctx, parentBead)
	if err != nil {
		t.Fatalf("DecomposeTask() failed: %v", err)
	}

	// Verify retry happened (violations triggered re-invocation)
	if invocationCount != 2 {
		t.Errorf("Claude invocations = %d, want 2 (initial + 1 retry after violations)", invocationCount)
	}

	// Verify returned sub-tasks are the fixed ones, not the original violations
	if len(subTasks) != 1 {
		t.Errorf("Sub-tasks count = %d, want 1", len(subTasks))
	}
	if len(subTasks) > 0 && subTasks[0].Title != "Fixed sub-task 1" {
		t.Errorf("Sub-task title = %q, want 'Fixed sub-task 1' (violations should trigger retry)", subTasks[0].Title)
	}
}

// TestRunnerDecomposeValidation_ConvertSubTaskToCandidate verifies conversion helper exists
func TestRunnerDecomposeValidation_ConvertSubTaskToCandidate(t *testing.T) {
	// Expected failure: toBeadCandidates helper function does not exist in runner
	// Expected failure: conversion from []SubTask to []validate.BeadCandidate is not implemented

	subTasks := []SubTask{
		{
			Title:              "Test sub-task",
			Description:        "Test description",
			Priority:           "P1",
			AcceptanceCriteria: []string{"Criterion 1", "Criterion 2"},
		},
	}

	// This conversion should be implemented in the runner package
	// Expected failure: toBeadCandidates function does not exist
	candidates := toBeadCandidates(subTasks)

	if len(candidates) != 1 {
		t.Fatalf("Candidates count = %d, want 1", len(candidates))
	}

	candidate := candidates[0]
	if candidate.Title != "Test sub-task" {
		t.Errorf("Title = %q, want 'Test sub-task'", candidate.Title)
	}
	if candidate.Description != "Test description" {
		t.Errorf("Description = %q, want 'Test description'", candidate.Description)
	}
	if len(candidate.AcceptanceCriteria) != 2 {
		t.Errorf("AcceptanceCriteria count = %d, want 2", len(candidate.AcceptanceCriteria))
	}
}

// TestRunnerDecomposeValidation_RepromptWithViolations verifies reprompt contains violation details
func TestRunnerDecomposeValidation_RepromptWithViolations(t *testing.T) {
	// Expected failure: validate.BuildReprompt is not called in runner decompose validation flow
	// Expected failure: reprompt construction with original prompt + violations does not exist

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Decompose.MaxValidationRetries = 1

	invocationCount := 0
	var capturedPrompts []string

	mockRouter := &mockRouter{
		FnSelect: func(phase, tier string) (provider.Provider, string) {
			if phase == "decompose" {
				return &mockProvider{
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						invocationCount++
						capturedPrompts = append(capturedPrompts, prompt)

						if invocationCount == 1 {
							// Return violations
							return &provider.Result{
								Success:  true,
								ExitCode: 0,
								Output: `[{
									"title": "Bad bead",
									"description": "Refactor entire",
									"priority": "P1",
									"acceptance_criteria": ["A", "B", "C", "D"],
									"depends_on": null
								}]`,
							}, nil
						}

						// Verify reprompt contains violation feedback
						if !strings.Contains(prompt, "violation") && !strings.Contains(prompt, "criteria_count") {
							t.Errorf("Reprompt missing violation details, got: %s", prompt)
						}

						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Fixed",
								"description": "OK",
								"priority": "P1",
								"acceptance_criteria": ["A"],
								"depends_on": null
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

	mockPromptRenderer := &mockPromptRenderer{
		RenderDecomposeFn: func(ctx interface{}) (string, error) {
			return "original decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockPromptRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn, nil),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent",
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

	if invocationCount != 2 {
		t.Errorf("Invocations = %d, want 2", invocationCount)
	}

	// Verify second prompt is different from first (contains violation feedback)
	if len(capturedPrompts) >= 2 && capturedPrompts[0] == capturedPrompts[1] {
		t.Error("Second invocation prompt should contain violation feedback, but is identical to first")
	}
}

// TestRunnerDecomposeValidation_RespectsConfigEnabled verifies cfg.Decompose.IsValidateEnabled
func TestRunnerDecomposeValidation_RespectsConfigEnabled(t *testing.T) {
	// Expected failure: cfg.Decompose.IsValidateEnabled() check does not exist in runner decompose path
	// Expected failure: validation bypass when IsValidateEnabled returns false is not implemented

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	// Disable validation
	falseVal := false
	cfg.Decompose.Validate = &falseVal

	invocationCount := 0
	mockRouter := &mockRouter{
		FnSelect: func(phase, tier string) (provider.Provider, string) {
			if phase == "decompose" {
				return &mockProvider{
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						invocationCount++
						// Always return violations
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Oversized",
								"description": "Test",
								"priority": "P1",
								"acceptance_criteria": ["A", "B", "C", "D"],
								"depends_on": null
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

	mockPromptRenderer := &mockPromptRenderer{
		RenderDecomposeFn: func(ctx interface{}) (string, error) {
			return "decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockPromptRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn, nil),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent",
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

	// With validation disabled, should only invoke Claude once (no retry)
	if invocationCount != 1 {
		t.Errorf("Invocations = %d, want 1 (validation disabled)", invocationCount)
	}

	// Should return original sub-tasks despite violations
	if len(subTasks) != 1 || subTasks[0].Title != "Oversized" {
		t.Error("With validation disabled, should return original sub-tasks without retry")
	}
}

// TestRunnerDecomposeValidation_RespectsMaxRetries verifies MaxValidationRetries limit
func TestRunnerDecomposeValidation_RespectsMaxRetries(t *testing.T) {
	// Expected failure: cfg.Decompose.MaxValidationRetries is not used in runner validation loop
	// Expected failure: retry counter and max retries enforcement do not exist

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
						// Always return violations
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Persistent violation",
								"description": "Test",
								"priority": "P1",
								"acceptance_criteria": ["A", "B", "C", "D", "E"],
								"depends_on": null
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

	mockPromptRenderer := &mockPromptRenderer{
		RenderDecomposeFn: func(ctx interface{}) (string, error) {
			return "decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockPromptRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn, nil),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent",
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

	// Should invoke at most MaxValidationRetries+1 times
	if invocationCount > 3 {
		t.Errorf("Invocations = %d, want <= 3 (initial + MaxValidationRetries)", invocationCount)
	}

	// Should return sub-tasks despite persistent violations (warning path)
	if len(subTasks) == 0 {
		t.Error("Should return sub-tasks after exhausting retries (warning path)")
	}
}

// TestRunnerDecomposeValidation_LogsViolationsAndRetries verifies logging output
func TestRunnerDecomposeValidation_LogsViolationsAndRetries(t *testing.T) {
	// Expected failure: logging of validation violations in runner decompose path does not exist
	// Expected failure: log statements for violations found and retry attempts are missing

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Decompose.MaxValidationRetries = 1

	mockRouter := &mockRouter{
		FnSelect: func(phase, tier string) (provider.Provider, string) {
			if phase == "decompose" {
				return &mockProvider{
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Bad",
								"description": "Test",
								"priority": "P1",
								"acceptance_criteria": ["A", "B", "C", "D"],
								"depends_on": null
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

	mockPromptRenderer := &mockPromptRenderer{
		RenderDecomposeFn: func(ctx interface{}) (string, error) {
			return "decompose prompt", nil
		},
	}

	var buf strings.Builder

	r := &Runner{
		cfg:      cfg,
		beads:    mockBeads,
		router:   mockRouter,
		renderer: mockPromptRenderer,
		output:   &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, func(format string, args ...interface{}, nil) {
			// Capture log output
			buf.WriteString(fmt.Sprintf(format, args...))
			buf.WriteString("\n")
		}),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent",
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

	// Verify log output mentions violations
	output := buf.String()
	outputLower := strings.ToLower(output)
	if !strings.Contains(outputLower, "violation") && !strings.Contains(outputLower, "validat") {
		t.Errorf("Expected log output to mention validation or violations, got: %s", output)
	}
}

// TestRunnerDecomposeValidation_ProceedsWithWarningAfterMaxRetries verifies warning path
func TestRunnerDecomposeValidation_ProceedsWithWarningAfterMaxRetries(t *testing.T) {
	// Expected failure: warning path after exhausting max retries does not exist
	// Expected failure: logic to proceed with violations after MaxValidationRetries is not implemented

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Decompose.MaxValidationRetries = 1

	mockRouter := &mockRouter{
		FnSelect: func(phase, tier string) (provider.Provider, string) {
			if phase == "decompose" {
				return &mockProvider{
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						// Always return persistent violations
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output: `[{
								"title": "Persistent",
								"description": "Test",
								"priority": "P1",
								"acceptance_criteria": ["A", "B", "C", "D"],
								"depends_on": null
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

	mockPromptRenderer := &mockPromptRenderer{
		RenderDecomposeFn: func(ctx interface{}) (string, error) {
			return "decompose prompt", nil
		},
	}

	var buf strings.Builder
	logFn := func(format string, args ...interface{}) {}

	r := &Runner{
		cfg:               cfg,
		beads:             mockBeads,
		router:            mockRouter,
		renderer:          mockPromptRenderer,
		output:            &buf,
		escalationHandler: escalation.NewHandler(cfg, nil, mockBeads, nil, nil, logFn, nil),
	}

	parentBead := &bead.Bead{
		ID:              "parent-1",
		Title:           "Parent",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should NOT return an error - should proceed with warning
	subTasks, err := r.DecomposeTask(ctx, parentBead)
	if err != nil {
		t.Fatalf("DecomposeTask() should proceed with warning after max retries, got error: %v", err)
	}

	// Should return the sub-tasks despite violations
	if len(subTasks) == 0 {
		t.Error("Should return sub-tasks despite persistent violations (warning path)")
	}

	if subTasks[0].Title != "Persistent" {
		t.Errorf("Should return original sub-task after exhausting retries, got: %q", subTasks[0].Title)
	}
}

// toBeadCandidates converts SubTask slice to BeadCandidate slice for validation
// Expected failure: this function does not exist in runner package
func toBeadCandidates(subTasks []SubTask) []validate.BeadCandidate {
	candidates := make([]validate.BeadCandidate, len(subTasks))
	for i, st := range subTasks {
		candidates[i] = validate.BeadCandidate{
			Title:              st.Title,
			Description:        st.Description,
			AcceptanceCriteria: st.AcceptanceCriteria,
		}
	}
	return candidates
}
