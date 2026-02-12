package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// --- Mock implementations ---

type mockBeadClient struct {
	ReadyFn                          func() (*bead.Bead, error)
	ReadyWithLabelFn                 func(label string) (*bead.Bead, error)
	ListWithLabelFn                  func(label string) ([]*bead.Bead, error)
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

	mu              sync.Mutex
	RunCalls        []mockClaudeCall
	StreamRunCalls  []mockClaudeCall
	ValidationCalls int
}

type mockClaudeCall struct {
	Prompt string
	Model  string
}

func (m *mockClaudeClient) Run(ctx context.Context, prompt string, model string) (*claude.Result, error) {
	m.mu.Lock()
	m.RunCalls = append(m.RunCalls, mockClaudeCall{Prompt: prompt, Model: model})
	m.mu.Unlock()
	if m.RunFn != nil {
		return m.RunFn(ctx, prompt, model)
	}
	return &claude.Result{Success: true, Output: "ok"}, nil
}

func (m *mockClaudeClient) StreamRun(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
	m.mu.Lock()
	m.StreamRunCalls = append(m.StreamRunCalls, mockClaudeCall{Prompt: prompt, Model: model})
	m.mu.Unlock()
	if m.StreamRunFn != nil {
		return m.StreamRunFn(ctx, prompt, model, output, handler, onToolCall)
	}
	return &claude.Result{Success: true, Output: "ok"}, nil
}

func (m *mockClaudeClient) RunValidation(ctx context.Context, commands []string, model string, workDir string) (*claude.Result, error) {
	m.mu.Lock()
	m.ValidationCalls++
	m.mu.Unlock()
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

func (m *mockIterationLogger) RunID() string {
	return "test-run"
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

// TestBeadClientInterfaceIncludesReadyWithLabel verifies that BeadClient interface
// includes the ReadyWithLabel method with correct signature.
func TestBeadClientInterfaceIncludesReadyWithLabel(t *testing.T) {
	// Verify the interface method exists by calling it on a mock implementation
	// This test will only compile if ReadyWithLabel is part of the BeadClient interface
	mock := &mockBeadClient{
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			return &bead.Bead{ID: "test-1", Title: "Test"}, nil
		},
	}

	// Call the method through the interface to verify the signature
	var client BeadClient = mock
	result, err := client.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() returned error: %v", err)
	}
	if result == nil {
		t.Error("ReadyWithLabel() returned nil bead")
	}
	if result.ID != "test-1" {
		t.Errorf("ReadyWithLabel() bead ID = %q, want 'test-1'", result.ID)
	}
}

// TestBeadClientInterfaceIncludesListWithLabel verifies that BeadClient interface
// includes the ListWithLabel method with correct signature.
func TestBeadClientInterfaceIncludesListWithLabel(t *testing.T) {
	// Verify the interface method exists by calling it on a mock implementation
	// This test will only compile if ListWithLabel is part of the BeadClient interface
	mock := &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return []*bead.Bead{
				{ID: "task-1", Title: "First task"},
				{ID: "task-2", Title: "Second task"},
			}, nil
		},
	}

	// Call the method through the interface to verify the signature
	var client BeadClient = mock
	results, err := client.ListWithLabel("spec:auth")
	if err != nil {
		t.Errorf("ListWithLabel() returned error: %v", err)
	}
	if results == nil {
		t.Error("ListWithLabel() returned nil slice")
	}
	if len(results) != 2 {
		t.Errorf("ListWithLabel() returned %d beads, want 2", len(results))
	}
	if results[0].ID != "task-1" {
		t.Errorf("ListWithLabel()[0] ID = %q, want 'task-1'", results[0].ID)
	}
	if results[1].ID != "task-2" {
		t.Errorf("ListWithLabel()[1] ID = %q, want 'task-2'", results[1].ID)
	}
}

