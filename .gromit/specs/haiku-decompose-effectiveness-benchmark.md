---
id: haiku-decompose-effectiveness-benchmark
source_ideas: []
created: 2026-02-28
---

# Haiku Decompose Effectiveness Benchmark

## Specification

Run a controlled benchmark to test whether `haiku` can decompose plans effectively compared with `sonnet` on the same completed-spec cohort.

The benchmark uses exactly 5 completed specs and runs decomposition twice per spec:
- once with `haiku` (low tier)
- once with `sonnet` (medium tier baseline)

Both runs must use identical plan inputs and validation settings so the model tier is the only variable.

The benchmark must be side-effect safe:
- no new beads are created
- no plan frontmatter is modified
- runs execute in review/simulation mode and only produce analysis artifacts

### Cohort Selection

A spec is eligible when:
- it has at least 1 closed bead with label `spec:<name>`
- it has 0 open beads with label `spec:<name>`
- a corresponding plan file exists in `.gromit/plans/<name>.md`

Default selection picks 5 specs deterministically:
1. Sort eligible specs by closed bead count descending
2. Break ties by spec name ascending
3. Select the first 5

Allow an explicit spec list override for reproducibility when desired.

### Comparison Metrics

For each `(spec, model)` run, record:
- bead count
- validation outcome:
  - per-bead violations from shared decompose validation
  - batch contract violations
- complexity outcome:
  - high-complexity bead count
  - high-complexity titles/reasons
- acceptance-criteria density:
  - total criteria
  - mean criteria per bead
- overlap signal count (sibling-overlap rule hits)
- runtime cost signals available from invocation logs (tokens/cost/latency when available)

### Report Output

Write artifacts under `.gromit/benchmarks/results/decompose-haiku-vs-sonnet/<timestamp>/`:
- `raw.json` with per-run outputs and metrics
- `summary.md` with per-spec side-by-side table and aggregate totals

`summary.md` must include:
- per-spec comparison rows (`haiku` vs `sonnet`)
- aggregate deltas across the 5-spec cohort
- a final recommendation status:
  - `haiku-acceptable`
  - `haiku-acceptable-with-guardrails`
  - `keep-sonnet-default`

Recommendation is based on configurable thresholds (for example: no increase in contract violations, bounded increase in high-complexity count, and meaningful cost/latency savings).

## Acceptance Criteria

- A single command executes the decomposition benchmark for exactly 5 completed specs using both `haiku` and `sonnet`.
- Cohort selection is deterministic by default and uses the completed-spec rules in this spec.
- The benchmark runs are side-effect safe: no beads are created and no plan files are marked decomposed/updated.
- Output captures bead count and quality metrics from the shared decompose validation path for each `(spec, model)` pair.
- Output includes a per-spec side-by-side comparison and aggregate cohort-level deltas.
- Output includes a recommendation status (`haiku-acceptable`, `haiku-acceptable-with-guardrails`, or `keep-sonnet-default`) with threshold checks.
- JSON and Markdown artifacts are written to `.gromit/benchmarks/results/decompose-haiku-vs-sonnet/<timestamp>/`.

## Decisions

1. **Use completed specs as the benchmark corpus.** Completed specs provide real-world plan complexity and prior delivery context, avoiding synthetic examples.

2. **Compare only one variable (tier/model).** Prompt shape, plan input, and validation settings remain fixed so the benchmark isolates decomposition effectiveness.

3. **Use review/simulation mode only.** The benchmark is an analysis workflow, not execution; it must not mutate tracker state or plan state.

4. **Use existing validation contracts as quality ground truth.** Shared decompose validation/complexity rules provide objective quality signals and avoid ad hoc scoring.

5. **Emit recommendation with explicit thresholds.** The benchmark should produce a go/no-go style result for routing policy decisions, not just raw tables.

## Research & Context

### Current State

- Decompose currently defaults to medium-tier behavior (`sonnet`) and already computes validation and complexity outcomes during decomposition (`internal/pipeline/decompose.go`).
- Shared decompose quality checks already exist, including sibling-overlap and batch-contract validation (`internal/validate/validate.go`).
- Benchmark infrastructure already exists for controlled multi-mode experiments and report artifacts (`cmd/gromit/benchmark.go`, `internal/benchmark/`), but it is currently focused on run methodology rather than decomposition model comparison.
- Existing benchmark artifacts already live under `.gromit/benchmarks/`, so this comparison should reuse that convention.

### Candidate Completed Specs (Observed)

Examples of currently eligible completed specs in this repo include:
- `thin-cmd-wrappers`
- `spec-branch-merge-pipeline`
- `event-system`
- `tui-foundation`
- `phase-4-token-efficiency-rag-evaluation`

Final cohort still follows deterministic selection at run time.

### Related Specs

- `methodology-benchmark-harness`
- `decompose-low-complexity-bias`
- `decompose-complexity-estimation`
- `cost-optimized-routing`
