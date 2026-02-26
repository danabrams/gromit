package escalation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Mock types for narrow interfaces ---

// mockFailureAnalyzer implements the FailureAnalyzer interface for escalation tests.
type mockFailureAnalyzer struct {
	analyzeFn    func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error)
	analyzeCalls int
}

func (m *mockFailureAnalyzer) Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
	m.analyzeCalls++
	if m.analyzeFn != nil {
		return m.analyzeFn(ctx, b, failureOutput)
	}
	return &analyzer.Analysis{Category: "logic_error", Recoverable: false, RootCause: "test"}, nil
}

// mockBeadClient implements the BeadClient interface for escalation tests.
type mockBeadClient struct {
	addCommentFn func(id, comment string) error
	comments     []mockComment
}

type mockComment struct {
	id, comment string
}

func (m *mockBeadClient) AddComment(id, comment string) error {
	m.comments = append(m.comments, mockComment{id, comment})
	if m.addCommentFn != nil {
		return m.addCommentFn(id, comment)
	}
	return nil
}

// --- Helper functions ---

func newTestBeadContext() *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead:              &bead.Bead{ID: "test-001", Title: "Test bead", Description: "Test description", Priority: 1},
		Tier:              provider.TierMedium,
		Model:             "sonnet",
		BuildPrompt:       "test prompt",
		Result:            &runtypes.IterationResult{},
		MaxRetries:        2,
		MaxRetriesPerBead: 5,
	}
}

func newTestConfig() *config.Config {
	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			Enabled:            true,
			Chain:              []string{"haiku", "sonnet", "opus"},
			MaxRetriesPerModel: 2,
			MaxRetriesPerBead:  5,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return cfg
}

// --- Handler tests ---

func TestNewHandler_AcceptsNarrowInterfaces(t *testing.T) {
	// Verify that NewHandler constructs a Handler from narrow dependency interfaces,
	// not from concrete runner types. This is the key extraction pattern.
	cfg := newTestConfig()
	mfa := &mockFailureAnalyzer{}
	mbc := &mockBeadClient{}

	// NewHandler should accept config, analyzer, bead client, and callback functions
	h := NewHandler(cfg, mfa, mbc, nil, nil, nil, nil)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
}

func TestHandleStallTimeout_ExceedsBeadLimit(t *testing.T) {
	// When TotalRetriesThisBead exceeds MaxRetriesPerBead, HandleStallTimeout
	// should return false and set an error on the result.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.TotalRetriesThisBead = 6
	bc.MaxRetriesPerBead = 5

	continueLoop := h.HandleStallTimeout(context.Background(), bc)
	if continueLoop {
		t.Error("HandleStallTimeout should return false when bead retry limit exceeded")
	}
	if bc.Result.Error == nil {
		t.Fatal("HandleStallTimeout should set bc.Result.Error when bead retry limit exceeded")
	}
	if got := bc.Result.Error.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestHandleStallTimeout_RetryWithSameModel(t *testing.T) {
	// When retries are available for the current model, HandleStallTimeout
	// should return true and increment retry counters.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Result.ToolCallCount = 0 // no activity => one same-tier retry allowed
	bc.RetriesThisModel = 0
	bc.TotalRetriesThisBead = 0
	bc.MaxRetries = 2

	continueLoop := h.HandleStallTimeout(context.Background(), bc)
	if !continueLoop {
		t.Error("HandleStallTimeout should return true when retries available")
	}
	if bc.RetriesThisModel != 1 {
		t.Errorf("RetriesThisModel = %d, want 1", bc.RetriesThisModel)
	}
	if bc.TotalRetriesThisBead != 1 {
		t.Errorf("TotalRetriesThisBead = %d, want 1", bc.TotalRetriesThisBead)
	}
}

func TestHandleStallTimeout_EscalatesToNextTier(t *testing.T) {
	// When model retries are exhausted and a higher tier exists,
	// HandleStallTimeout should escalate the tier.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"
	bc.Result.ToolCallCount = 1 // tool activity => no same-tier retry
	bc.RetriesThisModel = 3     // exceeds MaxRetries=2
	bc.MaxRetries = 2
	bc.TotalRetriesThisBead = 3

	continueLoop := h.HandleStallTimeout(context.Background(), bc)
	if !continueLoop {
		t.Error("HandleStallTimeout should return true when escalation is available")
	}
	if bc.Tier != provider.TierMedium {
		t.Errorf("Tier = %q, want %q after escalation", bc.Tier, provider.TierMedium)
	}
	if !bc.Result.Escalated {
		t.Error("Result.Escalated should be true after tier escalation")
	}
	if bc.RetriesThisModel != 0 {
		t.Errorf("RetriesThisModel = %d, want 0 after escalation", bc.RetriesThisModel)
	}
}

func TestHandleStallTimeout_OnlyOneNoToolRetry(t *testing.T) {
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"
	bc.Result.ToolCallCount = 0

	if !h.HandleStallTimeout(context.Background(), bc) {
		t.Fatal("expected first no-tool stall to retry once")
	}
	if bc.StallRetryWithoutToolUsed != true {
		t.Fatal("expected StallRetryWithoutToolUsed=true after first no-tool stall")
	}
	if bc.Tier != provider.TierLow {
		t.Fatalf("tier changed too early: got %s", bc.Tier)
	}

	// Second no-tool stall should consume timeout escalation (no second same-tier retry).
	if !h.HandleStallTimeout(context.Background(), bc) {
		t.Fatal("expected second no-tool stall to escalate once")
	}
	if bc.Tier != provider.TierMedium {
		t.Fatalf("expected escalation to %s, got %s", provider.TierMedium, bc.Tier)
	}
}

func TestHandleStallTimeout_FirstTimeoutDecomposesBeforeEscalation(t *testing.T) {
	cfg := newTestConfig()
	decomposeCalled := false
	createSubCalled := false
	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return []runtypes.SubTask{{Title: "split"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
			createSubCalled = true
			return nil
		},
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.ParentCtx = context.Background()
	bc.Result.ToolCallCount = 1
	bc.RetriesThisModel = bc.MaxRetries + 1

	if h.HandleStallTimeout(context.Background(), bc) {
		t.Fatal("expected first timeout to stop for decomposition before escalation")
	}
	if !decomposeCalled {
		t.Fatal("expected decomposition to run on first timeout")
	}
	if !createSubCalled {
		t.Fatal("expected create sub-beads to be invoked after decomposition")
	}
	if bc.Result.Escalated {
		t.Fatal("did not expect escalation before decomposition on first timeout")
	}
	if bc.TimeoutEscalationsThisBead != 0 {
		t.Fatalf("expected no timeout escalation count after decomposition, got %d", bc.TimeoutEscalationsThisBead)
	}
}

func TestHandleInvocationTimeout_WithoutDecomposerReturnsError(t *testing.T) {
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"

	if h.HandleInvocationTimeout(context.Background(), bc) {
		t.Fatal("expected invocation timeout to stop when decomposition is unavailable")
	}
	if bc.Result.TimeoutDecompositionAttempted != true {
		t.Fatal("expected timeout decomposition attempt marker to be set")
	}
	if bc.Result.TimeoutDecompositionSucceeded {
		t.Fatal("did not expect timeout decomposition success when decomposer is unavailable")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error when decomposition is unavailable")
	}
}

func TestAnalyzeAndHandleFailure_UnclearSpecStops(t *testing.T) {
	// When failure analysis returns CategoryUnclearSpec, the handler should
	// stop retrying (return false) and set a spec-related error.
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryUnclearSpec,
				Recoverable: false,
				RootCause:   "ambiguous acceptance criteria",
			}, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	claudeResult := &provider.Result{Output: "build failed output"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if continueLoop {
		t.Error("AnalyzeAndHandleFailure should return false for unclear spec")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error to be set for unclear spec")
	}
	if bc.Result.FailureCategory != string(analyzer.CategoryUnclearSpec) {
		t.Fatalf("FailureCategory = %q, want %q", bc.Result.FailureCategory, analyzer.CategoryUnclearSpec)
	}
}

func TestAnalyzeAndHandleFailure_TaskTooComplexStopsAndComments(t *testing.T) {
	// When analysis returns CategoryTaskTooComplex, the handler should stop,
	// add a comment to the bead, and set an error.
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryTaskTooComplex,
				Recoverable: false,
				RootCause:   "touches 12 files across 6 packages",
			}, nil
		},
	}
	mbc := &mockBeadClient{}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, mbc, nil, nil, nil, nil)

	bc := newTestBeadContext()
	claudeResult := &provider.Result{Output: "build output"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if continueLoop {
		t.Error("AnalyzeAndHandleFailure should return false for task too complex")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error to be set for task too complex")
	}
	if len(mbc.comments) == 0 {
		t.Error("expected a comment to be added to the bead for task too complex")
	}
	if bc.Result.FailureCategory != string(analyzer.CategoryTaskTooComplex) {
		t.Fatalf("FailureCategory = %q, want %q", bc.Result.FailureCategory, analyzer.CategoryTaskTooComplex)
	}
}

