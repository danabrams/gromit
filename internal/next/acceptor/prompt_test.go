package acceptor

import "testing"

func TestRenderAcceptancePrompt_ContainsCriterion(t *testing.T) {
	prompt, err := RenderAcceptancePrompt(AcceptancePromptInput{
		Criterion:         "refund endpoint returns 200",
		DiffSummary:       "Added refund handler",
		TaskResults:       "4/4 tasks passed",
		ValidationResults: "6/6 checks passed",
		ReviewFindings:    "0 findings",
	})
	if err != nil {
		t.Fatalf("RenderAcceptancePrompt: %v", err)
	}
	if !containsSubstring(prompt, "refund endpoint returns 200") {
		t.Error("prompt should contain the criterion text")
	}
	if !containsSubstring(prompt, "pass") && !containsSubstring(prompt, "fail") && !containsSubstring(prompt, "unclear") {
		t.Error("prompt should mention the three possible statuses")
	}
}

func TestRenderAcceptancePrompt_UnclearGuidance(t *testing.T) {
	prompt, err := RenderAcceptancePrompt(AcceptancePromptInput{
		Criterion:   "audit log entry created",
		DiffSummary: "Added handler",
	})
	if err != nil {
		t.Fatalf("RenderAcceptancePrompt: %v", err)
	}
	// Prompt should instruct the LLM about "unclear" meaning
	if !containsSubstring(prompt, "unclear") {
		t.Error("prompt should explain the unclear status")
	}
}

func TestRenderAcceptancePrompt_JSONOnlyInstruction(t *testing.T) {
	prompt, err := RenderAcceptancePrompt(AcceptancePromptInput{
		Criterion:   "logs written",
		DiffSummary: "Added logging",
	})
	if err != nil {
		t.Fatalf("RenderAcceptancePrompt: %v", err)
	}
	if !containsSubstring(prompt, "Return ONLY the JSON object. No markdown, no prose, no text before or after the JSON.") {
		t.Error("prompt should instruct the LLM to respond with only JSON")
	}
}
