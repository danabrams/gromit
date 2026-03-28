package reviewdistiller

import (
	"strings"
	"testing"
)

func TestScenario_BuildPrompt_OutputFormatForAcceptedAndReworkVisionChange(t *testing.T) {
	// Seed
	inputs := &DistillerInputs{
		RunID:       "run-123",
		SpecID:      "spec-456",
		SpecContent: "scenario spec content",
	}

	// Invoke
	acceptedPrompt := BuildPrompt(inputs, "accepted")
	reworkVisionChangePrompt := BuildPrompt(inputs, "rework_vision_change")

	// Assert (accepted)
	if !strings.Contains(acceptedPrompt, outputFormatHeading) {
		t.Errorf("accepted prompt should include %q", outputFormatHeading)
	}
	if !strings.Contains(acceptedPrompt, evidenceReferencesConstraint) {
		t.Errorf("accepted prompt should include evidence_references array syntax constraint")
	}

	// Assert (rework_vision_change)
	if !strings.Contains(reworkVisionChangePrompt, outputFormatHeading) {
		t.Errorf("rework_vision_change prompt should include %q", outputFormatHeading)
	}
	if !strings.Contains(reworkVisionChangePrompt, evidenceReferencesConstraint) {
		t.Errorf("rework_vision_change prompt should include evidence_references array syntax constraint")
	}
}
