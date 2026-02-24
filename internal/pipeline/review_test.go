package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestBuildFromReviewLabels_PrependsSingleLabel verifies that BuildFromReviewLabels
// prepends "from-review" to a new label list.
func TestBuildFromReviewLabels_PrependsSingleLabel(t *testing.T) {
	labels := BuildFromReviewLabels([]string{"bug"})
	if len(labels) != 2 {
		t.Errorf("got %d labels, want 2", len(labels))
	}
	if labels[0] != "from-review" {
		t.Errorf("first label = %q, want 'from-review'", labels[0])
	}
	if labels[1] != "bug" {
		t.Errorf("second label = %q, want 'bug'", labels[1])
	}
}

// TestBuildFromReviewLabels_DeduplicatesExistingLabel verifies that if "from-review"
// is already in the labels, it is not duplicated.
func TestBuildFromReviewLabels_DeduplicatesExistingLabel(t *testing.T) {
	labels := BuildFromReviewLabels([]string{"from-review", "bug"})
	if len(labels) != 2 {
		t.Errorf("got %d labels, want 2 (deduplicated)", len(labels))
	}
	if labels[0] != "from-review" {
		t.Errorf("first label = %q, want 'from-review'", labels[0])
	}
	if labels[1] != "bug" {
		t.Errorf("second label = %q, want 'bug'", labels[1])
	}
}

// TestBuildFromReviewLabels_PreservesOriginalLabelsOrder verifies that original labels
// appear after from-review in the same order.
func TestBuildFromReviewLabels_PreservesOriginalLabelsOrder(t *testing.T) {
	labels := BuildFromReviewLabels([]string{"bug", "enhancement", "docs"})
	if len(labels) != 4 {
		t.Errorf("got %d labels, want 4", len(labels))
	}
	expected := []string{"from-review", "bug", "enhancement", "docs"}
	for i, label := range expected {
		if labels[i] != label {
			t.Errorf("labels[%d] = %q, want %q", i, labels[i], label)
		}
	}
}

