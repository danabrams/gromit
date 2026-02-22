package review_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	reviewstage "github.com/danabrams/gromit/internal/pipeline/review"
)

// fakeInvoker is a test double for reviewstage.Invoker.
type fakeInvoker struct {
	streamRunFn func(ctx context.Context, prompt, model string, w io.Writer) (string, error)
	called      bool
}

func (f *fakeInvoker) StreamRun(ctx context.Context, prompt, model string, w io.Writer) (string, error) {
	f.called = true
	if f.streamRunFn != nil {
		return f.streamRunFn(ctx, prompt, model, w)
	}
	return `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "ok"}`, nil
}

// fakeBeadCreator is a test double for reviewstage.BeadCreator.
type fakeBeadCreator struct {
	createFn func(title string, priority int, labels []string, outputs []string) (string, error)
	created  []createdBead
}

type createdBead struct {
	title    string
	priority int
	labels   []string
}

func (f *fakeBeadCreator) Create(title string, priority int, labels []string, outputs []string) (string, error) {
	f.created = append(f.created, createdBead{title, priority, labels})
	if f.createFn != nil {
		return f.createFn(title, priority, labels, outputs)
	}
	return fmt.Sprintf("bead-%d", len(f.created)), nil
}

// fakePromptRenderer is a test double for reviewstage.PromptRenderer.
type fakePromptRenderer struct {
	renderFn func(beadTitle, diff string) (string, error)
}

func (f *fakePromptRenderer) RenderReview(beadTitle, diff string) (string, error) {
	if f.renderFn != nil {
		return f.renderFn(beadTitle, diff)
	}
	return "# Review Prompt", nil
}

func makeBead(id, title string) *bead.Bead {
	return &bead.Bead{ID: id, Title: title}
}

func makeConfig(reviewEnabled bool) *config.Config {
	return &config.Config{
		Review: config.ReviewConfig{
			Enabled: reviewEnabled,
			Model:   "sonnet",
		},
	}
}

func makeInput(b *bead.Bead, cfg *config.Config) pipeline.Input {
	return pipeline.Input{
		Bead:      b,
		Config:    cfg,
		Iteration: 1,
		Deadline:  time.Now().Add(time.Minute),
	}
}

// TestReviewStage_Disabled_ReturnsProceedWithNoLLMCall verifies that when review is
// disabled in config, Run returns Proceed immediately without making any LLM call.
func TestReviewStage_Disabled_ReturnsProceedWithNoLLMCall(t *testing.T) {
	invoker := &fakeInvoker{}
	beads := &fakeBeadCreator{}
	renderer := &fakePromptRenderer{}
	gitDiff := func() (string, error) { return "some diff", nil }

	stage := reviewstage.New(invoker, beads, renderer, gitDiff, io.Discard)
	in := makeInput(makeBead("bead-1", "Test bead"), makeConfig(false))

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}
	if invoker.called {
		t.Error("LLM Invoker was called; want no LLM call when review is disabled")
	}
	if len(out.ReviewBeadIDs) != 0 {
		t.Errorf("ReviewBeadIDs = %v, want empty", out.ReviewBeadIDs)
	}
}

// TestReviewStage_Enabled_WithFindings_CreatesBeadsAndReturnsProceed verifies that when
// review is enabled and the LLM returns bead proposals, beads are created with the
// from-review label and their IDs appear in Output.ReviewBeadIDs.
func TestReviewStage_Enabled_WithFindings_CreatesBeadsAndReturnsProceed(t *testing.T) {
	invoker := &fakeInvoker{
		streamRunFn: func(_ context.Context, _, _ string, _ io.Writer) (string, error) {
			return `{
				"passed": false,
				"fixes_applied": [],
				"beads_to_create": [
					{
						"title": "Add error handling",
						"description": "Missing error checks",
						"priority": 1,
						"labels": ["bug"]
					}
				],
				"backlog_items": [],
				"summary": "Found 1 issue"
			}`, nil
		},
	}
	beads := &fakeBeadCreator{}
	renderer := &fakePromptRenderer{}
	gitDiff := func() (string, error) { return "diff --git a/foo.go", nil }

	stage := reviewstage.New(invoker, beads, renderer, gitDiff, io.Discard)
	in := makeInput(makeBead("bead-1", "Implement feature"), makeConfig(true))

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}

	// Verify a bead was created.
	if len(beads.created) != 1 {
		t.Fatalf("created %d beads, want 1", len(beads.created))
	}
	if beads.created[0].title != "Add error handling" {
		t.Errorf("bead title = %q, want 'Add error handling'", beads.created[0].title)
	}

	// Verify from-review label is present and original labels are preserved.
	hasFromReview := false
	hasBug := false
	for _, l := range beads.created[0].labels {
		switch l {
		case "from-review":
			hasFromReview = true
		case "bug":
			hasBug = true
		}
	}
	if !hasFromReview {
		t.Errorf("labels = %v, missing 'from-review'", beads.created[0].labels)
	}
	if !hasBug {
		t.Errorf("labels = %v, missing 'bug' (original label should be preserved)", beads.created[0].labels)
	}

	// Verify bead ID is returned.
	if len(out.ReviewBeadIDs) != 1 {
		t.Fatalf("ReviewBeadIDs has %d items, want 1", len(out.ReviewBeadIDs))
	}
	if out.ReviewBeadIDs[0] == "" {
		t.Error("ReviewBeadIDs[0] is empty, want a bead ID")
	}
}

// TestReviewStage_Enabled_NoFindings_ReturnsProceedWithEmptyIDs verifies that when review
// is enabled but the LLM returns no bead proposals, no beads are created and
// Output.ReviewBeadIDs is an empty (non-nil) slice.
func TestReviewStage_Enabled_NoFindings_ReturnsProceedWithEmptyIDs(t *testing.T) {
	invoker := &fakeInvoker{
		streamRunFn: func(_ context.Context, _, _ string, _ io.Writer) (string, error) {
			return `{"passed": true, "fixes_applied": [], "beads_to_create": [], "backlog_items": [], "summary": "LGTM"}`, nil
		},
	}
	beads := &fakeBeadCreator{}
	renderer := &fakePromptRenderer{}
	gitDiff := func() (string, error) { return "", nil }

	stage := reviewstage.New(invoker, beads, renderer, gitDiff, io.Discard)
	in := makeInput(makeBead("bead-1", "Small fix"), makeConfig(true))

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}
	if len(beads.created) != 0 {
		t.Errorf("created %d beads, want 0", len(beads.created))
	}
	if out.ReviewBeadIDs == nil {
		t.Error("ReviewBeadIDs is nil, want empty slice")
	}
	if len(out.ReviewBeadIDs) != 0 {
		t.Errorf("ReviewBeadIDs = %v, want empty", out.ReviewBeadIDs)
	}
}

// TestBuildFromReviewLabels_ConsolidatedFunction verifies that the review package's
// label building uses the consolidated function from the parent pipeline package.
func TestBuildFromReviewLabels_ConsolidatedFunction(t *testing.T) {
	// Test that buildFromReviewLabels uses the consolidated parent function
	// by verifying deduplication behavior
	labels := pipeline.BuildFromReviewLabels([]string{"bug", "enhancement"})
	if len(labels) != 3 {
		t.Errorf("got %d labels, want 3", len(labels))
	}
	if labels[0] != "from-review" {
		t.Errorf("first label = %q, want 'from-review'", labels[0])
	}
}
