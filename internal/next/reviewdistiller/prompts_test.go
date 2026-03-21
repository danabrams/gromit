package reviewdistiller

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPrompt_IncludesPreamble(t *testing.T) {
	inputs := &DistillerInputs{
		RunID:       "run-123",
		SpecID:      "spec-456",
		SpecContent: "This is the spec content",
	}

	prompt := BuildPrompt(inputs, "accepted")

	// Preamble should include the spec content
	if !strings.Contains(prompt, "This is the spec content") {
		t.Errorf("prompt should include spec content")
	}
}

func TestBuildPrompt_IncludesArtifactJSON(t *testing.T) {
	inputs := &DistillerInputs{
		RunID:         "run-123",
		SpecID:        "spec-456",
		SpecContent:   "spec",
		ReviewOutcome: json.RawMessage(`{"key":"value"}`),
	}

	prompt := BuildPrompt(inputs, "accepted")

	// Prompt should include artifact JSON
	if !strings.Contains(prompt, `"key":"value"`) {
		t.Errorf("prompt should include review outcome JSON")
	}
}

func TestBuildPrompt_AcceptedOutcome(t *testing.T) {
	inputs := &DistillerInputs{
		RunID:       "run-123",
		SpecID:      "spec-456",
		SpecContent: "spec",
	}

	prompt := BuildPrompt(inputs, "accepted")

	// Should include accepted outcome instructions
	if !strings.Contains(prompt, "accepted") {
		t.Errorf("prompt should include accepted outcome instructions")
	}
	// Should include doctrine_rule reference
	if !strings.Contains(prompt, "doctrine_rule") {
		t.Errorf("prompt should reference doctrine_rule for accepted outcome")
	}
}

func TestBuildPrompt_ReworkImplementationGapOutcome(t *testing.T) {
	inputs := &DistillerInputs{
		RunID:       "run-123",
		SpecID:      "spec-456",
		SpecContent: "spec",
	}

	prompt := BuildPrompt(inputs, "rework_implementation_gap")

	// Should include rework_implementation_gap outcome instructions
	if !strings.Contains(prompt, "rework_implementation_gap") {
		t.Errorf("prompt should include rework_implementation_gap outcome instructions")
	}
}

func TestBuildPrompt_ReworkVisionChangeOutcome(t *testing.T) {
	inputs := &DistillerInputs{
		RunID:       "run-123",
		SpecID:      "spec-456",
		SpecContent: "spec",
	}

	prompt := BuildPrompt(inputs, "rework_vision_change")

	// Should include rework_vision_change outcome instructions
	if !strings.Contains(prompt, "rework_vision_change") {
		t.Errorf("prompt should include rework_vision_change outcome instructions")
	}
	// Should include refinement_guidance reference
	if !strings.Contains(prompt, "refinement_guidance") {
		t.Errorf("prompt should reference refinement_guidance for rework_vision_change outcome")
	}
}

func TestBuildPrompt_AllArtifacts(t *testing.T) {
	inputs := &DistillerInputs{
		RunID:           "run-123",
		SpecID:          "spec-456",
		SpecContent:     "spec",
		ReviewOutcome:   json.RawMessage(`{"outcome":"test"}`),
		ProductReview:   json.RawMessage(`{"product":"test"}`),
		ProcessReview:   json.RawMessage(`{"process":"test"}`),
		ManualChecklist: json.RawMessage(`{"manual":"test"}`),
		Validation:      json.RawMessage(`{"validation":"test"}`),
		Acceptance:      json.RawMessage(`{"acceptance":"test"}`),
		MachineReview:   json.RawMessage(`{"machine":"test"}`),
		TaskResults:     json.RawMessage(`{"tasks":"test"}`),
		RunMetadata:     json.RawMessage(`{"metadata":"test"}`),
	}

	prompt := BuildPrompt(inputs, "accepted")

	// All artifacts should be included
	if !strings.Contains(prompt, `"outcome":"test"`) {
		t.Errorf("prompt should include review outcome")
	}
	if !strings.Contains(prompt, `"product":"test"`) {
		t.Errorf("prompt should include product review")
	}
	if !strings.Contains(prompt, `"process":"test"`) {
		t.Errorf("prompt should include process review")
	}
	if !strings.Contains(prompt, `"manual":"test"`) {
		t.Errorf("prompt should include manual checklist")
	}
	if !strings.Contains(prompt, `"validation":"test"`) {
		t.Errorf("prompt should include validation")
	}
	if !strings.Contains(prompt, `"acceptance":"test"`) {
		t.Errorf("prompt should include acceptance")
	}
	if !strings.Contains(prompt, `"machine":"test"`) {
		t.Errorf("prompt should include machine review")
	}
	if !strings.Contains(prompt, `"tasks":"test"`) {
		t.Errorf("prompt should include task results")
	}
	if !strings.Contains(prompt, `"metadata":"test"`) {
		t.Errorf("prompt should include run metadata")
	}
}

func TestBuildPrompt_ProducesNonEmptyForEachOutcomeType(t *testing.T) {
	inputs := &DistillerInputs{
		RunID:       "run-123",
		SpecID:      "spec-456",
		SpecContent: "spec content",
	}

	outcomes := []string{
		"accepted",
		"rework_implementation_gap",
		"rework_vision_change",
	}

	for _, outcome := range outcomes {
		t.Run(outcome, func(t *testing.T) {
			prompt := BuildPrompt(inputs, outcome)

			if prompt == "" {
				t.Errorf("BuildPrompt should produce non-empty output for outcome %s", outcome)
			}
			if !strings.Contains(prompt, "spec content") {
				t.Errorf("BuildPrompt should include preamble for outcome %s", outcome)
			}
		})
	}
}