func TestAnalyzeAndHandleFailure_RecoverableRetries(t *testing.T) {
	// When analysis returns a recoverable failure with retries available,
	// the handler should return true (continue) and increment retry counters.
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			suggestion := "try adding the missing import"
			return &analyzer.Analysis{
				Category:    "logic_error",
				Recoverable: true,
				RootCause:   "missing import statement",
				Suggestion:  suggestion,
			}, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.RetriesThisModel = 0
	bc.MaxRetries = 2
	claudeResult := &provider.Result{Output: "compilation error"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if !continueLoop {
		t.Error("AnalyzeAndHandleFailure should return true for recoverable failure with retries available")
	}
	if bc.RetriesThisModel != 1 {
		t.Errorf("RetriesThisModel = %d, want 1 after recoverable retry", bc.RetriesThisModel)
	}
	if bc.TotalRetriesThisBead != 1 {
		t.Errorf("TotalRetriesThisBead = %d, want 1 after recoverable retry", bc.TotalRetriesThisBead)
	}
}

func TestAnalyzeAndHandleFailure_RecoverableRetrySetsPromptContext(t *testing.T) {
	// Expected failure: AnalyzeAndHandleFailure does not set bc.PromptCtx.IsRetry
	// or bc.PromptCtx.FailureContext when deciding to retry a recoverable failure.
	// The old runner-local method set these fields but the escalation extraction
	// lost that behavior. The fix must set IsRetry=true and FailureContext to the
	// analysis Suggestion so the retry prompt includes failure context.
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategorySyntax,
				Recoverable: true,
				RootCause:   "missing import",
				Suggestion:  "Add the missing import statement",
			}, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.PromptCtx = &prompt.Context{
		Model:              "sonnet",
		ConfirmedLearnings: []learnings.Learning{},
		RecentLearnings:    []learnings.Learning{},
	}
	bc.RetriesThisModel = 0
	bc.MaxRetries = 2
	claudeResult := &provider.Result{Success: false, Output: "compile error: missing import"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if !continueLoop {
		t.Fatal("expected continueLoop=true for recoverable failure")
	}

	// The handler must set prompt context fields so the retry prompt includes failure context.
	if !bc.PromptCtx.IsRetry {
		t.Error("expected IsRetry=true after recoverable failure; handler must set bc.PromptCtx.IsRetry")
	}
	if bc.PromptCtx.FailureContext != "Add the missing import statement" {
		t.Errorf("expected FailureContext=%q, got %q; handler must set bc.PromptCtx.FailureContext to analysis.Suggestion",
			"Add the missing import statement", bc.PromptCtx.FailureContext)
	}
}

func TestAnalyzeAndHandleFailure_RecoverableRetryTruncatesFailureContextTail(t *testing.T) {
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategorySyntax,
				Recoverable: true,
				RootCause:   "large compiler output",
				Suggestion:  "0123456789012345678901234567890123456789",
			}, nil
		},
	}
	cfg := newTestConfig()
	cfg.Claude.MaxFailureContextChars = 24
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.PromptCtx = &prompt.Context{
		Model:              "sonnet",
		ConfirmedLearnings: []learnings.Learning{},
		RecentLearnings:    []learnings.Learning{},
	}
	claudeResult := &provider.Result{Success: false, Output: "compile error"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if !continueLoop {
		t.Fatal("expected continueLoop=true for recoverable failure")
	}

	const prefix = "[truncated] "
	if !strings.HasPrefix(bc.PromptCtx.FailureContext, prefix) {
		t.Fatalf("expected FailureContext to start with %q, got %q", prefix, bc.PromptCtx.FailureContext)
	}
	if len(bc.PromptCtx.FailureContext) != cfg.Claude.MaxFailureContextChars {
		t.Fatalf("expected FailureContext length=%d, got %d", cfg.Claude.MaxFailureContextChars, len(bc.PromptCtx.FailureContext))
	}
	if !strings.HasSuffix(bc.PromptCtx.FailureContext, "890123456789") {
		t.Fatalf("expected tail-preserving truncation, got %q", bc.PromptCtx.FailureContext)
	}
}

func TestAnalyzeAndHandleFailure_AnalysisErrorRetriesCommonCauseFirst(t *testing.T) {
	// Analyzer outages are treated as common-cause by default, so the first
	// failure should retry at the same tier before escalation.
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return nil, fmt.Errorf("analysis service unavailable")
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"
	claudeResult := &provider.Result{Output: "failed build output"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if !continueLoop {
		t.Error("AnalyzeAndHandleFailure should retry when analysis fails on first common-cause failure")
	}
	if bc.Tier != provider.TierLow {
		t.Errorf("Tier = %q, want %q for same-tier retry", bc.Tier, provider.TierLow)
	}
	if bc.RetriesThisModel != 1 {
		t.Errorf("RetriesThisModel = %d, want 1", bc.RetriesThisModel)
	}
}

func TestAnalyzeAndHandleFailure_NonRecoverableCommonCauseRetriesSameTier(t *testing.T) {
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryLogic,
				Recoverable: false,
				RootCause:   "unexpected edge case",
				Suggestion:  "tighten parser branch",
			}, nil
		},
	}
	cfg := newTestConfig()
	cfg.Paths.GromitDir = t.TempDir()
	writeModelBuildFailureLimit(t, cfg.Paths.GromitDir, "sonnet", 0.20, 0.00, 0.40)
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierMedium
	bc.Model = "sonnet"
	claudeResult := &provider.Result{Output: "failed build output"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if !continueLoop {
		t.Fatal("AnalyzeAndHandleFailure should retry same tier for common-cause non-recoverable failure")
	}
	if bc.Tier != provider.TierMedium {
		t.Fatalf("Tier = %q, want %q (no escalation on common-cause)", bc.Tier, provider.TierMedium)
	}
	if bc.RetriesThisModel != 1 {
		t.Fatalf("RetriesThisModel = %d, want 1", bc.RetriesThisModel)
	}
}

