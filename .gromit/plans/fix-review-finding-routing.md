---
id: fix-review-finding-routing
source_spec: review-finding-routing-consistency
created: 2026-03-01
decomposed: false
research:
  - .gromit/reports/debug-20260301-123211.md
---

# Review Finding Routing Consistency Implementation Plan

**Goal:** Ensure `gromit review` persists findings consistently across interactive and non-interactive modes via a shared apply path.

**Architecture:** Extract bead/backlog/learnings persistence from `ReviewNonInteractive` into a shared `ApplyReviewFindings` Pipeline method, then wire both flows to use it. Interactive mode uses best-effort ingestion from a known file path; non-interactive mode fails closed.

**Tech Stack:** Go, existing pipeline dependency injection, `internal/review` JSON parsing

**Spec:** `.gromit/specs/review-finding-routing-consistency.md`

---

## Architecture

### Overview

The inline apply logic in `ReviewNonInteractive` (pipeline.go:348-388) becomes a standalone `Pipeline.ApplyReviewFindings` method in a new `review_apply.go` file. Both interactive and non-interactive flows call this method with a parsed `review.ReviewResult`.

### Key Components

1. **`ApplyReviewFindings(ctx, *review.ReviewResult) (*ReviewApplyResult, error)`** — New Pipeline method
   - Creates beads from `BeadsToCreate` via `TrackerClient.Create()` with `from-review` label
   - Creates backlog items from `BacklogItems` via `BacklogWriter.Add()` with labels
   - Persists learnings via `LearningsManager`
   - Returns `ReviewApplyResult` with created IDs and counts

2. **`ReviewApplyResult` type** — New type in `types.go`
   - `CreatedBeadIDs []string` — tracker issue IDs created
   - `CreatedBacklogCount int` — backlog items persisted
   - `LearningsSaved int` — learnings persisted

3. **Interactive findings ingestion (best-effort)** — Post-session step in CLI layer
   - Review prompt instructs agent to write findings JSON to `.gromit/tmp/review-findings.json`
   - CLI reads file after session, parses via `review.ParseReviewResult`, calls `ApplyReviewFindings`
   - Missing/malformed file → warning printed, session still completes successfully
   - Valid file → findings applied, summary printed with counts and IDs

### Error Handling Split

- **Non-interactive: fail closed.** Structured output is the contract. Missing or malformed JSON is a hard error.
- **Interactive: best-effort ingestion.** The review session itself is valuable (agent may have made commits, left comments, etc.). If the findings file is missing or malformed, print a warning and complete successfully. If it's present and valid, apply it and print the summary.

### Integration Points

- `ReviewNonInteractive` delegates apply to `ApplyReviewFindings` (replacing inline loop)
- `runReviewInteractive` in CLI calls `ApplyReviewFindings` after session completes (best-effort)
- Both flows produce `ReviewApplyResult` → CLI formats summary with counts and IDs

### Data Flow

```
Non-interactive:  ReviewInvoker.Run → ParseReviewResult → ApplyReviewFindings → ReviewResult (fail closed)
Interactive:      Agent session → writes findings.json → ParseReviewResult → ApplyReviewFindings → summary (best-effort)
```

### Files to Modify

- `internal/pipeline/pipeline.go` — Refactor `ReviewNonInteractive` to call `ApplyReviewFindings`
- `internal/pipeline/types.go` — Add `ReviewApplyResult` type
- `cmd/gromit/review.go` — Wire interactive completion to best-effort apply path

### Files to Create

- `internal/pipeline/review_apply.go` — Shared `ApplyReviewFindings` method
- `internal/pipeline/review_apply_test.go` — Tests for shared helper

### Tradeoffs

- **Agent output via file (not stdout)**: Interactive agents don't return structured output reliably. Writing to a known file path is deterministic and testable. Cost: prompt must instruct agent to write the file.
- **Logging/state stays in caller**: `ApplyReviewFindings` only handles artifact creation. Log writing and state updates remain in `ReviewNonInteractive` and `runReviewInteractive` respectively. Keeps the helper focused.
- **Best-effort for interactive, fail-closed for non-interactive**: Interactive sessions are inherently valuable even without structured output. Non-interactive mode's entire value is structured output, so missing output is a real failure.

---

## Test Strategy

### Unit Tests (`internal/pipeline/review_apply_test.go`)

- Mixed result (beads + backlog + learnings) → correct counts and IDs
- Empty result (no findings) → zero counts, no errors
- Nil dependencies → typed error
- Bead creation failure → error propagation
- Backlog creation failure → error propagation
- `from-review` label preserved on beads
- `from-review` + `backlog` labels on backlog entries
- Bead with no `expected_outputs` → falls back to title

### Refactor Verification (`internal/pipeline/review_test.go`)

- All existing `TestReviewNonInteractive*` tests pass without modification
- Confirms `ReviewNonInteractive` still fails closed on parse errors

### CLI Interactive Ingestion (`cmd/gromit/review_test.go`)

- Valid findings file → `ApplyReviewFindings` called, summary printed with counts and IDs
- Missing findings file → warning printed, no error returned, session completes
- Malformed JSON → warning printed, no error returned, session completes

### Parity Tests (`internal/pipeline/review_apply_test.go`)

- Given identical `review.ReviewResult`, verify same `TrackerClient.Create` and `BacklogWriter.Add` calls regardless of calling flow
- Mixed-result test verifies counts for both beads and backlog in output

### Mocking Strategy

- Reuse existing mock types from `review_test.go` (`reviewAcceptanceMock*` family)
- Real `review.ParseReviewResult` for JSON parsing paths
- Mock findings file on disk for interactive ingestion tests

