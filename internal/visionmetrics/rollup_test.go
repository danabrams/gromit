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

func TestComputeRollup_NormalScenarios(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		records   []Record
		wantTact  int // tactical numerator
		wantDebug int // debugging numerator
		wantPass  int // first pass numerator
		wantEsc   int // escaped regression numerator
		wantAccep int // accepted numerator
		wantDenom int // denominator (valid records)
		wantAcceptDenom int // accepted-without-rework denominator
	}{
		{
			name: "single accepted record with no interventions",
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
			wantTact:        0,
			wantDebug:       0,
			wantPass:        1,
			wantEsc:         0,
			wantAccep:       1,
			wantDenom:       1,
			wantAcceptDenom: 1,
		},
		{
			name: "single accepted record with tactical intervention",
			records: []Record{
				{
					SpecID:                     "spec-1",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  Yes,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  No,
				},
			},
			wantTact:        1,
			wantDebug:       0,
			wantPass:        1,
			wantEsc:         0,
			wantAccep:       1,
			wantDenom:       1,
			wantAcceptDenom: 1,
		},
		{
			name: "multiple records with mixed outcomes",
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
					ReviewOutcome:              ReviewOutcomeImplementationGap,
					HumanTacticalIntervention:  Yes,
					HumanDebuggingIntervention: Yes,
					EscapedRegressionWithin7D:  Yes,
				},
				{
					SpecID:                     "spec-3",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  Yes,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  No,
				},
			},
			wantTact:        2,
			wantDebug:       1,
			wantPass:        2,
			wantEsc:         1,
			wantAccep:       2,
			wantDenom:       3,
			wantAcceptDenom: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rollup := ComputeRollup(tt.records)

			if rollup.HumanTacticalInterventionRate.Numerator != tt.wantTact {
				t.Errorf("HumanTacticalInterventionRate.Numerator = %d, want %d",
					rollup.HumanTacticalInterventionRate.Numerator, tt.wantTact)
			}
			if rollup.HumanTacticalInterventionRate.Denominator != tt.wantDenom {
				t.Errorf("HumanTacticalInterventionRate.Denominator = %d, want %d",
					rollup.HumanTacticalInterventionRate.Denominator, tt.wantDenom)
			}

			if rollup.HumanDebuggingInterventionRate.Numerator != tt.wantDebug {
				t.Errorf("HumanDebuggingInterventionRate.Numerator = %d, want %d",
					rollup.HumanDebuggingInterventionRate.Numerator, tt.wantDebug)
			}
			if rollup.HumanDebuggingInterventionRate.Denominator != tt.wantDenom {
				t.Errorf("HumanDebuggingInterventionRate.Denominator = %d, want %d",
					rollup.HumanDebuggingInterventionRate.Denominator, tt.wantDenom)
			}

			if rollup.FirstIntegrationPassRate.Numerator != tt.wantPass {
				t.Errorf("FirstIntegrationPassRate.Numerator = %d, want %d",
					rollup.FirstIntegrationPassRate.Numerator, tt.wantPass)
			}
			if rollup.FirstIntegrationPassRate.Denominator != tt.wantDenom {
				t.Errorf("FirstIntegrationPassRate.Denominator = %d, want %d",
					rollup.FirstIntegrationPassRate.Denominator, tt.wantDenom)
			}

			if rollup.EscapedRegressionRate.Numerator != tt.wantEsc {
				t.Errorf("EscapedRegressionRate.Numerator = %d, want %d",
					rollup.EscapedRegressionRate.Numerator, tt.wantEsc)
			}
			if rollup.EscapedRegressionRate.Denominator != tt.wantDenom {
				t.Errorf("EscapedRegressionRate.Denominator = %d, want %d",
					rollup.EscapedRegressionRate.Denominator, tt.wantDenom)
			}

			if rollup.AcceptedWithoutReworkRate.Numerator != tt.wantAccep {
				t.Errorf("AcceptedWithoutReworkRate.Numerator = %d, want %d",
					rollup.AcceptedWithoutReworkRate.Numerator, tt.wantAccep)
			}
			if rollup.AcceptedWithoutReworkRate.Denominator != tt.wantAcceptDenom {
				t.Errorf("AcceptedWithoutReworkRate.Denominator = %d, want %d",
					rollup.AcceptedWithoutReworkRate.Denominator, tt.wantAcceptDenom)
			}
		})
	}
}
