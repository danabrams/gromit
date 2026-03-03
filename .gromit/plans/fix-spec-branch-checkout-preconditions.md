---
id: plan-20260303-spec-branch-checkout-preconditions
spec: Fix cascading spec-branch checkout failures caused by dirty worktree preconditions
created: 2026-03-03T10:40:35Z
decomposed: true
---

# Fix Spec Branch Checkout Preconditions

## Research & Context
- Investigation report: `.gromit/reports/debug-20260303-104035.md`
- Current behavior treats branch checkout precondition failures as per-bead failures, causing repeated false failures.
- Confirmed project learning requires deterministic checkout with pre-checkout state validation.

## Architecture
- Keep branch lifecycle operations in `internal/runner/specbranch`.
- Add explicit typed checkout-precondition error(s) in `specbranch` to preserve actionable semantics across package boundaries.
- Keep orchestrator as policy owner for run progression decisions; map typed precondition errors to run-level behavior (fail-fast/stop) instead of per-bead failure loops.

## Tasks
1. Implement pre-checkout state validation in `internal/runner/specbranch/git_ops.go`.
- Add helper(s) to inspect worktree state (e.g., porcelain status) before attempting branch-switch checkout.
- Detect and classify branch-switch blocking dirty state with a typed error and concise diagnostics.

2. Add/extend tests in `internal/runner/specbranch/git_ops_test.go`.
- Cover dirty-worktree branch switch scenario.
- Assert typed error classification and actionable message content.

3. Update orchestrator checkout error handling in `internal/runner/orchestrator.go`.
- Detect typed precondition error from `GitCheckout.CreateOrCheckoutSpecBranch`.
- Avoid cascading per-bead failures for this class of run-environment error.
- Emit one deterministic operator action path (clean/stash/commit or session worktree).

4. Add orchestrator regression tests.
- Verify checkout precondition failures do not mark each subsequent bead failed.
- Verify expected event/log behavior for stop/short-circuit path.

5. Validate behavior end-to-end.
- Run targeted tests for `specbranch` and orchestrator branches.
- Run full required quality gates.

## Dependencies
1. Task 1 before Task 2.
2. Task 1 before Task 3.
3. Task 3 before Task 4.
4. Task 2 and Task 4 before Task 5.

## Testing Strategy
- Targeted:
  - `go test ./internal/runner/specbranch -run CreateOrCheckoutSpecBranch`
  - `go test ./internal/runner -run Orchestrator.*Checkout`
- Full gates:
  - `go test -vet=off -p 2 -parallel 2 ./...`
  - `go vet ./...`
  - `go build ./...`

## Decomposition
- Parent epic: `gromit-xp9zc`
- `gromit-xp9zc.1` Add deterministic pre-checkout validation and typed error in specbranch
- `gromit-xp9zc.2` Add specbranch regression tests for dirty-worktree checkout preconditions
- `gromit-xp9zc.3` Handle checkout precondition failures in orchestrator without cascade
- `gromit-xp9zc.4` Add orchestrator regression tests for non-cascading checkout precondition failures
- `gromit-xp9zc.5` Run full quality gates for checkout-precondition fix and capture results
