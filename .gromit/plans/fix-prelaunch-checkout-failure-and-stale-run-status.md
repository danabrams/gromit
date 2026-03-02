---
id: fix-prelaunch-checkout-failure-and-stale-run-status
spec: fix-prelaunch-checkout-failure-and-stale-run-status
created: 2026-03-02T13:19:41Z
decomposed: false
---

# Fix Prelaunch Checkout Failure and Stale Run Status

## Research & Context
- Investigation report: `.gromit/reports/debug-20260302-131941.md`
- Triggering bead: `gromit-yfj6`
- Symptom cluster:
  - Prelaunch branch checkout failure for non-spec bead routed to `main`
  - No escalation retry path for prelaunch checkout failures
  - Loop exits after re-offer skip detection
  - Post-loop completeness assertion aborts before `RunCompleteEvent`
  - `status.json` remains `running:true` when run exits with error

## Architecture
- Keep spec-branch workflow for spec-scoped beads (`spec:<name>`).
- Prevent non-spec beads from being forced through base-branch checkout in session worktree contexts.
- Preserve existing failure accounting and event contract while making shutdown semantics deterministic on all exit paths.
- Keep efficiency completeness checks, but avoid false-fail for prelaunch sentinel iterations.

## Tasks

### Task 1: Make non-spec branch routing/checkout safe
- Files:
  - `internal/runner/specbranch/router.go`
  - `internal/runner/orchestrator.go`
  - `internal/runner/specbranch/*_test.go`
  - `internal/runner/orchestrator*_test.go`
- Changes:
  - Ensure non-spec beads do not trigger forced checkout to `main` in spec-branch mode.
  - Preserve spec-labeled behavior (`spec:<name>` -> `gromit/spec-<name>`).
  - Add tests covering non-spec bead behavior in session worktree mode.
- Acceptance:
  - Non-spec bead no longer fails prelaunch on base-branch checkout.
  - Spec-labeled bead still performs expected checkout.

### Task 2: Surface git checkout failure detail
- Files:
  - `internal/runner/specbranch/git_ops.go`
  - `internal/runner/specbranch/git_ops_test.go`
- Changes:
  - Include stderr/stdout context in checkout failure error path so root cause is diagnosable.
- Acceptance:
  - Error returned from `CreateOrCheckoutSpecBranch` includes actionable git context.

### Task 3: Guarantee clean shutdown signaling on error exits
- Files:
  - `internal/runner/orchestrator.go`
  - `internal/runner/constructor.go` (if needed for lifecycle wiring)
  - `internal/runner/status.go`
  - `internal/runner/orchestrator_event_contract_test.go`
  - `internal/runner/status_test.go`
- Changes:
  - Ensure final lifecycle semantics run on every return path: final status update (`running=false`) and run completion/error signaling.
  - Preserve current event order constraints; if a new terminal-error event is required, define and test contract clearly.
- Acceptance:
  - After any `Run()` return (success or error), `status.json` does not remain `running:true` with dead PID.
  - Event stream has deterministic terminal signal for observers.

### Task 4: Correct efficiency completeness gate for prelaunch sentinel rows
- Files:
  - `internal/logger/efficiency.go`
  - `internal/logger/efficiency_test.go`
  - `internal/runner/orchestrator_stats.go`
  - `internal/runner/orchestrator*_test.go`
- Changes:
  - Update completeness logic so expected prelaunch/invocation sentinel rows do not fail the run-completion assertion.
  - Keep fail-closed behavior for true missing current-run rows.
- Acceptance:
  - Run with only prelaunch failure rows does not abort terminal lifecycle due false incomplete efficiency data.
  - Missing-row and true-data-gap cases still fail with diagnostics.

## Dependencies
- Task 1 before Task 2 for stable checkout path behavior.
- Task 3 can run in parallel with Task 4, then integrate.
- Task 4 must align with current rules on sentinel attribution completeness.

## Testing Strategy
- Targeted package tests during development:
  - `go test ./internal/runner/...`
  - `go test ./internal/logger/...`
- Full validation gate before merge:
  - `go test -vet=off -p 2 -parallel 2 ./...`
  - `go vet ./...`
  - `go build ./...`

## Risks
- Adjusting terminal events may affect TUI/status consumers; contract tests must be updated with explicit intent.
- Loosening completeness checks risks masking real data gaps; implementation must key off failure phase/sentinel conditions, not blanket zero-value allowance.
