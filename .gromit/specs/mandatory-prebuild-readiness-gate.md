---
id: mandatory-prebuild-readiness-gate
source_ideas: []
created: 2026-02-28
epic: token-efficiency-program
---

# Mandatory Pre-Build Readiness Gate

## Specification

Gromit must enforce a mandatory readiness gate before the Build stage for every bead run by `gromit run`, regardless of whether the bead has a `spec:<name>` label.

The gate runs after the existing precheck "already complete" path and before any expensive build invocation. If readiness fails, the bead is blocked and Build is not invoked for that iteration.

### Readiness outcomes

The gate returns one of these outcomes:

- `ready`: bead proceeds to Build
- `not_ready_criteria`: bead is blocked due to missing or ambiguous acceptance criteria
- `not_ready_scope`: bead is blocked due to oversized or multi-concern scope

### Default behavior

- Readiness gating is enabled by default for all runs.
- Readiness operates fail-closed for readiness violations (criteria/scope violations block execution).
- Existing reason codes (`criteria_missing`, `criteria_ambiguous`, `scope_too_broad`) are emitted in gate output and telemetry so blocked beads are distinguishable from other gate outcomes.

### Rollout behavior

Initial enforcement focuses on deterministic checks that do not require model judgment:

- acceptance criteria presence
- acceptance criteria count upper bound
- expected outputs scope/file-count bound

Prompt/model-based readiness judgment can be added later, but mandatory prevention must not depend on the current stub assessor.

### Operator escape hatch

A temporary emergency override exists to bypass readiness blocking for a run when operators need to unblock critical work. The override must be explicit and visible in logs/status, and default behavior remains mandatory-on.

## Acceptance Criteria

- A bead without acceptance criteria is blocked before Build and emits `criteria_missing`
- A bead exceeding readiness criteria-count limits is blocked before Build and emits `criteria_ambiguous`
- A bead exceeding scope/output limits is blocked before Build and emits `scope_too_broad` (or takes configured decomposition path), and Build is not invoked
- A bead passing readiness proceeds through existing Build/Validate/Review/Epilogue flow unchanged
- Readiness gating is active by default for unlabeled beads and `spec:*` beads alike
- An explicit emergency override can disable readiness blocking for that run, and the run logs clearly indicate the override was used
- Telemetry distinguishes readiness blocks from precheck skips, stuck-bead blocks, and validation/build failures

## Decisions

1. **Apply to all beads, not only `spec:*` labels.** Wasted build cost comes from ambiguous bead definitions independent of labeling. Queue-wide gating closes the largest prevention gap.

2. **Mandatory-by-default with explicit emergency override.** Prevention should be the default operating mode; the override exists for incident response, not normal execution.

3. **Deterministic enforcement first.** The current prompt readiness assessor is a stub that always returns ready. Shipping mandatory gating on deterministic criteria/scope checks provides immediate prevention with predictable behavior.

4. **Block before Build rather than learning from failed builds.** Failed build attempts consume nearly the same expensive resources as successful attempts, so prevention before invocation provides better cost and cycle-time outcomes.

## Research & Context

### Current state in codebase

- Readiness gate integration point exists in `internal/pipeline/prepare/gate.go` via `WithReadinessAssessor(...)`.
- Runner wiring currently enables readiness only when `readiness_check.enabled` is true (`internal/runner/constructor.go`).
- Prompt readiness assessor currently returns ready for all beads (`internal/prompt/readiness_assessor.go`), so prompt-based readiness does not enforce blocking today.
- Deterministic readiness checks and reason codes already exist (`internal/pipeline/prepare/readiness.go`), including:
  - criteria presence (`criteria_missing`)
  - criteria count upper bound (`criteria_ambiguous`)
  - expected outputs scope bound (`scope_too_broad`)
- Prior refinement spec `.gromit/specs/prebuild-scope-and-criteria-readiness.md` defines the intended prevention direction; this spec makes that behavior mandatory and queue-wide by default.

### Why this fits

Gromit already separates pre-build gate decisions from expensive build execution. Making readiness mandatory uses that architecture to shift quality from post-build inspection to pre-build prevention and reduces avoidable compute spend on poorly defined beads.
