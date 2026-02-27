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
