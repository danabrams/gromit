---
id: gate-satisfaction-check
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-10
decomposed: false
---

# Gate Satisfaction Check Implementation Plan

**Goal:** Add a gate satisfaction check that queries open beads, evaluates their acceptance criteria against current project state, and automatically closes beads whose criteria are already satisfied.

**Architecture:** After the spec loop completes (whether in accept/review remediation cycles or after clean pass-with-improvements), run a gate satisfaction check. Query open beads that are scoped to this spec. For each bead, evaluate its acceptance criteria against the current diff and test results. If all criteria are met, close the bead with a comment. This prevents duplicated work on beads whose goals are already achieved.

## Implementation Tasks

### Task 1: Define Gate Satisfaction Types
**Files:** `internal/v2/gate/satisfaction/types.go`, `internal/v2/gate/satisfaction/types_test.go`
**What to Do:** Create `GateSatisfactionChecker` interface with a `Check(ctx context.Context, bead *trackertypes.Bead, diff string) (verdict string, satisfied bool, err error)` method. Create `SatisfactionResult` struct containing `BeadID string`, `Satisfied bool`, `EvaluationDetail string`, `ClosedAt *time.Time`. Create `SatisfactionCheckRequest` with fields: `Beads []*trackertypes.Bead`, `Diff string`, `TestResults string`, `SpecID string`.
**Acceptance Criteria:**
- GateSatisfactionChecker interface defined with Check method signature
- SatisfactionResult struct captures verdict and closure details
- SatisfactionCheckRequest captures all inputs needed for bulk checking
**Dependencies:** None

### Task 2: Implement Satisfaction Evaluation Logic
**Files:** `internal/v2/gate/satisfaction/checker.go`, `internal/v2/gate/satisfaction/checker_test.go`
**What to Do:** Implement `NewChecker()` constructor. Create `EvaluateBeadSatisfaction(ctx context.Context, bead *trackertypes.Bead, diff string, testResults string) (satisfied bool, detail string, err error)` that uses an LLM to evaluate if the bead's acceptance criteria are met by the current state. The method sends the bead title, description, and acceptance criteria alongside the diff and test results to the LLM, asking: "Are all acceptance criteria for this bead already satisfied by the changes shown? Respond with JSON: {\"satisfied\": true/false, \"detail\": \"explanation\"}". Parse the JSON response. Handle malformed responses with error return.
**Acceptance Criteria:**
- EvaluateBeadSatisfaction returns true when LLM confirms all criteria met
- EvaluateBeadSatisfaction returns false when any criterion is not yet met
- Malformed JSON responses return an error without false positive satisfaction
**Dependencies:** Task 1

### Task 3: Integrate Satisfaction Check into Spec Loop
**Files:** `internal/v2/loop/spec_loop.go`
**What to Do:** Add a `satisfactionChecker` field to `SpecLoop` or `SpecLoopConfig`. After the spec loop completes (after remediation cycles are done or after clean pass-with-improvements), call a new `checkGateSatisfaction()` method that queries open beads labeled with `spec:<specID>` from the task tracker. For each open bead, evaluate satisfaction. If satisfied, close the bead via task tracker with a comment like "Acceptance criteria satisfied by recent changes. Closed by gate satisfaction check." Continue even if closing one bead fails (collect errors but don't block). Log results and return.
**Acceptance Criteria:**
- Spec loop calls satisfaction check after remediation loop completes
- Queries open beads with correct spec label
- Closes satisfied beads with appropriate comment
- Failures closing one bead don't prevent checking others
**Dependencies:** Task 2

### Task 4: Wire Satisfaction Checker in run2_components
**Files:** `internal/v2/loop/run2_components.go`
**What to Do:** Create a `satisfactionChecker` instance in `NewRun2LoopComponents`, using a mid-tier model (sonnet) to balance cost and reasoning quality. Pass it to the `SpecLoop` during construction.
**Acceptance Criteria:**
- Satisfaction checker constructed with mid-tier model
- Passed to SpecLoop for use after remediation
**Dependencies:** Task 2

### Task 5: Add Satisfaction Check Integration Tests
**Files:** `internal/v2/gate/satisfaction/satisfaction_integration_test.go`
**What to Do:** Write integration tests covering: (1) satisfaction check closes bead when all criteria are met by the diff, (2) satisfaction check leaves bead open when some criteria remain unmet, (3) satisfaction check handles no open beads gracefully, (4) satisfaction check logs closure details. Mock LLM to return canned satisfied/unsatisfied responses. Mock task tracker for bead queries and closure. Verify correct beads are closed with correct comments.
**Acceptance Criteria:**
- Test verifies beads are closed when satisfied
- Test verifies beads remain open when partially satisfied
- Test handles empty bead list without error
- Closure comments are recorded correctly
**Dependencies:** Tasks 2, 3

