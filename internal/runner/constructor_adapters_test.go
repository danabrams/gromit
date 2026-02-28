package runner

import (
	"context"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline/epilogue"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/pipeline/review"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/readiness"
	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/internal/tracker"
)

func TestWorktreeMergerAdapterPendingBranches_NilManagerReturnsError(t *testing.T) {
	t.Parallel()
	adapter := &worktreeMergerAdapter{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PendingBranches() panicked with nil manager: %v", r)
		}
	}()

	_, err := adapter.PendingBranches()
	if err == nil {
		t.Fatal("expected error for nil worktree manager")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "worktree manager") {
		t.Fatalf("error = %v, want message mentioning worktree manager", err)
	}
}

func TestApplyCostFallback_EstimatesFromProviderPricing(t *testing.T) {
	t.Parallel()
	result := &provider.Result{
		Model:        "gpt-5.3-codex",
		CostUSD:      0,
		InputTokens:  1000,
		OutputTokens: 500,
	}
	defs := map[string]config.ProviderDef{
		"codex": {
			CostPer1kInput:  0.001,
			CostPer1kOutput: 0.002,
			ModelCosts: map[string]*config.ModelCost{
				"gpt-5.3-codex": {
					CostPer1kInput:  0.003,
					CostPer1kOutput: 0.012,
				},
			},
		},
	}

	applyCostFallback(result, "codex", defs)

	const want = 0.009
	if math.Abs(result.CostUSD-want) > 1e-12 {
		t.Fatalf("CostUSD = %v, want %v", result.CostUSD, want)
	}
}

func TestApplyCostFallback_DoesNotOverrideProviderReportedCost(t *testing.T) {
	t.Parallel()
	result := &provider.Result{
		Model:        "gpt-5.3-codex",
		CostUSD:      0.25,
		InputTokens:  1000,
		OutputTokens: 500,
	}
	defs := map[string]config.ProviderDef{
		"codex": {
			CostPer1kInput:  1,
			CostPer1kOutput: 1,
		},
	}

	applyCostFallback(result, "codex", defs)

	if result.CostUSD != 0.25 {
		t.Fatalf("CostUSD = %v, want unchanged 0.25", result.CostUSD)
	}
}

func TestApplyCostFallback_NoProviderDefLeavesZero(t *testing.T) {
	t.Parallel()
	result := &provider.Result{
		Model:        "gpt-5.3-codex",
		CostUSD:      0,
		InputTokens:  1000,
		OutputTokens: 500,
	}

	applyCostFallback(result, "codex", map[string]config.ProviderDef{})

	if result.CostUSD != 0 {
		t.Fatalf("CostUSD = %v, want 0", result.CostUSD)
	}
}

type testTrendTrigger struct {
	triggered int
}

func (t *testTrendTrigger) Trigger() {
	t.triggered++
}

