package visionmetrics

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

func Validate(rec Record) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(rec.SpecID) == "" {
		errs = append(errs, ValidationError{Field: "spec_id", Reason: "required"})
	}
	if rec.CycleStartTriggerAt.IsZero() {
		errs = append(errs, ValidationError{Field: "cycle_start_trigger_at", Reason: "required"})
	}
	if rec.CycleEndPresentedAt.IsZero() {
		errs = append(errs, ValidationError{Field: "cycle_end_presented_at", Reason: "required"})
	}
	if !rec.CycleStartTriggerAt.IsZero() && !rec.CycleEndPresentedAt.IsZero() && rec.CycleEndPresentedAt.Before(rec.CycleStartTriggerAt) {
		errs = append(errs, ValidationError{Field: FieldCycleEndPresentedAt, Reason: "must be after cycle start"})
	}
	if !rec.ReviewOutcome.Valid() {
		errs = append(errs, ValidationError{Field: "review_outcome", Reason: "invalid"})
	}
	if !rec.HumanTacticalIntervention.Valid() {
		errs = append(errs, ValidationError{Field: "human_tactical_intervention", Reason: "must be yes or no"})
	}
	if !rec.HumanDebuggingIntervention.Valid() {
		errs = append(errs, ValidationError{Field: "human_debugging_intervention", Reason: "must be yes or no"})
	}
	if !isValidEscapedRegressionStatus(rec.EscapedRegressionWithin7D) {
		errs = append(errs, ValidationError{Field: FieldEscapedRegressionWithin7D, Reason: "must be yes, no, or pending"})
	}
	if rec.HumanDebuggingIntervention == Yes && rec.HumanTacticalIntervention != Yes {
		errs = append(errs, ValidationError{Field: "human_debugging_intervention", Reason: "requires tactical intervention"})
	}
	if rec.ReviewOutcome == ReviewOutcomeVisionChange && strings.TrimSpace(rec.ReviewRationale) == "" {
		errs = append(errs, ValidationError{Field: "review_rationale", Reason: "required for vision-change reworks"})
	}

	return errs
}

func isValidEscapedRegressionStatus(val YesNo) bool {
	switch val {
	case Yes, No, EscapedRegressionPending:
		return true
	default:
		return false
	}
}
