---
id: immutable-pipeline
source_spec: immutable-pipeline
created: 2026-03-06
decomposed: false
---

# Immutable Pipeline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Preserve every meaningful state transition during v2 spec execution as a git commit on the spec's worktree branch, with a correlated JSONL event log committed alongside code changes.

**Architecture:** The loop (spec_loop and bead_loop) creates structured commits after each stage — stages remain stateless and unaware of commits. A new file-writing event subscriber writes to `.gromit/v2/events.jsonl` in the worktree. The Present stage handles per-bead squash for PRs. `gromit debug2` operates on preserved worktree branches for diagnosis.

**Spec:** `.gromit/specs/immutable-pipeline.md`

---

## Architecture

### Components

1. **Commit message format** — Already implemented at `internal/v2/pipeline/commit_message.go`. Format: `[bead:<id>/<stage>/iter:<n>] <Decision>` or `[spec/<stage>/iter:<n>] <Decision>`. Includes `FormatCommitMessage` and `ParseCommitMessage` with round-trip tests.

2. **FileSubscriber** (`internal/v2/event/file_subscriber.go`) — Event subscriber that appends JSON-encoded events to `.gromit/v2/events.jsonl` in the worktree. Uses `os.OpenFile` with `O_APPEND|O_CREATE|O_WRONLY`. Created lazily after `Checkout()` returns the worktree path. Subscribed to the typed `event.Emitter` via `SubscribeTo()`.

3. **StageCommitter** (`internal/v2/pipeline/stage_committer.go`) — Wraps a `GitCommitter` interface (Status + Commit) to create structured commits. Calls `git status --porcelain` first, skips commit when nothing changed. Uses `FormatCommitMessage` to produce the structured message.

4. **Loop integration** — `spec_loop.go` commits after Plan, Decompose, Accept, Present (spec-level, beadID=""). `bead_loop.go` commits after Build, Validate, Review (bead-level). Gate and Epilogue are excluded per spec. When `StageCommitter` is set, per-stage commits replace the existing single `commitBeadWork` call.

5. **GitAdapter extensions** — Add `Log`, `Show`, `SquashCommits` to `GitAdapter` interface in `internal/v2/adapter/adapter.go`. Implement in `ExecGitAdapter`. `Log` returns `[]LogEntry{Hash, Message}`. `SquashCommits` uses `git reset --soft <startRef>` + `git commit`.

6. **Per-bead squash** (`internal/v2/pipeline/squash.go`) — Before PR creation, reads git log, parses structured commit messages to identify bead boundaries, and squashes per-stage commits into one commit per bead. Full history stays on worktree branch via reflog.

7. **Branch lifecycle** — Already mostly implemented: `cleanupWorktree` preserves on failure (default `preserveOnFailure=true`), removes worktree+branch on success. No changes needed.

8. **Debug command** (`cmd/gromit/debug2.go`) — Reads preserved worktree's event log and git history. Identifies failure point. Builds diagnostic prompt. Invokes LLM for diagnosis/fix/learning. Outputs LEARNINGS entries autonomously; systemic recommendations for human review.

### Data Flow

```
Stage runs → events emitted → FileSubscriber appends to events.jsonl
                             → StageCommitter: git add -A && git commit (includes events.jsonl)
                             → Commit message encodes bead/stage/iter/decision

Present stage → SquashPerBead reads git log → groups by bead → git reset --soft + commit per bead
              → PR shows clean per-bead commits; worktree branch keeps full history

Debug command → reads events.jsonl + git log from preserved worktree → LLM diagnosis
```

### Key Interfaces

- `GitCommitter` (already in `internal/v2/loop/bead_loop.go:25-28`): `Status(ctx, worktree) (string, error)` + `Commit(ctx, worktree, message) (string, error)`. Redefine locally in `pipeline` package to avoid circular import.
- `GitAdapter` (in `internal/v2/adapter/adapter.go:12-18`): Extended with `Log`, `Show`, `SquashCommits`.
- `event.Emitter` (in `internal/v2/event/emitter.go`): Existing fan-out emitter with `Subscribe(func(TypedEvent))` and `Emit(TypedEvent)`.

## Test Strategy

- **Unit tests** for FileSubscriber (append, cumulative, directory creation), StageCommitter (structured message, skip-on-no-changes, spec-level scope), squash logic (group by bead, no-op on no structured commits) — all use fakes/mocks, no real git.
- **Integration tests** for GitAdapter new methods (Log, Show, SquashCommits) — use real temp git repos, no LLM. Separate integration test for commit-per-stage flow end-to-end.
- **Contract tests** for commit message parsing round-trip (already exist and pass).
- **Fakes**: `fakeGitCommitter` (records Status/Commit calls), `fakeSquashGit` (records Log/SquashCommits calls), fake stages (return canned decisions).
- **Build tag**: Integration tests use `//go:build integration`.

