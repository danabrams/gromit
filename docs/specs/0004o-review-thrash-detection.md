# Spec 0004o — Review Thrash Detection

## spec_id
0004o-review-thrash-detection

## Vision
The review stage currently has no memory of its own prior verdicts. When a
reviewer fires the same error finding in consecutive cycles, the pipeline
treats it as a fresh signal each time and spawns another fix task — burning
budget on attempts that have already proven ineffective. A single stuck
finding can exhaust the entire budget without progress.

## Summary
When the review stage detects that the same error-severity finding has
blocked two consecutive cycles, it escalates the next fix attempt to the
high model tier, targeting only the task(s) responsible for that finding.
If the finding persists through the escalated attempt, the run is marked
blocked.

## Goals
### Primary
- A review finding that blocks in cycle N and again in cycle N+1 triggers
  targeted high-tier escalation for the fix task addressing it
- A finding that persists through the escalated attempt terminates the run
  as blocked
- Thrash counts are tracked in RunState and survive resume

### Secondary
- A `review_thrash_escalated` event is emitted when escalation fires, with
  the stuck finding details

## Non-goals
- Escalating the reviewer model tier (only the fix executor escalates)
- Tracking thrash across non-consecutive cycles (a finding that disappears
  then recurs resets its count)
- Detecting thrash in warning-severity findings (errors only)
- Changing how multiple independent stuck findings interact (each finding
  tracks independently; the most severe state wins)

## Architecture

### `RunState.ReviewThrashCounts`
New field on `RunState`:
```go
ReviewThrashCounts map[string]int `json:"review_thrash_counts,omitempty"`
```
Keyed by `file + "\x00" + description`. Counts consecutive cycles a finding
has blocked. Persisted in run.json. `NormalizeNilFields` maps nil →
`map[string]int{}`.

Reset semantics: counts are recomputed each review cycle from scratch. A
finding absent from the current blocking set has no entry in the new map
(effectively reset to 0). Consecutive semantics fall out naturally.

### `FailureContext.EscalatedFailures`
New field on `FailureContext`:
```go
EscalatedFailures []string `json:"escalated_failures,omitempty"`
```
Populated by the review stage when any finding reaches count == 2. Contains
the exact formatted failure strings (from `review.ReviewFailuresToStrings`)
for the thrashing findings. `NormalizeNilFields` maps nil → `[]string{}`.

### Review stage thrash logic
After computing `blockingFiltered` (the final set of blocking findings):

1. Build `newCounts map[string]int`: for each error-severity finding in
   `blockingFiltered`, `newCounts[fingerprint(f)] = rs.ReviewThrashCounts[fingerprint(f)] + 1`
2. Update `rs.ReviewThrashCounts = newCounts`
3. Partition by threshold: count >= 3 → blocked set; count == 2 → escalated set
4. If blocked set non-empty: return `specloop.Blocked`
5. If escalated set non-empty: emit `review_thrash_escalated`, return
   `ReplanFrom` with `EscalatedFailures` set to the failure strings for
   escalated findings
6. Otherwise: normal `ReplanFrom`

Fingerprint: `f.File + "\x00" + f.Description` (consistent with existing
`findingExists`).

### Execute stage escalation
After the existing `ShouldEscalateModel` check:
```go
if rs.ReplanContext != nil && taskIntersectsEscalated(&tasksToRun[i], rs.ReplanContext.EscalatedFailures) {
    tasksToRun[i].ModelTier = "high"
}
```
`taskIntersectsEscalated`: returns true if any string in
`task.FailuresAddressed` exactly matches any string in `escalatedFailures`.
Safe fallback: no match → no escalation.

### New event type
```go
type ReviewThrashEscalatedEvent struct {
    BaseEvent
    FindingFile        string `json:"finding_file"`
    FindingDescription string `json:"finding_description"`
    ConsecutiveCount   int    `json:"consecutive_count"`
}
```

## Acceptance Criteria

1. When a review finding with severity `error` blocks in cycle N and the
   same finding (same file + description) blocks again in cycle N+1,
   `rs.ReviewThrashCounts` for that finding's fingerprint is 2 after cycle
   N+1's review.

2. When `rs.ReviewThrashCounts` for a finding reaches 2, the review stage
   returns `ReplanFrom` with `EscalatedFailures` containing the failure
   string for that finding.

3. When `rs.ReviewThrashCounts` for a finding reaches 3, the review stage
   returns `Blocked` (run terminates; no further replan).

4. When the execute stage processes a replan with non-empty
   `EscalatedFailures`, any task whose `FailuresAddressed` exactly matches
   one of those strings has `ModelTier` set to `"high"`. Tasks whose
   `FailuresAddressed` does not intersect `EscalatedFailures` are
   unaffected.

5. A `review_thrash_escalated` event is emitted when escalation fires
   (count reaches 2), carrying the finding's file, description, and
   consecutive count.

6. When a finding that previously had a thrash count blocks again after
   being absent for at least one cycle, its count resets to 1 (no
   escalation carried over from the prior streak).

7. Warning-severity findings do not contribute to `ReviewThrashCounts`
   regardless of how many consecutive cycles they appear.

8. `ReviewThrashCounts` is persisted in run.json and survives resume; a
   resumed run continues counting from the prior streak.

9. All existing review stage tests continue to pass.

## Scenarios

### Scenario: Finding fixes on first attempt — no thrash
**Given:** A run in cycle 1 where the review stage returns one error finding
on `planner.go` (description: "buildFixPlanPrompt lacks X")
**When:** A fix task is created, executed, and the next review cycle finds
no blocking findings
**Then:** `ReviewThrashCounts` is empty after cycle 2's review; no
`review_thrash_escalated` event is emitted; the run continues normally

### Scenario: One repeat triggers escalation
**Given:** A run where the review stage returns the same error finding
(file: `planner.go`, description: "buildFixPlanPrompt lacks X") in cycle 1
and cycle 2
**When:** Cycle 2's review completes
**Then:**
- `ReviewThrashCounts["planner.go\x00buildFixPlanPrompt lacks X"]` == 2
- Review returns `ReplanFrom` with `EscalatedFailures` containing the
  formatted failure string for that finding
- A `review_thrash_escalated` event is emitted with `consecutive_count: 2`
- The fix task created for this finding has `ModelTier == "high"`
- Any other tasks in the same replan whose `FailuresAddressed` does not
  include the escalated string are unaffected

### Scenario: Escalation fails — run blocked
**Given:** The same error finding has blocked in cycles 1, 2, and 3
**When:** Cycle 3's review completes
**Then:**
- Review returns `Blocked`
- The run transitions to `status: blocked`
- A `terminal_state` event is emitted with `reason` referencing the stuck
  finding
- No further tasks are created

### Scenario: Count resets after finding disappears
**Given:** Finding F blocked in cycles 1 and 2 (count == 2, escalation
fired); the escalated fix resolves F; F is absent in cycle 3's review
**When:** Finding F reappears in cycle 4's review
**Then:**
- `ReviewThrashCounts["F"]` == 1 after cycle 4
- No escalation fires; normal `ReplanFrom`

## Validation
```
go test ./internal/next/specloop/stages/... -run TestReview
go test ./internal/next/specloop/...
go test ./internal/next/runstore/...
go vet ./...
```
