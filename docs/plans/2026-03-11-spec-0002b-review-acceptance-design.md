# Spec 0002b — LLM Review, Acceptance Evaluation, and Fix-Cycle Replanning

## Summary

Extend the core execution loop from Spec 0002a with LLM-driven code review, acceptance evaluation against spec criteria, and fix-cycle replanning from review and acceptance failures. This completes the "success requires evidence" principle by adding non-deterministic quality gates on top of the deterministic validation from Spec 0002a.

---

## Problem

Spec 0002a delivers a working execution loop with deterministic validation gates. But deterministic checks alone cannot catch logic errors, test coverage gaps, spec misalignment, or code quality issues. The system needs LLM-driven review and acceptance evaluation to produce meaningful evidence for the human reviewer.

---

## Prerequisites

- Spec 0002a fully implemented and passing.

---

## Goals

### Primary

- Add multi-facet LLM code review between validation and acceptance.
- Evaluate each acceptance criterion from the approved spec individually with evidence.
- Extend the fix-cycle replan loop to cover review and acceptance failures.
- Ship with 2 review facets; support enabling more from the built-in registry via config.
- Make review finding severity thresholds configurable.
- Explicitly defer VISION.md review outcome label recording to Spec 0003.

### Secondary

- Keep review facets independently testable and configurable.
- Ensure review and acceptance failures produce actionable fix context for the planner.

---

## Non-goals

- More than 2 initial review facets (additional built-in facets can be enabled via config)
- Custom user-defined review facets (may be added in a later spec)
- Deterministic acceptance harnesses (rubric-based LLM evaluation for now)
- Recording VISION.md review outcome labels (`accepted`, `rework_implementation_gap`, `rework_vision_change`) — deferred to Spec 0003 which formalizes human review capture

---

## Use Cases / Scenarios

### Happy path: review and acceptance pass

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

[init] Created run 20260315-100000-f1g2h3
[compile] Spec packet compiled
[plan] Planner produced 4 tasks
[execute] All 4 tasks completed
[validate] Final validation: 6/6 passed
[review] spec_alignment: 0 findings
[review] code_quality: 1 info finding (consider extracting helper)
[accept] 5/5 acceptance criteria: pass
[evidence] Bundle written

Terminal state: ready_for_review
```

Info-level review findings are recorded but do not block.

### Review finding triggers fix cycle

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

...
[validate] Final validation: 6/6 passed
[review] spec_alignment: 1 suggestion finding
  "Spec requires idempotency key validation, but handler does not check for duplicate keys"
[replan] Cycle 2 (fix): 1 task targeting review finding
[execute] Task t-005: add idempotency key validation ... done
[validate] Final validation: 6/6 passed
[review] spec_alignment: 0 findings
[review] code_quality: 0 findings
[accept] 5/5 acceptance criteria: pass

Terminal state: ready_for_review
```

The review finding produced a targeted fix task. The second review pass is clean.

### Acceptance criterion unclear triggers fix attempt then needs_human

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

...
[accept] Criterion 1 "refund endpoint returns 200": pass
[accept] Criterion 2 "handles partial refunds": pass
[accept] Criterion 3 "audit log entry created": unclear
  "No test explicitly verifies audit log creation. Handler calls audit service
   but there is no mock or integration test proving the call succeeds."
[replan] Cycle 2 (fix): 1 task targeting acceptance gap
[execute] Task t-005: add audit log integration test ... done
[validate] 7/7 passed
[review] clean
[accept] Criterion 3 "audit log entry created": pass

Terminal state: ready_for_review
```

### Acceptance fail exhausts budget, becomes needs_human

```
...
[accept] Criterion 4 "supports multi-currency refunds": fail
  "Implementation only handles USD. Spec requires EUR and GBP support."
[replan] Cycle 2 (fix): 2 tasks targeting multi-currency support
[execute] ...
[accept] Criterion 4: fail
  "EUR added but GBP conversion uses wrong rate source"
