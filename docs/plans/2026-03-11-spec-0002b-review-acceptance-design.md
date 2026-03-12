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
[review] spec_alignment: 1 warning finding
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

### Configurable threshold: suggestions non-blocking (default behavior)

```
# execution policy has default: "review": {"replan_threshold": "warning"}

...
[review] code_quality: 2 suggestion findings (recorded, not blocking per default threshold)
[review] spec_alignment: 0 findings
[accept] 5/5 pass

Terminal state: ready_for_review
```

Suggestions appear in the evidence bundle but do not trigger fix cycles at the default `"warning"` threshold. Only warnings and errors consume fix-cycle budget.

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
  review/         # Multi-facet LLM code review (domain: finding types, severity, registry, facet invocation)
  acceptor/       # Evaluates acceptance criteria against evidence (domain: criterion evaluation, parsing)
```

Stage wrappers live in `specloop/stages/review.go` and `specloop/stages/accept.go`. Domain logic (finding types, severity, registry, facet invocation, criterion evaluation) lives in `internal/next/review/` and `internal/next/acceptor/`. Stages import domain packages — same pattern as `stages/plan.go` imports `planner/`.

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
    "replan_threshold": "warning"
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

Note: Only the first stage returning ReplanFrom in a given cycle triggers the replan. If ValidateStage replans, ReviewStage and AcceptStage do not run for that cycle.

ReviewStage and AcceptStage are inserted between ValidateStage and EvidenceStage. Both can produce `ReplanFrom`, consuming `max_spec_cycles` budget.

**Evaluator tier injection:** Tiers are injected via stage config at pipeline construction time (same pattern as ExecuteStage). ReviewStage gets `policy.Models.Evaluator` as its default tier, with per-facet overrides from `policy.Review.Tiers`. AcceptStage gets `policy.Models.Evaluator`. Stages do not read the policy at runtime — they receive their tier configuration at construction.

### RunState extensions

The following fields are added to RunState for review and acceptance tracking:

```go
FinalReviewPassed     bool     `json:"final_review_passed"`
FinalAcceptancePassed bool     `json:"final_acceptance_passed"`
ReviewFindings        []string `json:"review_findings,omitempty"`
AcceptanceResults     []string `json:"acceptance_results,omitempty"`
```

**Cycle reset:** The SpecLoop runner resets these fields at the start of each cycle iteration, before any stage's `Run()` method is called. Specifically, the three gate booleans (`FinalValidationPassed`, `FinalReviewPassed`, `FinalAcceptancePassed`) are reset to `false`. This prevents stale `true` values from carrying over if a stage is skipped or the pipeline ordering changes. The `[]string` fields (`ReviewFindings`, `AcceptanceResults`) are also cleared. Individual stages are not responsible for this reset.

**Design note on RunState string fields vs. structured evidence:**

- `ReviewFindings` and `AcceptanceResults` are `[]string` carrying human-readable summaries. These feed into the planner's `FailureContext` and into the `review.md` evidence summary.
- The structured evidence files (`review.json`, `acceptance.json`) are written directly by `ReviewStage` and `AcceptStage` using their domain types (`[]Finding`, `AcceptanceResult`), not derived from these `[]string` fields.
- `ReviewStage` maintains structured prior findings (`[]Finding`) in-memory for disposition matching across cycles. This structured state is stage-local and is not serialized into RunState.
- `ReviewStage` writes `review.json` after each cycle (it holds the structured findings in-memory). `EvidenceStage` writes `review.md` and `acceptance.json` only. `EvidenceStage` must NOT write `review.json`.

### Stage-to-terminal-state mapping (additions)

| Stage | Can produce `ReplanFrom`? | Can produce `NeedsHuman`? |
|-------|-------------------------|-------------------------|
| ReviewStage | Yes (findings above threshold) | No |
| AcceptStage | Yes (fail or unclear criteria) | Yes (spec lacks acceptance criteria) |

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
| test_coverage | Are there untested code paths, missing edge cases, or inadequate assertions? | medium |
| architecture_drift | Does the change respect boundaries from the project cell? | medium |

Each facet is a separate agent invocation, potentially parallel. Each receives the diff summary plus relevant slices from the project cell.

### Diff computation

ReviewStage computes the diff at runtime by running `git diff` against the base branch in the worktree. A separate `DiffProvider` interface (not on `GitOps`, which handles worktree lifecycle) provides `Diff(baseBranch string) (string, error)`. ReviewStageConfig includes a `DiffProvider` field. This keeps interface segregation clean — only ReviewStage needs diff computation, and test fakes for other stages are not polluted. The diff is computed fresh on each cycle to reflect the latest state after any fix cycles.

### Built-in facet prompt templates

All 5 built-in facets should have reference prompt templates provided. Each facet evaluates a distinct quality dimension:

| Facet | Evaluation focus |
|-------|-----------------|
| spec_alignment | Does the implementation match the spec's requirements and acceptance criteria? |
| code_quality | Are there code smells, dead code, overly complex functions, or naming issues? |
| logic_gaps | Off-by-one, nil handling, missing error paths |
| test_coverage | Are there untested code paths, missing edge cases, or inadequate assertions? |
| architecture_drift | Does the change respect boundaries from the project cell? |

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
- **warning** — should fix. Triggers re-plan by default (at the default `"warning"` threshold).
- **suggestion** — can be improved. Recorded in evidence but does NOT trigger re-plan at the default threshold. Only triggers re-plan when threshold is explicitly set to `"suggestion"`.
- **info** — informational. Never triggers re-plan. Recorded in evidence.

### Configurable threshold

`review.replan_threshold` controls which severities trigger replanning:

| Threshold value | Blocks on |
|----------------|-----------|
| `"error"` | error only |
| `"warning"` (default) | error + warning |
| `"suggestion"` | error + warning + suggestion |

The default `"warning"` threshold prevents churn from subjective LLM style preferences (suggestions) while still catching real bugs (errors and warnings). Suggestions appear in evidence but do not consume fix-cycle budget.

### Parallel facet failure handling

If one facet hits a hard error (timeout, API failure), other facets continue. The failed facet is marked as errored in the findings/evidence — its entry in `review.json` contains an error marker rather than findings. The review proceeds with partial results from the successful facets. The human reviewer sees which facets completed and which errored. This prevents a single flaky API call from discarding results from all other facets.

### Fix-cycle review behavior

On fix cycles, the review stage distinguishes new findings from pre-existing ones.

Findings from prior cycles are stored in the run's `review.json`. On fix cycles, the review agent receives prior findings and is prompted to label each current finding as **"new"** or **"pre-existing"** (matching by file + description similarity). Only new findings at or above the threshold trigger replanning. Prior findings for disposition matching are held in-memory by the ReviewStage instance (which persists across SpecLoop cycles), not read back from RunState.

A finding that matches a prior finding by file path and similar description (even if the line number shifted) is considered pre-existing. Matching strategy (v1): same file path AND exact substring match on description text. A future version may use cosine similarity > 0.8 on description text for fuzzy matching.

A fix cycle that resolves targeted findings but surfaces new info-level notes does not trigger another replan.

---

## Acceptance Evaluation

Evaluates each acceptance criterion from the approved spec individually.

### Acceptance criteria extraction

AcceptStage resolves criteria via a two-tier approach: if `AcceptStageConfig.Criteria` is non-empty, those criteria are used directly (useful for testing and overrides). Otherwise, AcceptStage reads `spec.md` from the run directory and calls `ParseAcceptanceCriteria(specMarkdown)` to extract criteria at runtime. `ParseAcceptanceCriteria` is a helper function in the acceptor package that looks for a `## Acceptance Criteria` section and parses bullet points.

