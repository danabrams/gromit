package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

const validPlanOutput = `---
id: test-spec
source_spec: test-spec
created: 2026-03-08
decomposed: false
---

# Test Spec Implementation Plan

**Goal:** Implement the test spec feature.
**Architecture:** Simple layered approach.

## Architecture

The system uses a service layer that communicates with a data layer through interfaces.

## Implementation Tasks

### Task 1: Add core types
**Files:** types.go
**What to Do:** Define the core types for the feature.
**Acceptance Criteria:** Types compile and are exported.
`

func TestStageWritesPlanAndInvokesLLM(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	specID := "cool-feature"
	specContent := "# Cool feature spec\nDetails"

	specsDir := filepath.Join(tmpDir, ".gromit", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}

	specPath := filepath.Join(specsDir, specID+".md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths: config.PathsConfig{
			Specs:     ".gromit/specs",
			GromitDir: ".gromit",
		},
	}

	baseLayer := "base"
	projectLayer := "project"
	fragmentLayer := "fragment"
	fake := &fakeLLMProvider{
		response: &llm.LLMInvokeResponse{
			Success: true,
			Output:  validPlanOutput,
		},
	}

	stageInstance, err := New(cfg, fake, baseLayer, projectLayer, fragmentLayer)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: specID},
		Config: cfg,
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil || res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("unexpected decision: %v", res)
	}

	artifacts, ok := res.Artifacts.(*PlanArtifacts)
	if !ok {
		t.Fatalf("unexpected artifacts type: %T", res.Artifacts)
	}

	expectedPlan := fake.response.Output
	if artifacts.Plan != expectedPlan {
		t.Fatalf("plan mismatch: got %q want %q", artifacts.Plan, expectedPlan)
	}

	planPath := filepath.Join(tmpDir, ".gromit", "v2", "plan.md")
	if artifacts.Path != planPath {
		t.Fatalf("plan path = %q, want %q", artifacts.Path, planPath)
	}

	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan file: %v", err)
	}
	if string(data) != expectedPlan {
		t.Fatalf("plan file contents mismatch: %q", string(data))
	}

	expectedPrompt := prompt.NewPromptAssembler(baseLayer, projectLayer, specContent, fragmentLayer).Assemble("", prompt.BeadInfo{})
	if fake.lastRequest.Prompt != expectedPrompt {
		t.Fatalf("prompt mismatch: got %q want %q", fake.lastRequest.Prompt, expectedPrompt)
	}
	if fake.lastRequest.Model != "opus" {
		t.Fatalf("model mismatch: got %q want opus", fake.lastRequest.Model)
	}
}

func TestStageUsesLLMProvider(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	specID := "almost-done"
	specContent := "# Spec\nMore details"

	specsDir := filepath.Join(tmpDir, ".gromit", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}

	specPath := filepath.Join(specsDir, specID+".md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths: config.PathsConfig{
			Specs:     ".gromit/specs",
			GromitDir: ".gromit",
		},
	}

	baseLayer := "base"
	projectLayer := "project"
	fragmentLayer := "fragment"
	provider := &fakeLLMProvider{
		resetResponse: validPlanOutput,
	}

	stageInstance, err := New(cfg, provider, baseLayer, projectLayer, fragmentLayer)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: specID},
		Config: cfg,
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res == nil || res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("unexpected decision: %v", res)
	}

	artifacts, ok := res.Artifacts.(*PlanArtifacts)
	if !ok {
		t.Fatalf("unexpected artifacts type: %T", res.Artifacts)
	}

	if artifacts.Plan != provider.response.Output {
		t.Fatalf("plan mismatch: got %q want %q", artifacts.Plan, provider.response.Output)
	}

	expectedPrompt := prompt.NewPromptAssembler(baseLayer, projectLayer, specContent, fragmentLayer).Assemble("", prompt.BeadInfo{})
	if provider.lastRequest.Prompt != expectedPrompt {
		t.Fatalf("prompt mismatch: got %q want %q", provider.lastRequest.Prompt, expectedPrompt)
	}
	if provider.lastRequest.Model != "opus" {
		t.Fatalf("model mismatch: got %q want opus", provider.lastRequest.Model)
	}
}

