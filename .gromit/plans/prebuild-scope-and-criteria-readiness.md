---
id: prebuild-scope-and-criteria-readiness
source_spec: prebuild-scope-and-criteria-readiness
created: 2026-02-28
decomposed: false
---

# Pre-Build Scope and Criteria Readiness Implementation Plan

**Goal:** Block beads that are unclear or oversized before build so failed-build spend is reduced.

**Architecture:** Extend Stage 1 Gate with a readiness assessor that runs after precheck-not-done, combines structured and prompt-based checks, and emits explicit readiness block reasons that flow into iteration telemetry.

**Tech Stack:** Go, existing pipeline stage interfaces, provider router invocation, prompt templates, JSONL iteration logs, process trend metrics.

**Spec:** `.gromit/specs/prebuild-scope-and-criteria-readiness.md`

---

## Architecture

**Overview:**
Add a first-class readiness assessment inside `Gate` immediately after the existing precheck "already done" path. The gate returns `Proceed` for `ready`, and `Block` with explicit readiness reason codes for criteria/scope failures, while preserving current build/validate/review flow for ready beads.

**Key Components:**
1. **`prepare.ReadinessAssessor` + readiness outcome model**: New gate dependency and outcome enum (`ready`, `not_ready_scope`, `not_ready_criteria`) with reason codes.
2. **Structured readiness checks in `prepare`**: Local checks for missing/empty criteria, criteria count bounds (1-3), and expected outputs single-concern sanity.
3. **Prompt-based readiness adapter in `runner`**: Router-backed lightweight classifier for ambiguity/overlap/scope breadth; fail closed on uncertainty.
4. **Gate telemetry/reason propagation**: Emit `GateBlockEvent` reasons like `criteria_missing`, `criteria_ambiguous`, `scope_too_broad`; keep `precheck_passed` unchanged.
5. **Iteration metrics plumbing**: Carry gate block reason into iteration logs so readiness blocks are separable from build failures.
6. **Config surface for readiness**: Add `readiness_check` config (enabled/model/timeout/fail_closed) with defaults and normalization.

**Integration Points:**
- Extend Stage 1 gate decision logic in `internal/pipeline/prepare/gate.go`.
- Wire readiness assessor in `internal/runner/constructor.go`.
- Add adapter/prompt invocation logic in `internal/runner/constructor_adapters.go`.
- Extend config types/defaults/tests in `internal/config/*`.
- Extend logging/metrics aggregation in `internal/logger/*` and gate-path orchestration logging in `internal/runner/orchestrator.go`.

**Data Flow:**
1. Gate precheck runs (`precheck_passed` still short-circuits to `Skip`).
2. If not precheck-passed, readiness assessor runs:
   - Structured checks run first.
   - If structured passes, prompt-based classifier runs.
3. Outcome mapping:
   - `ready` -> existing pipeline unchanged.
   - `not_ready_criteria` -> `Block` + criteria reason code.
   - `not_ready_scope` -> `Block` + scope reason code.
4. Orchestrator logs gate block reason in iteration log.
5. Trend builder aggregates readiness block counts/rates separately from build failure buckets.

**Files to Modify (high-level):**
- `internal/pipeline/prepare/gate.go`
- `internal/pipeline/prepare/gate_test.go`
- `internal/pipeline/stage.go`
- `internal/runner/constructor.go`
- `internal/runner/constructor_adapters.go`
- `internal/runner/orchestrator.go`
- `internal/config/config_types.go`
- `internal/config/config_defaults.go`
- `internal/config/config_test.go`
- `internal/logger/logger.go`
- `internal/logger/trend_builder.go`
- `internal/logger/process_trend.go`
- `internal/events/types_gate.go`

**Files to Create (expected):**
- `internal/pipeline/prepare/readiness.go`
- `internal/pipeline/prepare/readiness_test.go`

