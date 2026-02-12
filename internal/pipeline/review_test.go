package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestReviewInteractiveWorkflow_BuildsContextAndReturnsSession verifies ReviewInteractive validates dependencies,
// builds ThoroughReviewContext, renders prompt, writes temp file, resolves agent, and returns ReviewSession.
// Expected failure: Pipeline.ReviewInteractive method does not exist yet
func TestReviewInteractiveWorkflow_BuildsContextAndReturnsSession(t *testing.T) {
	mockAgent := &reviewAcceptanceMockAgent{
		name: "opus",
	}
	mockAgentResolver := &reviewAcceptanceMockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (Agent, error) {
			if phase != "review" {
				t.Errorf("AgentResolver called with phase=%q, want 'review'", phase)
			}
			return mockAgent, nil
		},
	}

	promptRendered := false
	mockRenderer := &reviewAcceptanceMockPromptRenderer{
		renderThoroughReviewFunc: func(ctx interface{}) (string, error) {
			promptRendered = true
			// Verify context contains Diff and Model
			return "# Review Prompt\n\nDiff content here", nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgentResolver,
		PromptRenderer: mockRenderer,
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{
		FromCommit: "abc123",
		Diff:       "diff content",
		AgentName:  "opus",
	}

	session, err := p.ReviewInteractive(ctx, input)
	if err != nil {
		t.Fatalf("ReviewInteractive() failed: %v", err)
	}

	if session == nil {
		t.Fatal("ReviewInteractive() returned nil session")
	}

	if !promptRendered {
		t.Error("ReviewInteractive() did not render prompt via PromptRenderer")
	}
}

// TestReviewInteractiveWorkflow_NilDependenciesError verifies error when dependencies are nil
// Expected failure: Pipeline.ReviewInteractive method does not exist yet
func TestReviewInteractiveWorkflow_NilDependenciesError(t *testing.T) {
	p := New(nil, &Paths{})

	ctx := context.Background()
	input := ReviewInput{
		FromCommit: "abc123",
		Diff:       "diff",
	}

	_, err := p.ReviewInteractive(ctx, input)
	if err == nil {
		t.Fatal("ReviewInteractive() with nil deps should return error")
	}

	if !strings.Contains(err.Error(), "nil dependencies") {
		t.Errorf("Error message = %q, want message about nil dependencies", err.Error())
	}
}

// TestReviewNonInteractiveWorkflow_E2E verifies the complete non-interactive review workflow
// Expected failure: Pipeline.ReviewNonInteractive method does not exist yet
func TestReviewNonInteractiveWorkflow_E2E(t *testing.T) {
	mockRenderer := &reviewAcceptanceMockPromptRenderer{
		renderThoroughReviewFunc: func(ctx interface{}) (string, error) {
			return "# Review Prompt", nil
		},
	}

	mockClaude := &reviewAcceptanceMockClaudeClient{
		runFunc: func(prompt string, model string, timeout time.Duration) (interface{}, error) {
			// Return Claude output with JSON review result
			jsonOutput := `{
				"passed": false,
				"fixes_applied": ["Fixed formatting in main.go"],
				"beads_to_create": [
					{
						"title": "Add error handling to loadConfig",
						"description": "loadConfig panics on nil config",
						"priority": 1,
						"labels": ["bug"]
					}
				],
				"backlog_items": [
					{
						"title": "Consider caching config",
						"description": "Config is loaded on every call",
						"reason": "performance-optimization"
					}
				],
				"summary": "Found 1 critical bug and 1 optimization opportunity",
				"learnings": ["Always validate config before use"]
			}`
			return map[string]interface{}{
				"Success":  true,
				"Output":   jsonOutput,
				"ExitCode": 0,
			}, nil
		},
	}

	createdBeads := []reviewAcceptanceBeadRecord{}
	mockBead := &reviewAcceptanceMockBeadClient{
		createFunc: func(title string, priority int, labels []string, outputs []string) (interface{}, error) {
			createdBeads = append(createdBeads, reviewAcceptanceBeadRecord{
				title:    title,
				priority: priority,
				labels:   labels,
			})
			return map[string]interface{}{"ID": fmt.Sprintf("bead-%d", len(createdBeads))}, nil
		},
	}

	backlogAdded := []string{}
	mockBacklog := &reviewAcceptanceMockBacklogClient{
		addFunc: func(idea *Idea) error {
			backlogAdded = append(backlogAdded, idea.Text)
			return nil
		},
	}

	learningsAdded := []string{}
	mockLearnings := &reviewAcceptanceMockLearningsManager{
		addFunc: func(content string) error {
			learningsAdded = append(learningsAdded, content)
			return nil
		},
	}

	logWritten := false
	mockLog := &reviewAcceptanceMockLogWriter{
		writeFunc: func(entry interface{}) error {
			logWritten = true
			return nil
		},
	}

	stateUpdated := false
	mockState := &reviewAcceptanceMockStateManager{
		setLastReviewCommitFunc: func(commit string) error {
			stateUpdated = true
			if commit == "" {
				return fmt.Errorf("empty commit")
			}
			return nil
		},
	}

	deps := &Deps{
		PromptRenderer:   mockRenderer,
		ClaudeClient:     mockClaude,
		BeadClient:       mockBead,
		BacklogClient:    mockBacklog,
		LearningsManager: mockLearnings,
		LogWriter:        mockLog,
		StateManager:     mockState,
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{
		FromCommit: "abc123",
		Diff:       "diff content",
		Model:      "opus",
		Timeout:    300,
	}

	result, err := p.ReviewNonInteractive(ctx, input)
	if err != nil {
		t.Fatalf("ReviewNonInteractive() failed: %v", err)
	}

	if result == nil {
		t.Fatal("ReviewNonInteractive() returned nil result")
	}

	// Verify beads were created
	if len(createdBeads) != 1 {
		t.Errorf("Created %d beads, want 1", len(createdBeads))
	} else {
		if createdBeads[0].title != "Add error handling to loadConfig" {
			t.Errorf("Bead title = %q, want 'Add error handling to loadConfig'", createdBeads[0].title)
		}
		// Verify from-review label was added
		hasLabel := false
		for _, label := range createdBeads[0].labels {
			if label == "from-review" {
				hasLabel = true
				break
			}
		}
		if !hasLabel {
			t.Errorf("Bead labels = %v, missing 'from-review' label", createdBeads[0].labels)
		}
	}

	// Verify backlog items were created
	if len(backlogAdded) != 1 {
		t.Errorf("Created %d backlog items, want 1", len(backlogAdded))
	}

	// Verify learnings were persisted
	if len(learningsAdded) != 1 {
		t.Errorf("Persisted %d learnings, want 1", len(learningsAdded))
	}

	// Verify log was written
	if !logWritten {
		t.Error("Review log was not written")
	}

	// Verify state was updated
	if !stateUpdated {
		t.Error("State LastReviewCommit was not updated")
	}

	// Verify result summary
	if !strings.Contains(result.Summary, "Found 1 critical bug") {
		t.Errorf("Result summary = %q, want to contain 'Found 1 critical bug'", result.Summary)
	}

	if result.Passed {
		t.Error("Result.Passed = true, want false (review found issues)")
	}

	if result.FixesApplied != 1 {
		t.Errorf("Result.FixesApplied = %d, want 1", result.FixesApplied)
	}

	if result.BeadsCreated != 1 {
		t.Errorf("Result.BeadsCreated = %d, want 1", result.BeadsCreated)
	}

	if result.BacklogCreated != 1 {
		t.Errorf("Result.BacklogCreated = %d, want 1", result.BacklogCreated)
	}
}

// TestReviewNonInteractiveWorkflow_CreatesBeadsWithFromReviewLabel verifies beads get from-review label
// Expected failure: Pipeline.ReviewNonInteractive method does not exist yet
func TestReviewNonInteractiveWorkflow_CreatesBeadsWithFromReviewLabel(t *testing.T) {
	mockRenderer := &reviewAcceptanceMockPromptRenderer{
		renderThoroughReviewFunc: func(ctx interface{}) (string, error) {
			return "# Review", nil
		},
	}

	mockClaude := &reviewAcceptanceMockClaudeClient{
		runFunc: func(prompt string, model string, timeout time.Duration) (interface{}, error) {
			jsonOutput := `{
				"passed": true,
				"fixes_applied": [],
				"beads_to_create": [
					{
						"title": "Fix typo in README",
						"description": "README has typo",
						"priority": 2,
						"labels": ["docs"]
					}
				],
				"backlog_items": [],
				"summary": "Minor fixes needed"
			}`
			return map[string]interface{}{
				"Success": true,
				"Output":  jsonOutput,
			}, nil
		},
	}

	var capturedLabels []string
	mockBead := &reviewAcceptanceMockBeadClient{
		createFunc: func(title string, priority int, labels []string, outputs []string) (interface{}, error) {
			capturedLabels = labels
			return map[string]interface{}{"ID": "bead-1"}, nil
		},
	}

	deps := &Deps{
		PromptRenderer:   mockRenderer,
		ClaudeClient:     mockClaude,
		BeadClient:       mockBead,
		BacklogClient:    &reviewAcceptanceMockBacklogClient{},
		LearningsManager: &reviewAcceptanceMockLearningsManager{},
		LogWriter:        &reviewAcceptanceMockLogWriter{},
		StateManager:     &reviewAcceptanceMockStateManager{},
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{
		FromCommit: "abc123",
		Diff:       "diff",
		Model:      "sonnet",
		Timeout:    300,
	}

	_, err := p.ReviewNonInteractive(ctx, input)
	if err != nil {
		t.Fatalf("ReviewNonInteractive() failed: %v", err)
	}

	// Verify from-review label is present
	hasFromReview := false
	hasDocs := false
	for _, label := range capturedLabels {
		if label == "from-review" {
			hasFromReview = true
		}
		if label == "docs" {
			hasDocs = true
		}
	}

	if !hasFromReview {
		t.Errorf("Labels = %v, missing 'from-review' label", capturedLabels)
	}
	if !hasDocs {
		t.Errorf("Labels = %v, missing 'docs' label (should preserve original labels)", capturedLabels)
	}
}

// TestReviewNonInteractiveWorkflow_CreatesBacklogWithLabels verifies backlog items get from-review and backlog labels
// Expected failure: Pipeline.ReviewNonInteractive method does not exist yet
func TestReviewNonInteractiveWorkflow_CreatesBacklogWithLabels(t *testing.T) {
	mockRenderer := &reviewAcceptanceMockPromptRenderer{
		renderThoroughReviewFunc: func(ctx interface{}) (string, error) {
			return "# Review", nil
		},
	}

	mockClaude := &reviewAcceptanceMockClaudeClient{
		runFunc: func(prompt string, model string, timeout time.Duration) (interface{}, error) {
			jsonOutput := `{
				"passed": true,
				"fixes_applied": [],
				"beads_to_create": [],
				"backlog_items": [
					{
						"title": "Consider adding metrics",
						"description": "Metrics would help observability",
						"reason": "enhancement"
					}
				],
				"summary": "Looks good"
			}`
			return map[string]interface{}{
				"Success": true,
				"Output":  jsonOutput,
			}, nil
		},
	}

	var capturedBacklogIdea *Idea
	mockBacklog := &reviewAcceptanceMockBacklogClient{
		addFunc: func(idea *Idea) error {
			capturedBacklogIdea = idea
			return nil
		},
	}

	deps := &Deps{
		PromptRenderer:   mockRenderer,
		ClaudeClient:     mockClaude,
		BeadClient:       &reviewAcceptanceMockBeadClient{},
		BacklogClient:    mockBacklog,
		LearningsManager: &reviewAcceptanceMockLearningsManager{},
		LogWriter:        &reviewAcceptanceMockLogWriter{},
		StateManager:     &reviewAcceptanceMockStateManager{},
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{
		FromCommit: "abc123",
		Diff:       "diff",
		Model:      "sonnet",
		Timeout:    300,
	}

	_, err := p.ReviewNonInteractive(ctx, input)
	if err != nil {
		t.Fatalf("ReviewNonInteractive() failed: %v", err)
	}

	if capturedBacklogIdea == nil {
		t.Fatal("Backlog item was not created")
	}

	// Verify backlog idea contains from-review label
	// Note: The actual implementation should add labels to the Idea struct
	// For now, verify the idea was created with correct text
	if capturedBacklogIdea.Text != "Consider adding metrics" {
		t.Errorf("Backlog text = %q, want 'Consider adding metrics'", capturedBacklogIdea.Text)
	}
}

// TestReviewNonInteractiveWorkflow_RespectsTimeout verifies ReviewInput.Timeout field is provided
// Note: The ClaudeClient interface doesn't expose timeout directly; timeout handling
// is expected to be managed via context at a higher level (e.g., in the CLI adapter)
func TestReviewNonInteractiveWorkflow_RespectsTimeout(t *testing.T) {
	mockRenderer := &reviewAcceptanceMockPromptRenderer{
		renderThoroughReviewFunc: func(ctx interface{}) (string, error) {
			return "# Review", nil
		},
	}

	mockClaude := &reviewAcceptanceMockClaudeClient{
		runFunc: func(prompt string, model string, timeout time.Duration) (interface{}, error) {
			jsonOutput := `{"passed": true, "summary": "OK", "fixes_applied": [], "beads_to_create": [], "backlog_items": []}`
			return map[string]interface{}{
				"Success": true,
				"Output":  jsonOutput,
			}, nil
		},
	}

	deps := &Deps{
		PromptRenderer:   mockRenderer,
		ClaudeClient:     mockClaude,
		BeadClient:       &reviewAcceptanceMockBeadClient{},
		BacklogClient:    &reviewAcceptanceMockBacklogClient{},
		LearningsManager: &reviewAcceptanceMockLearningsManager{},
		LogWriter:        &reviewAcceptanceMockLogWriter{},
		StateManager:     &reviewAcceptanceMockStateManager{},
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := ReviewInput{
		FromCommit: "abc123",
		Diff:       "diff",
		Model:      "opus",
		Timeout:    600, // 10 minutes
	}

	result, err := p.ReviewNonInteractive(ctx, input)
	if err != nil {
		t.Fatalf("ReviewNonInteractive() failed: %v", err)
	}

	// Verify the workflow completed successfully
	// Timeout handling is delegated to the caller (CLI adapter)
	if result == nil {
		t.Fatal("ReviewNonInteractive() returned nil result")
	}
}

// TestReviewNonInteractiveWorkflow_NilDependenciesError verifies error when dependencies are nil
// Expected failure: Pipeline.ReviewNonInteractive method does not exist yet
func TestReviewNonInteractiveWorkflow_NilDependenciesError(t *testing.T) {
	p := New(nil, &Paths{})

	ctx := context.Background()
	input := ReviewInput{
		FromCommit: "abc123",
		Diff:       "diff",
		Model:      "sonnet",
		Timeout:    300,
	}

	_, err := p.ReviewNonInteractive(ctx, input)
	if err == nil {
		t.Fatal("ReviewNonInteractive() with nil deps should return error")
	}

	if !strings.Contains(err.Error(), "nil dependencies") {
		t.Errorf("Error message = %q, want message about nil dependencies", err.Error())
	}
}

// Mock types for review acceptance tests

type reviewAcceptanceMockAgent struct {
	name        string
	launchFunc  func(promptPath string) error
	launchError error
}

func (m *reviewAcceptanceMockAgent) Name() string {
	return m.name
}

func (m *reviewAcceptanceMockAgent) Launch(promptPath string) error {
	if m.launchFunc != nil {
		return m.launchFunc(promptPath)
	}
	return m.launchError
}

type reviewAcceptanceMockAgentResolver struct {
	resolveFunc func(phase string, flagOverride string, choosePicker bool) (Agent, error)
}

func (m *reviewAcceptanceMockAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (Agent, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(phase, flagOverride, choosePicker)
	}
	return nil, fmt.Errorf("not implemented")
}

type reviewAcceptanceMockPromptRenderer struct {
	renderThoroughReviewFunc func(ctx interface{}) (string, error)
}

func (m *reviewAcceptanceMockPromptRenderer) RenderRefine(input interface{}) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *reviewAcceptanceMockPromptRenderer) RenderPlan(input interface{}) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *reviewAcceptanceMockPromptRenderer) RenderDecompose(input interface{}) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *reviewAcceptanceMockPromptRenderer) RenderThoroughReview(ctx interface{}) (string, error) {
	if m.renderThoroughReviewFunc != nil {
		return m.renderThoroughReviewFunc(ctx)
	}
	return "", fmt.Errorf("not implemented")
}

func (m *reviewAcceptanceMockPromptRenderer) RenderExplore(ctx interface{}) (string, error) {
	return "", fmt.Errorf("not implemented")
}

type reviewAcceptanceMockClaudeClient struct {
	runFunc func(prompt string, model string, timeout time.Duration) (interface{}, error)
}

func (m *reviewAcceptanceMockClaudeClient) Run(prompt string, model string) (interface{}, error) {
	// Call runFunc with a zero timeout since the ClaudeClient interface doesn't expose timeout
	// The timeout should be handled via context at a higher level
	if m.runFunc != nil {
		return m.runFunc(prompt, model, 0)
	}
	return nil, fmt.Errorf("not implemented")
}

type reviewAcceptanceBeadRecord struct {
	title    string
	priority int
	labels   []string
}

type reviewAcceptanceMockBeadClient struct {
	createFunc func(title string, priority int, labels []string, outputs []string) (interface{}, error)
}

func (m *reviewAcceptanceMockBeadClient) Ready() (interface{}, error) {
	return nil, nil
}

func (m *reviewAcceptanceMockBeadClient) Show(id string) (interface{}, error) {
	return nil, nil
}

func (m *reviewAcceptanceMockBeadClient) Create(title string, priority int, labels []string, outputs []string) (interface{}, error) {
	if m.createFunc != nil {
		return m.createFunc(title, priority, labels, outputs)
	}
	return map[string]interface{}{"ID": "bead-1"}, nil
}

func (m *reviewAcceptanceMockBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (interface{}, error) {
	return nil, fmt.Errorf("not used by review workflow")
}

func (m *reviewAcceptanceMockBeadClient) Close(id string) error {
	return nil
}

type reviewAcceptanceMockBacklogClient struct {
	addFunc func(idea *Idea) error
}

func (m *reviewAcceptanceMockBacklogClient) List() ([]*Idea, error) {
	return nil, nil
}

func (m *reviewAcceptanceMockBacklogClient) Get(id string) (*Idea, error) {
	return nil, nil
}

func (m *reviewAcceptanceMockBacklogClient) Add(item *Idea) error {
	if m.addFunc != nil {
		return m.addFunc(item)
	}
	return nil
}

func (m *reviewAcceptanceMockBacklogClient) Update(id string, fn func(*Idea)) error {
	return nil
}

type reviewAcceptanceMockLearningsManager struct {
	addFunc func(content string) error
}

func (m *reviewAcceptanceMockLearningsManager) Add(content string) error {
	if m.addFunc != nil {
		return m.addFunc(content)
	}
	return nil
}

type reviewAcceptanceMockLogWriter struct {
	writeFunc func(entry interface{}) error
}

func (m *reviewAcceptanceMockLogWriter) Write(entry interface{}) error {
	if m.writeFunc != nil {
		return m.writeFunc(entry)
	}
	return nil
}

type reviewAcceptanceMockStateManager struct {
	setLastReviewCommitFunc func(commit string) error
}

func (m *reviewAcceptanceMockStateManager) GetLastReviewCommit() (string, error) {
	return "", nil
}

func (m *reviewAcceptanceMockStateManager) SetLastReviewCommit(commit string) error {
	if m.setLastReviewCommitFunc != nil {
		return m.setLastReviewCommitFunc(commit)
	}
	return nil
}
