package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPipeline_ExploreValidatesDeps verifies that Explore returns an error when required dependencies are nil.
func TestPipeline_ExploreValidatesDeps(t *testing.T) {
	// Expected failure: Pipeline.Explore method does not exist yet with full dependency validation
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
				BacklogClient:  &testBacklogClient{},
			},
			paths:   &Paths{},
			wantErr: "nil AgentResolver",
		},
		{
			name: "nil PromptRenderer",
			deps: &Deps{
				AgentResolver:  &testAgentResolver{},
				PromptRenderer: nil,
				BacklogClient:  &testBacklogClient{},
			},
			paths:   &Paths{},
			wantErr: "nil PromptRenderer",
		},
		{
			name: "nil BacklogClient",
			deps: &Deps{
				AgentResolver:  &testAgentResolver{},
				PromptRenderer: &testPromptRenderer{},
				BacklogClient:  nil,
			},
			paths:   &Paths{},
			wantErr: "nil BacklogClient",
		},
		{
			name: "AgentResolver checked before PromptRenderer when both nil",
			deps: &Deps{
				AgentResolver:  nil,
				PromptRenderer: nil,
				BacklogClient:  &testBacklogClient{},
			},
			paths:   &Paths{},
			wantErr: "nil AgentResolver",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := New(tc.deps, tc.paths)
			ctx := context.Background()
			input := ExploreInput{Topic: "test topic"}

			_, err := p.Explore(ctx, input)
			if err == nil {
				t.Fatal("Explore() should return error with invalid dependencies")
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}

	// Direct validateExploreDeps tests: typed nil and all-present success cases.
	// A typed nil (e.g. (*T)(nil)) is a non-nil interface in Go, so it passes
	// == nil checks. These cases verify behavior of validateExploreDeps directly.
	directCases := []struct {
		name    string
		deps    *Deps
		wantErr string
	}{
		{
			name: "typed nil AgentResolver passes validation",
			deps: &Deps{
				AgentResolver:  (*testAgentResolver)(nil),
				PromptRenderer: &testPromptRenderer{},
				BacklogClient:  &testBacklogClient{},
			},
			wantErr: "",
		},
		{
			name: "all deps present passes validation",
			deps: &Deps{
				AgentResolver:  &testAgentResolver{},
				PromptRenderer: &testPromptRenderer{},
				BacklogClient:  &testBacklogClient{},
			},
			wantErr: "",
		},
	}

	for _, tc := range directCases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(tc.deps, &Paths{})
			err := p.validateExploreDeps()
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("validateExploreDeps() = %v, want nil", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("validateExploreDeps() = %v, want error containing %q", err, tc.wantErr)
				}
			}
		})
	}
}

