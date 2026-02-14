package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestClaudeClientRun_ReturnsTypedResult verifies ClaudeClient.Run returns *ClaudeRunResult instead of interface{}
// Expected failure: ClaudeClient.Run() currently returns (interface{}, error) and ClaudeRunResult type does not exist
func TestClaudeClientRun_ReturnsTypedResult(t *testing.T) {
	mockClaude := &typedInterfacesClaudeClient{
		runFn: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output:   "test output",
			}, nil
		},
	}

	result, err := mockClaude.Run("test prompt", "sonnet")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify result is typed struct, not interface{}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if !result.Success {
		t.Errorf("Success = %v, want true", result.Success)
	}

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Output != "test output" {
		t.Errorf("Output = %q, want %q", result.Output, "test output")
	}
}

// TestClaudeClientRun_FailureResult verifies ClaudeRunResult handles failure cases with typed fields
// Expected failure: ClaudeRunResult type does not exist and ClaudeClient.Run() returns interface{}
func TestClaudeClientRun_FailureResult(t *testing.T) {
	mockClaude := &typedInterfacesClaudeClient{
		runFn: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  false,
				ExitCode: 1,
				Output:   "build failed",
			}, nil
		},
	}

	result, err := mockClaude.Run("test prompt", "sonnet")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Success {
		t.Errorf("Success = %v, want false", result.Success)
	}

	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}

	if result.Output != "build failed" {
		t.Errorf("Output = %q, want %q", result.Output, "build failed")
	}
}

// TestBeadClientReady_ReturnsTypedResult verifies BeadClient.Ready returns *BeadInfo instead of interface{}
// Expected failure: BeadClient.Ready() currently returns (interface{}, error) and BeadInfo type does not exist
func TestBeadClientReady_ReturnsTypedResult(t *testing.T) {
	mockBead := &typedInterfacesBeadClient{
		readyFn: func() (*BeadInfo, error) {
			return &BeadInfo{
				ID:       "bead-123",
				Title:    "Test bead",
				Priority: 1,
				Labels:   []string{"spec:test"},
			}, nil
		},
	}

	result, err := mockBead.Ready()
	if err != nil {
		t.Fatalf("Ready() error = %v", err)
	}

	if result == nil {
		t.Fatal("Ready() returned nil result")
	}

	if result.ID != "bead-123" {
		t.Errorf("ID = %q, want %q", result.ID, "bead-123")
	}

	if result.Title != "Test bead" {
		t.Errorf("Title = %q, want %q", result.Title, "Test bead")
	}

	if result.Priority != 1 {
		t.Errorf("Priority = %d, want 1", result.Priority)
	}

	if len(result.Labels) != 1 || result.Labels[0] != "spec:test" {
		t.Errorf("Labels = %v, want [spec:test]", result.Labels)
	}
}

// TestBeadClientShow_ReturnsTypedResult verifies BeadClient.Show returns *BeadInfo instead of interface{}
// Expected failure: BeadClient.Show() currently returns (interface{}, error) and BeadInfo type does not exist
func TestBeadClientShow_ReturnsTypedResult(t *testing.T) {
	mockBead := &typedInterfacesBeadClient{
		showFn: func(id string) (*BeadInfo, error) {
			return &BeadInfo{
				ID:       id,
				Title:    "Retrieved bead",
				Priority: 2,
				Labels:   []string{"complexity:high"},
			}, nil
		},
	}

	result, err := mockBead.Show("bead-456")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}

	if result == nil {
		t.Fatal("Show() returned nil result")
	}

	if result.ID != "bead-456" {
		t.Errorf("ID = %q, want %q", result.ID, "bead-456")
	}

	if result.Title != "Retrieved bead" {
		t.Errorf("Title = %q, want %q", result.Title, "Retrieved bead")
	}

	if result.Priority != 2 {
		t.Errorf("Priority = %d, want 2", result.Priority)
	}
}

