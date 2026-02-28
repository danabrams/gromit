---
id: vision-metrics-pr-template-and-ci-enforcement
source_ideas: []
created: 2026-02-28
---

# Vision Metrics PR Template and CI Enforcement

## Specification

Define a standardized PR metadata capture mechanism and automated compliance checks so every spec cycle that reaches product-owner presentation includes complete `VISION.md` measurement data.

### PR Template Contract

Repository pull requests used for spec-cycle presentation must include a required section for cycle metrics metadata with these fields:

- `spec_id`
- `cycle_start_trigger_at`
- `cycle_end_presented_at`
- `review_outcome` (`accepted|rework_implementation_gap|rework_vision_change`)
- `review_rationale` (required only for `rework_vision_change`)
- `human_tactical_intervention` (`yes|no`)
- `human_debugging_intervention` (`yes|no`)
- `escaped_regression_within_7d` (`yes|no|pending`)

The template must make required/conditional fields explicit and provide unambiguous allowed values.

### CI Validation Behavior

An automated check validates PR metadata on pull request updates and enforces:

- Required fields are present and non-empty.
- Enum/boolean values are valid.
- `human_debugging_intervention=yes` implies `human_tactical_intervention=yes`.
- `review_rationale` exists when `review_outcome=rework_vision_change`.
- `escaped_regression_within_7d` accepts `pending` before 7-day window closure and requires final resolution workflow.

Validation failures must produce actionable error messages identifying the field and expected format/value domain.

### State Transition Semantics

Because escaped regression requires a 7-day window:

- At presentation/review time, `escaped_regression_within_7d` may be `pending`.
- A follow-up update path must exist to resolve `pending` to `yes` or `no`.
- Metrics rollups must exclude unresolved `pending` records from escaped-regression numerator/denominator until resolved, while surfacing pending count.

### Workflow Integration

The PR metadata contract and CI check behavior must be referenced from repository workflow documentation so contributors know:

- where to provide cycle metadata,
- when fields can be pending,
- and how to finalize post-window fields.

## Acceptance Criteria

- A repository PR template includes all required vision-metric fields with allowed values documented inline.
- CI rejects PRs with missing/invalid required fields and reports actionable errors.
- CI enforces conditional rules (`review_rationale` for `rework_vision_change`, debugging subset rule).
- The workflow supports `escaped_regression_within_7d=pending` at presentation and a documented path to resolve it.
- Reporting logic distinguishes resolved vs pending escaped-regression records and excludes pending records from escaped-regression rate calculations.
- At least one workflow document points contributors to the PR metadata contract and CI expectations.

## Decisions

1. **PR template as the primary capture point** It is closest to product-owner presentation and already part of review workflow.

2. **CI-enforced metadata quality** Manual process alone is too easy to bypass; automated gates keep records consistent.

3. **Explicit `pending` for delayed signals** Escaped regression cannot be known at presentation time, so the contract must encode delayed resolution safely.

4. **Actionable validation feedback** Fast correction depends on clear, field-specific CI errors.

## Research & Context

### Current State

- `VISION.md` defines metrics and definitions, and `vision-metrics-operationalization.md` defines semantic contract.
- The repository does not yet specify a concrete PR-template field block and CI validation behavior for these metrics.

### Related Specs

- `.gromit/specs/constancy-of-purpose-vision.md`
- `.gromit/specs/vision-metrics-operationalization.md`

### Problem Framing

Without a concrete capture and enforcement mechanism, semantic metric definitions will not produce reliable data. A PR template plus CI validation closes that execution gap.
