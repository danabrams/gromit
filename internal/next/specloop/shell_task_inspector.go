package specloop

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/validator"
)

// ShellTaskInspector implements TaskInspector by running proof checks via shell commands.
type ShellTaskInspector struct {
	workDirFn func() string
}

// NewShellTaskInspector creates a ShellTaskInspector that runs proof checks in
// the directory returned by workDirFn. The function is called at inspect time,
// enabling lazy resolution of the working directory (e.g. after a worktree is
// created by the init stage).
func NewShellTaskInspector(workDirFn func() string) *ShellTaskInspector {
	return &ShellTaskInspector{workDirFn: workDirFn}
}

// Inspect runs the task's proof checks and returns whether they all passed.
// If the task has no proof checks, it returns Pass=true immediately.
func (s *ShellTaskInspector) Inspect(ctx context.Context, task runstore.Task) InspectResult {
	if len(task.ProofChecks) == 0 {
		return InspectResult{Pass: true}
	}

	results, err := validator.NewRunner().RunTargeted(ctx, task.ProofChecks, s.workDirFn())
	if err != nil {
		return InspectResult{Pass: false, Failures: []string{err.Error()}}
	}

	var failures []string
	for _, r := range results.Results {
		if !r.Pass {
			failures = append(failures, fmt.Sprintf("%s: %s", r.Name, r.Output))
		}
	}

	return InspectResult{
		Pass:     results.AllPass(),
		Failures: failures,
	}
}
