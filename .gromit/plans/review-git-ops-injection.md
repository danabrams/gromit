---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T18:33:51Z"
id: review-git-ops-injection
source_spec: review-git-ops-injection
---

# Review Git Ops Injection and Bead ID Literal Matching Implementation Plan

**Goal:** Make review git helper subprocess calls injectable for deterministic unit testing and ensure bead IDs are matched literally in git log grep.

**Architecture:** Add a local execution seam in `cmd/gromit/review.go` for git subprocess invocation and rewire the five helper paths to use it, while preserving existing runtime behavior and error semantics.

**Tech Stack:** Go (`os/exec`, table-driven unit tests in `testing`)

**Spec:** `.gromit/specs/review-git-ops-injection.md`

---

## Architecture

**Overview:**  
Inject git subprocess execution into the five review helpers via package-level function dependencies in `cmd/gromit/review.go`, then test those helpers with deterministic stubs (no real git), while preserving current runtime behavior and error semantics.

**Key Components:**
1. **Review git exec seam (`cmd/gromit/review.go`)**: Add narrow injectable function vars for command construction/output execution used only by review git helpers.
2. **Helper rewiring (`cmd/gromit/review.go`)**: Update `findFirstCommitForBead`, `getCommitTimestamp`, `runGitDiffForReview`, and `getGitHeadForReview` to use injected exec path instead of direct `exec.Command`.
3. **Literal grep fix (`cmd/gromit/review.go`)**: Add `--fixed-strings` to `git log --grep` in `findFirstCommitForBead`.
4. **Deterministic tests (`cmd/gromit/review_test.go`)**: Add stub-based tests for command args, output handling, and unchanged validation/error behavior.

**Integration Points:**
- Keep scope local to review helpers; no repo-wide subprocess abstraction changes.
- Existing callers (`determineReviewScope`, dry-run diff, non-interactive state update) remain unchanged.
- Existing guardrails (`validateCommitRef`, flag-like bead ID rejection) remain and get explicit regression coverage.

**Data Flow:**
- Caller invokes review helper -> helper validates input -> helper builds git args -> injected command/output function executes -> helper parses output and returns same values/errors as today.
- In tests, injected functions return controlled output/errors and capture args for assertions (including `--fixed-strings` and `--` boundaries).

**Files to Modify:**
- `cmd/gromit/review.go` - add injection vars and rewire five helper paths; add `--fixed-strings`.
- `cmd/gromit/review_test.go` - add helper-level unit tests with injected stubs and restore hooks.

**Files to Create:**
- None required.

**Tradeoffs:**
- **Package-level function injection vs interface refactor:** choose function injection to keep scope tight and behavior stable.
- **Local seam in review.go vs shared git abstraction:** choose local seam per spec scope and lower risk.
- **Assert command args directly vs integration-with-real-git tests:** choose deterministic unit-level stubbing for speed and reliability.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** Target the five helpers in `cmd/gromit/review.go` with injected git stubs.
2. **Integration Tests:** None needed for this scoped change; existing higher-level review flows already cover command-path usage indirectly.
3. **Manual Testing:** Optional smoke check of `gromit review --dry-run` in a real repo.

**Key Test Cases:**
- `findFirstCommitForBead` uses git args including `--grep`, bead ID, and `--fixed-strings`.
- Bead ID containing regex metacharacters (for example `bead[1].*`) is passed literally and not transformed.
- `findFirstCommitForBead` returns earliest commit from multi-line git log output (last hash).
- `findFirstCommitForBead` keeps current “git command error => no commit, nil error” behavior.
- `getCommitTimestamp` rejects invalid refs unchanged and parses unix timestamp output correctly.
- `getGitDiffForReview` and `getGitDiffStatForReview` build expected args (`diff`, optional `--stat`, `<ref>`, `--`) and preserve wrapped error prefixes.
- `getGitHeadForReview` uses injected git command and preserves `git rev-parse HEAD` error wrapping semantics.
- Existing invalid input guardrails remain covered (`--flag` and empty input cases).

