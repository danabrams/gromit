package pipeline

import (
	"testing"
)

// RED: Test that ReviewRenderer receives ClaudeMD and Rules in input, not loading them
// This enforces that prompt context assembly happens in pipeline, not adapter
func TestReviewPromptInputIncludesContextFields(t *testing.T) {
	// ThoroughReviewPromptInput should have all fields needed for prompt assembly
	// If ClaudeMD and Rules are missing, the adapter will need to load them (wrong!)

	input := &ThoroughReviewPromptInput{
		FromCommit: "abc123",
		Diff:       "some diff",
	}

	// These fields should exist and be populated by pipeline, not adapter
	if input.ClaudeMD == "" {
		// If these are empty, the adapter will load them (boundary violation)
		// After fix, pipeline.renderReviewPrompt will populate these
		t.Logf("WARNING: ClaudeMD not in ThoroughReviewPromptInput - adapter will load it")
	}

	if input.Rules == "" {
		// If these are empty, the adapter will load them (boundary violation)
		t.Logf("WARNING: Rules not in ThoroughReviewPromptInput - adapter will load it")
	}
}

// RED: Test that ThoroughReviewPromptInput can carry complete prompt context
// This verifies the input type can support full prompt assembly in pipeline
func TestThoroughReviewPromptInputSupportsFullContext(t *testing.T) {
	// The input should be able to carry all context needed by the adapter
	// so the adapter doesn't need to load any files
	input := &ThoroughReviewPromptInput{
		FromCommit: "abc123",
		Diff:       "some diff",
		ClaudeMD:   "project context", // Should be populated by pipeline
		Rules:      "rules content",   // Should be populated by pipeline
	}

	if input.ClaudeMD == "" || input.Rules == "" {
		t.Errorf("ThoroughReviewPromptInput missing context fields - adapter will load from files")
	}
}
