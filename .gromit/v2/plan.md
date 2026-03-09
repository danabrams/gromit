The plan file already exists at `docs/plans/2026-03-09-spec-level-review-and-targeted-remediation.md` and is complete. Here it is as requested:

---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-09
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a spec-level holistic code review after the bead loop, produce structured findings from both accept and review, feed those findings into a targeted remediation decompose (not a plan re-decompose), and add a `--from-review` flag to run deferred improvement beads.

**Architecture:** A new `internal/v2/stage/specreview/` package implements the spec-level review stage (highest-tier model, cumulative diff + plan as input, structured JSON findings output). The spec loop runs accept then spec-level review after the bead loop, collects combined findings from both when either fails, and passes them to the remediation runner. The remediation runner's interface changes to accept findings and drive only decompose + bead loop (no internal accept cycle — the spec loop now owns the retry outer loop). The decompose stage gains a findings-based prompt template that maps each finding to one or more targeted fix beads. Pass-with-improvements creates from-review beads after a passing spec-level review. A `--from-review` flag on `run2` runs only beads labeled `from-review` through the bead loop with no accept/review gate.

**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

### Component Map

```
spec_loop.go
  ├── [bead loop]
  ├── accept stage              → AcceptArtifacts { GapSummary, Findings []Finding }
  ├── spec-level review stage   → SpecReviewArtifacts { Verdict, Findings []Finding }
  ├── [combine findings]
  │     both pass → pass-with-improvements (create from-review beads if any findings)
  │     either fails → remediation runner (findings as input)
  └── remediation runner
        └── decompose (findings template) → bead loop → return
              spec loop re-runs accept + review

internal/v2/stage/stage.go        Finding types, Findings field on StageRequest
internal/v2/stage/specreview/     new package: spec-level review stage
internal/v2/stage/accept/         add Findings []Finding to AcceptArtifacts
internal/v2/stage/decompose/      add findingsDecomposePromptTemplate
internal/v2/remediation/          Run(ctx, specID, worktree, findings) — no internal accept
internal/v2/loop/run2_components/ wire SpecReviewStage with highest tier
cmd/gromit/run2.go                --from-review flag
review_spec_v2.md                 spec-level review prompt fragment (project root)
```

### Finding Type (stage.go)

```go
type FindingSeverity string
const (
    FindingSeverityCritical   FindingSeverity = "critical"
    FindingSeverityWarning    FindingSeverity = "warning"
    FindingSeveritySuggestion FindingSeverity = "suggestion"
)

type FindingCategory string
const (
    FindingCategoryBug          FindingCategory = "bug"
    FindingCategorySecurity     FindingCategory = "security"
    FindingCategoryQuality      FindingCategory = "quality"
    FindingCategoryTestGap      FindingCategory = "test-gap"
    FindingCategoryArchitecture FindingCategory = "architecture"
    FindingCategoryAcceptance   FindingCategory = "acceptance"
)

type FindingScope string
const (
    FindingScopeSpec    FindingScope = "spec"
    FindingScopeGeneral FindingScope = "general"
)

type Finding struct {
    Severity      FindingSeverity
    Category      FindingCategory
    Scope         FindingScope
    Description   string
    AffectedFiles []string
}
```

### Verdict Logic

- Any `Finding.Severity == "critical"` → `verdict: "fail"`
- Only warning/suggestion findings → `verdict: "pass"`
- No findings → `verdict: "pass"`

### remediationRunner Interface Change

Before: `Run(ctx context.Context, specID, worktree string) error`
After:  `Run(ctx context.Context, specID, worktree string, findings []stage.Finding) error`

The remediation runner no longer has AcceptStage or a retry loop. It receives findings from the spec loop, decomposes targeted fix beads, runs the bead loop, and returns. The spec loop owns the retry outer loop.

### DiffFromBase Access

The spec-level review stage reads the cumulative diff via `git.DiffFromBase(ctx, req.Worktree)`. The plan is read from `req.Worktree + "/.gromit/v2/plan.md"`. Both the accept stage and the gate satisfaction stage already use this pattern.

### From-Review Bead Labels

- spec-scoped findings: `[]string{"from-review", "spec:" + specID}`
- general findings: `[]string{"from-review"}`

### Routing

