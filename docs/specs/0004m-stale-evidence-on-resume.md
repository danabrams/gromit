# Spec 0004m — Stale Evidence Cleanup on Resume

## spec_id
stale-evidence-on-resume

## Vision

When a run resumes after a rework review outcome, stages re-run against reworked code — but their prior output files persist in the evidence directory from the original run. The reviewer ends up reading findings anchored to old code. The exec layer should own this transition: load prior review findings into `RunState` before clearing stale evidence, so stages always start from a clean slate while the review runner retains continuity about what was and wasn't fixed.

## Summary

When resuming a run, `exec.go` reads `review.json` into `RunState.PriorReviewFindings`, then deletes all files in the evidence directory except `review-outcome.json`. `ReviewStage` sources prior findings from `RunState` (not disk) and passes them to the review runner. The runner carries forward findings that still apply (disposition `"pre-existing"`), drops those that were fixed, and marks new issues as `"new"`.

## Goals

### Primary
- Resume path in `exec.go` reads `review.json`, stores prior findings in `RunState`, then deletes stale evidence files
- `ReviewStage` sources prior findings from `RunState`, not from disk
- Review runner carries forward unresolved findings, drops fixed ones, identifies new ones

### Secondary
- Deletion of prior evidence files is best-effort (missing files are not errors)
- `review-outcome.json` is never deleted

## Non-goals
- Changing the diff scope (full branch diff against `main` is intentional)
- Stage-level file lifecycle management
- Deterministic or programmatic verification of whether a finding was fixed (the LLM judges from the diff)

## Architecture

### `RunState` — new field

`RunState` gains a `PriorReviewFindings json.RawMessage` field, serialized with `omitempty`. Using raw JSON avoids a new import dependency between `runstore` and `review`. The exec layer writes the raw bytes from `review.json`; the stage unmarshals them.

### Resume path in `exec.go`

When resuming a run (`resumeRunID != ""`), before stages execute:

1. If `review.json` exists in the run's evidence directory, read it and store raw bytes into `rs.PriorReviewFindings`. If `review.json` is missing or cannot be read, leave `PriorReviewFindings` empty and continue.
2. Delete all files in the evidence directory **except** `review-outcome.json`. If the directory does not exist, skip deletion. If deleting an individual file fails because it no longer exists, ignore the error and continue.

The read-and-delete process runs unconditionally whenever `resumeRunID != ""`, not only after rework outcomes. `ReviewStage` handles the empty-prior-findings case gracefully.

### `ReviewStage` — source prior findings from `RunState`

`ReviewStage` already has a `priorFindings []review.Finding` field. On `Run()`, if `rs.PriorReviewFindings` is non-empty, unmarshal into `[]review.Finding` and populate `priorFindings` before invoking the runner. The existing `RunInput.PriorFindings` wiring is unchanged.

### Review runner prompt — finding triage instruction

The review prompt gains an instruction when `PriorFindings` is non-empty:

> For each prior finding: if the issue is still present in the current diff, include it with `disposition: "pre-existing"`. If it has been resolved, omit it. New issues not in prior findings use `disposition: "new"`.

## Acceptance Criteria

1. `RunState` has a `PriorReviewFindings json.RawMessage` field serialized with `omitempty`.

2. When a run is resumed and `review.json` exists in the evidence directory, `rs.PriorReviewFindings` is populated with its raw contents before any stage executes.

3. When a run is resumed, the resume path attempts to read `review.json` into `rs.PriorReviewFindings` (AC 2), then unconditionally deletes all files in the evidence directory except `review-outcome.json`, regardless of whether `review.json` was present.

4. When a run is resumed and the evidence directory contains no files (or is missing), no error is returned.

5. `ReviewStage.Run()` populates `priorFindings` from `rs.PriorReviewFindings` when non-empty, and passes them to the runner via `RunInput.PriorFindings`. When `rs.PriorReviewFindings` is empty, `priorFindings` is left as its zero value (nil or empty slice).

6. When `RunInput.PriorFindings` is non-empty, the review prompt includes an instruction to carry forward unresolved findings as `"pre-existing"`, omit resolved findings, and mark new findings as `"new"`.

7. When `RunInput.PriorFindings` is empty, the review prompt contains no prior-finding triage instruction.

8. `review-outcome.json` is never deleted by the resume path.

9. All existing tests pass.

## Scenarios

### Scenario: Resume after rework — fixed finding is dropped

**Given:** a run in `ready_for_review` with `review.json` containing one finding: `SourceRunID dead type` at `promote.go:14` with `disposition: "new"`
**And:** the rework has removed `type SourceRunID string` from `promote.go`
**When:** the run is resumed and the review stage executes
**Then:** `rs.PriorReviewFindings` contains the prior finding before stages run
**And:** all evidence files except `review-outcome.json` are absent when stages begin
**And:** the review runner receives the prior finding in `PriorFindings`
**And:** the new `review.json` does not contain a finding for `SourceRunID` (it was fixed)

### Scenario: Resume after rework — unfixed finding is retained

**Given:** a run in `ready_for_review` with `review.json` containing a finding: `os.IsNotExist` at `rejection_history.go:55` with `disposition: "new"`
**And:** the rework did not fix the `os.IsNotExist` issue
**When:** the run is resumed and the review stage executes
**Then:** the new `review.json` contains the `os.IsNotExist` finding with `disposition: "pre-existing"`

### Scenario: Resume with no prior review.json

**Given:** a run that is resumed but `review.json` does not exist in the evidence directory (e.g. first resume, stages haven't completed yet)
**When:** the resume path executes
**Then:** `rs.PriorReviewFindings` is empty
**And:** no error is returned
**And:** the review prompt contains no prior-finding triage instruction

### Scenario: review-outcome.json is preserved

**Given:** a run with both `review.json` and `review-outcome.json` in the evidence directory
**When:** the run is resumed
**Then:** `review-outcome.json` still exists after evidence cleanup
**And:** `review.json` no longer exists before stages run

## Validation

```
go test ./cmd/gromit-next/...
go test ./internal/next/specloop/stages/...
go test ./internal/next/review/...
go test ./internal/next/runstore/...
go vet ./...
go build ./...
```
