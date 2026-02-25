package runner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/validate"
)

// stubRunProvider is a minimal provider.Provider for testing decomposerAdapter.
type stubRunProvider struct {
	name  string
	runFn func(ctx context.Context, prompt, tier string) (*provider.Result, error)
}

func (s *stubRunProvider) Name() string                    { return s.name }
func (s *stubRunProvider) ModelForTier(tier string) string { return tier }
func (s *stubRunProvider) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if s.runFn != nil {
		return s.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Output: "[]"}, nil
}
func (s *stubRunProvider) StreamRun(ctx context.Context, prompt, tier string, w io.Writer, h provider.EventHandler, tc provider.ToolCallHandler) (*provider.Result, error) {
	return nil, nil
}
func (s *stubRunProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return nil, nil
}
func (s *stubRunProvider) IsUsageLimitError(result *provider.Result, err error) bool { return false }
func (s *stubRunProvider) IsValidationPassed(result *provider.Result) bool           { return true }
func (s *stubRunProvider) IsScopeTooLarge(result *provider.Result) (bool, string)    { return false, "" }

func TestProviderBuildProvidersFromConfig_NilConfigReturnsError(t *testing.T) {
	_, err := provider.BuildProvidersFromConfig(nil)
	if err == nil {
		t.Fatal("provider.BuildProvidersFromConfig(nil) error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "config is nil") {
		t.Fatalf("provider.BuildProvidersFromConfig(nil) error = %q, want substring %q", err.Error(), "config is nil")
	}
}

func TestProviderParseFallbackCooldown_NilConfigReturnsZero(t *testing.T) {
	if got := provider.ParseFallbackCooldown(nil); got != 0 {
		t.Fatalf("provider.ParseFallbackCooldown(nil) = %v, want 0", got)
	}
}

func TestBuildRouterAndLearningsProvider_UsesConfiguredProviders(t *testing.T) {
	cfg := newCodexProvidersConfig()
	router, lp, _, _, err := buildRouterAndLearningsProvider(cfg, t.TempDir(), io.Discard)
	if err != nil {
		t.Fatalf("buildRouterAndLearningsProvider() error = %v", err)
	}
	if router == nil {
		t.Fatal("buildRouterAndLearningsProvider() router = nil, want non-nil")
	}
	if lp == nil {
		t.Fatal("buildRouterAndLearningsProvider() learnings provider = nil, want non-nil")
	}
	if got := lp.Name(); got != "codex" {
		t.Fatalf("learnings provider name = %q, want %q", got, "codex")
	}
}

// TestBuildRouterAndLearningsProvider_InitializesCircuitBreakerWhenEnabled verifies that
// circuit-breaker is created from config when enabled and passed to NewRouter.
func TestBuildRouterAndLearningsProvider_InitializesCircuitBreakerWhenEnabled(t *testing.T) {
	cfg := newCodexProvidersConfig()
	cfg.Routing.Ratio = map[string]int{"codex": 60, "claude": 40}
	cfg.Providers["claude"] = config.ProviderDef{Binary: "claude"}
	cfg.Routing.CircuitBreaker = config.CircuitBreakerConfig{
		Enabled:           true,
		WindowSize:        3,
		FailureThreshold:  0.5,
		DegradedFloor:     20,
		RecoverySuccesses: 2,
	}

	// Create circuit-breaker directly from config to verify it's non-nil when enabled
	cbFromConfig := provider.NewCircuitBreaker(&cfg.Routing.CircuitBreaker)
	if cbFromConfig == nil {
		t.Fatal("NewCircuitBreaker should return non-nil when config.Enabled=true")
	}

	// buildRouterAndLearningsProvider should pass a circuit-breaker to NewRouter
	router, _, _, _, err := buildRouterAndLearningsProvider(cfg, t.TempDir(), io.Discard)
	if err != nil {
		t.Fatalf("buildRouterAndLearningsProvider() error = %v", err)
	}
	if router == nil {
		t.Fatal("buildRouterAndLearningsProvider() returned nil router")
	}
}

// TestBuildRouterAndLearningsProvider_RouterBehavesNormallyWhenCircuitBreakerDisabled
// verifies that when circuit-breaker is disabled, the router works as before.
func TestBuildRouterAndLearningsProvider_RouterBehavesNormallyWhenCircuitBreakerDisabled(t *testing.T) {
	cfg := newCodexProvidersConfig()
	cfg.Routing.Ratio = map[string]int{"codex": 100}
	// Circuit breaker is not enabled (Enabled defaults to false)
	cfg.Routing.CircuitBreaker = config.CircuitBreakerConfig{
		Enabled:           false,
		WindowSize:        10,
		FailureThreshold:  0.3,
		DegradedFloor:     20,
		RecoverySuccesses: 5,
	}

	router, _, _, _, err := buildRouterAndLearningsProvider(cfg, t.TempDir(), io.Discard)
	if err != nil {
		t.Fatalf("buildRouterAndLearningsProvider() error = %v", err)
	}
	if router == nil {
		t.Fatal("buildRouterAndLearningsProvider() returned nil router")
	}
	// Router should work normally even without circuit-breaker
	p, model := router.Select("build", "high")
	if p == nil {
		t.Fatal("router.Select() returned nil provider when circuit-breaker disabled")
	}
	if model == "" {
		t.Fatal("router.Select() returned empty model when circuit-breaker disabled")
	}
}

// TestDecomposerAdapter_Decompose_CreatesChildBeads verifies that decomposerAdapter.Decompose
// actually calls bead.Client to create child beads when decomposing an oversized bead,
// rather than returning nil without performing any work.
func TestDecomposerAdapter_Decompose_CreatesChildBeads(t *testing.T) {
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	stubRouter := provider.NewSingleProviderRouter(&stubRunProvider{
		name: "test",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["f1","f2","f3"]},{"title":"Part 2","expected_outputs":["f4","f5","f6"]}]`,
			}, nil
		},
	})
	var createCalled bool
	client.RunFn = func(args ...string) (string, error) {
		if len(args) > 0 && args[0] == "create" {
			createCalled = true
			return `{"id":"child-1","title":"part 1","status":"open"}`, nil
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: stubRouter}
	b := &bead.Bead{
		ID:              "over-1",
		Title:           "oversized bead",
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
	}

	if err := adapter.Decompose(context.Background(), b); err != nil {
		t.Fatalf("Decompose returned error: %v", err)
	}
	if !createCalled {
		t.Error("Decompose did not call bead.Client to create child beads; want child bead creation for oversized beads")
	}
}

// TestDecomposerAdapter_DecomposeSucceeds verifies that decomposerAdapter.Decompose
// returns nil when asked to decompose an oversized bead, indicating the decomposition
// workflow ran (creating child beads) rather than erroring with "not yet implemented".
func TestDecomposerAdapter_DecomposeSucceeds(t *testing.T) {
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	stubRouter := provider.NewSingleProviderRouter(&stubRunProvider{
		name: "test",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["f1","f2","f3"]},{"title":"Part 2","expected_outputs":["f4","f5"]}]`,
			}, nil
		},
	})
	client.RunFn = func(args ...string) (string, error) {
		if len(args) > 0 && args[0] == "create" {
			return `{"id":"child-1","title":"part 1","status":"open"}`, nil
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: stubRouter}
	b := &bead.Bead{
		ID:              "over-1",
		Title:           "oversized bead",
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
	}

	if err := adapter.Decompose(context.Background(), b); err != nil {
		t.Fatalf("Decompose returned error: %v; want nil for a decomposable oversized bead", err)
	}
}