// TestBeadClientCreate_ReturnsTypedResult verifies BeadClient.Create returns *BeadInfo instead of interface{}
// Expected failure: BeadClient.Create() currently returns (interface{}, error) and BeadInfo type does not exist
func TestBeadClientCreate_ReturnsTypedResult(t *testing.T) {
	mockBead := &typedInterfacesBeadClient{
		createFn: func(title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
			return &BeadInfo{
				ID:       "bead-new",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}, nil
		},
	}

	result, err := mockBead.Create("New bead", 0, []string{"spec:test"}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if result == nil {
		t.Fatal("Create() returned nil result")
	}

	if result.ID != "bead-new" {
		t.Errorf("ID = %q, want %q", result.ID, "bead-new")
	}

	if result.Title != "New bead" {
		t.Errorf("Title = %q, want %q", result.Title, "New bead")
	}
}

// TestBeadClientCreateWithDepsAndDescription_ReturnsTypedResult verifies CreateWithDepsAndDescription returns *BeadInfo
// Expected failure: CreateWithDepsAndDescription() currently returns (interface{}, error) and BeadInfo type does not exist
func TestBeadClientCreateWithDepsAndDescription_ReturnsTypedResult(t *testing.T) {
	mockBead := &typedInterfacesBeadClient{
		createWithDepsFn: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			return &BeadInfo{
				ID:       "bead-complex",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}, nil
		},
	}

	result, err := mockBead.CreateWithDepsAndDescription(
		"Complex bead",
		1,
		[]string{"spec:auth"},
		[]string{"Login works"},
		[]string{"bead-123"},
		"Full description",
	)
	if err != nil {
		t.Fatalf("CreateWithDepsAndDescription() error = %v", err)
	}

	if result == nil {
		t.Fatal("CreateWithDepsAndDescription() returned nil result")
	}

	if result.ID != "bead-complex" {
		t.Errorf("ID = %q, want %q", result.ID, "bead-complex")
	}

	if result.Title != "Complex bead" {
		t.Errorf("Title = %q, want %q", result.Title, "Complex bead")
	}

	if result.Priority != 1 {
		t.Errorf("Priority = %d, want 1", result.Priority)
	}
}

// TestDecompose_NoReflectionUsed verifies decompose.go does not import reflect package
// Expected failure: decompose.go currently imports "reflect" for extractBeadID function
func TestDecompose_NoReflectionUsed(t *testing.T) {
	decomposePath := filepath.Join("..", "..", "internal", "pipeline", "decompose.go")
	content, err := os.ReadFile(decomposePath)
	if err != nil {
		t.Fatalf("reading decompose.go: %v", err)
	}

	contentStr := string(content)

	// Check for reflect import
	if hasReflectImport(contentStr) {
		t.Error("decompose.go imports reflect package - should not need reflection after typed interfaces")
	}
}

// TestExtractBeadID_FunctionRemoved verifies extractBeadID function is deleted from decompose.go
// Expected failure: extractBeadID function currently exists in decompose.go at lines 207-239
func TestExtractBeadID_FunctionRemoved(t *testing.T) {
	decomposePath := filepath.Join("..", "..", "internal", "pipeline", "decompose.go")
	content, err := os.ReadFile(decomposePath)
	if err != nil {
		t.Fatalf("reading decompose.go: %v", err)
	}

	contentStr := string(content)

	// Check that extractBeadID function does not exist
	if hasFunction(contentStr, "extractBeadID") {
		t.Error("extractBeadID function still exists - should be removed after typed interfaces")
	}
}

// TestDecompose_NoTypeAssertions verifies decompose workflow does not use type assertions on bead results
// Expected failure: decompose.go currently uses type assertions like .(map[string]interface{}) and calls extractBeadID
func TestDecompose_NoTypeAssertions(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")

	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "test.md")
	planContent := "---\ncreated: 2026-02-14\n---\n# Test Plan\n\nTest content"
	if err := os.WriteFile(planPath, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	var beadCreationCalled bool
	mockClaude := &typedInterfacesClaudeClient{
		runFn: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output:   `[{"title": "Test", "description": "Desc", "priority": "P1", "acceptance_criteria": ["AC1"], "depends_on_index": []}]`,
			}, nil
		},
	}

	mockBead := &typedInterfacesBeadClient{
		createWithDepsFn: func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
			beadCreationCalled = true
			// Return typed struct directly, no map wrapping needed
			return &BeadInfo{
				ID:       "bead-typed",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}, nil
		},
	}

	deps := &Deps{
		ClaudeClient: mockClaude,
		BeadClient:   mockBead,
	}
	paths := &Paths{
		GromitDir: tmpDir,
		PlansDir:  plansDir,
	}

	p := New(deps, paths)

	result, err := p.Decompose(context.Background(), DecomposeInput{PlanName: "test"})
	if err != nil {
		t.Fatalf("Decompose() error = %v", err)
	}

	if !beadCreationCalled {
		t.Error("BeadClient.CreateWithDepsAndDescription was not called")
	}

	if len(result.CreatedBeads) != 1 {
		t.Errorf("CreatedBeads count = %d, want 1", len(result.CreatedBeads))
	}

	// Verify bead ID was extracted without type assertions
	if result.CreatedBeads[0].ID != "bead-typed" {
		t.Errorf("CreatedBeads[0].ID = %q, want %q", result.CreatedBeads[0].ID, "bead-typed")
	}
}

