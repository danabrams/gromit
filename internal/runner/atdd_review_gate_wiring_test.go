//go:build acceptance

package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
)

// TestProcessBead_ATDDReviewGate_PassVerdict verifies that when the review
// gate returns "pass", processing continues to verify-tests-fail without rewriting.
func TestProcessBead_ATDDReviewGate_PassVerdict(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet
	// Expected failure: The review gate is not wired into processBead() yet

	var buf strings.Builder
	var acceptanceTestsCalls int
	var reviewCalls int

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			acceptanceTestsCalls++
			return &claude.Result{Success: true, Output: "tests written"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			reviewCalls++
			// Review returns pass
			return &claude.Result{
				Success: true,
				Output:  "VERDICT: PASS",
			}, nil
		},
	}

	mockRunner := &mockRunnerForATDDGate{
		claudeClient:           mockClaude,
		validatePassesOnSecond: true,
	}

	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: true},
		Validation:  config.ValidationConfig{Enabled: true, Commands: []string{"go test"}},
	}
	cfg.SetDefaults()

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    mockRunner,
		Router:   newMockRouterFromClaudeClient(mockClaude),
		Renderer: mockRunner,
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	b := &bead.Bead{
		ID:              "atdd-review-1",
		Title:           "Add feature X",
		Description:     "Implement feature X",
		Priority:        1,
		ExpectedOutputs: []string{"Feature works"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	// Review should be called exactly once
	if reviewCalls != 1 {
		t.Errorf("expected 1 review call (pass verdict), got %d", reviewCalls)
	}

	// Acceptance tests should be called exactly once (no rewrite)
	if acceptanceTestsCalls != 1 {
		t.Errorf("expected 1 acceptance tests call (no rewrite), got %d", acceptanceTestsCalls)
	}

	// Expected failure: ATDDReviewVerdict field does not exist on IterationResult yet
	if result.ATDDReviewVerdict != "pass" {
		t.Errorf("expected ATDDReviewVerdict='pass', got %q", result.ATDDReviewVerdict)
	}

	// Expected failure: ATDDReviewRewrite field does not exist on IterationResult yet
	if result.ATDDReviewRewrite {
		t.Error("expected ATDDReviewRewrite=false (no rewrite on pass)")
	}
}

// TestProcessBead_ATDDReviewGate_FailVerdictRewritesTests verifies that when
// the review gate returns "fail", tests are rewritten once and reviewed again.
func TestProcessBead_ATDDReviewGate_FailVerdictRewritesTests(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet
	// Expected failure: The review gate is not wired into processBead() yet
	// Expected failure: Test rewrite on review fail is not implemented yet

	var buf strings.Builder
	var acceptanceTestsCalls int
	var reviewCalls int

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			acceptanceTestsCalls++
			return &claude.Result{Success: true, Output: "tests written"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			reviewCalls++
			if reviewCalls == 1 {
				// First review fails
				return &claude.Result{
					Success: true,
					Output:  "VERDICT: FAIL\n\nTests are weak",
				}, nil
			}
			// Second review passes
			return &claude.Result{
				Success: true,
				Output:  "VERDICT: PASS",
			}, nil
		},
	}

	mockRunner := &mockRunnerForATDDGate{
		claudeClient:           mockClaude,
		validatePassesOnSecond: true,
	}

	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: true},
		Validation:  config.ValidationConfig{Enabled: true, Commands: []string{"go test"}},
	}
	cfg.SetDefaults()

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    mockRunner,
		Router:   newMockRouterFromClaudeClient(mockClaude),
		Renderer: mockRunner,
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	b := &bead.Bead{
		ID:              "atdd-review-2",
		Title:           "Add validation",
		Description:     "Add input validation",
		Priority:        1,
		ExpectedOutputs: []string{"Invalid inputs rejected"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	// Review should be called twice (initial + after rewrite)
	if reviewCalls != 2 {
		t.Errorf("expected 2 review calls (fail then pass), got %d", reviewCalls)
	}

	// Acceptance tests should be called twice (initial + rewrite)
	if acceptanceTestsCalls != 2 {
		t.Errorf("expected 2 acceptance tests calls (initial + rewrite), got %d", acceptanceTestsCalls)
	}

	// Expected failure: ATDDReviewVerdict field does not exist on IterationResult yet
	// Should record the final verdict (pass after rewrite)
	if result.ATDDReviewVerdict != "pass" {
		t.Errorf("expected ATDDReviewVerdict='pass' (final verdict), got %q", result.ATDDReviewVerdict)
	}

	// Expected failure: ATDDReviewRewrite field does not exist on IterationResult yet
	if !result.ATDDReviewRewrite {
		t.Error("expected ATDDReviewRewrite=true (rewrite occurred)")
	}
}

// TestProcessBead_ATDDReviewGate_MaxOneRewriteCycle verifies that after one
// rewrite cycle, processing continues even if the second review fails.
func TestProcessBead_ATDDReviewGate_MaxOneRewriteCycle(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet
	// Expected failure: The review gate is not wired into processBead() yet
	// Expected failure: Max one rewrite enforcement is not implemented yet

	var buf strings.Builder
	var acceptanceTestsCalls int
	var reviewCalls int

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			acceptanceTestsCalls++
			return &claude.Result{Success: true, Output: "tests written"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			reviewCalls++
			// Both reviews fail
			return &claude.Result{
				Success: true,
				Output:  "VERDICT: FAIL\n\nTests are weak",
			}, nil
		},
	}

	mockRunner := &mockRunnerForATDDGate{
		claudeClient:           mockClaude,
		validatePassesOnSecond: true,
	}

	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: true},
		Validation:  config.ValidationConfig{Enabled: true, Commands: []string{"go test"}},
	}
	cfg.SetDefaults()

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    mockRunner,
		Router:   newMockRouterFromClaudeClient(mockClaude),
		Renderer: mockRunner,
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	b := &bead.Bead{
		ID:              "atdd-review-3",
		Title:           "Add metrics",
		Description:     "Add metrics collection",
		Priority:        1,
		ExpectedOutputs: []string{"Metrics collected"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if !result.Success {
		t.Fatalf("expected success despite double fail, got error: %v", result.Error)
	}

	// Review should be called twice (initial + after rewrite)
	if reviewCalls != 2 {
		t.Errorf("expected 2 review calls (both fail), got %d", reviewCalls)
	}

	// Acceptance tests should be called twice (initial + one rewrite)
	if acceptanceTestsCalls != 2 {
		t.Errorf("expected 2 acceptance tests calls (initial + one rewrite), got %d", acceptanceTestsCalls)
	}

	// Should continue to verify-tests-fail after second review fail
	// (no third rewrite attempt)

	// Expected failure: ATDDReviewVerdict field does not exist on IterationResult yet
	if result.ATDDReviewVerdict != "fail" {
		t.Errorf("expected ATDDReviewVerdict='fail' (final verdict), got %q", result.ATDDReviewVerdict)
	}

	// Expected failure: ATDDReviewRewrite field does not exist on IterationResult yet
	if !result.ATDDReviewRewrite {
		t.Error("expected ATDDReviewRewrite=true (one rewrite occurred)")
	}
}

// TestProcessBead_ATDDReviewGate_InjectsFailureContextOnRewrite verifies that
// when the review fails, the review feedback is injected into FailureContext
// before rewriting acceptance tests.
func TestProcessBead_ATDDReviewGate_InjectsFailureContextOnRewrite(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet
	// Expected failure: Failure context injection on review fail is not implemented yet

	var buf strings.Builder
	var capturedPrompts []string

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output any, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			capturedPrompts = append(capturedPrompts, prompt)
			return &claude.Result{Success: true, Output: "tests written"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output:  "VERDICT: FAIL\n\nTest 1 checks existing behavior, not new behavior",
			}, nil
		},
	}

	mockRunner := &mockRunnerForATDDGate{
		claudeClient:           mockClaude,
		validatePassesOnSecond: true,
	}

	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: true},
		Validation:  config.ValidationConfig{Enabled: true, Commands: []string{"go test"}},
	}
	cfg.SetDefaults()

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    mockRunner,
		Router:   newMockRouterFromClaudeClient(mockClaude),
		Renderer: mockRunner,
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	b := &bead.Bead{
		ID:              "atdd-review-4",
		Title:           "Add caching",
		Description:     "Add response caching",
		Priority:        1,
		ExpectedOutputs: []string{"Responses cached"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	// Should have captured at least 2 prompts (initial + rewrite)
	if len(capturedPrompts) < 2 {
		t.Fatalf("expected at least 2 acceptance test prompts, got %d", len(capturedPrompts))
	}

	// The second prompt (rewrite) should contain the review feedback
	rewritePrompt := capturedPrompts[1]
	if !strings.Contains(rewritePrompt, "Test 1 checks existing behavior") {
		t.Errorf("expected rewrite prompt to contain review feedback, got:\n%s", rewritePrompt)
	}
}

// TestProcessBead_ATDDReviewGate_OnlyRunsWhenATDDActive verifies that the
// review gate does not run when ATDD is not active.
func TestProcessBead_ATDDReviewGate_OnlyRunsWhenATDDActive(t *testing.T) {
	// Expected failure: reviewAcceptanceTests method does not exist on Runner yet
	// Expected failure: Review gate conditional logic is not implemented yet

	var buf strings.Builder
	var reviewCalls int

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output any, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			reviewCalls++
			return &claude.Result{Success: true, Output: "VERDICT: PASS"}, nil
		},
	}

	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: false}, // ATDD disabled
		Validation:  config.ValidationConfig{Enabled: true, Commands: []string{"go test"}},
	}
	cfg.SetDefaults()

	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   newMockRouterFromClaudeClient(mockClaude),
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	b := &bead.Bead{
		ID:              "non-atdd-1",
		Title:           "Regular task",
		Description:     "Non-ATDD task",
		Priority:        1,
		ExpectedOutputs: []string{"Task done"},
	}

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	// Review should not be called when ATDD is disabled
	if reviewCalls != 0 {
		t.Errorf("expected 0 review calls (ATDD disabled), got %d", reviewCalls)
	}

	// Expected failure: ATDDReviewVerdict field does not exist on IterationResult yet
	if result.ATDDReviewVerdict != "" {
		t.Errorf("expected ATDDReviewVerdict='' (ATDD not active), got %q", result.ATDDReviewVerdict)
	}
}

