package accept

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestNewRejectsNilLLM(t *testing.T) {
	t.Parallel()

	_, err := New(&config.Config{}, &fakeGitAdapter{}, nil, "", "", "")
	if err == nil {
		t.Fatal("expected New to fail when the LLM provider is nil")
	}
}

func TestRunWritesGapAnalysisWhenCriterionFails(t *testing.T) {
	t.Parallel()

	specID := "spec-gap"
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, ".gromit", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}
	specPath := filepath.Join(specDir, specID+".md")
	content := "# Gap spec\n\n## Acceptance Criteria\n- satisfy A\n- satisfy B\n"
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmp,
		Paths: config.PathsConfig{
			GromitDir: ".gromit",
			Specs:     ".gromit/specs",
		},
	}

	git := &fakeGitAdapter{diff: "diff content"}
	llmProvider := &fakeLLM{
		responses: []*llm.LLMResponse{
			{Success: true, Output: `{"pass": true, "summary": "ok"}`},
			{Success: true, Output: `{"pass": false, "summary": "missing tests"}`},
		},
	}

	stageInstance, err := New(cfg, git, llmProvider, "BASE", "PROJECT", "FRAGMENT")
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: specID},
		Worktree: tmp,
	}

	res, err := stageInstance.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res.Decision != stagepkg.DecisionFail {
		t.Fatalf("decision = %v, want %v", res.Decision, stagepkg.DecisionFail)
	}

	artifacts, ok := res.Artifacts.(*AcceptArtifacts)
	if !ok {
		t.Fatalf("artifacts type = %T, want *AcceptArtifacts", res.Artifacts)
	}
	if artifacts.GapSummary == "" {
		t.Fatalf("gap summary is empty, want failure details")
	}

	gapPath := filepath.Join(tmp, ".gromit", "v2", gapFileName)
	data, err := os.ReadFile(gapPath)
	if err != nil {
		t.Fatalf("read gap file: %v", err)
	}
	if string(data) != artifacts.GapSummary {
		t.Fatalf("gap content = %q, want %q", string(data), artifacts.GapSummary)
	}

	if len(artifacts.Results) != 2 {
		t.Fatalf("results count = %d, want 2", len(artifacts.Results))
	}

	if !strings.Contains(llmProvider.calls[0].Prompt, "BASE") || !strings.Contains(llmProvider.calls[0].Prompt, "PROJECT") {
		t.Fatalf("prompt missing configured layers")
	}
	if !strings.Contains(llmProvider.calls[1].Prompt, "diff content") {
		t.Fatalf("prompt missing diff content")
	}
	if git.lastWorktree != tmp {
		t.Fatalf("diff invoked with %q, want %q", git.lastWorktree, tmp)
	}
}

type fakeLLM struct {
	calls     []llm.InvokeRequest
	responses []*llm.LLMResponse
}

func (f *fakeLLM) Invoke(ctx context.Context, req llm.InvokeRequest) (*llm.LLMResponse, error) {
	f.calls = append(f.calls, req)
	if len(f.responses) == 0 {
		return nil, fmt.Errorf("no responses")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (f *fakeLLM) StreamInvoke(ctx context.Context, req llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	return nil, fmt.Errorf("stream invoke not supported")
}

type fakeGitAdapter struct {
	diff         string
	lastWorktree string
}

func (f *fakeGitAdapter) Checkout(ctx context.Context, specID string) (string, error) {
	return "", nil
}

func (f *fakeGitAdapter) Diff(ctx context.Context, worktree string) (string, error) {
	f.lastWorktree = worktree
	return f.diff, nil
}
