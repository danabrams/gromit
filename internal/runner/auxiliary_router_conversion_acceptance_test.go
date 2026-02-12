//go:build acceptance

package runner

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/provider"
)

// TestAcceptance_RunPrecheckUsesRouterSelect verifies that runPrecheck
// calls router.Select() with phase="precheck" and tier="low".
// Expected failure: runPrecheck does not yet call router.Select() - it still uses r.claude.Run()
func TestAcceptance_RunPrecheckUsesRouterSelect(t *testing.T) {
	cfg := makeTestRunnerConfig()
	enabled := true
	cfg.Precheck.Enabled = &enabled
	cfg.Precheck.Model = "haiku"
	cfg.Precheck.TimeoutSeconds = 30

	// Track router.Select() calls
	var capturedPhase, capturedTier string

	mockProvider := &mockProviderWithRouterTracking{
		onSelect: func(phase, tier string) {
			capturedPhase = phase
			capturedTier = tier
		},
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "haiku", Output: "PRECHECK_PASSED"}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	testBead := &bead.Bead{
		ID:          "test-001",
		Title:       "Test bead",
		Description: "acceptance criteria 1",
		Priority:    1,
		Labels:      []string{},
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: &mockRenderer{},
		beads:    &mockBeadClient{},
		output:   io.Discard,
	}

	// Call runPrecheck
	passed, _ := r.runPrecheck(context.Background(), testBead)

	if !passed {
		t.Errorf("runPrecheck() returned false, expected true (PRECHECK_PASSED in output)")
	}

	// Verify router.Select() was called with correct phase
	if capturedPhase != "precheck" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "precheck")
	}

	// Verify tier is "low" (precheck always uses low tier)
	if capturedTier != provider.TierLow {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, provider.TierLow)
	}
}

// TestAcceptance_CheckScopeUsesRouterSelect verifies that checkScope
// calls router.Select() with phase="scope_check" and tier="low".
// Expected failure: checkScope does not yet call router.Select() - it still uses r.claude.Run()
func TestAcceptance_CheckScopeUsesRouterSelect(t *testing.T) {
	cfg := makeTestRunnerConfig()
	cfg.ScopeCheck.Enabled = true
	cfg.ScopeCheck.Model = "haiku"

	// Track router.Select() calls
	var capturedPhase, capturedTier string

	// Return valid JSON scope estimate
	scopeEstimateJSON := `{
		"complexity": "medium",
		"estimated_iterations": 1,
		"can_complete_in_single_iteration": true,
		"blockers": []
	}`

	mockProvider := &mockProviderWithRouterTracking{
		onSelect: func(phase, tier string) {
			capturedPhase = phase
			capturedTier = tier
		},
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "haiku", Output: scopeEstimateJSON}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRenderer := &mockRenderer{}

	testBead := &bead.Bead{
		ID:          "test-002",
		Title:       "Test bead",
		Description: "large scope",
		Priority:    1,
		Labels:      []string{},
	}

	mockBeads := &mockBeadClient{}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		beads:    mockBeads,
		output:   io.Discard,
	}

	// Call checkScope
	estimate := r.checkScope(context.Background(), testBead)

	if estimate == nil {
		t.Fatal("checkScope() returned nil, expected scope estimate")
	}

	// Verify router.Select() was called with correct phase
	if capturedPhase != "scope_check" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "scope_check")
	}

	// Verify tier is "low" (scope check always uses low tier)
	if capturedTier != provider.TierLow {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, provider.TierLow)
	}
}

// TestAcceptance_DecomposeTaskUsesRouterSelect verifies that DecomposeTask
// calls router.Select() with phase="decompose" and tier="high".
// Expected failure: DecomposeTask does not yet call router.Select() - it still uses r.claude.Run()
func TestAcceptance_DecomposeTaskUsesRouterSelect(t *testing.T) {
	cfg := makeTestRunnerConfig()

	// Track router.Select() calls
	var capturedPhase, capturedTier string

	// Return valid JSON decomposition
	decomposeJSON := `[
		{
			"title": "Sub-task 1",
			"description": "First part",
			"acceptance_criteria": ["criterion 1"]
		},
		{
			"title": "Sub-task 2",
			"description": "Second part",
			"acceptance_criteria": ["criterion 2"]
		}
	]`

	mockProvider := &mockProviderWithRouterTracking{
		onSelect: func(phase, tier string) {
			capturedPhase = phase
			capturedTier = tier
		},
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "opus", Output: decomposeJSON}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRenderer := &mockRenderer{}

	testBead := &bead.Bead{
		ID:          "test-003",
		Title:       "Large task",
		Description: "needs decomposition",
		Priority:    0,
		Labels:      []string{},
	}

	mockBeads := &mockBeadClient{}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		beads:    mockBeads,
		output:   io.Discard,
	}

	// Call DecomposeTask
	subTasks, err := r.DecomposeTask(context.Background(), testBead)
	if err != nil {
		t.Fatalf("DecomposeTask() error = %v", err)
	}

	if len(subTasks) != 2 {
		t.Errorf("DecomposeTask() returned %d sub-tasks, expected 2", len(subTasks))
	}

	// Verify router.Select() was called with correct phase
	if capturedPhase != "decompose" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "decompose")
	}

	// Verify tier is "high" (decompose always uses high tier/opus)
	if capturedTier != provider.TierHigh {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, provider.TierHigh)
	}
}

