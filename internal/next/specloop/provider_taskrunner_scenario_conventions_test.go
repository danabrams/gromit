package specloop

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
)

// TestScenario_ExecutorTaskReceivesConventions verifies that when RunState.ArchitectureConstraints
// is populated, those constraints are copied to task.ArchitectureConstraints during planning and
// the executor renders them into the prompt under an "Architecture Conventions" section.
//
// The plan stage copies rs.ArchitectureConstraints → task.ArchitectureConstraints; this test
// simulates that post-planning state and asserts the executor prompt contains the convention.
func TestScenario_ExecutorTaskReceivesConventions(t *testing.T) {
	// Seed: RunState with one architecture constraint.
	constraint := "Config.Tier always receives a tier label"

	// Invoke: build the executor prompt via ProviderTaskRunner.RunTask.
	inv := &mockInvoker{
		result: &provider.Result{
			Success:  true,
			Model:    "sonnet",
			Duration: 1 * time.Second,
		},
	}
	runner := NewProviderTaskRunner(inv, func() string { return "" })
	runner.SetContextProvider(func() TaskContext {
		return TaskContext{
			ArchitectureConstraints: []string{constraint},
		}
	})

	task := runstore.Task{
		TaskID:              "t-001",
		Objective:           "implement tier labeling",
		ExpectedTouchedArea: []string{"internal/config/"},
		ProofChecks:         []string{"go test ./internal/config/..."},
	}

	_, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prompt := inv.capturedPrompt

	// Assert: prompt contains the Architecture Conventions section header.
	if !strings.Contains(prompt, "### Architecture Conventions") {
		t.Fatal("prompt does not contain '### Architecture Conventions' section header")
	}

	// Assert: prompt contains the constraint text.
	if !strings.Contains(prompt, constraint) {
		t.Fatalf("prompt does not contain constraint %q", constraint)
	}

	// Assert: constraint appears after the section header (not before it).
	headerIdx := strings.Index(prompt, "### Architecture Conventions")
	constraintIdx := strings.Index(prompt, constraint)
	if headerIdx >= constraintIdx {
		t.Errorf("Architecture Conventions header (pos %d) must appear before constraint (pos %d)",
			headerIdx, constraintIdx)
	}

	// Assert: constraint is rendered as a list item with "- " prefix.
	if !strings.Contains(prompt, "- "+constraint) {
		t.Errorf("constraint should be rendered as a list item with '- ' prefix, prompt excerpt: %q",
			prompt[headerIdx:headerIdx+len("### Architecture Conventions")+len(constraint)+20])
	}
}

// TestScenario_ExecutorTaskReceivesConventions_GlobalToRun verifies that conventions are
// global to the run: all tasks in the run receive the same architecture constraints
// regardless of which files they touch.
func TestScenario_ExecutorTaskReceivesConventions_GlobalToRun(t *testing.T) {
	constraint := "Config.Tier always receives a tier label"

	// Two tasks touching different areas — both receive the run-level constraint.
	tasks := []runstore.Task{
		{
			TaskID:              "t-001",
			Objective:           "implement tier label in config loader",
			ExpectedTouchedArea: []string{"internal/config/loader.go"},
			ProofChecks:         []string{"go test ./internal/config/..."},
		},
		{
			TaskID:              "t-002",
			Objective:           "implement tier label in server startup",
			ExpectedTouchedArea: []string{"cmd/server/main.go"},
			ProofChecks:         []string{"go build ./cmd/server/..."},
		},
	}

	for _, task := range tasks {
		t.Run(task.TaskID, func(t *testing.T) {
			inv := &mockInvoker{
				result: &provider.Result{
					Success:  true,
					Model:    "sonnet",
					Duration: 1 * time.Second,
				},
			}
			runner := NewProviderTaskRunner(inv, func() string { return "" })
			runner.SetContextProvider(func() TaskContext {
				return TaskContext{
					ArchitectureConstraints: []string{constraint},
				}
			})

			_, err := runner.RunTask(context.Background(), task)
			if err != nil {
				t.Fatalf("task %s: unexpected error: %v", task.TaskID, err)
			}

			prompt := inv.capturedPrompt

			if !strings.Contains(prompt, "### Architecture Conventions") {
				t.Errorf("task %s: prompt missing '### Architecture Conventions' header", task.TaskID)
			}
			if !strings.Contains(prompt, constraint) {
				t.Errorf("task %s: prompt missing constraint %q", task.TaskID, constraint)
			}
		})
	}
}