// TestReadyWithLabelMockImplementation verifies the mock's ReadyWithLabel method works
func TestReadyWithLabelMockImplementation(t *testing.T) {
	mock := &mockBeadClient{}

	// Test nil callback returns nil bead
	result, err := mock.ReadyWithLabel("spec:test")
	if err != nil {
		t.Errorf("ReadyWithLabel() with nil callback should return nil error, got: %v", err)
	}
	if result != nil {
		t.Errorf("ReadyWithLabel() with nil callback should return nil bead, got: %v", result)
	}

	// Test custom callback
	mock.ReadyWithLabelFn = func(label string) (*bead.Bead, error) {
		return &bead.Bead{ID: "custom", Title: label}, nil
	}
	result, err = mock.ReadyWithLabel("custom-label")
	if err != nil {
		t.Errorf("ReadyWithLabel() with custom callback returned error: %v", err)
	}
	if result.Title != "custom-label" {
		t.Errorf("ReadyWithLabel() title = %q, want 'custom-label'", result.Title)
	}
}

// TestListWithLabelMockImplementation verifies the mock's ListWithLabel method works
func TestListWithLabelMockImplementation(t *testing.T) {
	mock := &mockBeadClient{}

	// Test nil callback returns empty slice
	results, err := mock.ListWithLabel("spec:test")
	if err != nil {
		t.Errorf("ListWithLabel() with nil callback should return nil error, got: %v", err)
	}
	if results == nil {
		t.Error("ListWithLabel() with nil callback should return empty slice not nil")
	}
	if len(results) != 0 {
		t.Errorf("ListWithLabel() with nil callback should return empty slice, got %d items", len(results))
	}

	// Test custom callback
	mock.ListWithLabelFn = func(label string) ([]*bead.Bead, error) {
		return []*bead.Bead{
			{ID: "task-1", Title: "Task for " + label},
			{ID: "task-2", Title: "Another task"},
		}, nil
	}
	results, err = mock.ListWithLabel("my-spec")
	if err != nil {
		t.Errorf("ListWithLabel() with custom callback returned error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ListWithLabel() returned %d beads, want 2", len(results))
	}
	if results[0].Title != "Task for my-spec" {
		t.Errorf("ListWithLabel()[0] title = %q, want 'Task for my-spec'", results[0].Title)
	}
}

