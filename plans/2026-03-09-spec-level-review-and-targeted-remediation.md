---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-09
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a post-bead-loop spec-level review stage that produces structured findings, replace plan-re-decomposition remediation with targeted findings-based decomposition, and add a `--from-review` flag for running deferred review beads.

**Architecture:** A new `internal/v2/stage/specreview/` package implements the spec-level review stage using the highest-tier model and structured JSON output. A shared `Finding` type lives in `internal/v2/stage/stage.go` so accept, specreview, remediation, and decompose all use one canonical type. The remediation runner's `executeRemediation` receives `[]stage.Finding` instead of a gap-analysis string, passing them into a new `findingsDecomposePromptTemplate`. The spec loop runs accept then spec-level review sequentially after the bead loop, collects combined findings, and gates success on both passing.

**Tech Stack:** Go, existing stage/LLM/tracker patterns, `jsonutil.ExtractObject` for JSON parsing, `internal/v2/stage` package conventions.

**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

**New package:** `internal/v2/stage/specreview/` — implements `stage.Stage` interface. Invokes highest-tier LLM with cumulative diff + plan + project context. Parses `{"verdict": "pass|fail", "findings": [...]}`. Returns `DecisionFail` when any critical finding exists, `DecisionProceed` otherwise. Produces `SpecReviewArtifacts{Verdict, Findings}`.

**Shared Finding type** in `stage.go`: `stage.Finding{Severity, Category, Scope, Description, AffectedFiles}`. Accept produces findings for unmet criteria (`severity: critical, category: acceptance, scope: spec`). Specreview parses findings from LLM JSON.

**Spec loop changes** (`spec_loop.go`): After `ensureAcceptance`, if accept passed, run spec-level review. Collect findings from both. If any failure, pass combined `[]stage.Finding` to remediation runner. When review passes with findings, create from-review beads (spec-scoped → `from-review` + `spec:<id>`, general → `from-review` only).

**Remediation changes** (`remediation.go`): `executeRemediation` receives `[]stage.Finding` in `stage.Request.Findings`. Decompose stage detects non-empty `Findings` and uses `findingsDecomposePromptTemplate`.

**From-review flag** (`run2.go`): `--from-review` skips plan/decompose/accept/review; queries open beads with `from-review` label and runs them through the bead loop directly. `--spec` scopes the query.

## Test Strategy

- **specreview unit tests**: structured output parsing, verdict logic (critical→fail, warning→pass, suggestion→pass, mixed with critical→fail), LLM error propagation, highest-tier model selection.
- **accept unit tests**: verify unmet criteria produce `stage.Finding` with correct severity/category/scope.
- **decompose unit tests**: verify `findingsDecomposePromptTemplate` is selected when `req.Findings` is non-empty, verify findings appear in prompt.
- **remediation unit tests**: verify findings passed through to decompose stage.
- **spec_loop unit tests**: verify spec-level review runs after accept, combined findings on dual failure, from-review bead creation.
- **from-review flag integration test**: verify bead query uses `from-review` label, no plan/decompose called.

---

## Implementation Tasks

### Task 1: Add `Finding` type and `Findings` field to `internal/v2/stage/stage.go`

**Files:**
- Modify: `internal/v2/stage/stage.go`
- Test: `internal/v2/stage/stage_test.go`

**What to Do:**

Add the shared `Finding` type to `stage.go`:

