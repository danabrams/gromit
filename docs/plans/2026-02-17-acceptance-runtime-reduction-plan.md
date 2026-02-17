# 2026-02-17 Acceptance Test Runtime Reduction Plan

> **Execution prompt:** `do docs/plans/2026-02-17-acceptance-runtime-reduction-plan.md`
>
> **Short alias prompt:** `do 2026-02-17-acceptance-runtime-reduction-plan.md`

## Goal
Reduce acceptance-suite runtime substantially by fixing hangs, removing redundant heavy acceptance coverage, and moving expensive shellout checks to the right lane.

## Baseline (Measured)
- `go test -tags acceptance ./... -count=1 -timeout=15m -json`
- `cmd/gromit`: `900.113s` (timeout failure)
- `internal/bead`: `241.323s`
- `internal/runner`: `176.918s`
- Largest single tests:
  - `TestListWithLabel_ReturnsUnlimitedResults` (`63.28s`)
  - `TestListWithLabel_CommandIncludesLimitZeroFlag` (`51.72s`)
  - `TestListWithLabel_ReturnsMoreThan50Beads` (`50.72s`)
  - `TestSplitRunnerFinalVerification_RunnerLintPasses` (`44.51s`)

## Success Criteria
1. `cmd/gromit` acceptance no longer hangs or hits package timeout.
2. `internal/bead` acceptance runtime drops by at least 60%.
3. `internal/runner` acceptance runtime drops by at least 50%.
4. Full acceptance run completes without 15-minute global timeout.
5. A timing budget check prevents regressions.

## Work Plan

### Phase 1: Fix `cmd/gromit` acceptance hang (highest impact)
**Problem**
- `TestCmdSmoke_DebugAgentResolutionEndToEnd` hangs when running via `testutil.RunGromitWithStdin(...)`.

**Files**
- `cmd/gromit/debug_agent_acceptance_test.go`
- `cmd/gromit/explore_codex_help_acceptance_test.go`
- `cmd/gromit/review_spec_validation_acceptance_test.go`
- `test/testutil/runner.go`
- `cmd/gromit/test_binary_helpers_test.go` (if helper plumbing update is needed)

**Changes**
1. Add a helper-process execution path in `test/testutil/runner.go` that supports timeout and deterministic process cleanup.
2. Update the three cmd smoke acceptance tests to use helper-process execution (same pattern as `runGromit`/`runGromitWithStdin`).
3. Enforce per-test timeout guard (e.g. `context.WithTimeout`) for child process execution.
4. Keep assertions unchanged; only change process orchestration.

**Validation**
- `go test -tags acceptance ./cmd/gromit -run TestCmdSmoke_DebugAgentResolutionEndToEnd -count=1 -timeout=60s -v`
- `go test -tags acceptance ./cmd/gromit -count=1`

---

### Phase 2: Cut heavy `internal/bead` acceptance runtime
**Problem**
- Multiple acceptance tests recreate large `bd` datasets and duplicate semantic checks.

**Files**
- `internal/bead/list_with_label_acceptance_test.go`
- `internal/bead/list_all_statuses_acceptance_test.go`
- `internal/bead/has_open_children_acceptance_test.go`
- `internal/bead/list_with_label_test.go`
- `internal/bead/bead_test.go` (setup helpers)

**Changes**
1. Keep one true acceptance case that validates `--limit 0` behavior with >50 results.
2. Reclassify duplicate flag-verification tests into unit tests using `runFn` argument assertions.
3. Reduce dataset sizes where large cardinality is not required.
4. Consolidate expensive setup using shared fixture helpers (single isolated repo per test file when safe).
5. Preserve behavior coverage while removing redundant slow acceptance flows.

**Validation**
- `go test -tags acceptance ./internal/bead -count=1`
- `go test ./internal/bead -count=1`

---

### Phase 3: Reduce `internal/runner` acceptance shellout cost
**Problem**
- Acceptance tests repeatedly run `go build ./...`, `go test ./internal/runner/...`, and `golangci-lint`.

**Files**
- `internal/runner/runner_split_phase_build_checks_test.go`
- `internal/runner/runner_split_final_verification_lint_test.go`
- `internal/runner/*reclassified*_test.go` (destination unit tests)

**Changes**
1. Keep structural/layout invariants in unit tests (AST/import/line-budget checks).
2. Remove repeated build/test shellouts from acceptance path, or gate them under `integration` tag.
3. Keep at most one targeted integration shellout check where needed.
4. Ensure split-phase acceptance intent remains covered without full toolchain reruns per phase.

**Validation**
- `go test -tags acceptance ./internal/runner -count=1`
- `go test ./internal/runner -count=1`
- Optional lane check if moved: `go test -tags integration ./internal/runner -count=1`

---

### Phase 4: Add timing budget guardrail
**Problem**
- No hard threshold enforcement specifically for acceptance lane runtime regressions.

**Files**
- `scripts/test_timing.sh` (or add `scripts/test_acceptance_timing.sh`)
- `scripts/test_package_budgets.txt` (or acceptance-specific budget file)
- `Makefile`

**Changes**
1. Add acceptance timing command that runs `go test -tags acceptance ... -json`.
2. Parse per-package elapsed time and fail if package exceeds budget.
3. Add initial budgets based on post-optimization baseline.
4. Add make target for local/CI use (`test-acceptance-timing`).

**Validation**
- `make test-acceptance-timing`
- `make test-acceptance`

## Execution Order
1. Phase 1 (unblock timeout/hang first)
2. Phase 2 (bead suite runtime reduction)
3. Phase 3 (runner shellout reduction)
4. Phase 4 (guardrail to lock gains)

## Risks
- Reclassifying tests may accidentally drop E2E coverage.
- Shared fixtures can introduce cross-test coupling if not isolated.
- Moving shellout checks may shift failures to a different lane if CI is not updated.

## Mitigations
- Keep one high-value smoke case per behavior class before reclassification.
- Use deterministic fixture setup and explicit cleanup.
- Update Make/CI targets in same change as lane movement.

## Completion Checklist
- [ ] Phase 1 merged and cmd smoke timeout eliminated
- [ ] Phase 2 merged and bead acceptance suite reduced
- [ ] Phase 3 merged and runner acceptance shellouts reduced
- [ ] Phase 4 merged with budgets enforced
- [ ] Post-change timing captured and compared against baseline

