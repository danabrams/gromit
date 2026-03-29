package acceptor

import (
	"context"
	"strings"
	"testing"
	"time"
)

type timeoutCaptureAcceptAgent struct {
	observedTimeout time.Duration
	sawDeadline     bool
	lastPrompt      string
}

func (a *timeoutCaptureAcceptAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	a.lastPrompt = prompt
	if deadline, ok := ctx.Deadline(); ok {
		a.sawDeadline = true
		a.observedTimeout = time.Until(deadline)
	}
	return CriterionResult{
		Status:    StatusPass,
		Rationale: "meets criterion",
	}, nil
}

func TestScenario_ComplexCriterionLargeDiffGetsComplexityBonus(t *testing.T) {
	// Seed
	const criterion = "end-to-end pipeline survives resume after crash"
	const diffBytes = 800_000
	agent := &timeoutCaptureAcceptAgent{}
	eval := NewEvaluator(agent, DefaultTimeoutConfig())
	input := EvaluateInput{
		Criteria:    []string{criterion},
		DiffSummary: strings.Repeat("d", diffBytes),
		TaskResults: "all tasks done",
	}

	// Invoke
	result, err := eval.Evaluate(context.Background(), input)

	// Assert
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if !agent.sawDeadline {
		t.Fatal("expected per-criterion timeout context with deadline")
	}

	want := 340 * time.Second // 60 + 800000/5000 + 120
	const tolerance = 2 * time.Second
	if agent.observedTimeout < want-tolerance || agent.observedTimeout > want+tolerance {
		t.Fatalf("observed timeout %v outside expected range [%v, %v]", agent.observedTimeout, want-tolerance, want+tolerance)
	}

	if got := ClassifyCriterionComplexity(criterion); got != "complex" {
		t.Fatalf("ClassifyCriterionComplexity(%q)=%q, want %q", criterion, got, "complex")
	}

	if !strings.Contains(agent.lastPrompt, criterion) {
		t.Fatalf("prompt missing criterion text %q", criterion)
	}
}
