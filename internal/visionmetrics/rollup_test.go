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
					EscapedRegressionWithin7D:  EscapedRegressionNo,
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
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
				{
					SpecID:                     "spec-2",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: Yes, // invalid: debugging without tactical
					EscapedRegressionWithin7D:  EscapedRegressionNo,
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
					EscapedRegressionWithin7D:  EscapedRegressionNo,
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

func TestComputeRollup_CarveOutScenarios(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name            string
		records         []Record
		wantAcceptNum   int
		wantTotalDenom  int
		wantAcceptDenom int // excluding carve-outs
		wantAcceptRate  float64
	}{
		{
			name: "single vision change carve-out",
			records: []Record{
				{
					SpecID:                     "spec-1",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeVisionChange,
					ReviewRationale:            "Product direction shifted",
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
			},
			wantAcceptNum:   0,
			wantTotalDenom:  1,
			wantAcceptDenom: 0, // carve-outs excluded from denominator
			wantAcceptRate:  0.0,
		},
		{
			name: "mix of accepted and carve-out records",
			records: []Record{
				{
					SpecID:                     "spec-1",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
				{
					SpecID:                     "spec-2",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeVisionChange,
					ReviewRationale:            "Market analysis required rethink",
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
				{
					SpecID:                     "spec-3",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  Yes,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
			},
			wantAcceptNum:   2,   // only accepted records
			wantTotalDenom:  3,   // all valid records
			wantAcceptDenom: 2,   // total - carve-outs
			wantAcceptRate:  1.0, // 2/2
		},
		{
			name: "multiple carve-outs",
			records: []Record{
				{
					SpecID:                     "spec-1",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeVisionChange,
					ReviewRationale:            "First carve-out",
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
				{
					SpecID:                     "spec-2",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
				{
					SpecID:                     "spec-3",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeVisionChange,
					ReviewRationale:            "Second carve-out",
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
			},
			wantAcceptNum:   1,
			wantTotalDenom:  3,
			wantAcceptDenom: 1, // 3 - 2 carve-outs
			wantAcceptRate:  1.0,
		},
		{
			name: "all records are carve-outs",
			records: []Record{
				{
					SpecID:                     "spec-1",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeVisionChange,
					ReviewRationale:            "Carve-out",
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
				{
					SpecID:                     "spec-2",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeVisionChange,
					ReviewRationale:            "Another carve-out",
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
			},
			wantAcceptNum:   0,
			wantTotalDenom:  2,
			wantAcceptDenom: 0, // all are carve-outs
			wantAcceptRate:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rollup := ComputeRollup(tt.records)

			// Verify numerator count
			if rollup.AcceptedWithoutReworkRate.Numerator != tt.wantAcceptNum {
				t.Errorf("AcceptedWithoutReworkRate.Numerator = %d, want %d",
					rollup.AcceptedWithoutReworkRate.Numerator, tt.wantAcceptNum)
			}

			// Verify denominator is total - carve-outs
			if rollup.AcceptedWithoutReworkRate.Denominator != tt.wantAcceptDenom {
				t.Errorf("AcceptedWithoutReworkRate.Denominator = %d, want %d",
					rollup.AcceptedWithoutReworkRate.Denominator, tt.wantAcceptDenom)
			}

			// Verify rate calculation
			if rollup.AcceptedWithoutReworkRate.Rate != tt.wantAcceptRate {
				t.Errorf("AcceptedWithoutReworkRate.Rate = %f, want %f",
					rollup.AcceptedWithoutReworkRate.Rate, tt.wantAcceptRate)
			}

			// Verify other metrics use total denominator
			if rollup.HumanTacticalInterventionRate.Denominator != tt.wantTotalDenom {
				t.Errorf("HumanTacticalInterventionRate.Denominator = %d, want %d",
					rollup.HumanTacticalInterventionRate.Denominator, tt.wantTotalDenom)
			}
		})
	}
}
func TestComputeRollup_EdgeCases(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		records   []Record
		wantDenom int
		checkRate func(*testing.T, MetricRate)
	}{
		{
			name:      "empty records slice",
			records:   []Record{},
			wantDenom: 0,
			checkRate: func(t *testing.T, mr MetricRate) {
				if mr.Numerator != 0 {
					t.Errorf("Numerator = %d, want 0", mr.Numerator)
				}
				if mr.Denominator != 0 {
					t.Errorf("Denominator = %d, want 0", mr.Denominator)
				}
				if mr.Rate != 0.0 {
					t.Errorf("Rate = %f, want 0.0", mr.Rate)
				}
			},
		},
		{
			name: "all records with interventions",
			records: []Record{
				{
					SpecID:                     "spec-1",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  Yes,
					HumanDebuggingIntervention: Yes,
					EscapedRegressionWithin7D:  EscapedRegressionYes,
				},
				{
					SpecID:                     "spec-2",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  Yes,
					HumanDebuggingIntervention: Yes,
					EscapedRegressionWithin7D:  EscapedRegressionYes,
				},
			},
			wantDenom: 2,
			checkRate: func(t *testing.T, mr MetricRate) {
				// All intervention metrics should be 2/2 = 1.0
				if mr.Numerator != 2 {
					t.Errorf("Numerator = %d, want 2", mr.Numerator)
				}
				if mr.Rate != 1.0 {
					t.Errorf("Rate = %f, want 1.0", mr.Rate)
				}
			},
		},
		{
			name: "all records without interventions",
			records: []Record{
				{
					SpecID:                     "spec-1",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
				{
					SpecID:                     "spec-2",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
			},
			wantDenom: 2,
			checkRate: func(t *testing.T, mr MetricRate) {
				// All intervention metrics should be 0/2 = 0.0
				if mr.Numerator != 0 {
					t.Errorf("Numerator = %d, want 0", mr.Numerator)
				}
				if mr.Rate != 0.0 {
					t.Errorf("Rate = %f, want 0.0", mr.Rate)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rollup := ComputeRollup(tt.records)

			if tt.name == "empty records slice" {
				// Check all metrics for empty case
				tt.checkRate(t, rollup.HumanTacticalInterventionRate)
				tt.checkRate(t, rollup.HumanDebuggingInterventionRate)
				tt.checkRate(t, rollup.FirstIntegrationPassRate)
				tt.checkRate(t, rollup.EscapedRegressionRate)
				tt.checkRate(t, rollup.AcceptedWithoutReworkRate)
			} else if tt.name == "all records with interventions" {
				tt.checkRate(t, rollup.HumanTacticalInterventionRate)
				tt.checkRate(t, rollup.HumanDebuggingInterventionRate)
				tt.checkRate(t, rollup.EscapedRegressionRate)
			} else if tt.name == "all records without interventions" {
				tt.checkRate(t, rollup.HumanTacticalInterventionRate)
				tt.checkRate(t, rollup.HumanDebuggingInterventionRate)
				tt.checkRate(t, rollup.EscapedRegressionRate)
			}
		})
	}
}
func TestComputeRollup_NormalScenarios(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name            string
		records         []Record
		wantTact        int // tactical numerator
		wantDebug       int // debugging numerator
		wantPass        int // first pass numerator
		wantEsc         int // escaped regression numerator
		wantAccep       int // accepted numerator
		wantDenom       int // denominator (valid records)
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
					EscapedRegressionWithin7D:  EscapedRegressionNo,
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
					EscapedRegressionWithin7D:  EscapedRegressionNo,
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
					EscapedRegressionWithin7D:  EscapedRegressionNo,
				},
				{
					SpecID:                     "spec-2",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeImplementationGap,
					HumanTacticalIntervention:  Yes,
					HumanDebuggingIntervention: Yes,
					EscapedRegressionWithin7D:  EscapedRegressionYes,
				},
				{
					SpecID:                     "spec-3",
					CycleStartTriggerAt:        now,
					CycleEndPresentedAt:        now.Add(time.Hour),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  Yes,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  EscapedRegressionNo,
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

func TestComputeRollup_EscapedRegressionPendingExclusion(t *testing.T) {
	now := time.Now()
	records := []Record{
		{
			SpecID:                     "spec-yes",
			CycleStartTriggerAt:        now,
			CycleEndPresentedAt:        now.Add(time.Hour),
			ReviewOutcome:              ReviewOutcomeAccepted,
			HumanTacticalIntervention:  No,
			HumanDebuggingIntervention: No,
			EscapedRegressionWithin7D:  EscapedRegressionYes,
		},
		{
			SpecID:                     "spec-pending",
			CycleStartTriggerAt:        now,
			CycleEndPresentedAt:        now.Add(time.Hour),
			ReviewOutcome:              ReviewOutcomeAccepted,
			HumanTacticalIntervention:  No,
			HumanDebuggingIntervention: No,
			EscapedRegressionWithin7D:  EscapedRegressionPending,
		},
		{
			SpecID:                     "spec-no",
			CycleStartTriggerAt:        now,
			CycleEndPresentedAt:        now.Add(time.Hour),
			ReviewOutcome:              ReviewOutcomeAccepted,
			HumanTacticalIntervention:  No,
			HumanDebuggingIntervention: No,
			EscapedRegressionWithin7D:  EscapedRegressionNo,
		},
	}

	rollup := ComputeRollup(records)

	if rollup.EscapedRegressionRate.Numerator != 1 {
		t.Fatalf("EscapedRegressionRate.Numerator = %d, want 1", rollup.EscapedRegressionRate.Numerator)
	}

	if rollup.EscapedRegressionRate.Denominator != 2 {
		t.Fatalf("EscapedRegressionRate.Denominator = %d, want 2", rollup.EscapedRegressionRate.Denominator)
	}
}

func TestComputeRollup_EscapedRegressionPendingCount(t *testing.T) {
	now := time.Now()
	records := []Record{
		{
			SpecID:                     "spec-one",
			CycleStartTriggerAt:        now,
			CycleEndPresentedAt:        now.Add(time.Hour),
			ReviewOutcome:              ReviewOutcomeAccepted,
			HumanTacticalIntervention:  No,
			HumanDebuggingIntervention: No,
			EscapedRegressionWithin7D:  EscapedRegressionPending,
		},
		{
			SpecID:                     "spec-two",
			CycleStartTriggerAt:        now,
			CycleEndPresentedAt:        now.Add(time.Hour),
			ReviewOutcome:              ReviewOutcomeAccepted,
			HumanTacticalIntervention:  No,
			HumanDebuggingIntervention: No,
			EscapedRegressionWithin7D:  EscapedRegressionPending,
		},
		{
			SpecID:                     "spec-three",
			CycleStartTriggerAt:        now,
			CycleEndPresentedAt:        now.Add(time.Hour),
			ReviewOutcome:              ReviewOutcomeAccepted,
			HumanTacticalIntervention:  No,
			HumanDebuggingIntervention: No,
			EscapedRegressionWithin7D:  EscapedRegressionNo,
		},
	}

	rollup := ComputeRollup(records)

	if rollup.EscapedRegressionPendingCount != 2 {
		t.Fatalf("EscapedRegressionPendingCount = %d, want 2", rollup.EscapedRegressionPendingCount)
	}
}
