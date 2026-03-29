# High-Leverage Pipeline Improvement Ideas

Based on analysis of the last 10 runs (2026-03-09 through 2026-03-10), all targeting `spec-level-review-and-targeted-remediation`. None succeeded. These three improvements address the root causes behind 9/10 failures.

---

## 1. Criterion Evaluation Reliability

**Problem:** The criterion evaluator is the terminal failure point in 8/10 runs. It calls an LLM to judge whether the implementation satisfies each acceptance criterion, passing the full cumulative diff (500KB–880KB) as context. This call hits the 15-minute deadline in 7 runs and returns unparseable Markdown instead of JSON in 1 run.

**Why this is highest leverage:** Every run successfully builds working code (0 validation failures during the bead loop). The pipeline dies at the evaluation step, not the implementation step. Fixing this unblocks the entire remediation loop.

### Sub-problems

**A. Evaluation timeout (7/10 runs)**

The evaluator sends the entire cumulative diff to a single LLM call per criterion. As remediation cycles add code, the diff grows monotonically (500K → 533K → 880K bytes), creating a death spiral: remediation adds code → diff grows → evaluation takes longer → timeout → run fails.

Proposed fixes:
- **Delta-diff evaluation**: Evaluate each criterion against only the changes made in the current remediation cycle, not the full cumulative diff. The evaluator already knows which criteria failed in the prior cycle — it only needs to check whether the new changes addressed those specific failures.
- **Per-criterion timeout scaling**: Instead of a flat 15-minute deadline for all criteria, scale the timeout based on diff size and criterion complexity. Simple criteria (type existence, flag presence) need seconds; complex criteria (end-to-end pipeline behavior) may need longer windows.
- **Incremental evaluation**: Evaluate criteria as remediation beads complete, not as a batch after all beads finish. This spreads the evaluation cost across the cycle and catches regressions earlier.

**B. Output format enforcement (1/10 runs)**

Run 2 (143039) failed because the LLM returned `## Criterion 1 Assessment: PASS` instead of a JSON object. The evaluator called `extract object` looking for `{` and got `#`.

Proposed fixes:
- **Structured output mode**: Use the LLM's JSON mode or tool_use to constrain output format at the API level, not via prompt instructions.
- **Fallback parser**: If JSON extraction fails, attempt to parse the freeform text for pass/fail signals before halting. A criterion that clearly says "PASS" in prose shouldn't kill the run.

**Relationship to existing specs:**
- `pipeline-timeout-config` makes decompose/review timeouts configurable but does not address criterion evaluation timeouts specifically.
- `enforce-timeout-first-decomposition` handles build timeouts with decomposition, not evaluation timeouts.
- `review-finding-routing-consistency` enforces JSON for review findings but not for criterion evaluation output.

**Gap:** No existing spec addresses criterion evaluation reliability as a first-class concern.

---

## 2. Remediation Regression Guard

**Problem:** Remediation cycles are non-monotonic. Run 5 reached 12/13 passing criteria on cycle 3, then regressed to 10/13 on cycle 4 — spending the final generation on a strictly worse state. Run 4 oscillated between 10/13 and 12/13 across 5 cycles. The pipeline has no mechanism to detect or prevent regression.

**Why this matters:** The generation cap (3) is the terminal event in the 2 runs that survived long enough to reach multiple accept cycles. Both runs got tantalizingly close (12/13) but wasted their final generation on a regression. A regression guard would have preserved the 12/13 state and either retried with a different strategy or granted an additional cycle.

### Proposed mechanism

**A. Best-state tracking**

After each accept cycle, record the pass count and the git commit SHA. Maintain a high-water mark across the remediation loop.

**B. Regression detection**

Before committing a remediation cycle's changes, compare the new pass count against the high-water mark. If the new count is strictly lower:
- **Do not commit** the regressive changes.
- **Revert** to the high-water-mark commit.
- **Log** which criteria regressed and which remediation beads likely caused the regression (by correlating the beads' touched files with the regressed criteria's scope).

**C. Strategy rotation on regression**

