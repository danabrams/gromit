Now I have enough to write the plan. Here it is:

---

```markdown
---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-10
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

**Goal:** Add a holistic spec-level code review stage after the bead loop, replace plan-re-decomposition with findings-based targeted remediation, and introduce a `--from-review` flag for executing deferred review beads.

**Architecture:** A new `internal/v2/stage/specreview/` package implements the spec-level review stage using the existing Stage interface pattern. Shared `SpecReviewFinding` types are added to `internal/v2/stage/stage.go` and flow through the spec loop into remediation decompose via a new `Findings` field on `StageRequest`. The accept stage gains structured finding output; the decompose stage gains a findings-based prompt template selected when `req.Findings` is non-empty; the spec loop adds the review call after accept and collects combined findings before delegating to the remediation runner.

**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

**New package:** `internal/v2/stage/specreview/` — implements `stage.Stage`. Receives cumulative diff (DiffFromBase), plan text, base instructions, project context, and a review fragment (`review_spec_v2.md`). Invokes LLM with highest-tier model. Parses structured JSON output into `SpecReviewArtifacts{Verdict, Findings}`.

**Shared finding types in `stage.go`:** `SpecReviewFinding{Severity, Category, Scope, Description, AffectedFiles}` and `SpecReviewArtifacts{Verdict, Findings}`. `StageRequest` gains `Findings []SpecReviewFinding` so remediation can pass findings to decompose without a separate channel.

**Accept → findings:** `AcceptArtifacts` gains `Findings []SpecReviewFinding` where each unmet criterion becomes a finding with severity=critical, category=acceptance, scope=spec.

**Decompose → findings template:** A third prompt template `findingsDecomposePromptTemplate` is selected when `req.Remediation && len(req.Findings) > 0`. Creates targeted fix beads from findings instead of from the original plan.

**Spec loop changes:** After accept passes, run spec-level review. Gate: succeed only when accept==pass AND review.verdict==pass. On failure from either, collect all findings into `req.Findings` and call `remediationRunner.Run`. On pass-with-improvements, create from-review beads before proceeding to present.

**Spec loop option:** `WithSpecReviewStage(stage)` added to `spec_loop.go`, wired in `run2_components.go` with highest-tier routing (tier=high / opus).

**`--from-review` flag:** `run2.go` gains flag; when set, queries beads with label `from-review`, skips plan/decompose/accept/review, runs bead loop only.

**Key files:**
- Create: `internal/v2/stage/specreview/specreview.go`, `specreview_test.go`
- Create: `review_spec_v2.md`
- Modify: `internal/v2/stage/stage.go` — finding types + Findings field on StageRequest
- Modify: `internal/v2/stage/accept/accept.go` — structured findings in AcceptArtifacts
- Modify: `internal/v2/stage/decompose/decompose.go` — findings-based template
- Modify: `internal/v2/loop/spec_loop.go` — review stage wiring, combined gate, findings → remediation, from-review bead creation
- Modify: `internal/v2/loop/run2_components.go` — wire specreview stage at highest tier
- Modify: `internal/v2/remediation/remediation.go` — pass findings through to decompose
- Modify: `cmd/gromit/run2.go` — `--from-review` flag

## Test Strategy

Unit tests for specreview stage: verdict logic (critical→fail, warning/suggestion→pass), JSON parsing, missing fields handling. Unit tests for accept findings extraction: each unmet criterion maps to a critical finding. Unit tests for findings-based decompose template selection and prompt formatting. Unit tests for from-review bead label assignment (spec-scoped vs general). Unit tests for `--from-review` flag: queries from-review label, no plan/decompose, no remediation.

Integration test: bead loop → accept fail → review findings → targeted bead decompose (not plan re-decompose).
Integration test: pass-with-improvements path → from-review beads created, spec proceeds to present.

Mocking strategy: use existing `llmtypes.LLMProvider` mock pattern; inject `GitDiffer` (already exists in accept stage); inject fake `TaskTracker` for bead creation assertions.

---

## Implementation Tasks

### Task 1: Shared Finding Types in stage.go

**Files:**
- Modify: `internal/v2/stage/stage.go`
- Test: `internal/v2/stage/stage_test.go`

**What to Do:**
Add `SpecReviewFinding` struct with fields `Severity string`, `Category string`, `Scope string` (values: "spec" | "general"), `Description string`, `AffectedFiles []string`. Add `SpecReviewArtifacts` struct with `Verdict string` (values: "pass" | "fail") and `Findings []SpecReviewFinding`. Add `Findings []SpecReviewFinding` field to `StageRequest`. Add string constants for severity, category, scope values.

**Acceptance Criteria:**
- `SpecReviewFinding` and `SpecReviewArtifacts` are exported from `internal/v2/stage`
- `StageRequest.Findings` field exists and is a `[]SpecReviewFinding`
- Severity constants `SeverityCritical`, `SeverityWarning`, `SeveritySuggestion` are exported
- Category constants `CategoryBug`, `CategorySecurity`, `CategoryQuality`, `CategoryTestGap`, `CategoryArchitecture`, `CategoryAcceptance` are exported
- Scope constants `ScopeSpec`, `ScopeGeneral` are exported

**Dependencies:** none

---

### Task 2: review_spec_v2.md Prompt Fragment

**Files:**
- Create: `review_spec_v2.md` (project root)

**What to Do:**
Create the spec-level review prompt fragment. It must instruct the LLM to evaluate the cumulative diff holistically across: correctness, security (OWASP top 10), error handling, test coverage gaps, code quality, and architectural fit. Output format must be the JSON schema from the spec: `{"verdict": "pass|fail", "findings": [{"severity": "critical|warning|suggestion", "category": "bug|security|quality|test-gap|architecture", "scope": "spec|general", "description": "...", "affected_files": [...]}]}`. Include verdict logic: any critical finding forces verdict=fail. Scope rule: spec-scope for files changed by this spec, general-scope for issues in existing code not changed by this spec.

**Acceptance Criteria:**
- File exists at project root as `review_spec_v2.md`
- Fragment describes all 6 review dimensions
- Fragment specifies the JSON output schema exactly matching the spec
- Fragment explains the scope distinction (spec vs general)
- Fragment explains the verdict rule (critical→fail)

**Dependencies:** none

---

### Task 3: Spec-Level Review Stage Core

**Files:**
- Create: `internal/v2/stage/specreview/specreview.go`
- Create: `internal/v2/stage/specreview/specreview_test.go`

**What to Do:**
Implement `specreview.Stage` satisfying `stage.Stage`. Constructor `New(cfg *config.Config, git GitDiffer, llm llmtypes.LLMProvider, baseInstructions, projectContext, fragment string) (*Stage, error)`. The `GitDiffer` interface needs `DiffFromBase(ctx, worktree) (string, error)` — reuse the same interface shape as accept stage. The `Run` method: (1) read plan from `.gromit/v2/plan.md`, (2) call `git.DiffFromBase` for cumulative diff, (3) build prompt from base instructions, project context, fragment, plan, and diff, (4) invoke LLM at the model in `req.Model` (caller sets highest tier), (5) parse JSON response into `SpecReviewArtifacts`, (6) apply verdict override: any critical finding forces verdict=fail, (7) return `DecisionFail` when verdict==fail, `DecisionProceed` when verdict==pass.

Parse the LLM response with JSON extraction (use `jsonutil.ExtractJSON` as in accept stage). On parse failure, return `DecisionFail` with a synthetic critical finding describing the parse error.

**Acceptance Criteria:**
- `specreview.Stage` implements `stage.Stage`
- `Run` returns `DecisionFail` when any finding has severity=critical
- `Run` returns `DecisionProceed` when all findings are warning/suggestion
- `Run` returns `DecisionProceed` with empty findings when no issues found (verdict=pass)
- Artifacts are `*stage.SpecReviewArtifacts` with populated Verdict and Findings
- Parse failure returns DecisionFail with a synthetic critical finding

**Dependencies:** Task 1, Task 2

---

### Task 4: Spec-Level Review Stage Tests

**Files:**
- Test: `internal/v2/stage/specreview/specreview_test.go`

**What to Do:**
Write unit tests covering: (a) critical finding forces verdict=fail and DecisionFail decision, (b) only warning+suggestion findings produce verdict=pass and DecisionProceed, (c) empty findings list produces verdict=pass, (d) LLM parse failure produces DecisionFail with synthetic critical finding, (e) `SpecReviewArtifacts` findings are correctly populated from LLM JSON, (f) `Name()` returns a non-empty string. Use fake LLM provider that returns canned JSON responses. Use fake GitDiffer. Use `t.TempDir()` for worktree with a mock plan file.

**Acceptance Criteria:**
- All tests pass with `go test ./internal/v2/stage/specreview/...`
- Tests cover all verdict logic paths
- No `os.Chdir` calls — use `t.Chdir()` if directory changes are needed
- Fake dependencies restored via `t.Cleanup` where applicable

**Dependencies:** Task 3

---

### Task 5: Accept Stage Structured Findings Output

**Files:**
- Modify: `internal/v2/stage/accept/accept.go`
- Test: `internal/v2/stage/accept/accept_test.go`

**What to Do:**
Add `Findings []stage.SpecReviewFinding` to `AcceptArtifacts`. After acceptance evaluation, when criteria fail, convert each unmet criterion to a `stage.SpecReviewFinding{Severity: stage.SeverityCritical, Category: stage.CategoryAcceptance, Scope: stage.ScopeSpec, Description: <criterion text + summary>}`. Populate `AffectedFiles` from the per-criterion diff mapping when available (targeted strategy already has this). Existing `GapSummary` and `Results` fields are preserved unchanged.

**Acceptance Criteria:**
- `AcceptArtifacts.Findings` is populated for each failed criterion
- Each unmet criterion maps to exactly one finding with severity=critical, category=acceptance
- Findings scope is always "spec" for acceptance failures
- Existing `GapSummary` and `Results` behavior is unchanged
- Existing accept tests still pass

**Dependencies:** Task 1

---

### Task 6: Findings-Based Decompose Template

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go`
- Test: `internal/v2/stage/decompose/decompose_test.go`