// mockRunnerForATDDGate is a combined mock that implements both BeadClient and
// PromptRenderer interfaces for ATDD review gate tests.
type mockRunnerForATDDGate struct {
	claudeClient           *mockClaudeClient
	validatePassesOnSecond bool
	validateCallCount      int
}

func (m *mockRunnerForATDDGate) Ready() (*bead.Bead, error) {
	return nil, nil
}

func (m *mockRunnerForATDDGate) ReadyWithLabel(label string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockRunnerForATDDGate) ListWithLabel(label string) ([]*bead.Bead, error) {
	return nil, nil
}

func (m *mockRunnerForATDDGate) Show(id string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockRunnerForATDDGate) Close(id string) error {
	return nil
}

func (m *mockRunnerForATDDGate) Sync() error {
	return nil
}

func (m *mockRunnerForATDDGate) AddComment(id, comment string) error {
	return nil
}

func (m *mockRunnerForATDDGate) GetParent(b *bead.Bead) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockRunnerForATDDGate) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockRunnerForATDDGate) CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockRunnerForATDDGate) HasOpenChildren(parentID string) (bool, error) {
	return false, nil
}

// PromptRenderer methods - minimal implementations
func (m *mockRunnerForATDDGate) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
	return &prompt.Context{WorkDir: "."}, nil
}

