---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-27T01:44:18Z"
id: interactive-agent-model-overrides
source_spec: interactive-agent-model-overrides
---

# Interactive Agent Model Overrides Implementation Plan

**Goal:** Implement consistent interactive `--model` behavior across `refine`, `plan`, `explore`, `debug`, and interactive `review`, including per-command config defaults and warning fallback for unsupported agents.

**Architecture:** Add interactive command model defaults to config, resolve effective model with strict precedence (`CLI --model` > config per-command default > existing behavior), and route agent launch through a shared model-override adapter that warns and continues when the selected agent cannot accept model override flags.

**Tech Stack:** Go, Cobra CLI commands, existing `internal/agent` and `internal/config` subsystems, table-driven/unit and command integration tests.

**Spec:** `.gromit/specs/interactive-agent-model-overrides.md`

---

## Architecture

**Overview:**
Introduce a shared interactive-model resolution and agent-override adapter used by `refine`, `plan`, `explore`, `debug`, and interactive `review`, with precedence `CLI --model > config interactive default > existing behavior`, plus warning-and-continue fallback for unsupported agents.

**Key Components:**
1. **Interactive model config schema (`internal/config`)**: Add per-command interactive model defaults.
2. **Shared model resolution helper (`cmd/gromit`)**: Compute effective model by command and CLI flag precedence.
3. **Shared agent model override helper (`internal/agent` + `cmd/gromit`)**: Attempt model injection for selected interactive agent; if unsupported, emit warning and continue with original agent.
4. **Command wiring updates (`cmd/gromit/*.go`)**: Ensure all scoped interactive commands expose/use `--model` consistently.

**Integration Points:**
- Reuse current agent resolution (`agent.NewResolver(...).Resolve(...)`) and post-resolution adaptation.
- Replace debug-only model override path with shared logic (preserving warning semantics).
- Keep `gromit run` and non-interactive review routing untouched.

**Data Flow:**
1. Command parses `--model` (and tracks if explicitly set).
2. Resolve effective model with precedence:
   1. CLI `--model` (if changed)
   2. `config.agents.interactive_models.<command>`
   3. existing default behavior
3. Resolve agent as today.
4. Attempt to produce model-overridden launch agent.
5. If unsupported agent/flags path, print warning and launch original agent.
6. Launch session normally (`LaunchInDir` path unchanged).

**Files to Modify:**
- `internal/config/config_types.go` - Add interactive model defaults struct under `AgentsConfig`.
- `internal/config/config_defaults.go` - Defaults/backward compatibility for new fields.
- `cmd/gromit/explore.go` - Use shared interactive model resolver + override adapter.
- `cmd/gromit/refine.go` - Add `--model` and wire through shared flow.
- `cmd/gromit/plan.go` - Add `--model` and wire through shared flow.
- `cmd/gromit/debug.go` - Migrate current model override/warn logic to shared flow.
- `cmd/gromit/review.go` - Add interactive-only `--model`; keep non-interactive behavior unchanged.
- `cmd/gromit/adapters.go` (or nearby shared helper file) - Shared command-side model precedence + warning plumbing.
- `internal/pipeline/explore.go` (if needed) - Ensure model-aware launch path is used consistently where pipeline directly launches.

**Files to Create:**
- `internal/agent/model_override.go` - Encapsulate “attempt model override” behavior and unsupported detection.
- `internal/agent/model_override_test.go` - Unit tests for supported/unsupported override behavior.
- `cmd/gromit/interactive_model_override.go` (optional if command-side helper is split out) - Shared precedence/warning helper for command layer.

**Tradeoffs:**
- Shared override helper over per-command custom logic to avoid drift and duplicate warnings/precedence bugs.
- Warning-and-continue instead of hard fail to satisfy resilience requirements.
- Config under `agents` (interactive command defaults) to avoid changing run-loop routing semantics.

## Test Strategy

