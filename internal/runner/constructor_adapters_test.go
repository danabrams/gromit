package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/integrationqueue"
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

type dummyRouter struct {
	phase string
	tier  string
}

func (d *dummyRouter) Select(phase, tier string) (provider.Provider, string) {
	d.phase = phase
	d.tier = tier
	return nil, "dummy-model"
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

func TestDeterministicReadinessAssessor_BlocksMissingCriteria(t *testing.T) {
	t.Parallel()
	assessor := NewDeterministicReadinessAssessor()
	ctx := context.Background()
	b := &bead.Bead{ID: "missing-criteria"}

	assessment, err := assessor.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}
	if assessment.Status != readiness.StatusNotReady {
		t.Fatalf("status = %q, want %q", assessment.Status, readiness.StatusNotReady)
	}
	if assessment.Reason != prepare.ReasonCriteriaMissing {
		t.Fatalf("reason = %q, want %q", assessment.Reason, prepare.ReasonCriteriaMissing)
	}
}

func TestDeterministicReadinessAssessor_BlocksOversizedScope(t *testing.T) {
	t.Parallel()
	assessor := NewDeterministicReadinessAssessor()
	ctx := context.Background()
	b := &bead.Bead{
		ID:              "oversized-scope",
		ExpectedOutputs: []string{"one", "two", "three", "four", "five", "six"},
	}

	assessment, err := assessor.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}
	if assessment.Status != readiness.StatusNotReady {
		t.Fatalf("status = %q, want %q", assessment.Status, readiness.StatusNotReady)
	}
	if assessment.Reason != prepare.ReasonScopeTooBroad {
		t.Fatalf("reason = %q, want %q", assessment.Reason, prepare.ReasonScopeTooBroad)
	}
}

func TestDeterministicReadinessAssessor_BlocksAmbiguousCriteria(t *testing.T) {
	t.Parallel()
	assessor := NewDeterministicReadinessAssessor()
	ctx := context.Background()
	b := &bead.Bead{
		ID:              "ambiguous-criteria",
		ExpectedOutputs: []string{"a", "b", "c", "d"},
	}

	assessment, err := assessor.Assess(ctx, b)
	if err != nil {
		t.Fatalf("Assess returned error: %v", err)
	}
	if assessment.Status != readiness.StatusNotReady {
		t.Fatalf("status = %q, want %q", assessment.Status, readiness.StatusNotReady)
	}
	if assessment.Reason != prepare.ReasonCriteriaAmbiguous {
		t.Fatalf("reason = %q, want %q", assessment.Reason, prepare.ReasonCriteriaAmbiguous)
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
	listFn   func(ctx context.Context, label string) ([]*bead.Bead, error)
	createFn func(ctx context.Context, title string, priority int, labels []string, outputs []string, parentID string) (*bead.Bead, error)
	closeFn  func(ctx context.Context, id string) error
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
	if f.closeFn == nil {
		return nil
	}
	return f.closeFn(ctx, id)
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
	if f.createFn == nil {
		return nil, nil
	}
	return f.createFn(ctx, title, priority, labels, outputs, parentID)
}
func (f *fakeBeadClient) CreateWithParentAndDescription(ctx context.Context, title string, priority int, labels []string, outputs []string, parentID string, description string) (*bead.Bead, error) {
	return nil, nil
}
func (f *fakeBeadClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return false, nil
}

type dummyGitOpsForSpec struct{}

func (d *dummyGitOpsForSpec) RebaseOnto(ctx context.Context, branch, onto string) error { return nil }
func (d *dummyGitOpsForSpec) FastForwardMerge(ctx context.Context, branch string) error { return nil }
func (d *dummyGitOpsForSpec) DeleteBranch(ctx context.Context, branch string) error     { return nil }

type dummyResolverForSpec struct{}

func (d *dummyResolverForSpec) Resolve(ctx context.Context, branch string, cause error) error {
	return nil
}

// TestDecomposerAdapter_Decompose_ThreadsContextToChildDuplicateCheck verifies that
// Decompose uses the context-aware childWithDedupeLabelExistsWithClient method when
// checking for existing child beads, threading the provided context through.
func TestDecomposerAdapter_Decompose_ThreadsContextToChildDuplicateCheck(t *testing.T) {
	t.Parallel()

	// Create a custom context with a value to verify it's threaded through
	testValue := "test-decompose-context"
	customCtx := context.WithValue(context.Background(), "test-key", testValue)

	var capturedCtx context.Context
	adapter := &decomposerAdapter{
		router: provider.NewSingleProviderRouter(&stubRunProvider{
			name: "test-provider",
			runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
				return &provider.Result{
					Success: true,
					Output:  `[{"title":"Part 1","expected_outputs":["f1"]},{"title":"Part 2","expected_outputs":["f2"]}]`,
				}, nil
			},
		}),
		beads: &fakeBeadClient{
			listFn: func(ctx context.Context, label string) ([]*bead.Bead, error) {
				capturedCtx = ctx
				// Return no existing children so the bead gets created
				return nil, nil
			},
			createFn: func(ctx context.Context, title string, priority int, labels []string, outputs []string, parentID string) (*bead.Bead, error) {
				return &bead.Bead{ID: "child-1", Title: title, Parent: parentID}, nil
			},
			closeFn: func(ctx context.Context, id string) error {
				return nil
			},
		},
	}

	parent := &bead.Bead{
		ID:       "parent-1",
		Title:    "Oversized Feature",
		Priority: 1,
	}

	err := adapter.Decompose(customCtx, parent)
	if err != nil {
		t.Fatalf("Decompose returned error: %v", err)
	}

	// Verify the context was threaded through to the duplicate check
	if capturedCtx == nil {
		t.Fatal("context was not captured by ListWithLabel - Decompose not threading context to childWithDedupeLabelExistsWithClient")
	}
	if capturedCtx.Value("test-key") != testValue {
		t.Fatal("context value not found in captured context - context not properly threaded through duplicate check")
	}
}

