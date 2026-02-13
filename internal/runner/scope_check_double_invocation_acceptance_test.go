//go:build acceptance

package runner

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestScopeCheckNotDuplicatedInProcessBead verifies that when block_oversized is
// enabled and a bead passes the scope gate, the estimate from the gate is passed
// into processBead so buildPromptForBead doesn't call checkScope again.
//
// Expected failure: Currently processBead calls buildPromptForBead without passing
// the cached scopeEstimate, causing buildPromptForBead to call checkScope again
// even though the scope gate already evaluated it. The fix will pass scopeEstimate
// from the gate (runner.go:412) into setupBeadContext (process.go:53) and then into
// buildPromptForBead so it can skip calling checkScope when scopeEstimate is non-nil.
func TestScopeCheckNotDuplicatedInProcessBead(t *testing.T) {
	cfg := baseScopeGateConfig()
	blockOversized := true
	cfg.ScopeCheck.BlockOversized = &blockOversized

	testBead := &bead.Bead{
		ID:              "test-no-duplicate",
		Title:           "Test bead for no duplicate scope check",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	estimate := &prompt.ScopeEstimate{
		Complexity:                   "medium",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return testBead, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	// Track how many times RenderScope is called (scope check invocations)
	var renderScopeMu sync.Mutex
	renderScopeCallCount := 0
	mockRenderer := &mockPromptRenderer{
		RenderScopeFn: func(ctx *prompt.ScopeContext) (string, error) {
			renderScopeMu.Lock()
			renderScopeCallCount++
			renderScopeMu.Unlock()
			data, err := json.Marshal(estimate)
			if err != nil {
				t.Fatalf("failed to marshal scope estimate: %v", err)
			}
			return string(data), nil
		},
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			return &prompt.Context{
				Bead:       b,
				ParentBead: parent,
				Iteration:  iteration,
				Model:      model,
			}, nil
		},
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			return "mock build prompt", nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: p}, nil
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "mock output"}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: mockRenderer,
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	// Simulate the scope gate check in Run() (first call at runner.go:414)
	scopeEstimate := r.checkScope(ctx, testBead)
	if scopeEstimate == nil {
		t.Fatal("checkScope returned nil")
	}

	// Verify the scope gate called RenderScope once
	renderScopeMu.Lock()
	gateCallCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if gateCallCount != 1 {
		t.Fatalf("expected 1 RenderScope call from scope gate, got %d", gateCallCount)
	}

	// Now simulate processBead which calls buildPromptForBead
	// This is where the duplicate call currently happens (process.go:117)
	deadline := time.Time{} // no deadline
	result := r.processBead(ctx, testBead, 1, deadline, scopeEstimate)

	// ACCEPTANCE CRITERION: No additional RenderScope call from buildPromptForBead
	// The current implementation calls checkScope again in process.go:117 when
	// scopeEstimate is nil, but the fix will pass scopeEstimate from the gate
	// into setupBeadContext and then into buildPromptForBead.
	renderScopeMu.Lock()
	finalCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if finalCount != 1 {
		t.Errorf("expected exactly 1 RenderScope call (scope gate only), got %d (processBead called checkScope again)", finalCount)
	}

	// Verify processBead completed without error
	if result.Error != nil {
		t.Logf("processBead error (expected in test setup): %v", result.Error)
	}
}