**Test Levels:**
1. **Unit Tests**
- Model precedence resolver (`CLI > config > existing default`).
- Agent override adapter behavior: supported agents get model flag; unsupported agents return warning + unchanged launch behavior.
- Config defaults/normalization for new interactive model fields.

2. **Command Integration Tests (`cmd/gromit`)**
- `explore`, `refine`, `plan`, `debug`, interactive `review` each:
  - accept `--model`
  - pass effective model to override helper
  - launch with overridden agent when supported
  - warn-and-continue when unsupported
- `review --non-interactive` remains unchanged by interactive model defaults.

3. **Pipeline/Behavioral Tests**
- `explore` regression test proving model is no longer dropped.
- Existing agent resolution precedence tests remain green (`--agent`, picker, phase defaults).
- `gromit run` unaffected tests remain green (no routing/model behavior changes).

4. **Manual Testing**
- Run representative commands with `--agent codex/gemini/claude/custom` and `--model <x>`.
- Verify warning text for unsupported custom agent and session still launches.
- Verify config default used when `--model` omitted.

**Key Test Cases:**
- `gromit explore --agent codex --model o3` attempts codex launch with `--model o3`.
- `gromit explore --agent gemini --model gemini-2.5-pro` attempts gemini launch with model flag.
- `gromit <interactive-cmd> --model X` overrides configured interactive default.
- `gromit <interactive-cmd>` with config default and no CLI model uses config default.
- Unsupported override path emits warning including agent name and “model override skipped,” then launches anyway.
- Non-interactive review and `run` model behavior unchanged.

**Mocking Strategy:**
- Mock `agent.Agent` launch and capture args/flags via injected seams (existing command test patterns).
- Mock/stub config instances directly for precedence tests.
- Keep command-level tests isolated from external subprocess execution.

**Coverage Goals:**
- Critical: precedence correctness and warning fallback.
- Critical: explore propagation regression.
- Critical: no behavior change for non-interactive review and run loop.
- Edge: missing model flag, unchanged default, custom agent definitions.

**Test Organization:**
- Extend existing command test files:
  - `cmd/gromit/explore_agent_test.go`
  - `cmd/gromit/debug_model_override_test.go`
  - `cmd/gromit/plan_agent_test.go`
  - `cmd/gromit/refine_agent_test.go`
  - `cmd/gromit/review_agent_test.go` and related review command tests
- Add focused helper tests in:
  - `internal/agent/model_override_test.go`
  - `internal/config/*_test.go` for new config fields/defaults

## Implementation Tasks

### Task 1: Add Interactive Model Defaults to Config Schema

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Test: `internal/config/*_test.go`

**What to Do:**
Introduce a new config section under `agents` for interactive per-command model defaults (for `refine`, `plan`, `explore`, `debug`, `review`). Wire defaults/normalization so missing fields remain zero-value and preserve current behavior.

**Acceptance Criteria:**
- Config supports reading `agents.interactive_models.<command>` values.
- Existing configs without this section load with no behavior changes.
- Tests validate parsing/default compatibility and no regressions in existing config behavior.

**Dependencies:**
- None.

**Notes:**
Keep field naming explicit and command-aligned to avoid ambiguity with run-loop model routing.

### Task 2: Implement Shared Interactive Model Resolution Precedence

**Files:**
- Modify/Create: `cmd/gromit/adapters.go` and/or `cmd/gromit/interactive_model_override.go`
- Test: `cmd/gromit/*_test.go` (shared helper-focused tests)

**What to Do:**
Create a command-layer helper that resolves the effective interactive model for a command using strict precedence: changed CLI `--model`, then config `agents.interactive_models.<command>`, then existing command default behavior.

**Acceptance Criteria:**
- Helper returns CLI value when the command flag is explicitly set.
- Helper returns config default when CLI flag is unset and config default exists.
- Helper falls back to existing behavior when neither source is set.

**Dependencies:**
- Task 1.

**Notes:**
Design helper to be reusable by all five interactive commands to prevent precedence drift.

### Task 3: Implement Agent Model Override Adapter with Warning Fallback

