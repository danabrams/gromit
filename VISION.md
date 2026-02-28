# VISION

## Purpose

This system exists to automate code generation so humans can focus on high-level strategic product decisions.

## Two-Year Outcome (By February 28, 2028)

1. Human intervention in planning, decomposition, implementation, and code review is rare:
   - <=10% of tasks require human tactical intervention
   - <=2% of tasks require human debugging intervention
2. Delivered code matches planned vision and acceptance criteria without accidental behavioral regressions:
   - >=95% of completed work passes acceptance criteria on first integration
   - Escaped regression rate <=1%
3. The system reliably outputs software that matches the creator's vision:
   - >=90% of delivered features are accepted by the creator as matching intended product direction without rework

## Metric Definitions (v0.1)

1. Task unit (cycle): one spec execution cycle from start trigger to end trigger.
2. Current start trigger: when `decompose` is run on an approved plan.
3. Target future start trigger: explicit plan approval.
4. End trigger: PR presented to product owner for subjective review.
5. Human tactical intervention: any unplanned human corrective action during a cycle, including:
   - Editing plan/decompose artifacts to unblock execution
   - Manual code changes to correct agent mistakes
   - Mid-cycle model/strategy rerouting
   - Run-loop failure correction work
   - Spec mismatch correction work
   - Regression correction work (including outside spec scope)
   - Pure approval/review without corrective changes does not count
6. Human debugging intervention (subset of tactical intervention): human detection, diagnosis, or fixing of failures, including run-loop failures, spec mismatches, and regressions.
7. First integration pass: first PR presented to product owner where objective gates run end-to-end once (tests plus acceptance checks) without follow-up corrective commits.
8. Escaped regression: any regression detected within 7 days after PR presentation.
9. Escaped regression rate:
   - Numerator: cycles with at least one escaped regression in the 7-day window
   - Denominator: all cycles that reached PR presentation
10. Accepted without rework:
   - Counts as rework: post-presentation code/behavior changes required because implementation failed agreed spec or acceptance criteria
   - Does not count as rework: changes due to product-owner vision change or late clarification after PR presentation
11. Review outcome labels (required for auditability):
   - `accepted`
   - `rework_implementation_gap`
   - `rework_vision_change`
   Only `rework_implementation_gap` counts against the accepted-without-rework target; `rework_vision_change` must include a short rationale note.

## Non-Negotiable Guardrails

1. Safety
2. Matching intent
3. Ability to continue evolving the system
4. Cost efficiency
5. Documented verifiability

## Design Principles

1. Prefer explicit contracts over hidden behaviors.
2. Optimize for compounding improvements over one-off wins.
3. Preserve human strategic control; automate execution details.
4. Require evidence for correctness (tests and acceptance checks) before completion.
5. Keep the system evolvable: changes must remain reversible, understandable, and maintainable.

## Alignment Test (Soft Gate)

When adding or changing `RULES.md`, `LEARNINGS.md`, or specs, evaluate against these checks:

1. Does this improve long-term reliability?
2. Does this preserve alignment to intended product outcomes?
3. Does this reduce required human tactical intervention?
4. Does this maintain or improve verifiability?
5. Does this keep the system evolvable and reversible?
6. Is the cost/benefit ratio justified?
7. Does this avoid local optimization that harms user value?

This is a soft gate. If any check fails, work may proceed only with explicit written justification.

## Anti-Goals

1. Brittle automation failure
2. Output over outcomes
3. Local optimization over user value

## Revision Policy

1. Vision owner: the creator only.
2. Review cadence: every other day during the early phase.
3. Revision threshold: update only when strategic direction changes or repeated evidence shows a persistent mismatch.