When regression is detected, don't simply re-run the same remediation plan. Instead:
- Narrow the remediation scope to only the still-failing criteria (don't re-fix what already passes).
- Exclude beads that touched files associated with the regressed criteria.
- If regression occurs twice in a row, escalate the remediation model tier.

**D. Adaptive generation cap**

- If pass count is strictly increasing across cycles, grant additional generations beyond the cap (up to a hard maximum).
- If pass count oscillates (up-down-up), halt early and report the best-achieved state rather than burning generations on instability.

**Relationship to existing specs:**
- `spec-level-review-and-targeted-remediation` addresses the root mechanism (targeted fix beads instead of full re-decomposition) but does not include a regression guard.
- `unstick-beads` handles permanently stuck individual beads, not accept-cycle-level regression.

**Gap:** No existing spec tracks best-state across remediation cycles or prevents regression.

---

## 3. Delta-Diff Scoped Evaluation

**Problem:** The criterion evaluator processes the full cumulative diff on every cycle. This is the underlying driver of both the timeout problem (#1) and the regression problem (#2). As remediation cycles accumulate, the diff grows from ~500KB to ~880KB. The LLM must re-read and re-reason about all prior changes to evaluate each criterion, even when only a small subset of files changed in the current cycle.

**Why this deserves its own treatment:** While idea #1 mentions delta-diff as a sub-fix for timeouts, the scoping problem is deeper than just performance. Full-diff evaluation causes the evaluator to hallucinate regressions that didn't happen (it re-interprets old code differently on each pass) and conflates changes from different cycles when assessing blame.

### Proposed mechanism

**A. Per-cycle change tracking**

At the start of each remediation cycle, record the git SHA. After remediation beads complete, compute the delta diff (current SHA vs cycle-start SHA). This is the "what changed this cycle" diff.

**B. Two-tier evaluation**

For each criterion:
1. **Fast path (delta-only):** If the criterion passed in the prior cycle and no files in its scope were touched in the current delta, carry forward the prior result without re-evaluation. This is safe because nothing relevant changed.
2. **Targeted evaluation:** If the criterion failed in the prior cycle, or files in its scope were touched, evaluate it against the delta diff plus a summary of the prior state (not the full cumulative diff). The summary provides context; the delta provides the signal.

**C. Criterion-to-file scope mapping**

During the decompose stage, record which files each criterion is expected to touch (many specs already include `expected_touched_area`). Use this mapping to determine which criteria need re-evaluation after each cycle. Criteria whose scope doesn't overlap with the delta can skip evaluation entirely.

**D. Cumulative fallback**

If targeted evaluation produces ambiguous results (the LLM can't determine pass/fail from the delta alone), fall back to full-diff evaluation for that specific criterion only. This keeps the expensive path as a fallback rather than the default.

### Expected impact

- **Timeout reduction:** Delta diffs are typically 10-50KB vs 500-880KB cumulative. Evaluation time drops proportionally.
- **Evaluation accuracy:** The LLM reasons about a focused change rather than re-interpreting 880KB of mixed old and new code.
- **Convergence improvement:** By not re-evaluating passed criteria against a growing diff, we eliminate the "hallucinated regression" failure mode where the evaluator changes its mind about previously-passing criteria.

**Relationship to existing specs:**
- `prompt-token-accounting-diagnostics` tracks token consumption but doesn't reduce it.
- `spec-level-review-and-targeted-remediation` generates targeted fix beads but still evaluates against the full diff.

**Gap:** No existing spec scopes evaluation input to per-cycle deltas.

---

## Implementation Priority

These three ideas form a dependency chain:

```
#3 Delta-Diff Scoped Evaluation
  └── enables #1 Criterion Evaluation Reliability (smaller input = fewer timeouts)
      └── enables #2 Remediation Regression Guard (reliable evaluation = meaningful pass counts)
```

However, #1 (structured output + timeout tuning) can be partially implemented independently and would unblock runs immediately. Recommended order:

1. **#1A+B** (output format enforcement + timeout scaling) — quick wins, unblock 8/10 failure modes
2. **#3** (delta-diff scoping) — structural fix that makes evaluation sustainable
3. **#2** (regression guard) — optimization that maximizes value from each generation

---

## Data Source

Analysis covered 10 consecutive runs from 2026-03-09 07:44 through 2026-03-10 12:10:

| Run | Outcome | Terminal Failure |
|-----|---------|-----------------|
| 20260309-114416 | FAILED | Criterion eval timeout (criterion 1) |
| 20260309-143039 | FAILED | JSON parse failure (criterion 1 returned Markdown) |
| 20260309-161430 | Unknown | Log truncated mid-accept |
| 20260309-195729 | FAILED | Criterion eval timeout (criterion 3) |
| 20260309-223240 | FAILED | Criterion eval timeout (criterion 6) |
| 20260310-013037 | FAILED | Generation cap reached (5 cycles, best 12/13) |
| 20260310-091900 | Unknown | Log ends mid-remediation bead loop |
| 20260310-112139 | Aborted | Silent startup failure (3-line log) |
| 20260310-112234 | Aborted | Silent startup failure (3-line log) |
| 20260310-113201 | FAILED | Generation cap reached (4 cycles, best 12/13) |
