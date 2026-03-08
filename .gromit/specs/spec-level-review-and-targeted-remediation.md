---
id: spec-level-review-and-targeted-remediation
source_ideas: []
created: 2026-03-08
depends_on:
  - immutable-pipeline
  - v2-run-loop
---

# Spec-Level Review and Targeted Remediation

## Problem

The remediation cycle re-decomposes the original plan when acceptance fails, creating duplicate beads with new IDs for work already completed under different IDs. The original open beads remain open, causing the next run to reprocess them. The loop never converges.

Three root causes combine:

1. **Remediation re-decomposes the original plan.** When acceptance fails, the remediation runner calls decompose with the original plan as input. This produces a fresh set of beads covering the same tasks, leaving prior open beads untouched.

2. **No spec-level code review.** The per-bead review catches issues within each bead's scope, but nobody evaluates the cumulative output holistically. Bugs that span beads, architectural drift, and integration issues go undetected until acceptance fails — and acceptance has no mechanism to describe what's wrong in actionable terms.

3. **Accept is binary.** Accept returns pass or fail. When it fails, the only information available is the gap analysis, which may be empty or vague. Remediation has nothing specific to act on, so it falls back to re-decomposing the whole plan.

## Specification

### Post-Bead-Loop Evaluation

After the bead loop completes, two evaluations run sequentially:

**Accept** evaluates the spec's acceptance criteria against the cumulative diff (DiffFromBase). Unchanged from current behavior. Returns pass or fail.

**Spec-Level Review** evaluates the cumulative diff holistically. This is distinct from the per-bead review that runs during the bead loop. The spec-level review uses the highest-tier model (opus for Claude) and receives:

- The cumulative diff (DiffFromBase)
- The original plan (for intent context)
- Project context (CLAUDE.md, RULES.md)
- A thorough review prompt covering: correctness, security (OWASP top 10), error handling, test coverage gaps, code quality, architectural fit

The spec-level review produces structured output:

```json
{
  "verdict": "pass | fail",
  "findings": [
    {
      "severity": "critical | warning | suggestion",
      "category": "bug | security | quality | test-gap | architecture",
      "scope": "spec | general",
      "description": "what is wrong",
      "affected_files": ["path/to/file.go"]
    }
  ]
}
```

**Verdict logic:** Any `critical` finding forces `verdict: fail`. Only `warning` and `suggestion` findings produce `verdict: pass` (pass with improvements).

**Gating:** The spec succeeds only when `accept == pass AND review.verdict == pass`.

### Targeted Remediation

When either accept or review fails, the combined findings become the remediation input. The remediation runner no longer re-decomposes the original plan.

Accept produces findings in the same structured format: each unmet criterion becomes a finding with `severity: critical`, `category: acceptance`, `scope: spec`.

The decompose stage receives a new findings-based prompt template. Instead of "break this plan into beads," the prompt says "create targeted fix beads for these specific findings." Each finding maps to one or more beads. Dependencies reference existing closed beads when the fix builds on prior work.

The gate satisfaction check (already wired with `WithSatisfactionCheck`) handles the stale bead problem: open beads from prior generations whose work is already done get closed at gate time without rebuilding.

The remediation cycle is: findings → decompose targeted beads → bead loop → accept + review again. Generation cap still applies.

### Pass-With-Improvements

When spec-level review passes but has findings:

**Spec-scoped findings** (`scope: spec`) — issues in code this spec changed — become beads labeled `from-review spec:<spec-id>`. These are deferred fixes relevant to the spec's work.

**General findings** (`scope: general`) — issues unrelated to spec changes — become beads labeled `from-review` without a spec label. These go on the general backlog.

In both cases, the spec proceeds to presentation and merge. The from-review beads are created but not executed in this run.

### From-Review Execution

A new `--from-review` flag on `run2` runs only from-review beads. No plan, no decompose — it queries open beads with the `from-review` label and runs them through the bead loop directly.

Optional scoping: `--from-review --spec immutable-pipeline` runs only from-review beads with `spec:immutable-pipeline`.

No remediation cycle for from-review beads. They are one-shot fixes. If a from-review bead fails build/validate, it stays open for retry on the next `--from-review` run but does not trigger spec-level accept/review.

### Model Selection

The spec-level review always uses the highest configured tier. This is not routed — it is hardcoded to the top tier. The review is the quality gate for the entire spec's output.

If the routing system has multiple providers, the spec-level review uses the provider configured for the highest tier (or the default provider at opus tier if no explicit mapping exists).

## Acceptance Criteria

- After the bead loop, a spec-level review evaluates the cumulative diff using the highest-tier model
- The spec-level review produces structured findings with severity, category, scope, description, and affected files
- The spec succeeds only when both accept passes and review verdict is pass
- Critical findings from review force a fail verdict
- Warning and suggestion findings allow a pass verdict
- When accept or review fails, their findings become the input to remediation decompose
- Remediation decompose creates targeted fix beads from findings, not from the original plan
- The gate satisfaction check closes open beads whose acceptance criteria are already satisfied
- When review passes with findings, spec-scoped findings become from-review beads labeled with the spec
- When review passes with findings, general findings become from-review beads without a spec label
- `run2 --from-review` runs only beads with the from-review label through the bead loop
- `run2 --from-review --spec <id>` scopes to from-review beads for a specific spec
- From-review beads do not trigger spec-level accept/review cycles

