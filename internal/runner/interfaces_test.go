package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Mock implementations ---

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
	// Fall back to ReadyFn and filter
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

func (m *mockRenderer) RenderSpecAcceptance(ctx *prompt.SpecAcceptanceContext) (string, error) {
	return "mock spec acceptance prompt", nil
}

func (m *mockRenderer) RenderSpecGate(ctx *prompt.SpecGateContext) (string, error) {
	return "mock spec gate prompt", nil
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

func (m *mockRenderer) LoadRulesForPhase(phase string) (string, error) {
	return "", nil
}

func (m *mockRenderer) GetLearningsFile() *learnings.File {
	return nil
}

func (m *mockRenderer) SetSiblingTouchedPackagesResolver(resolver prompt.SiblingTouchedPackagesResolver) {
}

func (m *mockRenderer) LastDiagnostics() *prompt.PromptDiagnostics {
	return nil
}

func (m *mockRenderer) RenderAcceptanceTests(ctx *prompt.Context) (string, error) {
	return "mock acceptance tests prompt", nil
}

func (m *mockRenderer) RenderATDDBuild(ctx *prompt.Context) (string, error) {
	return "mock atdd build prompt", nil
}

func (m *mockRenderer) RenderATDDDiagnostic(ctx *prompt.DiagnosticContext) (string, error) {
	return "mock atdd diagnostic prompt", nil
}

func (m *mockRenderer) RenderTDDBuild(ctx *prompt.Context) (string, error) {
	return "mock tdd build prompt", nil
}

func (m *mockRenderer) RenderTDDRed(ctx *prompt.TDDRedContext) (string, error) {
	return "mock tdd red prompt", nil
}

func (m *mockRenderer) RenderTDDGreen(ctx *prompt.TDDGreenContext) (string, error) {
	return "mock tdd green prompt", nil
}

func (m *mockRenderer) RenderRefactor(ctx *prompt.Context) (string, error) {
	return "mock refactor prompt", nil
}

func (m *mockRenderer) RenderTestFix(ctx *prompt.TestFixContext) (string, error) {
	return "mock test fix prompt", nil
}

func (m *mockRenderer) RenderCoverageValidation(ctx *prompt.CoverageValidationContext) (string, error) {
	return "mock coverage validation prompt", nil
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
		t.Fatal("ReadyWithLabel() returned nil bead")
	}
	if result.ID != "test-1" {
		t.Errorf("ReadyWithLabel() bead ID = %q, want 'test-1'", result.ID)
	}
}

