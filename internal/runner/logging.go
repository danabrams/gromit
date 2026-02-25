package runner

import (
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// ResultToIterationLog converts an IterationResult to an IterationLog.
// It maps all relevant fields including experiment tracking fields.
func ResultToIterationLog(result *runtypes.IterationResult) *logger.IterationLog {
	if result == nil {
		return &logger.IterationLog{}
	}

	return &logger.IterationLog{
		Timestamp:                 time.Now(),
		BeadID:                    result.BeadID,
		BeadTitle:                 result.BeadTitle,
		SpecID:                    result.SpecID,
		Model:                     result.Model,
		ReasoningEffort:           result.ReasoningEffort,
		Provider:                  result.Provider,
		FailurePhase:              result.FailurePhase,
		FailureCategory:           result.FailureCategory,
		Success:                   result.Success,
		Validated:                 result.Validated,
		Escalated:                 result.Escalated,
		EscalatedTo:               result.EscalatedTo,
		OriginalTier:              result.OriginalTier,
		ActualTier:                result.ActualTier,
		DurationMs:                result.Duration.Milliseconds(),
		CostUSD:                   result.CostUSD,
		InputTokens:               result.InputTokens,
		OutputTokens:              result.OutputTokens,
		Error:                     errorToString(result.Error),
		ValidationRetried:         result.ValidationRetried,
		TrivialAutoFixed:          result.TrivialAutoFixed,
		UsageLimited:              result.UsageLimited,
		ValidationMode:            result.ValidationMode,
		ValidationDurationMs:      result.ValidationDurationMs,
		ValidationTimeouts:        result.ValidationTimeouts,
		CompilationErrors:         result.CompilationErrors,
		TimeoutDecompositionAttempted: result.TimeoutDecompositionAttempted,
		TimeoutDecompositionSucceeded: result.TimeoutDecompositionSucceeded,
		TimeoutType:               result.TimeoutType,
		TimeoutPhase:              result.TimeoutPhase,
		TimeToFirstEventMs:        result.TimeToFirstEventMs,
		ToolCallCount:             result.ToolCallCount,
		StallCount:                result.StallCount,
		StallTier:                 result.StallTier,
		RateLimitHits:             result.RateLimitHits,
		RateLimitRecoveryMs:       result.RateLimitRecoveryMs,
		CacheHit:                  result.CacheHit,
		CacheMiss:                 result.CacheMiss,
		CacheWrite:                result.CacheWrite,
		CacheClass:                result.CacheClass,
		CacheKey:                  result.CacheKey,
		CacheInvalidationReason:   result.CacheInvalidationReason,
		CacheVersionMarker:        result.CacheVersionMarker,
		UtilityRoutingCategory:    result.UtilityRoutingCategory,
		UtilityRoutingTier:        result.UtilityRoutingTier,
		FallbackAttempts:          result.FallbackAttempts,
		FallbackSuccesses:         result.FallbackSuccesses,
		FallbackFailures:          result.FallbackFailures,
		FailureLayer:              result.FailureLayer,
		FailureSubCat:             result.FailureSubCat,
		FailureClass:              result.FailureClass,
		AndonLevel:                result.AndonLevel,
		TrimDecision:              result.TrimDecision,
		AutonomyEligible:          result.AutonomyEligible,
		AutonomySuccess:           result.AutonomySuccess,
		FirstPassSuccess:          result.FirstPassSuccess,
		FilesTouched:              result.FilesTouched,
		TouchedPackages:           result.TouchedPackages,
		MTTRProxyMs:               result.MTTRProxyMs,
		EscalationClass:           result.EscalationClass,
		RecurrenceCount:           result.RecurrenceCount,
		CriteriaTotal:             result.CriteriaTotal,
		CriteriaCovered:           result.CriteriaCovered,
		CriteriaUntestable:        result.CriteriaUntestable,
		UncoveredCriteria:         result.UncoveredCriteria,
		PromptDiagnostics:         result.PromptDiagnostics,
		ExperimentID:              result.ExperimentID,
		VariantID:                 result.VariantID,
	}
}

// errorToString safely converts an error to a string, handling nil errors.
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
