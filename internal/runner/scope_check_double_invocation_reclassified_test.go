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
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

type scopeCheckDedupHarnessOptions struct {
	ReadyFn         func() (*bead.Bead, error)
	GetParentFn     func(b *bead.Bead) (*bead.Bead, error)
	BuildContextFn  func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error)
	RenderBuildFn   func(ctx *prompt.Context) (string, error)
	RenderScopeFn   func(ctx *prompt.ScopeContext) (string, error)
	RunFn           func(ctx context.Context, p string, model string) (*claude.Result, error)
	StreamRunFn     func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error)
	RunValidationFn func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error)
}

type scopeCheckDedupHarness struct {
	runner          *Runner
	renderScopeMu   sync.Mutex
	renderScopeCall int
}

func (h *scopeCheckDedupHarness) RenderScopeCalls() int {
	h.renderScopeMu.Lock()
	defer h.renderScopeMu.Unlock()
	return h.renderScopeCall
}

func newScopeCheckBead(id, title string) *bead.Bead {
	return &bead.Bead{
		ID:              id,
		Title:           title,
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}
}

func newScopeEstimate(complexity string) *prompt.ScopeEstimate {
	return &prompt.ScopeEstimate{
		Complexity:                   complexity,
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}
}

func assertRenderScopeCallCount(t *testing.T, harness *scopeCheckDedupHarness, expected int) {
	t.Helper()
	if got := harness.RenderScopeCalls(); got != expected {
		t.Errorf("expected %d RenderScope calls, got %d", expected, got)
	}
}

func newScopeCheckDedupHarness(t *testing.T, cfg *config.Config, estimate *prompt.ScopeEstimate, opts scopeCheckDedupHarnessOptions) *scopeCheckDedupHarness {
	t.Helper()

	mockBeads := &mockBeadClient{}
	if opts.ReadyFn != nil {
		mockBeads.ReadyFn = opts.ReadyFn
	}
	if opts.GetParentFn != nil {
		mockBeads.GetParentFn = opts.GetParentFn
	} else {
		mockBeads.GetParentFn = func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		}
	}

	h := &scopeCheckDedupHarness{}
	mockRenderer := &mockPromptRenderer{
		RenderScopeFn: opts.RenderScopeFn,
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			if opts.BuildContextFn != nil {
				return opts.BuildContextFn(b, parent, iteration, model)
			}
			return &prompt.Context{
				Bead:       b,
				ParentBead: parent,
				Iteration:  iteration,
				Model:      model,
			}, nil
		},
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			if opts.RenderBuildFn != nil {
				return opts.RenderBuildFn(ctx)
			}
			return "mock build prompt", nil
		},
	}

	if mockRenderer.RenderScopeFn == nil {
		mockRenderer.RenderScopeFn = func(ctx *prompt.ScopeContext) (string, error) {
			h.renderScopeMu.Lock()
			h.renderScopeCall++
			h.renderScopeMu.Unlock()
			data, err := json.Marshal(estimate)
			if err != nil {
				t.Fatalf("failed to marshal scope estimate: %v", err)
			}
			return string(data), nil
		}
	}

	mockClaude := &mockClaudeClient{}
	mockClaude.RunFn = opts.RunFn
	if mockClaude.RunFn == nil {
		mockClaude.RunFn = func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: p}, nil
		}
	}
	mockClaude.StreamRunFn = opts.StreamRunFn
	mockClaude.RunValidationFn = opts.RunValidationFn

	var buf strings.Builder
	runner, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
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

	h.runner = runner
	return h
}

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

	testBead := newScopeCheckBead("test-no-duplicate", "Test bead for no duplicate scope check")

	estimate := newScopeEstimate("medium")
	h := newScopeCheckDedupHarness(t, cfg, estimate, scopeCheckDedupHarnessOptions{
		ReadyFn: func() (*bead.Bead, error) {
			return testBead, nil
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "mock output"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "mock validation pass"}, nil
		},
	})

	ctx := context.Background()

	// Simulate the scope gate check in Run() (first call at runner.go:414)
	scopeEstimate := h.runner.checkScope(ctx, testBead)
	if scopeEstimate == nil {
		t.Fatal("checkScope returned nil")
	}

	// Verify the scope gate called RenderScope once
	assertRenderScopeCallCount(t, h, 1)

	// Now simulate processBead which calls buildPromptForBead
	// This is where the duplicate call currently happens (process.go:117)
	deadline := time.Time{} // no deadline
	result := h.runner.processBead(ctx, testBead, 1, deadline, scopeEstimate)

	// ACCEPTANCE CRITERION: No additional RenderScope call from buildPromptForBead
	// The current implementation calls checkScope again in process.go:117 when
	// scopeEstimate is nil, but the fix will pass scopeEstimate from the gate
	// into setupBeadContext and then into buildPromptForBead.
	assertRenderScopeCallCount(t, h, 1)

	// Verify processBead completed without error
	if result.Error != nil {
		t.Logf("processBead error (expected in test setup): %v", result.Error)
	}
}

