package build

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
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

	want := stagedesc.Describe("build", cfg)
	if got := stage.Name(); got != want {
		t.Fatalf("stage name = %q, want %q", got, want)
	}
}

func TestBuildStageRunIncludesPriorFailuresAndEmitsEvents(t *testing.T) {
	cfg := &config.Config{Escalation: config.EscalationConfig{Enabled: false}}
	fragments := PromptFragments{Standard: "standard", TDD: "tdd fragment", Refactor: "refactor fragment"}
	expected := &llm.LLMResponse{
		Success:      true,
		Output:       "llm-output",
		Tokens:       42,
		InputTokens:  30,
		OutputTokens: 12,
		CostUSD:      0.25,
		Duration:     2 * time.Second,
	}
	adapter := &capturingLLM{response: expected}

	stageInstance, err := New(cfg, adapter, "base-layer", "project-layer", fragments, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error constructing stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:          "bead-123",
			Title:       "Implement auth module",
			Description: "Add JWT-based authentication",
			Labels:      []string{"tdd:true"},
		},
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
	for _, want := range []string{
		"base-layer", "project-layer", "tdd fragment", "bead-123", "validate failed",
		"Task: Implement auth module", "Description: Add JWT-based authentication",
	} {
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
	if artifacts.InputTokens != expected.InputTokens {
		t.Fatalf("input tokens = %d, want %d", artifacts.InputTokens, expected.InputTokens)
	}
	if artifacts.OutputTokens != expected.OutputTokens {
		t.Fatalf("output tokens = %d, want %d", artifacts.OutputTokens, expected.OutputTokens)
	}
	if artifacts.CostUSD != expected.CostUSD {
		t.Fatalf("cost = %v, want %v", artifacts.CostUSD, expected.CostUSD)
	}
	if artifacts.Success != expected.Success {
		t.Fatalf("success = %v, want %v", artifacts.Success, expected.Success)
	}

	if len(res.Events) != 0 {
		t.Fatalf("events count = %d, want 0", len(res.Events))
	}
}

func TestBuildStageEscalatesModelOnFailure(t *testing.T) {
	cfg := &config.Config{
		Escalation: config.EscalationConfig{Enabled: true, Chain: []string{"haiku", "sonnet"}},
	}
	fragments := PromptFragments{Standard: "standard"}
	responses := []*llm.LLMResponse{
		{Success: false, Output: "first fail", Tokens: 10},
		{Success: true, Output: "second success", Tokens: 42, Duration: 1500 * time.Millisecond, CostUSD: 0.33},
	}
	adapter := &sequencedLLM{responses: responses}

	stageInstance, err := New(cfg, adapter, "base", "project", fragments, io.Discard)
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: "escalate-bead"},
		Model:  "haiku",
		Config: cfg,
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(adapter.models) != 2 {
		t.Fatalf("expected 2 streamed invocations, got %d", len(adapter.models))
	}
	if want := "haiku"; adapter.models[0] != want {
		t.Fatalf("first model = %q, want %q", adapter.models[0], want)
	}
	if want := "sonnet"; adapter.models[1] != want {
		t.Fatalf("second model = %q, want %q", adapter.models[1], want)
	}

	artifacts, ok := res.Artifacts.(*BuildArtifacts)
	if !ok {
		t.Fatalf("artifacts type = %T", res.Artifacts)
	}
	if artifacts.Model != "sonnet" {
		t.Fatalf("artifact model = %q, want sonnet", artifacts.Model)
	}
	if artifacts.Output != "second success" {
		t.Fatalf("artifact output = %q", artifacts.Output)
	}
}

