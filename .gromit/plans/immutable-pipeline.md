---
id: immutable-pipeline
source_spec: immutable-pipeline
priority: P1
---

# Immutable Pipeline Implementation Plan

Based on spec `immutable-pipeline.md` acceptance criteria, implement the commit/event log/debug infrastructure for the v2 spec execution pipeline.

## Implementation Tasks

### Task 1: Implement Per-Stage Commit Message Formatting

Create the commit message structure `[bead:<id>/<stage>/iter:<n>] <Decision>` used by the loop after each stage runs.

**Files to modify/create:**
- `internal/v2/loop/commit.go` (new) — structured message builder
- `internal/v2/loop/commit_test.go` (new) — unit tests for message format

**Acceptance Criteria:**
- CommitMessage struct with fields for bead ID, stage name, iteration number, and decision
- Builder function creates properly formatted messages matching pattern `[bead:<id>/<stage>/iter:<n>] <Decision>`
- Spec-level stages use `spec` in place of bead ID
- Parsing function validates and extracts components from commit messages for machine consumption

### Task 2: Create Event Log File Subscriber

Wire a file-writing subscriber that appends events to `.gromit/v2/events.jsonl` in the spec's worktree, writing cumulative event history.

**Files to modify/create:**
- `internal/v2/event/subscriber_file.go` (new) — file-writing subscriber implementation
- `internal/v2/event/subscriber_file_test.go` (new) — unit tests for append and cumulative correctness

**Acceptance Criteria:**
- FileSubscriber appends events to `.gromit/v2/events.jsonl` in the worktree directory
- File contains cumulative event history (all events from start of execution)
- Events written in JSONL format (one JSON object per line)
- Each event record includes bead ID, stage name, and iteration number for correlation

### Task 3: Integrate Event Subscriber into Loop

Wire the file subscriber into the v2 loop event system so it runs alongside existing CLI and API subscribers.

**Files to modify/create:**
- `internal/v2/loop/loop.go` — add file subscriber initialization
- Integration with existing event subscriber pattern

**Acceptance Criteria:**
- File subscriber created and registered during loop initialization
- Events flow through all subscribers in parallel
- Worktree path is available to subscriber at construction time
- No breaking changes to existing CLI or API subscribers

### Task 4: Implement Per-Stage Commit Creation in Loop

Modify the spec and bead loops to create git commits after each stage runs (Plan, Decompose, Build, Validate, Review, Accept, Present), excluding Gate and Epilogue.

**Files to modify/create:**
- `internal/v2/loop/spec_loop.go` — add commit logic after spec-level stages
- `internal/v2/loop/bead_loop.go` — add commit logic after bead-level stages
- `internal/v2/adapter/git/commit.go` — implement git commit wrapper

**Acceptance Criteria:**
- After Plan, Decompose, Accept, Present stages complete, a git commit is created on the spec branch with structured message format
- After Build, Validate, Review stages complete, a git commit is created with bead ID and stage name
- Commit includes updated event log file (`.gromit/v2/events.jsonl`)
- Gate and Epilogue stages do not create commits
- Commit includes any code changes produced by the stage

### Task 5: Preserve Retry Attempts as Separate Commits

Ensure that when a stage retries (iteration increments), each attempt becomes a separate commit with distinct iteration numbers.

**Files to modify/create:**
- `internal/v2/loop/retry.go` — increment iteration counter per retry
- Integration with commit message formatting (Task 1)

**Acceptance Criteria:**
- Retry increments the iteration number in commit messages
- Each retry produces a new commit (does not amend previous)
- Prior stage output is accessible via `git log` and `git show` for prior iterations
- Retry context is preserved in event log with matching iteration number

### Task 6: Implement Per-Bead Squash Logic

Create logic in the Present stage to squash per-stage commits into one commit per bead with the bead title as commit message.

**Files to modify/create:**
- `internal/v2/stage/present/squash.go` (new) — per-bead squash logic
- `internal/v2/stage/present/squash_test.go` (new) — unit and integration tests

**Acceptance Criteria:**
- Identifies all commits for a given bead across all stages
- Groups commits by bead ID
- Creates single squashed commit per bead with message based on bead title
- Full stage-level history remains on worktree branch (squash is done via new commits, not force-push)

### Task 7: Implement Branch Cleanup on Success

