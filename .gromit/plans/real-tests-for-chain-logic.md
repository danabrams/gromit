---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T08:07:58-05:00"
id: real-tests-for-chain-logic
source_spec: real-tests-for-chain-logic
---

# Real Tests for Chain Logic Implementation Plan

**Goal:** Replace 10 documentary tests across 4 files with tests that call actual production functions.

**Architecture:** Inject `confirm`/`execute` function parameters into `chainAfterRefine`, extract `buildReviewArgs` from review.go, call `parseBeadOutput`/`parseBeadOutputExcluding` directly in bead tests, delete redundant counter test.

**Tech Stack:** Go, standard library testing

**Spec:** `.gromit/specs/real-tests-for-chain-logic.md`

---

## Architecture

### `chainAfterRefine` — Dependency Injection

Change signature from:
```go
func chainAfterRefine(specNames []string, plansDir string)
```
to:
```go
func chainAfterRefine(specNames []string, plansDir string, confirm func(string, bool) bool, execute func(...string) error)
```

Replace internal `confirmPrompt(reader, ...)` calls with `confirm(prompt, defaultYes)` and `execGromit(...)` calls with `execute(...)`. The call site in `refine.go:293` passes the real implementations:
```go
reader := bufio.NewReader(os.Stdin)
chainAfterRefine(specNames, plansDir,
    func(prompt string, defaultYes bool) bool { return confirmPrompt(reader, prompt, defaultYes) },
    execGromit,
)
```

### `buildReviewArgs` — Extract from `runReviewInteractive`

Extract lines 315-317 of `review.go` into:
```go
func buildReviewArgs(flags []string, initialPrompt string) []string {
    args := make([]string, 0, len(flags)+1)
    args = append(args, flags...)
    args = append(args, initialPrompt)
    return args
}
```

Then `runReviewInteractive` calls `buildReviewArgs(cfg.Claude.Flags, initialPrompt)`.

### Bead Tests — Direct Function Calls

Replace inline JSON parsing in `TestReadyVsReadyAny` and `TestReadyExcludesEpics` with calls to `parseBeadOutput(tt.jsonOutput)` and `parseBeadOutputExcluding(tt.jsonOutput, "epic")`. Same package, no exports needed.

### Process Test — Delete

Delete `TestBeadContextRetryCounters` (lines 380-409). Already covered by `TestIntegration_EscalationChainFullFlow`.

## Test Strategy

### `chainAfterRefine` Tests (5 replacements)

Each test uses `t.TempDir()` for the plans directory and injects fake `confirm`/`execute` functions:

1. **Phase 1 tracking**: confirm=always-yes, execute creates plan files on disk → verify correct specs proceed to Phase 2
2. **Phase 2 decompose**: confirm=always-yes through both phases → verify decompose called with correct spec names
3. **Phase 3 gating**: control execute outcomes → verify `gromit run` only offered when decomposedCount > 0
4. **Decomposed count bug**: execute returns nil (simulating ExitError→nil) → verify count increments even on "failure"
5. **Break on decline**: confirm declines on second spec → verify remaining specs skipped

### `buildReviewArgs` Tests (2 replacements)

1. Flags present → output includes flags before initial prompt
2. No flags → output is just the initial prompt

### Bead Parsing Tests (2 replacements)

1. `TestReadyVsReadyAny` → call `parseBeadOutput(tt.jsonOutput)`, assert on returned bead (reuse existing table data)
2. `TestReadyExcludesEpics` → call `parseBeadOutputExcluding(tt.jsonOutput, "epic")`, assert on returned bead (reuse existing table data)

## Implementation Tasks

### Task 1: Refactor chainAfterRefine and replace 5 documentary tests

**Files:**
- Modify: `cmd/gromit/chain.go`
- Modify: `cmd/gromit/refine.go`
- Modify: `cmd/gromit/chain_integration_test.go`

**What to Do:**

1. In `chain.go`, change `chainAfterRefine` signature to accept `confirm func(string, bool) bool` and `execute func(...string) error`. Replace all `confirmPrompt(reader, ...)` calls with `confirm(...)` and all `execGromit(...)` calls with `execute(...)`. Remove the `reader := bufio.NewReader(os.Stdin)` line.

