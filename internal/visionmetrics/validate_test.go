package visionmetrics

import (
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		modify     func(*Record)
		wantErr    bool
		wantFields []string
	}{
		{
			name:    "valid record",
			wantErr: false,
		},
		{
			name: "missing spec_id",
			modify: func(rec *Record) {
				rec.SpecID = "   "
			},
			wantErr:    true,
			wantFields: []string{FieldSpecID},
		},
		{
			name: "missing cycle_start_trigger_at",
			modify: func(rec *Record) {
				rec.CycleStartTriggerAt = time.Time{}
			},
			wantErr:    true,
			wantFields: []string{FieldCycleStartTriggerAt},
		},
		{
			name: "missing cycle_end_presented_at",
			modify: func(rec *Record) {
				rec.CycleEndPresentedAt = time.Time{}
			},
			wantErr:    true,
			wantFields: []string{FieldCycleEndPresentedAt},
		},
		{
			name: "invalid review_outcome",
			modify: func(rec *Record) {
				rec.ReviewOutcome = ReviewOutcome("unknown")
			},
			wantErr:    true,
			wantFields: []string{FieldReviewOutcome},
		},
		{
			name: "invalid human_tactical_intervention",
			modify: func(rec *Record) {
				rec.HumanTacticalIntervention = YesNo("maybe")
			},
			wantErr:    true,
			wantFields: []string{FieldHumanTacticalIntervention},
		},
		{
			name: "invalid human_debugging_intervention",
			modify: func(rec *Record) {
				rec.HumanDebuggingIntervention = YesNo("maybe")
			},
			wantErr:    true,
			wantFields: []string{FieldHumanDebuggingIntervention},
		},
		{
			name: "invalid escaped_regression_within_7d",
			modify: func(rec *Record) {
				rec.EscapedRegressionWithin7D = YesNo("maybe")
			},
			wantErr:    true,
			wantFields: []string{FieldEscapedRegressionWithin7D},
		},
		{
			name: "debugging requires tactical",
			modify: func(rec *Record) {
				rec.HumanDebuggingIntervention = Yes
				rec.HumanTacticalIntervention = No
			},
			wantErr:    true,
			wantFields: []string{FieldHumanDebuggingIntervention},
		},
		{
			name: "vision change requires rationale",
			modify: func(rec *Record) {
				rec.ReviewOutcome = ReviewOutcomeVisionChange
				rec.ReviewRationale = "   "
			},
			wantErr:    true,
			wantFields: []string{"review_rationale"},
		},
		{
			name: "cycle end must follow cycle start",
			modify: func(rec *Record) {
				rec.CycleEndPresentedAt = rec.CycleStartTriggerAt.Add(-time.Minute)
			},
			wantErr:    true,
			wantFields: []string{FieldCycleEndPresentedAt},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := validRecord()
			if tc.modify != nil {
				tc.modify(&rec)
			}

			errs := Validate(rec)
			if tc.wantErr {
				if len(errs) == 0 {
					t.Fatalf("%s: expected errors but got none", tc.name)
				}
				if len(errs) != len(tc.wantFields) {
					t.Fatalf("%s: expected %d error(s) but got %d: %+v", tc.name, len(tc.wantFields), len(errs), errs)
				}
				fieldCounts := make(map[string]int)
				for _, err := range errs {
					fieldCounts[err.Field]++
				}
				for _, wantField := range tc.wantFields {
					if fieldCounts[wantField] == 0 {
						t.Fatalf("%s: missing error for field %s, got %v", tc.name, wantField, errs)
					}
					fieldCounts[wantField]--
				}
			} else if len(errs) != 0 {
				t.Fatalf("%s: expected no errors but got %d: %+v", tc.name, len(errs), errs)
			}
		})
	}
}

func validRecord() Record {
	start := time.Date(2026, 2, 27, 8, 0, 0, 0, time.UTC)
	return Record{
		SpecID:                     "spec-accepted-clean",
		CycleStartTriggerAt:        start,
		CycleEndPresentedAt:        start.Add(2 * time.Hour),
		ReviewOutcome:              ReviewOutcomeAccepted,
		ReviewRationale:            "baseline rationale",
		HumanTacticalIntervention:  No,
		HumanDebuggingIntervention: No,
		EscapedRegressionWithin7D:  No,
	}
}
