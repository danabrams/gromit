//go:build acceptance

package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPipeline_PlanValidatesDeps verifies Plan returns error when dependencies are nil.
// Expected failure: Pipeline.Plan() does not validate all required dependencies yet
func TestPipeline_PlanValidatesDeps(t *testing.T) {
	tests := []struct {
		name    string
		deps    *Deps
		paths   *Paths
		wantErr string
	}{
		{
			name:    "nil dependencies",
			deps:    nil,
			paths:   &Paths{},
			wantErr: "nil dependencies",
		},
		{
			name: "nil AgentResolver",
			deps: &Deps{
				AgentResolver:  nil,
				PromptRenderer: &testPromptRenderer{},
			},
			paths:   &Paths{},
			wantErr: "nil AgentResolver",
		},
		{
			name: "nil PromptRenderer",
			deps: &Deps{
				AgentResolver:  &testAgentResolver{},
				PromptRenderer: nil,
			},
			paths:   &Paths{},
			wantErr: "nil PromptRenderer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := New(tc.deps, tc.paths)
			ctx := context.Background()
			input := PlanInput{SpecName: "test-spec"}

			_, err := p.Plan(ctx, input)
			if err == nil {
				t.Fatal("Plan() should return error with invalid dependencies")
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestPipeline_PlanRequiresSpecName verifies Plan returns error when SpecName is empty.
// Expected failure: Pipeline.Plan() does not validate SpecName yet
func TestPipeline_PlanRequiresSpecName(t *testing.T) {
	deps := &Deps{
		AgentResolver:  &testAgentResolver{},
		PromptRenderer: &testPromptRenderer{},
	}
	paths := &Paths{
		SpecsDir: t.TempDir(),
		PlansDir: t.TempDir(),
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: ""} // Empty spec name

	_, err := p.Plan(ctx, input)
	if err == nil {
		t.Fatal("Plan() should return error when SpecName is empty")
	}

	if !strings.Contains(err.Error(), "spec name") && !strings.Contains(err.Error(), "required") {
		t.Errorf("error = %q, want error about missing spec name", err.Error())
	}
}

// TestPipeline_PlanChecksSpecExists verifies Plan returns error when spec file does not exist.
// Expected failure: Pipeline.Plan() does not check spec existence yet
func TestPipeline_PlanChecksSpecExists(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()

	deps := &Deps{
		AgentResolver:  &testAgentResolver{},
		PromptRenderer: &testPromptRenderer{},
	}
	paths := &Paths{
		SpecsDir: specsDir,
		PlansDir: plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "nonexistent-spec"}

	_, err := p.Plan(ctx, input)
	if err == nil {
		t.Fatal("Plan() should return error when spec does not exist")
	}

	if !strings.Contains(err.Error(), "spec") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want error about spec not found", err.Error())
	}
}

// TestPipeline_PlanChecksPlanExists verifies Plan returns error when plan already exists (without Force=true).
// Expected failure: Pipeline.Plan() does not check plan existence yet
func TestPipeline_PlanChecksPlanExists(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	// Create existing plan file
	planPath := filepath.Join(plansDir, "test-spec.md")
	if err := os.WriteFile(planPath, []byte("# Existing Plan\n"), 0644); err != nil {
		t.Fatalf("failed to create existing plan: %v", err)
	}

	deps := &Deps{
		AgentResolver:  &testAgentResolver{},
		PromptRenderer: &testPromptRenderer{},
	}
	paths := &Paths{
		SpecsDir: specsDir,
		PlansDir: plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{
		SpecName: "test-spec",
		Force:    false, // Not forcing re-plan
	}

	_, err := p.Plan(ctx, input)
	if err == nil {
		t.Fatal("Plan() should return error when plan already exists without Force=true")
	}

	if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "already planned") {
		t.Errorf("error = %q, want error about plan already existing", err.Error())
	}
}

// TestPipeline_PlanAllowsForce verifies Plan allows re-planning when Force=true.
// Expected failure: Pipeline.Plan() does not respect Force flag yet
func TestPipeline_PlanAllowsForce(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	// Create existing plan file
	planPath := filepath.Join(plansDir, "test-spec.md")
	if err := os.WriteFile(planPath, []byte("# Old Plan\n"), 0644); err != nil {
		t.Fatalf("failed to create existing plan: %v", err)
	}

	agentLaunched := false
	mockAgent := &planTestMockAgent{
		LaunchFn: func(promptPath string) error {
			agentLaunched = true
			return nil
		},
	}

	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return mockAgent, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				return "# Plan Prompt", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{
		SpecName: "test-spec",
		Force:    true, // Force re-planning
	}

	_, err := p.Plan(ctx, input)
	// With Force=true, should NOT error about existing plan
	// Should proceed to agent launch
	if err != nil && strings.Contains(err.Error(), "already exists") {
		t.Errorf("Plan() with Force=true should not error about existing plan, got: %v", err)
	}

	if !agentLaunched {
		t.Error("Plan() with Force=true should launch agent even when plan exists")
	}
}

// TestPipeline_PlanLoadsSpecFile verifies Plan loads spec file content.
// Expected failure: Pipeline.Plan() does not load spec file via frontmatter.ReadFile yet
func TestPipeline_PlanLoadsSpecFile(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec with frontmatter
	specContent := `---
id: test-spec
created: 2026-01-01
---

# Test Spec

This is a test specification.
`
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	var capturedPromptInput interface{}
	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planTestMockAgent{}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				capturedPromptInput = input
				return "# Plan Prompt", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	_, _ = p.Plan(ctx, input)

	// Verify that RenderPlan was called with context containing spec content
	if capturedPromptInput == nil {
		t.Fatal("RenderPlan was not called with prompt context")
	}

	// The prompt context should contain the spec file content
	// (exact structure depends on implementation, but verify it was passed)
	contextMap, ok := capturedPromptInput.(map[string]interface{})
	if !ok {
		t.Fatalf("RenderPlan input is not a map, got %T", capturedPromptInput)
	}

	// Check that context contains spec-related data
	if _, hasSpec := contextMap["Spec"]; !hasSpec {
		t.Error("RenderPlan context missing 'Spec' field - spec file was not loaded")
	}
}

// TestPipeline_PlanGathersOpenBeadsContext verifies Plan gathers open beads context.
// Expected failure: Pipeline.Plan() does not gather open beads context yet
func TestPipeline_PlanGathersOpenBeadsContext(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	beadClientCalled := false
	var capturedPromptInput interface{}

	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planTestMockAgent{}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				capturedPromptInput = input
				return "# Plan Prompt", nil
			},
		},
		BeadClient: &planTestMockBeadClient{
			ReadyFn: func() (interface{}, error) {
				beadClientCalled = true
				return []interface{}{}, nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	_, _ = p.Plan(ctx, input)

	if !beadClientCalled {
		t.Error("Plan() did not call BeadClient.Ready() to gather open beads context")
	}

	// Verify context contains beads data
	if capturedPromptInput != nil {
		contextMap, ok := capturedPromptInput.(map[string]interface{})
		if ok {
			if _, hasBeads := contextMap["OpenBeads"]; !hasBeads {
				t.Error("RenderPlan context missing 'OpenBeads' field - open beads context not gathered")
			}
		}
	}
}

// TestPipeline_PlanBuildsSystemPrompt verifies Plan builds system prompt with spec context.
// Expected failure: Pipeline.Plan() does not build system prompt correctly yet
func TestPipeline_PlanBuildsSystemPrompt(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "api-spec.md")
	if err := os.WriteFile(specPath, []byte("# API Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	rendererCalled := false
	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planTestMockAgent{}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				rendererCalled = true
				return "# Plan Prompt\n\nPlan this spec.", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "api-spec"}

	_, _ = p.Plan(ctx, input)

	if !rendererCalled {
		t.Error("Plan() did not call PromptRenderer.RenderPlan() to build system prompt")
	}
}

// TestPipeline_PlanWritesTempFile verifies Plan writes prompt to temp file.
// Expected failure: Pipeline.Plan() does not write temp file yet
func TestPipeline_PlanWritesTempFile(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	var capturedPromptPath string
	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planTestMockAgent{
					LaunchFn: func(promptPath string) error {
						capturedPromptPath = promptPath
						return nil
					},
				}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				return "# Plan Prompt Content", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	_, _ = p.Plan(ctx, input)

	if capturedPromptPath == "" {
		t.Fatal("Agent.Launch() was not called with prompt path - temp file not written")
	}

	// Verify temp file was in .gromit/tmp directory
	if !strings.Contains(capturedPromptPath, filepath.Join(gromitDir, "tmp")) {
		t.Errorf("Prompt file path = %q, want path in %s", capturedPromptPath, filepath.Join(gromitDir, "tmp"))
	}

	// Verify temp file contains prompt content (if it still exists)
	if _, err := os.Stat(capturedPromptPath); err == nil {
		content, err := os.ReadFile(capturedPromptPath)
		if err != nil {
			t.Fatalf("failed to read temp file: %v", err)
		}
		if !strings.Contains(string(content), "Plan Prompt Content") {
			t.Errorf("Temp file content = %q, want content containing prompt", string(content))
		}
	}
}

// TestPipeline_PlanResolvesAgent verifies Plan resolves agent with correct phase.
// Expected failure: Pipeline.Plan() does not resolve agent yet
func TestPipeline_PlanResolvesAgent(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	var capturedPhase string
	var capturedFlagOverride string
	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				capturedPhase = phase
				capturedFlagOverride = flagOverride
				return &planTestMockAgent{}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				return "# Plan Prompt", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{
		SpecName:  "test-spec",
		AgentName: "opus",
	}

	_, _ = p.Plan(ctx, input)

	if capturedPhase != "plan" {
		t.Errorf("AgentResolver.Resolve() called with phase=%q, want 'plan'", capturedPhase)
	}

	if capturedFlagOverride != "opus" {
		t.Errorf("AgentResolver.Resolve() called with flagOverride=%q, want 'opus'", capturedFlagOverride)
	}
}

// TestPipeline_PlanGetCommand verifies Plan gets command from agent.
// Expected failure: Pipeline.Plan() does not get command from agent yet
func TestPipeline_PlanGetCommand(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	agentNameCalled := false
	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planTestMockAgent{
					NameFn: func() string {
						agentNameCalled = true
						return "opus"
					},
				}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				return "# Plan Prompt", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	_, _ = p.Plan(ctx, input)

	if !agentNameCalled {
		t.Error("Plan() did not call Agent.Name() to get command info")
	}
}

// TestPipeline_PlanReturnsNonNilSession verifies Plan returns non-nil session and launches agent.
// Expected failure: Pipeline.Plan() does not complete full workflow yet
func TestPipeline_PlanReturnsNonNilSession(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	agentLaunched := false
	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planTestMockAgent{
					LaunchFn: func(promptPath string) error {
						agentLaunched = true
						return nil
					},
				}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				return "# Plan Prompt", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	if session == nil {
		t.Fatal("Plan() returned nil session, want non-nil PlanSession")
	}

	if !agentLaunched {
		t.Error("Plan() did not launch agent - full workflow not implemented")
	}
}

// TestPlanSession_ResultReturnsTypedResult verifies PlanSession.Result() returns PlanResult.
// Expected failure: PlanSession.Result() method does not exist yet
func TestPlanSession_ResultReturnsTypedResult(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planTestMockAgent{
					LaunchFn: func(promptPath string) error {
						// Simulate agent completing
						return nil
					},
				}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				return "# Plan Prompt", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// Wait for session to complete
	if err := session.Wait(); err != nil {
		t.Fatalf("session.Wait() failed: %v", err)
	}

	// Call Result() - this is the new behavior being tested
	result, err := session.Result()
	if err != nil {
		t.Fatalf("session.Result() failed: %v", err)
	}

	// Verify result is PlanResult type (not interface{})
	var _ PlanResult = result

	// Verify result has initialized slice fields
	if result.CreatedPlans == nil {
		t.Error("Result().CreatedPlans is nil, want empty slice - PlanResult fields not initialized")
	}
}

// TestPlanSession_ResultDetectsNewPlanFile verifies Result() detects newly created plan files.
// Expected failure: PlanSession does not detect new plan files in post-processing yet
func TestPlanSession_ResultDetectsNewPlanFile(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "my-spec.md")
	if err := os.WriteFile(specPath, []byte("# My Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planTestMockAgent{
					LaunchFn: func(promptPath string) error {
						// Simulate agent creating plan file
						planPath := filepath.Join(plansDir, "my-spec.md")
						content := "# Implementation Plan\n\nSteps to implement..."
						return os.WriteFile(planPath, []byte(content), 0644)
					},
				}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				return "# Plan Prompt", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "my-spec"}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("session.Result() failed: %v", err)
	}

	// Verify CreatedPlans contains the new plan file
	if len(result.CreatedPlans) == 0 {
		t.Fatal("Result().CreatedPlans is empty, want detected plan file")
	}

	expectedPath := filepath.Join(plansDir, "my-spec.md")
	found := false
	for _, planPath := range result.CreatedPlans {
		if planPath == expectedPath {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Result().CreatedPlans = %v, want to contain %q", result.CreatedPlans, expectedPath)
	}
}

// TestPlanSession_ResultBeforeWait verifies Result() errors when called before Wait().
// Expected failure: PlanSession.Result() does not check completion state yet
func TestPlanSession_ResultBeforeWait(t *testing.T) {
	specsDir := t.TempDir()
	plansDir := t.TempDir()
	gromitDir := t.TempDir()

	// Create spec file
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create spec: %v", err)
	}

	deps := &Deps{
		AgentResolver: &planTestMockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planTestMockAgent{
					LaunchFn: func(promptPath string) error {
						// Never complete (test will call Result before Wait)
						return nil
					},
				}, nil
			},
		},
		PromptRenderer: &planTestMockPromptRenderer{
			RenderPlanFn: func(input interface{}) (string, error) {
				return "# Plan Prompt", nil
			},
		},
	}
	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// Call Result() BEFORE Wait()
	_, err = session.Result()
	if err == nil {
		t.Error("session.Result() before Wait() should return error")
	}

	if !strings.Contains(err.Error(), "not complete") && !strings.Contains(err.Error(), "not ready") {
		t.Errorf("Result() error = %q, want error about session not complete", err.Error())
	}
}