func TestAnalyzeAndHandleFailure_NonRecoverableConsecutiveFailureEscalates(t *testing.T) {
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryLogic,
				Recoverable: false,
				RootCause:   "edge case still failing",
			}, nil
		},
	}
	cfg := newTestConfig()
	cfg.Paths.GromitDir = t.TempDir()
	writeModelBuildFailureLimit(t, cfg.Paths.GromitDir, "sonnet", 0.20, 0.00, 0.40)
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierMedium
	bc.Model = "sonnet"
	bc.RetriesThisModel = 1 // consecutive failure on same bead => special cause
	claudeResult := &provider.Result{Output: "failed again"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if !continueLoop {
		t.Fatal("AnalyzeAndHandleFailure should escalate for consecutive non-recoverable failure")
	}
	if bc.Tier != provider.TierHigh {
		t.Fatalf("Tier = %q, want %q after special-cause escalation", bc.Tier, provider.TierHigh)
	}
}

func writeModelBuildFailureLimit(t *testing.T, gromitDir, model string, latest, lcl, ucl float64) {
	t.Helper()
	metricsDir := filepath.Join(gromitDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	trend := logger.ProcessTrend{
		StratifiedControlLimits: map[string][]logger.TrendControlLimit{
			"model:" + model: {
				{Metric: "rolling_build_failure_rate", Latest: latest, LCL: lcl, UCL: ucl},
			},
		},
	}
	data, err := json.Marshal(trend)
	if err != nil {
		t.Fatalf("Marshal process trend: %v", err)
	}
	path := filepath.Join(metricsDir, "process_trend.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile process trend: %v", err)
	}
}

func TestHandleEscalation_EscalatesToNextTier(t *testing.T) {
	// When a higher tier is available, HandleEscalation should escalate
	// and return true to continue the loop.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"
	claudeResult := &provider.Result{Output: "build failed"}

	continueLoop := h.HandleEscalation(context.Background(), bc, claudeResult)
	if !continueLoop {
		t.Error("HandleEscalation should return true when escalation available")
	}
	if bc.Tier != provider.TierMedium {
		t.Errorf("Tier = %q, want %q", bc.Tier, provider.TierMedium)
	}
	if !bc.Result.Escalated {
		t.Error("Result.Escalated should be true")
	}
	if bc.Result.EscalatedTo == "" {
		t.Error("Result.EscalatedTo should be set after escalation")
	}
}

func TestHandleEscalation_NoMoreTiersAttempsDecomposition(t *testing.T) {
	// When at the highest tier with no further escalation, HandleEscalation
	// should attempt decomposition.
	decomposeCalled := false
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return []runtypes.SubTask{{Title: "sub-1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
			return nil
		},
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.Tier = provider.TierHigh // highest tier, no further escalation
	bc.Model = "opus"
	claudeResult := &provider.Result{Output: "build failed"}

	h.HandleEscalation(context.Background(), bc, claudeResult)
	if !decomposeCalled {
		t.Error("HandleEscalation should attempt decomposition when no more tiers available")
	}
}

func TestHandleEscalation_MaxRetriesPerBeadExceededStops(t *testing.T) {
	// When total retries exceed the per-bead limit, HandleEscalation should
	// stop and set an error, even if a higher tier exists.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.TotalRetriesThisBead = 6
	bc.MaxRetriesPerBead = 5
	claudeResult := &provider.Result{Output: "build failed"}

	continueLoop := h.HandleEscalation(context.Background(), bc, claudeResult)
	if continueLoop {
		t.Error("HandleEscalation should return false when max retries per bead exceeded")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error when max retries per bead exceeded")
	}
}

func TestAttemptDecomposition_SuccessSetsDecomposedFlag(t *testing.T) {
	// When decomposition succeeds, Result.Decomposed should be set to true.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			return []runtypes.SubTask{
				{Title: "sub-task-1", Description: "first part"},
				{Title: "sub-task-2", Description: "second part"},
			}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
			return nil
		},
		nil,
		nil,
	)

	bc := newTestBeadContext()
	continueLoop := h.AttemptDecomposition(context.Background(), bc, "test failure")
	if continueLoop {
		t.Error("AttemptDecomposition should always return false")
	}
	if !bc.Result.Decomposed {
		t.Error("Result.Decomposed should be true after successful decomposition")
	}
	if bc.Result.Error != nil {
		t.Errorf("Result.Error should be nil after successful decomposition, got: %v", bc.Result.Error)
	}
}

func TestAttemptDecomposition_DecomposeFailureSetsError(t *testing.T) {
	// When the decompose callback fails, Result.Error should capture the reason.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			return nil, fmt.Errorf("LLM call failed")
		},
		nil,
		nil,
		nil,
	)

	bc := newTestBeadContext()
	continueLoop := h.AttemptDecomposition(context.Background(), bc, "stall timeout")
	if continueLoop {
		t.Error("AttemptDecomposition should always return false")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error when decomposition fails")
	}
	if bc.Result.Decomposed {
		t.Error("Result.Decomposed should be false when decomposition fails")
	}
}

func TestEscalateTier_UpdatesBeadContextFields(t *testing.T) {
	// EscalateTier should update all relevant fields on BeadContext:
	// Tier, Model, Result.Escalated, Result.EscalatedTo, RetriesThisModel.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"
	bc.RetriesThisModel = 2

	h.EscalateTier(bc, provider.TierMedium)

	if bc.Tier != provider.TierMedium {
		t.Errorf("Tier = %q, want %q", bc.Tier, provider.TierMedium)
	}
	if !bc.Result.Escalated {
		t.Error("Result.Escalated should be true")
	}
	if bc.Result.EscalatedTo == "" {
		t.Error("Result.EscalatedTo should be set")
	}
	if bc.RetriesThisModel != 0 {
		t.Errorf("RetriesThisModel = %d, want 0 after escalation", bc.RetriesThisModel)
	}
	// Model should be updated to the legacy model name for the new tier
	if bc.Model != "sonnet" {
		t.Errorf("Model = %q, want %q for medium tier", bc.Model, "sonnet")
	}
}

func TestExecuteWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	// When the InvokeFn succeeds on the first attempt, ExecuteWithRetry should
	// return true (success) without any retry or escalation.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		return &runtypes.InvocationResult{
			Result: &claude.Result{Success: true, Output: "build complete"},
		}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if !success {
		t.Error("ExecuteWithRetry should return true on first-attempt success")
	}
	if bc.Result.Error != nil {
		t.Errorf("Result.Error should be nil on success, got: %v", bc.Result.Error)
	}
}

func TestExecuteWithRetryWithEscalation_DisabledSkipsTierEscalation(t *testing.T) {
	cfg := newTestConfig()
	bc := newTestBeadContext()
	bc.Tier = provider.TierLow

	invocations := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invocations++
		return &runtypes.InvocationResult{
			Result: &claude.Result{Success: false, Output: "failed"},
		}, nil
	}

	h := NewHandler(cfg, &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryLogic,
				Recoverable: false,
				RootCause:   "non-recoverable",
			}, nil
		},
	}, &mockBeadClient{}, nil, nil, nil, nil)

	success := h.ExecuteWithRetryWithEscalation(context.Background(), bc, invokeFn, false)
	if success {
		t.Fatal("expected ExecuteWithRetryWithEscalation to return false when escalation disabled")
	}
	if invocations <= 1 {
		t.Fatalf("expected at least one same-tier retry with escalation disabled, got %d invocations", invocations)
	}
	if bc.Tier != provider.TierLow {
		t.Fatalf("expected tier to remain %q, got %q", provider.TierLow, bc.Tier)
	}
	if bc.Result.Escalated {
		t.Fatalf("expected Escalated=false with escalation disabled")
	}
}