func TestIterationLogWriterAdapter_TriggersTrendRefreshOnSuccess(t *testing.T) {
	t.Parallel()
	logDir := filepath.Join(t.TempDir(), "logs")
	l, err := logger.NewLogger(logDir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer l.Close()

	trigger := &testTrendTrigger{}
	adapter := &iterationLogWriterAdapter{
		logger:       l,
		trendUpdater: trigger,
	}

	entry := &logger.IterationLog{
		Iteration: 1,
		BeadID:    "bead-1",
		Success:   true,
	}
	if err := adapter.Write(entry); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if trigger.triggered != 1 {
		t.Fatalf("Trigger count = %d, want 1", trigger.triggered)
	}
}

type trackingProvider struct {
	streamCalled bool
	runCalled    bool
}

func (p *trackingProvider) Name() string                    { return "tracking" }
func (p *trackingProvider) ModelForTier(tier string) string { return "sonnet" }
func (p *trackingProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	p.runCalled = true
	return &provider.Result{Success: true, Output: "ok"}, nil
}
func (p *trackingProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	p.streamCalled = true
	if output != nil {
		output.Write([]byte(prompt))
	}
	return &provider.Result{Success: true, Output: "ok"}, nil
}
func (p *trackingProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}
func (p *trackingProvider) IsUsageLimitError(result *provider.Result, err error) bool { return false }
func (p *trackingProvider) IsValidationPassed(result *provider.Result) bool           { return result.Success }
func (p *trackingProvider) IsScopeTooLarge(result *provider.Result) (bool, string)    { return false, "" }

func TestReviewInvokerAdapter_UsesStreamRun(t *testing.T) {
	t.Parallel()
	prov := &trackingProvider{}
	router := provider.NewSingleProviderRouter(prov)
	adapter := &reviewInvokerAdapter{router: router}

	if _, err := adapter.StreamRun(context.Background(), "prompt", "high", io.Discard); err != nil {
		t.Fatalf("StreamRun returned error: %v", err)
	}

	if prov.runCalled {
		t.Fatal("expected Run() NOT to be called")
	}
	if !prov.streamCalled {
		t.Fatal("expected StreamRun() to be called")
	}
}

func TestDecomposerAdapterAcceptsTrackerClient(t *testing.T) {
	t.Parallel()

	// This test verifies that decomposerAdapter accepts tracker.Client
	// in its struct field instead of *bead.Client
	trackerClient := &mockTrackerClient{}
	router := provider.NewSingleProviderRouter(&trackingProvider{})

	// The decomposerAdapter should be able to hold a tracker.Client interface
	var _ prepare.Decomposer = &decomposerAdapter{
		tracker: trackerClient,
		router:  router,
	}
}

func TestSpecMergeRouterAdapter_SelectsUnderlyingRouter(t *testing.T) {
	t.Parallel()
	router := &dummyRouter{}
	adapter := &specMergeRouterAdapter{router: router}

	provider, model := adapter.Select("architecture", "high")
	if provider != nil {
		t.Fatalf("Select returned provider = %v, want nil", provider)
	}
	if model != "dummy-model" {
		t.Fatalf("model = %q, want dummy-model", model)
	}
	if router.phase != "architecture" {
		t.Fatalf("router phase recorded = %q, want architecture", router.phase)
	}
	if router.tier != "high" {
		t.Fatalf("router tier recorded = %q, want high", router.tier)
	}
}

func TestSpecMergeReviewRendererAdapter_ForwardsCalls(t *testing.T) {
	t.Parallel()
	fake := &dummyPromptRenderer{}
	adapter := &specMergeReviewRendererAdapter{renderer: fake}

	ctx := &prompt.ReviewContext{Diff: "diff"}
	out, err := adapter.RenderReview(ctx)
	if err != nil {
		t.Fatalf("RenderReview returned error: %v", err)
	}
	if out != "rendered" {
		t.Fatalf("RenderReview output = %q, want rendered", out)
	}
	if fake.renderCtx != ctx {
		t.Fatalf("renderCtx = %v, want %v", fake.renderCtx, ctx)
	}

	rules, err := adapter.LoadRulesForPhase("code_quality")
	if err != nil {
		t.Fatalf("LoadRulesForPhase returned error: %v", err)
	}
	if rules != "rules" {
		t.Fatalf("rules = %q, want rules", rules)
	}

	spec, err := adapter.LoadSpec("payments")
	if err != nil {
		t.Fatalf("LoadSpec returned error: %v", err)
	}
	if spec != "spec" {
		t.Fatalf("spec = %q, want spec", spec)
	}
}

type dummyRouter struct {
	phase string
	tier  string
}

func (d *dummyRouter) Select(phase, tier string) (provider.Provider, string) {
	d.phase = phase
	d.tier = tier
	return nil, "dummy-model"
}

// failingReadinessRenderer simulates a renderer that fails to render readiness prompts.
type failingReadinessRenderer struct{}

func (f *failingReadinessRenderer) RenderReadiness(ctx *prompt.ReadinessContext) (string, error) {
	return "", fmt.Errorf("rendering failed")
}

// trackingRouter records calls to Select for testing.
type trackingRouter struct {
	phase string
	tier  string
}

func (t *trackingRouter) Select(phase, tier string) (provider.Provider, string) {
	t.phase = phase
	t.tier = tier
	return nil, "model"
}

type dummyPromptRenderer struct {
	renderCtx *prompt.ReviewContext
}

func (d *dummyPromptRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	d.renderCtx = ctx
	return "rendered", nil
}

func (d *dummyPromptRenderer) LoadRulesForPhase(phase string) (string, error) {
	return "rules", nil
}

func (d *dummyPromptRenderer) LoadSpec(name string) (string, error) {
	return "spec", nil
}

func (d *dummyPromptRenderer) RenderReadiness(ctx *prompt.ReadinessContext) (string, error) {
	return "readiness_prompt", nil
}

// mockTrackerClient implements tracker.Client for testing
type mockTrackerClient struct{}

func (m *mockTrackerClient) Ready(ctx context.Context) (*tracker.Item, error) { return nil, nil }
func (m *mockTrackerClient) List(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) Show(ctx context.Context, id string) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) Search(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) Create(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) Update(ctx context.Context, req tracker.UpdateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) ListWithLabel(ctx context.Context, label string) ([]tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) Close(ctx context.Context, id string) error               { return nil }
func (m *mockTrackerClient) Sync(ctx context.Context) error                           { return nil }
func (m *mockTrackerClient) AddComment(ctx context.Context, id, comment string) error { return nil }
func (m *mockTrackerClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return false, nil
}

