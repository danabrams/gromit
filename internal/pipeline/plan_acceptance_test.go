//go:build acceptance

package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPlanWorkflow_E2E verifies the complete Plan workflow through Pipeline.Plan()
// Expected failure: Pipeline.Plan() implementation is incomplete and does not orchestrate the full workflow
func TestPlanWorkflow_E2E(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file with frontmatter
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "authentication.md")
	specContent := `---
id: authentication
created: 2026-02-11
---

# Authentication Spec

This spec describes the authentication system.

## Requirements

- User login with email/password
- Session management
- Token refresh logic
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock agent that creates a plan file
	planPath := filepath.Join(plansDir, "authentication.md")
	mockAgent := &planAcceptanceAgent{
		name: "test-agent",
		launchFunc: func(promptPath string) error {
			// Verify prompt file exists and contains expected content
			promptData, err := os.ReadFile(promptPath)
			if err != nil {
				return fmt.Errorf("reading prompt: %w", err)
			}
			promptStr := string(promptData)

			// Verify prompt contains spec content (not frontmatter)
			if !strings.Contains(promptStr, "This spec describes the authentication system") {
				return fmt.Errorf("prompt missing spec body content")
			}
			if strings.Contains(promptStr, "id: authentication") {
				return fmt.Errorf("prompt should not contain frontmatter")
			}

			// Create plan file
			if err := os.MkdirAll(plansDir, 0755); err != nil {
				return err
			}
			planContent := `---
spec: authentication
created: 2026-02-12
---

# Implementation Plan

## Phase 1: Database Schema
- Create users table
- Add session storage