The spec-level review stage always uses the highest tier. In `run2_components.go`, the LLM adapter passed to `specreview.New` is constructed with `routing.TierHigh` (or the router's highest configured tier). The phase name `"spec-review"` is registered in the phase models map.

---

## Test Strategy

**Unit tests per package:**
- `specreview_test.go`: verdict logic, JSON parsing, DiffFromBase invoked, findings returned
- `accept_test.go`: existing tests still pass; new findings fields populated on failure
- `decompose_test.go`: findings template route taken when `req.Findings` non-empty; beads map to findings
- `remediation_test.go`: new signature accepted; decompose + bead runner called with findings in request; no accept invoked

**Integration tests:**
- `spec_loop_specreview_test.go`: accept fails → spec-level review called → combined findings → remediation → accept + review re-run
- `spec_loop_specreview_test.go`: review passes with spec-scoped findings → from-review beads created with spec label → spec proceeds to present
- `spec_loop_specreview_test.go`: review passes with general findings → from-review beads created without spec label

**Mocking:** All LLM calls use `llmtypes.MockLLMProvider`. All git calls use the existing `testutil.MockGitAdapter` or inline implementations. TaskTracker uses `trackertypes.MockTaskTracker` or inline.

**Test helpers:** Reuse shared `test/toolcalls` helpers for any tool-call log parsing per project rules.

---

## Implementation Tasks

### Task 1: Add Structured Findings Types to Stage Package

**Files:**
- Modify: `internal/v2/stage/stage.go`

**What to Do:**

Add the Finding types and constants immediately after the `LLMCostSummary` struct. Add `Findings []Finding` to `StageRequest`.

```go
// FindingSeverity classifies the urgency of a review finding.
type FindingSeverity string

const (
    FindingSeverityCritical   FindingSeverity = "critical"
    FindingSeverityWarning    FindingSeverity = "warning"
    FindingSeveritySuggestion FindingSeverity = "suggestion"
)

// FindingCategory classifies the domain of a review finding.
type FindingCategory string

const (
    FindingCategoryBug          FindingCategory = "bug"
    FindingCategorySecurity     FindingCategory = "security"
    FindingCategoryQuality      FindingCategory = "quality"
    FindingCategoryTestGap      FindingCategory = "test-gap"
    FindingCategoryArchitecture FindingCategory = "architecture"
    FindingCategoryAcceptance   FindingCategory = "acceptance"
)

// FindingScope indicates whether a finding is within the spec's changed files or general.
type FindingScope string

const (
    FindingScopeSpec    FindingScope = "spec"
    FindingScopeGeneral FindingScope = "general"
)

// Finding captures a single issue identified by spec-level or accept review.
type Finding struct {
    Severity      FindingSeverity
    Category      FindingCategory
    Scope         FindingScope
    Description   string
    AffectedFiles []string
}
```

In `StageRequest`, add:
```go
    Findings []Finding
```
after the `GapAnalysis` field.

**Acceptance Criteria:**
1. `go build ./internal/v2/stage/...` passes with no errors
2. All fields and constants are exported and named exactly as specified
3. `StageRequest` has `Findings []Finding` field

**Dependencies:** none

---

### Task 2: Write the Spec-Level Review Prompt Fragment

**Files:**
- Create: `review_spec_v2.md` (at project root, same level as `review_v2.md`, `accept_v2.md`)

**What to Do:**

Model this after `accept_v2.md` and `review_v2.md`. The fragment tells the LLM how to evaluate the cumulative diff holistically. It must produce valid JSON with the exact schema the `specreview` stage parses.

```markdown
# Spec-Level Review Instructions

You are performing a holistic code review of all changes made during a spec implementation.
Your goal is to find critical bugs, security issues, test gaps, and architectural drift that
span multiple beads or only become visible when reviewing the cumulative output.

## Review Scope

Evaluate:
- **Correctness**: Logic errors, off-by-one errors, incorrect conditionals, wrong return values
- **Security**: OWASP Top 10 — injection, broken auth, sensitive data exposure, XSS, insecure deserialization
- **Error handling**: Unchecked errors, missing context propagation, panic paths without recovery
- **Test coverage**: Missing tests for critical paths, tests that only assert documentation
- **Code quality**: Dead code, duplicated logic, exported names without comments, unexported types leaking across package boundaries
- **Architecture**: Violations of package contracts, nil-safety without centralized guards, state outside canonical locations

## Scope Classification

- `spec`: the finding is in files changed by this spec (visible in the diff)
- `general`: the finding is in surrounding code not changed by this spec

## Severity Classification

- `critical`: will cause incorrect behavior, data loss, security vulnerability, or test regression
- `warning`: degrades reliability or maintainability; should be fixed before merging
- `suggestion`: improvement worth making but not blocking

## Verdict

- `fail`: one or more `critical` findings exist
- `pass`: no critical findings (warnings and suggestions are allowed)

## Output Format

Output ONLY a JSON object. Do NOT output markdown or prose.

{
  "verdict": "pass",
  "findings": [
    {
      "severity": "critical",
      "category": "bug",
      "scope": "spec",
      "description": "what is wrong and where",
      "affected_files": ["path/to/file.go"]
    }
  ]
}

If there are no findings, output: {"verdict": "pass", "findings": []}
```

**Acceptance Criteria:**
1. File exists at project root as `review_spec_v2.md`
2. JSON schema in the file matches exactly what `specreview.go` will parse (verdict + findings array with severity/category/scope/description/affected_files)
3. All four severity and category values documented

**Dependencies:** none

---

### Task 3: Spec-Level Review Stage — Failing Tests

**Files:**
- Create: `internal/v2/stage/specreview/specreview_test.go`

**What to Do:**

Write the failing tests first. The stage takes `git GitDiffer`, `llm llmtypes.LLMProvider`, `planPath string`, `fragment string`. Its `Run` method:
1. Calls `git.DiffFromBase(ctx, req.Worktree)`
2. Reads the plan from the file at `planPath` (relative to worktree, typically `.gromit/v2/plan.md`)
3. Builds a prompt from the fragment + diff + plan
4. Calls `llm.Complete(ctx, prompt, req.Tier)`
5. Parses JSON response into `SpecReviewArtifacts{Verdict string, Findings []stage.Finding}`
6. Returns `DecisionProceed` when `verdict == "pass"`, `DecisionFail` when `verdict == "fail"`

Write tests:

```go
package specreview_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    "github.com/danabrams/gromit/internal/v2/stage/specreview"
    // mock types as used elsewhere in the codebase
)

func TestSpecReview_PassWhenNoFindings(t *testing.T) {
    // LLM returns {"verdict":"pass","findings":[]}
    // expect Decision==DecisionProceed, Artifacts.Verdict=="pass"
}

func TestSpecReview_FailWhenCriticalFinding(t *testing.T) {
    // LLM returns {"verdict":"fail","findings":[{"severity":"critical",...}]}
    // expect Decision==DecisionFail, Artifacts.Findings has one critical finding
}

func TestSpecReview_PassWithWarnings(t *testing.T) {
    // LLM returns {"verdict":"pass","findings":[{"severity":"warning",...}]}
    // expect Decision==DecisionProceed (not fail), Artifacts.Findings has one warning
}

func TestSpecReview_VerdictForcedFailOnCritical(t *testing.T) {
    // LLM returns {"verdict":"pass"} but findings contain a critical
    // stage must override to fail — enforces local verdict logic
}

func TestSpecReview_DiffFromBaseCalledWithWorktree(t *testing.T) {
    // Verify git.DiffFromBase is called with req.Worktree
}

func TestSpecReview_FindingsSurfacedInArtifacts(t *testing.T) {
    // All fields: severity, category, scope, description, affected_files
    // are preserved in Artifacts.Findings
}
```

Run: `go test ./internal/v2/stage/specreview/... -v`
Expected: FAIL (package does not exist yet)

**Acceptance Criteria:**
1. Tests compile once the package is created (they will fail until implementation exists)
2. Each test covers a distinct behavioral path
3. No `os.Chdir` in tests (use `t.Chdir` if directory change needed)

**Dependencies:** Task 1

---

### Task 4: Spec-Level Review Stage — Implementation

**Files:**
- Create: `internal/v2/stage/specreview/specreview.go`

**What to Do:**

```go
package specreview

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/danabrams/gromit/internal/v2/llmtypes"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

// GitDiffer provides DiffFromBase for the spec-level review.
type GitDiffer interface {
    DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// SpecReviewArtifacts carries the review outcome.
type SpecReviewArtifacts struct {
    Verdict  string            // "pass" or "fail"
    Findings []stagepkg.Finding
}

type Stage struct {
    git      GitDiffer
    llm      llmtypes.LLMProvider
    fragment string
}

func New(git GitDiffer, llm llmtypes.LLMProvider, fragment string) (*Stage, error) {
    if git == nil { return nil, fmt.Errorf("git adapter required") }
    if llm == nil { return nil, fmt.Errorf("llm provider required") }
    return &Stage{git: git, llm: llm, fragment: fragment}, nil
}

func (s *Stage) Name() string { return "spec-review" }

func (s *Stage) Run(ctx context.Context, req *stagepkg.StageRequest) (*stagepkg.StageResult, error) {
    diff, err := s.git.DiffFromBase(ctx, req.Worktree)
    if err != nil {
        return nil, fmt.Errorf("diff from base: %w", err)
    }

    planPath := filepath.Join(req.Worktree, ".gromit", "v2", "plan.md")
    planBytes, _ := os.ReadFile(planPath) // missing plan is non-fatal

    prompt := s.buildPrompt(string(planBytes), diff)
    output, err := s.llm.Complete(ctx, prompt, req.Tier)
    if err != nil {
        return nil, fmt.Errorf("llm: %w", err)
    }

    result, err := parseReviewOutput(output)
    if err != nil {
        return nil, fmt.Errorf("parse review output: %w", err)
    }

    // Enforce: any critical finding forces fail regardless of LLM verdict field
    for _, f := range result.Findings {
        if f.Severity == stagepkg.FindingSeverityCritical {
            result.Verdict = "fail"
            break
        }
    }

    decision := stagepkg.DecisionProceed
    if result.Verdict == "fail" {
        decision = stagepkg.DecisionFail
    }

    return &stagepkg.StageResult{
        Decision:  decision,
        Artifacts: result,
    }, nil
}

type llmReviewOutput struct {
    Verdict  string `json:"verdict"`
    Findings []struct {
        Severity      string   `json:"severity"`
        Category      string   `json:"category"`
        Scope         string   `json:"scope"`
        Description   string   `json:"description"`
        AffectedFiles []string `json:"affected_files"`
    } `json:"findings"`
}

func parseReviewOutput(raw string) (*SpecReviewArtifacts, error) {
    raw = strings.TrimSpace(raw)
    var out llmReviewOutput
    if err := json.Unmarshal([]byte(raw), &out); err != nil {
        return nil, fmt.Errorf("json: %w (raw: %.200s)", err, raw)
    }
    findings := make([]stagepkg.Finding, 0, len(out.Findings))
    for _, f := range out.Findings {
        findings = append(findings, stagepkg.Finding{
            Severity:      stagepkg.FindingSeverity(f.Severity),
            Category:      stagepkg.FindingCategory(f.Category),
            Scope:         stagepkg.FindingScope(f.Scope),
            Description:   f.Description,
            AffectedFiles: f.AffectedFiles,
        })
    }
    return &SpecReviewArtifacts{Verdict: out.Verdict, Findings: findings}, nil
}

func (s *Stage) buildPrompt(plan, diff string) string {
    var sb strings.Builder
    sb.WriteString(s.fragment)
    sb.WriteString("\n\n## Plan\n\n")
    sb.WriteString(plan)
    sb.WriteString("\n\n## Cumulative Diff\n\n```diff\n")
    sb.WriteString(diff)
    sb.WriteString("\n```\n")
    return sb.String()
}
```

Run: `go test ./internal/v2/stage/specreview/... -v`
Expected: PASS

Run: `go build ./...`
Expected: no errors

**Acceptance Criteria:**
1. All tests from Task 3 pass
2. Critical findings force `verdict: fail` even if LLM returned `"pass"` in the verdict field
3. `go build ./...` passes

**Dependencies:** Task 3

---

### Task 5: Accept Stage — Structured Findings Output (Failing Tests)

**Files:**
- Modify: `internal/v2/stage/accept/accept_test.go`

**What to Do:**

Add tests verifying that when acceptance criteria fail, the `AcceptArtifacts.Findings` field is populated with `Finding{Severity: "critical", Category: "acceptance", Scope: "spec"}` for each failing criterion.

```go
func TestAccept_FindingsPopulatedOnFailure(t *testing.T) {
    // Setup: LLM returns {"pass":false,"summary":"criterion not met"} for one criterion
    // Expect: AcceptArtifacts.Findings has one Finding with:
    //   Severity == FindingSeverityCritical
    //   Category == FindingCategoryAcceptance
    //   Scope == FindingScopeSpec
    //   Description contains the criterion text
}

