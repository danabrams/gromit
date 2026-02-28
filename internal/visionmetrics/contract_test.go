package visionmetrics

import "testing"

func TestContractFieldNameConstants(t *testing.T) {
	// Verify that all required field name constants are defined
	if FieldSpecID == "" {
		t.Error("FieldSpecID constant must be defined")
	}
	if FieldCycleStartTriggerAt == "" {
		t.Error("FieldCycleStartTriggerAt constant must be defined")
	}
	if FieldCycleEndPresentedAt == "" {
		t.Error("FieldCycleEndPresentedAt constant must be defined")
	}
	if FieldReviewOutcome == "" {
		t.Error("FieldReviewOutcome constant must be defined")
	}
	if FieldHumanTacticalIntervention == "" {
		t.Error("FieldHumanTacticalIntervention constant must be defined")
	}
	if FieldHumanDebuggingIntervention == "" {
		t.Error("FieldHumanDebuggingIntervention constant must be defined")
	}
	if FieldEscapedRegressionWithin7D == "" {
		t.Error("FieldEscapedRegressionWithin7D constant must be defined")
	}
}

func TestReviewOutcomeDomainMembership(t *testing.T) {
	// Test that all allowed ReviewOutcome values are defined
	if ReviewOutcomeAccepted == "" {
		t.Error("ReviewOutcomeAccepted must be defined")
	}
	if ReviewOutcomeImplementationGap == "" {
		t.Error("ReviewOutcomeImplementationGap must be defined")
	}
	if ReviewOutcomeVisionChange == "" {
		t.Error("ReviewOutcomeVisionChange must be defined")
	}

	// Test that allowed values are recognized as valid
	tests := []ReviewOutcome{
		ReviewOutcomeAccepted,
		ReviewOutcomeImplementationGap,
		ReviewOutcomeVisionChange,
	}
	for _, outcome := range tests {
		if !outcome.Valid() {
			t.Errorf("ReviewOutcome %q should be valid", outcome)
		}
	}
}

func TestInterventionDomainMembership(t *testing.T) {
	// Test that all allowed YesNo (intervention) values are defined
	if Yes == "" {
		t.Error("Yes must be defined")
	}
	if No == "" {
		t.Error("No must be defined")
	}

	// Test that allowed values are recognized as valid
	tests := []YesNo{Yes, No}
	for _, yn := range tests {
		if !yn.Valid() {
			t.Errorf("YesNo %q should be valid", yn)
		}
	}
}