**What to Do:**
Add `findingsDecomposePromptTemplate` as a package-level var. The template takes: specID, finding list (JSON-serialized), skill instructions, spec label. Prompt preamble: "You are creating TARGETED fix beads to address specific review findings. Do NOT re-implement work that already exists." Include each finding's severity, category, description, and affected_files. Bead output format is the same JSON array as other templates.

In `Run()`, select the findings template when `req.Remediation && len(req.Findings) > 0`. Format findings as a readable list (one finding per line with severity/category/description). When findings are present, skip reading the plan file (findings are self-contained context for targeted fixes).

**Acceptance Criteria:**
- Findings template is selected when `req.Remediation && len(req.Findings) > 0`
- When findings template is used, plan file is not required (no error if absent)
- Existing `req.Remediation && gapAnalysis != ""` path is unchanged (gap analysis still works when findings are empty)
- New unit test verifies findings template selection with non-empty findings

**Dependencies:** Task 1

---

### Task 7: Remediation Runner Passes Findings to Decompose

**Files:**
- Modify: `internal/v2/remediation/remediation.go`
- Test: `internal/v2/remediation/remediation_test.go`

**What to Do:**
In `executeRemediation`, change the signature to accept `findings []stage.SpecReviewFinding` alongside the existing `gapAnalysis string`. Before calling `r.decompose(ctx, req)`, set `req.Findings = findings`. This wires the findings from accept+review into the decompose stage so the findings-based template is activated.