func TestNewSpecMergeFinalizeDependencies(t *testing.T) {
	t.Parallel()

	gitOps := &dummyGitOpsForSpec{}
	resolver := &dummyResolverForSpec{}
	deps := newSpecMergeFinalizeDependencies(gitOps, resolver, "canonical-main")

	want := specmerge.FinalizeDependencies{
		Git:              gitOps,
		ConflictResolver: resolver,
		MainBranch:       "canonical-main",
	}
	if !reflect.DeepEqual(deps, want) {
		t.Fatalf("deps = %+v, want %+v", deps, want)
	}
}

// decomposerAdapterWithTrackerClient is a version of decomposerAdapter that uses tracker.Client
type decomposerAdapterWithTrackerClient struct {
	tracker tracker.Client
	router  *provider.Router
}

func TestBeadCreatorAdapterAcceptsTrackerClient(t *testing.T) {
	t.Parallel()

	// This test verifies that beadCreatorAdapter accepts tracker.Client
	// in its struct field instead of *bead.Client
	trackerClient := &mockTrackerClient{}

	// The beadCreatorAdapter should be able to hold a tracker.Client interface
	var _ review.BeadCreator = &beadCreatorAdapter{
		beads: newTrackerBeadClient(trackerClient),
	}
}

func TestBeadLifecycleAdapterAcceptsTrackerClient(t *testing.T) {
	t.Parallel()

	// This test verifies that beadLifecycleAdapter accepts tracker.Client
	// in its struct field instead of *bead.Client
	trackerClient := &mockTrackerClient{}

	// The beadLifecycleAdapter should be able to hold a tracker.Client interface
	var _ epilogue.BeadLifecycle = &beadLifecycleAdapter{
		tracker: trackerClient,
	}
}

func TestDecomposerAdapterChildWithDedupeLabelExistsUsesBeadClient(t *testing.T) {
	t.Parallel()

	adapter := &decomposerAdapter{
		beads: &fakeBeadClient{
			listFn: func(ctx context.Context, label string) ([]*bead.Bead, error) {
				if label != "scope_decomp:foo" {
					t.Fatalf("label = %q, want scope_decomp:foo", label)
				}
				return []*bead.Bead{
					{ID: "child-1", Parent: "parent-1", Labels: []string{label}},
				}, nil
			},
		},
	}

	exists, err := adapter.childWithDedupeLabelExists("parent-1", "scope_decomp:foo")
	if err != nil {
		t.Fatalf("childWithDedupeLabelExists returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected child to exist")
	}
}

func TestDecomposerAdapterChildWithDedupeLabelExistsWithClient_ThreadsProvidedContext(t *testing.T) {
	t.Parallel()

	// Create a custom context with a value to verify it's threaded through
	testValue := "test-context-value"
	customCtx := context.WithValue(context.Background(), "test-key", testValue)

	var capturedCtx context.Context
	adapter := &decomposerAdapter{
		beads: &fakeBeadClient{
			listFn: func(ctx context.Context, label string) ([]*bead.Bead, error) {
				capturedCtx = ctx
				if label != "scope_decomp:foo" {
					t.Fatalf("label = %q, want scope_decomp:foo", label)
				}
				return []*bead.Bead{
					{ID: "child-1", Parent: "parent-1", Labels: []string{label}},
				}, nil
			},
		},
	}

	exists, err := adapter.childWithDedupeLabelExistsWithClient(customCtx, "parent-1", "scope_decomp:foo")
	if err != nil {
		t.Fatalf("childWithDedupeLabelExistsWithClient returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected child to exist")
	}

	// Verify the context was threaded through
	if capturedCtx.Value("test-key") != testValue {
		t.Fatal("context value not found in captured context - childWithDedupeLabelExistsWithClient not threading context properly")
	}
}

