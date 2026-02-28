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
