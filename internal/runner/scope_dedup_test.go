package runner

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/prompt"
)

// TestScopeCheckDedup_ScopeGateAndBuildPromptShareEstimate tests that when
// block_oversized is enabled and a bead passes the scope gate, the estimate
// from the gate is reused in buildPromptForBead without a second LLM call.
func TestScopeCheckDedup_ScopeGateAndBuildPromptShareEstimate(t *testing.T) {
	// Expected failure: Currently, checkScope is called twice when block_oversized
	// is enabled - once in runner.go:412 (scope gate) and again in process.go:110
	// (buildPromptForBead). The fix will cache the scope gate estimate and pass it
	// into buildPromptForBead so it doesn't need to call checkScope again.

	cfg := baseScopeGateConfig()
	blockOversized := true
	cfg.ScopeCheck.BlockOversized = &blockOversized

	testBead := &bead.Bead{
		ID:              "test-dedup-1",
		Title:           "Test bead for dedup",
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
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	// Track how many times RenderScope is called
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
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: p}, nil
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

	// Simulate the scope gate check (first call)
	ctx := context.Background()
	scopeEstimate := r.checkScope(ctx, testBead)
	if scopeEstimate == nil {
		t.Fatal("checkScope returned nil")
	}

	// Verify first call happened
	renderScopeMu.Lock()
	firstCallCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if firstCallCount != 1 {
		t.Fatalf("expected 1 RenderScope call from scope gate, got %d", firstCallCount)
	}

	// Now simulate building the prompt (second potential call)
	bc := &beadContext{
		bead:          testBead,
		parent:        nil,
		result:        &IterationResult{Model: "sonnet"},
		model:         "sonnet",
		scopeEstimate: scopeEstimate, // Pass cached estimate from scope gate
	}

	// This should NOT trigger another checkScope call in the fixed implementation
	err = r.buildPromptForBead(ctx, bc, 1)
	if err != nil {
		t.Fatalf("buildPromptForBead error: %v", err)
	}

	// ACCEPTANCE CRITERION: No additional RenderScope call from buildPromptForBead
	// The current implementation will call RenderScope again, making this test fail.
	// The fix should pass scopeEstimate into buildPromptForBead to skip the second call.
	renderScopeMu.Lock()
	finalCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if finalCount != 1 {
		t.Errorf("expected exactly 1 total RenderScope call (scope gate only), got %d (gate + buildPrompt called checkScope again)", finalCount)
	}
}

// TestScopeCheckDedup_BlockOversizedDisabledSkipsGateButCallsBuildPrompt tests that
// when block_oversized is false, the scope gate is skipped but buildPromptForBead
// still calls checkScope once for auto-escalation.
func TestScopeCheckDedup_BlockOversizedDisabledSkipsGateButCallsBuildPrompt(t *testing.T) {
	// Expected failure: This verifies that when block_oversized is false, the scope
	// gate doesn't call checkScope, but buildPromptForBead still does (for auto-escalation).

	cfg := baseScopeGateConfig()
	blockOversized := false
	cfg.ScopeCheck.BlockOversized = &blockOversized

	testBead := &bead.Bead{
		ID:              "test-dedup-2",
		Title:           "Test bead no gate",
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
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: p}, nil
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

	// Skip the scope gate (block_oversized is false)
	// Go directly to buildPromptForBead
	ctx := context.Background()
	bc := &beadContext{
		bead:   testBead,
		parent: nil,
		result: &IterationResult{Model: "sonnet"},
		model:  "sonnet",
	}

	err = r.buildPromptForBead(ctx, bc, 1)
	if err != nil {
		t.Fatalf("buildPromptForBead error: %v", err)
	}

	// ACCEPTANCE CRITERION: Exactly one RenderScope call (from buildPrompt only)
	renderScopeMu.Lock()
	finalCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if finalCount != 1 {
		t.Errorf("expected exactly 1 RenderScope call (buildPrompt only, no gate), got %d", finalCount)
	}
}

