package specloop

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/validator"
)

// ShellTaskInspector implements TaskInspector by running proof checks via shell commands.
type ShellTaskInspector struct {
	workDir string
}

// NewShellTaskInspector creates a ShellTaskInspector that runs proof checks in workDir.
func NewShellTaskInspector(workDir string) *ShellTaskInspector {
	return &ShellTaskInspector{workDir: workDir}
}

// Inspect runs the task's proof checks and returns whether they all passed.
// If the task has no proof checks, it returns Pass=true immediately.
func (s *ShellTaskInspector) Inspect(ctx context.Context, task runstore.Task) InspectResult {
	if len(task.ProofChecks) == 0 {
		return InspectResult{Pass: true}
	}

	results, err := validator.NewRunner().RunTargeted(ctx, task.ProofChecks, s.workDir)
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
