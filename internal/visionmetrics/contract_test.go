package visionmetrics

import (
	"reflect"
	"testing"
)

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

func TestRecordFieldWiring(t *testing.T) {
	// Verify that Record struct has all required fields with correct types and JSON tags
	t.Run("spec_id field", func(t *testing.T) {
		checkStructField(t, "Record", (*Record)(nil), "SpecID", "string", FieldSpecID)
	})
	t.Run("cycle_start_trigger_at field", func(t *testing.T) {
		checkStructField(t, "Record", (*Record)(nil), "CycleStartTriggerAt", "time.Time", FieldCycleStartTriggerAt)
	})
	t.Run("cycle_end_presented_at field", func(t *testing.T) {
		checkStructField(t, "Record", (*Record)(nil), "CycleEndPresentedAt", "time.Time", FieldCycleEndPresentedAt)
	})
	t.Run("review_outcome field", func(t *testing.T) {
		checkStructField(t, "Record", (*Record)(nil), "ReviewOutcome", "ReviewOutcome", FieldReviewOutcome)
	})
	t.Run("human_tactical_intervention field", func(t *testing.T) {
		checkStructField(t, "Record", (*Record)(nil), "HumanTacticalIntervention", "YesNo", FieldHumanTacticalIntervention)
	})
	t.Run("human_debugging_intervention field", func(t *testing.T) {
		checkStructField(t, "Record", (*Record)(nil), "HumanDebuggingIntervention", "YesNo", FieldHumanDebuggingIntervention)
	})
	t.Run("escaped_regression_within_7d field", func(t *testing.T) {
		checkStructField(t, "Record", (*Record)(nil), "EscapedRegressionWithin7D", "YesNo", FieldEscapedRegressionWithin7D)
	})
}

func checkStructField(t *testing.T, structName string, ptr interface{}, fieldName, expectedType, jsonTag string) {
	t.Helper()
	rt := reflect.TypeOf(ptr).Elem()
	field, ok := rt.FieldByName(fieldName)
	if !ok {
		t.Errorf("%s missing field %s", structName, fieldName)
		return
	}

	// Check type - extract just the type name without package prefix
	actualType := field.Type.String()
	// Handle types that may have package prefix (e.g., "visionmetrics.ReviewOutcome" -> "ReviewOutcome")
	if len(actualType) > 0 {
		// Check if the type name ends with the expected type
		if !endsWith(actualType, expectedType) && actualType != expectedType {
			// Also try without package prefix
			lastDot := len(actualType) - 1
			for i := len(actualType) - 1; i >= 0; i-- {
				if actualType[i] == '.' {
					lastDot = i
					break
				}
			}
			shortType := actualType[lastDot+1:]
			if shortType != expectedType {
				t.Errorf("%s.%s type mismatch: got %s, want %s", structName, fieldName, actualType, expectedType)
			}
		}
	}

	// Check JSON tag
	tag := field.Tag.Get("json")
	if tag == "" || tag == "-" {
		t.Errorf("%s.%s missing JSON tag", structName, fieldName)
		return
	}
	// Extract the field name from the tag (remove options like ,omitempty)
	tagName := tag
	if idx := len(tag) - 1; idx >= 0 && tag[0:1] != "-" {
		for i := 0; i < len(tag); i++ {
			if tag[i] == ',' {
				tagName = tag[:i]
				break
			}
		}
	}
	if tagName != jsonTag {
		t.Errorf("%s.%s JSON tag mismatch: got %q, want %q", structName, fieldName, tagName, jsonTag)
	}
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
