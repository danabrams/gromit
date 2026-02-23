package runtypes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestBeadContext_AllFieldsExported verifies that BeadContext has all expected
// exported fields for use by runner sub-packages.
func TestBeadContext_AllFieldsExported(t *testing.T) {
	scopeEst := &prompt.ScopeEstimate{}
	parentCtx := context.Background()
	deadline := time.Now().Add(30 * time.Minute)
	promptCtx := &prompt.Context{}
	testBead := &bead.Bead{ID: "test-1", Title: "Test bead"}
	parentBead := &bead.Bead{ID: "parent-1", Title: "Parent bead"}

	bc := BeadContext{
		Bead:                   testBead,
		Parent:                 parentBead,
		Result:                 &IterationResult{BeadID: "test-1"},
		Model:                  "opus",
		Tier:                   "high",
		BuildProvider:          "claude",
		PromptCtx:              promptCtx,
		BuildPrompt:            "build this feature",
		StartCommit:            "abc123",
		Iteration:              3,
		RetriesThisModel:       1,
		TotalRetriesThisBead:   2,
		MaxRetries:             3,
		MaxRetriesPerBead:      5,
		CumulativeInputTokens:  987,
		CumulativeOutputTokens: 654,
		ParentCtx:              parentCtx,
		BeadTimeout:            10 * time.Minute,
		RunDeadline:            deadline,
		ScopeEstimate:          scopeEst,
		TouchedPackages:        []string{"internal/runner", "internal/config"},
	}

	// Verify all fields are set correctly by reading them back
	if bc.Bead.ID != "test-1" {
		t.Errorf("Bead.ID = %q, want %q", bc.Bead.ID, "test-1")
	}
	if bc.Parent.ID != "parent-1" {
		t.Errorf("Parent.ID = %q, want %q", bc.Parent.ID, "parent-1")
	}
	if bc.Result.BeadID != "test-1" {
		t.Errorf("Result.BeadID = %q, want %q", bc.Result.BeadID, "test-1")
	}
	if bc.Model != "opus" {
		t.Errorf("Model = %q, want %q", bc.Model, "opus")
	}
	if bc.Tier != "high" {
		t.Errorf("Tier = %q, want %q", bc.Tier, "high")
	}
	if bc.BuildProvider != "claude" {
		t.Errorf("BuildProvider = %q, want %q", bc.BuildProvider, "claude")
	}
	if bc.PromptCtx != promptCtx {
		t.Error("PromptCtx does not match")
	}
	if bc.BuildPrompt != "build this feature" {
		t.Errorf("BuildPrompt = %q, want %q", bc.BuildPrompt, "build this feature")
	}
	if bc.StartCommit != "abc123" {
		t.Errorf("StartCommit = %q, want %q", bc.StartCommit, "abc123")
	}
	if bc.Iteration != 3 {
		t.Errorf("Iteration = %d, want %d", bc.Iteration, 3)
	}
	if bc.RetriesThisModel != 1 {
		t.Errorf("RetriesThisModel = %d, want %d", bc.RetriesThisModel, 1)
	}
	if bc.TotalRetriesThisBead != 2 {
		t.Errorf("TotalRetriesThisBead = %d, want %d", bc.TotalRetriesThisBead, 2)
	}
	if bc.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want %d", bc.MaxRetries, 3)
	}
	if bc.MaxRetriesPerBead != 5 {
		t.Errorf("MaxRetriesPerBead = %d, want %d", bc.MaxRetriesPerBead, 5)
	}
	if bc.CumulativeInputTokens != 987 {
		t.Errorf("CumulativeInputTokens = %d, want %d", bc.CumulativeInputTokens, 987)
	}
	if bc.CumulativeOutputTokens != 654 {
		t.Errorf("CumulativeOutputTokens = %d, want %d", bc.CumulativeOutputTokens, 654)
	}
	if bc.ParentCtx != parentCtx {
		t.Error("ParentCtx does not match")
	}
	if bc.BeadTimeout != 10*time.Minute {
		t.Errorf("BeadTimeout = %v, want %v", bc.BeadTimeout, 10*time.Minute)
	}
	if !bc.RunDeadline.Equal(deadline) {
		t.Errorf("RunDeadline = %v, want %v", bc.RunDeadline, deadline)
	}
	if bc.ScopeEstimate != scopeEst {
		t.Error("ScopeEstimate does not match")
	}
	if len(bc.TouchedPackages) != 2 || bc.TouchedPackages[0] != "internal/runner" {
		t.Errorf("TouchedPackages = %v, want [internal/runner internal/config]", bc.TouchedPackages)
	}
}

