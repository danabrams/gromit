---
id: vision-metrics-operationalization
source_ideas: []
created: 2026-02-28
---

# Vision Metrics Operationalization

## Specification

Establish a measurable, auditable workflow for tracking the `VISION.md` two-year outcomes across spec execution cycles. The system must capture standardized review and intervention metadata at PR time, validate required fields automatically, and produce rollup metrics that reflect the vision targets.

### Scope

This spec defines behavior for:

- Recording cycle metadata at PR presentation/review time
- Enforcing required metadata completeness and value validity
- Computing the three vision KPIs from recorded cycle data
- Handling explicit carve-outs for vision-change rework

This spec does not define implementation architecture, storage schema internals, or UI design.

### Cycle Record Contract

Each spec execution cycle must have a structured record containing:

- `spec_id`
- `cycle_start_trigger_at`
- `cycle_end_presented_at`
- `review_outcome` with allowed values:
  - `accepted`
  - `rework_implementation_gap`
  - `rework_vision_change`
- `review_rationale` (required when outcome is `rework_vision_change`)
- `human_tactical_intervention` (`yes` or `no`)
- `human_debugging_intervention` (`yes` or `no`)
- `escaped_regression_within_7d` (`yes` or `no`, resolved after observation window)

The record must be associated with the PR presented for product-owner subjective review and traceable to the spec cycle.

### Validation Behavior

Before a PR can be considered valid for presentation/review tracking:

- Required cycle fields must be present.
- Field values must match the allowed enum/boolean domain.
- `human_debugging_intervention=yes` implies `human_tactical_intervention=yes`.
- `review_rationale` is mandatory when `review_outcome=rework_vision_change`.
- Invalid or incomplete records are treated as failed compliance and must be corrected before metric inclusion.

### KPI Computation Behavior

From valid cycle records, compute:

1. **Human tactical intervention rate**
   - Numerator: cycles with `human_tactical_intervention=yes`
   - Denominator: all cycles reaching PR presentation

2. **Human debugging intervention rate**
   - Numerator: cycles with `human_debugging_intervention=yes`
   - Denominator: all cycles reaching PR presentation

3. **First integration pass rate**
   - Numerator: cycles that pass objective gates on first presentation without follow-up corrective commits
   - Denominator: all cycles reaching PR presentation

4. **Escaped regression rate (7-day window)**
   - Numerator: cycles with `escaped_regression_within_7d=yes`
   - Denominator: all cycles reaching PR presentation

5. **Accepted-without-rework rate**
   - Numerator: cycles with outcome `accepted`
   - Denominator: cycles reaching PR presentation, excluding those labeled `rework_vision_change`

All rates must be reproducible from stored cycle records.

### Carve-Out Semantics

`rework_vision_change` is an auditable carve-out for product-owner mind-change or clarification after presentation:

- It is excluded from accepted-without-rework penalty calculations.
- It must include a rationale note.
- It remains visible in reporting for transparency.

## Acceptance Criteria

- A documented cycle record contract exists with required fields and allowed values, including the three review outcomes.
- Validation rejects incomplete or invalid records, including missing rationale for `rework_vision_change`.
- Validation enforces subset consistency: debugging intervention cannot be true when tactical intervention is false.
- Reporting computes intervention, first-pass, escaped-regression, and accepted-without-rework rates from recorded cycle data.
- Accepted-without-rework reporting excludes `rework_vision_change` from denominator penalty while preserving audit visibility.
- At least one repository workflow document references this contract as the canonical measurement process for `VISION.md` outcomes.

## Decisions

1. **PR-linked cycle records as the measurement boundary** The PR presentation event is the operational checkpoint closest to product-owner review and supports durable audit trails.

2. **Strict field validation before inclusion** Metrics are only useful if record quality is enforced; incomplete records are excluded until corrected.

3. **Explicit vision-change carve-out label** Carve-outs are permitted but must be intentional and justified to prevent silent metric gaming.

4. **Derived metrics from canonical fields** KPI definitions are computed from raw cycle records to keep calculations deterministic and reviewable.

## Research & Context

### Current State

- `VISION.md` defines outcome targets and metric definitions, but no repository-wide operational contract yet enforces recording and validation.
- Existing artifacts (`README.md`, `LEARNINGS.md`, and specs in `.gromit/specs/`) provide process guidance but do not yet standardize cycle metric capture.

### Relevant Files

- `VISION.md` — defines targets, definitions, and carve-out policy
- `README.md` — likely location for operator workflow reference
- `.gromit/specs/` — refinement/planning artifacts that should align with vision measurement

### Problem Framing

Without a standardized record and validation flow, vision metrics become subjective and drift over time. A defined operational contract makes progress measurable, comparable across cycles, and resistant to retrospective reinterpretation.
