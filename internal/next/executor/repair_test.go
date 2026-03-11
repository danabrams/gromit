package executor

import (
	"context"
	"testing"
)

type countingAgent struct {
	fn func() string
}

func (a *countingAgent) Invoke(ctx context.Context, prompt, tier string) (AgentResult, error) {
	return AgentResult{Output: a.fn(), TokensIn: 10, TokensOut: 5, Cost: 0.001}, nil
}

type fakeValidator struct {
	passOnCall int
	callCount  int
}

func (v *fakeValidator) Validate(ctx context.Context, workDir string) (bool, []string, error) {
	v.callCount++
	if v.callCount >= v.passOnCall {
		return true, nil, nil
	}
	return false, []string{"check failed"}, nil
}

func TestRepairLoop_SucceedsOnRetry(t *testing.T) {
	callCount := 0
	agent := &countingAgent{fn: func() string {
		callCount++
		if callCount == 1 {
			return "still broken"
		}
		return "fixed"
	}}
	validator := &fakeValidator{passOnCall: 2}

	result, err := RepairLoop(context.Background(), RepairInput{
		Agent:      agent,
		Validator:  validator,
		MaxRetries: 1,
		Packet:     "fix the bug",
		WorkDir:    t.TempDir(),
		ModelTier:  "medium",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Attempts != 2 {
		t.Fatalf("want 2 attempts, got %d", result.Attempts)
	}
	if result.Status != "done" {
		t.Fatalf("want done, got %s", result.Status)
	}
}

func TestRepairLoop_ExhaustsRetries(t *testing.T) {
	agent := &countingAgent{fn: func() string { return "still broken" }}
	validator := &fakeValidator{passOnCall: 999}

	result, err := RepairLoop(context.Background(), RepairInput{
		Agent: agent, Validator: validator, MaxRetries: 1,
		Packet: "fix", WorkDir: t.TempDir(), ModelTier: "medium",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" {
		t.Fatalf("want failed, got %s", result.Status)
	}
	if result.Attempts != 2 {
		t.Fatalf("want 2 attempts (1 initial + 1 retry), got %d", result.Attempts)
	}
}

func TestRepairLoop_PassesFirstTime(t *testing.T) {
	agent := &countingAgent{fn: func() string { return "done" }}
	validator := &fakeValidator{passOnCall: 1}

	result, err := RepairLoop(context.Background(), RepairInput{
		Agent: agent, Validator: validator, MaxRetries: 3,
		Packet: "task", WorkDir: t.TempDir(), ModelTier: "medium",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "done" {
		t.Fatalf("want done, got %s", result.Status)
	}
	if result.Attempts != 1 {
		t.Fatalf("want 1 attempt, got %d", result.Attempts)
	}
}