func TestAccept_FindingsEmptyOnPass(t *testing.T) {
    // Setup: LLM returns {"pass":true} for all criteria
    // Expect: AcceptArtifacts.Findings is empty
}
```

Run: `go test ./internal/v2/stage/accept/... -v`
Expected: FAIL (Findings field does not exist on AcceptArtifacts yet)

**Acceptance Criteria:**
1. Tests added to the existing test file
2. Tests are behavioral (assert Findings content, not just documentation)
3. Tests fail before implementation (they reference the not-yet-added Findings field)

**Dependencies:** Task 1

---

### Task 6: Accept Stage — Structured Findings Implementation

**Files:**
- Modify: `internal/v2/stage/accept/accept.go`

**What to Do:**

Add `Findings []stage.Finding` to `AcceptArtifacts`:

```go
type AcceptArtifacts struct {
    Results    []presentation.AcceptanceResult
    GapSummary string
    Findings   []stage.Finding  // one per failing criterion
}
```

In the criterion evaluation loop, when a criterion fails, append a finding:

```go
failures = append(failures, criterion+" — "+summary)
artifacts.Findings = append(artifacts.Findings, stage.Finding{
    Severity:      stage.FindingSeverityCritical,
    Category:      stage.FindingCategoryAcceptance,
    Scope:         stage.FindingScopeSpec,
    Description:   criterion + ": " + summary,
    AffectedFiles: []string{},
})
```

The `GapSummary` field and `writeGapAnalysis` call remain unchanged for backward compat with any code that reads the gap analysis file.

Run: `go test ./internal/v2/stage/accept/... -v`
Expected: PASS (new tests from Task 5 now pass, existing tests still pass)

**Acceptance Criteria:**
1. All existing accept tests pass
2. New tests from Task 5 pass
3. `Findings` is nil/empty when all criteria pass
4. Each failing criterion produces exactly one Finding with severity=critical, category=acceptance, scope=spec

**Dependencies:** Task 5

---

### Task 7: Decompose Stage — Findings-Based Template (Failing Tests)

**Files:**
- Modify: `internal/v2/stage/decompose/decompose_test.go`

**What to Do:**

Add tests verifying that when `req.Findings` is non-empty, the decompose stage uses a findings-based prompt template instead of the gap analysis template.

```go
func TestDecompose_FindingsTemplateUsedWhenFindingsPresent(t *testing.T) {
    // Setup: req.Remediation=true, req.Findings=[{...}]
    // Expect: LLM prompt contains findings content (not original plan decompose text)
    // Expect: result contains beads created from findings
}

