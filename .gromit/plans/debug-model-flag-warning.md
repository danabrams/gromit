---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T09:02:57Z"
id: debug-model-flag-warning
source_spec: debug-model-flag-warning
---

# Debug --model Flag Warning for Non-Claude Agents Implementation Plan

**Goal:** Warn users at runtime when `debug --model` is provided with a non-Claude agent, while preserving existing debug behavior.

**Architecture:** Add a narrow warning branch in `runDebug` after agent selection and before launch, keyed on explicit `--model` usage with a non-Claude agent. Keep Claude-only model override behavior unchanged.

**Tech Stack:** Go, Cobra CLI, existing gromit agent resolution/launch flow, Go test.

**Spec:** `.gromit/specs/debug-model-flag-warning.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add a small runtime warning path in the debug command that detects when `--model` is explicitly set but the selected debug agent is not Claude, and emit a stderr warning without changing execution behavior.

**Key Components:**
1. **`runDebug` warning gate**: Evaluate `cmd.Flags().Changed(debugModelFlag)` and selected agent name, then print warning to `os.Stderr` for non-Claude agents.
2. **Existing model override path**: Keep `shouldOverrideDebugModel()` and Claude override construction unchanged.
3. **Test coverage updates**: Add focused tests validating warning emission and non-emission scenarios.

**Integration Points:**
- Integrates directly in the existing `runDebug` flow after `resolveDebugAgent`.
- Reuses existing constants (`debugModelFlag`, `claudeAgentName`) and selected agent abstraction.
- No changes to flags, config schema, or help text.

**Data Flow:**
CLI parses flags -> debug resolves selected agent -> warning condition is evaluated -> warning may be written to stderr -> Claude-only override behavior runs as today -> debug session launches.

**Files to Modify:**
- `cmd/gromit/debug.go` - add warning emission when `--model` is set for a non-Claude selected agent.
- `cmd/gromit/debug_model_override_test.go` and/or `cmd/gromit/debug_test.go` - add tests for warning conditions.

**Files to Create:**
- None expected.

**Tradeoffs:**
- **Inline conditional vs helper abstraction**: keep inline for minimal surface area and direct readability in `runDebug`.
- **Warning vs hard failure**: warning preserves supported non-Claude debugging while surfacing ignored `--model`.

## Test Strategy

**Test Levels:**
1. **Unit tests**: Validate warning-condition logic around model-flag changed state and selected agent identity.
2. **Command-flow tests**: Validate stderr warning behavior in debug flow with stubs/mocks and no external agent execution.
3. **Manual verification (optional)**: Confirm warning appears in real CLI usage for non-Claude + `--model`.

**Key Test Cases:**
- `--model` provided + non-Claude selected agent => warning written to stderr before launch.
- Warning text includes active agent name (for example `codex`).
- `--model` provided + Claude selected agent => no warning.
- `--model` not provided + non-Claude selected agent => no warning.
- Existing Claude override behavior remains unchanged.

**Mocking Strategy:**
- Reuse existing debug command seams (session launcher and test agent stubs).
- Capture stderr output via buffer where possible.
- Avoid invoking external binaries; keep tests deterministic and local.

**Coverage Goals:**
- Cover the exact runtime branch that produces user-facing warning.
- Guard against false-positive warnings in valid Claude workflows.
- Preserve behavior compatibility for existing debug pathways.

**Test Organization:**
- Prefer appending focused tests to existing debug model/agent test files to keep related behavior together.
- Use table-driven cases where practical for warning/no-warning combinations.

## Implementation Tasks

### Task 1: Add runtime warning for ignored non-Claude model override

**Files:**
- Modify: `cmd/gromit/debug.go`

**What to Do:**
Insert a warning branch in `runDebug` after selected agent resolution and before launch. If `--model` was explicitly set and the selected agent is not Claude, emit:
`warning: --model flag is only supported for the Claude agent; ignoring for <agent-name>`
to stderr, then continue normal execution.

**Acceptance Criteria:**
- Warning is emitted to stderr when `--model` is set and selected agent is non-Claude.
- Warning includes the selected agent name.
- Existing launch and override behavior remains unchanged.

**Dependencies:**
- None.

**Notes:**
- Keep help text and CLI/config surfaces unchanged.
- Preserve current non-Claude debug support (warning only, no failure).

### Task 2: Add regression tests for warning and non-warning paths

**Files:**
- Modify: `cmd/gromit/debug_model_override_test.go` and/or `cmd/gromit/debug_test.go`

**What to Do:**
Add focused tests covering warning emission for non-Claude `--model` usage and ensuring no warning in Claude or unset-model scenarios. Verify the warning text and agent-name interpolation.

**Acceptance Criteria:**
- Tests fail if warning is missing for non-Claude + changed model flag.
- Tests fail if warning appears for Claude or when model flag is unchanged.
- Existing model override tests continue to pass.

**Dependencies:**
- Task 1.

**Notes:**
- Prefer table-driven structure for scenario matrix clarity.

### Task 3: Validate behavior with targeted and package test runs

**Files:**
- Test: `cmd/gromit/debug*_test.go` suites

**What to Do:**
Run targeted debug tests first, then `go test ./cmd/gromit/...` to confirm no regressions in debug command behavior.

**Acceptance Criteria:**
- Targeted debug warning tests pass.
- Package-level gromit command tests pass.

**Dependencies:**
- Task 2.

**Notes:**
- No production code changes expected in this task.

---

## Notes

- This is a documentation-level runtime signal only; no behavior change to agent selection or launch semantics.
- Plan is intentionally small and scoped to one command path for safe decomposition into a few beads.
