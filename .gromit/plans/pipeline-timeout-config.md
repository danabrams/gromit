---
created: 2026-02-15T00:00:00Z
decomposed: true
decomposed_at: "2026-02-15T20:27:53Z"
id: pipeline-timeout-config
source_spec: pipeline-timeout-config
---

# Configurable Pipeline Invocation Timeout Implementation Plan

**Goal:** Make pipeline-phase Claude invocations (decompose and review) use a configurable timeout from `claude.pipeline_timeout` instead of a hardcoded 30-minute value.

**Architecture:** Extend `ClaudeConfig` with a `pipeline_timeout` field and default, then thread that value into `claudeClientAdapter` construction in decompose/review so adapter `Run()` uses injected timeout duration.

**Tech Stack:** Go, Cobra CLI, YAML config loading via existing config package, pipeline adapters in `cmd/gromit`.

**Spec:** `.gromit/specs/pipeline-timeout-config.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add a new `claude.pipeline_timeout` config field (seconds) with default `1800`, propagate it into `claudeClientAdapter`, and use it in adapter `Run()` so decompose/review pipeline invocations stop using hardcoded `30*time.Minute`.

**Key Components:**
1. **`internal/config/config.go` (`ClaudeConfig`)**: Add `PipelineTimeout int \`yaml:"pipeline_timeout"\``.
2. **`internal/config/config.go` (`SetDefaults`)**: Set `PipelineTimeout` to `1800` when zero.
3. **`cmd/gromit/adapters.go` (`claudeClientAdapter`)**: Add `Timeout time.Duration` field; replace hardcoded timeout in `Run()` with `a.Timeout`.
4. **`cmd/gromit/decompose.go`**: Pass configured timeout when constructing `claudeClientAdapter`.
5. **`cmd/gromit/review.go`**: Pass configured timeout when constructing `claudeClientAdapter`.
6. **Config surface/tests**: Update `gromit.yaml` docs and config tests for default + YAML load behavior.

**Integration Points:**
- Extends existing Claude timeout config family (`timeout`, `stall_timeout`, `bead_timeout`, `analysis_timeout`) without changing runner timeout behavior.
- Only pipeline orchestration paths (`decompose`, `review`) are changed.
- Existing review command timeout (`cfg.Review.Thorough.Timeout`) remains the outer command timeout; adapter timeout controls Claude pipeline invocations inside that context.

**Data Flow:**
- YAML `claude.pipeline_timeout` -> `config.Load()` -> `cfg.Claude.PipelineTimeout`.
- `decompose/review` build adapter with `time.Duration(cfg.Claude.PipelineTimeout) * time.Second`.
- `claudeClientAdapter.Run()` creates context timeout from adapter field and invokes `claude.Client.Run`.

**Files to Modify:**
- `internal/config/config.go` - new field + default.
- `internal/config/config_test.go` - default/load assertions for `PipelineTimeout`.
- `cmd/gromit/adapters.go` - adapter timeout field + usage.
- `cmd/gromit/decompose.go` - pass timeout into adapter construction.
- `cmd/gromit/review.go` - pass timeout into adapter construction.
- `gromit.yaml` - add `pipeline_timeout` comment in `claude:` section.

**Files to Create:**
- None expected.

**Tradeoffs:**
- **Single `pipeline_timeout` knob** over per-phase knobs: simpler and matches spec scope.
- **Duration passed at construction** over reading config in adapter: keeps adapter reusable/testable and avoids config coupling.
- **No fallback in adapter `Run()`** beyond configured value: relies on config defaults to preserve behavior; keeps runtime logic simple.

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit Tests (config):** verify YAML deserialization and defaults for `claude.pipeline_timeout`.
2. **Unit/structural tests (CLI wiring):** verify `claudeClientAdapter` has/uses `Timeout` and call sites pass configured value.
3. **Manual smoke check (optional):** run `gromit decompose` / `gromit review --non-interactive` with custom config to confirm no regressions.

**Key Test Cases:**
- `SetDefaults()` sets `cfg.Claude.PipelineTimeout == 1800` when unset.
- YAML with `claude.pipeline_timeout: 2400` loads `cfg.Claude.PipelineTimeout == 2400`.
- `claudeClientAdapter.Run()` uses adapter `Timeout` (no hardcoded `30*time.Minute`).
- Decompose path constructs adapter with `Timeout: time.Duration(cfg.Claude.PipelineTimeout) * time.Second`.
- Review non-interactive path constructs adapter with same configured timeout.

**Mocking Strategy:**
- No new mocks required for config tests.
- Reuse existing source-structure tests in `cmd/gromit/decompose_adapters_test.go` style for adapter behavior/wiring assertions if needed.
- Keep tests lightweight and focused on deterministic config + wiring guarantees.

