package visionmetrics

import (
	"fmt"
	"regexp"
	"strings"
	"time"
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

// ParseFromPRBody extracts vision metrics metadata from a PR body.
// It looks for key: value pairs in the format:
//
//	spec_id: value
//	cycle_start_trigger_at: 2026-02-25T10:00:00Z
//	...etc
func ParseFromPRBody(body string) (Record, error) {
	fields := make(map[string]string)

	// Parse key: value pairs from PR body
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Match pattern: key: value
		re := regexp.MustCompile(`^(\w+(?:_\w+)*)\s*:\s*(.*)$`)
		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			key := strings.TrimSpace(matches[1])
			value := strings.TrimSpace(matches[2])
			fields[key] = value
		}
	}

	rec := Record{}

	// Extract spec_id
	rec.SpecID = fields[FieldSpecID]

	// Extract and parse cycle_start_trigger_at
	if val, ok := fields[FieldCycleStartTriggerAt]; ok && val != "" {
		t, err := time.Parse(time.RFC3339, val)
		if err != nil {
			return Record{}, fmt.Errorf("invalid %s format: %w", FieldCycleStartTriggerAt, err)
		}
		rec.CycleStartTriggerAt = t
	}

	// Extract and parse cycle_end_presented_at
	if val, ok := fields[FieldCycleEndPresentedAt]; ok && val != "" {
		t, err := time.Parse(time.RFC3339, val)
		if err != nil {
			return Record{}, fmt.Errorf("invalid %s format: %w", FieldCycleEndPresentedAt, err)
		}
		rec.CycleEndPresentedAt = t
	}

	// Extract review_outcome
	if val, ok := fields[FieldReviewOutcome]; ok && val != "" {
		rec.ReviewOutcome = ReviewOutcome(val)
	}

	// Extract review_rationale
	if val, ok := fields["review_rationale"]; ok && val != "" {
		rec.ReviewRationale = val
	}

	// Extract human_tactical_intervention
	if val, ok := fields[FieldHumanTacticalIntervention]; ok && val != "" {
		rec.HumanTacticalIntervention = YesNo(val)
	}

	// Extract human_debugging_intervention
	if val, ok := fields[FieldHumanDebuggingIntervention]; ok && val != "" {
		rec.HumanDebuggingIntervention = YesNo(val)
	}

	// Extract escaped_regression_within_7d
	if val, ok := fields[FieldEscapedRegressionWithin7D]; ok && val != "" {
		rec.EscapedRegressionWithin7D = YesNo(val)
	}

	return rec, nil
}