// TestPromptRendererInterfaceIncludesRenderSpecAcceptance verifies that PromptRenderer includes
// RenderSpecAcceptance with the correct signature.
func TestPromptRendererInterfaceIncludesRenderSpecAcceptance(t *testing.T) {
	var r PromptRenderer = &mockPromptRenderer{}

	output, err := r.RenderSpecAcceptance(&prompt.SpecAcceptanceContext{Spec: "spec"})
	if err != nil {
		t.Fatalf("RenderSpecAcceptance() error = %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

// TestPromptRendererInterfaceIncludesRenderSpecGate verifies that PromptRenderer includes
// RenderSpecGate with the correct signature.
func TestPromptRendererInterfaceIncludesRenderSpecGate(t *testing.T) {
	var r PromptRenderer = &mockPromptRenderer{}

	output, err := r.RenderSpecGate(&prompt.SpecGateContext{SpecCriteria: "criteria"})
	if err != nil {
		t.Fatalf("RenderSpecGate() error = %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestPromptRendererInterfaceIncludesRenderCoverageValidation(t *testing.T) {
	var r PromptRenderer = &mockPromptRenderer{}

	output, err := r.RenderCoverageValidation(&prompt.CoverageValidationContext{
		TestCode:        "func TestCoverage(t *testing.T) {}",
		CriterionNumber: 1,
		CriterionText:   "Handles empty input",
	})
	if err != nil {
		t.Fatalf("RenderCoverageValidation() error = %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestPromptRendererInterfaceIncludesRenderATDDDiagnostic(t *testing.T) {
	var r PromptRenderer = &mockPromptRenderer{}

	output, err := r.RenderATDDDiagnostic(&prompt.DiagnosticContext{
		BeadTitle:          "Add diagnostic render method",
		BeadDescription:    "Render a pass-before-build verdict prompt",
		AcceptanceCriteria: "Return clear verdict line",
		TestDiff:           "+func TestRenderATDDDiagnostic(t *testing.T) {}",
		TestOutput:         "PASS",
	})
	if err != nil {
		t.Fatalf("RenderATDDDiagnostic() error = %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestPromptRendererInterfaceIncludesRenderTDDPhases(t *testing.T) {
	var r PromptRenderer = &mockPromptRenderer{}

	redOutput, err := r.RenderTDDRed(&prompt.TDDRedContext{BeadTitle: "b"})
	if err != nil {
		t.Fatalf("RenderTDDRed() error = %v", err)
	}
	if redOutput == "" {
		t.Fatal("expected non-empty red output")
	}

	greenOutput, err := r.RenderTDDGreen(&prompt.TDDGreenContext{BeadTitle: "b"})
	if err != nil {
		t.Fatalf("RenderTDDGreen() error = %v", err)
	}
	if greenOutput == "" {
		t.Fatal("expected non-empty green output")
	}
}

func TestPromptRendererInterfaceIncludesLastDiagnostics(t *testing.T) {
	var r PromptRenderer = &mockPromptRenderer{}
	if got := r.LastDiagnostics(); got != nil {
		t.Fatalf("LastDiagnostics() = %#v, want nil", got)
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
		Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
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
		Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Verify that config defaults were applied
	// Precheck defaults to disabled (false) due to false-positive risk.
	if cfg.Precheck.Enabled == nil {
		t.Error("expected Precheck.Enabled to be set to non-nil after NewRunnerWithDeps")
	}
	if cfg.Precheck.Enabled != nil && *cfg.Precheck.Enabled {
		t.Error("expected Precheck.Enabled to default to false")
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

func newRunnerWithMocks(t *testing.T, cfg *config.Config, deps Deps) (*Runner, *strings.Builder) {
	t.Helper()

	t.Setenv("TMUX", "")

	if cfg.Paths.Logs == "" {
		cfg.Paths.Logs = t.TempDir()
	}

	if deps.Beads == nil {
		deps.Beads = &mockBeadClient{}
	}
	if deps.Router == nil {
		deps.Router = newMockRouterFromClaudeClient(&mockClaudeClient{})
	}
	if deps.Analyzer == nil {
		deps.Analyzer = &mockFailureAnalyzer{}
	}
	if deps.Renderer == nil {
		deps.Renderer = &mockPromptRenderer{}
	}
	if deps.Logger == nil {
		deps.Logger = &mockIterationLogger{}
	}
	if deps.CmdRunner == nil {
		deps.CmdRunner = func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "", 0, nil
		}
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}
	return r, &buf
}

func TestNewRunnerWithMocksSetsLogs(t *testing.T) {
	cfg := &config.Config{}

	_, _ = newRunnerWithMocks(t, cfg, Deps{})

	if cfg.Paths.Logs == "" {
		t.Fatal("expected cfg.Paths.Logs to be set")
	}
	info, err := os.Stat(cfg.Paths.Logs)
	if err != nil {
		t.Fatalf("expected cfg.Paths.Logs to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected cfg.Paths.Logs to be a directory, got %v", info.Mode())
	}
}

func TestNewRunnerWithMocksClearsTmuxEnv(t *testing.T) {
	t.Setenv("TMUX", "1")

	_, _ = newRunnerWithMocks(t, &config.Config{}, Deps{})

	if got := os.Getenv("TMUX"); got != "" {
		t.Fatalf("expected TMUX env to be cleared, got %q", got)
	}
}

func TestNewRunnerWithMocksUsesNoopCmdRunner(t *testing.T) {
	workDir := t.TempDir()
	fileName := "noop-runner.txt"
	filePath := filepath.Join(workDir, fileName)

	r, _ := newRunnerWithMocks(t, &config.Config{}, Deps{})

	if _, _, _, err := r.cmdRunnerFn(context.Background(), "touch "+fileName, workDir); err != nil {
		t.Fatalf("cmdRunnerFn returned error: %v", err)
	}

	if _, err := os.Stat(filePath); err == nil {
		t.Fatalf("expected no file created by noop cmd runner at %s", filePath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected stat error: %v", err)
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

	r, buf := newRunnerWithMocks(
		t,
		&config.Config{Models: config.ModelsConfig{P1: "sonnet"}},
		Deps{Beads: beads},
	)

	err := r.Run(context.Background(), 5, time.Time{}, nil, true)
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

	r, buf := newRunnerWithMocks(t, &config.Config{}, Deps{Beads: beads})

	err := r.Run(context.Background(), 0, time.Time{}, nil, false)
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

	r, buf := newRunnerWithMocks(t, &config.Config{}, Deps{Beads: beads})

	err := r.Run(context.Background(), 2, time.Time{}, nil, true)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Reached max iterations") {
		t.Errorf("expected max iterations message, got: %s", buf.String())
	}
}

func TestRunWithMocks_UsesLoopMaxIterationsWhenSessionIterationsZero(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			switch callCount {
			case 1:
				return &bead.Bead{ID: "test-1", Title: "First bead", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}, nil
			case 2:
				return &bead.Bead{ID: "test-2", Title: "Second bead", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}, nil
			default:
				return nil, nil
			}
		},
	}

	cfg := &config.Config{Loop: config.LoopConfig{MaxIterations: 1}}
	r, _ := newRunnerWithMocks(t, cfg, Deps{Beads: beads})

	err := r.Run(context.Background(), 0, time.Time{}, nil, true)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected Ready to be called once due to loop max iterations, got %d", callCount)
	}
}

func TestStatusWithMocks(t *testing.T) {
	withFastStatusReaders(t)

	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{ID: "bead-42", Title: "Important task", Priority: 0, Labels: []string{"complexity:high"}, ExpectedOutputs: []string{}}, nil
		},
	}

	var buf strings.Builder
	cfg := &config.Config{Models: config.ModelsConfig{P0: "opus", Labels: map[string]string{"complexity:high": "opus"}}}
	cfg.Paths.Specs = ".gromit/specs"
	cfg.Paths.Plans = ".gromit/plans"
	gromitDir := t.TempDir()
	status := Status{
		Running:   true,
		Iteration: 2,
		BeadID:    "bead-42",
		BeadTitle: "Important task",
		Model:     "opus",
		StartedAt: time.Now().Add(-3 * time.Minute),
		ElapsedS:  180,
		PID:       424242,
	}
	if err := writeStatusFile(gromitDir, status); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}
	r, _ := NewRunnerWithDeps(
		cfg,
		&buf, gromitDir,
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})
	r.processChecker = func(pid int) bool {
		return true
	}

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
	if !strings.Contains(output, "Run: iteration 2") {
		t.Errorf("expected 'Run: iteration 2' in output, got: %s", output)
	}
	if !strings.Contains(output, "Health:") {
		t.Errorf("expected 'Health:' in output, got: %s", output)
	}
	if strings.Contains(output, "Warning: stale run detected") {
		t.Errorf("did not expect stale run warning, got: %s", output)
	}
}

func TestStatusWithMocks_NoWork(t *testing.T) {
	withFastStatusReaders(t)

	beads := &mockBeadClient{ReadyFn: func() (*bead.Bead, error) { return nil, nil }}

	var buf strings.Builder
	cfg := &config.Config{}
	cfg.Paths.Specs = ".gromit/specs"
	cfg.Paths.Plans = ".gromit/plans"
	gromitDir := t.TempDir()
	status := Status{
		Running:   true,
		Iteration: 3,
		BeadID:    "bead-77",
		BeadTitle: "No-op",
		Model:     "haiku",
		StartedAt: time.Now().Add(-1 * time.Minute),
		ElapsedS:  60,
		PID:       424242,
	}
	if err := writeStatusFile(gromitDir, status); err != nil {
		t.Fatalf("failed to write status.json: %v", err)
	}
	r, _ := NewRunnerWithDeps(cfg, &buf, gromitDir,
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})
	r.processChecker = func(pid int) bool {
		return true
	}

	if err := r.Status(); err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	// New status shows pipeline, run, health sections with "No work in pipeline" recommendation
	if !strings.Contains(buf.String(), "Run: iteration 3") {
		t.Errorf("expected 'Run: iteration 3' in output, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "No work in pipeline") {
		t.Errorf("expected 'No work in pipeline' in output, got: %s", buf.String())
	}
	if strings.Contains(buf.String(), "Warning: stale run detected") {
		t.Errorf("did not expect stale run warning, got: %s", buf.String())
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

	bc := &runtypes.BeadContext{
		Bead:              &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:            &IterationResult{},
		Model:             "sonnet",
		PromptCtx:         &prompt.Context{Model: "sonnet", ConfirmedLearnings: []learnings.Learning{}, RecentLearnings: []learnings.Learning{}},
		RetriesThisModel:  0,
		MaxRetries:        2,
		MaxRetriesPerBead: 5,
	}

	claudeResult := &claude.Result{Success: false, Output: "compile error: missing import"}
	continueLoop := r.escalationHandler.AnalyzeAndHandleFailure(context.Background(), bc, claudeResult)

	if !continueLoop {
		t.Error("expected continueLoop=true for recoverable failure")
	}
	if bc.RetriesThisModel != 1 {
		t.Errorf("expected retriesThisModel=1, got %d", bc.RetriesThisModel)
	}
	if !bc.PromptCtx.IsRetry {
		t.Error("expected IsRetry=true after recoverable failure")
	}
	if bc.PromptCtx.FailureContext != "Add the missing import statement" {
		t.Errorf("expected failure context from analysis, got %q", bc.PromptCtx.FailureContext)
	}
}

func TestRunWithMocks_ContextCancellation(t *testing.T) {
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{ID: "test-1", Title: "Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}, nil
		},
	}

	r, _ := newRunnerWithMocks(t, &config.Config{}, Deps{Beads: beads})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := r.Run(ctx, 0, time.Time{}, nil, false)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}
}

func TestIterationLogWithMocks(t *testing.T) {
	mockLog := &mockIterationLogger{}
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "done"}, nil
		},
	}

	r, _ := newRunnerWithMocks(
		t,
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}},
		Deps{Router: newMockRouterFromClaudeClient(mockClaude), Logger: mockLog},
	)

	b := &bead.Bead{ID: "log-test", Title: "Log Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)
	r.writeIterationLog(1, result)
	_ = r.logger.Close()

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
	cfg := &config.Config{Models: config.ModelsConfig{P0: "opus", P1: "sonnet", P2: "haiku", Labels: map[string]string{"complexity:high": "opus", "complexity:low": "haiku"}}}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

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
			if got := escalation.SelectModel(cfg, tt.bead); got != tt.expected {
				t.Errorf("SelectModel() = %q, want %q", got, tt.expected)
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

	b := &bead.Bead{ID: "fail-test", Title: "Fail Test", Description: "Test build failure handling", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if result.Success {
		t.Error("expected failure")
	}
	if result.Error == nil {
		t.Error("expected error to be set")
	}
	if mockAnalyzerObj.AnalyzeCalls < 1 {
		t.Errorf("expected at least 1 analyze call, got %d", mockAnalyzerObj.AnalyzeCalls)
	}
}

func TestHandleScopeTooLargeWithMocks(t *testing.T) {
	beads := &mockBeadClient{}
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(&config.Config{}, &buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(&mockClaudeClient{}), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "big-1", Title: "Big Task", Labels: []string{}, ExpectedOutputs: []string{}},
		Result: &IterationResult{},
	}
	providerResult := &provider.Result{Output: "SCOPE_TOO_LARGE: This task needs to be split"}

	r.handleScopeTooLarge(bc, providerResult, "needs split")

	if bc.Result.Error == nil || !strings.Contains(bc.Result.Error.Error(), "scope too large") {
		t.Errorf("expected scope too large error, got: %v", bc.Result.Error)
	}
	if len(beads.Comments) != 1 || !strings.Contains(beads.Comments[0].Comment, "Scope too large") {
		t.Error("expected scope too large comment on bead")
	}
}

func TestRunWithMocks_BeadReadyError(t *testing.T) {
	beads := &mockBeadClient{ReadyFn: func() (*bead.Bead, error) { return nil, fmt.Errorf("bd CLI not found") }}

	r, _ := newRunnerWithMocks(t, &config.Config{}, Deps{Beads: beads})

	err := r.Run(context.Background(), 1, time.Time{}, nil, false)
	if err == nil || !strings.Contains(err.Error(), "getting next bead") {
		t.Errorf("expected 'getting next bead' error, got: %v", err)
	}
}

func TestProcessBeadWithMocks_ValidationEnabled(t *testing.T) {
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build ok"}, nil
		},
	}

	// Validation runs commands directly via validation.Runner's CmdRunnerFn,
	// injected through Deps.CmdRunner at construction time.
	cmdRunCount := 0
	mockCmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		cmdRunCount++
		return "ok", "", 0, nil
	}

	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:     config.ClaudeConfig{BeadTimeout: 60},
			Validation: config.ValidationConfig{Enabled: true, Commands: []string{"go test ./..."}},
			Models:     config.ModelsConfig{Validation: "haiku"},
		},
		&buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}, CmdRunner: mockCmdRunner})

	b := &bead.Bead{ID: "val-test", Title: "Validation Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if !result.Validated {
		t.Error("expected Validated=true")
	}
	if cmdRunCount != 1 { // 1 validation command (compile_command is empty, so no compilation check)
		t.Errorf("expected 1 command execution (validation), got %d", cmdRunCount)
	}
}

