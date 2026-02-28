---
id: mandatory-prebuild-readiness-gate
source_spec: mandatory-prebuild-readiness-gate
created: 2026-02-28
decomposed: false
---

# Mandatory Pre-Build Readiness Gate Implementation Plan

**Goal:** Enforce a mandatory, deterministic pre-build readiness gate for every bead run, with an explicit emergency override that is visible and auditable.

**Architecture:** Stage 1 Gate always runs deterministic readiness checks before Build, emits existing readiness reason codes on block, and supports a per-run emergency bypass that preserves clear telemetry/log signaling.

**Tech Stack:** Go, existing Gromit pipeline stages (`prepare`, `execute`, `runner`), config loading/defaults, event emission, unit/integration/acceptance tests.

**Spec:** `.gromit/specs/mandatory-prebuild-readiness-gate.md`

---

## Architecture

**Overview:**  
Make readiness gating always wired in Stage 1 for all beads, backed by deterministic criteria/scope checks only. Keep gate ordering as precheck -> readiness -> stuck/data-quality/scope. Add an explicit emergency override path that bypasses readiness blocking while clearly signaling override use in logs/status/telemetry.

**Key Components:**
1. **Deterministic readiness assessor (`prepare`/`runner`)**: Implement `internal/readiness.Assessor` using existing deterministic checks in `internal/pipeline/prepare/readiness.go`.
2. **Gate decision logic (`internal/pipeline/prepare/gate.go`)**: Retain readiness blocking semantics and reason propagation; add explicit override observability when readiness blocking is bypassed.
3. **Runner stage wiring (`internal/runner/constructor.go`)**: Wire readiness assessor unconditionally (mandatory-on), no longer gated by `readiness_check.enabled`.
4. **Emergency override configuration/runtime control (`internal/config` + run plumbing)**: Add explicit override field/flag for run-time bypass.
5. **Telemetry differentiation (`events`/status/tests)**: Preserve reason-code granularity and ensure readiness blocks remain distinguishable from other gate or pipeline outcomes.

**Integration Points:**
- `internal/runner/constructor.go`: replace conditional readiness wiring with mandatory deterministic wiring.
- `internal/runner/constructor_adapters.go` (or a new focused file): provide deterministic adapter implementation and remove LLM-dependent behavior from mandatory path.
- `internal/pipeline/prepare/gate.go`: preserve `GateReadinessBlockEvent` for true blocks and emit explicit signal when override bypass is used.
- `internal/config/*`: defaults and accessors for override path.

**Data Flow:**
1. Precheck runs first; `precheck_passed` still skips work.
2. Readiness deterministic checks run for every bead.
3. Not ready + no override => gate `Block` with reason (`criteria_missing`, `criteria_ambiguous`, `scope_too_broad`).
4. Not ready + override => proceed with explicit override log/status signal.
5. Ready => existing Build/Validate/Review/Epilogue flow unchanged.

**Files to Modify:**
- `internal/runner/constructor.go` - mandatory readiness wiring and override plumbing.
- `internal/runner/constructor_adapters.go` - deterministic readiness adapter (or extraction to new file).
- `internal/pipeline/prepare/gate.go` - override visibility behavior and readiness decision handling.
- `internal/config/config_types.go` - emergency override config field(s).
- `internal/config/config_defaults.go` - default mandatory-on behavior with override default off.
- `internal/config/config_accessors.go` - override accessor.
- `internal/runner/cmd_run.go` - explicit run-level override option wiring.
- Related tests in `internal/pipeline/prepare`, `internal/runner`, `internal/config`, and acceptance suites.

**Files to Create:**
- `internal/runner/readiness_deterministic_assessor.go` (if separating concerns from constructor adapters improves maintainability).

**Tradeoffs:**
- **Deterministic-first enforcement:** chosen over LLM judgment because current readiness LLM path is stub/fail-closed and not production-reliable for mandatory gating.
- **Mandatory default:** chosen for prevention efficacy; operational escape hatch remains available for incident response.
- **Visible override:** chosen over silent bypass to support auditability and prevent accidental long-term disablement.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** deterministic readiness checks and reason mapping.
2. **Integration Tests:** gate/build orchestration behavior and override plumbing.
3. **Acceptance Tests:** queue-wide applicability (unlabeled + `spec:*`) and telemetry distinction.

**Key Test Cases:**
- Bead with no acceptance criteria is blocked before Build with `criteria_missing`.
- Bead exceeding criteria count limit is blocked with `criteria_ambiguous`.
- Bead exceeding expected outputs scope limit is blocked with `scope_too_broad` (or decomposition path), and Build is not invoked.
- Ready bead proceeds through Build/Validate/Review/Epilogue unchanged.
- Readiness gating is active by default for unlabeled and `spec:*` beads.
- Explicit emergency override bypasses readiness blocking for that run and emits clear override signal.
- Telemetry clearly distinguishes readiness block outcomes from precheck skips, stuck blocks, and build/validation failures.

**Mocking Strategy:**
- Use stub Build stage to assert build invocation suppression on readiness block.
- Use real deterministic readiness evaluation in unit tests to prevent brittle mock-only assertions.
- Use event sink/status assertions to validate reason codes and override visibility.

**Coverage Goals:**
- Constructor mandatory wiring path.
- Gate ordering and precedence.
- Readiness reason-code propagation.
- Emergency override observability and limited blast radius.

