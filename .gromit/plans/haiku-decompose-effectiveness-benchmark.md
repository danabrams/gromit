---
created: 2026-02-28T00:00:00Z
decomposed: true
decomposed_at: "2026-02-28T03:56:01Z"
id: haiku-decompose-effectiveness-benchmark
source_spec: haiku-decompose-effectiveness-benchmark
---

# Haiku Decompose Effectiveness Benchmark Implementation Plan

**Goal:** Add a side-effect-safe benchmark command that compares `haiku` vs `sonnet` decomposition quality on a deterministic cohort of 5 completed specs and emits actionable recommendation artifacts.

**Architecture:** Implement a dedicated decompose-comparison benchmark flow under `gromit benchmark` that reuses shared decompose validation/complexity contracts in review mode, computes per-spec and aggregate deltas, and writes `raw.json` + `summary.md` artifacts.

**Tech Stack:** Go, Cobra CLI, existing `internal/pipeline` decompose workflow, existing `internal/benchmark` package patterns, `bd` tracker integration via bead/tracker adapters.

**Spec:** `.gromit/specs/haiku-decompose-effectiveness-benchmark.md`

---

## Architecture

### Overview

Add a benchmark subcommand that:
1. Selects exactly 5 completed specs deterministically (or validates explicit override list).
2. Runs decomposition twice per spec using identical inputs except tier (`haiku`/low vs `sonnet`/medium).
3. Forces review/simulation execution so tracker and plan state are never mutated.
4. Computes comparison metrics from shared decompose validation and complexity contracts.
5. Writes JSON and Markdown artifacts under `.gromit/benchmarks/results/decompose-haiku-vs-sonnet/<timestamp>/`.

### Key Components

1. **CLI Entrypoint (`cmd/gromit/benchmark_decompose_compare.go`)**
   - Register new `gromit benchmark decompose-compare` command.
   - Parse selection and recommendation-threshold flags.
   - Invoke benchmark orchestration and print artifact paths.

2. **Completed-Spec Cohort Selector (`internal/benchmark/decompose_cohort.go`)**
   - Resolve eligible specs where:
     - closed bead count for `spec:<name>` is `>= 1`
     - open bead count for `spec:<name>` is `0`
     - `.gromit/plans/<name>.md` exists
   - Default deterministic selection:
     - sort by closed count descending
     - tie-break by spec name ascending
     - first 5 only
   - Validate optional explicit override list.

3. **Decompose Compare Runner (`internal/benchmark/decompose_compare.go`)**
   - For each selected spec, call pipeline decompose twice with:
     - `Review=true`
     - `SkipValidation=false`
     - same retry/sub-bead limits and plan input
     - only `Tier` differs (`low` vs `medium`)
   - Capture raw run outputs and derived metrics.

4. **Benchmark Metrics Extractor (`internal/benchmark/decompose_metrics.go`)**
   - Compute:
     - bead count
     - per-bead and batch contract violations
     - complexity high-count and detailed reasons
     - acceptance-criteria totals and mean per bead
     - sibling-overlap hit count
     - runtime cost/latency/token signals if available
   - Reuse shared validators (`validate.ValidateDecomposeOutputWithMax`, `validate.ValidateDecomposeCandidates`) to avoid drift.

5. **Result/Recommendation Writer (`internal/benchmark/decompose_report.go`)**
   - Write:
     - `raw.json` with per-run payloads and metrics
     - `summary.md` with side-by-side per-spec rows and aggregate deltas
   - Evaluate threshold policy and set recommendation:
     - `haiku-acceptable`
     - `haiku-acceptable-with-guardrails`
     - `keep-sonnet-default`

6. **Review-Mode Benchmark Data Contract (pipeline/benchmark touchpoint)**
   - Ensure benchmark can access enough decomposition detail from review-mode output to compute required metrics (including acceptance criteria, expected outputs, dependencies, and complexity reasons) without enabling side effects.
   - If existing `DecomposeResult` is insufficient, extend it with a non-breaking proposed-bead detail payload used by the benchmark.

### Integration Points

- `cmd/gromit/benchmark.go`: command registration.
- `internal/pipeline/decompose.go`: review-mode invocation path and validation stats.
- `internal/validate/validate.go`: shared contract checks and complexity classification.
- `.gromit/benchmarks/results/`: benchmark artifact conventions.

