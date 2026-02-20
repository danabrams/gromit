package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/validation"
)

func TestBuildSpecGate_UsesRunnerDependencies(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.Validation.FullCommands = []string{"go test ./..."}
	cfg.SpecGate.Model = "sonnet"
	cfg.SpecGate.MaxCycles = 4

	var gotCommand string
	var gotWorkDir string
	r := &Runner{
		cfg:       cfg,
		renderer:  &mockPromptRenderer{},
		router:    provider.NewSingleProviderRouter(&mockProviderWithRouterTracking{}),
		gromitDir: "/tmp/gromit",
		validationRunner: validation.NewRunner(cfg, func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			gotCommand = command
			gotWorkDir = workDir
			return "ok", "", 0, nil
		}, nil, nil),
	}

	gate, err := r.buildSpecGate()
	if err != nil {
		t.Fatalf("buildSpecGate() error: %v", err)
	}
	if gate == nil {
		t.Fatal("buildSpecGate() returned nil gate")
	}
	if gate.Model != "sonnet" {
		t.Fatalf("gate.Model = %q, want sonnet", gate.Model)
	}
	if gate.MaxCycles != 4 {
		t.Fatalf("gate.MaxCycles = %d, want 4", gate.MaxCycles)
	}

	if _, err := gate.RunTests(context.Background()); err != nil {
		t.Fatalf("RunTests() error: %v", err)
	}
	if gotCommand != "go test ./..." {
		t.Fatalf("validation command = %q, want %q", gotCommand, "go test ./...")
	}
	if gotWorkDir != "/tmp/gromit" {
		t.Fatalf("validation workdir = %q, want %q", gotWorkDir, "/tmp/gromit")
	}
}

func TestBuildSpecGate_RenderPromptUsesSpecGateContext(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var gotCtx *prompt.SpecGateContext
	r := &Runner{
		cfg: cfg,
		renderer: &mockPromptRenderer{RenderSpecGateFn: func(ctx *prompt.SpecGateContext) (string, error) {
			gotCtx = ctx
			return "rendered", nil
		}},
		router: provider.NewSingleProviderRouter(&mockProviderWithRouterTracking{}),
		validationRunner: validation.NewRunner(cfg, func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "ok", "", 0, nil
		}, nil, nil),
	}

	gate, err := r.buildSpecGate()
	if err != nil {
		t.Fatalf("buildSpecGate() error: %v", err)
	}

	criteria := []string{"works", "is deterministic"}
	promptText, err := gate.RenderPrompt(context.Background(), "demo-spec", "test output", "diff output", criteria)
	if err != nil {
		t.Fatalf("RenderPrompt() error: %v", err)
	}
	if promptText != "rendered" {
		t.Fatalf("RenderPrompt() = %q, want rendered", promptText)
	}
	if gotCtx == nil {
		t.Fatal("expected SpecGateContext to be captured")
	}
	if gotCtx.SpecCriteria != "spec:demo-spec" {
		t.Fatalf("SpecCriteria = %q, want %q", gotCtx.SpecCriteria, "spec:demo-spec")
	}
	if gotCtx.TestOutput != "test output" {
		t.Fatalf("TestOutput = %q, want %q", gotCtx.TestOutput, "test output")
	}
	if gotCtx.CumulativeDiff != "diff output" {
		t.Fatalf("CumulativeDiff = %q, want %q", gotCtx.CumulativeDiff, "diff output")
	}
	if !strings.Contains(gotCtx.AcceptanceCriteria, "- works") {
		t.Fatalf("AcceptanceCriteria = %q, expected formatted criteria", gotCtx.AcceptanceCriteria)
	}
}

func TestBuildSpecGate_InvokeLLMUsesConfiguredTier(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	cfg.SpecGate.Model = "sonnet"

	var gotTier string
	r := &Runner{
		cfg:      cfg,
		renderer: &mockPromptRenderer{},
		router: provider.NewSingleProviderRouter(&mockProviderWithRouterTracking{runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			gotTier = tier
			return &provider.Result{Success: true, Output: "{}"}, nil
		}}),
		validationRunner: validation.NewRunner(cfg, func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "ok", "", 0, nil
		}, nil, nil),
	}

	gate, err := r.buildSpecGate()
	if err != nil {
		t.Fatalf("buildSpecGate() error: %v", err)
	}

	result, err := gate.InvokeLLM(context.Background(), "sonnet", "prompt")
	if err != nil {
		t.Fatalf("InvokeLLM() error: %v", err)
	}
	if string(result) != "{}" {
		t.Fatalf("InvokeLLM() output = %q, want {}", string(result))
	}
	if gotTier != "medium" {
		t.Fatalf("provider tier = %q, want medium", gotTier)
	}
}

func TestExtractAcceptanceCriteria_ParsesBulletsAndNumbers(t *testing.T) {
	body := "## Acceptance Criteria\n- first\n2. second\n* third\n\n## Notes\nignored"
	criteria, block := extractAcceptanceCriteria(body)
	if len(criteria) != 3 {
		t.Fatalf("len(criteria) = %d, want 3", len(criteria))
	}
	if criteria[0] != "first" || criteria[1] != "second" || criteria[2] != "third" {
		t.Fatalf("criteria = %v, want [first second third]", criteria)
	}
	if !strings.Contains(block, "2. second") {
		t.Fatalf("block = %q, expected acceptance section", block)
	}
}