**Tradeoffs:**
- **Explicit readiness assessor vs reusing `DataQualityBlocker`**: explicit assessor chosen for clear outcome semantics and targeted tests.
- **Reason metadata in `pipeline.Output` vs event-only derivation**: output metadata chosen for deterministic orchestrator logging.
- **Conservative fail-closed behavior**: uncertainty blocks are cheaper than wasted full build cycles.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Gate readiness decision logic, outcome mapping, precedence, reason propagation.
2. **Integration Tests**: Orchestrator gate-block logging path and trend aggregation including readiness metrics.
3. **Manual/Smoke Validation**: Scoped `go test` execution and verification of readiness fields in generated logs/trend snapshots.

**Key Test Cases:**
- Readiness runs only when bead is not precheck-skipped.
- Missing/empty acceptance criteria returns `Block` with `criteria_missing`.
- Criteria count outside 1-3 returns `Block` with criteria-quality reason.
- Prompt-based ambiguous criteria returns `Block` with `criteria_ambiguous`.
- Prompt-based broad scope returns `Block` with `scope_too_broad`.
- Uncertain/parse-failure path fails closed to readiness block reason.
- `ready` outcome preserves existing `Proceed` behavior.
- Existing stuck/scope gates still work with expected precedence.
- Gate block reason is recorded in iteration log on gate-block path.
- Process trend reports readiness-block count/rate separately from build/validation/timeout rates.

**Mocking Strategy:**
- Mock readiness assessor in gate tests for sequence/precedence coverage.
- Mock router/provider in runner adapter tests for classifier output parsing.
- Use real logger/trend builder for aggregation tests from synthetic `IterationLog` entries.

**Coverage Goals:**
- Critical path: precheck -> readiness -> downstream gate decisions.
- Critical telemetry path: gate reason -> iteration log -> trend metrics.
- Edge cases: nil bead/config, disabled readiness, assessor timeout/error, malformed classifier output.

**Test Organization:**
- Gate logic tests in `internal/pipeline/prepare/`.
- Adapter/wiring tests in `internal/runner/`.
- Metrics aggregation tests in `internal/logger/`.
- Table-driven cases with exact reason-code assertions.

## Implementation Tasks

### Task 1: Add Readiness Config Surface

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add a `readiness_check` config section with fields for `enabled`, `model`, `timeout_seconds`, and `fail_closed`. Wire defaults and normalization consistent with existing `precheck`/`scope_check` patterns.

**Acceptance Criteria:**
- Config struct includes a typed readiness section accessible through `config.Config`.
- Defaults are applied when readiness config is omitted.
- YAML parsing tests cover explicit values, omitted values, and invalid value handling.

**Dependencies:**
- None

### Task 2: Implement Readiness Domain Model and Structured Checks

**Files:**
- Create: `internal/pipeline/prepare/readiness.go`
- Test: `internal/pipeline/prepare/readiness_test.go`

**What to Do:**
Define readiness outcomes, reason codes, and structured validation helpers for criteria presence/count and expected outputs scope sanity. Include deterministic conservative behavior for uncertain structured checks.

**Acceptance Criteria:**
- Readiness outcome and reason-code types are defined and unit tested.
- Structured checks return `ready`, `not_ready_criteria`, or `not_ready_scope` with explicit reasons.
- Structured checks enforce criteria count guidance (1-3) and non-empty criteria requirements.

**Dependencies:**
- Task 1

### Task 3: Add Prompt-Based Readiness Assessment Adapter

**Files:**
- Modify: `internal/runner/constructor_adapters.go`
- Modify: `internal/prompt/context_types.go`
- Modify: `internal/prompt/render_methods.go` (and related prompt tests)

**What to Do:**
Implement a runner-side readiness assessor that performs structured checks first, then invokes an LLM prompt-based classifier when needed. Parse classifier output into readiness outcomes and reason codes; fail closed when response is invalid/uncertain.

