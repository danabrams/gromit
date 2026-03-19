DONE 2026-03-19
# Spec 0003c — Review Stage Graceful Degradation

## spec_id
0003c-review-graceful-degradation

## Depends on
None

## Vision
The review stage currently requires a git diff to function. When `s.cfg.DiffProvider.Diff(s.cfg.BaseBranch)` returns an error — e.g., because the worktree is no longer a valid git repository — the error propagates as `fmt.Errorf("review diff: %w", err)`. Because the specloop treats any stage error as a blocker (`rs.BlockerSummary = err.Error()`), this blocks the entire run. In an observed incident, the review stage had enough evidence to evaluate all acceptance criteria (and did so successfully — 15/16 criteria passed), but the run still blocked because the diff generation failed. The review stage should degrade gracefully, proceeding with available evidence when the diff is unavailable, and only blocking on actual code quality or spec alignment findings.

## Summary
Modify the review stage to treat diff generation failure as a degraded mode rather than a hard blocker. When `DiffProvider.Diff()` returns an error, the review logs a warning event, notes "diff unavailable" in the review output, and proceeds to evaluate using available evidence (task results, acceptance criteria, file contents in the worktree). The review only returns `Blocked` or `ReplanFrom` based on actual review findings (code quality issues, spec alignment warnings), never due to diff unavailability alone.

## Goals
### Primary
- Review stage continues functioning when `DiffProvider.Diff()` returns an error
- Review evaluates acceptance criteria and code quality using available evidence (task results, file contents)
- Diff unavailability is logged as a warning event, not treated as a blocking error
- Only actual review findings (code quality, spec alignment) can block the run

### Secondary
- Review output clearly indicates when operating in degraded mode so humans can see it

## Non-goals
- Fixing the underlying cause of diff failure (that's 0003a's worktree recovery)
- Alternative diff generation methods (e.g., diffing file contents without git)
- Changing the review LLM prompt template or evaluation facet definitions — the reviewer receives different data (placeholder instead of diff) through the existing template
- Modifying the DiffProvider interface or its GitDiffProvider implementation — only the review stage's error handling changes
- Fixing the accept stage's identical hard-failure pattern for DiffProvider errors (out of scope; see Architecture notes)
- Deferred: infrastructure detection (0003a), replan context dedup (0003b), task escalation (0003d)

## Architecture

The review stage (`internal/next/specloop/stages/review.go`) currently calls `s.cfg.DiffProvider.Diff(s.cfg.BaseBranch)` — a single call that internally handles git add and git diff. When this call returns an error, `Run()` returns `fmt.Errorf("review diff: %w", err)`, which the specloop (specloop.go line 79) converts to `rs.BlockerSummary = err.Error()`, blocking the run.

The fix catches the error from `DiffProvider.Diff()` and degrades gracefully instead of propagating it. The DiffProvider interface and its implementations do not change.

**Precedent:** This pattern follows the evidence stage (`evidence.go` lines 73-79), which already degrades gracefully when `DiffProvider.Diff()` fails — it sets `diffSummary = fmt.Sprintf("[diff unavailable: %v]", err)` and continues. The review stage adopts the same approach.

**Accept stage:** The accept stage (`accept.go` lines 84-88) has the same hard-failure pattern for DiffProvider errors. Fixing it is out of scope for this spec.

**Nil DiffProvider:** The existing nil-provider path (lines 61-67 of review.go) produces an empty diff string with no warning and is unchanged — no new handling is needed.

**New event type:** A `DiffUnavailableEvent` type (with a `Message string` field) is added to `internal/next/runstore/events.go` and registered in its unmarshal switch, so the warning is recorded in the event log.

**Review evidence:** The review result written to `evidence/review.json` gains a `diff_unavailable` boolean field, set to `true` when the diff could not be generated. The `ReviewResult` struct (or equivalent returned by the LLM runner) gains a `DiffUnavailable bool` field. The review stage sets this before passing to the evidence bundler. The `WriteReviewFindings` method includes it in the evidence JSON output.

```go
// In review.go — replace the current DiffProvider error path:
if s.cfg.DiffProvider != nil {
    d, err := s.cfg.DiffProvider.Diff(s.cfg.BaseBranch)
    if err != nil {
        diffSummary = fmt.Sprintf("[diff unavailable: %v]", err)
        if s.eventLog != nil {
            s.eventLog.Append(runstore.DiffUnavailableEvent{
                BaseEvent: runstore.BaseEvent{Type: "diff_unavailable", Timestamp: time.Now()},
                Message:   fmt.Sprintf("review: diff generation failed: %v", err),
            })
        }
        // Continue — do not return error
    } else {
        diffSummary = d
    }
}
```

