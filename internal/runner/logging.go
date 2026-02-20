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
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

var artifactBeadIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

const (
	acceptanceFailureDirName = "failures"
	artifactFallbackBeadID   = "unknown-bead"
	artifactDirPerm          = 0o755
	artifactFilePerm         = 0o644
	tddPhaseRecordType       = "tdd_phase"
	tddSummaryRecordType     = "tdd_summary"
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

	r.logIterationWithWarning(newIterationLogEntry(iteration, result, errStr, outcome, artifactPath))
	if len(result.PhaseMetrics) > 0 {
		r.writeTDDMetrics(result.BeadID, result.Success, result.PhaseMetrics)
	}
}

type tddMetricsLogger interface {
	LogTDDPhase(rec *logger.TDDPhaseRecord) error
	LogTDDSummary(rec *logger.TDDSummaryRecord) error
}

func (r *Runner) writeTDDMetrics(beadID string, success bool, phaseMetrics []runtypes.PhaseMetric) {
	if r == nil || r.logger == nil || len(phaseMetrics) == 0 {
		return
	}

	tddLogger, ok := r.logger.(tddMetricsLogger)
	if !ok {
		return
	}

	phaseTotals := make(map[string]int, len(phaseMetrics))
	phaseSuccess := make(map[string]int, len(phaseMetrics))
	cycles := make(map[int]struct{}, len(phaseMetrics))
	var totalInputTokens int
	var totalOutputTokens int
	var totalDurationMs int64
	var escalationCount int

	for _, metric := range phaseMetrics {
		if beadID == "" && metric.BeadID != "" {
			beadID = metric.BeadID
		}

		if err := tddLogger.LogTDDPhase(newTDDPhaseRecord(metric)); err != nil {
			r.log("Warning: failed to write TDD phase log: %v", err)
			return
		}

		cycles[metric.CycleNumber] = struct{}{}
		phaseTotals[metric.Phase]++
		if metric.Success {
			phaseSuccess[metric.Phase]++
		}
		totalInputTokens += metric.InputTokens
		totalOutputTokens += metric.OutputTokens
		totalDurationMs += metric.DurationMs
		if metric.Escalated {
			escalationCount++
		}
	}

	phaseSuccessRates := calculatePhaseSuccessRates(phaseTotals, phaseSuccess)

	if beadID == "" {
		beadID = resultBeadIDFromMetrics(phaseMetrics)
	}
	if beadID == "" {
		beadID = artifactFallbackBeadID
	}

	if err := tddLogger.LogTDDSummary(&logger.TDDSummaryRecord{
		Type:              tddSummaryRecordType,
		Timestamp:         time.Now(),
		BeadID:            beadID,
		TotalCycles:       len(cycles),
		TotalPhases:       len(phaseMetrics),
		Success:           success,
		TotalDurationMs:   totalDurationMs,
		TotalInputTokens:  totalInputTokens,
		TotalOutputTokens: totalOutputTokens,
		PhaseSuccessRates: phaseSuccessRates,
		EscalationCount:   escalationCount,
	}); err != nil {
		r.log("Warning: failed to write TDD summary log: %v", err)
	}
}

func newTDDPhaseRecord(metric runtypes.PhaseMetric) *logger.TDDPhaseRecord {
	return &logger.TDDPhaseRecord{
		Type:               tddPhaseRecordType,
		Timestamp:          time.Now(),
		BeadID:             metric.BeadID,
		Phase:              metric.Phase,
		CycleNumber:        metric.CycleNumber,
		Model:              metric.Model,
		Tier:               metric.Tier,
		InputTokens:        metric.InputTokens,
		OutputTokens:       metric.OutputTokens,
		DurationMs:         metric.DurationMs,
		Success:            metric.Success,
		Escalated:          metric.Escalated,
		EscalatedFrom:      metric.EscalatedFrom,
		CriteriaTotal:      metric.CriteriaTotal,
		CriteriaCovered:    metric.CriteriaCovered,
		CriteriaUntestable: metric.CriteriaUntestable,
	}
}

func calculatePhaseSuccessRates(phaseTotals, phaseSuccess map[string]int) map[string]float64 {
	rates := make(map[string]float64, len(phaseTotals))
	for phase, total := range phaseTotals {
		if total == 0 {
			rates[phase] = 0
			continue
		}
		rates[phase] = float64(phaseSuccess[phase]) / float64(total)
	}
	return rates
}

func resultBeadIDFromMetrics(metrics []runtypes.PhaseMetric) string {
	for _, metric := range metrics {
		if metric.BeadID != "" {
			return metric.BeadID
		}
	}
	return ""
}

func newIterationLogEntry(iteration int, result *IterationResult, errStr, outcome, artifactPath string) *logger.IterationLog {
	return &logger.IterationLog{
		Timestamp:                 time.Now(),
		Iteration:                 iteration,
		BeadID:                    result.BeadID,
		BeadTitle:                 result.BeadTitle,
		SpecID:                    result.SpecID,
		Model:                     result.Model,
		Provider:                  result.Provider,
		FailurePhase:              result.FailurePhase,
		FailureCategory:           result.FailureCategory,
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
		ValidationDurationMs:      result.ValidationDurationMs,
		CompilationErrors:         result.CompilationErrors,
		HardStopPendingApproval:   result.HardStopPendingApproval,
		TimeoutType:               result.TimeoutType,
		TimeoutPhase:              result.TimeoutPhase,
		TimeToFirstEventMs:        result.TimeToFirstEventMs,
		ToolCallCount:             result.ToolCallCount,
		StallCount:                result.StallCount,
		StallTier:                 result.StallTier,
		RateLimitHits:             result.RateLimitHits,
		RateLimitRecoveryMs:       result.RateLimitRecoveryMs,
		AcceptanceFailureSummary:  result.AcceptanceFailureSummary,
		AcceptanceFailureOutput:   result.AcceptanceFailureOutput,
		AcceptanceFailureArtifact: artifactPath,
		AcceptanceFailureExitCode: result.AcceptanceFailureExitCode,
		FailureClass:              string(result.FailureClass),
		AndonLevel:                string(result.AndonLevel),
		TrimDecision:              result.TrimDecision,
		AutonomyEligible:          result.AutonomyEligible,
		AutonomySuccess:           result.AutonomySuccess,
		FirstPassSuccess:          result.FirstPassSuccess,
		FilesTouched:              result.FilesTouched,
		TouchedPackages:           result.TouchedPackages,
		MTTRProxyMs:               result.MTTRProxyMs,
		EscalationClass:           string(result.EscalationClass),
		RecurrenceCount:           result.RecurrenceCount,
		CriteriaTotal:             result.CriteriaTotal,
		CriteriaCovered:           result.CriteriaCovered,
		CriteriaUntestable:        result.CriteriaUntestable,
		UncoveredCriteria:         result.UncoveredCriteria,
		PromptDiagnostics:         result.PromptDiagnostics,
	}
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
		"timestamp: %s\niteration: %d\nbead_id: %s\nbead_title: %s\nsummary: %s\nerror: %v\nexit_code: %d\n\nvalidation_output:\n%s\n",
		time.Now().UTC().Format(time.RFC3339),
		iteration,
		result.BeadID,
		result.BeadTitle,
		result.AcceptanceFailureSummary,
		result.Error,
		result.AcceptanceFailureExitCode,
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
		if result.TimeoutPhase != "" {
			r.log("  phase: %s", result.TimeoutPhase)
		}
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