**Acceptance Criteria:**
- Readiness adapter implements the gate readiness interface and can be wired from constructor.
- Prompt-based classifier supports the three outcomes and reason-code parsing.
- Invalid or uncertain classifier output resolves to a readiness block, not `ready`.

**Dependencies:**
- Task 2

### Task 4: Integrate Readiness Into Gate Decision Flow

**Files:**
- Modify: `internal/pipeline/prepare/gate.go`
- Modify: `internal/pipeline/prepare/gate_test.go`
- Modify: `internal/events/types_gate.go`

**What to Do:**
Extend gate sequencing so readiness executes after precheck-not-done and before stuck/scope flow (or in explicitly chosen final order), producing `Block` with readiness reason codes and emitting typed gate events.

**Acceptance Criteria:**
- Gate invokes readiness check for non-precheck-passed beads.
- `not_ready_criteria` and `not_ready_scope` both block before build.
- Gate event emissions include distinct readiness reason codes for block decisions.

**Dependencies:**
- Task 2
- Task 3

### Task 5: Propagate Gate Reason Metadata Through Pipeline and Orchestrator

**Files:**
- Modify: `internal/pipeline/stage.go`
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Add gate-reason metadata to stage output and ensure orchestrator writes this reason into failure-path iteration logs for gate-blocked beads.

**Acceptance Criteria:**
- `pipeline.Output` can carry gate block reason metadata.
- Orchestrator includes gate reason in iteration log entries for blocked gate outcomes.
- Existing skip/proceed behavior and other failure-path logging remain backward compatible.

**Dependencies:**
- Task 4

### Task 6: Extend Iteration Log Schema for Readiness Block Attribution

**Files:**
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/logging.go`
- Test: `internal/logger/iteration_log_test.go`

**What to Do:**
Introduce iteration-log fields for gate decision attribution (especially readiness block reasons) and ensure serialization omits empty values.

**Acceptance Criteria:**
- Iteration logs encode readiness gate reason fields when present.
- Empty/absent reason fields are omitted from JSON output.
- Existing log readers remain compatible with added optional fields.

**Dependencies:**
- Task 5

### Task 7: Add Readiness Metrics to Process Trend Aggregation

**Files:**
- Modify: `internal/logger/trend_builder.go`
- Modify: `internal/logger/process_trend.go`
- Test: `internal/logger/process_trend_test.go` (or nearest trend tests)

**What to Do:**
Compute rolling readiness block counts/rates from iteration logs and expose them in trend window output so readiness prevention impact is measurable separately from build failures.

**Acceptance Criteria:**
- Process trend includes readiness-block count/rate metrics.
- Readiness blocks are not conflated with build/validation/timeout failure rates.
- Aggregation tests verify mixed windows with readiness blocks and normal failures.

**Dependencies:**
- Task 6

### Task 8: Wire Constructor and Verify End-to-End Gate Behavior

**Files:**
- Modify: `internal/runner/constructor.go`
- Test: `internal/runner/constructor_test.go`
- Test: `internal/pipeline/prepare/gate_test.go` (integration-style gate path updates)

**What to Do:**
Wire readiness assessor into gate construction with config-controlled enablement, then add end-to-end wiring tests that confirm readiness blocks prevent build invocation and emit expected telemetry.

**Acceptance Criteria:**
- Constructor wires readiness assessor when configured.
- Integration test demonstrates readiness-blocked bead does not enter build stage.
- Telemetry assertions verify reason code visibility in gate/orchestrator outputs.

**Dependencies:**
- Task 1
- Task 3
- Task 4
- Task 5
- Task 7

---

## Notes

- Keep reason codes stable and machine-friendly; treat them as contract strings for dashboards and regression tests.
- Prefer adding optional fields over changing existing required log schema to preserve backward compatibility for older tooling.
- Decomposition-as-remedy for scope blocks can remain future-configurable; this plan implements conservative pre-build blocking first.
