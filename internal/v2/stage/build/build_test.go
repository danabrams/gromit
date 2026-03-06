package build

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
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

func TestBuildStageRunIncludesPriorFailuresAndEmitsEvents(t *testing.T) {
	cfg := &config.Config{Escalation: config.EscalationConfig{Enabled: false}}
	fragments := PromptFragments{Standard: "standard", TDD: "tdd fragment", Refactor: "refactor fragment"}
	expected := &llm.LLMResponse{
		Success:  true,
		Output:   "llm-output",
		Tokens:   42,
		CostUSD:  0.25,
		Duration: 2 * time.Second,
	}
	adapter := &capturingLLM{response: expected}

	stageInstance, err := New(cfg, adapter, "base-layer", "project-layer", fragments, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error constructing stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: "bead-123", Labels: []string{"tdd:true"}},
		Model:  "haiku",
		Config: cfg,
		RetryContext: &stagepkg.RetryContext{
			Attempt:       1,
			PriorFailures: []string{"validate failed: timeout"},
		},
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if adapter.callCount != 1 {
		t.Fatalf("expected 1 invocation, got %d", adapter.callCount)
	}

	prompt := adapter.lastPrompt
	for _, want := range []string{"base-layer", "project-layer", "tdd fragment", "bead-123", "validate failed"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}

	artifacts, ok := res.Artifacts.(*BuildArtifacts)
	if !ok {
		t.Fatalf("artifacts type = %T", res.Artifacts)
	}
	if artifacts.Model != req.Model {
		t.Fatalf("artifact model = %q, want %q", artifacts.Model, req.Model)
	}
	if artifacts.Prompt != prompt {
		t.Fatalf("prompt artifact mismatch: got %q", artifacts.Prompt)
	}
	if artifacts.Output != expected.Output {
		t.Fatalf("output = %q, want %q", artifacts.Output, expected.Output)
	}
	if artifacts.Duration != expected.Duration {
		t.Fatalf("duration = %v, want %v", artifacts.Duration, expected.Duration)
	}
	if artifacts.Tokens != expected.Tokens {
		t.Fatalf("tokens = %d, want %d", artifacts.Tokens, expected.Tokens)
	}
	if artifacts.CostUSD != expected.CostUSD {
		t.Fatalf("cost = %v, want %v", artifacts.CostUSD, expected.CostUSD)
	}
	if artifacts.Success != expected.Success {
		t.Fatalf("success = %v, want %v", artifacts.Success, expected.Success)
	}

	if len(res.Events) != 2 {
		t.Fatalf("events count = %d, want 2", len(res.Events))
	}
	start, ok := res.Events[0].(*events.BuildStartEvent)
	if !ok {
		t.Fatalf("first event type = %T, want *events.BuildStartEvent", res.Events[0])
	}
	if start.Model != req.Model {
		t.Fatalf("start event model = %q, want %q", start.Model, req.Model)
	}
	if start.Attempt != req.RetryContext.Attempt+1 {
		t.Fatalf("start event attempt = %d, want %d", start.Attempt, req.RetryContext.Attempt+1)
	}
	startDone, ok := res.Events[1].(*events.BuildCompleteEvent)
	if !ok {
		t.Fatalf("second event type = %T, want *events.BuildCompleteEvent", res.Events[1])
	}
	if startDone.Success != true {
		t.Fatalf("complete event success = %v, want true", startDone.Success)
	}
}

type noopLLM struct{}

func (noopLLM) Invoke(_ context.Context, _ llm.InvokeRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Success: true}, nil
}

func (noopLLM) StreamInvoke(_ context.Context, _ llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Success: true}, nil
}

type capturingLLM struct {
	response   *llm.LLMResponse
	lastPrompt string
	lastModel  string
	callCount  int
}

func (c *capturingLLM) Invoke(_ context.Context, _ llm.InvokeRequest) (*llm.LLMResponse, error) {
	return c.response, nil
}

func (c *capturingLLM) StreamInvoke(_ context.Context, req llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	c.callCount++
	c.lastPrompt = req.Prompt
	c.lastModel = req.Model
	return c.response, nil
}
