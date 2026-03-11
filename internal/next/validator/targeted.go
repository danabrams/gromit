package validator

import (
	"context"
	"fmt"
)

// RunTargeted executes proof check commands for a specific task.
// Each command string is converted to a Check and executed sequentially.
func (r *Runner) RunTargeted(ctx context.Context, proofChecks []string, workDir string) (CheckResults, error) {
	checks := make([]Check, len(proofChecks))
	for i, cmd := range proofChecks {
		checks[i] = Check{
			Name:    fmt.Sprintf("proof-%d", i+1),
			Command: cmd,
			Type:    "proof",
		}
	}
	return r.RunAlwaysRun(ctx, checks, workDir)
}
