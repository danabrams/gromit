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

// TestRefineWorkflowWithBacklogID verifies Refine workflow creates specs from backlog items
// Expected failure: Pipeline.Refine() does not implement backlog item refinement yet
func TestRefineWorkflowWithBacklogID(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	backlogDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create mock backlog file with an unrefined item
	backlogPath := filepath.Join(backlogDir, "backlog.jsonl")
	backlogContent := `{"id":"idea-123","text":"Add authentication","status":"open","created":"2026-02-12T10:00:00Z"}
`
	if err := os.WriteFile(backlogPath, []byte(backlogContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock dependencies
	mockBacklog := &mockBacklogClient{
		getFunc: func(id string) (interface{}, error) {
			if id == "idea-123" {
				return map[string]interface{}{
					"id":      "idea-123",
					"text":    "Add authentication",
					"status":  "open",
					"created": "2026-02-12T10:00:00Z",
				}, nil
			}
			return nil, fmt.Errorf("not found")
		},
		updateFunc: func(id string, fn func(interface{})) error {
			return nil // Accept update
		},
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			// Return a mock agent that creates a spec file
			return mockAgentThatCreatesSpec(specsDir, "authentication.md"), nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
		BacklogClient: mockBacklog,
	}
	paths := &Paths{
		GromitDir: backlogDir,
		SpecsDir:  specsDir,
	}

	p := New(deps, paths)

	// Execute
	ctx := context.Background()
	input := RefineInput{
		IdeaID: "idea-123",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Wait for session to complete
	if err := session.Wait(); err != nil {
		t.Fatalf("Session.Wait() failed: %v", err)
	}

	// Get results
	result, err := session.Result()
	if err != nil {
		t.Fatalf("Result() failed: %v", err)
	}

	// Verify: spec was created
	if len(result.CreatedSpecs) != 1 {
		t.Errorf("CreatedSpecs count = %d, want 1", len(result.CreatedSpecs))
	}

	// Verify: backlog item was marked as refined
	if len(result.RefinedItems) != 1 || result.RefinedItems[0] != "idea-123" {
		t.Errorf("RefinedItems = %v, want [idea-123]", result.RefinedItems)
	}

	// Verify spec file exists
	specPath := filepath.Join(specsDir, "authentication.md")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		t.Error("Spec file was not created")
	}
}

// TestRefineWorkflowWithAdHocIdea verifies Refine workflow handles ad-hoc ideas
// Expected failure: Pipeline.Refine() does not handle ad-hoc idea text yet
func TestRefineWorkflowWithAdHocIdea(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return mockAgentThatCreatesSpec(specsDir, "caching.md"), nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
	}
	paths := &Paths{
		SpecsDir: specsDir,
	}

	p := New(deps, paths)

	// Execute with ad-hoc idea text
	ctx := context.Background()
	input := RefineInput{
		IdeaText: "Add caching layer to reduce database queries",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("Session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("Result() failed: %v", err)
	}

	// Verify: spec was created
	if len(result.CreatedSpecs) != 1 {
		t.Errorf("CreatedSpecs count = %d, want 1", len(result.CreatedSpecs))
	}

	// Verify: no backlog items were marked as refined (ad-hoc idea)
	if len(result.RefinedItems) != 0 {
		t.Errorf("RefinedItems = %v, want empty (ad-hoc idea)", result.RefinedItems)
	}
}

// TestRefineWorkflowBlankSessionCreatesBacklogItem verifies blank sessions that produce specs create backlog items
// Expected failure: Pipeline.Refine() does not create backlog items for blank sessions yet
func TestRefineWorkflowBlankSessionCreatesBacklogItem(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	backlogDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	var addedItems []interface{}
	mockBacklog := &mockBacklogClient{
		addFunc: func(item interface{}) error {
			addedItems = append(addedItems, item)
			return nil
		},
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return mockAgentThatCreatesSpec(specsDir, "new-feature.md"), nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
		BacklogClient: mockBacklog,
	}
	paths := &Paths{
		GromitDir: backlogDir,
		SpecsDir:  specsDir,
	}

	p := New(deps, paths)

	// Execute with neither IdeaID nor IdeaText (blank session)
	ctx := context.Background()
	input := RefineInput{}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("Session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("Result() failed: %v", err)
	}

	// Verify spec was created
	if len(result.CreatedSpecs) == 0 {
		t.Fatal("No specs created in blank session")
	}

	// Verify backlog item was created
	if len(addedItems) != 1 {
		t.Errorf("Backlog items added = %d, want 1 (blank session should create backlog item)", len(addedItems))
	}
}

// TestRefineWorkflowScansForNewSpecs verifies post-processing detects new spec files
// Expected failure: Pipeline.Refine() does not scan for new specs in post-processing yet
func TestRefineWorkflowScansForNewSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing spec before session
	existingSpec := filepath.Join(specsDir, "existing.md")
	if err := os.WriteFile(existingSpec, []byte("# Existing Spec"), 0644); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			// Agent will create 2 new specs during session
			return mockAgentThatCreatesMultipleSpecs(specsDir, []string{"spec1.md", "spec2.md"}), nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
	}
	paths := &Paths{
		SpecsDir: specsDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := RefineInput{
		IdeaText: "Create multiple features",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	if err := session.Wait(); err != nil {
		t.Fatalf("Session.Wait() failed: %v", err)
	}

	result, err := session.Result()
	if err != nil {
		t.Fatalf("Result() failed: %v", err)
	}

	// Verify only NEW specs are reported (not existing.md)
	if len(result.CreatedSpecs) != 2 {
		t.Errorf("CreatedSpecs count = %d, want 2 (only new specs)", len(result.CreatedSpecs))
	}

	// Verify existing spec is not in results
	for _, spec := range result.CreatedSpecs {
		if strings.Contains(spec, "existing.md") {
			t.Error("CreatedSpecs includes existing spec, should only list new specs")
		}
	}
}

// TestRefineWorkflowRespectsContextCancellation verifies context cancellation stops the workflow
// Expected failure: Pipeline.Refine() does not respect context cancellation yet
func TestRefineWorkflowRespectsContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return mockLongRunningAgent(), nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
	}
	paths := &Paths{
		SpecsDir: specsDir,
	}

	p := New(deps, paths)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	input := RefineInput{
		IdeaText: "Long running refinement",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Cancel immediately
	cancel()

	// Wait should return quickly (not block)
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

// TestRefineWorkflowAgentOverride verifies agent name override is used
// Expected failure: Pipeline.Refine() does not pass agent override to resolver yet
func TestRefineWorkflowAgentOverride(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	var resolvedWithOverride string
	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			resolvedWithOverride = flagOverride
			return mockAgentThatCreatesSpec(specsDir, "spec.md"), nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgent,
	}
	paths := &Paths{
		SpecsDir: specsDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := RefineInput{
		IdeaText:  "Test idea",
		AgentName: "opus-agent",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify agent override was passed to resolver
	if resolvedWithOverride != "opus-agent" {
		t.Errorf("Agent resolver received override = %q, want %q", resolvedWithOverride, "opus-agent")
	}

	_ = session.Wait()
}

// TestRefineWorkflowNilDependenciesError verifies proper error handling for nil dependencies
// Expected failure: Pipeline.Refine() does not validate dependencies correctly yet
func TestRefineWorkflowNilDependenciesError(t *testing.T) {
	p := New(nil, &Paths{})

	ctx := context.Background()
	input := RefineInput{IdeaText: "Test"}

	_, err := p.Refine(ctx, input)
	if err == nil {
		t.Error("Refine() with nil dependencies returned nil error, want error")
	}

	if !strings.Contains(err.Error(), "nil dependencies") && !strings.Contains(err.Error(), "dependencies") {
		t.Errorf("Error message = %q, want message about nil dependencies", err.Error())
	}
}

// TestRefineWorkflowWritesTempPrompt verifies prompt is written to temp file for agent
// Expected failure: Pipeline.Refine() does not write prompt to temp file yet
func TestRefineWorkflowWritesTempPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	promptDir := filepath.Join(tmpDir, "prompts")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatal(err)
	}

	var promptFilePath string
	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return &capturePromptAgent{
				captureFunc: func(path string) {
					promptFilePath = path
				},
			}, nil
		},
	}

	mockRenderer := &mockPromptRenderer{
		renderRefineFunc: func(input interface{}) (string, error) {
			return "Refine this idea: test idea", nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgent,
		PromptRenderer: mockRenderer,
	}
	paths := &Paths{
		GromitDir: tmpDir,
		SpecsDir:  specsDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := RefineInput{IdeaText: "test idea"}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	_ = session.Wait()

	// Verify prompt file was created
	if promptFilePath == "" {
		t.Fatal("No prompt file path captured, want temp file created")
	}

	// Verify prompt file existed during session (may be cleaned up after)
	// The existence check happens in the agent launch, so if we got here, it worked
}

// Mock types for testing

type mockAgentResolver struct {
	resolveFunc func(phase string, flagOverride string, choosePicker bool) (interface{}, error)
}

func (m *mockAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(phase, flagOverride, choosePicker)
	}
	return nil, fmt.Errorf("not implemented")
}

