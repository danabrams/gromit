//go:build acceptance

package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRefineWorkflowEndToEndWithBacklogItem verifies complete Refine workflow
// from backlog item to spec creation with all post-processing steps.
// Expected failure: RefineSession does not have a Result() method yet - sessions currently
// only provide event streams but don't capture structured results from post-processing.
func TestRefineWorkflowEndToEndWithBacklogItem(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	backlogDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Track what backlog updates occur
	var updatedItemID string
	mockBacklog := &mockBacklogClient{
		getFunc: func(id string) (interface{}, error) {
			return map[string]interface{}{
				"id":      id,
				"text":    "Add user authentication system",
				"status":  "open",
				"context": "Need OAuth2 and JWT support",
			}, nil
		},
		updateFunc: func(id string, fn func(interface{})) error {
			updatedItemID = id
			return nil
		},
	}

	// Agent creates a spec during session
	createdSpecName := "authentication.md"
	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return &testSessionAgent{
				onLaunch: func() {
					specPath := filepath.Join(specsDir, createdSpecName)
					specContent := `---
id: authentication
created: 2026-02-12
---

# User Authentication System

## Overview
OAuth2 and JWT-based authentication.
`
					os.WriteFile(specPath, []byte(specContent), 0644)
				},
			}, nil
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

	// Execute workflow
	ctx := context.Background()
	input := RefineInput{
		IdeaID: "idea-auth-123",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Drain events
	go func() {
		for range session.Events() {
		}
	}()

	if err := session.Wait(); err != nil {
		t.Fatalf("Session.Wait() failed: %v", err)
	}

	// Get structured results from post-processing
	result, err := session.Result()
	if err != nil {
		t.Fatalf("Result() failed: %v", err)
	}

	// Verify: spec was detected in post-processing
	if len(result.CreatedSpecs) != 1 {
		t.Errorf("CreatedSpecs = %d, want 1", len(result.CreatedSpecs))
	} else {
		specPath := result.CreatedSpecs[0]
		if filepath.Base(specPath) != createdSpecName {
			t.Errorf("Created spec basename = %s, want %s", filepath.Base(specPath), createdSpecName)
		}
	}

	// Verify: backlog item was updated in post-processing
	if len(result.RefinedItems) != 1 {
		t.Errorf("RefinedItems = %d, want 1", len(result.RefinedItems))
	} else if result.RefinedItems[0] != "idea-auth-123" {
		t.Errorf("RefinedItems[0] = %s, want idea-auth-123", result.RefinedItems[0])
	}

	// Verify backlog update was called
	if updatedItemID != "idea-auth-123" {
		t.Errorf("Backlog update called with ID = %s, want idea-auth-123", updatedItemID)
	}
}

// TestRefineWorkflowEventStream verifies session produces events during execution.
// Expected failure: RefineSession does not emit EventSessionStarted and EventSessionEnded
// events yet - the event stream mechanism is not implemented in Refine().
func TestRefineWorkflowEventStream(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return &testSessionAgent{
				onLaunch: func() {
					time.Sleep(50 * time.Millisecond)
				},
			}, nil
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
		IdeaText: "Test idea",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Collect events
	var events []Event
	done := make(chan struct{})
	go func() {
		for event := range session.Events() {
			events = append(events, event)
		}
		close(done)
	}()

	_ = session.Wait()
	<-done

	// Verify we got start and end events
	var hasStart, hasEnd bool
	for _, event := range events {
		if event.Type == EventSessionStarted {
			hasStart = true
		}
		if event.Type == EventSessionEnded {
			hasEnd = true
		}
	}

	if !hasStart {
		t.Error("Event stream did not include EventSessionStarted")
	}
	if !hasEnd {
		t.Error("Event stream did not include EventSessionEnded")
	}
}

// TestRefineWorkflowPostProcessingDetectsMultipleSpecs verifies post-processing
// correctly identifies all new specs created during session.
// Expected failure: Refine() does not implement pre-session snapshot and post-session
// diff logic to detect multiple new spec files yet.
func TestRefineWorkflowPostProcessingDetectsMultipleSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing spec
	existingSpec := filepath.Join(specsDir, "existing.md")
	if err := os.WriteFile(existingSpec, []byte("# Existing"), 0644); err != nil {
		t.Fatal(err)
	}

	// Agent creates multiple new specs
	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return &testSessionAgent{
				onLaunch: func() {
					os.WriteFile(filepath.Join(specsDir, "new1.md"), []byte("# New 1"), 0644)
					os.WriteFile(filepath.Join(specsDir, "new2.md"), []byte("# New 2"), 0644)
					os.WriteFile(filepath.Join(specsDir, "new3.md"), []byte("# New 3"), 0644)
				},
			}, nil
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
		IdeaText: "Multi-spec idea",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	go func() {
		for range session.Events() {
		}
	}()

	_ = session.Wait()

	result, err := session.Result()
	if err != nil {
		t.Fatalf("Result() failed: %v", err)
	}

	// Should detect exactly 3 new specs (not existing.md)
	if len(result.CreatedSpecs) != 3 {
		t.Errorf("CreatedSpecs count = %d, want 3 (only new specs)", len(result.CreatedSpecs))
	}

	// Verify no existing spec in results
	for _, spec := range result.CreatedSpecs {
		if filepath.Base(spec) == "existing.md" {
			t.Error("CreatedSpecs includes existing.md, should only contain newly created specs")
		}
	}
}

