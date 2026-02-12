package pipeline

import (
	"context"
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
	name string
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Launch(promptPath string) error {
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