func TestDecompose_GapAnalysisTemplateUsedWhenNoFindings(t *testing.T) {
    // Setup: req.Remediation=true, req.GapAnalysis="some gap", req.Findings=nil
    // Expect: LLM prompt contains gap analysis (original behavior preserved)
}
```

Run: `go test ./internal/v2/stage/decompose/... -v`
Expected: Tests likely compile but fail (routing logic not yet changed)

**Acceptance Criteria:**
1. Tests express the routing distinction clearly
2. Tests reference `req.Findings` (now valid after Task 1)

**Dependencies:** Task 1, Task 6

---

### Task 8: Decompose Stage — Findings-Based Template Implementation

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go`

**What to Do:**

Add a `findingsDecomposePromptTemplate`. The template receives a JSON-serialized list of findings and instructs the LLM to create one or more targeted fix beads per finding. Keep the existing templates unchanged.

```go
const findingsDecomposePromptTemplate = `You are creating targeted fix beads for specific findings from a spec-level code review.

## Spec: %s

## Original Plan (for context only — do not re-decompose it)
%s

## Findings to Fix
%s

Create one or more beads that directly address each finding above. Do NOT create beads for work already done. Each bead must reference the specific finding it fixes in its description.

Output the beads as a JSON array using the same schema as normal decompose output.`
```

