package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
)

// --- Mock implementations ---

type mockBeadClient struct {
	ReadyFn                          func() (*bead.Bead, error)
	ShowFn                           func(id string) (*bead.Bead, error)
	CloseFn                          func(id string) error
	SyncFn                           func() error
	AddCommentFn                     func(id, comment string) error
	GetParentFn                      func(b *bead.Bead) (*bead.Bead, error)
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

type mockClaudeClient struct {
	RunFn           func(ctx context.Context, prompt string, model string) (*claude.Result, error)
	StreamRunFn     func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error)
	RunValidationFn func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error)

	RunCalls        []mockClaudeCall
	StreamRunCalls  []mockClaudeCall
	ValidationCalls int
}

type mockClaudeCall struct {
	Prompt string
	Model  string
}

func (m *mockClaudeClient) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	m.RunCalls = append(m.RunCalls, mockClaudeCall{Prompt: prompt, Model: model})
	if m.RunFn != nil {
		return m.RunFn(ctx, prompt, model)
	}
	return &claude.Result{Success: true, Output: "ok"}, nil
}

func (m *mockClaudeClient) StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
	m.StreamRunCalls = append(m.StreamRunCalls, mockClaudeCall{Prompt: prompt, Model: model})
	if m.StreamRunFn != nil {
		return m.StreamRunFn(ctx, prompt, model, output, handler, onToolCall)
	}
	return &claude.Result{Success: true, Output: "ok"}, nil
}

func (m *mockClaudeClient) RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
	m.ValidationCalls++
	if m.RunValidationFn != nil {
		return m.RunValidationFn(ctx, commands, model, workDir)
	}
	return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
}

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