**Mocking Strategy:**
- Mock the injected command/output functions at package scope in `cmd/gromit/review_test.go`.
- Capture command name/args and return synthetic stdout/errors.
- Restore original function vars in `t.Cleanup` per test to avoid cross-test contamination.
- Do not launch real subprocesses in helper unit tests.

**Coverage Goals:**
- All five target helper functions execute through injected dependency path.
- Literal grep behavior and guardrails are both regression-tested.
- Error surface parity (including wrapped prefixes) is verified for diff/head helpers.

**Test Organization:**
- Keep tests in `cmd/gromit/review_test.go` near existing validation tests.
- Add focused test names like `TestFindFirstCommitForBead_UsesFixedStrings`.
- Use table-driven style for invalid-ref and invalid-bead variants where helpful.

## Implementation Tasks

### Task 1: Add Review-Local Git Execution Injection Seam

**Files:**
- Modify: `cmd/gromit/review.go`

**What to Do:**
Add package-level injectable function variables for review helper git subprocess execution, with production defaults that preserve current behavior (`exec.Command` + `cmd.Output`). Keep this seam local to review helper code and avoid broader refactors.

**Acceptance Criteria:**
- `cmd/gromit/review.go` defines injectable git execution dependencies used by review helpers.
- Production defaults preserve the same subprocess invocation semantics as current code.
- No behavior changes for callers outside these helper internals.

**Dependencies:**
- None

**Notes:**
- Keep names specific to review helpers to avoid implying repository-wide abstraction.

### Task 2: Rewire Helper Functions and Enforce Literal Bead ID Matching

**Files:**
- Modify: `cmd/gromit/review.go`

**What to Do:**
Update `findFirstCommitForBead`, `getCommitTimestamp`, `runGitDiffForReview`, and `getGitHeadForReview` to use injected subprocess dependencies. In `findFirstCommitForBead`, add `--fixed-strings` to git log grep args to enforce literal bead ID matching. Preserve validation checks and existing error-return behavior.

**Acceptance Criteria:**
- The five target helper paths no longer call `exec.Command` directly.
- `findFirstCommitForBead` uses `--fixed-strings` with `--grep`.
- Guardrails for empty or flag-like refs and bead IDs remain unchanged.
- Existing output parsing and error-wrapping semantics are preserved.

**Dependencies:**
- Task 1

**Notes:**
- Preserve special behavior where `findFirstCommitForBead` treats git command failure as “no commit found”.

### Task 3: Add Deterministic Unit Tests for Injected Git Helpers

**Files:**
- Modify: `cmd/gromit/review_test.go`

**What to Do:**
Add unit tests that stub injected subprocess functions, assert git command arguments, and verify output/error handling for each helper. Include explicit regression coverage for literal matching with regex-metacharacter bead IDs and existing invalid-input guardrails.

**Acceptance Criteria:**
- Tests can verify helper behavior without launching real git subprocesses.
- Tests assert `findFirstCommitForBead` includes `--fixed-strings` and handles metacharacter IDs literally.
- Tests cover diff, diff stat, timestamp, and head helper command wiring plus error surfaces.
- Existing validation coverage remains and passes unchanged.

**Dependencies:**
- Task 2

**Notes:**
- Use `t.Cleanup` to restore global injected function vars after each test.

### Task 4: Validate and Stabilize

**Files:**
- Modify: `cmd/gromit/review.go` (if needed for fixups)
- Modify: `cmd/gromit/review_test.go` (if needed for fixups)

**What to Do:**
Run focused tests for review command package, ensure all new tests pass, and verify no unintended behavior changes were introduced.

**Acceptance Criteria:**
- `go test ./cmd/gromit -run Review` (or equivalent targeted subset) passes.
- New helper-injection tests are stable and deterministic.
- No regressions in existing review tests.

**Dependencies:**
- Task 3

**Notes:**
- Keep test scope narrow and fast; full-suite validation can be done by the implementing bead(s) if needed.

---

## Notes

- Scope is intentionally constrained to review helper functions per spec decisions.
- This plan avoids introducing a shared cross-package git execution abstraction.
- The implementation should prioritize behavior parity over stylistic refactors.
