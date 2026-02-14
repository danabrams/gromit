package escalation

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Mock types for narrow interfaces ---

// mockFailureAnalyzer implements the FailureAnalyzer interface for escalation tests.
type mockFailureAnalyzer struct {
	analyzeFn func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error)
}

func (m *mockFailureAnalyzer) Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
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
		Bead:              &bead.Bead{ID: "test-001", Title: "Test bead", Priority: 1},
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
	bc.RetriesThisModel = 3 // exceeds MaxRetries=2
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
	claudeResult := &claude.Result{Output: "build failed output"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	if continueLoop {
		t.Error("AnalyzeAndHandleFailure should return false for unclear spec")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected error to be set for unclear spec")
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
	claudeResult := &claude.Result{Output: "build output"}

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
	claudeResult := &claude.Result{Output: "compilation error"}

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
	claudeResult := &claude.Result{Success: false, Output: "compile error: missing import"}

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

func TestAnalyzeAndHandleFailure_AnalysisErrorEscalates(t *testing.T) {
	// When the analyzer returns an error, the handler should fall back to
	// HandleEscalation logic (escalate tier or decompose).
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
	claudeResult := &claude.Result{Output: "failed build output"}

	continueLoop := h.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)
	// Should escalate from low->medium tier
	if !continueLoop {
		t.Error("AnalyzeAndHandleFailure should escalate when analysis fails and higher tier available")
	}
	if bc.Tier != provider.TierMedium {
		t.Errorf("Tier = %q, want %q after escalation on analysis failure", bc.Tier, provider.TierMedium)
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
	claudeResult := &claude.Result{Output: "build failed"}

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
	claudeResult := &claude.Result{Output: "build failed"}

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
	claudeResult := &claude.Result{Output: "build failed"}

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
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error) {
		return &InvocationResult{
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
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error) {
		callCount++
		if callCount == 1 {
			// First call: stall
			return &InvocationResult{
				StallFired: true,
				Result:     &claude.Result{Success: false},
			}, nil
		}
		// Second call: success after escalation
		return &InvocationResult{
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

func TestExecuteWithRetry_ContextCancellationStops(t *testing.T) {
	// When the context is cancelled, ExecuteWithRetry should stop and return false.
	cfg := newTestConfig()
	h := NewHandler(cfg, &mockFailureAnalyzer{}, &mockBeadClient{}, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	bc := newTestBeadContext()
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error) {
		t.Fatal("invokeFn should not be called when context is cancelled")
		return nil, nil
	}

	success := h.ExecuteWithRetry(ctx, bc, invokeFn)
	if success {
		t.Error("ExecuteWithRetry should return false when context is cancelled")
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
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error) {
		callCount++
		if callCount == 1 {
			return &InvocationResult{
				Result: &claude.Result{Success: false, Output: "nil pointer dereference"},
			}, nil
		}
		return &InvocationResult{
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