Delete the spec's worktree branch after successful PR merge.

**Files to modify/create:**
- `internal/v2/stage/present/branch_cleanup.go` (new) — branch deletion logic
- Integration into success path of Present stage

**Acceptance Criteria:**
- After PR merge confirmation, worktree branch is deleted
- Only applies to successful merge (not on failure or cancellation)
- Cleans up local and remote branch if configured

### Task 8: Implement Branch Preservation on Failure

Preserve the spec's worktree branch with full commit and event history on Andon or generation cap failure.

**Files to modify/create:**
- `internal/v2/loop/failure_handler.go` (new) — failure path logic
- Integration into Andon and generation cap paths

**Acceptance Criteria:**
- On Andon, worktree branch remains with all stage commits and event log
- On generation cap, worktree branch remains with all stage commits and event log
- Branch is preserved until manually cleaned or resolved via `gromit debug`

### Task 9: Implement Debug Command Skeleton

Create `gromit debug <spec-name>` command that finds a preserved worktree branch and provides access to event log and git history.

**Files to modify/create:**
- `cmd/gromit/debug2.go` (new) — debug command implementation
- `internal/v2/debug/finder.go` (new) — worktree branch finding logic
- Integration with CLI

**Acceptance Criteria:**
- `gromit debug <spec-name>` finds the preserved worktree branch
- Command reads and displays event log from `.gromit/v2/events.jsonl`
- Command can display git history via `git log` and `git show`
- Proper error handling when worktree branch not found

### Task 10: Implement Debug Root Cause Diagnosis

Extend debug command to analyze event log and commit history to identify failure root cause.

**Files to modify/create:**
- `internal/v2/debug/diagnose.go` (new) — root cause analysis logic
- `internal/v2/debug/diagnose_test.go` (new) — unit tests for diagnosis patterns

**Acceptance Criteria:**
- Traces failure through event log to identify which stage failed
- Examines stage-level details (validation output, build errors, etc.)
- Categorizes root cause (bad build, flaky test, unclear spec, bad decomposition)
- Outputs diagnosis in human-readable form

### Task 11: Implement Debug Code Fix and Validation

Extend debug to check out the failure point, apply a code fix, and validate via the relevant stage.

**Files to modify/create:**
- `internal/v2/debug/fixer.go` (new) — code fix application
- `internal/v2/debug/validator.go` (new) — fix validation via stage re-run

**Acceptance Criteria:**
- Checks out the worktree branch at the failure point
- Applies provided code fix to the spec branch
- Re-runs the relevant stage to validate the fix works
- Preserves fix in git commit on spec branch

### Task 12: Implement Debug Learning Path (Autonomous)

When root cause is a learnable pattern, debug adds a LEARNINGS entry and the fix is autonomous (no human approval needed).

**Files to modify/create:**
- `internal/v2/debug/learning.go` (new) — LEARNINGS entry generation
- `internal/v2/debug/learning_test.go` (new) — tests for learning identification

**Acceptance Criteria:**
- Identifies when fix is a learnable pattern (convention, naming, structure, etc.)
- Generates LEARNINGS entry with pattern description and example
- Appends to `LEARNINGS.md` in spec directory
- Applied fix is autonomous — debug can complete without human approval

### Task 13: Implement Debug Systemic Recommendation Path

When root cause requires systemic changes, debug fixes the immediate problem and presents recommendation for human review.

**Files to modify/create:**
- `internal/v2/debug/systemic.go` (new) — systemic recommendation generation
- `internal/v2/debug/systemic_test.go` (new) — tests for recommendation identification

**Acceptance Criteria:**
- Identifies when fix requires systemic change (prompt, guard, process, rule)
- Fixes immediate code problem on spec branch
- Generates human-readable recommendation describing systemic change and rationale
- Outputs recommendation and awaits human approval before proceeding

### Task 14: Implement Debug Safety Guards

Ensure debug does not apply systemic changes without human approval.

**Files to modify/create:**
- `internal/v2/debug/guardrails.go` (new) — safety checks
- Integration into fix application and learning paths

**Acceptance Criteria:**
- Detects when a proposed fix would modify prompt fragments, guards, or process rules
- Blocks automatic application of systemic changes
- Requires explicit human approval via `--approve` flag or interactive prompt
- Logs all blocked changes for audit