// TestSetupBeadContextAcceptsScopeEstimate verifies that setupBeadContext
// accepts a scopeEstimate parameter and stores it in the beadContext so
// buildPromptForBead can access it.
//
// Expected failure: The current setupBeadContext signature at process.go:53
// does not accept a scopeEstimate parameter. The fix will add this parameter
// and store it in the returned beadContext.
func TestSetupBeadContextAcceptsScopeEstimate(t *testing.T) {
	cfg := baseScopeGateConfig()

	testBead := &bead.Bead{
		ID:              "test-setup-context",
		Title:           "Test setup context",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	estimate := &prompt.ScopeEstimate{
		Complexity:                   "low",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}

	mockBeads := &mockBeadClient{
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouter(),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()
	deadline := time.Time{} // no deadline

	// Call setupBeadContext with a scopeEstimate
	bc, _, cancel, err := r.setupBeadContext(ctx, testBead, 1, deadline, estimate)
	defer cancel()
	if err != nil {
		t.Fatalf("setupBeadContext error: %v", err)
	}

	// ACCEPTANCE CRITERION: The scopeEstimate should be stored in beadContext
	// The current implementation doesn't store scopeEstimate, so bc.ScopeEstimate
	// will be nil. The fix will store it in beadContext.scopeEstimate.
	if bc.ScopeEstimate != estimate {
		t.Errorf("expected beadContext.scopeEstimate to be %v, got %v", estimate, bc.ScopeEstimate)
	}
}

// TestBuildPromptForBeadSkipsScopeCheckWhenEstimateCached verifies that when
// buildPromptForBead is called with a non-nil bc.ScopeEstimate, it does not
// call checkScope again.
//
// Expected failure: The current buildPromptForBead (process.go:103) always
// calls checkScope when r.cfg.ScopeCheck.Enabled is true, even if bc.ScopeEstimate
// is non-nil. The fix will check if bc.ScopeEstimate is non-nil before calling
// checkScope at process.go:116.
func TestBuildPromptForBeadSkipsScopeCheckWhenEstimateCached(t *testing.T) {
	cfg := baseScopeGateConfig()

	testBead := &bead.Bead{
		ID:              "test-cached-estimate",
		Title:           "Test cached estimate",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// Pre-computed estimate (from scope gate)
	cachedEstimate := &prompt.ScopeEstimate{
		Complexity:                   "medium",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}

	mockBeads := &mockBeadClient{
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	// Track RenderScope calls
	var renderScopeMu sync.Mutex
	renderScopeCallCount := 0
	mockRenderer := &mockPromptRenderer{
		RenderScopeFn: func(ctx *prompt.ScopeContext) (string, error) {
			renderScopeMu.Lock()
			renderScopeCallCount++
			renderScopeMu.Unlock()
			// This should not be called when estimate is cached
			data, err := json.Marshal(cachedEstimate)
			if err != nil {
				t.Fatalf("failed to marshal scope estimate: %v", err)
			}
			return string(data), nil
		},
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			return &prompt.Context{
				Bead:       b,
				ParentBead: parent,
				Iteration:  iteration,
				Model:      model,
			}, nil
		},
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			return "mock build prompt", nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouter(),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: mockRenderer,
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	// Create a beadContext with cached scopeEstimate
	bc := &runtypes.BeadContext{
		Bead:          testBead,
		Parent:        nil,
		Result:        &IterationResult{Model: "sonnet"},
		Model:         "sonnet",
		ScopeEstimate: cachedEstimate, // Pre-populated from scope gate
	}

	// Call buildPromptForBead with cached estimate
	err = r.buildPromptForBead(ctx, bc, 1)
	if err != nil {
		t.Fatalf("buildPromptForBead error: %v", err)
	}

	// ACCEPTANCE CRITERION: RenderScope should NOT be called because estimate is cached
	// The current implementation at process.go:117 calls checkScope regardless of
	// whether bc.ScopeEstimate is set. The fix will check if bc.ScopeEstimate != nil
	// before calling checkScope.
	renderScopeMu.Lock()
	finalCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if finalCount != 0 {
		t.Errorf("expected 0 RenderScope calls (estimate cached), got %d", finalCount)
	}

	// ACCEPTANCE CRITERION: Auto-escalation should use the cached estimate
	// If complexity is "high", model should be escalated to opus using cached data
	bc.ScopeEstimate.Complexity = "high"
	bc.Model = "sonnet" // Reset model
	err = r.buildPromptForBead(ctx, bc, 1)
	if err != nil {
		t.Fatalf("buildPromptForBead error on retry: %v", err)
	}

	// Model should be escalated based on cached complexity without calling checkScope
	if bc.Model != "opus" {
		t.Errorf("expected model escalated to opus based on cached complexity=high, got %s", bc.Model)
	}

	// Verify still no additional RenderScope calls
	renderScopeMu.Lock()
	finalCount = renderScopeCallCount
	renderScopeMu.Unlock()
	if finalCount != 0 {
		t.Errorf("expected 0 RenderScope calls (cached for escalation too), got %d", finalCount)
	}
}

// TestProcessBeadReceivesScopeEstimateFromRun verifies that the Run() method
// passes the scopeEstimate from the scope gate (runner.go:414) into processBead
// (runner.go:480) as the final parameter.
//
// Expected failure: The current processBead call at runner.go:480 does not pass
// scopeEstimate. The fix will add scopeEstimate as a parameter to processBead
// and pass the cached estimate from the gate: r.processBead(ctx, b, iteration, deadline, scopeEstimate)
func TestProcessBeadReceivesScopeEstimateFromRun(t *testing.T) {
	// This test verifies the wiring between Run() and processBead()
	// by checking that processBead is called with the scopeEstimate parameter.

	cfg := baseScopeGateConfig()
	blockOversized := true
	cfg.ScopeCheck.BlockOversized = &blockOversized

	testBead := &bead.Bead{
		ID:              "test-wiring",
		Title:           "Test Run to processBead wiring",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	estimate := &prompt.ScopeEstimate{
		Complexity:                   "medium",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return testBead, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	var renderScopeMu sync.Mutex
	renderScopeCallCount := 0
	mockRenderer := &mockPromptRenderer{
		RenderScopeFn: func(ctx *prompt.ScopeContext) (string, error) {
			renderScopeMu.Lock()
			renderScopeCallCount++
			renderScopeMu.Unlock()
			data, err := json.Marshal(estimate)
			if err != nil {
				t.Fatalf("failed to marshal scope estimate: %v", err)
			}
			return string(data), nil
		},
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			return &prompt.Context{
				Bead:       b,
				ParentBead: parent,
				Iteration:  iteration,
				Model:      model,
			}, nil
		},
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			return "mock build prompt", nil
		},
	}

	// Mock Claude to succeed quickly so we can verify scope check call count
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: p}, nil
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "mock success"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "mock validation pass"}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: mockRenderer,
			Logger:   &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()

	// Run a single iteration (max 1 iteration)
	err = r.Run(ctx, 1, time.Time{}, false)
	if err != nil {
		// Errors are expected in test setup (e.g., git operations), don't fail
		t.Logf("Run error (expected in test): %v", err)
	}

	// ACCEPTANCE CRITERION: RenderScope should be called exactly once
	// (from scope gate only, not from buildPromptForBead)
	renderScopeMu.Lock()
	finalCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if finalCount != 1 {
		t.Errorf("expected exactly 1 RenderScope call (scope gate only), got %d", finalCount)
	}
}

// TestProcessBeadSignatureIncludesScopeEstimate verifies that processBead's
// signature includes scopeEstimate as the final parameter.
//
// Expected failure: The current processBead signature at process.go:630 is:
// func (r *Runner) processBead(ctx context.Context, b *bead.Bead, iteration int, deadline time.Time)
// The fix will add scopeEstimate parameter:
// func (r *Runner) processBead(ctx context.Context, b *bead.Bead, iteration int, deadline time.Time, scopeEstimate *prompt.ScopeEstimate)
func TestProcessBeadSignatureIncludesScopeEstimate(t *testing.T) {
	// This test verifies the signature by attempting to call processBead with
	// the scopeEstimate parameter. If the parameter doesn't exist, this won't compile.

	cfg := baseScopeGateConfig()

	testBead := &bead.Bead{
		ID:              "test-signature",
		Title:           "Test processBead signature",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	estimate := &prompt.ScopeEstimate{
		Complexity:                   "low",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}

	mockBeads := &mockBeadClient{
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "mock output"}, nil
		},
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{
				BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
					return &prompt.Context{}, nil
				},
				RenderBuildFn: func(ctx *prompt.Context) (string, error) {
					return "mock", nil
				},
			},
			Logger: &mockIterationLogger{},
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	ctx := context.Background()
	deadline := time.Time{}

	// ACCEPTANCE CRITERION: This call should compile with scopeEstimate parameter
	// The current processBead signature doesn't accept scopeEstimate, so this will
	// fail to compile. The fix will add the parameter.
	result := r.processBead(ctx, testBead, 1, deadline, estimate)

	// Basic verification that processBead ran
	if result == nil {
		t.Fatal("processBead returned nil result")
	}
	if result.BeadID != testBead.ID {
		t.Errorf("expected result.BeadID=%s, got %s", testBead.ID, result.BeadID)
	}
}