func TestExecuteWithRetry_UsesProviderResultWhenClaudeResultMissing(t *testing.T) {
	cfg := newTestConfig()
	capturedFailureOutput := ""
	h := NewHandler(cfg, &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			capturedFailureOutput = failureOutput
			return &analyzer.Analysis{
				Category:    analyzer.CategoryLogic,
				Recoverable: false,
				RootCause:   "non-recoverable",
			}, nil
		},
	}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		return &runtypes.InvocationResult{
			ProviderResult: &provider.Result{
				Success: false,
				Output:  "provider failure output",
				Model:   "sonnet",
			},
		}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected ExecuteWithRetry to return false for non-recoverable failure")
	}
	if capturedFailureOutput != "provider failure output" {
		t.Fatalf("analyzer failure output = %q, want %q", capturedFailureOutput, "provider failure output")
	}
}

func TestExecuteWithRetry_StallFiresRetryAndEscalate(t *testing.T) {
	// When the invocation returns a stall, ExecuteWithRetry should handle it
	// via HandleStallTimeout (retry same model or escalate).
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"
	bc.MaxRetries = 0 // exhaust retries immediately to trigger escalation

	callCount := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		callCount++
		if callCount == 1 {
			// First call: stall
			bc.Result.ToolCallCount = 1 // treat as active stall => escalate, not same-tier retry
			return &runtypes.InvocationResult{
				StallFired: true,
				Result:     &claude.Result{Success: false},
			}, context.Canceled
		}
		// Second call: success after escalation
		return &runtypes.InvocationResult{
			Result: &claude.Result{Success: true, Output: "ok"},
		}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if !success {
		t.Error("ExecuteWithRetry should succeed after stall + escalation")
	}
	if callCount < 2 {
		t.Errorf("invokeFn called %d times, want >= 2 (stall + retry)", callCount)
	}
	if bc.Tier != provider.TierMedium {
		t.Errorf("Tier = %q, want %q after stall escalation", bc.Tier, provider.TierMedium)
	}
}

func TestExecuteWithRetry_InvocationTimeoutWithoutDecomposerFails(t *testing.T) {
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"

	callCount := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		callCount++
		if callCount == 1 {
			return &runtypes.InvocationResult{TimeoutType: "invocation"}, fmt.Errorf("context deadline exceeded")
		}
		return &runtypes.InvocationResult{Result: &claude.Result{Success: true, Output: "ok"}}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected timeout-first decomposition path to stop when decomposer is unavailable")
	}
	if callCount != 1 {
		t.Fatalf("expected exactly 1 invocation before stop, got %d", callCount)
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error from timeout-first decomposition path")
	}
}

