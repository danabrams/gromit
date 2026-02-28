---
created: "2026-02-28T20:26:59Z"
decomposed: true
decomposed_at: "2026-02-28T20:35:59Z"
id: plan-20260228-ci-stabilization
spec: Stabilize CI test job by resolving guard blockers and acceptance-suite regressions
---

# Stabilize CI Guards And Acceptance Suite

## Research & Context
- Investigation report: `.gromit/reports/debug-20260228-202659.md`
- CI order: guard checks (`test-parallel-safe-top5`) then unit tests then acceptance tests.
- Unit-test cancellation assertion bug is fixed in this session; remaining work is guard + acceptance stabilization.

## Architecture Notes
- Guard scripts are intentional policy gates; avoid weakening guard semantics.
- Acceptance fixes should follow current public APIs (not reintroduce old signatures).
- Shared-state test behavior should prefer scoped test isolation over global process-state mutation.

## Tasks
1. Resolve shared-state guard violation in `cmd/gromit/test_binary_helpers_test.go`
   - Replace `os.Chdir` usage with test helper seams or per-test isolation.
   - If replacement is impossible, update `scripts/shared_state_test_calls.allowlist` with rationale in-code.
   - Validate with `make test-parallel-safe-top5`.

2. Reconcile acceptance tests with bead client API signatures
   - Update acceptance tests in `internal/bead` and `cmd/gromit` to pass `context.Context`.
   - Ensure call-site consistency for `Ready`, `ReadyWithLabel`, `ListReadyIDs`, `Create`, `Close`, `CreateWithParent`, `CreateWithDepsAndDescription`.

3. Fix acceptance-tag build issues and import-cycle failures
   - Resolve `internal/provider` acceptance import cycle via test package boundaries/helpers.
   - Restore missing helper symbols referenced by `test/contracts` and `test/e2e` acceptance tests.

4. Fix acceptance runtime assumptions
   - Address `internal/retro` template path failures in acceptance tests.
   - Align `internal/runner/acceptance` expected output strings with current rendering behavior.

5. Re-run CI-equivalent checks and capture green evidence
   - `make test-parallel-safe-top5`
   - `make test-unit`
   - `make test-acceptance`

## Dependencies
- Task 1 should complete before Task 5 because it blocks the first CI gate.
- Tasks 2-4 should complete before final acceptance validation in Task 5.
- Tasks 2 and 3 can proceed in parallel if package ownership is separated.

## Testing Strategy
- Fast iteration:
  - Targeted package tests while editing (`go test ./internal/bead -tags acceptance`, etc.).
- Gate verification:
  - Full `make test-parallel-safe-top5`, `make test-unit`, and `make test-acceptance`.
- Regression guard:
  - Keep unit test `TestPipeline_StartExploreSessionCancelClosesEventChannel` in repeated runs (`-count=20`) to confirm non-flaky behavior.
