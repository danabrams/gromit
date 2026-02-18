---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T16:25:43Z"
id: result-output-semantics
source_spec: result-output-semantics
---

# Clean Result.Output Semantics Implementation Plan

**Goal:** Remove the dead `Stdout` field from `provider.Result` and fix `Output` to always contain only the provider's semantic response text, never process-level stderr.

**Architecture:** Remove `Stdout` from the struct, stop mixing stderr into `Output` in Codex paths, update test assertions. No consumer code changes needed.

**Tech Stack:** Go

**Spec:** `.gromit/specs/result-output-semantics.md`

---

## Architecture

**Overview:**
Remove the dead `Stdout` field from `provider.Result`, enforce that `Output` is always the semantic response text (never includes process-level stderr), and update test assertions to match the new semantics.

**Semantic Contract:**

| Field | Meaning |
|-------|---------|
| `Output` | The provider's semantic response text. Consumers parse this for markers, JSON, and structured content. Never includes process-level stderr. |
| `Stderr` | Raw stderr from the subprocess. Used by ATDD logging and diagnostics. |
| `Diagnostics` | Structured diagnostic info built from args, environment, and stderr. |

**Integration Points:**
- No consumer code changes — every consumer already reads `Output`
- `Stderr` stays intact (ATDD logging reads it)
- `Diagnostics` stays intact
- Shared helpers in `helpers.go` stay — they read `Output`

**Files to Modify:**
- `internal/provider/provider.go` — Remove `Stdout` from Result struct
- `internal/provider/claude.go` — Remove `Stdout` line from `convertResult`
- `internal/provider/codex.go` — Fix 3 Result construction sites
- `internal/provider/codex_test.go` — Update 5 test assertion sites

**Tradeoffs:**
- Chose stdout-only `Output` over combined stdout+stderr because marker detection and JSON parsing need clean output, and stderr is already available via the dedicated `Stderr` field

## Test Strategy

**Test Levels:**
1. Unit tests: All changes in existing `codex_test.go` — no new test files
2. Compilation: Removing `Stdout` field causes compile-time failure if any reference is missed

**Key Test Cases:**
- `TestCodexProviderRunCapturesStdout`: Assert `result.Output` contains stdout lines
- `TestCodexProviderRunCapturesStderr`: Assert `result.Output` is empty (stderr-only command), `result.Stderr` has the error
- `TestCodexProviderRun_CreatesMissingCODEXHOME`: Parse `result.Output` for CODEX_HOME line
- `TestCodexProviderRun_RetriesTransientFailureOnce`: Assert `result.Output` contains "ok after retry"
- `TestCodexProviderStreamRun_RetriesTransientFailuresAndPreservesJSONSemantics`: Assert `result.Output` contains "done after retry"

**Coverage Goals:**
- Every Result construction site in codex.go is exercised by existing tests
- Stderr-only, stdout-only, and combined scenarios all covered
- Compile-time enforcement via field removal catches missed references

## Implementation Tasks

### Task 1: Remove Stdout from Result struct and Claude provider

**Files:**
- Modify: `internal/provider/provider.go`
- Modify: `internal/provider/claude.go`

**What to Do:**
Remove the `Stdout string` field from the `Result` struct in provider.go (line 29). Remove the `Stdout: claudeResult.Output` line from `convertResult` in claude.go (line 76).

**Acceptance Criteria:**
- `Result` struct has no `Stdout` field
- `convertResult` does not reference `Stdout`
- Code compiles after this change (modulo codex.go and codex_test.go which are fixed in subsequent tasks)

**Dependencies:**
- None (foundational)

### Task 2: Fix Codex Output semantics and remove Stdout references

**Files:**
- Modify: `internal/provider/codex.go`

**What to Do:**
Three sites in codex.go construct `Result` with `Stdout` and mixed `Output`:

1. `runOnce` (line 270-287): Change `Output` from `stdout.String() + stderr.String()` to `stdout.String()`. Remove `Stdout` field.
2. `streamRunOnce` failure path (line 165-179, cmd.Wait error): Change `Output` from `resultText + stderr.String()` to `resultText`. Remove `Stdout` field.
3. `streamRunOnce` success path (line 203-215) and stream-error path (line 187-201): Remove `Stdout` field (already identical to `Output`).

**Acceptance Criteria:**
- No Result literal in codex.go references `Stdout`
- `Output` in `runOnce` is `stdout.String()` (not combined with stderr)
- `Output` in `streamRunOnce` failure path is `resultText` (not combined with stderr)
- Code compiles

**Dependencies:**
- Task 1 (Stdout field must be removed first)

### Task 3: Update codex_test.go assertions

**Files:**
- Modify: `internal/provider/codex_test.go`

**What to Do:**
Update 5 test sites that read `result.Stdout`:

1. `TestCodexProviderRunCapturesStdout` (line 168-169): Change `result.Stdout` to `result.Output`.
2. `TestCodexProviderRunCapturesStderr` (line 208-209): Remove assertion that `result.Stdout` is empty. Add/update assertion that `result.Output` does NOT contain "Error message" (since Output no longer includes stderr). Keep `result.Stderr` assertion.
3. `TestCodexProviderRun_CreatesMissingCODEXHOME` (line 253-258): Change `result.Stdout` to `result.Output`.
4. `TestCodexProviderRun_RetriesTransientFailureOnce` (line 297-298): Change `result.Stdout` to `result.Output`.
5. `TestCodexProviderStreamRun_RetriesTransientFailuresAndPreservesJSONSemantics` (line 1060-1061): Change `result.Stdout` to `result.Output`.

Also update `TestCodexProviderRunCapturesStderr` (line 202-203): The assertion `result.Output` contains "Error message" must change — `Output` should now be empty for a stderr-only command.

**Acceptance Criteria:**
- No test references `result.Stdout`
- `TestCodexProviderRunCapturesStderr` asserts `result.Output` is empty and `result.Stderr` contains the error
- All tests pass: `go test ./internal/provider/...`

**Dependencies:**
- Task 2 (Output semantics must be fixed first)

---

## Notes

- The compiler is the primary safety net: removing the `Stdout` field will cause compile errors at any missed reference site.
- The `TestCodexProviderRunCapturesStderr` test is the most nuanced change: previously `Output` contained stderr content, now it should be empty for a stderr-only command. The `Stderr` field still captures it.
- No changes needed in `internal/runner/`, `internal/claude/`, or any consumer code.
