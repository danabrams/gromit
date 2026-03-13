package specloop

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
)

// Compile-time interface check.
var _ TaskRunner = (*ProviderTaskRunner)(nil)

// ProviderTaskRunner executes tasks by invoking an LLM via llmadapter.Invoker.
type ProviderTaskRunner struct {
	invoker llmadapter.Invoker
}

// NewProviderTaskRunner creates a ProviderTaskRunner backed by the given invoker.
func NewProviderTaskRunner(invoker llmadapter.Invoker) *ProviderTaskRunner {
	return &ProviderTaskRunner{invoker: invoker}
}

// RunTask renders a task prompt and invokes the LLM. It maps the provider result
// to a TaskResult. FilesChanged is always empty — the TaskLoop fills that in.
func (r *ProviderTaskRunner) RunTask(ctx context.Context, task runstore.Task) (TaskResult, error) {
	prompt := renderTaskPrompt(task)
	result, err := r.invoker.Invoke(ctx, prompt)
	tr := mapResult(result)
	if err != nil {
		return tr, err
	}
	return tr, nil
}

// RepairTask renders a repair prompt that includes failure context, then invokes
// the LLM. Result mapping is the same as RunTask.
func (r *ProviderTaskRunner) RepairTask(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error) {
	prompt := renderRepairPrompt(task, failures)
	result, err := r.invoker.Invoke(ctx, prompt)
	tr := mapResult(result)
	if err != nil {
		return tr, err
	}
	return tr, nil
}

// renderTaskPrompt builds the prompt sent to the LLM for a new task execution.
func renderTaskPrompt(task runstore.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Task: %s\n\n", task.TaskID)
	fmt.Fprintf(&b, "### Objective\n%s\n\n", task.Objective)

	if len(task.ExpectedTouchedArea) > 0 {
		b.WriteString("### Expected Touched Area\n")
		for _, area := range task.ExpectedTouchedArea {
			fmt.Fprintf(&b, "- %s\n", area)
		}
		b.WriteString("\n")
	}

	if len(task.ProofChecks) > 0 {
		b.WriteString("### Proof Checks\n")
		for _, check := range task.ProofChecks {
			fmt.Fprintf(&b, "- %s\n", check)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderRepairPrompt builds a prompt that includes the original task context
// plus the failures that need to be addressed.
func renderRepairPrompt(task runstore.Task, failures []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Repair Task: %s\n\n", task.TaskID)
	fmt.Fprintf(&b, "### Objective\n%s\n\n", task.Objective)

	if len(task.ExpectedTouchedArea) > 0 {
		b.WriteString("### Expected Touched Area\n")
		for _, area := range task.ExpectedTouchedArea {
			fmt.Fprintf(&b, "- %s\n", area)
		}
		b.WriteString("\n")
	}

	b.WriteString("### Failures to Address\n")
	for _, f := range failures {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	b.WriteString("\n")

	return b.String()
}

// mapResult converts a provider.Result to a TaskResult.
func mapResult(r *provider.Result) TaskResult {
	if r == nil {
		return TaskResult{FilesChanged: []string{}}
	}
	status := "failed"
	if r.Success {
		status = "done"
	}
	return TaskResult{
		Status:       status,
		TokensUsed:   r.InputTokens + r.OutputTokens,
		Cost:         r.CostUSD,
		DurationMs:   r.Duration.Milliseconds(),
		Model:        r.Model,
		FilesChanged: []string{},
	}
}
