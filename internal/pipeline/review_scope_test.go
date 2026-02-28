package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestResolveReviewScopePriorityChain(t *testing.T) {
	ctx := context.Background()

	t.Run("since flag returns its value before touching tracker or state", func(t *testing.T) {
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
		fakeGit := newFakeGitExecutor(t)
		specsDir := t.TempDir()
		specContent := `---
id: epic-spec
epic: priority-epic
created: 2026-02-29
---

# Epic Spec
`
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
		if got, want := fakeGit.CommandCount(), 1; got != want {
			t.Fatalf("expected 1 git command for epic resolution, got %d", got)
		}
		if !reflect.DeepEqual(tracker.Labels(), []string{"spec:epic-spec"}) {
			t.Fatalf("tracker labels = %v, want %v", tracker.Labels(), []string{"spec:epic-spec"})
		}
	})

	t.Run("no flags uses last review commit from state", func(t *testing.T) {
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

type fakeGitExecutor struct {
	t            *testing.T
	mu           sync.Mutex
	commands     [][]string
	outputs      map[string][]byte
	prevCmdFn    func(name string, args ...string) *exec.Cmd
	prevOutputFn func(cmd *exec.Cmd) ([]byte, error)
}

func newFakeGitExecutor(t *testing.T) *fakeGitExecutor {
	t.Helper()
	f := &fakeGitExecutor{
		t:       t,
		outputs: map[string][]byte{},
	}
	f.prevCmdFn = reviewScopeGitCommandFn
	f.prevOutputFn = reviewScopeGitOutputFn

	reviewScopeGitCommandFn = func(name string, args ...string) *exec.Cmd {
		cmd := &exec.Cmd{Path: name, Args: append([]string{name}, args...)}
		f.mu.Lock()
		f.commands = append(f.commands, cmd.Args)
		f.mu.Unlock()
		return cmd
	}

	reviewScopeGitOutputFn = func(cmd *exec.Cmd) ([]byte, error) {
		key := strings.Join(cmd.Args, "\x00")
		f.mu.Lock()
		defer f.mu.Unlock()
		out, ok := f.outputs[key]
		if !ok {
			return nil, fmt.Errorf("fake git: unexpected command %q", cmd.Args)
		}
		return append([]byte(nil), out...), nil
	}

	t.Cleanup(func() {
		reviewScopeGitCommandFn = f.prevCmdFn
		reviewScopeGitOutputFn = f.prevOutputFn
	})

	return f
}

func (f *fakeGitExecutor) RegisterOutput(output string, args ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.Join(args, "\x00")
	f.outputs[key] = append([]byte(nil), output...)
}

func (f *fakeGitExecutor) CommandCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.commands)
}

type fakeTracker struct {
	t     *testing.T
	mu    sync.Mutex
	beads map[string][]string
	calls []string
}

func newFakeTracker(t *testing.T, beads map[string][]string) *fakeTracker {
	t.Helper()
	copy := make(map[string][]string, len(beads))
	for k, v := range beads {
		copy[k] = append([]string(nil), v...)
	}
	return &fakeTracker{t: t, beads: copy}
}

func (f *fakeTracker) Ready(ctx context.Context) (*BeadInfo, error) {
	f.t.Fatalf("fakeTracker.Ready called unexpectedly")
	return nil, nil
}

func (f *fakeTracker) Show(ctx context.Context, id string) (*BeadInfo, error) {
	f.t.Fatalf("fakeTracker.Show called unexpectedly")
	return nil, nil
}

func (f *fakeTracker) Create(ctx context.Context, title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
	f.t.Fatalf("fakeTracker.Create called unexpectedly")
	return nil, nil
}

func (f *fakeTracker) CreateWithDepsAndDescription(ctx context.Context, title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
	f.t.Fatalf("fakeTracker.CreateWithDepsAndDescription called unexpectedly")
	return nil, nil
}

func (f *fakeTracker) Close(ctx context.Context, id string) error {
	f.t.Fatalf("fakeTracker.Close called unexpectedly")
	return nil
}

func (f *fakeTracker) ListWithLabel(ctx context.Context, label string) ([]string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, label)
	f.mu.Unlock()
	if beads, ok := f.beads[label]; ok {
		return append([]string(nil), beads...), nil
	}
	return nil, nil
}

func (f *fakeTracker) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeTracker) Labels() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

type fakeStateManager struct {
	t     *testing.T
	mu    sync.Mutex
	calls int
	fn    func() (string, error)
}

func newFakeStateManager(t *testing.T, fn func() (string, error)) *fakeStateManager {
	t.Helper()
	if fn == nil {
		fn = func() (string, error) { return "", nil }
	}
	return &fakeStateManager{t: t, fn: fn}
}

func (f *fakeStateManager) GetLastReviewCommit() (string, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.fn()
}

func (f *fakeStateManager) SetLastReviewCommit(commit string) error {
	return nil
}

func (f *fakeStateManager) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}