```go
// FindingSeverity classifies the impact of a review finding.
type FindingSeverity string

const (
    FindingSeverityCritical   FindingSeverity = "critical"
    FindingSeverityWarning    FindingSeverity = "warning"
    FindingSeveritySuggestion FindingSeverity = "suggestion"
)

// FindingCategory classifies the nature of a review finding.
type FindingCategory string

const (
    FindingCategoryBug          FindingCategory = "bug"
    FindingCategorySecurity     FindingCategory = "security"
    FindingCategoryQuality      FindingCategory = "quality"
    FindingCategoryTestGap      FindingCategory = "test-gap"
    FindingCategoryArchitecture FindingCategory = "architecture"
    FindingCategoryAcceptance   FindingCategory = "acceptance"
)

// FindingScope indicates whether the finding relates to spec-changed code or general code.
type FindingScope string

const (
    FindingScopeSpec    FindingScope = "spec"
    FindingScopeGeneral FindingScope = "general"
)

// Finding represents a structured observation from spec-level review or acceptance evaluation.
type Finding struct {
    Severity      FindingSeverity `json:"severity"`
    Category      FindingCategory `json:"category"`
    Scope         FindingScope    `json:"scope"`
    Description   string          `json:"description"`
    AffectedFiles []string        `json:"affected_files"`
}

// HasCritical returns true if any finding has severity critical.
func HasCritical(findings []Finding) bool {
    for _, f := range findings {
        if f.Severity == FindingSeverityCritical {
            return true
        }
    }
    return false
}
```

Add `Findings []Finding` field to `StageRequest`:

```go
type StageRequest struct {
    // ... existing fields ...
    Findings    []Finding   // structured findings for findings-based decompose
}
```

Write a table-driven test in `stage_test.go` verifying `HasCritical` returns true when any finding is critical and false when all are warning/suggestion.

**Acceptance Criteria:**
- `stage.Finding` is defined with all fields matching the spec JSON schema
- `stage.HasCritical` returns correct bool for mixed finding sets
- `StageRequest.Findings` field compiles and is zero-valued by default

**Dependencies:** None

---

### Task 2: Create `review_spec_v2.md` prompt fragment

**Files:**
- Create: `review_spec_v2.md` (project root)

**What to Do:**

Create the spec-level review prompt fragment. This is loaded by `run2_components.go` and prepended to the review stage's system prompt.

```markdown
# Spec-Level Code Review

You are performing a holistic review of the cumulative code changes for a spec implementation. Your goal is to identify issues that span multiple beads or emerge from the combined output.

## Review Dimensions

1. **Correctness** — logic errors, off-by-one, incorrect conditionals, nil dereferences, race conditions
2. **Security** — OWASP Top 10: injection, broken auth, sensitive data exposure, XXE, broken access control, misconfiguration, XSS, insecure deserialization, known-vulnerable components, insufficient logging
3. **Error Handling** — unhandled errors, error swallowing, missing context wrapping, incomplete error paths
4. **Test Coverage** — missing unit tests for changed code, missing edge cases, test assertions that don't exercise behavior, documentation-only tests
5. **Code Quality** — duplication, premature abstraction, over-engineering, YAGNI violations, unclear naming
6. **Architectural Fit** — deviations from established patterns (nil-field normalization, stage interface conventions, event emission patterns, test seam patterns), unexpected cross-package dependencies
7. **Integration Completeness** — wiring gaps (stage not registered, flag not connected, prompt fragment not loaded), missing nil checks at boundaries

## Output Format

Output ONLY a JSON object:
```json
{
  "verdict": "pass | fail",
  "findings": [
    {
      "severity": "critical | warning | suggestion",
      "category": "bug | security | quality | test-gap | architecture",
      "scope": "spec | general",
      "description": "specific description of the issue and location",
      "affected_files": ["path/to/file.go"]
    }
  ]
}
```

**Verdict logic:**
- `fail` — one or more `critical` findings
- `pass` — only `warning` and `suggestion` findings (or no findings)

**Scope:**
- `spec` — the finding is in code this spec changed or added
- `general` — the finding is in pre-existing code unrelated to this spec

Output ONLY valid JSON. No prose before or after.
```

**Acceptance Criteria:**
- File exists at `review_spec_v2.md` in project root
- Contains JSON output format specification with all required fields
- Defines verdict logic (critical → fail)
- Defines scope classification

**Dependencies:** None

---

### Task 3: Create `internal/v2/stage/specreview/specreview.go` with unit tests

**Files:**
- Create: `internal/v2/stage/specreview/specreview.go`
- Create: `internal/v2/stage/specreview/specreview_test.go`

**What to Do:**

