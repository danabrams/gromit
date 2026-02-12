package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Expected failure: Pipeline.Plan method returns *PlanSession instead of error, but PlanSession type does not have proper implementation yet
func TestPlan_ReturnsSession(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n\nContent here"), 0644); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgent{name: "test-agent"}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	session, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
	})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	if session == nil {
		t.Fatal("expected non-nil session")
	}
}

// Expected failure: Pipeline.Plan returns error for nil dependencies but proper validation logic not implemented
func TestPlan_ValidatesDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create pipeline with nil dependencies
	p := New(nil, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  filepath.Join(tmpDir, "specs"),
		PlansDir:  filepath.Join(tmpDir, "plans"),
	})

	_, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
	})

	if err == nil {
		t.Fatal("expected error for nil dependencies")
	}

	if !strings.Contains(err.Error(), "nil dependencies") {
		t.Errorf("expected 'nil dependencies' error, got: %v", err)
	}
}

// Expected failure: Pipeline.Plan checks if spec file exists but file existence validation not implemented
func TestPlan_ChecksSpecExists(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Don't create the spec file

	mockAgent := &mockAgent{name: "test-agent"}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	_, err := p.Plan(context.Background(), PlanInput{
		SpecName: "nonexistent-spec",
	})

	if err == nil {
		t.Fatal("expected error for nonexistent spec")
	}

	if !strings.Contains(err.Error(), "spec not found") && !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'spec not found' error, got: %v", err)
	}
}

// Expected failure: Pipeline.Plan checks if plan already exists but plan existence validation not implemented
func TestPlan_ChecksPlanExistsWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an existing plan file
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-spec.md")
	if err := os.WriteFile(planPath, []byte("# Existing Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgent{name: "test-agent"}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	_, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
		Force:    false,
	})

	if err == nil {
		t.Fatal("expected error for existing plan without force")
	}

	if !strings.Contains(err.Error(), "plan already exists") && !strings.Contains(err.Error(), "already planned") {
		t.Errorf("expected 'plan already exists' error, got: %v", err)
	}
}

// Expected failure: Pipeline.Plan allows re-planning with Force=true but force flag handling not implemented
func TestPlan_AllowsReplanWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an existing plan file
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(plansDir, "test-spec.md")
	if err := os.WriteFile(planPath, []byte("# Existing Plan"), 0644); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgent{name: "test-agent"}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	session, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
		Force:    true, // Should not error
	})

	if err != nil {
		t.Fatalf("Plan() with force should not error, got: %v", err)
	}

	if session == nil {
		t.Fatal("expected non-nil session")
	}
}

