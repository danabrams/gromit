package runner

import (
	"runtime"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/runbook"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

const maxRunbookFailureOutputBytes = 5 * 1024

func shouldCaptureRunbookEntry(result *IterationResult) bool {
	if result == nil {
		return false
	}
	if result.Success || result.Decomposed || result.AlreadyDone {
		return false
	}
	return true
}

func (r *Runner) captureRunbookEntry(bc *runtypes.BeadContext, result *IterationResult) {
	if r == nil || result == nil || !shouldCaptureRunbookEntry(result) {
		return
	}

	entry := runbook.NewEntry(result.BeadID, time.Now())
	entry.BeadTitle = result.BeadTitle
	entry.SpecID = result.SpecID
	entry.FailureCategory = result.FailureCategory
	entry.Env = runbook.Env{
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	if r.cfg != nil {
		entry.ValidationCommands = append([]string{}, r.cfg.Validation.FastCommandsOrDefault()...)
		entry.EscalationChain = append([]string{}, r.cfg.Escalation.Chain...)
	}

	if bc != nil {
		if bc.Bead != nil && entry.BeadTitle == "" {
			entry.BeadTitle = bc.Bead.Title
		}
		entry.StartCommit = bc.StartCommit
		entry.Prompt = bc.BuildPrompt
		if bc.PromptCtx != nil && strings.TrimSpace(bc.PromptCtx.ScopedTestCommand) != "" {
			entry.ValidationCommands = []string{strings.TrimSpace(bc.PromptCtx.ScopedTestCommand)}
		}
	}

	entry.FailureOutput = truncateRunbookFailureOutput(pickRunbookFailureOutput(result))

	head, err := r.getHead()
	if err != nil {
		r.log("Warning: failed to capture runbook failure commit for bead %s: %v", result.BeadID, err)
	} else {
		entry.FailureCommit = head
	}

	if err := runbook.Append(r.gromitDir, entry); err != nil {
		r.log("Warning: failed to append runbook entry for bead %s: %v", result.BeadID, err)
	}
}

func pickRunbookFailureOutput(result *IterationResult) string {
	if result == nil {
		return ""
	}
	if strings.TrimSpace(result.AcceptanceFailureOutput) != "" {
		return result.AcceptanceFailureOutput
	}
	if strings.TrimSpace(result.Output) != "" {
		return result.Output
	}
	if result.Error != nil {
		return result.Error.Error()
	}
	return ""
}

func truncateRunbookFailureOutput(output string) string {
	if len(output) <= maxRunbookFailureOutputBytes {
		return output
	}
	return output[len(output)-maxRunbookFailureOutputBytes:]
}