In the routing logic (around the `resolveGapAnalysis` call), add:

```go
if req.Remediation && len(req.Findings) > 0 {
    findingsJSON, _ := json.Marshal(req.Findings)
    promptText = fmt.Sprintf(findingsDecomposePromptTemplate, specID, string(planBody), string(findingsJSON))
} else if req.Remediation && gapAnalysis != "" {
    promptText = fmt.Sprintf(remediationDecomposePromptTemplate, ...)
} else {
    // normal template
}
```

Run: `go test ./internal/v2/stage/decompose/... -v`
Expected: PASS

**Acceptance Criteria:**
1. All tests from Task 7 pass
2. All pre-existing decompose tests pass
3. When `req.Findings` is non-empty AND `req.Remediation` is true, findings template is selected
4. When `req.GapAnalysis` is set and `req.Findings` is empty, gap analysis template is selected (no regression)

**Dependencies:** Task 7

---

### Task 9: Remediation Runner — Findings-Based Interface (Failing Tests)

**Files:**
- Modify: `internal/v2/remediation/remediation_test.go`

**What to Do:**

The remediation runner's `Run` method will change signature to accept findings. It will no longer run accept internally. Add tests:

```go
func TestRemediationRunner_RunWithFindings_CallsDecomposeAndBeadRunner(t *testing.T) {
    // Setup: provide findings, mock decompose and bead runner
    // Expect: decompose called with req.Findings set, bead runner called
    // Expect: accept stage NOT called
}

func TestRemediationRunner_RunWithFindings_GenerationCapRespected(t *testing.T) {
    // Setup: generationCap=1, call Run twice
    // Expect: second call returns generation cap error
}
```

Run: `go test ./internal/v2/remediation/... -v`
Expected: FAIL (signature mismatch or new behavior not yet present)

**Acceptance Criteria:**
1. Tests express the new contract clearly: findings in, no accept invoked
2. Tests reference the new `Run(..., findings []stage.Finding)` signature

**Dependencies:** Task 1

---

### Task 10: Remediation Runner — Findings-Based Implementation

**Files:**
- Modify: `internal/v2/remediation/remediation.go`

**What to Do:**

Change `Run` signature and remove the internal accept loop:

