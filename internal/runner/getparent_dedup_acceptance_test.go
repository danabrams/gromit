//go:build acceptance

package runner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// mockProviderRouter is a simple router mock for these tests
type mockProviderRouter struct {
	SelectFn func(phase string, tier string) (provider.Provider, string)
}

func (m *mockProviderRouter) Select(phase string, tier string) (provider.Provider, string) {
	if m.SelectFn != nil {
		return m.SelectFn(phase, tier)
	}
	return &mockProvider{}, "mock"
}

// mockProvider is a simple provider mock
type mockProvider struct {
	FnName func() string
	FnRun  func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
}

func (m *mockProvider) Name() string {
	if m.FnName != nil {
		return m.FnName()
	}
	return "mock-provider"
}

func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.FnRun != nil {
		return m.FnRun(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, ExitCode: 0, Output: "mock output"}, nil
}

func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, writer provider.StreamWriter) (*provider.Result, error) {
	// For acceptance tests, just call Run
	return m.Run(ctx, prompt, tier)
}

// TestGetParent_CalledOncePerIteration verifies that GetParent is called
// at most once per iteration when a bead has a parent, even when precheck,
// scope check, and build all run.
func TestGetParent_CalledOncePerIteration(t *testing.T) {
	// Expected failure: Currently GetParent is called up to 3 times per iteration:
	// - runPrecheck() at runner.go:1863
	// - checkScope() at runner.go:1929
	// - setupBeadContext() at process.go:83 (via buildPromptForBead)
	// This test verifies it's called only once in the main loop and passed to functions.

	getParentCallCount := 0
	testBead := &bead.Bead{
		ID:       "test-bead-1",
		Title:    "Test Bead",
		Priority: 1,
		Parent:   "parent-bead-123",
		Labels:   []string{},
	}

	parentBead := &bead.Bead{
		ID:       "parent-bead-123",
		Title:    "Parent Epic",
		Priority: 0,
		Type:     "epic",
		Labels:   []string{},
	}

	mockClient := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return testBead, nil
		},
		ShowFn: func(id string) (*bead.Bead, error) {
			if id == "parent-bead-123" {
				getParentCallCount++
				return parentBead, nil
			}
			return nil, fmt.Errorf("bead not found: %s", id)
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			if b == nil || b.Parent == "" {
				return nil, nil
			}
			// GetParent calls Show internally
			getParentCallCount++
			return parentBead, nil
		},
		CloseFn: func(id string) error {
			return nil
		},
		SyncFn: func() error {
			return nil
		},
	}

	cfg := &config.Config{
		Models: config.ModelsConfig{
			Haiku:  config.ModelConfig{Name: "claude-haiku"},
			Sonnet: config.ModelConfig{Name: "claude-sonnet"},
			Opus:   config.ModelConfig{Name: "claude-opus"},
		},
		Precheck: config.PrecheckConfig{
			Enabled: true,
			Model:   "haiku",
		},
		ScopeCheck: config.ScopeCheckConfig{
			Enabled: true,
		},
		Validation: config.ValidationConfig{
			Enabled: false, // Disable validation to keep test focused
		},
		Loop: config.LoopConfig{
			MaxIterations:         1,
			StuckBeadThreshold:    3,
			MaxConsecutiveSkips:   10,
			BetweenIterationHooks: []string{},
		},
		Claude: config.ClaudeConfig{
			BinaryPath: "claude",
		},
		Paths: config.PathsConfig{
			Templates: ".gromit/templates",
			Specs:     ".gromit/specs",
			Logs:      ".gromit/logs",
		},
	}
	cfg.SetDefaults()

	// Mock renderer that tracks RenderPrecheck, RenderScope, and BuildContext calls
	mockPromptRenderer := &mockPromptRenderer{
		RenderPrecheckFn: func(ctx *prompt.PrecheckContext) (string, error) {
			// Verify parent is passed in, not fetched here
			if ctx.ParentBead == nil && testBead.Parent != "" {
				t.Error("RenderPrecheck received nil ParentBead - should be fetched once in main loop")
			}
			return "precheck prompt", nil
		},
		RenderScopeFn: func(ctx *prompt.ScopeContext) (string, error) {
			// Verify parent is passed in, not fetched here
			if ctx.ParentBead == nil && testBead.Parent != "" {
				t.Error("RenderScope received nil ParentBead - should be fetched once in main loop")
			}
			return "scope: low\ncomplexity: 2", nil
		},
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			// Verify parent is passed in, not fetched here
			if parent == nil && b.Parent != "" {
				t.Error("BuildContext received nil parent - should be fetched once in main loop")
			}
			return &prompt.Context{
				Bead:       b,
				ParentBead: parent,
			}, nil
		},
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			return "build prompt", nil
		},
	}

	// Mock router that returns providers for each phase
	precheckCallCount := 0
	scopeCallCount := 0
	mockProviderRouter := &mockProviderRouter{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			switch phase {
			case "precheck":
				precheckCallCount++
				return &mockProvider{
					FnName: func() string { return "mock-haiku" },
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						// Precheck should NOT pass (so build runs)
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output:   "PRECHECK_NOT_PASSED",
						}, nil
					},
				}, "mock-haiku"
			case "scope_check":
				scopeCallCount++
				return &mockProvider{
					FnName: func() string { return "mock-haiku" },
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output:   "scope: low\ncomplexity: 2",
						}, nil
					},
				}, "mock-haiku"
			case "build":
				return &mockProvider{
					FnName: func() string { return "mock-sonnet" },
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						// Build succeeds to complete iteration
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output:   "build output",
						}, nil
					},
				}, "mock-sonnet"
			default:
				return &mockProvider{
					FnName: func() string { return "mock-haiku" },
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output:   "output",
						}, nil
					},
				}, "mock-haiku"
			}
		},
	}

	r := &Runner{
		cfg:      cfg,
		beads:    mockClient,
		renderer: mockPromptRenderer,
		router:   mockProviderRouter,
		logFn:    func(format string, args ...interface{}) {},
	}

	// Run one iteration
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := r.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify GetParent was called exactly once
	// The optimized implementation fetches parent once in main loop and passes it
	// to runPrecheck, checkScope, and setupBeadContext
	if getParentCallCount != 1 {
		t.Errorf("GetParent called %d times, expected exactly 1 (fetch once in main loop, pass to all functions)", getParentCallCount)
	}

	// Verify precheck and scope check both ran (so we know parent was needed)
	if precheckCallCount != 1 {
		t.Errorf("Precheck called %d times, expected 1", precheckCallCount)
	}
	if scopeCallCount != 1 {
		t.Errorf("Scope check called %d times, expected 1", scopeCallCount)
	}
}

