package escalation

import (
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func (h *Handler) setBuildFailurePhase(bc *runtypes.BeadContext) {
	if bc == nil || bc.Result == nil {
		return
	}
	bc.Result.FailurePhase = failurephase.Build
}

func (h *Handler) setBuildTimeoutFailurePhase(bc *runtypes.BeadContext) {
	if bc == nil || bc.Result == nil {
		return
	}
	bc.Result.FailurePhase = failurephase.Timeout
	bc.Result.TimeoutPhase = "build"
}

func (h *Handler) addInvocationTokensToCumulative(bc *runtypes.BeadContext) {
	if bc == nil || bc.Result == nil {
		return
	}
	bc.CumulativeInputTokens += bc.Result.InputTokens
	bc.CumulativeOutputTokens += bc.Result.OutputTokens
}

func (h *Handler) applyTriageClassification(bc *runtypes.BeadContext, triageResult *TriageResult) {
	if bc == nil || bc.Result == nil || triageResult == nil {
		return
	}
	bc.Result.FailureLayer = string(triageResult.Layer)
	bc.Result.FailureSubCat = triageResult.SubCategory
}
