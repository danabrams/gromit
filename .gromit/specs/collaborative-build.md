---
id: collaborative-build
source_ideas: []
created: 2026-03-02
---

# Collaborative Build

## Specification

Insert a lightweight, non-blocking review step between the Build and Validate stages of the iteration pipeline. A secondary "Reviewer" agent (haiku-tier) examines the builder's output against the spec and acceptance criteria before validation runs, catching spec misalignment and obvious mistakes early — before they burn validation retries or escape into post-validation review.

### Mid-Build Review Step

After the Build stage completes successfully and before the Validate stage begins, a new review step runs:

1. **Input**: The builder's code diff (from iteration start commit), the spec, and the acceptance criteria.
2. **Execution**: A haiku-tier LLM invocation reviews the diff against the spec and acceptance criteria. It answers: "Does this code look like it's heading toward satisfying the spec, and are there any obvious mistakes?"
3. **Output**: A structured result containing zero or more findings. Each finding is a non-blocking suggestion describing the issue and where it occurs.

### Conditional Fix-Build

- If the mid-build review returns **zero findings**: proceed directly to the Validate stage. No additional cost beyond the single haiku call.
- If the mid-build review returns **one or more findings**: trigger a fix-build invocation that receives the original build output plus the review findings as context. The fix-build produces an updated diff, and the pipeline then proceeds to Validate.
- The fix-build runs once. There is no retry loop between the mid-build reviewer and the fix-builder.

### Non-Blocking Behavior

The mid-build review is strictly non-blocking:

- Findings are suggestions, not commands. The fix-build may address them, partially address them, or ignore them.
- If the mid-build review itself fails (LLM error, timeout), the pipeline proceeds to Validate as if zero findings were returned. The failure is logged but does not block the iteration.
- The mid-build review does not have authority to halt, skip, or restart the iteration.

### Relationship to Post-Validation Review

The existing post-validation Reviewer continues to run unchanged. The two reviewers are complementary:

| Aspect | Mid-Build Reviewer | Post-Validation Reviewer |
|---|---|---|
| When | After Build, before Validate | After Validate passes |
| Tier | Haiku (lightweight) | Configured per review settings |
| Scope | Spec alignment + obvious mistakes | Deep code review, findings → beads |
| Blocking | Never | Never (existing behavior) |
| Output | Findings → fix-build context | Findings → new beads for future iterations |

### Observability

Mid-build review results are logged in the iteration log for each iteration:

- Whether the mid-build review ran
- Number of findings returned
- Whether a fix-build was triggered
- Duration and token cost of the review call
- Duration and token cost of the fix-build call (if triggered)

This data enables measuring the hit rate (how often the reviewer finds issues) and save rate (how often fix-builds prevent validation failures) over time.

## Acceptance Criteria

- A haiku-tier review invocation runs between Build and Validate on each iteration, receiving the code diff, spec, and acceptance criteria.
- When the review returns zero findings, the pipeline proceeds directly to Validate with no additional build invocation.
- When the review returns one or more findings, a fix-build is triggered with the findings as context before proceeding to Validate.
- The fix-build runs at most once per iteration (no mid-build review ↔ fix-build retry loop).
- Mid-build review failure (LLM error, timeout) does not block the pipeline — the iteration proceeds to Validate and the failure is logged.
- Mid-build review findings are non-blocking suggestions — they never halt, skip, or restart an iteration.
- The existing post-validation Reviewer continues to run unchanged.
- Iteration logs include mid-build review metrics: ran (bool), finding count, fix-build triggered (bool), review duration/tokens, fix-build duration/tokens.

## Decisions

1. **Complement, not replace, the post-validation Reviewer.** The mid-build reviewer catches quick/obvious issues early. The post-validation reviewer does deeper analysis and creates beads for future iterations. Both serve different purposes at different points in the pipeline.

2. **Haiku tier for the mid-build review.** The review must be cheap and fast — its value comes from catching issues before they burn expensive validation cycles. A haiku call with spec + diff is the right cost/quality tradeoff.

3. **Include spec and acceptance criteria in the review prompt.** Without the spec, the reviewer can only catch code-quality issues (which validation already catches). Spec misalignment — building the wrong thing — is the most expensive class of error and the one validation cannot detect. The spec is the critical input.

4. **Non-blocking with no retry loop.** Keeping the review strictly non-blocking and single-pass avoids adding complexity to the pipeline's control flow. The mid-build reviewer is a lightweight quality gate, not a second build orchestrator.

5. **Single fix-build, no iteration.** If the fix-build doesn't fully address the findings, validation and the existing recovery loop handle the remainder. This prevents the mid-build review from becoming its own mini-orchestrator.

6. **Log findings for measurement.** The vision requires "documented verifiability" and "optimize for compounding improvements." Logging hit rate and save rate enables tuning the reviewer's prompt and deciding whether to invest further in this approach.

## Research & Context

### Current State

The orchestrator (`internal/runner/orchestrator.go`) runs a 5-stage pipeline per iteration: Gate → Build → Validate (with recovery loop) → Review → Epilogue. The Build and Review stages are strictly sequential with no feedback path between them during a single iteration.

The Reviewer (`internal/runner/reviewpkg/reviewer.go`) runs only after validation passes. Its findings create new beads for future iterations — feedback cannot influence the current build. This means obvious mistakes and spec misalignment survive through the entire validation+recovery cycle before being caught.

The validation recovery loop (`orchestrator.go:625-725`) retries Build+Validate up to `MaxValidationRetries` times on failure. Each retry is expensive (sonnet/opus tier). A cheap haiku pre-check that prevents even a fraction of these retries pays for itself quickly.

### Vision Alignment

This feature directly supports multiple VISION.md targets:

- **">=95% first integration pass"**: Catching spec misalignment before validation improves first-pass quality.
- **"<=10% human tactical intervention"**: Fewer escaped errors means fewer human corrections.
- **"Matching intent" guardrail**: The mid-build reviewer explicitly checks builder output against the spec.
- **"Cost efficiency" guardrail**: A cheap haiku call that prevents expensive validation retries is net cost-positive.
- **Design Principle 2 (compounding improvements)**: Every iteration benefits from a lightweight quality gate.
- **Design Principle 4 (evidence for correctness)**: Correctness evidence is gathered earlier in the pipeline.
