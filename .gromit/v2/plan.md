Now I have enough context to produce the implementation plan. The Fragment instructs me to output only the plan content directly.

---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-09
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

**Goal:** Add a spec-level holistic code review stage after the bead loop, replace gap-analysis-based remediation with structured findings-based targeted decomposition, and add `--from-review` execution mode to run deferred fix beads.

**Architecture:** A new `internal/v2/stage/specreview/` package implements the post-bead-loop review stage, producing structured `SpecFinding` values (severity/category/scope) that flow through a shared type in `internal/v2/stage/stage.go`. The spec loop combines accept and spec-review verdicts, passes findings to the remediation runner via `StageRequest.Findings`, and the decompose stage gains a third prompt template that targets findings rather than re-decomposing the original plan.

**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

**New package:** `internal/v2/stage/specreview/` — implements `Stage` interface; receives cumulative `DiffFromBase`, plan text, and project context; invokes highest-tier LLM with `review_spec_v2.md` fragment; parses JSON findings; returns `SpecReviewArtifacts` (verdict + findings + created from-review beads). Depends on `internal/v2/trackertypes` for bead creation.

**Shared finding type in `stage.go`:** `SpecFinding` struct (severity, category, scope, description, affected_files). Added to `StageRequest.Findings []SpecFinding` so findings flow from accept/specreview through spec_loop to remediation to decompose without circular imports.

**Integration points:**
- `spec_loop.go`: after `runBeadLoop`, run accept then spec-review; if either fails, collect findings and trigger remediation with `req.Findings` populated.
- `remediation.go`: `executeRemediation` receives findings slice instead of gap-analysis string; sets `req.Findings` on the decompose request.
- `decompose.go`: third template `findingsDecomposePromptTemplate` activates when `req.Findings` is non-empty (overrides remediation gap-analysis path).
- `run2_components.go`: loads `review_spec_v2.md`; wires specreview stage with `TierHigh` forced routing.
- `run2.go`: `--from-review` flag queries beads by `from-review` label, bypasses plan/decompose/accept/review.

**Stage sequence addition:** `StageSequence` gains `"spec-review"` between `"accept"` and `"present"`.

## Test Strategy

**Unit tests** in `specreview/specreview_test.go`: verdict logic (critical→fail, warning/suggestion→pass), JSON parsing (valid/malformed), from-review bead labeling (scope=spec vs scope=general), nil/empty-findings cases.

**Unit tests** in `accept/accept_test.go`: structured findings emitted for each failed criterion (severity critical, category acceptance, scope spec).

**Unit tests** in `decompose/decompose_test.go`: findings template activates when `req.Findings` non-empty; gap-analysis template still works when `req.GapAnalysis` set and `req.Findings` empty.

**Unit tests** in `remediation/remediation_test.go`: findings passed through to decompose request; gap analysis no longer required.

**Unit tests** in `loop/spec_loop_test.go`: spec-review stage called after bead loop; combined verdict gates present stage; findings collected from both accept and spec-review.

**Integration test** for full post-bead-loop pipeline: bead loop → accept (fail) + spec-review (fail) → findings-based decompose → targeted beads. Integration test for pass-with-improvements: spec-review passes with warnings → from-review beads created → spec proceeds to present.

Mocking strategy: use existing `mockStage` pattern (`stagepkg.Stage` interface); tracker mock for bead creation; git mock for `DiffFromBase`.

## Implementation Tasks

### Task 1: SpecFinding shared type and StageRequest field

**Files:**
- Modify: `internal/v2/stage/stage.go`

**What to Do:**
Add `SpecFinding` struct and `SpecFindingSeverity`/`SpecFindingCategory`/`SpecFindingScope` string constants to `stage.go`. Add `Findings []SpecFinding` field to `StageRequest`. These types must be defined in the stage package to avoid import cycles — both spec_loop and decompose import stage, so no new package is needed.