func TestNewRunnerWithDeps(t *testing.T) {
	cfg := &config.Config{}
	var buf strings.Builder

	r, err := NewRunnerWithDeps(cfg, &buf, "/tmp/gromit", Deps{
		Beads:    &mockBeadClient{},
		Router: newMockRouterFromClaudeClient(&mockClaudeClient{}),
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

func TestNewRunnerWithDeps_ApplesDefaultsToUninitialisedConfig(t *testing.T) {
	// Test that NewRunnerWithDeps applies config defaults to prevent
	// accidental precheck execution in tests that don't explicitly test it.
	cfg := &config.Config{}
	var buf strings.Builder

	r, err := NewRunnerWithDeps(cfg, &buf, "/tmp/gromit", Deps{
		Beads:    &mockBeadClient{},
		Router: newMockRouterFromClaudeClient(&mockClaudeClient{}),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Verify that config defaults were applied
	// Precheck should be enabled by default (true)
	if cfg.Precheck.Enabled == nil {
		t.Error("expected Precheck.Enabled to be set to non-nil after NewRunnerWithDeps")
	}
	if cfg.Precheck.Enabled != nil && !*cfg.Precheck.Enabled {
		t.Error("expected Precheck.Enabled to be true by default")
	}

	// Precheck should have a default model
	if cfg.Precheck.Model == "" {
		t.Error("expected Precheck.Model to have default value")
	}

	// Loop.MaxConsecutiveSkips should have a default value
	if cfg.Loop.MaxConsecutiveSkips == 0 {
		t.Error("expected Loop.MaxConsecutiveSkips to have default value")
	}

	// Models should have defaults
	if cfg.Models.P0 == "" || cfg.Models.P1 == "" || cfg.Models.P2 == "" {
		t.Error("expected Models P0, P1, P2 to have default values")
	}

	if r == nil {
		t.Fatal("expected non-nil runner")
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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}},
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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: mockAnalyzerObj, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

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
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		{"test-only P1 bead routes to haiku", &bead.Bead{ID: "t6", Priority: 1, Title: "Add tests for runner escalation", Labels: []string{}}, "haiku"},
		{"test-only P0 bead routes to haiku", &bead.Bead{ID: "t7", Priority: 0, Title: "Write tests for prompt rendering", Labels: []string{}}, "haiku"},
		{"test-only with complexity:high still uses opus", &bead.Bead{ID: "t8", Priority: 1, Title: "Add tests for complex integration", Labels: []string{"complexity:high"}}, "opus"},
		{"non-test bead P1 still uses sonnet", &bead.Bead{ID: "t9", Priority: 1, Title: "Fix failing tests in runner", Labels: []string{}}, "sonnet"},
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
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "build-test", Title: "Build Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

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
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: mockAnalyzerObj, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "fail-test", Title: "Fail Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "val-test", Title: "Validation Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

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
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: mockAnalyzerObj, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "escalate-test", Title: "Escalation Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

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
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: mockAnalyzerObj, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	b := &bead.Bead{ID: "unclear-test", Title: "Unclear Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

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
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

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

	precheckEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}, Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

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

func TestRunWithMocks_PrecheckNotMet(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "precheck-notmet",
				Title:           "Not completed yet",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature is implemented", "tests pass"},
			}, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Precheck returns PRECHECK_NOT_MET
			return &claude.Result{Success: true, Output: "PRECHECK_NOT_MET\n\nCriteria not satisfied yet."}, nil
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			// Build phase succeeds
			return &claude.Result{Success: true, Output: "implemented the feature"}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}, Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify precheck ran
	if len(mockClaude.RunCalls) != 1 {
		t.Errorf("expected 1 Claude.Run call (precheck), got %d", len(mockClaude.RunCalls))
	}

	// Verify processBead ran (StreamRun was called)
	if len(mockClaude.StreamRunCalls) != 1 {
		t.Errorf("expected 1 Claude.StreamRun call (build), got %d", len(mockClaude.StreamRunCalls))
	}

	// Verify bead was closed after successful build
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "precheck-notmet" {
		t.Errorf("expected bead 'precheck-notmet' to be closed, got: %v", beads.ClosedIDs)
	}

	// Verify normal iteration log (not precheck_skipped)
	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 iteration logged, got %d", len(mockLog.Logs))
	}
	log := mockLog.Logs[0]
	if log.Outcome == "precheck_skipped" {
		t.Error("expected normal outcome, not precheck_skipped")
	}
}

func TestRunWithMocks_PrecheckError(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "precheck-error",
				Title:           "Precheck fails",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature is implemented"},
			}, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Precheck returns an error
			return nil, fmt.Errorf("Claude API error")
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			// Build phase succeeds
			return &claude.Result{Success: true, Output: "implemented despite precheck error"}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}, Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify precheck was attempted
	if len(mockClaude.RunCalls) != 1 {
		t.Errorf("expected 1 Claude.Run call (precheck), got %d", len(mockClaude.RunCalls))
	}

	// Verify processBead ran despite precheck error (StreamRun was called)
	if len(mockClaude.StreamRunCalls) != 1 {
		t.Errorf("expected 1 Claude.StreamRun call (build), got %d", len(mockClaude.StreamRunCalls))
	}

	// Verify bead was closed after successful build
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "precheck-error" {
		t.Errorf("expected bead 'precheck-error' to be closed, got: %v", beads.ClosedIDs)
	}

	// Verify warning was logged
	output := buf.String()
	if !strings.Contains(output, "precheck invocation failed") {
		t.Errorf("expected precheck warning in output, got: %s", output)
	}

	// Verify normal iteration log (not precheck_skipped)
	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 iteration logged, got %d", len(mockLog.Logs))
	}
	log := mockLog.Logs[0]
	if log.Outcome == "precheck_skipped" {
		t.Error("expected normal outcome, not precheck_skipped")
	}
}