### Data Flow

1. CLI resolves options and timestamp.
2. Cohort selector builds deterministic 5-spec set.
3. Runner executes `(spec, tier)` matrix (`5 x 2` runs).
4. Metrics extractor computes per-run quality/cost metrics.
5. Aggregator computes deltas per spec and across cohort.
6. Recommendation engine applies thresholds.
7. Writer emits `raw.json` and `summary.md` under timestamped directory.

### Tradeoffs

- **Reuse decompose pipeline path vs standalone prompt+validator path**
  - Choose pipeline path to keep benchmark behavior aligned with real decomposition.
- **Expose richer typed review output vs parsing logs/output text**
  - Prefer typed output fields for deterministic metrics; use runtime logs only for optional cost/latency signals.
- **Dedicated compare subcommand vs extending generic benchmark manifests**
  - Use dedicated command because this benchmark operates on plans/spec cohorts and decompose quality contracts rather than methodology run modes.

## Test Strategy

### Test Levels

1. **Unit Tests**
   - Cohort eligibility and deterministic ordering.
   - Metrics extraction and overlap/count calculations.
   - Recommendation threshold transitions.
   - Report rendering and artifact path correctness.

2. **Integration Tests**
   - CLI command executes full compare flow using fakes for tracker and pipeline.
   - Verifies exactly 10 runs (5 specs x 2 tiers), output writing, and recommendation rendering.

3. **Manual Validation**
   - Execute command in a real repo state.
   - Confirm side-effect safety (no new beads, no plan frontmatter updates).
   - Confirm artifact readability and threshold behavior.

### Key Test Cases

- Selection rejects cohorts that are not exactly 5 specs.
- Selection tie-breaker uses spec name ascending when closed counts match.
- Explicit spec override list is validated for completion rules and plan existence.
- Runner enforces identical non-tier decompose inputs across `haiku` and `sonnet`.
- Runner always forces review/simulation mode and leaves tracker/plan state unchanged.
- Metrics include both per-bead violations and batch contract violations.
- Complexity output includes high-complexity count plus title/reason details.
- Acceptance density fields (`total_criteria`, `mean_criteria_per_bead`) are correct.
- Sibling-overlap hits are counted from shared validation rules.
- Runtime signals are included when present and gracefully omitted when unavailable.
- Recommendation states map correctly to threshold conditions.
- Report contains required sections and aggregate deltas.

### Mocking Strategy

- Mock tracker/bead queries for label + status cohort eligibility.
- Mock decompose invocations with deterministic review outputs.
- Mock optional runtime signal records for cost/latency/tokens.
- Use temp filesystem dirs for artifact writes and golden-style markdown assertions.

### Coverage Goals

- Full coverage of deterministic cohort selection rules.
- Full coverage of recommendation decision matrix.
- High coverage for report schema and markdown contract.
- Regression tests for side-effect safety guarantees.

### Test Organization

- `cmd/gromit/benchmark_decompose_compare_test.go` for CLI wiring and argument validation.
- `internal/benchmark/decompose_cohort_test.go` for eligibility/selection logic.
- `internal/benchmark/decompose_metrics_test.go` for quality metric derivation.
- `internal/benchmark/decompose_report_test.go` for JSON/Markdown output contracts.
- `internal/benchmark/decompose_compare_test.go` for orchestration and side-effect-safe execution.

## Implementation Tasks

### Task 1: Add Decompose Compare CLI Surface

**Files:**
- Modify: `cmd/gromit/benchmark.go`
- Create: `cmd/gromit/benchmark_decompose_compare.go`
- Test: `cmd/gromit/benchmark_decompose_compare_test.go`

**What to Do:**
Add a new benchmark subcommand for decompose comparison, parse flags for optional spec overrides and threshold tuning, and delegate execution to internal benchmark orchestration with deterministic timestamp handling.

**Acceptance Criteria:**
- `gromit benchmark decompose-compare` command is registered and runnable.
- Command validates incompatible/invalid flags and reports clear errors.
- Command prints written artifact paths on success.

**Dependencies:**
- None

**Notes:**
- Keep command behavior parallel to existing benchmark command patterns.

### Task 2: Implement Completed-Spec Cohort Selection

