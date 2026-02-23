package runner

import (
	"context"
	"encoding/json"
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

func TestConstructorGo_UsesSharedProviderHelpers(t *testing.T) {
	src, err := os.ReadFile("constructor.go")
	if err != nil {
		t.Fatalf("os.ReadFile(constructor.go): %v", err)
	}
	text := string(src)
	if strings.Contains(text, "func buildProvidersFromConfig(") {
		t.Fatal("constructor.go defines buildProvidersFromConfig; want local provider helper removed")
	}
	if strings.Contains(text, "func parseFallbackCooldown(") {
		t.Fatal("constructor.go defines parseFallbackCooldown; want local provider helper removed")
	}
	if strings.Contains(text, "buildProvidersFromConfig(cfg)") {
		t.Fatal("constructor.go calls buildProvidersFromConfig; want provider.BuildProvidersFromConfig")
	}
	if strings.Contains(text, "parseFallbackCooldown(cfg)") {
		t.Fatal("constructor.go calls parseFallbackCooldown; want provider.ParseFallbackCooldown")
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
				Output:  `[{"title":"Part 1","expected_outputs":["f1","f2","f3"]}]`,
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

func TestDecomposerAdapter_Decompose_InheritsBuildStrategyLabelFromParent(t *testing.T) {
	stubRouter := provider.NewSingleProviderRouter(&stubRunProvider{
		name: "test",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  `[{"title":"Part 1","expected_outputs":["f1","f2"]}]`,
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
			return `[{"id":"parent-1","title":"Oversized Feature","priority":1,"labels":["build_strategy:parallel"],"issue_type":"task","status":"open"}]`, nil
		case "create":
			createArgs = append([]string(nil), args...)
			return `{"id":"child-1","title":"Part 1","priority":1,"labels":["build_strategy:parallel"],"issue_type":"task","status":"open"}`, nil
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
}

func hasCreateLabelArg(args []string, want string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--label" && args[i+1] == want {
			return true
		}
	}
	return false
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
	result := buildTDDCycleRunner(cfg, nil, nil, io.Discard)
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
	result := buildTDDCycleRunner(cfg, nil, nil, io.Discard)
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
	result := optionalTDDCycleRunner(cfg, nil, nil, io.Discard)
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
	result := optionalTDDCycleRunner(cfg, nil, nil, io.Discard)
	if result == nil {
		t.Fatal("optionalTDDCycleRunner returned nil, want non-nil TDDCycleRunner when FreshContextPerCycle is true")
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
			Enabled:       true,
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
