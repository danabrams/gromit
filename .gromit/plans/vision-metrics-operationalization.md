---
id: vision-metrics-operationalization
source_spec: vision-metrics-operationalization
created: 2026-02-28
decomposed: false
---

# Vision Metrics Operationalization Implementation Plan

**Goal:** Establish a canonical, auditable PR-cycle measurement workflow for `VISION.md` outcomes with strict validation and reproducible KPI rollups.

**Architecture:** Introduce a dedicated `visionmetrics` domain package for cycle record contract, validation, storage, and rollup computation; expose it via a focused CLI command and document it as the canonical workflow.

**Tech Stack:** Go, Cobra CLI, JSONL repository artifact storage, existing gromit command/test patterns.

**Spec:** `.gromit/specs/vision-metrics-operationalization.md`

---

## Architecture

**Overview:**  
Capture standardized cycle records at PR presentation boundary, validate records before inclusion, and compute KPI rollups only from valid records. Keep this system separate from run-loop process trend metrics so denominator semantics stay correct and auditable.

**Key Components:**
1. **Cycle Record Contract (`internal/visionmetrics/contract.go`)**: typed model and constants for required fields and allowed values.
2. **Validation Engine (`internal/visionmetrics/validate.go`)**: rule-based compliance checks with field-level errors.
3. **Record Store (`internal/visionmetrics/store.go`)**: deterministic read/write helpers for canonical cycle record artifacts.
4. **Rollup Engine (`internal/visionmetrics/rollup.go`)**: deterministic KPI calculations from validated records.
5. **CLI Interface (`cmd/gromit/vision_metrics.go`)**: validation and reporting commands for operators and automation.
6. **Workflow Documentation (`README.md` or `docs/QUICKSTART.md`)**: reference the contract as the canonical `VISION.md` measurement process.

**Integration Points:**
- Integrate with existing spec/PR workflow boundaries (spec merge pipeline lifecycle) as the source event for cycle records.
- Keep PR-cycle metrics independent from `internal/logger/process_trend.go` iteration metrics.
- Allow downstream automation to consume JSON output from report command.

**Data Flow:**
1. A spec cycle reaches PR presentation.
2. A cycle record is persisted to the canonical dataset.
3. Validation checks contract completeness and logical consistency.
4. Invalid records are flagged and excluded from KPI inclusion until corrected.
5. Rollup computation outputs intervention, first-pass, escaped-regression, and accepted-without-rework rates from valid records.
6. Documentation directs contributors to this contract and workflow.

**Tradeoffs:**
- **Separate package vs extending process-trend logger:** avoids mixing iteration and cycle semantics.
- **Canonical stored records vs reconstructing from PR text:** improves reproducibility and auditability.
- **Dedicated command vs implicit stats coupling:** clearer operator behavior and easier adoption.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** contract parsing/constants, validation rules, KPI formula behavior.
2. **Integration Tests:** store round-trip and end-to-end rollup behavior from mixed record sets.
3. **CLI Tests:** command output/exit behavior for validation failures and report generation.
4. **Documentation Guard:** verify at least one workflow doc references this contract as canonical.

**Key Test Cases:**
- Required fields missing produce explicit validation failures.
- Invalid enum/yes-no values are rejected.
- `human_debugging_intervention=yes` with `human_tactical_intervention=no` is rejected.
- `review_outcome=rework_vision_change` without `review_rationale` is rejected.
- Accepted-without-rework denominator excludes `rework_vision_change`.
- Escaped regression rate computes from resolved yes/no records and remains reproducible.
- Report output includes numerators, denominators, and rate values for all KPIs.

**Mocking Strategy:**
- Use real temp filesystem for store tests.
- Use table-driven fixtures for validation/rollup unit tests.
- Keep CLI tests self-contained without network dependencies.

**Coverage Goals:**
- Full branch coverage for validation rule paths.
- Positive and edge-case tests for each KPI formula.
- Deterministic JSON/text output checks for CLI.

## Implementation Tasks

### Task 1: Define cycle record contract types

**Files:**
- Create: `internal/visionmetrics/contract.go`
- Test: `internal/visionmetrics/contract_test.go`

**What to Do:**
Define the canonical cycle record struct, required field names, and allowed value constants (`review_outcome`, intervention yes/no domains). Include small helpers for normalization and domain checks.

**Acceptance Criteria:**
- Contract type includes every field required by the spec.
- Allowed values are centrally defined and reusable by validator/report logic.
- Contract tests verify allowed domains and field wiring.

**Dependencies:**
- None.

### Task 2: Implement validation rules for cycle records