[replan] Cycle 3 (fix): 1 task targeting GBP rate source
[execute] ...
[accept] Criterion 4: fail
  "GBP rate now correct but EUR conversion regressed"
[budget] max_spec_cycles (3) exhausted

Terminal state: needs_human
Blocker: Acceptance criterion 4 "supports multi-currency refunds" failed across 3 cycles.
  Each fix cycle resolved one currency but regressed another.
  Recommended action: Multi-currency support may need a different architectural
  approach. Consider a currency conversion service abstraction.
```

### Review fix cycle produces new findings, exhausts budget

```
$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

[init] Created run 20260315-100000-x9y8z7
[compile] Spec packet compiled
[plan] Planner produced 3 tasks
[execute] All 3 tasks completed
[validate] Final validation: 5/5 passed
[review] spec_alignment: 1 error finding
  "Spec requires idempotency key validation, but handler does not check for duplicate keys"
[replan] Cycle 2 (fix): 1 task targeting review finding
[execute] Task t-004: add idempotency key validation ... done
[validate] Final validation: 5/5 passed
[review] spec_alignment: 0 findings
[review] code_quality: 1 warning finding (new)
  "idempotency check duplicates logic from existing middleware"
[replan] Cycle 3 (fix): 1 task targeting review finding
[execute] Task t-005: extract shared idempotency middleware ... done
[validate] Final validation: 5/5 passed
[review] code_quality: 1 warning finding (new)
  "extracted middleware missing unit test"
[budget] max_spec_cycles (3) exhausted

Terminal state: needs_human
Blocker: Review finding "extracted middleware missing unit test" remains after 3 cycles.
  Each fix cycle resolved its targeted finding but introduced a new one.
```

Each review fix cycle consumed one cycle from the shared `max_spec_cycles` budget.

### Configurable threshold: warnings non-blocking

```
# execution policy has: "review": {"replan_threshold": "error"}

...
[review] code_quality: 2 warning findings (recorded, not blocking per threshold)
[review] spec_alignment: 0 findings
[accept] 5/5 pass

Terminal state: ready_for_review
```

Warnings appear in the evidence bundle but do not trigger fix cycles.

### Adding a review facet via config

```
# User edits policy/execution.json:
# "review": {"facets": ["spec_alignment", "code_quality", "logic_gaps"], ...}

$ gromit-next exec spec --project payments-api --spec ./specs/add-refund-endpoint.md

...
[review] spec_alignment: 0 findings
[review] code_quality: 0 findings
[review] logic_gaps: 1 error finding
  "refund_handler.go:42 — nil pointer if refund amount is zero"
[replan] Cycle 2 (fix): 1 task
...
```

No code change needed — the built-in facet runs automatically once enabled in config.

---

## New Packages

```
internal/next/
  review/         # Multi-facet LLM code review
  acceptor/       # Evaluates acceptance criteria against evidence
```

---

## Execution Policy Extensions

The execution policy gains review configuration:

```json
{
  "review": {
    "facets": ["spec_alignment", "code_quality"],
    "tiers": {
      "spec_alignment": "high",
      "code_quality": "medium"
    },
    "replan_threshold": "suggestion"
  }
}
```

And an additional model tier:

```json
{
  "models": {
    "planner": "high",
    "executor": "medium",
    "evaluator": "high"
  }
}
```

---

## Extended Stage Pipeline

```
Init -> Compile -> Plan -> Execute -> Validate -> Review -> Accept -> Evidence -> Finalize
                    ^                    |           |        |
                    |____________________|___________|________|
                     failures loop back to Plan
