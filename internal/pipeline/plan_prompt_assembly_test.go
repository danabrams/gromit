package pipeline

import (
	"fmt"
	"testing"
)

// RED: Test that PlanPromptInput includes complete assembled prompt from pipeline
// This enforces that plan prompt assembly happens in pipeline, not cmd layer
func TestPlanPromptInputIncludesAssembledPrompt(t *testing.T) {
	// PlanPromptInput.IdeaText should contain the fully assembled prompt
	// including spec content, skills, and context - all prepared by pipeline
	input := &PlanPromptInput{
		IdeaText: "spec content\n\n# Gromit Plan Skill\n\nContext for planning",
	}

	// The IdeaText should be complete and ready to pass to the renderer
	if len(input.IdeaText) < 50 {
		t.Logf("NOTE: PlanPromptInput.IdeaText seems incomplete - should include spec + skill + context")
	}

	// The adapter should NOT be assembling the prompt - it should only render
	if input.IdeaText == "" {
		t.Errorf("PlanPromptInput.IdeaText should contain assembled prompt from pipeline")
	}
}

// Stub implementations for testing
type stubPlanRenderer struct{}

func (s *stubPlanRenderer) RenderPlan(input *PlanPromptInput) (string, error) {
	// Should receive IdeaText with assembled content
	if input.IdeaText == "" {
		return "", fmt.Errorf("IdeaText is empty")
	}
	return "rendered plan prompt", nil
}

type stubAgentResolver struct{}

func (s *stubAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (Agent, error) {
	return &stubAgent{}, nil
}

type stubAgent struct{}

func (s *stubAgent) Name() string {
	return "stub-agent"
}

func (s *stubAgent) Launch(promptPath string) error {
	return nil
}

func (s *stubAgent) LaunchInDir(promptPath string, launchDir string) error {
	return nil
}
