package triage_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/triage"
	"github.com/danabrams/gromit/internal/v2/testutil"
)

func validConfig() *config.Config {
	return &config.Config{}
}

func TestNew_NilConfig(t *testing.T) {
	t.Parallel()
	_, err := triage.New(nil, testutil.NewFakeLLM())
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}
	if !strings.Contains(err.Error(), "config required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_NilProvider(t *testing.T) {
	t.Parallel()
	_, err := triage.New(validConfig(), nil)
	if err == nil {
		t.Fatal("expected error for nil provider, got nil")
	}
	if !strings.Contains(err.Error(), "llm provider required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_Valid(t *testing.T) {
	t.Parallel()
	stage, err := triage.New(validConfig(), testutil.NewFakeLLM())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stage.Name() == "" {
		t.Fatal("Name() returned empty string")
	}
}

func TestRun_NilRequest(t *testing.T) {
	t.Parallel()
	stage, err := triage.New(validConfig(), testutil.NewFakeLLM())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = stage.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
	if !strings.Contains(err.Error(), "request required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_DecomposeCategory(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeLLM()
	fake.SetResponse("", &llm.LLMResponse{
		Success: true,
		Output:  `{"category": "decompose", "reasoning": "too many files to change"}`,
	})
	stage, err := triage.New(validConfig(), fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-1", Title: "implement feature X"},
		RetryContext: &stagepkg.RetryContext{
			PriorFailures: []string{"build failed: too many changes"},
		},
	}
	res, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	artifacts, ok := res.Artifacts.(*triage.TriageArtifacts)
	if !ok {
		t.Fatalf("expected *triage.TriageArtifacts, got %T", res.Artifacts)
	}
	if artifacts.Category != triage.CategoryDecompose {
		t.Fatalf("category = %q, want %q", artifacts.Category, triage.CategoryDecompose)
	}
	if artifacts.Reasoning != "too many files to change" {
		t.Fatalf("reasoning = %q, want %q", artifacts.Reasoning, "too many files to change")
	}
}

func TestRun_RetryCategory(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeLLM()
	fake.SetResponse("", &llm.LLMResponse{
		Success: true,
		Output:  `{"category": "retry", "reasoning": "network timeout"}`,
	})
	stage, err := triage.New(validConfig(), fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-2"},
		RetryContext: &stagepkg.RetryContext{
			PriorFailures: []string{"connection timed out"},
		},
	}
	res, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	artifacts := res.Artifacts.(*triage.TriageArtifacts)
	if artifacts.Category != triage.CategoryRetry {
		t.Fatalf("category = %q, want %q", artifacts.Category, triage.CategoryRetry)
	}
}

func TestRun_UnclearSpecCategory(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeLLM()
	fake.SetResponse("", &llm.LLMResponse{
		Success: true,
		Output:  `{"category": "unclear_spec", "reasoning": "requirements are contradictory"}`,
	})
	stage, err := triage.New(validConfig(), fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-3"},
		RetryContext: &stagepkg.RetryContext{
			PriorFailures: []string{"cannot determine expected behavior"},
		},
	}
	res, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	artifacts := res.Artifacts.(*triage.TriageArtifacts)
	if artifacts.Category != triage.CategoryUnclearSpec {
		t.Fatalf("category = %q, want %q", artifacts.Category, triage.CategoryUnclearSpec)
	}
}

func TestRun_UnsafeCategory(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeLLM()
	fake.SetResponse("", &llm.LLMResponse{
		Success: true,
		Output:  `{"category": "unsafe", "reasoning": "would delete production database"}`,
	})
	stage, err := triage.New(validConfig(), fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-4"},
		RetryContext: &stagepkg.RetryContext{
			PriorFailures: []string{"rm -rf / detected"},
		},
	}
	res, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	artifacts := res.Artifacts.(*triage.TriageArtifacts)
	if artifacts.Category != triage.CategoryUnsafe {
		t.Fatalf("category = %q, want %q", artifacts.Category, triage.CategoryUnsafe)
	}
}

func TestRun_LLMError(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeLLM()
	// No response set — FakeLLM returns error when no match found.
	stage, err := triage.New(validConfig(), fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-5"},
		RetryContext: &stagepkg.RetryContext{
			PriorFailures: []string{"some failure"},
		},
	}
	_, err = stage.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when LLM fails, got nil")
	}
	if !strings.Contains(err.Error(), "invoking llm") {
		t.Fatalf("error should mention invoking llm, got: %v", err)
	}
}

func TestRun_InvalidJSON(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeLLM()
	fake.SetResponse("", &llm.LLMResponse{
		Success: true,
		Output:  "this is not json at all",
	})
	stage, err := triage.New(validConfig(), fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-6"},
		RetryContext: &stagepkg.RetryContext{
			PriorFailures: []string{"build error"},
		},
	}
	_, err = stage.Run(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse response") {
		t.Fatalf("error should mention parse response, got: %v", err)
	}
}

func TestRun_NoPriorFailures(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeLLM()
	fake.SetResponse("", &llm.LLMResponse{
		Success: true,
		Output:  `{"category": "retry", "reasoning": "no explicit failure, treating as transient"}`,
	})
	stage, err := triage.New(validConfig(), fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No RetryContext at all — should still work using bead title.
	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{ID: "bead-7", Title: "add logging"},
	}
	res, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	artifacts, ok := res.Artifacts.(*triage.TriageArtifacts)
	if !ok {
		t.Fatalf("expected *triage.TriageArtifacts, got %T", res.Artifacts)
	}
	if artifacts.Category != triage.CategoryRetry {
		t.Fatalf("category = %q, want %q", artifacts.Category, triage.CategoryRetry)
	}

	// Verify the prompt included the bead title since there were no failures.
	if len(fake.Calls) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(fake.Calls))
	}
	if !strings.Contains(fake.Calls[0].Prompt, "add logging") {
		t.Fatalf("prompt should contain bead title, got: %q", fake.Calls[0].Prompt)
	}

	// Suppress unused import warning for fmt.
	_ = fmt.Sprintf
}