// TestAcceptance_ExtractSuccessLearningUsesRouterSelect verifies that
// extractSuccessLearning calls router.Select() with phase="build" and tier="low".
// Expected failure: extractSuccessLearning does not yet call router.Select() - it still uses r.claude.Run()
func TestAcceptance_ExtractSuccessLearningUsesRouterSelect(t *testing.T) {
	cfg := makeTestRunnerConfig()
	learnFromSuccess := true
	cfg.Loop.LearnFromSuccess = &learnFromSuccess

	// Track router.Select() calls
	var capturedPhase, capturedTier string

	// Return valid JSON learning extraction
	learningJSON := `{
		"learning": "Test pattern observed",
		"category": "patterns"
	}`

	mockProvider := &mockProviderWithRouterTracking{
		onSelect: func(phase, tier string) {
			capturedPhase = phase
			capturedTier = tier
		},
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{Success: true, Model: "haiku", Output: learningJSON}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	mockRenderer := &mockRenderer{}

	testBead := &bead.Bead{
		ID:          "test-004",
		Title:       "Completed task",
		Description: "successfully implemented",
		Priority:    1,
		Labels:      []string{},
	}

	bc := &beadContext{
		bead:   testBead,
		result: &IterationResult{},
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	// Call extractSuccessLearning
	r.extractSuccessLearning(context.Background(), bc)

	// Verify router.Select() was called with correct phase
	if capturedPhase != "build" {
		t.Errorf("router.Select() phase = %q, want %q", capturedPhase, "build")
	}

	// Verify tier is "low" (success learning extraction uses haiku/low tier)
	if capturedTier != provider.TierLow {
		t.Errorf("router.Select() tier = %q, want %q", capturedTier, provider.TierLow)
	}
}

// TestAcceptance_RunPrecheckCallsProviderRun verifies that runPrecheck
// invokes provider.Run() with the selected tier.
// Expected failure: provider.Run() is not yet called - runPrecheck still uses r.claude.Run()
func TestAcceptance_RunPrecheckCallsProviderRun(t *testing.T) {
	cfg := makeTestRunnerConfig()
	enabled := true
	cfg.Precheck.Enabled = &enabled
	cfg.Precheck.TimeoutSeconds = 30

	runCalled := false
	var runTier string

	mockProvider := &mockProviderWithRouterTracking{
		onSelect: func(phase, tier string) {},
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCalled = true
			runTier = tier
			return &provider.Result{Success: true, Model: "haiku", Output: "PRECHECK_PASSED"}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)
	mockRenderer := &mockRenderer{}
	mockBeads := &mockBeadClient{}

	testBead := &bead.Bead{
		ID:       "test-005",
		Title:    "Test",
		Priority: 1,
		Labels:   []string{},
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		beads:    mockBeads,
		output:   io.Discard,
	}

	r.runPrecheck(context.Background(), testBead)

	if !runCalled {
		t.Error("provider.Run() was not called - runPrecheck should invoke provider.Run()")
	}

	if runTier != provider.TierLow {
		t.Errorf("provider.Run() tier = %q, want %q", runTier, provider.TierLow)
	}
}

// TestAcceptance_CheckScopeCallsProviderRun verifies that checkScope
// invokes provider.Run() with the selected tier.
// Expected failure: provider.Run() is not yet called - checkScope still uses r.claude.Run()
func TestAcceptance_CheckScopeCallsProviderRun(t *testing.T) {
	cfg := makeTestRunnerConfig()
	cfg.ScopeCheck.Enabled = true

	runCalled := false
	var runTier string

	scopeEstimateJSON := `{
		"complexity": "low",
		"estimated_iterations": 1,
		"can_complete_in_single_iteration": true,
		"blockers": []
	}`

	mockProvider := &mockProviderWithRouterTracking{
		onSelect: func(phase, tier string) {},
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCalled = true
			runTier = tier
			return &provider.Result{Success: true, Model: "haiku", Output: scopeEstimateJSON}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)
	mockRenderer := &mockRenderer{}
	mockBeads := &mockBeadClient{}

	testBead := &bead.Bead{
		ID:       "test-006",
		Title:    "Test",
		Priority: 1,
		Labels:   []string{},
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		beads:    mockBeads,
		output:   io.Discard,
	}

	r.checkScope(context.Background(), testBead)

	if !runCalled {
		t.Error("provider.Run() was not called - checkScope should invoke provider.Run()")
	}

	if runTier != provider.TierLow {
		t.Errorf("provider.Run() tier = %q, want %q", runTier, provider.TierLow)
	}
}

// TestAcceptance_DecomposeTaskCallsProviderRun verifies that DecomposeTask
// invokes provider.Run() with the selected tier.
// Expected failure: provider.Run() is not yet called - DecomposeTask still uses r.claude.Run()
func TestAcceptance_DecomposeTaskCallsProviderRun(t *testing.T) {
	cfg := makeTestRunnerConfig()

	runCalled := false
	var runTier string

	decomposeJSON := `[
		{
			"title": "Sub-task",
			"description": "Part 1",
			"acceptance_criteria": ["criterion"]
		}
	]`

	mockProvider := &mockProviderWithRouterTracking{
		onSelect: func(phase, tier string) {},
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCalled = true
			runTier = tier
			return &provider.Result{Success: true, Model: "opus", Output: decomposeJSON}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)
	mockRenderer := &mockRenderer{}
	mockBeads := &mockBeadClient{}

	testBead := &bead.Bead{
		ID:       "test-007",
		Title:    "Large task",
		Priority: 0,
		Labels:   []string{},
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		beads:    mockBeads,
		output:   io.Discard,
	}

	r.DecomposeTask(context.Background(), testBead)

	if !runCalled {
		t.Error("provider.Run() was not called - DecomposeTask should invoke provider.Run()")
	}

	if runTier != provider.TierHigh {
		t.Errorf("provider.Run() tier = %q, want %q", runTier, provider.TierHigh)
	}
}

// TestAcceptance_ExtractSuccessLearningCallsProviderRun verifies that
// extractSuccessLearning invokes provider.Run() with the selected tier.
// Expected failure: provider.Run() is not yet called - extractSuccessLearning still uses r.claude.Run()
func TestAcceptance_ExtractSuccessLearningCallsProviderRun(t *testing.T) {
	cfg := makeTestRunnerConfig()
	learnFromSuccess := true
	cfg.Loop.LearnFromSuccess = &learnFromSuccess

	runCalled := false
	var runTier string

	learningJSON := `{
		"learning": "Pattern observed",
		"category": "patterns"
	}`

	mockProvider := &mockProviderWithRouterTracking{
		onSelect: func(phase, tier string) {},
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCalled = true
			runTier = tier
			return &provider.Result{Success: true, Model: "haiku", Output: learningJSON}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)
	mockRenderer := &mockRenderer{}

	testBead := &bead.Bead{
		ID:       "test-008",
		Title:    "Task",
		Priority: 1,
		Labels:   []string{},
	}

	bc := &beadContext{
		bead:   testBead,
		result: &IterationResult{},
	}

	r := &Runner{
		cfg:      cfg,
		router:   mockRouter,
		renderer: mockRenderer,
		output:   io.Discard,
	}

	r.extractSuccessLearning(context.Background(), bc)

	if !runCalled {
		t.Error("provider.Run() was not called - extractSuccessLearning should invoke provider.Run()")
	}

	if runTier != provider.TierLow {
		t.Errorf("provider.Run() tier = %q, want %q", runTier, provider.TierLow)
	}
}

// Mock provider for router tracking tests

type mockProviderWithRouterTracking struct {
	name            string
	onSelect        func(phase, tier string)
	runFn           func(ctx context.Context, prompt, tier string) (*provider.Result, error)
	streamRunResult *provider.Result
}

func (m *mockProviderWithRouterTracking) Name() string {
	if m.name != "" {
		return m.name
	}
	return "test-provider"
}

func (m *mockProviderWithRouterTracking) ModelForTier(tier string) string {
	switch tier {
	case provider.TierHigh:
		return "test-opus"
	case provider.TierMedium:
		return "test-sonnet"
	case provider.TierLow:
		return "test-haiku"
	default:
		return "test-model"
	}
}

func (m *mockProviderWithRouterTracking) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.onSelect != nil {
		// Track the Select() call indirectly through Run()
		m.onSelect("", tier)
	}
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProviderWithRouterTracking) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunResult != nil {
		return m.streamRunResult, nil
	}
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProviderWithRouterTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "test-model", Output: "VALIDATION_PASSED"}, nil
}

func (m *mockProviderWithRouterTracking) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