func TestRunWithMocks_PrecheckDoesNotCountAsIteration(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			switch callCount {
			case 1:
				// First bead: precheck passes (should be skipped, not count as iteration)
				return &bead.Bead{
					ID:              "bead-1",
					Title:           "Already completed",
					Priority:        1,
					Labels:          []string{},
					ExpectedOutputs: []string{"feature is implemented"},
				}, nil
			case 2:
				// Second bead: precheck fails, needs real work
				return &bead.Bead{
					ID:              "bead-2",
					Title:           "Needs implementation",
					Priority:        1,
					Labels:          []string{},
					ExpectedOutputs: []string{"feature is implemented"},
				}, nil
			default:
				return nil, nil
			}
		},
	}

	claudeCallCount := 0
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			claudeCallCount++
			if claudeCallCount == 1 {
				// First precheck: PASSED
				return &claude.Result{Success: true, Output: "PRECHECK_PASSED\n\nAll criteria met."}, nil
			}
			// Second precheck: NOT_MET
			return &claude.Result{Success: true, Output: "PRECHECK_NOT_MET\n\nNeeds work."}, nil
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			// Build phase for second bead
			return &claude.Result{Success: true, Output: "implemented the feature"}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}, Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	// Run with maxIterations=1. First bead passes precheck (skipped), second bead does real work.
	// Both should complete because precheck skip doesn't count as an iteration.
	if err := r.Run(context.Background(), 1, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify both beads were closed
	if len(beads.ClosedIDs) != 2 {
		t.Errorf("expected 2 beads closed, got %d: %v", len(beads.ClosedIDs), beads.ClosedIDs)
	}
	if len(beads.ClosedIDs) >= 2 {
		if beads.ClosedIDs[0] != "bead-1" {
			t.Errorf("expected first closed bead to be 'bead-1', got %q", beads.ClosedIDs[0])
		}
		if beads.ClosedIDs[1] != "bead-2" {
			t.Errorf("expected second closed bead to be 'bead-2', got %q", beads.ClosedIDs[1])
		}
	}

	// Verify two iteration logs: one precheck_skipped, one normal
	if len(mockLog.Logs) != 2 {
		t.Fatalf("expected 2 iteration logs, got %d", len(mockLog.Logs))
	}

	// First log should be precheck_skipped for bead-1
	if mockLog.Logs[0].BeadID != "bead-1" {
		t.Errorf("expected first log BeadID 'bead-1', got %q", mockLog.Logs[0].BeadID)
	}
	if mockLog.Logs[0].Outcome != "precheck_skipped" {
		t.Errorf("expected first log Outcome 'precheck_skipped', got %q", mockLog.Logs[0].Outcome)
	}
	if mockLog.Logs[0].Iteration != 1 {
		t.Errorf("expected first log Iteration=1, got %d", mockLog.Logs[0].Iteration)
	}

	// Second log should be normal for bead-2
	if mockLog.Logs[1].BeadID != "bead-2" {
		t.Errorf("expected second log BeadID 'bead-2', got %q", mockLog.Logs[1].BeadID)
	}
	if mockLog.Logs[1].Outcome == "precheck_skipped" {
		t.Errorf("expected second log to have normal outcome, got 'precheck_skipped'")
	}
	if mockLog.Logs[1].Iteration != 1 {
		t.Errorf("expected second log Iteration=1 (not incremented from precheck skip), got %d", mockLog.Logs[1].Iteration)
	}

	// Verify console output mentions both beads
	output := buf.String()
	if !strings.Contains(output, "auto-closing bead bead-1") {
		t.Errorf("expected auto-closing message for bead-1 in output, got: %s", output)
	}
	if !strings.Contains(output, "Iteration 1") {
		t.Errorf("expected 'Iteration 1' in output (for bead-2), got: %s", output)
	}
	if strings.Contains(output, "Iteration 2") {
		t.Errorf("unexpected 'Iteration 2' in output (precheck should not count), got: %s", output)
	}

	// Verify 2 precheck calls and 1 build call
	if len(mockClaude.RunCalls) != 2 {
		t.Errorf("expected 2 Claude.Run calls (2 prechecks), got %d", len(mockClaude.RunCalls))
	}
	if len(mockClaude.StreamRunCalls) != 1 {
		t.Errorf("expected 1 Claude.StreamRun call (1 build), got %d", len(mockClaude.StreamRunCalls))
	}
}