func TestIntegrationQueueGitOpsAdapter_FetchAndRebaseCommands(t *testing.T) {
	t.Parallel()

	repoDir := "/repo"
	calls := make([]string, 0, 3)
	adapter := &integrationQueueGitOpsAdapter{
		repoDir:    repoDir,
		baseBranch: "main",
		runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
			if dir != repoDir {
				t.Fatalf("dir = %q, want %q", dir, repoDir)
			}
			calls = append(calls, strings.Join(args, " "))
			return "", nil
		},
	}

	entry := integrationqueue.Entry{Branch: "feature/fetch"}
	if err := adapter.FetchAndRebase(context.Background(), entry); err != nil {
		t.Fatalf("FetchAndRebase returned error: %v", err)
	}

	want := []string{
		"fetch origin main",
		"checkout feature/fetch",
		"rebase main",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("commands = %v, want %v", calls, want)
	}
}

func TestIntegrationQueueGitOpsAdapter_FetchAndRebaseTrimsBranch(t *testing.T) {
	t.Parallel()

	repoDir := "/repo"
	calls := make([]string, 0, 3)
	adapter := &integrationQueueGitOpsAdapter{
		repoDir:    repoDir,
		baseBranch: "main",
		runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
			if dir != repoDir {
				t.Fatalf("dir = %q, want %q", dir, repoDir)
			}
			calls = append(calls, strings.Join(args, " "))
			return "", nil
		},
	}

	entry := integrationqueue.Entry{Branch: " feature/trim "}
	if err := adapter.FetchAndRebase(context.Background(), entry); err != nil {
		t.Fatalf("FetchAndRebase returned error: %v", err)
	}

	want := []string{
		"fetch origin main",
		"checkout feature/trim",
		"rebase main",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("commands = %v, want %v", calls, want)
	}
}

func TestIntegrationQueueGitOpsAdapter_MergeToMainCommands(t *testing.T) {
	t.Parallel()

	repoDir := "/repo"
	calls := make([]string, 0, 2)
	adapter := &integrationQueueGitOpsAdapter{
		repoDir:    repoDir,
		baseBranch: "main",
		runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
			if dir != repoDir {
				t.Fatalf("dir = %q, want %q", dir, repoDir)
			}
			calls = append(calls, strings.Join(args, " "))
			return "", nil
		},
	}

	entry := integrationqueue.Entry{Branch: "feature/merge"}
	if err := adapter.MergeToMain(context.Background(), entry); err != nil {
		t.Fatalf("MergeToMain returned error: %v", err)
	}

	want := []string{
		"checkout main",
		"merge --ff-only feature/merge",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("commands = %v, want %v", calls, want)
	}
}

func TestIntegrationQueueGitOpsAdapter_MergeToMainTrimsBranch(t *testing.T) {
	t.Parallel()

	repoDir := "/repo"
	calls := make([]string, 0, 2)
	adapter := &integrationQueueGitOpsAdapter{
		repoDir:    repoDir,
		baseBranch: "main",
		runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
			if dir != repoDir {
				t.Fatalf("dir = %q, want %q", dir, repoDir)
			}
			calls = append(calls, strings.Join(args, " "))
			return "", nil
		},
	}

	entry := integrationqueue.Entry{Branch: " feature/merge "}
	if err := adapter.MergeToMain(context.Background(), entry); err != nil {
		t.Fatalf("MergeToMain returned error: %v", err)
	}

	want := []string{
		"checkout main",
		"merge --ff-only feature/merge",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("commands = %v, want %v", calls, want)
	}
}

