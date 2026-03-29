---
name: reviewing-gromit-runs
description: Use when reviewing a gromit-next run output to decide whether to accept, reject and start fresh, or resume with targeted implementation guidance. Triggered by "review the run", "should I accept this", "what did the run produce", or any gromit-next run evaluation task.
---

# Reviewing Gromit-Next Runs

Walk the user through a structured review of a gromit-next run and execute the chosen outcome via `gromit-next review guided`.

## Step 1: Locate the Run

**If the user provides a run ID** (format: `run-<8hex>`): use it directly.

**If the user says "the last run" or doesn't specify:**
```bash
gromit-next exec list
```
Pick the most recent. If multiple `ready_for_review` runs appear, confirm with user which one.

**To preview what the run produced before entering guided review:**
```bash
gromit-next review show [run-id]          # Human-readable summary
gromit-next review show [run-id] --details # Include linked technical artifacts
```

## Step 2: Read the Run and Form a Recommendation

Before entering the interactive review, read the artifacts so you can guide the user intelligently:

```
.gromit-next/runs/<run-id>/evidence/review.json      # ReviewPacket: findings, outcome
.gromit-next/runs/<run-id>/evidence/acceptance.json  # AcceptanceResults: per-criterion pass/fail
```

Form a recommendation:

| Situation | Recommendation |
|-----------|----------------|
| All acceptance criteria pass, no critical/high review findings | **Accept** |
| Specific criteria failed with a clear bounded gap | **Resume** (`rework_implementation_gap`) |
| Fundamental misalignment, wrong approach, or multiple unrelated failures | **Fresh run** (`rework_vision_change`) |

**STOP HERE and present your recommendation to the user.** Include:
- Your recommended outcome and why (cite acceptance counts, trust level, key findings)
- Any notable warnings or caveats from the review findings
- A summary of what the run produced

Then ask: **"How would you like to proceed — accept, rework (implementation gap), or fresh run?"**

Wait for the user's explicit answer before continuing to Step 3. Do not enter guided review autonomously.

## Step 3: Run the Guided Review

```bash
gromit-next review guided [run-id]
# omit run-id to use latest run
```

This interactive command:
1. Displays the product review
2. Steps through each manual checklist item — enter `pass`, `fail`, `unsure`, or `skipped` plus optional notes
3. Prompts for outcome: `accepted`, `rework_implementation_gap`, or `rework_vision_change`
4. Asks for a **summary** — this is where the gap description goes for rework outcomes
5. Writes `evidence/review-outcome.json` and triggers automatic distillation

**Enter the outcome the user chose in Step 2.** For the outcome prompt, type exactly what the user said (e.g., `accepted`). For checklist items with no instructions, enter `pass`. For the summary, write a factual description of the result.

## Step 4: Writing the Gap Description (for rework outcomes)

The **summary** field in `review guided` is how you communicate the implementation gap to the next run. Vague summaries produce another failed cycle.

**For `rework_implementation_gap`** — the summary must include:
1. Which acceptance criteria failed (quote the criterion text)
2. What was implemented — cite specific files and functions
3. What is missing or wrong — observable behavior vs expected behavior
4. Where to make changes — specific files, functions, or packages
5. Verification signal — what test or output proves it's fixed

**Bad summary (DO NOT use):**
> "The validation isn't working correctly. Please fix it."

**Good summary:**
> "Acceptance criterion 'Stale fix detection terminates the retry loop early' failed.
> `internal/runner/validation/validation.go:runValidationLoop` fires the stale-fix check at line 142 but never calls `setTerminated()` — it logs the detection and continues.
> Fix: after `isStale = true`, set `run.Terminated = true` and break from the loop.
> Verify: `go test -run TestIntegration_StaleFixDetectionEarlyTermination ./internal/runner/validation/...`"

**For `rework_vision_change`** — the summary should explain the directional misalignment: what the run produced, what was actually needed, and whether the spec needs to be updated before relaunching.

## Step 5: Execute the Follow-Through

### After `accepted`
```bash
gromit-next exec complete <run-id>   # Marks spec DONE, archives run
git add -p
git commit -m "..."
# Merge to main per normal project workflow
```

### After `rework_implementation_gap`
```bash
gromit-next exec --resume <run-id>   # Resume run, pipeline picks up PriorReviewFindings
```
The next cycle inherits prior review findings automatically. The summary you wrote in `review guided` is the gap instruction for the resumed session.

### After `rework_vision_change`
Consider updating the spec or acceptance criteria first, then:
```bash
gromit-next exec --spec <spec-path> --project <project>  # Fresh run
```

## Quick Reference

| Action | Command |
|--------|---------|
| Find last run | `gromit-next exec list` |
| Preview run | `gromit-next review show [run-id]` |
| Run guided review | `gromit-next review guided [run-id]` |
| Accept (post-review) | `gromit-next exec complete <run-id>` |
| Resume (post-review) | `gromit-next exec --resume <run-id>` |
| Fresh run | `gromit-next exec --spec <path> --project <name>` |
