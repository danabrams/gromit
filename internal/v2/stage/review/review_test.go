package review_test

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/v2/adapter/llm"
    "github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    "github.com/danabrams/gromit/internal/v2/stage/review"
)

func TestStageSkipsWhenReviewDisabled(t *testing.T) {
    cfg := &config.Config{Review: config.ReviewConfig{Enabled: false, Tier: "sonnet"}}
    stage, err := review.New(cfg, func(context.Context, string) (string, error) { return "", nil }, &fakeLLM{}, &fakeTracker{}, "", "", "")
    if err != nil {
        t.Fatalf("unexpected error creating stage: %v", err)
    }

    req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec-1"}, Config: cfg}
    res, err := stage.Run(context.Background(), req)
    if err != nil {
        t.Fatalf("Run() error = %v", err)
    }
    if res == nil || res.Decision != stagepkg.DecisionProceed {
        t.Fatalf("unexpected decision: %#v", res)
    }
}

type fakeLLM struct {
    lastPrompt string
    response   *llm.LLMResponse
}

func (f *fakeLLM) Invoke(_ context.Context, req llm.InvokeRequest) (*llm.LLMResponse, error) {
    f.lastPrompt = req.Prompt
    if f.response != nil {
        return f.response, nil
    }
    return &llm.LLMResponse{Success: true, Output: "{}"}, nil
}

func (f *fakeLLM) StreamInvoke(context.Context, llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
    return &llm.LLMResponse{Success: true, Output: ""}, nil
}

type fakeTracker struct{}

func (f *fakeTracker) NextBead(context.Context) (*tasktracker.Bead, error) { return nil, nil }
func (f *fakeTracker) CreateBead(context.Context, string, string, int, []string, []string) (*tasktracker.Bead, error) { return nil, nil }
func (f *fakeTracker) CloseBead(context.Context, string) error { return nil }
func (f *fakeTracker) QueryBeads(context.Context, []string, string, string) ([]tasktracker.Bead, error) {
    return nil, nil
}

func TestStageIncludesDiffAndAcceptanceCriteriaInPrompt(t *testing.T) {
    temp := t.TempDir()
    specsDir := filepath.Join(temp, ".gromit", "specs")
    if err := os.MkdirAll(specsDir, 0o755); err != nil {
        t.Fatalf("setup specs dir: %v", err)
    }
    specPath := filepath.Join(specsDir, "spec-1.md")
    specBody := "## Acceptance Criteria\n- satisfy A\n- satisfy B"
    if err := os.WriteFile(specPath, []byte(specBody), 0o644); err != nil {
        t.Fatalf("write spec: %v", err)
    }

    const diffText = "diff --git a/foo b/foo"
    gitDiff := func(context.Context, string) (string, error) {
        return diffText, nil
    }

    cfg := &config.Config{Review: config.ReviewConfig{Enabled: true, Tier: "sonnet"}}
    llmStub := &fakeLLM{response: &llm.LLMResponse{Success: true, Output: `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "ok"}`}}
    stage, err := review.New(cfg, gitDiff, llmStub, &fakeTracker{}, "base", "project", "fragment")
    if err != nil {
        t.Fatalf("unexpected error creating stage: %v", err)
    }

    req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec-1"}, Worktree: temp, Config: cfg}
    if _, err := stage.Run(context.Background(), req); err != nil {
        t.Fatalf("Run() error = %v", err)
    }

    if !strings.Contains(llmStub.lastPrompt, diffText) {
        t.Fatalf("prompt missing diff, got %q", llmStub.lastPrompt)
    }
    if !strings.Contains(llmStub.lastPrompt, "satisfy A") || !strings.Contains(llmStub.lastPrompt, "satisfy B") {
        t.Fatalf("prompt missing acceptance criteria, got %q", llmStub.lastPrompt)
    }
}
