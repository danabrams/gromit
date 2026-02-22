//go:build acceptance

// Package acceptance_test contains acceptance tests for the runner pipeline.
// These tests verify behavioral correctness through the runner's public API,
// without depending on internal implementation details.
package acceptance_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner"
)

// --- mockBeadClient ---

type mockBeadClient struct {
	ReadyFn                          func() (*bead.Bead, error)
	ReadyExcludingFn                 func(excludeIDs map[string]bool) (*bead.Bead, error)
	ReadyWithLabelFn                 func(label string) (*bead.Bead, error)
	ListWithLabelFn                  func(label string) ([]*bead.Bead, error)
	ShowFn                           func(id string) (*bead.Bead, error)
	CloseFn                          func(id string) error
	SyncFn                           func() error
	AddCommentFn                     func(id, comment string) error
	GetParentFn                      func(b *bead.Bead) (*bead.Bead, error)
	CreateFn                         func(title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error)
	CreateWithParentFn               func(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error)
	CreateWithParentAndDescriptionFn func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error)
	HasOpenChildrenFn                func(parentID string) (bool, error)

	ClosedIDs []string
	SyncCalls int
	Comments  []mockComment
}

type mockComment struct {
	ID      string
	Comment string
}

func (m *mockBeadClient) Ready() (*bead.Bead, error) {
	if m.ReadyFn != nil {
		return m.ReadyFn()
	}
	return nil, nil
}

func (m *mockBeadClient) ReadyExcluding(excludeIDs map[string]bool) (*bead.Bead, error) {
	if m.ReadyExcludingFn != nil {
		return m.ReadyExcludingFn(excludeIDs)
	}
	if m.ReadyFn != nil {
		b, err := m.ReadyFn()
		if err != nil || b == nil {
			return nil, err
		}
		if excludeIDs[b.ID] {
			return nil, nil
		}
		return b, nil
	}
	return nil, nil
}

func (m *mockBeadClient) ReadyWithLabel(label string) (*bead.Bead, error) {
	if m.ReadyWithLabelFn != nil {
		return m.ReadyWithLabelFn(label)
	}
	return nil, nil
}

func (m *mockBeadClient) ListWithLabel(label string) ([]*bead.Bead, error) {
	if m.ListWithLabelFn != nil {
		return m.ListWithLabelFn(label)
	}
	return []*bead.Bead{}, nil
}

func (m *mockBeadClient) Show(id string) (*bead.Bead, error) {
	if m.ShowFn != nil {
		return m.ShowFn(id)
	}
	return nil, nil
}

func (m *mockBeadClient) Close(id string) error {
	m.ClosedIDs = append(m.ClosedIDs, id)
	if m.CloseFn != nil {
		return m.CloseFn(id)
	}
	return nil
}

func (m *mockBeadClient) Sync() error {
	m.SyncCalls++
	if m.SyncFn != nil {
		return m.SyncFn()
	}
	return nil
}

func (m *mockBeadClient) AddComment(id, comment string) error {
	m.Comments = append(m.Comments, mockComment{ID: id, Comment: comment})
	if m.AddCommentFn != nil {
		return m.AddCommentFn(id, comment)
	}
	return nil
}

func (m *mockBeadClient) GetParent(b *bead.Bead) (*bead.Bead, error) {
	if m.GetParentFn != nil {
		return m.GetParentFn(b)
	}
	return nil, nil
}

func (m *mockBeadClient) Create(title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error) {
	if m.CreateFn != nil {
		return m.CreateFn(title, priority, labels, expectedOutputs)
	}
	return &bead.Bead{ID: "mock-create-1", Title: title, Labels: []string{}, ExpectedOutputs: []string{}}, nil
}

func (m *mockBeadClient) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	if m.CreateWithParentFn != nil {
		return m.CreateWithParentFn(title, priority, labels, expectedOutputs, parentID)
	}
	return &bead.Bead{ID: "mock-sub-1", Title: title, Labels: []string{}, ExpectedOutputs: []string{}}, nil
}