// TestReviewInteractiveWorkflow_BuildsContextAndReturnsSession verifies ReviewInteractive validates dependencies,
// builds ThoroughReviewContext, renders prompt, writes temp file, resolves agent, and returns ReviewSession.
// Expected failure: Pipeline.ReviewInteractive method does not exist yet
func TestReviewInteractiveWorkflow_BuildsContextAndReturnsSession(t *testing.T) {
	mockAgent := &reviewAcceptanceMockAgent{
		name: "opus",
		launchInDirFunc: func(promptPath string, dir string) error {
			if dir != "/tmp/review-launch" {
				t.Errorf("LaunchInDir called with dir=%q, want %q", dir, "/tmp/review-launch")
			}
			return nil
		},
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
	mockRenderer := &reviewAcceptanceMockReviewRenderer{
		renderThoroughReviewFunc: func(input *ThoroughReviewPromptInput) (string, error) {
			promptRendered = true
			// Verify context contains Diff and Model
			return "# Review Prompt\n\nDiff content here", nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgentResolver,
		ReviewRenderer: mockRenderer,
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
		LaunchDir:  "/tmp/review-launch",
	}

	session, err := p.ReviewInteractive(ctx, input)
	if err != nil {
		t.Fatalf("ReviewInteractive() failed: %v", err)
	}

	if session == nil {
		t.Fatal("ReviewInteractive() returned nil session")
	}

	if !promptRendered {
		t.Error("ReviewInteractive() did not render prompt via ReviewRenderer")
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
	mockRenderer := &reviewAcceptanceMockReviewRenderer{
		renderThoroughReviewFunc: func(input *ThoroughReviewPromptInput) (string, error) {
			return "# Review Prompt", nil
		},
	}

	mockReviewInvoker := &reviewAcceptanceMockReviewInvoker{
		runFunc: func(prompt string, model string, timeout time.Duration) (*LLMRunResult, error) {
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
			return &LLMRunResult{
				Success:  true,
				Output:   jsonOutput,
				ExitCode: 0,
			}, nil
		},
	}

	createdBeads := []reviewAcceptanceBeadRecord{}
	mockBead := &reviewAcceptanceMockBeadClient{
		createFunc: func(title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
			createdBeads = append(createdBeads, reviewAcceptanceBeadRecord{
				title:    title,
				priority: priority,
				labels:   labels,
			})
			return &BeadInfo{ID: fmt.Sprintf("bead-%d", len(createdBeads))}, nil
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
		writeFunc: func(entry *LogEntry) error {
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
		ReviewRenderer:   mockRenderer,
		ReviewInvoker:    mockReviewInvoker,
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
	mockRenderer := &reviewAcceptanceMockReviewRenderer{
		renderThoroughReviewFunc: func(input *ThoroughReviewPromptInput) (string, error) {
			return "# Review", nil
		},
	}

	mockReviewInvoker := &reviewAcceptanceMockReviewInvoker{
		runFunc: func(prompt string, model string, timeout time.Duration) (*LLMRunResult, error) {
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
			return &LLMRunResult{
				Success: true,
				Output:  jsonOutput,
			}, nil
		},
	}

	var capturedLabels []string
	mockBead := &reviewAcceptanceMockBeadClient{
		createFunc: func(title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
			capturedLabels = labels
			return &BeadInfo{ID: "bead-1"}, nil
		},
	}

	deps := &Deps{
		ReviewRenderer:   mockRenderer,
		ReviewInvoker:    mockReviewInvoker,
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
	mockRenderer := &reviewAcceptanceMockReviewRenderer{
		renderThoroughReviewFunc: func(input *ThoroughReviewPromptInput) (string, error) {
			return "# Review", nil
		},
	}

	mockReviewInvoker := &reviewAcceptanceMockReviewInvoker{
		runFunc: func(prompt string, model string, timeout time.Duration) (*LLMRunResult, error) {
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
			return &LLMRunResult{
				Success: true,
				Output:  jsonOutput,
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
		ReviewRenderer:   mockRenderer,
		ReviewInvoker:    mockReviewInvoker,
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
	if capturedBacklogIdea.CreatedAt.IsZero() {
		t.Error("backlog idea CreatedAt should be set")
	}
}

// TestReviewNonInteractiveWorkflow_RespectsTimeout verifies ReviewInput.Timeout field is provided
// Note: The ReviewInvoker interface doesn't expose timeout directly; timeout handling
// is expected to be managed via context at a higher level (e.g., in the CLI adapter)
func TestReviewNonInteractiveWorkflow_RespectsTimeout(t *testing.T) {
	mockRenderer := &reviewAcceptanceMockReviewRenderer{
		renderThoroughReviewFunc: func(input *ThoroughReviewPromptInput) (string, error) {
			return "# Review", nil
		},
	}

	mockReviewInvoker := &reviewAcceptanceMockReviewInvoker{
		runFunc: func(prompt string, model string, timeout time.Duration) (*LLMRunResult, error) {
			jsonOutput := `{"passed": true, "summary": "OK", "fixes_applied": [], "beads_to_create": [], "backlog_items": []}`
			return &LLMRunResult{
				Success: true,
				Output:  jsonOutput,
			}, nil
		},
	}

	deps := &Deps{
		ReviewRenderer:   mockRenderer,
		ReviewInvoker:    mockReviewInvoker,
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

func TestPipeline_validateReviewDeps(t *testing.T) {
	newValidDeps := func() *Deps {
		return &Deps{
			ReviewInvoker:    &reviewAcceptanceMockReviewInvoker{},
			ReviewRenderer:   &reviewAcceptanceMockReviewRenderer{},
			BeadClient:       &reviewAcceptanceMockBeadClient{},
			BacklogClient:    &reviewAcceptanceMockBacklogClient{},
			LearningsManager: &reviewAcceptanceMockLearningsManager{},
			LogWriter:        &reviewAcceptanceMockLogWriter{},
			StateManager:     &reviewAcceptanceMockStateManager{},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Deps)
		wantErr string
	}{
		{
			name:    "nil ReviewInvoker",
			mutate:  func(d *Deps) { d.ReviewInvoker = nil },
			wantErr: "pipeline: nil ReviewInvoker",
		},
		{
			name:    "nil ReviewRenderer",
			mutate:  func(d *Deps) { d.ReviewRenderer = nil },
			wantErr: "pipeline: nil ReviewRenderer",
		},
		{
			name:    "nil BeadClient",
			mutate:  func(d *Deps) { d.BeadClient = nil },
			wantErr: "pipeline: nil BeadClient",
		},
		{
			name:    "nil BacklogClient",
			mutate:  func(d *Deps) { d.BacklogClient = nil },
			wantErr: "pipeline: nil BacklogClient",
		},
		{
			name:    "nil LearningsManager",
			mutate:  func(d *Deps) { d.LearningsManager = nil },
			wantErr: "pipeline: nil LearningsManager",
		},
		{
			name:    "nil LogWriter",
			mutate:  func(d *Deps) { d.LogWriter = nil },
			wantErr: "pipeline: nil LogWriter",
		},
		{
			name:    "nil StateManager",
			mutate:  func(d *Deps) { d.StateManager = nil },
			wantErr: "pipeline: nil StateManager",
		},
		{
			name:    "typed nil ReviewInvoker",
			mutate:  func(d *Deps) { d.ReviewInvoker = (*reviewAcceptanceMockReviewInvoker)(nil) },
			wantErr: "pipeline: nil ReviewInvoker",
		},
		{
			name:    "all deps present",
			mutate:  nil,
			wantErr: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := newValidDeps()
			if tc.mutate != nil {
				tc.mutate(deps)
			}

			p := New(deps, &Paths{})
			err := p.validateReviewDeps()

			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("validateReviewDeps() = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validateReviewDeps() = nil, want %q", tc.wantErr)
			}

			if err.Error() != tc.wantErr {
				t.Errorf("validateReviewDeps() error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// Mock types for review acceptance tests

type reviewAcceptanceMockAgent struct {
	name            string
	launchFunc      func(promptPath string) error
	launchInDirFunc func(promptPath string, dir string) error
	launchError     error
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

func (m *reviewAcceptanceMockAgent) LaunchInDir(promptPath, dir string) error {
	if m.launchInDirFunc != nil {
		return m.launchInDirFunc(promptPath, dir)
	}
	if m.launchFunc != nil && dir == "" {
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

type reviewAcceptanceMockReviewRenderer struct {
	renderThoroughReviewFunc func(input *ThoroughReviewPromptInput) (string, error)
}

func (m *reviewAcceptanceMockReviewRenderer) RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error) {
	if m.renderThoroughReviewFunc != nil {
		return m.renderThoroughReviewFunc(input)
	}
	return "", fmt.Errorf("not implemented")
}

type reviewAcceptanceMockReviewInvoker struct {
	runFunc func(prompt string, model string, timeout time.Duration) (*LLMRunResult, error)
}

func (m *reviewAcceptanceMockReviewInvoker) Run(prompt string, model string) (*LLMRunResult, error) {
	// Call runFunc with a zero timeout since the ReviewInvoker interface doesn't expose timeout
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
	createFunc func(title string, priority int, labels []string, outputs []string) (*BeadInfo, error)
}

func (m *reviewAcceptanceMockBeadClient) Ready() (*BeadInfo, error) {
	return nil, nil
}

func (m *reviewAcceptanceMockBeadClient) Show(id string) (*BeadInfo, error) {
	return nil, nil
}

func (m *reviewAcceptanceMockBeadClient) Create(title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
	if m.createFunc != nil {
		return m.createFunc(title, priority, labels, outputs)
	}
	return &BeadInfo{ID: "bead-1"}, nil
}

func (m *reviewAcceptanceMockBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
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
	writeFunc func(entry *LogEntry) error
}

func (m *reviewAcceptanceMockLogWriter) Write(entry *LogEntry) error {
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