// Mock types for plan_test.go
// Expected failure: These mock types conflict with mocks in explore_test.go and review_test.go

type planTestMockAgent struct {
	NameFn   func() string
	LaunchFn func(promptPath string) error
}

func (m *planTestMockAgent) Name() string {
	if m.NameFn != nil {
		return m.NameFn()
	}
	return "test-agent"
}

func (m *planTestMockAgent) Launch(promptPath string) error {
	if m.LaunchFn != nil {
		return m.LaunchFn(promptPath)
	}
	return nil
}

type planTestMockAgentResolver struct {
	ResolveFn func(phase, flagOverride string, choosePicker bool) (Agent, error)
}

func (m *planTestMockAgentResolver) Resolve(phase, flagOverride string, choosePicker bool) (Agent, error) {
	if m.ResolveFn != nil {
		return m.ResolveFn(phase, flagOverride, choosePicker)
	}
	return nil, fmt.Errorf("ResolveFn not set")
}

type planTestMockPromptRenderer struct {
	RenderPlanFn func(input interface{}) (string, error)
}

func (m *planTestMockPromptRenderer) RenderRefine(input interface{}) (string, error) {
	return "", nil
}

func (m *planTestMockPromptRenderer) RenderPlan(input interface{}) (string, error) {
	if m.RenderPlanFn != nil {
		return m.RenderPlanFn(input)
	}
	return "plan prompt", nil
}

