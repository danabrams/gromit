package visionmetrics

import (
	"testing"
	"time"
)

func TestComputeRollup_FiltersInvalidRecords(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		records   []Record
		wantValid int
		wantTotal int // total numerator count for metrics with total denominator
	}{
		{
			name: "all valid records",
			records: []Record{
				{
					SpecID:                     "spec-1",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  No,
				},
			},
			wantValid: 1,
			wantTotal: 1,
		},
		{
			name: "mix of valid and invalid records",
			records: []Record{
				{
					SpecID:                     "spec-1",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  No,
				},
				{
					SpecID:                     "spec-2",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: Yes, // invalid: debugging without tactical
					EscapedRegressionWithin7D:  No,
				},
			},
			wantValid: 1,
			wantTotal: 1,
		},
		{
			name: "all invalid records",
			records: []Record{
				{
					SpecID:                     "", // missing
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  No,
				},
			},
			wantValid: 0,
			wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rollup := ComputeRollup(tt.records)

			// All metric denominators should reflect only valid records
			if rollup.HumanTacticalInterventionRate.Denominator != tt.wantValid {
				t.Errorf("HumanTacticalInterventionRate.Denominator = %d, want %d",
					rollup.HumanTacticalInterventionRate.Denominator, tt.wantValid)
			}
			if rollup.HumanDebuggingInterventionRate.Denominator != tt.wantValid {
				t.Errorf("HumanDebuggingInterventionRate.Denominator = %d, want %d",
					rollup.HumanDebuggingInterventionRate.Denominator, tt.wantValid)
			}
			if rollup.FirstIntegrationPassRate.Denominator != tt.wantValid {
				t.Errorf("FirstIntegrationPassRate.Denominator = %d, want %d",
					rollup.FirstIntegrationPassRate.Denominator, tt.wantValid)
			}
			if rollup.EscapedRegressionRate.Denominator != tt.wantValid {
				t.Errorf("EscapedRegressionRate.Denominator = %d, want %d",
					rollup.EscapedRegressionRate.Denominator, tt.wantValid)
			}
		})
	}
}
