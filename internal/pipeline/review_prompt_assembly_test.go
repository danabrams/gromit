package pipeline

import (
	"testing"
)

// RED: Test that ReviewRenderer receives ClaudeMD and Rules from pipeline
// This enforces that prompt context loading happens in pipeline layer,
// and the adapter (cli_adapters.go) should NOT load these from files
func TestRenderReviewPromptPassesContextToRenderer(t *testing.T) {
	var receivedInput *ThoroughReviewPromptInput

	// Mock ReviewRenderer that captures what it receives
	mockRenderer := &mockReviewRendererTracker{
		onRenderThoroughReview: func(input *ThoroughReviewPromptInput) (string, error) {
			receivedInput = input
			return "rendered prompt", nil
		},
	}

	p := &Pipeline{
		paths: &Paths{
			GromitDir: "/tmp/test-gromit",
		},
		deps: &Deps{
			ReviewRenderer: mockRenderer,
		},
	}

	input := ReviewInput{
		FromCommit: "abc123",
		Diff:       "test diff",
	}

	result, err := p.renderReviewPrompt(input)
	if err != nil {
		t.Fatalf("renderReviewPrompt failed: %v", err)
	}

	if result == "" {
		t.Errorf("expected non-empty prompt, got empty string")
	}

	if receivedInput == nil {
		t.Errorf("ReviewRenderer.RenderThoroughReview was not called")
	}

	// After the fix: pipeline should populate ClaudeMD and Rules in the input
	// so the adapter (cli_adapters.go) doesn't need to load them from files
	if receivedInput.ClaudeMD == "" && receivedInput.Rules == "" {
		t.Errorf("RenderReviewPrompt should populate ClaudeMD and Rules - currently empty")
	}
}

// mockReviewRendererTracker allows inspecting what the ReviewRenderer receives
type mockReviewRendererTracker struct {
	onRenderThoroughReview func(*ThoroughReviewPromptInput) (string, error)
}

func (m *mockReviewRendererTracker) RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error) {
	if m.onRenderThoroughReview != nil {
		return m.onRenderThoroughReview(input)
	}
	return "", nil
}

func (m *mockReviewRendererTracker) LoadClaudeMD() (string, error) {
	return "mock claude.md content", nil
}

func (m *mockReviewRendererTracker) LoadRulesForPhase(phase string) (string, error) {
	return "mock rules content", nil
}
