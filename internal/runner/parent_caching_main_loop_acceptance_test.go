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

// TestMainLoop_FetchesParentImmediatelyAfterGetNextBead verifies that the main loop
// fetches the parent bead exactly once immediately after getNextBead() returns
// a bead with a non-empty Parent field, at line ~520 in runner.go, before calling
// any other functions. This ensures parent is available for runPrecheck, checkScope,
// and processBead without requiring additional GetParent calls.
func TestMainLoop_FetchesParentImmediatelyAfterGetNextBead(t *testing.T) {
	// Expected failure: The main loop currently does NOT fetch parent after getNextBead.
	// Instead, runPrecheck, checkScope, and setupBeadContext each call GetParent internally.
	// After implementation, the main loop should call GetParent once when b.Parent != "",
	// then pass parent to these functions via updated signatures:
	//   - runPrecheck(ctx context.Context, b *bead.Bead, parent *bead.Bead) (bool, time.Duration)
	//   - checkScope(ctx context.Context, b *bead.Bead, parent *bead.Bead) *prompt.ScopeEstimate
	//   - setupBeadContext(..., parent *bead.Bead, ...) (...)

	getParentCallCount := 0
	testBead := &bead.Bead{
		ID:       "test-bead-main",
		Title:    "Test Main Loop Parent Caching",
		Priority: 1,
		Parent:   "parent-main-123",
		Labels:   []string{},
	}

	parentBead := &bead.Bead{
		ID:       "parent-main-123",
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
			if id == "parent-main-123" {
				getParentCallCount++
				return parentBead, nil
			}
			return nil, fmt.Errorf("bead not found: %s", id)
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			if b == nil || b.Parent == "" {
				return nil, nil
			}
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

	precheckReceivedParent := false
	scopeReceivedParent := false
	buildContextReceivedParent := false

	mockPromptRenderer := &mockPromptRenderer{
		RenderPrecheckFn: func(ctx *prompt.PrecheckContext) (string, error) {
			if ctx.ParentBead != nil && ctx.ParentBead.ID == "parent-main-123" {
				precheckReceivedParent = true
			}
			return "precheck prompt", nil
		},
		RenderScopeFn: func(ctx *prompt.ScopeContext) (string, error) {
			if ctx.ParentBead != nil && ctx.ParentBead.ID == "parent-main-123" {
				scopeReceivedParent = true
			}
			return "scope: low\ncomplexity: 2", nil
		},
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			if parent != nil && parent.ID == "parent-main-123" {
				buildContextReceivedParent = true
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

	mockProviderRouter := &mockProviderRouter{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
			switch phase {
			case "precheck":
				return &mockProvider{
					FnName: func() string { return "mock-haiku" },
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output:   "PRECHECK_NOT_PASSED",
						}, nil
					},
				}, "mock-haiku"
			case "scope_check":
				return &mockProvider{
					FnName: func() string { return "mock-haiku" },
					FnRun: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
						return &provider.Result{
							Success:  true,
							ExitCode: 0,
							Output:   "scope: low\ncomplexity: 2\ncan_complete_in_single_iteration: true",
						}, nil
					},
				}, "mock-haiku"
			case "build":
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := r.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// CRITICAL ASSERTION: GetParent should be called exactly once in the main loop
	// The current implementation calls it 3 times (runPrecheck, checkScope, setupBeadContext)
	if getParentCallCount != 1 {
		t.Errorf("GetParent called %d times, expected exactly 1 (fetched once in main loop after getNextBead, passed to all functions)", getParentCallCount)
	}

	// Verify parent was passed to all three functions that need it
	if !precheckReceivedParent {
		t.Error("runPrecheck did not receive parent bead - should be passed from main loop")
	}
	if !scopeReceivedParent {
		t.Error("checkScope did not receive parent bead - should be passed from main loop")
	}
	if !buildContextReceivedParent {
		t.Error("BuildContext did not receive parent bead - should be passed from main loop")
	}
}

// TestMainLoop_ConditionalParentFetch verifies that when a bead has no parent
// (b.Parent == ""), the main loop does NOT call GetParent at all. When b.Parent != "",
// it fetches exactly once. This conditional logic should be at line ~520 in runner.go:
//
//	var parent *bead.Bead
//	if b.Parent != "" {
//	    parent, err = r.beads.GetParent(b)
//	}
func TestMainLoop_ConditionalParentFetch(t *testing.T) {
	// Expected failure: Current implementation may call GetParent even when b.Parent == "",
	// which wastes a subprocess call. The optimized version should check b.Parent == "" in
	// the main loop and skip the GetParent call entirely, passing nil parent to functions.

	getParentCallCount := 0
	testBead := &bead.Bead{
		ID:       "test-bead-no-parent",
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
			t.Error("GetParent should NOT be called when bead has no parent (b.Parent == \"\")")
			return nil, nil
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
			Sonnet: config.ModelConfig{Name: "claude-sonnet"},
		},
		Precheck: config.PrecheckConfig{
			Enabled: false,
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
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			if parent != nil {
				t.Error("BuildContext received non-nil parent when bead has no parent")
			}
			return &prompt.Context{
				Bead:       b,
				ParentBead: nil,
			}, nil
		},
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			return "build prompt", nil
		},
	}

	mockProviderRouter := &mockProviderRouter{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
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
		t.Errorf("GetParent called %d times for bead with no parent, expected 0 calls (should check b.Parent == \"\" before calling)", getParentCallCount)
	}
}