If criteria resolution produces an error (section not found) or returns an empty slice, AcceptStage returns `NeedsHuman` with a clear message: "spec lacks acceptance criteria section — cannot evaluate acceptance. Revise the spec to include acceptance criteria." This is a spec quality issue, not an infrastructure failure, so it produces `NeedsHuman` rather than `Blocked` — the human must fix the spec before the system can evaluate acceptance.

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

`FailureContext.Failures` remains `[]string`. Structured review and acceptance data is serialized into formatted strings using helper functions:

- `ReviewFailuresToStrings(findings []Finding) []string` — formats each finding as `"review:<facet>:<severity>:<file>:<line> — <description>"`
- `AcceptanceFailuresToStrings(results []CriterionResult) []string` — formats differently based on status:
  - fail: `"acceptance:fail: <criterion> — implement missing behavior"`
  - unclear: `"acceptance:unclear: <criterion> — add tests or evidence to prove/disprove"`

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

FinalizeStage sets `ready_for_review` when all three gates pass:

```go
if allDone && rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed {
    rs.Status = StatusReadyForReview
}
```

Note: `ready_for_review` means ready for human review.

### Blocked worktree cleanup

**Behavioral change from 0002a:** Spec 0002a's FinalizeStage removes worktrees for `blocked` runs (only preserving `needs_human` and `ready_for_review`). This spec changes that behavior — FinalizeStage now preserves worktrees for ALL terminal states (`blocked`, `needs_human`, `ready_for_review`). The existing `RemoveWorktree` call for `blocked` runs is removed so a human can inspect blocked runs' state. This is intentional: blocked runs represent infrastructure failures whose worktrees may contain useful diagnostic information.