Create the specreview stage. Follow the exact pattern of `internal/v2/stage/accept/accept.go` for stage structure, LLM invocation, and error handling.

```go
package specreview

import (
    "context"
    "fmt"
    "path/filepath"
    "strings"

    "github.com/danabrams/gromit/internal/jsonutil"
    "github.com/danabrams/gromit/internal/v2/llmtypes"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

const StageName = stagedesc.SpecReview  // add to names package

// GitDiffer provides the cumulative diff from branch base.
type GitDiffer interface {
    DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// Stage performs holistic spec-level code review after the bead loop.
type Stage struct {
    name        string
    llm         llmtypes.LLMProvider
    git         GitDiffer
    promptFrag  string  // review_spec_v2.md contents
    projectCtx  string  // CLAUDE.md + RULES.md
}

// SpecReviewArtifacts holds the outcome of spec-level review.
type SpecReviewArtifacts struct {
    Verdict  string            // "pass" | "fail"
    Findings []stagepkg.Finding
}

func New(llm llmtypes.LLMProvider, git GitDiffer, promptFrag, projectCtx string) *Stage { ... }

func (s *Stage) Name() string { return s.name }

func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
    diff, err := s.git.DiffFromBase(ctx, req.Worktree)
    if err != nil { return nil, fmt.Errorf("specreview: diff: %w", err) }

    plan := readPlan(req.Worktree)  // read .gromit/v2/plan.md

    prompt := buildPrompt(s.promptFrag, s.projectCtx, plan, diff)
    resp, err := s.llm.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: prompt, Model: req.Model})
    if err != nil { return nil, fmt.Errorf("specreview: llm: %w", err) }

    result, err := parseReviewResponse(resp.Output)
    if err != nil { return nil, fmt.Errorf("specreview: parse: %w", err) }

    artifacts := SpecReviewArtifacts{Verdict: result.Verdict, Findings: result.Findings}

    if stagepkg.HasCritical(result.Findings) {
        return &stagepkg.Result{Decision: stagepkg.DecisionFail, Artifacts: artifacts}, nil
    }
    return &stagepkg.Result{Decision: stagepkg.DecisionProceed, Artifacts: artifacts}, nil
}

type reviewResponse struct {
    Verdict  string           `json:"verdict"`
    Findings []stagepkg.Finding `json:"findings"`
}

func parseReviewResponse(output string) (reviewResponse, error) {
    var r reviewResponse
    if err := jsonutil.ExtractObject(strings.TrimSpace(output), &r); err != nil {
        return r, err
    }
    return r, nil
}
```

Write tests covering:
1. **Critical finding → DecisionFail**: LLM returns a finding with `severity: critical`. Verify `Decision == DecisionFail`, `Verdict == "fail"`.
2. **Warning only → DecisionProceed**: LLM returns only warning findings. Verify `Decision == DecisionProceed`, `Verdict == "pass"`.
3. **No findings → DecisionProceed**: LLM returns empty findings array. Verify proceed.
4. **Mixed critical + warning → DecisionFail**: HasCritical drives the decision even when warnings also present.
5. **LLM error → propagated error**: fakeLLM returns error. Verify `Run` returns non-nil error.
6. **Parse failure → error**: LLM returns non-JSON. Verify error returned.

Use `fakeLLM` and `fakeGitAdapter` structs in the test file (unexported, inline — same pattern as `internal/v2/stage/review/review_test.go`).

**Acceptance Criteria:**
- `Stage.Run` returns `DecisionFail` when any finding is critical
- `Stage.Run` returns `DecisionProceed` when all findings are warning/suggestion
- `SpecReviewArtifacts.Findings` contains all parsed findings regardless of verdict
- All 6 test cases pass

**Dependencies:** Task 1

---

### Task 4: Add `SpecReview` to stage names package

**Files:**
- Modify: `internal/v2/stage/names/names.go` (or wherever stage name constants live)

**What to Do:**

Add `SpecReview = "spec-review"` constant to the names package. This is required for the specreview stage's `Name()` method and for the spec loop to record the stage.