// TestGetParent_NotCalledWhenNoParent verifies that when a bead has no parent,
// GetParent is not called at all.
func TestGetParent_NotCalledWhenNoParent(t *testing.T) {
	// Expected failure: Current implementation may call GetParent and return nil
	// immediately, rather than checking b.Parent == "" first in the main loop.

	getParentCallCount := 0
	testBead := &bead.Bead{
		ID:       "test-bead-2",
		Title:    "Test Bead Without Parent",
		Priority: 1,
		Parent:   "", // No parent
		Labels:   []string{},
	}

	mockClient := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return testBead, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			getParentCallCount++
			if b == nil || b.Parent == "" {
				return nil, nil
			}
			return nil, fmt.Errorf("unexpected GetParent call")
		},
		FnClose: func(id string) error {
			return nil
		},
		FnSync: func() error {
			return nil
		},
	}

	cfg := &config.Config{
		Models: config.ModelsConfig{
			Haiku:  config.ModelConfig{Name: "claude-haiku"},
			Sonnet: config.ModelConfig{Name: "claude-sonnet"},
			Opus:   config.ModelConfig{Name: "claude-opus"},
		},
		Precheck: config.PrecheckConfig{
			Enabled: false, // Disable to keep test simple
		},
		ScopeCheck: config.ScopeCheckConfig{
			Enabled: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Loop: config.LoopConfig{
			MaxIterations:         1,
			StuckBeadThreshold:    3,
			MaxConsecutiveSkips:   10,
			BetweenIterationHooks: []string{},
		},
		Claude: config.ClaudeConfig{
			BinaryPath: "claude",
		},
		Paths: config.PathsConfig{
			Templates: ".gromit/templates",
			Specs:     ".gromit/specs",
			Logs:      ".gromit/logs",
		},
	}
	cfg.SetDefaults()

	mockPromptRenderer := &mockPromptRenderer{
		FnBuildContext: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.BuildContext, error) {
			return &prompt.BuildContext{
				Bead:       b,
				ParentBead: parent,
			}, nil
		},
		FnRender: func(ctx *prompt.BuildContext) (string, error) {
			return "build prompt", nil
		},
	}

	mockProviderRouter := &mockProviderRouter{
		FnSelect: func(phase string, tier string) (provider.Provider, string) {
			return &mockProvider{
				FnName: func() string { return "mock-sonnet" },
				FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
					return &provider.Result{
						Success:  true,
						ExitCode: 0,
						Output:   "build output",
					}, nil
				},
			}, "mock-sonnet"
		},
	}

	r := &Runner{
		cfg:      cfg,
		beads:    mockClient,
		renderer: mockPromptRenderer,
		router:   mockProviderRouter,
		logFn:    func(format string, args ...interface{}) {},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := r.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify GetParent was NOT called (bead has no parent)
	if getParentCallCount > 0 {
		t.Errorf("GetParent called %d times for bead with no parent, expected 0 calls (should check b.Parent == \"\" first)", getParentCallCount)
	}
}

