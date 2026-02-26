---
id: claude-timeout-classification-tests
source_spec: claude-timeout-classification-tests
created: 2026-02-26
decomposed: false
---

# Claude Timeout Classification Tests Implementation Plan

**Goal:** Add deterministic timeout classification tests for invocation, stall, and bead timeout scenarios in the Claude execution path.

**Architecture:** Reuse the existing fake-binary testing approach in Claude tests, drive each timeout condition deterministically, and assert timeout type using the same classification semantics used by runner escalation.

**Tech Stack:** Go (`testing`, `context`, `os/exec`), existing internal Claude/runner test helpers.

**Spec:** `.gromit/specs/claude-timeout-classification-tests.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add deterministic timeout-classification tests by using a controllable fake Claude binary harness and explicit classification inputs (`stallFired`, parent context error, invocation context error), while removing skip-based timeout placeholders.

**Key Components:**
1. **Deterministic fake Claude process harness (tests only)**: In `internal/claude/claude_test.go`, add helper(s) to simulate hangs before output, emits-output-then-hangs, and parent-canceled runs.
2. **Timeout classification assertion helper (tests only)**: Reuse runner timeout classification contract (`policy.ClassifyTimeout`) so tests validate timeout semantics used by escalation.
3. **Scenario-specific tests**:
   - `TestClaudeClient_InvocationTimeoutClassification`
   - `TestClaudeClient_StallTimeoutClassification`
   - `TestClaudeClient_BeadTimeoutClassification`

**Integration Points:**
- Keep production timeout behavior unchanged.
- Replace/remove skip-based timeout placeholder coverage in Claude tests.
- Validate classification semantics against escalation policy behavior rather than ad-hoc message checks alone.

**Data Flow:**
- Test invokes Claude run path with controlled fake process behavior.
- Test captures resulting error/context signal plus stall flag (where applicable).
- Test classifies via policy classification and asserts expected type (`invocation`, `stall`, `bead`).

**Files to Modify:**
- `internal/claude/claude_test.go` - add deterministic timeout scenario tests and replace skip-based placeholder timeout coverage.

**Files to Create:**
- None expected.

**Tradeoffs:**
- **Policy classifier assertions vs string-only checks**: chose policy classification assertions so test intent matches runner escalation behavior.
- **Deterministic process harness vs shell-dependent timing tests**: chose deterministic harness to reduce flakes and support `go test -count=5` stability.

## Test Strategy

**Test Levels:**
1. **Unit Tests (primary):** Add deterministic scenario tests in `internal/claude/claude_test.go` for invocation/stall/bead timeout classification.
2. **Policy-semantic assertions:** Use `policy.ClassifyTimeout` inputs/outputs to assert timeout type matches escalation semantics.
3. **Stability verification:** Execute targeted timeout tests with `go test -count=5`.

**Key Test Cases:**
- **Invocation timeout classification:** fake binary hangs before meaningful output; invocation times out; classification resolves to `invocation`.
- **Stall timeout classification:** fake binary emits output then hangs; stall cancellation path sets stall signal; classification resolves to `stall`.
- **Bead timeout classification:** parent context deadline expires mid-run; classification resolves to `bead`.
- **No skip placeholders:** new tests contain no `t.Skip` guards for these timeout scenarios.

**Mocking Strategy:**
- Reuse existing fake-binary pattern and/or `execCommandContext` override in Claude tests.
- Keep assertions independent from shell parsing behavior.
- Use deterministic deadlines and bounded waits to avoid flaky timing edges.

**Coverage Goals:**
- One deterministic passing test per timeout type (`invocation`, `stall`, `bead`).
- Ensure deadline errors remain wrap-compatible with `errors.Is(..., context.DeadlineExceeded)` where expected.
- Replace/remove existing skip-based timeout placeholder coverage in the same test area.

**Test Organization:**
- Place tests near existing timeout tests in `internal/claude/claude_test.go`.
- Use exact names:
  - `TestClaudeClient_InvocationTimeoutClassification`
  - `TestClaudeClient_StallTimeoutClassification`
  - `TestClaudeClient_BeadTimeoutClassification`

## Implementation Tasks

### Task 1: Build deterministic timeout scenario test harness

**Files:**
- Modify: `internal/claude/claude_test.go`

**What to Do:**
Add/rework local test helpers to deterministically simulate timeout-producing process behaviors needed for invocation/stall/bead scenarios without skip-based platform branching.

**Acceptance Criteria:**
- A reusable helper exists for creating fake Claude process behaviors needed by all three timeout tests.
- Helper usage does not require `t.Skip` for POSIX-only assertions.
- Helpers use bounded timing and avoid fragile wall-clock assumptions.

**Dependencies:**
- None.

**Notes:**
- Prefer test-controlled process behavior and explicit context cancellation points.

### Task 2: Add invocation timeout classification test

**Files:**
- Modify: `internal/claude/claude_test.go`

**What to Do:**
Implement `TestClaudeClient_InvocationTimeoutClassification` covering invocation-level timeout and asserting timeout type as `invocation` using policy classification semantics.

**Acceptance Criteria:**
- Test fails if timeout is classified as anything other than `invocation`.
- Test validates expected deadline error wrapping.
- Test runs deterministically under repeated execution.

**Dependencies:**
- Task 1.

**Notes:**
- Keep assertions centered on classification behavior, not just error string contents.

### Task 3: Add stall timeout classification test

**Files:**
- Modify: `internal/claude/claude_test.go`

**What to Do:**
Implement `TestClaudeClient_StallTimeoutClassification` where process emits initial output then hangs, stall detection fires, and timeout classification is asserted as `stall`.

**Acceptance Criteria:**
- Test deterministically triggers stall condition (not invocation timeout).
- Classification assertion expects `stall`.
- No `t.Skip` and no flaky sleep races.

**Dependencies:**
- Task 1.

**Notes:**
- Ensure context error inputs to classification mirror actual stall path semantics.

### Task 4: Add bead timeout classification test and remove skip placeholder coverage

**Files:**
- Modify: `internal/claude/claude_test.go`

**What to Do:**
Implement `TestClaudeClient_BeadTimeoutClassification` where parent context expires mid-run and classification resolves to `bead`; remove or replace existing skip-based timeout placeholder test coverage.

**Acceptance Criteria:**
- Test deterministically classifies parent-context timeout as `bead`.
- Existing timeout placeholder with skip-based gating is removed or superseded.
- File contains no new `t.Skip` calls for timeout-classification scenarios.

**Dependencies:**
- Task 1.
- Task 2 and Task 3 can proceed in parallel with this task after Task 1.

**Notes:**
- Be explicit in test comments about parent deadline vs invocation deadline distinction.

### Task 5: Stability verification for deterministic behavior

**Files:**
- Modify: `internal/claude/claude_test.go` (if minor stabilization fixes are needed)

**What to Do:**
Run targeted timeout tests repeatedly and adjust only if determinism issues surface.

**Acceptance Criteria:**
- `go test ./internal/claude -run 'TestClaudeClient_(Invocation|Stall|Bead)TimeoutClassification' -count=5` passes consistently.
- No flaky failures across the repeated run.

**Dependencies:**
- Tasks 2-4.

**Notes:**
- Keep fixes minimal and local to the timeout tests/helpers.

---

## Notes

- Timeout type semantics are already defined in `internal/runner/policy/escalation.go` and should remain the source of truth for classification assertions.
- This plan intentionally focuses on deterministic tests and does not require production code behavior changes unless testability blockers appear during implementation.
- If a blocker is discovered (for example, inability to surface stall conditions without broader wiring), create a follow-up bead linked via `discovered-from` before widening scope.