```go
type SpecFindingSeverity = string
const (
    SpecFindingSeverityCritical   SpecFindingSeverity = "critical"
    SpecFindingSeverityWarning    SpecFindingSeverity = "warning"
    SpecFindingSeveritySuggestion SpecFindingSeverity = "suggestion"
)

type SpecFindingCategory = string
const (
    SpecFindingCategoryBug          SpecFindingCategory = "bug"
    SpecFindingCategorySecurity     SpecFindingCategory = "security"
    SpecFindingCategoryQuality      SpecFindingCategory = "quality"
    SpecFindingCategoryTestGap      SpecFindingCategory = "test-gap"
    SpecFindingCategoryArchitecture SpecFindingCategory = "architecture"
    SpecFindingCategoryAcceptance   SpecFindingCategory = "acceptance"
)

type SpecFindingScope = string
const (
    SpecFindingScopeSpec    SpecFindingScope = "spec"
    SpecFindingScopeGeneral SpecFindingScope = "general"
)

type SpecFinding struct {
    Severity      SpecFindingSeverity `json:"severity"`
    Category      SpecFindingCategory `json:"category"`
    Scope         SpecFindingScope    `json:"scope"`
    Description   string              `json:"description"`
    AffectedFiles []string            `json:"affected_files"`
}
```

Add `Findings []SpecFinding` to `StageRequest` after `GapAnalysis string`.

**Acceptance Criteria:**
1. `stage.SpecFinding` is defined with all required fields and constants
2. `StageRequest.Findings` compiles and is zero-value nil (no breaking changes)
3. `go test ./internal/v2/stage/...` passes with no new failures

**Dependencies:** None

---

### Task 2: SpecReview stage core — types, JSON parsing, verdict logic

**Files:**
- Create: `internal/v2/stage/specreview/specreview.go`
- Create: `internal/v2/stage/specreview/specreview_test.go`

**What to Do:**
Create the package with:
- `SpecReviewArtifacts` struct: `Verdict string`, `Findings []stage.SpecFinding`, `CreatedBeads []*trackertypes.Bead`
- `verdictFromFindings(findings []stage.SpecFinding) string` — returns `"fail"` if any `severity == "critical"`, else `"pass"`
- JSON parsing: `parseSpecReviewOutput(output string) (string, []stage.SpecFinding, error)` — extracts `{"verdict": ..., "findings": [...]}` using `jsonutil.ExtractObject`
- `Stage` struct with `name`, `cfg`, `git GitDiffer`, `llm`, `tracker`, `base`, `project`, `fragment` fields
- `New(...)` constructor with nil checks on cfg/git/llm/tracker
- `Name() string` and `var _ stagepkg.Stage = (*Stage)(nil)`

The `GitDiffer` interface needs `DiffFromBase(ctx, worktree) (string, error)`.

Write tests: verdict from critical finding → fail; verdict from only warnings → pass; verdict from empty findings → pass; JSON parse valid input; JSON parse missing fields defaults gracefully.

**Acceptance Criteria:**
1. `verdictFromFindings` returns `"fail"` for any critical severity, `"pass"` otherwise
2. `parseSpecReviewOutput` extracts verdict and findings array from valid JSON
3. `go test ./internal/v2/stage/specreview/...` passes

**Dependencies:** Task 1

---

### Task 3: SpecReview stage Run() — LLM invocation and prompt assembly

**Files:**
- Modify: `internal/v2/stage/specreview/specreview.go`
- Modify: `internal/v2/stage/specreview/specreview_test.go`

**What to Do:**
Implement `Run(ctx, req)`:
1. Get cumulative diff via `s.git.DiffFromBase(ctx, root)`
2. Read plan from `<worktree>/.gromit/v2/plan.md`
3. Build instance layer: `"## Cumulative Diff\n<diff>\n\n## Original Plan\n<plan>"`
4. Assemble prompt via `prompt.NewPromptAssembler(s.base, s.project, instance, s.fragment).Assemble("spec-review", prompt.BeadInfo{})`
5. Resolve provider from `req.Provider` or `s.llm`; resolve model from `req.Model` (always forced to high tier by wiring, so if empty use `req.Model` from routing)
6. Invoke LLM, parse output with `parseSpecReviewOutput`
7. Override verdict: any critical finding forces `"fail"`; otherwise use parsed verdict
8. Return `DecisionProceed` with artifacts when verdict=pass, `DecisionFail` when verdict=fail

