package runner

import (
	"context"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline/epilogue"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/pipeline/review"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
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

// mockTrackerClient implements tracker.Client for testing
type mockTrackerClient struct{}

func (m *mockTrackerClient) Ready(ctx context.Context) (*tracker.Item, error) { return nil, nil }
func (m *mockTrackerClient) List(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) Show(ctx context.Context, id string) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) Create(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
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
		tracker: trackerClient,
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

type dummyGitOpsForSpec struct{}

func (d *dummyGitOpsForSpec) RebaseOnto(ctx context.Context, branch, onto string) error { return nil }
func (d *dummyGitOpsForSpec) FastForwardMerge(ctx context.Context, branch string) error { return nil }
func (d *dummyGitOpsForSpec) DeleteBranch(ctx context.Context, branch string) error     { return nil }

type dummyResolverForSpec struct{}

func (d *dummyResolverForSpec) Resolve(ctx context.Context, branch string, cause error) error {
	return nil
}
