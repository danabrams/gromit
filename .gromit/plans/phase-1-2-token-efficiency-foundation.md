---
id: phase-1-2-token-efficiency-foundation
source_spec: phase-1-2-token-efficiency-foundation
created: 2026-02-24
decomposed: false
---

# Token Efficiency Foundation (Phase 1-2) Implementation Plan

**Goal:** Establish trustworthy token/cost/latency baselines and deliver low-risk, reversible token reductions without quality regression.

**Architecture:** Extend existing runner/prompt telemetry with deterministic per-run summaries, deterministic tool-output pruning, and differential context delivery with safe full-snapshot fallback; gate each optimization behind independent config flags.

**Tech Stack:** Go, existing gromit runner pipeline (`internal/pipeline`), prompt renderer (`internal/prompt`), logger/JSONL artifacts (`internal/logger`), YAML config (`internal/config`).

**Spec:** `.gromit/specs/phase-1-2-token-efficiency-foundation.md`

---

## Architecture

**Overview:**
Add a token-efficiency layer around existing runner telemetry and prompt assembly: (1) deterministic per-run summaries, (2) configurable tool-output pruning, and (3) differential context payload delivery with safe full-snapshot fallback.

**Key Components:**
1. **Run Summary Aggregator (`internal/logger/run_summary.go`)**: Builds deterministic run artifact from current run JSONL + TDD phase records, including per-phase tokens/cost/latency/retries and prompt source-bucket attribution.
2. **Prompt/Invocation Telemetry Wiring (`internal/runner` + `internal/pipeline`)**: Captures per-stage/per-phase timings and whether pruning/delta modes were used, then persists these markers into iteration logs.
3. **Tool Output Pruner (`internal/prompt/tool_output_prune.go`)**: Deterministic pruning utility (error sections + pass/fail counts + optional tail) with explicit `[...pruned...]` marker and configurable limits.
4. **Context Delta Tracker (`internal/prompt/context_delta.go`)**: Tracks previous editor/context snapshot per run; emits compact delta when safe, or full snapshot with baseline reset when delta application is unsafe.
5. **Config Flags (`internal/config/config_types.go`, defaults/normalize/accessors)**: Independent flags for `run_summary`, `tool_output_pruning`, and `context_delta`, all togglable separately.

**Integration Points:**
- Wire summary artifact generation at orchestrator run completion in `internal/runner/orchestrator.go`.
- Apply tool-output pruner where high-volume outputs are currently attached to prompt context/failure context (validate + failure retry paths).
- Apply context delta transform during prompt context build/render path in `internal/prompt/renderer_context.go` and adapters in `internal/runner/constructor_adapters.go`.

**Data Flow:**
- Invocation/stage runs emit normal JSONL records.
- Telemetry enricher annotates prompt diagnostics with bucketed sizes and mode flags.
- On run completion, summary aggregator reads only the current run records and writes one stable summary JSON (and optional markdown companion) for baseline comparison.
- For each prompt render, context delta tracker chooses `delta` or `full`; on checksum/version mismatch it auto-resyncs with full snapshot.

**Files to Modify:**
- `internal/runner/orchestrator.go` - Trigger per-run summary writing and thread summary dependencies.
- `internal/logger/logger.go` - Extend iteration log schema with operability/mode fields and any required phase telemetry pointers.
- `internal/prompt/diagnostics.go` - Add prompt-size bucket attribution and mode metadata.
- `internal/prompt/render_methods.go` - Populate expanded diagnostics fields consistently by prompt type.
- `internal/prompt/renderer_context.go` - Apply differential context flow hooks during context assembly.
- `internal/runner/validation/runner.go` - Route failure output through deterministic pruner when enabled.
- `internal/config/config_types.go` - Add token efficiency config blocks and prune limits.
- `internal/config/config_defaults.go` - Set default-on flags and sane limits.
- `internal/config/config_accessors.go` - Accessors for enable/disable behavior and limits.

**Files to Create:**
- `internal/logger/run_summary.go` - Deterministic per-run summary builder/writer.
- `internal/logger/run_summary_test.go` - Determinism and aggregation coverage.
- `internal/prompt/tool_output_prune.go` - Deterministic high-volume output pruning utilities.
- `internal/prompt/tool_output_prune_test.go` - Error preservation/count extraction/markers/limits tests.
- `internal/prompt/context_delta.go` - Delta/full snapshot selection and baseline reset logic.
- `internal/prompt/context_delta_test.go` - Delta generation, mismatch fallback, and reset tests.

