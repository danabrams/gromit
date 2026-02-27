package pipeline

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestPipeline_Plan_MissingSpec(t *testing.T) {
    tmpDir := t.TempDir()
    specsDir := filepath.Join(tmpDir, "specs")
    plansDir := filepath.Join(tmpDir, "plans")

    if err := os.MkdirAll(specsDir, 0o755); err != nil {
        t.Fatalf("failed to create specs dir: %v", err)
    }
    if err := os.MkdirAll(plansDir, 0o755); err != nil {
        t.Fatalf("failed to create plans dir: %v", err)
    }

    deps := &Deps{
        AgentResolver: &testAgentResolver{},
        PlanRenderer:  &testPlanRenderer{},
    }
    paths := &Paths{
        GromitDir: tmpDir,
        SpecsDir:  specsDir,
        PlansDir:  plansDir,
    }

    p := New(deps, paths)

    _, err := p.Plan(context.Background(), PlanInput{SpecName: "missing-spec"})
    if err == nil {
        t.Fatalf("expected error when spec is missing")
    }
    if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "spec") {
        t.Fatalf("unexpected error for missing spec: %v", err)
    }
}

func TestPipeline_Plan_DuplicatePlanError(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	specName := "authentication"
	specPath := filepath.Join(specsDir, specName+".md")
	if err := os.WriteFile(specPath, []byte("# Auth Spec"), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}
	plansPath := filepath.Join(plansDir, specName+".md")
	if err := os.WriteFile(plansPath, []byte("# Auth Plan"), 0644); err != nil {
		t.Fatalf("failed to write plan file: %v", err)
	}

	deps := &Deps{
		AgentResolver: &testAgentResolver{},
		PlanRenderer:  &testPlanRenderer{},
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	_, err := p.Plan(context.Background(), PlanInput{SpecName: specName})
	if err == nil {
		t.Fatalf("expected error when plan already exists")
	}
	if !strings.Contains(err.Error(), "plan" ) || !strings.Contains(err.Error(), "already") {
		t.Fatalf("unexpected error for duplicate plan: %v", err)
	}
}

func TestPipeline_Plan_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")
	gromitDir := tmpDir

	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	specName := "feature-plan"
	specPath := filepath.Join(specsDir, specName+".md")
	if err := os.WriteFile(specPath, []byte("# Feature Plan Spec\n\nThis is a test spec."), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	deps := &Deps{
		AgentResolver: &testAgentResolver{},
		PlanRenderer:  &testPlanRenderer{},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	session, err := p.Plan(context.Background(), PlanInput{SpecName: specName})
	if err != nil {
		t.Fatalf("Plan() should succeed with valid spec: %v", err)
	}
	if session == nil {
		t.Fatalf("Plan() should return non-nil session on success")
	}
}