// TestRunPrecheck_AcceptsParentParameter verifies that runPrecheck accepts
// a parent bead as a parameter instead of calling GetParent internally.
func TestRunPrecheck_AcceptsParentParameter(t *testing.T) {
	// Expected failure: runPrecheck() currently calls r.beads.GetParent(b) at runner.go:1863.
	// After optimization, it should accept parent as a parameter.

	testBead := &bead.Bead{
		ID:       "test-bead-3",
		Title:    "Test Bead",
		Priority: 1,
		Parent:   "parent-123",
		Labels:   []string{},
	}

	parentBead := &bead.Bead{
		ID:       "parent-123",
		Title:    "Parent",
		Priority: 0,
		Type:     "epic",
		Labels:   []string{},
	}

	getParentCallCount := 0
	mockClient := &mockBeadClient{
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			getParentCallCount++
			return parentBead, nil
		},
	}

	cfg := &config.Config{
		Models: config.ModelsConfig{
			Haiku: config.ModelConfig{Name: "claude-haiku"},
		},
		Precheck: config.PrecheckConfig{
			Enabled: true,
			Model:   "haiku",
		},
		Claude: config.ClaudeConfig{
			BinaryPath: "claude",
		},
	}
	cfg.SetDefaults()

	parentPassedToRenderer := false
	mockPromptRenderer := &mockPromptRenderer{
		FnRenderPrecheck: func(ctx *prompt.PrecheckContext) (string, error) {
			if ctx.ParentBead != nil && ctx.ParentBead.ID == "parent-123" {
				parentPassedToRenderer = true
			}
			return "precheck prompt", nil
		},
	}

	mockProviderRouter := &mockProviderRouter{
		FnSelect: func(phase string, tier string) (provider.Provider, string) {
			return &mockProvider{
				FnName: func() string { return "mock-haiku" },
				FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
					return &provider.Result{
						Success:  true,
						ExitCode: 0,
						Output:   "PRECHECK_PASSED",
					}, nil
				},
			}, "mock-haiku"
		},
	}

	r := &Runner{
		cfg:      cfg,
		beads:    mockClient,
		renderer: mockPromptRenderer,
		router:   mockProviderRouter,
		logFn:    func(format string, args ...interface{}) {},
	}

	// The optimized version should have a signature like:
	// runPrecheck(ctx context.Context, b *bead.Bead, parent *bead.Bead) (bool, time.Duration)
	// and should NOT call GetParent internally

	ctx := context.Background()
	passed, _ := r.runPrecheck(ctx, testBead)

	if !passed {
		t.Error("runPrecheck should return true for PRECHECK_PASSED")
	}

	// In the current implementation, runPrecheck calls GetParent internally
	// In the optimized version, parent should be passed as parameter
	if getParentCallCount > 0 {
		t.Errorf("runPrecheck called GetParent %d times - should accept parent as parameter instead", getParentCallCount)
	}

	if !parentPassedToRenderer {
		t.Error("Parent was not passed to RenderPrecheck - should be passed from main loop")
	}
}