type mockBacklogClient struct {
	getFunc    func(id string) (interface{}, error)
	updateFunc func(id string, fn func(interface{})) error
	addFunc    func(item interface{}) error
}

func (m *mockBacklogClient) Get(id string) (interface{}, error) {
	if m.getFunc != nil {
		return m.getFunc(id)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBacklogClient) Update(id string, fn func(interface{})) error {
	if m.updateFunc != nil {
		return m.updateFunc(id, fn)
	}
	return fmt.Errorf("not implemented")
}

func (m *mockBacklogClient) Add(item interface{}) error {
	if m.addFunc != nil {
		return m.addFunc(item)
	}
	return fmt.Errorf("not implemented")
}

func (m *mockBacklogClient) List() ([]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

type mockPromptRenderer struct {
	renderRefineFunc func(input interface{}) (string, error)
}

func (m *mockPromptRenderer) RenderRefine(input interface{}) (string, error) {
	if m.renderRefineFunc != nil {
		return m.renderRefineFunc(input)
	}
	return "", fmt.Errorf("not implemented")
}

func (m *mockPromptRenderer) RenderPlan(input interface{}) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockPromptRenderer) RenderDecompose(input interface{}) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// Helper functions to create mock agents

func mockAgentThatCreatesSpec(specsDir, filename string) interface{} {
	// Returns an agent that creates a spec file when "launched"
	return &simpleFileCreator{
		dir:      specsDir,
		filename: filename,
	}
}

func mockAgentThatCreatesMultipleSpecs(specsDir string, filenames []string) interface{} {
	return &multiFileCreator{
		dir:       specsDir,
		filenames: filenames,
	}
}

func mockLongRunningAgent() interface{} {
	return &longRunningAgent{}
}

type simpleFileCreator struct {
	dir      string
	filename string
}

type multiFileCreator struct {
	dir       string
	filenames []string
}

type longRunningAgent struct{}

type capturePromptAgent struct {
	captureFunc func(path string)
}