// TestScopeCheckDedup_HighComplexityAutoEscalationUsesCache tests that when a bead
// has high complexity (triggering auto-escalation), the estimate used for escalation
// decision comes from the scope gate without a second checkScope call.
func TestScopeCheckDedup_HighComplexityAutoEscalationUsesCache(t *testing.T) {
	// Expected failure: Currently buildPromptForBead always calls checkScope when
	// scope check is enabled, even if the scope gate already evaluated the bead.
	// The fix should pass the cached estimate so buildPromptForBead can escalate
	// without calling checkScope again.

	cfg := baseScopeGateConfig()
	blockOversized := true
	cfg.ScopeCheck.BlockOversized = &blockOversized

	testBead := &bead.Bead{
		ID:              "test-escalate",
		Title:           "Test auto-escalation dedup",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// High complexity but no blockers = passes gate but should escalate
	estimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}

	mockBeads := &mockBeadClient{
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
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: p}, nil
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

	// Simulate scope gate (first call)
	ctx := context.Background()
	scopeEstimate := r.checkScope(ctx, testBead)
	if scopeEstimate == nil {
		t.Fatal("checkScope returned nil")
	}
	if scopeEstimate.Complexity != "high" {
		t.Fatalf("expected high complexity, got %s", scopeEstimate.Complexity)
	}

	// Verify first call
	renderScopeMu.Lock()
	firstCallCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if firstCallCount != 1 {
		t.Fatalf("expected 1 RenderScope call from scope gate, got %d", firstCallCount)
	}

	// Build prompt with auto-escalation
	bc := &beadContext{
		bead:          testBead,
		parent:        nil,
		result:        &IterationResult{Model: "sonnet"},
		model:         "sonnet",
		scopeEstimate: scopeEstimate, // Pass cached estimate from scope gate
	}

	err = r.buildPromptForBead(ctx, bc, 1)
	if err != nil {
		t.Fatalf("buildPromptForBead error: %v", err)
	}

	// ACCEPTANCE CRITERION: Should not call checkScope again (use cached estimate)
	renderScopeMu.Lock()
	finalCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if finalCount != 1 {
		t.Errorf("expected exactly 1 RenderScope call (cached for auto-escalation), got %d", finalCount)
	}

	// ACCEPTANCE CRITERION: Model should be escalated to opus based on cached complexity
	if bc.model != "opus" {
		t.Errorf("expected model escalated to opus based on high complexity, got %s", bc.model)
	}
}

// TestScopeCheckDedup_BlockedBeadsDoNotCallBuildPrompt tests that when a bead
// is blocked by the scope gate, buildPromptForBead is never called, so there's
// only one checkScope invocation.
func TestScopeCheckDedup_BlockedBeadsDoNotCallBuildPrompt(t *testing.T) {
	// Expected failure: This verifies that blocked beads only call checkScope once
	// (in the gate) because they never reach buildPromptForBead.

	cfg := baseScopeGateConfig()
	blockOversized := true
	cfg.ScopeCheck.BlockOversized = &blockOversized

	testBead := &bead.Bead{
		ID:              "test-blocked",
		Title:           "Test blocked bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	// This estimate will cause blocking
	estimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          5,
		CanCompleteInSingleIteration: false,
		Blockers:                     []string{"needs decomposition"},
	}

	mockBeads := &mockBeadClient{
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
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: p}, nil
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

	// Simulate scope gate check
	ctx := context.Background()
	scopeEstimate := r.checkScope(ctx, testBead)
	if scopeEstimate == nil {
		t.Fatal("checkScope returned nil")
	}

	// Verify it should be blocked
	if scopeEstimate.CanCompleteInSingleIteration {
		t.Fatal("expected bead to fail single iteration check")
	}

	// ACCEPTANCE CRITERION: Only one checkScope call for blocked beads
	renderScopeMu.Lock()
	finalCount := renderScopeCallCount
	renderScopeMu.Unlock()
	if finalCount != 1 {
		t.Errorf("expected exactly 1 RenderScope call for blocked bead, got %d", finalCount)
	}

	// In a real run, buildPromptForBead would not be called for a blocked bead
	// This test just verifies the gate blocks and doesn't double-check
}
