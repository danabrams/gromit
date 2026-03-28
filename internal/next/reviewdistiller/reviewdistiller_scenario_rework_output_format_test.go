package reviewdistiller

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScenario_BuildPrompt_ReworkImplementationGap_IncludesOutputFormatArrayConstraint(t *testing.T) {
	// Seed
	inputs := &DistillerInputs{
		RunID:         "run-123",
		SpecID:        "spec-456",
		SpecContent:   "spec content",
		ReviewOutcome: json.RawMessage(`{"outcome":"rework_implementation_gap"}`),
	}

	// Invoke
	prompt := BuildPrompt(inputs, "rework_implementation_gap")

	// Assert
	if !strings.Contains(prompt, "## Output Format") {
		t.Fatalf("expected prompt to include ## Output Format section")
	}
	if !strings.Contains(prompt, "evidence_references") {
		t.Fatalf("expected prompt to include evidence_references constraint text")
	}

	sectionStart := strings.Index(prompt, "## Output Format")
	if sectionStart == -1 {
		t.Fatalf("could not find Output Format section")
	}
	outputFormatSection := prompt[sectionStart:]
	if !strings.Contains(outputFormatSection, "[") {
		t.Fatalf("expected Output Format section to include '[' to indicate array formatting")
	}
}
