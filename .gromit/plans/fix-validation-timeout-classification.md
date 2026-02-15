---
created: 2026-02-15T21:24:01Z
decomposed: true
decomposed_at: "2026-02-15T21:27:32Z"
id: fix-validation-timeout-classification
spec: debug-validation-timeout-classification
---

# Fix Validation Timeout Classification

## Research & Context
- Investigation report: `.gromit/reports/debug-20260215-212401.md`
- Current behavior conflates bead-level deadline exhaustion with validation command timeout when `validation.command_timeout` is unset.
- This causes misleading failures like `validation command "go test ./...": context deadline exceeded` during post-refactor validation.

## Architecture
- Keep ownership of validation execution logic in `internal/runner/validation`.
- Preserve existing sentinel semantics:
  - `ErrValidationFailed` for actual command/test failures and explicit command timeout failures.
  - direct execution/cancellation errors for bead-level context cancellation/deadline.
- Keep runner-level wrapping in `internal/runner/runner.go`, but improve surface message if timeout source is bead-level.

## Tasks
1. Add timeout-origin classification in validation runner
- File: `internal/runner/validation/runner.go`
- In `runValidationWithCommands`, when command returns error and `commandCtx.Err()==context.DeadlineExceeded`:
  - detect whether parent `ctx` has expired/canceled.
  - if parent expired, return explicit wrapped context error (not `ErrValidationFailed`).
  - if explicit `validation.command_timeout` fired, keep existing timeout failure output + `ErrValidationFailed`.

2. Add/adjust tests for classification behavior
- File: `internal/runner/validation/validation_test.go`
- Add tests for:
  - parent context deadline exceeded with `command_timeout=0` returns non-sentinel execution error.
  - explicit `command_timeout` still returns sentinel `ErrValidationFailed` with timeout output.
  - canceled parent context path (context.Canceled) is surfaced distinctly.

3. Improve refactor-path error messaging
- File: `internal/runner/runner.go`
- Where re-validation wraps errors (`validation failed after refactoring`), preserve timeout origin language when error is bead/context deadline.

4. Verify no behavior regressions in direct-validation flow
- Files: existing runner validation wiring tests as needed.
- Ensure analyzer/recovery still triggers only for sentinel validation failures.

## Dependencies
1. Task 1 before Task 2 (tests depend on new behavior).
2. Task 2 before Task 3 (messaging may rely on typed/wrapped errors).
3. Task 4 after Tasks 1-3.

## Testing Strategy
- Targeted unit tests:
  - `go test ./internal/runner/validation ./internal/runner`
- Full project validation after changes:
  - `go test ./...`
  - `go vet ./...`
  - `go build ./...`

## Notes
- Operational knobs remain useful (`claude.bead_timeout`, optional `validation.command_timeout`), but this plan focuses first on correctness and debuggability of error classification.