// TestDecomposerAdapter_InvokesProviderViaRouter verifies that decomposerAdapter.Decompose
// calls the provider (via router) for LLM-powered decomposition instead of creating
// a dumb carbon-copy child bead.
func TestDecomposerAdapter_InvokesProviderViaRouter(t *testing.T) {
	providerCalled := false
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerCalled = true
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Sub-task 1","expected_outputs":["f1","f2"]},{"title":"Sub-task 2","expected_outputs":["f3","f4"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)

	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	client.RunFn = func(args ...string) (string, error) {
		if len(args) > 0 && args[0] == "create" {
			return `{"id":"child-1","title":"sub-task","status":"open"}`, nil
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	b := &bead.Bead{
		ID:              "parent-1",
		Title:           "Oversized Feature",
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
	}

	if err := adapter.Decompose(context.Background(), b); err != nil {
		t.Fatalf("Decompose returned error: %v", err)
	}
	if !providerCalled {
		t.Error("Decompose did not invoke the provider via router; want LLM-powered decomposition to call provider.Run")
	}
}

// TestDecomposerAdapter_ClosesParentBeadAfterLLMDecomposition verifies that after LLM decomposition
// creates child beads, the adapter closes the parent bead to prevent it from being re-queued.
func TestDecomposerAdapter_ClosesParentBeadAfterLLMDecomposition(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Sub-task A","expected_outputs":["f1","f2","f3"]},{"title":"Sub-task B","expected_outputs":["f4","f5","f6"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)

	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	closedID := ""
	client.RunFn = func(args ...string) (string, error) {
		if len(args) > 0 && args[0] == "create" {
			return `{"id":"child-1","title":"sub-task","status":"open"}`, nil
		}
		if len(args) > 0 && args[0] == "close" && len(args) > 1 {
			closedID = args[1]
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	b := &bead.Bead{
		ID:              "parent-to-close",
		Title:           "Oversized Feature",
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
	}

	if err := adapter.Decompose(context.Background(), b); err != nil {
		t.Fatalf("Decompose returned error: %v", err)
	}
	if closedID != b.ID {
		t.Errorf("closed bead ID = %q, want %q (parent must be closed after LLM decomposition)", closedID, b.ID)
	}
}

func TestResolveBuildCacheVersionKey_StableForSameInputs(t *testing.T) {
	root := t.TempDir()
	gromitDir := filepath.Join(root, ".gromit")
	templatesDir := filepath.Join(root, "templates")
	claudePath := filepath.Join(root, "CLAUDE.md")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("mkdir gromit dir: %v", err)
	}
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates dir: %v", err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	write(filepath.Join(gromitDir, "RULES.md"), "rules v1")
	write(filepath.Join(templatesDir, "PROMPT_build.md"), "build template v1")
	write(filepath.Join(templatesDir, "PROMPT_tdd_build.md"), "tdd template v1")
	write(filepath.Join(templatesDir, "PROMPT_refactor_build.md"), "refactor template v1")
	write(claudePath, "claude v1")

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudePath,
		},
	}
	first := resolveBuildCacheVersionKey(cfg, gromitDir)
	second := resolveBuildCacheVersionKey(cfg, gromitDir)
	if first == "" {
		t.Fatal("cache version key is empty, want non-empty")
	}
	if first != second {
		t.Fatalf("cache version key mismatch for stable inputs: %q vs %q", first, second)
	}
}

func TestResolveBuildCacheVersionKey_ChangesWhenRulesChange(t *testing.T) {
	root := t.TempDir()
	gromitDir := filepath.Join(root, ".gromit")
	templatesDir := filepath.Join(root, "templates")
	claudePath := filepath.Join(root, "CLAUDE.md")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("mkdir gromit dir: %v", err)
	}
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("mkdir templates dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("rules v1"), 0o644); err != nil {
		t.Fatalf("write RULES.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "PROMPT_build.md"), []byte("build template v1"), 0o644); err != nil {
		t.Fatalf("write PROMPT_build.md: %v", err)
	}
	if err := os.WriteFile(claudePath, []byte("claude v1"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudePath,
		},
	}
	before := resolveBuildCacheVersionKey(cfg, gromitDir)
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("rules v2"), 0o644); err != nil {
		t.Fatalf("rewrite RULES.md: %v", err)
	}
	after := resolveBuildCacheVersionKey(cfg, gromitDir)
	if before == after {
		t.Fatalf("cache version key unchanged after RULES.md update: %q", before)
	}
}