---

## Implementation Tasks

### Task 1: JSONL Event Log File Subscriber

**Files:**
- Create: `internal/v2/event/file_subscriber.go`
- Test: `internal/v2/event/file_subscriber_test.go`

**What to Do:**
Create a `FileSubscriber` struct that appends JSON-encoded events to a JSONL file. Constructor: `NewFileSubscriber(path string) *FileSubscriber`. Method: `Handle(evt TypedEvent)` — marshals event to JSON, appends newline, writes to file (creating parent directories with `os.MkdirAll` on first write). Uses a `sync.Mutex` for thread safety. Convenience method `SubscribeTo(emitter *Emitter)` registers `Handle` as a subscriber.

Write tests covering: (1) two events produce two valid JSON lines, (2) parent directories are created automatically, (3) events are cumulative (second write appends, doesn't overwrite).

**Acceptance Criteria:**
- `NewFileSubscriber` + `Handle` produces valid JSONL output with one JSON object per line
- Parent directories are created if they don't exist
- Multiple calls to `Handle` append (cumulative, not overwrite)

**Dependencies:** None

---

### Task 2: Stage Committer — Per-Stage Structured Git Commits

**Files:**
- Create: `internal/v2/pipeline/stage_committer.go`
- Test: `internal/v2/pipeline/stage_committer_test.go`

**What to Do:**
Define a local `GitCommitter` interface in `pipeline` package (same shape as `loop.GitCommitter`: `Status` + `Commit`) to avoid circular import. Create `StageCommitter` struct wrapping `GitCommitter`. Method: `CommitStage(ctx, worktree, beadID, stageName string, iteration int, decision string) (hash string, err error)` — calls `Status`, skips if whitespace-only output, otherwise calls `Commit` with `FormatCommitMessage(beadID, stageName, iteration, decision)`.

Write tests using a `fakeGitCommitter` covering: (1) bead-level commit produces `[bead:003/build/iter:1] Proceed`, (2) empty status skips commit and returns empty hash, (3) spec-level (empty beadID) uses `[spec/plan/iter:1] Proceed`, (4) whitespace-only status skips commit.

**Acceptance Criteria:**
- `CommitStage` produces structured commit messages via `FormatCommitMessage`
- No commit is created when `git status` returns empty or whitespace-only output
- Spec-level stages (empty beadID) use "spec" scope in commit message

**Dependencies:** None (uses existing `FormatCommitMessage` from `commit_message.go`)

---

### Task 3: Extend GitAdapter with Log, Show, and SquashCommits

**Files:**
- Modify: `internal/v2/adapter/adapter.go` (add `LogEntry` type and 3 methods to `GitAdapter`)
- Modify: `internal/v2/adapter/git/exec_git_adapter.go` (implement methods)
- Test: `internal/v2/adapter/git/exec_git_adapter_ext_test.go`

**What to Do:**
Add `LogEntry` struct (`Hash`, `Message` string fields) to `adapter` package. Add three methods to `GitAdapter` interface:
- `Log(ctx, worktree string, maxCount int) ([]LogEntry, error)` — runs `git log --format="%H %s"`, parses output into `[]LogEntry` (most recent first)
- `Show(ctx, worktree, commitHash string) (string, error)` — runs `git show <hash>`
- `SquashCommits(ctx, worktree, startRef, message string) (string, error)` — runs `git reset --soft <startRef>` then `git commit -m <message>`, returns new HEAD hash

Implement all three in `ExecGitAdapter`. Write integration tests using `initTestRepo` helper (creates temp git repo with initial commit). Tests: (1) `Log` returns entries with correct hashes and messages in most-recent-first order, (2) `Show` returns diff containing changed file names, (3) `SquashCommits` collapses 3 commits into 1 while preserving all file content.

After implementation, grep for any existing fakes/mocks implementing `GitAdapter` and add stub methods so the build compiles.

**Acceptance Criteria:**
- `Log` returns commit entries in most-recent-first order with correct hash and message
- `Show` returns the diff/content for a specific commit
- `SquashCommits` collapses commits between startRef and HEAD into a single commit, preserving file content

**Dependencies:** None

---

### Task 4: Wire StageCommitter into Bead Loop

**Files:**
- Modify: `internal/v2/loop/bead_loop.go` (add `StageCommitter` field, per-stage commit logic)
- Test: `internal/v2/loop/bead_loop_test.go` (add new tests)

**What to Do:**
Add `StageCommitter` field to `BeadLoopConfig` (type: import `pipeline.StageCommitter` pointer, or define a local `StageCommitter` interface to avoid coupling). Store it on `BeadLoop` struct. Add a `commitAfterStage` method that calls `stageCommitter.CommitStage` with the bead ID, stage name, attempt number (from `runStageEntry`'s `attempt` counter), and decision string (capitalize `stage.Decision.String()`).

In `runStageEntry` (line ~397), after a successful stage run and before `return nil`, call `commitAfterStage`. This captures Build, Validate, and Review commits individually — including on retry iterations, preserving each attempt as a separate commit.

In `processBead` (line ~284), when `stageCommitter != nil`, skip the legacy `commitBeadWork` call. When `stageCommitter` is nil, preserve existing behavior.

Write tests: (1) with `StageCommitter` set — 3 commits (build/validate/review) with structured messages, no legacy commit, (2) without `StageCommitter` — 1 legacy commit from `commitBeadWork`.

**Acceptance Criteria:**
- When `StageCommitter` is configured, each of Build/Validate/Review produces its own structured commit
- When `StageCommitter` is nil, the existing `commitBeadWork` behavior is preserved
- Retry attempts produce separate commits with incrementing iteration numbers

**Dependencies:** Task 2

---

### Task 5: Wire FileSubscriber and StageCommitter into Spec Loop

**Files:**
- Modify: `internal/v2/loop/spec_loop.go` (add fields, commit after spec-level stages)
- Modify: `internal/v2/loop/spec_loop_test.go` (add tests)
- Modify: `internal/v2/loop/run2_components.go` (create and inject StageCommitter)

**What to Do:**
Add `stageCommitter` field and typed emitter field to `SpecLoop`. Add `WithStageCommitter` and `WithTypedEmitter` option functions.

In `Run()`, after `Checkout()` returns `worktree`:
1. Create `FileSubscriber` targeting `filepath.Join(worktree, gromitDir, "v2", "events.jsonl")`
2. Subscribe it to the typed emitter (if present)

After each spec-level stage, call `stageCommitter.CommitStage`:
- After `runPlanStage` success: `("", "plan", 1, "Proceed")`
- After `runDecompose` success: `("", "decompose", 1, "Proceed")`
- After `ensureAcceptance` success: `("", "accept", iteration, "Proceed")`
- After `presentSummary` success: `("", "present", 1, "Proceed")`

In `run2_components.go`: create `StageCommitter` from `adapters.Git` and pass via `WithStageCommitter` to both `SpecLoop` and `BeadLoopConfig`.

Write tests verifying commit messages appear after plan and decompose stages.

**Acceptance Criteria:**
- `FileSubscriber` is created and subscribed after worktree checkout
- Plan, Decompose, Accept, and Present stages each produce a structured spec-level commit
- Gate and Epilogue do not produce commits

**Dependencies:** Tasks 1, 2, 4

---

### Task 6: Per-Bead Squash Logic

**Files:**
- Create: `internal/v2/pipeline/squash.go`
- Test: `internal/v2/pipeline/squash_test.go`

**What to Do:**
Define `SquashGit` interface (`Log` + `SquashCommits` methods — subset of `GitAdapter`). Implement `SquashPerBead(ctx, git SquashGit, worktree string, beads []presentation.BeadSummary) error`:
1. Read git log (all entries)
2. Reverse to chronological order
3. Parse each commit message with `ParseCommitMessage`
4. Find the first structured commit; the commit before it is the squash base
5. If no structured commits, return nil (no-op)
6. For single bead: message = `"bead <ID>: <Title>"`; for multiple: summary message with bullet list
7. Call `SquashCommits` with base hash and assembled message

Write tests using `fakeSquashGit`: (1) groups bead commits correctly and calls squash, (2) no structured commits = no-op.

**Acceptance Criteria:**
- Per-stage commits are squashed into clean per-bead commits for PR presentation
- No-op when there are no structured commits in the log
- Squash message derives from bead titles

**Dependencies:** Task 3 (for `SquashCommits` and `Log` methods)

---

### Task 7: Wire Squash into Present Stage

**Files:**
- Modify: `internal/v2/stage/present/present.go` (add squasher function field)
- Modify: `internal/v2/stage/present/present_test.go` (add test)
- Modify: `internal/v2/loop/run2_components.go` (wire squasher closure)

**What to Do:**
Add a `squasher` function field to the Present stage: `func(context.Context, string, []presentation.BeadSummary) error`. Add `WithSquasher` option. In `Run()`, when `squasher != nil` and `ctx.Success == true`, call `squasher(ctx, worktree, beadSummaries)` before presenting. This decouples present from pipeline package.

In `run2_components.go`, create the squasher closure: `func(ctx, wt, beads) error { return pipeline.SquashPerBead(ctx, adapters.Git, wt, beads) }` and pass via `WithSquasher`.

Write test: verify squasher is called before presenting when success=true, not called on failure.

**Acceptance Criteria:**
- Present stage calls squasher before presenting when the spec succeeded
- Squasher is not called when the spec failed
- Present stage still works when no squasher is configured (nil = no-op)

**Dependencies:** Tasks 5, 6

---

### Task 8: Integration Test — Commit-Per-Stage Flow with Real Git

**Files:**
- Create: `internal/v2/pipeline/integration_test.go`

**What to Do:**
Write an integration test (build tag `integration`) that exercises the full commit-per-stage flow with a real git repo and no LLM:
1. Create temp git repo with initial commit
2. Checkout worktree via `ExecGitAdapter`
3. Create `FileSubscriber` and `Emitter`, subscribe
4. Create `StageCommitter` backed by `ExecGitAdapter`
5. Simulate spec-level stage: write plan file, emit `SpecStartedEvent`, flush emitter, commit with `CommitStage`
6. Simulate bead-level stage: write code file, emit `StageCompletedEvent`, flush emitter, commit
7. Verify: git log has structured commit messages parseable by `ParseCommitMessage`
8. Verify: `events.jsonl` is cumulative and each line is valid JSON
9. Verify: events and commits are correlated by bead ID, stage name, and iteration

**Acceptance Criteria:**
- Full flow produces parseable structured commits in git history
- Event log is cumulative and committed alongside code changes at each stage
- Events and commits correlate by bead ID, stage name, and iteration number

**Dependencies:** Tasks 1, 2, 3, 5

---

### Task 9: Debug Command — `gromit debug2 <spec-name>`

**Files:**
- Create: `cmd/gromit/debug2.go`
- Test: `cmd/gromit/debug2_test.go`

**What to Do:**
Implement helper functions:
- `resolveDebug2Worktree(repoRoot, specID string) (string, error)` — looks up `.gromit/spec-worktrees/{specID}`, returns error if not found
- `readDebug2EventLog(worktree string) ([]map[string]any, error)` — reads `.gromit/v2/events.jsonl`, parses each line as JSON
- `findFailureEvent(events []map[string]any) map[string]any` — finds last event with type `stage.failed` or `spec.failed`
- `buildDebug2Prompt(specID string, events []map[string]any, gitLog string) string` — assembles diagnostic prompt including event log, git history, failure diff

Wire Cobra command `debug2` with positional arg `<spec-name>`. The command:
1. Resolves worktree from spec name
2. Reads event log and git log (via `ExecGitAdapter.Log`)
3. Identifies failure point
4. Builds diagnostic prompt
5. Launches LLM session in the worktree (reuse existing agent session infrastructure from `debug.go`)
6. LLM output handling: LEARNINGS entry → append to `LEARNINGS.md`; systemic recommendation → print for human review

Write tests: (1) `resolveDebug2Worktree` finds existing worktree, (2) returns error for missing worktree, (3) `readDebug2EventLog` parses JSONL correctly, (4) `findFailureEvent` identifies failure events.

**Acceptance Criteria:**
- `gromit debug2 <spec-name>` finds the preserved worktree branch and reads its event log and git history
- Debug identifies the failure point from the event log
- Helper functions are unit-testable without LLM invocation

**Dependencies:** Tasks 3, 5

---

## Notes

- **Task 0 (commit_message.go) is already done** — `internal/v2/pipeline/commit_message.go` and tests exist and pass. Not included in tasks.

- **Iteration tracking**: In `runStageEntry`, the `attempt` counter tracks per-stage retry attempts — use this for the commit message `iter:N`. The bead loop's outer `iteration` tracks which bead, not retries.

- **Decision capitalization**: `stage.Decision.String()` returns lowercase. `FormatCommitMessage` should capitalize. Check `decisionStrings` map or use `strings.Title`.

- **FileSubscriber lifecycle**: Created in `SpecLoop.Run()` after `Checkout()`. Emitter's `Close()` drains the subscriber queue. No explicit cleanup needed.

- **Branch preservation**: Already works — `spec_loop.go:508` returns early when `preserveOnFailure == true`. No changes needed.

- **Squash safety**: `git reset --soft` rewrites only the ephemeral worktree branch (`gromit/spec/{specID}`). Original commits recoverable via reflog.

- **Debug command naming**: `debug2` during development, rename at v1→v2 cutover (matches `run2` pattern).

- **Circular import avoidance**: `pipeline` package defines its own `GitCommitter` interface (same shape as `loop.GitCommitter`) to avoid importing `loop` from `pipeline`.
