package build

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	stagesbuild "github.com/danabrams/gromit/internal/v2/stages/build"
)

func TestNewBuildStageRequiresConfigAndProvider(t *testing.T) {
	fragments := PromptFragments{Standard: "standard", TDD: "tdd", Refactor: "refactor"}

	if _, err := New(nil, noopLLM{}, "base", "project", fragments, io.Discard); err == nil {
		t.Fatalf("expected config required error")
	}

	if _, err := New(&config.Config{}, nil, "base", "project", fragments, io.Discard); err == nil {
		t.Fatalf("expected provider required error")
	}

	cfg := &config.Config{}
	stage, err := New(cfg, noopLLM{}, "base-layer", "project-layer", fragments, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := stagesbuild.Describe(cfg)
	if got := stage.Name(); got != want {
		t.Fatalf("stage name = %q, want %q", got, want)
	}
}

type noopLLM struct{}

func (noopLLM) Invoke(_ context.Context, _ llm.InvokeRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Success: true}, nil
}

func (noopLLM) StreamInvoke(_ context.Context, _ llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Success: true}, nil
}
