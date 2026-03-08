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
