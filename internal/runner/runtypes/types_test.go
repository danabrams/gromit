package runtypes

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
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
		Bead:                 testBead,
		Parent:               parentBead,
		Result:               &IterationResult{BeadID: "test-1"},
		Model:                "opus",
		Tier:                 "high",
		BuildProvider:        "claude",
		PromptCtx:            promptCtx,
		BuildPrompt:          "build this feature",
		StartCommit:          "abc123",
		Iteration:            3,
		RetriesThisModel:     1,
		TotalRetriesThisBead: 2,
		MaxRetries:           3,
		MaxRetriesPerBead:    5,
		ParentCtx:            parentCtx,
		BeadTimeout:          10 * time.Minute,
		RunDeadline:          deadline,
		ScopeEstimate:        scopeEst,
		TouchedPackages:      []string{"internal/runner", "internal/config"},
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
	result := IterationResult{
		BeadID:                "bead-42",
		BeadTitle:             "Add feature X",
		Model:                 "sonnet",
		Success:               true,
		Validated:             true,
		Duration:              5 * time.Minute,
		Error:                 nil,
		Escalated:             false,
		EscalatedTo:           "",
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
	if result.RateLimitHits != 2 {
		t.Errorf("RateLimitHits = %d, want %d", result.RateLimitHits, 2)
	}
	if result.RateLimitRecoveryMs != 1500 {
		t.Errorf("RateLimitRecoveryMs = %d, want %d", result.RateLimitRecoveryMs, 1500)
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