```go
// Run executes one remediation generation: decompose targeted fix beads from findings, then run them.
// The caller (spec loop) owns the outer accept+review retry cycle.
func (r *RemediationRunner) Run(ctx context.Context, specID, worktree string, findings []stage.Finding) error {
    if specID == "" {
        return ErrSpecIDRequired
    }
    if !r.canRemediate() {
        return r.handleGenerationCap(ctx, specID)
    }

    req := stage.Request{
        Bead:        stage.BeadInfo{ID: specID},
        Worktree:    worktree,
        Remediation: true,
        Findings:    findings,
    }

    beads, err := r.decompose(ctx, &req)
    if err != nil {
        return err
    }

    if r.cfg.BeadRunner == nil {
        return ErrBeadRunnerRequired
    }
    if err := r.cfg.BeadRunner.Run(ctx, beads); err != nil {
        return err
    }

    r.generationCount++
    return nil
}
```

Remove `AcceptStage` from `RemediationRunnerConfig`. Update config validation to remove `ErrAcceptStageRequired` check.

Run: `go test ./internal/v2/remediation/... -v`
Expected: PASS

Run: `go build ./...`
Expected: compile errors in `spec_loop.go` (interface mismatch — expected, fixed in Task 11)

**Acceptance Criteria:**
1. All tests from Task 9 pass
2. `Run` no longer calls accept stage
3. `Run` sets `req.Findings` from the provided findings slice
4. Generation cap still enforced

**Dependencies:** Task 9

---

### Task 11: Spec Loop — Spec-Level Review Stage + Combined Gating (Failing Tests)

**Files:**
- Create: `internal/v2/loop/spec_loop_specreview_test.go`

**What to Do:**

Write integration tests for the new spec loop behavior:

```go
func TestSpecLoop_SpecReviewCalledAfterAcceptPass(t *testing.T) {
    // accept passes, spec-level review called
    // review returns pass → spec proceeds to present
}

func TestSpecLoop_SpecReviewFailTriggerRemediation(t *testing.T) {
    // accept passes, spec-level review returns critical finding
    // remediation called with combined findings (review findings)
    // after remediation: accept + review called again, both pass
    // spec proceeds to present
}

func TestSpecLoop_AcceptFailTriggerRemediationWithFindings(t *testing.T) {
    // accept fails with unmet criterion
    // remediation called with accept's structured findings
}

func TestSpecLoop_BothFailCombinesFindings(t *testing.T) {
    // accept fails (1 finding) + review fails (1 critical finding)
    // remediation called with 2 combined findings
}

func TestSpecLoop_PassWithImprovements_SpecScopedBeadsCreated(t *testing.T) {
    // accept passes, review passes with spec-scoped warning finding
    // from-review bead created with labels ["from-review", "spec:<specID>"]
    // spec proceeds to present
}

func TestSpecLoop_PassWithImprovements_GeneralBeadsCreated(t *testing.T) {
    // review passes with general-scoped finding
    // from-review bead created with label ["from-review"] only (no spec label)
}
```

Run: `go test ./internal/v2/loop/... -run TestSpecLoop_SpecReview -v`
Expected: FAIL (specReviewStage field does not exist on SpecLoop yet)

**Acceptance Criteria:**
1. Tests are behavioral, not documentation-only
2. Tests do not use `os.Chdir`
3. Tests restore any package-level vars via `t.Cleanup`

**Dependencies:** Task 4, Task 10

---

