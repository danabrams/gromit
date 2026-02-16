---
id: fix-refactor-time-budget-handling
spec: n/a
created: 2026-02-16T12:17:57Z
decomposed: false
---

# Fix Refactor Time-Budget Handling Plan

**Goal:** Prevent optional refactor post-checks from failing otherwise-successful beads when bead timeout is nearly exhausted.

## Research & Context
- Investigation report: `.gromit/reports/debug-20260216-121757.md`
- Refactor post-check flow: `internal/runner/process_methodology.go`
- Timeout/error wrapping: `internal/runner/runner.go`, `internal/runner/validation/runner.go`
- Existing deadline-aware skip precedent: `internal/runner/reviewpkg/reviewer.go`

## Tasks

### Task 1: Add remaining-time guard for optional refactor phase
**Files:**
- `internal/runner/process_methodology.go`
- `internal/runner/process_test.go`

**Work:**
- Add a helper in runner methodology flow that checks remaining bead time from `ctx.Deadline()`.
- If time is expired or below configured/safe threshold, skip `RunRefactorPhase` and post-refactor re-validation.
- Log explicit skip reason including needed vs remaining time.

**Acceptance Criteria:**
- Refactor/post-check stage is skipped when remaining time is insufficient.
- Bead result remains successful when pre-refactor validation already passed.
- Skip behavior is covered by unit tests.

### Task 2: Preserve strict handling for non-timeout validation failures
**Files:**
- `internal/runner/process_methodology.go`
- `internal/runner/refactor_validation_error_test.go`

**Work:**
- Ensure the new guard only affects near-deadline behavior.
- Keep existing wrap behavior for actual refactor re-validation failures that are not deadline exhaustion.

**Acceptance Criteria:**
- Existing error wrapping tests still pass.
- Non-timeout refactor validation failures still fail the bead.

### Task 3: Validate with project quality gates
**Commands:**
- `go test -vet=off -p 4 -parallel 4 ./...`
- `go vet ./...`
- `go build ./...`

**Acceptance Criteria:**
- All commands pass.
- New/updated tests verify deadline-aware refactor skip behavior.

## Dependencies
1. Task 1 before Task 2.
2. Task 1 and Task 2 before Task 3.

## Testing Strategy
- Add focused tests that simulate near-deadline contexts for the methodology/refactor path.
- Assert refactor callback/validation are skipped in insufficient-budget cases.
- Retain existing coverage for deadline wrapping and regular validation failure behavior.
