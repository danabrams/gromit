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

// TestPlanWorkflowCreatesPlans verifies Plan workflow creates plan files
// Expected failure: Pipeline.Plan() does not implement plan creation yet
func TestPlanWorkflowCreatesPlans(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a spec to plan
	specPath := filepath.Join(specsDir, "feature.md")
	if err := os.WriteFile(specPath, []byte("# Feature Spec\nThis is a feature spec."), 0644); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			// Agent creates plan file during session
			planPath := filepath.Join(plansDir, "feature.md")
			content := "# Feature Plan\nSteps to implement..."
			return &fileCreatingAgent{files: map[string]string{planPath: content}}, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
	}
	paths := &Paths{
		SpecsDir: specsDir,
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := PlanInput{
		SpecName: "feature",
	}

	session, err := p.Plan(ctx, input)
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("Session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("Result() failed: %v", err)
	}

	// Verify plan was created
	if len(result.CreatedPlans) != 1 {
		t.Errorf("CreatedPlans count = %d, want 1", len(result.CreatedPlans))
	}

	// Verify plan file exists
	planPath := filepath.Join(plansDir, "feature.md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		t.Error("Plan file was not created")
	}
}

// TestDecomposeWorkflowCreatesBeads verifies Decompose workflow creates beads from plan
// Expected failure: Pipeline.Decompose() does not create beads yet
func TestDecomposeWorkflowCreatesBeads(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a plan to decompose
	planPath := filepath.Join(plansDir, "feature.md")
	planContent := `---
id: feature
created: 2026-02-12
---

# Feature Plan

1. Create interface
2. Implement core logic
3. Add tests
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	var createdBeads []interface{}
	mockBead := &mockBeadClient{
		createFunc: func(title string, priority int, labels []string, outputs []string) (interface{}, error) {
			bead := map[string]interface{}{
				"id":       fmt.Sprintf("bead-%d", len(createdBeads)+1),
				"title":    title,
				"priority": priority,
				"labels":   labels,
			}
			createdBeads = append(createdBeads, bead)
			return bead, nil
		},
	}

	// Mock Claude returns JSON with bead definitions
	mockClaude := &mockClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"beads": []interface{}{
					map[string]interface{}{
						"title":               "Create interface",
						"description":         "Define the interface",
						"priority":            "P1",
						"acceptance_criteria": []string{"Interface compiles"},
						"depends_on_index":    []int{},
					},
					map[string]interface{}{
						"title":               "Implement core logic",
						"description":         "Write the implementation",
						"priority":            "P1",
						"acceptance_criteria": []string{"Tests pass"},
						"depends_on_index":    []int{0},
					},
				},
			}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "feature",
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// Verify beads were created
	if len(result.CreatedBeads) != 2 {
		t.Errorf("CreatedBeads count = %d, want 2", len(result.CreatedBeads))
	}

	// Verify beads have spec label
	for _, bead := range result.CreatedBeads {
		hasSpecLabel := false
		for _, label := range bead.Labels {
			if strings.HasPrefix(label, "spec:") {
				hasSpecLabel = true
				break
			}
		}
		if !hasSpecLabel {
			t.Errorf("Bead %q missing spec: label", bead.Title)
		}
	}
}

// TestDecomposeWorkflowUpdatesPlanFrontmatter verifies plan frontmatter is updated after decomposition
// Expected failure: Pipeline.Decompose() does not update plan frontmatter yet
func TestDecomposeWorkflowUpdatesPlanFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "feature.md")
	planContent := `---
id: feature
created: 2026-02-12
---

# Feature Plan
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockBead := &mockBeadClient{
		createFunc: func(title string, priority int, labels []string, outputs []string) (interface{}, error) {
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	mockClaude := &mockClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"beads": []interface{}{
					map[string]interface{}{
						"title":               "Task 1",
						"priority":            "P1",
						"acceptance_criteria": []string{"Done"},
					},
				},
			}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "feature",
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// Verify PlanUpdated flag is set
	if !result.PlanUpdated {
		t.Error("PlanUpdated = false, want true after decomposition")
	}

	// Verify plan file was updated with decomposed: true
	updatedContent, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(updatedContent)
	if !strings.Contains(contentStr, "decomposed: true") {
		t.Error("Plan frontmatter does not contain 'decomposed: true'")
	}
	if !strings.Contains(contentStr, "decomposed_at:") {
		t.Error("Plan frontmatter does not contain 'decomposed_at' timestamp")
	}
}

// TestReviewWorkflowInteractiveMode verifies Review workflow in interactive mode
// Expected failure: Pipeline.Review() does not support interactive mode yet
func TestReviewWorkflowInteractiveMode(t *testing.T) {
	tmpDir := t.TempDir()

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return &simpleAgent{}, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
	}
	paths := &Paths{
		GromitDir: tmpDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{
		Since: "HEAD~5",
	}

	session, err := p.Review(ctx, input)
	if err != nil {
		t.Fatalf("Review() failed: %v", err)
	}

	// Interactive mode returns a session
	if session == nil {
		t.Fatal("Review() returned nil session for interactive mode")
	}

	_ = session.Wait()
}

// TestReviewWorkflowNonInteractiveMode verifies Review workflow runs autonomously
// Expected failure: Pipeline.Review() does not support non-interactive mode yet
func TestReviewWorkflowNonInteractiveMode(t *testing.T) {
	tmpDir := t.TempDir()

	var claudeRunCalled bool
	mockClaude := &mockClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			claudeRunCalled = true
			return map[string]interface{}{
				"learnings": []string{"Learning 1"},
				"beads":     []string{},
			}, nil
		},
	}

	mockLearnings := &mockLearningsManager{
		addFunc: func(content string) error {
			return nil
		},
	}

	deps := &Deps{
		ClaudeClient:     mockClaude,
		LearningsManager: mockLearnings,
	}
	paths := &Paths{
		GromitDir: tmpDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{
		Since: "HEAD~5",
		// NonInteractive flag should be part of input
	}

	// For non-interactive mode, Review should return a result directly
	// This test expects a separate method or the same method to detect non-interactive mode
	// from the input struct. Adjust based on actual API design.

	// Assuming the API will have a way to specify non-interactive mode via input
	// For now, this tests that non-interactive path exists
	session, err := p.Review(ctx, input)
	if err != nil {
		t.Fatalf("Review() failed: %v", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	// In non-interactive mode, Claude.Run() should be called
	if !claudeRunCalled {
		t.Error("Claude.Run() was not called in non-interactive review")
	}
}

// TestExploreWorkflowCreatesArtifacts verifies Explore workflow detects created artifacts
// Expected failure: Pipeline.Explore() does not detect artifacts yet
func TestExploreWorkflowCreatesArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	epicsDir := filepath.Join(tmpDir, "epics")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			// Agent creates spec and epic during exploration
			return &fileCreatingAgent{
				files: map[string]string{
					filepath.Join(specsDir, "new-feature.md"): "# New Feature",
					filepath.Join(epicsDir, "epic-1.md"):      "# Epic 1",
				},
			}, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := ExploreInput{
		Topic: "new feature ideas",
	}

	session, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("Session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("Result() failed: %v", err)
	}

	// Verify specs were detected
	if len(result.CreatedSpecs) != 1 {
		t.Errorf("CreatedSpecs count = %d, want 1", len(result.CreatedSpecs))
	}

	// Verify epics were detected
	if len(result.CreatedEpics) != 1 {
		t.Errorf("CreatedEpics count = %d, want 1", len(result.CreatedEpics))
	}
}

// TestExploreWorkflowUsesAgentAbstraction verifies Explore uses agent.Resolve() instead of exec.Command
// Expected failure: Pipeline.Explore() does not use agent abstraction yet
func TestExploreWorkflowUsesAgentAbstraction(t *testing.T) {
	tmpDir := t.TempDir()

	var agentResolved bool
	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			agentResolved = true
			// Verify phase is "explore"
			if phase != "explore" {
				t.Errorf("Resolve phase = %q, want %q", phase, "explore")
			}
			return &simpleAgent{}, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
	}
	paths := &Paths{
		GromitDir: tmpDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := ExploreInput{
		Topic: "test topic",
	}

	_, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	// Verify agent was resolved through abstraction (not direct exec.Command)
	if !agentResolved {
		t.Error("AgentResolver.Resolve() was not called, Explore should use agent abstraction")
	}
}

// TestDecomposeWorkflowReviewMode verifies review mode returns beads for approval
// Expected failure: Pipeline.Decompose() does not implement review mode yet
func TestDecomposeWorkflowReviewMode(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "feature.md")
	if err := os.WriteFile(planPath, []byte("# Feature"), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &mockClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"beads": []interface{}{
					map[string]interface{}{
						"title":               "Task 1",
						"priority":            "P1",
						"acceptance_criteria": []string{"Done"},
					},
				},
			}, nil
		},
	}

	// In review mode, beads should NOT be created
	var createCalled bool
	mockBead := &mockBeadClient{
		createFunc: func(title string, priority int, labels []string, outputs []string) (interface{}, error) {
			createCalled = true
			return nil, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "feature",
		Review:   true, // Review mode
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	// In review mode, beads are proposed but not created
	if len(result.CreatedBeads) == 0 {
		t.Error("CreatedBeads empty in review mode, should contain proposed beads for review")
	}

	// BeadClient.Create() should NOT be called in review mode
	if createCalled {
		t.Error("BeadClient.Create() was called in review mode, should only propose beads")
	}
}

// TestDecomposeWorkflowForceRedecompose verifies Force flag allows re-decomposition
// Expected failure: Pipeline.Decompose() does not implement Force flag yet
func TestDecomposeWorkflowForceRedecompose(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create plan that's already decomposed
	planPath := filepath.Join(plansDir, "feature.md")
	planContent := `---
id: feature
decomposed: true
decomposed_at: 2026-02-11T10:00:00Z
---

# Feature
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &mockClaudeClient{
		runFunc: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"beads": []interface{}{},
			}, nil
		},
	}

	mockBead := &mockBeadClient{
		createFunc: func(title string, priority int, labels []string, outputs []string) (interface{}, error) {
			return map[string]interface{}{"id": "bead-1"}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()

	// Without Force flag, should fail for already-decomposed plan
	input := DecomposeInput{
		PlanName: "feature",
		Force:    false,
	}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Error("Decompose() without Force on already-decomposed plan returned nil error, want error")
	}

	// With Force flag, should succeed
	inputForced := DecomposeInput{
		PlanName: "feature",
		Force:    true,
	}

	_, err = p.Decompose(ctx, inputForced)
	if err != nil {
		t.Errorf("Decompose() with Force flag failed: %v, want success", err)
	}
}

// Mock types

type mockBeadClient struct {
	createFunc func(title string, priority int, labels []string, outputs []string) (interface{}, error)
}

func (m *mockBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	if m.createFunc != nil {
		return m.createFunc(title, priority, labels, outputs)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBeadClient) Ready() (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBeadClient) Show(id string) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBeadClient) Close(id string) error {
	return fmt.Errorf("not implemented")
}

type mockClaudeClient struct {
	runFunc func(prompt string, model string) (interface{}, error)
}

func (m *mockClaudeClient) Run(prompt string, model string) (interface{}, error) {
	if m.runFunc != nil {
		return m.runFunc(prompt, model)
	}
	return nil, fmt.Errorf("not implemented")
}

type mockLearningsManager struct {
	addFunc func(content string) error
}

func (m *mockLearningsManager) Add(content string) error {
	if m.addFunc != nil {
		return m.addFunc(content)
	}
	return fmt.Errorf("not implemented")
}

type fileCreatingAgent struct {
	files map[string]string // path -> content
}

type simpleAgent struct{}