Update the call site in `Run()` — when `res.Decision == stage.DecisionFail`, extract findings from `AcceptArtifacts.Findings` (add helper `extractFindings(res *stage.Result) []stage.SpecReviewFinding`) and pass them to `executeRemediation`.

Keep the `gapAnalysis` path as fallback: when `len(findings) == 0 && gapAnalysis != ""`, continue using gap-analysis-based remediation.

**Acceptance Criteria:**
- When `AcceptArtifacts.Findings` is non-empty, `req.Findings` is set before calling decompose
- When findings are empty, existing gap-analysis path is used unchanged
- Existing remediation tests pass
- New unit test verifies findings are propagated to the decompose request

**Dependencies:** Task 1, Task 5, Task 6

---

### Task 8: Spec Loop — Wire Spec-Level Review and Combined Gate

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`

**What to Do:**
Add `specReviewStage stage.Stage` field to `SpecLoop`. Add `WithSpecReviewStage(s stage.Stage) SpecLoopOption`. Add the `StageSequence` entry "specreview" after "accept".

In `Run()`, after `s.ensureAcceptance(...)` succeeds, call spec-level review:
```go
s.recordStage("specreview")
reviewRes, err := s.runSpecReview(ctx, &req, specID)
if err != nil {
    handleFailureCleaned = true
    return s.handleFailure(ctx, specID, baseSummary, err)
}
```

Add `runSpecReview` helper that:
1. If `s.specReviewStage == nil`, return nil (skip — optional stage)
2. Apply routing with phase "specreview"
3. Call `s.specReviewStage.Run(ctx, &req)`
4. If decision == DecisionFail: collect findings from artifacts, combine with any accept findings already in `req.Findings`, set `req.Findings` on the request, call `s.remediationRunner.Run(ctx, specID, req.Worktree, reviewRes)`
5. If decision == DecisionProceed with non-empty findings: call `s.createFromReviewBeads(ctx, specID, reviewArtifacts.Findings)`

Add `createFromReviewBeads` helper: iterates findings, creates a bead per finding via `s.adapters.TaskTracker.CreateBead`. Spec-scoped findings (scope=="spec") get label `from-review spec:<specID>`; general findings get label `from-review`. Priority = P1 for warnings, P2 for suggestions. Skip critical findings (they should have triggered fail and been remediated).

**Acceptance Criteria:**
- Spec-level review runs after accept passes
- If review returns DecisionFail, remediation runner is called with findings in req.Findings
- If review returns DecisionProceed with findings, from-review beads are created with correct labels
- If review returns DecisionProceed with no findings, spec proceeds unchanged
- If specReviewStage is nil (not configured), spec loop proceeds without error
- Spec-scoped findings get label `from-review spec:<specID>`, general get `from-review`

**Dependencies:** Task 1, Task 3, Task 7

---

### Task 9: Spec Loop Tests for Review Integration

**Files:**
- Test: `internal/v2/loop/spec_loop_test.go`

**What to Do:**
Add unit tests for the spec-level review integration:
- (a) When specReviewStage returns DecisionFail, `remediationRunner.Run` is called
- (b) When specReviewStage returns DecisionProceed with spec-scoped findings, beads are created with `from-review spec:<specID>` label
- (c) When specReviewStage returns DecisionProceed with general findings, beads are created with `from-review` label (no spec prefix)
- (d) When specReviewStage is nil, spec loop completes without error
- (e) When specReviewStage returns DecisionProceed with no findings, no beads are created

Use the existing fake stage pattern in the test file (check for `fakeStage` or similar helpers). Use fake `TaskTracker` that records created beads.

**Acceptance Criteria:**
- All 5 test cases pass
- Tests use `t.Chdir()` not `os.Chdir()`
- Fake dependency overrides restored via `t.Cleanup`

**Dependencies:** Task 8

---

### Task 10: Run2 Components — Wire Spec-Level Review at Highest Tier

**Files:**
- Modify: `internal/v2/loop/run2_components.go`

**What to Do:**
Load the review spec fragment: `specReviewFragment, err := loadFragment(cfg.ProjectRoot, "review_spec_v2.md")`. Create the specreview stage: `specReviewStage, err := specreviewstage.New(cfg, adapters.Git, adapters.LLM, baseInstructions, projectContext, specReviewFragment)`. Add to `Run2LoopComponents` struct: `SpecReviewStage stage.Stage`. Wire it into the spec loop via `loop.WithSpecReviewStage(specReviewStage)`.

Routing for specreview: it must always use the highest configured tier. Apply routing with phase "specreview" — the `phaseModels` map should map "specreview" to "high" tier. Since the spec mandates highest tier always, add a constant or document in `run2_components.go` that specreview is hardcoded to tier "high". In `applyRouting` in `spec_loop.go`, use `routing.TierHigh` as the default for phase "specreview" instead of `routing.TierMedium`.

Add import: `specreviewstage "github.com/danabrams/gromit/internal/v2/stage/specreview"`.

**Acceptance Criteria:**
- `NewRun2LoopComponents` creates a specreview stage and wires it to the spec loop
- `SpecReviewStage` field is populated in the returned `Run2LoopComponents`
- The specreview phase routes to "high" tier when router is configured
- Build compiles cleanly: `go build ./...`

**Dependencies:** Task 3, Task 8

---

### Task 11: `--from-review` Flag in run2.go

**Files:**
- Modify: `cmd/gromit/run2.go`
- Test: `cmd/gromit/run2_test.go` (or nearest test file)

**What to Do:**
Add flag in `init()`: `run2Cmd.Flags().Bool("from-review", false, "Run only beads with the from-review label")`. Add optional scoping: `run2Cmd.Flags().String("spec", "", "Scope from-review run to a specific spec")`.

In `run2()`, check the flag: if `--from-review` is set, call `runFromReview(ctx, cfg, adapters, ...)` instead of the normal spec loop path.

`runFromReview` function: (1) build label query: `labels := []string{"from-review"}`; if `--spec <id>` provided, also add `fmt.Sprintf("spec:%s", specID)` to labels; (2) query beads via TaskTracker; (3) if no beads found, print "no from-review beads found" and return nil; (4) build a `BeadLoop` (reuse `Run2LoopComponents`) and call `beadLoop.Run(ctx, beads, stopCh)`; (5) no plan, no decompose, no accept, no specreview, no remediation cycle.

Note: `--from-review` is incompatible with providing a spec-file positional arg. When `--from-review` is set, the `Args` validator should accept 0 positional args.

**Acceptance Criteria:**
- `gromit run2 --from-review` queries beads with label `from-review` and runs them
- `gromit run2 --from-review --spec myspec` additionally filters by `spec:myspec` label
- `--from-review` run does not execute plan, decompose, accept, or specreview stages
- When no from-review beads exist, command exits 0 with a message
- Build compiles cleanly

**Dependencies:** Task 8, Task 10

---

### Task 12: Integration Test — Post-Bead-Loop Pipeline

**Files:**
- Create or modify: `internal/v2/loop/spec_loop_integration_test.go` (or nearest integration test file)

**What to Do:**
Write an integration test that exercises the full post-bead-loop path: bead loop completes → accept runs and passes → spec-level review runs and returns critical finding → remediation is triggered with findings → decompose creates targeted beads (not plan beads). Use a fake accept stage that always passes (returns DecisionProceed), a fake specreview stage that returns DecisionFail with one critical finding, a fake remediation runner that records whether it was called and what findings it received, and a fake TaskTracker.

Assert: remediation runner was called; the request passed to it had `Findings` with the critical finding from specreview.

Also write a pass-with-improvements integration test: specreview returns DecisionProceed with one spec-scoped warning finding → assert that TaskTracker.CreateBead was called with label `from-review spec:<specID>` and the spec proceeds to present.

**Acceptance Criteria:**
- Findings-based remediation path is exercised end-to-end in the test
- Pass-with-improvements path creates from-review beads with correct labels
- All existing spec loop tests still pass

**Dependencies:** Task 8, Task 9
```