Check what existing constants look like (e.g., `Accept`, `Decompose`, `Review`) and follow that pattern exactly.

**Acceptance Criteria:**
- `names.SpecReview` constant exists and compiles
- Value is `"spec-review"` (kebab-case, matching existing convention)

**Dependencies:** None (can be done in parallel with Task 1)

---

### Task 5: Update `accept` stage to produce `[]stage.Finding` for unmet criteria

**Files:**
- Modify: `internal/v2/stage/accept/accept.go`
- Modify: `internal/v2/stage/accept/accept_test.go`

**What to Do:**

Update `AcceptArtifacts` to include `Findings []stage.Finding`:

```go
type AcceptArtifacts struct {
    GapSummary string
    Findings   []stage.Finding  // NEW: one per unmet criterion
}
```

In the failure path (where `GapSummary` is written), also build `Findings`:

```go
func buildAcceptFindings(unmetCriteria []string) []stage.Finding {
    findings := make([]stage.Finding, 0, len(unmetCriteria))
    for _, criterion := range unmetCriteria {
        findings = append(findings, stage.Finding{
            Severity:    stage.FindingSeverityCritical,
            Category:    stage.FindingCategoryAcceptance,
            Scope:       stage.FindingScopeSpec,
            Description: criterion,
        })
    }
    return findings
}
```

Call this in the failure path and populate `AcceptArtifacts.Findings`.

Add tests:
1. **Unmet criteria → findings**: mock LLM returns criteria as failing. Verify `AcceptArtifacts.Findings` has one `critical/acceptance/spec` finding per unmet criterion.
2. **All pass → empty findings**: all criteria pass. Verify `Findings` is empty (or nil).

**Acceptance Criteria:**
- `AcceptArtifacts.Findings` is populated for each unmet criterion on failure
- Each finding has `severity: critical`, `category: acceptance`, `scope: spec`
- Passing accept produces empty/nil Findings
- Existing accept tests still pass

**Dependencies:** Task 1

---

### Task 6: Add `findingsDecomposePromptTemplate` to `internal/v2/stage/decompose/decompose.go`

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go`
- Modify: `internal/v2/stage/decompose/decompose_test.go` (or create alongside)

**What to Do:**

Add a third prompt template used when `req.Findings` is non-empty:

```go
const findingsDecomposePromptTemplate = `You are creating targeted fix beads for specific findings from spec-level review and acceptance evaluation.

## Original Plan (for context only — do not re-decompose it)
{{.Plan}}

## Findings to Fix
{{range .Findings}}
### {{.Severity}} — {{.Category}} ({{.Scope}})
{{.Description}}
Affected files: {{join .AffectedFiles ", "}}
{{end}}

## Instructions
Create one or more beads that specifically address the findings above. Do NOT recreate beads for work already done. Each bead must target a specific finding. Reference the finding description in the bead's acceptance criteria.

Output a JSON array of bead definitions: [{"title": "...", "description": "...", "priority": 1, "labels": ["..."], "expected_outputs": [...]}]
`
```

In `Stage.Run`, add a condition before the existing `req.Remediation` check:

```go
var tmplStr string
switch {
case len(req.Findings) > 0:
    tmplStr = findingsDecomposePromptTemplate
case req.Remediation && req.GapAnalysis != "":
    tmplStr = remediationDecomposePromptTemplate
default:
    tmplStr = defaultDecomposePromptTemplate
}
```

Add tests:
1. **Findings non-empty → findings template selected**: set `req.Findings` with one finding. Capture the prompt passed to fake LLM. Verify it contains the finding description, not the original plan decomposition header.
2. **Findings empty, Remediation true → remediation template**: verify remediation template is used.
3. **Both empty → default template**: verify default template.

**Acceptance Criteria:**
- `findingsDecomposePromptTemplate` is selected when `req.Findings` is non-empty
- Findings appear in the rendered prompt
- Existing template selection logic is unchanged for empty-findings cases
- Tests cover all three template selection paths

**Dependencies:** Task 1

---

### Task 7: Update `internal/v2/remediation/remediation.go` to receive and forward findings

**Files:**
- Modify: `internal/v2/remediation/remediation.go`

**What to Do:**

Add `Findings []stage.Finding` to whatever struct or parameters `executeRemediation` uses internally. The simplest approach: add a `findings` parameter or embed findings into the `stage.Request` that gets passed to decompose.

In `executeRemediation`, when building the decompose request, set `req.Findings = findings` if findings are non-empty. This lets Task 6's template selection kick in automatically.

Update `RemediationRunner.Run` signature (or the call from spec_loop) to accept `findings []stage.Finding`. When findings are provided, pass them through to decompose. When findings are empty, fall back to existing gap-analysis behavior.

The key invariant: if both `req.Findings` is non-empty AND `req.GapAnalysis` is set, prefer findings (findings take precedence as specified in Task 6's switch statement).

Add/update tests:
1. **Findings forwarded to decompose**: mock decompose stage captures the request. Set findings in Run call. Verify `req.Findings` matches.
2. **Empty findings → gap-analysis path unchanged**: verify existing remediation path still works.

**Acceptance Criteria:**
- When called with non-empty findings, remediation passes them into decompose's request
- The gap-analysis string path remains unchanged when findings are empty
- Existing remediation tests still pass

**Dependencies:** Tasks 1, 5, 6

---

### Task 8: Update `internal/v2/loop/spec_loop.go` to run spec-level review and collect combined findings

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`
- Modify: `internal/v2/loop/spec_loop_test.go` (or create a new spec_loop_specreview_test.go)