// TestRefineWorkflowBlankSessionCreatesBacklogItemWithSpecTitle verifies blank
// refinement sessions automatically create backlog items using spec title.
// Expected failure: Refine() does not extract spec title from created spec file and
// create backlog item in post-processing for blank sessions yet.
func TestRefineWorkflowBlankSessionCreatesBacklogItemWithSpecTitle(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	backlogDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backlogDir, 0755); err != nil {
		t.Fatal(err)
	}

	var addedItem map[string]interface{}
	mockBacklog := &mockBacklogClient{
		addFunc: func(item interface{}) error {
			addedItem = item.(map[string]interface{})
			return nil
		},
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return &testSessionAgent{
				onLaunch: func() {
					specContent := `---
id: payment-processing
created: 2026-02-12
---

# Payment Processing System

Integrate Stripe for payment handling.
`
					os.WriteFile(filepath.Join(specsDir, "payment-processing.md"), []byte(specContent), 0644)
				},
			}, nil
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

	// Blank session: no IdeaID or IdeaText
	ctx := context.Background()
	input := RefineInput{}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	go func() {
		for range session.Events() {
		}
	}()

	_ = session.Wait()

	// Verify backlog item was created
	if addedItem == nil {
		t.Fatal("No backlog item added for blank session with spec creation")
	}

	// Verify item has spec title as text
	if text, ok := addedItem["text"].(string); !ok || text != "Payment Processing System" {
		t.Errorf("Backlog item text = %q, want %q (extracted from spec title)", addedItem["text"], "Payment Processing System")
	}

	// Verify item links to spec
	if specName, ok := addedItem["spec_name"].(string); !ok || specName != "payment-processing" {
		t.Errorf("Backlog item spec_name = %q, want %q", addedItem["spec_name"], "payment-processing")
	}

	// Verify status is refined
	if status, ok := addedItem["status"].(string); !ok || status != "refined" {
		t.Errorf("Backlog item status = %q, want refined", addedItem["status"])
	}
}

// TestRefineWorkflowSystemPromptBuilding verifies system prompt includes idea text,
// specs directory, and embedded skill content.
// Expected failure: Refine() does not build system prompt with correct structure yet -
// PromptRenderer.RenderRefine() is not called with proper context including specs dir.
func TestRefineWorkflowSystemPromptBuilding(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	var renderedInput interface{}
	mockRenderer := &mockPromptRenderer{
		renderRefineFunc: func(input interface{}) (string, error) {
			renderedInput = input
			return "rendered prompt", nil
		},
	}

	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return &testSessionAgent{}, nil
		},
	}

	deps := &Deps{
		AgentResolver:  mockAgent,
		PromptRenderer: mockRenderer,
	}
	paths := &Paths{
		SpecsDir: specsDir,
	}

	p := New(deps, paths)

	ctx := context.Background()
	input := RefineInput{
		IdeaText: "Build notification system with email and SMS",
	}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	go func() {
		for range session.Events() {
		}
	}()

	_ = session.Wait()

	// Verify renderer was called
	if renderedInput == nil {
		t.Fatal("PromptRenderer.RenderRefine() was not called")
	}

	// Verify input contains required fields
	inputMap, ok := renderedInput.(map[string]interface{})
	if !ok {
		t.Fatalf("Render input is not a map, got type %T", renderedInput)
	}

	if ideaText, ok := inputMap["idea_text"].(string); !ok || ideaText != "Build notification system with email and SMS" {
		t.Errorf("Render input idea_text = %q, want the idea text from RefineInput", inputMap["idea_text"])
	}

	if specsPath, ok := inputMap["specs_dir"].(string); !ok || specsPath != specsDir {
		t.Errorf("Render input specs_dir = %q, want %q", inputMap["specs_dir"], specsDir)
	}
}

// TestRefineWorkflowTempPromptFileCleanup verifies temp prompt file is cleaned up after session.
// Expected failure: Refine() does not implement defer-based cleanup of temp prompt file yet.
func TestRefineWorkflowTempPromptFileCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	var promptFilePath string
	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			return &capturePathAgent{
				onLaunch: func(path string) {
					promptFilePath = path
				},
			}, nil
		},
	}

	mockRenderer := &mockPromptRenderer{
		renderRefineFunc: func(input interface{}) (string, error) {
			return "test prompt content", nil
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
	input := RefineInput{IdeaText: "test"}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	go func() {
		for range session.Events() {
		}
	}()

	_ = session.Wait()

	// Verify prompt file was created during session
	if promptFilePath == "" {
		t.Fatal("No prompt file was created")
	}

	// Verify prompt file is cleaned up after session
	if _, err := os.Stat(promptFilePath); !os.IsNotExist(err) {
		t.Errorf("Temp prompt file %s still exists after session completion, should be cleaned up", promptFilePath)
	}
}

// TestRefineWorkflowResolvesPhaseName verifies agent resolver is called with "refine" phase.
// Expected failure: Refine() does not pass correct phase parameter to AgentResolver.Resolve() yet.
func TestRefineWorkflowResolvesPhaseName(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	var resolvedPhase string
	mockAgent := &mockAgentResolver{
		resolveFunc: func(phase string, flagOverride string, choosePicker bool) (interface{}, error) {
			resolvedPhase = phase
			return &testSessionAgent{}, nil
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
	input := RefineInput{IdeaText: "test"}

	session, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify phase before waiting
	if resolvedPhase != "refine" {
		t.Errorf("AgentResolver.Resolve() called with phase = %q, want %q", resolvedPhase, "refine")
	}

	go func() {
		for range session.Events() {
		}
	}()

	_ = session.Wait()
}

// Test helper types

type testSessionAgent struct {
	onLaunch func()
}

func (a *testSessionAgent) Launch(promptPath string) error {
	if a.onLaunch != nil {
		a.onLaunch()
	}
	return nil
}

type capturePathAgent struct {
	onLaunch func(path string)
}

func (a *capturePathAgent) Launch(promptPath string) error {
	if a.onLaunch != nil {
		a.onLaunch(promptPath)
	}
	return nil
}