// TestIterationResult_InRuntypes verifies that IterationResult is defined in runtypes
// with all expected fields matching the original runner.IterationResult.
func TestIterationResult_InRuntypes(t *testing.T) {
	promptDiagnostics := &prompt.PromptDiagnostics{
		PromptType:      "build",
		EstimatedTokens: 1234,
		SectionTokens: map[string]int{
			prompt.SectionRules: 100,
		},
	}

	result := IterationResult{
		BeadID:                "bead-42",
		BeadTitle:             "Add feature X",
		Model:                 "sonnet",
		Provider:              "codex",
		FailureCategory:       "rate_limited",
		Success:               true,
		Validated:             true,
		Duration:              5 * time.Minute,
		Error:                 nil,
		Escalated:             false,
		EscalatedTo:           "",
		OriginalTier:          "low",
		ActualTier:            "medium",
		Decomposed:            false,
		Output:                "build output here",
		CostUSD:               0.15,
		InputTokens:           5000,
		OutputTokens:          3000,
		ReviewBrokeValidation: false,
		AlreadyDone:           false,
		ValidationRetried:     true,
		TrivialAutoFixed:      false,
		UsageLimited:          false,
		ValidationMode:        "direct",
		TimeoutType:           "stall",
		TimeToFirstEventMs:    250,
		ToolCallCount:         12,
		StallCount:            1,
		StallTier:             "active",
		RateLimitHits:         2,
		RateLimitRecoveryMs:   1500,
		PromptDiagnostics:     promptDiagnostics,
	}

	// Verify key fields to confirm the struct shape is correct
	if result.BeadID != "bead-42" {
		t.Errorf("BeadID = %q, want %q", result.BeadID, "bead-42")
	}
	if result.Success != true {
		t.Error("Success should be true")
	}
	if result.Duration != 5*time.Minute {
		t.Errorf("Duration = %v, want %v", result.Duration, 5*time.Minute)
	}
	if result.CostUSD != 0.15 {
		t.Errorf("CostUSD = %f, want %f", result.CostUSD, 0.15)
	}
	if result.ValidationMode != "direct" {
		t.Errorf("ValidationMode = %q, want %q", result.ValidationMode, "direct")
	}
	if result.TimeoutType != "stall" {
		t.Errorf("TimeoutType = %q, want %q", result.TimeoutType, "stall")
	}
	if result.Provider != "codex" {
		t.Errorf("Provider = %q, want %q", result.Provider, "codex")
	}
	if result.FailureCategory != "rate_limited" {
		t.Errorf("FailureCategory = %q, want %q", result.FailureCategory, "rate_limited")
	}
	if result.RateLimitHits != 2 {
		t.Errorf("RateLimitHits = %d, want %d", result.RateLimitHits, 2)
	}
	if result.RateLimitRecoveryMs != 1500 {
		t.Errorf("RateLimitRecoveryMs = %d, want %d", result.RateLimitRecoveryMs, 1500)
	}
	if result.OriginalTier != "low" {
		t.Errorf("OriginalTier = %q, want %q", result.OriginalTier, "low")
	}
	if result.ActualTier != "medium" {
		t.Errorf("ActualTier = %q, want %q", result.ActualTier, "medium")
	}
	if result.PromptDiagnostics != promptDiagnostics {
		t.Error("PromptDiagnostics should match assigned diagnostics pointer")
	}
}

// TestIterationResult_TierFieldsJSONTags verifies that IterationResult marshals
// tier provenance fields using snake_case JSON keys and omitempty behavior.
func TestIterationResult_TierFieldsJSONTags(t *testing.T) {
	result := IterationResult{
		OriginalTier: "low",
		ActualTier:   "medium",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal iteration result: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "\"original_tier\":\"low\"") {
		t.Fatalf("expected original_tier in JSON, got %s", s)
	}
	if !strings.Contains(s, "\"actual_tier\":\"medium\"") {
		t.Fatalf("expected actual_tier in JSON, got %s", s)
	}

	emptyData, err := json.Marshal(IterationResult{})
	if err != nil {
		t.Fatalf("marshal empty iteration result: %v", err)
	}
	empty := string(emptyData)
	if strings.Contains(empty, "original_tier") {
		t.Fatalf("expected original_tier omitted from empty result, got %s", empty)
	}
	if strings.Contains(empty, "actual_tier") {
		t.Fatalf("expected actual_tier omitted from empty result, got %s", empty)
	}
}

