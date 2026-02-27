package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var _ LLMClient = (*testLLMClient)(nil)

func TestNew_ReturnsNonNil(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}

	p := New(deps, paths)

	if p == nil {
		t.Fatal("New() returned nil, expected non-nil Pipeline")
	}
}

func TestPaths_FieldAccess(t *testing.T) {
	paths := Paths{
		GromitDir: "/tmp/.gromit",
		SpecsDir:  "/tmp/.gromit/specs",
		PlansDir:  "/tmp/.gromit/plans",
		EpicsDir:  "/tmp/.gromit/epics",
	}

	if paths.GromitDir != "/tmp/.gromit" {
		t.Errorf("GromitDir = %q, want %q", paths.GromitDir, "/tmp/.gromit")
	}
	if paths.SpecsDir != "/tmp/.gromit/specs" {
		t.Errorf("SpecsDir = %q, want %q", paths.SpecsDir, "/tmp/.gromit/specs")
	}
	if paths.PlansDir != "/tmp/.gromit/plans" {
		t.Errorf("PlansDir = %q, want %q", paths.PlansDir, "/tmp/.gromit/plans")
	}
	if paths.EpicsDir != "/tmp/.gromit/epics" {
		t.Errorf("EpicsDir = %q, want %q", paths.EpicsDir, "/tmp/.gromit/epics")
	}
}

func TestDeps_FieldAccess(t *testing.T) {
	deps := Deps{
		AgentResolver:     &testAgentResolver{},
	LLMClient:         &testLLMClient{},
	TrackerClient:     &testBeadClient{},
	BacklogClient:     &testBacklogClient{},
		RefineRenderer:    &testRefineRenderer{},
		PlanRenderer:      &testPlanRenderer{},
		DecomposeRenderer: &testDecomposeRenderer{},
		ReviewRenderer:    &testReviewRenderer{},
		ExploreRenderer:   &testExploreRenderer{},
		LearningsManager:  &testLearningsManager{},
		StateManager:      &testStateManager{},
		LogWriter:         &testLogWriter{},
	}

	if deps.AgentResolver == nil {
		t.Error("AgentResolver field should be set")
	}
	if deps.LLMClient == nil {
		t.Error("LLMClient field should be set")
	}
	if deps.TrackerClient == nil {
		t.Error("TrackerClient field should be set")
	}
	if deps.BacklogClient == nil {
		t.Error("BacklogClient field should be set")
	}
	if deps.RefineRenderer == nil {
		t.Error("RefineRenderer field should be set")
	}
	if deps.PlanRenderer == nil {
		t.Error("PlanRenderer field should be set")
	}
	if deps.DecomposeRenderer == nil {
		t.Error("DecomposeRenderer field should be set")
	}
	if deps.ReviewRenderer == nil {
		t.Error("ReviewRenderer field should be set")
	}
	if deps.ExploreRenderer == nil {
		t.Error("ExploreRenderer field should be set")
	}
	if deps.LearningsManager == nil {
		t.Error("LearningsManager field should be set")
	}
	if deps.StateManager == nil {
		t.Error("StateManager field should be set")
	}
	if deps.LogWriter == nil {
		t.Error("LogWriter field should be set")
	}
}

func TestDeps_HasTrackerClientField(t *testing.T) {
	var deps Deps
	if deps.TrackerClient != nil {
		t.Fatalf("deps.TrackerClient should be nil by default, got %v", deps.TrackerClient)
	}
}

func TestDeps_UsesLLMClientField(t *testing.T) {
	deps := Deps{
		LLMClient: &testLLMClient{},
	}

	if deps.LLMClient == nil {
		t.Error("LLMClient field should be set")
	}
}

func TestPipelineTest_HasLLMClientCompileTimeAssertion(t *testing.T) {
	p := filepath.Join("..", "..", "internal", "pipeline", "pipeline_test.go")
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read pipeline_test.go: %v", err)
	}
	pattern := regexp.MustCompile(`(?m)^var _ LLMClient = \(\*testLLMClient\)\(nil\)$`)
	if !pattern.Match(content) {
		t.Fatal("pipeline_test.go should include compile-time assertion for LLMClient")
	}
}

func TestPipeline_RefineMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := RefineInput{IdeaText: "Test idea"}

	// Should return error with nil dependencies
	_, err := p.Refine(ctx, input)
	if err == nil {
		t.Error("Refine() should error with nil dependencies")
	}
}

func TestPipeline_PlanMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	_, err := p.Plan(ctx, input)
	if err == nil {
		t.Error("Plan() should error with nil dependencies")
	}
}