func TestHandleInvocationTimeout_NoHigherTierAttemptsDecomposition(t *testing.T) {
	cfg := newTestConfig()
	decomposeCalled := false
	createSubCalled := false
	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return []runtypes.SubTask{{Title: "subtask 1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
			createSubCalled = true
			return nil
		},
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.Tier = provider.TierHigh
	bc.ParentCtx = context.Background()

	continueLoop := h.HandleInvocationTimeout(context.Background(), bc)
	if continueLoop {
		t.Fatal("expected no retry when already at highest tier")
	}
	if !decomposeCalled {
		t.Fatal("expected decomposition attempt when no higher tier is available")
	}
	if !createSubCalled {
		t.Fatal("expected sub-bead creation after decomposition")
	}
	if !bc.Result.Decomposed {
		t.Fatal("expected result to be marked decomposed")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected nil error after successful decomposition, got: %v", bc.Result.Error)
	}
}

func TestHandleInvocationTimeout_FirstTimeoutDecomposesWithoutEscalation(t *testing.T) {
	cfg := newTestConfig()
	decomposeCalled := false
	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return []runtypes.SubTask{{Title: "subtask 1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error { return nil },
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"
	bc.ParentCtx = context.Background()

	continueLoop := h.HandleInvocationTimeout(context.Background(), bc)
	if continueLoop {
		t.Fatal("expected no retry loop continuation after timeout-first decomposition")
	}
	if !decomposeCalled {
		t.Fatal("expected first invocation timeout to trigger decomposition")
	}
	if bc.Result.Escalated {
		t.Fatal("did not expect escalation before decomposition on first timeout")
	}
	if bc.Tier != provider.TierLow {
		t.Fatalf("tier changed from %s to %s", provider.TierLow, bc.Tier)
	}
	if !bc.Result.Decomposed {
		t.Fatal("expected decomposition success to be recorded")
	}
	if !bc.Result.TimeoutDecompositionAttempted {
		t.Fatal("expected timeout decomposition attempt audit to be recorded")
	}
	if !bc.Result.TimeoutDecompositionSucceeded {
		t.Fatal("expected timeout decomposition audit to mark success")
	}
}

func TestHandleStallTimeout_FirstTimeoutBudgetThresholdSkipsDecomposition(t *testing.T) {
	cfg := newTestConfig()
	decomposeCalled := false
	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return nil, fmt.Errorf("unexpected decomposition call")
		},
		nil,
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.ParentCtx = context.Background()
	bc.Result.ToolCallCount = 1
	bc.RetriesThisModel = bc.MaxRetries + 1
	bc.BeadTimeout = time.Minute
	bc.BeadStartTime = time.Now().Add(-50 * time.Second)

	continueLoop := h.HandleStallTimeout(context.Background(), bc)
	if !continueLoop {
		t.Fatal("expected escalation to continue when budget threshold already consumed")
	}
	if decomposeCalled {
		t.Fatal("did not expect decomposition when budget threshold exceeded")
	}
	if !bc.Result.TimeoutDecompositionAttempted {
		t.Fatal("expected timeout decomposition audit to be recorded even when skipped")
	}
	if bc.Result.TimeoutDecompositionOutcome != "skipped" {
		t.Fatalf("TimeoutDecompositionOutcome = %q, want %q", bc.Result.TimeoutDecompositionOutcome, "skipped")
	}
	if !strings.Contains(bc.Result.TimeoutDecompositionReason, "skipped") {
		t.Fatalf("unexpected timeout decomposition reason: %q", bc.Result.TimeoutDecompositionReason)
	}
	if bc.Result.TimeoutDecompositionAttemptTime.IsZero() {
		t.Fatal("expected timeout decomposition attempt time to be recorded")
	}
	if bc.TimeoutEscalationsThisBead != 1 {
		t.Fatalf("TimeoutEscalationsThisBead = %d, want 1", bc.TimeoutEscalationsThisBead)
	}
	if !bc.Result.Escalated {
		t.Fatal("expected escalation after budget threshold prevented decomposition")
	}
}

func TestExecuteWithRetry_BeadTimeoutDecomposesOnFirstTimeout(t *testing.T) {
	cfg := newTestConfig()
	cfg.Escalation.Chain = []string{"haiku", "sonnet", "opus"}

	decomposeCalled := false
	invocationTiers := []string{}

	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return []runtypes.SubTask{{Title: "subtask 1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error { return nil },
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.Tier = provider.TierMedium // sonnet — opus is available
	bc.ParentCtx = context.Background()

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invocationTiers = append(invocationTiers, bc.Tier)
		return &runtypes.InvocationResult{TimeoutType: "bead"}, fmt.Errorf("bead timeout")
	}

	h.ExecuteWithRetry(context.Background(), bc, invokeFn)

	if len(invocationTiers) != 1 {
		t.Fatalf("expected exactly 1 invocation before decomposition, got %d: %v", len(invocationTiers), invocationTiers)
	}
	if !decomposeCalled {
		t.Error("expected decomposition after first bead timeout")
	}
}

func TestExecuteWithRetry_BeadTimeoutFirstTimeoutDecomposesWithoutEscalation(t *testing.T) {
	cfg := newTestConfig()
	cfg.Escalation.Chain = []string{"haiku", "sonnet", "opus"}

	decomposeCalled := false
	invocationTiers := []string{}

	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return []runtypes.SubTask{{Title: "subtask 1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error { return nil },
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.Tier = provider.TierMedium
	bc.ParentCtx = context.Background()

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invocationTiers = append(invocationTiers, bc.Tier)
		return &runtypes.InvocationResult{TimeoutType: "bead"}, fmt.Errorf("bead timeout")
	}

	h.ExecuteWithRetry(context.Background(), bc, invokeFn)

	if len(invocationTiers) != 1 {
		t.Fatalf("expected exactly 1 invocation before decomposition, got %d (%v)", len(invocationTiers), invocationTiers)
	}
	if !decomposeCalled {
		t.Fatal("expected first bead timeout to trigger decomposition")
	}
	if bc.Result.Escalated {
		t.Fatal("did not expect escalation before decomposition on bead timeout")
	}
}

func TestExecuteWithRetry_BeadTimeoutEscalationLogsReason(t *testing.T) {
	// When bead timeout triggers escalation, the log message should mention
	// "bead timeout" so operators can distinguish it from stall/invocation escalations.
	cfg := newTestConfig()
	cfg.Escalation.Chain = []string{"haiku", "sonnet", "opus"}

	var logs []string

	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			return []runtypes.SubTask{{Title: "sub 1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error { return nil },
		func(format string, args ...interface{}) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
		nil,
	)

	bc := newTestBeadContext()
	bc.Tier = provider.TierMedium
	bc.ParentCtx = context.Background()

	call := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		call++
		return &runtypes.InvocationResult{TimeoutType: "bead"}, fmt.Errorf("bead timeout")
	}

	h.ExecuteWithRetry(context.Background(), bc, invokeFn)

	combined := strings.ToLower(strings.Join(logs, "\n"))
	if !strings.Contains(combined, "bead timeout") {
		t.Errorf("expected 'bead timeout' in escalation log, got logs: %q", logs)
	}
}

func TestExecuteWithRetry_BeadTimeoutSkipsEscalationWhenLimitReached(t *testing.T) {
	// When timeout escalation limit is already reached, bead timeout goes directly
	// to decomposition without attempting another escalation.
	cfg := newTestConfig()
	cfg.Escalation.Chain = []string{"haiku", "sonnet", "opus"}

	decomposeCalled := false
	invocationCount := 0

	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return []runtypes.SubTask{{Title: "sub 1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error { return nil },
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.Tier = provider.TierMedium     // sonnet, opus available — but limit already reached
	bc.TimeoutEscalationsThisBead = 1 // escalation limit already exhausted
	bc.ParentCtx = context.Background()

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invocationCount++
		return &runtypes.InvocationResult{TimeoutType: "bead"}, fmt.Errorf("bead timeout")
	}

	h.ExecuteWithRetry(context.Background(), bc, invokeFn)

	if invocationCount != 1 {
		t.Errorf("expected exactly 1 invocation (no escalation), got %d", invocationCount)
	}
	if !decomposeCalled {
		t.Error("expected decomposition when timeout escalation limit already reached")
	}
}

func TestExecuteWithRetry_BeadTimeoutAtHighestTierDecomposesDirectly(t *testing.T) {
	// When a bead timeout occurs at the highest tier, decompose without escalation.
	cfg := newTestConfig()
	cfg.Escalation.Chain = []string{"haiku", "sonnet", "opus"}

	decomposeCalled := false
	invocationCount := 0

	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return []runtypes.SubTask{{Title: "sub 1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error { return nil },
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.Tier = provider.TierHigh // opus — no higher tier available
	bc.ParentCtx = context.Background()

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invocationCount++
		return &runtypes.InvocationResult{TimeoutType: "bead"}, fmt.Errorf("bead timeout")
	}

	h.ExecuteWithRetry(context.Background(), bc, invokeFn)

	if invocationCount != 1 {
		t.Errorf("expected exactly 1 invocation at highest tier, got %d", invocationCount)
	}
	if !decomposeCalled {
		t.Error("expected decomposition when already at highest tier on bead timeout")
	}
}

func TestExecuteWithRetry_BeadTimeoutAttemptsDecomposition(t *testing.T) {
	cfg := newTestConfig()
	decomposeCalled := false
	createSubCalled := false
	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			if err := ctx.Err(); err != nil {
				t.Fatalf("decompose context should be active, got error: %v", err)
			}
			return []runtypes.SubTask{{Title: "subtask 1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
			createSubCalled = true
			if len(tasks) != 1 {
				t.Fatalf("expected 1 subtask, got %d", len(tasks))
			}
			return nil
		},
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.ParentCtx = context.Background()

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		return &runtypes.InvocationResult{TimeoutType: "bead"}, fmt.Errorf("bead timeout")
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected failure return after decomposition path (bead processing stops)")
	}
	if !decomposeCalled {
		t.Fatal("expected bead timeout to trigger decomposition attempt")
	}
	if !createSubCalled {
		t.Fatal("expected bead timeout decomposition to create sub-beads")
	}
	if !bc.Result.Decomposed {
		t.Fatal("expected result to mark decomposition success")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected nil error after successful decomposition, got: %v", bc.Result.Error)
	}
}

func TestExecuteWithRetry_BeadTimeoutWithCanceledParentContextSkipsDecomposition(t *testing.T) {
	cfg := newTestConfig()
	decomposeCalled := false
	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return nil, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error { return nil },
		nil,
		nil,
	)

	parentCtx, cancel := context.WithCancel(context.Background())
	cancel()

	bc := newTestBeadContext()
	bc.ParentCtx = parentCtx

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		return &runtypes.InvocationResult{TimeoutType: "bead"}, fmt.Errorf("bead timeout")
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected failure return when parent context is canceled")
	}
	if decomposeCalled {
		t.Fatal("did not expect decomposition attempt when parent context is canceled")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error when parent context is canceled")
	}
	if got := bc.Result.Error.Error(); !strings.Contains(got, "decomposition skipped: parent context canceled") {
		t.Fatalf("unexpected error: %v", bc.Result.Error)
	}
}

func TestExecuteWithRetry_StopsWhenAttemptBudgetExceeded(t *testing.T) {
	cfg := newTestConfig()
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{Category: analyzer.CategoryLogic, Recoverable: true, Suggestion: "retry"}, nil
		},
	}
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.MaxAttemptsPerBead = 1

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		return &runtypes.InvocationResult{Result: &claude.Result{Success: false, Output: "fail"}}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected failure when attempt budget is exhausted")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected retry budget error")
	}
	if got := bc.Result.Error.Error(); got == "" || !strings.Contains(got, "retry budget exceeded") {
		t.Fatalf("unexpected error: %v", bc.Result.Error)
	}
	if bc.Result.FailurePhase != failurephase.Build {
		t.Fatalf("FailurePhase = %q, want %q", bc.Result.FailurePhase, failurephase.Build)
	}
}