func (m *mockBeadClient) CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
	if m.CreateWithParentAndDescriptionFn != nil {
		return m.CreateWithParentAndDescriptionFn(title, priority, labels, expectedOutputs, parentID, description)
	}
	return &bead.Bead{ID: "mock-sub-1", Title: title, Description: description, Labels: []string{}, ExpectedOutputs: []string{}}, nil
}

func (m *mockBeadClient) HasOpenChildren(parentID string) (bool, error) {
	if m.HasOpenChildrenFn != nil {
		return m.HasOpenChildrenFn(parentID)
	}
	return false, nil
}

// --- mockProviderWithRouterTracking ---

type mockProviderWithRouterTracking struct {
	name            string
	onSelect        func(phase, tier string)
	runFn           func(ctx context.Context, prompt, tier string) (*provider.Result, error)
	streamRunFn     func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
	streamRunResult *provider.Result
}

func (m *mockProviderWithRouterTracking) Name() string {
	if m.name != "" {
		return m.name
	}
	return "test-provider"
}

func (m *mockProviderWithRouterTracking) ModelForTier(tier string) string {
	switch tier {
	case provider.TierHigh:
		return "test-opus"
	case provider.TierMedium:
		return "test-sonnet"
	case provider.TierLow:
		return "test-haiku"
	default:
		return "test-model"
	}
}

func (m *mockProviderWithRouterTracking) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.onSelect != nil {
		m.onSelect("", tier)
	}
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProviderWithRouterTracking) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	if m.streamRunResult != nil {
		return m.streamRunResult, nil
	}
	return &provider.Result{Success: true, Model: "test-model"}, nil
}

func (m *mockProviderWithRouterTracking) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "test-model", Output: "VALIDATION_PASSED"}, nil
}

func (m *mockProviderWithRouterTracking) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

func (m *mockProviderWithRouterTracking) IsValidationPassed(result *provider.Result) bool {
	return result.Success
}

func (m *mockProviderWithRouterTracking) IsScopeTooLarge(result *provider.Result) (bool, string) {
	return false, ""
}

// --- mockFailureAnalyzer ---

type mockFailureAnalyzer struct {
	AnalyzeFn    func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error)
	AnalyzeCalls int
}

func (m *mockFailureAnalyzer) Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
	m.AnalyzeCalls++
	if m.AnalyzeFn != nil {
		return m.AnalyzeFn(ctx, b, failureOutput)
	}
	return &analyzer.Analysis{
		Category:    analyzer.CategoryLogic,
		Recoverable: false,
		RootCause:   "test failure",
	}, nil
}

// --- mockPromptRenderer ---

type mockPromptRenderer struct {
	BuildContextFn          func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error)
	RenderBuildFn           func(ctx *prompt.Context) (string, error)
	RenderAnalyzeFn         func(ctx *prompt.AnalyzeContext) (string, error)
	RenderLearnFn           func(ctx *prompt.LearnContext) (string, error)
	RenderDecomposeFn       func(ctx *prompt.DecomposeContext) (string, error)
	RenderScopeFn           func(ctx *prompt.ScopeContext) (string, error)
	RenderPrecheckFn        func(ctx *prompt.PrecheckContext) (string, error)
	RenderSpecAcceptanceFn  func(ctx *prompt.SpecAcceptanceContext) (string, error)
	RenderSpecGateFn        func(ctx *prompt.SpecGateContext) (string, error)
	RenderReviewFn          func(ctx *prompt.ReviewContext) (string, error)
	RenderThoroughReviewFn  func(ctx *prompt.ThoroughReviewContext) (string, error)
	RenderAcceptanceTestsFn func(ctx *prompt.Context) (string, error)
	RenderATDDBuildFn       func(ctx *prompt.Context) (string, error)
	RenderATDDDiagnosticFn  func(ctx *prompt.DiagnosticContext) (string, error)
	RenderTDDBuildFn        func(ctx *prompt.Context) (string, error)
	RenderTDDRedFn          func(ctx *prompt.TDDRedContext) (string, error)
	RenderTDDGreenFn        func(ctx *prompt.TDDGreenContext) (string, error)
	RenderRefactorFn        func(ctx *prompt.Context) (string, error)
	RenderTestFixFn         func(ctx *prompt.TestFixContext) (string, error)
	RenderCoverageValidFn   func(ctx *prompt.CoverageValidationContext) (string, error)
	LoadSpecFn              func(name string) (string, error)
	LoadClaudeMDFn          func() (string, error)
	LoadRulesFn             func() (string, error)
	LoadRulesForPhaseFn     func(phase string) (string, error)
	SetSiblingResolverFn    func(resolver prompt.SiblingTouchedPackagesResolver)
	LastDiagnosticsFn       func() *prompt.PromptDiagnostics
	LearningsFile           *learnings.File
}

