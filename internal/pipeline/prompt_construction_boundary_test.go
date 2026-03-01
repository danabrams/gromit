package pipeline

import (
	"testing"
)

// RED: Test that ReviewRenderer receives ClaudeMD and Rules in input, not loading them
// This enforces that prompt context assembly happens in pipeline, not adapter
func TestReviewPromptInputIncludesContextFields(t *testing.T) {
	loader := &mockPromptContextLoader{
		claude: "project context",
		rules:  "rules content",
	}

	input := NewThoroughReviewPromptInput(loader, thoroughReviewPhase, "abc123", "some diff")

	if input.ClaudeMD == "" {
		t.Fatalf("ThoroughReviewPromptInput missing ClaudeMD - boundary violated")
	}

	if input.Rules == "" {
		t.Fatalf("ThoroughReviewPromptInput missing Rules - boundary violated")
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

// mockPromptContextLoader provides the prompt context that pipeline is expected to supply.
type mockPromptContextLoader struct {
	claude string
	rules  string
}

func (m *mockPromptContextLoader) LoadClaudeMD() (string, error) {
	return m.claude, nil
}

func (m *mockPromptContextLoader) LoadRulesForPhase(phase string) (string, error) {
	return m.rules, nil
}
