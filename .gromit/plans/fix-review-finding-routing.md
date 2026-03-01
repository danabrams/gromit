---
id: fix-review-finding-routing
spec: n/a-debug-investigation
created: 2026-03-01T12:32:11Z
decomposed: false
research:
  - .gromit/reports/debug-20260301-123211.md
---

# Fix Review Finding Routing Plan

## Goal
Ensure review findings from `gromit review` are consistently materialized in bd issue tracking and cannot be silently redirected into backlog-only artifacts when the user expects beads.

## Architecture
Unify review artifact persistence across interactive and non-interactive modes by introducing a shared ingestion/apply path in pipeline code. Keep rendering/invocation separate from persistence, but make persistence deterministic and testable for both modes.

## Tasks

### Task 1: Extract shared review-result apply path
- Move parse-and-apply logic in `ReviewNonInteractive` into a shared helper used by both modes.
- Helper responsibilities:
  - parse JSON review result
  - create `beads_to_create` through tracker client
  - create `backlog_items` through explicit backlog sink
  - return created IDs/counts for reporting
- Files:
  - `internal/pipeline/pipeline.go`
  - (new) `internal/pipeline/review_apply.go`
  - tests in `internal/pipeline/review_test.go`

### Task 2: Add interactive-mode result ingestion
- Extend `ReviewInteractive` flow so session completion can provide structured output for ingestion.
- Parse/apply via the shared helper from Task 1.
- Ensure interactive mode emits deterministic summary: `beads created`, `backlog created`, and created IDs.
- Files:
  - `internal/pipeline/pipeline.go`
  - `cmd/gromit/review.go`
  - session wrapper types if needed

### Task 3: Clarify mode behavior and user-facing guarantees
- Update help text and docs to explicitly describe what each mode persists.
- If interactive mode cannot produce structured output in a given run, fail closed with a clear warning and no ambiguous success message.
- Files:
  - `cmd/gromit/review.go`
  - `cmd/gromit/testdata/golden/review.help.txt`
  - docs as needed

### Task 4: Regression tests for routing correctness
- Add coverage for:
  - interactive review result with beads/backlog both present
  - malformed/no structured output in interactive mode
  - parity of counts and sink routing across modes
- Files:
  - `internal/pipeline/review_test.go`
  - `cmd/gromit/review*_test.go`

## Dependencies
1. Task 1 before Task 2
2. Task 2 before Task 3
3. Task 4 after Task 1 and Task 2

## Testing Strategy
- Unit tests around shared apply helper for parsing/routing semantics.
- Command-layer tests for mode-specific behavior and output messaging.
- Validation commands after implementation:
  - `go test -vet=off -p 2 -parallel 2 ./...`
  - `go vet ./...`
  - `go build ./...`