```

ReviewStage and AcceptStage are inserted between ValidateStage and EvidenceStage. Both use the `evaluator` model tier from the execution policy. Both can produce `ReplanFrom`, consuming `max_spec_cycles` budget.

### Stage-to-terminal-state mapping (additions)

| Stage | Can produce `ReplanFrom`? |
|-------|-------------------------|
| ReviewStage | Yes (findings above threshold) |
| AcceptStage | Yes (fail or unclear criteria) |

---

## Review Stage

Sits between Validate and Accept. Multi-facet LLM review that catches problems deterministic checks miss.

### Initial Facets (2)

| Facet | What it checks | Default tier |
|-------|---------------|-------------|
| spec_alignment | Does the diff implement what the spec asked for? | high |
| code_quality | Naming, structure, duplication, readability | medium |

Facets are selected from a known built-in registry. Each built-in facet has a prompt template. Users select which facets to enable via execution policy config, not arbitrary custom facets. Custom facet support may be added in a later spec.

Additional built-in facets available for enabling:

| Facet | What it checks | Suggested tier |
|-------|---------------|-------------|
| logic_gaps | Off-by-one, nil handling, missing error paths | high |
| test_coverage | Are new code paths tested? Missing edge cases? | medium |
| architecture_drift | Does the change respect boundaries from the project cell? | medium |

Each facet is a separate agent invocation, potentially parallel. Each receives the diff summary plus relevant slices from the project cell.

### Finding format

```json
{
  "facet": "code_quality",
  "findings": [
    {
      "severity": "suggestion",
      "file": "internal/next/validator/runner.go",
      "line": 42,
      "description": "nil pointer if commands list is empty",
      "suggested_fix": "add empty check before iteration"
    }
  ]
}
```

### Severity levels

- **error** — must fix. Always triggers re-plan.
- **warning** — should fix. Triggers re-plan by default.
- **suggestion** — can be improved. Triggers re-plan by default.
- **info** — informational. Never triggers re-plan. Recorded in evidence.

### Configurable threshold

`review.replan_threshold` controls which severities trigger replanning:

| Threshold value | Blocks on |
|----------------|-----------|
| `"error"` | error only |
| `"warning"` | error + warning |
| `"suggestion"` (default) | error + warning + suggestion |

This prevents subjective findings from burning fix cycles when not desired.

### Fix-cycle review behavior

On fix cycles, the review stage distinguishes new findings from pre-existing ones.

Findings from prior cycles are stored in the run's `review.json`. On fix cycles, the review agent receives prior findings and is prompted to label each current finding as **"new"** or **"pre-existing"** (matching by file + description similarity). Only new findings at or above the threshold trigger replanning.

A finding that matches a prior finding by file path and similar description (even if the line number shifted) is considered pre-existing. Matching strategy (v1): same file path AND exact substring match on description text. A future version may use cosine similarity > 0.8 on description text for fuzzy matching.

A fix cycle that resolves targeted findings but surfaces new info-level notes does not trigger another replan.

---

## Acceptance Evaluation

Evaluates each acceptance criterion from the approved spec individually.

### Input

- Approved spec's acceptance criteria
- Final validation results
- Review findings (all, including info)
- Changed files / diff summary
- Task results

### Output per criterion

```json
{
  "criterion": "Zero repo pollution",
  "status": "pass",
  "rationale": "No gromit files found in target repo tracked files. All artifacts stored in external workspace.",
  "evidence_refs": ["evidence/diff-summary.md", "evidence/worktree-info.json"]
}
```

### Rules

- Any deterministic validation failure prevents `ready_for_review` (inherited from 0002a).
- Any review finding above threshold prevents `ready_for_review`.
- Any acceptance `fail` triggers re-plan (within budget).
- Any acceptance `unclear` triggers re-plan (within budget). When a criterion is `unclear`, the fix-plan targets adding evidence or tests to make the determination possible, not re-implementing the feature. The planner is prompted: "This criterion could not be evaluated. Add tests or observable evidence that proves or disproves it."
- Budget exhausted with remaining failures -> `needs_human`.

---

## Failure Context Contracts

When ReviewStage or AcceptStage triggers replanning, each provides a structured `FailureContext` so the planner can produce targeted fix tasks.

### ReviewStage FailureContext

Contains the list of blocking findings. Each entry includes:

```json
{
  "facet": "spec_alignment",
  "severity": "error",
  "file": "internal/next/validator/runner.go",
  "line": 42,
  "description": "nil pointer if commands list is empty",
  "suggested_fix": "add empty check before iteration"
}
```

The planner uses these to produce targeted fix tasks — one task per finding or grouped by file at the planner's discretion.

### AcceptStage FailureContext

Contains the list of failed or unclear criteria. Each entry includes:

```json
{
  "criterion": "audit log entry created",
  "status": "fail",
  "rationale": "Implementation calls audit service but no test verifies the call succeeds.",
  "evidence_refs": ["evidence/acceptance.json", "evidence/test-results.json"]
}
```

The planner uses these to produce tasks that either implement missing behavior (for `fail` status) or add evidence/tests (for `unclear` status).

---

## Extended Terminal States

### `ready_for_review` (updated)

All deterministic validation passes. All review facets clear (no findings above threshold). All acceptance criteria `pass`. Evidence bundle complete. Human still decides whether to accept into the product.

### VISION Review Outcome Labels

This spec does not record the VISION.md review outcome labels (`accepted`, `rework_implementation_gap`, `rework_vision_change`). The machine stops at `ready_for_review`. Spec 0003 formalizes capturing the human's review decision with these labels and feeding them into the vision metrics loop.

---

## Extended Evidence Bundle

### Additional artifacts

```
evidence/
  review.json           # Aggregated review findings (all facets)
  acceptance.json       # Per-criterion evaluation results