func TestBuildStageEscalationHonorsRequestConfig(t *testing.T) {
	stageCfg := &config.Config{Escalation: config.EscalationConfig{Enabled: false}}
	requestCfg := &config.Config{Escalation: config.EscalationConfig{Enabled: true, Chain: []string{"haiku", "sonnet"}}}
	fragments := PromptFragments{Standard: "standard"}
	responses := []*llm.LLMResponse{
		{Success: false, Output: "first fail", Tokens: 10},
		{Success: true, Output: "second success", Tokens: 42},
	}
	adapter := &sequencedLLM{responses: responses}

	stageInstance, err := New(stageCfg, adapter, "base", "project", fragments, io.Discard)
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: "escalate-bead"},
		Model:  "haiku",
		Config: requestCfg,
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(adapter.models) != 2 {
		t.Fatalf("expected 2 streamed invocations, got %d", len(adapter.models))
	}
	if adapter.models[0] != "haiku" {
		t.Fatalf("first model = %q, want haiku", adapter.models[0])
	}
	if adapter.models[1] != "sonnet" {
		t.Fatalf("second model = %q, want sonnet", adapter.models[1])
	}

	artifacts, ok := res.Artifacts.(*BuildArtifacts)
	if !ok {
		t.Fatalf("artifacts type = %T", res.Artifacts)
	}
	if artifacts.Model != "sonnet" {
		t.Fatalf("artifact model = %q, want sonnet", artifacts.Model)
	}
}

func TestBuildStageEscalationBreaksOnCircularChain(t *testing.T) {
	// Chain contains a cycle: haiku -> sonnet -> haiku -> sonnet -> ...
	// Without the seen-set guard, invokeWithEscalation would loop forever.
	cfg := &config.Config{
		Escalation: config.EscalationConfig{Enabled: true, Chain: []string{"haiku", "sonnet", "haiku"}},
	}
	fragments := PromptFragments{Standard: "standard"}
	// All responses fail, so escalation keeps trying.
	adapter := &sequencedLLM{responses: []*llm.LLMResponse{
		{Success: false, Output: "fail-1"},
		{Success: false, Output: "fail-2"},
		{Success: false, Output: "fail-3"},
		{Success: false, Output: "fail-4"},
		{Success: false, Output: "fail-5"},
	}}

	stageInstance, err := New(cfg, adapter, "base", "project", fragments, io.Discard)
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: "circular-bead"},
		Model:  "haiku",
		Config: cfg,
	}

	// This must return (not hang) even though the chain is circular.
	_, err = stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error from exhausted escalation, got nil")
	}

	// The loop should have tried haiku then sonnet, then stopped when haiku
	// was seen again. That means exactly 2 invocations.
	if len(adapter.models) != 2 {
		t.Fatalf("expected 2 invocations (haiku, sonnet), got %d: %v", len(adapter.models), adapter.models)
	}
	if adapter.models[0] != "haiku" || adapter.models[1] != "sonnet" {
		t.Fatalf("unexpected model sequence: %v", adapter.models)
	}
}

func TestBuildStageEscalationRespectsMaxIterationBound(t *testing.T) {
	// Even without a literal cycle, the safety bound of len(chain)+1 should cap iterations.
	cfg := &config.Config{
		Escalation: config.EscalationConfig{Enabled: true, Chain: []string{"a", "b", "c"}},
	}
	fragments := PromptFragments{Standard: "standard"}
	// Provide many failing responses; the loop must not consume them all.
	adapter := &sequencedLLM{responses: []*llm.LLMResponse{
		{Success: false, Output: "fail-a"},
		{Success: false, Output: "fail-b"},
		{Success: false, Output: "fail-c"},
		{Success: false, Output: "fail-d"},
		{Success: false, Output: "fail-e"},
	}}

	stageInstance, err := New(cfg, adapter, "base", "project", fragments, io.Discard)
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: "bound-bead"},
		Model:  "a",
		Config: cfg,
	}

	_, err = stageInstance.Run(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error from exhausted escalation")
	}

	// Chain is [a, b, c], starting at "a". Escalation: a->b->c->(end).
	// That is 3 invocations, which is <= len(chain)+1 = 4.
	if len(adapter.models) != 3 {
		t.Fatalf("expected 3 invocations (a, b, c), got %d: %v", len(adapter.models), adapter.models)
	}
}

