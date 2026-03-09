package specreview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/routing"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestRunIncludesPlanAndDiff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writePlan(t, root, "PLAN_CONTENT")

	git := &fakeGitDiffer{diff: "DIFF_CONTENT"}
	provider := &fakeProvider{responses: []*llmtypes.LLMInvokeResponse{{Success: true, Output: `{"verdict":"pass","findings":[]}`}}}

	stage, err := New(&config.Config{ProjectRoot: root}, git, provider, "base", "project", "# fragment")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "spec-1", Title: "spec title"},
		Worktree: root,
	}

	res, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want %v", res.Decision, stagepkg.DecisionProceed)
	}

	if got := len(provider.requests); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
	reqMeta := provider.requests[0]
	if tier := reqMeta.Metadata["tier"]; tier != routing.TierHigh {
		t.Fatalf("tier metadata = %q, want %q", tier, routing.TierHigh)
	}
	if reqMeta.Model != config.ModelOpus {
		t.Fatalf("model = %q, want %q", reqMeta.Model, config.ModelOpus)
	}
	if !strings.Contains(reqMeta.Prompt, "DIFF_CONTENT") {
		t.Fatalf("prompt missing diff")
	}
	if !strings.Contains(reqMeta.Prompt, "PLAN_CONTENT") {
		t.Fatalf("prompt missing plan")
	}
}

func TestRunCriticalFindingFailsAndEmitsEvent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writePlan(t, root, "PLAN")

	git := &fakeGitDiffer{diff: "DIFF"}
	output := `{"verdict":"pass","findings":[{"severity":"critical","category":"bug","scope":"spec","description":"danger","affected_files":["file.go"]}]}`
	provider := &fakeProvider{responses: []*llmtypes.LLMInvokeResponse{{Success: true, Output: output}}}

	stage, err := New(&config.Config{ProjectRoot: root}, git, provider, "base", "project", "# fragment")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	eventsCh := make(chan event.TypedEvent, 1)
	emitter := event.NewEmitter()
	emitter.Subscribe(func(evt event.TypedEvent) {
		eventsCh <- evt
	})
	stage = stage.WithTypedEmitter(emitter)

	res, err := stage.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec-id"}, Worktree: root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Decision != stagepkg.DecisionFail {
		t.Fatalf("decision = %v, want %v", res.Decision, stagepkg.DecisionFail)
	}
	artifacts := res.Artifacts.(*SpecReviewArtifacts)
	if artifacts.Verdict != "fail" {
		t.Fatalf("verdict = %q, want fail", artifacts.Verdict)
	}
	if len(artifacts.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(artifacts.Findings))
	}
	var evt event.TypedEvent
	select {
	case evt = <-eventsCh:
	case <-time.After(time.Second):
		t.Fatalf("expected event, got none")
	}
	specEvt, ok := evt.(*event.SpecReviewCompletedEvent)
	if !ok {
		t.Fatalf("event type = %T, want *event.SpecReviewCompletedEvent", evt)
	}
	if specEvt.Verdict != "fail" || specEvt.Success {
		t.Fatalf("event verdict/success mismatch: %#v", specEvt)
	}
	if specEvt.FindingCount != 1 || specEvt.CriticalFindings != 1 {
		t.Fatalf("unexpected counts: %+v", specEvt)
	}
}

func TestRunRetryOnParseFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writePlan(t, root, "PLAN")

	git := &fakeGitDiffer{diff: "DIFF"}
	provider := &fakeProvider{
		responses: []*llmtypes.LLMInvokeResponse{
			{Success: true, Output: "not json"},
			{Success: true, Output: `{"verdict":"pass","findings":[]}`},
		},
	}

	stage, err := New(&config.Config{ProjectRoot: root}, git, provider, "base", "project", "# fragment")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := stage.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec-id"}, Worktree: root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want %v", res.Decision, stagepkg.DecisionProceed)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(provider.requests))
	}
	if !strings.Contains(provider.requests[1].Prompt, "not valid JSON") {
		t.Fatalf("repair prompt missing notice")
	}
}

func TestRunFailsAfterRetry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writePlan(t, root, "PLAN")

	git := &fakeGitDiffer{diff: "DIFF"}
	provider := &fakeProvider{
		responses: []*llmtypes.LLMInvokeResponse{
			{Success: true, Output: "bad"},
			{Success: true, Output: "bad again"},
		},
	}

	stage, err := New(&config.Config{ProjectRoot: root}, git, provider, "base", "project", "# fragment")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := stage.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec-id"}, Worktree: root}); err == nil {
		t.Fatal("expected parse error")
	}
}

type fakeGitDiffer struct {
	diff   string
	called bool
}

func (f *fakeGitDiffer) DiffFromBase(ctx context.Context, worktree string) (string, error) {
	f.called = true
	return f.diff, nil
}

func writePlan(t *testing.T, root, content string) {
	t.Helper()
	path := filepath.Join(root, ".gromit", "v2")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
}

type fakeProvider struct {
	responses []*llmtypes.LLMInvokeResponse
	requests  []llmtypes.LLMInvokeRequest
}

func (f *fakeProvider) Invoke(ctx context.Context, req llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	f.requests = append(f.requests, req)
	idx := len(f.requests) - 1
	if idx >= len(f.responses) {
		return nil, fmt.Errorf("no response configured")
	}
	return f.responses[idx], nil
}

func (f *fakeProvider) StreamInvoke(ctx context.Context, req llmtypes.LLMStreamInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return nil, fmt.Errorf("stream invoke not implemented")
}
