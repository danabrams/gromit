package gate

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/v2/llmtypes"
)

func TestSatisfactionTier_Gen0SkipsCheck(t *testing.T) {
	if got := satisfactionTier(0); got != "" {
		t.Errorf("satisfactionTier(0) = %q, want %q", got, "")
	}
}

func TestSatisfactionTier_Gen1ReturnsLow(t *testing.T) {
	if got := satisfactionTier(1); got != "low" {
		t.Errorf("satisfactionTier(1) = %q, want %q", got, "low")
	}
}

func TestSatisfactionTier_Gen2ReturnsMedium(t *testing.T) {
	if got := satisfactionTier(2); got != "medium" {
		t.Errorf("satisfactionTier(2) = %q, want %q", got, "medium")
	}
}

func TestSatisfactionTier_Gen3ReturnsHigh(t *testing.T) {
	if got := satisfactionTier(3); got != "high" {
		t.Errorf("satisfactionTier(3) = %q, want %q", got, "high")
	}
}

func TestSatisfactionTier_Gen5ReturnsHigh(t *testing.T) {
	if got := satisfactionTier(5); got != "high" {
		t.Errorf("satisfactionTier(5) = %q, want %q", got, "high")
	}
}

func TestIsStructuralBead_RefactorTitle(t *testing.T) {
	if !isStructuralBead("Refactor debug command", "") {
		t.Error("expected true for refactor title")
	}
}

func TestIsStructuralBead_TestTitle(t *testing.T) {
	if !isStructuralBead("Add test coverage for router", "") {
		t.Error("expected true for test coverage title")
	}
}

func TestIsStructuralBead_ReorganizeDescription(t *testing.T) {
	if !isStructuralBead("Clean up", "reorganize the debug package") {
		t.Error("expected true for reorganize in description")
	}
}

func TestIsStructuralBead_NormalBead(t *testing.T) {
	if isStructuralBead("Implement debug command entry point", "") {
		t.Error("expected false for normal bead")
	}
}

func TestIsStructuralBead_ExtractTitle(t *testing.T) {
	if !isStructuralBead("Extract validation logic into helper", "") {
		t.Error("expected true for extract title")
	}
}

func TestIsStructuralBead_MoveTitle(t *testing.T) {
	if !isStructuralBead("Move types to shared package", "") {
		t.Error("expected true for move title")
	}
}

func TestIsStructuralBead_RenameTitle(t *testing.T) {
	if !isStructuralBead("Rename adapter methods for consistency", "") {
		t.Error("expected true for rename title")
	}
}

// fakeLLM implements llmtypes.LLMProvider for testing.
type fakeLLM struct {
	responses []string
	callIndex int
}

func (f *fakeLLM) Invoke(_ context.Context, req llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	if f.callIndex >= len(f.responses) {
		return nil, fmt.Errorf("unexpected call %d", f.callIndex)
	}
	resp := f.responses[f.callIndex]
	f.callIndex++
	return &llmtypes.LLMInvokeResponse{
		Success: true,
		Output:  resp,
	}, nil
}

func (f *fakeLLM) StreamInvoke(_ context.Context, _ llmtypes.LLMStreamInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestCheckSatisfaction_AllCriteriaPass_ReturnsTrue(t *testing.T) {
	llm := &fakeLLM{responses: []string{
		`{"pass": true, "summary": "criterion 1 met"}`,
		`{"pass": true, "summary": "criterion 2 met"}`,
	}}
	ok, err := checkSatisfaction(context.Background(), llm, "low", "diff content", "bead-1", []string{"crit1", "crit2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected true when all criteria pass")
	}
}

func TestCheckSatisfaction_AnyCriterionFails_ReturnsFalse(t *testing.T) {
	llm := &fakeLLM{responses: []string{
		`{"pass": true, "summary": "criterion 1 met"}`,
		`{"pass": false, "summary": "criterion 2 not met"}`,
	}}
	ok, err := checkSatisfaction(context.Background(), llm, "low", "diff content", "bead-1", []string{"crit1", "crit2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false when any criterion fails")
	}
}

func TestCheckSatisfaction_NoCriteria_ReturnsFalse(t *testing.T) {
	llm := &fakeLLM{}
	ok, err := checkSatisfaction(context.Background(), llm, "low", "diff content", "bead-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected false when no criteria provided")
	}
}