func (m *mockRunnerForATDDGate) RenderBuild(ctx *prompt.Context) (string, error) {
	return "build prompt", nil
}

func (m *mockRunnerForATDDGate) RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error) {
	return "analyze prompt", nil
}

func (m *mockRunnerForATDDGate) RenderLearn(ctx *prompt.LearnContext) (string, error) {
	return "learn prompt", nil
}

func (m *mockRunnerForATDDGate) RenderDecompose(ctx *prompt.DecomposeContext) (string, error) {
	return "decompose prompt", nil
}

func (m *mockRunnerForATDDGate) RenderScope(ctx *prompt.ScopeContext) (string, error) {
	return "scope prompt", nil
}

func (m *mockRunnerForATDDGate) RenderPrecheck(ctx *prompt.PrecheckContext) (string, error) {
	return "precheck prompt", nil
}

func (m *mockRunnerForATDDGate) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	return "review prompt", nil
}

func (m *mockRunnerForATDDGate) RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error) {
	return "thorough review prompt", nil
}

func (m *mockRunnerForATDDGate) RenderAcceptanceTests(ctx *prompt.Context) (string, error) {
	return "acceptance tests prompt", nil
}

func (m *mockRunnerForATDDGate) RenderATDDBuild(ctx *prompt.Context) (string, error) {
	return "atdd build prompt", nil
}

func (m *mockRunnerForATDDGate) RenderTDDBuild(ctx *prompt.Context) (string, error) {
	return "tdd build prompt", nil
}

func (m *mockRunnerForATDDGate) RenderRefactor(ctx *prompt.Context) (string, error) {
	return "refactor prompt", nil
}

func (m *mockRunnerForATDDGate) LoadSpec(name string) (string, error) {
	return "", nil
}

func (m *mockRunnerForATDDGate) LoadClaudeMD() (string, error) {
	return "", nil
}

func (m *mockRunnerForATDDGate) LoadRules() (string, error) {
	return "", nil
}

func (m *mockRunnerForATDDGate) LoadRulesForPhase(phase string) (string, error) {
	return "", nil
}

func (m *mockRunnerForATDDGate) GetLearningsFile() *learnings.File {
	return nil
}