func TestExecuteWithRetry_BlocksSameScopeRetryAfterTimeoutWithoutDecision(t *testing.T) {
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"

	callCount := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		callCount++
		if callCount == 1 {
			return &runtypes.InvocationResult{StallFired: true}, fmt.Errorf("stall timeout")
		}
		return &runtypes.InvocationResult{Result: &claude.Result{Success: true, Output: "ok"}}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected failure because same-scope retry should be blocked after timeout")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 invocation before block, got %d", callCount)
	}
	if bc.Result.Error == nil {
		t.Fatal("expected blocking error")
	}
	if got := bc.Result.Error.Error(); !strings.Contains(got, "Same-scope retry blocked: timeout requires decomposition or escalation decision") {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestHandleInvocationTimeout_RecordsTimeoutDecompositionOutcome(t *testing.T) {
	cfg := newTestConfig()
	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			return []runtypes.SubTask{{Title: "subtask 1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error { return nil },
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.ParentCtx = context.Background()

	h.HandleInvocationTimeout(context.Background(), bc)

	if !bc.Result.TimeoutDecompositionAttempted {
		t.Fatal("expected timeout decomposition attempt to be recorded")
	}
	if !bc.Result.TimeoutDecompositionSucceeded {
		t.Fatal("expected timeout decomposition success to be recorded")
	}
}

func TestExecuteWithRetry_AccumulatesTokensAcrossInvocations(t *testing.T) {
	cfg := newTestConfig()
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{Category: analyzer.CategoryLogic, Recoverable: true, Suggestion: "retry"}, nil
		},
	}
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	callCount := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		callCount++
		if callCount == 1 {
			bc.Result.InputTokens = 120
			bc.Result.OutputTokens = 40
			return &runtypes.InvocationResult{Result: &claude.Result{Success: false, Output: "fail 1"}}, nil
		}
		bc.Result.InputTokens = 30
		bc.Result.OutputTokens = 10
		return &runtypes.InvocationResult{Result: &claude.Result{Success: true, Output: "ok"}}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if !success {
		t.Fatal("expected success after retry")
	}
	if bc.CumulativeInputTokens != 150 {
		t.Fatalf("CumulativeInputTokens=%d, want 150", bc.CumulativeInputTokens)
	}
	if bc.CumulativeOutputTokens != 50 {
		t.Fatalf("CumulativeOutputTokens=%d, want 50", bc.CumulativeOutputTokens)
	}
}

func TestExecuteWithRetry_TokenBudgetExceededAttemptsDecompositionBeforeInvocation(t *testing.T) {
	cfg := newTestConfig()
	cfg.Claude.MaxInputTokensPerBead = 100

	decomposeCalled := false
	createSubCalled := false
	h := NewHandler(
		cfg,
		&mockFailureAnalyzer{},
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			decomposeCalled = true
			return []runtypes.SubTask{{Title: "split-1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
			createSubCalled = true
			return nil
		},
		nil,
		nil,
	)

	bc := newTestBeadContext()
	bc.CumulativeInputTokens = 101
	bc.ParentCtx = context.Background()

	invokeCount := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invokeCount++
		return &runtypes.InvocationResult{Result: &claude.Result{Success: true, Output: "unexpected"}}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected false because decomposition path stops bead execution")
	}
	if invokeCount != 0 {
		t.Fatalf("expected no invocation when token budget already exceeded, got %d", invokeCount)
	}
	if !decomposeCalled {
		t.Fatal("expected decomposition when token budget exceeded before attempt")
	}
	if !createSubCalled {
		t.Fatal("expected sub-bead creation after decomposition")
	}
	if !bc.Result.Decomposed {
		t.Fatal("expected decomposed flag set after successful decomposition")
	}
	if bc.Result.Error != nil {
		t.Fatalf("expected nil error on successful decomposition, got %v", bc.Result.Error)
	}
}

func newRecoverableRefactorAnalyzer(analyzeCalls *int) FailureAnalyzer {
	return &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			(*analyzeCalls)++
			return &analyzer.Analysis{
				Category:    analyzer.CategoryLogic,
				Recoverable: true,
				Suggestion:  "retry after refactor",
			}, nil
		},
	}
}

func newTokenBudgetHandlerWithDecomposition(
	cfg *config.Config,
	analyzer FailureAnalyzer,
	decomposeCalled, createSubCalled *bool,
	logf func(format string, args ...interface{}),
) *Handler {
	return NewHandler(
		cfg,
		analyzer,
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			*decomposeCalled = true
			return []runtypes.SubTask{{Title: "split-1"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
			*createSubCalled = true
			return nil
		},
		logf,
		nil,
	)
}

func TestExecuteWithRetry_RefactorTokensCanExhaustBudgetBeforeNextAttempt(t *testing.T) {
	cfg := newTestConfig()
	cfg.Claude.MaxInputTokensPerBead = 100

	analyzeCalls := 0
	mfa := newRecoverableRefactorAnalyzer(&analyzeCalls)

	decomposeCalled := false
	createSubCalled := false
	h := newTokenBudgetHandlerWithDecomposition(cfg, mfa, &decomposeCalled, &createSubCalled, nil)

	bc := newTestBeadContext()
	bc.ParentCtx = context.Background()

	invokeCalls := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invokeCalls++
		bc.Result.InputTokens = 40 // initial build usage below cap
		if invokeCalls == 1 {
			// Simulate refactor-phase input tokens recorded before the next retry-budget check.
			bc.CumulativeInputTokens += 70
			return &runtypes.InvocationResult{Result: &claude.Result{Success: false, Output: "fail 1"}}, nil
		}
		return &runtypes.InvocationResult{Result: &claude.Result{Success: true, Output: "unexpected"}}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected retry flow to stop after refactor-augmented token budget exhaustion")
	}
	if invokeCalls != 1 {
		t.Fatalf("expected second attempt to be blocked by budget gate, invokeCalls=%d", invokeCalls)
	}
	if !decomposeCalled {
		t.Fatal("expected decomposition when refactor tokens pushed cumulative tokens over budget")
	}
	if !createSubCalled {
		t.Fatal("expected sub-bead creation after decomposition")
	}
	if !bc.Result.Decomposed {
		t.Fatal("expected decomposed result when retry is blocked by token budget")
	}
	if analyzeCalls != 1 {
		t.Fatalf("Analyze calls=%d, want 1", analyzeCalls)
	}
}

func TestExecuteWithRetry_RefactorTokensUnderCapStillPermitRetryAttempt(t *testing.T) {
	cfg := newTestConfig()
	cfg.Claude.MaxInputTokensPerBead = 100

	analyzeCalls := 0
	mfa := newRecoverableRefactorAnalyzer(&analyzeCalls)

	decomposeCalled := false
	createSubCalled := false
	h := newTokenBudgetHandlerWithDecomposition(cfg, mfa, &decomposeCalled, &createSubCalled, nil)

	bc := newTestBeadContext()
	bc.ParentCtx = context.Background()

	invokeCalls := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invokeCalls++
		switch invokeCalls {
		case 1:
			bc.Result.InputTokens = 40
			// Refactor tokens stay under cap; next attempt should be allowed.
			bc.CumulativeInputTokens += 50
			return &runtypes.InvocationResult{Result: &claude.Result{Success: false, Output: "fail 1"}}, nil
		case 2:
			bc.Result.InputTokens = 5
			return &runtypes.InvocationResult{Result: &claude.Result{Success: true, Output: "ok"}}, nil
		default:
			t.Fatalf("unexpected invoke call %d", invokeCalls)
			return nil, nil
		}
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if !success {
		t.Fatal("expected retry to proceed when refactor-augmented cumulative tokens remain under cap")
	}
	if invokeCalls != 2 {
		t.Fatalf("expected second attempt to run, invokeCalls=%d", invokeCalls)
	}
	if decomposeCalled {
		t.Fatal("did not expect decomposition while cumulative tokens are under cap")
	}
	if createSubCalled {
		t.Fatal("did not expect sub-bead creation while cumulative tokens are under cap")
	}
	if bc.CumulativeInputTokens != 95 {
		t.Fatalf("CumulativeInputTokens=%d, want 95", bc.CumulativeInputTokens)
	}
	if analyzeCalls != 1 {
		t.Fatalf("Analyze calls=%d, want 1", analyzeCalls)
	}
}

