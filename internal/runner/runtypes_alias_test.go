//go:build acceptance

package runner

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestIterationResult_AliasesRuntypes verifies that runner.IterationResult is a type alias
// for runtypes.IterationResult, ensuring backward compatibility. Callers that reference
// runner.IterationResult should transparently get runtypes.IterationResult.
func TestIterationResult_AliasesRuntypes(t *testing.T) {
	// Create via runtypes, use as runner type — only works if they're the same type
	rtResult := runtypes.IterationResult{
		BeadID:    "alias-test",
		BeadTitle: "Test backward compat",
		Model:     "haiku",
		Success:   true,
		Duration:  2 * time.Minute,
		CostUSD:   0.05,
	}

	// This assignment only compiles if IterationResult is `type IterationResult = runtypes.IterationResult`
	var runnerResult IterationResult = rtResult

	if runnerResult.BeadID != "alias-test" {
		t.Errorf("BeadID = %q, want %q", runnerResult.BeadID, "alias-test")
	}
	if runnerResult.Model != "haiku" {
		t.Errorf("Model = %q, want %q", runnerResult.Model, "haiku")
	}
	if runnerResult.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want %f", runnerResult.CostUSD, 0.05)
	}
}

// TestSubTask_AliasesRuntypes verifies that runner.SubTask is a type alias
// for runtypes.SubTask, ensuring backward compatibility.
func TestSubTask_AliasesRuntypes(t *testing.T) {
	rtTask := runtypes.SubTask{
		Title:              "Alias compat task",
		Description:        "Verify backward compat",
		AcceptanceCriteria: []string{"compiles", "runs"},
	}

	// This assignment only compiles if SubTask is `type SubTask = runtypes.SubTask`
	var runnerTask SubTask = rtTask

	if runnerTask.Title != "Alias compat task" {
		t.Errorf("Title = %q, want %q", runnerTask.Title, "Alias compat task")
	}
	if len(runnerTask.AcceptanceCriteria) != 2 {
		t.Errorf("AcceptanceCriteria length = %d, want 2", len(runnerTask.AcceptanceCriteria))
	}
}