InitStage handles cleanup of stale blocked worktrees: when creating a new run for a spec, it scans the store for prior runs with the same `spec_id`. For any prior run with `status: blocked` and a non-empty `worktree_path`, InitStage removes the worktree and clears the path in `run.json`. It emits a `blocked_worktree_cleaned` event with the old run ID.

### VISION Review Outcome Labels

This spec does not record the VISION.md review outcome labels (`accepted`, `rework_implementation_gap`, `rework_vision_change`). The machine stops at `ready_for_review`. Spec 0003 formalizes capturing the human's review decision with these labels and feeding them into the vision metrics loop.

### Extended Observability

The event log (`events.jsonl`) gains the following event types:

- `review_result` — emitted by ReviewStage after each cycle. Contains: facets reviewed, finding counts by severity, whether blocking findings were found, errored facets (if any).
- `acceptance_result` — emitted by AcceptStage after each evaluation. Contains: criteria evaluated, per-criterion status (pass/fail/unclear), whether all passed.
- `replan_triggered` gains a `source` field: `"validation"`, `"review"`, or `"acceptance"` to identify which stage triggered the replan.

---

## Extended Evidence Bundle

### Additional artifacts

```
evidence/
  review.json           # Aggregated review findings (all facets)
  acceptance.json       # Per-criterion evaluation results
```

### acceptance.json schema

Uses a structured wrapper:

```json
{"results": [...], "all_pass": true, "has_fail_or_unclear": false}
```

The `AcceptanceResult` Go type has fields `Results []CriterionResult`, `AllPass bool`, `HasFailOrUnclear bool`.

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

### Evidence file authorship

- **ReviewStage** writes `review.json` after each cycle as a side effect of its `Run()` method. It holds structured findings (`[]Finding`) in-memory and writes them directly using the domain types from `review/`. ReviewStage is the sole writer of `review.json`.
- **AcceptStage** writes `acceptance.json` after evaluation as a side effect of its `Run()` method, using its structured `AcceptanceResult` directly. This mirrors ReviewStage's pattern. AcceptStage also populates RunState fields (`FinalAcceptancePassed`, `AcceptanceResults []string`) — the `[]string` fields feed the planner's FailureContext, not the evidence file.
- **EvidenceStage** writes `review.md` (human-readable summary combining review findings and acceptance results). EvidenceStage reads `review.json` and `acceptance.json` from disk when generating `review.md`. EvidenceStage does NOT write `review.json` or `acceptance.json` — those are owned by ReviewStage and AcceptStage respectively.

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
10. **Blocked worktree preservation** — FinalizeStage preserves worktrees for `blocked` runs instead of removing them, so a human can inspect the state. InitStage cleans up stale blocked worktrees when starting a new run for the same spec.

---

## Evidence Required

- Integration test showing review findings blocking `ready_for_review`.
- Integration test showing acceptance `fail` triggering a fix cycle.
- Integration test showing acceptance `unclear` resulting in `needs_human` after budget exhaustion.
- Verification that `review.replan_threshold` setting changes which findings trigger replanning.
- Example evidence bundle with per-criterion acceptance evaluation reviewed by a human.
- Verification that adding a review facet via config (without code changes) works.
- Verification that review findings from fix cycles distinguish new findings from pre-existing ones.

---

## Deferred Items

### Integration tests

Integration tests (full pipeline with real LLM calls) are deferred to 0002b's wiring phase, since 0002a's StageProvider is still a stub. Unit tests with mock agents cover the stage logic in the meantime.

### Per-task artifact files

Per-task artifact writing (`task-packet.md`, `agent-output.txt` per task) will be implemented as part of 0002b when the real executor integration lands. The current design writes only aggregate evidence files.

---

## Bugs Carried Forward from 0002a Manual Testing (2026-03-12)

### 1. Agent Provider Wiring (blocking all manual scenarios)

`cmd/gromit-next/exec.go:57-63` — `defaultStageProvider.BuildStages()` returns a stub error. 0002b must implement a real `StageProvider` that wires `Agent` implementations into stages. This blocks all manual test plan scenarios (including `--dry-run`, since `BuildStages` is called before the dry-run stage filter).

### 2. `spec list` default path resolution bug

`cmd/gromit-next/spec.go:119` — `LoadProjectConfig(".")` looks for `project.json` in the current working directory instead of resolving the project cell path from `--project` flag (should resolve to `~/.local/share/gromit/projects/<project>/project.json`). Additionally, the `ProjectConfig` struct only has `SpecsDir` but the actual `project.json` schema has `name`, `repo_path`, `created_at`. The command works with explicit `--specs-dir` as a workaround.

### 3. `exec list` exit code on empty results

`exec list --project fixture-calc` returns exit code 1 when no runs exist. Debatable whether empty results should be exit 0 (success, just nothing to show) or exit 1. Consider aligning with convention (e.g., `git log` on empty repo exits 0).