**Tradeoffs:**
- **Deterministic pruning over model summarization:** chosen for repeatability, debuggability, and easy rollback.
- **Additive schema extension over schema replacement:** chosen to avoid breaking existing log consumers.
- **Run-end aggregation over hot-path heavy aggregation:** chosen to minimize runtime overhead while keeping observability complete.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: run-summary determinism, pruning behavior/limits/markers, delta generation+fallback, and config default/accessor behavior.
2. **Integration Tests**: orchestrator summary artifact emission, iteration log operability fields, prompt diagnostics bucket attribution/reconciliation.
3. **Manual Benchmark Validation**: fixed-workload baseline (>=3 runs) vs optimized (>=3 runs) median comparisons.

**Key Test Cases:**
- Summary contains per-phase tokens/cost/latency and retry/validation outcomes.
- Summary output is deterministic for identical input logs.
- Pruner preserves failure details and compact pass/fail counts while truncating bulk output.
- Prune marker always appears when pruning occurs.
- Unchanged context does not resend full editor/context payload.
- Delta apply failure path emits full snapshot and resets baseline safely.
- Independent feature flags can disable each optimization without side effects.

**Mocking Strategy:**
- JSONL fixture logs for summary tests.
- String fixtures for stdout/stderr pruning.
- In-memory baseline state for context-delta behavior.
- Real config parser/defaults path for feature-flag behavior.

**Coverage Goals:**
- Critical paths: summary generation, prune decision branches, delta fallback/resync, operability flag emission.
- Edge cases: empty logs, malformed records, large output payloads, UTF-8-safe truncation boundaries, disabled-feature behavior.

**Test Organization:**
- `internal/logger/run_summary_test.go`
- `internal/prompt/tool_output_prune_test.go`
- `internal/prompt/context_delta_test.go`
- Targeted updates to:
  - `internal/runner/orchestrator_test.go`
  - `internal/logger/logger_test.go`
  - `internal/prompt/diagnostics_test.go`
  - `internal/config/*token|prompt*_test.go` as needed for new config branches.

---

## Implementation Tasks

### Task 1: Add Config Surface for Token Efficiency Controls

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config_accessors.go`
- Test: `internal/config/config_test.go` (or focused config tests)

**What to Do:**
Introduce config sections for:
- run summary emission (enabled/path behavior)
- tool output pruning (enabled + limits)
- context delta mode (enabled + safety toggles)
Provide defaults that match rollout intent (instrumentation first, pruning/delta independently switchable).

**Acceptance Criteria:**
- New config keys parse from YAML and are available via accessors.
- Defaults are deterministic and backward compatible for existing configs.
- Each optimization can be enabled/disabled independently.

**Dependencies:**
- None

**Notes:**
Keep naming consistent with existing `prompt.budget`/`stream` patterns and avoid ambiguous boolean semantics.

### Task 2: Implement Deterministic Tool Output Pruning Utilities

**Files:**
- Create: `internal/prompt/tool_output_prune.go`
- Create: `internal/prompt/tool_output_prune_test.go`

**What to Do:**
Implement deterministic pruning transforms for high-volume tool outputs:
- extract/retain failure/error sections
- compact pass/fail counts
- optional tail excerpt within limit
- append explicit prune marker when content omitted

**Acceptance Criteria:**
- Same input always yields same pruned output.
- Error/failure signal required for debugging is preserved.
- Marker is present whenever pruning occurs.

**Dependencies:**
- Task 1 (config limits/types)

**Notes:**
No model-based summarization; transforms must be pure and testable.

### Task 3: Wire Pruning into Validation/Failure Context Injection

**Files:**
- Modify: `internal/runner/validation/runner.go`
- Modify: `internal/pipeline/validate/validate.go`
- Test: `internal/runner/validation/validation_test.go`
- Test: `internal/pipeline/validate/validate_test.go`

**What to Do:**
Apply pruning before high-volume validation output is injected into LLM-facing context/history while keeping full raw output in logs/artifacts.
Record whether pruning was used for observability.

**Acceptance Criteria:**
- Long outputs are no longer fully embedded by default when pruning is enabled.
- Failure details remain present in injected context.
- Disabled flag restores previous non-pruned behavior.

**Dependencies:**
- Task 2

**Notes:**
Respect existing truncation behavior and avoid double-pruning artifacts.

### Task 4: Extend Prompt Diagnostics with Source Buckets and Reconciliation Metadata

**Files:**
- Modify: `internal/prompt/diagnostics.go`
- Modify: `internal/prompt/render_methods.go`
- Modify: `internal/prompt/prompt.go`
- Test: `internal/prompt/diagnostics_test.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Extend prompt diagnostics to explicitly report prompt-size contributions by source bucket where available (template, rules, learnings, tool output, editor/context delta/full), and ensure reconciliation fields can be populated consistently.

