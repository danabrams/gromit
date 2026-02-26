package logger

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/failurephase"
)

const (
	failureAttributionSystem    = "system"
	failureAttributionModel     = "model"
	failureAttributionTransient = "transient"
	sameScopeRetryBlockedMessage = "Same-scope retry blocked: timeout requires decomposition or escalation decision"
	partialDecompositionStateMessage = "Partial/unsafe decomposition state: retry or escalate before continuing"
)

func readAllIterationLogsSorted(logsDir string) ([]IterationLog, error) {
	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	entries := make([]IterationLog, 0, 128)
	for _, f := range files {
		logs, err := readLogFile(f)
		if err != nil {
			continue
		}
		entries = append(entries, logs...)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Timestamp.Equal(entries[j].Timestamp) {
			if entries[i].Iteration == entries[j].Iteration {
				return entries[i].BeadID < entries[j].BeadID
			}
			return entries[i].Iteration < entries[j].Iteration
		}
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	return entries, nil
}

func buildIterationMetrics(entries []IterationLog, windowSize int) []IterationMetric {
	if len(entries) == 0 {
		return []IterationMetric{}
	}
	beadIndices := buildBeadEntryIndices(entries)

	metrics := make([]IterationMetric, 0, len(entries))
	successValues := make([]float64, 0, len(entries))
	costValues := make([]float64, 0, len(entries))
	durationValues := make([]float64, 0, len(entries))
	inputTokenValues := make([]float64, 0, len(entries))
	var (
		prevSuccessZ  float64
		prevCostZ     float64
		prevDurationZ float64
		prevInputZ    float64
		hasPrevious   bool
	)
	for idx, entry := range entries {
		windowStart := idx - windowSize + 1
		if windowStart < 0 {
			windowStart = 0
		}
		window := entries[windowStart : idx+1]
		w := summarizeWindow(window)
		successValue := boolToFloat64(entry.Success)
		costValue := entry.CostUSD
		durationValue := float64(entry.DurationMs)
		inputTokenValue := float64(entry.InputTokens)
		successValues = append(successValues, successValue)
		costValues = append(costValues, costValue)
		durationValues = append(durationValues, durationValue)
		inputTokenValues = append(inputTokenValues, inputTokenValue)

		successEWMA := computeEWMAState(metricEWMASuccessRate, successValue, successValues, prevSuccessZ, hasPrevious)
		costEWMA := computeEWMAState(metricEWMACostUSD, costValue, costValues, prevCostZ, hasPrevious)
		durationEWMA := computeEWMAState(metricEWMADurationMs, durationValue, durationValues, prevDurationZ, hasPrevious)
		inputTokenEWMA := computeEWMAState(metricEWMAInputTokens, inputTokenValue, inputTokenValues, prevInputZ, hasPrevious)
		prevSuccessZ = successEWMA.Z
		prevCostZ = costEWMA.Z
		prevDurationZ = durationEWMA.Z
		prevInputZ = inputTokenEWMA.Z
		hasPrevious = true

		metrics = append(metrics, IterationMetric{
			Timestamp:         entry.Timestamp,
			Iteration:         entry.Iteration,
			BeadID:            entry.BeadID,
			Model:             entry.Model,
			ReasoningEffort:   entry.ReasoningEffort,
			Provider:          entry.Provider,
			FailurePhase:      entry.FailurePhase,
			DefectOriginPhase: deriveDefectOriginPhase(entry),
			FailureCategory:   entry.FailureCategory,
			FailureAttribution: classifyFailureAttribution(
				entries,
				beadIndices[entry.BeadID],
				idx,
			),
			Success:          entry.Success,
			FirstPassSuccess: entry.FirstPassSuccess,
			Escalated:        entry.Escalated,
			QualityScore: ComputeQualityScore(
				entry.CriteriaTotal,
				entry.CriteriaCovered,
				entry.ValidationRetried,
				entry.TrivialAutoFixed,
				entry.Escalated,
				entry.ReviewFixesNeeded,
			),
			DurationMs:                   entry.DurationMs,
			ValidationDurationMs:         entry.ValidationDurationMs,
			CostUSD:                      entry.CostUSD,
			InputTokens:                  entry.InputTokens,
			OutputTokens:                 entry.OutputTokens,
			MTTRProxyMs:                  entry.MTTRProxyMs,
			FilesTouched:                 entry.FilesTouched,
			PromptDiagnostics:            entry.PromptDiagnostics,
			RollingSuccessRate:           w.SuccessRate,
			RollingFailureRate:           w.FailureRate,
			RollingFirstPassSuccess:      w.FirstPassSuccess,
			RollingReworkRate:            w.ReworkRate,
			RollingEscalationRate:        w.EscalationRate,
			RollingQualityScore:          w.QualityScore,
			RollingAvgDurationMs:         w.AvgDurationMs,
			RollingP95DurationMs:         w.P95DurationMs,
			RollingAvgValidationMs:       w.AvgValidationMs,
			RollingP95ValidationMs:       w.P95ValidationMs,
			RollingAvgCostUSD:            w.AvgCostUSD,
			RollingAvgInputTokens:        w.AvgInputTokens,
			RollingAvgCostPerBeadUSD:     w.AvgCostPerBeadUSD,
			RollingAvgMTTRProxyMs:        w.AvgMTTRProxyMs,
			RollingPreflightFailureRate:  w.PreflightFailureRate,
			RollingBuildFailureRate:      w.BuildFailureRate,
			RollingValidationFailureRate: w.ValidationFailureRate,
			RollingTimeoutFailureRate:    w.TimeoutFailureRate,
			RollingTimeoutDecompositionAttempts: w.TimeoutDecompositionAttempts,
			RollingTimeoutDecompositionSuccessRate: w.TimeoutDecompositionSuccessRate,
			RollingTimeoutRetryBlockCount: w.TimeoutRetryBlockCount,
			RollingTimeoutRetryBlockRate:  w.TimeoutRetryBlockRate,
			TimeoutType:                  entry.TimeoutType,
			TimeoutDecompositionAttempted: entry.TimeoutDecompositionAttempted,
			TimeoutDecompositionSucceeded: entry.TimeoutDecompositionSucceeded,
			TimeoutDecompositionOutcome:   entry.TimeoutDecompositionOutcome,
			TimeoutDecompositionReason:    entry.TimeoutDecompositionReason,
			EWMASuccessRate:              successEWMA,
			EWMACostUSD:                  costEWMA,
			EWMADurationMs:               durationEWMA,
			EWMAInputTokens:              inputTokenEWMA,
		})
	}

	return metrics
}

func buildBeadEntryIndices(entries []IterationLog) map[string][]int {
	indices := make(map[string][]int, len(entries))
	for i := range entries {
		indices[entries[i].BeadID] = append(indices[entries[i].BeadID], i)
	}
	return indices
}

func classifyFailureAttribution(entries []IterationLog, beadSeries []int, idx int) string {
	if idx < 0 || idx >= len(entries) {
		return ""
	}
	entry := entries[idx]
	if entry.Success {
		return ""
	}
	if isTransientFailureSignal(entry) {
		return failureAttributionTransient
	}
	seriesPos := indexInSeries(beadSeries, idx)
	if seriesPos < 0 {
		return failureAttributionModel
	}
	if isSingleFailureThenSameTierSuccess(entries, beadSeries, seriesPos) {
		return failureAttributionTransient
	}
	if hasRepeatedCrossTierFailures(entries, beadSeries, seriesPos) {
		return failureAttributionSystem
	}
	return failureAttributionModel
}

func deriveDefectOriginPhase(entry IterationLog) string {
	if entry.Success {
		return ""
	}
	if entry.CompilationErrors {
		return failurephase.Build
	}
	return entry.FailurePhase
}

func isTransientFailureSignal(entry IterationLog) bool {
	if entry.FailurePhase == failurephase.Timeout || entry.TimeoutType != "" {
		return true
	}
	switch entry.FailureCategory {
	case transportDisconnectFailure, "rate_limited", "startup_error":
		return true
	default:
		return false
	}
}

func indexInSeries(series []int, idx int) int {
	for pos, entryIdx := range series {
		if entryIdx == idx {
			return pos
		}
	}
	return -1
}

func isSingleFailureThenSameTierSuccess(entries []IterationLog, series []int, pos int) bool {
	if pos < 0 || pos >= len(series) {
		return false
	}
	if pos+1 >= len(series) {
		return false
	}
	current := entries[series[pos]]
	next := entries[series[pos+1]]
	if !next.Success {
		return false
	}
	currentTier := resolvedTier(current)
	nextTier := resolvedTier(next)
	return currentTier != "" && currentTier == nextTier
}

func hasRepeatedCrossTierFailures(entries []IterationLog, series []int, pos int) bool {
	if pos < 0 || pos >= len(series) {
		return false
	}
	runStart := pos
	for runStart > 0 && !entries[series[runStart-1]].Success {
		runStart--
	}
	runEnd := pos
	for runEnd+1 < len(series) && !entries[series[runEnd+1]].Success {
		runEnd++
	}
	if runEnd-runStart < 1 {
		return false
	}
	tiers := map[string]struct{}{}
	for i := runStart; i <= runEnd; i++ {
		tier := resolvedTier(entries[series[i]])
		if tier == "" {
			continue
		}
		tiers[tier] = struct{}{}
	}
	return len(tiers) > 1
}

func resolvedTier(entry IterationLog) string {
	if entry.ActualTier != "" {
		return entry.ActualTier
	}
	return entry.Model
}

func summarizeWindow(window []IterationLog) ProcessTrendWindow {
	if len(window) == 0 {
		return ProcessTrendWindow{}
	}

	var (
		successes, firstPasses, escalations int
		preflightFailures, buildFailures, validationFailures, timeoutFailures int
		timeoutDecompositionAttempts int
		timeoutDecompositionSuccesses int
		timeoutRetryBlockCount int
	)
	var durationTotal int64
	var validationDurationTotal int64
	var validationDurationCount int
	var costTotal float64
	var qualityTotal float64
	var inputTokenTotal int64
	var mttrTotal int64
	var mttrCount int
	durations := make([]int64, 0, len(window))
	validationDurations := make([]int64, 0, len(window))
	beadCosts := map[string]beadCostAccum{}

	for _, e := range window {
		if e.Success {
			successes++
		}
		if e.FirstPassSuccess {
			firstPasses++
		}
		if e.Escalated {
			escalations++
		}
		if !e.Success {
			switch e.FailurePhase {
			case failurephase.Preflight:
				preflightFailures++
			case failurephase.Build:
				buildFailures++
			case failurephase.Validation:
				validationFailures++
			case failurephase.Timeout:
				timeoutFailures++
			}
		}

		durationTotal += e.DurationMs
		if e.ValidationDurationMs > 0 {
			validationDurationTotal += e.ValidationDurationMs
			validationDurationCount++
			validationDurations = append(validationDurations, e.ValidationDurationMs)
		}
		costTotal += e.CostUSD
		qualityTotal += ComputeQualityScore(
			e.CriteriaTotal,
			e.CriteriaCovered,
			e.ValidationRetried,
			e.TrivialAutoFixed,
			e.Escalated,
			e.ReviewFixesNeeded,
		)
		inputTokenTotal += int64(e.InputTokens)
		durations = append(durations, e.DurationMs)

		updateBeadCostAccum(beadCosts, e)

		if e.MTTRProxyMs > 0 {
			mttrTotal += e.MTTRProxyMs
			mttrCount++
		}
		if e.TimeoutDecompositionAttempted {
			timeoutDecompositionAttempts++
			if e.TimeoutDecompositionSucceeded {
				timeoutDecompositionSuccesses++
			}
		}
		if isSameScopeRetryBlocked(e.Error) {
			timeoutRetryBlockCount++
		}
	}

	totalIterations := len(window)
	count := float64(totalIterations)
	reworkIterations := totalIterations - firstPasses
	avgDuration := float64(durationTotal) / count
	avgValidationDuration := 0.0
	if validationDurationCount > 0 {
		avgValidationDuration = float64(validationDurationTotal) / float64(validationDurationCount)
	}
	avgCost := costTotal / count
	avgInputTokens := float64(inputTokenTotal) / count
	avgMTTR := 0.0
	if mttrCount > 0 {
		avgMTTR = float64(mttrTotal) / float64(mttrCount)
	}

	avgCostPerBead := averageCompletedBeadCost(beadCosts)

	return ProcessTrendWindow{
		SuccessRate:           float64(successes) / count,
		FailureRate:           float64(len(window)-successes) / count,
		FirstPassSuccess:      float64(firstPasses) / count,
		ReworkRate:            float64(reworkIterations) / count,
		EscalationRate:        float64(escalations) / count,
		QualityScore:          qualityTotal / count,
		AvgDurationMs:         avgDuration,
		P95DurationMs:         percentileInt64(durations, p95Percentile),
		AvgValidationMs:       avgValidationDuration,
		P95ValidationMs:       percentileInt64(validationDurations, p95Percentile),
		AvgCostUSD:            avgCost,
		AvgInputTokens:        avgInputTokens,
		AvgCostPerBeadUSD:     avgCostPerBead,
		AvgMTTRProxyMs:        avgMTTR,
		PreflightFailureRate:  float64(preflightFailures) / count,
		BuildFailureRate:      float64(buildFailures) / count,
		ValidationFailureRate: float64(validationFailures) / count,
		TimeoutFailureRate:    float64(timeoutFailures) / count,
		TimeoutDecompositionAttempts: timeoutDecompositionAttempts,
		TimeoutDecompositionSuccessRate: func() float64 {
			if timeoutDecompositionAttempts == 0 {
				return 0
			}
			return float64(timeoutDecompositionSuccesses) / float64(timeoutDecompositionAttempts)
		}(),
		TimeoutRetryBlockCount: timeoutRetryBlockCount,
		TimeoutRetryBlockRate:  float64(timeoutRetryBlockCount) / count,
	}
}

type beadCostAccum struct {
	totalCost  float64
	hasSuccess bool
}

func updateBeadCostAccum(beadCosts map[string]beadCostAccum, entry IterationLog) {
	if entry.BeadID == "" {
		return
	}
	accum := beadCosts[entry.BeadID]
	accum.totalCost += entry.CostUSD
	if entry.Success {
		accum.hasSuccess = true
	}
	beadCosts[entry.BeadID] = accum
}

// averageCompletedBeadCost returns average total bead cost for beads completed in the window.
func averageCompletedBeadCost(beadCosts map[string]beadCostAccum) float64 {
	var completedBeadCostSum float64
	var completedBeadCount int
	for _, accum := range beadCosts {
		if accum.hasSuccess {
			completedBeadCostSum += accum.totalCost
			completedBeadCount++
		}
	}
	if completedBeadCount == 0 {
		return 0
	}
	return completedBeadCostSum / float64(completedBeadCount)
}

func isSameScopeRetryBlocked(err string) bool {
	return strings.Contains(err, sameScopeRetryBlockedMessage) ||
		strings.Contains(err, partialDecompositionStateMessage)
}
