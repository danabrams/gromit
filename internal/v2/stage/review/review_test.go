package review_test

import (
	"context"
	"fmt"
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
	stage, err := review.New(cfg, &fakeGitAdapter{}, &fakeLLM{}, &fakeTracker{}, "", "", "")
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

func TestStageUsesGitAdapterDiff(t *testing.T) {
	diffText := "diff --git a/foo b/foo"
	git := &fakeGitAdapter{diff: diffText}
	llmStub := &fakeLLM{
		response: &llm.LLMResponse{
			Success: true,
			Output:  `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "ok"}`,
		},
	}
	cfg := &config.Config{Review: config.ReviewConfig{Enabled: true, Tier: "sonnet"}}
	stage, err := review.New(cfg, git, llmStub, &fakeTracker{}, "base", "project", "fragment")
	if err != nil {
		t.Fatalf("unexpected error creating stage: %v", err)
	}

	worktree := t.TempDir()
	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "spec-1"},
		Worktree: worktree,
		Config:   cfg,
	}

	if _, err := stage.Run(context.Background(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if git.diffCalls != 1 {
		t.Fatalf("expected git diff called once, got %d", git.diffCalls)
	}
	if git.lastWorktree != worktree {
		t.Fatalf("diff called with worktree %q, want %q", git.lastWorktree, worktree)
	}
	if !strings.Contains(llmStub.lastPrompt, diffText) {
		t.Fatalf("prompt missing diff: %q", llmStub.lastPrompt)
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

type fakeTracker struct {
	created []*tasktracker.Bead
}

func (f *fakeTracker) NextBead(context.Context) (*tasktracker.Bead, error) { return nil, nil }
func (f *fakeTracker) ShowBead(context.Context, string) (*tasktracker.Bead, error) {
	return nil, nil
}
func (f *fakeTracker) CreateBead(ctx context.Context, title, description string, priority int, labels, dependencies []string) (*tasktracker.Bead, error) {
	bead := &tasktracker.Bead{
		ID:          fmt.Sprintf("bead-%d", len(f.created)+1),
		Title:       title,
		Description: description,
		Priority:    priority,
		Labels:      append([]string(nil), labels...),
		DependsOn:   append([]string(nil), dependencies...),
	}
	f.created = append(f.created, bead)
	return bead, nil
}
func (f *fakeTracker) CloseBead(context.Context, string) error { return nil }
func (f *fakeTracker) QueryBeads(context.Context, []string, string, string) ([]tasktracker.Bead, error) {
	return nil, nil
}

type fakeGitAdapter struct {
	diff         string
	diffErr      error
	lastWorktree string
	diffCalls    int
}

func (f *fakeGitAdapter) Checkout(context.Context, string) (string, error) {
	return "", nil
}

func (f *fakeGitAdapter) Diff(_ context.Context, worktree string) (string, error) {
	f.diffCalls++
	f.lastWorktree = worktree
	if f.diffErr != nil {
		return "", f.diffErr
	}
	return f.diff, nil
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
	git := &fakeGitAdapter{diff: diffText}

	cfg := &config.Config{Review: config.ReviewConfig{Enabled: true, Tier: "sonnet"}}
	llmStub := &fakeLLM{response: &llm.LLMResponse{Success: true, Output: `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "ok"}`}}
	stage, err := review.New(cfg, git, llmStub, &fakeTracker{}, "base", "project", "fragment")
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

func TestReviewStageCreatesTaskTrackerBeads(t *testing.T) {
	diff := "diff --git a/foo.go b/foo.go"
	response := `{
        "passed": false,
        "beads_to_create": [
            {"title": "Fix bug", "description": "details", "priority": 2, "labels": ["bug"]}
        ],
        "backlog_items": [],
        "fixes_applied": [],
        "summary": "found issue"
    }`
	git := &fakeGitAdapter{diff: diff}
	llmStub := &fakeLLM{response: &llm.LLMResponse{Success: true, Output: response}}
	tracker := &fakeTracker{}
	cfg := &config.Config{Review: config.ReviewConfig{Enabled: true, Tier: "sonnet"}}
	stageInst, err := review.New(cfg, git, llmStub, tracker, "", "", "")
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec-1", Labels: []string{"gen:1"}}, Worktree: t.TempDir(), Config: cfg}
	if _, err := stageInst.Run(context.Background(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(tracker.created) != 1 {
		t.Fatalf("created %d beads, want 1", len(tracker.created))
	}
	created := tracker.created[0]
	if !hasLabel(created.Labels, "from-review") {
		t.Fatalf("labels missing from-review: %v", created.Labels)
	}
	if !hasLabel(created.Labels, "gen:2") {
		t.Fatalf("labels missing gen:2: %v", created.Labels)
	}
	if len(created.DependsOn) != 1 || created.DependsOn[0] != "spec-1" {
		t.Fatalf("unexpected dependencies = %v", created.DependsOn)
	}
}

func hasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}
