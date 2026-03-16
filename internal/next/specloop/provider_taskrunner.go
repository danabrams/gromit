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
	invoker   llmadapter.Invoker
	workDirFn func() string
}

// NewProviderTaskRunner creates a ProviderTaskRunner backed by the given invoker.
// workDirFn is called at invoke time to resolve the working directory; if it
// returns a non-empty string, InvokeInDir is used instead of Invoke so the LLM
// process runs in the specified directory. Lazy resolution allows the directory
// to be set after construction (e.g. by an init stage that creates a worktree).
func NewProviderTaskRunner(invoker llmadapter.Invoker, workDirFn func() string) *ProviderTaskRunner {
	return &ProviderTaskRunner{invoker: invoker, workDirFn: workDirFn}
}

// invoke calls InvokeInDir when workDirFn returns a non-empty string, otherwise calls Invoke.
func (r *ProviderTaskRunner) invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	dir := r.workDirFn()
	if dir != "" {
		return r.invoker.InvokeInDir(ctx, prompt, dir)
	}
	return r.invoker.Invoke(ctx, prompt)
}

// RunTask renders a task prompt and invokes the LLM. It maps the provider result
// to a TaskResult. FilesChanged is always empty — the TaskLoop fills that in.
func (r *ProviderTaskRunner) RunTask(ctx context.Context, task runstore.Task) (TaskResult, error) {
	prompt := renderTaskPrompt(task)
	result, err := r.invoke(ctx, prompt)
	tr := mapResult(result)
	if err != nil {
		return tr, err
	}
	if result == nil {
		return tr, fmt.Errorf("taskrunner: provider returned nil result")
	}
	return tr, nil
}

// RepairTask renders a repair prompt that includes failure context, then invokes
// the LLM. Result mapping is the same as RunTask.
func (r *ProviderTaskRunner) RepairTask(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error) {
	prompt := renderRepairPrompt(task, failures)
	result, err := r.invoke(ctx, prompt)
	tr := mapResult(result)
	if err != nil {
		return tr, err
	}
	if result == nil {
		return tr, fmt.Errorf("taskrunner: provider returned nil result")
	}
	return tr, nil
}

// renderTaskBody writes the common task sections (Objective, Spec Constraints, Expected Touched Area, Proof Checks).
// Spec Constraints appear before Proof Checks so the agent anchors on hard limits before reading success criteria.
func renderTaskBody(b *strings.Builder, task runstore.Task) {
	fmt.Fprintf(b, "### Objective\n%s\n\n", task.Objective)

	if task.SpecConstraints != "" {
		b.WriteString("### Spec Constraints\n")
		b.WriteString("The following constraints are HARD REQUIREMENTS from the spec. Do NOT violate them under any circumstances.\n")
		b.WriteString("'Modify' includes editing, deleting, renaming, or moving a file — any change to an existing file counts as modification.\n\n")
		b.WriteString(task.SpecConstraints)
		b.WriteString("\n\n")
	}

	if len(task.ExpectedTouchedArea) > 0 {
		b.WriteString("### Expected Touched Area\n")
		for _, area := range task.ExpectedTouchedArea {
			fmt.Fprintf(b, "- %s\n", area)
		}
		b.WriteString("\n")
	}

	if len(task.ProofChecks) > 0 {
		b.WriteString("### Proof Checks\n")
		for _, check := range task.ProofChecks {
			fmt.Fprintf(b, "- %s\n", check)
		}
		b.WriteString("\n")
	}
}

// renderTaskPrompt builds the prompt sent to the LLM for a new task execution.
// For fix tasks with FailuresAddressed, the prompt includes the specific failures
// and instructs Claude to make surgical fixes without recreating the entire feature.
func renderTaskPrompt(task runstore.Task) string {
	var b strings.Builder
	if task.Kind == "fix" {
		fmt.Fprintf(&b, "## Fix Task: %s\n\n", task.TaskID)
	} else {
		fmt.Fprintf(&b, "## Task: %s\n\n", task.TaskID)
	}
	renderTaskBody(&b, task)

	if task.Kind == "fix" && len(task.FailuresAddressed) > 0 {
		b.WriteString("### Failures to Address\n")
		b.WriteString("This is a targeted fix task. Address ONLY the specific issues listed below.\n")
		b.WriteString("Do not recreate or rewrite the entire feature — make surgical changes to fix these issues.\n\n")
		for _, f := range task.FailuresAddressed {
			fmt.Fprintf(&b, "- %s\n", f)
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
	renderTaskBody(&b, task)
	if len(failures) > 0 {
		b.WriteString("### Failures to Address\n")
		for _, f := range failures {
			fmt.Fprintf(&b, "- %s\n", f)
		}
		b.WriteString("\n")
	}
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
