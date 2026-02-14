# Fix: Precheck false-positive auto-closes on refactoring beads

**Date:** 2026-02-14
**Status:** Applied, validation passing

## Symptom

Five "Split runner.go" beads (gromit-7fem, gromit-hxqv, gromit-c42l, gromit-36h6, gromit-7mdq) were auto-closed by precheck three consecutive runs in a row. None of the target files (adapters.go, callbacks.go, heartbeat_facade.go, decompose.go, gates.go, logging.go, helpers.go, lifecycle.go) existed. Zero work had been done.

## Root Cause

The precheck system relies entirely on LLM judgment to determine if acceptance criteria are already met. For refactoring/file-split beads, this fails because:

1. **Acceptance criteria are inherently true before refactoring** — "go build passes" and "go test passes" are true both before AND after a refactoring. The precheck template warns about this ("Do NOT assume structural changes are complete just because the code would compile") but models don't reliably follow this instruction.

2. **95% of precheck invocations go to Codex models** — The routing config (`openai: 95, claude: 5`) sends almost all prechecks to Codex. Both phase 1 (low/spark) and phase 2 (medium/codex) go to the same provider family, eliminating the cross-provider verification benefit the two-phase system was designed to provide.

3. **Models don't reliably verify file existence** — Despite the prompt saying "you MUST verify those files exist by attempting to read them", both Codex models conclude PRECHECK_PASSED without actually checking that `adapters.go`, `callbacks.go`, etc. exist on disk.

## Fix Applied

Added a **deterministic file existence check** in `runPrecheck()` that runs before any model invocation. It parses the bead description for "Create <filepath>" patterns (e.g., `Create internal/runner/adapters.go`) and checks if those files exist on disk. If any mentioned file doesn't exist, the precheck immediately returns NOT_MET without invoking a model.

### Files changed

- `internal/runner/runner.go` — Added `extractExpectedFiles()`, `anyFileMissing()`, and a pre-model check in `runPrecheck()`
- `internal/runner/runner_test.go` — Added `TestExtractExpectedFiles` and `TestAnyFileMissing` unit tests

### How it works

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

It only matches paths starting with `internal/`, `cmd/`, `pkg/`, or `test/` and ending with `.go` to avoid false positives.

### Why this works regardless of model/provider

This is a code-level check, not a model-level check. It runs before any LLM invocation and produces a deterministic result based on filesystem state. It works identically for Claude, Codex, or any future provider.

## What this does NOT fix

- Beads without "Create <path>" patterns in their description (e.g., pure logic changes) still rely on model judgment
- The two-phase verification still goes to the same provider family 95% of the time (consider adding `precheck: cross` routing preference if this remains a problem)
- The `ExpectedOutputs` bead field is not checked in precheck (it could be, but test data uses non-path strings in this field)

## If this doesn't fix it

If false-positive auto-closes continue after this fix, the next steps would be:

1. **Add `precheck: cross` routing** — Force phase 2 to use a different provider from phase 1, ensuring genuine cross-provider verification
2. **Expand the pattern** — The regex currently matches `Create <path>.go` at line start. If beads use different phrasing (e.g., "Move X to Y", "Extract Z into new file"), expand the pattern
3. **Populate `ExpectedOutputs`** — Have the decompose step set `expected_outputs` on beads that create files, and check that field in precheck
4. **Disable precheck for refactoring labels** — Add a label like `precheck:skip` that bypasses precheck entirely for known-problematic bead types

## Validation

```
go test ./...     — All pass
go vet ./...      — Clean
go build ./...    — Clean
```

All 5 beads reopened: gromit-7fem, gromit-hxqv, gromit-c42l, gromit-36h6, gromit-7mdq.