// TestCheckScope_AcceptsParentParameter verifies that checkScope accepts
// a parent bead as a parameter instead of calling GetParent internally.
func TestCheckScope_AcceptsParentParameter(t *testing.T) {
	// Expected failure: checkScope() currently calls r.beads.GetParent(b) at runner.go:1929.
	// After optimization, it should accept parent as a parameter.

	testBead := &bead.Bead{
		ID:       "test-bead-4",
		Title:    "Test Bead",
		Priority: 1,
		Parent:   "parent-456",
		Labels:   []string{},
	}

	parentBead := &bead.Bead{
		ID:       "parent-456",
		Title:    "Parent",
		Priority: 0,
		Type:     "epic",
		Labels:   []string{},
	}

	getParentCallCount := 0
	mockClient := &mockBeadClient{
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			getParentCallCount++
			return parentBead, nil
		},
	}

	cfg := &config.Config{
		Models: config.ModelsConfig{
			Haiku: config.ModelConfig{Name: "claude-haiku"},
		},
		ScopeCheck: config.ScopeCheckConfig{
			Enabled: true,
		},
		Claude: config.ClaudeConfig{
			BinaryPath: "claude",
		},
	}
	cfg.SetDefaults()

	parentPassedToRenderer := false
	mockPromptRenderer := &mockPromptRenderer{
		FnRenderScope: func(ctx *prompt.ScopeContext) (string, error) {
			if ctx.ParentBead != nil && ctx.ParentBead.ID == "parent-456" {
				parentPassedToRenderer = true
			}
			return "scope: medium\ncomplexity: 5", nil
		},
	}

	mockProviderRouter := &mockProviderRouter{
		FnSelect: func(phase string, tier string) (provider.Provider, string) {
			return &mockProvider{
				FnName: func() string { return "mock-haiku" },
				FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
					return &provider.Result{
						Success:  true,
						ExitCode: 0,
						Output:   "scope: medium\ncomplexity: 5",
					}, nil
				},
			}, "mock-haiku"
		},
	}

	r := &Runner{
		cfg:      cfg,
		beads:    mockClient,
		renderer: mockPromptRenderer,
		router:   mockProviderRouter,
		logFn:    func(format string, args ...interface{}) {},
	}

	// The optimized version should have a signature like:
	// checkScope(ctx context.Context, b *bead.Bead, parent *bead.Bead) *prompt.ScopeEstimate
	// and should NOT call GetParent internally

	ctx := context.Background()
	estimate := r.checkScope(ctx, testBead)

	if estimate == nil {
		t.Fatal("checkScope returned nil")
	}

	// In the current implementation, checkScope calls GetParent internally
	// In the optimized version, parent should be passed as parameter
	if getParentCallCount > 0 {
		t.Errorf("checkScope called GetParent %d times - should accept parent as parameter instead", getParentCallCount)
	}

	if !parentPassedToRenderer {
		t.Error("Parent was not passed to RenderScope - should be passed from main loop")
	}
}

// TestSetupBeadContext_AcceptsParentParameter verifies that setupBeadContext
// accepts a parent bead as a parameter instead of fetching it via BuildContext.
func TestSetupBeadContext_AcceptsParentParameter(t *testing.T) {
	// Expected failure: setupBeadContext calls BuildContext which internally calls
	// GetParent at process.go:83. After optimization, parent should be passed in.

	testBead := &bead.Bead{
		ID:       "test-bead-5",
		Title:    "Test Bead",
		Priority: 1,
		Parent:   "parent-789",
		Labels:   []string{},
	}

	parentBead := &bead.Bead{
		ID:       "parent-789",
		Title:    "Parent",
		Priority: 0,
		Type:     "epic",
		Labels:   []string{},
	}

	getParentCallCount := 0
	mockClient := &mockBeadClient{
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			getParentCallCount++
			return parentBead, nil
		},
	}

	cfg := &config.Config{
		Models: config.ModelsConfig{
			Sonnet: config.ModelConfig{Name: "claude-sonnet"},
		},
		Claude: config.ClaudeConfig{
			BinaryPath: "claude",
		},
	}
	cfg.SetDefaults()

	parentPassedToBuildContext := false
	mockPromptRenderer := &mockPromptRenderer{
		FnBuildContext: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.BuildContext, error) {
			if parent != nil && parent.ID == "parent-789" {
				parentPassedToBuildContext = true
			}
			return &prompt.BuildContext{
				Bead:       b,
				ParentBead: parent,
			}, nil
		},
		FnRender: func(ctx *prompt.BuildContext) (string, error) {
			return "build prompt", nil
		},
	}

	mockProviderRouter := &mockProviderRouter{
		FnSelect: func(phase string, tier string) (provider.Provider, string) {
			return &mockProvider{
				FnName: func() string { return "mock-sonnet" },
			}, "mock-sonnet"
		},
	}

	r := &Runner{
		cfg:      cfg,
		beads:    mockClient,
		renderer: mockPromptRenderer,
		router:   mockProviderRouter,
		logFn:    func(format string, args ...interface{}) {},
	}

	// The optimized version should have a signature like:
	// setupBeadContext(ctx context.Context, b *bead.Bead, parent *bead.Bead, iteration int, ...) (...)
	// and BuildContext should be called with parent already fetched

	ctx := context.Background()
	bc, _, cancel, err := r.setupBeadContext(ctx, testBead, 1, time.Time{}, nil)
	if cancel != nil {
		defer cancel()
	}

	if err != nil {
		t.Fatalf("setupBeadContext error: %v", err)
	}

	if bc == nil {
		t.Fatal("setupBeadContext returned nil BeadContext")
	}

	// In the current implementation, BuildContext may call GetParent
	// In the optimized version, parent should be passed from main loop
	if getParentCallCount > 0 {
		t.Errorf("setupBeadContext path called GetParent %d times - parent should be passed from main loop", getParentCallCount)
	}

	if !parentPassedToBuildContext {
		t.Error("Parent was not passed to BuildContext - should be fetched in main loop and passed through")
	}
}
