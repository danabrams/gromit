package runner

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/tdd"
)

type callbacksTDDProviderStub struct {
	name        string
	streamRunFn func(ctx context.Context, prompt, tier string, w io.Writer, h provider.EventHandler, tc provider.ToolCallHandler) (*provider.Result, error)
}

func (s *callbacksTDDProviderStub) Name() string                    { return s.name }
func (s *callbacksTDDProviderStub) ModelForTier(tier string) string { return tier }
func (s *callbacksTDDProviderStub) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}
func (s *callbacksTDDProviderStub) StreamRun(ctx context.Context, prompt, tier string, w io.Writer, h provider.EventHandler, tc provider.ToolCallHandler) (*provider.Result, error) {
	if s.streamRunFn != nil {
		return s.streamRunFn(ctx, prompt, tier, w, h, tc)
	}
	return &provider.Result{Success: true}, nil
}
func (s *callbacksTDDProviderStub) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}
func (s *callbacksTDDProviderStub) IsUsageLimitError(result *provider.Result, err error) bool { return false }
func (s *callbacksTDDProviderStub) IsValidationPassed(result *provider.Result) bool           { return true }
func (s *callbacksTDDProviderStub) IsScopeTooLarge(result *provider.Result) (bool, string)    { return false, "" }

// TestAppendTDDPhaseMetric_AppendsPhaseMetricToBcResult verifies that
// appendTDDPhaseMetric appends one PhaseMetric entry to bc.Result.PhaseMetrics
// with the correct phase, cycle, model, tier, delta tokens/cost, duration, and
// success flag — capturing the data that InvokeFn previously discarded.
func TestAppendTDDPhaseMetric_AppendsPhaseMetricToBcResult(t *testing.T) {
	bc := &runtypes.BeadContext{
		Bead:  &bead.Bead{ID: "bead-1"},
		Model: "claude-sonnet-4-6",
		Tier:  "medium",
		Result: &runtypes.IterationResult{
			CostUSD:      0.05,
			InputTokens:  500,
			OutputTokens: 250,
		},
	}
	start := time.Now().Add(-200 * time.Millisecond)

	appendTDDPhaseMetric(bc, "red", 1, 0.02, 200, 100, start)

	if len(bc.Result.PhaseMetrics) != 1 {
		t.Fatalf("expected 1 PhaseMetric, got %d", len(bc.Result.PhaseMetrics))
	}
	pm := bc.Result.PhaseMetrics[0]
	if pm.Phase != "red" {
		t.Errorf("Phase = %q, want %q", pm.Phase, "red")
	}
	if pm.CycleNumber != 1 {
		t.Errorf("CycleNumber = %d, want 1", pm.CycleNumber)
	}
	if pm.Tier != "medium" {
		t.Errorf("Tier = %q, want %q", pm.Tier, "medium")
	}
	if pm.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", pm.Model, "claude-sonnet-4-6")
	}
	if pm.BeadID != "bead-1" {
		t.Errorf("BeadID = %q, want %q", pm.BeadID, "bead-1")
	}
	// Use tolerance for float64 comparison to avoid constant-vs-runtime precision mismatch.
	wantCostUSD := float64(0.05) - float64(0.02)
	if math.Abs(pm.CostUSD-wantCostUSD) > 1e-10 {
		t.Errorf("CostUSD = %.20f, want %.20f", pm.CostUSD, wantCostUSD)
	}
	wantInputTokens := 500 - 200
	if pm.InputTokens != wantInputTokens {
		t.Errorf("InputTokens = %d, want %d", pm.InputTokens, wantInputTokens)
	}
	wantOutputTokens := 250 - 100
	if pm.OutputTokens != wantOutputTokens {
		t.Errorf("OutputTokens = %d, want %d", pm.OutputTokens, wantOutputTokens)
	}
	if pm.DurationMs <= 0 {
		t.Error("DurationMs should be positive")
	}
	if !pm.Success {
		t.Error("Success should be true")
	}
}

func TestBuildRenderRedFn_UsesRedPhaseTierOverride(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_tdd_red.md"), []byte("{{.SpecExcerpt}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_tdd_red.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("# Rules\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	cfg := &config.Config{}
	cfg.Methodology.PhaseModels.Red = "low"

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "b1", Title: "title"},
		Tier: "high",
	}
	handoff := &tdd.RedHandoff{SpecExcerpt: "spec"}

	fn := buildRenderRedFn(cfg, renderer)
	if _, err := fn(handoff, bc); err != nil {
		t.Fatalf("buildRenderRedFn() error = %v", err)
	}
	if bc.Tier != "low" {
		t.Fatalf("bc.Tier = %q, want %q", bc.Tier, "low")
	}
}