// TestPromptRenderer_ThoroughReview_TypedInput verifies RenderThoroughReview accepts typed ThoroughReviewPromptInput
// Expected failure: RenderThoroughReview currently accepts interface{} and ThoroughReviewPromptInput type does not exist
func TestPromptRenderer_ThoroughReview_TypedInput(t *testing.T) {
	mockRenderer := &typedInterfacesPromptRenderer{
		renderThoroughReviewFn: func(input *ThoroughReviewPromptInput) (string, error) {
			if input.FromCommit != "abc123" {
				return "", fmt.Errorf("unexpected FromCommit: %s", input.FromCommit)
			}
			if input.Diff != "test diff" {
				return "", fmt.Errorf("unexpected Diff: %s", input.Diff)
			}
			return "rendered prompt", nil
		},
	}

	input := &ThoroughReviewPromptInput{
		FromCommit: "abc123",
		Diff:       "test diff",
	}

	result, err := mockRenderer.RenderThoroughReview(input)
	if err != nil {
		t.Fatalf("RenderThoroughReview() error = %v", err)
	}

	if result != "rendered prompt" {
		t.Errorf("RenderThoroughReview() = %q, want %q", result, "rendered prompt")
	}
}

// TestLogWriter_AcceptsAny verifies LogWriter.Write accepts any parameter
// Per Decision 3: LogWriter remains as 'any' for fire-and-forget serialization
func TestLogWriter_AcceptsAny(t *testing.T) {
	var capturedEntry any
	mockLog := &typedInterfacesLogWriter{
		writeFn: func(entry any) error {
			capturedEntry = entry
			return nil
		},
	}

	entry := map[string]interface{}{
		"type":    "test",
		"bead_id": "bead-123",
	}

	err := mockLog.Write(entry)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if capturedEntry == nil {
		t.Fatal("LogWriter did not receive entry")
	}

	entryMap, ok := capturedEntry.(map[string]interface{})
	if !ok {
		t.Fatal("Entry is not a map")
	}

	if entryMap["type"] != "test" {
		t.Errorf("Entry type = %v, want %q", entryMap["type"], "test")
	}

	if entryMap["bead_id"] != "bead-123" {
		t.Errorf("Entry bead_id = %v, want %q", entryMap["bead_id"], "bead-123")
	}
}

// TestReviewNonInteractive_UsesTypedClaudeResult verifies ReviewNonInteractive works with typed ClaudeRunResult
// Expected failure: ReviewNonInteractive currently uses type assertions on interface{} result
func TestReviewNonInteractive_UsesTypedClaudeResult(t *testing.T) {
	tmpDir := t.TempDir()

	mockClaude := &typedInterfacesClaudeClient{
		runFn: func(prompt string, model string) (*ClaudeRunResult, error) {
			return &ClaudeRunResult{
				Success:  true,
				ExitCode: 0,
				Output:   `{"passed": true, "summary": "All good", "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "learnings": []}`,
			}, nil
		},
	}

	mockBead := &typedInterfacesBeadClient{}
	mockBacklog := &typedInterfacesBacklogClient{}
	mockLearnings := &typedInterfacesLearningsManager{}
	mockState := &typedInterfacesStateManager{}
	mockLog := &typedInterfacesLogWriter{
		writeFn: func(entry any) error {
			return nil
		},
	}
	mockRenderer := &typedInterfacesPromptRenderer{
		renderThoroughReviewFn: func(input *ThoroughReviewPromptInput) (string, error) {
			return "review prompt", nil
		},
	}

	deps := &Deps{
		ClaudeClient:     mockClaude,
		BeadClient:       mockBead,
		BacklogClient:    mockBacklog,
		LearningsManager: mockLearnings,
		StateManager:     mockState,
		LogWriter:        mockLog,
		PromptRenderer:   mockRenderer,
	}
	paths := &Paths{
		GromitDir: tmpDir,
	}

	p := New(deps, paths)

	input := ReviewInput{
		FromCommit: "abc123",
		Diff:       "test diff",
		Model:      "sonnet",
	}

	result, err := p.ReviewNonInteractive(context.Background(), input)
	if err != nil {
		t.Fatalf("ReviewNonInteractive() error = %v", err)
	}

	if !result.Passed {
		t.Error("Review should pass")
	}

	if result.Summary != "All good" {
		t.Errorf("Summary = %q, want %q", result.Summary, "All good")
	}
}