func TestProcessBeadWithMocks_ValidationUsesInjectedCmdRunner(t *testing.T) {
	// Expected failure: Deps struct does not have a CmdRunner field yet.
	// NewRunnerWithDeps passes defaultCmdRunner directly to validation.NewRunner
	// at construction time (line 343 of runner.go), so setting r.cmdRunnerFn after
	// construction has no effect on the validation runner. The fix must add a
	// CmdRunner field to Deps and wire it into validation.NewRunner so tests
	// (and callers) can inject a mock command runner at construction time.
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "build ok"}, nil
		},
	}

	cmdRunCount := 0
	mockCmdRunner := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		cmdRunCount++
		return "ok", "", 0, nil
	}

	var buf strings.Builder
	r, err := NewRunnerWithDeps(
		&config.Config{
			Claude:     config.ClaudeConfig{BeadTimeout: 60},
			Validation: config.ValidationConfig{Enabled: true, Commands: []string{"go test ./..."}},
			Models:     config.ModelsConfig{Validation: "haiku"},
		},
		&buf, t.TempDir(),
		Deps{
			Beads:     &mockBeadClient{},
			Router:    newMockRouterFromClaudeClient(mockClaude),
			Analyzer:  &mockFailureAnalyzer{},
			Renderer:  &mockPromptRenderer{},
			Logger:    &mockIterationLogger{},
			CmdRunner: mockCmdRunner, // Expected failure: CmdRunner field does not exist on Deps
		})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps: %v", err)
	}

	b := &bead.Bead{ID: "val-inject-test", Title: "Validation Injection Test", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if !result.Validated {
		t.Error("expected Validated=true when validation commands pass")
	}
	if cmdRunCount != 1 { // 1 validation command (compile_command is empty, so no compilation check)
		t.Errorf("expected injected CmdRunner called 1 time (validation only), got %d", cmdRunCount)
	}
}