Key design decisions:
- The diff string is set to a descriptive placeholder when unavailable, not empty — so the LLM reviewer knows the diff was attempted but failed
- The review LLM prompt template does not change — it already handles varying amounts of evidence; the placeholder flows through the existing `DiffSummary` field
- `Run()` no longer returns an error for diff failure, which prevents the specloop from setting `BlockerSummary` from that error
- A `diff_unavailable` field is added to the review evidence JSON (`evidence/review.json`) for observability

## Acceptance Criteria

1. When `DiffProvider.Diff()` returns an error, the review stage continues execution instead of returning an error
2. The review stage passes a `"[diff unavailable: <error>]"` placeholder to the LLM reviewer in place of the actual diff
3. The review stage emits a `DiffUnavailableEvent` via `s.eventLog.Append()` when operating in degraded mode
4. The review evidence JSON (`evidence/review.json`) includes a `diff_unavailable` field set to `true` when the diff could not be generated
5. The review stage only returns `Blocked` or `ReplanFrom` based on actual review findings (blocking code quality or spec alignment issues), never due to diff unavailability
6. When diff is available, behavior is unchanged — existing review flow works exactly as before
7. The review output (review.md / review evidence) indicates "diff unavailable" when operating in degraded mode
8. All existing review stage tests continue to pass

## Scenarios

### Scenario: Diff available, review proceeds normally
**Given:** A run with a valid worktree where `DiffProvider.Diff()` succeeds, producing a diff of 50 lines
**When:** The review stage runs
**Then:** The review proceeds exactly as today — diff is sent to LLM reviewer, findings evaluated, result returned based on findings. No `DiffUnavailableEvent` emitted. `diff_unavailable` is `false` (or absent) in evidence.
**Notes:** This is the common case — degradation is invisible when things work.

### Scenario: DiffProvider returns error, review proceeds without diff
**Given:** A run where `DiffProvider.Diff()` returns an error (e.g., "fatal: not a git repository" from a git add failure, or "exit status 128" from a git diff failure — the distinction is in the error text, not in separate code paths)
**When:** The review stage runs
**Then:** The review stage emits a `DiffUnavailableEvent` with the error message, sets the diff to `"[diff unavailable: <error>]"`, proceeds to send this placeholder plus task results and acceptance criteria to the LLM reviewer, and evaluates the review result. `diff_unavailable` is `true` in evidence. `Run()` does not return an error, so the specloop does not set `BlockerSummary` from it.
**Notes:** The LLM reviewer can still evaluate acceptance criteria from task results and file evidence.

### Scenario: Diff unavailable but review finds blocking issues
**Given:** A run where `DiffProvider.Diff()` returns an error, but the LLM reviewer identifies blocking code quality issues from task results (e.g., "acceptance criterion X not met based on task evidence")
**When:** The review stage processes the review result
**Then:** The review stage returns `ReplanFrom` with the review findings as failures. The review findings reflect actual code quality or spec alignment issues, not the diff error. This is correct behavior — the review found real issues.
**Notes:** Degraded mode doesn't mean "auto-pass." The review can still block on real findings.

### Scenario: DiffProvider is nil, review proceeds with empty diff
**Given:** `DiffProvider` is nil
**When:** The review stage runs
**Then:** Behaves as today — empty diff, no warning, `diff_unavailable` is `false`. No `DiffUnavailableEvent` emitted.
**Notes:** The existing nil-provider path is unchanged; this scenario documents the baseline.

### Scenario: Diff unavailable and no blocking findings
**Given:** A run where `DiffProvider.Diff()` returns an error, but the LLM reviewer finds no blocking issues from available evidence (task results show all criteria met)
**When:** The review stage processes the review result
**Then:** The review stage returns pass. `FinalReviewPassed` is set to `true`. The review output notes "diff unavailable" but the review itself passed based on available evidence.
**Notes:** This is the key improvement — previously this case would block the run.

## Validation
- `go test ./internal/next/specloop/stages/ -count=1 -run Review`
- `go test ./internal/next/runstore/ -count=1`
- `go test ./cmd/gromit-next/ -count=1`
- `go vet ./...`
