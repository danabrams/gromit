# Spec 0004r — Delta-Diff Scoped Evaluation

## spec_id
0004r-delta-diff-scoped-evaluation

## Vision

The evaluator processes the full cumulative diff (500KB-880KB) on every cycle.
This drives both timeouts and hallucinated regressions. As remediation cycles
accumulate code, the diff grows monotonically — a death spiral: remediation
adds code, diff grows, evaluation takes longer, timeout, run fails. Evaluating
only what changed per cycle is the structural fix.

## Summary

Track per-cycle git SHAs, compute delta diffs, and implement two-tier
evaluation: (1) fast-path carry-forward for criteria whose file scope wasn't
touched, (2) targeted evaluation against the delta diff for touched/failed
criteria, with cumulative fallback on ambiguity.

## Goals
### Primary
- The accept stage records the git SHA at cycle start and computes a delta diff
  after remediation, replacing the full cumulative diff as the default
  evaluation input
- Criteria that passed in the prior cycle and whose file scope was not touched
  by the current delta are carried forward without an LLM call
- Criteria that failed, were unclear, or whose file scope overlaps the delta
  are evaluated against the delta diff (not the full cumulative diff)
- When targeted evaluation returns `unclear`, the evaluator falls back to the
  full cumulative diff for that criterion only

### Secondary
- An `acceptance_evaluation_mode` event is emitted per criterion, recording
  whether it used fast-path, targeted, or fallback evaluation
- Evaluation mode counts (fast-path / targeted / fallback) are included in
  the `acceptance_result` event

## Non-goals
- Criterion-to-file scope mapping from the decompose stage (deferred to a
  future spec; this spec uses `CriterionResult.EvidenceRefs` from prior
  evaluation results to approximate which files a criterion depends on)
- Incremental evaluation during bead execution (evaluation remains a batch
  after all beads complete)
- Changing the evaluation prompt format or JSON schema (the existing
  `AcceptancePromptInput` template is preserved; only which diff populates
  `DiffSummary` changes)
- Per-criterion timeout scaling (orthogonal; can layer on top)

## Architecture

### New `RunState` fields

```go
CycleStartSHA        string                                `json:"cycle_start_sha,omitempty"`
PriorCriterionResults map[string]acceptor.CriterionResult   `json:"prior_criterion_results,omitempty"`
```

`CycleStartSHA` is the git HEAD SHA recorded at the start of each cycle.
`PriorCriterionResults` is keyed by criterion text and holds the most recent
evaluation result for each criterion. The type is `acceptor.CriterionResult`
(no circular dependency: `runstore` does not import `acceptor` today, and
`acceptor` does not import `runstore`). `NormalizeNilFields` maps nil →
`map[string]acceptor.CriterionResult{}`.

These fields persist in `run.json` and survive resume. `ResetForNewCycle` does
NOT clear them — they are cross-cycle state like `TaskLineage`.

### New `DiffProvider` method

Add a `DeltaDiff` method to the `review.DiffProvider` interface:

```go
type DiffProvider interface {
    Diff(baseBranch string) (string, error)
    DeltaDiff(fromSHA string) (string, error)
}
```

`DeltaDiff` computes `git diff <fromSHA> HEAD` in the worktree directory.
`GitDiffProvider` implements this by running `git diff <fromSHA> HEAD -- . :!.gromit-next`.

Callers that only implement `Diff` (e.g., test fakes) can embed a
`NoDeltaDiff` stub that returns `("", ErrDeltaDiffUnsupported)`, signaling the
accept stage to fall back to cumulative diff for all criteria.

### SHA capture in `specloop.Run`

In `specloop.Run`, immediately after `ResetForNewCycle(rs)` and before running
stages:

```go
if sha, err := gitHeadSHA(rs.WorktreePath); err == nil {
    rs.CycleStartSHA = sha
}
```

`gitHeadSHA` runs `git rev-parse HEAD` in the worktree. On error (e.g., no
worktree yet on cycle 1), `CycleStartSHA` remains empty and the accept stage
falls back to cumulative diff.

### Delta diff computation in `AcceptStage.Run`

In `accept.go`, after the existing cumulative diff computation:

```go
var deltaDiff string
var deltaFiles []string
if rs.CycleStartSHA != "" && s.cfg.DiffProvider != nil {
    dd, err := s.cfg.DiffProvider.DeltaDiff(rs.CycleStartSHA)
    if err == nil && dd != "" {
        deltaDiff = dd
        deltaFiles = parseDiffFiles(dd) // extract file paths from diff headers
    }
}
```

`parseDiffFiles` extracts file paths from `diff --git a/... b/...` headers in
the delta diff output. This is the set of files touched in the current cycle.

### Two-tier evaluation logic

Replace the current single-pass `for _, criterion := range input.Criteria`
loop in `Evaluator.Evaluate` with:

```go
type EvaluateInput struct {
    Criteria             []string
    DiffSummary          string   // cumulative diff (existing)
    DeltaDiffSummary     string   // delta diff (new)
    DeltaFiles           []string // files touched in delta (new)
    TaskResults          string
    ValidationResults    string
    ReviewFindings       string
    PriorResults         map[string]CriterionResult // prior cycle results (new)
}
```

Per-criterion logic:

1. **Fast path**: If `PriorResults[criterion].Status == StatusPass` AND
   `DeltaDiffSummary != ""` AND none of `DeltaFiles` overlap with
   `PriorResults[criterion].EvidenceRefs` → carry forward the prior result.
   Set `cr.EvalMode = "fast_path"`. No LLM call.

2. **Targeted**: If `DeltaDiffSummary != ""` → render prompt with
   `DiffSummary = DeltaDiffSummary` (the delta, not cumulative). Call
   `agent.EvaluateCriterion`. Set `cr.EvalMode = "targeted"`.

3. **Fallback**: If targeted evaluation returns `StatusUnclear` AND
   `DiffSummary != ""` → re-render prompt with `DiffSummary = DiffSummary`
   (the full cumulative diff). Call `agent.EvaluateCriterion` again. Set
   `cr.EvalMode = "fallback"`.

4. **Degrade gracefully**: If `DeltaDiffSummary == ""` (no SHA captured, delta
   computation failed, or cycle 1) → use cumulative diff as today. Set
   `cr.EvalMode = "cumulative"`.

### New `CriterionResult` field

```go
EvalMode string `json:"eval_mode,omitempty"` // fast_path, targeted, fallback, cumulative
```

### Storing results for next cycle

After evaluation completes, in `AcceptStage.Run`:

```go
rs.PriorCriterionResults = make(map[string]acceptor.CriterionResult, len(result.Results))
for _, cr := range result.Results {
    rs.PriorCriterionResults[cr.Criterion] = cr
}
```

### File-scope overlap detection

`criterionScopeOverlapsDelta(criterion CriterionResult, deltaFiles []string) bool`
(in the `acceptor` package, alongside `Evaluator.Evaluate`):
Returns true if any path in `criterion.EvidenceRefs` shares a common prefix
with any path in `deltaFiles`. This is a conservative heuristic — if the
criterion's evidence refs mention `internal/next/acceptor/evaluator.go` and
the delta touches `internal/next/acceptor/types.go`, the package-level overlap
triggers re-evaluation. An empty `EvidenceRefs` always returns true (cannot
prove non-overlap).

### New event types

```go
type AcceptanceEvalModeEvent struct {
    BaseEvent
    Criterion string `json:"criterion"`
    EvalMode  string `json:"eval_mode"` // fast_path, targeted, fallback, cumulative
}
```

The existing `AcceptanceResultEvent` gains three new fields:

```go
FastPathCount  int `json:"fast_path_count"`
TargetedCount  int `json:"targeted_count"`
FallbackCount  int `json:"fallback_count"`
```

### Files in scope

- `internal/next/runstore/types.go` — new RunState fields, NormalizeNilFields
- `internal/next/runstore/store.go` — ResetForNewCycle unchanged (new fields persist)
- `internal/next/runstore/events.go` — new event type, extended AcceptanceResultEvent
- `internal/next/review/diff.go` — DeltaDiff method on DiffProvider/GitDiffProvider
- `internal/next/acceptor/types.go` — EvalMode field on CriterionResult
- `internal/next/acceptor/evaluator.go` — two-tier evaluation logic, new EvaluateInput fields, scope overlap detection
- `internal/next/acceptor/prompt.go` — no changes (template unchanged)
- `internal/next/specloop/specloop.go` — SHA capture at cycle start
- `internal/next/specloop/stages/accept.go` — delta diff computation, parseDiffFiles, result storage

