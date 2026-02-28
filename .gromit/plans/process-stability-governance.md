---
id: process-stability-governance
source_spec: process-stability-governance
created: 2026-02-28
decomposed: false
---

# Process Stability Governance Implementation Plan

**Goal:** Replace static run-loop caps with SPC-driven stability controls that stop unstable TDD churn early, surface test-maintenance bloat, and enforce systemic self-correction in gate/decomposition behavior.

**Architecture:** Add a convergence monitor in the TDD loop, extend process trend modeling with convergence and maintenance signals, and enforce spec-level anomaly policies in Gate while surfacing maintenance warnings in status and retro.

**Tech Stack:** Go, Cobra CLI, internal SPC/trend telemetry (`internal/logger`), pipeline stages (`internal/pipeline/*`), runner/TDD orchestration (`internal/runner/*`).

**Spec:** `.gromit/specs/process-stability-governance.md`

---

## Architecture

**Overview:**
Introduce a process-stability control plane that uses measured instability (convergence deadlock/oscillation and SPC anomalies) rather than hard numeric quotas (`max_tdd_cycles`) to decide whether to continue work, block scope, or trigger decomposition.

**Key Components:**
1. **TDD Convergence Monitor (`internal/runner/tdd`)**: Compute a per-cycle complexity delta from diff footprint, validation failure pressure, and token burn; evaluate the last 3 cycles for deadlock and oscillation.
2. **Trend Model Extensions (`internal/logger`)**: Persist convergence telemetry and package-level validation-time streak signals to `process_trend.json`, with deterministic anomaly severity classification.
3. **Spec-Level SPC Gate Policy (`internal/pipeline/prepare`)**: Read process trend and block beads whose `spec:*` label maps to high-severity rolling rework anomalies.
4. **Status/Retro Maintenance Signaling (`internal/runner/display`, `internal/retro`)**: Highlight packages exceeding validation-time UCL for 3 consecutive iterations as **High Maintenance Cost**.
5. **Config Deprecation Behavior (`internal/config`, runner wiring)**: Keep parsing `max_tdd_cycles` for compatibility but remove it from TDD termination semantics.

**Integration Points:**
- TDD orchestrator loop transitions from `MaxCycles`-based stop condition to convergence-based stop condition with readiness-classified Andon-style outcome (`not_ready_scope` / `not_ready_criteria`).
- Gate stage consumes trend anomalies keyed by spec label and emits a deterministic block reason when systemic rework instability is detected.
- Existing proactive decomposition flow gains SPC-trigger checks for duration/token outliers by tier.
- Status SPC section and retro prompt context include high-maintenance package warnings from trend snapshot.

**Data Flow:**
1. Iteration/TDD logs -> `buildIterationMetrics` and SPC builders -> enriched `process_trend.json`.
2. TDD runtime records cycle deltas -> convergence evaluator -> instability classification -> early stop decision.
3. Gate runtime reads `process_trend.json` + bead spec label -> block/proceed decision.
4. Status/retro renderers read trend warnings -> user-visible maintenance guidance.

**Files to Modify:**
- `internal/logger/process_trend.go` - add trend model fields for convergence and maintenance-cost signals.
- `internal/logger/trend_builder.go` - compute convergence-derived summaries and package UCL breach streaks.
- `internal/logger/trend_spc.go` - ensure anomaly/control-limit support for new signals and severity handling.
- `internal/runner/tdd/handoff.go` - remove completion dependency on `MaxCycles`.
- `internal/runner/tdd/orchestrator.go` - enforce convergence-based termination and emit instability metadata.
- `internal/runner/callbacks_tdd.go` - stop relying on `cfg.Methodology.MaxTDDCycles` for loop completion.
- `internal/pipeline/prepare/gate.go` - integrate spec-level rework anomaly blocking policy.
- `internal/runner/display/display.go` - render High Maintenance Cost warnings in status output.
- `internal/retro/retro.go` (and template context usage) - surface maintenance warnings in retro analysis input.
- `internal/config/config_types.go` / `internal/config/config_defaults.go` - mark `max_tdd_cycles` deprecated and ignored by runtime termination.

**Files to Create:**
- `internal/runner/tdd/convergence.go` - convergence scoring and deadlock/oscillation detection logic.
- `internal/runner/tdd/convergence_test.go` - unit tests for convergence heuristics.
- `internal/pipeline/prepare/spec_spc_blocker.go` - isolated trend/spec blocking evaluator.
- `internal/pipeline/prepare/spec_spc_blocker_test.go` - evaluator tests for block/no-block behavior.