**Files:**
- Create: `internal/visionmetrics/validate.go`
- Test: `internal/visionmetrics/validate_test.go`

**What to Do:**
Implement record validation with explicit failures for missing required fields, invalid domains, debugging subset violations, and missing rationale for `rework_vision_change`.

**Acceptance Criteria:**
- Validation reports actionable field-level errors.
- All semantic constraints in the spec are enforced.
- Validation result clearly indicates record eligibility for metric inclusion.

**Dependencies:**
- Task 1.

### Task 3: Add canonical record store read/write support

**Files:**
- Create: `internal/visionmetrics/store.go`
- Test: `internal/visionmetrics/store_test.go`

**What to Do:**
Implement deterministic JSONL persistence for cycle records (load all, append/update strategy, stable decode behavior, robust error handling).

**Acceptance Criteria:**
- Records can be written and read back without information loss.
- Malformed lines/files fail with clear errors.
- Store behavior is deterministic and testable.

**Dependencies:**
- Task 1.

### Task 4: Implement KPI rollup computation

**Files:**
- Create: `internal/visionmetrics/rollup.go`
- Test: `internal/visionmetrics/rollup_test.go`

**What to Do:**
Compute tactical intervention, debugging intervention, first integration pass, escaped regression, and accepted-without-rework rates from validated records with explicit numerator/denominator counts.

**Acceptance Criteria:**
- All five KPI formulas match spec definitions.
- Accepted-without-rework excludes `rework_vision_change` from denominator penalty.
- Rollup output includes transparent counts and derived rates.

**Dependencies:**
- Task 2.
- Task 3.

### Task 5: Add CLI command for validation and reporting

**Files:**
- Create: `cmd/gromit/vision_metrics.go`
- Modify: `cmd/gromit/main.go`
- Test: `cmd/gromit/vision_metrics_test.go`

**What to Do:**
Add `gromit vision-metrics` command group with subcommands for record validation and KPI reporting (`--json` support included).

**Acceptance Criteria:**
- Command can validate stored records and surface compliance failures.
- Command can output full KPI rollup in text and JSON modes.
- Command wiring is discoverable in root help and tests.

**Dependencies:**
- Task 2.
- Task 3.
- Task 4.

### Task 6: Hook PR-cycle boundary record capture path

**Files:**
- Modify: `internal/runner/specmerge/pipeline.go`
- Create: `internal/runner/specmerge/vision_metrics_adapter.go`
- Test: `internal/runner/specmerge/pipeline_test.go`

**What to Do:**
Add a lightweight integration seam so spec PR presentation/review events can emit/update cycle records via `visionmetrics` package without coupling pipeline internals to storage details.

**Acceptance Criteria:**
- PR presentation boundary can produce traceable cycle records keyed by spec/PR context.
- Integration is isolated behind adapter-style interface for testability.
- Existing pipeline behavior remains intact when vision-metrics capture is disabled/unavailable.

**Dependencies:**
- Task 1.
- Task 2.
- Task 3.

### Task 7: Document canonical workflow reference

**Files:**
- Modify: `README.md` (or `docs/QUICKSTART.md`)
- Test: `doc_conventions_test.go` (or a new focused documentation assertion test)

**What to Do:**
Add a workflow section that defines the cycle record contract path and reporting command as the canonical `VISION.md` measurement process.

**Acceptance Criteria:**
- At least one repository workflow doc references this contract as canonical.
- Documentation includes where records live, how validation runs, and how rollups are generated.
- Doc tests remain green.

**Dependencies:**
- Task 5.

### Task 8: End-to-end acceptance coverage for compliance and rollups

**Files:**
- Create: `internal/visionmetrics/acceptance_test.go`
- Possibly modify: `validation_test.go` or existing acceptance harness registration file

**What to Do:**
Add an end-to-end fixture-driven test that loads sample records, validates them, and verifies final rollup outputs including carve-out behavior and exclusion logic.

**Acceptance Criteria:**
- End-to-end test demonstrates deterministic reproduction of all KPI metrics.
- Invalid records are excluded until corrected.
- Carve-out (`rework_vision_change`) remains visible for audit while excluded from accepted-without-rework penalty denominator.

**Dependencies:**
- Task 2.
- Task 3.
- Task 4.
- Task 5.

---

## Notes

- Keep this implementation contract-focused and storage-agnostic enough to evolve format later without changing KPI semantics.
- Avoid coupling PR-cycle vision metrics to existing iteration SPC metrics; they serve different decision layers.
- Prefer explicit counts in report output (not rates only) to preserve auditability and retrospective debugging.