2. In `refine.go:293`, update the call site to pass real implementations:
   ```go
   reader := bufio.NewReader(os.Stdin)
   chainAfterRefine(specNames, plansDir,
       func(p string, d bool) bool { return confirmPrompt(reader, p, d) },
       execGromit,
   )
   ```
   Add `"bufio"` and `"os"` to refine.go imports if not already present.

3. In `chain_integration_test.go`, delete these 5 tests:
   - `TestChainAfterRefinePhase1Planning` (lines 135-174)
   - `TestChainAfterRefinePhase2Decompose` (lines 179-206)
   - `TestChainAfterRefinePhase3RunOnlyIfDecomposed` (lines 210-237)
   - `TestChainAfterRefineDecomposedCountIncrementsIncorrectly` (lines 242-300)
   - `TestChainAfterRefineBreakOnDecline` (lines 304-327)

4. Write 5 replacement tests that call `chainAfterRefine` with injected fakes:

   - `TestChainAfterRefinePhase1Planning`: Pass confirm=always-yes. The execute func creates plan files in the temp plansDir when called with "plan" args. Verify that the execute func was called with decompose args for the specs whose plan files exist (proving Phase 1 tracked planned names correctly and transitioned to Phase 2).

   - `TestChainAfterRefinePhase2Decompose`: Pass confirm=always-yes. Execute creates plan files for Phase 1. In Phase 2, record decompose calls. Verify decompose was offered for each planned spec.

   - `TestChainAfterRefinePhase3RunOnlyIfDecomposed`: Two sub-tests. (a) Execute returns nil in all phases → verify "run" args appear in execute calls. (b) Execute returns error (non-ExitError) for all decompose calls → verify "run" args do NOT appear. This tests the decomposedCount > 0 gate.

   - `TestChainAfterRefineDecomposedCountIncrementsIncorrectly`: Execute returns nil for decompose (simulating the bug where ExitError maps to nil). Verify that Phase 3 offers "run" even though decompose "failed" — documenting the known bug through the real code path.

   - `TestChainAfterRefineBreakOnDecline`: Pass a confirm func that returns true for the first spec and false for the second. Verify execute was only called for the first spec (remaining specs skipped).