type fakeBeadClient struct {
	listFn func(ctx context.Context, label string) ([]*bead.Bead, error)
}

func (f *fakeBeadClient) Ready(ctx context.Context) (*bead.Bead, error) {
	return nil, nil
}
func (f *fakeBeadClient) ReadyExcluding(ctx context.Context, excludeIDs map[string]bool) (*bead.Bead, error) {
	return nil, nil
}
func (f *fakeBeadClient) ReadyWithLabel(ctx context.Context, label string) (*bead.Bead, error) {
	return nil, nil
}
func (f *fakeBeadClient) ListWithLabel(ctx context.Context, label string) ([]*bead.Bead, error) {
	if f.listFn == nil {
		return nil, nil
	}
	return f.listFn(ctx, label)
}
func (f *fakeBeadClient) Show(ctx context.Context, id string) (*bead.Bead, error) {
	return nil, nil
}
func (f *fakeBeadClient) Close(ctx context.Context, id string) error {
	return nil
}
func (f *fakeBeadClient) Sync(ctx context.Context) error {
	return nil
}
func (f *fakeBeadClient) AddComment(ctx context.Context, id, comment string) error {
	return nil
}
func (f *fakeBeadClient) GetParent(ctx context.Context, b *bead.Bead) (*bead.Bead, error) {
	return nil, nil
}
func (f *fakeBeadClient) Create(ctx context.Context, title string, priority int, labels []string, outputs []string) (*bead.Bead, error) {
	return nil, nil
}
func (f *fakeBeadClient) CreateWithParent(ctx context.Context, title string, priority int, labels []string, outputs []string, parentID string) (*bead.Bead, error) {
	return nil, nil
}
func (f *fakeBeadClient) CreateWithParentAndDescription(ctx context.Context, title string, priority int, labels []string, outputs []string, parentID string, description string) (*bead.Bead, error) {
	return nil, nil
}
func (f *fakeBeadClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return false, nil
}

// trackingReadinessProvider records calls to Run() for testing.
type trackingReadinessProvider struct {
	runCalled      bool
	capturedPrompt string
	capturedTier   string
}

func (p *trackingReadinessProvider) Name() string                    { return "tracking" }
func (p *trackingReadinessProvider) ModelForTier(tier string) string { return "sonnet" }
func (p *trackingReadinessProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	p.runCalled = true
	p.capturedPrompt = prompt
	p.capturedTier = tier
	return &provider.Result{Success: true, Output: "READY"}, nil
}
func (p *trackingReadinessProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Success: true, Output: "READY"}, nil
}
func (p *trackingReadinessProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}
func (p *trackingReadinessProvider) IsUsageLimitError(result *provider.Result, err error) bool { return false }
func (p *trackingReadinessProvider) IsValidationPassed(result *provider.Result) bool           { return result.Success }
func (p *trackingReadinessProvider) IsScopeTooLarge(result *provider.Result) (bool, string)    { return false, "" }

// readinessTrackerRouter returns a tracked provider for testing.
type readinessTrackerRouter struct {
	provider provider.Provider
}

func (r *readinessTrackerRouter) Select(phase, tier string) (provider.Provider, string) {
	return r.provider, "test-model"
}

type dummyGitOpsForSpec struct{}

func (d *dummyGitOpsForSpec) RebaseOnto(ctx context.Context, branch, onto string) error { return nil }
func (d *dummyGitOpsForSpec) FastForwardMerge(ctx context.Context, branch string) error { return nil }
func (d *dummyGitOpsForSpec) DeleteBranch(ctx context.Context, branch string) error     { return nil }

type dummyResolverForSpec struct{}

func (d *dummyResolverForSpec) Resolve(ctx context.Context, branch string, cause error) error {
	return nil
}