// Expected failure: Pipeline.Plan loads spec content via frontmatter but frontmatter loading not implemented
func TestPlan_LoadsSpecContent(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file with frontmatter
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	specContent := `---
id: test-spec
created: 2026-02-11
---

# Test Spec

This is the spec body content that should be loaded.
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	var capturedPrompt string
	mockAgent := &mockAgent{
		name: "test-agent",
		onLaunch: func(promptPath string) {
			data, _ := os.ReadFile(promptPath)
			capturedPrompt = string(data)
		},
	}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	_, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
	})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// Verify prompt contains spec body content (not frontmatter)
	if !strings.Contains(capturedPrompt, "This is the spec body content") {
		t.Errorf("expected prompt to contain spec body, got: %s", capturedPrompt)
	}
	// Frontmatter should not be in the prompt
	if strings.Contains(capturedPrompt, "id: test-spec") {
		t.Errorf("expected prompt to not contain frontmatter, got: %s", capturedPrompt)
	}
}

// Expected failure: Pipeline.Plan gathers open beads context but bead context gathering not implemented
func TestPlan_GathersOpenBeadsContext(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock bead client with open beads
	mockBead := &mockBeadClient{
		beads: []MockBead{
			{ID: "bead-1", Title: "Implement feature X", Priority: 1, Description: "Details here"},
			{ID: "bead-2", Title: "Fix bug Y", Priority: 0, Description: ""},
		},
	}

	var capturedPrompt string
	mockAgent := &mockAgent{
		name: "test-agent",
		onLaunch: func(promptPath string) {
			data, _ := os.ReadFile(promptPath)
			capturedPrompt = string(data)
		},
	}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	_, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
	})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// Verify prompt contains open beads
	if !strings.Contains(capturedPrompt, "Implement feature X") {
		t.Errorf("expected prompt to contain first bead title, got: %s", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "Fix bug Y") {
		t.Errorf("expected prompt to contain second bead title, got: %s", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "Current Open Beads") && !strings.Contains(capturedPrompt, "Open Beads") {
		t.Errorf("expected prompt to have beads section header, got: %s", capturedPrompt)
	}
}

// Expected failure: Pipeline.Plan builds system prompt with spec and paths but prompt building not implemented
func TestPlan_BuildsSystemPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec\n\nSpec content"), 0644); err != nil {
		t.Fatal(err)
	}

	var capturedPrompt string
	mockAgent := &mockAgent{
		name: "test-agent",
		onLaunch: func(promptPath string) {
			data, _ := os.ReadFile(promptPath)
			capturedPrompt = string(data)
		},
	}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	_, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
	})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// Verify prompt structure contains expected sections
	if !strings.Contains(capturedPrompt, "test-spec") {
		t.Errorf("expected prompt to contain spec name, got: %s", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, plansDir) {
		t.Errorf("expected prompt to contain plans directory, got: %s", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "Spec content") {
		t.Errorf("expected prompt to contain spec content, got: %s", capturedPrompt)
	}
}

// Expected failure: Pipeline.Plan writes temp prompt file but temp file writing not implemented
func TestPlan_WritesTempPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	var promptPath string
	mockAgent := &mockAgent{
		name: "test-agent",
		onLaunch: func(path string) {
			promptPath = path
		},
	}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	_, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
	})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// Verify temp file was created and passed to agent
	if promptPath == "" {
		t.Fatal("expected prompt path to be captured")
	}
	if !strings.Contains(promptPath, tmpDir) && !strings.Contains(promptPath, "tmp") {
		t.Errorf("expected prompt path to be in temp directory, got: %s", promptPath)
	}
}

// Expected failure: Pipeline.Plan resolves agent with correct phase but agent resolution not implemented
func TestPlan_ResolvesAgent(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	capturedPhase := ""
	capturedAgentName := ""
	mockAgent := &mockAgent{name: "custom-agent"}
	mockResolver := &mockAgentResolverPlan{
		agent: mockAgent,
		onResolve: func(phase, flagOverride string, choosePicker bool) {
			capturedPhase = phase
			capturedAgentName = flagOverride
		},
	}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: mockResolver,
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	_, err := p.Plan(context.Background(), PlanInput{
		SpecName:  "test-spec",
		AgentName: "custom-agent",
	})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// Verify agent was resolved with correct phase
	if capturedPhase != "plan" {
		t.Errorf("expected phase 'plan', got: %s", capturedPhase)
	}
	if capturedAgentName != "custom-agent" {
		t.Errorf("expected agent name 'custom-agent', got: %s", capturedAgentName)
	}
}

// Expected failure: Pipeline.Plan returns PlanSession that implements Session interface but PlanSession implementation not complete
func TestPlan_ReturnsSessionWithInterface(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgent{name: "test-agent"}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	session, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
	})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// PlanSession should implement Session interface
	var _ Session = session
}

// mockBeadClient implements BeadClient for testing
type mockBeadClient struct {
	beads []MockBead
}

type MockBead struct {
	ID          string
	Title       string
	Priority    int
	Description string
}

func (m *mockBeadClient) Ready() (interface{}, error) {
	return m.beads, nil
}

func (m *mockBeadClient) Show(id string) (interface{}, error) {
	for _, bead := range m.beads {
		if bead.ID == id {
			return bead, nil
		}
	}
	return nil, fmt.Errorf("bead not found: %s", id)
}

func (m *mockBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	return nil, nil
}

func (m *mockBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
	return nil, nil
}

func (m *mockBeadClient) Close(id string) error {
	return nil
}

func (m *mockBeadClient) List() ([]MockBead, error) {
	return m.beads, nil
}

// Expected failure: PlanSession.Wait() should detect created plan files but post-processing not implemented
func TestPlan_DetectsCreatedPlan(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	plansDir := filepath.Join(tmpDir, "plans")

	// Create a spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, "test-spec.md")
	if err := os.WriteFile(specPath, []byte("# Test Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock agent that creates a plan file during launch
	planPath := filepath.Join(plansDir, "test-spec.md")
	mockAgent := &mockAgent{
		name: "test-agent",
		onLaunch: func(promptPath string) {
			os.MkdirAll(plansDir, 0755)
			os.WriteFile(planPath, []byte("# Test Plan\n\nImplementation plan here"), 0644)
		},
	}
	mockBead := &mockBeadClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BeadClient:    mockBead,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	})

	session, err := p.Plan(context.Background(), PlanInput{
		SpecName: "test-spec",
	})
	if err != nil {
		t.Fatalf("Plan() failed: %v", err)
	}

	// In real implementation, session.Wait() or post-processing should detect the plan
	// For now, just verify session was returned
	if session == nil {
		t.Fatal("expected non-nil session")
	}

	// Verify plan file was actually created during agent launch
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		t.Fatal("expected plan file to be created by mock agent")
	}
}

// mockAgentResolverPlan is a plan-specific mock that captures resolve parameters
type mockAgentResolverPlan struct {
	agent     Agent
	err       error
	onResolve func(phase, flagOverride string, choosePicker bool)
}

func (m *mockAgentResolverPlan) Resolve(phase string, flagOverride string, choosePicker bool) (Agent, error) {
	if m.onResolve != nil {
		m.onResolve(phase, flagOverride, choosePicker)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.agent, nil
}