Write tests using mock LLM provider: critical finding → DecisionFail; warning only → DecisionProceed with artifacts; malformed JSON → error propagated.

**Acceptance Criteria:**
1. `Run()` returns `DecisionFail` when parsed findings contain any critical severity
2. `Run()` returns `DecisionProceed` when no critical findings
3. `SpecReviewArtifacts.Findings` populated from parsed JSON

**Dependencies:** Task 2

---

### Task 4: review_spec_v2.md prompt fragment

**Files:**
- Create: `review_spec_v2.md` (project root)

**What to Do:**
Create the spec-level review prompt fragment. It should instruct the LLM to evaluate the cumulative diff holistically against these dimensions: correctness, security (OWASP top 10), error handling, test coverage gaps, code quality, architectural fit. Output format must be the structured JSON:

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

Include rules: critical → verdict fail; scope=spec for issues in files changed by this spec; scope=general for pre-existing issues unrelated to spec changes. Respond with ONLY the JSON object.

**Acceptance Criteria:**
1. File exists at `review_spec_v2.md` in project root
2. Fragment includes output format with all required fields
3. Fragment explains scope distinction between spec and general findings

**Dependencies:** None

---

### Task 5: Accept stage structured findings output

**Files:**
- Modify: `internal/v2/stage/accept/accept.go`
- Modify: `internal/v2/stage/accept/accept_test.go`

**What to Do:**
Add `Findings []stage.SpecFinding` to `AcceptArtifacts`. After collecting `failures`, convert each to a `SpecFinding`:

```go
for _, criterion := range criteria {
    if !pass {
        findings = append(findings, stage.SpecFinding{
            Severity:    stage.SpecFindingSeverityCritical,
            Category:    stage.SpecFindingCategoryAcceptance,
            Scope:       stage.SpecFindingScopeSpec,
            Description: fmt.Sprintf("Criterion %d failed: %s — %s", criterion.Number, trimmed, summary),
        })
    }
}
artifacts.Findings = findings
```

Write tests: two failing criteria → two critical acceptance findings in artifacts; all pass → empty findings.

**Acceptance Criteria:**
1. `AcceptArtifacts.Findings` populated with one `SpecFinding` per failed criterion
2. Each finding has `severity=critical`, `category=acceptance`, `scope=spec`
3. `go test ./internal/v2/stage/accept/...` passes

**Dependencies:** Task 1

---

### Task 6: Decompose stage findings-based prompt template

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go`
- Modify: `internal/v2/stage/decompose/decompose_test.go`

**What to Do:**
Add `findingsDecomposePromptTemplate` constant — similar to `remediationDecomposePromptTemplate` but takes serialized findings JSON instead of prose gap analysis. In `Run()`, update the template selection logic:

```go
if req.Remediation && len(req.Findings) > 0 {
    findingsJSON, _ := json.Marshal(req.Findings)
    promptText = fmt.Sprintf(findingsDecomposePromptTemplate, specID, string(planBody), string(findingsJSON), skills.DecomposeSkill, specID)
} else if req.Remediation && gapAnalysis != "" {
    promptText = fmt.Sprintf(remediationDecomposePromptTemplate, specID, string(planBody), gapAnalysis, skills.DecomposeSkill, specID)
} else {
    promptText = fmt.Sprintf(s.promptTemplate, specID, string(planBody), skills.DecomposeSkill, specID)
}
```

The findings template should instruct: "create targeted fix beads for these specific findings, do not re-implement already-completed work."

Write tests: `req.Findings` non-empty + `req.Remediation=true` → findings template selected; gap analysis still selected when findings empty and gap non-empty.

**Acceptance Criteria:**
1. Findings template selected when `req.Findings` non-empty and `req.Remediation` true
2. Gap-analysis template still selected when findings empty and gap analysis non-empty
3. Default template selected when neither remediation condition is met

**Dependencies:** Task 1

---

### Task 7: Spec loop integration — specReviewStage field and option

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`
- Modify: `internal/v2/loop/spec_loop_test.go`