All other files are out of scope. Rework tasks must not touch them.

## Acceptance Criteria

1. At the start of each cycle (after `ResetForNewCycle`), `rs.CycleStartSHA`
   is set to the current git HEAD SHA of the worktree. If the worktree is
   unavailable (cycle 1, no worktree), `CycleStartSHA` is empty and evaluation
   degrades to cumulative diff.

2. When `CycleStartSHA` is non-empty and `DiffProvider.DeltaDiff` succeeds,
   the accept stage computes a delta diff and extracts the list of touched
   files from diff headers.

3. When a criterion passed in the prior cycle (`PriorCriterionResults` has
   status `pass`) and none of the delta files overlap with the criterion's
   `EvidenceRefs`, the criterion result is carried forward with
   `EvalMode == "fast_path"` and no LLM call is made.

4. When a criterion requires evaluation (failed/unclear in prior cycle, or file
   scope overlaps delta), the LLM receives the delta diff as `DiffSummary` —
   not the full cumulative diff. The result has `EvalMode == "targeted"`.

5. When targeted evaluation returns `unclear` and the full cumulative diff is
   available, the evaluator retries that criterion with the cumulative diff.
   The result has `EvalMode == "fallback"`.

6. After evaluation completes, `rs.PriorCriterionResults` contains the result
   for every criterion (keyed by criterion text), persisted in `run.json` and
   available to the next cycle.

7. The `acceptance_result` event includes `fast_path_count`, `targeted_count`,
   and `fallback_count` fields reflecting how many criteria used each path.

8. All existing acceptance evaluator tests continue to pass — the two-tier
   logic is additive and degrades to cumulative-only when `DeltaDiffSummary`
   is empty.

## Scenarios

### Scenario: Cycle 1 — no prior results, cumulative diff used
**Given:** A run in cycle 1 with no `PriorCriterionResults` and
`CycleStartSHA` is empty (worktree was just created)
**When:** The accept stage evaluates 3 criteria
**Then:** All 3 criteria are evaluated via LLM with the cumulative diff;
`EvalMode` is `"cumulative"` for each; `PriorCriterionResults` is populated
with all 3 results after evaluation

### Scenario: Cycle 2 — passed criteria with no file overlap skip LLM
**Given:** Cycle 1 evaluated criteria A (pass, evidence_refs: `["internal/next/acceptor/types.go"]`),
B (fail), C (pass, evidence_refs: `["internal/next/review/diff.go"]`). Cycle 2
delta diff touches only `internal/next/review/diff.go`.
**When:** The accept stage evaluates criteria A, B, C in cycle 2
**Then:**
- Criterion A: carried forward as pass, `EvalMode == "fast_path"`, no LLM call
- Criterion B: evaluated against delta diff, `EvalMode == "targeted"`
- Criterion C: evaluated against delta diff (file overlap), `EvalMode == "targeted"`
- `acceptance_result` event has `fast_path_count: 1, targeted_count: 2`

### Scenario: Targeted evaluation unclear triggers fallback
**Given:** Cycle 2, criterion B failed in cycle 1, delta diff is 15KB,
cumulative diff is 500KB. Targeted evaluation of B returns `status: "unclear"`.
**When:** Fallback fires for criterion B
**Then:** B is re-evaluated with the full cumulative diff as `DiffSummary`;
if the second evaluation returns `pass` or `fail`, that result is used with
`EvalMode == "fallback"`; the `acceptance_result` event has `fallback_count: 1`

### Scenario: Delta diff unavailable — graceful degradation
**Given:** `CycleStartSHA` is non-empty but `DeltaDiff` returns an error
(e.g., SHA was garbage-collected)
**When:** The accept stage runs
**Then:** All criteria are evaluated with the cumulative diff; `EvalMode` is
`"cumulative"` for each; no error is returned — the stage completes normally

## Validation
```bash
go test ./internal/next/acceptor/...
go test ./internal/next/specloop/stages/... -run TestAccept
go test ./internal/next/review/...
go test ./internal/next/runstore/...
go test ./internal/next/specloop/...
go vet ./...
```
