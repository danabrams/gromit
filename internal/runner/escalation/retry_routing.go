package escalation

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func (h *Handler) shouldEscalateAsSpecialCause(bc *runtypes.BeadContext) bool {
	if bc == nil {
		return true
	}
	// Repeated failures on the same bead are treated as special cause.
	if bc.RetriesThisModel > 0 {
		return true
	}
	limit, ok := h.readModelBuildFailureControlLimit(bc.Model)
	if !ok {
		return false
	}
	return limit.Latest > limit.UCL || limit.Latest < limit.LCL
}

func (h *Handler) readModelBuildFailureControlLimit(model string) (logger.TrendControlLimit, bool) {
	if h == nil || h.cfg == nil {
		return logger.TrendControlLimit{}, false
	}
	trendPath := filepath.Join(h.cfg.Paths.GromitDir, processTrendMetricsDir, processTrendFileName)
	trend, err := logger.ReadProcessTrend(trendPath)
	if err != nil || trend == nil {
		return logger.TrendControlLimit{}, false
	}
	modelKey := modelStratumPrefix + strings.ToLower(strings.TrimSpace(model))
	limits, ok := trend.StratifiedControlLimits[modelKey]
	if !ok {
		return logger.TrendControlLimit{}, false
	}
	for _, limit := range limits {
		if limit.Metric == metricBuildFailureRate {
			return limit, true
		}
	}
	return logger.TrendControlLimit{}, false
}

func (h *Handler) incrementRetryCounters(bc *runtypes.BeadContext) bool {
	bc.RetriesThisModel++
	bc.TotalRetriesThisBead++
	if bc.TotalRetriesThisBead > bc.MaxRetriesPerBead {
		h.log("Max retries per bead exceeded (%d/%d)", bc.TotalRetriesThisBead, bc.MaxRetriesPerBead)
		bc.Result.Error = fmt.Errorf("build failed: exceeded max retries per bead (%d)", bc.MaxRetriesPerBead)
		return false
	}
	return true
}

func (h *Handler) setRetryPromptContext(bc *runtypes.BeadContext, failureContext string) {
	if bc.PromptCtx == nil {
		return
	}
	bc.PromptCtx.IsRetry = true
	bc.PromptCtx.FailureContext = h.truncateFailureContext(failureContext)
}

func (h *Handler) retryCommonCauseFailure(bc *runtypes.BeadContext, failureContext string) bool {
	if bc == nil || bc.Result == nil {
		return false
	}
	l1RetryCap := h.resolveL1RetryCap(bc)
	if bc.RetriesThisModel >= l1RetryCap {
		return false
	}
	if !h.incrementRetryCounters(bc) {
		return false
	}
	h.setRetryPromptContext(bc, failureContext)
	h.log("Common-cause failure: retrying on same tier (attempt %d/%d)", bc.RetriesThisModel, l1RetryCap)
	return true
}
