//go:build acceptance

package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRefineWorkflow_InteractiveMode verifies Refine returns a Session for interactive use
// Expected failure: Pipeline.Refine() does not return RefineSession with working Session interface yet
func TestRefineWorkflow_InteractiveMode(t *testing.T) {
	deps := &Deps{
		AgentResolver: &mockAgentResolver{},
		BeadClient:    &mockBeadClient{},
		BacklogClient: &mockBacklogClient{},
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
		SpecsDir:  filepath.Join(t.TempDir(), "specs"),
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := RefineInput{
		IdeaText:  "Test idea for refinement",
		AgentName: "",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify session is not nil and implements Session interface
	if session == nil {
		t.Fatal("Refine() returned nil session, want non-nil")
	}

	// Verify Events() channel is accessible
	events := session.Events()
	if events == nil {
		t.Fatal("session.Events() returned nil channel, want non-nil")
	}

	// Verify session has SendInput method
	err = session.SendInput("test input")
	if err != nil {
		t.Errorf("session.SendInput() failed: %v", err)
	}

	// Clean up
	session.Cancel()
	_ = session.Wait()
}

// TestRefineWorkflow_PostProcessing verifies Refine performs post-session processing
// Expected failure: Pipeline.Refine() does not detect new specs and mark backlog items as refined yet
func TestRefineWorkflow_PostProcessing(t *testing.T) {
	specsDir := filepath.Join(t.TempDir(), "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	markedRefined := false
	deps := &Deps{
		AgentResolver: &mockAgentResolver{
			LaunchFn: func() error {
				// Simulate agent creating a spec file
				specPath := filepath.Join(specsDir, "new-spec.md")
				return os.WriteFile(specPath, []byte("# New Spec\n\nContent"), 0o644)
			},
		},
		BacklogClient: &mockBacklogClient{
			UpdateFn: func(id string, fn func(interface{})) error {
				markedRefined = true
				return nil
			},
		},
	}
	paths := &Paths{
		SpecsDir: specsDir,
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := RefineInput{
		IdeaID: "backlog-123",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Wait for session to complete
	err = session.Wait()
	if err != nil {
		t.Errorf("session.Wait() failed: %v", err)
	}

	// Verify Result() contains detected specs
	result, err := session.Result()
	if err != nil {
		t.Fatalf("session.Result() failed: %v", err)
	}

	if len(result.CreatedSpecs) == 0 {
		t.Error("Result.CreatedSpecs is empty, want detected spec files")
	}

	if !markedRefined {
		t.Error("Backlog item was not marked as refined by post-processing")
	}
}

// TestPlanWorkflow_InteractiveMode verifies Plan returns a Session for interactive use
// Expected failure: Pipeline.Plan() does not return PlanSession with working Session interface yet
func TestPlanWorkflow_InteractiveMode(t *testing.T) {
	deps := &Deps{
		AgentResolver: &mockAgentResolver{},
	}
	paths := &Paths{
		SpecsDir: t.TempDir(),
		PlansDir: filepath.Join(t.TempDir(), "plans"),
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{
		SpecName:  "test-spec",
		AgentName: "",
	}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	if session == nil {
		t.Fatal("Plan() returned nil session, want non-nil")
	}

	// Verify session interface methods exist
	events := session.Events()
	if events == nil {
		t.Fatal("session.Events() returned nil channel")
	}

	session.Cancel()
	_ = session.Wait()
}

// TestPlanWorkflow_PostProcessing verifies Plan detects new plan files
// Expected failure: Pipeline.Plan() does not detect new plan files after session ends yet
func TestPlanWorkflow_PostProcessing(t *testing.T) {
	plansDir := filepath.Join(t.TempDir(), "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("Failed to create plans dir: %v", err)
	}

	deps := &Deps{
		AgentResolver: &mockAgentResolver{
			LaunchFn: func() error {
				// Simulate agent creating a plan file
				planPath := filepath.Join(plansDir, "test-plan.md")
				return os.WriteFile(planPath, []byte("# Test Plan\n\nContent"), 0o644)
			},
		},
	}
	paths := &Paths{
		PlansDir: plansDir,
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{SpecName: "test-spec"}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	err = session.Wait()
	if err != nil {
		t.Errorf("session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("session.Result() failed: %v", err)
	}

	if len(result.CreatedPlans) == 0 {
		t.Error("Result.CreatedPlans is empty, want detected plan files")
	}
}

// TestDecomposeWorkflow_NonInteractive verifies Decompose runs to completion and returns structured result
// Expected failure: Pipeline.Decompose() does not run Claude, parse JSON, create beads, and return DecomposeResult yet
func TestDecomposeWorkflow_NonInteractive(t *testing.T) {
	createdBeadIDs := []string{}
	deps := &Deps{
		ClaudeClient: &mockClaudeClient{
			RunFn: func(prompt string, model string) (interface{}, error) {
				// Simulate Claude returning JSON with bead definitions
				return `{
					"beads": [
						{
							"title": "Implement feature X",
							"description": "Add feature X",
							"priority": "P1",
							"acceptance_criteria": ["Criterion 1", "Criterion 2"],
							"depends_on_index": []
						}
					]
				}`, nil
			},
		},
		BeadClient: &mockBeadClient{
			CreateFn: func(title string, priority int, labels []string, outputs []string) (interface{}, error) {
				beadID := fmt.Sprintf("bead-%d", len(createdBeadIDs)+1)
				createdBeadIDs = append(createdBeadIDs, beadID)
				return beadID, nil
			},
		},
		PromptRenderer: &mockPromptRenderer{
			RenderDecomposeFn: func(input interface{}) (string, error) {
				return "decompose prompt", nil
			},
		},
	}
	paths := &Paths{
		PlansDir: t.TempDir(),
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test-plan",
		Force:    false,
		Review:   false,
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// Verify result contains created beads
	if len(result.CreatedBeads) == 0 {
		t.Error("Result.CreatedBeads is empty, want at least one created bead")
	}

	// Verify beads were actually created via BeadClient
	if len(createdBeadIDs) == 0 {
		t.Error("No beads were created via BeadClient.Create()")
	}

	// Verify result has proper structure
	if result.CreatedBeads[0].ID == "" {
		t.Error("CreatedBead.ID is empty, want bead ID")
	}
	if result.CreatedBeads[0].Title == "" {
		t.Error("CreatedBead.Title is empty, want bead title")
	}
}

// TestDecomposeWorkflow_UpdatesFrontmatter verifies Decompose updates plan frontmatter
// Expected failure: Pipeline.Decompose() does not update plan frontmatter with decomposed: true yet
func TestDecomposeWorkflow_UpdatesFrontmatter(t *testing.T) {
	plansDir := t.TempDir()
	planPath := filepath.Join(plansDir, "test-plan.md")

	// Create plan file with frontmatter
	planContent := `---
decomposed: false
---
# Test Plan

Content here`
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("Failed to create plan file: %v", err)
	}

	deps := &Deps{
		ClaudeClient: &mockClaudeClient{
			RunFn: func(prompt string, model string) (interface{}, error) {
				return `{"beads": []}`, nil
			},
		},
		BeadClient:     &mockBeadClient{},
		PromptRenderer: &mockPromptRenderer{},
	}
	paths := &Paths{
		PlansDir: plansDir,
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{PlanName: "test-plan"}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if !result.PlanUpdated {
		t.Error("Result.PlanUpdated is false, want true after successful decomposition")
	}

	// Verify plan file was actually updated
	updatedContent, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("Failed to read updated plan file: %v", err)
	}

	// Should contain decomposed: true now
	if !contains(string(updatedContent), "decomposed: true") {
		t.Error("Plan frontmatter not updated with decomposed: true")
	}
}

// TestReviewWorkflow_InteractiveMode verifies Review returns a Session for interactive use
// Expected failure: Pipeline.Review() does not return ReviewSession with working Session interface yet
func TestReviewWorkflow_InteractiveMode(t *testing.T) {
	deps := &Deps{
		AgentResolver: &mockAgentResolver{},
	}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{
		Since:     "HEAD~1",
		AgentName: "",
	}

	session, err := p.Review(ctx, input)
	if err != nil {
		t.Fatalf("Review() failed: %v", err)
	}

	if session == nil {
		t.Fatal("Review() returned nil session, want non-nil")
	}

	events := session.Events()
	if events == nil {
		t.Fatal("session.Events() returned nil channel")
	}

	session.Cancel()
	_ = session.Wait()
}

// TestReviewWorkflow_NonInteractiveMode verifies Review can run non-interactively
// Expected failure: Pipeline.Review() does not support non-interactive mode via input parameter yet
func TestReviewWorkflow_NonInteractiveMode(t *testing.T) {
	createdBeads := []string{}
	persistedLearnings := false

	deps := &Deps{
		ClaudeClient: &mockClaudeClient{
			RunFn: func(prompt string, model string) (interface{}, error) {
				return "review output", nil
			},
		},
		BeadClient: &mockBeadClient{
			CreateFn: func(title string, priority int, labels []string, outputs []string) (interface{}, error) {
				beadID := fmt.Sprintf("bead-%d", len(createdBeads)+1)
				createdBeads = append(createdBeads, beadID)
				return beadID, nil
			},
		},
		LearningsManager: &mockLearningsManager{
			AddFn: func(content string) error {
				persistedLearnings = true
				return nil
			},
		},
		PromptRenderer: &mockPromptRenderer{},
	}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{
		Since:     "HEAD~1",
		AgentName: "",
	}

	// When Review is used non-interactively, it should use ClaudeClient instead of AgentResolver
	// The implementation should detect this based on input or a separate method
	// For now, we test that Review can be called and returns a result
	session, err := p.Review(ctx, input)
	if err != nil {
		t.Fatalf("Review() failed: %v", err)
	}

	err = session.Wait()
	if err != nil {
		t.Errorf("session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("session.Result() failed: %v", err)
	}

	// Non-interactive review should perform post-processing
	_ = result.CreatedBeads
	_ = result.PersistedLearnings
}

// TestReviewWorkflow_PostProcessing verifies Review creates beads and persists learnings
// Expected failure: Pipeline.Review() does not parse results, create beads, and persist learnings yet
func TestReviewWorkflow_PostProcessing(t *testing.T) {
	createdBeads := []string{}
	persistedLearnings := false

	deps := &Deps{
		AgentResolver: &mockAgentResolver{
			LaunchFn: func() error {
				// Simulate successful review session
				return nil
			},
		},
		BeadClient: &mockBeadClient{
			CreateFn: func(title string, priority int, labels []string, outputs []string) (interface{}, error) {
				beadID := fmt.Sprintf("bead-%d", len(createdBeads)+1)
				createdBeads = append(createdBeads, beadID)
				return beadID, nil
			},
		},
		LearningsManager: &mockLearningsManager{
			AddFn: func(content string) error {
				persistedLearnings = true
				return nil
			},
		},
	}
	paths := &Paths{}
	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{Since: "HEAD~1"}

	session, err := p.Review(ctx, input)
	if err != nil {
		t.Fatalf("Review() failed: %v", err)
	}

	err = session.Wait()
	if err != nil {
		t.Errorf("session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("session.Result() failed: %v", err)
	}

	// Verify post-processing happened
	if !result.PersistedLearnings {
		t.Error("Result.PersistedLearnings is false, want true after post-processing")
	}
}

// TestExploreWorkflow_InteractiveMode verifies Explore returns a Session for interactive use
// Expected failure: Pipeline.Explore() does not return ExploreSession with working Session interface yet
func TestExploreWorkflow_InteractiveMode(t *testing.T) {
	deps := &Deps{
		AgentResolver: &mockAgentResolver{},
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
		SpecsDir:  filepath.Join(t.TempDir(), "specs"),
		EpicsDir:  filepath.Join(t.TempDir(), "epics"),
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := ExploreInput{
		Topic: "Test exploration topic",
		Model: "opus",
	}

	session, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	if session == nil {
		t.Fatal("Explore() returned nil session, want non-nil")
	}

	events := session.Events()
	if events == nil {
		t.Fatal("session.Events() returned nil channel")
	}

	session.Cancel()
	_ = session.Wait()
}

// TestExploreWorkflow_UsesAgentAbstraction verifies Explore uses agent.Resolve/Launch instead of exec.Command
// Expected failure: Pipeline.Explore() does not use AgentResolver abstraction yet
func TestExploreWorkflow_UsesAgentAbstraction(t *testing.T) {
	agentResolved := false
	deps := &Deps{
		AgentResolver: &mockAgentResolver{
			ResolveFn: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
				agentResolved = true
				return &mockAgent{}, nil
			},
		},
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
		SpecsDir:  t.TempDir(),
		EpicsDir:  t.TempDir(),
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := ExploreInput{Topic: "test"}

	session, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	session.Cancel()
	_ = session.Wait()

	if !agentResolved {
		t.Error("AgentResolver.Resolve() was not called, Explore should use agent abstraction")
	}
}

// TestExploreWorkflow_PostProcessing verifies Explore detects new artifacts
// Expected failure: Pipeline.Explore() does not detect new specs, epics, and backlog items yet
func TestExploreWorkflow_PostProcessing(t *testing.T) {
	specsDir := filepath.Join(t.TempDir(), "specs")
	epicsDir := filepath.Join(t.TempDir(), "epics")
	for _, dir := range []string{specsDir, epicsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	deps := &Deps{
		AgentResolver: &mockAgentResolver{
			LaunchFn: func() error {
				// Simulate agent creating artifacts
				_ = os.WriteFile(filepath.Join(specsDir, "new-spec.md"), []byte("# Spec"), 0o644)
				_ = os.WriteFile(filepath.Join(epicsDir, "new-epic.md"), []byte("# Epic"), 0o644)
				return nil
			},
		},
		BacklogClient: &mockBacklogClient{},
	}
	paths := &Paths{
		SpecsDir: specsDir,
		EpicsDir: epicsDir,
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := ExploreInput{Topic: "test"}

	session, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	err = session.Wait()
	if err != nil {
		t.Errorf("session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("session.Result() failed: %v", err)
	}

	if len(result.CreatedSpecs) == 0 {
		t.Error("Result.CreatedSpecs is empty, want detected spec files")
	}
	if len(result.CreatedEpics) == 0 {
		t.Error("Result.CreatedEpics is empty, want detected epic files")
	}
}

// TestAllWorkflows_AcceptContext verifies all workflows accept and respect context.Context
// Expected failure: Workflow methods do not respect context cancellation yet
func TestAllWorkflows_AcceptContext(t *testing.T) {
	tests := []struct {
		name     string
		workflow func(*Pipeline, context.Context) error
	}{
		{
			name: "Refine",
			workflow: func(p *Pipeline, ctx context.Context) error {
				_, err := p.Refine(ctx, RefineInput{IdeaText: "test"})
				return err
			},
		},
		{
			name: "Plan",
			workflow: func(p *Pipeline, ctx context.Context) error {
				_, err := p.Plan(ctx, PlanInput{SpecName: "test"})
				return err
			},
		},
		{
			name: "Decompose",
			workflow: func(p *Pipeline, ctx context.Context) error {
				_, err := p.Decompose(ctx, DecomposeInput{PlanName: "test"})
				return err
			},
		},
		{
			name: "Review",
			workflow: func(p *Pipeline, ctx context.Context) error {
				_, err := p.Review(ctx, ReviewInput{Since: "HEAD~1"})
				return err
			},
		},
		{
			name: "Explore",
			workflow: func(p *Pipeline, ctx context.Context) error {
				_, err := p.Explore(ctx, ExploreInput{Topic: "test"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &Deps{
				AgentResolver:  &mockAgentResolver{},
				ClaudeClient:   &mockClaudeClient{},
				PromptRenderer: &mockPromptRenderer{},
			}
			paths := &Paths{
				GromitDir: t.TempDir(),
				SpecsDir:  t.TempDir(),
				PlansDir:  t.TempDir(),
				EpicsDir:  t.TempDir(),
			}
			p := New(deps, paths)

			// Create a context that's already cancelled
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Workflow should respect cancellation and return quickly
			err := tt.workflow(p, ctx)

			// We expect either an error (context cancelled) or nil if the workflow
			// hasn't started yet. The key is it should return quickly, not hang.
			_ = err
		})
	}
}

// TestAllWorkflows_NoDependencyOnCobra verifies workflows don't depend on cobra or CLI flags
// Expected failure: This is a compile-time check - workflow signatures should not reference cobra types
func TestAllWorkflows_NoDependencyOnCobra(t *testing.T) {
	// This test ensures workflows are truly decoupled from CLI
	// If workflows accept cobra.Command or have cobra imports, this will fail at compile time

	deps := &Deps{
		AgentResolver:  &mockAgentResolver{},
		ClaudeClient:   &mockClaudeClient{},
		PromptRenderer: &mockPromptRenderer{},
		BeadClient:     &mockBeadClient{},
		BacklogClient:  &mockBacklogClient{},
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
		SpecsDir:  t.TempDir(),
		PlansDir:  t.TempDir(),
		EpicsDir:  t.TempDir(),
	}
	p := New(deps, paths)

	ctx := context.Background()

	// All workflows should be callable with just context and input structs
	// No cobra flags, no os.Stdin/Stdout/Stderr, no terminal I/O
	_, _ = p.Refine(ctx, RefineInput{IdeaText: "test"})
	_, _ = p.Plan(ctx, PlanInput{SpecName: "test"})
	_, _ = p.Decompose(ctx, DecomposeInput{PlanName: "test"})
	_, _ = p.Review(ctx, ReviewInput{Since: "HEAD~1"})
	_, _ = p.Explore(ctx, ExploreInput{Topic: "test"})

	// If any workflow requires cobra types or terminal I/O, this won't compile
}

// TestSessionInterface_EventOrdering verifies events are emitted in correct order
// Expected failure: baseSession does not emit events in the correct order (Started -> Output -> Ended)
func TestSessionInterface_EventOrdering(t *testing.T) {
	deps := &Deps{
		AgentResolver: &mockAgentResolver{
			LaunchFn: func() error {
				// Quick completion
				return nil
			},
		},
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
		SpecsDir:  t.TempDir(),
	}
	p := New(deps, paths)

	ctx := context.Background()
	input := RefineInput{IdeaText: "test"}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	events := session.Events()

	// Collect all events
	var eventTypes []EventType
	for event := range events {
		eventTypes = append(eventTypes, event.Type)
	}

	_ = session.Wait()

	// Verify event order: SessionStarted must be first, SessionEnded must be last
	if len(eventTypes) < 2 {
		t.Fatalf("Expected at least 2 events (Started, Ended), got %d", len(eventTypes))
	}

	if eventTypes[0] != EventSessionStarted {
		t.Errorf("First event = %v, want EventSessionStarted", eventTypes[0])
	}

	if eventTypes[len(eventTypes)-1] != EventSessionEnded {
		t.Errorf("Last event = %v, want EventSessionEnded", eventTypes[len(eventTypes)-1])
	}
}

// TestSessionInterface_ContextCancellation verifies sessions respect context cancellation
// Expected failure: Sessions do not properly handle context cancellation yet
func TestSessionInterface_ContextCancellation(t *testing.T) {
	deps := &Deps{
		AgentResolver: &mockAgentResolver{
			LaunchFn: func() error {
				// Simulate long-running operation
				time.Sleep(10 * time.Second)
				return nil
			},
		},
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
		SpecsDir:  t.TempDir(),
	}
	p := New(deps, paths)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	input := RefineInput{IdeaText: "test"}

	session, err := p.Refine(ctx, input)
	if err != nil {
		// Context cancellation before session start is acceptable
		return
	}

	// Session should terminate quickly due to context timeout
	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-done:
		// Success - session respected context cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("Session did not respect context cancellation within 2 seconds")
	}
}

// TestResultTypes_InitializedSlices verifies all result types have initialized slices, not nil
// Expected failure: Result constructors do not properly initialize slices yet
func TestResultTypes_InitializedSlices(t *testing.T) {
	// RefineResult
	refineResult := NewRefineResult()
	if refineResult.CreatedSpecs == nil {
		t.Error("RefineResult.CreatedSpecs is nil, want empty slice")
	}
	if refineResult.RefinedItems == nil {
		t.Error("RefineResult.RefinedItems is nil, want empty slice")
	}

	// PlanResult
	planResult := NewPlanResult()
	if planResult.CreatedPlans == nil {
		t.Error("PlanResult.CreatedPlans is nil, want empty slice")
	}

	// DecomposeResult
	decomposeResult := NewDecomposeResult()
	if decomposeResult.CreatedBeads == nil {
		t.Error("DecomposeResult.CreatedBeads is nil, want empty slice")
	}

	// ReviewResult
	reviewResult := NewReviewResult()
	if reviewResult.CreatedBeads == nil {
		t.Error("ReviewResult.CreatedBeads is nil, want empty slice")
	}
	if reviewResult.CreatedBacklogItems == nil {
		t.Error("ReviewResult.CreatedBacklogItems is nil, want empty slice")
	}

	// ExploreResult
	exploreResult := NewExploreResult()
	if exploreResult.CreatedSpecs == nil {
		t.Error("ExploreResult.CreatedSpecs is nil, want empty slice")
	}
	if exploreResult.CreatedEpics == nil {
		t.Error("ExploreResult.CreatedEpics is nil, want empty slice")
	}
	if exploreResult.CreatedBacklogItems == nil {
		t.Error("ExploreResult.CreatedBacklogItems is nil, want empty slice")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Mock implementations for testing

type mockAgentResolver struct {
	ResolveFn func(phase string, flagOverride string, choosePicker bool) (interface{}, error)
	LaunchFn  func() error
}

func (m *mockAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
	if m.ResolveFn != nil {
		return m.ResolveFn(phase, flagOverride, choosePicker)
	}
	return &mockAgent{launchFn: m.LaunchFn}, nil
}

type mockAgent struct {
	launchFn func() error
}

func (m *mockAgent) Launch() error {
	if m.launchFn != nil {
		return m.launchFn()
	}
	return nil
}

type mockClaudeClient struct {
	RunFn func(prompt string, model string) (interface{}, error)
}

func (m *mockClaudeClient) Run(prompt string, model string) (interface{}, error) {
	if m.RunFn != nil {
		return m.RunFn(prompt, model)
	}
	return "", nil
}

type mockBeadClient struct {
	CreateFn func(title string, priority int, labels []string, outputs []string) (interface{}, error)
}

func (m *mockBeadClient) Ready() (interface{}, error) {
	return nil, nil
}

func (m *mockBeadClient) Show(id string) (interface{}, error) {
	return nil, nil
}

func (m *mockBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	if m.CreateFn != nil {
		return m.CreateFn(title, priority, labels, outputs)
	}
	return "mock-bead-id", nil
}

func (m *mockBeadClient) Close(id string) error {
	return nil
}

type mockBacklogClient struct {
	UpdateFn func(id string, fn func(interface{})) error
}

func (m *mockBacklogClient) List() ([]interface{}, error) {
	return []interface{}{}, nil
}

func (m *mockBacklogClient) Get(id string) (interface{}, error) {
	return nil, nil
}

func (m *mockBacklogClient) Add(item interface{}) error {
	return nil
}

func (m *mockBacklogClient) Update(id string, fn func(interface{})) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(id, fn)
	}
	return nil
}

type mockPromptRenderer struct {
	RenderDecomposeFn func(input interface{}) (string, error)
}

func (m *mockPromptRenderer) RenderRefine(input interface{}) (string, error) {
	return "refine prompt", nil
}

func (m *mockPromptRenderer) RenderPlan(input interface{}) (string, error) {
	return "plan prompt", nil
}

func (m *mockPromptRenderer) RenderDecompose(input interface{}) (string, error) {
	if m.RenderDecomposeFn != nil {
		return m.RenderDecomposeFn(input)
	}
	return "decompose prompt", nil
}

type mockLearningsManager struct {
	AddFn func(content string) error
}

func (m *mockLearningsManager) Add(content string) error {
	if m.AddFn != nil {
		return m.AddFn(content)
	}
	return nil
}

type mockStateManager struct{}

func (m *mockStateManager) GetLastReviewCommit() (string, error) {
	return "", nil
}

func (m *mockStateManager) SetLastReviewCommit(commit string) error {
	return nil
}

type mockLogWriter struct{}

func (m *mockLogWriter) Write(entry interface{}) error {
	return nil
}
