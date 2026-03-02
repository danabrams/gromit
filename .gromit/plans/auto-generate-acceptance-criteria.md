---
id: auto-generate-acceptance-criteria
source_spec: auto-generate-acceptance-criteria
created: 2026-03-02
decomposed: false
---

# Auto-Generate Acceptance Criteria Implementation Plan

**Goal:** Auto-generate acceptance criteria for beads with empty `ExpectedOutputs` at the gate stage, and surface missing-criteria visibility in `gromit status`.

**Architecture:** A new `CriteriaEnricher` interface on the Gate runs after precheck and before readiness. It loads spec/plan context, calls LLM (validation tier), writes criteria back via `bd update --acceptance`, and fails open on errors. Status reporting gets a `MissingCriteriaCount` field.

**Tech Stack:** Go, bd CLI (`--acceptance` flag on update), Claude LLM via `provider.Provider`

**Spec:** `.gromit/specs/auto-generate-acceptance-criteria.md`

---

## Architecture

### Overview

Add a `CriteriaEnricher` step to `prepare.Gate` that auto-generates acceptance criteria for beads with empty `ExpectedOutputs`. The enricher uses the richest available context (spec → plan → title+description), calls an LLM at `TierLow` (haiku), and persists generated criteria via `bd update <id> --acceptance`. A new config toggle `gate.auto_generate_criteria` (default true) controls the feature. Status reporting is extended with a missing-criteria count.

### Key Components

1. **`CriteriaEnricher` interface** (`internal/pipeline/prepare/gate.go`): New interface `Enrich(ctx, *bead.Bead) error` — distinct from `DataQualityBlocker` because enrichers mutate beads rather than blocking them.

2. **`LLMCriteriaEnricher` implementation** (`internal/pipeline/prepare/criteria_enricher.go`): Loads context via spec/plan files, builds prompt, calls LLM, parses 1–5 criteria from response, writes back via `bd update --acceptance`.

3. **`bead.Client.UpdateExpectedOutputs`** (`internal/bead/bead.go`): New method calling `bd update <id> --acceptance <joined-criteria>`.

4. **`GateConfig`** (`internal/config/config_types.go`): New config section with `AutoGenerateCriteria *bool` (defaults true via helper method).

5. **`MissingCriteriaCount`** (`internal/pipeline/status.go`): New field on `PipelineStatus`, populated by iterating ready beads and checking for empty `ExpectedOutputs`.

### Integration Points

- **Gate flow position**: After precheck, before readiness check — so readiness sees the generated criteria and doesn't block with `criteria_missing`
- **Builder wiring**: `Gate.WithCriteriaEnricher(e CriteriaEnricher)` follows existing `With*` pattern
- **Constructor wiring**: `internal/runner/constructor.go` creates and wires the enricher when config enables it
- **Config**: New `gate` section in YAML config, follows `ReadinessCheckConfig` pattern
- **Status**: `BeadQueryClient` interface extended or new query method added

### Data Flow

```
Bead enters gate → precheck
  → CriteriaEnricher:
      1. Check ExpectedOutputs — if populated, skip
      2. FindSpecLabel(bead.Labels) → spec name
      3. Load spec file (.gromit/specs/<name>.md), load plan file (.gromit/plans/<name>.md)
      4. Build prompt with richest context: spec+plan > spec > title+description
      5. provider.Run(ctx, prompt, TierLow) → parse 1-5 criteria
      6. bd update <id> --acceptance <criteria>
      7. Set bead.ExpectedOutputs in-memory
      (On any error: log warning, continue without criteria)
  → readiness check (now sees criteria) → stuck → data quality → scope → proceed
```

### Tradeoffs

- **New interface vs reusing `DataQualityBlocker`**: Chose new `CriteriaEnricher` — enrichers mutate (return error), blockers gate (return bool+reason+error). Different semantics.
- **Gate-level vs Pipeline method**: Gate-level because the enricher runs at a specific point in gate flow (before readiness).
- **Persistence via bd update**: Criteria survive retries and are visible in the tracker. `bd update --acceptance` is confirmed available.
- **Fail-open**: LLM failures log warnings but don't block the pipeline — avoids pipeline stalls on best-effort enrichment.