func (m *planTestMockPromptRenderer) RenderDecompose(input interface{}) (string, error) {
	return "", nil
}

func (m *planTestMockPromptRenderer) RenderThoroughReview(ctx interface{}) (string, error) {
	return "", nil
}

func (m *planTestMockPromptRenderer) RenderExplore(ctx interface{}) (string, error) {
	return "", nil
}

type planTestMockBeadClient struct {
	ReadyFn                        func() (interface{}, error)
	ShowFn                         func(id string) (interface{}, error)
	CreateFn                       func(title string, priority int, labels []string, outputs []string) (interface{}, error)
	CreateWithDepsAndDescriptionFn func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error)
	CloseFn                        func(id string) error
}

func (m *planTestMockBeadClient) Ready() (interface{}, error) {
	if m.ReadyFn != nil {
		return m.ReadyFn()
	}
	return []interface{}{}, nil
}

func (m *planTestMockBeadClient) Show(id string) (interface{}, error) {
	if m.ShowFn != nil {
		return m.ShowFn(id)
	}
	return nil, nil
}

func (m *planTestMockBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	if m.CreateFn != nil {
		return m.CreateFn(title, priority, labels, outputs)
	}
	return nil, nil
}

func (m *planTestMockBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
	if m.CreateWithDepsAndDescriptionFn != nil {
		return m.CreateWithDepsAndDescriptionFn(title, priority, labels, criteria, deps, desc)
	}
	return nil, nil
}

func (m *planTestMockBeadClient) Close(id string) error {
	if m.CloseFn != nil {
		return m.CloseFn(id)
	}
	return nil
}