### Coverage Goals

- All `ApplyReviewFindings` branches covered
- Both success and error paths for each artifact type
- Interactive best-effort path (present, missing, malformed)
- Parity between interactive and non-interactive apply semantics

---

## Implementation Tasks

### Task 1: Define ReviewApplyResult type and ApplyReviewFindings stub

**Files:**
- Modify: `internal/pipeline/types.go`
- Create: `internal/pipeline/review_apply.go`
- Test: `internal/pipeline/review_apply_test.go`

**What to Do:**
Add `ReviewApplyResult` type with `CreatedBeadIDs`, `CreatedBacklogCount`, `LearningsSaved` fields and a `NewReviewApplyResult()` constructor. Add `Pipeline.ApplyReviewFindings(ctx, *review.ReviewResult) (*ReviewApplyResult, error)` stub that validates nil deps and returns empty result.

**Acceptance Criteria:**
- `ReviewApplyResult` type compiles with proper nil-safe constructor
- `ApplyReviewFindings` with nil deps returns typed error
- `ApplyReviewFindings` with valid deps + empty ReviewResult returns zero-count result

**Dependencies:** None

### Task 2: Implement ApplyReviewFindings bead creation logic

**Files:**
- Modify: `internal/pipeline/review_apply.go`
- Test: `internal/pipeline/review_apply_test.go`

**What to Do:**
Implement the bead creation loop: iterate `BeadsToCreate`, call `TrackerClient.Create()` with `review.BuildReviewBeadLabels` and `review.ExpectedOutputsOrTitle`, collect created IDs into `ReviewApplyResult.CreatedBeadIDs`.

**Acceptance Criteria:**
- Beads created with `from-review` label + original labels
- Created bead IDs returned in result
- Bead creation error propagates (no partial result)

**Dependencies:** Task 1

### Task 3: Implement ApplyReviewFindings backlog + learnings logic

**Files:**
- Modify: `internal/pipeline/review_apply.go`
- Test: `internal/pipeline/review_apply_test.go`

**What to Do:**
Implement backlog item creation loop (with description + reason assembly, `review.BuildBacklogLabels()`, `ExpectedOutputsOrTitle`) and learnings persistence loop. Update `ReviewApplyResult` counts.

**Acceptance Criteria:**
- Backlog items created with `from-review` + `backlog` labels and correct description
- Learnings persisted via `LearningsManager.Add`
- Counts in result match actual items created

**Dependencies:** Task 2

### Task 4: Refactor ReviewNonInteractive to use ApplyReviewFindings

**Files:**
- Modify: `internal/pipeline/pipeline.go`
- Test: `internal/pipeline/review_test.go` (existing tests must still pass)

**What to Do:**
Replace the inline bead creation loop, backlog creation loop, and learnings persistence in `ReviewNonInteractive` with a single call to `ApplyReviewFindings`. Map `ReviewApplyResult` fields back into `ReviewResult` for the return value. Keep log writing and state update in `ReviewNonInteractive`.

**Acceptance Criteria:**
- All existing `TestReviewNonInteractive*` tests pass without modification
- `ReviewNonInteractive` body is shorter (inline loops replaced by single call)
- `ReviewResult` still carries correct counts

**Dependencies:** Task 3

### Task 5: Wire interactive review completion to ApplyReviewFindings (best-effort)

**Files:**
- Modify: `cmd/gromit/review.go`
- Modify: `internal/pipeline/types.go` (if `ReviewSession` needs findings path)
- Test: `cmd/gromit/review_test.go`

**What to Do:**
After interactive agent session completes, attempt to read structured findings from `.gromit/tmp/review-findings.json`. If the file exists and parses successfully, call `Pipeline.ApplyReviewFindings` and print summary with counts and IDs. If the file is missing or malformed, print a warning and complete successfully (best-effort — the session itself was valuable). The review prompt template should instruct the agent to write findings to this path.

**Acceptance Criteria:**
- Interactive review with valid findings file → beads and backlog created, summary printed with counts
- Missing findings file → warning printed, no error, session completes successfully
- Malformed JSON → warning printed, no error, session completes successfully

**Dependencies:** Task 4

### Task 6: Routing parity regression tests

**Files:**
- Modify: `internal/pipeline/review_apply_test.go`
- Modify: `cmd/gromit/review_test.go`

**What to Do:**
Add parity test: given identical `review.ReviewResult` input, verify that `ApplyReviewFindings` produces identical `TrackerClient.Create` calls (same titles, priorities, labels, outputs) and `BacklogWriter.Add` calls (same entries) regardless of whether called from interactive or non-interactive path. Add test for mixed result (both arrays populated) verifying exact counts and IDs in summary output.

**Acceptance Criteria:**
- Parity test passes proving identical routing semantics
- Mixed-result test verifies counts for both beads and backlog in output
- All existing tests still pass

**Dependencies:** Task 5

---

## Notes

- The `cliBacklogClient` adapter in `cmd/gromit/cli_adapters.go` actually creates bd beads (via `bead.Client.Create`), not separate backlog file entries. The `BacklogWriter` interface abstracts this — the shared helper doesn't need to know the implementation.
- Existing mock types in `review_test.go` can be reused for `review_apply_test.go` (same package).
- The review prompt template change (instructing agent to write findings JSON) is part of Task 5 but may require updating `internal/prompt/templates/` — scope that during implementation.
- Validation commands: `go test ./internal/pipeline/... ./cmd/gromit/...`, `go vet ./...`, `go build ./...`