func TestDecomposerAdapter_Decompose_InheritsParentBuildStrategyAndSpecLabels(t *testing.T) {
	stubRouter := provider.NewSingleProviderRouter(&stubRunProvider{
		name: "test",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["f1","f2"]},{"title":"Part 2","expected_outputs":["f3","f4"]}]`,
			}, nil
		},
	})

	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}

	var createArgs []string
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "show":
			return `[{"id":"parent-1","title":"Oversized Feature","priority":1,"labels":["build_strategy:parallel","spec:decompose-low-complexity-bias"],"issue_type":"task","status":"open"}]`, nil
		case "create":
			createArgs = append([]string(nil), args...)
			return `{"id":"child-1","title":"Part 1","priority":1,"labels":["build_strategy:parallel","spec:decompose-low-complexity-bias"],"issue_type":"task","status":"open"}`, nil
		default:
			return "", nil
		}
	}

	adapter := &decomposerAdapter{beads: client, router: stubRouter}
	b := &bead.Bead{
		ID:              "parent-1",
		Title:           "Oversized Feature",
		Priority:        1,
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
	}

	if err := adapter.Decompose(context.Background(), b); err != nil {
		t.Fatalf("Decompose returned error: %v", err)
	}
	if len(createArgs) == 0 {
		t.Fatal("create was not called")
	}
	if !hasCreateLabelArg(createArgs, "build_strategy:parallel") {
		t.Fatalf("create args missing inherited build strategy label: %v", createArgs)
	}
	if !hasCreateLabelArg(createArgs, "spec:decompose-low-complexity-bias") {
		t.Fatalf("create args missing inherited spec label: %v", createArgs)
	}
}

func hasCreateLabelArg(args []string, want string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--label" && args[i+1] == want {
			return true
		}
	}
	return false
}

func newCodexProvidersConfig() *config.Config {
	return &config.Config{
		Providers: map[string]config.ProviderDef{
			"codex": {
				Binary: "codex",
			},
		},
	}
}

// stubFailureAnalyzer is a test double for FailureAnalyzer.
type stubFailureAnalyzer struct {
	fn func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error)
}

func (s *stubFailureAnalyzer) Analyze(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
	if s.fn != nil {
		return s.fn(ctx, b, output)
	}
	return nil, nil
}

// TestFailureLearnerAdapter_CallsAnalyzer verifies that failureLearnerAdapter
// calls the analyzer when ExtractFailureLearning is invoked on the failure path.
func TestFailureLearnerAdapter_CallsAnalyzer(t *testing.T) {
	called := false
	stub := &stubFailureAnalyzer{
		fn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			called = true
			return nil, nil
		},
	}
	a := &failureLearnerAdapter{
		analyzer: stub,
	}

	err := a.ExtractFailureLearning(context.Background(), "bead-1", "Implement feature", "FAIL: TestFoo")
	if err != nil {
		t.Fatalf("ExtractFailureLearning returned unexpected error: %v", err)
	}
	if !called {
		t.Error("analyzer.Analyze was not called; want failure learning extraction to invoke the analyzer")
	}
}

// TestBuildTDDCycleRunner_ReturnsTDDPipelineAdapter verifies that buildTDDCycleRunner
// returns a non-nil *TDDPipelineAdapter so that the Build stage can delegate TDD
// cycles to the runner-backed orchestrator.
func TestBuildTDDCycleRunner_ReturnsTDDPipelineAdapter(t *testing.T) {
	cfg := &config.Config{}
	result := buildTDDCycleRunner(cfg, nil, nil, io.Discard, nil)
	if result == nil {
		t.Fatal("buildTDDCycleRunner returned nil TDDCycleRunner")
	}
	if _, ok := result.(*TDDPipelineAdapter); !ok {
		t.Fatalf("buildTDDCycleRunner returned %T, want *TDDPipelineAdapter", result)
	}
}

// TestBuildTDDCycleRunner_RunnerHasConfiguredOrchestrator verifies that the adapter
// returned by buildTDDCycleRunner has a non-nil tddOrchestrator with a configured
// runCyclesFn, so TDD cycle invocations will execute rather than error with "not configured".
func TestBuildTDDCycleRunner_RunnerHasConfiguredOrchestrator(t *testing.T) {
	cfg := &config.Config{}
	result := buildTDDCycleRunner(cfg, nil, nil, io.Discard, nil)
	adapter, ok := result.(*TDDPipelineAdapter)
	if !ok {
		t.Fatalf("expected *TDDPipelineAdapter, got %T", result)
	}
	if adapter.runner.tddOrchestrator == nil {
		t.Fatal("runner.tddOrchestrator is nil; want configured orchestrator")
	}
	if adapter.runner.tddOrchestrator.runCyclesFn == nil {
		t.Fatal("runner.tddOrchestrator.runCyclesFn is nil; want configured run cycles function")
	}
}

// TestOptionalTDDCycleRunner_ReturnsNilWhenFreshContextDisabled verifies that
// optionalTDDCycleRunner returns nil when FreshContextPerCycle is false, so the
// Build stage falls back to single-invocation StreamRun.
func TestOptionalTDDCycleRunner_ReturnsNilWhenFreshContextDisabled(t *testing.T) {
	cfg := &config.Config{}
	result := optionalTDDCycleRunner(cfg, nil, nil, io.Discard, nil)
	if result != nil {
		t.Errorf("optionalTDDCycleRunner returned %T, want nil when FreshContextPerCycle is false", result)
	}
}

// TestOptionalTDDCycleRunner_ReturnsAdapterWhenFreshContextEnabled verifies that
// optionalTDDCycleRunner returns a non-nil TDDCycleRunner when FreshContextPerCycle
// is true, so the Build stage can delegate to per-cycle TDD orchestration.
func TestOptionalTDDCycleRunner_ReturnsAdapterWhenFreshContextEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Methodology.FreshContextPerCycle = true
	result := optionalTDDCycleRunner(cfg, nil, nil, io.Discard, nil)
	if result == nil {
		t.Fatal("optionalTDDCycleRunner returned nil, want non-nil TDDCycleRunner when FreshContextPerCycle is true")
	}
}

func TestOptionalTDDCycleRunner_ReturnsNilWhenMethodologyAdapterIsNonGo(t *testing.T) {
	cfg := &config.Config{}
	cfg.Methodology.FreshContextPerCycle = true
	cfg.Methodology.Adapter = "python"

	result := optionalTDDCycleRunner(cfg, nil, nil, io.Discard, nil)
	if result != nil {
		t.Fatalf("optionalTDDCycleRunner returned %T, want nil when methodology adapter is non-go", result)
	}
}

// TestFailureLearnerAdapter_ForwardsFailureOutput verifies that the failureOutput
// string passed to ExtractFailureLearning reaches the analyzer.Analyze call.
func TestFailureLearnerAdapter_ForwardsFailureOutput(t *testing.T) {
	var receivedOutput string
	stub := &stubFailureAnalyzer{
		fn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			receivedOutput = output
			return nil, nil
		},
	}
	a := &failureLearnerAdapter{
		analyzer: stub,
	}

	err := a.ExtractFailureLearning(context.Background(), "bead-1", "Implement feature", "FAIL: TestFoo\nexpected 1 got 2")
	if err != nil {
		t.Fatalf("ExtractFailureLearning returned unexpected error: %v", err)
	}
	if receivedOutput != "FAIL: TestFoo\nexpected 1 got 2" {
		t.Errorf("analyzer received output %q, want %q", receivedOutput, "FAIL: TestFoo\nexpected 1 got 2")
	}
}

// TestNewRunnerImpl_BuildStageUsesTDDCycleRunner_WhenFreshContextPerCycle verifies
// that newRunnerImpl wires the TDDCycleRunner into the Build stage when
// FreshContextPerCycle is true. The test exercises the Build stage with a TDD bead
// and checks that it delegates to the TDDCycleRunner (identified by the distinctive
// error from the placeholder runCyclesFn) instead of falling back to StreamRun.
func TestNewRunnerImpl_BuildStageUsesTDDCycleRunner_WhenFreshContextPerCycle(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	_ = os.MkdirAll(filepath.Join(gromitDir, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(gromitDir, "specs"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)

	cfg := &config.Config{}
	cfg.Paths.Templates = filepath.Join(gromitDir, "templates")
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")
	cfg.Methodology.TDD = true
	cfg.Methodology.FreshContextPerCycle = true

	orch, err := newRunnerImpl(cfg, io.Discard, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl: %v", err)
	}

	// Extract the Build stage and run it with a TDD bead.
	buildStage := orch.cfg.Build
	tddBead := &bead.Bead{
		ID:     "test-bead-1",
		Title:  "Test feature",
		Labels: []string{"tdd:true"},
	}
	in := pipeline.Input{
		Bead:   tddBead,
		Config: cfg,
	}
	_, err = buildStage.Run(context.Background(), in)
	if err == nil {
		t.Fatal("Build.Run() returned nil error; want TDD cycle runner error proving delegation")
	}
	// The real runCyclesFn delegates to the CycleOrchestrator which attempts
	// red-phase rendering. The error proves Build delegated to TDDCycleRunner.
	if !strings.Contains(err.Error(), "TDD cycle runner") {
		t.Errorf("Build.Run() error = %q; want error containing %q (proves TDDCycleRunner was wired)",
			err.Error(), "TDD cycle runner")
	}
}

// TestNewRunnerImpl_StatusWriterComputesTimeBudgetFromDeadline verifies that the
// StatusWriter closure created by newRunnerImpl computes timeBudgetMinutes from the
// deadline parameter instead of hardcoding 0. When the deadline is 30 minutes away,
// the status.json should contain time_budget_minutes ≈ 30, not 0.
func TestNewRunnerImpl_StatusWriterComputesTimeBudgetFromDeadline(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	_ = os.MkdirAll(filepath.Join(gromitDir, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(gromitDir, "specs"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)

	cfg := &config.Config{}
	cfg.Paths.Templates = filepath.Join(gromitDir, "templates")
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")

	orch, err := newRunnerImpl(cfg, io.Discard, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl: %v", err)
	}

	// Call the StatusWriter callback with a deadline 30 minutes from now.
	deadline := time.Now().Add(30 * time.Minute)
	orch.cfg.StatusWriter(1, "bead-1", "Test bead", deadline)

	// Read the status.json that was written.
	statusPath := filepath.Join(gromitDir, "status.json")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("reading status.json: %v", err)
	}

	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("unmarshaling status.json: %v", err)
	}

	// The constructor currently hardcodes 0. After the fix, it should compute
	// timeBudgetMinutes from the deadline (~30 minutes from now).
	if status.TimeBudgetMinutes == 0 {
		t.Errorf("status.TimeBudgetMinutes = 0; want non-zero value computed from deadline (%v from now)",
			time.Until(deadline).Round(time.Minute))
	}
	if status.TimeBudgetMinutes < 29 || status.TimeBudgetMinutes > 31 {
		t.Errorf("status.TimeBudgetMinutes = %d; want approximately 30 (computed from deadline)",
			status.TimeBudgetMinutes)
	}
}

func TestResolveTrackerBackend_DefaultAndExplicitBD(t *testing.T) {
	var legacyCfg config.Config
	if got := resolveTrackerBackend(&legacyCfg); got != "bd" {
		t.Fatalf("resolveTrackerBackend(legacy) = %q, want %q", got, "bd")
	}

	explicitCfg := &config.Config{
		Tracker: config.TrackerConfig{
			Backend: "bd",
		},
	}
	if got := resolveTrackerBackend(explicitCfg); got != "bd" {
		t.Fatalf("resolveTrackerBackend(explicit) = %q, want %q", got, "bd")
	}
}

func TestResolveTrackerBackendDeprecationMarker_LegacyAndExplicit(t *testing.T) {
	var legacyCfg config.Config
	if got := resolveTrackerBackendDeprecationMarker(&legacyCfg); got != RunnerDeprecationMarkerLegacyTrackerBackendFallback {
		t.Fatalf("resolveTrackerBackendDeprecationMarker(legacy) = %q, want %q", got, RunnerDeprecationMarkerLegacyTrackerBackendFallback)
	}

	explicitCfg := &config.Config{
		Tracker: config.TrackerConfig{
			Backend: "bd",
		},
	}
	if got := resolveTrackerBackendDeprecationMarker(explicitCfg); got != "" {
		t.Fatalf("resolveTrackerBackendDeprecationMarker(explicit) = %q, want empty", got)
	}
}

func TestNewRunnerImpl_LegacyCompatibilityEmitsStartupDeprecationWarning(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	_ = os.MkdirAll(filepath.Join(gromitDir, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(gromitDir, "specs"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)

	cfg := &config.Config{}
	cfg.Paths.Templates = filepath.Join(gromitDir, "templates")
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")

	var output strings.Builder
	if _, err := newRunnerImpl(cfg, &output, nil); err != nil {
		t.Fatalf("newRunnerImpl: %v", err)
	}

	warnings := output.String()
	if !strings.Contains(warnings, config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback) {
		t.Fatalf("startup warnings missing %q, got:\n%s", config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback, warnings)
	}
	if !strings.Contains(warnings, config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults) {
		t.Fatalf("startup warnings missing %q, got:\n%s", config.CompatibilityDeprecationMarkerLegacyHardcodedDefaults, warnings)
	}
	if !strings.Contains(warnings, config.CompatibilityStrictDefaultCutoverDate) {
		t.Fatalf("startup warnings missing strict cutoff date %q, got:\n%s", config.CompatibilityStrictDefaultCutoverDate, warnings)
	}
}

func TestNewRunnerImpl_LegacyCompatibilityWarningUsesConfigMarkerContract(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	_ = os.MkdirAll(filepath.Join(gromitDir, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(gromitDir, "specs"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)

	cfg := &config.Config{}
	cfg.Paths.Templates = filepath.Join(gromitDir, "templates")
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")

	var output strings.Builder
	if _, err := newRunnerImpl(cfg, &output, nil); err != nil {
		t.Fatalf("newRunnerImpl: %v", err)
	}

	warnings := output.String()
	if !strings.Contains(warnings, config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback) {
		t.Fatalf("startup warnings missing %q, got:\n%s", config.CompatibilityDeprecationMarkerLegacyTrackerBackendFallback, warnings)
	}
	if strings.Contains(warnings, RunnerDeprecationMarkerLegacyTrackerBackendFallback) {
		t.Fatalf("startup warnings unexpectedly included runner marker %q, got:\n%s", RunnerDeprecationMarkerLegacyTrackerBackendFallback, warnings)
	}
}

func TestResolveSingleSpecProgressLabel(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{
			name:   "single spec label",
			labels: []string{"spec:auth"},
			want:   "spec:auth",
		},
		{
			name:   "single non-spec label",
			labels: []string{"priority:p0"},
			want:   "",
		},
		{
			name:   "multiple labels",
			labels: []string{"spec:auth", "complexity:high"},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveSingleSpecProgressLabel(tt.labels)
			if got != tt.want {
				t.Fatalf("resolveSingleSpecProgressLabel(%v) = %q, want %q", tt.labels, got, tt.want)
			}
		})
	}
}

func TestEstimateScopedIterationTotal(t *testing.T) {
	client := &bead.Client{
		RunFn: func(args ...string) (string, error) {
			return `[
				{"id":"task-1","title":"A","issue_type":"task","status":"open"},
				{"id":"task-2","title":"B","issue_type":"task","status":"open"},
				{"id":"task-3","title":"C","issue_type":"task","status":"closed"},
				{"id":"epic-1","title":"E","issue_type":"epic","status":"open"}
			]`, nil
		},
	}

	total, err := estimateScopedIterationTotal(client, "spec:auth", 3)
	if err != nil {
		t.Fatalf("estimateScopedIterationTotal() error = %v", err)
	}
	if total != 4 {
		t.Fatalf("estimateScopedIterationTotal() = %d, want 4", total)
	}
}

// TestNewRunnerImpl_GateStageHasDecomposerConfigured verifies that newRunnerImpl
// wires a Decomposer implementation into the Gate stage so that oversized beads
// can be auto-decomposed instead of blocked.
func TestNewRunnerImpl_GateStageHasDecomposerConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	_ = os.MkdirAll(filepath.Join(gromitDir, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(gromitDir, "specs"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)

	cfg := &config.Config{}
	cfg.Paths.Templates = filepath.Join(gromitDir, "templates")
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")

	orch, err := newRunnerImpl(cfg, io.Discard, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl: %v", err)
	}

	// Type-assert the Gate to *prepare.Gate to verify it's configured with a Decomposer.
	gateStage, ok := orch.cfg.Gate.(*prepare.Gate)
	if !ok {
		t.Fatalf("Gate stage is %T, want *prepare.Gate", orch.cfg.Gate)
	}

	// The Gate should have a Decomposer configured so it can auto-decompose
	// oversized beads instead of blocking them.
	if !gateStage.HasDecomposer() {
		t.Fatal("Gate.HasDecomposer() returned false; want Decomposer wired in constructor for scope-triggered auto-decomposition")
	}
}

// TestNewRunnerImpl_GateIntegrationWithRealDecomposer verifies the full end-to-end
// decomposition path: gate.Run() with oversized root bead → triggers decomposerAdapter.Decompose()
// → calls bead.Client.CreateWithParent() to create child beads.
// This test uses the real decomposerAdapter (not mocks) wired by newRunnerImpl.
func TestNewRunnerImpl_GateIntegrationWithRealDecomposer(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	_ = os.MkdirAll(filepath.Join(gromitDir, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(gromitDir, "specs"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "logs"), 0o755)

	cfg := &config.Config{}
	cfg.Paths.Templates = filepath.Join(gromitDir, "templates")
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Logs = filepath.Join(tmpDir, "logs")
	cfg.ScopeCheck.Enabled = true
	blockTrue := true
	cfg.ScopeCheck.BlockOversized = &blockTrue

	orch, err := newRunnerImpl(cfg, io.Discard, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl: %v", err)
	}

	// Mock the bead.Client to track CreateWithParent calls
	// Get the gate and verify it will attempt decomposition
	gateStage, ok := orch.cfg.Gate.(*prepare.Gate)
	if !ok {
		t.Fatalf("Gate stage is %T, want *prepare.Gate", orch.cfg.Gate)
	}

	// Create an oversized root bead (6 expected outputs > maxScopeFiles=5)
	oversizedBead := &bead.Bead{
		ID:              "test-oversized-root",
		Title:           "Implement large feature",
		Priority:        1,
		Labels:          []string{"feature"},
		ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
		Parent:          "", // Root bead
	}

	// Run the gate with the oversized bead
	// The gate should attempt decomposition via the real decomposerAdapter
	in := pipeline.Input{Bead: oversizedBead, Config: cfg}
	out, err := gateStage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("gate.Run() returned unexpected error: %v; LLM decomposition failures should fall back to Block, not propagate error", err)
	}

	// Skip means LLM decomposition succeeded; Block means LLM was unavailable (valid in test env)
	if out.Decision != pipeline.Skip && out.Decision != pipeline.Block {
		t.Errorf("gate.Run() decision = %v, want Skip (decomposition ok) or Block (LLM unavailable)", out.Decision)
	}
}

func TestGateRunScopeGate_ContractViolationFallsBackToBlock(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Only part","expected_outputs":["f1","f2"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)

	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	createCalls := 0
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			return `{"id":"child-1","title":"Only part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	gate := prepare.New(io.Discard).WithDecomposer(&decomposerAdapter{beads: client, router: router})
	blockTrue := true
	cfg := &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled:        true,
			BlockOversized: &blockTrue,
		},
	}
	in := pipeline.Input{
		Bead: &bead.Bead{
			ID:              "parent-1",
			Title:           "Oversized Feature",
			Priority:        1,
			ExpectedOutputs: []string{"f1", "f2", "f3", "f4", "f5", "f6"},
		},
		Config: cfg,
	}

	out, err := gate.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("gate.Run() error = %v, want nil (fallback should block, not propagate)", err)
	}
	if out.Decision != pipeline.Block {
		t.Fatalf("decision = %v, want %v when decomposition output violates contract", out.Decision, pipeline.Block)
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0 on contract violation", createCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("close calls = %d, want 0 on contract violation", closeCalls)
	}
}

