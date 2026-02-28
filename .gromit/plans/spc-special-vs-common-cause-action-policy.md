---
id: spc-special-vs-common-cause-action-policy
source_spec: spc-special-vs-common-cause-action-policy
created: 2026-02-28
decomposed: false
---

# SPC Special-vs-Common Cause Action Policy Implementation Plan

**Goal:** Add explicit SPC cause classification for key economic metrics, render anti-tampering guidance, and auto-triage sustained signals into deduplicated tracker issues.

**Architecture:** Extend process trend generation with classification records and evidence, surface policy-aware guidance in status/retro output, and run a post-loop auto-triage pass with persistence, open-issue dedupe, and 7-day cooldown guardrails.

**Tech Stack:** Go (`internal/logger`, `internal/runner`, `internal/retro`, `internal/state`), existing tracker client abstraction (`bd` backend), JSON process trend/state artifacts.

**Spec:** `.gromit/specs/spc-special-vs-common-cause-action-policy.md`

---

## Architecture

**Overview:**  
Add an explicit SPC cause-classification layer to `ProcessTrend`, render it in status/retro, and run a post-loop auto-triage pass in the orchestrator that creates deduplicated tracker issues when persisted signals cross the 2-window gate.

**Key Components:**
1. **`internal/logger` classification model + evaluator**: compute `special_cause|common_cause|stable` for each scoped economic metric, globally and per stratum.
2. **`internal/logger.ProcessTrend` schema extension**: store classification records plus evidence needed by status/retro and triage.
3. **`internal/runner/display` SPC guidance renderer**: show per-metric classification and policy guidance, including anti-tampering messaging for common-cause-only signals.
4. **`internal/retro` prompt enrichment**: expose cause-classification and policy guidance in the retro prompt sections.
5. **`internal/runner` auto-triage service**: post-run evaluator that applies persistence/cooldown/open-issue dedupe and creates `bug`/`task` issues via tracker integration.
6. **`internal/state` cooldown persistence**: keep last-created timestamps per signal identity to enforce the 7-day cooldown deterministically.

**Integration Points:**
- Extend `buildProcessTrend` in `internal/logger/process_trend.go` to produce classification artifacts after control-limit/EWMA/Nelson computations.
- Add classification rendering in `internal/runner/display/display.go` near existing SPC sections.
- Add retro prompt sections in `.gromit/templates/PROMPT_retro.md` using new trend fields.
- Invoke auto-triage near existing post-run SPC check in `internal/runner/orchestrator.go`.
- Wire service dependencies in `internal/runner/constructor.go`.

**Data Flow:**
1. Iteration logs -> `buildProcessTrend` computes control limits, anomalies, EWMA, Nelson violations.
2. New classifier maps those signals plus sustained adverse drift (2 windows) to cause class per metric and stratum.
3. `process_trend.json` includes classification records and evidence.
4. `status` and `retro` render these records with policy-specific recommendations.
5. End-of-run orchestrator reads latest trend classification records, filters persisted (`>=2 windows`) non-stable signals, checks:
   - open issue with same signal identity label,
   - cooldown timestamp in state (`7 days`),
   then creates tracker issue (`bug` for special, `task` for common) with evidence payload and recommended action.
6. On creation, state stores `last_created_at` by identity key.

**Files to Modify:**
- `internal/logger/process_trend.go` - new trend fields, nil normalization, classification invocation.
- `internal/logger/trend_spc.go` - helper reuse for severity/metric matching where needed.
- `internal/runner/display/display.go` - classification/policy output.
- `.gromit/templates/PROMPT_retro.md` - retro classification guidance section.
- `internal/runner/orchestrator.go` - post-run auto-triage call.
- `internal/runner/constructor.go` - wire triage service.
- `internal/state/state.go` - cooldown map plus accessors.