**Files:**
- Create: `internal/benchmark/decompose_cohort.go`
- Test: `internal/benchmark/decompose_cohort_test.go`

**What to Do:**
Build cohort selection logic that scans spec labels, computes open/closed counts, confirms plan-file presence, and returns exactly 5 specs using deterministic sorting; support optional explicit list validation.

**Acceptance Criteria:**
- Eligibility strictly enforces completed-spec rules from the spec.
- Default selection deterministically returns 5 specs by required ordering.
- Explicit override list is accepted only when all entries pass validation.

**Dependencies:**
- Task 1

**Notes:**
- Keep label parsing robust (`spec:<name>`) and reject malformed names.

### Task 3: Expose Review-Mode Decompose Details Needed for Metrics

**Files:**
- Modify: `internal/pipeline/types.go`
- Modify: `internal/pipeline/decompose.go`
- Test: `internal/pipeline/decompose_test.go`

**What to Do:**
Ensure review-mode decomposition returns a typed proposed-bead payload sufficient for benchmark metric calculation (criteria/outputs/dependencies/estimates) without changing mutation behavior.

**Acceptance Criteria:**
- Review mode returns detailed proposed-bead fields required by benchmark metrics.
- Existing non-review decompose behavior and output contract remain compatible.
- Tests verify no side effects while exposing the new detail payload.

**Dependencies:**
- Task 1

**Notes:**
- Maintain backward compatibility for existing consumers of `DecomposeResult`.

### Task 4: Build Decompose Compare Orchestration and Metric Extraction

**Files:**
- Create: `internal/benchmark/decompose_compare.go`
- Create: `internal/benchmark/decompose_metrics.go`
- Test: `internal/benchmark/decompose_compare_test.go`
- Test: `internal/benchmark/decompose_metrics_test.go`

**What to Do:**
Implement matrix execution across selected specs and two tiers, run decompose in review mode with identical settings, and compute all required per-run quality metrics using shared validation contracts.

**Acceptance Criteria:**
- Benchmark executes exactly 10 runs for a 5-spec cohort.
- Every run uses review/simulation mode and does not create tracker items.
- Metrics include bead count, validation/batch violations, complexity, criteria density, and overlap signals.

**Dependencies:**
- Task 2
- Task 3

**Notes:**
- Keep model-tier as the only variable between paired runs.

### Task 5: Add Recommendation Engine and Artifact Writers

**Files:**
- Create: `internal/benchmark/decompose_report.go`
- Test: `internal/benchmark/decompose_report_test.go`

**What to Do:**
Implement aggregation, threshold evaluation, and artifact emission under `.gromit/benchmarks/results/decompose-haiku-vs-sonnet/<timestamp>/` with both raw machine-readable output and human summary markdown.

**Acceptance Criteria:**
- `raw.json` captures per-run outputs and computed metrics for all `(spec, model)` pairs.
- `summary.md` includes side-by-side rows, aggregate deltas, and final recommendation status.
- Recommendation is chosen from the 3 allowed statuses using explicit threshold checks.

**Dependencies:**
- Task 4

**Notes:**
- Ensure timestamped output directory naming is deterministic when timestamp override is provided.

### Task 6: Wire End-to-End Flow and Harden with Integration Tests

**Files:**
- Modify: `cmd/gromit/benchmark_decompose_compare.go`
- Modify: `cmd/gromit/benchmark_decompose_compare_test.go`
- Modify: `cmd/gromit/benchmark_test.go`

**What to Do:**
Connect command-layer execution to cohort selection, compare runner, and report writer; add end-to-end tests that assert final behavior, artifact paths, and recommendation output.

**Acceptance Criteria:**
- Single command completes full workflow for exactly 5 completed specs and both tiers.
- End-to-end tests verify side-effect-safe execution contract and artifact generation.
- Failure modes (insufficient cohort, invalid override, write failure) are covered with actionable errors.

**Dependencies:**
- Task 5

**Notes:**
- Keep test doubles minimal and aligned with existing benchmark command testing style.

---

## Notes

- This benchmark is analysis-only; all implementation choices must preserve side-effect safety by default.
- Reuse existing validation contracts as the quality source of truth to avoid benchmark/production drift.
- Runtime cost/latency/tokens are optional signals; missing data should not break report generation.