### Task 12: Spec Loop — Spec-Level Review Integration Implementation

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`

**What to Do:**

**Step 1:** Update the `remediationRunner` interface:
```go
type remediationRunner interface {
    Run(ctx context.Context, specID, worktree string, findings []stagepkg.Finding) error
}
```

**Step 2:** Add `specReviewStage stagepkg.Stage` field and `WithSpecReviewStage` option:
```go
func WithSpecReviewStage(stage stagepkg.Stage) SpecLoopOption {
    return func(s *SpecLoop) { s.specReviewStage = stage }
}
```

**Step 3:** Replace `ensureAcceptance` with `ensureAcceptanceAndReview`:
```go
func (s *SpecLoop) ensureAcceptanceAndReview(ctx context.Context, req *stagepkg.Request, specID string) (*stagepkg.Result, error) {
    retriesRemaining := maxAcceptanceRetries
    for {
        if err := s.ctxErr(ctx); err != nil {
            return nil, err
        }

        s.applyRouting(req, "accept")
        acceptRes, err := s.runAcceptStage(ctx, req)
        if err != nil {
            return acceptRes, err
        }

        var reviewRes *stagepkg.Result
        if !s.acceptFailed(acceptRes) && s.specReviewStage != nil {
            s.applyRouting(req, "spec-review")
            reviewRes, err = s.specReviewStage.Run(ctx, req)
            if err != nil {
                return reviewRes, err
            }
        }

        acceptFindings := extractFindings(acceptRes)
        reviewFindings := extractSpecReviewFindings(reviewRes)
        combined := append(acceptFindings, reviewFindings...)

        bothPass := !s.acceptFailed(acceptRes) && !s.specReviewFailed(reviewRes)
        if bothPass {
            if err := s.createFromReviewBeads(ctx, specID, reviewFindings); err != nil {
                return acceptRes, err
            }
            return acceptRes, nil
        }

        if s.remediationRunner == nil {
            return acceptRes, fmt.Errorf("accept or spec-review failed")
        }
        if retriesRemaining <= 0 {
            return acceptRes, fmt.Errorf("%w: limit %d reached", ErrAcceptanceRetriesExceeded, maxAcceptanceRetries)
        }
        if err := s.remediationRunner.Run(ctx, specID, req.Worktree, combined); err != nil {
            return acceptRes, err
        }
        retriesRemaining--
    }
}
```

**Step 4:** In `Run()`, replace the `ensureAcceptance` call with `ensureAcceptanceAndReview`.

**Step 5:** Add helpers `specReviewFailed`, `extractFindings`, `extractSpecReviewFindings`.

Run: `go test ./internal/v2/loop/... -run TestSpecLoop_SpecReview -v`
Expected: some tests pass (depends on from-review bead creation in Task 13)

**Acceptance Criteria:**
1. `remediationRunner` interface updated to new signature
2. `ensureAcceptanceAndReview` drives both accept and spec-level review
3. Combined findings passed to remediation when either fails
4. `specReviewStage == nil` is safe: skips review, accept-only gating preserved

**Dependencies:** Task 11

---

### Task 13: Spec Loop — Pass-With-Improvements Bead Creation

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`

**What to Do:**

Add `createFromReviewBeads` method:

```go
func (s *SpecLoop) createFromReviewBeads(ctx context.Context, specID string, findings []stagepkg.Finding) error {
    if len(findings) == 0 || s.adapters.TaskTracker == nil {
        return nil
    }
    for _, f := range findings {
        labels := []string{"from-review"}
        if f.Scope == stagepkg.FindingScopeSpec {
            labels = append(labels, "spec:"+specID)
        }
        _, err := s.adapters.TaskTracker.CreateBead(ctx, trackertypes.CreateBeadRequest{
            Title:       fmt.Sprintf("Review improvement: %s", f.Description),
            Description: f.Description,
            Priority:    bead.PriorityP1,
            Labels:      labels,
        })
        if err != nil {
            return fmt.Errorf("create from-review bead: %w", err)
        }
    }
    return nil
}
```

Run: `go test ./internal/v2/loop/... -run TestSpecLoop -v`
Expected: all spec loop specreview tests pass

**Acceptance Criteria:**
1. Spec-scoped findings produce beads labeled `["from-review", "spec:<specID>"]`
2. General findings produce beads labeled `["from-review"]` only
3. No beads created when findings slice is empty
4. `TaskTracker == nil` is safe (returns nil, no panic)

**Dependencies:** Task 12

---

### Task 14: Run2 Components — Wire Spec-Level Review + Update RemediationRunner

**Files:**
- Modify: `internal/v2/loop/run2_components.go`

**What to Do:**

**Step 1:** Add `SpecReviewStage stagepkg.Stage` to `Run2LoopComponents`.

**Step 2:** In `NewRun2LoopComponents`, load `review_spec_v2.md` fragment:
```go
specReviewFragment, err := loadOptionalFragment(cfg.ProjectRoot, "review_spec_v2.md")
```

**Step 3:** Construct the spec-level review stage:
```go
specReviewStage, err := specreviewstage.New(adapters.Git, adapters.LLM, specReviewFragment)
if err != nil {
    cleanup()
    return nil, fmt.Errorf("spec review stage: %w", err)
}
components.SpecReviewStage = specReviewStage
```

**Step 4:** Remove `AcceptStage` from `RemediationRunnerConfig` construction call.

**Step 5:** In `run2.go`, pass `SpecReviewStage` via `WithSpecReviewStage`:
```go
loopOpts = append(loopOpts, loop.WithSpecReviewStage(components.SpecReviewStage))
```

**Step 6:** Add `"spec-review"` to `StageSequence` in `spec_loop.go`.

Run: `go build ./...`
Expected: PASS

Run: `go test ./...`
Expected: PASS

**Acceptance Criteria:**
1. `go build ./...` passes
2. `go test ./...` passes
3. `SpecReviewStage` constructed with git adapter + LLM + fragment
4. RemediationRunner no longer receives AcceptStage

**Dependencies:** Task 4, Task 12, Task 13