**Test Organization:**
- `internal/pipeline/prepare/gate_test.go`
- `internal/pipeline/prepare/readiness_test.go`
- `internal/runner/constructor_test.go`
- `internal/runner/acceptance/*` readiness-focused acceptance tests
- `internal/config/config_test.go`

## Implementation Tasks

### Task 1: Implement deterministic readiness assessor for mandatory gate

**Files:**
- Modify: `internal/runner/constructor_adapters.go`
- Optional create: `internal/runner/readiness_deterministic_assessor.go`
- Test: `internal/runner/constructor_adapters_test.go`

**What to Do:**
Replace the mandatory path’s current LLM-coupled/fail-closed readiness behavior with a deterministic assessor that maps structured outcomes to existing readiness reason codes and `readiness.Status*` values.

**Acceptance Criteria:**
- Deterministic assessor returns `StatusNotReady` + `criteria_missing` when criteria are absent.
- Deterministic assessor returns `StatusNotReady` + `criteria_ambiguous` when criteria exceed limit.
- Deterministic assessor returns `StatusNotReady` + `scope_too_broad` when expected outputs exceed scope bound.

**Dependencies:**
- None

**Notes:**
Keep any future prompt-based readiness logic out of mandatory enforcement path.

### Task 2: Make readiness wiring mandatory in runner constructor

**Files:**
- Modify: `internal/runner/constructor.go`
- Test: `internal/runner/constructor_test.go`

**What to Do:**
Always wire a readiness assessor into Gate for every run, independent of `readiness_check.enabled`. Ensure no label-based exclusions.

**Acceptance Criteria:**
- Gate has readiness assessor wired even when `readiness_check.enabled` is unset/false.
- Existing non-readiness stages continue to initialize unchanged.
- Constructor tests cover default mandatory readiness wiring.

**Dependencies:**
- Task 1

**Notes:**
Preserve stage initialization order and existing adapters.

### Task 3: Add explicit emergency override to bypass readiness blocking

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config_accessors.go`
- Modify: `internal/runner/cmd_run.go`
- Modify: `internal/pipeline/prepare/gate.go`
- Test: `internal/config/config_test.go`
- Test: `internal/pipeline/prepare/gate_test.go`

**What to Do:**
Add a run-scoped emergency override that allows readiness failures to proceed for incident response. Ensure default remains mandatory-on and bypass usage is explicit in logs/status/events.

**Acceptance Criteria:**
- Override defaults to disabled.
- Enabling override for a run bypasses readiness block only (does not disable other gate checks).
- Run output/status clearly indicates override was used when a not-ready bead proceeds.

**Dependencies:**
- Task 2

**Notes:**
Keep naming explicit (e.g., `readiness_emergency_override`) to avoid accidental use.

### Task 4: Preserve readiness reason-code telemetry and decision distinctions

**Files:**
- Modify: `internal/pipeline/prepare/gate.go`
- Modify: `internal/logger/trend_builder.go` (if needed for explicit override marker handling)
- Test: `internal/pipeline/prepare/gate_test.go`
- Test: `internal/logger/process_trend_test.go` (if behavior changes)

**What to Do:**
Ensure readiness block decisions continue to emit existing reason codes and remain analytically distinct from precheck skips, stuck blocks, data-quality blocks, and downstream failures. Add/adjust event or metadata handling for override observability as needed.

**Acceptance Criteria:**
- Readiness block emits `GateReadinessBlockEvent` with one of the existing reason codes.
- Precheck and stuck block paths remain behaviorally and telemetry-distinct.
- Override usage is traceable without misclassifying as readiness block.

**Dependencies:**
- Task 3

**Notes:**
Prefer minimal schema churn; extend existing event/status structures only where necessary.

### Task 5: Add acceptance/integration coverage for queue-wide mandatory readiness

**Files:**
- Modify: `internal/runner/acceptance/constructor_no_spec_gate_wiring_test.go` (or appropriate acceptance file)
- Modify: `internal/runner/constructor_test.go`
- Modify: `internal/pipeline/prepare/readiness_test.go`

**What to Do:**
Add end-to-end oriented tests proving default behavior for unlabeled and `spec:*` beads, build suppression on readiness block, and unchanged flow for ready beads.

**Acceptance Criteria:**
- Tests cover unlabeled bead readiness blocking behavior.
- Tests cover `spec:*` labeled bead readiness blocking behavior.
- Tests verify ready beads still run through Build/Validate/Review/Epilogue path.

**Dependencies:**
- Task 4

**Notes:**
Reuse existing acceptance harness to avoid introducing new fixture machinery.

### Task 6: Regression hardening and docs-level config coverage

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `README.md` and/or `docs/QUICKSTART.md` (if readiness override is operator-facing)

**What to Do:**
Add regression tests for default-on readiness semantics and override parsing; document temporary emergency override usage and visibility expectations if externally configurable.

**Acceptance Criteria:**
- Config tests assert mandatory behavior defaults and override default-off.
- Documentation (if applicable) describes override as emergency-only and explicit.
- No existing readiness/precheck tests regress.

**Dependencies:**
- Task 5

**Notes:**
Keep docs concise and consistent with “temporary escape hatch” intent.

---

## Notes

- Existing `internal/pipeline/prepare/readiness.go` already defines the required deterministic rules and reason constants; implementation should maximize reuse to avoid drift.
- The current `readinessAdapterWithLLM` in `internal/runner/constructor_adapters.go` is unsuitable as the mandatory path because it is tied to incomplete LLM invocation behavior.
- Override observability is a hard requirement; silent bypass should be treated as a bug.
