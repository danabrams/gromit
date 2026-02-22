---
id: decompose-low-complexity-bias
source_ideas: []
created: 2026-02-22
---

# Decompose Low Complexity Bias

## Specification

`gromit decompose` should bias output toward low-complexity beads that are practical for low-tier model execution (for example Haiku), without banning high-complexity beads. The decomposition workflow should detect likely over-scoped bead candidates, automatically reprompt the model with targeted feedback, and iterate adaptively until complexity is reduced as far as practical.

This behavior applies to the plan-to-beads decomposition path (`gromit decompose`) only. It does not change runtime decomposition of active beads in other pipeline stages.

The decompose workflow should evaluate each candidate bead using a heuristic complexity assessment that uses both:
- Candidate output fields (`title`, `description`, `acceptance_criteria`, `estimated_files`, dependency shape)
- Relevant plan context (task wording, declared scope, and dependency intent)

Heuristic complexity signals should include broad-scope language, multi-package breadth, mixed concerns in one bead, criteria suggesting multiple independently deliverable behaviors, and other indicators that the bead is likely too large for low-tier execution.

When high-complexity candidates are detected, decompose should auto-reprompt with structured feedback that asks for finer decomposition while preserving semantic intent and avoiding sibling overlap. Adaptive retry behavior should:
- Retry while high-complexity beads remain
- Stop early if zero high-complexity beads remain
- Stop when no further improvement is observed across attempts
- Respect configured retry ceilings to cap cost/time

The final output may still include high-complexity beads when further practical decomposition is not achieved within retry limits. In these cases, the user should receive a clear warning summary of remaining high-complexity beads and why they remained high.

## Acceptance Criteria

- Running `gromit decompose` evaluates bead complexity with a heuristic that combines candidate output fields and plan context, rather than using file count alone.
- When any candidate bead is classified high-complexity, decompose automatically issues targeted reprompts to request finer-grained decomposition.
- Adaptive retry loop stops early only when no high-complexity beads remain, or stops on non-improving attempts, or reaches configured retry limits.
- Decompose is not hard-blocked by remaining high-complexity beads after retry limits; it proceeds with warnings and creates beads.
- Decompose output/logging includes a concise summary of complexity outcomes per attempt and the final remaining high-complexity bead count.
- Runtime bead decomposition outside `gromit decompose` is unchanged.

## Decisions

1. **Bias, not ban**  
   High-complexity beads are allowed as fallback. The system should prefer low complexity where practical but avoid hard failure modes that stall delivery.

2. **Auto-reprompt over warning-only**  
   The workflow should actively improve decomposition quality instead of merely reporting complexity issues.

3. **Heuristic scoring instead of single-threshold rules**  
   Over-scope is not reliably captured by one signal (for example estimated files). A multi-signal heuristic gives better practical detection.

4. **Include plan context in scoring**  
   Candidate text alone can hide scope coupling. Using plan task context improves classification quality and reduces false positives/negatives.

5. **Adaptive retries with strict success target**  
   Retries aim for zero high-complexity beads, but stop on non-improving trajectories or configured limits to control time and token cost.

6. **Scope limited to plan decomposition path**  
   This refinement intentionally targets `gromit decompose` only and does not alter runtime decomposition behavior.

## Research & Context

### Current State

- `internal/pipeline/decompose.go` runs decompose non-interactively and includes a validation/reprompt loop.
- Current decompose validation (`internal/validate/validate.go`) checks criteria count, scope-signal keywords, and sibling overlap, but does not classify low vs high execution complexity for low-tier models.
- Existing prompt/skill guidance in `skills/gromit-decompose/SKILL.md` and `.gromit/templates/PROMPT_decompose.md` defines sizing philosophy, but practical outputs can still produce over-scoped beads.
- Decompose currently uses a fixed model for decomposition (`decomposeModel = "sonnet"`) and does not have an explicit complexity optimization loop aimed at low-tier executability.

### Relationship to Existing Specs

- `.gromit/specs/decomposition-granularity.md` establishes natural implementation units and anti-overlap-oriented sizing guidance.
- This spec extends that direction by adding explicit low-complexity optimization behavior and adaptive reprompting in decompose orchestration.

### Non-Goals

- No hard prohibition of `complexity:high` beads.
- No changes to non-plan decomposition paths.
- No implementation plan/task breakdown in this spec; this is a refinement-level behavioral contract.
