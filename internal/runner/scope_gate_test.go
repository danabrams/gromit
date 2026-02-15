package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
)

// scopeGateTestSetup creates a runner with standard test configuration for scope gate tests.
// The returned mockBeadClient and mockIterationLogger can be inspected after the test.
func scopeGateTestSetup(t *testing.T, cfg *config.Config, scopeEstimate *prompt.ScopeEstimate) (*Runner, *mockBeadClient, *mockIterationLogger) {
	t.Helper()

	callCount := 0
	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount == 1 {
				return &bead.Bead{
					ID:              "test-1",
					Title:           "Test bead",
					Priority:        1,
					Labels:          []string{},
					ExpectedOutputs: []string{},
				}, nil
			}
			return nil, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	// Build scope estimate JSON for mock claude response
	var scopeJSON string
	if scopeEstimate != nil {
		data, err := json.Marshal(scopeEstimate)
		if err != nil {
			t.Fatalf("failed to marshal scope estimate: %v", err)
		}
		scopeJSON = string(data)
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: scopeJSON}, nil
		},
	}

	mockLogger := &mockIterationLogger{}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{
			Beads:    mockBeads,
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   mockLogger,
		})
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	return r, mockBeads, mockLogger
}

func baseScopeGateConfig() *config.Config {
	precheckDisabled := false
	autoPushDisabled := false
	return &config.Config{
		ScopeCheck: config.ScopeCheckConfig{
			Enabled: true,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}
}

func TestScopeGate_BlocksCannotCompleteInSingleIteration(t *testing.T) {
	cfg := baseScopeGateConfig()
	estimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          3,
		CanCompleteInSingleIteration: false,
		Blockers:                     []string{},
	}

	r, mockBeads, mockLogger := scopeGateTestSetup(t, cfg, estimate)

	err := r.Run(context.Background(), 1, time.Time{}, nil, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify bead was not closed (blocked, not processed)
	if len(mockBeads.ClosedIDs) > 0 {
		t.Errorf("expected no beads closed, got %v", mockBeads.ClosedIDs)
	}

	// Verify scope_blocked outcome was logged
	found := false
	for _, log := range mockLogger.Logs {
		if log.Outcome == "scope_blocked" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected scope_blocked outcome in logs")
	}
}

func TestScopeGate_BlocksHighComplexityWithBlockers(t *testing.T) {
	cfg := baseScopeGateConfig()
	estimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{"needs API design decision"},
	}

	r, mockBeads, mockLogger := scopeGateTestSetup(t, cfg, estimate)

	err := r.Run(context.Background(), 1, time.Time{}, nil, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(mockBeads.ClosedIDs) > 0 {
		t.Errorf("expected no beads closed, got %v", mockBeads.ClosedIDs)
	}

	found := false
	for _, log := range mockLogger.Logs {
		if log.Outcome == "scope_blocked" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected scope_blocked outcome in logs")
	}
}

func TestScopeGate_PassesMediumComplexitySingleIteration(t *testing.T) {
	cfg := baseScopeGateConfig()
	estimate := &prompt.ScopeEstimate{
		Complexity:                   "medium",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}

	r, _, mockLogger := scopeGateTestSetup(t, cfg, estimate)

	// Use dry-run to avoid needing the full processBead flow
	err := r.Run(context.Background(), 1, time.Time{}, nil, true)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify no scope_blocked outcome (bead passed the gate)
	for _, log := range mockLogger.Logs {
		if log.Outcome == "scope_blocked" {
			t.Error("expected no scope_blocked outcome for passing bead")
		}
	}
}

func TestScopeGate_PassesHighComplexityNoBlockers(t *testing.T) {
	cfg := baseScopeGateConfig()
	estimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          1,
		CanCompleteInSingleIteration: true,
		Blockers:                     []string{},
	}

	r, _, mockLogger := scopeGateTestSetup(t, cfg, estimate)

	err := r.Run(context.Background(), 1, time.Time{}, nil, true)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, log := range mockLogger.Logs {
		if log.Outcome == "scope_blocked" {
			t.Error("expected no scope_blocked outcome for high complexity with no blockers")
		}
	}
}

func TestScopeGate_SkippedWhenBlockOversizedFalse(t *testing.T) {
	cfg := baseScopeGateConfig()
	blockOversized := false
	cfg.ScopeCheck.BlockOversized = &blockOversized

	estimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          5,
		CanCompleteInSingleIteration: false,
		Blockers:                     []string{"too big"},
	}

	r, _, mockLogger := scopeGateTestSetup(t, cfg, estimate)

	err := r.Run(context.Background(), 1, time.Time{}, nil, true)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Scope gate should not block when block_oversized is false
	for _, log := range mockLogger.Logs {
		if log.Outcome == "scope_blocked" {
			t.Error("expected no scope_blocked outcome when block_oversized=false")
		}
	}
}

func TestScopeGate_SkippedWhenScopeCheckDisabled(t *testing.T) {
	cfg := baseScopeGateConfig()
	cfg.ScopeCheck.Enabled = false

	estimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          5,
		CanCompleteInSingleIteration: false,
		Blockers:                     []string{"too big"},
	}

	r, _, mockLogger := scopeGateTestSetup(t, cfg, estimate)

	err := r.Run(context.Background(), 1, time.Time{}, nil, true)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, log := range mockLogger.Logs {
		if log.Outcome == "scope_blocked" {
			t.Error("expected no scope_blocked outcome when scope check is disabled")
		}
	}
}

func TestScopeGate_CommentAddedToBlockedBead(t *testing.T) {
	cfg := baseScopeGateConfig()
	estimate := &prompt.ScopeEstimate{
		Complexity:                   "high",
		EstimatedIterations:          3,
		CanCompleteInSingleIteration: false,
		Blockers:                     []string{},
	}

	r, mockBeads, _ := scopeGateTestSetup(t, cfg, estimate)

	err := r.Run(context.Background(), 1, time.Time{}, nil, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify a comment was added to the blocked bead
	if len(mockBeads.Comments) == 0 {
		t.Fatal("expected comment on blocked bead, got none")
	}

	comment := mockBeads.Comments[0]
	if comment.ID != "test-1" {
		t.Errorf("expected comment on bead test-1, got %s", comment.ID)
	}
	if !strings.Contains(comment.Comment, "scope gate") {
		t.Errorf("expected comment to mention scope gate, got %q", comment.Comment)
	}
	if !strings.Contains(comment.Comment, "decompose") {
		t.Errorf("expected comment to suggest decomposition, got %q", comment.Comment)
	}
}