func (m *mockPromptRenderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
	if m.BuildContextFn != nil {
		return m.BuildContextFn(b, parent, iteration, model)
	}
	return &prompt.Context{
		Bead:               b,
		ParentBead:         parent,
		Iteration:          iteration,
		Model:              model,
		ConfirmedLearnings: []learnings.Learning{},
		RecentLearnings:    []learnings.Learning{},
	}, nil
}

func (m *mockPromptRenderer) RenderBuild(ctx *prompt.Context) (string, error) {
	if m.RenderBuildFn != nil {
		return m.RenderBuildFn(ctx)
	}
	return "mock build prompt", nil
}

func (m *mockPromptRenderer) RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error) {
	if m.RenderAnalyzeFn != nil {
		return m.RenderAnalyzeFn(ctx)
	}
	return "mock analyze prompt", nil
}

func (m *mockPromptRenderer) RenderLearn(ctx *prompt.LearnContext) (string, error) {
	if m.RenderLearnFn != nil {
		return m.RenderLearnFn(ctx)
	}
	return "mock learn prompt", nil
}

func (m *mockPromptRenderer) RenderDecompose(ctx *prompt.DecomposeContext) (string, error) {
	if m.RenderDecomposeFn != nil {
		return m.RenderDecomposeFn(ctx)
	}
	return "mock decompose prompt", nil
}

func (m *mockPromptRenderer) RenderScope(ctx *prompt.ScopeContext) (string, error) {
	if m.RenderScopeFn != nil {
		return m.RenderScopeFn(ctx)
	}
	return "mock scope prompt", nil
}

func (m *mockPromptRenderer) RenderPrecheck(ctx *prompt.PrecheckContext) (string, error) {
	if m.RenderPrecheckFn != nil {
		return m.RenderPrecheckFn(ctx)
	}
	return "mock precheck prompt", nil
}

func (m *mockPromptRenderer) RenderSpecAcceptance(ctx *prompt.SpecAcceptanceContext) (string, error) {
	if m.RenderSpecAcceptanceFn != nil {
		return m.RenderSpecAcceptanceFn(ctx)
	}
	return "mock spec acceptance prompt", nil
}

func (m *mockPromptRenderer) RenderSpecGate(ctx *prompt.SpecGateContext) (string, error) {
	if m.RenderSpecGateFn != nil {
		return m.RenderSpecGateFn(ctx)
	}
	return "mock spec gate prompt", nil
}

func (m *mockPromptRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	if m.RenderReviewFn != nil {
		return m.RenderReviewFn(ctx)
	}
	return "mock review prompt", nil
}

func (m *mockPromptRenderer) RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error) {
	if m.RenderThoroughReviewFn != nil {
		return m.RenderThoroughReviewFn(ctx)
	}
	return "mock thorough review prompt", nil
}

func (m *mockPromptRenderer) RenderAcceptanceTests(ctx *prompt.Context) (string, error) {
	if m.RenderAcceptanceTestsFn != nil {
		return m.RenderAcceptanceTestsFn(ctx)
	}
	return "mock acceptance tests prompt", nil
}

