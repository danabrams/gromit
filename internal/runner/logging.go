package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/runner/andon"
)

var artifactBeadIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

const (
	acceptanceFailureDirName = "failures"
	artifactFallbackBeadID   = "unknown-bead"
	artifactDirPerm          = 0o755
	artifactFilePerm         = 0o644
)

func (r *Runner) writeIterationLog(iteration int, result *IterationResult) {
	if r == nil || r.logger == nil || result == nil {
		return
	}
	ensureFailureAndonEnvelope(result)

	errStr := ""
	if result.Error != nil {
		errStr = result.Error.Error()
	}

	outcome := ""
	if result.AlreadyDone {
		outcome = "atdd_already_done"
	}

	artifactPath := result.AcceptanceFailureArtifact
	if artifactPath == "" && result.AcceptanceFailureOutput != "" {
		artifactPath = r.writeAcceptanceFailureArtifact(iteration, result)
		result.AcceptanceFailureArtifact = artifactPath
	}

	r.logIterationWithWarning(&logger.IterationLog{
		Timestamp:                 time.Now(),
		Iteration:                 iteration,
		BeadID:                    result.BeadID,
		BeadTitle:                 result.BeadTitle,
		Model:                     result.Model,
		Success:                   result.Success,
		Validated:                 result.Validated,
		Escalated:                 result.Escalated,
		EscalatedTo:               result.EscalatedTo,
		DurationMs:                result.Duration.Milliseconds(),
		CostUSD:                   result.CostUSD,
		InputTokens:               result.InputTokens,
		OutputTokens:              result.OutputTokens,
		Error:                     errStr,
		Outcome:                   outcome,
		ValidationRetried:         result.ValidationRetried,
		TrivialAutoFixed:          result.TrivialAutoFixed,
		UsageLimited:              result.UsageLimited,
		ValidationMode:            result.ValidationMode,
		CompilationErrors:         result.CompilationErrors,
		HardStopPendingApproval:   result.HardStopPendingApproval,
		TimeoutType:               result.TimeoutType,
		TimeToFirstEventMs:        result.TimeToFirstEventMs,
		ToolCallCount:             result.ToolCallCount,
		StallCount:                result.StallCount,
		StallTier:                 result.StallTier,
		RateLimitHits:             result.RateLimitHits,
		RateLimitRecoveryMs:       result.RateLimitRecoveryMs,
		AcceptanceFailureSummary:  result.AcceptanceFailureSummary,
		AcceptanceFailureOutput:   result.AcceptanceFailureOutput,
		AcceptanceFailureArtifact: artifactPath,
		FailureClass:              string(result.FailureClass),
		AndonLevel:                string(result.AndonLevel),
		TrimDecision:              result.TrimDecision,
		AutonomyEligible:          result.AutonomyEligible,
		AutonomySuccess:           result.AutonomySuccess,
		FirstPassSuccess:          result.FirstPassSuccess,
		MTTRProxyMs:               result.MTTRProxyMs,
		EscalationClass:           string(result.EscalationClass),
		RecurrenceCount:           result.RecurrenceCount,
	})
}

func ensureFailureAndonEnvelope(result *IterationResult) {
	if result == nil || result.Success {
		return
	}
	if result.FailureClass == "" {
		result.FailureClass = string(andon.FailureClassWorkflow)
	}
	if result.AndonLevel == "" {
		result.AndonLevel = string(andon.LevelL1)
	}
}

func (r *Runner) logIterationWithWarning(log *logger.IterationLog) {
	if r == nil || r.logger == nil || log == nil {
		return
	}
	if err := r.logger.LogIteration(log); err != nil {
		r.log("Warning: failed to write iteration log: %v", err)
		return
	}
	if r.trendUpdater != nil {
		r.trendUpdater.Trigger()
	}
}

func (r *Runner) startTrendUpdater() {
	if r == nil || r.cfg == nil || r.trendUpdater != nil {
		return
	}
	if r.cfg.Paths.Logs == "" || r.gromitDir == "" {
		return
	}

	metricsDir := filepath.Join(r.gromitDir, "metrics")
	r.trendUpdater = logger.NewAsyncTrendUpdater(r.cfg.Paths.Logs, metricsDir, 30, func(err error) {
		r.log("Warning: failed to refresh process trend metrics: %v", err)
	})
	r.trendUpdater.Trigger()
}

func (r *Runner) stopTrendUpdater() {
	if r == nil || r.trendUpdater == nil {
		return
	}
	r.trendUpdater.Close()
	r.trendUpdater = nil
}

func (r *Runner) writeAcceptanceFailureArtifact(iteration int, result *IterationResult) string {
	if r == nil || result == nil || result.AcceptanceFailureOutput == "" || r.cfg == nil {
		return ""
	}

	failuresDir := filepath.Join(r.cfg.Paths.Logs, acceptanceFailureDirName)
	if err := os.MkdirAll(failuresDir, artifactDirPerm); err != nil {
		r.log("Warning: failed to create acceptance failure logs directory: %v", err)
		return ""
	}

	safeBeadID := artifactBeadIDSanitizer.ReplaceAllString(result.BeadID, "_")
	if safeBeadID == "" {
		safeBeadID = artifactFallbackBeadID
	}
	filename := fmt.Sprintf("acceptance-%s-it%d-%s.log", time.Now().UTC().Format("20060102-150405"), iteration, safeBeadID)
	fullPath := filepath.Join(failuresDir, filename)

	body := strings.TrimSpace(fmt.Sprintf(
		"timestamp: %s\niteration: %d\nbead_id: %s\nbead_title: %s\nsummary: %s\nerror: %v\n\nvalidation_output:\n%s\n",
		time.Now().UTC().Format(time.RFC3339),
		iteration,
		result.BeadID,
		result.BeadTitle,
		result.AcceptanceFailureSummary,
		result.Error,
		result.AcceptanceFailureOutput,
	))
	if err := os.WriteFile(fullPath, []byte(body+"\n"), artifactFilePerm); err != nil {
		r.log("Warning: failed to write acceptance failure artifact: %v", err)
		return ""
	}

	return fullPath
}

func (r *Runner) logResult(result *IterationResult) {
	if r == nil || result == nil {
		return
	}
	if result.Decomposed && result.Error == nil {
		r.log("DECOMPOSED: %s into sub-beads in %v", result.BeadID, result.Duration)
		return
	}
	if result.AlreadyDone {
		r.log("ALREADY DONE: %s — ATDD tests pass, work previously completed (%v)", result.BeadID, result.Duration)
		return
	}
	if result.Success {
		r.log("SUCCESS: %s completed in %v", result.BeadID, result.Duration)
		if result.Escalated {
			r.log("  (escalated to %s)", result.EscalatedTo)
		}
		if result.Validated {
			r.log("  (validation passed)")
		}
	} else {
		r.log("FAILED: %s - %v", result.BeadID, result.Error)
		if result.AcceptanceFailureSummary != "" {
			r.log("  acceptance summary: %s", result.AcceptanceFailureSummary)
		}
		if line := firstNonEmptyLine(result.AcceptanceFailureOutput); line != "" {
			r.log("  acceptance output: %s", line)
		}
	}
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func (r *Runner) log(format string, args ...any) {
	if r.output == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	_, _ = fmt.Fprint(r.output, msg) // best-effort logging, explicitly discard error
}
