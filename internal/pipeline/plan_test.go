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
	if !strings.Contains(err.Error(), "plan") || !strings.Contains(err.Error(), "already") {
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

func TestPipeline_Plan_TempFileClearedOnSuccess(t *testing.T) {
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

	specName := "test-cleanup"
	specPath := filepath.Join(specsDir, specName+".md")
	if err := os.WriteFile(specPath, []byte("# Test Cleanup Spec\n\nTest spec."), 0644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	// Use a tracking agent that records which file path it received
	trackingAgent := &testTrackingAgent{recordedPromptPath: ""}

	deps := &Deps{
		AgentResolver: &testTrackingAgentResolver{agent: trackingAgent},
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
		t.Fatalf("Plan() should succeed: %v", err)
	}
	if session == nil {
		t.Fatalf("Plan() should return non-nil session on success")
	}

	// Check that the temp file was created
	recordedPath := trackingAgent.recordedPromptPath
	if recordedPath == "" {
		t.Fatalf("tracking agent should have recorded a prompt path")
	}

	// After Plan() returns successfully, the temp file should be deleted (cleanup() called)
	if _, err := os.Stat(recordedPath); err == nil {
		t.Fatalf("temp prompt file should be deleted on success, but %q still exists", recordedPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking temp file: %v", err)
	}
}

// testTrackingAgent records the prompt path it receives
type testTrackingAgent struct {
	recordedPromptPath string
}

func (m *testTrackingAgent) Name() string {
	return "test-tracking-agent"
}

func (m *testTrackingAgent) Launch(promptPath string) error {
	m.recordedPromptPath = promptPath
	return nil
}

func (m *testTrackingAgent) LaunchInDir(promptPath, dir string) error {
	m.recordedPromptPath = promptPath
	return nil
}

// testTrackingAgentResolver returns a specific tracking agent
type testTrackingAgentResolver struct {
	agent *testTrackingAgent
}

func (m *testTrackingAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (Agent, error) {
	return m.agent, nil
}