// TestReadinessAdapterWithLLM_CreatesInstanceWithDependencies verifies the adapter can be instantiated.
func TestReadinessAdapterWithLLM_CreatesInstanceWithDependencies(t *testing.T) {
	t.Parallel()
	renderer := &dummyPromptRenderer{}
	router := &dummyRouter{}
	assessor := NewReadinessAdapterWithLLM(renderer, router)
	if assessor == nil {
		t.Fatal("expected non-nil readiness adapter with LLM")
	}
}

// TestReadinessAdapterWithLLM_AssessValidatesNilRenderer tests that Assess validates nil renderer dependency.
func TestReadinessAdapterWithLLM_AssessValidatesNilRenderer(t *testing.T) {
	t.Parallel()
	a := &readinessAdapterWithLLM{
		renderer: nil,
		router:   &dummyRouter{},
	}
	ctx := context.Background()
	b := &bead.Bead{ID: "test-bead"}
	assessment, err := a.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error for nil renderer: %v", err)
	}
	if assessment.Status != readiness.StatusNotReady {
		t.Fatalf("Assess returned status %q, want %q for nil renderer", assessment.Status, readiness.StatusNotReady)
	}
}

// TestReadinessAdapterWithLLM_ImplementsAssessor verifies the adapter implements readiness.Assessor.
func TestReadinessAdapterWithLLM_ImplementsAssessor(t *testing.T) {
	t.Parallel()
	renderer := &dummyPromptRenderer{}
	router := &dummyRouter{}
	adapter := NewReadinessAdapterWithLLM(renderer, router)
	if adapter == nil {
		t.Fatal("adapter should not be nil")
	}
	// This test verifies that *readinessAdapterWithLLM can be used where readiness.Assessor is expected.
	// This is a compile-time check that the Assess method exists and has the right signature.
	var _ readiness.Assessor = adapter
}

// TestPromptRenderer_RenderReadinessMethodExists tests that Renderer has RenderReadiness method.
func TestPromptRenderer_RenderReadinessMethodExists(t *testing.T) {
	t.Parallel()
	// This test verifies that prompt.Renderer can be used as readinessPromptRenderer
	// If this fails at compile time, we need to add RenderReadiness method to Renderer
	var r *prompt.Renderer
	var _ readinessPromptRenderer = r
}

// TestReadinessAdapterWithLLM_AssessCallsRouterForProvider tests that Assess invokes router.Select.
func TestReadinessAdapterWithLLM_AssessCallsRouterForProvider(t *testing.T) {
	t.Parallel()
	renderer := &dummyPromptRenderer{}
	router := &trackingRouter{}
	adapter := NewReadinessAdapterWithLLM(renderer, router)

	ctx := context.Background()
	b := &bead.Bead{ID: "test-bead", Title: "Test Task", ExpectedOutputs: []string{"deliverable"}}

	_, err := adapter.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}
	if router.phase == "" {
		t.Fatal("Assess should have called router.Select with readiness phase")
	}
}

// TestReadinessAdapterWithLLM_AssessHandlesFailedRenderingGracefully tests that Assess fails closed when rendering fails.
func TestReadinessAdapterWithLLM_AssessHandlesFailedRenderingGracefully(t *testing.T) {
	t.Parallel()
	failingRenderer := &failingReadinessRenderer{}
	router := &dummyRouter{}
	adapter := NewReadinessAdapterWithLLM(failingRenderer, router)

	ctx := context.Background()
	b := &bead.Bead{ID: "test-bead", Title: "Test Task"}

	assessment, err := adapter.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}
	// Fail closed: when rendering fails, should return NotReady
	if assessment.Status != readiness.StatusNotReady {
		t.Fatalf("Assess returned status %q, want %q for failing renderer", assessment.Status, readiness.StatusNotReady)
	}
}

func TestReadinessAdapterWithLLM_AssessShortCircuitsMissingCriteria(t *testing.T) {
	t.Parallel()
	renderer := &dummyPromptRenderer{}
	router := &trackingRouter{}
	adapter := NewReadinessAdapterWithLLM(renderer, router)

	ctx := context.Background()
	b := &bead.Bead{ID: "test-bead", Title: "Missing Criteria"}

	assessment, err := adapter.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}
	if assessment.Status != readiness.StatusNotReady {
		t.Fatalf("Assess returned status %q, want %q", assessment.Status, readiness.StatusNotReady)
	}
	if assessment.Reason != "criteria_count" {
		t.Fatalf("Assess returned reason %q, want %q", assessment.Reason, "criteria_count")
	}
	if router.phase != "" {
		t.Fatalf("router.Select called despite missing criteria: phase=%q", router.phase)
	}
}