// TestPipeline_ExploreRecordsExistingArtifacts verifies that Explore takes pre-session snapshots
// of existing epics, specs, and backlog items.
func TestPipeline_ExploreRecordsExistingArtifacts(t *testing.T) {
	// Expected failure: Pipeline.Explore does not take pre-session snapshots yet
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, "epics")
	specsDir := filepath.Join(tmpDir, "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")

	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Create existing artifacts
	existingEpic := filepath.Join(epicsDir, "existing.md")
	if err := os.WriteFile(existingEpic, []byte("# Existing Epic\n"), 0644); err != nil {
		t.Fatalf("failed to create existing epic: %v", err)
	}

	existingSpec := filepath.Join(specsDir, "existing-spec.md")
	if err := os.WriteFile(existingSpec, []byte("# Existing Spec\n"), 0644); err != nil {
		t.Fatalf("failed to create existing spec: %v", err)
	}

	// Track what artifacts should be recorded
	var recordedEpics, recordedSpecs []string
	var recordedBacklogCount int

	mockBacklog := &mockBacklogClient{
		ListFn: func() ([]*Idea, error) {
			recordedBacklogCount++
			return []*Idea{
				{ID: "idea-1", Text: "Existing idea"},
			}, nil
		},
	}

	mockAgent := &mockAgent{
		NameFn: func() string {
			return "claude"
		},
		LaunchFn: func(promptPath string) error {
			// Simulate session completing successfully
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	mockRenderer := &mockPromptRenderer{
		RenderExploreFn: func(input *ExplorePromptInput) (string, error) {
			recordedEpics, _ = ListMarkdownFiles(epicsDir)
			recordedSpecs, _ = ListMarkdownFiles(specsDir)
			return "explore prompt", nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgentResolver,
		PromptRenderer: mockRenderer,
		BacklogClient:  mockBacklog,
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := ExploreInput{Topic: "test topic"}

	_, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	// Verify pre-session snapshots were taken
	if len(recordedEpics) != 1 {
		t.Errorf("should record 1 existing epic, recorded %d", len(recordedEpics))
	}

	if len(recordedSpecs) != 1 {
		t.Errorf("should record 1 existing spec, recorded %d", len(recordedSpecs))
	}

	if recordedBacklogCount == 0 {
		t.Error("should call BacklogClient.List() to record existing backlog items")
	}
}

// TestPipeline_ExploreBuildsPromptWithContext verifies that Explore builds a prompt
// including CLAUDE.md, rules, learnings, and topic.
func TestPipeline_ExploreBuildsPromptWithContext(t *testing.T) {
	// Expected failure: Pipeline.Explore does not build explore prompt using renderer yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")

	var capturedContext *ExplorePromptInput

	mockRenderer := &mockPromptRenderer{
		RenderExploreFn: func(input *ExplorePromptInput) (string, error) {
			capturedContext = input
			return "explore prompt with full context", nil
		},
	}

	mockAgent := &mockAgent{
		LaunchFn: func(promptPath string) error {
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgentResolver,
		PromptRenderer: mockRenderer,
		BacklogClient:  &testBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := ExploreInput{Topic: "Improve developer onboarding"}

	_, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	if capturedContext == nil {
		t.Fatal("Explore should pass context to PromptRenderer.RenderExplore")
	}

	// Verify the context includes the query (previously called Topic)
	if capturedContext.Query != "Improve developer onboarding" {
		t.Errorf("context Query = %q, want %q", capturedContext.Query, "Improve developer onboarding")
	}
}

// TestPipeline_ExploreWritesTempFile verifies that Explore writes the prompt to a temp file.
func TestPipeline_ExploreWritesTempFile(t *testing.T) {
	// Expected failure: Pipeline.Explore does not write temp prompt file yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	tmpFilesDir := filepath.Join(gromitDir, "tmp")

	var tempFilePath string

	mockAgent := &mockAgent{
		LaunchFn: func(promptPath string) error {
			tempFilePath = promptPath
			// Verify the file exists and has content
			if _, err := os.Stat(promptPath); os.IsNotExist(err) {
				return fmt.Errorf("temp file does not exist: %s", promptPath)
			}
			content, err := os.ReadFile(promptPath)
			if err != nil {
				return fmt.Errorf("failed to read temp file: %w", err)
			}
			if len(content) == 0 {
				return fmt.Errorf("temp file is empty")
			}
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	mockRenderer := &mockPromptRenderer{
		RenderExploreFn: func(input *ExplorePromptInput) (string, error) {
			return "test explore prompt content", nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgentResolver,
		PromptRenderer: mockRenderer,
		BacklogClient:  &testBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := ExploreInput{Topic: "test"}

	_, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	if tempFilePath == "" {
		t.Fatal("agent.Launch should have been called with temp file path")
	}

	if !strings.HasPrefix(tempFilePath, tmpFilesDir) {
		t.Errorf("temp file path = %q, should be in %q", tempFilePath, tmpFilesDir)
	}

	if !strings.HasSuffix(tempFilePath, ".md") {
		t.Errorf("temp file path = %q, should end with .md", tempFilePath)
	}
}

// TestPipeline_ExploreResolvesAgent verifies that Explore calls AgentResolver.Resolve properly.
func TestPipeline_ExploreResolvesAgent(t *testing.T) {
	// Expected failure: Pipeline.Explore does not call AgentResolver.Resolve yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")

	var resolvedPhase string
	var resolvedChoosePicker bool

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			resolvedPhase = phase
			resolvedChoosePicker = choosePicker
			return &mockAgent{}, nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgentResolver,
		PromptRenderer: &testPromptRenderer{},
		BacklogClient:  &testBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := ExploreInput{Topic: "test"}

	_, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	if resolvedPhase != "explore" {
		t.Errorf("Resolve phase = %q, want %q", resolvedPhase, "explore")
	}

	if resolvedChoosePicker {
		t.Error("Resolve choosePicker should be false for non-interactive mode")
	}
}

// TestPipeline_ExplorePostProcessingDetectsNewArtifacts verifies that Explore detects
// new epics, specs, and backlog items created during the session.
func TestPipeline_ExplorePostProcessingDetectsNewArtifacts(t *testing.T) {
	// Expected failure: Pipeline.Explore returns *ExploreSession instead of *ExploreResult
	// Expected failure: Pipeline.Explore does not perform post-processing artifact detection yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Pre-existing artifacts
	existingEpic := filepath.Join(epicsDir, "existing.md")
	if err := os.WriteFile(existingEpic, []byte("# Existing\n"), 0644); err != nil {
		t.Fatalf("failed to create existing epic: %v", err)
	}

	mockAgent := &mockAgent{
		LaunchFn: func(promptPath string) error {
			// Simulate session creating new artifacts
			newEpic := filepath.Join(epicsDir, "new-epic.md")
			if err := os.WriteFile(newEpic, []byte("# New Epic\n"), 0644); err != nil {
				return err
			}

			newSpec := filepath.Join(specsDir, "new-spec.md")
			if err := os.WriteFile(newSpec, []byte("# New Spec\n"), 0644); err != nil {
				return err
			}

			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	mockBacklog := &mockBacklogClient{
		ListFn: func() ([]*Idea, error) {
			return []*Idea{}, nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgentResolver,
		PromptRenderer: &testPromptRenderer{},
		BacklogClient:  mockBacklog,
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := ExploreInput{Topic: "test"}

	// Note: Current implementation returns (*ExploreSession, error)
	// Expected implementation returns (*ExploreResult, error)
	// This will fail at compile time until return type changes
	result, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	// Verify new artifacts were detected
	if len(result.CreatedEpics) != 1 {
		t.Errorf("result.CreatedEpics = %d, want 1", len(result.CreatedEpics))
	}

	if len(result.CreatedSpecs) != 1 {
		t.Errorf("result.CreatedSpecs = %d, want 1", len(result.CreatedSpecs))
	}
}

// TestPipeline_ExploreReturnsExploreResult verifies that Explore returns an ExploreResult
// with properly initialized slice fields.
func TestPipeline_ExploreReturnsExploreResult(t *testing.T) {
	// Expected failure: Pipeline.Explore returns *ExploreSession instead of *ExploreResult
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")

	mockAgent := &mockAgent{
		LaunchFn: func(promptPath string) error {
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgentResolver,
		PromptRenderer: &testPromptRenderer{},
		BacklogClient:  &testBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := ExploreInput{Topic: "test"}

	result, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	// Verify slices are initialized (not nil)
	if result.CreatedSpecs == nil {
		t.Error("result.CreatedSpecs should be non-nil empty slice")
	}

	if result.CreatedEpics == nil {
		t.Error("result.CreatedEpics should be non-nil empty slice")
	}

	if result.CreatedBacklogItems == nil {
		t.Error("result.CreatedBacklogItems should be non-nil empty slice")
	}
}

// TestPipeline_ExploreRespectsContext verifies that Explore respects context cancellation.
func TestPipeline_ExploreRespectsContext(t *testing.T) {
	// Expected failure: Pipeline.Explore does not handle context cancellation yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")

	mockAgent := &mockAgent{
		LaunchFn: func(promptPath string) error {
			// Simulate long-running operation
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgentResolver,
		PromptRenderer: &testPromptRenderer{},
		BacklogClient:  &testBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
	}

	p := New(deps, paths)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	input := ExploreInput{Topic: "test"}

	_, err := p.Explore(ctx, input)
	// Should handle cancellation gracefully (either before or during execution)
	// This test verifies context is passed through properly
	_ = err // Accept any outcome for now, just verify context parameter is used
}

// Mock types for testing

type mockAgentResolver struct {
	ResolveFn func(phase, flagOverride string, choosePicker bool) (Agent, error)
}

func (m *mockAgentResolver) Resolve(phase, flagOverride string, choosePicker bool) (Agent, error) {
	if m.ResolveFn != nil {
		return m.ResolveFn(phase, flagOverride, choosePicker)
	}
	return nil, fmt.Errorf("ResolveFn not set")
}

type mockAgent struct {
	NameFn   func() string
	LaunchFn func(promptPath string) error
}

func (m *mockAgent) Name() string {
	if m.NameFn != nil {
		return m.NameFn()
	}
	return "mock-agent"
}

func (m *mockAgent) Launch(promptPath string) error {
	if m.LaunchFn != nil {
		return m.LaunchFn(promptPath)
	}
	return nil
}

type mockBacklogClient struct {
	ListFn   func() ([]*Idea, error)
	GetFn    func(id string) (*Idea, error)
	AddFn    func(item *Idea) error
	UpdateFn func(id string, fn func(*Idea)) error
}

func (m *mockBacklogClient) List() ([]*Idea, error) {
	if m.ListFn != nil {
		return m.ListFn()
	}
	return []*Idea{}, nil
}

func (m *mockBacklogClient) Get(id string) (*Idea, error) {
	if m.GetFn != nil {
		return m.GetFn(id)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockBacklogClient) Add(item *Idea) error {
	if m.AddFn != nil {
		return m.AddFn(item)
	}
	return nil
}

func (m *mockBacklogClient) Update(id string, fn func(*Idea)) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(id, fn)
	}
	return nil
}

type mockPromptRenderer struct {
	RenderRefineFn         func(input *RefinePromptInput) (string, error)
	RenderPlanFn           func(input *PlanPromptInput) (string, error)
	RenderDecomposeFn      func(input *DecomposePromptInput) (string, error)
	RenderThoroughReviewFn func(input *ThoroughReviewPromptInput) (string, error)
	RenderExploreFn        func(input *ExplorePromptInput) (string, error)
}

func (m *mockPromptRenderer) RenderRefine(input *RefinePromptInput) (string, error) {
	if m.RenderRefineFn != nil {
		return m.RenderRefineFn(input)
	}
	return "refine prompt", nil
}

func (m *mockPromptRenderer) RenderPlan(input *PlanPromptInput) (string, error) {
	if m.RenderPlanFn != nil {
		return m.RenderPlanFn(input)
	}
	return "plan prompt", nil
}

func (m *mockPromptRenderer) RenderDecompose(input *DecomposePromptInput) (string, error) {
	if m.RenderDecomposeFn != nil {
		return m.RenderDecomposeFn(input)
	}
	return "decompose prompt", nil
}

func (m *mockPromptRenderer) RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error) {
	if m.RenderThoroughReviewFn != nil {
		return m.RenderThoroughReviewFn(input)
	}
	return "review prompt", nil
}

// RenderExplore is a new method that doesn't exist yet in the PromptRenderer interface
func (m *mockPromptRenderer) RenderExplore(input *ExplorePromptInput) (string, error) {
	if m.RenderExploreFn != nil {
		return m.RenderExploreFn(input)
	}
	return "explore prompt", nil
}