func TestBuildRenderRedFn_LoadsRulesForRedPhase(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_tdd_red.md"), []byte("{{.Rules}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_tdd_red.md): %v", err)
	}
	rules := `# Rules

## BuildOnly <!-- phases: build -->
- only-build

## RedOnly <!-- phases: red -->
- only-red
`
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rules), 0o644); err != nil {
		t.Fatalf("WriteFile(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	cfg := &config.Config{}
	bc := &runtypes.BeadContext{Bead: &bead.Bead{ID: "b1", Title: "title"}, Tier: "medium"}
	handoff := &tdd.RedHandoff{}

	fn := buildRenderRedFn(cfg, renderer)
	out, err := fn(handoff, bc)
	if err != nil {
		t.Fatalf("buildRenderRedFn() error = %v", err)
	}
	if !strings.Contains(out, "only-red") {
		t.Fatalf("rendered rules missing red-only content: %q", out)
	}
	if strings.Contains(out, "only-build") {
		t.Fatalf("rendered rules should exclude build-only content: %q", out)
	}
}

func TestBuildRenderRedFn_ReturnsErrorWhenRedRulesLoadFails(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_tdd_red.md"), []byte("{{.Rules}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_tdd_red.md): %v", err)
	}
	// Force ReadFile(RULES.md) to fail with a non-ENOENT error.
	if err := os.Mkdir(filepath.Join(gromitDir, "RULES.md"), 0o755); err != nil {
		t.Fatalf("Mkdir(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	cfg := &config.Config{}
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "b1", Title: "title"},
		Tier: "medium",
	}
	handoff := &tdd.RedHandoff{}

	fn := buildRenderRedFn(cfg, renderer)
	_, err = fn(handoff, bc)
	if err == nil {
		t.Fatal("expected error when loading red phase rules fails")
	}
}

func TestBuildRenderGreenFn_UsesGreenPhaseTierOverride(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_tdd_green.md"), []byte("{{.FailingTest}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_tdd_green.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("# Rules\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	cfg := &config.Config{}
	cfg.Methodology.PhaseModels.Green = "low"

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "b1", Title: "title"},
		Tier: "high",
	}
	handoff := &tdd.GreenHandoff{FailingTest: "test fails"}

	fn := buildRenderGreenFn(cfg, renderer)
	if _, err := fn(handoff, bc); err != nil {
		t.Fatalf("buildRenderGreenFn() error = %v", err)
	}
	if bc.Tier != "low" {
		t.Fatalf("bc.Tier = %q, want %q", bc.Tier, "low")
	}
}

func TestBuildRenderGreenFn_LoadsRulesForGreenPhase(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_tdd_green.md"), []byte("{{.Rules}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_tdd_green.md): %v", err)
	}
	rules := `# Rules

## BuildOnly <!-- phases: build -->
- only-build

## GreenOnly <!-- phases: green -->
- only-green
`
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rules), 0o644); err != nil {
		t.Fatalf("WriteFile(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	cfg := &config.Config{}
	bc := &runtypes.BeadContext{Bead: &bead.Bead{ID: "b1", Title: "title"}, Tier: "medium"}
	handoff := &tdd.GreenHandoff{}

	fn := buildRenderGreenFn(cfg, renderer)
	out, err := fn(handoff, bc)
	if err != nil {
		t.Fatalf("buildRenderGreenFn() error = %v", err)
	}
	if !strings.Contains(out, "only-green") {
		t.Fatalf("rendered rules missing green-only content: %q", out)
	}
	if strings.Contains(out, "only-build") {
		t.Fatalf("rendered rules should exclude build-only content: %q", out)
	}
}

func TestBuildRenderGreenFn_ReturnsErrorWhenGreenRulesLoadFails(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_tdd_green.md"), []byte("{{.Rules}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_tdd_green.md): %v", err)
	}
	// Force ReadFile(RULES.md) to fail with a non-ENOENT error.
	if err := os.Mkdir(filepath.Join(gromitDir, "RULES.md"), 0o755); err != nil {
		t.Fatalf("Mkdir(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	cfg := &config.Config{}
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "b1", Title: "title"},
		Tier: "medium",
	}
	handoff := &tdd.GreenHandoff{}

	fn := buildRenderGreenFn(cfg, renderer)
	_, err = fn(handoff, bc)
	if err == nil {
		t.Fatal("expected error when loading green phase rules fails")
	}
}