```

### review.json schema

Object keyed by facet name, each containing an array of findings:

```json
{
  "spec_alignment": [
    {
      "severity": "error",
      "file": "internal/handler/refund.go",
      "line": 87,
      "description": "Spec requires idempotency key validation, but handler does not check for duplicate keys",
      "suggested_fix": "Add idempotency key lookup before processing refund",
      "cycle": 1,
      "disposition": "new"
    }
  ],
  "code_quality": []
}
```

The `cycle` field records which execution cycle produced the finding. The `disposition` field is `"new"` or `"pre-existing"` (set during fix-cycle review).

### review.md (updated)

Adds sections for: review findings by facet, per-criterion acceptance table with status/rationale/evidence.

---

## Extended Run Storage

```
<run-id>/
  evidence/
    ...existing from 0002a...
    review.json           # Aggregated review findings
    acceptance.json       # Per-criterion evaluation
```

---

## Acceptance Criteria

1. **Review gate** — `ready_for_review` is impossible if review finds findings above the configured threshold.
2. **Acceptance evidence** — Every acceptance criterion has an explicit pass, fail, or unclear result with rationale and evidence references.
3. **Configurable threshold** — The `review.replan_threshold` setting controls which severity levels trigger replanning.
4. **Fix-cycle from review** — Review findings above threshold trigger a fix-plan cycle that targets the specific findings.
5. **Fix-cycle from acceptance** — Acceptance `fail` or `unclear` results trigger a fix-plan cycle targeting the specific gaps.
6. **Facet configurability** — Review facets are selected from a built-in registry and can be enabled or disabled via execution policy without code changes.
7. **Severity levels** — Findings are categorized as error/warning/suggestion/info with distinct blocking behavior.
8. **VISION label deferral** — The system does not auto-label work as `accepted`. VISION review outcome labels are explicitly deferred to Spec 0003.
9. **Budget sharing** — Validation, review, and acceptance fix cycles all consume from the same `max_spec_cycles` budget. A run that uses cycle 1 for initial execution, cycle 2 for a validation fix, and cycle 3 for a review fix correctly exhausts the budget.

---

## Evidence Required

- Integration test showing review findings blocking `ready_for_review`.
- Integration test showing acceptance `fail` triggering a fix cycle.
- Integration test showing acceptance `unclear` resulting in `needs_human` after budget exhaustion.
- Verification that `review.replan_threshold` setting changes which findings trigger replanning.
- Example evidence bundle with per-criterion acceptance evaluation reviewed by a human.
- Verification that adding a review facet via config (without code changes) works.
- Verification that review findings from fix cycles distinguish new findings from pre-existing ones.