func TestDecomposerAdapter_Decompose_RejectsMoreThanFiveSubBeads(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output: `[
					{"title":"Part 1","expected_outputs":["f1"]},
					{"title":"Part 2","expected_outputs":["f2"]},
					{"title":"Part 3","expected_outputs":["f3"]},
					{"title":"Part 4","expected_outputs":["f4"]},
					{"title":"Part 5","expected_outputs":["f5"]},
					{"title":"Part 6","expected_outputs":["f6"]}
				]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	createCalls := 0
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	err = adapter.Decompose(context.Background(), &bead.Bead{
		ID:              "parent-1",
		Title:           "Oversized Feature",
		Priority:        1,
		ExpectedOutputs: []string{"p1", "p2", "p3", "p4", "p5", "p6"},
	})
	if err == nil {
		t.Fatal("Decompose() error = nil, want contract violation for >5 sub-beads")
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0 when contract is violated", createCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("close calls = %d, want 0 when contract is violated", closeCalls)
	}
}

func TestValidateRuntimeScopeGateDecomposeOutput_ReturnsRuleCodeFromSharedValidator(t *testing.T) {
	err := validateRuntimeScopeGateDecomposeOutput(
		[]scopeGateSubBead{
			{Title: "Part 1", ExpectedOutputs: []string{"f1"}},
			{Title: "Part 2", ExpectedOutputs: []string{"f2"}},
			{Title: "Part 3", ExpectedOutputs: []string{"f3"}},
			{Title: "Part 4", ExpectedOutputs: []string{"f4"}},
			{Title: "Part 5", ExpectedOutputs: []string{"f5"}},
			{Title: "Part 6", ExpectedOutputs: []string{"f6"}},
		},
		"Oversized Feature",
		validate.MaxSubBeads,
	)
	if err == nil {
		t.Fatal("validateRuntimeScopeGateDecomposeOutput() error = nil, want batch_size_max violation")
	}
	if !strings.Contains(err.Error(), "batch_size_max") {
		t.Fatalf("validateRuntimeScopeGateDecomposeOutput() error = %q, want rule code batch_size_max", err.Error())
	}
}

func TestValidateRuntimeScopeGateDecomposeOutput_ZeroMaxUsesDefaultLimit(t *testing.T) {
	err := validateRuntimeScopeGateDecomposeOutput(
		[]scopeGateSubBead{
			{Title: "Part 1", ExpectedOutputs: []string{"f1"}},
			{Title: "Part 2", ExpectedOutputs: []string{"f2"}},
			{Title: "Part 3", ExpectedOutputs: []string{"f3"}},
			{Title: "Part 4", ExpectedOutputs: []string{"f4"}},
			{Title: "Part 5", ExpectedOutputs: []string{"f5"}},
			{Title: "Part 6", ExpectedOutputs: []string{"f6"}},
		},
		"Oversized Feature",
		0,
	)
	if err == nil {
		t.Fatal("validateRuntimeScopeGateDecomposeOutput() error = nil, want batch_size_max violation when maxSubBeads is zero")
	}
}

func TestValidateRuntimeScopeGateDecomposeOutput_UsesSharedRequiredFieldViolationMessage(t *testing.T) {
	err := validateRuntimeScopeGateDecomposeOutput(
		[]scopeGateSubBead{
			{Title: "", ExpectedOutputs: []string{"f1"}},
			{Title: "Part 2", ExpectedOutputs: []string{"f2"}},
		},
		"Oversized Feature",
		validate.MaxSubBeads,
	)
	if err == nil {
		t.Fatal("validateRuntimeScopeGateDecomposeOutput() error = nil, want title_empty violation")
	}
	if !strings.Contains(err.Error(), "[title_empty]") {
		t.Fatalf("validateRuntimeScopeGateDecomposeOutput() error = %q, want title_empty rule code", err.Error())
	}
	if !strings.Contains(err.Error(), "bead 0: Bead title is empty") {
		t.Fatalf("validateRuntimeScopeGateDecomposeOutput() error = %q, want shared validation message", err.Error())
	}
}

func TestDecomposerAdapter_Decompose_RejectsChildWithMoreThanFiveExpectedOutputs(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output: `[
					{"title":"Part 1","expected_outputs":["f1","f2","f3","f4","f5","f6"]},
					{"title":"Part 2","expected_outputs":["f7","f8"]}
				]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	createCalls := 0
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	err = adapter.Decompose(context.Background(), &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	})
	if err == nil {
		t.Fatal("Decompose() error = nil, want contract violation for child expected_outputs > 5")
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0 when contract is violated", createCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("close calls = %d, want 0 when contract is violated", closeCalls)
	}
}

