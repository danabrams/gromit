# Worktree Isolation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace noopGitOps with real git worktrees and redirect all pipeline stages to execute in the worktree.

**Architecture:** The init stage already creates a branch name and calls `GitOps.CreateWorktree`, storing the result in `rs.WorktreePath`. Today `noopGitOps` just copies the repo to a temp dir, and downstream stages ignore `WorktreePath` — they use the original `cfg.WorkDir`. The fix: (1) write a `realGitOps` that calls `git worktree add`, (2) make `ProviderTaskRunner` and `ShellTaskInspector` resolve workDir lazily via a `func() string` that reads `rs.WorktreePath`, (3) have execute and validate stages use `rs.WorktreePath` at runtime, (4) fix `lazyDiffProvider` to prefer `WorktreePath`.

**Tech Stack:** Go, git CLI

---

### Task 1: Write realGitOps — failing test

**Files:**
- Create: `cmd/gromit-next/real_git_ops_test.go`

**Step 1: Write the failing test**

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRealGitOps_CreateWorktree(t *testing.T) {
	// Create a real git repo for testing
	repoDir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	run("init")
	run("commit", "--allow-empty", "-m", "initial")

	ops := &realGitOps{}
	branch := "gromit/test-branch"
	worktreePath, err := ops.CreateWorktree(repoDir, branch)
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer os.RemoveAll(worktreePath)

	// Worktree directory should exist
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatal("worktree directory does not exist")
	}

	// Should be on the correct branch
	cmd := exec.Command("git", "-C", worktreePath, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch: %v", err)
	}
	if got := string(out[:len(out)-1]); got != branch {
		t.Errorf("branch = %q, want %q", got, branch)
	}
}

func TestRealGitOps_RemoveWorktree(t *testing.T) {
	repoDir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	run("init")
	run("commit", "--allow-empty", "-m", "initial")

	ops := &realGitOps{}
	worktreePath, err := ops.CreateWorktree(repoDir, "gromit/test-remove")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	err = ops.RemoveWorktree(worktreePath)
	if err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}
}