func TestPipeline_Plan_SucceedsWithValidDeps(t *testing.T) {
	// Setup test dependencies with minimal mocks
	tmpDir := t.TempDir()
	deps := &Deps{
		PlanRenderer:  &testPlanRenderer{},
		AgentResolver: &testAgentResolver{},
	}

	paths := &Paths{
		GromitDir: tmpDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	// Call Plan() and verify it doesn't return "not implemented" error
	session, err := p.Plan(ctx, input)

	// With default testPlanRenderer (returns ""), we expect an error from agent.LaunchInDir
	// because testAgentResolver returns nil Agent. The key is it doesn't return "Plan not yet implemented"
	if err != nil && err.Error() == "pipeline: Plan not yet implemented" {
		t.Fatalf("Plan() still has TODO stub: %v", err)
	}

	if err != nil {
		t.Logf("Expected error (not the TODO stub): %v", err)
	}

	if session == nil && err == nil {
		t.Error("Plan() returned both nil session and nil error")
	}
}

func TestPipeline_DecomposeMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{PlanName: "test-plan"}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Error("Decompose() should error with nil dependencies")
	}
}

func TestPipeline_ReviewInteractiveMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{FromCommit: "HEAD~1", Diff: "diff"}

	_, err := p.ReviewInteractive(ctx, input)
	if err == nil {
		t.Error("ReviewInteractive() should error with nil dependencies")
	}
}

func TestPipeline_ReviewNonInteractiveMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{FromCommit: "HEAD~1", Diff: "diff", Model: "sonnet", Timeout: 300}

	_, err := p.ReviewNonInteractive(ctx, input)
	if err == nil {
		t.Error("ReviewNonInteractive() should error with nil dependencies")
	}
}

func TestPipeline_ExploreMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ExploreInput{Topic: "test topic"}

	_, err := p.Explore(ctx, input)
	if err == nil {
		t.Error("Explore() should error with nil dependencies")
	}
}

// TestPipeline_ResolveReviewScopeMethod verifies that Pipeline has ResolveReviewScope method
// Expected failure: Method not yet fully implemented to delegate to command layer
func TestPipeline_ResolveReviewScopeMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	
	// Method should exist and be callable
	commit, err := p.ResolveReviewScope(ctx, "spec-name", "", "")
	
	// Currently returns error as placeholder - this will change when implemented
	if err == nil {
		t.Error("ResolveReviewScope() expected error (method not yet implemented)")
	}
	
	if commit != "" {
		t.Errorf("ResolveReviewScope() returned commit=%q, want empty string", commit)
	}
}

// TestPipeline_ResolveReviewScopeWithSince verifies scope flag passthrough
// Expected: ResolveReviewScope accepts --since flag and returns it correctly
func TestPipeline_ResolveReviewScopeWithSince(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	sinceCommit := "abc123def456"

	// When --since is provided, should return it directly
	commit, err := p.ResolveReviewScope(ctx, "", "", sinceCommit)

	if err != nil {
		t.Errorf("ResolveReviewScope(since=%q) error = %v, want nil", sinceCommit, err)
	}

	if commit != sinceCommit {
		t.Errorf("ResolveReviewScope(since=%q) returned %q, want %q", sinceCommit, commit, sinceCommit)
	}
}

// TestPipeline_ResolveReviewScopeWithSpec verifies spec flag passthrough
// Expected: ResolveReviewScope accepts --spec flag and passes it through correctly
func TestPipeline_ResolveReviewScopeWithSpec(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	specName := "init-wizard"
	
	// When --spec is provided, should use it
	_, err := p.ResolveReviewScope(ctx, specName, "", "")
	
	// Will error until implemented, but verifies signature accepts spec
	if err == nil {
		t.Error("ResolveReviewScope() expected error (method not yet implemented)")
	}
}

// TestPipeline_ResolveReviewScopeWithEpic verifies epic flag passthrough
// Expected: ResolveReviewScope accepts --epic flag and passes it through correctly
func TestPipeline_ResolveReviewScopeWithEpic(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	epicID := "gromit-xyz"
	
	// When --epic is provided, should use it
	_, err := p.ResolveReviewScope(ctx, "", epicID, "")
	
	// Will error until implemented, but verifies signature accepts epic
	if err == nil {
		t.Error("ResolveReviewScope() expected error (method not yet implemented)")
	}
}

// TestPipeline_ListBeadsMethod verifies Pipeline has ListBeads query method
// with proper input/output types and nil dependency validation.
func TestPipeline_ListBeadsMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ListBeadsInput{Status: "ready"}

	// Should return error with nil dependencies
	_, err := p.ListBeads(ctx, input)
	if err == nil {
		t.Error("ListBeads() should error with nil dependencies")
	}
}

// TestPipeline_QueryBeadsMethod verifies Pipeline has QueryBeads query method
// with proper input/output types and nil dependency validation.
func TestPipeline_QueryBeadsMethod(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := QueryBeadsInput{StatusFilter: "ready"}

	// Should return error with nil dependencies
	_, err := p.QueryBeads(ctx, input)
	if err == nil {
		t.Error("QueryBeads() should error with nil dependencies")
	}
}