**Files:**
- Create: `internal/agent/model_override.go`
- Create: `internal/agent/model_override_test.go`
- Modify: `cmd/gromit/adapters.go` (integration wiring)

**What to Do:**
Add shared logic that attempts to construct a launchable agent carrying model override for supported agents (Claude, Codex, Gemini, and compatible custom definitions). When unsupported, return a structured warning signal so caller can print warning and continue with original agent.

**Acceptance Criteria:**
- Supported agents receive model override at launch configuration time.
- Unsupported override paths do not fail launch and expose warning metadata.
- Tests cover supported agents, unsupported custom patterns, and non-destructive fallback behavior.

**Dependencies:**
- Task 2.

**Notes:**
Keep override logic provider-agnostic and local to interactive launch flow only.

### Task 4: Wire Universal `--model` and Override Flow into Interactive Commands

**Files:**
- Modify: `cmd/gromit/explore.go`
- Modify: `cmd/gromit/refine.go`
- Modify: `cmd/gromit/plan.go`
- Modify: `cmd/gromit/debug.go`
- Modify: `cmd/gromit/review.go`
- Test: `cmd/gromit/explore_agent_test.go`
- Test: `cmd/gromit/refine_agent_test.go`
- Test: `cmd/gromit/plan_agent_test.go`
- Test: `cmd/gromit/debug_model_override_test.go`
- Test: `cmd/gromit/review_agent_test.go`

**What to Do:**
Expose/standardize `--model` on all scoped interactive commands and route launch setup through shared precedence + override helpers. For interactive `review`, apply this only to interactive mode and leave non-interactive model logic unchanged.

**Acceptance Criteria:**
- `refine`, `plan`, `explore`, `debug`, and interactive `review` all honor `--model` consistently.
- Commands emit clear warning and continue when selected agent cannot accept override.
- Existing agent selection precedence (`--agent`, picker, phase default, fallback) remains intact.

**Dependencies:**
- Task 2
- Task 3

**Notes:**
Refactor debug command to use shared behavior rather than command-specific Claude-only logic.

### Task 5: Ensure Explore Pipeline Propagates Model Override End-to-End

**Files:**
- Modify: `internal/pipeline/explore.go`
- Test: `internal/pipeline/explore_test.go`
- Test: `internal/pipeline/explore_agent_input_test.go`

**What to Do:**
Close the current gap where `ExploreInput.Model` can be lost. Ensure effective model chosen in command layer reaches actual agent launch path in explore workflows.

**Acceptance Criteria:**
- Explore launch path uses effective model when provided.
- Regression test proves model is no longer silently ignored.
- Existing explore behavior (artifact scanning, prompt rendering, result diffs) remains unchanged.

**Dependencies:**
- Task 3
- Task 4

**Notes:**
Prefer minimal changes that preserve current pipeline responsibilities.

### Task 6: Guardrails and Regression Coverage for Unchanged Paths

**Files:**
- Modify: relevant tests in `cmd/gromit/review*_test.go`, `cmd/gromit/*_test.go`, and targeted pipeline/agent tests

**What to Do:**
Add/adjust regression tests to assert that non-interactive `review` and `gromit run` model routing semantics are unchanged and that existing command/agent resolution tests still pass.

**Acceptance Criteria:**
- Tests explicitly confirm no behavior changes in out-of-scope paths.
- Full interactive command model override suite passes with precedence and warning coverage.
- Existing command/agent resolution tests continue to pass.

**Dependencies:**
- Task 4
- Task 5

**Notes:**
Keep this task focused on preventing accidental spillover into routing/escalation systems.

---

## Notes

- This plan intentionally scopes model defaulting/overrides to interactive command launches only.
- `gromit run`, tier routing, escalation chains, and methodology phase-model policies are explicitly excluded.
- Warning copy should include both agent identity and that model override was skipped to satisfy the spec’s clarity requirement.
- During implementation, keep helpers centralized to avoid command-specific drift and simplify future agent additions.
