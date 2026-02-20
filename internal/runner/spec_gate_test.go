package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/validation"
)

type mockSpecGateValidationRunner struct {
	runDirectFn func(ctx context.Context, commands []string, workDir string) (*claude.Result, error)
}

func (m *mockSpecGateValidationRunner) RunDirect(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
	if m.runDirectFn != nil {
		return m.runDirectFn(ctx, commands, workDir)
	}
	return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
}

func TestBuildSpecGate_UsesRunnerDependencies(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	r := &Runner{
		cfg:              cfg,
		renderer:         &mockPromptRenderer{},
		router:           provider.NewSingleProviderRouter(&mockProviderWithRouterTracking{}),
		validationRunner: validation.NewRunner(cfg, nil, nil, nil),
	}

	gate, err := r.buildSpecGate()
	if err != nil {
		t.Fatalf("buildSpecGate() error: %v", err)
	}
	if gate == nil {
		t.Fatal("buildSpecGate() returned nil gate")
	}
	if gate.cfg != cfg {
		t.Fatal("expected gate cfg to reference runner cfg")
	}
	if gate.validationRunner == nil {
		t.Fatal("expected gate validation runner to be wired")
	}
}

func TestSpecGateVerify_ReturnsPassedWhenValidationSucceeds(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Validation.FullCommands = []string{"go test ./..."}

	renderCalls := 0
	validationCalls := 0
	gate := &SpecGate{
		cfg: cfg,
		validationRunner: &mockSpecGateValidationRunner{runDirectFn: func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			validationCalls++
			if strings.Join(commands, " ") != "go test ./..." {
				t.Fatalf("commands = %v, want [go test ./...]", commands)
			}
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
		}},
		renderer: &mockPromptRenderer{RenderSpecGateFn: func(ctx *prompt.SpecGateContext) (string, error) {
			renderCalls++
			return "unused", nil
		}},
		router: provider.NewSingleProviderRouter(&mockProviderWithRouterTracking{}),
	}

	result, err := gate.Verify(context.Background(), "demo-spec", "## Acceptance Criteria\n- works")
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if !result.Passed {
		t.Fatalf("result.Passed = false, want true")
	}
	if len(result.Failures) != 0 {
		t.Fatalf("result.Failures = %v, want empty", result.Failures)
	}
	if validationCalls != 1 {
		t.Fatalf("validation calls = %d, want 1", validationCalls)
	}
	if renderCalls != 0 {
		t.Fatalf("renderer calls = %d, want 0", renderCalls)
	}
}

func TestSpecGateVerify_ParsesStructuredFailures(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var renderedFailureOutput string
	var renderedSpec string
	providerRuns := 0
	gate := &SpecGate{
		cfg: cfg,
		validationRunner: &mockSpecGateValidationRunner{runDirectFn: func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: false, Output: "failing test output"}, nil
		}},
		renderer: &mockPromptRenderer{RenderSpecGateFn: func(ctx *prompt.SpecGateContext) (string, error) {
			renderedFailureOutput = ctx.FailureOutput
			renderedSpec = ctx.SpecCriteria
			return "rendered gate prompt", nil
		}},
		router: provider.NewSingleProviderRouter(&mockProviderWithRouterTracking{runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerRuns++
			if prompt != "rendered gate prompt" {
				t.Fatalf("prompt = %q, want rendered gate prompt", prompt)
			}
			return &provider.Result{Success: true, Output: `{"passed":false,"failures":[{"test_name":"TestSpecBehavior","message":"expected 200 got 500","suggested_fix":"handle nil response"}]}`}, nil
		}}),
	}

	result, err := gate.Verify(context.Background(), "demo-spec", "## Acceptance Criteria\n- returns 200")
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if result.Passed {
		t.Fatal("result.Passed = true, want false")
	}
	if len(result.Failures) != 1 {
		t.Fatalf("len(result.Failures) = %d, want 1", len(result.Failures))
	}
	if result.Failures[0].TestName != "TestSpecBehavior" {
		t.Fatalf("TestName = %q, want TestSpecBehavior", result.Failures[0].TestName)
	}
	if renderedFailureOutput != "failing test output" {
		t.Fatalf("FailureOutput = %q, want failing test output", renderedFailureOutput)
	}
	if !strings.Contains(renderedSpec, "Acceptance Criteria") {
		t.Fatalf("SpecCriteria = %q, want spec content", renderedSpec)
	}
	if providerRuns != 1 {
		t.Fatalf("provider runs = %d, want 1", providerRuns)
	}
}

func TestSpecGateVerify_ProviderErrorsReturnStructuredFailure(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	gate := &SpecGate{
		cfg: cfg,
		validationRunner: &mockSpecGateValidationRunner{runDirectFn: func(ctx context.Context, commands []string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: false, Output: "validation failed"}, nil
		}},
		renderer: &mockPromptRenderer{RenderSpecGateFn: func(ctx *prompt.SpecGateContext) (string, error) {
			return "rendered prompt", nil
		}},
		router: provider.NewSingleProviderRouter(&mockProviderWithRouterTracking{runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return nil, errors.New("provider unavailable")
		}}),
	}

	result, err := gate.Verify(context.Background(), "demo-spec", "## Acceptance Criteria\n- works")
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if result.Passed {
		t.Fatal("result.Passed = true, want false")
	}
	if len(result.Failures) != 1 {
		t.Fatalf("len(result.Failures) = %d, want 1", len(result.Failures))
	}
	if !strings.Contains(result.Failures[0].Message, "provider invocation failed") {
		t.Fatalf("failure message = %q, want provider invocation failure", result.Failures[0].Message)
	}
}

func TestParseGateResult_ConvertsLegacyGateVerdict(t *testing.T) {
	result, err := parseGateResult([]byte(`{"passed":false,"results":[{"criterion":"criterion-1","passed":false,"evidence":"failing evidence"}]}`))
	if err != nil {
		t.Fatalf("parseGateResult() error: %v", err)
	}
	if result.Passed {
		t.Fatal("result.Passed = true, want false")
	}
	if len(result.Failures) != 1 {
		t.Fatalf("len(result.Failures) = %d, want 1", len(result.Failures))
	}
	if result.Failures[0].TestName != "criterion-1" {
		t.Fatalf("TestName = %q, want criterion-1", result.Failures[0].TestName)
	}
}