func TestEscalationWithMocks(t *testing.T) {
	// The escalation handler allows one common-cause retry on the same tier before
	// escalating. The mock always fails on haiku so the retry also fails, triggering
	// escalation. Sonnet then succeeds on the third call.
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			if model == "haiku" {
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

	b := &bead.Bead{ID: "escalate-test", Title: "Escalation Test", Description: "Test tier escalation on failure", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
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
	// The handler retries once on haiku (common-cause), then escalates to sonnet:
	// call 1: haiku (fail), call 2: haiku retry (fail), call 3: sonnet (success).
	if len(mockClaude.StreamRunCalls) != 3 {
		t.Fatalf("expected 3 stream run calls (2 haiku + 1 sonnet), got %d", len(mockClaude.StreamRunCalls))
	}
	if mockClaude.StreamRunCalls[0].Model != "haiku" {
		t.Errorf("expected first call with haiku, got %q", mockClaude.StreamRunCalls[0].Model)
	}
	if mockClaude.StreamRunCalls[1].Model != "haiku" {
		t.Errorf("expected second call with haiku (common-cause retry), got %q", mockClaude.StreamRunCalls[1].Model)
	}
	if mockClaude.StreamRunCalls[2].Model != "sonnet" {
		t.Errorf("expected third call with sonnet (after escalation), got %q", mockClaude.StreamRunCalls[2].Model)
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

	b := &bead.Bead{ID: "unclear-test", Title: "Unclear Test", Description: "Test unclear spec handling", Priority: 1, Labels: []string{}, ExpectedOutputs: []string{}}
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
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Git:    config.GitConfig{AutoPush: boolPtrInterfaces(false)},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: &mockIterationLogger{}})

	if err := r.Run(context.Background(), 1, time.Time{}, nil, false); err != nil {
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
		&config.Config{
			Claude:   config.ClaudeConfig{BeadTimeout: 60},
			Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30},
			Git:      config.GitConfig{AutoPush: boolPtrInterfaces(false)},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, nil, false); err != nil {
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
	if log.Model != "precheck" {
		t.Errorf("expected Model 'precheck', got %q", log.Model)
	}
	if !log.Success {
		t.Error("expected Success=true for precheck_skipped")
	}

	// Verify iteration counter was NOT incremented (should be 1, not 2)
	// The iteration field in the log should be 1 (iteration + 1 where iteration starts at 0)
	if log.Iteration != 1 {
		t.Errorf("expected Iteration=1 (not incremented), got %d", log.Iteration)
	}

	// Verify console output mentions precheck (with verification since it defaults to enabled)
	output := buf.String()
	if !strings.Contains(output, "Pre-check: acceptance criteria already met (verified)") {
		t.Errorf("expected precheck verified message in output, got: %s", output)
	}
	if !strings.Contains(output, "auto-closing bead precheck-test") {
		t.Errorf("expected auto-closing message in output, got: %s", output)
	}

	// Verify Claude.Run was called twice (precheck + verification) but StreamRun was NOT called (no build)
	if len(mockClaude.RunCalls) != 2 {
		t.Errorf("expected 2 Claude.Run calls (precheck + verification), got %d", len(mockClaude.RunCalls))
	}
	if len(mockClaude.StreamRunCalls) != 0 {
		t.Errorf("expected 0 Claude.StreamRun calls (no build), got %d", len(mockClaude.StreamRunCalls))
	}
}

func TestRunWithMocks_PrecheckVerificationRejects(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "verify-reject",
				Title:           "Needs work",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature implemented"},
			}, nil
		},
	}

	runCallCount := 0
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			runCallCount++
			if runCallCount == 1 {
				return &claude.Result{Success: true, Output: "PRECHECK_PASSED"}, nil
			}
			return &claude.Result{Success: true, Output: "PRECHECK_NOT_MET"}, nil
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "Build complete"}, nil
		},
	}

	mockLog := &mockIterationLogger{}
	precheckEnabled := true
	verifyEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Precheck: config.PrecheckConfig{
				Enabled:        &precheckEnabled,
				Model:          "haiku",
				TimeoutSeconds: 30,
				Verification: config.PrecheckVerificationConfig{
					Enabled:        &verifyEnabled,
					TimeoutSeconds: 30,
				},
			},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Bead should be closed by the normal build (not by precheck)
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "verify-reject" {
		t.Errorf("expected bead 'verify-reject' to be closed by build, got: %v", beads.ClosedIDs)
	}

	if len(mockClaude.StreamRunCalls) == 0 {
		t.Error("expected StreamRun to be called for normal build after verification rejection")
	}

	output := buf.String()
	if !strings.Contains(output, "verification rejected") {
		t.Errorf("expected verification rejection message in output, got: %s", output)
	}
}

