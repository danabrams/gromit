package acceptor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type perCriterionTimeoutProbeAgent struct {
	callCount       int
	deadlines       []time.Time
	invocationTimes []time.Time
}

func (a *perCriterionTimeoutProbeAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	a.callCount++
	invocationTime := time.Now()

	deadline, ok := ctx.Deadline()
	if !ok {
		return CriterionResult{}, errors.New("missing per-criterion deadline")
	}
	a.deadlines = append(a.deadlines, deadline)
	a.invocationTimes = append(a.invocationTimes, invocationTime)

	if strings.Contains(prompt, "simple fast criterion") {
		return CriterionResult{
			Status:    StatusPass,
			Rationale: "fast path completed",
		}, nil
	}

	if strings.Contains(prompt, "complex slow criterion") {
		<-ctx.Done()
		return CriterionResult{}, ctx.Err()
	}

	return CriterionResult{
		Status:    StatusPass,
		Rationale: "default",
	}, nil
}

func TestScenario_PerCriterionTimeoutFiresOnSlowCriterion(t *testing.T) {
	fastCriterion := "simple fast criterion"
	slowCriterion := "end-to-end complex slow criterion"
	timeoutCfg := TimeoutConfig{
		BaseSeconds:         1,
		RateConstant:        1,
		ComplexityBonusSecs: 1,
		HardMaximumSecs:     2,
	}

	// Seed
	agent := &perCriterionTimeoutProbeAgent{}
	evaluator := NewEvaluator(agent, timeoutCfg)
	input := EvaluateInput{
		Criteria: []string{
			fastCriterion,
			slowCriterion,
		},
		DiffSummary:       "large diff touching multiple modules",
		TaskResults:       "tests mostly pass",
		ValidationResults: "validation pending",
		ReviewFindings:    "needs deeper analysis",
	}

	// Invoke
	_, err := evaluator.Evaluate(context.Background(), input)

	// Assert
	if err == nil {
		t.Fatal("expected timeout error from slow criterion")
	}
	if !strings.Contains(err.Error(), slowCriterion) {
		t.Fatalf("expected error to include timed-out criterion text, got: %v", err)
	}
	if !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("expected error to include deadline exceeded, got: %v", err)
	}

	if agent.callCount != 2 {
		t.Fatalf("expected 2 criterion evaluations, got %d", agent.callCount)
	}
	if len(agent.deadlines) != 2 {
		t.Fatalf("expected 2 per-criterion deadlines, got %d", len(agent.deadlines))
	}
	if len(agent.invocationTimes) != 2 {
		t.Fatalf("expected 2 per-criterion invocation times, got %d", len(agent.invocationTimes))
	}

	// Each criterion should receive its own timeout context.
	if !agent.deadlines[1].After(agent.deadlines[0]) && !agent.deadlines[1].Equal(agent.deadlines[0]) {
		t.Fatalf("expected second criterion deadline to be independently computed; got first=%v second=%v", agent.deadlines[0], agent.deadlines[1])
	}

	if !agent.deadlines[1].After(agent.invocationTimes[1]) {
		t.Fatalf("expected second criterion deadline to be after invocation time, deadline=%v invocation=%v", agent.deadlines[1], agent.invocationTimes[1])
	}
}