func TestRunWithMocks_PrecheckPassedCloseFailsLoopTerminates(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			// Always return the same bead (simulating Close() not working)
			return &bead.Bead{
				ID:              "stuck-bead",
				Title:           "Already completed but Close fails",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature is implemented"},
			}, nil
		},
		CloseFn: func(id string) error {
			// Close always fails
			return fmt.Errorf("bd close failed: permission denied")
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Precheck always passes
			return &claude.Result{Success: true, Output: "PRECHECK_PASSED\n\nAll criteria met."}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}, Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	// Run should terminate after the second loop iteration (first iteration adds to skippedBeads, second detects it)
	if err := r.Run(context.Background(), 10, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify that Ready was called twice (once for first attempt, once for second attempt that detects stuck)
	if callCount != 2 {
		t.Errorf("expected Ready called 2 times, got %d", callCount)
	}

	// Verify Close was attempted once (first iteration)
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "stuck-bead" {
		t.Errorf("expected Close attempted once for 'stuck-bead', got: %v", beads.ClosedIDs)
	}

	// Verify the loop terminated with the "all ready beads are stuck" message
	output := buf.String()
	if !strings.Contains(output, "All ready beads are stuck and have been skipped") {
		t.Errorf("expected 'All ready beads are stuck' message in output, got: %s", output)
	}

	// Verify warning about Close failure was logged
	if !strings.Contains(output, "failed to close bead") {
		t.Errorf("expected 'failed to close bead' warning in output, got: %s", output)
	}

	// Verify one precheck_skipped log was written (for the first iteration)
	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 iteration log, got %d", len(mockLog.Logs))
	}
	if mockLog.Logs[0].Outcome != "precheck_skipped" {
		t.Errorf("expected Outcome 'precheck_skipped', got %q", mockLog.Logs[0].Outcome)
	}
	if mockLog.Logs[0].BeadID != "stuck-bead" {
		t.Errorf("expected BeadID 'stuck-bead', got %q", mockLog.Logs[0].BeadID)
	}
}

func TestRunWithMocks_ConsecutivePrecheckSkipsHitsLimit(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			// Always return the same bead (simulating Close() appearing to succeed but not working)
			return &bead.Bead{
				ID:              "stuck-bead",
				Title:           "Already completed but Close doesn't work",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature is implemented"},
			}, nil
		},
		CloseFn: func(id string) error {
			// Close appears to succeed (returns nil) but doesn't actually work
			return nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Precheck always passes
			return &claude.Result{Success: true, Output: "PRECHECK_PASSED\n\nAll criteria met."}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:   config.ClaudeConfig{BeadTimeout: 60},
			Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30},
			Loop:     config.LoopConfig{MaxConsecutiveSkips: 3},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	// Run should fail after 3 consecutive precheck skips
	err := r.Run(context.Background(), 10, time.Time{}, false)
	if err == nil {
		t.Fatal("expected error from consecutive skip limit, got nil")
	}

	// Verify the error message mentions consecutive skips
	if !strings.Contains(err.Error(), "consecutive precheck skips") {
		t.Errorf("expected error about consecutive skips, got: %v", err)
	}

	// Verify Ready was called 3 times (limit is checked AFTER each skip, so we exit on the 3rd)
	if callCount != 3 {
		t.Errorf("expected Ready called 3 times, got %d", callCount)
	}

	// Verify Close was attempted 3 times (once per skip before limit check)
	if len(beads.ClosedIDs) != 3 {
		t.Errorf("expected Close attempted 3 times, got %d: %v", len(beads.ClosedIDs), beads.ClosedIDs)
	}

	// Verify 3 precheck_skipped logs were written (one for each skip before the error)
	if len(mockLog.Logs) != 3 {
		t.Fatalf("expected 3 iteration logs, got %d", len(mockLog.Logs))
	}
	for i, log := range mockLog.Logs {
		if log.Outcome != "precheck_skipped" {
			t.Errorf("log %d: expected Outcome 'precheck_skipped', got %q", i, log.Outcome)
		}
		if log.BeadID != "stuck-bead" {
			t.Errorf("log %d: expected BeadID 'stuck-bead', got %q", i, log.BeadID)
		}
	}
}