func TestRunWithMocks_PrecheckVerificationConfirms(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "verify-confirm",
				Title:           "Already done",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature implemented"},
			}, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "PRECHECK_PASSED"}, nil
		},
	}

	mockLog := &mockIterationLogger{}
	precheckEnabled := true
	verifyEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Precheck: config.PrecheckConfig{
				Enabled:        &precheckEnabled,
				Model:          "haiku",
				TimeoutSeconds: 30,
				Verification: config.PrecheckVerificationConfig{
					Enabled:        &verifyEnabled,
					TimeoutSeconds: 30,
				},
			},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "verify-confirm" {
		t.Errorf("expected bead 'verify-confirm' to be closed, got: %v", beads.ClosedIDs)
	}

	if len(mockClaude.RunCalls) != 2 {
		t.Errorf("expected 2 Run calls (phase 1 + phase 2), got %d", len(mockClaude.RunCalls))
	}
	if len(mockClaude.StreamRunCalls) != 0 {
		t.Errorf("expected 0 StreamRun calls, got %d", len(mockClaude.StreamRunCalls))
	}
}

func TestRunWithMocks_PrecheckVerificationError(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "verify-error",
				Title:           "Check fails",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature implemented"},
			}, nil
		},
	}

	runCallCount := 0
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			runCallCount++
			if runCallCount == 1 {
				return &claude.Result{Success: true, Output: "PRECHECK_PASSED"}, nil
			}
			return nil, fmt.Errorf("provider unavailable")
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "Build complete"}, nil
		},
	}

	mockLog := &mockIterationLogger{}
	precheckEnabled := true
	verifyEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude: config.ClaudeConfig{BeadTimeout: 60},
			Precheck: config.PrecheckConfig{
				Enabled:        &precheckEnabled,
				Model:          "haiku",
				TimeoutSeconds: 30,
				Verification: config.PrecheckVerificationConfig{
					Enabled:        &verifyEnabled,
					TimeoutSeconds: 30,
				},
			},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Bead should be closed by the normal build (not by precheck)
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "verify-error" {
		t.Errorf("expected bead 'verify-error' to be closed by build, got: %v", beads.ClosedIDs)
	}

	if len(mockClaude.StreamRunCalls) == 0 {
		t.Error("expected StreamRun to be called for normal build after verification error")
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

	if err := r.Run(context.Background(), 5, time.Time{}, nil, false); err != nil {
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

	if err := r.Run(context.Background(), 5, time.Time{}, nil, false); err != nil {
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

func TestRunWithMocks_PrecheckCountsAsIteration(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			switch callCount {
			case 1:
				// First Bead: precheck passes (should be skipped, not count as iteration)
				return &bead.Bead{
					ID:              "bead-1",
					Title:           "Already completed",
					Priority:        1,
					Labels:          []string{},
					ExpectedOutputs: []string{"feature is implemented"},
				}, nil
			case 2:
				// Second Bead: precheck fails, needs real work
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
	verificationDisabled := false
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}, Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30, Verification: config.PrecheckVerificationConfig{Enabled: &verificationDisabled}}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	// Run with maxIterations=1. First bead passes precheck and consumes the run budget.
	// The second bead should not be processed in this run.
	if err := r.Run(context.Background(), 1, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify only the precheck-skipped bead was closed
	if len(beads.ClosedIDs) != 1 {
		t.Errorf("expected 1 bead closed, got %d: %v", len(beads.ClosedIDs), beads.ClosedIDs)
	}
	if len(beads.ClosedIDs) == 1 && beads.ClosedIDs[0] != "bead-1" {
		t.Errorf("expected closed bead to be 'bead-1', got %q", beads.ClosedIDs[0])
	}

	// Verify one iteration log: precheck_skipped
	if len(mockLog.Logs) != 1 {
		t.Fatalf("expected 1 iteration log, got %d", len(mockLog.Logs))
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

	// Verify console output shows max-iteration stop after precheck close.
	output := buf.String()
	if !strings.Contains(output, "auto-closing bead bead-1") {
		t.Errorf("expected auto-closing message for bead-1 in output, got: %s", output)
	}
	if !strings.Contains(output, "Reached max iterations (1), stopping") {
		t.Errorf("expected max-iteration stop message in output, got: %s", output)
	}

	// Verify only one precheck call and no build call
	if len(mockClaude.RunCalls) != 1 {
		t.Errorf("expected 1 Claude.Run call (1 precheck), got %d", len(mockClaude.RunCalls))
	}
	if len(mockClaude.StreamRunCalls) != 0 {
		t.Errorf("expected 0 Claude.StreamRun calls (no build), got %d", len(mockClaude.StreamRunCalls))
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
	if err := r.Run(context.Background(), 10, time.Time{}, nil, false); err != nil {
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

	// Verify the loop terminated with the blocked/stuck message
	output := buf.String()
	if !strings.Contains(output, "All ready beads are blocked or stuck") {
		t.Errorf("expected 'All ready beads are blocked or stuck' message in output, got: %s", output)
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

func TestRunWithMocks_ReadyBeadAlreadyClosedIsSkippedBeforeProcessing(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			return &bead.Bead{
				ID:              "closed-bead",
				Title:           "Already closed elsewhere",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature is implemented"},
			}, nil
		},
		ShowFn: func(id string) (*bead.Bead, error) {
			return &bead.Bead{
				ID:     id,
				Title:  "Already closed elsewhere",
				Status: "closed",
			}, nil
		},
	}

	mockClaude := &mockClaudeClient{}
	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{Claude: config.ClaudeConfig{BeadTimeout: 60}, Precheck: config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30}},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog},
	)

	if err := r.Run(context.Background(), 10, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected Ready called 2 times, got %d", callCount)
	}
	if len(beads.ClosedIDs) != 0 {
		t.Errorf("expected no close attempts for already closed bead, got %v", beads.ClosedIDs)
	}
	if len(mockClaude.RunCalls) != 0 {
		t.Errorf("expected no precheck calls for already closed bead, got %d", len(mockClaude.RunCalls))
	}
	if len(mockClaude.StreamRunCalls) != 0 {
		t.Errorf("expected no build calls for already closed bead, got %d", len(mockClaude.StreamRunCalls))
	}
	if len(mockLog.Logs) != 0 {
		t.Errorf("expected no iteration logs for skipped closed bead, got %d", len(mockLog.Logs))
	}

	output := buf.String()
	if !strings.Contains(output, "already closed; skipping") {
		t.Errorf("expected already-closed skip message in output, got: %s", output)
	}
	if !strings.Contains(output, "All ready beads are blocked or stuck") {
		t.Errorf("expected stuck-beads stop message in output, got: %s", output)
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
	err := r.Run(context.Background(), 10, time.Time{}, nil, false)
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
	verificationDisabled := false
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:     config.ClaudeConfig{BeadTimeout: 60},
			Precheck:   config.PrecheckConfig{Enabled: &precheckEnabled, Model: "haiku", TimeoutSeconds: 30, Verification: config.PrecheckVerificationConfig{Enabled: &verificationDisabled}},
			Loop:       config.LoopConfig{MaxConsecutiveSkips: 3},
			Validation: config.ValidationConfig{Enabled: false},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	// Run should complete without hitting consecutive skip limit
	if err := r.Run(context.Background(), 10, time.Time{}, nil, false); err != nil {
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
	err := r.Run(context.Background(), 10, time.Time{}, nil, false)
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

func setupSessionCompletionProtocolRunner(
	t *testing.T,
	cmdRunner func(ctx context.Context, command string, workDir string) (string, string, int, error),
) (*Runner, *[]string, *mockBeadClient) {
	t.Helper()

	callCount := 0
	events := make([]string, 0, 16)
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "session-completion",
				Title:           "Session completion protocol",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"session completion protocol enforced"},
			}, nil
		},
		CloseFn: func(id string) error {
			events = append(events, "bd close")
			return nil
		},
		SyncFn: func() error {
			events = append(events, "bd sync")
			return nil
		},
	}

	precheckEnabled := false
	mockClaude := &mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "implemented"}, nil
		},
	}

	cfg := &config.Config{
		Claude: config.ClaudeConfig{BeadTimeout: 60},
		Precheck: config.PrecheckConfig{
			Enabled:        &precheckEnabled,
			Model:          "haiku",
			TimeoutSeconds: 30,
		},
		Validation: config.ValidationConfig{
			Enabled:      true,
			Commands:     []string{"go test ./...", "go vet ./...", "go build ./..."},
			FullCommands: []string{"go test ./...", "go vet ./...", "go build ./..."},
		},
		Git: config.GitConfig{
			AutoPush:    boolPtrInterfaces(true),
			PushFailure: "stop",
		},
	}

	r, err := NewRunnerWithDeps(
		cfg,
		&strings.Builder{},
		t.TempDir(),
		Deps{
			Beads:    beads,
			Router:   newMockRouterFromClaudeClient(mockClaude),
			Analyzer: &mockFailureAnalyzer{},
			Renderer: &mockPromptRenderer{},
			Logger:   &mockIterationLogger{},
			CmdRunner: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
				events = append(events, command)
				if cmdRunner != nil {
					return cmdRunner(ctx, command, workDir)
				}
				return "", "", 0, nil
			},
			ArgvRunner: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
				command := program
				if len(args) > 0 {
					command = command + " " + strings.Join(args, " ")
				}
				events = append(events, command)
				if cmdRunner != nil {
					return cmdRunner(ctx, command, workDir)
				}
				return "", "", 0, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() failed: %v", err)
	}

	return r, &events, beads
}

