package runner

import (
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/logger"
)

func (r *Runner) writeIterationLog(iteration int, result *IterationResult) {
	if r == nil || r.logger == nil || result == nil {
		return
	}

	errStr := ""
	if result.Error != nil {
		errStr = result.Error.Error()
	}

	outcome := ""
	if result.AlreadyDone {
		outcome = "atdd_already_done"
	}

	r.logger.LogIteration(&logger.IterationLog{
		Timestamp:           time.Now(),
		Iteration:           iteration,
		BeadID:              result.BeadID,
		BeadTitle:           result.BeadTitle,
		Model:               result.Model,
		Success:             result.Success,
		Validated:           result.Validated,
		Escalated:           result.Escalated,
		EscalatedTo:         result.EscalatedTo,
		DurationMs:          result.Duration.Milliseconds(),
		CostUSD:             result.CostUSD,
		InputTokens:         result.InputTokens,
		OutputTokens:        result.OutputTokens,
		Error:               errStr,
		Outcome:             outcome,
		ValidationRetried:   result.ValidationRetried,
		TrivialAutoFixed:    result.TrivialAutoFixed,
		UsageLimited:        result.UsageLimited,
		ValidationMode:      result.ValidationMode,
		TimeoutType:         result.TimeoutType,
		TimeToFirstEventMs:  result.TimeToFirstEventMs,
		ToolCallCount:       result.ToolCallCount,
		StallCount:          result.StallCount,
		StallTier:           result.StallTier,
		RateLimitHits:       result.RateLimitHits,
		RateLimitRecoveryMs: result.RateLimitRecoveryMs,
	})
}

func (r *Runner) logResult(result *IterationResult) {
	if r == nil || result == nil {
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
	}
}

func (r *Runner) log(format string, args ...any) {
	if r.output == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprint(r.output, msg)
}