**Tradeoffs:**
- **Runtime convergence check vs post-run trend-only detection**: Runtime checks were chosen so unstable loops stop immediately instead of after wasted cycles.
- **Spec-stratified anomaly gating vs global anomaly gating**: Spec scoping reduces collateral blocking and aligns corrective action with the failing scope.
- **Deprecate-and-ignore vs remove config key**: Compatibility is preserved for existing YAML while behavior shifts to SPC-driven controls.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Convergence scoring/detection; trend maintenance streak logic; spec SPC blocker policy decisions; deprecated-config behavior invariants.
2. **Integration Tests**: Orchestrator convergence stop behavior; Gate blocking with spec anomalies; status and retro warning propagation.
3. **Manual/Smoke Validation**: Targeted package tests and command-level verification with synthetic trend fixtures.

**Key Test Cases:**
- Deadlock detection: near-zero deltas across 3 cycles triggers `not_ready_criteria` stop.
- Oscillation detection: repeating/high-variance deltas triggers `not_ready_scope` stop.
- Healthy convergence path: no premature stop when deltas trend toward resolution.
- Spec block policy: `spec:foo` bead blocks only when foo has high-severity rework anomaly.
- Maintenance warning policy: package flagged only after 3 consecutive validation-time UCL breaches.
- Config compatibility: `max_tdd_cycles` still parses but does not terminate TDD loops.

**Mocking Strategy:**
- Use synthetic iteration metrics and in-memory trend fixtures for logger/policy tests.
- Reuse gate and orchestrator test fakes to isolate decision logic from provider execution.
- Avoid mocking SPC math primitives; test real calculations with deterministic input sets.

**Coverage Goals:**
- Critical: convergence stop path, spec-level gate blocking, warning propagation in status/retro.
- Edge: missing trend file fallback, unknown spec labels, zero-stddev anomaly paths, clamped rate metrics.

**Test Organization:**
- Keep tests adjacent to modules (`*_test.go` in-package).
- Add focused new test files for net-new components (`convergence_test.go`, `spec_spc_blocker_test.go`).
- Extend existing tests in `orchestrator_test.go`, `gate_test.go`, `process_trend_test.go`, `display_test.go`, `retro_test.go`.

## Implementation Tasks

### Task 1: Add TDD Convergence Model and Detector

**Files:**
- Create: `internal/runner/tdd/convergence.go`
- Create: `internal/runner/tdd/convergence_test.go`
- Modify: `internal/runner/tdd/handoff.go`

**What to Do:**
Define a convergence data model for a 3-cycle window and implement scoring utilities that produce a normalized complexity delta and classify instability as deadlock or oscillation. Update `CycleState` completion semantics to remove `MaxCycles` as a terminating condition.

**Acceptance Criteria:**
- A convergence evaluator exists that consumes at least 3 cycle snapshots and returns: stable/deadlock/oscillation.
- Deadlock and oscillation thresholds are deterministic and covered by unit tests.
- `CycleState.IsComplete()` no longer depends on `MaxCycles`.

**Dependencies:**
- None.

**Notes:**
Use explicit structs/enums so downstream orchestrator and logger code can share the same classification vocabulary.

### Task 2: Integrate Convergence Stop Logic into TDD Orchestrator

**Files:**
- Modify: `internal/runner/tdd/orchestrator.go`
- Modify: `internal/runner/callbacks_tdd.go`
- Modify: `internal/runner/tdd/orchestrator_test.go`

**What to Do:**
Wire convergence tracking into each red/green/refactor cycle, maintain a rolling 3-cycle window, and terminate the loop when deadlock/oscillation is detected. Map instability to readiness-style reasons (`not_ready_scope` or `not_ready_criteria`) and ensure result metadata is logged for downstream Andon handling.

**Acceptance Criteria:**
- Orchestrator exits on deadlock/oscillation even when criteria remain.
- Stop reason is deterministic and propagated in result/log fields.
- Existing tests are updated to reflect removal of MaxCycles termination behavior.

**Dependencies:**
- Task 1.

**Notes:**
Keep normal success path unchanged; only inject early-stop behavior on instability.

