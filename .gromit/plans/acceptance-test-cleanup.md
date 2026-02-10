---
id: acceptance-test-cleanup
source_spec: acceptance-test-cleanup
created: 2026-02-10
decomposed: true
---

# Acceptance Test Cleanup Implementation Plan

**Goal:** Correctly classify, deduplicate, and prune 8,370 lines of misnamed acceptance tests — preserving all meaningful coverage while reducing line count by 30%+.

**Architecture:** Rename/merge mislabeled `*_acceptance_test.go` files into existing `*_test.go` files by package, extract shared helpers, convert repetitive tests to table-driven, delete stdlib-verifying and permanently-skipped tests. Only files with `//go:build acceptance` keep the acceptance name.

**Tech Stack:** Go, standard `testing` package

**Spec:** `.gromit/specs/acceptance-test-cleanup.md`

---

## Architecture

### Current State

19 `*_acceptance_test.go` files totaling 8,370 lines across 5 packages. Only 1 file (`internal/retro/filtered_hash_eviction_acceptance_test.go`) has the `//go:build acceptance` build tag. The other 18 are unit/integration tests that run during regular `go test ./...`.

### Approach

Organize by package. For each package:
1. Audit each acceptance test file against the classification criteria
2. Merge useful tests into existing `*_test.go` files
3. Extract shared setup helpers (co-located, unexported)
4. Convert repetitive tests to table-driven
5. Delete stdlib-verifying tests and permanently-skipped tests
6. Delete the now-empty acceptance test files

### Classification Criteria

A test is a **true acceptance test** if it:
- Exercises behavior through the public command/API surface (not internal helpers)
- Tests a user-visible outcome, not an implementation detail
- Could serve as a "definition of done" for a bead

Tests that call internal functions directly are **unit tests**. Tests that wire up multiple components with mocks are **integration tests**. Both belong in `*_test.go` files.

### Tagging Rule

Every `*_acceptance_test.go` file **must** have `//go:build acceptance`. This ensures acceptance tests only run during explicit ATDD verification and don't bloat the regular test cycle.

### Package Breakdown

| Package | Files | Lines | Target Lines | Action |
|---------|-------|-------|-------------|--------|
| `cmd/gromit/` explore | 5 | 2,016 | ~600 | Merge into `explore_test.go`, table-driven, delete stdlib tests |
| `cmd/gromit/` scope | 4 | 1,915 | ~500 | Delete all-skipped file, merge rest into `review_test.go` + `retro_integration_test.go` |
| `internal/runner/` | 5 | 2,302 | ~500 | Table-driven, deduplicate overlapping mock tests, merge into `label_filter_test.go` |
| `internal/bead/` | 1 | 480 | ~300 | Merge into existing label test files |
| `internal/scope/` | 1 | 419 | ~200 | Merge into `scope_test.go` |
| `internal/retro/` | 1 | 627 | ~400 | Keep with tag, delete skipped tests, add helpers |

## Test Strategy

### Verification Approach

- **Before starting**: Run `go test ./...` and `go test -tags acceptance ./...` to establish baseline
- **After each task**: Run `go test` for affected package
- **Final verification**: Full suite + `go vet` + `golangci-lint`

### What Gets Deleted (no coverage loss)

- `TestExploreCommand_E2E_EnsuresEpicsDirExists` — tests `os.MkdirAll`
- `TestExploreCommand_WritesPromptToTempFile` — tests `os.CreateTemp` + `WriteString`
- `run_scope_acceptance_test.go` (all 11 tests) — every test is `t.Skip("Pending integration")`
- 2 skipped tests in `filtered_hash_eviction_acceptance_test.go`
- 2 skipped tests in `review_scope_acceptance_test.go`

### Coverage Goal

- Zero behavioral coverage loss
- 30%+ line reduction (8,370 → ≤ 5,859)
- No untagged `*_acceptance_test.go` files

---

## Implementation Tasks

### Task 1: Capture skipped test ideas in backlog and delete entirely-skipped files

