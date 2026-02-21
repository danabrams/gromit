package escalation

import (
	"context"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// firstNonNilContext returns the first non-nil context from the provided list,
// falling back to context.Background() when all are nil.
func firstNonNilContext(contexts ...context.Context) context.Context {
	for _, ctx := range contexts {
		if ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

func (h *Handler) checkRetryBudgetBeforeAttempt(ctx context.Context, bc *runtypes.BeadContext) (handled bool, err error) {
	if bc == nil {
		return false, fmt.Errorf("bead context is nil")
	}
	if bc.MaxAttemptsPerBead > 0 && bc.AttemptsThisBead >= bc.MaxAttemptsPerBead {
		return false, fmt.Errorf("retry budget exceeded: attempts %d/%d", bc.AttemptsThisBead, bc.MaxAttemptsPerBead)
	}
	if bc.BeadTimeout > 0 && !bc.BeadStartTime.IsZero() {
		elapsed := time.Since(bc.BeadStartTime)
		if elapsed >= bc.BeadTimeout {
			return false, fmt.Errorf("retry budget exceeded: bead wall-clock %s reached (timeout=%s)", elapsed.Round(time.Second), bc.BeadTimeout)
		}
	}
	if h != nil && h.cfg != nil {
		tokenBudgetCap := h.cfg.Claude.TokenBudgetForModel(bc.Model)
		if tokenBudgetCap > 0 && bc.CumulativeInputTokens > tokenBudgetCap {
			failureReason := fmt.Sprintf(
				"retry budget exceeded: cumulative input tokens %d/%d",
				bc.CumulativeInputTokens,
				tokenBudgetCap,
			)
			decomposeCtx := firstNonNilContext(bc.ParentCtx, ctx)
			if decomposeCtx.Err() != nil {
				return false, fmt.Errorf("%s (decomposition skipped: parent context canceled: %w)", failureReason, decomposeCtx.Err())
			}
			h.AttemptDecomposition(decomposeCtx, bc, failureReason)
			return true, nil
		}
	}
	return false, nil
}

func (h *Handler) checkRetryBudgetAfterFailure(bc *runtypes.BeadContext) error {
	if bc == nil {
		return fmt.Errorf("bead context is nil")
	}
	if bc.MaxAttemptsPerBead > 0 && bc.AttemptsThisBead >= bc.MaxAttemptsPerBead {
		return fmt.Errorf("retry budget exceeded after failed attempt: %d/%d", bc.AttemptsThisBead, bc.MaxAttemptsPerBead)
	}
	if bc.BeadTimeout > 0 && !bc.BeadStartTime.IsZero() {
		elapsed := time.Since(bc.BeadStartTime)
		if elapsed >= bc.BeadTimeout {
			return fmt.Errorf("retry budget exceeded after failed attempt: bead wall-clock %s reached (timeout=%s)", elapsed.Round(time.Second), bc.BeadTimeout)
		}
	}
	return nil
}