**Files to Create:**
- `internal/logger/trend_cause_classification.go` - classification types, scoped metric set, evaluator logic.
- `internal/logger/trend_cause_classification_test.go` - classification rule tests (special/common/stable, drift persistence, strata identity).
- `internal/runner/spc_auto_triage.go` - auto-create/dedupe/cooldown service.
- `internal/runner/spc_auto_triage_test.go` - triage logic tests including open-issue and cooldown suppression.

**Tradeoffs:**
- Keep classification in `logger` (data-production layer) instead of display/runner so one source of truth feeds status, retro, and automation.
- Persist cooldown in `state.json` instead of inferring from tracker timestamps; tracker item model does not reliably expose creation times across backends.
- Use deterministic identity labels (`spc-signal:<hash-or-key>`) for open-issue dedupe instead of fuzzy title/description matching.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: classifier and triage decision logic (no I/O).
2. **Integration Tests**: process trend generation and status/retro rendering include new fields/sections.
3. **Manual Testing**: run status/retro against sample metrics to verify guidance wording and low-noise issue behavior.

**Key Test Cases:**
- Special-cause classification when control-limit anomaly severity is `moderate` or `high`.
- Special-cause classification when Nelson-rule violation exists (even without anomaly).
- Common-cause classification when adverse drift persists 2 consecutive windows with no special signal.
- Stable classification when neither condition applies.
- Classification emitted for all 4 scoped metrics and stratified identities.
- Auto-triage creates `bug` for persisted special-cause and `task` for persisted common-cause.
- Auto-triage skips creation when an open matching issue exists.
- Auto-triage skips creation within 7-day cooldown for the same identity.
- Auto-triage issue body includes required evidence fields.

**Mocking Strategy:**
- Mock `tracker.Client` in runner triage tests (existing runner pattern).
- Use temp state files or in-memory seams for cooldown persistence tests.
- Keep logger classification tests pure with synthetic metric windows.

**Coverage Goals:**
- Critical path: classification correctness plus triage dedupe/cooldown semantics.
- Edge cases: missing strata, nil/empty trend fields, boundary at exactly 2 windows, cooldown expiry boundary at 7 days.

**Test Organization:**
- `internal/logger/trend_cause_classification_test.go`
- `internal/logger/process_trend_test.go` (extend existing trend output assertions)
- `internal/runner/display/display_test.go` (SPC guidance formatting assertions)
- `internal/runner/spc_auto_triage_test.go`
- `internal/state/state_test.go` (cooldown map round-trip and nil normalization)

## Implementation Tasks

### Task 1: Add Cause-Classification Domain Model in Logger

**Files:**
- Create: `internal/logger/trend_cause_classification.go`
- Test: `internal/logger/trend_cause_classification_test.go`

**What to Do:**
Define cause-classification enums, scoped metric metadata, identity construction (`metric + classification + stratum`), and pure evaluation logic that maps anomaly/Nelson/drift signals to `special_cause`, `common_cause`, or `stable`.

**Acceptance Criteria:**
- Classification logic supports all four scoped metrics and both global and stratified identities.
- `special_cause` is emitted when moderate/high anomaly or Nelson signal is present for a metric.
- `common_cause` requires no special-cause signal and 2 consecutive unfavorable drift windows.

**Dependencies:**
- None.

**Notes:**
- Keep this package-level logic side-effect free so it is directly reusable by trend generation and triage.

### Task 2: Extend ProcessTrend Artifacts With Classification Records

**Files:**
- Modify: `internal/logger/process_trend.go`
- Modify: `internal/logger/trend_spc.go`
- Test: `internal/logger/process_trend_test.go`

**What to Do:**
Add classification slices/maps to `ProcessTrend`, compute them inside `buildProcessTrend`, include evidence fields (latest, baseline/limits, run persistence indicators, first-detected timestamp where derivable), and normalize nil fields for stable JSON output.

**Acceptance Criteria:**
- `process_trend.json` includes classification records for all four scoped economic metrics.
- Existing SPC anomaly/pattern behavior remains intact while classifications are added deterministically.
- Nil normalization includes all new fields and maintains empty collection invariants.