type mockPromptRenderer struct {
	BuildContextFn          func(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error)
	RenderBuildFn           func(ctx *prompt.Context) (string, error)
	RenderAnalyzeFn         func(ctx *prompt.AnalyzeContext) (string, error)
	RenderLearnFn           func(ctx *prompt.LearnContext) (string, error)
	RenderDecomposeFn       func(ctx *prompt.DecomposeContext) (string, error)
	RenderScopeFn           func(ctx *prompt.ScopeContext) (string, error)
	RenderPrecheckFn        func(ctx *prompt.PrecheckContext) (string, error)
	RenderReviewFn          func(ctx *prompt.ReviewContext) (string, error)
	RenderThoroughReviewFn  func(ctx *prompt.ThoroughReviewContext) (string, error)
	RenderAcceptanceTestsFn func(ctx *prompt.Context) (string, error)
	RenderATDDBuildFn       func(ctx *prompt.Context) (string, error)
	RenderTDDBuildFn        func(ctx *prompt.Context) (string, error)
	RenderRefactorFn        func(ctx *prompt.Context) (string, error)
	LoadSpecFn              func(name string) (string, error)
	LoadClaudeMDFn          func() (string, error)
	LoadRulesFn             func() (string, error)
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

func (m *mockPromptRenderer) LoadSpec(name string) (string, error) {
	if m.LoadSpecFn != nil {
		return m.LoadSpecFn(name)
	}
	return "", nil
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

func (m *mockPromptRenderer) GetLearningsFile() *learnings.File {
	return m.LearningsFile
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

func (m *mockPromptRenderer) RenderTDDBuild(ctx *prompt.Context) (string, error) {
	if m.RenderTDDBuildFn != nil {
		return m.RenderTDDBuildFn(ctx)
	}
	return "mock tdd build prompt", nil
}

func (m *mockPromptRenderer) RenderRefactor(ctx *prompt.Context) (string, error) {
	if m.RenderRefactorFn != nil {
		return m.RenderRefactorFn(ctx)
	}
	return "mock refactor prompt", nil
}

type mockIterationLogger struct {
	Logs   []*logger.IterationLog
	Closed bool
}

func (m *mockIterationLogger) LogIteration(log *logger.IterationLog) error {
	m.Logs = append(m.Logs, log)
	return nil
}

func (m *mockIterationLogger) LogReview(log *logger.ReviewLog) error {
	// Mock implementation - just return nil for now
	return nil
}

func (m *mockIterationLogger) Close() error {
	m.Closed = true
	return nil
}

func (m *mockIterationLogger) FilePath() string {
	return "/mock/logs/run-test.jsonl"
}

// mockRenderer is a minimal PromptRenderer used in nil-check and simple
// validation tests where actual rendering behavior doesn't matter.
type mockRenderer struct{}

func (m *mockRenderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
	return &prompt.Context{
		Bead:               b,
		ParentBead:         parent,
		Iteration:          iteration,
		Model:              model,
		ConfirmedLearnings: []learnings.Learning{},
		RecentLearnings:    []learnings.Learning{},
	}, nil
}

func (m *mockRenderer) RenderBuild(ctx *prompt.Context) (string, error) {
	return "mock build prompt", nil
}

func (m *mockRenderer) RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error) {
	return "mock analyze prompt", nil
}

func (m *mockRenderer) RenderLearn(ctx *prompt.LearnContext) (string, error) {
	return "mock learn prompt", nil
}

func (m *mockRenderer) RenderDecompose(ctx *prompt.DecomposeContext) (string, error) {
	return "mock decompose prompt", nil
}

func (m *mockRenderer) RenderScope(ctx *prompt.ScopeContext) (string, error) {
	return "mock scope prompt", nil
}

func (m *mockRenderer) RenderPrecheck(ctx *prompt.PrecheckContext) (string, error) {
	return "mock precheck prompt", nil
}

func (m *mockRenderer) LoadSpec(name string) (string, error) {
	return "", nil
}

func (m *mockRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	return "mock review prompt", nil
}

func (m *mockRenderer) RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error) {
	return "mock thorough review prompt", nil
}

func (m *mockRenderer) LoadClaudeMD() (string, error) {
	return "", nil
}

func (m *mockRenderer) LoadRules() (string, error) {
	return "", nil
}

func (m *mockRenderer) GetLearningsFile() *learnings.File {
	return nil
}

func (m *mockRenderer) RenderAcceptanceTests(ctx *prompt.Context) (string, error) {
	return "mock acceptance tests prompt", nil
}

func (m *mockRenderer) RenderATDDBuild(ctx *prompt.Context) (string, error) {
	return "mock atdd build prompt", nil
}

func (m *mockRenderer) RenderTDDBuild(ctx *prompt.Context) (string, error) {
	return "mock tdd build prompt", nil
}

func (m *mockRenderer) RenderRefactor(ctx *prompt.Context) (string, error) {
	return "mock refactor prompt", nil
}

type mockStateFile struct{}

func (m *mockStateFile) LastReviewCommit() string {
	return "abc123"
}

func (m *mockStateFile) IterationsSinceReview() int {
	return 0
}

func (m *mockStateFile) IncrementIterationsSinceReview() {
}

func (m *mockStateFile) RecordReview(commit string, iteration int) error {
	return nil
}

func (m *mockStateFile) Save() error {
	return nil
}

func (m *mockStateFile) Load() error {
	return nil
}

func (m *mockStateFile) LastRetro() time.Time {
	return time.Time{}
}

// --- Tests ---

func TestNewRunnerWithDeps(t *testing.T) {
	cfg := &config.Config{}
	var buf strings.Builder

	r, err := NewRunnerWithDeps(cfg, &buf, "/tmp/gromit", Deps{
		Beads:    &mockBeadClient{},
		Claude:   &mockClaudeClient{},
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil runner")
	}
}

func TestNewRunnerWithDeps_NilConfig(t *testing.T) {
	_, err := NewRunnerWithDeps(nil, &strings.Builder{}, "/tmp", Deps{})
	if err == nil || !strings.Contains(err.Error(), "config is nil") {
		t.Errorf("expected 'config is nil' error, got: %v", err)
	}
}

func TestRunWithMocks_DryRun(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID: "test-1", Title: "Test bead", Priority: 1,
				Labels: []string{}, ExpectedOutputs: []string{},
			}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Models: config.ModelsConfig{P1: "sonnet"}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}},
	)

	err := r.Run(context.Background(), 5, time.Time{}, true)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[DRY RUN]") {
		t.Errorf("expected dry run output, got: %s", output)
	}
	if !strings.Contains(output, "test-1") {
		t.Errorf("expected bead ID in output, got: %s", output)
	}
}

