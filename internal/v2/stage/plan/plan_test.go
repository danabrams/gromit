package plan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

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
			Output:  "planned!",
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

	expectedPrompt := prompt.NewPromptAssembler(baseLayer, projectLayer, specContent, fragmentLayer).Assemble()
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
		resetResponse: "llm plan",
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

	expectedPrompt := prompt.NewPromptAssembler(baseLayer, projectLayer, specContent, fragmentLayer).Assemble()
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
}

func (f *fakeLLMProvider) Invoke(_ context.Context, req llm.LLMInvokeRequest) (*llm.LLMInvokeResponse, error) {
	f.lastRequest = req
	if f.response == nil {
		f.response = &llm.LLMInvokeResponse{Success: true, Output: f.resetResponse}
	}
	return f.response, nil
}

func (f *fakeLLMProvider) StreamInvoke(context.Context, llm.LLMStreamInvokeRequest) (*llm.LLMStreamInvokeResponse, error) {
	panic("not implemented")
}
