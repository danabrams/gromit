---
id: methodology-benchmark-harness
source_spec: methodology-benchmark-harness
created: 2026-02-23
decomposed: false
---

# Methodology Benchmark Harness Implementation Plan

**Goal:** Add a single-command benchmark harness that runs the same bead cohort across three methodology modes from one base commit and emits deterministic JSON/Markdown comparison reports.

**Architecture:** Introduce a new `gromit benchmark run` workflow backed by an `internal/benchmark` package for manifest parsing, cohort selection (`--beads`, `--bead-count`), deterministic mode execution in isolated worktrees, metrics aggregation from JSONL logs, and report rendering.

**Tech Stack:** Go, Cobra CLI, YAML (`gopkg.in/yaml.v3`), existing runner/logger/worktree infrastructure

**Spec:** `.gromit/specs/methodology-benchmark-harness.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add a new benchmark command/workflow that reads a manifest, validates a fixed cohort, runs three isolated mode executions from the same base commit, then emits deterministic JSON+Markdown comparison reports.

**Key Components:**
1. **Benchmark CLI command**: add a `benchmark` command group to load/run a manifest and write artifacts.
2. **Manifest + validation layer**: parse `.gromit/benchmarks/*.yaml`, validate bead/mode/provider/model constraints, and verify bead openness + complexity-tier coverage.
3. **Deterministic run harness**: execute three runs (`single_pass`, `tdd_shared_context`, `tdd_fresh_context`) in isolated worktrees pinned to one base commit.
4. **Sequence runner adapter**: add a runner/orchestrator entrypoint that consumes an explicit bead ID sequence (manifest order) so each mode runs the exact same selected beads in the same order.
5. **Mode config overlay builder**: generate temporary per-mode config overrides for methodology toggles and provider/model-family pinning while keeping all other config identical.
6. **Metrics aggregator**: compute by-tier token/cost totals (`actual_tier`), overall totals, elapsed time, first-pass rate, average quality score, review fixes/findings, and post-review validation status from JSONL logs.
7. **Report renderer**: write deterministic outputs to `.gromit/benchmarks/results/<id>/<timestamp>.json` and `.md` with stable sorting and winner hints.

**Integration Points:**
- Reuse config + routing model in `internal/config/config_types.go`.
- Reuse iteration/review log schema in `internal/logger/logger.go`.
- Reuse worktree/session mechanics in `internal/worktree/worktree.go`.
- Extend runner orchestration surface in `internal/runner/orchestrator.go` for explicit bead-sequence execution.

**Data Flow:**
1. Load manifest and validate candidate beads.
2. Resolve selected cohort via CLI + manifest (`--beads`, `--bead-count`).
3. Resolve base commit once.
4. For each mode: create isolated worktree from base commit.
5. Materialize config overlay (`build_strategy`, `fresh_context_per_cycle`, tier model pins, low-tier build/validation, high-tier final review).
6. Run sequence runner on exact selected bead order.
7. Collect run start/end timestamps and parse logs.
8. Aggregate mode metrics and cleanup worktree.
9. Emit consolidated JSON+MD report with normalized winner hints.

**Files to Modify:**
- `cmd/gromit/main.go` (register benchmark command)
- `internal/runner/...` (explicit sequence-run capability without changing default `run` behavior)
- `internal/config/...` (only if benchmark-scoped hooks are required)

**Files to Create:**
- `.gromit/benchmarks/tdd-vs-single-pass.yaml` (sample manifest)
- `cmd/gromit/benchmark.go`
- `internal/benchmark/manifest.go`
- `internal/benchmark/selection.go`
- `internal/benchmark/validate.go`
- `internal/benchmark/harness.go`
- `internal/benchmark/worktree_run.go`
- `internal/benchmark/metrics.go`
- `internal/benchmark/report.go`
- `internal/benchmark/*_test.go`
- `cmd/gromit/benchmark_test.go`

**Tradeoffs:**
- Explicit bead-sequence runner over label-filtered `bd ready`: chosen for deterministic cohort + order guarantees.
- Worktree-per-mode over in-place branch reset: chosen for stronger isolation and easier failure handling.
- Tier-based reporting over model-name reporting: aligns with experiment question and acceptance criteria.
- Harness-level elapsed timing over inferred log timing: avoids ambiguity in wall-clock comparisons.

**Approved adjustment:**
- Cohort size is configurable via CLI.
- Add `--beads` to explicitly choose bead IDs and order.
- Add `--bead-count` to truncate selected list deterministically.

---

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit Tests**
- Manifest parsing/validation for variable cohort sizes
- Cohort resolution logic (`--beads`, `--bead-count`, manifest fallback)
- Tier coverage checks on selected subset
- Mode overlay generation (only methodology/fresh-context differ)
- Metrics rollups by `actual_tier` and overall totals
- Winner hint scoring and stable tie-breaking

2. **Integration Tests**
- Benchmark run with fake runner/worktree/log inputs across 3 modes
- Same base commit and same selected bead order enforced per mode
- Output artifacts written to expected JSON/MD paths
- Deterministic output across repeated runs with fixed fixtures

3. **CLI Tests**
- `gromit benchmark run ... --bead-count N`
- `gromit benchmark run ... --beads ...`
- Combined flags precedence and error cases
- Helpful failures for invalid counts, unknown bead IDs, insufficient tier diversity

4. **Manual/Smoke**
- One real dry run against local repo with small cohort
- Verify elapsed time, cost/tokens, review metrics, and final validation fields look sane

**Key Test Cases:**
- `--beads` fully overrides manifest beads
- `--bead-count` truncates selected ordered list deterministically
- `--bead-count` larger than available fails clearly
- Selected subset missing low/medium/high fails validation
- Mode config diffs contain only `build_strategy` and `fresh_context_per_cycle`
- All three modes use identical `selected_beads` and `base_commit`
- Tier totals (`low|medium|high`) + overall totals match fixture expectations
- Review findings/fixes and post-review validation status aggregate correctly
- JSON + Markdown report sorting is stable across repeated runs

**Mocking Strategy:**
- Mock bead client for existence/open/labels
- Mock worktree manager and git commit resolution
- Mock runner execution and inject fixture JSONL logs
- Keep metrics/report logic pure and file-I/O minimal with `t.TempDir()`

**Coverage Goals:**
- Critical path: manifest/CLI selection → 3 mode runs → report emission
- Edge cases: invalid cohort size, duplicate beads, closed beads, missing complexity labels
- Determinism: repeatable output for same inputs

**Test Organization:**
- `internal/benchmark/manifest_test.go`
- `internal/benchmark/selection_test.go`
- `internal/benchmark/validate_test.go`
- `internal/benchmark/harness_test.go`
- `internal/benchmark/metrics_test.go`
- `internal/benchmark/report_test.go`
- `cmd/gromit/benchmark_test.go`

---

## Implementation Tasks

### Task 1: Add benchmark CLI surface and flags

**Files:**
- Modify: `cmd/gromit/main.go`
- Create: `cmd/gromit/benchmark.go`
- Test: `cmd/gromit/benchmark_test.go`

**What to Do:**
Add a `benchmark` command group with `run` subcommand:
- `gromit benchmark run --manifest <path>`
- `--beads id1,id2,...` (optional explicit ordered override)
- `--bead-count N` (optional deterministic truncation)
- `--base-commit <sha>` (optional override)
- `--output-ts <timestamp>` (optional deterministic output name for tests)

Validate flag combinations and ensure clear usage/help text.

**Acceptance Criteria:**
- Command is registered and discoverable in help output.
- Invalid inputs fail with actionable errors.
- CLI passes parsed options to harness entrypoint.

**Dependencies:** None

### Task 2: Implement manifest model and parser

**Files:**
- Create: `internal/benchmark/manifest.go`
- Test: `internal/benchmark/manifest_test.go`
- Create: `.gromit/benchmarks/tdd-vs-single-pass.yaml`

**What to Do:**
Define manifest types and loader for benchmark YAML fields (`id`, `beads`, `modes`, provider/model-family, tier models, final review). Add strict validation for required keys and allowed mode names.

**Acceptance Criteria:**
- Valid manifests parse into typed structs.
- Unknown modes or malformed YAML fail with useful messages.
- Sample manifest is checked in and valid.

**Dependencies:** None

### Task 3: Implement cohort selection (`--beads`, `--bead-count`)

**Files:**
- Create: `internal/benchmark/selection.go`
- Test: `internal/benchmark/selection_test.go`

**What to Do:**
Resolve `selected_beads` deterministically:
1. Use `--beads` list when provided.
2. Else use manifest bead order.
3. Apply `--bead-count` truncation if set.

Add duplicate detection, empty result checks, and out-of-range count errors.

**Acceptance Criteria:**
- `--beads` overrides manifest list and preserves explicit order.
- `--bead-count` truncates deterministically.
- Invalid counts and duplicates are rejected.

**Dependencies:** Task 2

### Task 4: Validate selected cohort against bead state and complexity coverage

**Files:**
- Create: `internal/benchmark/validate.go`
- Test: `internal/benchmark/validate_test.go`

**What to Do:**
Using bead client lookups, verify each selected bead exists, is open, and classify complexity (`complexity:low|high`, medium = explicit medium or unlabeled default). Enforce minimum cohort constraints:
- selected size >= 3
- includes at least one low, one medium/default, one high

**Acceptance Criteria:**
- Closed/missing beads fail validation.
- Missing tier coverage fails validation with explicit gap.
- Happy path returns normalized cohort metadata.

**Dependencies:** Task 3

### Task 5: Add explicit bead-sequence runner adapter

**Files:**
- Modify: `internal/runner/orchestrator.go` (or constructor wiring surface)
- Create/Modify: `internal/runner/...` adapter file(s)
- Test: `internal/runner/..._test.go`

**What to Do:**
Add a runner entrypoint that processes an explicit ordered bead ID list instead of relying on `Ready/ReadyWithLabel` queue selection. Preserve existing `gromit run` behavior unchanged.

**Acceptance Criteria:**
- Runner can execute a provided ordered sequence deterministically.
- Existing default run path remains unchanged.
- Sequence adapter is covered by tests.

**Dependencies:** None

### Task 6: Build deterministic per-mode execution in isolated worktrees

**Files:**
- Create: `internal/benchmark/worktree_run.go`
- Create: `internal/benchmark/harness.go`
- Test: `internal/benchmark/harness_test.go`

**What to Do:**
For each mode, create worktree from one base commit, materialize per-mode config overlay, run sequence runner, collect run timestamps/log path, and cleanup. Ensure mode differences are limited to methodology fields:
- `single_pass`
- `tdd + fresh_context_per_cycle=false`
- `tdd + fresh_context_per_cycle=true`

**Acceptance Criteria:**
- All modes start from identical base commit.
- All modes use identical selected bead order.
- Only methodology settings vary across modes.

**Dependencies:** Task 5

### Task 7: Implement provider/model pinning and review/validation policy overlay

**Files:**
- Create/Modify: `internal/benchmark/harness.go`
- Test: `internal/benchmark/harness_test.go`

**What to Do:**
Apply manifest constraints into temporary run config:
- Provider/model-family pinned consistently across all modes
- Low tier defaults for build/validation phases
- Final non-interactive high-tier review with `apply_fixes=true`
- Final validation pass after review

**Acceptance Criteria:**
- Effective config matches manifest pinning in all modes.
- Build/validation run at low tier defaults.
- Final high-tier review and final validation are always executed.

**Dependencies:** Task 6

### Task 8: Implement metrics extraction and aggregation

**Files:**
- Create: `internal/benchmark/metrics.go`
- Test: `internal/benchmark/metrics_test.go`

**What to Do:**
Aggregate per-mode metrics from run JSONL:
- Token/cost totals by `actual_tier` (`low|medium|high`)
- Overall token/cost totals
- Wall-clock (`run_started_at`, `run_finished_at`, elapsed seconds)
- Quality metrics: avg `quality_score`, first-pass success rate
- Review metrics: findings/fixes applied
- Post-review validation pass/fail

Use stable sorting and deterministic numeric formatting for report consumers.

**Acceptance Criteria:**
- Aggregates match fixture expectations exactly.
- Missing optional fields are handled safely.
- Tier buckets are always present in output.

**Dependencies:** Task 6

### Task 9: Implement report JSON/Markdown writers

**Files:**
- Create: `internal/benchmark/report.go`
- Test: `internal/benchmark/report_test.go`

**What to Do:**
Write two artifacts:
- `.gromit/benchmarks/results/<benchmark-id>/<timestamp>.json`
- `.gromit/benchmarks/results/<benchmark-id>/<timestamp>.md`

Include manifest metadata, per-mode summary table, by-tier token/cost tables, quality tables, and normalized winner hints (fastest, cheapest, best quality, best cost/quality ratio).

**Acceptance Criteria:**
- JSON schema includes all required sections.
- Markdown table output is deterministic.
- Winner hints are reproducible and tie-broken stably.

**Dependencies:** Task 8

### Task 10: End-to-end command integration and regression coverage

**Files:**
- Modify: `cmd/gromit/benchmark.go`
- Modify: `cmd/gromit/benchmark_test.go`

**What to Do:**
Wire parser → selection → validation → harness → metrics → report pipeline in CLI command. Add integration-style tests with fixture manifests/logs and deterministic timestamps.

**Acceptance Criteria:**
- A single CLI command runs all three modes end-to-end (in test via mocks/fakes).
- Output files are written in expected locations.
- Repeated runs with fixed fixtures produce identical artifacts.

**Dependencies:** Tasks 2-9

---

## Notes

- This plan intentionally adds deterministic sequence execution to avoid queue drift from `bd ready` behavior.
- `--beads` gives full explicit control over which beads participate and in what order.
- `--bead-count` is treated as deterministic truncation, not random sampling.
- The original spec’s “exactly 5 beads” is generalized to configurable cohorts while preserving tier coverage guarantees.
- Keep benchmark harness isolated from normal run path to minimize regression risk.