---

### Task 15: `--from-review` Flag on run2 (Failing Tests)

**Files:**
- Modify: `cmd/gromit/run2_test.go` (or create if absent)

**What to Do:**

```go
func TestRun2_FromReviewFlag_QueriesFromReviewBeads(t *testing.T) {
    // Setup: task tracker has two open beads: one with "from-review", one without
    // Run with --from-review
    // Expect: only the from-review bead is passed to bead loop
}

func TestRun2_FromReviewFlag_WithSpec_FiltersToSpecLabel(t *testing.T) {
    // Setup: two from-review beads: one with "spec:foo", one with "spec:bar"
    // Run with --from-review --spec foo
    // Expect: only the "spec:foo" bead passed to bead loop
}

func TestRun2_FromReviewFlag_NoAcceptOrReviewInvoked(t *testing.T) {
    // Run with --from-review
    // Expect: accept stage not called, spec-review stage not called
}
```

Run: `go test ./cmd/gromit/... -run TestRun2_FromReview -v`
Expected: FAIL

**Acceptance Criteria:**
1. Tests cover bead filtering by label
2. Tests verify accept/review stages are not invoked in from-review mode

**Dependencies:** Task 12

---

### Task 16: `--from-review` Flag Implementation

**Files:**
- Modify: `cmd/gromit/run2.go`

**What to Do:**

```go
fromReview, _ := cmd.Flags().GetBool("from-review")
fromReviewSpec, _ := cmd.Flags().GetString("spec") // reuse existing flag if present
```

When `fromReview` is true:
1. Skip plan, skip decompose
2. Query open beads with label `"from-review"` (and optional spec label)
3. Pass them directly to `components.BeadLoop.Run`
4. Do NOT run accept or spec-level review after the bead loop

```go
if fromReview {
    labels := []string{"from-review"}
    if fromReviewSpec != "" {
        labels = append(labels, "spec:"+fromReviewSpec)
    }
    resp, err := taskTracker.QueryBeads(ctx, trackertypes.TaskTrackerQueryBeadsRequest{
        Labels: labels,
        Status: "open",
    })
    if err != nil {
        return fmt.Errorf("query from-review beads: %w", err)
    }
    beads := convertBeads(resp.Beads)
    _, err = components.BeadLoop.Run(ctx, beads, stopCh)
    return err
}
```

Register the flag:
```go
run2Cmd.Flags().Bool("from-review", false, "run only beads labeled from-review through the bead loop")
```

Run: `go test ./cmd/gromit/... -run TestRun2_FromReview -v`
Expected: PASS

Run: `go build ./...`
Expected: PASS

**Acceptance Criteria:**
1. All tests from Task 15 pass
2. `--from-review` skips plan, decompose, accept, and spec-level review stages
3. `--spec <id>` with `--from-review` scopes to beads with `spec:<id>` label
4. From-review beads are not re-triaged for accept/review after bead loop

**Dependencies:** Task 15, Task 14

---

### Task 17: Full Test Suite Pass + Build

**Files:** none

**What to Do:**

```bash
go build ./...
go test ./... -v 2>&1 | tail -50
```

Check for:
- Any test that calls `remediationRunner.Run` with old signature (2 args → needs updating to 4 args)
- Any test that injects `AcceptStage` into `RemediationRunnerConfig` (remove those)
- Any test that asserts accept is called by the remediation runner (update expectations)

Fix each failure individually.

**Acceptance Criteria:**
1. `go build ./...` exits 0
2. `go test ./...` exits 0 with no test failures
3. No `os.Chdir` in test files (`grep -rn 'os\.Chdir' cmd/ internal/ --include='*_test.go'` returns zero hits)

**Dependencies:** Task 16

---

### Task 18: Commit

**Files:** all modified/created

**What to Do:**

```bash
git add \
  internal/v2/stage/stage.go \
  internal/v2/stage/specreview/ \
  internal/v2/stage/accept/accept.go \
  internal/v2/stage/accept/accept_test.go \
  internal/v2/stage/decompose/decompose.go \
  internal/v2/stage/decompose/decompose_test.go \
  internal/v2/remediation/remediation.go \
  internal/v2/remediation/remediation_test.go \
  internal/v2/loop/spec_loop.go \
  internal/v2/loop/spec_loop_specreview_test.go \
  internal/v2/loop/run2_components.go \
  cmd/gromit/run2.go \
  review_spec_v2.md \
  docs/plans/2026-03-09-spec-level-review-and-targeted-remediation.md

git commit -m "feat: spec-level review, targeted remediation, and --from-review flag"
```

**Acceptance Criteria:**
1. Commit includes all new and modified files
2. No unrelated files staged

**Dependencies:** Task 17