## Decisions

1. **Separate stage from per-bead review.** The per-bead review runs during the bead loop and evaluates each bead's changes in isolation. The spec-level review evaluates the cumulative output after all beads complete. These serve different purposes — local quality vs holistic assessment — and use different context (single bead diff vs cumulative diff). Combining them would compromise both.

2. **Highest tier always for spec-level review.** The review is the final quality gate. Using a cheaper model risks missing issues that a more capable model would catch. The cost of one opus invocation per spec run is small relative to the total API spend across all bead builds.

3. **Findings-based remediation over plan re-decomposition.** Re-decomposing the original plan creates duplicate beads for already-completed work. Findings-based remediation produces only the beads needed to fix specific issues. This eliminates the duplicate bead problem at its source.

4. **Gate satisfaction check as safety net.** Even with findings-based remediation, stale beads from prior generations may exist. The gate satisfaction check (using DiffFromBase and LLM evaluation) closes beads whose work is already done, preventing redundant rebuilds regardless of how the beads were created.

5. **From-review beads as deferred work.** When review passes with improvements, executing those improvements in the same run would delay spec completion for non-critical work. Creating beads and deferring them to a separate `--from-review` run lets the spec merge promptly while ensuring improvements are tracked and actionable.

6. **No remediation cycle for from-review beads.** From-review beads are discrete fixes, not spec deliverables. Running accept/review after each one would be disproportionate overhead. If a fix fails, it stays open for the next from-review run.

7. **Structured findings over prose.** Machine-parseable findings enable the decompose stage to create targeted beads without LLM interpretation of free-form text. Severity classification automates the pass/fail verdict. Scope classification automates bead labeling.

## Architecture Direction

A new `internal/v2/stage/specreview/` package implements the spec-level review stage. It follows the same `Stage` interface as other stages. The review prompt lives in a project-root fragment file (`review_spec_v2.md`) loaded by the components wiring.

The spec loop (`spec_loop.go`) adds the spec-level review call after accept. The gating logic combines both verdicts. When either fails, it collects findings from both and passes them to the remediation runner.

The remediation runner (`remediation.go`) changes its `executeRemediation` method to receive findings instead of a gap analysis string. It passes findings to the decompose stage via a new field on `stage.Request`.

The decompose stage (`decompose.go`) gains a third prompt template (`findingsDecomposePromptTemplate`) that takes a list of findings and produces targeted fix beads.

The accept stage gains structured finding output so its unmet criteria can be combined with review findings in a uniform format.

`run2.go` gains the `--from-review` flag. When set, it skips plan/decompose/accept/review and queries from-review beads directly.

## Test Strategy

Unit tests for the spec-level review stage: structured output parsing, verdict logic (critical → fail, warning/suggestion → pass), finding classification. Unit tests for findings-based decompose template: verify beads map to findings, not to original plan tasks. Unit tests for from-review bead creation: correct labeling by scope. Unit tests for the `--from-review` flag: bead query, no plan/decompose, no remediation cycle. Integration test for the full post-bead-loop pipeline: bead loop → accept → review → remediation with findings → targeted beads. Integration test for pass-with-improvements: review passes, from-review beads created, spec proceeds to present.

## Research & Context

### Current State

The remediation runner (`internal/v2/remediation/remediation.go`) calls the decompose stage with the original plan when acceptance fails. If `req.Remediation` is true and a gap analysis exists, the `remediationDecomposePromptTemplate` is used; otherwise it falls back to the default template which re-decomposes the entire plan.

The per-bead review (`internal/v2/stage/review/`) creates P0 bug beads with the `from-review` label during the bead loop. This pattern is established and the new spec-level review follows the same labeling convention.

The gate satisfaction check (`internal/v2/stage/gate/satisfaction.go`) was implemented in the stale-bead-prevention feature branch and wired into the run2 components as part of this debugging session.

### Files to Change

- Create: `internal/v2/stage/specreview/specreview.go`, `specreview_test.go`
- Modify: `internal/v2/loop/spec_loop.go` — add spec-level review, combine verdicts, pass findings
- Modify: `internal/v2/loop/run2_components.go` — wire spec-level review with highest tier
- Modify: `internal/v2/remediation/remediation.go` — receive findings, not gap analysis
- Modify: `internal/v2/stage/decompose/decompose.go` — add findings-based prompt template
- Modify: `internal/v2/stage/accept/accept.go` — structured finding output for unmet criteria
- Modify: `cmd/gromit/run2.go` — add `--from-review` flag
- Create: `review_spec_v2.md` — spec-level review prompt fragment
