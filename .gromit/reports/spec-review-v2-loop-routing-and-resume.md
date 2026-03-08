# Spec Branch Review: v2-loop-routing-and-resume

**Branch:** `gromit/spec/v2-loop-routing-and-resume`
**Base:** `main` (`bd1478d9`)
**Date:** 2026-03-07

---

## Summary

The spec branch adds 4 commits on top of main, implementing the plan and initial router for the `v2-loop-routing-and-resume` spec:

1. **ab2b8531** `[bead:v2-loop-routing-and-resume/plan/iter:0] proceed` -- Automated plan commit containing the plan document (`.gromit/v2/plan.md`), the full plan output (`.gromit/plans/2026-03-07-v2-loop-routing-and-resume.md`), and event logs (`.gromit/v2/events.jsonl`).

2. **938766e8** `red: tests for router phase preferences, ratio balancing, and fallback` (Co-Authored-By: Claude Sonnet 4.6) -- TDD red-phase tests for the Router type in `internal/v2/routing/router_test.go`. Covers phase preference override, ratio balancing, fallback behavior, and unavailable provider cooldown.

3. **b71e0661** `green: implement Router with phase preferences, ratio balancing, and fallback` (Co-Authored-By: Claude Sonnet 4.6) -- Implementation of the Router in `internal/v2/routing/router.go`. Implements 3-layer provider selection: phase preferences, ratio balancing, and any-available fallback. Includes provider unavailability tracking with cooldown.

4. **5590cac9** `[bead:gromit-4ot02/build:default/iter:2] proceed` -- Automated build stage commit.

### Files Changed (vs main)

```
 .gromit/v2/events.jsonl                            |   10 +
 .gromit/v2/plan.md                                 |   20 +
 .gromit/plans/2026-03-07-v2-loop-routing-and-resume.md | 1741 +
 internal/v2/routing/router.go                      |  161 +
 internal/v2/routing/router_test.go                 |  164 +
 5 files changed, 2096 insertions(+)
```

---

## Commit Log

```
5590cac9 [bead:gromit-4ot02/build:default/iter:2] proceed
b71e0661 green: implement Router with phase preferences, ratio balancing, and fallback
938766e8 red: tests for router phase preferences, ratio balancing, and fallback
ab2b8531 [bead:v2-loop-routing-and-resume/plan/iter:0] proceed
```

---

## Full Diff

The full diff (2127 lines) is available at:

`.gromit/reports/spec-review-v2-loop-routing-and-resume-diff.patch`

---

## Root Cause Analysis: Spec Commits Landing on Main

### Problem

When `gromit run2` executed the `v2-loop-routing-and-resume` spec, the ~80 commits generated during the spec run landed directly on the `main` branch instead of being isolated to the `gromit/spec/v2-loop-routing-and-resume` worktree branch.

### Investigation

#### 1. The Worktree Path Is Relative

In `cmd/gromit/run2.go` line 92, the `ExecGitAdapter` is constructed with `"."` as the repo root:

```go
adapters := adapter.AdapterSet{
    Git: gitadapter.NewExecGitAdapter(".", worktreesDir),
    ...
}
```

And `worktreesDir` is derived from the gromit dir (line 84):

```go
worktreesDir := filepath.Join(gromitDir, "spec-worktrees")
```

This means the worktree path returned by `Checkout()` is **relative** (e.g., `.gromit/spec-worktrees/v2-loop-routing-and-resume`). The events file confirms this:

```json
{"worktree":".gromit/spec-worktrees/v2-loop-routing-and-resume"}
```

#### 2. Checkout Creates the Branch but Claude Runs in the Wrong Directory

In `internal/v2/adapter/git/exec_git_adapter.go` line 46, `Checkout()` creates the worktree:

```go
cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-B", branchName, wtPath, "HEAD")
cmd.Dir = a.repoRoot  // "."
```

This creates the worktree correctly. The branch `gromit/spec/v2-loop-routing-and-resume` is created and the worktree directory exists. **Git commits made with `cmd.Dir` set to the worktree path will correctly land on the spec branch.**

#### 3. The Real Issue: Claude CLI Invocations Run in the Main Repo

The plan, decompose, and build stages all pass `req.Worktree` as the `Dir` parameter to the LLM adapter:

