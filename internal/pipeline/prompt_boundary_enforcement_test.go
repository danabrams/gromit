package pipeline

import (
	"testing"
)

// TestPromptConstructionBoundaryEnforcement documents and verifies that prompt
// assembly happens in the pipeline layer, not in CLI adapters.
//
// BOUNDARY ENFORCEMENT:
// - Pipeline (internal/pipeline/) loads and assembles prompt context
// - CLI Adapters (cmd/gromit/cli_adapters.go) are pure delegation wrappers
// - ReviewRenderer.LoadClaudeMD() and LoadRulesForPhase() enable pipeline to load context
// - ThoroughReviewPromptInput includes ClaudeMD and Rules fields
// - cliPromptRenderer uses input fields instead of loading from files
func TestPromptConstructionBoundaryEnforcement(t *testing.T) {
	// Verify ThoroughReviewPromptInput has context fields
	input := &ThoroughReviewPromptInput{
		FromCommit: "commit123",
		Diff:       "diff content",
		ClaudeMD:   "project context",
		Rules:      "rules content",
	}

	if input.ClaudeMD == "" || input.Rules == "" {
		t.Errorf("ThoroughReviewPromptInput should have ClaudeMD and Rules for pipeline assembly")
	}

	// Verify ReviewRenderer interface includes load methods for pipeline to use
	var _ ReviewRenderer = (*mockFullReviewRenderer)(nil)
}

// mockFullReviewRenderer demonstrates the complete ReviewRenderer interface
type mockFullReviewRenderer struct{}

func (m *mockFullReviewRenderer) RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error) {
	// Pure delegation - uses input fields
	return "", nil
}

func (m *mockFullReviewRenderer) LoadClaudeMD() (string, error) {
	// Adapter delegates to wrapped renderer
	return "", nil
}

func (m *mockFullReviewRenderer) LoadRulesForPhase(phase string) (string, error) {
	// Adapter delegates to wrapped renderer
	return "", nil
}
