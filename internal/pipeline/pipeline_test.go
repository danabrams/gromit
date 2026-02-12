package pipeline

import (
	"testing"
)

// These should compile if the interfaces are properly defined
var _ AgentResolver = (*mockAgentResolver)(nil)
var _ ClaudeClient = (*mockClaudeClient)(nil)
var _ BeadClient = (*mockBeadClient)(nil)
var _ BacklogClient = (*mockBacklogClient)(nil)
var _ PromptRenderer = (*mockPromptRenderer)(nil)
var _ LearningsManager = (*mockLearningsManager)(nil)
var _ StateManager = (*mockStateManager)(nil)
var _ LogWriter = (*mockLogWriter)(nil)

// Mock implementations for testing
type mockAgentResolver struct{}

func (m *mockAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
	return nil, nil
}

type mockClaudeClient struct{}

func (m *mockClaudeClient) Run(prompt string, model string) (interface{}, error) {
	return nil, nil
}

type mockBeadClient struct{}

func (m *mockBeadClient) Ready() (interface{}, error) {
	return nil, nil
}

func (m *mockBeadClient) Show(id string) (interface{}, error) {
	return nil, nil
}

func (m *mockBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	return nil, nil
}

func (m *mockBeadClient) Close(id string) error {
	return nil
}

type mockBacklogClient struct{}

func (m *mockBacklogClient) List() ([]interface{}, error) {
	return nil, nil
}

func (m *mockBacklogClient) Get(id string) (interface{}, error) {
	return nil, nil
}

func (m *mockBacklogClient) Add(item interface{}) error {
	return nil
}

func (m *mockBacklogClient) Update(id string, fn func(interface{})) error {
	return nil
}

type mockPromptRenderer struct{}

func (m *mockPromptRenderer) RenderRefine(input interface{}) (string, error) {
	return "", nil
}

func (m *mockPromptRenderer) RenderPlan(input interface{}) (string, error) {
	return "", nil
}

func (m *mockPromptRenderer) RenderDecompose(input interface{}) (string, error) {
	return "", nil
}

type mockLearningsManager struct{}

func (m *mockLearningsManager) Add(content string) error {
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

func TestNew_ReturnsNonNil(t *testing.T) {
	deps := &Deps{}
	paths := &Paths{}

	p := New(deps, paths)

	if p == nil {
		t.Fatal("New() returned nil, expected non-nil Pipeline")
	}
}

func TestPaths_FieldAccess(t *testing.T) {
	paths := Paths{
		GromitDir: "/tmp/.gromit",
		SpecsDir:  "/tmp/.gromit/specs",
		PlansDir:  "/tmp/.gromit/plans",
		EpicsDir:  "/tmp/.gromit/epics",
	}

	if paths.GromitDir != "/tmp/.gromit" {
		t.Errorf("GromitDir = %q, want %q", paths.GromitDir, "/tmp/.gromit")
	}
	if paths.SpecsDir != "/tmp/.gromit/specs" {
		t.Errorf("SpecsDir = %q, want %q", paths.SpecsDir, "/tmp/.gromit/specs")
	}
	if paths.PlansDir != "/tmp/.gromit/plans" {
		t.Errorf("PlansDir = %q, want %q", paths.PlansDir, "/tmp/.gromit/plans")
	}
	if paths.EpicsDir != "/tmp/.gromit/epics" {
		t.Errorf("EpicsDir = %q, want %q", paths.EpicsDir, "/tmp/.gromit/epics")
	}
}
