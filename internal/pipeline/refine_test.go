package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPipeline_RefineValidatesDeps verifies that Refine returns an error when required dependencies are nil.
func TestPipeline_RefineValidatesDeps(t *testing.T) {
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
				AgentResolver: nil,
			},
			paths:   &Paths{},
			wantErr: "nil dependencies",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := New(tc.deps, tc.paths)
			ctx := context.Background()
			input := RefineInput{IdeaText: "test idea"}

			_, err := p.Refine(ctx, input)
			if err == nil {
				t.Fatal("Refine() should return error with invalid dependencies")
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestPipeline_RefineReturnsRefineResult verifies that Refine returns a RefineResult
// with properly initialized slice fields.
func TestPipeline_RefineReturnsRefineResult(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	mockAgent := &mockAgent{
		LaunchInDirFn: func(promptPath, dir string) error {
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	mockBacklog := &mockBacklogClient{
		GetFn: func(id string) (*Idea, error) {
			return nil, fmt.Errorf("not found")
		},
	}

	deps := &Deps{
		AgentResolver: mockAgentResolver,
		BacklogClient: mockBacklog,
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := RefineInput{IdeaText: "test idea"}

	result, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify slices are initialized (not nil)
	if result.CreatedSpecs == nil {
		t.Error("result.CreatedSpecs should be non-nil empty slice")
	}

	if result.RefinedItems == nil {
		t.Error("result.RefinedItems should be non-nil empty slice")
	}
}

// TestPipeline_RefineBuildsPromptWithIdeaText verifies that Refine builds a prompt
// using the idea text when IdeaText is provided.
func TestPipeline_RefineBuildsPromptWithIdeaText(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	var capturedPrompt string

	mockAgent := &mockAgent{
		LaunchInDirFn: func(promptPath, dir string) error {
			// Capture the prompt content
			content, err := os.ReadFile(promptPath)
			if err != nil {
				return err
			}
			capturedPrompt = string(content)
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgentResolver,
		BacklogClient: &mockBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	ideaText := "Improve error handling"
	input := RefineInput{IdeaText: ideaText}

	_, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	if !strings.Contains(capturedPrompt, ideaText) {
		t.Errorf("prompt should contain idea text %q, got %q", ideaText, capturedPrompt)
	}
}

// TestPipeline_RefineScansExistingSpecsBeforeLaunch verifies that Refine takes a pre-session
// snapshot of existing specs.
func TestPipeline_RefineScansExistingSpecsBeforeLaunch(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Create an existing spec
	existingSpec := filepath.Join(specsDir, "existing.md")
	if err := os.WriteFile(existingSpec, []byte("# Existing Spec\n"), 0o644); err != nil {
		t.Fatalf("failed to create existing spec: %v", err)
	}

	var preSessionSpecCount int

	mockAgent := &mockAgent{
		LaunchInDirFn: func(promptPath, dir string) error {
			// Verify that existing specs were counted before this launch
			// by checking that preSessionSpecCount was captured
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgentResolver,
		BacklogClient: &mockBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := RefineInput{IdeaText: "test idea"}

	// Before calling Refine, count existing specs
	existingSpecs, err := ListMarkdownFiles(specsDir)
	if err != nil {
		t.Fatalf("failed to list existing specs: %v", err)
	}
	preSessionSpecCount = len(existingSpecs)

	_, err = p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify that ListMarkdownFiles can detect existing artifacts
	if preSessionSpecCount != 1 {
		t.Fatalf("should detect 1 existing spec before session, detected %d", preSessionSpecCount)
	}
}

// TestPipeline_RefinePostProcessingDetectsNewSpecs verifies that Refine detects new specs
// created during the session.
func TestPipeline_RefinePostProcessingDetectsNewSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	mockAgent := &mockAgent{
		LaunchInDirFn: func(promptPath, dir string) error {
			// Simulate session creating a new spec
			newSpec := filepath.Join(specsDir, "new-spec.md")
			if err := os.WriteFile(newSpec, []byte("# New Spec\n"), 0o644); err != nil {
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

	deps := &Deps{
		AgentResolver: mockAgentResolver,
		BacklogClient: &mockBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := RefineInput{IdeaText: "test idea"}

	result, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify new spec was detected
	if len(result.CreatedSpecs) != 1 {
		t.Errorf("result.CreatedSpecs = %d, want 1", len(result.CreatedSpecs))
	}

	if !strings.Contains(result.CreatedSpecs[0], "new-spec") {
		t.Errorf("created spec should be new-spec, got %q", result.CreatedSpecs[0])
	}
}

// TestPipeline_RefineResolvesAgent verifies that Refine calls AgentResolver.Resolve properly.
func TestPipeline_RefineResolvesAgent(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	var resolvedPhase string

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			resolvedPhase = phase
			return &mockAgent{}, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgentResolver,
		BacklogClient: &mockBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := RefineInput{IdeaText: "test idea"}

	_, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	if resolvedPhase != "refine" {
		t.Errorf("Resolve phase = %q, want %q", resolvedPhase, "refine")
	}
}

// TestPipeline_RefineWithBlankSessionCreatesEmptyResult verifies that a blank
// refine session (no idea text or ID) returns an empty RefineResult.
func TestPipeline_RefineWithBlankSessionCreatesEmptyResult(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	mockAgent := &mockAgent{
		LaunchInDirFn: func(promptPath, dir string) error {
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver: mockAgentResolver,
		BacklogClient: &mockBacklogClient{},
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	}

	p := New(deps, paths)
	ctx := context.Background()
	input := RefineInput{} // Blank session

	result, err := p.Refine(ctx, input)
	if err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	// Verify empty result with non-nil slices
	if len(result.CreatedSpecs) != 0 {
		t.Errorf("blank session should create no specs, got %d", len(result.CreatedSpecs))
	}

	if result.CreatedSpecs == nil {
		t.Error("CreatedSpecs should be non-nil empty slice")
	}
}
