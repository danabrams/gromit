# Session Worktree Contention Detection Contract (gromit-r7lcc)

## Context
`CreateSessionWorktree` must distinguish retryable session-name contention from terminal failures without depending on brittle one-off stderr substrings.

## Decision
Use a two-layer classifier:
1. Normalize git failure text from command output first, with error-text fallback.
2. For ambiguous lock-ref failures, run deterministic fallback probes against repository state.

## Retryable contention classes
- Branch name collision: branch already exists for the generated session branch name.
- Ref lock contention that resolves to existing branch via probe (`show-ref --verify --quiet refs/heads/<branch>`).
- Worktree path contention that resolves to registered worktree path via probe (`worktree list --porcelain`).
- Existing worktree checkout collisions where git indicates branch/worktree already in use.

## Terminal (non-retryable) classes
- Remote/config/repository state errors not tied to session branch/path contention.
- Permission, I/O, or repository corruption failures.
- Ambiguous failures where fallback probes do not confirm branch/path contention.

## Ambiguous failure handling
When normalized text is ambiguous (for example lock-ref failures that can represent either contention or broader ref-state issues), classification is deterministic:
1. Probe for exact branch existence for the attempted session branch.
2. If not present, probe for exact session worktree path registration.
3. Retry only if at least one probe is positive; otherwise fail fast as terminal.

## Cross-version and environment behavior
- Classification reads normalized command output when available, so generic `exit status 128` wrappers still classify correctly.
- Probe decisions are based on git state queries, not localized phrasing, improving behavior across git versions and locales.
- If probe commands fail unexpectedly, treat result as inconclusive and classify as terminal (no blind retries).

## Chosen over alternatives
- Rejected: pure stderr phrase matching only. Reason: unstable across versions/locales and wrapper error formats.
- Rejected: retry-all lock errors. Reason: can hide terminal corruption/config problems and waste retries.
- Chosen: normalized parser plus fallback probes for deterministic, state-based disambiguation.

## Follow-up implementation scope
Follow-up bead: `gromit-1fjzj` (blocked by this decision).
Scope:
- Extract classifier into dedicated unit with table-driven tests.
- Expand normalized signature map with fixture-backed samples from multiple git versions.
- Add explicit telemetry fields for class, probe path, and terminal reason.