func TestExecuteWithRetry_MultiRefactorCyclesStopAtTokenBudget(t *testing.T) {
	cfg := newTestConfig()
	cfg.Claude.MaxInputTokensPerBead = 140
	cfg.Andon.L1RetryCap = 10

	analyzeCalls := 0
	mfa := newRecoverableRefactorAnalyzer(&analyzeCalls)

	decomposeCalled := false
	createSubCalled := false
	var logLines []string
	h := newTokenBudgetHandlerWithDecomposition(
		cfg,
		mfa,
		&decomposeCalled,
		&createSubCalled,
		func(format string, args ...interface{}) {
			logLines = append(logLines, fmt.Sprintf(format, args...))
		},
	)

	bc := newTestBeadContext()
	bc.ParentCtx = context.Background()
	bc.MaxRetries = 10
	bc.MaxRetriesPerBead = 10

	const (
		buildInputPerAttempt    = 20
		refactorInputPerFailure = 30
	)
	invokeCalls := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invokeCalls++
		switch invokeCalls {
		case 1, 2, 3:
			bc.Result.InputTokens = buildInputPerAttempt
			// Simulate each failed attempt entering a refactor cycle before the next retry.
			bc.CumulativeInputTokens += refactorInputPerFailure
			return &runtypes.InvocationResult{Result: &claude.Result{Success: false, Output: fmt.Sprintf("fail %d", invokeCalls)}}, nil
		case 4:
			bc.Result.InputTokens = buildInputPerAttempt
			return &runtypes.InvocationResult{Result: &claude.Result{Success: true, Output: "unexpected success"}}, nil
		default:
			t.Fatalf("unexpected invoke call %d", invokeCalls)
			return nil, nil
		}
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected retry flow to stop when refactor-augmented cumulative tokens exceed budget")
	}
	if invokeCalls != 3 {
		t.Fatalf("expected budget gate to block attempt 4, invokeCalls=%d", invokeCalls)
	}
	if analyzeCalls != 3 {
		t.Fatalf("Analyze calls=%d, want 3", analyzeCalls)
	}
	if !decomposeCalled {
		t.Fatal("expected decomposition when cumulative token budget is exceeded")
	}
	if !createSubCalled {
		t.Fatal("expected sub-bead creation after token-budget decomposition")
	}
	if !bc.Result.Decomposed {
		t.Fatal("expected decomposed result when token budget gate stops retries")
	}
	if bc.CumulativeInputTokens != 150 {
		t.Fatalf("CumulativeInputTokens=%d, want 150", bc.CumulativeInputTokens)
	}
	joinedLogs := strings.Join(logLines, "\n")
	if !strings.Contains(joinedLogs, "retry budget exceeded: cumulative input tokens 150/140") {
		t.Fatalf("logs did not identify token budget gate, got:\n%s", joinedLogs)
	}
	// Regression guard: invocation tokens alone are only 60 after 3 attempts.
	// Without refactor-phase additions, attempt 4 would not be blocked by token budget.
	if bc.CumulativeInputTokens-buildInputPerAttempt*invokeCalls >= cfg.Claude.MaxInputTokensPerBead {
		t.Fatalf("test invariant broken: invocation-only tokens should stay under cap")
	}
}

func TestExecuteWithRetry_ContextCancellationStops(t *testing.T) {
	// When the context is cancelled, ExecuteWithRetry should stop and return false.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	bc := newTestBeadContext()
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		t.Fatal("invokeFn should not be called when context is cancelled")
		return nil, nil
	}

	success := h.ExecuteWithRetry(ctx, bc, invokeFn)
	if success {
		t.Error("ExecuteWithRetry should return false when context is cancelled")
	}
	if bc.Result.FailurePhase != failurephase.Timeout {
		t.Fatalf("FailurePhase = %q, want %q", bc.Result.FailurePhase, failurephase.Timeout)
	}
	if bc.Result.TimeoutPhase != "build" {
		t.Fatalf("TimeoutPhase = %q, want %q", bc.Result.TimeoutPhase, "build")
	}
}

func TestExecuteWithRetry_BuildFailureAnalyzesAndRetries(t *testing.T) {
	// When a build fails with a recoverable error, ExecuteWithRetry should
	// analyze the failure and retry with failure context injected.
	recoverableAnalysis := &analyzer.Analysis{
		Category:    "logic_error",
		Recoverable: true,
		RootCause:   "missing nil check",
		Suggestion:  "add nil check before accessing field",
	}
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return recoverableAnalysis, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.MaxRetries = 2

	callCount := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		callCount++
		if callCount == 1 {
			return &runtypes.InvocationResult{
				Result: &claude.Result{Success: false, Output: "nil pointer dereference"},
			}, nil
		}
		return &runtypes.InvocationResult{
			Result: &claude.Result{Success: true, Output: "fixed"},
		}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if !success {
		t.Error("ExecuteWithRetry should succeed after recoverable failure + retry")
	}
	if callCount != 2 {
		t.Errorf("invokeFn called %d times, want 2", callCount)
	}
}

func TestExecuteWithRetry_TriageTransportDisconnectRetriesWithoutAnalyzer(t *testing.T) {
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			t.Fatal("analyzer should not run for retryable provider transport triage")
			return nil, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	invokeCalls := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invokeCalls++
		if invokeCalls == 1 {
			return &runtypes.InvocationResult{
				Result: &claude.Result{Success: false, Output: "transport disconnected"},
				ProviderResult: &provider.Result{
					FailureCategory: provider.FailureCategoryTransportDisconnect,
				},
			}, nil
		}
		return &runtypes.InvocationResult{Result: &claude.Result{Success: true, Output: "ok"}}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if !success {
		t.Fatal("expected retryable transport disconnect to continue and succeed on retry")
	}
	if invokeCalls != 2 {
		t.Fatalf("invoke calls=%d, want 2", invokeCalls)
	}
	if mfa.analyzeCalls != 0 {
		t.Fatalf("Analyze calls=%d, want 0", mfa.analyzeCalls)
	}
	if bc.Result.FailureLayer != string(LayerProviderTransport) {
		t.Fatalf("FailureLayer=%q, want %q", bc.Result.FailureLayer, LayerProviderTransport)
	}
	if bc.Result.FailureSubCat != "disconnect" {
		t.Fatalf("FailureSubCat=%q, want %q", bc.Result.FailureSubCat, "disconnect")
	}
}

