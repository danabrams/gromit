---
id: prebuild-scope-and-criteria-readiness
source_ideas: [1]
created: 2026-02-28
epic: token-efficiency-program
accepted: true
---

# Pre-Build Scope and Criteria Readiness Gate

## Specification

Extend the existing Gate-stage precheck so beads are screened for execution readiness before any full build invocation. The readiness screen runs when a bead is not already complete and determines whether the bead is both:

1. Defined by unambiguous acceptance criteria
2. Narrow enough to complete in a single build iteration

If readiness fails, the bead is blocked before build instead of spending a full build attempt that is likely to fail.

### Readiness Behavior

After the current "already done?" precheck path, the gate performs a readiness assessment that returns one of three outcomes:

- `ready` - proceed to normal build/validate/review stages
- `not_ready_scope` - block the bead with a scope-readiness reason
- `not_ready_criteria` - block the bead with a criteria-quality reason

Blocked beads remain visible for decomposition/refinement workflows and are not sent to build until their scope/criteria quality is improved.

### Readiness Signals

The readiness assessment uses both structured bead data and prompt-based judgment:

- Structured checks:
  - Acceptance criteria exist and are non-empty
  - Criteria count stays within the existing decomposition sizing guidance (1-3)
  - Expected outputs are present and align with a single concern
- Prompt-based checks:
  - Criteria are specific, testable, and non-overlapping
  - Scope does not bundle multiple independent concerns or broad "and also" work
  - Completion target appears feasible in one iteration

When uncertain, the gate should choose `not_ready_*` rather than allowing an expensive build attempt.

### Observability

Gate output and iteration telemetry include readiness outcomes and reason codes so operators can distinguish:

- already-done skips (`precheck_passed`)
- stuck/scope blocks (existing reasons)
- new readiness blocks (`criteria_ambiguous`, `scope_too_broad`, `criteria_missing`, etc.)

Readiness blocks are tracked separately from build failures so efficiency analysis can measure prevented failed-build spend.

## Acceptance Criteria

- Gate runs a readiness check before build for beads that are not auto-closed by precheck
- If readiness identifies ambiguous/insufficient acceptance criteria, the gate returns `Block` and build is not invoked
- If readiness identifies over-broad scope for a single iteration, the gate returns `Block` (or scope decomposition path if configured) and build is not invoked
- Readiness outcomes emit explicit reason codes in gate events/logging so they are distinguishable from existing block reasons
- Iteration/process metrics include readiness-block counts to support failure-rate and cost analysis
- A bead that passes readiness continues through the existing pipeline unchanged

## Decisions

1. **Block before build, not after first failed attempt.** The cost data shows failed builds are nearly as expensive as successful builds, so screening quality/scope before build has better ROI than learning only from failure.

2. **Extend existing precheck flow instead of creating a separate phase.** This keeps the gate path centralized and preserves current stage ordering while adding readiness semantics.

3. **Use conservative gating on uncertainty.** False negatives (blocking beads that might have succeeded) are cheaper than false positives that trigger full build cycles on under-scoped or ambiguous work.

4. **Preserve decomposition as the scope remedy.** When scope is the blocker, the existing decomposition path remains the primary remediation rather than forcing manual intervention.

5. **Instrument readiness separately from build failure.** Without separate reason tracking, the system cannot verify whether the new gate reduces failed-build share and total execution cost.

## Research & Context

### Current State

- Gate stage sequencing is implemented in `internal/pipeline/prepare/gate.go` and currently supports precheck skip, stuck block, optional data-quality block hook, and scope gate decisions.
- Runtime scope gate currently keys on expected output/file-count threshold (`maxScopeFiles = 5`) and may decompose or block based on configuration.
- The precheck prompt in `cmd/gromit/templates.go` is designed for binary completion verdicts (`PRECHECK_PASSED` vs `PRECHECK_NOT_MET`) and does not currently classify readiness quality.
- Config surfaces for related behavior already exist in `internal/config/config_types.go` (`precheck`, `scope_check`) and can be extended for readiness controls.
- A `DataQualityBlocker` extension point already exists in Gate but is not yet wired in constructor flow, making it a plausible integration point for readiness checks.

### Why This Fits

The existing Gate stage already owns "should this bead proceed to build?" decisions. Adding criteria/scope readiness screening here keeps early-exit logic in one place and targets the highest avoidable cost bucket: failed full-build attempts caused by unclear or oversized bead definitions.