func (m *mockPromptRenderer) RenderATDDBuild(ctx *prompt.Context) (string, error) {
	if m.RenderATDDBuildFn != nil {
		return m.RenderATDDBuildFn(ctx)
	}
	return "mock atdd build prompt", nil
}

func (m *mockPromptRenderer) RenderATDDDiagnostic(ctx *prompt.DiagnosticContext) (string, error) {
	if m.RenderATDDDiagnosticFn != nil {
		return m.RenderATDDDiagnosticFn(ctx)
	}
	return "mock atdd diagnostic prompt", nil
}

func (m *mockPromptRenderer) RenderTDDBuild(ctx *prompt.Context) (string, error) {
	if m.RenderTDDBuildFn != nil {
		return m.RenderTDDBuildFn(ctx)
	}
	return "mock tdd build prompt", nil
}

func (m *mockPromptRenderer) RenderTDDRed(ctx *prompt.TDDRedContext) (string, error) {
	if m.RenderTDDRedFn != nil {
		return m.RenderTDDRedFn(ctx)
	}
	return "mock tdd red prompt", nil
}

func (m *mockPromptRenderer) RenderTDDGreen(ctx *prompt.TDDGreenContext) (string, error) {
	if m.RenderTDDGreenFn != nil {
		return m.RenderTDDGreenFn(ctx)
	}
	return "mock tdd green prompt", nil
}

func (m *mockPromptRenderer) RenderRefactor(ctx *prompt.Context) (string, error) {
	if m.RenderRefactorFn != nil {
		return m.RenderRefactorFn(ctx)
	}
	return "mock refactor prompt", nil
}

func (m *mockPromptRenderer) RenderTestFix(ctx *prompt.TestFixContext) (string, error) {
	if m.RenderTestFixFn != nil {
		return m.RenderTestFixFn(ctx)
	}
	return "mock test fix prompt", nil
}

func (m *mockPromptRenderer) RenderCoverageValidation(ctx *prompt.CoverageValidationContext) (string, error) {
	if m.RenderCoverageValidFn != nil {
		return m.RenderCoverageValidFn(ctx)
	}
	return "mock coverage validation prompt", nil
}

func (m *mockPromptRenderer) LoadSpec(name string) (string, error) {
	if m.LoadSpecFn != nil {
		return m.LoadSpecFn(name)
	}
	return "", nil
}

func (m *mockPromptRenderer) LoadClaudeMD() (string, error) {
	if m.LoadClaudeMDFn != nil {
		return m.LoadClaudeMDFn()
	}
	return "", nil
}

func (m *mockPromptRenderer) LoadRules() (string, error) {
	if m.LoadRulesFn != nil {
		return m.LoadRulesFn()
	}
	return "", nil
}

func (m *mockPromptRenderer) LoadRulesForPhase(phase string) (string, error) {
	if m.LoadRulesForPhaseFn != nil {
		return m.LoadRulesForPhaseFn(phase)
	}
	return "", nil
}

func (m *mockPromptRenderer) GetLearningsFile() *learnings.File {
	return m.LearningsFile
}

func (m *mockPromptRenderer) SetSiblingTouchedPackagesResolver(resolver prompt.SiblingTouchedPackagesResolver) {
	if m.SetSiblingResolverFn != nil {
		m.SetSiblingResolverFn(resolver)
	}
}

func (m *mockPromptRenderer) LastDiagnostics() *prompt.PromptDiagnostics {
	if m.LastDiagnosticsFn != nil {
		return m.LastDiagnosticsFn()
	}
	return nil
}

// --- mockIterationLogger ---

type mockIterationLogger struct {
	Logs   []*logger.IterationLog
	Closed bool
}

func (m *mockIterationLogger) LogIteration(log *logger.IterationLog) error {
	m.Logs = append(m.Logs, log)
	return nil
}

func (m *mockIterationLogger) LogReview(log *logger.ReviewLog) error {
	return nil
}