**Dependencies:**
- Task 1.

**Notes:**
- Preserve existing sort behavior for deterministic snapshots/tests.

### Task 3: Render Classification and Anti-Tampering Guidance in Status and Retro

**Files:**
- Modify: `internal/runner/display/display.go`
- Modify: `.gromit/templates/PROMPT_retro.md`
- Test: `internal/runner/display/display_test.go`
- Test: `internal/retro/retro_test.go`

**What to Do:**
Update status SPC formatting and retro prompt process-trend sections to include cause class and class-specific guidance. Ensure `common_cause` messaging explicitly warns against one-off tampering and recommends system-level interventions.

**Acceptance Criteria:**
- Status output includes classification and distinct guidance for special/common/stable.
- Retro prompt includes classification evidence and policy text for downstream analysis.
- Existing output sections remain readable and deterministic for tests.

**Dependencies:**
- Task 2.

**Notes:**
- Keep guidance concise and action-oriented to avoid overwhelming status output.

### Task 4: Implement Auto-Triage Service With Dedupe and Cooldown

**Files:**
- Create: `internal/runner/spc_auto_triage.go`
- Test: `internal/runner/spc_auto_triage_test.go`
- Modify: `internal/state/state.go`
- Test: `internal/state/state_test.go`

**What to Do:**
Implement a runner service that evaluates latest classification records post-run and creates tracker issues when non-stable signals persist for 2 windows. Add state-backed cooldown bookkeeping keyed by signal identity and checker logic for existing open matching issues.

**Acceptance Criteria:**
- Persisted special-cause signal creates tracker `bug`; persisted common-cause signal creates tracker `task`.
- No issue is created when open matching issue exists or cooldown has not expired.
- Created issue includes required evidence and recommended next action text.

**Dependencies:**
- Task 2.

**Notes:**
- Use deterministic labels/metadata to support strict dedupe across reruns/backends.

### Task 5: Wire Auto-Triage Into Runner Lifecycle

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/runner/constructor.go`
- Test: `internal/runner/orchestrator_test.go`

**What to Do:**
Add auto-triage invocation to the orchestrator post-run sequence (near existing SPC control-limit alert checks), inject dependencies through constructor wiring, and ensure failures are surfaced as warnings without breaking run completion.

**Acceptance Criteria:**
- Auto-triage is invoked exactly once at run completion when dependencies are configured.
- Runner remains resilient: triage failures do not crash the run loop completion path.
- Wiring tests verify service is attached and callable in real constructor paths.

**Dependencies:**
- Task 4.

**Notes:**
- Keep this as soft-policy behavior; do not block execution.

### Task 6: End-to-End Regression Coverage for Spec Acceptance

**Files:**
- Modify: `internal/logger/process_trend_test.go`
- Modify: `internal/runner/display/display_test.go`
- Modify: `internal/retro/retro_test.go`
- Modify: `internal/runner/spc_auto_triage_test.go`

**What to Do:**
Add/extend tests to validate all acceptance criteria end-to-end: classification correctness, differentiated guidance, issue-type mapping, persistence gate, dedupe, cooldown, and evidence payload expectations.

**Acceptance Criteria:**
- Tests explicitly cover each acceptance criterion from the source spec.
- Regression suite remains deterministic and does not require live tracker/network.
- Failure messages clearly identify mismatched policy behavior.

**Dependencies:**
- Tasks 1-5.

**Notes:**
- Prefer table-driven cases for classification permutations and dedupe branches.

---

## Notes

- This plan intentionally keeps enforcement advisory plus auto-triage only; no run-blocking or auto-mutation logic.
- Signal identity design should remain backend-agnostic and deterministic to keep dedupe behavior stable.
- `rolling_avg_validation_ms` is in-scope and should be treated as first-class economic behavior in both classification and guidance.