### Task 3: Extend Process Trend Data for Convergence and Maintenance Signals

**Files:**
- Modify: `internal/logger/process_trend.go`
- Modify: `internal/logger/trend_builder.go`
- Modify: `internal/logger/trend_spc.go`

**What to Do:**
Add process trend schema support for convergence score summaries and package-level validation-time UCL breach streaks. Compute “High Maintenance Cost” markers when a package exceeds validation-time UCL for 3 consecutive iterations.

**Acceptance Criteria:**
- `ProcessTrend` includes convergence and package-maintenance fields with nil-safe normalization.
- Trend building computes maintenance flags only after 3 consecutive breaches.
- SPC/anomaly severity logic remains deterministic and sorted in outputs.

**Dependencies:**
- Task 2 (for convergence telemetry availability).

**Notes:**
Preserve backward compatibility when older trend files do not include new fields.

### Task 4: Enforce Spec-Level Rework Anomaly Blocking in Gate

**Files:**
- Create: `internal/pipeline/prepare/spec_spc_blocker.go`
- Create: `internal/pipeline/prepare/spec_spc_blocker_test.go`
- Modify: `internal/pipeline/prepare/gate.go`

**What to Do:**
Implement an evaluator that checks whether the bead’s spec is currently in a high-severity rework anomaly state and integrate it into gate decisions before proceeding to build.

**Acceptance Criteria:**
- Gate blocks beads with `spec:*` labels when matching spec anomaly is high severity.
- Block reason is explicit and suitable for operator action (`refine`/`plan` review).
- Non-matching specs and missing trend data fail open (no accidental global block).

**Dependencies:**
- Task 3.

**Notes:**
Keep the evaluator isolated for straightforward unit testing and future policy tuning.

### Task 5: Surface High Maintenance Cost Warnings in Status and Retro

**Files:**
- Modify: `internal/runner/display/display.go`
- Modify: `internal/runner/display/display_test.go`
- Modify: `internal/retro/retro.go`

**What to Do:**
Render package maintenance warnings in `gromit status` SPC output and include the same warning context in retro prompt/template data so retros explicitly call out test-debt hotspots.

**Acceptance Criteria:**
- Status output includes a clear “High Maintenance Cost” section when flagged packages exist.
- Retro context includes flagged package warnings for analysis generation.
- No warning section is shown when no packages are flagged.

**Dependencies:**
- Task 3.

**Notes:**
Prefer concise warning formatting to keep status readable while still actionable.

### Task 6: Deprecate `max_tdd_cycles` Runtime Behavior and Finalize Coverage

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config_test.go`

**What to Do:**
Mark `max_tdd_cycles` as deprecated compatibility input and ensure runtime behavior ignores it for termination decisions. Update config and behavior tests to lock in the new contract.

**Acceptance Criteria:**
- Config parsing still accepts `max_tdd_cycles` without breaking existing YAML.
- TDD termination no longer depends on `max_tdd_cycles`.
- Tests explicitly document deprecated-but-ignored behavior.

**Dependencies:**
- Task 1 and Task 2.

**Notes:**
If desired later, a follow-up can remove the field entirely after a migration window.

### Task 7: End-to-End Verification Across Affected Subsystems

**Files:**
- Modify: `internal/pipeline/prepare/gate_test.go`
- Modify: `internal/logger/process_trend_test.go`
- Modify: `internal/retro/retro_test.go`

**What to Do:**
Add/adjust cross-cutting tests to ensure convergence stop, spec blocking, and maintenance warnings behave coherently from telemetry generation through user-facing outputs.

**Acceptance Criteria:**
- Regression tests demonstrate acceptance criteria coverage across logger, gate, TDD, status, and retro.
- No existing SPC summary/test contracts regress.
- Targeted package test suite passes for touched areas.

**Dependencies:**
- Tasks 2, 3, 4, 5, and 6.

**Notes:**
Keep this task focused on integration consistency, not net-new feature logic.

---

## Notes

- This plan intentionally scopes behavior changes to SPC-informed controls while reusing existing event and escalation infrastructure.
- The spec references an `AndonEvent`; current codebase uses typed gate/runner events rather than a single generic event, so implementation should map instability to existing event taxonomy unless a new canonical event type is introduced in a follow-up.
- Decomposition strategy should remain conservative: trigger proactively on tier-relative duration/token anomaly, but avoid recursive decomposition loops.
