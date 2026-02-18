package specgate

import (
	"context"
	"errors"
	"testing"
)

func TestGate_Run_allPass(t *testing.T) {
	verdictJSON := `{"passed":true,"results":[{"criterion":"No TODOs","passed":true,"evidence":"none found"}]}`
	g := Gate{
		Model:     "haiku",
		MaxCycles: 1,
		RunTests: func(ctx context.Context) (string, error) {
			return "ok", nil
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return "diff output", nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			return "rendered prompt", nil
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			return []byte(verdictJSON), nil
		},
	}

	verdict, err := g.Run(context.Background(), "myspec", []string{"No TODOs"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !verdict.Passed {
		t.Errorf("verdict.Passed = false, want true")
	}
	if len(verdict.Results) != 1 {
		t.Fatalf("len(verdict.Results) = %d, want 1", len(verdict.Results))
	}
}

func TestGate_Run_someFail(t *testing.T) {
	verdictJSON := `{"passed":false,"results":[{"criterion":"Tests pass","passed":false,"evidence":"test output"}]}`
	g := Gate{
		Model:     "haiku",
		MaxCycles: 1,
		RunTests: func(ctx context.Context) (string, error) {
			return "FAIL", nil
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return "diff output", nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			return "rendered prompt", nil
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			return []byte(verdictJSON), nil
		},
	}

	verdict, err := g.Run(context.Background(), "myspec", []string{"Tests pass"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if verdict.Passed {
		t.Errorf("verdict.Passed = true, want false")
	}
	failed := verdict.FailedCriteria()
	if len(failed) != 1 {
		t.Fatalf("len(FailedCriteria()) = %d, want 1", len(failed))
	}
}

func TestGate_Run_testError_returnsError(t *testing.T) {
	g := Gate{
		Model:     "haiku",
		MaxCycles: 1,
		RunTests: func(ctx context.Context) (string, error) {
			return "", errors.New("tests failed to run")
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return "diff output", nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			return "rendered prompt", nil
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			return nil, nil
		},
	}

	_, err := g.Run(context.Background(), "myspec", []string{"Tests pass"})
	if err == nil {
		t.Error("Run() expected error when RunTests fails, got nil")
	}
}

func TestGate_Run_llmError_returnsError(t *testing.T) {
	g := Gate{
		Model:     "haiku",
		MaxCycles: 1,
		RunTests: func(ctx context.Context) (string, error) {
			return "ok", nil
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return "diff output", nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			return "rendered prompt", nil
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			return nil, errors.New("LLM unavailable")
		},
	}

	_, err := g.Run(context.Background(), "myspec", []string{"Tests pass"})
	if err == nil {
		t.Error("Run() expected error when InvokeLLM fails, got nil")
	}
}