**Files:**
- Delete: `cmd/gromit/run_scope_acceptance_test.go`

**What to Do:**
Run `gromit add` for each skipped test that describes desired but unimplemented behavior:
- `run_scope_acceptance_test.go`: scope flags (`--spec`, `--epic`) integration with `runLoop()`
- `review_scope_acceptance_test.go`: 2 skipped tests about `--spec` flag priority and no-matching-beads handling
- `filtered_hash_eviction_acceptance_test.go`: 2 skipped tests about single-save optimization and archived learnings

Then delete `run_scope_acceptance_test.go` entirely (all 11 tests are skipped).

**Acceptance Criteria:**
- Backlog ideas captured via `gromit add` for skipped test behaviors
- `run_scope_acceptance_test.go` deleted
- `go test ./cmd/gromit/...` passes

**Dependencies:** None

---

### Task 2: Reclassify and consolidate `cmd/gromit/explore_*` tests

**Files:**
- Modify: `cmd/gromit/explore_test.go`
- Delete: `cmd/gromit/explore_acceptance_test.go`
- Delete: `cmd/gromit/explore_prompt_acceptance_test.go`
- Delete: `cmd/gromit/explore_session_acceptance_test.go`
- Delete: `cmd/gromit/explore_command_acceptance_test.go`
- Delete: `cmd/gromit/explore_integration_acceptance_test.go`

**What to Do:**
1. Extract `setupExploreTest(t *testing.T)` helper into `explore_test.go` — returns config, gromitDir, handles temp dir + CLAUDE.md + templates
2. Delete `TestExploreCommand_E2E_EnsuresEpicsDirExists` (stdlib test) and `TestExploreCommand_WritesPromptToTempFile` (stdlib test)
3. Convert `buildExplorePrompt` tests (from `explore_prompt_acceptance_test.go` and `explore_command_acceptance_test.go`) to a single table-driven test
4. Merge remaining useful tests from all 5 files into `explore_test.go`
5. Delete the 5 acceptance test files

**Acceptance Criteria:**
- No `explore_*_acceptance_test.go` files exist
- `setupExploreTest` helper exists in `explore_test.go`
- `buildExplorePrompt` tests are table-driven
- `go test ./cmd/gromit/...` passes

**Dependencies:** None

---

### Task 3: Reclassify and consolidate `cmd/gromit/review_*` and `retro_*` scope tests

**Files:**
- Modify: `cmd/gromit/review_test.go`
- Modify: `cmd/gromit/retro_integration_test.go`
- Delete: `cmd/gromit/review_scope_acceptance_test.go`
- Delete: `cmd/gromit/review_mutual_exclusivity_acceptance_test.go`
- Delete: `cmd/gromit/retro_scope_acceptance_test.go`
- Delete: `cmd/gromit/retro_cli_acceptance_test.go`
- Delete: `cmd/gromit/retro_e2e_acceptance_test.go`

**What to Do:**
1. Merge non-skipped tests from `review_scope_acceptance_test.go` into `review_test.go`, delete 2 skipped tests (ideas already captured in Task 1)
2. Merge `review_mutual_exclusivity_acceptance_test.go` into `review_test.go`
3. Merge `retro_scope_acceptance_test.go`, `retro_cli_acceptance_test.go`, and `retro_e2e_acceptance_test.go` into `retro_integration_test.go`
4. Delete the 5 acceptance test files

**Acceptance Criteria:**
- No `review_*_acceptance_test.go` or `retro_*_acceptance_test.go` files exist in `cmd/gromit/`
- All non-skipped test behavior preserved in merge targets
- `go test ./cmd/gromit/...` passes

**Dependencies:** Task 1 (backlog ideas captured before deleting skipped tests)

---

### Task 4: Reclassify and consolidate `internal/runner/` acceptance tests

