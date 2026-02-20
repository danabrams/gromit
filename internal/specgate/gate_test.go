package specgate

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGate_Run_allPass(t *testing.T) {
	verdictJSON := `{"passed":true,"results":[{"criterion":"No TODOs","passed":true,"evidence":"none found"}]}`
	g := newGateFixture()
	g.InvokeLLM = func(ctx context.Context, model, prompt string) ([]byte, error) {
		return []byte(verdictJSON), nil
	}

	verdict, err := runGate(t, g, []string{"No TODOs"})
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
	g := newGateFixture()
	g.RunTests = func(ctx context.Context) (string, error) {
		return "FAIL", nil
	}
	g.InvokeLLM = func(ctx context.Context, model, prompt string) ([]byte, error) {
		return []byte(verdictJSON), nil
	}

	verdict, err := runGate(t, g, []string{"Tests pass"})
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
	g := newGateFixture()
	g.RunTests = func(ctx context.Context) (string, error) {
		return "", errors.New("tests failed to run")
	}

	_, err := runGate(t, g, []string{"Tests pass"})
	if err == nil {
		t.Error("Run() expected error when RunTests fails, got nil")
	}
}

func TestGate_Run_diffError_returnsError(t *testing.T) {
	g := newGateFixture()
	g.GetDiff = func(ctx context.Context) (string, error) {
		return "", errors.New("diff failed")
	}

	_, err := runGate(t, g, []string{"Tests pass"})
	if err == nil {
		t.Error("Run() expected error when GetDiff fails, got nil")
	}
}

func TestGate_Run_renderPromptError_returnsError(t *testing.T) {
	g := newGateFixture()
	g.RenderPrompt = func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
		return "", errors.New("render failed")
	}

	_, err := runGate(t, g, []string{"Tests pass"})
	if err == nil {
		t.Error("Run() expected error when RenderPrompt fails, got nil")
	}
}

func TestGate_Run_llmError_returnsFailedVerdict(t *testing.T) {
	g := newGateFixture()
	g.InvokeLLM = func(ctx context.Context, model, prompt string) ([]byte, error) {
		return nil, errors.New("LLM unavailable")
	}

	verdict, err := runGate(t, g, []string{"Tests pass"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if verdict.Passed {
		t.Fatal("verdict.Passed = true, want false")
	}
	if len(verdict.Results) != 1 {
		t.Fatalf("len(verdict.Results) = %d, want 1", len(verdict.Results))
	}
	if verdict.Results[0].Criterion != "LLM invocation" {
		t.Fatalf("result criterion = %q, want %q", verdict.Results[0].Criterion, "LLM invocation")
	}
	if !strings.Contains(verdict.Results[0].Evidence, "LLM unavailable") {
		t.Fatalf("result evidence = %q, expected LLM error details", verdict.Results[0].Evidence)
	}
}

func TestGate_Run_parseVerdictError_returnsError(t *testing.T) {
	g := newGateFixture()
	g.InvokeLLM = func(ctx context.Context, model, prompt string) ([]byte, error) {
		return []byte("not-json"), nil
	}

	_, err := runGate(t, g, []string{"Tests pass"})
	if err == nil {
		t.Error("Run() expected error when ParseVerdict fails, got nil")
	}
}

func TestGate_Run_invokeAfterRenderWithExpectedInputs(t *testing.T) {
	g := newGateFixture()
	var (
		gotSpecName  string
		gotTestsOut  string
		gotDiff      string
		gotCriteria  []string
		gotModel     string
		gotPrompt    string
		renderCalled bool
	)
	g.RunTests = func(ctx context.Context) (string, error) {
		return "tests-output", nil
	}
	g.GetDiff = func(ctx context.Context) (string, error) {
		return "git-diff", nil
	}
	g.RenderPrompt = func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
		renderCalled = true
		gotSpecName = specName
		gotTestsOut = testOutput
		gotDiff = diff
		gotCriteria = append([]string{}, criteria...)
		return "rendered-prompt", nil
	}
	g.InvokeLLM = func(ctx context.Context, model, prompt string) ([]byte, error) {
		if !renderCalled {
			t.Fatal("InvokeLLM called before RenderPrompt")
		}
		gotModel = model
		gotPrompt = prompt
		return []byte(`{"passed": true, "results": []}`), nil
	}

	criteria := []string{"criterion a", "criterion b"}
	_, err := runGate(t, g, criteria)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotSpecName != "myspec" {
		t.Fatalf("specName = %q, want %q", gotSpecName, "myspec")
	}
	if gotTestsOut != "tests-output" {
		t.Fatalf("testOutput = %q, want %q", gotTestsOut, "tests-output")
	}
	if gotDiff != "git-diff" {
		t.Fatalf("diff = %q, want %q", gotDiff, "git-diff")
	}
	if len(gotCriteria) != 2 || gotCriteria[0] != "criterion a" || gotCriteria[1] != "criterion b" {
		t.Fatalf("criteria = %#v, want %#v", gotCriteria, criteria)
	}
	if gotModel != "haiku" {
		t.Fatalf("model = %q, want %q", gotModel, "haiku")
	}
	if gotPrompt != "rendered-prompt" {
		t.Fatalf("prompt = %q, want %q", gotPrompt, "rendered-prompt")
	}
}

func TestGate_Run_nilDependency_returnsError(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Gate)
		wantErr string
	}{
		{
			name: "run tests",
			mutate: func(g *Gate) {
				g.RunTests = nil
			},
			wantErr: "run tests dependency is nil",
		},
		{
			name: "get diff",
			mutate: func(g *Gate) {
				g.GetDiff = nil
			},
			wantErr: "get diff dependency is nil",
		},
		{
			name: "render prompt",
			mutate: func(g *Gate) {
				g.RenderPrompt = nil
			},
			wantErr: "render prompt dependency is nil",
		},
		{
			name: "invoke llm",
			mutate: func(g *Gate) {
				g.InvokeLLM = nil
			},
			wantErr: "invoke llm dependency is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newGateFixture()
			tt.mutate(&g)
			_, err := runGate(t, g, []string{"Tests pass"})
			if err == nil {
				t.Fatal("Run() expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("Run() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGate_Run_nilReceiver_returnsError(t *testing.T) {
	var g *Gate
	_, err := g.Run(context.Background(), "myspec", []string{"criterion"})
	if err == nil {
		t.Fatal("Run() expected error for nil receiver, got nil")
	}
	if err.Error() != "gate is nil" {
		t.Fatalf("Run() error = %q, want %q", err.Error(), "gate is nil")
	}
}

func runGate(t *testing.T, g Gate, acceptanceCriteria []string) (*GateVerdict, error) {
	t.Helper()

	return g.Run(context.Background(), "myspec", acceptanceCriteria)
}

func newGateFixture() Gate {
	return Gate{
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
			return nil, nil
		},
	}
}