func TestRunWithMocks_NoWork(t *testing.T) {
	beads := &mockBeadClient{ReadyFn: func() (*bead.Bead, error) { return nil, nil }}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(&config.Config{}, &buf, t.TempDir(),
		Deps{Beads: beads, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	err := r.Run(context.Background(), 0, time.Time{}, false)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if !strings.Contains(buf.String(), "No more work available") {
		t.Errorf("expected 'No more work available', got: %s", buf.String())
	}
}

func TestRunWithMocks_MaxIterations(t *testing.T) {
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{ID: "test-1", Title: "Infinite bead", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(&config.Config{}, &buf, t.TempDir(),
		Deps{Beads: beads, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	err := r.Run(context.Background(), 2, time.Time{}, true)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Reached max iterations") {
		t.Errorf("expected max iterations message, got: %s", buf.String())
	}
}

func TestStatusWithMocks(t *testing.T) {
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{ID: "bead-42", Title: "Important task", Priority: 0, Labels: []string{"complexity:high"}, ExpectedOutputs: []string{}}, nil
		},
	}

	var buf strings.Builder
	cfg := &config.Config{Models: config.ModelsConfig{P0: "opus", Labels: map[string]string{"complexity:high": "opus"}}}
	cfg.Paths.Specs = ".gromit/specs"
	cfg.Paths.Plans = ".gromit/plans"
	r, _ := NewRunnerWithDeps(
		cfg,
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	if err := r.Status(); err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	output := buf.String()
	// New status shows pipeline, run, health, and recommendation sections
	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("expected 'Pipeline:' in output, got: %s", output)
	}
	if !strings.Contains(output, "Run:") {
		t.Errorf("expected 'Run:' in output, got: %s", output)
	}
	if !strings.Contains(output, "Health:") {
		t.Errorf("expected 'Health:' in output, got: %s", output)
	}
}

func TestStatusWithMocks_NoWork(t *testing.T) {
	beads := &mockBeadClient{ReadyFn: func() (*bead.Bead, error) { return nil, nil }}

	var buf strings.Builder
	cfg := &config.Config{}
	cfg.Paths.Specs = ".gromit/specs"
	cfg.Paths.Plans = ".gromit/plans"
	r, _ := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{Beads: beads, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	if err := r.Status(); err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	// New status shows pipeline, run, health sections with "No work in pipeline" recommendation
	if !strings.Contains(buf.String(), "No work in pipeline") {
		t.Errorf("expected 'No work in pipeline' in output, got: %s", buf.String())
	}
}

func TestCreateSubBeadsWithMocks(t *testing.T) {
	created := []string{}
	createdDescriptions := map[string]string{}
	beads := &mockBeadClient{
		CreateWithParentAndDescriptionFn: func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
			id := fmt.Sprintf("sub-%d", len(created)+1)
			created = append(created, id)
			createdDescriptions[id] = description
			return &bead.Bead{ID: id, Title: title, Description: description, Priority: priority, Labels: labels, ExpectedOutputs: []string{}}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(&config.Config{}, &buf, t.TempDir(),
		Deps{Beads: beads, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	parent := &bead.Bead{ID: "parent-1", Title: "Parent task", Priority: 1, Labels: []string{"spec:auth"}, ExpectedOutputs: []string{}}
	subTasks := []SubTask{
		{Title: "Sub-task A", Description: "Do A", AcceptanceCriteria: []string{"A done"}},
		{Title: "Sub-task B", Description: "Do B", AcceptanceCriteria: []string{"B done"}},
	}

	if err := r.CreateSubBeads(context.Background(), parent, subTasks); err != nil {
		t.Fatalf("CreateSubBeads() failed: %v", err)
	}

	if len(created) != 2 {
		t.Errorf("expected 2 sub-beads created, got %d", len(created))
	}
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "parent-1" {
		t.Errorf("expected parent bead to be closed, got: %v", beads.ClosedIDs)
	}
	if len(beads.Comments) != 1 || !strings.Contains(beads.Comments[0].Comment, "Decomposed into 2 sub-beads") {
		t.Errorf("expected decomposition comment")
	}
	if beads.SyncCalls != 1 {
		t.Errorf("expected 1 sync call, got %d", beads.SyncCalls)
	}

	// Verify descriptions were preserved
	if !strings.Contains(createdDescriptions["sub-1"], "Do A") {
		t.Errorf("expected description to contain 'Do A', got %q", createdDescriptions["sub-1"])
	}
	if !strings.Contains(createdDescriptions["sub-1"], "A done") {
		t.Errorf("expected description to contain acceptance criteria 'A done', got %q", createdDescriptions["sub-1"])
	}
	if !strings.Contains(createdDescriptions["sub-2"], "Do B") {
		t.Errorf("expected description to contain 'Do B', got %q", createdDescriptions["sub-2"])
	}
	if !strings.Contains(createdDescriptions["sub-2"], "B done") {
		t.Errorf("expected description to contain acceptance criteria 'B done', got %q", createdDescriptions["sub-2"])
	}
}

func TestDecomposeTaskWithMocks(t *testing.T) {
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{
				Success: true,
				Output: `[{"title":"Task 1","description":"First","depends_on":null,"acceptance_criteria":["Done"]},` +
					`{"title":"Task 2","description":"Second","depends_on":0,"acceptance_criteria":["Done"]}]`,
			}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(&config.Config{}, &buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "big-task", Title: "Big Task", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	subTasks, err := r.DecomposeTask(context.Background(), b)
	if err != nil {
		t.Fatalf("DecomposeTask() failed: %v", err)
	}
	if len(subTasks) != 2 {
		t.Errorf("expected 2 sub-tasks, got %d", len(subTasks))
	}
	if len(mockClaude.RunCalls) != 1 || mockClaude.RunCalls[0].Model != "opus" {
		t.Errorf("expected Claude called with opus model")
	}
}

func TestAnalyzeAndHandleFailureWithMocks(t *testing.T) {
	mockAnalyzerObj := &mockFailureAnalyzer{
		AnalyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{
				Category:    analyzer.CategorySyntax,
				Recoverable: true,
				RootCause:   "missing import",
				Suggestion:  "Add the missing import statement",
			}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{AnalysisTimeout: 30}},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Claude: &mockClaudeClient{}, Analyzer: mockAnalyzerObj, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	bc := &beadContext{
		bead:              &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}, ExpectedOutputs: []string{}},
		result:            &IterationResult{},
		model:             "sonnet",
		promptCtx:         &prompt.Context{Model: "sonnet", ConfirmedLearnings: []learnings.Learning{}, RecentLearnings: []learnings.Learning{}},
		retriesThisModel:  0,
		maxRetries:        2,
		maxRetriesPerBead: 5,
	}

	claudeResult := &claude.Result{Success: false, Output: "compile error: missing import"}
	continueLoop := r.analyzeAndHandleFailure(context.Background(), bc, claudeResult)

	if !continueLoop {
		t.Error("expected continueLoop=true for recoverable failure")
	}
	if bc.retriesThisModel != 1 {
		t.Errorf("expected retriesThisModel=1, got %d", bc.retriesThisModel)
	}
	if !bc.promptCtx.IsRetry {
		t.Error("expected IsRetry=true after recoverable failure")
	}
	if bc.promptCtx.FailureContext != "Add the missing import statement" {
		t.Errorf("expected failure context from analysis, got %q", bc.promptCtx.FailureContext)
	}
}

func TestRunWithMocks_ContextCancellation(t *testing.T) {
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{ID: "test-1", Title: "Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(&config.Config{}, &buf, t.TempDir(),
		Deps{Beads: beads, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.Run(ctx, 0, time.Time{}, false)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}
}

func TestIterationLogWithMocks(t *testing.T) {
	mockLog := &mockIterationLogger{}
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{ID: "log-test", Title: "Log Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 1, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 iteration logged, got %d", len(mockLog.Logs))
	}
	if mockLog.Logs[0].BeadID != "log-test" {
		t.Errorf("expected BeadID 'log-test', got %q", mockLog.Logs[0].BeadID)
	}
	if !mockLog.Logs[0].Success {
		t.Error("expected Success=true")
	}
	if !mockLog.Closed {
		t.Error("expected logger to be closed")
	}
}

func TestStuckBeadWithMocks(t *testing.T) {
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Loop: config.LoopConfig{StuckBeadThreshold: 3}},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	beadStats := map[string]logger.BeadStats{
		"stuck-1": {BeadID: "stuck-1", Failures: 5, Comments: []string{}},
	}

	b := &bead.Bead{ID: "stuck-1", Title: "Stuck Bead", Labels: []string{}, ExpectedOutputs: []string{}}
	if !r.isStuckBeadWithStats(b, beadStats) {
		t.Error("expected bead to be detected as stuck")
	}

	b2 := &bead.Bead{ID: "ok-1", Title: "OK Bead", Labels: []string{}, ExpectedOutputs: []string{}}
	if r.isStuckBeadWithStats(b2, beadStats) {
		t.Error("expected bead with no history to not be stuck")
	}
}

func TestSelectModelWithMocks(t *testing.T) {
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Models: config.ModelsConfig{P0: "opus", P1: "sonnet", P2: "haiku", Labels: map[string]string{"complexity:high": "opus", "complexity:low": "haiku"}}},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	tests := []struct {
		name     string
		bead     *bead.Bead
		expected string
	}{
		{"P0 uses opus", &bead.Bead{ID: "t1", Priority: 0, Labels: []string{}}, "opus"},
		{"P1 uses sonnet", &bead.Bead{ID: "t2", Priority: 1, Labels: []string{}}, "sonnet"},
		{"P2 uses haiku", &bead.Bead{ID: "t3", Priority: 2, Labels: []string{}}, "haiku"},
		{"complexity:high overrides to opus", &bead.Bead{ID: "t4", Priority: 2, Labels: []string{"complexity:high"}}, "opus"},
		{"complexity:low overrides to haiku", &bead.Bead{ID: "t5", Priority: 0, Labels: []string{"complexity:low"}}, "haiku"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.selectModel(tt.bead); got != tt.expected {
				t.Errorf("selectModel() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestProcessBeadWithMocks_SuccessfulBuild(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "code implemented"}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "build-test", Title: "Build Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{})

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if result.BeadID != "build-test" {
		t.Errorf("expected BeadID 'build-test', got %q", result.BeadID)
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestProcessBeadWithMocks_BuildFailure(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: false, Output: "compile error", ExitCode: 1}, nil
		},
	}
	mockAnalyzerObj := &mockFailureAnalyzer{
		AnalyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{Category: analyzer.CategoryLogic, Recoverable: false, RootCause: "fundamental issue"}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60, AnalysisTimeout: 30}},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Claude: mockClaude, Analyzer: mockAnalyzerObj, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "fail-test", Title: "Fail Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{})

	if result.Success {
		t.Error("expected failure")
	}
	if result.Error == nil {
		t.Error("expected error to be set")
	}
	if mockAnalyzerObj.AnalyzeCalls != 1 {
		t.Errorf("expected 1 analyze call, got %d", mockAnalyzerObj.AnalyzeCalls)
	}
}

func TestHandleScopeTooLargeWithMocks(t *testing.T) {
	beads := &mockBeadClient{}
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(&config.Config{}, &buf, t.TempDir(),
		Deps{Beads: beads, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	bc := &beadContext{
		bead:   &bead.Bead{ID: "big-1", Title: "Big Task", Labels: []string{}, ExpectedOutputs: []string{}},
		result: &IterationResult{},
	}
	claudeResult := &claude.Result{Output: "SCOPE_TOO_LARGE: This task needs to be split"}

	r.handleScopeTooLarge(bc, claudeResult, "needs split")

	if bc.result.Error == nil || !strings.Contains(bc.result.Error.Error(), "scope too large") {
		t.Errorf("expected scope too large error, got: %v", bc.result.Error)
	}
	if len(beads.Comments) != 1 || !strings.Contains(beads.Comments[0].Comment, "Scope too large") {
		t.Error("expected scope too large comment on bead")
	}
}

func TestRunWithMocks_BeadReadyError(t *testing.T) {
	beads := &mockBeadClient{ReadyFn: func() (*bead.Bead, error) { return nil, fmt.Errorf("bd CLI not found") }}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(&config.Config{}, &buf, t.TempDir(),
		Deps{Beads: beads, Claude: &mockClaudeClient{}, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	err := r.Run(context.Background(), 1, time.Time{}, false)
	if err == nil || !strings.Contains(err.Error(), "getting next bead") {
		t.Errorf("expected 'getting next bead' error, got: %v", err)
	}
}

func TestProcessBeadWithMocks_ValidationEnabled(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build ok"}, nil
		},
		RunValidationFn: func(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "VALIDATION_PASSED"}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:     config.ClaudeConfig{BeadTimeout: 60},
			Validation: config.ValidationConfig{Enabled: true, Commands: []string{"go test ./..."}},
			Models:     config.ModelsConfig{Validation: "haiku"},
		},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "val-test", Title: "Validation Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{})

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if !result.Validated {
		t.Error("expected Validated=true")
	}
	if mockClaude.ValidationCalls != 1 {
		t.Errorf("expected 1 validation call, got %d", mockClaude.ValidationCalls)
	}
}

func TestEscalationWithMocks(t *testing.T) {
	callCount := 0
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			callCount++
			if callCount == 1 {
				return &claude.Result{Success: false, Output: "fail", ExitCode: 1}, nil
			}
			return &claude.Result{Success: true, Output: "success"}, nil
		},
	}
	mockAnalyzerObj := &mockFailureAnalyzer{
		AnalyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{Category: analyzer.CategoryLogic, Recoverable: false, RootCause: "needs stronger model"}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:     config.ClaudeConfig{BeadTimeout: 60, AnalysisTimeout: 30},
			Escalation: config.EscalationConfig{Enabled: true, Chain: []string{"haiku", "sonnet", "opus"}, MaxRetriesPerModel: 0, MaxRetriesPerBead: 10},
			Models:     config.ModelsConfig{P1: "haiku"},
		},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Claude: mockClaude, Analyzer: mockAnalyzerObj, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "escalate-test", Title: "Escalation Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{})

	if !result.Success {
		t.Errorf("expected success after escalation, got error: %v", result.Error)
	}
	if !result.Escalated {
		t.Error("expected Escalated=true")
	}
	if result.EscalatedTo != "sonnet" {
		t.Errorf("expected EscalatedTo='sonnet', got %q", result.EscalatedTo)
	}
	if len(mockClaude.StreamRunCalls) != 2 {
		t.Fatalf("expected 2 stream run calls, got %d", len(mockClaude.StreamRunCalls))
	}
	if mockClaude.StreamRunCalls[0].Model != "haiku" {
		t.Errorf("expected first call with haiku, got %q", mockClaude.StreamRunCalls[0].Model)
	}
	if mockClaude.StreamRunCalls[1].Model != "sonnet" {
		t.Errorf("expected second call with sonnet, got %q", mockClaude.StreamRunCalls[1].Model)
	}
}

func TestWriteIterationLogWithMocks(t *testing.T) {
	mockLog := &mockIterationLogger{}
	r := &Runner{logger: mockLog, output: &strings.Builder{}}

	result := &IterationResult{BeadID: "log-1", Model: "sonnet", Success: true, Duration: 5 * time.Second}
	r.writeIterationLog(1, result)

	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 iteration logged, got %d", len(mockLog.Logs))
	}
	if mockLog.Logs[0].BeadID != "log-1" {
		t.Errorf("expected BeadID 'log-1', got %q", mockLog.Logs[0].BeadID)
	}
	if mockLog.Logs[0].DurationMs != 5000 {
		t.Errorf("expected DurationMs=5000, got %d", mockLog.Logs[0].DurationMs)
	}
}

func TestProcessBeadWithMocks_UnclearSpec(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: false, Output: "unclear spec", ExitCode: 1}, nil
		},
	}
	mockAnalyzerObj := &mockFailureAnalyzer{
		AnalyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{Category: analyzer.CategoryUnclearSpec, Recoverable: false, RootCause: "spec is ambiguous"}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60, AnalysisTimeout: 30}},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Claude: mockClaude, Analyzer: mockAnalyzerObj, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "unclear-test", Title: "Unclear Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{})

	if result.Success {
		t.Error("expected failure for unclear spec")
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "spec unclear") {
		t.Errorf("expected 'spec unclear' in error, got: %v", result.Error)
	}
}