## Phase 2: API Endpoints
- POST /login
- POST /refresh
`
			return os.WriteFile(planPath, []byte(planContent), 0644)
		},
	}

	mockBead := &planAcceptanceBeadClient{
		beads: []planAcceptanceBeadInfo{
			{ID: "bead-1", Title: "Existing feature", Priority: 1},
		},
	}

	mockResolver := &planAcceptanceAgentResolver{
		resolveFunc: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			if phase != "plan" {
				return nil, fmt.Errorf("expected phase 'plan', got %q", phase)
			}
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockResolver,
		BeadClient:    mockBead,
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	// Execute Plan workflow
	ctx := context.Background()
	input := PlanInput{
		SpecName: "authentication",
	}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	if session == nil {
		t.Fatal("Plan() returned nil session")
	}

	// Wait for session to complete
	if err := session.Wait(); err != nil {
		t.Fatalf("session.Wait() failed: %v", err)
	}

	// Verify plan file was created
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		t.Error("Plan file was not created by workflow")
	}

	// Verify plan file contains expected content
	planData, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("reading plan file: %v", err)
	}
	planStr := string(planData)
	if !strings.Contains(planStr, "Implementation Plan") {
		t.Error("Plan file missing expected content")
	}
}

// TestPlanWorkflow_PostProcessingDetectsCreatedPlan verifies post-processing detects new plan files
// Expected failure: Pipeline.Plan() does not implement post-processing to detect created plan files
func TestPlanWorkflow_PostProcessingDetectsCreatedPlan(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create spec
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "caching.md")
	if err := os.WriteFile(specPath, []byte("# Caching Spec\n\nAdd caching layer"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create existing plan BEFORE the session (should not be reported as new)
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	existingPlanPath := filepath.Join(plansDir, "existing-plan.md")
	if err := os.WriteFile(existingPlanPath, []byte("# Old Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock agent that creates NEW plan
	newPlanPath := filepath.Join(plansDir, "caching.md")
	mockAgent := &planAcceptanceAgent{
		name: "test-agent",
		launchFunc: func(promptPath string) error {
			return os.WriteFile(newPlanPath, []byte("# New Caching Plan"), 0644)
		},
	}

	deps := &Deps{
		AgentResolver: &planAcceptanceAgentResolver{
			resolveFunc: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return mockAgent, nil
			},
		},
		BeadClient: &planAcceptanceBeadClient{},
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{
		SpecName: "caching",
	}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("session.Wait() failed: %v", err)
	}

	// Get result - should detect only the NEW plan
	result, err := session.Result()
	if err != nil {
		t.Fatalf("session.Result() failed: %v", err)
	}

	// Verify only the new plan is reported
	if len(result.CreatedPlans) != 1 {
		t.Errorf("CreatedPlans count = %d, want 1 (only new plan)", len(result.CreatedPlans))
	}

	if len(result.CreatedPlans) > 0 && !strings.Contains(result.CreatedPlans[0], "caching.md") {
		t.Errorf("CreatedPlans[0] = %q, want path containing 'caching.md'", result.CreatedPlans[0])
	}

	// Verify existing plan was NOT reported
	for _, plan := range result.CreatedPlans {
		if strings.Contains(plan, "existing-plan.md") {
			t.Error("CreatedPlans includes existing plan, should only list new plans")
		}
	}
}

// TestPlanWorkflow_GathersBeadContext verifies open beads are included in prompt
// Expected failure: Pipeline.Plan() does not gather open beads context
func TestPlanWorkflow_GathersBeadContext(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create spec
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock bead client with multiple open beads
	mockBead := &planAcceptanceBeadClient{
		beads: []planAcceptanceBeadInfo{
			{
				ID:          "bead-123",
				Title:       "Implement user authentication",
				Priority:    0,
				Description: "Add login and logout endpoints",
			},
			{
				ID:          "bead-456",
				Title:       "Fix caching bug",
				Priority:    1,
				Description: "",
			},
		},
	}

	var capturedPrompt string
	mockAgent := &planAcceptanceAgent{
		name: "test-agent",
		launchFunc: func(promptPath string) error {
			data, err := os.ReadFile(promptPath)
			if err != nil {
				return err
			}
			capturedPrompt = string(data)
			// Don't actually create a plan for this test
			return nil
		},
	}

	deps := &Deps{
		AgentResolver: &planAcceptanceAgentResolver{
			resolveFunc: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return mockAgent, nil
			},
		},
		BeadClient: mockBead,
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{
		SpecName: "test-spec",
	}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	_ = session.Wait()

	// Verify prompt contains bead information
	if !strings.Contains(capturedPrompt, "Implement user authentication") {
		t.Error("Prompt missing first bead title")
	}
	if !strings.Contains(capturedPrompt, "Fix caching bug") {
		t.Error("Prompt missing second bead title")
	}
	if !strings.Contains(capturedPrompt, "bead-123") {
		t.Error("Prompt missing bead ID")
	}
	if !strings.Contains(capturedPrompt, "Add login and logout endpoints") {
		t.Error("Prompt missing bead description")
	}

	// Verify there's a section header for beads
	if !strings.Contains(capturedPrompt, "Open Beads") && !strings.Contains(capturedPrompt, "Current") {
		t.Error("Prompt missing beads section header")
	}
}

// TestPlanWorkflow_RespectsForceFlag verifies Force flag allows re-planning
// Expected failure: Pipeline.Plan() does not respect Force flag for re-planning
func TestPlanWorkflow_RespectsForceFlag(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create spec
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create EXISTING plan
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-spec.md")
	if err := os.WriteFile(planPath, []byte("# Old Plan Version 1"), 0644); err != nil {
		t.Fatal(err)
	}

	mockAgent := &planAcceptanceAgent{
		name: "test-agent",
		launchFunc: func(promptPath string) error {
			// Overwrite existing plan
			return os.WriteFile(planPath, []byte("# New Plan Version 2"), 0644)
		},
	}

	deps := &Deps{
		AgentResolver: &planAcceptanceAgentResolver{
			resolveFunc: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return mockAgent, nil
			},
		},
		BeadClient: &planAcceptanceBeadClient{},
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()

	// First attempt WITHOUT force - should fail
	inputNoForce := PlanInput{
		SpecName: "test-spec",
		Force:    false,
	}

	_, err := p.Plan(ctx, inputNoForce)
	if err == nil {
		t.Error("Plan() without force should fail when plan exists, got nil error")
	}
	if err != nil && !strings.Contains(err.Error(), "plan already exists") && !strings.Contains(err.Error(), "already planned") {
		t.Errorf("Expected 'plan already exists' error, got: %v", err)
	}

	// Second attempt WITH force - should succeed
	inputWithForce := PlanInput{
		SpecName: "test-spec",
		Force:    true,
	}

	session, err := p.Plan(ctx, inputWithForce)
	if err != nil {
		t.Fatalf("Plan() with force=true failed: %v (should allow re-planning)", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("session.Wait() failed: %v", err)
	}

	// Verify plan was updated
	planData, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("reading plan file: %v", err)
	}
	planStr := string(planData)
	if !strings.Contains(planStr, "Version 2") {
		t.Error("Plan file was not updated by force re-plan")
	}
	if strings.Contains(planStr, "Version 1") {
		t.Error("Plan file still contains old content")
	}
}

// TestPlanWorkflow_RespectsContextCancellation verifies context cancellation stops workflow
// Expected failure: Pipeline.Plan() does not respect context cancellation
func TestPlanWorkflow_RespectsContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create spec
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock agent that runs for a while
	mockAgent := &planAcceptanceAgent{
		name: "slow-agent",
		launchFunc: func(promptPath string) error {
			time.Sleep(5 * time.Second) // Long operation
			return nil
		},
	}

	deps := &Deps{
		AgentResolver: &planAcceptanceAgentResolver{
			resolveFunc: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return mockAgent, nil
			},
		},
		BeadClient: &planAcceptanceBeadClient{},
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	input := PlanInput{
		SpecName: "test-spec",
	}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// Cancel immediately
	cancel()

	// Wait should return quickly
	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-done:
		// Success - cancellation stopped the session
	case <-time.After(2 * time.Second):
		t.Fatal("Context cancellation did not stop session within 2 seconds")
	}
}

// TestPlanWorkflow_AgentNameOverride verifies AgentName input is used
// Expected failure: Pipeline.Plan() does not pass AgentName to resolver
func TestPlanWorkflow_AgentNameOverride(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create spec
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	var resolvedWithOverride string
	mockAgent := &planAcceptanceAgent{name: "opus-agent"}
	mockResolver := &planAcceptanceAgentResolver{
		resolveFunc: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			resolvedWithOverride = flagOverride
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockResolver,
		BeadClient:    &planAcceptanceBeadClient{},
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{
		SpecName:  "test-spec",
		AgentName: "opus-agent", // Override
	}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	_ = session.Wait()

	// Verify override was passed to resolver
	if resolvedWithOverride != "opus-agent" {
		t.Errorf("AgentResolver received flagOverride = %q, want %q", resolvedWithOverride, "opus-agent")
	}
}

// TestPlanWorkflow_SpecNotFoundError verifies error when spec doesn't exist
// Expected failure: Pipeline.Plan() does not validate spec file existence
func TestPlanWorkflow_SpecNotFoundError(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Don't create the spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	deps := &Deps{
		AgentResolver: &planAcceptanceAgentResolver{
			resolveFunc: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return &planAcceptanceAgent{name: "test"}, nil
			},
		},
		BeadClient: &planAcceptanceBeadClient{},
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{
		SpecName: "nonexistent-spec",
	}

	_, err := p.Plan(ctx, input)
	if err == nil {
		t.Fatal("Plan() should fail for nonexistent spec, got nil error")
	}

	if !strings.Contains(err.Error(), "spec not found") && !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'spec not found' error, got: %v", err)
	}
}

// TestPlanWorkflow_SessionReturnsResult verifies PlanSession.Result() returns PlanResult
// Expected failure: PlanSession.Result() method does not exist yet
func TestPlanWorkflow_SessionReturnsResult(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create spec
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "test-spec.md")
	mockAgent := &planAcceptanceAgent{
		name: "test-agent",
		launchFunc: func(promptPath string) error {
			os.MkdirAll(plansDir, 0755)
			return os.WriteFile(planPath, []byte("# Test Plan"), 0644)
		},
	}

	deps := &Deps{
		AgentResolver: &planAcceptanceAgentResolver{
			resolveFunc: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return mockAgent, nil
			},
		},
		BeadClient: &planAcceptanceBeadClient{},
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{
		SpecName: "test-spec",
	}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("session.Wait() failed: %v", err)
	}

	// Get result
	result, err := session.Result()
	if err != nil {
		t.Fatalf("session.Result() failed: %v", err)
	}

	if result == nil {
		t.Fatal("session.Result() returned nil, want non-nil result")
	}

	// Verify result type
	var _ *PlanResult = result

	// Verify result contains created plans
	if len(result.CreatedPlans) == 0 {
		t.Error("Result.CreatedPlans is empty, expected at least one plan")
	}
}

// TestPlanWorkflow_PlanSessionImplementsSession verifies PlanSession implements Session interface
// Expected failure: PlanSession does not properly implement Session interface
func TestPlanWorkflow_PlanSessionImplementsSession(t *testing.T) {
	// Type assertion - will fail at compile time if PlanSession doesn't implement Session
	var _ Session = (*PlanSession)(nil)
}

// Mock types for acceptance tests - use unique names to avoid conflicts

type planAcceptanceAgent struct {
	name       string
	launchFunc func(promptPath string) error
}

func (m *planAcceptanceAgent) Name() string {
	return m.name
}

func (m *planAcceptanceAgent) Launch(promptPath string) error {
	if m.launchFunc != nil {
		return m.launchFunc(promptPath)
	}
	return nil
}

type planAcceptanceAgentResolver struct {
	resolveFunc func(phase, flagOverride string, choosePicker bool) (Agent, error)
}

func (m *planAcceptanceAgentResolver) Resolve(phase, flagOverride string, choosePicker bool) (Agent, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(phase, flagOverride, choosePicker)
	}
	return nil, fmt.Errorf("not implemented")
}

type planAcceptanceBeadInfo struct {
	ID          string
	Title       string
	Priority    int
	Description string
}

type planAcceptanceBeadClient struct {
	beads []planAcceptanceBeadInfo
}

func (m *planAcceptanceBeadClient) Ready() (interface{}, error) {
	return m.beads, nil
}

func (m *planAcceptanceBeadClient) Show(id string) (interface{}, error) {
	for _, b := range m.beads {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, fmt.Errorf("bead not found")
}

func (m *planAcceptanceBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	return nil, nil
}

func (m *planAcceptanceBeadClient) Close(id string) error {
	return nil
}