**What to Do:**

Add `specReviewStage stage.Stage` field to `SpecLoop`. Add a `WithSpecReviewStage(s stage.Stage)` option function.

In `ensureAcceptance` (or immediately after it in `Run`), after accept passes, run spec-level review:

```go
// After accept passes:
if s.specReviewStage != nil {
    reviewReq := s.specStageRequest(specID, worktree)
    reviewReq.Model = s.highestTierModel()  // always highest tier
    reviewRes, err := s.specReviewStage.Run(ctx, &reviewReq)
    if err != nil {
        return nil, fmt.Errorf("spec-level review: %w", err)
    }
    // Collect findings
    if artifacts, ok := reviewRes.Artifacts.(specreview.SpecReviewArtifacts); ok {
        reviewFindings = artifacts.Findings
    }
    if reviewRes.Decision == stage.DecisionFail {
        // Combine with accept findings (empty since accept passed) and remediate
        return nil, &specReviewError{findings: reviewFindings}
    }
}
```

Update the failure path in `ensureAcceptance` to collect accept findings from `AcceptArtifacts.Findings` and combine with any review findings before passing to the remediation runner.

Add `highestTierModel()` helper that returns `routing.TierHigh` ("high") regardless of phase routing.

For from-review bead creation: after review returns `DecisionProceed` with findings, iterate over `reviewFindings`:
- Scope `spec` → create bead with labels `["from-review", "spec:<specID>"]`
- Scope `general` → create bead with labels `["from-review"]`

Use the tracker (already available on `SpecLoop`) to create beads. Follow the pattern in `review.go` for `BuildReviewBeadLabels`.

Write tests:
1. **Spec-level review runs after accept pass**: stub accept→pass, stub specReview→pass. Verify specReview.Run was called.
2. **Spec-level review fail → combined findings to remediation**: stub accept→pass, stub specReview→fail with critical finding. Verify remediation called with findings.
3. **Accept fail → accept findings to remediation**: stub accept→fail. Verify specReview not called, remediation called with accept findings.
4. **Both fail → combined findings**: stub accept→fail, remediation calls specReview (for the next iteration), stub specReview→fail. Combined findings in next remediation.
5. **Pass-with-improvements → from-review beads created**: stub both→pass, specReview has spec-scoped and general findings. Verify tracker.CreateBead called with correct labels.
6. **No specReviewStage → spec succeeds after accept pass**: backward-compatible nil check.

