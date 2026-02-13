---
created: 2026-02-13T00:00:00Z
decomposed: true
decomposed_at: "2026-02-13T11:34:16Z"
id: tskip-audit
source_spec: tskip-audit
---

# t.Skip() Audit Implementation Plan

**Goal:** Eliminate ~100 t.Skip violations by deleting dead tests, adding build tags, and converting inappropriate skips to fatals.

**Architecture:** Pure test-file cleanup across three independent tracks — delete dead tests (B), add acceptance build tags (C), convert skip→fatal (D). No production code changes.

**Tech Stack:** Go test files only

**Spec:** `.gromit/specs/tskip-audit.md`

---

## Architecture

Three independent cleanup tracks executed in parallel:

1. **Category B (DELETE):** Remove ~56 dead/never-run tests across 10 files. Includes entire file deletion (explore_pipeline_adapter_test.go), tautological nil-check tests (provider_test.go), disabled integration tests awaiting router refactor (integration_test.go), and scattered single dead tests across 6 files.

2. **Category C (BUILD TAGS):** Add `//go:build acceptance` to 2 files (label_integration_test.go, epic_test.go) and split bead_test.go to separate 4 integration tests from ~95% unit tests.

3. **Category D (SKIP→FATAL):** Convert 17 t.Skip/t.Skipf calls to t.Fatal/t.Fatalf across 7 files where skips guard against missing source/template files that should always exist.

**Overlap with existing beads:**
- `gromit-uugq` (decompose_test.go skip) — superseded by Task 3
- `gromit-08s1` (epic_test.go build tag) — superseded by Task 4

## Test Strategy

- **Compilation**: `go build ./...` after each task
- **Unit tests**: `go test ./...` after each task — confirms no regressions
- **Acceptance tests**: `go test -tags acceptance ./...` for build-tagged files
- **Skip count**: Baseline before, compare after — expect ~100 reduction
- **bead_test.go split**: Count test functions before/after to ensure none lost

No mocks needed. No production code changes. All verification is compile + test.

---

## Implementation Tasks

### Task 1: Delete explore adapter and provider nil-check tests (B1 + B2)

**Files:**
- Delete: `cmd/gromit/explore_pipeline_adapter_test.go`
- Modify: `internal/provider/provider_test.go`

**What to Do:**
Delete the entire `explore_pipeline_adapter_test.go` file (28 tests that all immediately call t.Skip — dead scaffolding from a completed refactor). In `provider_test.go`, delete the 7 tests using the tautological `var p Provider; if p == nil { t.Skip(...) }` pattern: `TestProviderRunMethod`, `TestProviderStreamRunMethod`, `TestProviderRunValidationMethod`, `TestProviderIsUsageLimitErrorMethod`, and the 3 subtests in `TestProviderRunWithCustomTier`, `TestProviderRunValidationWithCustomTier`, `TestProviderIsUsageLimitErrorTableDriven`.

**Acceptance Criteria:**
- `explore_pipeline_adapter_test.go` no longer exists
- `provider_test.go` has no tautological nil-check skip patterns
- `go test ./cmd/gromit/... ./internal/provider/...` passes

**Dependencies:** None

### Task 2: Delete disabled runner integration tests (B3 + B4)

**Files:**
- Modify: `internal/runner/integration_test.go`
- Modify: `internal/runner/globalstats_integration_test.go`

**What to Do:**
In `integration_test.go`, delete all 13 tests that skip with "needs update for router-based provider calls" or "TODO: Update test to use Router pattern": `TestIntegration_EscalationChainFullFlow`, `TestIntegration_ValidationFailureKeepsBeadOpen`, `TestIntegration_DecompositionOnExhaustedEscalation`, `TestIntegration_RecoverableRetryThenSuccess`, `TestIntegration_StopOnFailure`, `TestIntegration_MixedResultsMultiBead`, `TestIntegration_ScopeTooLargeDetection`, `TestIntegration_FullFlowBuildValidateCloseNext`, `TestIntegration_UnclearSpecStopsProcessing`, `TestIntegration_BetweenIterationsCommand`, `TestIntegration_MultipleEscalationsWithRetries`, `TestIntegration_DryRunMultipleBeads`, `TestIntegration_LabelOverrideModelSelection`. In `globalstats_integration_test.go`, delete `TestRun_MergesWithExistingGlobalStats` at line 355. If deleting all tests from `integration_test.go` leaves it empty (or only with helper functions), delete the entire file.

**Acceptance Criteria:**
- No tests with "router-based provider calls" skip messages remain
- `go test ./internal/runner/...` passes
- Any orphaned helper functions used only by deleted tests are also removed

**Dependencies:** None

### Task 3: Delete scattered single dead tests (B5)

**Files:**
- Modify: `internal/claude/claude_test.go` (delete 3 tests)
- Modify: `cmd/gromit/explore_test.go` (delete 1 test)
- Modify: `internal/pipeline/decompose_test.go` (delete 1 test)
- Modify: `internal/logger/usage_limit_field_test.go` (delete 1 test)
- Modify: `cmd/gromit/chain_test.go` (delete 1 test)
- Modify: `internal/runner/runner_claude_field_removal_test.go` (delete 1 test)

