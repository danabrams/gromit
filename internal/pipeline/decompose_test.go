package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDecompose_ValidatesDependencies verifies nil dependencies are rejected.
func TestDecompose_ValidatesDependencies(t *testing.T) {
	p := New(nil, nil)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "test-plan",
	}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Fatal("Decompose() with nil deps should return error")
	}

	expectedMsg := "nil dependencies"
	if err.Error() != fmt.Sprintf("pipeline: %s", expectedMsg) {
		t.Errorf("error = %q, want substring %q", err.Error(), expectedMsg)
	}
}

// TestDecompose_ChecksPlanExists verifies that non-existent plan files return an error.
func TestDecompose_ChecksPlanExists(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeTestClaudeClient{}
	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   &decomposeTestBeadClient{},
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "nonexistent",
	}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Fatal("Decompose() with nonexistent plan should return error")
	}

	// Should mention plan not found
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "no such file") {
		t.Errorf("error = %q, want error about plan not found", err.Error())
	}
}

// TestDecompose_RejectsAlreadyDecomposed verifies that already-decomposed plans are rejected unless Force is true.
func TestDecompose_RejectsAlreadyDecomposed(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a plan file with decomposed:true frontmatter
	planPath := filepath.Join(plansDir, "already-done.md")
	planContent := `---
spec: already-done
decomposed: true
decomposed_at: 2026-02-11T10:00:00Z
---

# Already Done Plan

Some content.
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeTestClaudeClient{}
	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   &decomposeTestBeadClient{},
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "already-done",
		Force:    false,
	}

	_, err := p.Decompose(ctx, input)
	if err == nil {
		t.Fatal("Decompose() with already-decomposed plan should return error when Force=false")
	}

	if !strings.Contains(err.Error(), "already decomposed") {
		t.Errorf("error = %q, want error about already decomposed", err.Error())
	}
}

// TestDecompose_ForceAllowsRedecompose verifies that Force=true bypasses the already-decomposed check.
func TestDecompose_ForceAllowsRedecompose(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a plan file with decomposed:true frontmatter
	planPath := filepath.Join(plansDir, "already-done.md")
	planContent := `---
spec: already-done
decomposed: true
---

# Already Done Plan
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeTestClaudeClient{
		RunFn: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output":   "[]",
			}, nil
		},
	}
	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   &decomposeTestBeadClient{},
	}
	paths := &Paths{
		PlansDir: plansDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := DecomposeInput{
		PlanName: "already-done",
		Force:    true,
	}

	// Should not error when Force=true
	_, err := p.Decompose(ctx, input)
	// We expect this to proceed past the check, so it should reach the "not yet implemented" error
	// or succeed (once we implement the full workflow)
	if err != nil && strings.Contains(err.Error(), "already decomposed") {
		t.Errorf("Decompose() with Force=true should not return 'already decomposed' error, got: %v", err)
	}
}

// TestDecompose_CallsClaudeWithPlanBody verifies that Claude is called with the plan body (not frontmatter).
func TestDecompose_CallsClaudeWithPlanBody(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "test-plan.md")
	planContent := `---
spec: test-plan
created: 2026-02-11
---

# Test Plan

## Phase 1
- Task A
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	var capturedPrompt string
	mockClaude := &decomposeTestClaudeClient{
		RunFn: func(prompt string, model string) (interface{}, error) {
			capturedPrompt = prompt
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Task A",
					"description": "First task",
					"priority": "P1",
					"acceptance_criteria": ["Criterion 1"],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	mockBead := &decomposeTestBeadClient{
		CreateWithDepsAndDescriptionFn: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			return map[string]interface{}{
				"ID":       "bead-1",
				"Title":    title,
				"Priority": priority,
				"Labels":   labels,
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
		PlanName: "test-plan",
	}

	_, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed unexpectedly: %v", err)
	}

	// Verify prompt contains plan body
	if !strings.Contains(capturedPrompt, "# Test Plan") {
		t.Errorf("prompt should contain plan body, got: %s", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "Task A") {
		t.Errorf("prompt should contain plan tasks, got: %s", capturedPrompt)
	}
	// Verify prompt does NOT contain frontmatter
	if strings.Contains(capturedPrompt, "spec: test-plan") {
		t.Errorf("prompt should not contain frontmatter, got: %s", capturedPrompt)
	}
}

// TestDecompose_ReviewModeReturnsProposedBeads verifies that Review mode returns beads without creating them.
func TestDecompose_ReviewModeReturnsProposedBeads(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "review-test.md")
	planContent := `# Review Test Plan

