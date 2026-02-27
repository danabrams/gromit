---
id: explore-model-forwarding-non-claude
source_spec: explore-model-forwarding-non-claude
created: 2026-02-27
decomposed: false
---

# Explore Model Forwarding For Non-Claude Agents Implementation Plan

**Goal:** Ensure `gromit explore --model <x>` is forwarded to non-Claude interactive agent launches (Codex, Gemini, and custom agents) with warning-and-continue fallback when unsupported.

**Architecture:** Add a best-effort model-forwarding adapter in `internal/agent` and invoke it in the explore pipeline immediately after agent resolution and before launch, preserving existing behavior when model is omitted.

**Tech Stack:** Go, Cobra CLI, existing `internal/agent` and `internal/pipeline` abstractions, Go test framework.

**Spec:** `.gromit/specs/explore-model-forwarding-non-claude.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Keep changes scoped to explore's interactive launch path by adding an agent-level best-effort model forwarding adapter, then wiring it in `pipeline.Explore` right before launch.

**Key Components:**
1. **`internal/agent` model-forwarding adapter**: Add helper(s) that take a resolved agent plus requested model and return either a model-overridden agent or a warning.
2. **`internal/pipeline/explore.go` launch integration**: Apply forwarding when `ExploreInput.Model` is non-empty, emit warning, and continue launch if unsupported.
3. **Explore warning output seam**: Add a small injectable warning writer/function in pipeline so tests can assert warning-and-continue behavior without brittle stdout/stderr coupling.

**Integration Points:**
- `pipeline.Explore` remains responsible for prompt rendering, resolver call, launch, and artifact diffing.
- `agent` package remains owner of how CLI args are constructed for each agent type.
- No behavior change when `--model` is omitted.

**Data Flow:**
1. `runExplore` sets `ExploreInput.Model`.
2. `Pipeline.Explore` resolves selected agent.
3. If `input.Model != ""`, call agent adapter (best-effort):
- Supported: returns wrapped/overridden agent that includes model launch args.
- Unsupported: returns warning string; pipeline emits warning and continues with original agent.
4. Launch proceeds via `LaunchInDir`.
5. Existing artifact post-processing remains unchanged.

**Files to Modify:**
- `internal/pipeline/explore.go` - invoke model-forwarding adapter and warning fallback before launch.
- `internal/pipeline/pipeline.go` (or existing pipeline wiring file) - add warning seam if needed.
- `internal/agent/resolve.go` - optionally expose preset-aware metadata/hooks if needed by adapter.
- `internal/agent/agent.go` - minimally extend agent internals only if required for safe cloning/arg injection.

**Files to Create:**
- `internal/agent/model_forwarding.go` - best-effort explore model forwarding logic + result type.
- `internal/agent/model_forwarding_test.go` - codex/gemini/custom/unsupported coverage.
- `internal/pipeline/explore_model_forwarding_test.go` - end-to-end explore propagation + warning-and-continue assertions.

**Tradeoffs:**
- **Agent-level adapter vs command-level branching**: choose agent-level to avoid command-specific duplication and keep behavior consistent across explore paths.
- **Warning-and-continue vs hard error**: choose warning to preserve interactive explore availability per spec.
- **Best-effort custom forwarding**: attempt generic `--model <x>` append for prompt-file-arg style custom agents where feasible; fall back to warning if not safe/detectable.

## Test Strategy

**Test Levels:**
1. **Unit Tests (`internal/agent`)**: Validate model-forwarding adapter behavior per agent type.
2. **Integration Tests (`internal/pipeline`)**: Validate `ExploreInput.Model` propagation reaches launch behavior and warnings are emitted with continue-on-unsupported.
3. **Regression Tests (`cmd/gromit` / existing explore tests)**: Ensure no behavior drift when model is omitted and Claude-selected flows remain unchanged.

**Key Test Cases:**
- `codex` + model override:
- Input model set.
- Launch args include model forwarding (no silent drop).
- `gemini` + model override:
- Input model set.
- Launch args include model forwarding for gemini invocation style.
- Custom agent + model override supported path:
- Best-effort forwarding attempt happens and launch continues.
- Custom/unsupported forwarding path:
- Warning includes selected agent name.
- Explore still launches successfully without model override.
- Omitted model:
- No forwarding attempt.
- Existing launch args unchanged.
- Claude compatibility:
- Existing Claude behavior remains intact in explore path.

**Mocking Strategy:**
- Use existing mock/stub agent patterns from `internal/pipeline/explore_test.go` and `internal/agent/*_test.go`.
- Capture launch args via `cliAgent` seams (or adapter outputs) instead of full subprocess execution.
- Capture warnings through an injectable warning function/writer seam in pipeline tests.

**Coverage Goals:**
- Critical path: `ExploreInput.Model` must not be dropped.
- Behavior fallback: unsupported forwarding must warn and not fail session.
- Agent variety: built-in (`codex`, `gemini`) + custom.
- Non-regression: no change when model empty.

**Test Organization:**
- Keep adapter tests in `internal/agent/model_forwarding_test.go`.
- Keep explore integration behavior in a dedicated `internal/pipeline/explore_model_forwarding_test.go` (or append focused cases to existing `explore_test.go` if preferred by project style).
- Follow current table-driven style used across `internal/agent` and `cmd/gromit` tests.

## Implementation Tasks

### Task 1: Build Agent-Side Model Forwarding Adapter

**Files:**
- Create: `internal/agent/model_forwarding.go`
- Test: `internal/agent/model_forwarding_test.go`
- Modify: `internal/agent/agent.go` (only if needed for safe cloning/introspection)

**What to Do:**
Implement a helper that accepts a resolved `agent.Agent` and requested model, then returns either:
- an overridden agent configured with model arguments for supported launch surfaces, or
- a warning result indicating forwarding is unsupported.

Support built-in `codex` and `gemini` presets explicitly. For custom agents, attempt best-effort forwarding when the launch shape is compatible with appending model arguments.

**Acceptance Criteria:**
- Codex forwarding returns an agent launch configuration that includes the requested model.
- Gemini forwarding returns an agent launch configuration that includes the requested model.
- Unsupported/custom-incompatible forwarding returns a warning result and preserves the original agent.

**Dependencies:**
- None.

**Notes:**
- Keep adapter API narrow and explore-focused to avoid cross-command coupling.
- Avoid mutating shared slices on original agent instances; clone before appending args.

### Task 2: Wire Explore Pipeline to Use Model Forwarding with Warning Fallback

**Files:**
- Modify: `internal/pipeline/explore.go`
- Modify: `internal/pipeline/pipeline.go` (or an adjacent pipeline utility file for warning seam)
- Test: `internal/pipeline/explore_model_forwarding_test.go`

**What to Do:**
After `AgentResolver.Resolve` in `Pipeline.Explore`, check `ExploreInput.Model`. If non-empty, invoke the new adapter:
- If override succeeds, launch with the overridden agent.
- If unsupported, emit warning naming the selected agent and continue launching the original agent.

Introduce a testable warning emission seam so unit/integration tests can assert warning output deterministically.

**Acceptance Criteria:**
- `ExploreInput.Model` reaches launch-time forwarding logic and is no longer silently dropped.
- Unsupported forwarding emits a warning and does not fail `Explore`.
- Existing artifact scanning/diff behavior and launch flow remain unchanged.

**Dependencies:**
- Task 1.

**Notes:**
- Preserve current behavior when `ExploreInput.Model` is empty.
- Keep warning text stable enough for assertion, including agent name.

### Task 3: Add Regression Coverage for Explore Command Delegation and Compatibility

**Files:**
- Modify: `internal/pipeline/explore_agent_input_test.go`
- Modify: `internal/pipeline/explore_test.go`
- Optional Modify: `cmd/gromit/explore_test.go` (only if command-layer assertion is needed)

**What to Do:**
Expand existing explore tests to assert that command/pipeline delegation preserves compatibility while adding non-Claude forwarding behavior:
- Codex and Gemini model forwarding coverage.
- Custom agent warning-and-continue fallback.
- Claude-selected explore flows remain compatible.
- Empty model path remains unchanged.

**Acceptance Criteria:**
- New tests explicitly cover codex/gemini propagation and unsupported/custom fallback warnings.
- Existing explore delegation tests continue to pass without semantic drift.
- No regressions in unchanged explore behavior (prompt rendering, session launch, artifact detection).

**Dependencies:**
- Task 2.

**Notes:**
- Prefer table-driven cases to keep agent permutations easy to extend.

---

## Notes

- This plan is intentionally explore-scoped and does not alter `gromit run` routing or non-explore command semantics.
- If implementation reveals broader shared override needs, file follow-up beads linked as `discovered-from` this spec rather than expanding scope mid-task.