- **Plan stage** (`internal/v2/stage/plan/plan.go:165`): `Dir: req.Worktree`
- **Decompose stage** (`internal/v2/stage/decompose/decompose.go:232`): `Dir: dir` (from `req.Worktree`)
- **Build stage** (`internal/v2/stage/build/build.go:394`): `Dir: dir` (from `req.Worktree`)

The Claude adapter (`internal/v2/adapter/llm/claude.go:116-117`) then sets `cmd.Dir = dir` for the Claude CLI process. This means Claude CLI runs with its working directory set to the **worktree** path (`.gromit/spec-worktrees/v2-loop-routing-and-resume`).

**However**, Claude CLI itself has its own git behavior. When Claude Code makes file edits and commits, it operates based on the git repo it detects from its working directory. The worktree IS a valid git checkout of the spec branch, so Claude's edits within the worktree directory should land on the spec branch.

#### 4. The Actual Root Cause: Stage Commits Use the Worktree But Something Bypasses It

The `commitStage` method in `spec_loop.go` (line 685-689) delegates to a `StageCommitter`, which calls `git.Commit(ctx, worktree, ...)`. The `Commit` method in `exec_git_adapter.go` (line 70-90) correctly sets `cmd.Dir = worktree` for both `git add -A` and `git commit -m`.

**The most likely root cause is that `gromit run2` was executed directly in the main repo directory (not via a worktree-aware launcher), and the process's own CWD was the main repo.** The relative worktree path `.gromit/spec-worktrees/...` resolves correctly from the main repo, but:

1. **Claude CLI's `--allowedTools` or project-level CLAUDE.md** may cause it to read the main repo's `.claude/` configuration, which could instruct it to commit to the main branch.
2. **bd (bead) operations** run outside the worktree context. The `bead.NewClient()` in `run2.go` line 88 is not scoped to the worktree. Bead backup operations (`bd: backup ...` commits visible in main's history) commit to whatever branch the main repo's HEAD points to.
3. **The `gromit run2` process itself** runs on the main branch. Any git operations that don't explicitly set `cmd.Dir` to the worktree will affect main. The events file, plan file, and other artifacts written to `.gromit/v2/` are written relative to the worktree, but if any stage writes files to the main repo directory instead, those get committed to main.

### Specific Evidence

The commit history on main contains commits like:
- `bd: backup 2026-03-07 15:06` / `15:46` / `16:04` -- These are bead backup commits that should have been on the spec branch but landed on main because `bd` operates on the main repo's HEAD.
- The plan commit `[bead:v2-loop-routing-and-resume/plan/iter:0] proceed` appears on BOTH the spec branch and main, suggesting the stage committer committed to main.

### Recommended Fixes

1. **Use absolute paths for worktrees.** Change `NewExecGitAdapter(".", worktreesDir)` in `run2.go` to use `filepath.Abs(".")` so the worktree path is always absolute. This eliminates ambiguity.

2. **Scope bead operations to the worktree.** The `bead.NewClient()` and `tasktracker.NewBDAdapter()` should be configured to operate within the worktree directory, not the main repo.

3. **Ensure the gromit process does not commit to main during spec runs.** The `run2` command should either:
   - Detach HEAD in the main repo before starting the spec loop, or
   - Verify that all git operations (including bd backups) target the worktree path.

4. **Add a guard in `Commit()`.** The `ExecGitAdapter.Commit()` method could verify that the worktree path is actually a git worktree (not the main repo) before committing, to prevent accidental commits to main.

---

## Review Prompt

Review this as if it were a PR from the spec branch to main. Check for correctness, test coverage, adherence to project conventions (RULES.md, CLAUDE.md), and any concerns. Specifically:

- Does the Router implementation correctly handle the 3-layer selection (phase preferences, ratio balancing, fallback)?
- Is the test coverage sufficient for the Router's edge cases (all providers unavailable, cooldown expiry, ratio rebalancing)?
- Do the new files follow project conventions (package naming, nil-field normalization, error handling)?
- Are there any concurrency concerns with the Router's mutex usage?
- Does the plan document accurately reflect what was implemented?
- Should the `.gromit/v2/events.jsonl` and `.gromit/v2/plan.md` files be included in the PR, or are they build artifacts that should be gitignored?