Task 1
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeTestClaudeClient{
		RunFn: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[
					{
						"title": "Implement Task 1",
						"description": "Task 1 implementation",
						"priority": "P1",
						"acceptance_criteria": ["Done"],
						"depends_on_index": []
					}
				]`,
			}, nil
		},
	}

	beadCreationCalled := false
	mockBead := &decomposeTestBeadClient{
		CreateWithDepsAndDescriptionFn: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			beadCreationCalled = true
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
		PlanName: "review-test",
		Review:   true,
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() in Review mode failed: %v", err)
	}

	if result == nil {
		t.Fatal("Decompose() returned nil result")
	}

	// In Review mode, beads should be returned but not created
	if beadCreationCalled {
		t.Error("Review mode should not create beads")
	}

	// Result should indicate proposed beads
	if len(result.CreatedBeads) != 1 {
		t.Errorf("CreatedBeads count = %d, want 1 (proposed but not created)", len(result.CreatedBeads))
	}

	if result.PlanUpdated {
		t.Error("Review mode should not update plan frontmatter")
	}
}

// TestDecompose_CreatesBeadsWithDependencies verifies that beads are created with proper dependency mapping.
func TestDecompose_CreatesBeadsWithDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "deps-test.md")
	planContent := `# Deps Test Plan

Tasks with dependencies
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeTestClaudeClient{
		RunFn: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[
					{
						"title": "Task A",
						"description": "First task",
						"priority": "P1",
						"acceptance_criteria": ["A done"],
						"depends_on_index": []
					},
					{
						"title": "Task B",
						"description": "Second task",
						"priority": "P1",
						"acceptance_criteria": ["B done"],
						"depends_on_index": [0]
					},
					{
						"title": "Task C",
						"description": "Third task",
						"priority": "P2",
						"acceptance_criteria": ["C done"],
						"depends_on_index": [0, 1]
					}
				]`,
			}, nil
		},
	}

	var createdBeads []struct {
		title    string
		priority int
		labels   []string
		criteria []string
		deps     []string
		desc     string
	}

	mockBead := &decomposeTestBeadClient{
		CreateWithDepsAndDescriptionFn: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			id := fmt.Sprintf("bead-%d", len(createdBeads)+1)
			createdBeads = append(createdBeads, struct {
				title    string
				priority int
				labels   []string
				criteria []string
				deps     []string
				desc     string
			}{title, priority, labels, criteria, deps, desc})
			return map[string]interface{}{
				"ID":       id,
				"Title":    title,
				"Priority": priority,
				"Labels":   labels,
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
		PlanName: "deps-test",
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if len(createdBeads) != 3 {
		t.Fatalf("created %d beads, want 3", len(createdBeads))
	}

	// Verify Task A has no dependencies
	if len(createdBeads[0].deps) != 0 {
		t.Errorf("Task A deps = %v, want []", createdBeads[0].deps)
	}

	// Verify Task B depends on Task A
	if len(createdBeads[1].deps) != 1 || createdBeads[1].deps[0] != "bead-1" {
		t.Errorf("Task B deps = %v, want [bead-1]", createdBeads[1].deps)
	}

	// Verify Task C depends on Task A and B
	if len(createdBeads[2].deps) != 2 || createdBeads[2].deps[0] != "bead-1" || createdBeads[2].deps[1] != "bead-2" {
		t.Errorf("Task C deps = %v, want [bead-1, bead-2]", createdBeads[2].deps)
	}

	// Verify result contains created beads
	if len(result.CreatedBeads) != 3 {
		t.Errorf("result.CreatedBeads count = %d, want 3", len(result.CreatedBeads))
	}

	// Verify spec label is added
	for i, bead := range createdBeads {
		found := false
		for _, label := range bead.labels {
			if label == "spec:deps-test" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("bead %d missing spec:deps-test label", i)
		}
	}
}

// TestDecompose_UpdatesPlanFrontmatter verifies that plan frontmatter is updated with decomposed=true and timestamp.
func TestDecompose_UpdatesPlanFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "frontmatter-test.md")
	planContent := `---
spec: frontmatter-test
created: 2026-02-11
---

# Frontmatter Test Plan

Task 1
`
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockClaude := &decomposeTestClaudeClient{
		RunFn: func(prompt string, model string) (interface{}, error) {
			return map[string]interface{}{
				"Success":  true,
				"ExitCode": 0,
				"Output": `[{
					"title": "Task 1",
					"description": "First task",
					"priority": "P1",
					"acceptance_criteria": ["Done"],
					"depends_on_index": []
				}]`,
			}, nil
		},
	}

	mockBead := &decomposeTestBeadClient{
		CreateWithDepsAndDescriptionFn: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
			return map[string]interface{}{
				"ID":       "bead-1",
				"Title":    title,
				"Priority": priority,
				"Labels":   labels,
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
		PlanName: "frontmatter-test",
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		t.Fatalf("Decompose() failed: %v", err)
	}

	if !result.PlanUpdated {
		t.Error("PlanUpdated = false, want true")
	}

	// Read plan file and verify frontmatter was updated
	planData, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("reading plan file: %v", err)
	}
	planStr := string(planData)

	if !strings.Contains(planStr, "decomposed: true") {
		t.Error("plan frontmatter missing 'decomposed: true'")
	}
	if !strings.Contains(planStr, "decomposed_at:") {
		t.Error("plan frontmatter missing 'decomposed_at' timestamp")
	}

	// Verify original frontmatter fields are preserved
	if !strings.Contains(planStr, "spec: frontmatter-test") {
		t.Error("plan frontmatter should preserve existing fields")
	}
	if !strings.Contains(planStr, "created: 2026-02-11") {
		t.Error("plan frontmatter should preserve existing fields")
	}
}

// decomposeTestClaudeClient is a mock with injectable functions for decompose tests.
type decomposeTestClaudeClient struct {
	RunFn func(prompt string, model string) (interface{}, error)
}

func (m *decomposeTestClaudeClient) Run(prompt string, model string) (interface{}, error) {
	if m.RunFn != nil {
		return m.RunFn(prompt, model)
	}
	return nil, nil
}

// decomposeTestBeadClient is a mock with injectable functions for decompose tests.
type decomposeTestBeadClient struct {
	CreateWithDepsAndDescriptionFn func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error)
}

func (m *decomposeTestBeadClient) Ready() (interface{}, error) {
	return nil, nil
}

func (m *decomposeTestBeadClient) Show(id string) (interface{}, error) {
	return nil, nil
}

func (m *decomposeTestBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	return nil, nil
}

func (m *decomposeTestBeadClient) Close(id string) error {
	return nil
}

func (m *decomposeTestBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
	if m.CreateWithDepsAndDescriptionFn != nil {
		return m.CreateWithDepsAndDescriptionFn(title, priority, labels, criteria, deps, desc)
	}
	return nil, nil
}
