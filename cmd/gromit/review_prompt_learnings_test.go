package main

import (
	"os"
	"strings"
	"testing"
)

func TestReviewPrompt_LearningsOnlyForViolationsAndNovelPatterns(t *testing.T) {
	t.Parallel()
	// Read the review prompt template

	templatePath := resolveProjectPath("", ".gromit/templates/PROMPT_review.md")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("reading PROMPT_review.md: %v", err)
	}

	text := string(content)

	// Verify the learnings section explicitly states they should only be for violations,
	// novel patterns, or failure gotchas — NOT for confirming existing practices
	if !strings.Contains(text, "violation") && !strings.Contains(text, "Violation") {
		t.Error("learnings section should mention 'violation'")
	}

	if !strings.Contains(text, "novel") && !strings.Contains(text, "Novel") {
		t.Error("learnings section should mention 'novel'")
	}

	// Verify we DON'T encourage learnings that just confirm existing patterns
	if strings.Contains(text, "confirmed existing practices") ||
		strings.Contains(text, "confirming patterns") ||
		strings.Contains(text, "pattern is consistent") {
		t.Error("learnings section should NOT mention confirming existing practices or consistent patterns")
	}

	// Verify the example learnings don't show positive confirmations
	// Look for the examples in the JSON block
	jsonStart := strings.Index(text, `"learnings":`)
	if jsonStart == -1 {
		t.Fatal("could not find learnings field in template")
	}

	jsonEnd := strings.Index(text[jsonStart:], "]")
	if jsonEnd == -1 {
		t.Fatal("could not find end of learnings array in template")
	}

	learningsSection := text[jsonStart : jsonStart+jsonEnd+1]

	// Check that examples are NOT about confirming conventions or patterns
	badExamples := []string{
		"pattern in service.go is cleaner than older code",
		"Test naming convention followed consistently",
		"Error handling pattern in service.go is cleaner",
	}

	for _, badExample := range badExamples {
		if strings.Contains(learningsSection, badExample) {
			t.Errorf("learnings example should not be: %q", badExample)
		}
	}
}
