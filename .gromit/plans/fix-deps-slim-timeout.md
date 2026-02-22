---
id: fix-deps-slim-timeout
spec: runner-pipeline
created: 2026-02-22
decomposed: false
---

# Fix: Slim Deps to Router-only (gromit-igprl.2 timeout)

## Context

Bead `gromit-igprl.2` timed out because it tried to update 40 test files with 342 call sites in a single pass. Meanwhile, bead `gromit-zbp2e` is supposed to **delete** 28 of those test files (all non-acceptance runner tests). These two beads are currently unordered — igprl.2 is attempting to update files that zbp2e will delete.

Investigation report: `.gromit/reports/debug-20260222-013616.md`

## Architecture

No architectural changes needed. The fix is a sequencing correction: delete old tests first, then update the survivors.

After `gromit-zbp2e` deletes the 28 non-acceptance runner test files, the remaining files that reference `Deps` are:

**Acceptance tests (4 files, ~25 call sites):**
- `internal/runner/acceptance/status_acceptance_test.go`
- `internal/runner/acceptance/loop_acceptance_test.go`
- `internal/runner/acceptance/runner_pipeline_acceptance_test.go`
- `internal/runner/acceptance/status_integration_acceptance_test.go`

**Pipeline tests (7 files, ~47 call sites):**
- `internal/pipeline/explore_test.go`
- `internal/pipeline/decompose_test.go`
- `internal/pipeline/pipeline_test.go`
- `internal/pipeline/review_test.go`
- `internal/pipeline/typed_interfaces_test.go`
- `internal/pipeline/typed_interfaces_behavioral_test.go`
- `internal/pipeline/refine_launch_in_dir_test.go`
- `internal/pipeline/explore_agent_input_test.go`

**CLI tests (2 files, ~4 call sites):**
- `cmd/gromit/explore_test.go`
- `cmd/gromit/refine_test.go`

Total after deletion: ~13 files, ~76 call sites — well within the 40-minute budget.

## Tasks

### Task 1: Add dependency — igprl.2 blocked by zbp2e

Add `gromit-zbp2e` as a blocker for `gromit-igprl.2`. This ensures the old runner tests are deleted before attempting the Deps simplification.

**Commands:**
```bash
bd dep add gromit-igprl.2 gromit-zbp2e
```

### Task 2: Update igprl.2 description to reflect reduced scope

Update the bead description to reference ~13 files instead of 26, and note the dependency on zbp2e for test deletion.

### Task 3: Investigate and fix gromit-zbp2e

`gromit-zbp2e` is also failing (skipped after 3 failures). Since igprl.2 now depends on it, zbp2e needs to be unblocked. This may need its own debug investigation.

## Dependencies

- Task 1 and 2 can run in parallel
- Task 3 depends on understanding why zbp2e is failing (separate investigation)

## Testing Strategy

After the dependency is added:
1. `gromit-zbp2e` runs and deletes old runner tests
2. `gromit-igprl.2` runs and updates Deps struct + ~13 surviving test files
3. `go test -tags acceptance ./internal/runner/acceptance/...` passes
4. `go build ./...` passes