func TestExecuteWithRetry_TriageTransportAuthStopsWithoutAnalyzer(t *testing.T) {
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			t.Fatal("analyzer should not run for auth triage")
			return nil, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		return &runtypes.InvocationResult{
			Result: &claude.Result{Success: false, Output: "auth failed"},
			ProviderResult: &provider.Result{
				FailureCategory: provider.FailureCategoryAuth,
			},
		}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected auth failure to stop loop")
	}
	if mfa.analyzeCalls != 0 {
		t.Fatalf("Analyze calls=%d, want 0", mfa.analyzeCalls)
	}
	if bc.Result.Error == nil || !strings.Contains(strings.ToLower(bc.Result.Error.Error()), "authentication") {
		t.Fatalf("expected actionable auth error, got %v", bc.Result.Error)
	}
	if bc.Result.FailureLayer != string(LayerProviderTransport) {
		t.Fatalf("FailureLayer=%q, want %q", bc.Result.FailureLayer, LayerProviderTransport)
	}
	if bc.Result.FailureSubCat != "auth" {
		t.Fatalf("FailureSubCat=%q, want %q", bc.Result.FailureSubCat, "auth")
	}
}

func TestExecuteWithRetry_TriageEnvironmentStopsWithActionableErrorWithoutAnalyzer(t *testing.T) {
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			t.Fatal("analyzer should not run for environment triage")
			return nil, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		return &runtypes.InvocationResult{
			Result: &claude.Result{Success: false, Output: "go not found"},
			ProviderResult: &provider.Result{
				Stderr: "exec: go: executable file not found in $PATH",
			},
		}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected environment failure to stop loop")
	}
	if mfa.analyzeCalls != 0 {
		t.Fatalf("Analyze calls=%d, want 0", mfa.analyzeCalls)
	}
	if bc.Result.Error == nil || !strings.Contains(strings.ToLower(bc.Result.Error.Error()), "environment error") {
		t.Fatalf("expected actionable environment error, got %v", bc.Result.Error)
	}
	if bc.Result.Error == nil || !strings.Contains(strings.ToLower(bc.Result.Error.Error()), "go") {
		t.Fatalf("expected environment error to include command detail, got %v", bc.Result.Error)
	}
	if bc.Result.FailureLayer != string(LayerEnvironment) {
		t.Fatalf("FailureLayer=%q, want %q", bc.Result.FailureLayer, LayerEnvironment)
	}
	if bc.Result.FailureSubCat != "missing_tool" {
		t.Fatalf("FailureSubCat=%q, want %q", bc.Result.FailureSubCat, "missing_tool")
	}
}

func TestExecuteWithRetry_TriageCodeLayerCallsAnalyzer(t *testing.T) {
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryLogic,
				Recoverable: true,
				RootCause:   "missing guard",
				Suggestion:  "add guard",
			}, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	invokeCalls := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		invokeCalls++
		if invokeCalls == 1 {
			return &runtypes.InvocationResult{
				Result: &claude.Result{Success: false, Output: "compile failed"},
				ProviderResult: &provider.Result{
					FailureCategory: provider.FailureCategoryOther,
				},
			}, nil
		}
		return &runtypes.InvocationResult{Result: &claude.Result{Success: true, Output: "fixed"}}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if !success {
		t.Fatal("expected code-layer failure to recover via analyzer path")
	}
	if mfa.analyzeCalls != 1 {
		t.Fatalf("Analyze calls=%d, want 1", mfa.analyzeCalls)
	}
	if bc.Result.FailureLayer != string(LayerCode) {
		t.Fatalf("FailureLayer=%q, want %q", bc.Result.FailureLayer, LayerCode)
	}
	if bc.Result.FailureSubCat != "default" {
		t.Fatalf("FailureSubCat=%q, want %q", bc.Result.FailureSubCat, "default")
	}
}

func TestExecuteWithRetry_AndonBoundedRecoverableFlowStopsLine(t *testing.T) {
	// Expected failure: Handler.ApplyAndonDecision(...) does not exist yet, so
	// ExecuteWithRetry still follows tier/model retry patterns instead of Andon
	// L1 -> L2 bounded recovery and stop-line escalation.
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryLogic,
				Recoverable: true,
				RootCause:   "transient build failure",
				Suggestion:  "retry with tightened patch",
			}, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"
	bc.MaxRetries = 1
	bc.MaxRetriesPerBead = 20
	bc.MaxAttemptsPerBead = 20

	callCount := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		callCount++
		return &runtypes.InvocationResult{
			Result: &claude.Result{
				Success: false,
				Output:  "build failed",
			},
		}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected failure after bounded Andon recovery path reaches stop-line")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected stop-line error after L1/L2 bounded flow")
	}
	if !strings.Contains(strings.ToLower(bc.Result.Error.Error()), "stop-line") {
		t.Fatalf("expected stop-line error, got: %v", bc.Result.Error)
	}
	if callCount > 4 {
		t.Fatalf("expected bounded L1/L2 attempts before stop-line, got %d invocations", callCount)
	}
}

func TestAnalyzeAndHandleFailure_IntegrityUnsafeStateTriggersImmediateL3(t *testing.T) {
	// Expected failure: analyzer.CategoryIntegrityUnsafeState does not exist yet,
	// and Handler.routeIntegrityFailureToL3(...) is not implemented; current
	// behavior escalates/retries instead of immediate L3 stop-line.
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.Category("integrity_unsafe_state"),
				Recoverable: false,
				RootCause:   "repo state integrity violated",
				Suggestion:  "stop and request human intervention",
			}, nil
		},
	}
	cfg := newTestConfig()
	h := NewHandler(cfg, mfa, &mockBeadClient{}, nil, nil, nil, nil)

	bc := newTestBeadContext()
	bc.Tier = provider.TierLow
	bc.Model = "haiku"
	claudeResult := &provider.Result{Output: "fatal integrity failure"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if continueLoop {
		t.Fatal("expected immediate stop (L3) for integrity/unsafe-state failure")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected L3 stop-line error for integrity/unsafe-state failure")
	}
	if !strings.Contains(strings.ToLower(bc.Result.Error.Error()), "l3") {
		t.Fatalf("expected error to identify L3 stop-line, got: %v", bc.Result.Error)
	}
}

func TestExecuteWithRetry_DecompositionRemainsAvailableAsL4Option(t *testing.T) {
	// Expected failure: Handler.AttemptL4Decomposition(...) does not exist yet,
	// so ExecuteWithRetry does not expose decomposition explicitly as an L4 Andon
	// option even when decomposition is used.
	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategoryLogic,
				Recoverable: true,
				RootCause:   "retry still failing",
				Suggestion:  "decompose task",
			}, nil
		},
	}

	var logs []string
	logFn := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	cfg := newTestConfig()
	h := NewHandler(
		cfg,
		mfa,
		&mockBeadClient{},
		func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
			return []runtypes.SubTask{{Title: "split work"}}, nil
		},
		func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
			return nil
		},
		logFn,
		nil,
	)

	bc := newTestBeadContext()
	bc.Tier = provider.TierHigh
	bc.Model = "opus"
	bc.MaxRetries = 0
	bc.MaxRetriesPerBead = 5

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		return &runtypes.InvocationResult{
			Result: &claude.Result{
				Success: false,
				Output:  "still failing",
			},
		}, nil
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Fatal("expected loop to end after L4 decomposition path")
	}
	if !bc.Result.Decomposed {
		t.Fatal("expected decomposition path to remain available")
	}

	joinedLogs := strings.ToLower(strings.Join(logs, "\n"))
	if !strings.Contains(joinedLogs, "l4") {
		t.Fatalf("expected L4 decomposition signal in logs, got logs: %q", joinedLogs)
	}
}