func TestRunWithMocks_ConsecutiveSkipCounterResetsAfterRealBuild(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			// Return sequence: 2 precheck-pass beads, 1 real-build bead (precheck disabled), 2 more precheck-pass beads
			switch callCount {
			case 1, 2:
				return &bead.Bead{
					ID:              fmt.Sprintf("precheck-bead-%d", callCount),
					Title:           "Already completed",
					Priority:        1,
					Labels:          []string{},
					ExpectedOutputs: []string{"feature is implemented"},
				}, nil
			case 3:
				// A bead where precheck fails (returns false)
				return &bead.Bead{
					ID:              "real-build-bead",
					Title:           "Needs implementation",
					Priority:        1,
					Labels:          []string{},
					ExpectedOutputs: []string{"feature is implemented"},
				}, nil
			case 4, 5:
				return &bead.Bead{
					ID:              fmt.Sprintf("precheck-bead-%d", callCount),
					Title:           "Already completed",
					Priority:        1,
					Labels:          []string{},
					ExpectedOutputs: []string{"feature is implemented"},
				}, nil
			default:
				return nil, nil
			}
		},
	}

	callNum := 0
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			callNum++
			isPrecheck := strings.Contains(prompt, "precheck")
			// Calls 1-2: precheck pass, Call 3: precheck fail, Call 4: real build, Calls 5-6: precheck pass
			if isPrecheck {
				if callNum == 3 {
					// Bead 3 precheck fails - criteria not met
					return &claude.Result{Success: true, Output: "Criteria not yet met"}, nil
				}
				// All other prechecks pass
				return &claude.Result{Success: true, Output: "PRECHECK_PASSED\n\nAll criteria met."}, nil
			}
			// Real builds succeed
			return &claude.Result{Success: true, Output: "Implementation complete"}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:     config.ClaudeConfig{BeadTimeout: 60},
			Precheck:   config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30},
			Loop:       config.LoopConfig{MaxConsecutiveSkips: 3},
			Validation: config.ValidationConfig{Enabled: false},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	// Run should complete without hitting consecutive skip limit
	if err := r.Run(context.Background(), 10, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify all 5 beads were processed
	if callCount != 6 {
		t.Errorf("expected Ready called 6 times (5 beads + final nil check), got %d", callCount)
	}

	// Verify 5 beads were closed
	if len(beads.ClosedIDs) != 5 {
		t.Errorf("expected 5 beads closed, got %d: %v", len(beads.ClosedIDs), beads.ClosedIDs)
	}

	// Verify 5 iteration logs (2 precheck_skipped, 1 regular, 2 more precheck_skipped)
	if len(mockLog.Logs) != 5 {
		t.Fatalf("expected 5 iteration logs, got %d", len(mockLog.Logs))
	}

	// Count precheck_skipped outcomes - should be 4
	precheckCount := 0
	for _, log := range mockLog.Logs {
		if log.Outcome == "precheck_skipped" {
			precheckCount++
		}
	}
	if precheckCount != 4 {
		t.Errorf("expected 4 precheck_skipped outcomes, got %d", precheckCount)
	}

	// Key assertion: we processed 2 precheck skips, then a real build, then 2 more precheck skips
	// without hitting the limit of 3 consecutive skips
	// (If the counter wasn't reset, we'd have hit the limit on the 4th precheck skip)
}