type fakeLLMProvider struct {
	response      *llm.LLMInvokeResponse
	lastRequest   llm.LLMInvokeRequest
	resetResponse string
	err           error
}

func (f *fakeLLMProvider) Invoke(_ context.Context, req llm.LLMInvokeRequest) (*llm.LLMInvokeResponse, error) {
	f.lastRequest = req
	if f.err != nil {
		return nil, f.err
	}
	if f.response == nil {
		f.response = &llm.LLMInvokeResponse{Success: true, Output: f.resetResponse}
	}
	return f.response, nil
}

func (f *fakeLLMProvider) StreamInvoke(context.Context, llm.LLMStreamInvokeRequest) (*llm.LLMStreamInvokeResponse, error) {
	panic("not implemented")
}

func setupPlanStage(t *testing.T, provider *fakeLLMProvider) (*Stage, *config.Config, string) {
	t.Helper()
	tmpDir := t.TempDir()
	specID := "test-spec"
	specContent := "# Test spec\nDetails"

	specsDir := filepath.Join(tmpDir, ".gromit", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	specPath := filepath.Join(specsDir, specID+".md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths: config.PathsConfig{
			Specs:     ".gromit/specs",
			GromitDir: ".gromit",
		},
	}

	stageInstance, err := New(cfg, provider, "base", "project", "fragment")
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	return stageInstance, cfg, specID
}

func TestRunReturnsErrorWhenLLMFails(t *testing.T) {
	t.Parallel()
	provider := &fakeLLMProvider{err: fmt.Errorf("llm unavailable")}
	stageInstance, cfg, specID := setupPlanStage(t, provider)

	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Config: cfg}
	_, err := stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when LLM returns error, got nil")
	}
	if !strings.Contains(err.Error(), "invoke llm") {
		t.Fatalf("error should mention invoke llm, got: %v", err)
	}
}

func TestRunReturnsErrorWhenLLMReturnsNil(t *testing.T) {
	t.Parallel()
	provider := &fakeLLMProvider{response: nil, resetResponse: ""}
	// Override Invoke to return nil response without error.
	nilProvider := &nilResponseLLMProvider{}
	stageInstance, cfg, specID := setupPlanStage(t, provider)
	// Reconstruct with nilProvider.
	stageInstance, _ = New(cfg, nilProvider, "base", "project", "fragment")

	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Config: cfg}
	_, err := stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when LLM returns nil response, got nil")
	}
	if !strings.Contains(err.Error(), "nil response") {
		t.Fatalf("error should mention nil response, got: %v", err)
	}
}

func TestRunReturnsErrorWhenLLMReportsFailure(t *testing.T) {
	t.Parallel()
	provider := &fakeLLMProvider{
		response: &llm.LLMInvokeResponse{Success: false, Output: "budget exceeded"},
	}
	stageInstance, cfg, specID := setupPlanStage(t, provider)

	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Config: cfg}
	_, err := stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when LLM reports Success=false, got nil")
	}
	if !strings.Contains(err.Error(), "unsuccessful") {
		t.Fatalf("error should mention unsuccessful, got: %v", err)
	}
	if !strings.Contains(err.Error(), "budget exceeded") {
		t.Fatalf("error should include provider output detail, got: %v", err)
	}
}

type nilResponseLLMProvider struct{}

func (n *nilResponseLLMProvider) Invoke(context.Context, llm.LLMInvokeRequest) (*llm.LLMInvokeResponse, error) {
	return nil, nil
}

func (n *nilResponseLLMProvider) StreamInvoke(context.Context, llm.LLMStreamInvokeRequest) (*llm.LLMStreamInvokeResponse, error) {
	panic("not implemented")
}

func TestRunFallsBackToStoredConfig(t *testing.T) {
	t.Parallel()

	provider := &fakeLLMProvider{
		response: &llm.LLMInvokeResponse{Success: true, Output: validPlanOutput},
	}
	stageInstance, cfg, specID := setupPlanStage(t, provider)

	// Pass a request with nil Config — should fall back to the cfg stored in the constructor.
	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Config: nil}
	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("expected Run to succeed with stored config, got error: %v", err)
	}
	if res == nil || res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("unexpected decision: %v", res)
	}

	artifacts, ok := res.Artifacts.(*PlanArtifacts)
	if !ok {
		t.Fatalf("unexpected artifacts type: %T", res.Artifacts)
	}
	if artifacts.Plan != provider.response.Output {
		t.Fatalf("plan mismatch: got %q want %q", artifacts.Plan, provider.response.Output)
	}

	// Verify the plan was written using the stored config's project root.
	planPath := filepath.Join(cfg.ProjectRoot, ".gromit", "v2", "plan.md")
	if artifacts.Path != planPath {
		t.Fatalf("plan path = %q, want %q", artifacts.Path, planPath)
	}
}

