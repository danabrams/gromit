package visionmetrics

import "time"

// YesNo represents a boolean-like field that accepts only "yes" or "no" at the contract layer.
type YesNo string

const (
	Yes YesNo = "yes"
	No  YesNo = "no"
)

func (y YesNo) Valid() bool {
	return y == Yes || y == No
}

// ReviewOutcome captures the final review disposition for a cycle.
type ReviewOutcome string

const (
	ReviewOutcomeAccepted          ReviewOutcome = "accepted"
	ReviewOutcomeImplementationGap ReviewOutcome = "rework_implementation_gap"
	ReviewOutcomeVisionChange      ReviewOutcome = "rework_vision_change"
)

func (r ReviewOutcome) Valid() bool {
	switch r {
	case ReviewOutcomeAccepted, ReviewOutcomeImplementationGap, ReviewOutcomeVisionChange:
		return true
	default:
		return false
	}
}

func (r ReviewOutcome) IsCarveOut() bool {
	return r == ReviewOutcomeVisionChange
}

func (r ReviewOutcome) IsAccepted() bool {
	return r == ReviewOutcomeAccepted
}

// Record represents a validated cycle record at PR presentation time.
type Record struct {
	SpecID                     string        `json:"spec_id"`
	CycleStartTriggerAt        time.Time     `json:"cycle_start_trigger_at"`
	CycleEndPresentedAt        time.Time     `json:"cycle_end_presented_at"`
	ReviewOutcome              ReviewOutcome `json:"review_outcome"`
	ReviewRationale            string        `json:"review_rationale,omitempty"`
	HumanTacticalIntervention  YesNo         `json:"human_tactical_intervention"`
	HumanDebuggingIntervention YesNo         `json:"human_debugging_intervention"`
	EscapedRegressionWithin7D  YesNo         `json:"escaped_regression_within_7d"`
}