func TestRunWithMocks_CustomConsecutiveSkipLimit(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			// Always return the same bead (simulating Close() appearing to succeed but not working)
			return &bead.Bead{
				ID:              "stuck-bead",
				Title:           "Already completed but Close doesn't work",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature is implemented"},
			}, nil
		},
		CloseFn: func(id string) error {
			// Close appears to succeed (returns nil) but doesn't actually work
			return nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Precheck always passes
			return &claude.Result{Success: true, Output: "PRECHECK_PASSED\n\nAll criteria met."}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:   config.ClaudeConfig{BeadTimeout: 60},
			Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30},
			Loop:     config.LoopConfig{MaxConsecutiveSkips: 2}, // Custom limit of 2
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	// Run should fail after 2 consecutive precheck skips (custom limit)
	err := r.Run(context.Background(), 10, time.Time{}, false)
	if err == nil {
		t.Fatal("expected error from consecutive skip limit, got nil")
	}

	// Verify the error message mentions consecutive skips
	if !strings.Contains(err.Error(), "consecutive precheck skips") {
		t.Errorf("expected error about consecutive skips, got: %v", err)
	}

	// Verify Ready was called 2 times (limit is checked AFTER each skip, so we exit on the 2nd)
	if callCount != 2 {
		t.Errorf("expected Ready called 2 times, got %d", callCount)
	}

	// Verify Close was attempted 2 times (once per skip before limit check)
	if len(beads.ClosedIDs) != 2 {
		t.Errorf("expected Close attempted 2 times, got %d: %v", len(beads.ClosedIDs), beads.ClosedIDs)
	}

	// Verify 2 precheck_skipped logs were written (one for each skip before the error)
	if len(mockLog.Logs) != 2 {
		t.Fatalf("expected 2 iteration logs, got %d", len(mockLog.Logs))
	}
	for i, log := range mockLog.Logs {
		if log.Outcome != "precheck_skipped" {
			t.Errorf("log %d: expected Outcome 'precheck_skipped', got %q", i, log.Outcome)
		}
		if log.BeadID != "stuck-bead" {
			t.Errorf("log %d: expected BeadID 'stuck-bead', got %q", i, log.BeadID)
		}
	}
}

// mockClaudeProviderAdapter wraps a mockClaudeClient to implement provider.Provider interface
type mockClaudeProviderAdapter struct {
	client *mockClaudeClient
}

func (m *mockClaudeProviderAdapter) Name() string {
	return "mock-claude"
}

func (m *mockClaudeProviderAdapter) ModelForTier(tier string) string {
	switch tier {
	case provider.TierHigh:
		return "opus"
	case provider.TierMedium:
		return "sonnet"
	case provider.TierLow:
		return "haiku"
	default:
		return "haiku"
	}
}

func (m *mockClaudeProviderAdapter) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	model := m.ModelForTier(tier)
	result, err := m.client.Run(ctx, prompt, model)
	if err != nil {
		return nil, err
	}
	return &provider.Result{
		Success:  result.Success,
		Output:   result.Output,
		ExitCode: result.ExitCode,
		Duration: result.Duration,
		Model:    result.Model,
	}, nil
}

func (m *mockClaudeProviderAdapter) StreamRun(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	model := m.ModelForTier(tier)
	// Convert provider handlers back to claude handlers for the mock
	var claudeHandler claude.EventHandler
	if handler != nil {
		claudeHandler = func(line []byte) {
			handler(line)
		}
	}
	var claudeToolHandler claude.ToolCallHandler
	if onToolCall != nil {
		claudeToolHandler = func(event claude.ToolEvent) {
			onToolCall(provider.ToolEvent{
				ToolName:  event.ToolName,
				FilePath:  event.FilePath,
				Timestamp: event.Timestamp,
			})
		}
	}
	result, err := m.client.StreamRun(ctx, prompt, model, output, claudeHandler, claudeToolHandler)
	if err != nil {
		return nil, err
	}
	return &provider.Result{
		Success:  result.Success,
		Output:   result.Output,
		ExitCode: result.ExitCode,
		Duration: result.Duration,
		Model:    result.Model,
	}, nil
}

func (m *mockClaudeProviderAdapter) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	model := m.ModelForTier(tier)
	result, err := m.client.RunValidation(ctx, commands, model, workDir)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &provider.Result{
		Success:  result.Success,
		Output:   result.Output,
		ExitCode: result.ExitCode,
		Duration: result.Duration,
		Model:    result.Model,
	}, nil
}

func (m *mockClaudeProviderAdapter) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}

// newMockRouterFromClaudeClient creates a router wrapping a mockClaudeClient for unit tests
func newMockRouterFromClaudeClient(client *mockClaudeClient) *provider.Router {
	adapter := &mockClaudeProviderAdapter{client: client}
	return provider.NewSingleProviderRouter(adapter)
}
