---
id: prompt-token-accounting-diagnostics
source_ideas: []
created: 2026-02-19
epic: token-efficiency-program
---

# Prompt Token Accounting Diagnostics

## Specification

Add end-to-end prompt token accounting and budget diagnostics so Gromit can systematically reduce total token consumption across all prompt-producing workflows.

The system will capture per-invocation, per-section token estimates for prompt inputs and persist them alongside iteration/run metrics, with a consistent schema across runtime and pipeline workflows.

### Scope

Token accounting must cover all prompt types used by the project, including:
- Runtime prompts from `internal/prompt` renderer (build, acceptance tests, atdd/tdd/refactor, review, thorough review, spec acceptance, spec gate, precheck, scope, validate, analyze, learn, test fix, coverage validation, tdd red/green)
- Retro prompt rendering
- Pipeline-stage prompts and wrappers used by explore/refine/plan/decompose/review command flows

### Per-Invocation Diagnostics

For every prompt invocation, record diagnostics even when no trimming occurs. Diagnostics include:
- Prompt type/phase identifier
- Total estimated prompt input tokens
- Per-section estimated tokens for key prompt sections (including CLAUDE.md, RULES, spec, learnings, diff, template/static instructions, and prompt-type-specific sections)
- Budget configuration active for the invocation (if any)
- Budget shaping actions applied (trim/drop/truncate/cap decisions), if any
- Pre-shape and post-shape section token estimates when shaping is applicable
- Final rendered prompt estimated token count

Section accounting should be normalized into a stable section taxonomy so stats can aggregate across prompt types while still allowing prompt-specific sections.

### Reconciliation and Drift Tracking

For invocations that later produce provider usage data, include reconciliation fields:
- Estimated prompt input tokens
- Provider-reported input tokens
- Absolute and percentage delta

This reconciliation data will support estimator quality monitoring and iterative tuning.

### Storage and Observability

Persist token diagnostics to logs/metrics so they can be:
- Inspected per invocation/iteration
- Aggregated in continuous metrics and run/global stats outputs
- Queried by prompt type, section, and model/provider

`gromit stats` must expose prompt-token diagnostics focused on optimization, including:
- Highest-cost prompt types by estimated input tokens
- Section-level token contributors (top contributors overall and by prompt type)
- Budget action frequency and observed token reduction from shaping
- Estimator reconciliation summaries

Diagnostics must remain backwards-compatible with existing log readers by using additive fields and safe defaults when fields are absent.

### Optimization Feedback Loop

The output must make it straightforward to identify and prioritize token-reduction opportunities by:
- Highlighting largest recurring section costs
- Surfacing prompt types with highest average and total token usage
- Showing where budget shaping reduces tokens and where it does not
- Making pre/post comparison possible for future optimization changes

## Acceptance Criteria

- A prompt invocation in every major workflow category (runtime, retro, pipeline) emits token diagnostics with total estimated input tokens and per-section estimates.
- Diagnostics are emitted for every prompt invocation, including invocations where no budget trimming occurs.
- Logs/metrics persist section-level diagnostics using additive, backward-compatible fields that do not break existing readers when absent.
- `gromit stats` (text and JSON outputs) includes prompt-token diagnostic summaries that identify top token-consuming prompt types and sections.
- Budget shaping diagnostics include pre/post estimated token totals and applied trim actions when shaping is active.
- Reconciliation data is captured where provider input token usage is available, including estimated vs reported delta metrics.
- At least one automated test per integration layer (prompt rendering, logging/metrics serialization, stats presentation) verifies the new diagnostics are produced and surfaced.
- Existing behavior for prompt rendering and run execution remains intact when diagnostics are enabled (no functional regression in prompt content generation).

## Decisions

1. **Always-on diagnostics** All prompt invocations record accounting data, not only over-budget cases, to enable baseline analysis, trend tracking, and reliable measurement of optimization impact.

2. **Token-first accounting** The system tracks estimated tokens rather than only character counts because the optimization goal is token consumption reduction.

3. **Full pipeline coverage** Scope includes runtime, retro, and pipeline-stage prompts so optimization targets overall system token usage, not only bead execution prompts.

4. **Section-level attribution** Diagnostics must break down token usage by section to make optimization actionable; total-only accounting is insufficient for targeted reductions.

5. **Estimator reconciliation required** Estimated counts are compared against provider-reported input token usage when available to track and improve estimator quality.

6. **Additive schema evolution** New diagnostics are introduced via additive log/metrics fields to preserve compatibility with existing historical data and tooling.

## Research & Context

### Current State

- Prompt budget shaping currently tracks section sizes in characters (`ShapeReport.SectionSizes`) and trim actions, but not token-level section accounting.
- Prompt budget logging currently prints coarse stderr messages with before/after character counts and trim actions.
- Iteration logs include aggregate `input_tokens` and `output_tokens` but no per-section prompt-token diagnostics.
- Continuous metrics and efficiency reporting ingest iteration-level totals but do not aggregate prompt-section token attribution.
- `gromit stats` currently reports model performance and cost per bead/spec, but not prompt-section token diagnostics.
- Pipeline and command flows include additional large prompt wrappers (skills/system instruction blocks) that are currently untracked at section-token granularity.

### Relevant Code Areas

- Prompt shaping and render path: `internal/prompt/budget.go`, `internal/prompt/prompt.go`
- Runtime invocation and iteration logging: `internal/runner/callbacks.go`, `internal/runner/logging.go`, `internal/runner/runtypes/types.go`, `internal/logger/logger.go`
- Metrics pipelines: `internal/logger/process_trend.go`, `internal/logger/efficiency.go`
- Stats presentation: `cmd/gromit/stats.go`
- Retro prompt flow: `internal/retro/retro.go`
- Pipeline prompt construction/wrappers: `internal/pipeline/refine.go`, `internal/pipeline/decompose.go`, `cmd/gromit/plan.go`, `cmd/gromit/explore.go`, `cmd/gromit/review.go`

### Constraints

- Prompt diagnostics should not alter rendered prompt semantics.
- Estimation must be deterministic enough for trend comparisons across runs.
- Storage overhead should remain bounded; section payloads should be structured and concise.
