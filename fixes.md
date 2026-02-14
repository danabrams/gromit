# Fix: False-positive auto-closes on refactoring beads

**Date:** 2026-02-14
**Commit:** a0ff2bd
**Status:** Applied, validation passing

## Symptom

Five "Split runner.go" beads (gromit-7fem, gromit-hxqv, gromit-c42l, gromit-36h6, gromit-7mdq) were auto-closed three consecutive runs in a row. None of the target files (adapters.go, callbacks.go, heartbeat_facade.go, decompose.go, gates.go, logging.go, helpers.go, lifecycle.go) existed. Zero work had been done.

Two independent paths caused the false closures:

1. **Precheck** auto-closed beads because models (especially Codex) concluded PRECHECK_PASSED without verifying file existence
2. **ATDD verify-fail** auto-closed beads because acceptance criteria like "build passes" are tautologically true before AND after refactoring

## Root Cause

### Precheck path

The precheck system relies entirely on LLM judgment to determine if acceptance criteria are already met. For refactoring/file-split beads, this fails because:

- Acceptance criteria are inherently true before refactoring — "go build passes" and "go test passes" are true both before AND after a refactoring
- 95% of precheck invocations go to Codex models — both phase 1 (low/spark) and phase 2 (medium/codex) go to the same provider family, eliminating cross-provider verification
- Models don't reliably verify file existence — despite the prompt instruction, both Codex models conclude PRECHECK_PASSED without checking that target files exist on disk

### ATDD verify-fail path

ATDD's red-green-refactor model is fundamentally incompatible with structural/refactoring beads:

- Phase 1: Claude writes acceptance tests that check "code compiles" and "tests pass"
- Phase 2: `VerifyTestsFailWithRetry` expects tests to fail before implementation — but they pass because the criteria are tautologically true
- After analysis retry and diff-aware retry, returns `ErrATDDAlreadyDone`
- `processBead` at runner.go treats this as success and auto-closes the bead

## Fix Applied

### 1. Deterministic file existence check in precheck

Added `extractExpectedFiles()` and `anyFileMissing()` functions that run before any model invocation in `runPrecheck()`. Parses bead descriptions for "Create <filepath>" patterns and checks if those files exist on disk. If any mentioned file doesn't exist, precheck immediately returns NOT_MET without invoking a model.

```go
// In runPrecheck(), before any model invocation:
if parsed := extractExpectedFiles(b.Description); len(parsed) > 0 && anyFileMissing(parsed) {
    r.log("Pre-check: description mentions files to create that don't exist, skipping model check")
    return false, time.Since(start)
}
```

The regex pattern matches lines like:
- `1. Create internal/runner/adapters.go with these moved from runner.go:`
- `Create cmd/gromit/newcmd.go with the new command`
- `2. Create pkg/utils/helper.go with helpers`

Only matches paths starting with `internal/`, `cmd/`, `pkg/`, or `test/` and ending with `.go` to avoid false positives.

### 2. Skip ATDD verify-fail for file-creation beads

In `processBead()`, added a check before ATDD Phase 2 (verify tests fail) using the same `extractExpectedFiles()` + `anyFileMissing()` helpers. When the bead describes creating files that don't exist, the verify-fail phase is skipped — but ATDD otherwise proceeds normally:

- Phase 1 (write acceptance tests): Still runs — Claude writes tests that should assert file existence
- Phase 2 (verify tests fail): **Skipped** — avoids false "already done" conclusion
- Build phase: Runs normally
- Phase 3 (verify acceptance tests pass after build): Still runs — confirms the work was done

```go
skipVerifyFail := false
if parsed := extractExpectedFiles(bc.Bead.Description); len(parsed) > 0 && anyFileMissing(parsed) {
    r.log("Skipping ATDD verify-fail: bead creates files that don't exist yet (structural change)")
    skipVerifyFail = true
}
```

### 3. Enhanced precheck template

Added a "Code Organization / Refactoring Tasks" section to `PROMPT_precheck.md` with explicit guidance for models to verify file existence for structural changes.

## Files Changed

- `internal/runner/runner.go` — Added `extractExpectedFiles()`, `anyFileMissing()`, precheck guard, ATDD verify-fail skip
- `internal/runner/runner_test.go` — Unit tests for `extractExpectedFiles` and `anyFileMissing`
- `internal/runner/test_only_atdd_skip_test.go` — Integration test for verify-fail skip in `processBead`
- `.gromit/templates/PROMPT_precheck.md` — Added refactoring-specific guidance section

## Why this works regardless of model/provider

Both fixes are deterministic code-level checks based on filesystem state, not model judgment. They run before any LLM invocation and produce the same result for Claude, Codex, or any future provider.

## What this does NOT fix

- Beads without "Create <path>" patterns in their description (e.g., pure logic changes) still rely on model judgment for precheck and ATDD verify-fail
- The two-phase precheck verification still goes to the same provider family 95% of the time
- The `ExpectedOutputs` bead field is not checked (it could be, but test data uses non-path strings)

## If false closures continue

1. **Add `precheck: cross` routing** — Force phase 2 to use a different provider from phase 1
2. **Expand the pattern** — If beads use different phrasing (e.g., "Move X to Y", "Extract Z into new file"), expand the regex
3. **Populate `ExpectedOutputs`** — Have the decompose step set `expected_outputs` on beads that create files, and check that field in precheck
4. **Disable precheck for refactoring labels** — Add a label like `precheck:skip` that bypasses precheck entirely

## Validation

```
go test ./...     — All pass
go vet ./...      — Clean
go build ./...    — Clean
```

All 5 beads reopened: gromit-7fem, gromit-hxqv, gromit-c42l, gromit-36h6, gromit-7mdq.