5. Update `TestChainAfterRefineThreePhasesEmptyInput` (line 120) to pass the new parameters (confirm and execute can be nil or no-op since they won't be called for empty input).

**Acceptance Criteria:**
- `chainAfterRefine` accepts injected `confirm` and `execute` functions
- All 5 replacement tests call `chainAfterRefine` directly with fakes
- `TestChainAfterRefineThreePhasesEmptyInput` still passes with updated signature

**Dependencies:** None

**Notes:**
- The `confirm` fake needs to track which prompts it received to verify correct spec names
- The `execute` fake needs to record all calls (args) AND optionally create plan files on disk for Phase 1
- The existing bug (decomposedCount increments on exit failures) is documented by the test, not fixed — fixing is a separate bead (`ralph-runner-8wqa`)

### Task 2: Replace 2 documentary tests in bead_test.go

**Files:**
- Modify: `internal/bead/bead_test.go`

**What to Do:**

1. Rewrite `TestReadyVsReadyAny` (lines 644-752): Keep the test table data (JSON inputs, expected bead fields, wantNil flags). Replace the test body that manually parses JSON and checks for empty arrays with a call to `parseBeadOutput(tt.jsonOutput)`. Assert on the returned bead's ID and Type fields, or assert nil when expected.

2. Rewrite `TestReadyExcludesEpics` (lines 754-913): Keep the test table data. Replace the test body that manually parses JSON and filters epics with a call to `parseBeadOutputExcluding(tt.jsonOutput, "epic")`. Assert on the returned bead's ID and Type fields, or assert nil when expected.

**Acceptance Criteria:**
- Both tests call `parseBeadOutput` / `parseBeadOutputExcluding` directly
- Same test cases and expected outcomes as before
- No changes to production code

**Dependencies:** None

**Notes:**
- `parseBeadOutput` calls `jsonutil.ExtractArray` internally, which may handle some edge cases differently from raw `json.Unmarshal`. The "empty string" and "whitespace only" test cases should still return nil since `parseBeadOutput` explicitly handles `TrimSpace(out) == ""`.
- `parseBeadOutput` also calls `Validate()` on the returned bead, so test data must pass validation (existing data already does).

### Task 3: Extract buildReviewArgs and replace 2 documentary tests

**Files:**
- Modify: `cmd/gromit/review.go`
- Modify: `cmd/gromit/review_test.go`

**What to Do:**

1. In `review.go`, extract lines 315-317 into a new function:
   ```go
   func buildReviewArgs(flags []string, initialPrompt string) []string {
       args := make([]string, 0, len(flags)+1)
       args = append(args, flags...)
       args = append(args, initialPrompt)
       return args
   }
   ```
   Update `runReviewInteractive` to call `buildReviewArgs(cfg.Claude.Flags, initialPrompt)` and assign the result to `args` used on line 319.

2. Delete `TestReviewPassesClaudeFlags` and `TestReviewWithoutFlags` from `review_test.go`.

3. Write replacement tests that verify structural properties (not just hardcoded expected values). The properties that must hold for `exec.Command(binary, args...)` to work correctly with the Claude CLI:

   - `TestBuildReviewArgsWithFlags`: Call `buildReviewArgs([]string{"--dangerously-skip-permissions", "--some-flag"}, "Read the prompt")`. Assert:
     - Length equals `len(flags) + 1` (no flags lost or duplicated)
     - Last element is the prompt (Claude CLI positional arg contract)
     - All input flags appear in output, in order, before the prompt
     - Matches the inline construction: `append(append([]string{}, flags...), prompt)` — this is the extraction-fidelity check, verifying `buildReviewArgs` produces the same result as the original 3-line inline code

   - `TestBuildReviewArgsWithoutFlags`: Call `buildReviewArgs([]string{}, "Read the prompt")`. Assert:
     - Length is 1
     - The single element is the prompt
     - Matches the inline construction with empty flags

   - `TestBuildReviewArgsNilFlags`: Call `buildReviewArgs(nil, "prompt")`. Assert:
     - Length is 1, element is "prompt"
     - Verifies `append(nil, ...)` doesn't panic or produce unexpected results

**Acceptance Criteria:**
- `buildReviewArgs` is extracted and called by `runReviewInteractive`
- Tests verify structural properties (last element is prompt, flags precede prompt, correct length)
- Tests include inline-reconstruction comparison to verify extraction fidelity
- `review_test.go` no longer imports `config` (it was only used by the documentary tests)

**Dependencies:** None

**Notes:**
- The inline-reconstruction comparison (`append(append([]string{}, flags...), prompt)`) acts as a one-time extraction verification. If someone later changes `buildReviewArgs` to diverge from what `runReviewInteractive` originally did, this assertion catches it.
- The structural property assertions (last element = prompt, flags before prompt) test the actual contract that matters to `exec.Command` — these are the durable assertions.

### Task 4: Delete documentary counter test

**Files:**
- Modify: `internal/runner/process_test.go`

**What to Do:**

Delete `TestBeadContextRetryCounters` (lines 380-409). This test manually increments struct fields and asserts `i++` makes `i` equal `i+1`. The real escalation behavior is covered by `TestIntegration_EscalationChainFullFlow` in `integration_test.go`.

**Acceptance Criteria:**
- `TestBeadContextRetryCounters` is deleted
- No replacement test needed (covered by existing integration tests)
- Remaining tests in `process_test.go` are unchanged

**Dependencies:** None

### Task 5: Verify all tests pass

**Files:** None (verification only)

**What to Do:**

Run `go test ./...` and verify all tests pass. Run `go build ./cmd/gromit` to verify the build is clean. Run `go vet ./...` for any issues.

**Acceptance Criteria:**
- `go test ./...` passes
- `go build ./cmd/gromit` succeeds
- No new warnings from `go vet`

**Dependencies:** Tasks 1, 2, 3, 4

---

## Notes

- Tasks 1-4 are independent and can be worked in parallel or any order. Task 5 is the final verification gate.
- The decomposedCount bug (Task 1's fourth test) is intentionally documented, not fixed. The fix is tracked separately as bead `ralph-runner-8wqa`.
- Existing tests that already call real functions (`TestExecGromitSuccessExitZero`, `TestExecGromitNonZeroExit`, `TestExecGromitLaunchFailure`, `TestConfirmPromptDefaultBehavior`, and all bead validation tests) must remain unchanged and passing.