func TestRunWithMocks_ClosesBeadOnSuccess(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{ID: "close-test", Title: "Should be closed", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	if err := r.Run(context.Background(), 1, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "close-test" {
		t.Errorf("expected bead 'close-test' to be closed, got: %v", beads.ClosedIDs)
	}
	if beads.SyncCalls != 1 {
		t.Errorf("expected 1 sync call, got %d", beads.SyncCalls)
	}
}

func TestRunWithMocks_PrecheckPassed(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "precheck-test",
				Title:           "Already completed",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature is implemented", "tests pass"},
			}, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Precheck returns PRECHECK_PASSED
			return &claude.Result{Success: true, Output: "PRECHECK_PASSED\n\nAll acceptance criteria are satisfied."}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify bead was closed
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "precheck-test" {
		t.Errorf("expected bead 'precheck-test' to be closed, got: %v", beads.ClosedIDs)
	}

	// Verify bd sync was called
	if beads.SyncCalls != 1 {
		t.Errorf("expected 1 sync call, got %d", beads.SyncCalls)
	}

	// Verify iteration log was written with precheck_skipped outcome
	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 iteration logged, got %d", len(mockLog.Logs))
	}
	log := mockLog.Logs[0]
	if log.BeadID != "precheck-test" {
		t.Errorf("expected BeadID 'precheck-test', got %q", log.BeadID)
	}
	if log.Outcome != "precheck_skipped" {
		t.Errorf("expected Outcome 'precheck_skipped', got %q", log.Outcome)
	}
	if log.Model != "haiku" {
		t.Errorf("expected Model 'haiku', got %q", log.Model)
	}
	if !log.Success {
		t.Error("expected Success=true for precheck_skipped")
	}

	// Verify iteration counter was NOT incremented (should be 1, not 2)
	// The iteration field in the log should be 1 (iteration + 1 where iteration starts at 0)
	if log.Iteration != 1 {
		t.Errorf("expected Iteration=1 (not incremented), got %d", log.Iteration)
	}

	// Verify console output mentions precheck
	output := buf.String()
	if !strings.Contains(output, "Pre-check: acceptance criteria already met") {
		t.Errorf("expected precheck message in output, got: %s", output)
	}
	if !strings.Contains(output, "auto-closing bead precheck-test") {
		t.Errorf("expected auto-closing message in output, got: %s", output)
	}

	// Verify Claude.Run was called (for precheck) but StreamRun was NOT called (no build)
	if len(mockClaude.RunCalls) != 1 {
		t.Errorf("expected 1 Claude.Run call (precheck), got %d", len(mockClaude.RunCalls))
	}
	if len(mockClaude.StreamRunCalls) != 0 {
		t.Errorf("expected 0 Claude.StreamRun calls (no build), got %d", len(mockClaude.StreamRunCalls))
	}
}
