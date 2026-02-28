package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveReviewScopePriorityChain(t *testing.T) {
	ctx := context.Background()

	t.Run("since flag returns its value before touching tracker or state", func(t *testing.T) {
		t.Parallel()
		fakeGit := newFakeGitExecutor(t)
		tracker := newFakeTracker(t, nil)
		state := newFakeStateManager(t, func() (string, error) {
			t.Fatalf("state should not be called when --since is provided")
			return "", nil
		})

		deps := &Deps{
			TrackerClient: tracker,
			StateManager:  state,
		}
		p := New(deps, &Paths{})

		const sinceCommit = "since-commit"
		commit, err := p.ResolveReviewScope(ctx, "unused-spec", "unused-epic", sinceCommit)
		if err != nil {
			t.Fatalf("ResolveReviewScope returned error: %v", err)
		}
		if commit != sinceCommit {
			t.Fatalf("ResolveReviewScope returned %q, want %q", commit, sinceCommit)
		}
		if got, want := fakeGit.CommandCount(), 0; got != want {
			t.Fatalf("expected no git commands when --since is provided, got %d", got)
		}
		if got, want := tracker.CallCount(), 0; got != want {
			t.Fatalf("tracker was called unexpectedly %d times", got)
		}
		if got, want := state.CallCount(), 0; got != want {
			t.Fatalf("state was called unexpectedly %d times", got)
		}
	})

	t.Run("spec flag resolves earliest commit using git output", func(t *testing.T) {
		t.Parallel()
		fakeGit := newFakeGitExecutor(t)
		tracker := newFakeTracker(t, map[string][]string{
			"spec:priority": {"bead-one", "bead-two"},
		})
		state := newFakeStateManager(t, func() (string, error) {
			t.Fatalf("state should not be called when --spec is provided")
			return "", nil
		})

		const (
			beadOneOldCommit = "bead-one-old"
			beadTwoCommit    = "bead-two-earliest"
		)

		fakeGit.RegisterOutput("bead-one-new\nbead-one-old", "git", "log", "--all", "--format=%H", "--grep", "bead-one", "--fixed-strings")
		fakeGit.RegisterOutput(beadTwoCommit, "git", "log", "--all", "--format=%H", "--grep", "bead-two", "--fixed-strings")
		fakeGit.RegisterOutput("200", "git", "log", "-1", "--format=%at", beadOneOldCommit, "--")
		fakeGit.RegisterOutput("100", "git", "log", "-1", "--format=%at", beadTwoCommit, "--")

		deps := &Deps{
			TrackerClient: tracker,
			StateManager:  state,
		}
		p := New(deps, &Paths{})

		commit, err := p.ResolveReviewScope(ctx, "priority", "", "")
		if err != nil {
			t.Fatalf("ResolveReviewScope returned error: %v", err)
		}
		if commit != beadTwoCommit {
			t.Fatalf("ResolveReviewScope returned %q, want %q", commit, beadTwoCommit)
		}
		if got, want := fakeGit.CommandCount(), 4; got != want {
			t.Fatalf("expected 4 git commands for spec resolution, got %d", got)
		}
		if !reflect.DeepEqual(tracker.Labels(), []string{"spec:priority"}) {
			t.Fatalf("tracker labels = %v, want %v", tracker.Labels(), []string{"spec:priority"})
		}
	})

	t.Run("epic flag resolves earliest commit across specs", func(t *testing.T) {
		t.Parallel()
		fakeGit := newFakeGitExecutor(t)
		specsDir := t.TempDir()
		specContent := `---
id: epic-spec
epic: priority-epic
---`
		if err := os.WriteFile(filepath.Join(specsDir, "epic-spec.md"), []byte(specContent), 0o644); err != nil {
			t.Fatalf("failed to write spec file: %v", err)
		}

		tracker := newFakeTracker(t, map[string][]string{
			"spec:epic-spec": {"epic-bead"},
		})
		state := newFakeStateManager(t, func() (string, error) {
			t.Fatalf("state should not be called when --epic is provided")
			return "", nil
		})

		const epicCommit = "epic-bead-old"
		fakeGit.RegisterOutput("epic-bead-new\n"+epicCommit, "git", "log", "--all", "--format=%H", "--grep", "epic-bead", "--fixed-strings")
		fakeGit.RegisterOutput("50", "git", "log", "-1", "--format=%at", epicCommit, "--")

		deps := &Deps{
			TrackerClient: tracker,
			StateManager:  state,
		}
		p := New(deps, &Paths{SpecsDir: specsDir})

		commit, err := p.ResolveReviewScope(ctx, "", "priority-epic", "")
		if err != nil {
			t.Fatalf("ResolveReviewScope returned error: %v", err)
		}
		if commit != epicCommit {
			t.Fatalf("ResolveReviewScope returned %q, want %q", commit, epicCommit)
		}
		if got, want := fakeGit.CommandCount(), 2; got != want {
			t.Fatalf("expected 2 git commands for epic resolution, got %d", got)
		}
		if !reflect.DeepEqual(tracker.Labels(), []string{"spec:epic-spec"}) {
			t.Fatalf("tracker labels = %v, want %v", tracker.Labels(), []string{"spec:epic-spec"})
		}
	})

	t.Run("no flags uses last review commit from state", func(t *testing.T) {
		t.Parallel()
		fakeGit := newFakeGitExecutor(t)
		state := newFakeStateManager(t, func() (string, error) {
			return "state-commit", nil
		})

		deps := &Deps{
			StateManager: state,
		}
		p := New(deps, &Paths{})

		commit, err := p.ResolveReviewScope(ctx, "", "", "")
		if err != nil {
			t.Fatalf("ResolveReviewScope returned error: %v", err)
		}
		if commit != "state-commit" {
			t.Fatalf("ResolveReviewScope returned %q, want %q", commit, "state-commit")
		}
		if got, want := fakeGit.CommandCount(), 0; got != want {
			t.Fatalf("expected no git commands when state fallback runs, got %d", got)
		}
		if got, want := state.CallCount(), 1; got != want {
			t.Fatalf("state.GetLastReviewCommit called %d times, want 1", got)
		}
	})
}