---

## Test Strategy

### Unit Tests
- Core enricher logic (skip, generate, fail-open, context resolution)
- Config parsing and defaults
- Status missing-criteria count
- `UpdateExpectedOutputs` method

### Integration Tests
- Gate flow with enricher wired in
- Constructor wiring based on config

### Key Test Cases
- Enricher skips beads with existing criteria
- Enricher generates criteria from spec context
- Enricher uses spec+plan when both available
- Enricher falls back to title+description when no spec
- Enricher fails open on LLM error (logs warning, bead proceeds)
- Enricher fails open on bd update error
- Enricher respects 1–5 criteria cap (truncates >5)
- Gate.Run with enricher: criteria generated before readiness check
- Gate.Run without enricher: existing behavior unchanged
- Config toggle: enabled (default) vs disabled
- UpdateExpectedOutputs calls bd with correct args
- Status MissingCriteriaCount counts correctly

### Mocking Strategy
- Mock LLM provider via interface for generation calls
- Mock bead client via interface for UpdateExpectedOutputs
- Mock spec/plan loading via interface or test fixtures
- Real config parsing (deterministic YAML deserialization)

### Test Organization
- `internal/pipeline/prepare/criteria_enricher_test.go`
- `internal/bead/bead_test.go` (UpdateExpectedOutputs tests)
- `internal/config/config_types_test.go` (GateConfig tests)
- `internal/pipeline/status_test.go` (MissingCriteriaCount tests)

---

## Implementation Tasks

### Task 1: Add GateConfig to config types

**Files:**
- Modify: `internal/config/config_types.go`
- Test: `internal/config/config_types_test.go`

**What to Do:**
Add a `GateConfig` struct with `AutoGenerateCriteria *bool` field and an `EffectiveAutoGenerateCriteria() bool` helper that defaults to `true` when nil. Add the struct as a `Gate GateConfig` field on the top-level `Config` struct. Follow the existing `ReadinessCheckConfig` pattern.

**Acceptance Criteria:**
- `GateConfig` struct exists with `AutoGenerateCriteria *bool` yaml tag `auto_generate_criteria`
- `EffectiveAutoGenerateCriteria()` returns `true` when nil (default), respects explicit `false`
- Config field wired on `Config` struct with `yaml:"gate"` tag

**Dependencies:** None

### Task 2: Add UpdateExpectedOutputs to bead client

**Files:**
- Modify: `internal/bead/bead.go`
- Test: `internal/bead/bead_test.go`

**What to Do:**
Add `UpdateExpectedOutputs(ctx context.Context, id string, criteria []string) error` method to `*Client`. It joins criteria with newlines and calls `bd update <id> --acceptance <joined>`. Follow the `UpdatePriority` pattern. Handle nil client and empty criteria edge cases.

**Acceptance Criteria:**
- `UpdateExpectedOutputs` calls `bd update <id> --acceptance <newline-joined criteria>`
- Returns error on nil client
- Handles empty criteria slice (no-op or clear)

**Dependencies:** None

### Task 3: Define CriteriaEnricher interface and wire into Gate

**Files:**
- Modify: `internal/pipeline/prepare/gate.go`
- Test: `internal/pipeline/prepare/gate_test.go`

**What to Do:**
Define `CriteriaEnricher` interface with `Enrich(ctx context.Context, b *bead.Bead) error`. Add `criteriaEnricher CriteriaEnricher` field to `Gate` struct. Add `WithCriteriaEnricher(e CriteriaEnricher) *Gate` builder method. Insert enricher call in `Run()` after precheck, before readiness check. On error, log warning and continue (fail-open). Only call if enricher is non-nil and bead has empty `ExpectedOutputs`.

**Acceptance Criteria:**
- `CriteriaEnricher` interface defined with `Enrich(ctx, *bead.Bead) error`
- Gate.Run calls enricher after precheck, before readiness — when bead has empty ExpectedOutputs
- Enricher errors log warning and don't block (fail-open)
- Gate without enricher wired behaves identically to current behavior

