package midreview_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/pipeline/midreview"
)

func TestStage_RunSkipsWhenDisabled(t *testing.T) {
	t.Parallel()

	stage := midreview.NewStage(nil, nil, nil, io.Discard)

	cfg := &config.Config{}
	cfg.MidBuildReview.Enabled = false

	out, err := stage.Run(context.Background(), pipeline.Input{Config: cfg})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Fatalf("Decision = %v, want Proceed", out.Decision)
	}
	if len(out.MidBuildReviewFindings) != 0 {
		t.Fatalf("Findings = %v, want none", out.MidBuildReviewFindings)
	}
}

func TestStage_RunInvokesLLMAndReturnsFindings(t *testing.T) {
	t.Parallel()

	diff := "diff content"
	gitDiffCalled := false
	gitDiff := func(ctx context.Context) (string, error) {
		gitDiffCalled = true
		return diff, nil
	}

	renderer := &fakeRenderer{
		prompt: "mid review prompt",
	}
	expectedTier := "high"
	invoker := &fakeInvoker{
		result: &provider.Result{
			Output: `{
				"findings": [
					{"category": "logging", "message": "fix logging"},
					{"category": "docs", "message": "add docs"}
				],
				"summary": "Need more docs"
			}`,
			Duration:     123 * time.Millisecond,
			CostUSD:      0.27,
			InputTokens:  50,
			OutputTokens: 30,
		},
	}

	cfg := &config.Config{}
	cfg.MidBuildReview.Enabled = true
	cfg.MidBuildReview.Tier = expectedTier
	cfg.MidBuildReview.Timeout = config.DurationSeconds(5 * time.Second)

	beadObj := &bead.Bead{
		Title:              "Fix issue",
		Description:        "Fix the thing",
		AcceptanceCriteria: "- ensure tests pass",
	}

	stage := midreview.NewStage(renderer, invoker, gitDiff, io.Discard)

	out, err := stage.Run(context.Background(), pipeline.Input{
		Bead:   beadObj,
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !gitDiffCalled {
		t.Fatal("git diff was not called")
	}
	if renderer.ctx == nil {
		t.Fatal("renderer context was not set")
	}
	if renderer.ctx.Diff != diff {
		t.Fatalf("renderer diff = %q, want %q", renderer.ctx.Diff, diff)
	}
	if renderer.ctx.BeadTitle != beadObj.Title {
		t.Fatalf("renderer bead title = %q, want %q", renderer.ctx.BeadTitle, beadObj.Title)
	}
	if renderer.ctx.BeadDescription != beadObj.Description {
		t.Fatalf("renderer bead description = %q, want %q", renderer.ctx.BeadDescription, beadObj.Description)
	}

	if !invoker.called {
		t.Fatal("invoker was not called")
	}
	if invoker.tier != expectedTier {
		t.Fatalf("invoker tier = %q, want %q", invoker.tier, expectedTier)
	}

	if out.Decision != pipeline.Proceed {
		t.Fatalf("Decision = %v, want Proceed", out.Decision)
	}
	if len(out.MidBuildReviewFindings) != 2 {
		t.Fatalf("Findings = %v, want 2 items", out.MidBuildReviewFindings)
	}
	if out.DurationMs != 123 {
		t.Fatalf("DurationMs = %d, want %d", out.DurationMs, 123)
	}
	if out.CostUSD != 0.27 {
		t.Fatalf("CostUSD = %f, want %f", out.CostUSD, 0.27)
	}
	if out.InputTokens != 50 {
		t.Fatalf("InputTokens = %d, want %d", out.InputTokens, 50)
	}
	if out.OutputTokens != 30 {
		t.Fatalf("OutputTokens = %d, want %d", out.OutputTokens, 30)
	}
}

type fakeRenderer struct {
	prompt string
	ctx    *midreview.MidBuildReviewContext
}

func (f *fakeRenderer) RenderMidBuildReview(ctx *midreview.MidBuildReviewContext) (string, error) {
	f.ctx = ctx
	return f.prompt, nil
}

type fakeInvoker struct {
	called bool
	tier   string
	prompt string
	ctx    context.Context
	result *provider.Result
	err    error
}

func (f *fakeInvoker) StreamRun(ctx context.Context, prompt, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	f.called = true
	f.ctx = ctx
	f.prompt = prompt
	f.tier = tier
	return f.result, f.err
}