**Acceptance Criteria:**
- Diagnostics include bucket attribution for relevant prompt types.
- Diagnostics remain backward-compatible in JSON shape for existing consumers.
- Reconciliation fields are stable and tested.

**Dependencies:**
- Task 1

**Notes:**
Prefer additive fields and stable key names.

### Task 5: Implement Differential Context Update Engine with Safe Fallback

**Files:**
- Create: `internal/prompt/context_delta.go`
- Create: `internal/prompt/context_delta_test.go`
- Modify: `internal/prompt/renderer_context.go`
- Test: `internal/prompt/renderer_context_test.go`

**What to Do:**
Add context delta tracking for editor/context payloads:
- compute deterministic delta/summaries against prior baseline
- emit mode metadata (`delta` vs `full`)
- when delta cannot be safely applied, emit full snapshot and reset baseline

**Acceptance Criteria:**
- Unchanged state does not resend full snapshot in delta mode.
- Delta failure path automatically recovers with full snapshot baseline reset.
- Behavior can be disabled via config.

**Dependencies:**
- Task 1
- Task 4

**Notes:**
Keep baseline state scoped to run/session; avoid cross-run stale contamination.

### Task 6: Add Per-Run Summary Artifact Generation

**Files:**
- Create: `internal/logger/run_summary.go`
- Create: `internal/logger/run_summary_test.go`
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Build deterministic run summary artifact generation using current run logs:
- per-phase tokens/cost/latency
- retry/validation outcomes
- aggregation totals and mode usage indicators (pruning/delta)
Write summary artifact at run completion for before/after comparison.

**Acceptance Criteria:**
- Summary artifact is produced for runs with logs.
- Summary contains required baseline metrics and phase breakdowns.
- Repeated processing of same log set yields deterministic output.

**Dependencies:**
- Task 4
- Task 5

**Notes:**
Use run ID scoping to avoid mixing data from other runs.

### Task 7: Add Iteration Log Operability Fields and End-to-End Wiring

**Files:**
- Modify: `internal/logger/logger.go`
- Modify: `internal/pipeline/epilogue/epilogue.go`
- Modify: `internal/runner/orchestrator.go`
- Test: `internal/logger/logger_test.go`
- Test: `internal/pipeline/epilogue/epilogue_test.go`

**What to Do:**
Ensure iteration/run telemetry captures whether pruning/delta modes were used per invocation and propagates this to persisted artifacts.

**Acceptance Criteria:**
- Log records include mode flags when features are active.
- Fields are omitted or false/default in disabled mode as designed.
- Existing consumers/tests remain green after schema extension.

**Dependencies:**
- Task 3
- Task 5
- Task 6

**Notes:**
Prefer optional JSON fields with explicit tests for presence/absence behavior.

### Task 8: Quality Gates and Measurement Protocol Execution

**Files:**
- Modify: docs/report artifact path as needed (e.g. `.gromit/logs` or benchmark output)
- Test/Run: existing test suites and fixed-workload benchmark harness

**What to Do:**
Run required quality gates and execute measurement protocol:
- baseline workload >=3 runs pre-change
- same workload >=3 runs post-change
- compare medians and variance notes against targets

**Acceptance Criteria:**
- No material regression in success/validation failure rates.
- Report includes token/cost/duration median comparisons.
- Rollback path validated by toggling each feature independently.

**Dependencies:**
- Tasks 1-7

**Notes:**
Capture exact command lines and artifact paths to make comparison repeatable.

---

## Notes

- Deliver in rollout order from the spec: instrumentation/summaries first, then pruning, then context delta.
- Keep transforms deterministic and observable; avoid hidden heuristics.
- Preserve full-fidelity raw logs out-of-band; prompt context should receive compact, failure-focused payloads.
- This plan is intentionally decompose-ready: tasks are scoped for mapping into 1-3 beads each.