**Coverage Goals:**
- Critical path: config -> adapter construction -> adapter run timeout source.
- Backward compatibility: default remains 1800 seconds (30 minutes).
- Guard against regression to hardcoded timeout.

**Test Organization:**
- Extend existing tests in `internal/config/config_test.go`.
- Add/adjust adapter/wiring assertions in existing CLI test files (`cmd/gromit/decompose_adapters_test.go`, and if needed `cmd/gromit/review_*test.go`).
- Keep naming aligned with existing pattern (`Test...PipelineTimeout...`).

## Implementation Tasks

### Task 1: Add Pipeline Timeout to Claude Config Model and Defaults

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `PipelineTimeout int` to `ClaudeConfig` with YAML tag `pipeline_timeout`, and apply a default of `1800` in `SetDefaults()` when unset. Add config tests for defaulting and YAML deserialization.

**Acceptance Criteria:**
- `ClaudeConfig` includes `PipelineTimeout int` mapped from `pipeline_timeout`.
- `SetDefaults()` sets `PipelineTimeout` to `1800` when zero.
- Loading YAML with `claude.pipeline_timeout: 2400` yields `cfg.Claude.PipelineTimeout == 2400`.

**Dependencies:**
- None.

**Notes:**
- Keep behavior consistent with existing timeout fields (int seconds in config, duration conversion at use site).

### Task 2: Make Claude Pipeline Adapter Timeout Configurable

**Files:**
- Modify: `cmd/gromit/adapters.go`
- Test: `cmd/gromit/decompose_adapters_test.go`

**What to Do:**
Add `Timeout time.Duration` to `claudeClientAdapter` and update `Run()` to use `a.Timeout` in `context.WithTimeout` instead of hardcoded `30*time.Minute`. Update/extend adapter tests to ensure timeout is sourced from struct field and hardcoded duration is absent.

**Acceptance Criteria:**
- `claudeClientAdapter` has a `Timeout time.Duration` field.
- `Run()` uses `context.WithTimeout(..., a.Timeout)`.
- No hardcoded `30*time.Minute` remains in adapter run path.

**Dependencies:**
- Task 1 (default value is needed to safely construct adapter everywhere).

**Notes:**
- Keep existing output mapping logic untouched (`claude.Result` -> `pipeline.ClaudeRunResult`).

### Task 3: Wire Configured Timeout into Decompose and Review Adapter Construction

**Files:**
- Modify: `cmd/gromit/decompose.go`
- Modify: `cmd/gromit/review.go`
- Optional test updates: `cmd/gromit/review_agent_test.go` (only if needed to reflect source changes)

**What to Do:**
At both adapter construction sites, pass `Timeout: time.Duration(cfg.Claude.PipelineTimeout) * time.Second` alongside `Client`. Ensure decompose and non-interactive review paths both use configured pipeline timeout.

**Acceptance Criteria:**
- Decompose adapter construction includes configured timeout duration conversion from config seconds.
- Review non-interactive adapter construction includes configured timeout duration conversion from config seconds.
- Pipeline adapter timeout is no longer implicit/hardcoded at call sites.

**Dependencies:**
- Task 1 (config field).
- Task 2 (adapter field).

**Notes:**
- Do not alter existing outer review context timeout (`cfg.Review.Thorough.Timeout`) semantics.

### Task 4: Update Reference Config Documentation and Verify End-to-End Behavior via Tests

**Files:**
- Modify: `gromit.yaml`
- Verify tests in: `internal/config/config_test.go`, `cmd/gromit/decompose_adapters_test.go`, plus any impacted command tests

**What to Do:**
Document `pipeline_timeout` in the sample `claude:` config block with a comment explaining it governs decompose/review pipeline phases. Run targeted tests covering config and adapter wiring.

**Acceptance Criteria:**
- `gromit.yaml` includes documented `claude.pipeline_timeout` with default semantics.
- Relevant config and CLI adapter tests pass after changes.
- Acceptance criteria from spec are all covered by code + tests.

**Dependencies:**
- Task 1, Task 2, Task 3.

**Notes:**
- Keep config comments concise and consistent with existing timeout comment style.

---

## Notes

- This plan intentionally keeps pipeline timeout separate from runner invocation timeouts (`TimeoutsForModel`) to avoid behavior changes in build/refactor/escalation flows.
- Default remains 1800 seconds to preserve current behavior for users who do not set `claude.pipeline_timeout`.
- After implementation, this plan should be decomposed into beads with at most 1-2 files per bead to match project bead sizing guidance.
