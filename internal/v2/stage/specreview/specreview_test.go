package specreview_test

import (
	context "context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/specreview"
)

type fakeGit struct {
	diff string
	err  error
}

func (f *fakeGit) DiffFromBase(_ context.Context, _ string) (string, error) {
	return f.diff, f.err
}

type fakeLLM struct {
	responses []*llmtypes.LLMInvokeResponse
	requests  []llmtypes.LLMInvokeRequest
}

func (f *fakeLLM) Invoke(_ context.Context, req llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	idx := len(f.requests)
	f.requests = append(f.requests, req)
	if idx >= len(f.responses) {
		return &llmtypes.LLMInvokeResponse{Success: true, Output: `{"passed": true}`}, nil
	}
	return f.responses[idx], nil
}

func (f *fakeLLM) StreamInvoke(_ context.Context, _ llmtypes.LLMStreamInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return nil, nil
}

func TestRun_requiresRequest(t *testing.T) {
	cfg := &config.Config{}
	stage, _ := specreview.New(cfg, &fakeGit{}, &fakeLLM{}, "", "", "")
	if _, err := stage.Run(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestRun_missingPlanReturnsError(t *testing.T) {
	cfg := &config.Config{ProjectRoot: t.TempDir(), Paths: config.PathsConfig{GromitDir: ".gromit"}, Models: config.ModelsConfig{P0: "opus"}}
	stage, err := specreview.New(cfg, &fakeGit{}, &fakeLLM{}, "", "", "")
	if err != nil {
		t.Fatalf("unexpected new error: %v", err)
	}
	req := stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec"}, Worktree: cfg.ProjectRoot}
	if _, err := stage.Run(context.Background(), &req); err == nil {
		t.Fatal("expected plan file error")
	}
}

func TestRun_usesPlanAndModel(t *testing.T) {
	root := t.TempDir()
	planDir := filepath.Join(root, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	planFile := filepath.Join(planDir, "plan.md")
	if err := os.WriteFile(planFile, []byte("# Test Plan"), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	cfg := &config.Config{ProjectRoot: root, Paths: config.PathsConfig{GromitDir: ".gromit"}, Models: config.ModelsConfig{P0: "super-model"}}
	llm := &fakeLLM{responses: []*llmtypes.LLMInvokeResponse{{Success: true, Output: `{"passed": true, "summary": "ok"}`}}}
	stage, err := specreview.New(cfg, &fakeGit{diff: "diff"}, llm, "base", "project", "")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	req := stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec"}, Worktree: root}
	res, err := stage.Run(context.Background(), &req)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want %v", res.Decision, stagepkg.DecisionProceed)
	}
	if len(llm.requests) != 1 {
		t.Fatalf("expected one invocation, got %d", len(llm.requests))
	}
	if got := llm.requests[0].Model; got != "super-model" {
		t.Fatalf("model = %q, want super-model", got)
	}
	if !strings.Contains(llm.requests[0].Prompt, "# Test Plan") {
		t.Fatalf("prompt missing plan section")
	}
}

func TestRun_retriesOnParseFailure(t *testing.T) {
	root := t.TempDir()
	planDir := filepath.Join(root, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "plan.md"), []byte("# Plan"), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	llm := &fakeLLM{
		responses: []*llmtypes.LLMInvokeResponse{
			{Success: true, Output: "not json"},
			{Success: true, Output: `{"passed": false}`},
		},
	}
	cfg := &config.Config{ProjectRoot: root, Paths: config.PathsConfig{GromitDir: ".gromit"}, Models: config.ModelsConfig{P0: "opus"}}
	stage, _ := specreview.New(cfg, &fakeGit{diff: "d"}, llm, "", "", "")
	res, err := stage.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec"}, Worktree: root})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != stagepkg.DecisionFail {
		t.Fatalf("expected fail when review not passed")
	}
	if len(llm.requests) != 2 {
		t.Fatalf("expected retry, got %d invocations", len(llm.requests))
	}
}