// Helper types for testing typed interfaces

type typedInterfacesClaudeClient struct {
	runFn func(prompt string, model string) (*ClaudeRunResult, error)
}

func (m *typedInterfacesClaudeClient) Run(prompt string, model string) (*ClaudeRunResult, error) {
	if m.runFn != nil {
		return m.runFn(prompt, model)
	}
	return nil, fmt.Errorf("not implemented")
}

type typedInterfacesBeadClient struct {
	readyFn          func() (*BeadInfo, error)
	showFn           func(id string) (*BeadInfo, error)
	createFn         func(title string, priority int, labels []string, outputs []string) (*BeadInfo, error)
	createWithDepsFn func(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error)
}

func (m *typedInterfacesBeadClient) Ready() (*BeadInfo, error) {
	if m.readyFn != nil {
		return m.readyFn()
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *typedInterfacesBeadClient) Show(id string) (*BeadInfo, error) {
	if m.showFn != nil {
		return m.showFn(id)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *typedInterfacesBeadClient) Create(title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
	if m.createFn != nil {
		return m.createFn(title, priority, labels, outputs)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *typedInterfacesBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
	if m.createWithDepsFn != nil {
		return m.createWithDepsFn(title, priority, labels, criteria, deps, desc)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *typedInterfacesBeadClient) Close(id string) error {
	return fmt.Errorf("not implemented")
}

type typedInterfacesPromptRenderer struct {
	renderThoroughReviewFn func(input *ThoroughReviewPromptInput) (string, error)
}

func (m *typedInterfacesPromptRenderer) RenderRefine(input *RefinePromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *typedInterfacesPromptRenderer) RenderPlan(input *PlanPromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *typedInterfacesPromptRenderer) RenderDecompose(input *DecomposePromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *typedInterfacesPromptRenderer) RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error) {
	if m.renderThoroughReviewFn != nil {
		return m.renderThoroughReviewFn(input)
	}
	return "", fmt.Errorf("not implemented")
}

func (m *typedInterfacesPromptRenderer) RenderExplore(input *ExplorePromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

type typedInterfacesLogWriter struct {
	writeFn func(entry any) error
}

func (m *typedInterfacesLogWriter) Write(entry any) error {
	if m.writeFn != nil {
		return m.writeFn(entry)
	}
	return fmt.Errorf("not implemented")
}

type typedInterfacesBacklogClient struct{}

func (m *typedInterfacesBacklogClient) List() ([]*Idea, error) {
	return nil, nil
}

func (m *typedInterfacesBacklogClient) Get(id string) (*Idea, error) {
	return nil, nil
}

func (m *typedInterfacesBacklogClient) Add(item *Idea) error {
	return nil
}

func (m *typedInterfacesBacklogClient) Update(id string, fn func(*Idea)) error {
	return nil
}

type typedInterfacesLearningsManager struct{}

func (m *typedInterfacesLearningsManager) Add(content string) error {
	return nil
}

type typedInterfacesStateManager struct{}

func (m *typedInterfacesStateManager) GetLastReviewCommit() (string, error) {
	return "", nil
}

func (m *typedInterfacesStateManager) SetLastReviewCommit(commit string) error {
	return nil
}

// Helper functions for file content analysis

func hasReflectImport(content string) bool {
	// Simple check for reflect import - look for import statement
	return containsImport(content, "reflect")
}

func containsImport(content, pkg string) bool {
	// Check for both single-line and multi-line import styles
	singleLine := fmt.Sprintf(`import "%s"`, pkg)
	multiLineQuoted := fmt.Sprintf(`"%s"`, pkg)

	return containsString(content, singleLine) || containsString(content, multiLineQuoted)
}

func hasFunction(content, funcName string) bool {
	// Check for function definition
	funcDef := fmt.Sprintf("func %s(", funcName)
	return containsString(content, funcDef)
}

func containsString(content, substr string) bool {
	return len(content) > 0 && len(substr) > 0 && indexString(content, substr) >= 0
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Ensure mocks compile against the new interface signatures
var _ ClaudeClient = (*typedInterfacesClaudeClient)(nil)
var _ BeadClient = (*typedInterfacesBeadClient)(nil)
var _ PromptRenderer = (*typedInterfacesPromptRenderer)(nil)
var _ LogWriter = (*typedInterfacesLogWriter)(nil)
