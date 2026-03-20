package specloop

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
)

// TaskContext holds additional context injected into task prompts so the
// executor has project conventions and spec knowledge beyond the bare task fields.
type TaskContext struct {
	// ProjectConventions is the content of CLAUDE.md (project coding conventions).
	// Empty string means no CLAUDE.md was found.
	ProjectConventions string
	// SpecContent is the full spec text (from spec.md in the run directory).
	// Empty string means no spec was available.
	SpecContent string
}

// TaskContextProviderFunc loads TaskContext at call time. It is called once per
// prompt render so the context can reflect runtime state (e.g. worktree path).
type TaskContextProviderFunc func() TaskContext

// Compile-time interface check.
var _ TaskRunner = (*ProviderTaskRunner)(nil)

// ProviderTaskRunner executes tasks by invoking an LLM via llmadapter.Invoker.
type ProviderTaskRunner struct {
	invoker   llmadapter.Invoker
	workDirFn func() string
	contextFn TaskContextProviderFunc
}

// NewProviderTaskRunner creates a ProviderTaskRunner backed by the given invoker.
// workDirFn is called at invoke time to resolve the working directory; if it
// returns a non-empty string, InvokeInDir is used instead of Invoke so the LLM
// process runs in the specified directory. Lazy resolution allows the directory
// to be set after construction (e.g. by an init stage that creates a worktree).
func NewProviderTaskRunner(invoker llmadapter.Invoker, workDirFn func() string) *ProviderTaskRunner {
	return &ProviderTaskRunner{invoker: invoker, workDirFn: workDirFn}
}

// SetContextProvider sets an optional TaskContextProviderFunc that supplies
// project conventions (CLAUDE.md) and spec content to task prompts.
func (r *ProviderTaskRunner) SetContextProvider(fn TaskContextProviderFunc) {
	r.contextFn = fn
}

// FileTaskContextProvider returns a TaskContextProviderFunc that reads CLAUDE.md
// from the work directory root and spec.md from the run directory. Both are
// best-effort: if a file is missing, its field is left empty.
func FileTaskContextProvider(workDirFn func() string, runDir string) TaskContextProviderFunc {
	return func() TaskContext {
		var tc TaskContext

		// Read CLAUDE.md from work directory root (best-effort).
		dir := workDirFn()
		if dir != "" {
			claudePath := filepath.Join(dir, "CLAUDE.md")
			data, err := os.ReadFile(claudePath)
			if err == nil {
				tc.ProjectConventions = string(data)
			} else if !os.IsNotExist(err) {
				log.Printf("warning: failed to read CLAUDE.md at %s: %v", claudePath, err)
			}
		}

		// Read spec.md from run directory (best-effort).
		if runDir != "" {
			specPath := filepath.Join(runDir, "spec.md")
			data, err := os.ReadFile(specPath)
			if err == nil {
				tc.SpecContent = string(data)
			} else if !os.IsNotExist(err) {
				log.Printf("warning: failed to read spec.md at %s: %v", specPath, err)
			}
		}

		return tc
	}
}

// invoke calls InvokeInDir when workDirFn returns a non-empty string, otherwise calls Invoke.
func (r *ProviderTaskRunner) invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	dir := r.workDirFn()
	if dir != "" {
		return r.invoker.InvokeInDir(ctx, prompt, dir)
	}
	return r.invoker.Invoke(ctx, prompt)
}

// taskContext returns the current TaskContext from contextFn, or an empty
// TaskContext if no provider is configured.
func (r *ProviderTaskRunner) taskContext() TaskContext {
	if r.contextFn != nil {
		return r.contextFn()
	}
	return TaskContext{}
}

// RunTask renders a task prompt and invokes the LLM. It maps the provider result
// to a TaskResult. FilesChanged is always empty — the TaskLoop fills that in.
func (r *ProviderTaskRunner) RunTask(ctx context.Context, task runstore.Task) (TaskResult, error) {
	prompt := renderTaskPrompt(task, r.taskContext())
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
	prompt := renderRepairPrompt(task, failures, r.taskContext())
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

// renderContextSections writes the project conventions (CLAUDE.md) before the
// task-specific sections. Full spec is only included for fix/repair tasks to
// avoid polluting context on original tasks where the objective should suffice.
func renderContextSections(b *strings.Builder, tc TaskContext, includeSpec bool) {
	if tc.ProjectConventions != "" {
		b.WriteString("### Project Conventions\n")
		b.WriteString("The following are project conventions from CLAUDE.md. Follow these conventions in all code you write.\n\n")
		b.WriteString(tc.ProjectConventions)
		b.WriteString("\n\n")
	}

	if includeSpec && tc.SpecContent != "" {
		b.WriteString("### Full Spec\n")
		b.WriteString("The following is the full specification for this work. Use it for context but focus on your specific task objective.\n\n")
		b.WriteString(tc.SpecContent)
		b.WriteString("\n\n")
	}
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
// Context sections (project conventions, spec) are rendered first, followed by
// the task header, body, and fix-specific sections.
func renderTaskPrompt(task runstore.Task, tc TaskContext) string {
	var b strings.Builder

	renderContextSections(&b, tc, task.Kind == "fix")

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

// renderRepairPrompt builds a prompt that includes context, the original task,
// and the failures that need to be addressed.
func renderRepairPrompt(task runstore.Task, failures []string, tc TaskContext) string {
	var b strings.Builder

	renderContextSections(&b, tc, true)

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