**Files:**
- Modify: `internal/runner/label_filter_test.go` (or `runner_labels_test.go`)
- Delete: `internal/runner/runner_label_filtering_acceptance_test.go`
- Delete: `internal/runner/bead_client_interface_acceptance_test.go`
- Delete: `internal/runner/interfaces_label_methods_acceptance_test.go`
- Delete: `internal/runner/bead_client_label_methods_acceptance_test.go`
- Delete: `internal/runner/label_filtering_acceptance_test.go`

**What to Do:**
1. Convert `runner_label_filtering_acceptance_test.go` (1,026 lines) to table-driven format targeting ~300 lines. Extract `setupLabelFilterTest` helper.
2. Deduplicate the 4 smaller files — they all test mock implementations of `ReadyWithLabel`/`ListWithLabel`. Merge unique tests, discard duplicates.
3. Merge everything into existing `label_filter_test.go` or `runner_labels_test.go`
4. Delete all 5 acceptance test files

**Acceptance Criteria:**
- No `*_acceptance_test.go` files in `internal/runner/`
- Label filtering tests are table-driven and under 400 lines
- `setupLabelFilterTest` helper exists
- `go test ./internal/runner/...` passes

**Dependencies:** None

---

### Task 5: Reclassify `internal/bead/` and `internal/scope/` acceptance tests

**Files:**
- Modify: `internal/bead/bead_test.go` (or existing label test files)
- Modify: `internal/scope/scope_test.go`
- Delete: `internal/bead/label_methods_acceptance_test.go`
- Delete: `internal/scope/validate_flags_three_way_acceptance_test.go`

**What to Do:**
1. Merge `label_methods_acceptance_test.go` into existing bead test files. Keep `BD_AVAILABLE`-gated tests (contract verification against `bd` CLI).
2. Merge `validate_flags_three_way_acceptance_test.go` into `scope_test.go`
3. Delete both acceptance test files

**Acceptance Criteria:**
- No `*_acceptance_test.go` files in `internal/bead/` or `internal/scope/`
- `go test ./internal/bead/...` passes
- `go test ./internal/scope/...` passes

**Dependencies:** None

---

### Task 6: Clean up `internal/retro/filtered_hash_eviction_acceptance_test.go`

**Files:**
- Modify: `internal/retro/filtered_hash_eviction_acceptance_test.go`

**What to Do:**
1. Delete the 2 skipped tests (`TestFilteredHashEviction_SingleSaveWhenBothAddAndPrune`, `TestFilteredHashEviction_HandlesArchivedLearnings`) — ideas already captured in Task 1
2. Extract setup helper for remaining tests
3. Verify `//go:build acceptance` tag is present

**Acceptance Criteria:**
- No `t.Skip` tests remain in the file
- Setup helper extracted for remaining tests
- `//go:build acceptance` tag present
- `go test -tags acceptance ./internal/retro/...` passes

**Dependencies:** Task 1 (backlog ideas captured first)

---

### Task 7: Final verification and line count audit

**Files:** None (read-only verification)

**What to Do:**
1. Run `go test ./...` — all pass
2. Run `go test -tags acceptance ./...` — all pass
3. Run `go vet ./...` — clean
4. Run `golangci-lint run ./...` — clean
5. Verify no `*_acceptance_test.go` file exists without `//go:build acceptance`
6. Count total test lines before vs. after — confirm 30%+ reduction
7. Spot-check that key behaviors still have test coverage

**Acceptance Criteria:**
- All verification commands pass
- No untagged `*_acceptance_test.go` files
- Total line reduction >= 30% (8,370 → ≤ 5,859)

**Dependencies:** Tasks 1-6

---

## Notes

- **No production code changes.** This is entirely test refactoring.
- Tasks 1, 2, 4, and 5 are independent and can be parallelized. Tasks 3 and 6 depend on Task 1.
- When merging tests into existing files, place them after existing tests with clear function names — no need for section comments.
- The `BD_AVAILABLE`-gated tests in `internal/bead/` are intentionally environment-gated, not skipped. They serve a real purpose (contract verification) and should be preserved.
- `run_scope_acceptance_test.go` deletion is safe because it contains zero executable test assertions — every test body is `t.Skip(...)`.