func TestBuildStageRetryConfigReturnsConfiguredMaxRetries(t *testing.T) {
	t.Parallel()

	fragments := PromptFragments{Standard: "standard"}

	// Test with configured MaxRetriesPerModel = 3.
	cfg := &config.Config{Escalation: config.EscalationConfig{MaxRetriesPerModel: 3}}
	stage, err := New(cfg, noopLLM{}, "base", "project", fragments, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := stage.RetryConfig()
	if rc.MaxRetries != 3 {
		t.Fatalf("RetryConfig().MaxRetries = %d, want 3", rc.MaxRetries)
	}

	// Test minimum-1 default when MaxRetriesPerModel is 0.
	cfgZero := &config.Config{Escalation: config.EscalationConfig{MaxRetriesPerModel: 0}}
	stageZero, err := New(cfgZero, noopLLM{}, "base", "project", fragments, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rcZero := stageZero.RetryConfig()
	if rcZero.MaxRetries != 1 {
		t.Fatalf("RetryConfig().MaxRetries = %d, want 1 (minimum default)", rcZero.MaxRetries)
	}
}

func TestBuildInstanceLayerIncludesTitleDescriptionAndID(t *testing.T) {
	t.Parallel()

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:          "bead-42",
			Title:       "Add logging",
			Description: "Structured logging with slog",
		},
	}
	got := buildInstanceLayer(req)
	for _, want := range []string{"Task: Add logging", "Description: Structured logging with slog", "Bead ID: bead-42"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildInstanceLayer missing %q, got:\n%s", want, got)
		}
	}
}

func TestBuildInstanceLayerTitleBeforeIDBeforeFailures(t *testing.T) {
	t.Parallel()

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:    "b-1",
			Title: "Fix bug",
		},
		RetryContext: &stagepkg.RetryContext{
			PriorFailures: []string{"test failed"},
		},
	}
	got := buildInstanceLayer(req)
	titleIdx := strings.Index(got, "Task: Fix bug")
	idIdx := strings.Index(got, "Bead ID: b-1")
	failIdx := strings.Index(got, "Prior failures:")
	if titleIdx == -1 || idIdx == -1 || failIdx == -1 {
		t.Fatalf("missing expected content, got:\n%s", got)
	}
	if titleIdx >= idIdx {
		t.Fatalf("title should appear before ID")
	}
	if idIdx >= failIdx {
		t.Fatalf("ID should appear before prior failures")
	}
}

func TestBuildInstanceLayerOmitsEmptyFields(t *testing.T) {
	t.Parallel()

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "only-id"},
	}
	got := buildInstanceLayer(req)
	if strings.Contains(got, "Task:") {
		t.Fatalf("should not contain Task: when title is empty, got:\n%s", got)
	}
	if strings.Contains(got, "Description:") {
		t.Fatalf("should not contain Description: when description is empty, got:\n%s", got)
	}
	if !strings.Contains(got, "Bead ID: only-id") {
		t.Fatalf("should contain Bead ID, got:\n%s", got)
	}
}

func TestBuildInstanceLayerNilRequest(t *testing.T) {
	t.Parallel()
	if got := buildInstanceLayer(nil); got != "" {
		t.Fatalf("expected empty string for nil request, got %q", got)
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

type sequencedLLM struct {
	responses []*llm.LLMResponse
	models    []string
}

func (s *sequencedLLM) Invoke(_ context.Context, _ llm.InvokeRequest) (*llm.LLMResponse, error) {
	if len(s.responses) == 0 {
		return &llm.LLMResponse{Success: true}, nil
	}
	return s.responses[0], nil
}

func (s *sequencedLLM) StreamInvoke(_ context.Context, req llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	idx := len(s.models)
	s.models = append(s.models, req.Model)
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return s.responses[idx], nil
}