func (m *mockIterationLogger) Close() error {
	m.Closed = true
	return nil
}

func (m *mockIterationLogger) FilePath() string {
	return "/mock/logs/run-test.jsonl"
}

func (m *mockIterationLogger) RunID() string {
	return "test-run"
}

// --- mockWorktreeManager ---

type mockWorktreeManager struct {
	EnsureWorktreeFn  func() (string, error)
	CreateBranchFn    func(command string) (string, error)
	MergeBackFn       func(branch string) error
	PendingBranchesFn func() ([]string, error)
	CleanupFn         func() error
}

func (m *mockWorktreeManager) EnsureWorktree() (string, error) {
	if m.EnsureWorktreeFn != nil {
		return m.EnsureWorktreeFn()
	}
	return "/tmp/mock-worktree", nil
}

func (m *mockWorktreeManager) CreateBranch(command string) (string, error) {
	if m.CreateBranchFn != nil {
		return m.CreateBranchFn(command)
	}
	return "gromit/mock-branch", nil
}

func (m *mockWorktreeManager) MergeBack(branch string) error {
	if m.MergeBackFn != nil {
		return m.MergeBackFn(branch)
	}
	return nil
}

func (m *mockWorktreeManager) PendingBranches() ([]string, error) {
	if m.PendingBranchesFn != nil {
		return m.PendingBranchesFn()
	}
	return []string{}, nil
}

func (m *mockWorktreeManager) Cleanup() error {
	if m.CleanupFn != nil {
		return m.CleanupFn()
	}
	return nil
}

// --- router factories ---

// newMockRouter creates a simple mock router for tests that don't care about provider specifics.
func newMockRouter() *provider.Router {
	p := &mockProviderWithRouterTracking{
		streamRunResult: &provider.Result{Success: true, Model: "test-model", Output: "done"},
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "test-model",
				Output:  `[{"title": "Sub-task 1", "acceptance_criteria": ["Criterion 1"]}]`,
			}, nil
		},
	}
	return provider.NewSingleProviderRouter(p)
}

// --- worktree merge test helpers ---

// baseWorktreeMergeConfig creates a base config for worktree merge tests.
func baseWorktreeMergeConfig() *config.Config {
	precheckDisabled := false
	autoPushDisabled := false
	return &config.Config{
		Loop: config.LoopConfig{
			StopOnFailure: false,
		},
		Validation: config.ValidationConfig{
			Enabled: false,
		},
		Precheck: config.PrecheckConfig{
			Enabled: &precheckDisabled,
		},
		Review: config.ReviewConfig{
			Enabled: false,
		},
		Git: config.GitConfig{
			AutoPush: &autoPushDisabled,
		},
	}
}

// configureWorktreeMerge sets worktree settings on cfg.
func configureWorktreeMerge(cfg *config.Config, enabled bool, mode string) {
	autoMerge := true
	cfg.Worktree.Enabled = &enabled
	cfg.Worktree.AutoMerge = &autoMerge
	cfg.Worktree.MergeFailure = mode
}

// setupRunnerForWorktreeMerge creates a Runner wired for worktree merge tests.
func setupRunnerForWorktreeMerge(t *testing.T, cfg *config.Config, wm runner.WorktreeManager) *runner.Runner {
	t.Helper()
	var buf strings.Builder
	beadReady := false
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			if beadReady {
				return nil, nil
			}
			beadReady = true
			return &bead.Bead{
				ID:              "worktree-test-1",
				Title:           "Worktree test bead",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{},
			}, nil
		},
	}
	r, err := runner.NewRunnerWithDeps(cfg, &buf, t.TempDir(), runner.Deps{
		Beads:           beads,
		Router:          newMockRouter(),
		Analyzer:        &mockFailureAnalyzer{},
		Renderer:        &mockPromptRenderer{},
		Logger:          &mockIterationLogger{},
		WorktreeManager: wm,
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}
	return r
}

// Ensure unused import is used (time is used in structs above).
var _ = time.Now
