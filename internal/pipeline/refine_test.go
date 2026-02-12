package pipeline

import (
	"context"
	"os"
	"testing"
)

func TestRefine_BlankSession(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := tmpDir + "/specs"

	mockAgent := &mockAgent{name: "test-agent"}
	mockBacklog := &mockBacklogClient{}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: mockAgent},
		BacklogClient: mockBacklog,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
	})

	session, err := p.Refine(context.Background(), RefineInput{})
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	if session == nil {
		t.Fatal("expected non-nil session")
	}
}

func TestRefine_ScansExistingSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := tmpDir + "/specs"

	// Create existing spec file
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	existingSpec := specsDir + "/existing.md"
	if err := os.WriteFile(existingSpec, []byte("# Existing"), 0644); err != nil {
		t.Fatal(err)
	}

	p := New(&Deps{
		AgentResolver: &mockAgentResolver{agent: &mockAgent{name: "test-agent"}},
		BacklogClient: &mockBacklogClient{},
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
	})

	session, err := p.Refine(context.Background(), RefineInput{})
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify Refine scanned existing specs
	snapshot := session.GetBeforeSnapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 spec in before snapshot, got %d", len(snapshot))
	}
	if snapshot[0] != existingSpec {
		t.Errorf("expected snapshot to contain %s, got %s", existingSpec, snapshot[0])
	}
}

func TestRefine_BuildsPromptWithIdeaText(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := tmpDir + "/specs"

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
		BacklogClient: &mockBacklogClient{},
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
	})

	_, err := p.Refine(context.Background(), RefineInput{IdeaText: "Add feature X"})
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify prompt contains idea text
	if capturedPrompt == "" {
		t.Fatal("expected prompt to be captured")
	}
	if !contains(capturedPrompt, "Add feature X") {
		t.Errorf("expected prompt to contain idea text, got: %s", capturedPrompt)
	}
	if !contains(capturedPrompt, specsDir) {
		t.Errorf("expected prompt to contain specs directory, got: %s", capturedPrompt)
	}
}

func TestRefine_LoadsBacklogIdea(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := tmpDir + "/specs"

	// Create backlog with an idea
	backlogIdea := &Idea{
		ID:      "idea-123",
		Text:    "Implement feature Y",
		Context: "This is important",
	}
	mockBacklog := &mockBacklogClient{
		ideas: []*Idea{backlogIdea},
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
		BacklogClient: mockBacklog,
	}, &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
	})

	_, err := p.Refine(context.Background(), RefineInput{IdeaID: "idea-123"})
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify prompt contains idea text and context
	if !contains(capturedPrompt, "Implement feature Y") {
		t.Errorf("expected prompt to contain idea text, got: %s", capturedPrompt)
	}
	if !contains(capturedPrompt, "This is important") {
		t.Errorf("expected prompt to contain idea context, got: %s", capturedPrompt)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockAgentResolver implements AgentResolver for testing
type mockAgentResolver struct {
	agent Agent
	err   error
}

func (m *mockAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (Agent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.agent, nil
}

// mockAgent implements Agent for testing
type mockAgent struct {
	name     string
	onLaunch func(promptPath string)
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Launch(promptPath string) error {
	if m.onLaunch != nil {
		m.onLaunch(promptPath)
	}
	return nil
}

func (m *mockAgent) Command(promptPath string) (interface{}, error) {
	return nil, nil
}

// mockBacklogClient implements BacklogClient for testing
type mockBacklogClient struct {
	ideas []*Idea
}

func (m *mockBacklogClient) List() ([]*Idea, error) {
	return m.ideas, nil
}

func (m *mockBacklogClient) Get(id string) (*Idea, error) {
	for _, idea := range m.ideas {
		if idea.ID == id {
			return idea, nil
		}
	}
	return nil, nil
}

func (m *mockBacklogClient) Add(idea *Idea) error {
	m.ideas = append(m.ideas, idea)
	return nil
}

func (m *mockBacklogClient) Update(id string, fn func(*Idea)) error {
	for _, idea := range m.ideas {
		if idea.ID == id {
			fn(idea)
			return nil
		}
	}
	return nil
}