// TestSubTask_InRuntypes verifies that SubTask is defined in runtypes with JSON tags.
func TestSubTask_InRuntypes(t *testing.T) {
	dependsOn := 1
	task := SubTask{
		Title:              "Create helper function",
		Description:        "Add a helper for parsing config",
		DependsOn:          &dependsOn,
		AcceptanceCriteria: []string{"Helper parses YAML", "Helper returns error on invalid input"},
	}

	if task.Title != "Create helper function" {
		t.Errorf("Title = %q, want %q", task.Title, "Create helper function")
	}
	if task.Description != "Add a helper for parsing config" {
		t.Errorf("Description = %q, want %q", task.Description, "Add a helper for parsing config")
	}
	if task.DependsOn == nil || *task.DependsOn != 1 {
		t.Errorf("DependsOn = %v, want pointer to 1", task.DependsOn)
	}
	if len(task.AcceptanceCriteria) != 2 {
		t.Errorf("AcceptanceCriteria length = %d, want 2", len(task.AcceptanceCriteria))
	}
}

// TestSubTask_NormalizeNilFields verifies that normalizeNilFields converts nil
// slices to empty slices and is nil-safe on the receiver.
func TestSubTask_NormalizeNilFields(t *testing.T) {
	t.Run("nil AcceptanceCriteria becomes empty slice", func(t *testing.T) {
		task := SubTask{Title: "Test", AcceptanceCriteria: nil}
		task.NormalizeNilFields()
		if task.AcceptanceCriteria == nil {
			t.Error("AcceptanceCriteria should be non-nil after NormalizeNilFields")
		}
		if len(task.AcceptanceCriteria) != 0 {
			t.Errorf("AcceptanceCriteria length = %d, want 0", len(task.AcceptanceCriteria))
		}
	})

	t.Run("non-nil AcceptanceCriteria unchanged", func(t *testing.T) {
		task := SubTask{AcceptanceCriteria: []string{"criterion"}}
		task.NormalizeNilFields()
		if len(task.AcceptanceCriteria) != 1 || task.AcceptanceCriteria[0] != "criterion" {
			t.Errorf("AcceptanceCriteria = %v, want [criterion]", task.AcceptanceCriteria)
		}
	})

	t.Run("nil receiver does not panic", func(t *testing.T) {
		var task *SubTask
		task.NormalizeNilFields() // should not panic
	})
}

// TestInvocationResult_ResultField verifies that InvocationResult has a Result
// field of type *claude.Result.
func TestInvocationResult_ResultField(t *testing.T) {
	claudeResult := &claude.Result{
		Success: true,
		Output:  "build output",
		Model:   "sonnet",
	}
	inv := InvocationResult{
		Result: claudeResult,
	}
	if inv.Result != claudeResult {
		t.Error("InvocationResult.Result does not match assigned *claude.Result")
	}
	if inv.Result.Success != true {
		t.Error("InvocationResult.Result.Success should be true")
	}
}

// TestInvocationResult_AllFields verifies that InvocationResult has all 7 required fields
// with correct types: Stats, StallFired, ModelName, ProviderName, ProviderResult, TimeoutType.
func TestInvocationResult_AllFields(t *testing.T) {
	stats, err := logger.NewStreamStats()
	if err != nil {
		t.Fatalf("NewStreamStats() error: %v", err)
	}
	provResult := &provider.Result{
		Success: true,
		Output:  "provider output",
		Model:   "claude-sonnet",
	}
	inv := InvocationResult{
		Result:         &claude.Result{Success: true},
		Stats:          stats,
		StallFired:     true,
		ModelName:      "claude-sonnet-4-6",
		ProviderName:   "claude",
		ProviderResult: provResult,
		TimeoutType:    "stall",
	}

	if inv.Stats != stats {
		t.Error("Stats does not match assigned *logger.StreamStats")
	}
	if !inv.StallFired {
		t.Error("StallFired should be true")
	}
	if inv.ModelName != "claude-sonnet-4-6" {
		t.Errorf("ModelName = %q, want %q", inv.ModelName, "claude-sonnet-4-6")
	}
	if inv.ProviderName != "claude" {
		t.Errorf("ProviderName = %q, want %q", inv.ProviderName, "claude")
	}
	if inv.ProviderResult != provResult {
		t.Error("ProviderResult does not match assigned *provider.Result")
	}
	if inv.TimeoutType != "stall" {
		t.Errorf("TimeoutType = %q, want %q", inv.TimeoutType, "stall")
	}
}

