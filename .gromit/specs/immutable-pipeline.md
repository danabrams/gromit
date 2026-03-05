---
id: immutable-pipeline
source_ideas: [gromit-v2-autonomous-product-engineer]
created: 2026-03-05
---

# Immutable Pipeline

## Specification

The immutable pipeline preserves every meaningful state transition during a v2 spec execution as a git commit on the spec's worktree branch, with a correlated event log committed alongside code changes. This gives an LLM debugger full visibility into what happened at each stage without reconstructing state from scattered logs.

### Per-Stage Commits

Every stage that produces meaningful state changes gets a git commit on the spec's worktree branch after it runs. This includes stages that only advance the event log (e.g., Validate adds validation results even if no code changes).

**Stages that commit:** Plan, Decompose, Build, Validate, Review, Accept, Present.

**Stages that do not commit:** Gate (pure eligibility check, no state change), Epilogue (bead bookkeeping only).

Each commit includes:
- Any code changes the stage produced
- The updated event log file (`.gromit/v2/events.jsonl`)

### Structured Commit Messages

Commit messages encode the system state for machine parsing:

```
[bead:<id>/<stage>/iter:<n>] <Decision>
```

Examples:
- `[spec/plan/iter:1] Proceed`
- `[bead:003/build/iter:1] Proceed`
- `[bead:003/validate/iter:1] Fail`
- `[bead:003/build/iter:2] Proceed`
- `[bead:003/validate/iter:2] Proceed`
- `[spec/accept/iter:1] Fail`

Spec-level stages (Plan, Decompose, Accept, Present) use `spec` in place of a bead ID. The iteration number increments per retry of the same stage for the same bead.

### Event Log in Worktree

The v2 event system's log subscriber writes to `.gromit/v2/events.jsonl` inside the spec's worktree. This file is committed alongside code changes at each stage commit, so every commit contains the cumulative event history up to that point.

Events and commits are correlated by bead ID, stage name, and iteration number — not by commit SHA. Both the commit message and each event carry these fields, making cross-referencing straightforward for machine consumers.

The event log contains the full detail: prompts, LLM responses, validation output, timing, cost, RetryContext. The commit message is the index; the event log is the content.

### Retry Preservation

When a stage fails and the loop retries (e.g., Validate fails, Build reruns, Validate reruns), each attempt is a separate commit. The code from Build iter:1 is preserved in git history even after Build iter:2 overwrites it. An LLM debugger can find any prior attempt via `git log` and `git show`.

### Per-Bead Squash at Presentation

When Present creates a PR, the per-stage commits are squashed into one commit per bead. The PR shows a clean sequence of logical work units, each with a message derived from the bead's title. The full stage-level history remains on the worktree branch for debugging.

### Branch Lifecycle

**On success (PR merged):** The worktree branch is deleted. The debugging history is no longer needed — the PR diff and merged commits are sufficient for any future reference.

**On failure (Andon triggered or generation cap hit):** The worktree branch is preserved with all per-stage commits and the event log intact. This is the primary debugging artifact. The branch remains until the human resolves the failure (via `gromit debug` or manual intervention).

### Debug Command

`gromit debug <spec-name>` is the entry point for diagnosing and fixing failed spec executions. It finds the preserved worktree branch, reads the event log and git history, and performs three jobs:

**1. Diagnose.** Read the event log and stage commit history to identify where and why the spec failed. Trace the failure to its root cause — was it a bad Build output, a flaky test, an unclear bead description, an incorrect decomposition?

**2. Fix.** Check out the failure point on the worktree branch, apply a code fix, and validate that the fix passes the relevant stage (e.g., re-run Validate after fixing a test failure).

**3. Learn.** Extract a lesson from the failure and persist it in the appropriate form. The debug LLM chooses the output form based on what it finds:

- **LEARNINGS entry:** For patterns and conventions the system should remember. Added to `LEARNINGS.md`. This is the autonomous path — debug adds the learning, applies the code fix, and the loop can resume without human intervention.

- **Recommendation for human review:** For changes that go beyond a learning — prompt fragment additions, code guards in the pipeline, process changes, or rule updates. Debug fixes the immediate code problem on the spec branch, then presents the systemic recommendation describing what to change and why. The human decides whether and when to act on it.

The distinction: if the fix is "remember this pattern," debug handles it end-to-end. If the fix is "change how the system works," debug proposes and the human decides.

## Acceptance Criteria