**What to Do:**
Add `specReviewStage stagepkg.Stage` field to `SpecLoop`. Add `WithSpecReviewStage(stage stagepkg.Stage) SpecLoopOption`. Add `"spec-review"` to `StageSequence` after `"accept"`.

In `Run()`, after the bead loop and before `ensureAcceptance`, restructure the post-loop phase:
- Run accept (as before, but now collect its findings)
- Run spec-review (new): `s.runSpecReview(ctx, &req, specID)`
- Gate: succeed only when both pass; when either fails, collect findings from both and call remediation with findings populated in req

Extract findings from `AcceptArtifacts.Findings` and from `SpecReviewArtifacts.Findings`, merge, set on req before calling `remediationRunner.Run`.

The `remediationRunner` interface needs to change to accept findings: `Run(ctx, specID, worktree string, findings []stagepkg.SpecFinding) error`. Update the `remediationRunner` interface in spec_loop.go and the call site.

Write tests: spec-review stage called after bead loop; combined findings passed to remediation runner; spec proceeds when both accept and spec-review return DecisionProceed.

**Acceptance Criteria:**
1. `WithSpecReviewStage` option installs specReviewStage
2. Spec-review runs after bead loop; combined findings trigger remediation when either fails
3. Spec proceeds to present when accept=pass AND spec-review=pass

**Dependencies:** Tasks 2, 3, 5

---

### Task 8: Remediation runner findings-based input

**Files:**
- Modify: `internal/v2/remediation/remediation.go`
- Modify: `internal/v2/remediation/remediation_test.go`

**What to Do:**
Change `Run` signature to accept findings: `Run(ctx context.Context, specID, worktree string, findings []stage.SpecFinding) error`. In `executeRemediation`, set `req.Findings = findings` before calling decompose. Keep gap-analysis fallback: if findings empty and gap analysis file exists, read it and set `req.GapAnalysis` (backward compat for tests/legacy path).

Remove the internal accept re-run from `RemediationRunner.Run()` — the spec loop now controls accept/review; the runner only runs decompose + bead loop. This simplifies the runner significantly: it receives findings, decomposes, runs beads, returns.

Update `RemediationRunnerConfig` to remove `AcceptStage` field (no longer needed). Update call site in `run2_components.go`.

Write tests: findings passed to decompose request; generation cap still applies; bead runner called with decomposed beads.

**Acceptance Criteria:**
1. `RemediationRunner.Run` accepts `[]stage.SpecFinding` parameter
2. Non-empty findings set on decompose request as `req.Findings`
3. `AcceptStage` removed from `RemediationRunnerConfig` (no re-run accept inside runner)

**Dependencies:** Tasks 1, 6

---

### Task 9: SpecReview — from-review bead creation

**Files:**
- Modify: `internal/v2/stage/specreview/specreview.go`
- Modify: `internal/v2/stage/specreview/specreview_test.go`

**What to Do:**
When verdict=pass but findings non-empty, create beads in `Run()`. Add `tracker trackertypes.TaskTracker` to `Stage` (already in `New()` signature from Task 2).

For each finding when verdict=pass:
- `scope=spec` → labels: `["from-review", "spec:<specID>"]`
- `scope=general` → labels: `["from-review"]`

Call `s.tracker.CreateBead(...)` for each. Populate `SpecReviewArtifacts.CreatedBeads`.

When verdict=fail, do NOT create from-review beads (remediation handles the work).

Write tests using mock tracker: pass verdict with spec-scoped finding → bead created with `from-review` + spec label; pass verdict with general finding → bead created with only `from-review`; fail verdict → no beads created.