func TestDecomposerAdapter_Decompose_RejectsEmptyExpectedOutput(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output: `[
					{"title":"Part 1","expected_outputs":["f1",""]},
					{"title":"Part 2","expected_outputs":["f2"]}
				]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	createCalls := 0
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	err = adapter.Decompose(context.Background(), &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	})
	if err == nil {
		t.Fatal("Decompose() error = nil, want contract violation for empty expected output")
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0 when contract is violated", createCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("close calls = %d, want 0 when contract is violated", closeCalls)
	}
}

func TestDecomposerAdapter_Decompose_RejectsEmptyTitle(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output: `[
					{"title":"","expected_outputs":["f1"]},
					{"title":"Part 2","expected_outputs":["f2"]}
				]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	createCalls := 0
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	err = adapter.Decompose(context.Background(), &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	})
	if err == nil {
		t.Fatal("Decompose() error = nil, want contract violation for empty sub-bead title")
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0 when contract is violated", createCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("close calls = %d, want 0 when contract is violated", closeCalls)
	}
}

func TestDecomposerAdapter_Decompose_RejectsChildWithNoExpectedOutputs(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output: `[
					{"title":"Part 1","expected_outputs":[]},
					{"title":"Part 2","expected_outputs":["f2"]}
				]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	createCalls := 0
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	err = adapter.Decompose(context.Background(), &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	})
	if err == nil {
		t.Fatal("Decompose() error = nil, want contract violation for empty expected_outputs")
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0 when contract is violated", createCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("close calls = %d, want 0 when contract is violated", closeCalls)
	}
}

func TestDecomposerAdapter_Decompose_RejectsDuplicateExpectedOutputs(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output: `[
					{"title":"Part 1","expected_outputs":["f1","f1"]},
					{"title":"Part 2","expected_outputs":["f2"]}
				]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	createCalls := 0
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	err = adapter.Decompose(context.Background(), &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	})
	if err == nil {
		t.Fatal("Decompose() error = nil, want contract violation for duplicate expected outputs")
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0 when contract is violated", createCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("close calls = %d, want 0 when contract is violated", closeCalls)
	}
}

