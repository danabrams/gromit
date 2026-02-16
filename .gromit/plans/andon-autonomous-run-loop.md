---
id: andon-autonomous-run-loop
source_spec: andon-autonomous-run-loop
created: 2026-02-16
decomposed: true
---

# Andon-Style Autonomous Run Loop Implementation Plan

**Goal:** Implement a bounded Andon failure policy in the run loop that maximizes autonomy while enforcing safety stops and mandatory quality gates.

**Architecture:** Add a dedicated Andon policy module that classifies failures and drives L1/L2/L3/L4 decisions, then wire it into existing runner/escalation flow with stop-line packeting and completion-gate enforcement.

**Tech Stack:** Go, existing runner/escalation packages, YAML config (`gromit.yaml`), bd/git workflow integration.

**Spec:** `.gromit/specs/andon-autonomous-run-loop.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add an explicit Andon policy layer that classifies failures and routes them through L1/L2/L3/L4 handling with bounded autonomy, while reusing the existing runner/escalation execution path.

**Key Components:**
1. **`internal/runner/andon` package**: Defines failure classes, escalation levels, thresholds, hard-stop detection, and escalation packet builder.
2. **Escalation handler integration**: `internal/runner/escalation/handler.go` delegates classification and level transitions to Andon policy decisions instead of ad-hoc branching.
3. **Runner-level stop-line gate**: `internal/runner/runner.go` / `internal/runner/run_iteration.go` halts state-changing actions on L3 and surfaces L4 options.
4. **Config extension**: `internal/config/config.go` adds an `AndonConfig` section (assumption budget, L1 retry/time cap, L2 15m cap, hard-stop controls).
5. **Observability/logging**: Structured logs include failure class, current level, elapsed recovery time, and packet summaries for operator visibility.

**Integration Points:**
- `ExecuteWithRetry` remains the execution loop, but calls into Andon policy for classify/decide transitions.
- Existing `FailureAnalyzer` output feeds Andon class mapping (Transient/Workflow/Quality/Intent/Data).
- Existing validation pipeline remains the quality gate mechanism; Andon marks tests/lint/build as mandatory completion criteria.
- Existing decomposition path remains available as one option in L4 tradeoff set.

**Data Flow:**
- Invocation fails -> analyzer + context classify failure -> Andon policy chooses level/action.
- L1: bounded quick fix attempts.
- L2: focused recovery with deterministic sequence and 15-minute cap.
- L3: stop-line state (no state-changing commands), gather repo/bead snapshot.
- L4: produce 3-option decision packet with explicit tradeoffs and recommendation.
- On success path, completion gate requires tests + lint + build before bead/session completion logic continues.

**Files to Modify:**
- `internal/runner/escalation/handler.go` - route failure handling through Andon policy; enforce level transitions.
- `internal/runner/runner.go` - add stop-line behavior hooks and completion-gate enforcement integration.
- `internal/runner/run_iteration.go` - halt/continue semantics tied to Andon stop-line state.
- `internal/config/config.go` - add `AndonConfig`, defaults, and helpers.
- `gromit.yaml` - document/configure Andon defaults.

**Files to Create:**
- `internal/runner/andon/types.go` - enums/types for levels, classes, actions.
- `internal/runner/andon/policy.go` - decision engine, thresholds, assumption budget, hard-stop checks.
- `internal/runner/andon/packet.go` - escalation packet model/formatter with 3 options + tradeoffs.
- `internal/runner/andon/policy_test.go` - unit tests for class routing and level transitions.
- `internal/runner/andon/packet_test.go` - packet completeness/format tests.

**Tradeoffs:**
- **Policy package vs embedding in handler**: chose dedicated package to keep escalation logic testable and avoid further bloating `handler.go`.
- **Configurable thresholds vs hardcoded constants**: chose config-backed defaults for flexibility while preserving deterministic defaults from the spec.
- **Strict stop-line halt vs partial continuation**: chose strict halt for L3 to satisfy integrity/safety guarantees, even at some autonomy cost.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Validate Andon classification, level transitions, thresholds, assumption-budget behavior, hard-stop detection, and escalation packet formatting.
2. **Integration Tests**: Validate runner/escalation wiring so real failure paths in `ExecuteWithRetry` produce expected L1/L2/L3/L4 behavior and stop-line halts state-changing actions.
3. **Manual/Smoke Validation**: Run representative failure scenarios and confirm logs/outputs include class, level, timing caps, and L4 3-option packet.

**Key Test Cases:**
- Transient failure retries at L1 are capped (2 retries or 2 minutes), then route to L2.
- L2 focused recovery stops at 15 minutes and escalates to L3.
- Intent ambiguity allows at most 2 assumptions; unresolved ambiguity escalates.
- Data/integrity risk triggers immediate L3 without L1/L2 retries.
- Quality-gate failures (tests/lint/build) are mandatory for completion; unclear post-recovery failures escalate.
- Hard-stop actions (bulk delete outside temp scope, irreversible migration, credential change) require explicit approval and trigger stop-line behavior when unapproved.
- Escalation packet contains all required fields and exactly 3 options with tradeoffs plus recommendation.
- Completion workflow enforces `git pull --rebase`, `bd sync`, `git push`, and up-to-date verification behavior in session completion path.

**Mocking Strategy:**
- Mock analyzer outputs to force each failure class deterministically.
- Mock command runner/bead client for workflow and hard-stop scenarios.
- Use real policy + packet code in integration tests; avoid mocking policy internals so transition logic is truly exercised.
- Reuse existing runner/escalation test scaffolding for invocation/timeout behavior.

**Coverage Goals:**
- Critical paths: all 5 failure classes mapped to correct Andon levels/actions.
- Safety paths: every hard-stop class validated.
- Completion gate: tests/lint/build required and stop-line behavior when unresolved.
- Edge cases: nil/empty analyzer output, timeout budget boundary conditions, repeated failures across levels.

**Test Organization:**
- New unit tests in `internal/runner/andon/*_test.go`.
- Escalation integration tests in `internal/runner/escalation/handler_test.go`.
- Runner behavior tests in `internal/runner/runner_test.go` and/or `internal/runner/process_test.go`.
- Config default/override tests in `internal/config/config_test.go`.

## Implementation Tasks

### Task 1: Add Andon Domain Types and Policy Skeleton

**Files:**
- Create: `internal/runner/andon/types.go`
- Create: `internal/runner/andon/policy.go`
- Test: `internal/runner/andon/policy_test.go`

**What to Do:**
Define core Andon enums and structs (failure class, level, decision, thresholds, recovery state). Implement policy entry points for classifying failures and choosing next action by level with default thresholds aligned to spec.

**Acceptance Criteria:**
- Policy supports classes `Transient|Workflow|Quality|Intent|Data` and levels `L1|L2|L3|L4`.
- L1 and L2 bounds are represented and enforced in policy decisions.
- Unit tests cover at least one decision path per failure class.

**Dependencies:**
- None

**Notes:**
Keep policy pure (no command execution) so it is deterministic and easy to test.

### Task 2: Add Config Surface for Andon Thresholds and Assumption Budget

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Modify: `gromit.yaml`

**What to Do:**
Add `AndonConfig` with defaults for autonomy controls (assumption budget=2, L1 retry/time caps, L2 15-minute cap, hard-stop toggles/allowlist conventions). Wire defaulting and YAML parsing.

**Acceptance Criteria:**
- `SetDefaults` yields spec-aligned Andon thresholds when fields are absent.
- YAML override tests verify custom values load correctly.
- `gromit.yaml` documents new fields and intent.

**Dependencies:**
- Task 1

**Notes:**
Do not remove existing escalation config; Andon config should layer on top.

### Task 3: Implement Escalation Packet Builder

**Files:**
- Create: `internal/runner/andon/packet.go`
- Test: `internal/runner/andon/packet_test.go`

**What to Do:**
Implement a standard escalation packet model/formatter containing failed command, exact error excerpt, L1/L2 attempts, state snapshot, risk level, and exactly three options with tradeoffs plus recommendation.

**Acceptance Criteria:**
- Packet includes all required fields from spec.
- Formatter outputs exactly three options and one explicit recommendation.
- Tests verify missing required fields fail validation.

**Dependencies:**
- Task 1

**Notes:**
Keep packet data structure separate from rendering so CLI/UI formatting can evolve.

### Task 4: Wire Andon Policy Into Escalation Handler

**Files:**
- Modify: `internal/runner/escalation/handler.go`
- Test: `internal/runner/escalation/handler_test.go`

**What to Do:**
Integrate policy decisions into `ExecuteWithRetry`, `AnalyzeAndHandleFailure`, timeout handling, and escalation transitions. Ensure L1/L2/L3/L4 behavior supersedes unbounded retry patterns.

**Acceptance Criteria:**
- Recoverable failures follow L1 then L2 bounded flow, then stop-line escalation.
- Integrity/unsafe-state class triggers immediate L3.
- Existing decomposition path remains available as an L4 option.

**Dependencies:**
- Task 1
- Task 2
- Task 3

**Notes:**
Preserve existing successful-path behavior and retry accounting fields where still applicable.

### Task 5: Add Runner Stop-Line Semantics and Human Decision Surfacing

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/run_iteration.go`
- Test: `internal/runner/runner_test.go`

**What to Do:**
When escalation reaches L3, halt state-changing actions (close/sync/push/merge) and emit escalation packet details; route to L4 output with three decision options and tradeoffs.

**Acceptance Criteria:**
- L3 state prevents mutation steps in current iteration.
- L4 prompt/output includes packet and options.
- Tests verify stop-line path exits safely without side effects.

**Dependencies:**
- Task 4

**Notes:**
Keep status/log writing enabled for observability even when state changes are halted.

### Task 6: Enforce Mandatory Quality Gates for Completion

**Files:**
- Modify: `internal/runner/run_iteration.go`
- Modify: `internal/runner/lifecycle.go`
- Test: `internal/runner/process_test.go`

**What to Do:**
Ensure bead/session completion path requires passing tests, lint, and build gates. If unresolved after L2 recovery, escalate instead of marking completion.

**Acceptance Criteria:**
- Completion cannot occur when any required quality gate fails.
- Unclear post-recovery quality failures escalate to stop-line path.
- Tests cover pass/fail outcomes across fast/full gate flows.

**Dependencies:**
- Task 4
- Task 5

**Notes:**
Reuse existing validation command infrastructure; avoid duplicating command execution code.

### Task 7: Add Hard-Stop Action Guardrails

**Files:**
- Modify: `internal/runner/andon/policy.go`
- Modify: `internal/runner/escalation/handler.go`
- Test: `internal/runner/andon/policy_test.go`

**What to Do:**
Detect hard-stop classes (bulk delete outside scoped tmp dirs, irreversible migration, credential/secrets changes) and force explicit approval/escalation behavior rather than autonomous execution.

**Acceptance Criteria:**
- Hard-stop actions bypass L1/L2 autonomous execution and trigger escalation.
- Bulk delete allowlist behavior is explicit and test-covered.
- Tests verify no autonomous path proceeds without approval flag/state.

**Dependencies:**
- Task 1
- Task 2
- Task 4

**Notes:**
Use conservative matching first; expand detection patterns iteratively.

### Task 8: Add Reliability Metrics and Structured Andon Logging

**Files:**
- Modify: `internal/runner/logging.go`
- Modify: `internal/runner/runtypes/types.go`
- Test: `internal/runner/format_test.go`

**What to Do:**
Record and emit metrics needed by spec (autonomy rate, first-pass success, MTTR proxy, escalation rates by class, recurrence counters). Add structured output fields for class, level, and trim decisions.

**Acceptance Criteria:**
- Iteration/run logs include failure class and Andon level for failures.
- Metrics required by spec are derivable from emitted data.
- Formatting tests validate new fields in human-readable output.

**Dependencies:**
- Task 4
- Task 5

**Notes:**
Prefer additive log schema changes to keep existing tooling compatible.

### Task 9: Session Completion Protocol Alignment

**Files:**
- Modify: `internal/runner/lifecycle.go`
- Test: `internal/runner/interfaces_test.go`

**What to Do:**
Align end-of-session flow to explicit protocol order: follow-up issues, quality gates, bead updates, `git pull --rebase`, `bd sync`, `git push`, and up-to-date verification.

**Acceptance Criteria:**
- Completion sequence contains required commands in required order.
- Failure in push/rebase path is surfaced and retried/stopped per policy.
- Tests verify up-to-date verification logic is executed.

**Dependencies:**
- Task 6

**Notes:**
Ensure this aligns with `AGENTS.md` mandatory workflow while preserving current automation flags.

---

## Notes

- Keep Andon policy logic isolated from provider-specific details; runner/escalation should pass normalized signals.
- Favor deterministic branching over heuristics for first pass to keep behavior predictable and testable.
- If an existing partial implementation appears in-flight, preserve compatibility and migrate incrementally behind clear defaults.
