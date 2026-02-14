---
id: fix-atdd-invisible-tests
spec: null
created: 2026-02-14
decomposed: false
---

# Fix: ATDD tests invisible to validation (beads closing without implementation)

## Problem

ATDD-written acceptance tests use `//go:build acceptance` tags and `*_acceptance_test.go` naming. Validation runs `go test ./...` which excludes these tests. `VerifyTestsFail` can't see the new tests, concludes "work already done," and closes beads without implementation.

See investigation report: `.gromit/reports/debug-20260214-171320.md`

## Architecture

No structural changes needed. Fix is in the ATDD prompt template and cleanup of broken test files.

## Tasks

### Task 1: Update ATDD prompt template to prevent `//go:build acceptance` on ATDD tests

**File:** `.gromit/templates/PROMPT_acceptance_tests.md`

Add explicit instructions:
- ATDD tests must be written in regular `*_test.go` files, NOT `*_acceptance_test.go`
- ATDD tests must NOT use `//go:build acceptance` tags
- ATDD tests must be runnable by `go test ./...` so the ATDD verification phase can detect their failures
- Reserve `*_acceptance_test.go` naming only for true E2E tests needing external binaries/network

Update existing line 137 ("Use build tags for true acceptance tests...") to clarify the distinction between ATDD tests and true E2E acceptance tests.

### Task 2: Delete broken acceptance test files from incorrectly closed beads

Find and delete all `*_acceptance_test.go` files that fail to compile (reference nonexistent methods/fields). These are artifacts of beads that were closed without implementation.

Known affected packages:
- `internal/runner/` (archived_hash_wiring_test.go, atdd_review_gate_wiring_test.go, final_verification_acceptance_test.go, final_verification_cleanup_acceptance_test.go)
- `internal/retro/` (archived_hash_wiring_test.go)
- `internal/config/` (decompose_config_acceptance_test.go)

Verify: `go test -tags acceptance ./...` should compile cleanly after cleanup.

### Task 3: Reopen incorrectly closed beads

Identify beads whose acceptance tests were deleted in Task 2 and reopen them so their work can be re-attempted.

## Dependencies

- Task 1 must complete before any new ATDD beads run (otherwise the same bug recurs)
- Task 2 can run in parallel with Task 1
- Task 3 depends on Task 2 (need to know which beads to reopen)

## Testing Strategy

1. After Task 1: Manually verify the template contains clear naming instructions
2. After Task 2: Run `go test -tags acceptance ./...` and confirm no compilation errors
3. After Task 3: Run `bd list --status open` and confirm affected beads are reopened
