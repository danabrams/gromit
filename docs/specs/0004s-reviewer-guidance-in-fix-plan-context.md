# Spec 0004s — Reviewer Guidance in Fix Plan Context

## spec_id
reviewer-guidance-in-fix-plan-context

## Vision
When a human reviewer records a `rework_implementation_gap` outcome with specific instructions,
those instructions go nowhere. They're written to `review-outcome.json` but the planner never
reads that file. The pipeline resumes and generates its own fix tasks based only on automated
review findings, ignoring the human's explicit guidance entirely. A reviewer who writes "fix the
bypass test at line 173" watches the pipeline add more unit tests instead. Human expertise in the
review loop should have a direct path to the planner.

## Summary
When `plan.go` builds a `FixPlanRequest` on a resumed run, it reads `review-outcome.json` from
the evidence directory and — if the outcome is `rework_implementation_gap` or
`rework_vision_change` — injects the reviewer's `summary` as a `ReviewerGuidance` field. The
planner surfaces this as a prominent "Reviewer Instructions" section in `buildFixPlanPrompt`,
placed before the automated review findings, so the human's specific instructions take precedence
over the pipeline's own judgment.

## Goals
### Primary
- Reviewer's summary from `gromit-next review record` is visible to the planner on every fix
  cycle after resume
- No RunState schema changes required — `review-outcome.json` is read directly at plan-build time

### Secondary
- Reviewer instructions appear before automated review findings in the prompt

## Non-goals
- Structured parsing of reviewer guidance (passed as free-form string)
- Clearing guidance after one fix cycle
- Changes to the `review record` CLI or `ReviewOutcome` struct
- Support for stacking multiple sequential review outcomes

## Architecture
The change touches three files: `plan.go` (read + inject), `planner/types.go` (new field), and
`planner/planner.go` (prompt section).

**Reading the outcome (`internal/next/specloop/stages/plan.go`):**

```go
// loadReviewerGuidance reads reviewer instructions from evidence if this is a rework resume.
// Returns empty string if file is absent, unreadable, or outcome is not a rework type.
func loadReviewerGuidance(evidenceDir string) string {
    data, err := os.ReadFile(filepath.Join(evidenceDir, "review-outcome.json"))
    if err != nil {
        return ""
    }
    var outcome reviewsession.ReviewOutcome
    if err := json.Unmarshal(data, &outcome); err != nil {
        return ""
    }
    if outcome.Outcome != "rework_implementation_gap" && outcome.Outcome != "rework_vision_change" {
        return ""
    }
    return outcome.Summary
}
```

Called when constructing `FixPlanRequest`, conditioned on `rs.Resumed`:

```go
req := planner.FixPlanRequest{
    // ... existing fields ...
    ReviewerGuidance: loadReviewerGuidance(evidenceDir),
}
```

**New field (`internal/next/planner/types.go`):**

```go
type FixPlanRequest struct {
    // ... existing fields ...
    ReviewerGuidance string // Human reviewer instructions from review-outcome.json; empty if none
}
```

**Prompt section (`internal/next/planner/planner.go`, `buildFixPlanPrompt`), inserted before
"Review Findings to Fix":**

```
## Reviewer Instructions
The human reviewer has provided the following specific instructions.
Address these directly when generating tasks — they take priority over
the automated findings below:

{ReviewerGuidance}
```

Only rendered when `ReviewerGuidance != ""`.

**No changes to:** `RunState`, `runstore/types.go`, `exec.go` resume flow, `review record` CLI,
or `ReviewOutcome` struct. The `review-outcome.json` file is already preserved across resume by
`exec.go:280`.

## Acceptance Criteria
1. When a run resumes after a `rework_implementation_gap` outcome, the fix plan prompt contains a
   "Reviewer Instructions" section with the reviewer's summary text.
2. When a run resumes after a `rework_vision_change` outcome, the fix plan prompt contains a
   "Reviewer Instructions" section with the reviewer's summary text.
3. When a run resumes after an `accepted` outcome, no "Reviewer Instructions" section appears in
   the fix plan prompt.
4. When no `review-outcome.json` exists in the evidence directory, the plan stage proceeds without
   error and no "Reviewer Instructions" section appears.
5. The "Reviewer Instructions" section appears before the "Review Findings to Fix" section in the
   fix plan prompt.
6. All existing planner and plan-stage tests continue to pass.

## Scenarios

### Scenario: Rework resume injects reviewer guidance into fix plan
**Given:** A resumed run with `review-outcome.json` in evidence containing
`outcome: "rework_implementation_gap"` and `summary: "Fix the bypass test at
exec_scenario_escalation_fails_run_blocked_test.go:173 — remove the manual Blocked override and
drive through the real count>=3 path"`
**When:** The plan stage runs on cycle 2 (fix cycle, `rs.Resumed == true`)
**Then:** The fix plan prompt contains a "Reviewer Instructions" section with the exact summary
text, appearing before any "Review Findings to Fix" content

### Scenario: Vision-change resume injects reviewer guidance
**Given:** A resumed run with `review-outcome.json` containing `outcome: "rework_vision_change"`
and a non-empty summary
**When:** The plan stage runs
**Then:** The fix plan prompt contains a "Reviewer Instructions" section with the summary text

### Scenario: Accepted outcome produces no reviewer guidance
**Given:** A resumed run with `review-outcome.json` containing `outcome: "accepted"`
**When:** The plan stage runs
**Then:** The fix plan prompt does NOT contain a "Reviewer Instructions" section

### Scenario: Missing review-outcome.json is silent
**Given:** A run with no `review-outcome.json` in the evidence directory
**When:** The plan stage runs
**Then:** Plan proceeds normally, no error, no "Reviewer Instructions" section in the prompt

### Scenario: Guidance persists across multiple rework cycles
**Given:** A resumed run with `rework_implementation_gap` outcome; cycle 2 fails and triggers
cycle 3
**When:** The plan stage runs for cycle 3
**Then:** The fix plan prompt for cycle 3 also contains "Reviewer Instructions" — guidance
persists because the file is re-read each cycle

## Validation
```
go test ./internal/next/specloop/stages/... -run TestPlan -count=1
go test ./internal/next/planner/... -count=1
go build ./...
```