// TestReadinessAdapterWithLLM_AssessShortCircuitsZeroCriteriaCount tests short-circuit for 0 criteria.
func TestReadinessAdapterWithLLM_AssessShortCircuitsZeroCriteriaCount(t *testing.T) {
	t.Parallel()
	renderer := &dummyPromptRenderer{}
	router := &trackingRouter{}
	adapter := NewReadinessAdapterWithLLM(renderer, router)

	ctx := context.Background()
	b := &bead.Bead{ID: "test-bead", Title: "Zero Criteria"}

	assessment, err := adapter.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}
	if assessment.Status != readiness.StatusNotReady {
		t.Fatalf("Assess returned status %q, want %q", assessment.Status, readiness.StatusNotReady)
	}
	if assessment.Reason != "criteria_count" {
		t.Fatalf("Assess returned reason %q, want %q", assessment.Reason, "criteria_count")
	}
	if router.phase != "" {
		t.Fatalf("router.Select should not be called for criteria_count short-circuit: phase=%q", router.phase)
	}
}

// TestReadinessAdapterWithLLM_AssessShortCircuitsTooManyCriteria tests short-circuit for > 3 criteria.
func TestReadinessAdapterWithLLM_AssessShortCircuitsTooManyCriteria(t *testing.T) {
	t.Parallel()
	renderer := &dummyPromptRenderer{}
	router := &trackingRouter{}
	adapter := NewReadinessAdapterWithLLM(renderer, router)

	ctx := context.Background()
	b := &bead.Bead{
		ID:    "test-bead",
		Title: "Too Many Criteria",
		ExpectedOutputs: []string{"output1", "output2", "output3", "output4"},
	}

	assessment, err := adapter.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}
	if assessment.Status != readiness.StatusNotReady {
		t.Fatalf("Assess returned status %q, want %q", assessment.Status, readiness.StatusNotReady)
	}
	if assessment.Reason != "criteria_count" {
		t.Fatalf("Assess returned reason %q, want %q", assessment.Reason, "criteria_count")
	}
	if router.phase != "" {
		t.Fatalf("router.Select should not be called for criteria_count short-circuit: phase=%q", router.phase)
	}
}

func TestParseReadinessResponse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		input string
		want readiness.Assessment
	}{
		{
			name: "ready",
			input: "READY",
			want: readiness.Assessment{Status: readiness.StatusReady},
		},
		{
			name: "criteria missing",
			input: "NOT_READY_CRITERIA_criteria_missing",
			want: readiness.Assessment{Status: readiness.StatusNotReady, Reason: prepare.ReasonCriteriaMissing},
		},
		{
			name: "scope too broad",
			input: "NOT_READY_SCOPE_scope_too_broad",
			want: readiness.Assessment{Status: readiness.StatusNotReady, Reason: prepare.ReasonScopeTooBroad},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseReadinessResponse(tc.input)
			if err != nil {
				t.Fatalf("parseReadinessResponse(%q) returned error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseReadinessResponse(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}
}

// TestReadinessAdapterWithLLM_AssessInvokesProviderWithRenderedPrompt verifies that Assess calls provider.Run() with the rendered prompt.
func TestReadinessAdapterWithLLM_AssessInvokesProviderWithRenderedPrompt(t *testing.T) {
	t.Parallel()
	renderer := &dummyPromptRenderer{}
	trackerProvider := &trackingReadinessProvider{}
	router := &readinessTrackerRouter{provider: trackerProvider}
	adapter := NewReadinessAdapterWithLLM(renderer, router)

	ctx := context.Background()
	b := &bead.Bead{ID: "test-bead", Title: "Test Task", ExpectedOutputs: []string{"deliverable"}}

	_, err := adapter.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}

	if !trackerProvider.runCalled {
		t.Fatal("expected provider.Run() to be called")
	}
	if trackerProvider.capturedPrompt != "readiness_prompt" {
		t.Fatalf("provider.Run() called with prompt %q, want %q", trackerProvider.capturedPrompt, "readiness_prompt")
	}
}