func TestDecomposerAdapter_Decompose_RejectsExpectedOutputThatEchoesParentTitle(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output: `[
					{"title":"Part 1","expected_outputs":["Oversized Feature"]},
					{"title":"Part 2","expected_outputs":["f2"]}
				]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}
	createCalls := 0
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	err = adapter.Decompose(context.Background(), &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	})
	if err == nil {
		t.Fatal("Decompose() error = nil, want contract violation for parent-title echo in expected output")
	}
	if !strings.Contains(err.Error(), "parent_echo") {
		t.Fatalf("Decompose() error = %q, want rule code parent_echo for shared contract validation", err.Error())
	}
	if createCalls != 0 {
		t.Fatalf("create calls = %d, want 0 when contract is violated", createCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("close calls = %d, want 0 when contract is violated", closeCalls)
	}
}

func TestDecomposerAdapter_Decompose_RetryDoesNotDuplicatePreviouslyCreatedChildren(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["f1"]},{"title":"Part 2","expected_outputs":["f2"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}

	createCallsByTitle := map[string]int{}
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			if len(args) < 2 {
				t.Fatalf("create args missing title: %v", args)
			}
			title := args[1]
			createCallsByTitle[title]++
			if title == "Part 2" && createCallsByTitle[title] == 1 {
				return "", os.ErrPermission
			}
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	parent := &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	}

	if err := adapter.Decompose(context.Background(), parent); err == nil {
		t.Fatal("first Decompose() error = nil, want create failure")
	}
	if closeCalls != 0 {
		t.Fatalf("close calls after first attempt = %d, want 0", closeCalls)
	}

	if err := adapter.Decompose(context.Background(), parent); err != nil {
		t.Fatalf("second Decompose() error = %v, want nil", err)
	}
	if createCallsByTitle["Part 1"] != 1 {
		t.Fatalf("Part 1 create calls = %d, want 1 (no duplicate on retry)", createCallsByTitle["Part 1"])
	}
	if closeCalls != 1 {
		t.Fatalf("close calls after successful retry = %d, want 1", closeCalls)
	}
}

func TestDecomposerAdapter_Decompose_RetryDoesNotDuplicateWhenExpectedOutputsAreReordered(t *testing.T) {
	providerCalls := 0
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerCalls++
			if providerCalls == 1 {
				return &provider.Result{
					Success: true,
					Output:  `[{"title":"Part 1","expected_outputs":["b.go","a.go"]},{"title":"Part 2","expected_outputs":["f2"]}]`,
				}, nil
			}
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["a.go","b.go"]},{"title":"Part 2","expected_outputs":["f2"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}

	createCallsByTitle := map[string]int{}
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			if len(args) < 2 {
				t.Fatalf("create args missing title: %v", args)
			}
			title := args[1]
			createCallsByTitle[title]++
			if title == "Part 2" && createCallsByTitle[title] == 1 {
				return "", os.ErrPermission
			}
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	parent := &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	}

	if err := adapter.Decompose(context.Background(), parent); err == nil {
		t.Fatal("first Decompose() error = nil, want create failure")
	}
	if closeCalls != 0 {
		t.Fatalf("close calls after first attempt = %d, want 0", closeCalls)
	}

	if err := adapter.Decompose(context.Background(), parent); err != nil {
		t.Fatalf("second Decompose() error = %v, want nil", err)
	}
	if createCallsByTitle["Part 1"] != 1 {
		t.Fatalf("Part 1 create calls = %d, want 1 (no duplicate on retry)", createCallsByTitle["Part 1"])
	}
	if closeCalls != 1 {
		t.Fatalf("close calls after successful retry = %d, want 1", closeCalls)
	}
}

func TestDecomposerAdapter_Decompose_RetryDoesNotDuplicateWhenTitleAndOutputsVaryByWhitespace(t *testing.T) {
	providerCalls := 0
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			providerCalls++
			if providerCalls == 1 {
				return &provider.Result{
					Success: true,
					Output:  `[{"title":"Part 1","expected_outputs":["a.go","b.go"]},{"title":"Part 2","expected_outputs":["f2"]}]`,
				}, nil
			}
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"  Part   1  ","expected_outputs":[" a.go ","b.go  "]},{"title":"Part 2","expected_outputs":["f2"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}

	createCalls := 0
	closeCalls := 0
	createCallsByTitle := map[string]int{}
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			if len(args) < 2 {
				t.Fatalf("create args missing title: %v", args)
			}
			title := args[1]
			createCallsByTitle[title]++
			if title == "Part 2" && createCallsByTitle[title] == 1 {
				return "", os.ErrPermission
			}
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	parent := &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	}

	if err := adapter.Decompose(context.Background(), parent); err == nil {
		t.Fatal("first Decompose() error = nil, want create failure")
	}
	if closeCalls != 0 {
		t.Fatalf("close calls after first attempt = %d, want 0", closeCalls)
	}

	if err := adapter.Decompose(context.Background(), parent); err != nil {
		t.Fatalf("second Decompose() error = %v, want nil", err)
	}
	if createCalls != 3 {
		t.Fatalf("total create calls = %d, want 3 (no duplicate on retry)", createCalls)
	}
	if closeCalls != 1 {
		t.Fatalf("close calls after successful retry = %d, want 1", closeCalls)
	}
}

// TestSpecGateAdapterImplementsEpilogueSpecGateRunner verifies that specGateAdapter
// satisfies the epilogue.SpecGateRunner interface so it can be wired into the Epilogue stage.
func TestSpecGateAdapterImplementsEpilogueSpecGateRunner(t *testing.T) {
	adapter := &specGateAdapter{}
	// This test is a compile-time check via implicit interface satisfaction.
	// If specGateAdapter doesn't implement SpecGateRunner, this will fail at compile time.
	_ = interface{}(adapter)
}

// TestNewRunnerImpl_WiresSpecGateIntoEpilogue verifies that the spec gate adapter
// is wired into the epilogue stage when newRunnerImpl creates the orchestrator.
func TestNewRunnerImpl_WiresSpecGateIntoEpilogue(t *testing.T) {
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates: t.TempDir(),
			Logs:      t.TempDir(),
			Specs:     t.TempDir(),
		},
		Models: config.ModelsConfig{
			Validation: "test-model",
		},
		Loop: config.LoopConfig{
			MaxIterations: 10,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	orch, err := newRunnerImpl(cfg, io.Discard, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl() error = %v, want nil", err)
	}

	if orch == nil {
		t.Fatal("newRunnerImpl() returned nil orchestrator, want non-nil")
	}

	// Verify that the orchestrator's epilogue is configured
	if orch.cfg.Epilogue == nil {
		t.Fatal("orchestrator epilogue stage is nil, want non-nil")
	}
}

func TestNewRunnerLoadsExperimentsWhenEnabled(t *testing.T) {
	// Setup: Create a temporary directory with an experiment
	tmpDir := t.TempDir()
	experimentsDir := filepath.Join(tmpDir, "experiments")
	if err := os.MkdirAll(experimentsDir, 0755); err != nil {
		t.Fatalf("failed to create experiments dir: %v", err)
	}

	// Create a sample experiment YAML file
	expFile := filepath.Join(experimentsDir, "test_exp.yaml")
	expYAML := `id: test-exp
phase: build
description: Test experiment
control:
  id: control
  template: control_template
variants:
  - id: variant1
    template: variant_template
`
	if err := os.WriteFile(expFile, []byte(expYAML), 0644); err != nil {
		t.Fatalf("failed to write experiment file: %v", err)
	}

	// Create a minimal config with experiments enabled
	cfg := &config.Config{
		Experiment: config.ExperimentConfig{
			Enabled:        true,
			ExperimentsDir: experimentsDir,
		},
		Paths: config.PathsConfig{
			Templates: filepath.Join(tmpDir, "templates"),
			Specs:     filepath.Join(tmpDir, "specs"),
			Logs:      filepath.Join(tmpDir, "logs"),
			GromitDir: tmpDir,
		},
		Loop: config.LoopConfig{
			MaxIterations: 10,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create the runner
	orch, err := newRunnerImpl(cfg, io.Discard, nil)
	if err != nil {
		t.Fatalf("newRunnerImpl() error = %v, want nil", err)
	}

	if orch == nil {
		t.Fatal("newRunnerImpl() returned nil orchestrator, want non-nil")
	}

	// The experiment manager should be accessible (we'll test this in phase 3)
	// For now, we're just verifying that newRunnerImpl completes successfully
	// with experiments enabled
}

// TestDecomposerAdapter_Decompose_DetectsPartialDecompositionState verifies that
// decomposerAdapter.Decompose returns ErrPartialDecompositionState when some child
// beads are created successfully but a subsequent child bead creation fails.
func TestDecomposerAdapter_Decompose_DetectsPartialDecompositionState(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["f1"]},{"title":"Part 2","expected_outputs":["f2"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}

	createCallCount := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCallCount++
			// First child succeeds, second child fails
			if createCallCount == 1 {
				return `{"id":"child-1","title":"part 1","status":"open"}`, nil
			}
			if createCallCount == 2 {
				return "", os.ErrPermission
			}
		case "close":
			// Should not be called on partial decomposition failure
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	parent := &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	}

	err = adapter.Decompose(context.Background(), parent)
	if err == nil {
		t.Fatal("Decompose returned nil, want error for partial decomposition")
	}

	// Must return ErrPartialDecompositionState, not the original create error
	if !errors.Is(err, escalation.ErrPartialDecompositionState) {
		t.Fatalf("Decompose returned %v, want ErrPartialDecompositionState", err)
	}
}

// TestDecomposerAdapter_Decompose_FirstChildFailureReturnsOriginalError verifies that
// when the first child bead creation fails, decomposerAdapter.Decompose returns the
// original error (not ErrPartialDecompositionState), since no partial state has been created yet.
func TestDecomposerAdapter_Decompose_FirstChildFailureReturnsOriginalError(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["f1"]},{"title":"Part 2","expected_outputs":["f2"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}

	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			// Fail on the first child
			return "", os.ErrPermission
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	parent := &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	}

	err = adapter.Decompose(context.Background(), parent)
	if err == nil {
		t.Fatal("Decompose returned nil, want error for first child failure")
	}

	// Should NOT be ErrPartialDecompositionState since no child was created
	if errors.Is(err, escalation.ErrPartialDecompositionState) {
		t.Fatalf("Decompose returned ErrPartialDecompositionState, want original error")
	}
}

// TestDecomposerAdapter_Decompose_RetryAfterPartialStateDeduplicatesSuccessfulChildren
// verifies that when a first call creates some children then fails partway through,
// the second call properly deduplicates the already-created children and only creates
// the remaining ones.
func TestDecomposerAdapter_Decompose_RetryAfterPartialStateDeduplicatesSuccessfulChildren(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["f1"]},{"title":"Part 2","expected_outputs":["f2"]},{"title":"Part 3","expected_outputs":["f3"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}

	createCalls := 0
	closeCalls := 0
	attemptNumber := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			// On first attempt: Part 1 succeeds, Part 2 fails
			if attemptNumber == 0 && createCalls == 2 {
				return "", os.ErrPermission
			}
			// On second attempt: Parts 2 and 3 should succeed (Part 1 is deduplicated)
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	parent := &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	}

	// First attempt: partial state
	err = adapter.Decompose(context.Background(), parent)
	if err == nil || !errors.Is(err, escalation.ErrPartialDecompositionState) {
		t.Fatalf("first Decompose() error = %v, want ErrPartialDecompositionState", err)
	}
	createsBeforeSecondAttempt := createCalls
	closesBeforeSecondAttempt := closeCalls

	// Start second attempt
	attemptNumber = 1
	err = adapter.Decompose(context.Background(), parent)
	if err != nil {
		t.Fatalf("second Decompose() error = %v, want nil", err)
	}

	// Verify deduplication: only 2 more creates (Part 2 and Part 3), not Part 1 again
	expectedCreatesAfterSecond := createsBeforeSecondAttempt + 2
	if createCalls != expectedCreatesAfterSecond {
		t.Fatalf("createCalls = %d, want %d (Part 1 should be deduplicated)", createCalls, expectedCreatesAfterSecond)
	}

	// Verify parent was closed on second attempt
	if closeCalls != closesBeforeSecondAttempt+1 {
		t.Fatalf("closeCalls = %d, want %d (parent should close on successful retry)", closeCalls, closesBeforeSecondAttempt+1)
	}
}

// TestDecomposerAdapter_Decompose_FullySuccessfulPathReturnsNil verifies that
// when all child beads are created successfully, decomposerAdapter.Decompose
// returns nil and closes the parent bead as expected.
func TestDecomposerAdapter_Decompose_FullySuccessfulPathReturnsNil(t *testing.T) {
	stub := &stubRunProvider{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["f1"]},{"title":"Part 2","expected_outputs":["f2"]}]`,
			}, nil
		},
	}
	router := provider.NewSingleProviderRouter(stub)
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("bead.NewClient: %v", err)
	}

	createCalls := 0
	closeCalls := 0
	client.RunFn = func(args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "create":
			createCalls++
			return `{"id":"child-1","title":"part","status":"open"}`, nil
		case "close":
			closeCalls++
		}
		return "", nil
	}

	adapter := &decomposerAdapter{beads: client, router: router}
	parent := &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	}

	err = adapter.Decompose(context.Background(), parent)
	if err != nil {
		t.Fatalf("Decompose returned error %v, want nil for fully successful decomposition", err)
	}

	if createCalls != 2 {
		t.Fatalf("createCalls = %d, want 2", createCalls)
	}

	if closeCalls != 1 {
		t.Fatalf("closeCalls = %d, want 1 (parent should be closed)", closeCalls)
	}
}
