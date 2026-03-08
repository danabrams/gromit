package debug

import (
	"strings"
	"testing"
)

func TestExtractLearning_PatternEntryMarkedAutonomous(t *testing.T) {
	input := LearningExtractionInput{
		LearningsEntry: "### 2026-03-08 | debug2_learning | CODE_PATTERN\n\nAlways preserve failing context before retries.\n",
	}

	output := ExtractLearning(input)

	if !output.Autonomous {
		t.Fatal("Autonomous = false, want true")
	}
	if output.SystemicRecommendation != "" {
		t.Fatalf("SystemicRecommendation = %q, want empty", output.SystemicRecommendation)
	}
	if !strings.Contains(output.LearningsEntry, "Always preserve failing context before retries.") {
		t.Fatalf("LearningsEntry missing source content: %q", output.LearningsEntry)
	}
	if !strings.Contains(output.LearningsEntry, "*Autonomous: true*") {
		t.Fatalf("LearningsEntry missing autonomous marker: %q", output.LearningsEntry)
	}
}

func TestExtractLearning_SystemicEntryUsesRecommendationPath(t *testing.T) {
	input := LearningExtractionInput{
		LearningsEntry: "Add a prompt fragment and pipeline code guard so this class of issue is blocked in CI.",
	}

	output := ExtractLearning(input)

	if output.Autonomous {
		t.Fatal("Autonomous = true, want false")
	}
	if output.LearningsEntry != "" {
		t.Fatalf("LearningsEntry = %q, want empty", output.LearningsEntry)
	}
	if output.SystemicRecommendation == "" {
		t.Fatal("SystemicRecommendation = empty, want non-empty")
	}
	if !strings.Contains(output.SystemicRecommendation, "prompt fragment") {
		t.Fatalf("SystemicRecommendation = %q, want to include prompt fragment guidance", output.SystemicRecommendation)
	}
}

func TestExtractLearning_DerivesEntryFromDiagnosis(t *testing.T) {
	diag := Diagnosis{
		StageTrace: StageTrace{
			StageName:      "build",
			FailureMessage: "syntax error: missing return value",
		},
		RootCause: RootCauseBadBuildOutput,
	}

	output := ExtractLearning(LearningExtractionInput{
		Diagnosis: &diag,
	})

	if !output.Autonomous {
		t.Fatal("Autonomous = false, want true")
	}
	if output.SystemicRecommendation != "" {
		t.Fatalf("SystemicRecommendation = %q, want empty", output.SystemicRecommendation)
	}
	if !strings.Contains(output.LearningsEntry, "Capture syntax-error build diagnostics") {
		t.Fatalf("LearningsEntry = %q, want pattern description", output.LearningsEntry)
	}
}

func TestExtractLearning_SystemicRootCauseGeneratesRecommendation(t *testing.T) {
	input := LearningExtractionInput{
		RootCause: RootCauseUnclearBead,
	}

	output := ExtractLearning(input)

	if output.Autonomous {
		t.Fatal("Autonomous = true, want false")
	}
	if output.LearningsEntry != "" {
		t.Fatalf("LearningsEntry = %q, want empty", output.LearningsEntry)
	}
	if output.SystemicRecommendation == "" {
		t.Fatal("SystemicRecommendation = empty, want non-empty")
	}
	if !strings.Contains(strings.ToLower(output.SystemicRecommendation), "human review") {
		t.Fatalf("SystemicRecommendation = %q, want human-review guidance", output.SystemicRecommendation)
	}
}
