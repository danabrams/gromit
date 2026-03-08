package present

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
	v2review "github.com/danabrams/gromit/internal/v2/review"
	"github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/testutil"
)

func TestPresentStageCallsPresenter(t *testing.T) {
	specID := "spec-123"
	ctx := &SummaryContext{
		Plan:               "plan details",
		Worktree:           "/tmp/worktree",
		BeadSummaries:      []presentation.BeadSummary{{ID: "bead-1", Title: "First bead", Description: "desc1"}},
		AcceptanceResults:  []presentation.AcceptanceResult{{Title: "Criterion", Description: "desc"}},
		OutOfScopeFindings: []v2review.Finding{{Title: "OOS", Description: "desc", AffectedFiles: []string{"README.md"}}},
		BranchLink:         "https://example.com/branch",
		DiffLink:           "https://example.com/diff",
		Success:            true,
		IntegrationBranch:  "integration-main",
	}
	presenter := &spyPresenter{}
	stageInstance, err := New(nil, presenter, ctx)
	if err != nil {
		t.Fatalf("unexpected error creating stage: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stage.Request{Bead: stage.BeadInfo{ID: specID}})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if res == nil || res.Decision != stage.DecisionProceed {
		t.Fatalf("unexpected decision: %#v", res)
	}
	if presenter.lastSpec != specID {
		t.Fatalf("presenter called with spec %q", presenter.lastSpec)
	}

	summary := presenter.lastSummary
	if summary.SpecName != specID {
		t.Fatalf("spec name = %q", summary.SpecName)
	}
	if summary.SpecBranch != presentation.SpecBranchName(specID) {
		t.Fatalf("spec branch = %q", summary.SpecBranch)
	}
	if summary.IntegrationBranch != ctx.IntegrationBranch {
		t.Fatalf("integration branch = %q", summary.IntegrationBranch)
	}
	if summary.Plan != ctx.Plan || summary.Worktree != ctx.Worktree {
		t.Fatalf("plan/worktree mismatch: %v", summary)
	}
	if summary.BranchLink != ctx.BranchLink || summary.DiffLink != ctx.DiffLink {
		t.Fatalf("links mismatch: %v", summary)
	}
	if len(summary.BeadSummaries) != len(ctx.BeadSummaries) {
		t.Fatalf("expected bead summaries")
	}
	if len(summary.AcceptanceResults) != len(ctx.AcceptanceResults) {
		t.Fatalf("expected acceptance results")
	}
	if len(summary.OutOfScopeFindings) != len(ctx.OutOfScopeFindings) {
		t.Fatalf("expected out-of-scope findings")
	}
	if !summary.Success {
		t.Fatalf("expected success summary")
	}
}

var _ adapter.PresenterAdapter = (*spyPresenter)(nil)

func TestPresentStageHandlesPresenterError(t *testing.T) {
	presenter := &spyPresenter{err: errors.New("boom")}
	ctx := &SummaryContext{
		Plan:              "plan details",
		Worktree:          "/tmp/worktree",
		Success:           false,
		FailureSummary:    "oops",
		RemainingWork:     []string{"todo"},
		IntegrationBranch: "integration-main",
	}
	stageInstance, err := New(nil, presenter, ctx)
	if err != nil {
		t.Fatalf("unexpected error creating stage: %v", err)
	}

	res, err := stageInstance.Run(context.Background(), &stage.Request{Bead: stage.BeadInfo{ID: "spec-error"}})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if res != nil {
		t.Fatalf("expected nil result on presenter error, got %#v", res)
	}
	if presenter.lastSpec != "spec-error" {
		t.Fatalf("presenter not invoked on failure")
	}
}

type spyPresenter struct {
	lastSpec    string
	lastSummary presentation.PresentationSummary
	err         error
}

func (s *spyPresenter) PresentSummary(ctx context.Context, specID string, summary presentation.PresentationSummary) error {
	s.lastSpec = specID
	s.lastSummary = summary
	return s.err
}

func TestPresentStageCallsSquasher(t *testing.T) {
	called := false
	squasher := func(ctx context.Context) error {
		called = true
		return nil
	}
	ctx := &SummaryContext{Plan: "p", Worktree: "/tmp/w"}
	presenter := &spyPresenter{}
	stageInstance, err := New(nil, presenter, ctx, WithSquasher(squasher))
	if err != nil {
		t.Fatalf("unexpected error creating stage: %v", err)
	}
	if _, err := stageInstance.Run(context.Background(), &stage.Request{Bead: stage.BeadInfo{ID: "spec-sq"}}); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !called {
		t.Fatal("squasher was not called")
	}
}

func TestPresentStageSquasherErrorPropagates(t *testing.T) {
	squashErr := errors.New("squash failed")
	squasher := func(ctx context.Context) error { return squashErr }
	ctx := &SummaryContext{Plan: "p", Worktree: "/tmp/w"}
	presenter := &spyPresenter{}
	stageInstance, err := New(nil, presenter, ctx, WithSquasher(squasher))
	if err != nil {
		t.Fatalf("unexpected error creating stage: %v", err)
	}
	_, runErr := stageInstance.Run(context.Background(), &stage.Request{Bead: stage.BeadInfo{ID: "spec-sq-err"}})
	if runErr == nil {
		t.Fatal("expected error from squasher, got nil")
	}
	if presenter.lastSpec != "" {
		t.Fatal("presenter should not be called when squasher errors")
	}
}

func TestPresentStageNilSquasherIsBackwardCompatible(t *testing.T) {
	ctx := &SummaryContext{Plan: "p", Worktree: "/tmp/w"}
	presenter := &spyPresenter{}
	stageInstance, err := New(nil, presenter, ctx)
	if err != nil {
		t.Fatalf("unexpected error creating stage: %v", err)
	}
	_, runErr := stageInstance.Run(context.Background(), &stage.Request{Bead: stage.BeadInfo{ID: "spec-no-sq"}})
	if runErr != nil {
		t.Fatalf("expected no error without squasher, got: %v", runErr)
	}
	if presenter.lastSpec != "spec-no-sq" {
		t.Fatal("presenter was not called without squasher")
	}
}

func TestPresentStageTrimsLinks(t *testing.T) {
	ctx := &SummaryContext{
		Plan:              "plan details",
		Worktree:          "/tmp/worktree",
		BranchLink:        "\nhttps://example.com/branch\n",
		DiffLink:          "  https://example.com/diff  ",
		IntegrationBranch: "integration-main",
	}
	presenter := &spyPresenter{}
	stageInstance, err := New(nil, presenter, ctx)
	if err != nil {
		t.Fatalf("unexpected error creating stage: %v", err)
	}

	if _, err := stageInstance.Run(context.Background(), &stage.Request{Bead: stage.BeadInfo{ID: "spec-links"}}); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if got, want := presenter.lastSummary.BranchLink, "https://example.com/branch"; got != want {
		t.Fatalf("branch link = %q; want %q", got, want)
	}
	if got, want := presenter.lastSummary.DiffLink, "https://example.com/diff"; got != want {
		t.Fatalf("diff link = %q; want %q", got, want)
	}
}

func TestPresentStageSquashesPerBeadBeforePresenter(t *testing.T) {
	git := testutil.NewFakeGit()
	git.LogEntries = []adapter.LogEntry{
		{Hash: "h2", Message: "[bead:001/review/iter:1] Proceed"},
		{Hash: "h1", Message: "[bead:001/build/iter:1] Proceed"},
		{Hash: "h0", Message: "initial commit"},
	}
	ctx := &SummaryContext{
		Plan:          "plan details",
		Worktree:      "/tmp/worktree",
		BeadSummaries: []presentation.BeadSummary{{ID: "001", Title: "Feature"}},
	}
	presenter := &spyPresenter{}
	stageInstance, err := New(nil, presenter, ctx, WithSquashGit(git))
	if err != nil {
		t.Fatalf("unexpected error creating stage: %v", err)
	}

	if _, err := stageInstance.Run(context.Background(), &stage.Request{Bead: stage.BeadInfo{ID: "spec-squash"}}); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if len(git.SquashCalls) == 0 {
		t.Fatal("expected squash to be called before presenting")
	}
	if len(git.CommitMessages) == 0 || git.CommitMessages[0] != "bead 001: Feature" {
		t.Fatalf("commit messages = %v, want first commit bead message", git.CommitMessages)
	}
	if presenter.lastSpec != "spec-squash" {
		t.Fatalf("presenter called with spec %q", presenter.lastSpec)
	}
}