**Acceptance Criteria:**
- Spec-level review runs after accept passes
- Critical findings from review → DecisionFail, findings passed to remediation
- When both pass, from-review beads created with correct scope-based labels
- When specReviewStage is nil, behavior is unchanged from today
- Highest-tier model is always used for specReview regardless of phase routing

**Dependencies:** Tasks 1, 2, 3, 4, 5, 7

---

### Task 9: Wire spec-level review in `internal/v2/loop/run2_components.go`

**Files:**
- Modify: `internal/v2/loop/run2_components.go`

**What to Do:**

Load `review_spec_v2.md` fragment alongside other prompt fragments:

```go
specReviewFrag, err := loadFragment(projectRoot, "review_spec_v2.md")
// allow missing file: if err != nil, specReviewFrag = ""
```

Create the specreview stage. It must always use the highest-tier LLM provider. Use the `highestTierProvider` from routing — select provider for `routing.TierHigh`:

```go
highProvider, _, _, _ := router.Select("spec-review", routing.TierHigh)
specReviewStage := specreview.New(highProvider, gitAdapter, specReviewFrag, projectCtx)
```

Add `WithSpecReviewStage(specReviewStage)` to the `SpecLoop` construction.

Add it to `Run2LoopComponents` struct if needed for testing.

Update `run2_components_test.go` to verify:
1. `specReviewStage` is non-nil in the returned components when `review_spec_v2.md` exists.
2. When `review_spec_v2.md` is missing, either graceful fallback (nil stage) or fail-fast based on design choice. Prefer graceful (nil → skip review) to avoid breaking existing runs.

**Acceptance Criteria:**
- `specreview.Stage` is constructed and wired into `SpecLoop`
- Highest-tier LLM provider is used (not the phase-routed model)
- Missing `review_spec_v2.md` does not crash — either falls back to empty prompt or stage is nil

**Dependencies:** Tasks 3, 4, 8

---

### Task 10: Add `--from-review` flag to `cmd/gromit/run2.go`

**Files:**
- Modify: `cmd/gromit/run2.go`

**What to Do:**

Add two flags:
```go
fromReview := cmd.Bool("from-review", false, "run only from-review beads through the bead loop")
fromReviewSpec := cmd.String("spec", "", "scope --from-review to beads for a specific spec")
```

When `--from-review` is set:
1. Skip spec file argument (no plan/decompose needed)
2. Query open beads with label `from-review` via the tracker
3. If `--spec <id>` also set, additionally filter by label `spec:<id>`
4. Run these beads through the bead loop directly (use existing `BeadLoop.Run`)
5. Do NOT run accept, spec-level review, or remediation
6. Do NOT present or merge

```go
if *fromReview {
    beads, err := queryFromReviewBeads(ctx, tracker, *fromReviewSpec)
    if err != nil { ... }
    _, err = beadLoop.Run(ctx, beads, mainWorktree, stopCh)
    return err
}
```

`queryFromReviewBeads`: queries tracker for open beads, filters by `from-review` label, optionally also by `spec:<specID>` label.

**Acceptance Criteria:**
- `gromit run2 --from-review` compiles and queries from-review beads
- `gromit run2 --from-review --spec <id>` scopes to spec-specific from-review beads
- No plan, decompose, accept, review, or remediation runs when `--from-review` is set
- If no from-review beads found, exits cleanly with a log message (not an error)

**Dependencies:** Tasks 8, 9

---

## Test Execution Order

1. `go test ./internal/v2/stage/... -run TestFinding` (Task 1)
2. `go test ./internal/v2/stage/specreview/...` (Task 3)
3. `go test ./internal/v2/stage/accept/...` (Task 5)
4. `go test ./internal/v2/stage/decompose/...` (Task 6)
5. `go test ./internal/v2/remediation/...` (Task 7)
6. `go test ./internal/v2/loop/...` (Task 8, 9)
7. `go build ./cmd/gromit/...` (Task 10)
8. `go test ./...` — full suite to check no regressions