// TestIterationResult_FailurePhase verifies that IterationResult has a FailurePhase field.
func TestIterationResult_FailurePhase(t *testing.T) {
	result := IterationResult{
		BeadID:       "bead-1",
		FailurePhase: "validation",
	}
	if result.FailurePhase != "validation" {
		t.Errorf("FailurePhase = %q, want %q", result.FailurePhase, "validation")
	}
}

func TestIterationResult_FilesTouched(t *testing.T) {
	result := IterationResult{
		BeadID:       "test-1",
		FilesTouched: 3,
	}
	if result.FilesTouched != 3 {
		t.Errorf("FilesTouched = %d, want 3", result.FilesTouched)
	}
}

func TestIterationResult_TouchedPackages(t *testing.T) {
	result := IterationResult{
		BeadID:          "test-1",
		TouchedPackages: []string{"internal/runner", "internal/logger"},
	}
	if len(result.TouchedPackages) != 2 {
		t.Fatalf("TouchedPackages length = %d, want 2", len(result.TouchedPackages))
	}
	if result.TouchedPackages[0] != "internal/runner" {
		t.Errorf("TouchedPackages[0] = %q, want %q", result.TouchedPackages[0], "internal/runner")
	}
	if result.TouchedPackages[1] != "internal/logger" {
		t.Errorf("TouchedPackages[1] = %q, want %q", result.TouchedPackages[1], "internal/logger")
	}
}

// TestIterationResult_SpecID verifies that IterationResult has a SpecID field.
func TestIterationResult_SpecID(t *testing.T) {
	result := IterationResult{
		BeadID: "bead-1",
		SpecID: "spec-abc",
	}
	if result.SpecID != "spec-abc" {
		t.Errorf("SpecID = %q, want %q", result.SpecID, "spec-abc")
	}
}

// TestIterationResult_CoverageFields verifies that IterationResult has the four
// coverage result fields required by the coverage tracker feature.
func TestIterationResult_CoverageFields(t *testing.T) {
	result := IterationResult{
		BeadID:             "bead-cov",
		CriteriaTotal:      10,
		CriteriaCovered:    7,
		CriteriaUntestable: 1,
		UncoveredCriteria:  []string{"criterion A", "criterion B"},
	}

	if result.CriteriaTotal != 10 {
		t.Errorf("CriteriaTotal = %d, want 10", result.CriteriaTotal)
	}
	if result.CriteriaCovered != 7 {
		t.Errorf("CriteriaCovered = %d, want 7", result.CriteriaCovered)
	}
	if result.CriteriaUntestable != 1 {
		t.Errorf("CriteriaUntestable = %d, want 1", result.CriteriaUntestable)
	}
	if len(result.UncoveredCriteria) != 2 || result.UncoveredCriteria[0] != "criterion A" {
		t.Errorf("UncoveredCriteria = %v, want [criterion A criterion B]", result.UncoveredCriteria)
	}
}

// TestPhaseMetric_CoreFields verifies PhaseMetric has all core per-phase fields.
func TestPhaseMetric_CoreFields(t *testing.T) {
	pm := PhaseMetric{
		Phase:        "red",
		CycleNumber:  2,
		BeadID:       "bead-abc",
		Model:        "haiku",
		Tier:         "low",
		CostUSD:      0.42,
		InputTokens:  1500,
		OutputTokens: 800,
		DurationMs:   3200,
		Success:      true,
		Escalated:    false,
	}

	if pm.Phase != "red" {
		t.Errorf("Phase = %q, want %q", pm.Phase, "red")
	}
	if pm.CycleNumber != 2 {
		t.Errorf("CycleNumber = %d, want 2", pm.CycleNumber)
	}
	if pm.BeadID != "bead-abc" {
		t.Errorf("BeadID = %q, want %q", pm.BeadID, "bead-abc")
	}
	if pm.Model != "haiku" {
		t.Errorf("Model = %q, want %q", pm.Model, "haiku")
	}
	if pm.Tier != "low" {
		t.Errorf("Tier = %q, want %q", pm.Tier, "low")
	}
	if pm.CostUSD != 0.42 {
		t.Errorf("CostUSD = %f, want 0.42", pm.CostUSD)
	}
	if pm.InputTokens != 1500 {
		t.Errorf("InputTokens = %d, want 1500", pm.InputTokens)
	}
	if pm.OutputTokens != 800 {
		t.Errorf("OutputTokens = %d, want 800", pm.OutputTokens)
	}
	if pm.DurationMs != 3200 {
		t.Errorf("DurationMs = %d, want 3200", pm.DurationMs)
	}
	if !pm.Success {
		t.Error("Success should be true")
	}
	if pm.Escalated {
		t.Error("Escalated should be false")
	}
}

