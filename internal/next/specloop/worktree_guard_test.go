package specloop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func TestWorktreeGuard_StrayFiles_MovedToWorktreeAndContinues(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()
	strayRel := "internal/pkg/foo_test.go"
	strayContent := []byte("package pkg")

	// Write stray file to main repo before the stage runs (simulating the stage's side effect).
	strayPath := filepath.Join(repoDir, strayRel)
	if err := os.MkdirAll(filepath.Dir(strayPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strayPath, strayContent, 0644); err != nil {
		t.Fatal(err)
	}

	inner := &fakeStage{name: "write_scenario_tests", action: NextAction{Kind: Continue}}

	callCount := 0
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: repoDir,
		GitStatus: func(dir string) (string, error) {
			callCount++
			if callCount == 1 {
				return "", nil // clean before
			}
			return "?? " + strayRel + "\n", nil // stray file after
		},
	}

	rs := &runstore.RunState{WorktreePath: worktreeDir}
	action, err := guard.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}
	// Stray file gone from main repo.
	if _, err := os.Stat(filepath.Join(repoDir, strayRel)); !os.IsNotExist(err) {
		t.Error("stray file should be removed from main repo")
	}
	// File exists in worktree.
	data, err := os.ReadFile(filepath.Join(worktreeDir, strayRel))
	if err != nil {
		t.Fatalf("file not moved to worktree: %v", err)
	}
	if string(data) != string(strayContent) {
		t.Errorf("content mismatch: got %q, want %q", data, strayContent)
	}
}

func TestWorktreeGuard_StrayFiles_MoveFailsBlocksFallback(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()
	strayRel := "internal/pkg/foo_test.go"

	// Write stray file.
	strayPath := filepath.Join(repoDir, strayRel)
	os.MkdirAll(filepath.Dir(strayPath), 0755)
	os.WriteFile(strayPath, []byte("package pkg"), 0644)

	// Make worktreeDir unwritable so move fails.
	os.Chmod(worktreeDir, 0555)
	defer os.Chmod(worktreeDir, 0755)

	inner := &fakeStage{name: "write_scenario_tests", action: NextAction{Kind: Continue}}

	callCount := 0
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: repoDir,
		GitStatus: func(dir string) (string, error) {
			callCount++
			if callCount == 1 {
				return "", nil
			}
			return "?? " + strayRel + "\n", nil
		},
	}

	rs := &runstore.RunState{WorktreePath: worktreeDir}
	action, err := guard.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != Blocked {
		t.Errorf("expected Blocked on move failure, got %v", action.Kind)
	}
}

func TestWorktreeGuard_StrayFiles_InnerReplanPreservedOnSuccessfulMove(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()
	strayRel := "pkg/bar_test.go"

	strayPath := filepath.Join(repoDir, strayRel)
	os.MkdirAll(filepath.Dir(strayPath), 0755)
	os.WriteFile(strayPath, []byte("package pkg"), 0644)

	inner := &fakeStage{name: "execute", action: NextAction{Kind: ReplanFrom}}

	callCount := 0
	guard := &WorktreeGuard{
		Inner:   inner,
		RepoDir: repoDir,
		GitStatus: func(dir string) (string, error) {
			callCount++
			if callCount == 1 {
				return "", nil
			}
			return "?? " + strayRel + "\n", nil
		},
	}

	rs := &runstore.RunState{WorktreePath: worktreeDir}
	action, err := guard.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != ReplanFrom {
		t.Errorf("expected ReplanFrom preserved, got %v", action.Kind)
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
			got := ParseStatusLines(tc.input)
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