// TestSetupBeadContextAcceptsScopeEstimate verifies that setupBeadContext
// accepts a scopeEstimate parameter and stores it in the BeadContext so
// buildPromptForBead can access it.
func TestSetupBeadContextAcceptsScopeEstimate(t *testing.T) {
	cfg := baseScopeGateConfig()

	testBead := newScopeCheckBead("test-setup-context", "Test setup context")

	estimate := newScopeEstimate("low")

	h := newScopeCheckDedupHarness(t, cfg, estimate, scopeCheckDedupHarnessOptions{
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) { return nil, nil },
	})

	ctx := context.Background()
	deadline := time.Time{} // no deadline

	// Call setupBeadContext with a scopeEstimate
	bc, _, cancel, err := h.runner.setupBeadContext(ctx, testBead, 1, deadline, estimate)
	defer cancel()
	if err != nil {
		t.Fatalf("setupBeadContext error: %v", err)
	}

	// ACCEPTANCE CRITERION: The scopeEstimate should be stored in BeadContext
	if bc.ScopeEstimate != estimate {
		t.Errorf("expected BeadContext.ScopeEstimate to be %v, got %v", estimate, bc.ScopeEstimate)
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
func TestScopeCheckReclassified_CachedEstimateSkipsDuplicateInvocation(t *testing.T) {
	cfg := baseScopeGateConfig()

	testBead := newScopeCheckBead("test-cached-estimate", "Test cached estimate")

	cases := []struct {
		name          string
		complexity    string
		expectedModel string
	}{
		{name: "cached estimate does not re-check scope", complexity: "medium", expectedModel: "sonnet"},
		{name: "cached high complexity escalates", complexity: "high", expectedModel: "opus"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			estimate := newScopeEstimate(tc.complexity)
			h := newScopeCheckDedupHarness(t, cfg, estimate, scopeCheckDedupHarnessOptions{
				GetParentFn: func(b *bead.Bead) (*bead.Bead, error) { return nil, nil },
			})
			ctx := context.Background()

			bc := &runtypes.BeadContext{
				Bead:          testBead,
				Parent:        nil,
				Result:        &IterationResult{Model: "sonnet"},
				Model:         "sonnet",
				ScopeEstimate: estimate,
			}

			if err := h.runner.buildPromptForBead(ctx, bc, 1); err != nil {
				t.Fatalf("buildPromptForBead error: %v", err)
			}

			assertRenderScopeCallCount(t, h, 0)
			if bc.Model != tc.expectedModel {
				t.Errorf("expected model %q, got %s", tc.expectedModel, bc.Model)
			}
		})
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

	testBead := newScopeCheckBead("test-wiring", "Test Run to processBead wiring")

	estimate := newScopeEstimate("medium")

	h := newScopeCheckDedupHarness(t, cfg, estimate, scopeCheckDedupHarnessOptions{
		ReadyFn: func() (*bead.Bead, error) {
			return testBead, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: p}, nil
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "mock success"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "mock validation pass"}, nil
		},
	})

	ctx := context.Background()

	// Run a single iteration (max 1 iteration)
	err := h.runner.Run(ctx, 1, time.Time{}, nil, false)
	if err != nil {
		// Errors are expected in test setup (e.g., git operations), don't fail
		t.Logf("Run error (expected in test): %v", err)
	}

	// ACCEPTANCE CRITERION: RenderScope should be called exactly once
	// (from scope gate only, not from buildPromptForBead)
	assertRenderScopeCallCount(t, h, 1)
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

	testBead := newScopeCheckBead("test-signature", "Test processBead signature")

	estimate := newScopeEstimate("low")
	h := newScopeCheckDedupHarness(t, cfg, nil, scopeCheckDedupHarnessOptions{
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
		RenderScopeFn: func(ctx *prompt.ScopeContext) (string, error) {
			return "", nil
		},
		BuildContextFn: func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
			return &prompt.Context{}, nil
		},
		RenderBuildFn: func(ctx *prompt.Context) (string, error) {
			return "mock", nil
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "mock output"}, nil
		},
	})

	ctx := context.Background()
	deadline := time.Time{}

	// ACCEPTANCE CRITERION: This call should compile with scopeEstimate parameter
	// The current processBead signature doesn't accept scopeEstimate, so this will
	// fail to compile. The fix will add the parameter.
	result := h.runner.processBead(ctx, testBead, 1, deadline, estimate)

	// Basic verification that processBead ran
	if result == nil {
		t.Fatal("processBead returned nil result")
	}
	if result.BeadID != testBead.ID {
		t.Errorf("expected result.BeadID=%s, got %s", testBead.ID, result.BeadID)
	}
}