// TestPhaseMetric_EscalatedFromAndCriteriaFields verifies PhaseMetric has
// escalated_from and criteria tracking fields.
func TestPhaseMetric_EscalatedFromAndCriteriaFields(t *testing.T) {
	pm := PhaseMetric{
		Phase:              "green",
		EscalatedFrom:      "haiku",
		CriteriaTotal:      5,
		CriteriaCovered:    3,
		CriteriaUntestable: 1,
	}

	if pm.EscalatedFrom != "haiku" {
		t.Errorf("EscalatedFrom = %q, want %q", pm.EscalatedFrom, "haiku")
	}
	if pm.CriteriaTotal != 5 {
		t.Errorf("CriteriaTotal = %d, want 5", pm.CriteriaTotal)
	}
	if pm.CriteriaCovered != 3 {
		t.Errorf("CriteriaCovered = %d, want 3", pm.CriteriaCovered)
	}
	if pm.CriteriaUntestable != 1 {
		t.Errorf("CriteriaUntestable = %d, want 1", pm.CriteriaUntestable)
	}
}

// TestIterationResult_PhaseMetrics verifies IterationResult has a PhaseMetrics slice
// that is zero-value safe (nil by default) and accepts PhaseMetric values.
func TestIterationResult_PhaseMetrics(t *testing.T) {
	t.Run("zero value has nil PhaseMetrics", func(t *testing.T) {
		result := IterationResult{}
		if result.PhaseMetrics != nil {
			t.Error("zero-value IterationResult.PhaseMetrics should be nil")
		}
	})

	t.Run("PhaseMetrics accepts appended values", func(t *testing.T) {
		result := IterationResult{BeadID: "bead-1"}
		result.PhaseMetrics = append(result.PhaseMetrics, PhaseMetric{
			Phase:       "red",
			CycleNumber: 1,
			BeadID:      "bead-1",
			Success:     false,
		})
		result.PhaseMetrics = append(result.PhaseMetrics, PhaseMetric{
			Phase:       "green",
			CycleNumber: 1,
			BeadID:      "bead-1",
			Success:     true,
		})

		if len(result.PhaseMetrics) != 2 {
			t.Errorf("PhaseMetrics length = %d, want 2", len(result.PhaseMetrics))
		}
		if result.PhaseMetrics[0].Phase != "red" {
			t.Errorf("PhaseMetrics[0].Phase = %q, want %q", result.PhaseMetrics[0].Phase, "red")
		}
		if result.PhaseMetrics[1].Phase != "green" {
			t.Errorf("PhaseMetrics[1].Phase = %q, want %q", result.PhaseMetrics[1].Phase, "green")
		}
	})
}

// TestCallbackFunctionTypes verifies that the callback function types are defined
// and can be used with the correct signatures.
func TestCallbackFunctionTypes(t *testing.T) {
	t.Run("GitDiffFn signature", func(t *testing.T) {
		// GitDiffFn takes a commit string and returns (diff string, error)
		var fn GitDiffFn = func(startCommit string) (string, error) {
			return "diff --git a/file.go b/file.go", nil
		}
		diff, err := fn("abc123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if diff != "diff --git a/file.go b/file.go" {
			t.Errorf("diff = %q, want git diff output", diff)
		}
	})

	t.Run("CmdRunnerFn signature", func(t *testing.T) {
		// CmdRunnerFn takes (ctx, command, workDir) and returns (stdout, stderr, exitCode, error)
		var fn CmdRunnerFn = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "PASS", "", 0, nil
		}
		stdout, stderr, exitCode, err := fn(context.Background(), "go test ./...", "/workspace")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stdout != "PASS" {
			t.Errorf("stdout = %q, want %q", stdout, "PASS")
		}
		if stderr != "" {
			t.Errorf("stderr = %q, want empty", stderr)
		}
		if exitCode != 0 {
			t.Errorf("exitCode = %d, want 0", exitCode)
		}
	})

	t.Run("AutoFixFn signature", func(t *testing.T) {
		// AutoFixFn takes a startCommit string and returns error
		var fn AutoFixFn = func(startCommit string) error {
			return nil
		}
		err := fn("def456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