func TestRunUsesReqModelOverride(t *testing.T) {
	t.Parallel()

	provider := &fakeLLMProvider{
		response: &llm.LLMInvokeResponse{Success: true, Output: validPlanOutput},
	}
	stageInstance, cfg, specID := setupPlanStage(t, provider)

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: specID},
		Config: cfg,
		Model:  "custom-model",
	}
	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}

	artifacts, ok := res.Artifacts.(*PlanArtifacts)
	if !ok {
		t.Fatalf("unexpected artifacts type: %T", res.Artifacts)
	}

	if provider.lastRequest.Model != "custom-model" {
		t.Fatalf("model sent to LLM = %q, want %q", provider.lastRequest.Model, "custom-model")
	}
	if artifacts.Model != "custom-model" {
		t.Fatalf("artifacts.Model = %q, want %q", artifacts.Model, "custom-model")
	}
}

func TestRunUsesConfigP0Model(t *testing.T) {
	t.Parallel()

	provider := &fakeLLMProvider{
		response: &llm.LLMInvokeResponse{Success: true, Output: validPlanOutput},
	}
	stageInstance, cfg, specID := setupPlanStage(t, provider)

	// Set a custom P0 model in config.
	cfg.Models.P0 = "my-opus-variant"

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: specID},
		Config: cfg,
	}
	_, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}

	if provider.lastRequest.Model != "my-opus-variant" {
		t.Fatalf("model sent to LLM = %q, want %q", provider.lastRequest.Model, "my-opus-variant")
	}
}

func TestRunDefaultsToOpusWhenNoModelConfigured(t *testing.T) {
	t.Parallel()

	provider := &fakeLLMProvider{
		response: &llm.LLMInvokeResponse{Success: true, Output: validPlanOutput},
	}
	stageInstance, cfg, specID := setupPlanStage(t, provider)

	// Clear P0 so it falls through to the default.
	cfg.Models.P0 = ""

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: specID},
		Config: cfg,
	}
	_, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}

	if provider.lastRequest.Model != "opus" {
		t.Fatalf("model sent to LLM = %q, want %q", provider.lastRequest.Model, "opus")
	}
}

func TestValidatePlanContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "rejects empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "rejects meta-statement one-liner",
			input:   "All three exploration agents have completed. The plan is finalized at `.gromit/v2/plan.md`.",
			wantErr: true,
		},
		{
			name:    "rejects short text without structure",
			input:   "Here is a plan for the feature. It should work well.",
			wantErr: true,
		},
		{
			name: "accepts valid plan with frontmatter and tasks",
			input: `---
id: cool-feature
source_spec: cool-feature
created: 2026-03-08
decomposed: false
---

# Cool Feature Implementation Plan

**Goal:** Implement the cool feature.
**Architecture:** Simple service layer.

## Architecture

Components and data flow here.

## Implementation Tasks

### Task 1: Add types
**Files:** types.go
**What to Do:** Define the types.
**Acceptance Criteria:** Types compile.
`,
			wantErr: false,
		},
		{
			name: "accepts plan with Task sections but no frontmatter",
			input: `# Implementation Plan

## Implementation Tasks

### Task 1: Set up the module
**Files:** go.mod, main.go
**What to Do:** Initialize the Go module.
**Acceptance Criteria:** Module compiles.

### Task 2: Add handler
**Files:** handler.go
**What to Do:** Add HTTP handler.
**Acceptance Criteria:** Handler responds to requests.
`,
			wantErr: false,
		},
		{
			name: "accepts plan with Architecture section",
			input: `---
id: feature
---

# Plan

## Architecture

The system uses a layered architecture with clear separation of concerns.
Components include the API layer, service layer, and data layer.
Each layer communicates through well-defined interfaces.
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePlanContent(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for input %q, got nil", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunReturnsErrorForMetaStatementOutput(t *testing.T) {
	t.Parallel()

	provider := &fakeLLMProvider{
		response: &llm.LLMInvokeResponse{
			Success: true,
			Output:  "All three exploration agents have completed. The plan is finalized at `.gromit/v2/plan.md`.",
		},
	}
	stageInstance, cfg, specID := setupPlanStage(t, provider)

	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Config: cfg}
	_, err := stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when LLM returns meta-statement instead of plan, got nil")
	}
	if !strings.Contains(err.Error(), "plan content validation") {
		t.Fatalf("error should mention plan content validation, got: %v", err)
	}
}

func setupPlanStageWithProvider(t *testing.T, provider llm.LLMProvider) (*Stage, *config.Config, string) {
	t.Helper()
	tmpDir := t.TempDir()
	specID := "test-spec"
	specContent := "# Test spec\nDetails"

	specsDir := filepath.Join(tmpDir, ".gromit", "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	specPath := filepath.Join(specsDir, specID+".md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths: config.PathsConfig{
			Specs:     ".gromit/specs",
			GromitDir: ".gromit",
		},
	}

	stageInstance, err := New(cfg, provider, "base", "project", "fragment")
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	return stageInstance, cfg, specID
}

// sequentialLLMProvider returns different responses on successive Invoke calls.
type sequentialLLMProvider struct {
	responses []*llm.LLMInvokeResponse
	callCount int
}

func (s *sequentialLLMProvider) Invoke(_ context.Context, _ llm.LLMInvokeRequest) (*llm.LLMInvokeResponse, error) {
	idx := s.callCount
	s.callCount++
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	return s.responses[idx], nil
}

func (s *sequentialLLMProvider) StreamInvoke(context.Context, llm.LLMStreamInvokeRequest) (*llm.LLMInvokeResponse, error) {
	panic("not implemented")
}

func TestPlanStage_RetriesOnValidationFailure(t *testing.T) {
	t.Parallel()

	provider := &sequentialLLMProvider{
		responses: []*llm.LLMInvokeResponse{
			{Success: true, Output: "Plan saved to file.md"},
			{Success: true, Output: validPlanOutput},
		},
	}

	stageInstance, cfg, specID := setupPlanStageWithProvider(t, provider)

	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Config: cfg}
	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if res == nil || res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("unexpected decision: %v", res)
	}

	artifacts, ok := res.Artifacts.(*PlanArtifacts)
	if !ok {
		t.Fatalf("unexpected artifacts type: %T", res.Artifacts)
	}
	if artifacts.Plan != validPlanOutput {
		t.Fatalf("plan should be the valid output from retry, got %q", artifacts.Plan)
	}
	if provider.callCount != 2 {
		t.Fatalf("expected provider to be invoked 2 times, got %d", provider.callCount)
	}
}

func TestPlanStage_RetryExhausted_IncludesPreview(t *testing.T) {
	t.Parallel()

	invalidOutput := "Plan saved to file.md"
	provider := &sequentialLLMProvider{
		responses: []*llm.LLMInvokeResponse{
			{Success: true, Output: invalidOutput},
			{Success: true, Output: invalidOutput},
		},
	}

	stageInstance, cfg, specID := setupPlanStageWithProvider(t, provider)

	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Config: cfg}
	_, err := stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when all retries exhausted, got nil")
	}
	if !strings.Contains(err.Error(), "plan content validation") {
		t.Fatalf("error should mention plan content validation, got: %v", err)
	}
	if !strings.Contains(err.Error(), invalidOutput) {
		t.Fatalf("error should include preview of invalid output, got: %v", err)
	}
}

func TestPlanStage_NoRetryOnValidOutput(t *testing.T) {
	t.Parallel()

	provider := &sequentialLLMProvider{
		responses: []*llm.LLMInvokeResponse{
			{Success: true, Output: validPlanOutput},
		},
	}

	stageInstance, cfg, specID := setupPlanStageWithProvider(t, provider)

	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Config: cfg}
	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success on valid first response, got error: %v", err)
	}
	if res == nil || res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("unexpected decision: %v", res)
	}
	if provider.callCount != 1 {
		t.Fatalf("expected provider to be invoked 1 time, got %d", provider.callCount)
	}
}