**What to Do:**
Delete these dead test functions:
- `claude_test.go`: `TestRunWithTimeout`, `TestStreamRunTimeout`, `TestMultipleContextLayers` (empty tests skipping "for CI compatibility")
- `explore_test.go`: `TestExploreCommand_UsesPipeline` at line 69 ("full workflow tested in pipeline package")
- `decompose_test.go`: `TestDecomposeWorkflow_RespectsContextCancellation` at line 682 ("not yet implemented")
- `usage_limit_field_test.go`: `TestWriteIterationLog_PropagatesUsageLimited` at line 204 (cross-package placeholder)
- `chain_test.go`: `TestExecGromit` at line 157 ("requires integration testing")
- `runner_claude_field_removal_test.go`: `TestNewRunnerWithDepsWorksWithOnlyRouter` at line 94 ("documents desired behavior")

If any file becomes empty after deletion, delete the entire file.

**Acceptance Criteria:**
- All 8 named test functions are removed
- `go test ./...` passes (no compilation errors)

**Dependencies:** None

**Notes:** Supersedes bead `gromit-uugq` (decompose_test.go context cancellation skip).

### Task 4: Add acceptance build tags (C1 + C2)

**Files:**
- Modify: `internal/bead/label_integration_test.go`
- Modify: `cmd/gromit/epic_test.go`

**What to Do:**
Add `//go:build acceptance` as the first line of both files (followed by a blank line before the package declaration). These files contain only integration tests requiring external dependencies (`bd` binary, `gromit` binary) and should not run in normal `go test ./...`.

**Acceptance Criteria:**
- Both files have `//go:build acceptance` build tag
- `go test ./internal/bead/... ./cmd/gromit/...` passes (tests excluded from normal run)
- `go test -tags acceptance ./internal/bead/...` includes the tagged tests

**Dependencies:** None

**Notes:** Supersedes bead `gromit-08s1` (epic_test.go build tag).

### Task 5: Split bead_test.go integration tests (C-note)

**Files:**
- Modify: `internal/bead/bead_test.go`
- Create: `internal/bead/bead_integration_test.go`

**What to Do:**
Extract 4 integration tests and the `newIsolatedClient` helper from `bead_test.go` into a new `bead_integration_test.go` with `//go:build acceptance`. Tests to move: `TestClientCreate`, `TestClientCreateWithParent`, `TestClientCreateInheritsCreateWithParent`, `TestClientCreateWithDeps`. The new file gets `//go:build acceptance`, `package bead`, and the necessary imports. Verify the remaining `bead_test.go` has no references to `newIsolatedClient`.

**Acceptance Criteria:**
- `bead_integration_test.go` exists with `//go:build acceptance` and all 4 integration tests
- `bead_test.go` no longer contains `newIsolatedClient` or the 4 integration tests
- `go test ./internal/bead/...` passes (integration tests excluded)
- Total test function count across both files equals original count

**Dependencies:** None

### Task 6: Convert t.Skip to t.Fatal in source/template guards (D1 + D2)

**Files:**
- Modify: `cmd/gromit/review_agent_test.go` (7 conversions)
- Modify: `cmd/gromit/plan_agent_test.go` (2 conversions)
- Modify: `cmd/gromit/refine_agent_test.go` (1 conversion)
- Modify: `internal/prompt/prompt_test.go` (1 conversion)
- Modify: `internal/prompt/build_template_validation_section_test.go` (1 conversion)
- Modify: `internal/prompt/decompose_guidelines_acceptance_test.go` (4 conversions)
- Modify: `internal/rules/rules_test.go` (1 conversion)

**What to Do:**
In all 7 files, replace `t.Skipf` with `t.Fatalf` and `t.Skip` with `t.Fatal` for the specific skip patterns that guard against missing source files, template files, or RULES.md. These files are core project infrastructure and must always be readable — a missing file is a test infrastructure failure, not a skip condition. Do not change skip messages, only the function name (Skip→Fatal, Skipf→Fatalf).

**Acceptance Criteria:**
- All 17 t.Skip/t.Skipf calls in these files are converted to t.Fatal/t.Fatalf
- `go test ./cmd/gromit/... ./internal/prompt/... ./internal/rules/...` passes
- No new t.Skip calls introduced

**Dependencies:** None

### Task 7: Final verification and bead cleanup

**Files:** None (verification only)

**What to Do:**
Run full test suite to confirm all changes are clean. Compare skip count before and after (expect ~100 reduction from baseline of ~168). Verify `go build ./...` and `go test ./...` both pass. Close overlapping beads `gromit-uugq` and `gromit-08s1` as superseded by this work.

**Acceptance Criteria:**
- `go build ./...` passes
- `go test ./...` passes with zero test failures
- t.Skip count reduced by ~100 from baseline

**Dependencies:** Tasks 1-6

---

## Notes

- Tasks 1-6 are fully independent and can be executed in parallel
- Task 7 is the only task with dependencies (waits for all others)
- Category E (acceptance test setup failures) is explicitly out of scope per the spec
- The `bead_test.go` split (Task 5) is the most delicate change — verify test counts before and after
- If `integration_test.go` becomes empty after B3 deletions, delete the entire file rather than leaving an empty shell
- After B3 deletion, check if any helper functions in the file are orphaned (only used by deleted tests)
