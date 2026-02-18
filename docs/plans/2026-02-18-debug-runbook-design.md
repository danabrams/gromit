# Deterministic Debug Runbook

## Problem

When a bead fails after exhausting the escalation chain, the failure context vanishes. The user must reconstruct what happened — which prompt was sent, what validation failed, what state the code was in — before they can investigate. This reconstruction is tedious and error-prone.

## Solution

Capture a **runbook** on every terminal failure and surface captured runbooks through `gromit debug`.

Two components:

1. **Auto-capture**: The runner appends a runbook entry to `.gromit/runbooks.jsonl` after a bead exhausts all retries. Each entry records the prompt, bead context, validation output, commit references, escalation history, and environment snapshot.

2. **Debug picker**: `gromit debug` reads the runbook file, filters by TTL, and presents recent unresolved failures. The user selects one, and the debug session launches with full failure context pre-loaded. A `--restore` flag creates a worktree at the exact failure state.

## Runbook Entry Schema

```json
{
  "id": "rb-<unix-timestamp>-<bead-id>",
  "timestamp": "2026-02-18T15:30:00Z",
  "bead_id": "beads-abc",
  "bead_title": "Fix login validation",
  "spec_id": "login-validation",
  "start_commit": "abc1234def",
  "failure_commit": "567890abc",
  "prompt": "<truncated build prompt>",
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

- `start_commit`: git HEAD before the iteration began.
- `failure_commit`: git HEAD after all retries exhausted. The diff between these two commits captures Claude's partial work.
- `failure_output`: capped at 5KB (tail). Full output lives in the iteration logs.
- `prompt`: the rendered build prompt, so the debug session knows exactly what Claude was asked to do.

## Capture Point

The runner calls `runbook.Append()` after the escalation handler exhausts retries and before marking the bead as failed/skipped. At this point, all required context is available:

- `startCommit` from `getHead()` at iteration start
- Current HEAD from `getHead()` at failure time
- Bead metadata from `*bead.Bead`
- Rendered prompt from the build phase
- Failure output from validation
- Failure category from the analyzer
- Escalation chain from config

## Debug Picker Flow

When the user runs `gromit debug` with no arguments:

1. Read `.gromit/runbooks.jsonl`.
2. Filter out entries older than TTL (default: 14 days, configurable via `runbook.ttl_days`).
3. If entries exist, present a numbered menu:

```
Recent failures available for investigation:

  1. [2h ago]  beads-abc: Fix login validation (test_failure)
  2. [1d ago]  beads-def: API rate limiting (timeout)
  3. [3d ago]  beads-ghi: Cache invalidation (compilation_error)

  0. Start fresh investigation (no runbook)

Select failure to investigate [0]:
```

4. If the user selects a runbook entry, inject its data into the debug prompt. Claude receives the bead context, failure output, validation commands, commit refs, and env snapshot — a complete starting point.
5. If no entries exist or the user selects 0, behave as today.

When `gromit debug "some description"` is passed (pre-seeded), skip the picker and launch directly, same as today.

## `--restore` Worktree Mode

When the user passes `--restore` after selecting a runbook entry:

1. Create a git worktree at `failure_commit` in `.gromit/worktrees/debug-<runbook-id>/`.
2. Launch the debug session with the working directory set to the worktree.
3. After the session exits, prompt: "Keep worktree for further investigation? [y/N]"
4. If no, clean up the worktree.

Without `--restore`, the debug session runs in the current working directory with the runbook data as context only. This is the default and requires no git state changes.

## New Package: `internal/runbook`

- `Entry` — struct matching the JSONL schema
- `Append(gromitDir string, entry Entry) error` — appends one entry to `runbooks.jsonl`
- `List(gromitDir string, ttlDays int) ([]Entry, error)` — reads file, filters by TTL
- `Cleanup(gromitDir string, ttlDays int) error` — rewrites file without expired entries

## Config Addition

```yaml
runbook:
  ttl_days: 14  # Runbook entries older than this are hidden and eventually cleaned up
```

## Decisions

1. **JSONL over directories.** A single file matches gromit's existing patterns (iteration logs, backlog) and simplifies TTL filtering. Large artifacts (diffs) are derived from commit refs rather than stored inline.

2. **Commit refs over inline diffs.** The runner tracks `startCommit` per iteration. Storing two commit SHAs instead of the full diff keeps entries small and makes the diff always available via `git diff start_commit..failure_commit`.

3. **Failure output capped at 5KB.** The tail of validation output contains the actionable information. Full output is in `.gromit/logs/`.

4. **Context-only replay by default.** Manipulating git state is risky. The default injects failure context into Claude's prompt without changing the working directory. `--restore` uses a worktree for true reproduction when needed.

5. **TTL-based expiry over bead-lifecycle coupling.** Simpler and avoids tight coupling between runbooks and bead status. Stale failures age out regardless of whether the bead was re-attempted.

6. **Picker only on bare `gromit debug`.** Pre-seeded invocations (`gromit debug "description"`) skip the picker, preserving the existing workflow.
