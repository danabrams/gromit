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
		BeadClient:        &testBeadClient{},
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
	if deps.BeadClient == nil {
		t.Error("BeadClient field should be set")
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
