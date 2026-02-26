---
id: debug-runbook
source_ideas: []
created: 2026-02-18
epic: observability-and-diagnostics
---

# Deterministic Debug Runbook

## Specification

When a bead fails after exhausting the escalation chain, the failure context vanishes. The user must reconstruct what happened before investigating. This spec adds automatic failure capture and a replay mechanism through `gromit debug`.

### Runbook Capture

The runner appends a runbook entry to `.gromit/runbooks.jsonl` after a bead exhausts all retries. Each entry records:

- **Bead context**: bead ID, title, spec ID
- **Prompt**: the rendered build prompt sent to Claude
- **Commit refs**: `start_commit` (HEAD before iteration) and `failure_commit` (HEAD after all retries). The diff between these captures Claude's partial work.
- **Validation**: commands that ran and their output (capped at 5KB, tail)
- **Failure analysis**: category from the analyzer, escalation chain attempted
- **Environment**: Go version, OS, architecture

#### Entry Schema

```json
{
  "id": "rb-<unix-timestamp>-<bead-id>",
  "timestamp": "2026-02-18T15:30:00Z",
  "bead_id": "beads-abc",
  "bead_title": "Fix login validation",
  "spec_id": "login-validation",
  "start_commit": "abc1234def",
  "failure_commit": "567890abc",
  "prompt": "<rendered build prompt>",
  "validation_commands": ["go test ./...", "go vet ./..."],
  "failure_output": "<last 5KB of validation output>",
  "failure_category": "test_failure",
  "escalation_chain": ["haiku", "sonnet", "opus"],
  "env": {
    "go_version": "go1.23.1",
    "os": "linux",
    "arch": "amd64"
  }
}
```

#### Capture Point

After the escalation handler exhausts retries and before marking the bead as failed/skipped. At this point the runner has access to all required context: startCommit, bead metadata, rendered prompt, validation output, failure category, and escalation history.

### Debug Picker

When the user runs `gromit debug` with no arguments:

1. Read `.gromit/runbooks.jsonl`.
2. Filter out entries older than TTL (default: 14 days).
3. If entries exist, present a numbered menu:

```
Recent failures available for investigation:

  1. [2h ago]  beads-abc: Fix login validation (test_failure)
  2. [1d ago]  beads-def: API rate limiting (timeout)
  3. [3d ago]  beads-ghi: Cache invalidation (compilation_error)

  0. Start fresh investigation (no runbook)

Select failure to investigate [0]:
```

4. If the user selects a runbook entry, inject its data into the debug prompt. Claude receives the full failure context as a starting point.
5. If no entries exist or the user selects 0, behave as today.

When `gromit debug "description"` is passed (pre-seeded), skip the picker and launch directly.

### `--restore` Worktree Mode

When the user passes `--restore` after selecting a runbook entry:

1. Create a git worktree at `failure_commit` in `.gromit/worktrees/debug-<runbook-id>/`.
2. Launch the debug session in the worktree.
3. After the session exits, prompt: "Keep worktree? [y/N]"
4. Clean up the worktree if declined.

Without `--restore` (the default), the debug session runs in the current working directory with runbook data as context only. No git state changes.

### TTL Expiry

Runbook entries expire after a configurable TTL. Expired entries are hidden from the picker and cleaned up periodically.

### Configuration

```yaml
runbook:
  ttl_days: 14
```

### New Package: `internal/runbook`

- `Entry` struct matching the JSONL schema
- `Append(gromitDir string, entry Entry) error` — append one entry
- `List(gromitDir string, ttlDays int) ([]Entry, error)` — read and filter by TTL
- `Cleanup(gromitDir string, ttlDays int) error` — rewrite file without expired entries

## Acceptance Criteria

- The runner appends a runbook entry to `.gromit/runbooks.jsonl` when a bead fails after exhausting all retries
- Each runbook entry contains bead context, rendered prompt, commit refs, validation output (capped at 5KB), failure category, escalation chain, and environment snapshot
- `gromit debug` with no arguments presents a picker of recent unresolved failures filtered by TTL
- Selecting a runbook entry injects the failure context into the debug session prompt
- `gromit debug "description"` skips the picker and behaves as before
- `--restore` creates a worktree at the failure commit for true state reproduction
- Runbook entries older than `runbook.ttl_days` (default 14) are hidden from the picker

## Decisions

1. **JSONL over directories.** A single file matches gromit's existing patterns (iteration logs, backlog) and simplifies TTL filtering. Large artifacts (diffs) derive from commit refs rather than stored inline.

2. **Commit refs over inline diffs.** The runner tracks `startCommit` per iteration. Two commit SHAs keep entries small; `git diff start_commit..failure_commit` reconstructs the exact state.

3. **Failure output capped at 5KB.** The tail of validation output contains the actionable information. Full output lives in `.gromit/logs/`.

4. **Context-only replay by default.** Manipulating git state carries risk. The default injects failure context into Claude's prompt without changing the working directory. `--restore` uses a worktree for true reproduction when needed.

5. **TTL-based expiry over bead-lifecycle coupling.** Simpler and avoids tight coupling between runbooks and bead status. Stale failures age out regardless of whether the bead was re-attempted.

6. **Picker only on bare `gromit debug`.** Pre-seeded invocations skip the picker, preserving the existing workflow.

## Research & Context

### Key Files

- `cmd/gromit/debug.go` — Existing debug command implementation; needs picker integration
- `internal/runner/escalation/handler.go` — Escalation handler; capture point for runbook entries
- `internal/runner/helpers.go` — `getGitHead()`, `getGitDiff()` functions for commit tracking
- `internal/runner/runner.go` — Runner struct with access to all required context
- `internal/runner/run_iteration.go` — Iteration lifecycle where startCommit is captured
- `gromit.yaml` — Config file; add `runbook` section