func TestRealGitOps_WorktreeHasRepoContents(t *testing.T) {
	repoDir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	run("init")
	os.WriteFile(filepath.Join(repoDir, "hello.txt"), []byte("hello"), 0o644)
	run("add", "hello.txt")
	run("commit", "-m", "add hello")

	ops := &realGitOps{}
	worktreePath, err := ops.CreateWorktree(repoDir, "gromit/test-contents")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer ops.RemoveWorktree(worktreePath)

	data, err := os.ReadFile(filepath.Join(worktreePath, "hello.txt"))
	if err != nil {
		t.Fatalf("read hello.txt in worktree: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("hello.txt = %q, want %q", string(data), "hello")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/gromit-next/ -run TestRealGitOps -v -count=1`
Expected: FAIL — `realGitOps` type does not exist

---

### Task 2: Write realGitOps — implementation

**Files:**
- Create: `cmd/gromit-next/real_git_ops.go`

**Step 1: Write minimal implementation**

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// realGitOps implements stages.GitOps using real git worktree commands.
type realGitOps struct{}

// CreateWorktree creates a git worktree at a temp directory on the given branch.
// The worktree is created under the system temp dir with a predictable prefix.
func (r *realGitOps) CreateWorktree(repoDir, branch string) (string, error) {
	dir, err := os.MkdirTemp("", "gromit-worktree-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// git worktree add needs the target dir to not exist
	os.Remove(dir)

	cmd := exec.Command("git", "-C", repoDir, "worktree", "add", "-b", branch, dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("git worktree add: %s: %w", string(out), err)
	}
	return dir, nil
}

// RemoveWorktree removes a git worktree and its directory.
func (r *realGitOps) RemoveWorktree(path string) error {
	// Find the main repo by reading the .git file in the worktree
	gitFile := filepath.Join(path, ".git")
	if _, err := os.Stat(gitFile); err == nil {
		// This is a worktree — use git worktree remove
		// We need to find the main repo. Read the .git file which contains "gitdir: <path>"
		data, err := os.ReadFile(gitFile)
		if err == nil {
			// Parse "gitdir: /path/to/repo/.git/worktrees/<name>"
			// Go up from worktrees/<name> to get the repo .git dir, then up once more
			// But simpler: just run git worktree remove from the worktree itself
			cmd := exec.Command("git", "-C", path, "worktree", "remove", "--force", path)
			if out, err := cmd.CombinedOutput(); err != nil {
				// Fallback to rm if git worktree remove fails
				_ = out
				return os.RemoveAll(path)
			}
			return nil
		}
	}
	// Fallback: just remove the directory
	return os.RemoveAll(path)
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./cmd/gromit-next/ -run TestRealGitOps -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/gromit-next/real_git_ops.go cmd/gromit-next/real_git_ops_test.go
git commit -m "feat: add realGitOps using git worktree add/remove"
```

---

### Task 3: Make ProviderTaskRunner use lazy workDir — failing test

**Files:**
- Modify: `internal/next/specloop/provider_taskrunner.go`
- Modify: `internal/next/specloop/provider_taskrunner_test.go` (create if not exists)

**Step 1: Write the failing test**

Find or create `provider_taskrunner_test.go`:

```go
package specloop

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestProviderTaskRunner_UsesWorkDirFn(t *testing.T) {
	called := ""
	invoker := &fakeInvoker{
		invokeInDirFn: func(ctx context.Context, prompt, dir string) {
			called = dir
		},
	}
	runner := NewProviderTaskRunner(invoker, func() string { return "/worktree/path" })
	task := runstore.Task{TaskID: "t1", Objective: "test"}
	task.NormalizeNilFields()
	runner.RunTask(context.Background(), task)

	if called != "/worktree/path" {
		t.Errorf("InvokeInDir called with dir=%q, want %q", called, "/worktree/path")
	}
}
```

Note: You'll need a `fakeInvoker` that records the dir passed to `InvokeInDir`. Check if one already exists in the test files. The test verifies the runner calls `workDirFn()` at invocation time, not at construction time.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/specloop/ -run TestProviderTaskRunner_UsesWorkDirFn -v -count=1`
Expected: FAIL — `NewProviderTaskRunner` signature doesn't accept `func() string`

---

### Task 4: Make ProviderTaskRunner use lazy workDir — implementation

**Files:**
- Modify: `internal/next/specloop/provider_taskrunner.go`
- Modify: `cmd/gromit-next/stage_provider.go` (update call site)

**Step 1: Change ProviderTaskRunner to use workDirFn**

In `provider_taskrunner.go`, change:
```go
type ProviderTaskRunner struct {
	invoker   llmadapter.Invoker
	workDirFn func() string
}

func NewProviderTaskRunner(invoker llmadapter.Invoker, workDirFn func() string) *ProviderTaskRunner {
	return &ProviderTaskRunner{invoker: invoker, workDirFn: workDirFn}
}

func (r *ProviderTaskRunner) invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	dir := r.workDirFn()
	if dir != "" {
		return r.invoker.InvokeInDir(ctx, prompt, dir)
	}
	return r.invoker.Invoke(ctx, prompt)
}
```

**Step 2: Update call site in stage_provider.go**

In `stage_provider.go`, line 131, change:
```go
// Old:
taskRunner = specloop.NewProviderTaskRunner(execAdapter, p.cfg.WorkDir)
// New:
taskRunner = specloop.NewProviderTaskRunner(execAdapter, func() string {
    if rs.WorktreePath != "" {
        return rs.WorktreePath
    }
    return p.cfg.WorkDir
})
```

**Step 3: Run tests**

Run: `go test ./internal/next/specloop/ -run TestProviderTaskRunner -v -count=1 && go test ./cmd/gromit-next/ -v -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/next/specloop/provider_taskrunner.go cmd/gromit-next/stage_provider.go
git commit -m "feat: ProviderTaskRunner resolves workDir lazily from RunState.WorktreePath"
```

---

### Task 5: Make ShellTaskInspector use lazy workDir

**Files:**
- Modify: `internal/next/specloop/shell_task_inspector.go`
- Modify: `cmd/gromit-next/stage_provider.go` (update call site)

**Step 1: Change ShellTaskInspector to use workDirFn**

Same pattern as ProviderTaskRunner:
```go
type ShellTaskInspector struct {
	workDirFn func() string
}

func NewShellTaskInspector(workDirFn func() string) *ShellTaskInspector {
	return &ShellTaskInspector{workDirFn: workDirFn}
}
```

Update all uses of `s.workDir` to `s.workDirFn()`.

**Step 2: Update call site in stage_provider.go**

Line 193:
```go
// Old:
Inspector: specloop.NewShellTaskInspector(p.cfg.WorkDir),
// New:
Inspector: specloop.NewShellTaskInspector(func() string {
    if rs.WorktreePath != "" {
        return rs.WorktreePath
    }
    return p.cfg.WorkDir
}),
```

**Step 3: Run tests**

Run: `go test ./internal/next/specloop/ -v -count=1 && go test ./cmd/gromit-next/ -v -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/next/specloop/shell_task_inspector.go cmd/gromit-next/stage_provider.go
git commit -m "feat: ShellTaskInspector resolves workDir lazily from RunState.WorktreePath"
```

---

### Task 6: Redirect ExecuteStage to use rs.WorktreePath

**Files:**
- Modify: `internal/next/specloop/stages/execute.go`

**Step 1: Write failing test**

In `internal/next/specloop/stages/execute_test.go` (create or extend):

```go
func TestExecuteStage_UsesWorktreePath(t *testing.T) {
	// Create a RunState with WorktreePath set
	rs := runstore.NewRunState("spec-1", "proj-1")
	rs.WorktreePath = "/worktree/dir"
	rs.Tasks = []runstore.Task{{TaskID: "t1", Status: "pending", Objective: "test"}}
	rs.Tasks[0].NormalizeNilFields()

	recorded := ""
	runner := &workDirCapturingRunner{captureFn: func(dir string) { recorded = dir }}

	stage := NewExecuteStage(runner, ExecuteStageConfig{
		WorkDir: "/original/dir",
	})
	stage.Run(context.Background(), rs)

	// The task loop should have received the worktree path, not the original
	// This is verified via TaskLoopConfig.WorkDir
}
```

Note: The exact assertion depends on how TaskLoopConfig.WorkDir flows. The key behavior: if `rs.WorktreePath` is non-empty, `TaskLoopConfig.WorkDir` should use it.

**Step 2: Implement**

In `execute.go`, at the top of `Run()`:
```go
func (s *ExecuteStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	workDir := s.cfg.WorkDir
	if rs.WorktreePath != "" {
		workDir = rs.WorktreePath
	}
	results, err := specloop.RunTaskLoop(ctx, pendingTasks(rs.Tasks), s.runner, specloop.TaskLoopConfig{
		// ... existing fields ...
		WorkDir: workDir,
		// ...
	})
```

**Step 3: Run tests**

Run: `go test ./internal/next/specloop/stages/ -v -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/next/specloop/stages/execute.go
git commit -m "feat: ExecuteStage uses rs.WorktreePath when available"
```

---

### Task 7: Redirect ValidateStage to use rs.WorktreePath

**Files:**
- Modify: `internal/next/specloop/stages/validate.go`

**Step 1: Implement**

In `validate.go`, at the top of `Run()`:
```go
func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	workDir := s.cfg.WorkDir
	if rs.WorktreePath != "" {
		workDir = rs.WorktreePath
	}
	result, err := s.validator.RunFinal(ctx, s.cfg.AlwaysRun, s.cfg.ProjectChecks, workDir)
```

**Step 2: Run tests**

Run: `go test ./internal/next/specloop/stages/ -run TestValidate -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/next/specloop/stages/validate.go
git commit -m "feat: ValidateStage uses rs.WorktreePath when available"
```

---

### Task 8: Fix lazyDiffProvider to prefer WorktreePath

**Files:**
- Modify: `cmd/gromit-next/stage_provider.go`

**Step 1: Swap priority in lazyDiffProvider.Diff()**

```go
func (l *lazyDiffProvider) Diff(baseBranch string) (string, error) {
	// Prefer WorktreePath (where the executor runs) over fallbackDir.
	dir := l.rs.WorktreePath
	if dir == "" {
		dir = l.fallbackDir
	}
	return (&review.GitDiffProvider{WorkDir: dir}).Diff(baseBranch)
}
```

Also update the comment on lines 369-373 to reflect the new behavior.

**Step 2: Run tests**

Run: `go test ./cmd/gromit-next/ -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/gromit-next/stage_provider.go
git commit -m "feat: lazyDiffProvider prefers WorktreePath over fallbackDir"
```

---

### Task 9: Wire realGitOps into BuildStages

**Files:**
- Modify: `cmd/gromit-next/stage_provider.go`

**Step 1: Replace noopGitOps with realGitOps**

Line 82:
```go
// Old:
gitOps := &noopGitOps{workDir: p.cfg.WorkDir}
// New:
gitOps := &realGitOps{}
```

**Step 2: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS. Existing init tests use `fakeGitOps` so they won't be affected.

**Step 3: Commit**

```bash
git add cmd/gromit-next/stage_provider.go
git commit -m "feat: wire realGitOps into BuildStages, replacing noopGitOps"
```

---

### Task 10: Remove noopGitOps (dead code)

**Files:**
- Modify: `cmd/gromit-next/stage_provider.go`

**Step 1: Delete the noopGitOps type**

Remove the `noopGitOps` struct, `CreateWorktree`, and `RemoveWorktree` methods (lines 305-327).

**Step 2: Run tests**

Run: `go vet ./... && go test ./... -count=1`
Expected: PASS — nothing references noopGitOps anymore.

**Step 3: Commit**

```bash
git add cmd/gromit-next/stage_provider.go
git commit -m "cleanup: remove dead noopGitOps code"
```

---

### Task 11: Integration test — full worktree lifecycle

**Files:**
- Create: `cmd/gromit-next/worktree_integration_test.go`

**Step 1: Write integration test**

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorktreeIntegration_FullLifecycle(t *testing.T) {
	// Create a real git repo
	repoDir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	run("init")
	os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n"), 0o644)
	run("add", ".")
	run("commit", "-m", "initial")

	ops := &realGitOps{}

	// Create worktree
	wt, err := ops.CreateWorktree(repoDir, "gromit/spec-test-run1")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// Worktree has repo contents
	if _, err := os.Stat(filepath.Join(wt, "main.go")); os.IsNotExist(err) {
		t.Fatal("worktree missing main.go")
	}

	// Can make changes in worktree without affecting main repo
	os.WriteFile(filepath.Join(wt, "new.go"), []byte("package main\n"), 0o644)
	if _, err := os.Stat(filepath.Join(repoDir, "new.go")); !os.IsNotExist(err) {
		t.Fatal("worktree change leaked to main repo")
	}

	// Cleanup
	err = ops.RemoveWorktree(wt)
	if err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatal("worktree not removed")
	}
}

func TestWorktreeIntegration_ChangesIsolated(t *testing.T) {
	repoDir := t.TempDir()
	run := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	run(repoDir, "init")
	os.WriteFile(filepath.Join(repoDir, "calc.go"), []byte("package calc\n"), 0o644)
	run(repoDir, "add", ".")
	run(repoDir, "commit", "-m", "initial")

	ops := &realGitOps{}
	wt, err := ops.CreateWorktree(repoDir, "gromit/spec-isolation")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer ops.RemoveWorktree(wt)

	// Modify file in worktree
	os.WriteFile(filepath.Join(wt, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644)

	// Main repo should still have original content
	data, _ := os.ReadFile(filepath.Join(repoDir, "calc.go"))
	if string(data) != "package calc\n" {
		t.Errorf("main repo modified: %q", string(data))
	}

	// Worktree should have modified content
	data, _ = os.ReadFile(filepath.Join(wt, "calc.go"))
	if string(data) == "package calc\n" {
		t.Error("worktree should have modified content")
	}
}
```

**Step 2: Run tests**

Run: `go test ./cmd/gromit-next/ -run TestWorktreeIntegration -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/gromit-next/worktree_integration_test.go
git commit -m "test: integration tests for worktree isolation lifecycle"
```

---

### Task 12: Final verification

**Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 2: Run vet**

Run: `go vet ./...`
Expected: No issues

**Step 3: Verify build**

Run: `go build ./cmd/gromit-next/`
Expected: Success