**Dependencies:** None

### Task 4: Implement LLMCriteriaEnricher

**Files:**
- Create: `internal/pipeline/prepare/criteria_enricher.go`
- Create: `internal/pipeline/prepare/criteria_enricher_test.go`

**What to Do:**
Implement the `CriteriaEnricher` interface. The enricher takes dependencies via constructor: an LLM provider interface (for `Run(ctx, prompt, tier)`), a spec loader interface (for loading spec/plan content), and a bead updater interface (for `UpdateExpectedOutputs`).

Context resolution logic:
1. `bead.FindSpecLabel(b.Labels)` → spec name
2. Load spec from `.gromit/specs/<name>.md` — if found, also try `.gromit/plans/<name>.md`
3. If no spec label, use bead title + description
4. Build prompt asking LLM for 1–5 concrete, testable acceptance criteria
5. Parse LLM response into string slice (split on numbered list or newlines)
6. Truncate to 5 if more
7. Call `UpdateExpectedOutputs(ctx, bead.ID, criteria)`
8. Update `bead.ExpectedOutputs` in-memory so downstream gate steps see criteria

**Acceptance Criteria:**
- Generates criteria from spec+plan context when available
- Falls back to spec-only, then title+description
- Parses LLM response into 1–5 criteria strings
- Writes criteria back via bead updater and sets in-memory
- Fails open on any error (LLM, spec load, bd update)

**Dependencies:** Task 2 (UpdateExpectedOutputs), Task 3 (CriteriaEnricher interface)

### Task 5: Wire enricher in constructor

**Files:**
- Modify: `internal/runner/constructor.go`

**What to Do:**
When `cfg.Gate.EffectiveAutoGenerateCriteria()` is true, create an `LLMCriteriaEnricher` with the provider, spec/plan paths, and bead client. Wire it into the gate via `gateStage.WithCriteriaEnricher(enricher)`. The provider is already available in the constructor. Spec/plan paths come from `cfg.Paths`.

**Acceptance Criteria:**
- Enricher wired into gate when config enabled (default)
- Enricher not wired when config explicitly disabled
- Uses existing provider and bead client from constructor scope

**Dependencies:** Task 1 (GateConfig), Task 3 (Gate.WithCriteriaEnricher), Task 4 (LLMCriteriaEnricher)

### Task 6: Add missing-criteria count to status

**Files:**
- Modify: `internal/pipeline/status.go`
- Test: `internal/pipeline/status_test.go`

**What to Do:**
Add `MissingCriteriaCount int` and `MissingCriteriaIDs []string` fields to `PipelineStatus`. In `ReadStatusWithDeps`, after getting ready beads, query for beads with empty `ExpectedOutputs`. This requires either extending `BeadQueryClient` with a `ListMissingCriteria` method, or iterating ready beads and filtering in-process. Include the count in `generateRecommendation` when non-zero. Support a `--missing-criteria` query flag for listing affected bead IDs.

**Acceptance Criteria:**
- `PipelineStatus.MissingCriteriaCount` populated correctly
- `MissingCriteriaIDs` lists IDs of beads missing criteria
- Status recommendation mentions missing criteria when count > 0

**Dependencies:** None

---

## Notes

- `bd update --acceptance <text>` is confirmed to exist — no CLI changes needed.
- The readiness check (`CheckCriteriaPresence`) currently blocks beads with empty criteria. With the enricher running before readiness, enriched beads will pass through. Beads that fail enrichment (LLM error) will still be blocked by readiness — this is acceptable and provides a safety net.
- The `from-review` bead fallback in `beadForReadinessAssessment` synthesizes criteria from title. The enricher should check for this case and potentially skip `from-review` beads that already have the title-based fallback, or let the enricher generate richer criteria. This is a detail to resolve during implementation.
- Prompt engineering for the LLM criteria generation call should emphasize concrete, testable criteria aligned with the existing max-5 validation constraint.