- Every stage except Gate and Epilogue produces a git commit on the spec's worktree branch after running
- Commit messages follow the structured format `[bead:<id>/<stage>/iter:<n>] <Decision>` and are machine-parseable
- The event log file (`.gromit/v2/events.jsonl`) is committed at each stage commit and contains cumulative events up to that point
- Events and commits are correlated by bead ID, stage name, and iteration number
- Retry attempts are preserved as separate commits — prior stage output is not lost when a retry overwrites it
- When Present creates a PR, per-stage commits are squashed into one commit per bead with bead title as the message
- The full stage-level commit history remains on the worktree branch after squash
- On successful PR merge, the worktree branch is deleted
- On failure (Andon or generation cap), the worktree branch is preserved with full commit and event history
- `gromit debug <spec-name>` finds the preserved worktree branch and reads its event log and git history
- Debug diagnoses the root cause, applies a code fix on the spec branch, and validates the fix
- When the root cause is a learnable pattern, debug adds a LEARNINGS entry and the fix is autonomous
- When the root cause requires systemic changes (prompt fragments, code guards, process changes), debug fixes the immediate problem and presents a recommendation for human review
- Debug does not apply systemic changes without human approval

## Decisions

1. **Per-stage commits over per-bead commits.** Per-stage granularity captures the exact state before a retry overwrites it — the key debugging artifact. Per-bead would lose the intermediate states that matter most for diagnosis. The cost (more commits) is negligible since commits are cheap and the branch is ephemeral.

2. **Event log in worktree over external sidecar.** Committing the event log alongside code changes makes each commit self-contained for debugging. An external log requires cross-referencing and can become separated from the code state it describes. The consumer is a machine (LLM debugger), so JSONL in the repo is ideal.

3. **Correlation by bead/stage/iteration over commit SHA.** Events are written during stage execution, before the commit exists. Embedding the commit SHA in events would require a two-phase write. The composite key (bead ID + stage name + iteration number) is already unique and present in both events and commit messages, making SHA correlation unnecessary complexity.

4. **Per-bead squash for PRs over full history or single squash.** Per-bead squash gives the product owner a readable PR where each commit is a logical unit of work, while collapsing stage-level noise. Single squash loses the logical structure. Full history overwhelms reviewers with implementation mechanics.

5. **Keep branch on failure, delete on success.** On success, the PR and merged code are sufficient reference material. On failure, the worktree branch with its per-stage commits and event log is the primary debugging artifact and must be preserved until the failure is resolved.

6. **No snapshot files — events are sufficient.** The event log already captures all runtime metadata (stage decisions, validation results, retry context, cost, timing). A separate snapshot format would duplicate this data and require its own schema maintenance. The LLM debugger can parse JSONL directly.

7. **Debug output form depends on root cause.** LEARNINGS entries are autonomous because they don't change system behavior — they inform future LLM invocations. Prompt fragments, code guards, and process changes alter how the system works and require human judgment. This distinction keeps debug useful without being dangerous.

8. **Gate and Epilogue excluded from commits.** Gate is a pure eligibility check that produces no state changes worth preserving. Epilogue is bead bookkeeping (closing the bead in the task tracker). Neither produces artifacts that aid debugging. Including them would add noise to the commit history.

## Research & Context

### Current State

V1 debugging uses `gromit debug`, which prompts an LLM with copy-pasted errors or log file contents. State is not preserved — once later beads run or retries overwrite code, the failure state is gone. The `internal/logger/` package has `LogIteration()` and `DiagnosticSnapshot()` for telemetry, but these are runtime diagnostics, not preserved debugging artifacts.

V1 has `LEARNINGS.md` and `LEARNINGS_ARCHIVE.md` for persistent lessons. `RULES.md` is built from learnings. The v2 debug command's learning output feeds into this existing system.

### V2 Integration Points

This spec layers on top of the v2 run loop (`v2-run-loop.md`):

- **Loop commits:** The loop (spec_loop and bead_loop) is responsible for creating commits after each stage runs. This is loop infrastructure, not stage logic — stages remain stateless and unaware of commits.
- **Event subscriber:** A file-writing event subscriber appends to `.gromit/v2/events.jsonl` in the worktree. This subscriber is wired in the same way as the CLI and API subscribers defined in the v2 event system.
- **Present stage:** Present handles the per-bead squash when creating the PR. The squash logic is part of the Present stage or the Git adapter.
- **Branch cleanup:** On success, worktree removal (already part of the spec loop) deletes the branch. On failure, the spec loop preserves the worktree (already specified in v2-run-loop for Andon failures).
- **Debug command:** `gromit debug` becomes a new CLI command (`cmd/gromit/debug2.go` during development, renamed at cutover) that operates on preserved worktree branches.

### Epic Context

This spec implements the "Immutable Pipeline" and "Observable" outcomes from the `gromit-v2-autonomous-product-engineer` epic. It depends on the v2 run loop for the stage/commit/event infrastructure and the worktree branch lifecycle.