**Acceptance Criteria:**
1. Pass-with-spec-finding creates bead with `from-review` + `spec:<id>` labels
2. Pass-with-general-finding creates bead with `from-review` label only
3. Fail verdict creates no from-review beads

**Dependencies:** Tasks 3

---

### Task 10: Run2 components wiring — specreview stage with highest tier

**Files:**
- Modify: `internal/v2/loop/run2_components.go`

**What to Do:**
In `NewRun2LoopComponents`:
1. Load `review_spec_v2.md` fragment via `loadFragment(cfg.ProjectRoot, "review_spec_v2.md")`
2. Instantiate specreview stage: `specreviewstage.New(cfg, adapters.Git, adapters.LLM, adapters.TaskTracker, baseInstructions, projectContext, specReviewFragment)`
3. Add `SpecReviewStage stagepkg.Stage` to `Run2LoopComponents` struct
4. Wire into `NewSpecLoop` via `WithSpecReviewStage(specReviewStage)` where the spec loop is constructed (in `run2.go`)
5. When routing the spec-review phase, use `routing.TierHigh` hardcoded — spec-review always uses the highest tier. Set this in `applyRouting` by adding `"spec-review"` to the phase-model map with value `routing.TierHigh` as default when not explicitly configured.
6. Remove `AcceptStage` from `RemediationRunnerConfig` construction (Task 8 removed it).

**Acceptance Criteria:**
1. `specreview.New(...)` called with correct dependencies
2. `Run2LoopComponents.SpecReviewStage` populated
3. `loadFragment(cfg.ProjectRoot, "review_spec_v2.md")` called without error when file missing (returns empty, falls back to package default)

**Dependencies:** Tasks 3, 8, 9

---

### Task 11: run2 --from-review flag

**Files:**
- Modify: `cmd/gromit/run2.go`

**What to Do:**
Add flags in `init()`:
```go
run2Cmd.Flags().Bool("from-review", false, "Run only beads with the from-review label")
run2Cmd.Flags().String("spec", "", "Scope --from-review to a specific spec ID")
```

In `run2()`, check the flag early:
```go
fromReview, _ := cmd.Flags().GetBool("from-review")
if fromReview {
    return run2FromReview(cmd, cfg, adapters, ...)
}
```

Implement `run2FromReview`: query tracker for beads with label `from-review` (and optionally `spec:<id>`); run them through the bead loop only (no plan, no decompose, no accept, no spec-review). No remediation cycle — if a bead fails, it stays open for next `--from-review` run. Log a summary of beads found and executed.

Write tests for label query construction: `--from-review` alone → label `from-review`; `--from-review --spec foo` → labels `["from-review", "spec:foo"]`.

**Acceptance Criteria:**
1. `--from-review` flag exists and triggers `run2FromReview` path
2. `--from-review --spec <id>` scopes query to spec-labeled from-review beads
3. No accept/spec-review/remediation cycle triggered in from-review mode

**Dependencies:** Task 10

---

### Task 12: Integration tests — post-bead-loop pipeline

**Files:**
- Create: `internal/v2/loop/spec_loop_specreview_integration_test.go`

**What to Do:**
Write two integration tests using mock stages:

**Test 1: Full remediation path**
Setup: accept returns fail (one failing criterion), specreview returns fail (one critical finding). Verify: remediation runner receives combined findings from both stages; `req.Findings` has 2 entries; targeted decompose called.

**Test 2: Pass-with-improvements path**
Setup: accept returns pass (no failures), specreview returns pass with one warning finding (scope=spec). Verify: spec proceeds to present stage; from-review bead created in tracker mock; remediation runner NOT called.

Use `t.Cleanup` for any package-level injectable var overrides per project rules.

**Acceptance Criteria:**
1. Test 1 verifies combined findings flow to remediation runner
2. Test 2 verifies from-review bead created when spec-review passes with findings
3. Both tests pass: `go test ./internal/v2/loop/... -run TestIntegration_SpecReview`

**Dependencies:** Tasks 7, 8, 9