func boolPtrInterfaces(v bool) *bool {
	return &v
}

func assertEventOrderContainsSubsequence(t *testing.T, events []string, expected []string) {
	t.Helper()

	next := 0
	for _, event := range events {
		if next < len(expected) && event == expected[next] {
			next++
		}
	}
	if next != len(expected) {
		t.Fatalf("missing ordered subsequence %v in events %v", expected, events)
	}
}

func TestRunWithMocks_SessionCompletionProtocolOrder(t *testing.T) {
	r, events, _ := setupSessionCompletionProtocolRunner(t, nil)

	if err := r.Run(context.Background(), 1, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	assertEventOrderContainsSubsequence(t, *events, AndonSessionCompletionRequiredSequence)
}

func TestRunWithMocks_SessionCompletionRetriesRebaseBeforePush(t *testing.T) {
	pullCalls := 0
	pushCalls := 0
	r, _, _ := setupSessionCompletionProtocolRunner(t, func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		switch command {
		case "git pull --rebase":
			pullCalls++
			if pullCalls == 1 {
				return "", "fatal: could not rebase", 1, fmt.Errorf("rebase failed")
			}
			return "ok", "", 0, nil
		case "git push":
			pushCalls++
		}
		return "", "", 0, nil
	})

	if err := r.Run(context.Background(), 1, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if pullCalls != SessionCompletionRebaseRetryCount {
		t.Fatalf("expected %d rebase attempts, got %d", SessionCompletionRebaseRetryCount, pullCalls)
	}
	if pushCalls != 1 {
		t.Fatalf("expected git push after successful retry, got %d pushes", pushCalls)
	}
}

func TestRunWithMocks_SessionCompletionVerifiesUpToDateStatus(t *testing.T) {
	statusCalls := 0
	r, _, _ := setupSessionCompletionProtocolRunner(t, func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		if command == "git status --short --branch" {
			statusCalls++
			return "## main...origin/main", "", 0, nil
		}
		return "", "", 0, nil
	})

	if err := r.Run(context.Background(), 1, time.Time{}, nil, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if statusCalls != 1 {
		t.Fatalf("expected %q to execute once, got %d", SessionCompletionUpToDateCommand, statusCalls)
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

func (m *mockClaudeProviderAdapter) IsValidationPassed(result *provider.Result) bool {
	return result.Success
}

func (m *mockClaudeProviderAdapter) IsScopeTooLarge(result *provider.Result) (bool, string) {
	return false, ""
}

// newMockRouterFromClaudeClient creates a router wrapping a mockClaudeClient for unit tests
func newMockRouterFromClaudeClient(client *mockClaudeClient) *provider.Router {
	adapter := &mockClaudeProviderAdapter{client: client}
	return provider.NewSingleProviderRouter(adapter)
}