// TestProcessBead_ReceivesParentFromMainLoop verifies that processBead
// receives parent as a parameter and passes it to setupBeadContext.
func TestProcessBead_ReceivesParentFromMainLoop(t *testing.T) {
	// Expected failure: processBead signature does not include parent parameter yet.
	// Current signature: processBead(ctx, b, iteration, deadline, scopeEstimate)
	// After implementation: processBead(ctx, b, parent, iteration, deadline, scopeEstimate)
	// This ensures processBead can pass parent to setupBeadContext without fetching it.

	testBead := &bead.Bead{
		ID:       "test-process",
		Title:    "Test Process Signature",
		Priority: 1,
		Parent:   "parent-process-xyz",
		Labels:   []string{},
	}

	parentBead := &bead.Bead{
		ID:       "parent-process-xyz",
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
		CloseFn: func(id string) error {
			return nil
		},
		SyncFn: func() error {
			return nil
		},
	}

	cfg := &config.Config{
		Models: config.ModelsConfig{
			Sonnet: config.ModelConfig{Name: "claude-sonnet"},
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Claude: config.ClaudeConfig{
			BinaryPath: "claude",
		},
	}
	cfg.SetDefaults()

	parentPassedToBuildContext := false
	mockPromptRenderer := &mockPromptRenderer{
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			if parent != nil && parent.ID == "parent-process-xyz" {
				parentPassedToBuildContext = true
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

	mockProviderRouter := &mockProviderRouter{
		SelectFn: func(phase string, tier string) (provider.Provider, string) {
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

	ctx := context.Background()

	// The optimized signature should be:
	// result := r.processBead(ctx, testBead, parentBead, 1, time.Time{}, nil)
	// Expected failure: This will not compile until the signature is updated to accept parent parameter
	result := r.processBead(ctx, testBead, parentBead, 1, time.Time{}, nil)

	if !result.Success {
		t.Error("processBead should succeed")
	}

	// After optimization, processBead should NOT call GetParent (parent passed from main loop)
	if getParentCallCount > 0 {
		t.Errorf("processBead path called GetParent %d times - parent should be passed as parameter from main loop", getParentCallCount)
	}

	if !parentPassedToBuildContext {
		t.Error("Parent was not passed to BuildContext - should flow through processBead -> setupBeadContext -> BuildContext")
	}
}