func TestIntegrationQueueGitOpsAdapter_PushCommands(t *testing.T) {
	t.Parallel()

	repoDir := "/repo"
	calls := make([]string, 0, 1)
	adapter := &integrationQueueGitOpsAdapter{
		repoDir:    repoDir,
		baseBranch: "main",
		runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
			if dir != repoDir {
				t.Fatalf("dir = %q, want %q", dir, repoDir)
			}
			calls = append(calls, strings.Join(args, " "))
			return "", nil
		},
	}

	if err := adapter.Push(context.Background()); err != nil {
		t.Fatalf("Push returned error: %v", err)
	}

	want := []string{"push origin main"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("commands = %v, want %v", calls, want)
	}
}

func TestIntegrationQueueGitOpsAdapter_PushCommandFailure(t *testing.T) {
	t.Parallel()

	repoDir := "/repo"
	adapter := &integrationQueueGitOpsAdapter{
		repoDir:    repoDir,
		baseBranch: "main",
		runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
			return "", fmt.Errorf("git command failed")
		},
	}

	err := adapter.Push(context.Background())
	if err == nil {
		t.Fatal("Push returned nil error; want failure when git command fails")
	}
	if !errors.Is(err, errGitPushFailed) {
		t.Fatalf("error = %v, want wrapped %v", err, errGitPushFailed)
	}
	if !strings.Contains(err.Error(), "pushing main") {
		t.Fatalf("error %q missing \"pushing main\"", err)
	}
}

func TestIntegrationQueueGitOpsAdapter_CleanupCommands(t *testing.T) {
	t.Parallel()

	repoDir := "/repo"
	var calls []string
	adapter := &integrationQueueGitOpsAdapter{
		repoDir: repoDir,
		runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
			if dir != repoDir {
				t.Fatalf("dir = %q, want %q", dir, repoDir)
			}
			calls = append(calls, strings.Join(args, " "))
			return "", nil
		},
	}

	entry := integrationqueue.Entry{Branch: "feature/cleanup"}
	if err := adapter.Cleanup(context.Background(), entry); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}

	want := []string{"branch -D feature/cleanup"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("commands = %v, want %v", calls, want)
	}
}

func TestIntegrationQueueGitOpsAdapter_CleanupCommandFailure(t *testing.T) {
	t.Parallel()

	repoDir := "/repo"
	adapter := &integrationQueueGitOpsAdapter{
		repoDir: repoDir,
		runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
			return "", fmt.Errorf("git command failed")
		},
	}

	entry := integrationqueue.Entry{Branch: "feature/cleanup"}
	err := adapter.Cleanup(context.Background(), entry)
	if err == nil {
		t.Fatal("Cleanup returned nil error; want failure when git command fails")
	}
	if !errors.Is(err, errGitCleanupFailed) {
		t.Fatalf("error = %v, want wrapped %v", err, errGitCleanupFailed)
	}
	if !strings.Contains(err.Error(), "cleanup branch feature/cleanup") {
		t.Fatalf("error %q missing \"cleanup branch feature/cleanup\"", err)
	}
}

func TestIntegrationQueueGitOpsAdapter_RequiresRepoDir(t *testing.T) {
	t.Parallel()

	adapter := &integrationQueueGitOpsAdapter{
		runGitCommand: func(ctx context.Context, dir string, args ...string) (string, error) {
			t.Fatalf("runGitCommand called even though repo dir is missing")
			return "", nil
		},
	}

	entry := integrationqueue.Entry{Branch: "feature/guard"}
	err := adapter.FetchAndRebase(context.Background(), entry)
	if err == nil {
		t.Fatalf("FetchAndRebase returned nil error; want failure when repo dir missing")
	}
	if !strings.Contains(err.Error(), "repo dir") {
		t.Fatalf("error = %v, want message mentioning repo dir", err)
	}
}

func TestIntegrationQueueScopedGateAdapter_RunSuccess(t *testing.T) {
	t.Parallel()

	var seen integrationqueue.Entry
	adapter := &integrationQueueScopedGateAdapter{
		evaluator: func(ctx context.Context, entry integrationqueue.Entry) error {
			seen = entry
			return nil
		},
	}

	entry := integrationqueue.Entry{Branch: "feature/gate"}
	if err := adapter.Run(context.Background(), entry); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if seen.Branch != entry.Branch {
		t.Fatalf("seen branch = %q, want %q", seen.Branch, entry.Branch)
	}
}

func TestIntegrationQueueScopedGateAdapter_RunFailure(t *testing.T) {
	t.Parallel()

	adapter := &integrationQueueScopedGateAdapter{
		evaluator: func(ctx context.Context, entry integrationqueue.Entry) error {
			return fmt.Errorf("gate error")
		},
	}

	entry := integrationqueue.Entry{Branch: "feature/gate"}
	err := adapter.Run(context.Background(), entry)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), entry.Branch) {
		t.Fatalf("error %q missing branch %q", err, entry.Branch)
	}
}
