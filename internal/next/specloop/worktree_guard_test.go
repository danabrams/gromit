package specloop

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// fakeStage is a test stage that records calls and returns a fixed action.
type fakeStage struct {
	name   string
	action NextAction
	err    error
	called bool
	runFn  func() // optional side-effect executed during Run
}

func (s *fakeStage) Name() string { return s.name }
func (s *fakeStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	s.called = true
	if s.runFn != nil {
		s.runFn()
	}
	return s.action, s.err
}

func TestWorktreeGuard_NoWorktree_Passthrough(t *testing.T) {
	inner := &fakeStage{name: "execute", action: NextAction{Kind: Continue}}
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: "/tmp/repo",
		GitStatus: func(dir string) (string, error) {
			t.Fatal("GitStatus should not be called when no worktree is set")
			return "", nil
		},
	}

	rs := runstore.NewRunState("s1", "p1")
	// WorktreePath is empty — no worktree active.

	action, err := guard.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != Continue {
		t.Fatalf("want Continue, got %v", action.Kind)
	}
	if !inner.called {
		t.Fatal("inner stage should have been called")
	}
}

func TestWorktreeGuard_CleanAfter_Passthrough(t *testing.T) {
	inner := &fakeStage{name: "execute", action: NextAction{Kind: Continue}}
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: "/tmp/repo",
		GitStatus: func(dir string) (string, error) {
			// Same output before and after — no new changes.
			return " M existing.go\n", nil
		},
	}

	rs := runstore.NewRunState("s1", "p1")
	rs.WorktreePath = "/tmp/worktree"

	action, err := guard.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != Continue {
		t.Fatalf("want Continue, got %v", action.Kind)
	}
	if !inner.called {
		t.Fatal("inner stage should have been called")
	}
}

func TestWorktreeGuard_NewFiles_Blocked(t *testing.T) {
	callCount := 0
	inner := &fakeStage{name: "execute", action: NextAction{Kind: Continue}}
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: "/tmp/repo",
		GitStatus: func(dir string) (string, error) {
			callCount++
			if callCount == 1 {
				// Before: clean
				return "", nil
			}
			// After: new file appeared
			return "?? leaked_file.go\n", nil
		},
	}

	rs := runstore.NewRunState("s1", "p1")
	rs.WorktreePath = "/tmp/worktree"

	action, err := guard.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != Blocked {
		t.Fatalf("want Blocked, got %v", action.Kind)
	}
	if action.Context == nil || len(action.Context.Failures) == 0 {
		t.Fatal("expected failure context with message")
	}
	msg := action.Context.Failures[0]
	if msg == "" {
		t.Fatal("expected non-empty failure message")
	}
	t.Logf("blocked message: %s", msg)
}

func TestWorktreeGuard_ModifiedExisting_Blocked(t *testing.T) {
	callCount := 0
	inner := &fakeStage{name: "validate", action: NextAction{Kind: Continue}}
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: "/tmp/repo",
		GitStatus: func(dir string) (string, error) {
			callCount++
			if callCount == 1 {
				return "", nil
			}
			// After: a tracked file was modified in the main repo.
			return " M main_repo_file.go\n", nil
		},
	}

	rs := runstore.NewRunState("s1", "p1")
	rs.WorktreePath = "/tmp/worktree"

	action, err := guard.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != Blocked {
		t.Fatalf("want Blocked, got %v", action.Kind)
	}
}

func TestWorktreeGuard_PreExistingDirty_NotBlocked(t *testing.T) {
	// If the main repo was already dirty before the stage ran,
	// those same files should not trigger blocking.
	inner := &fakeStage{name: "execute", action: NextAction{Kind: Continue}}
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: "/tmp/repo",
		GitStatus: func(dir string) (string, error) {
			// Same dirty file before and after.
			return " M already_dirty.go\n", nil
		},
	}

	rs := runstore.NewRunState("s1", "p1")
	rs.WorktreePath = "/tmp/worktree"

	action, err := guard.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != Continue {
		t.Fatalf("want Continue, got %v", action.Kind)
	}
}

func TestWorktreeGuard_DelegatesName(t *testing.T) {
	inner := &fakeStage{name: "my_stage"}
	guard := &WorktreeGuard{Inner: inner, RepoDir: "/tmp/repo"}
	if guard.Name() != "my_stage" {
		t.Fatalf("want my_stage, got %s", guard.Name())
	}
}

func TestWorktreeGuard_InnerError_PropagatedWhenClean(t *testing.T) {
	inner := &fakeStage{
		name:   "execute",
		action: NextAction{},
		err:    fmt.Errorf("inner boom"),
	}
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: "/tmp/repo",
		GitStatus: func(dir string) (string, error) {
			return "", nil // clean before and after
		},
	}

	rs := runstore.NewRunState("s1", "p1")
	rs.WorktreePath = "/tmp/worktree"

	_, err := guard.Run(context.Background(), rs)
	if err == nil || err.Error() != "inner boom" {
		t.Fatalf("expected inner error propagated, got: %v", err)
	}
}

func TestWorktreeGuard_ViolationTakesPrecedenceOverInnerResult(t *testing.T) {
	// Even if the inner stage returns ReplanFrom, a violation should block.
	callCount := 0
	inner := &fakeStage{
		name:   "validate",
		action: NextAction{Kind: ReplanFrom},
	}
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: "/tmp/repo",
		GitStatus: func(dir string) (string, error) {
			callCount++
			if callCount == 1 {
				return "", nil
			}
			return "?? oops.txt\n", nil
		},
	}

	rs := runstore.NewRunState("s1", "p1")
	rs.WorktreePath = "/tmp/worktree"

	action, err := guard.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != Blocked {
		t.Fatalf("want Blocked (violation), got %v", action.Kind)
	}
}

func TestParseStatusLines(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		expect []string
	}{
		{"empty", "", nil},
		{"untracked", "?? foo.go\n", []string{"foo.go"}},
		{"modified", " M bar.go\n", []string{"bar.go"}},
		{"multiple", "?? a.go\n M b.go\nA  c.go\n", []string{"a.go", "b.go", "c.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseStatusLines(tc.input)
			for _, want := range tc.expect {
				if _, ok := got[want]; !ok {
					t.Errorf("missing expected file %q in parsed output", want)
				}
			}
			if tc.expect == nil && len(got) != 0 {
				t.Errorf("expected empty set, got %v", got)
			}
		})
	}
}