func TestBuildRunRefactorFn_SelectsProviderForRefactorPhase(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_refactor.md"), []byte("refactor"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_refactor.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("# Rules\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	buildCalled := false
	refactorCalled := false
	router := provider.NewRouter(
		map[string]provider.Provider{
			"build-provider": &callbacksTDDProviderStub{
				name: "build-provider",
				streamRunFn: func(ctx context.Context, prompt, tier string, w io.Writer, h provider.EventHandler, tc provider.ToolCallHandler) (*provider.Result, error) {
					buildCalled = true
					return &provider.Result{Success: true}, nil
				},
			},
			"refactor-provider": &callbacksTDDProviderStub{
				name: "refactor-provider",
				streamRunFn: func(ctx context.Context, prompt, tier string, w io.Writer, h provider.EventHandler, tc provider.ToolCallHandler) (*provider.Result, error) {
					refactorCalled = true
					return &provider.Result{Success: true}, nil
				},
			},
		},
		map[string]string{
			"build":    "build-provider",
			"refactor": "refactor-provider",
		},
		map[string]int{
			"build-provider":    50,
			"refactor-provider": 50,
		},
		0,
		nil,
		nil,
	)

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "b1", Title: "title"},
		Tier: "medium",
	}

	cfg := &config.Config{}
	fn := buildRunRefactorFn(cfg, renderer, router, io.Discard)
	if err := fn(context.Background(), bc); err != nil {
		t.Fatalf("buildRunRefactorFn() error = %v", err)
	}
	if !refactorCalled {
		t.Fatal("expected refactor-provider to be selected for refactor phase")
	}
	if buildCalled {
		t.Fatal("build-provider should not be selected for refactor phase")
	}
}

func TestBuildRunRefactorFn_LoadsRulesForRefactorPhase(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_refactor.md"), []byte("{{.Rules}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_refactor.md): %v", err)
	}
	rules := `# Rules

## BuildOnly <!-- phases: build -->
- only-build

## RefactorOnly <!-- phases: refactor -->
- only-refactor
`
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rules), 0o644); err != nil {
		t.Fatalf("WriteFile(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	var renderedPrompt string
	router := provider.NewSingleProviderRouter(&callbacksTDDProviderStub{
		name: "refactor-provider",
		streamRunFn: func(ctx context.Context, prompt, tier string, w io.Writer, h provider.EventHandler, tc provider.ToolCallHandler) (*provider.Result, error) {
			renderedPrompt = prompt
			return &provider.Result{Success: true}, nil
		},
	})

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "b1", Title: "title"},
		Tier: "medium",
	}

	cfg := &config.Config{}
	fn := buildRunRefactorFn(cfg, renderer, router, io.Discard)
	if err := fn(context.Background(), bc); err != nil {
		t.Fatalf("buildRunRefactorFn() error = %v", err)
	}
	if !strings.Contains(renderedPrompt, "only-refactor") {
		t.Fatalf("rendered prompt missing refactor-only rules: %q", renderedPrompt)
	}
	if strings.Contains(renderedPrompt, "only-build") {
		t.Fatalf("rendered prompt should exclude build-only rules: %q", renderedPrompt)
	}
}

func TestBuildRunRefactorFn_ReturnsErrorWhenRefactorRulesLoadFails(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_refactor.md"), []byte("{{.Rules}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_refactor.md): %v", err)
	}
	// Force ReadFile(RULES.md) to fail with a non-ENOENT error.
	if err := os.Mkdir(filepath.Join(gromitDir, "RULES.md"), 0o755); err != nil {
		t.Fatalf("Mkdir(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	router := provider.NewSingleProviderRouter(&callbacksTDDProviderStub{name: "refactor-provider"})
	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "b1", Title: "title"},
		Tier: "medium",
	}
	cfg := &config.Config{}

	fn := buildRunRefactorFn(cfg, renderer, router, io.Discard)
	err = fn(context.Background(), bc)
	if err == nil {
		t.Fatal("expected error when loading refactor phase rules fails")
	}
}

func TestBuildRunRefactorFn_UsesRefactorPhaseTierOverride(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(templates): %v", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(specs): %v", err)
	}
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(.gromit): %v", err)
	}

	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_refactor.md"), []byte("refactor"), 0o644); err != nil {
		t.Fatalf("WriteFile(PROMPT_refactor.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("# Rules\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(RULES.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md): %v", err)
	}

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("prompt.NewRenderer: %v", err)
	}

	cfg := &config.Config{}
	cfg.Methodology.PhaseModels.Refactor = "low"

	var invokedTier string
	router := provider.NewSingleProviderRouter(&callbacksTDDProviderStub{
		name: "refactor-provider",
		streamRunFn: func(ctx context.Context, prompt, tier string, w io.Writer, h provider.EventHandler, tc provider.ToolCallHandler) (*provider.Result, error) {
			invokedTier = tier
			return &provider.Result{Success: true}, nil
		},
	})

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{ID: "b1", Title: "title"},
		Tier: "high",
	}

	fn := buildRunRefactorFn(cfg, renderer, router, io.Discard)
	if err := fn(context.Background(), bc); err != nil {
		t.Fatalf("buildRunRefactorFn() error = %v", err)
	}
	wantTier := cfg.PhaseModelTier("refactor", "high")
	if invokedTier != wantTier {
		t.Fatalf("invoked tier = %q, want %q", invokedTier, wantTier)
	}
	if bc.Tier != wantTier {
		t.Fatalf("bc.Tier = %q, want %q", bc.Tier, wantTier)
	}
}
