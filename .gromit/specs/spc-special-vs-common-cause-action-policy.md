---
id: spc-special-vs-common-cause-action-policy
source_ideas: []
created: 2026-02-28
accepted: true
---

# SPC Special-vs-Common Cause Classification and Auto-Triage

## Specification

Gromit already computes SPC control limits, EWMA anomalies, and Nelson-rule signals for iteration metrics. This spec adds an explicit decision layer that classifies economic metric variation as either:

- **Special cause**: an out-of-control or abrupt signal likely tied to a specific incident (for example flaky tests, provider outage, or a bad prompt/template change).
- **Common cause**: in-control but persistently shifted process behavior that indicates systemic design or architecture constraints.

The goal is to reduce tampering by preventing one-off reactions to normal variation while still escalating genuine incidents quickly.

### Metrics in Scope

Classification and policy apply to:

- `rolling_avg_input_tokens`
- `rolling_avg_cost_usd`
- `rolling_avg_duration_ms`
- `rolling_avg_validation_ms`

### Classification Rules

For each in-scope metric, each trend update computes one classification:

1. `special_cause` when either condition is true:
   - Latest point breaches SPC limits (`detectAnomaly` or `EWMAAnomalies`) with `moderate` or `high` severity.
   - A Nelson-rule violation is active for that metric in the current window.
2. `common_cause` when both conditions are true:
   - No `special_cause` condition is active.
   - An unfavorable shift versus center-line baseline persists for at least two consecutive windows.
3. `stable` when neither `special_cause` nor `common_cause` criteria are met.

The classification output is part of process-trend artifacts and available to status/retro rendering.

### Anti-Tampering Action Policy

Policy is both advisory and automatic:

1. `special_cause` policy (incident-oriented):
   - Status/retro output recommends targeted investigation of the triggering phase/provider/model stratum.
   - Auto-create a `bd` issue of type `bug` when the same signal persists for 2 consecutive windows.
2. `common_cause` policy (system-oriented):
   - Status/retro output recommends system-level improvements (for example decomposition quality, test architecture, routing, prompt structure) rather than one-off tuning.
   - Auto-create a `bd` issue of type `task` when the same signal persists for 2 consecutive windows.
3. `stable` policy:
   - No issue is auto-created.
   - Status indicates the tracked economic metrics are in control.

### Auto-Triage Guardrails

To prevent issue spam, auto-creation uses strict dedupe:

- Signal identity key includes metric + classification + primary stratum (`global`, `provider:<name>`, or `model:<name>`).
- Cooldown is **7 days** per signal identity.
- If an open issue already exists for the same signal identity, do not create another.
- Auto-created issue bodies include metric evidence (latest, limits, recent window stats, first detected timestamp) and recommended next action.

### Integration Points

- Extend trend-generation output in `internal/logger` to emit explicit cause classification records for the four in-scope metrics.
- Extend status/retro display paths (`internal/runner/display`, `internal/retro`) to render classification and policy guidance.
- Add an auto-triage seam in runner/orchestration that can create `bd` issues using existing tracker integration patterns, with deterministic dedupe and cooldown checks.

### Out of Scope

- No hard stop/block on run-loop execution.
- No automatic config/prompt mutation.
- No expansion beyond the four scoped economic metrics in this spec.

## Acceptance Criteria

- Process trend output includes explicit classification (`special_cause`, `common_cause`, `stable`) for `rolling_avg_input_tokens`, `rolling_avg_cost_usd`, `rolling_avg_duration_ms`, and `rolling_avg_validation_ms`.
- A metric with out-of-control limit breach or Nelson-rule signal is classified `special_cause`.
- A metric with sustained adverse drift for 2 consecutive windows and no special-cause signal is classified `common_cause`.
- Status/retro output includes actionable guidance that differs by classification and explicitly warns against tampering on common-cause-only signals.
- Auto-triage creates `bug` for persisted special-cause signals (2 consecutive windows) and `task` for persisted common-cause signals (2 consecutive windows).
- Auto-triage dedupe prevents duplicate issue creation for the same signal identity within 7 days.
- If an open matching issue exists, no new issue is created.
- Evidence attached to each auto-created issue includes metric name, classification, latest value, control limits or drift evidence, and detection timestamp.

## Decisions

1. **Soft policy over hard enforcement.** The system provides advisory guidance plus automated triage, but does not block execution or auto-mutate config, minimizing operational risk.

2. **Persistence gate is 2 consecutive windows.** Single-window spikes are treated as insufficient for auto-triage to avoid noise and overreaction.

3. **Issue type split is semantic.** `bug` maps to special-cause incidents; `task` maps to common-cause systemic improvement work.

4. **Cooldown is 7 days with identity-based dedupe.** This constrains repeated issue creation while preserving visibility if a problem recurs after a meaningful interval.

5. **Validation duration is first-class economic signal.** Validation-time instability materially affects throughput and cost, so it is included with token/cost/time metrics.

## Research & Context

### Current State in Codebase

- SPC control limits, anomaly detection, EWMA, and Nelson-rule support already exist in `internal/logger/process_trend.go` and `internal/logger/trend_spc.go`.
- In-scope metric constants already exist for `rolling_avg_input_tokens`, `rolling_avg_cost_usd`, `rolling_avg_duration_ms`, and `rolling_avg_validation_ms`.
- Status already renders SPC summaries via `internal/runner/display/display.go` (`FormatSPCSummary`) and `internal/runner/print_status.go`.
- Existing specs establish related SPC groundwork:
  - `.gromit/specs/validation-duration-spc.md`
  - `.gromit/specs/phase-failure-rate-control-limits.md`
  - `.gromit/specs/process-trend-split.md`

### Gap This Spec Closes

The system currently detects anomalies but does not explicitly encode **cause class** or enforce a consistent **action policy**. This leaves operators to infer whether to treat variation as an incident or a systemic process property, increasing tampering risk. This spec formalizes that decision and automates triage with bounded noise.
