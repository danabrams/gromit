---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-10
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a spec-level code review stage that evaluates the cumulative diff after all beads complete, and route its structured findings (combined with accept failures) into targeted remediation decomposition instead of re-decomposing the whole plan.

**Architecture:** Introduce `internal/v2/stage/specreview/` as a new Stage that runs after accept, produces structured `ReviewFinding` values, and combines with accept's unmet-criteria findings to drive a new findings-based decompose template. The `remediationRunner` interface gains a `Remediate` method that takes findings instead of gap analysis text. SpecLoop's `ensureAcceptance` expands to run specreview, collect combined findings, and call `Remediate`. A `--from-review` flag on `run2` runs open `from-review` beads directly through the bead loop.

**Tech Stack:** Go 1.24+, existing `internal/v2/stage`, `internal/v2/llmtypes`, `internal/v2/routing`, `internal/v2/trackertypes`, cobra CLI flags, JSON output parsing via `internal/jsonutil`.

---

## Architecture

### Shared finding type
`ReviewFinding` lives in a new `internal/v2/stage/findings.go` file so every package (`accept`, `specreview`, `decompose`, `remediation`, `loop`) can import it without circular deps. `StageRequest` gains two new fields: `FindingsDecompose bool` and `Findings []ReviewFinding`.

### Accept stage
`AcceptArtifacts.Findings []ReviewFinding` is populated for each failed criterion with `severity: critical`, `category: acceptance`, `scope: spec`.

### Specreview stage
New package `internal/v2/stage/specreview/`. Reads `DiffFromBase` and the plan file from the worktree; invokes the highest-tier LLM with the `review_spec_v2.md` fragment prompt; parses structured JSON; derives verdict from findings. Creates `from-review` beads in TaskTracker for pass-with-improvements paths.

### Decompose stage
New `findingsDecomposePromptTemplate` selected when `req.FindingsDecompose && len(req.Findings) > 0`. Each finding becomes one or more targeted beads.

### Remediation runner
New `Remediate(ctx, specID, worktree, findings []stage.ReviewFinding) error` method replaces the gap-analysis path. Sets `req.FindingsDecompose = true`, `req.Findings = findings`, then decomposes and runs the bead loop once. Generation cap still applies.

### Spec loop
`ensureAcceptance` renamed `ensureAcceptanceAndReview`. Runs accept, then specreview. If both pass, return. Otherwise collect combined findings (accept.Findings + specreview.Findings) and call `remediationRunner.Remediate`. The `remediationRunner` interface changes to expose `Remediate`.

### run2 --from-review
New flag skips spec resolution, plan, decompose, accept, and review. Queries open beads with label `from-review` (scoped to `spec:<id>` when `--spec` is provided). Passes beads directly to the bead loop.

## Test Strategy

Unit tests per package: specreview (verdict logic, JSON parsing, bead creation), decompose (findings template selection), remediation (Remediate method), accept (findings field populated), spec loop (specreview runs, combined findings, Remediate called). Integration test for the full post-bead-loop pipeline and pass-with-improvements bead creation. All new packages follow the existing pattern: `var _ stagepkg.Stage = (*Stage)(nil)` compile-time check.

## Implementation Tasks

---

### Task 1: ReviewFinding type and Request fields

**Files:**
- Create: `internal/v2/stage/findings.go`
- Modify: `internal/v2/stage/stage.go`
- Create: `internal/v2/stage/findings_test.go`

**What to Do:**

Create `internal/v2/stage/findings.go`:

```go
package stage

// ReviewFinding represents a single observation from accept or spec-level review.
type ReviewFinding struct {
    Severity     string   // "critical" | "warning" | "suggestion"
    Category     string   // "bug" | "security" | "quality" | "test-gap" | "architecture" | "acceptance"
    Scope        string   // "spec" | "general"
    Description  string
    AffectedFiles []string
}

// SpecReviewArtifacts captures the output of the spec-level review stage.
type SpecReviewArtifacts struct {
    Verdict  string          // "pass" | "fail"
    Findings []ReviewFinding
}
```

Add two fields to `StageRequest` in `internal/v2/stage/stage.go` (after the existing `GapAnalysis` field):

```go
    FindingsDecompose bool
    Findings          []ReviewFinding
```

**Step 1: Write the failing test**

In `internal/v2/stage/findings_test.go`:

```go
package stage_test

import (
    "testing"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestReviewFinding_Fields(t *testing.T) {
    f := stagepkg.ReviewFinding{
        Severity:      "critical",
        Category:      "acceptance",
        Scope:         "spec",
        Description:   "criterion not met",
        AffectedFiles: []string{"foo.go"},
    }
    if f.Severity != "critical" {
        t.Errorf("expected critical, got %s", f.Severity)
    }
    if len(f.AffectedFiles) != 1 {
        t.Errorf("expected 1 file, got %d", len(f.AffectedFiles))
    }
}

func TestSpecReviewArtifacts_Fields(t *testing.T) {
    a := stagepkg.SpecReviewArtifacts{
        Verdict:  "fail",
        Findings: []stagepkg.ReviewFinding{{Severity: "critical", Category: "bug", Scope: "spec"}},
    }
    if a.Verdict != "fail" {
        t.Errorf("expected fail, got %s", a.Verdict)
    }
}

func TestStageRequest_FindingsFields(t *testing.T) {
    req := stagepkg.StageRequest{
        FindingsDecompose: true,
        Findings: []stagepkg.ReviewFinding{
            {Severity: "warning", Category: "quality", Scope: "general"},
        },
    }
    if !req.FindingsDecompose {
        t.Error("expected FindingsDecompose true")
    }
    if len(req.Findings) != 1 {
        t.Errorf("expected 1 finding, got %d", len(req.Findings))
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd internal/v2/stage && go test ./... -run TestReviewFinding -v
```
Expected: FAIL — `ReviewFinding undefined`

**Step 3: Implement**

Create `internal/v2/stage/findings.go` with the types above. Add `FindingsDecompose bool` and `Findings []ReviewFinding` to `StageRequest` in `stage.go`.

**Step 4: Run test to verify it passes**

```bash
cd internal/v2/stage && go test ./... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/stage/findings.go internal/v2/stage/stage.go internal/v2/stage/findings_test.go
git commit -m "red/green: ReviewFinding type + StageRequest fields for findings-based decompose"
```

**Acceptance Criteria:**
- `ReviewFinding` struct exists in `internal/v2/stage` with Severity, Category, Scope, Description, AffectedFiles
- `SpecReviewArtifacts` exists with Verdict and Findings
- `StageRequest` has `FindingsDecompose bool` and `Findings []ReviewFinding`

---

### Task 2: Accept stage structured findings output

**Files:**
- Modify: `internal/v2/stage/accept/accept.go`
- Modify: `internal/v2/stage/accept/accept_test.go` (or create if tests live elsewhere)

**What to Do:**

Add `Findings []stage.ReviewFinding` to `AcceptArtifacts`. When criteria fail, populate a `ReviewFinding` per unmet criterion with `Severity: "critical"`, `Category: "acceptance"`, `Scope: "spec"`.

In `accept.go`, update `AcceptArtifacts`:

```go
import stagepkg "github.com/danabrams/gromit/internal/v2/stage"

type AcceptArtifacts struct {
    Results    []presentation.AcceptanceResult
    GapSummary string
    Findings   []stagepkg.ReviewFinding
}
```

In each evaluation path that builds failures (batch, targeted, per-criterion), convert failures to findings. Add a helper:

```go
func criterionToFinding(criterionText, summary string) stagepkg.ReviewFinding {
    return stagepkg.ReviewFinding{
        Severity:    "critical",
        Category:    "acceptance",
        Scope:       "spec",
        Description: fmt.Sprintf("%s — %s", criterionText, summary),
    }
}
```

Call `criterionToFinding` for every failed criterion and append to `artifacts.Findings`.

**Step 1: Write the failing test**

In `accept_test.go`, add:

```go
func TestAcceptStage_FailedCriteriaPopulateFindings(t *testing.T) {
    // Use an LLM provider that returns {"pass": false, "summary": "not done"}
    // and a spec with one criterion.
    // Verify AcceptArtifacts.Findings has one entry with severity=critical, category=acceptance.
    artifacts := runAcceptWithMockFailure(t)
    if len(artifacts.Findings) != 1 {
        t.Fatalf("expected 1 finding, got %d", len(artifacts.Findings))
    }
    f := artifacts.Findings[0]
    if f.Severity != "critical" {
        t.Errorf("expected critical severity, got %s", f.Severity)
    }
    if f.Category != "acceptance" {
        t.Errorf("expected acceptance category, got %s", f.Category)
    }
    if f.Scope != "spec" {
        t.Errorf("expected spec scope, got %s", f.Scope)
    }
}
```

The helper `runAcceptWithMockFailure` invokes the stage with a fake LLM that returns `{"pass": false, "summary": "not done"}` against a spec file with one criterion.

**Step 2: Run test to verify it fails**

```bash
cd internal/v2/stage/accept && go test ./... -run TestAcceptStage_FailedCriteriaPopulateFindings -v
```
Expected: FAIL — `Findings` field not yet defined

**Step 3: Implement**

Add `Findings []stagepkg.ReviewFinding` to `AcceptArtifacts`. Add `criterionToFinding` helper. In `buildBatchResult`, `runPerCriterionEvaluation`, and `runTargetedEvaluation`, append a finding for each failure. In `allCriteriaFailed`, populate findings for every criterion.

**Step 4: Run tests**

```bash
cd internal/v2/stage/accept && go test ./... -v
```
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/v2/stage/accept/accept.go internal/v2/stage/accept/accept_test.go
git commit -m "green: AcceptArtifacts.Findings populated for failed acceptance criteria"
```

**Acceptance Criteria:**
- `AcceptArtifacts.Findings []stage.ReviewFinding` field exists
- Each failed criterion produces a finding with `severity: critical`, `category: acceptance`, `scope: spec`
- Passing criteria produce no findings

---

### Task 3: Findings-based decompose template

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go`
- Modify: `internal/v2/stage/decompose/decompose_test.go`

**What to Do:**

Add a new `findingsDecomposePromptTemplate` constant. In `Run()`, add a new branch before the existing `req.Remediation` check:

```go
if req.FindingsDecompose && len(req.Findings) > 0 {
    promptText = buildFindingsDecomposePrompt(specID, string(planBody), req.Findings)
} else if req.Remediation && gapAnalysis != "" {
    promptText = fmt.Sprintf(remediationDecomposePromptTemplate, ...)
} else {
    promptText = fmt.Sprintf(s.promptTemplate, ...)
}
```

Add the template and builder:

```go
var findingsDecomposePromptTemplate = `# Targeted Fix Decompose: %s

You are creating TARGETED beads to address specific review findings. Do NOT re-implement work that has already been completed.

## Full Plan (for architectural context only)

%s

## Findings Requiring Fix Beads

%s

## Skill Instructions

%s

## Rules

- Create one or more beads per finding.
- Beads must address the specific finding; do not duplicate existing work.
- acceptance_criteria: each criterion MUST describe an observable behavior, NOT a file path or function name.
- depends_on_index: chain beads that depend on each other.

## Output

Output ONLY a JSON array of bead definitions. No markdown, no explanations.
Each bead must include: title, description, priority, acceptance_criteria, expected_outputs, covers_tasks, depends_on_index.

The spec label will be added automatically: spec:%s
`

func buildFindingsDecomposePrompt(specID, planBody string, findings []stagepkg.ReviewFinding) string {
    var sb strings.Builder
    for i, f := range findings {
        fmt.Fprintf(&sb, "%d. [%s/%s] %s", i+1, f.Severity, f.Category, f.Description)
        if len(f.AffectedFiles) > 0 {
            fmt.Fprintf(&sb, " (files: %s)", strings.Join(f.AffectedFiles, ", "))
        }
        sb.WriteByte('\n')
    }
    return fmt.Sprintf(findingsDecomposePromptTemplate, specID, planBody, sb.String(), skills.DecomposeSkill, specID)
}
```

**Step 1: Write the failing test**

In `decompose_test.go`, add:

```go
func TestDecompose_FindingsDecomposeTemplateSelected(t *testing.T) {
    // Arrange: mock LLM that captures prompt text; tracker that records created beads
    capturedPrompt := ""
    mockLLM := &mockProvider{
        invokeFn: func(ctx context.Context, req llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
            capturedPrompt = req.Prompt
            return &llmtypes.LLMInvokeResponse{Success: true, Output: `[{"title":"Fix bug","description":"fix it","priority":"P1","acceptance_criteria":["behavior works"],"expected_outputs":["fixed behavior"],"covers_tasks":[1],"depends_on_index":[]}]`}, nil
        },
    }
    // ... set up stage, write plan.md to temp dir ...
    req := &stagepkg.StageRequest{
        Bead:              stagepkg.BeadInfo{ID: "my-spec"},
        Worktree:          tmpDir,
        Config:            cfg,
        FindingsDecompose: true,
        Findings: []stagepkg.ReviewFinding{
            {Severity: "critical", Category: "bug", Scope: "spec", Description: "nil dereference in handler"},
        },
    }
    _, err := stage.Run(ctx, req)
    if err != nil { t.Fatal(err) }
    if !strings.Contains(capturedPrompt, "Targeted Fix Decompose") {
        t.Errorf("expected findings template, got: %s", capturedPrompt[:200])
    }
    if !strings.Contains(capturedPrompt, "nil dereference") {
        t.Errorf("expected finding description in prompt")
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd internal/v2/stage/decompose && go test ./... -run TestDecompose_FindingsDecomposeTemplateSelected -v
```
Expected: FAIL

**Step 3: Implement**

Add `findingsDecomposePromptTemplate`, `buildFindingsDecomposePrompt`, and the new branch in `Run()`.

**Step 4: Run tests**

```bash
cd internal/v2/stage/decompose && go test ./... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/stage/decompose/decompose.go internal/v2/stage/decompose/decompose_test.go
git commit -m "green: findings-based decompose template for targeted remediation"
```

**Acceptance Criteria:**
- When `req.FindingsDecompose == true && len(req.Findings) > 0`, the findings template is used
- Each finding's description appears in the prompt
- The existing remediation and default templates still work when their conditions are met

---

### Task 4: Remediation runner Remediate method

**Files:**
- Modify: `internal/v2/remediation/remediation.go`
- Modify: `internal/v2/remediation/remediation_test.go`

**What to Do:**

Add a `Remediate(ctx context.Context, specID, worktree string, findings []stage.ReviewFinding) error` method to `RemediationRunner`. This replaces the role of `executeRemediation` for the spec-loop-driven path.

```go
// Remediate runs one targeted remediation cycle for the provided findings.
// It sets req.FindingsDecompose and req.Findings, decomposes targeted beads,
// runs the bead loop, and increments the generation counter.
// Returns an error if the generation cap is reached.
func (r *RemediationRunner) Remediate(ctx context.Context, specID, worktree string, findings []stage.ReviewFinding) error {
    if specID == "" {
        return ErrSpecIDRequired
    }
    if !r.canRemediate() {
        return r.handleGenerationCap(ctx, specID)
    }

    req := &stage.Request{
        Bead:              stage.BeadInfo{ID: specID},
        Worktree:          worktree,
        Remediation:       true,
        FindingsDecompose: true,
        Findings:          findings,
    }

    // Persist a record of this remediation generation.
    if worktree != "" {
        var sb strings.Builder
        for i, f := range findings {
            fmt.Fprintf(&sb, "%d. [%s/%s] %s\n", i+1, f.Severity, f.Category, f.Description)
        }
        planPath := r.remediationPlanPath(worktree)
        if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
            return fmt.Errorf("create remediation plan dir: %w", err)
        }
        if err := os.WriteFile(planPath, []byte(sb.String()), 0o644); err != nil {
            return fmt.Errorf("persist remediation findings: %w", err)
        }
    }

    beads, err := r.decompose(ctx, req)
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

**Step 1: Write the failing test**

```go
func TestRemediationRunner_Remediate_CallsDecomposeWithFindings(t *testing.T) {
    var capturedReq *stage.Request
    mockDecompose := &mockStage{
        runFn: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
            capturedReq = req
            return &stage.Result{
                Decision:  stage.DecisionProceed,
                Artifacts: &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "b1"}}},
            }, nil
        },
    }
    mockBeadRunner := &mockBeadRunner{}
    runner := NewRemediationRunner(RemediationRunnerConfig{
        AcceptStage:    &mockStage{},
        DecomposeStage: mockDecompose,
        BeadRunner:     mockBeadRunner,
        GenerationCap:  3,
    })

    findings := []stage.ReviewFinding{
        {Severity: "critical", Category: "bug", Scope: "spec", Description: "nil pointer"},
    }
    err := runner.Remediate(context.Background(), "my-spec", t.TempDir(), findings)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if capturedReq == nil {
        t.Fatal("decompose not called")
    }
    if !capturedReq.FindingsDecompose {
        t.Error("expected FindingsDecompose=true")
    }
    if len(capturedReq.Findings) != 1 {
        t.Errorf("expected 1 finding, got %d", len(capturedReq.Findings))
    }
    if !mockBeadRunner.ranBeads {
        t.Error("bead runner not called")
    }
}

func TestRemediationRunner_Remediate_RespectsGenerationCap(t *testing.T) {
    runner := NewRemediationRunner(RemediationRunnerConfig{
        AcceptStage:    &mockStage{},
        DecomposeStage: &mockStage{},
        BeadRunner:     &mockBeadRunner{},
        GenerationCap:  0, // cap at 0 means no remediation allowed
    })
    err := runner.Remediate(context.Background(), "my-spec", t.TempDir(), nil)
    if err == nil {
        t.Error("expected error when generation cap reached")
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd internal/v2/remediation && go test ./... -run TestRemediationRunner_Remediate -v
```
Expected: FAIL — `Remediate` method not defined

**Step 3: Implement**

Add `Remediate` method to `remediation.go`.

**Step 4: Run tests**

```bash
cd internal/v2/remediation && go test ./... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/remediation/remediation.go internal/v2/remediation/remediation_test.go
git commit -m "green: RemediationRunner.Remediate takes findings not gap analysis"
```

**Acceptance Criteria:**
- `RemediationRunner.Remediate(ctx, specID, worktree, findings) error` exists
- Sets `req.FindingsDecompose = true` and `req.Findings = findings` before calling decompose
- Respects generation cap (returns error when cap reached)
- Persists a findings record to `.gromit/v2/remediation-N.md`

---

### Task 5: Spec-level review stage

**Files:**
- Create: `internal/v2/stage/specreview/specreview.go`
- Create: `internal/v2/stage/specreview/specreview_test.go`

**What to Do:**

Create the specreview stage. It:
1. Reads `DiffFromBase` from the worktree git adapter
2. Reads `plan.md` from `<worktree>/.gromit/v2/plan.md`
3. Assembles prompt from base + project + fragment + instance (plan + diff)
4. Invokes the LLM (highest tier — uses `req.Provider` / `req.Model` as set by caller)
5. Parses JSON response into `SpecReviewArtifacts`
6. Derives verdict: any `critical` finding → "fail", else "pass"
7. For pass-with-improvements: creates `from-review` beads via TaskTracker (spec-scoped or general per `Scope`)

Key structs:

```go
package specreview

import (
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    "github.com/danabrams/gromit/internal/v2/llmtypes"
    "github.com/danabrams/gromit/internal/v2/trackertypes"
    "github.com/danabrams/gromit/internal/config"
    // ...
)

// GitDiffer provides the diff capability.
type GitDiffer interface {
    DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// Stage performs spec-level holistic review after all beads complete.
type Stage struct {
    name     string
    cfg      *config.Config
    git      GitDiffer
    llm      llmtypes.LLMProvider
    tracker  trackertypes.TaskTracker
    base     string
    project  string
    fragment string
}

var _ stagepkg.Stage = (*Stage)(nil)
```

JSON response from LLM:

```go
type reviewResponse struct {
    Verdict  string `json:"verdict"` // "pass" | "fail"
    Findings []struct {
        Severity      string   `json:"severity"`
        Category      string   `json:"category"`
        Scope         string   `json:"scope"`
        Description   string   `json:"description"`
        AffectedFiles []string `json:"affected_files"`
    } `json:"findings"`
}
```

Verdict derivation:

```go
func deriveVerdict(findings []stagepkg.ReviewFinding) string {
    for _, f := range findings {
        if f.Severity == "critical" {
            return "fail"
        }
    }
    return "pass"
}
```

Pass-with-improvements bead creation (called when verdict is "pass" but findings exist):

```go
func (s *Stage) createFromReviewBeads(ctx context.Context, specID string, findings []stagepkg.ReviewFinding) error {
    for _, f := range findings {
        if f.Severity == "critical" {
            continue // critical findings go to remediation, not from-review
        }
        labels := []string{"from-review"}
        if f.Scope == "spec" {
            labels = append(labels, fmt.Sprintf("spec:%s", specID))
        }
        _, err := s.tracker.CreateBead(ctx, trackertypes.TaskTrackerCreateBeadRequest{
            Title:       fmt.Sprintf("[%s] %s", f.Category, f.Description[:min(50, len(f.Description))]),
            Description: f.Description,
            Priority:    2, // P2 for review improvements
            Labels:      labels,
        })
        if err != nil {
            return fmt.Errorf("create from-review bead: %w", err)
        }
    }
    return nil
}
```

**Step 1: Write the failing tests**

In `specreview_test.go`:

```go
func TestSpecReview_CriticalFindingForcesFailVerdict(t *testing.T) {
    // LLM returns JSON with a critical finding
    // Expect Result.Decision == DecisionFail
    // Expect SpecReviewArtifacts.Verdict == "fail"
}

func TestSpecReview_OnlyWarningsForcesPassVerdict(t *testing.T) {
    // LLM returns JSON with only warning findings
    // Expect Result.Decision == DecisionProceed
    // Expect SpecReviewArtifacts.Verdict == "pass"
    // Expect from-review beads created in tracker
}

func TestSpecReview_PassWithNoFindings(t *testing.T) {
    // LLM returns {"verdict": "pass", "findings": []}
    // Expect DecisionProceed, no beads created
}

func TestSpecReview_ParsesStructuredOutput(t *testing.T) {
    // Verifies JSON parsing: severity, category, scope, description, affected_files
}
```

**Step 2: Run test to verify it fails**

```bash
cd internal/v2/stage/specreview && go test ./... -v
```
Expected: FAIL — package does not exist yet

**Step 3: Implement**

Create `specreview.go` with `Stage`, `New`, `Name`, `Run`, `deriveVerdict`, `createFromReviewBeads`.

**Step 4: Run tests**

```bash
cd internal/v2/stage/specreview && go test ./... -v
```
Expected: PASS

**Step 5: Compile check the full module**

```bash
go build ./...
```
Expected: no compile errors

**Step 6: Commit**

```bash
git add internal/v2/stage/specreview/
git commit -m "green: spec-level review stage with structured findings and from-review bead creation"
```

**Acceptance Criteria:**
- Stage implements `stagepkg.Stage` (compile-time check `var _ stagepkg.Stage = (*Stage)(nil)`)
- Any critical finding produces `DecisionFail` and `SpecReviewArtifacts.Verdict == "fail"`
- Only warning/suggestion findings produce `DecisionProceed` and `Verdict == "pass"`
- For pass-with-improvements: spec-scoped findings create beads with labels `["from-review", "spec:<id>"]`; general findings create beads with `["from-review"]`
- Critical findings do NOT produce from-review beads (they go to remediation)

---

### Task 6: review_spec_v2.md prompt fragment

**Files:**
- Create: `review_spec_v2.md` (project root)

**What to Do:**

Create the spec-level review prompt fragment at the project root. This file is loaded by `run2_components.go` (in Task 8) via `loadFragment`. It instructs the LLM to review the cumulative diff holistically.

```markdown
# Spec-Level Review Instructions

You are performing a holistic review of all code changes introduced by this spec. The cumulative diff and the implementation plan are provided in the instance context above.

This is distinct from the per-bead review that runs during implementation. Your role is to evaluate the complete output as a whole.

## Review Dimensions

### 1. Correctness
- Does the implementation satisfy the spec's intent beyond just passing tests?
- Are there logic errors, off-by-one errors, or incorrect assumptions?
- Are error conditions properly handled on all return paths?

### 2. Security (OWASP Top 10)
- SQL injection, command injection, XSS risks?
- Authentication/authorization bypass?
- Secrets logged or exposed?
- Insecure deserialization, path traversal, SSRF?

### 3. Error Handling
- Are errors wrapped with context (fmt.Errorf("...: %w", err))?
- Are nil returns guarded before dereferencing?
- Do error paths clean up resources they acquired?

### 4. Test Coverage Gaps
- Are critical paths untested?
- Are edge cases (empty input, nil, zero values, max values) covered?
- Do tests assert behavior or just absence of panics?

### 5. Code Quality
- Dead code, unused variables, or imports?
- Overly complex logic that should be simplified?
- Missing or misleading comments on non-obvious logic?

### 6. Architectural Fit
- Does the new code follow existing patterns in the project?
- Are abstractions introduced only when they provide clear value?
- Are there layering violations (e.g., infrastructure leaking into domain logic)?

## Severity Classification

- **critical**: Bug, security vulnerability, or logic error that will cause incorrect behavior.
- **warning**: Code quality issue, test gap, or architectural concern worth fixing soon.
- **suggestion**: Minor improvement; optional.

## Scope Classification

- **spec**: The finding is in code introduced or modified by this spec.
- **general**: The finding is in pre-existing code not changed by this spec.

## Output Format

Output ONLY a JSON object:
```json
{
  "verdict": "pass",
  "findings": [
    {
      "severity": "critical",
      "category": "bug",
      "scope": "spec",
      "description": "what is wrong and why",
      "affected_files": ["path/to/file.go"]
    }
  ]
}
```

Verdict rules:
- "fail": any finding with severity "critical"
- "pass": no critical findings (may have warnings or suggestions)

Do NOT output markdown, commentary, or anything other than the JSON object.
```

**Step 1: Create the file**

Create `review_spec_v2.md` at project root with the content above.

**Step 2: Verify it loads**

```bash
# From project root, verify the file exists and is non-empty
cat review_spec_v2.md | head -5
```

**Step 3: Commit**

```bash
git add review_spec_v2.md
git commit -m "feat: review_spec_v2.md prompt fragment for spec-level holistic review"
```

**Acceptance Criteria:**
- File exists at project root
- Contains JSON output format instructions with `verdict` + `findings` fields
- Severity/scope classification instructions are present

---

### Task 7: Spec loop orchestration

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`
- Modify: `internal/v2/loop/spec_loop_test.go` (or relevant test file)

**What to Do:**

**Change the `remediationRunner` interface** to use `Remediate` instead of `Run`:

```go
type remediationRunner interface {
    Remediate(ctx context.Context, specID, worktree string, findings []stagepkg.ReviewFinding) error
}
```

**Add a `specReviewStage` field and option to `SpecLoop`:**

```go
// In SpecLoop struct:
specReviewStage stagepkg.Stage

// New option:
func WithSpecReviewStage(stage stagepkg.Stage) SpecLoopOption {
    return func(s *SpecLoop) {
        s.specReviewStage = stage
    }
}
```

**Refactor `ensureAcceptance` to `ensureAcceptanceAndReview`** (keep the existing name for backward compat, or rename — rename is cleaner):

```go
func (s *SpecLoop) ensureAcceptance(ctx context.Context, req *stagepkg.Request, specID string) (*stagepkg.Result, error) {
    retriesRemaining := maxAcceptanceRetries
    for {
        if err := s.ctxErr(ctx); err != nil {
            return nil, err
        }

        // Run accept stage
        s.applyRouting(req, "accept")
        acceptRes, err := s.runAcceptStage(ctx, req)
        if err != nil {
            return acceptRes, err
        }

        // Run spec-level review stage (highest tier)
        var reviewRes *stagepkg.Result
        if s.specReviewStage != nil {
            s.applyRouting(req, "specreview")
            reviewRes, err = s.specReviewStage.Run(ctx, req)
            if err != nil {
                return reviewRes, err
            }
        }

        // Both must pass
        acceptPassed := !s.acceptFailed(acceptRes)
        reviewPassed := !s.reviewFailed(reviewRes)
        if acceptPassed && reviewPassed {
            return acceptRes, nil
        }

        // Collect combined findings
        findings := s.collectFindings(acceptRes, reviewRes)

        if s.remediationRunner == nil {
            return acceptRes, fmt.Errorf("accept or review failed")
        }
        if retriesRemaining <= 0 {
            return acceptRes, fmt.Errorf("%w: limit %d reached", ErrAcceptanceRetriesExceeded, maxAcceptanceRetries)
        }
        if err := s.remediationRunner.Remediate(ctx, specID, req.Worktree, findings); err != nil {
            return acceptRes, err
        }
        retriesRemaining--
    }
}

func (s *SpecLoop) reviewFailed(res *stagepkg.Result) bool {
    if res == nil {
        return false // no review stage = pass
    }
    return res.Decision == stagepkg.DecisionFail
}

func (s *SpecLoop) collectFindings(acceptRes, reviewRes *stagepkg.Result) []stagepkg.ReviewFinding {
    var findings []stagepkg.ReviewFinding

    // Extract from accept artifacts
    if acceptRes != nil && acceptRes.Artifacts != nil {
        if a, ok := acceptRes.Artifacts.(*stageaccept.AcceptArtifacts); ok {
            findings = append(findings, a.Findings...)
        }
    }

    // Extract from specreview artifacts
    if reviewRes != nil && reviewRes.Artifacts != nil {
        if a, ok := reviewRes.Artifacts.(*stagepkg.SpecReviewArtifacts); ok {
            for _, f := range a.Findings {
                if f.Severity == "critical" {
                    findings = append(findings, f)
                }
            }
        }
    }

    return findings
}
```

Also update `Run()` to record the specreview stage name:

```go
// After accept in StageSequence and Run():
s.recordStage("specreview")
```

Add `"specreview"` to `StageSequence`.

**Step 1: Write failing tests**

```go
func TestSpecLoop_SpecReviewRunsAfterAccept(t *testing.T) {
    // Wire spec loop with a mock accept (pass) and mock specreview (fail)
    // Verify specreview is invoked after accept
    // Verify remediationRunner.Remediate is called with specreview findings
}

func TestSpecLoop_CombinesAcceptAndReviewFindings(t *testing.T) {
    // Accept fails with 1 finding, specreview fails with 1 critical finding
    // Verify Remediate receives 2 findings
}

func TestSpecLoop_SkipsSpecReviewWhenNotConfigured(t *testing.T) {
    // No specReviewStage configured
    // Accept passes → succeeds without error
}
```

**Step 2: Run test to verify it fails**

```bash
cd internal/v2/loop && go test ./... -run TestSpecLoop_SpecReview -v
```
Expected: FAIL

**Step 3: Implement**

Apply changes to `spec_loop.go` as described.

**Step 4: Run all loop tests**

```bash
cd internal/v2/loop && go test ./... -v
```
Expected: PASS

**Step 5: Verify full module compiles**

```bash
go build ./...
```

**Step 6: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/loop/spec_loop_test.go
git commit -m "green: spec loop runs specreview after accept, combines findings for Remediate"
```

**Acceptance Criteria:**
- `SpecLoop` has `specReviewStage` field and `WithSpecReviewStage` option
- `remediationRunner` interface uses `Remediate(ctx, specID, worktree, findings) error`
- Specreview runs after accept on every iteration
- When both accept and specreview pass, loop exits successfully
- Findings from both stages are combined and passed to `Remediate`
- `"specreview"` appears in `StageSequence`

---

### Task 8: Wire specreview in run2_components

**Files:**
- Modify: `internal/v2/loop/run2_components.go`

**What to Do:**

Load `review_spec_v2.md` fragment, create the specreview stage, set highest tier for it, and expose it via `Run2LoopComponents`.

In `NewRun2LoopComponents`, after loading `acceptFragment`:

```go
specReviewFragment, err := loadFragment(cfg.ProjectRoot, "review_spec_v2.md")
if err != nil {
    cleanup()
    return nil, err
}
```

Create the stage:

```go
import specreviewstage "github.com/danabrams/gromit/internal/v2/stage/specreview"

specReviewStage, err := specreviewstage.New(cfg, adapters.Git, adapters.LLM, adapters.TaskTracker, baseInstructions, projectContext, specReviewFragment)
if err != nil {
    cleanup()
    return nil, err
}
```

Add to `Run2LoopComponents`:

```go
type Run2LoopComponents struct {
    // existing fields ...
    SpecReviewStage stagepkg.Stage
}
```

And return it:

```go
return &Run2LoopComponents{
    // existing fields ...
    SpecReviewStage: specReviewStage,
}, nil
```

In `run2.go`, wire `WithSpecReviewStage`:

```go
loopInstance, err := newSpecLoopFn(adapters, cfg, gate,
    // ... existing options ...
    loop.WithSpecReviewStage(components.SpecReviewStage),
)
```

Also update the `phaseModels` / routing to route specreview as "high" tier. Add to routing: `TierForPhase("specreview", ...)` should return `TierHigh`. This can be done by adding `"specreview": "high"` to the phase models map in config or by hardcoding it in `run2_components.go`:

```go
// In NewRun2LoopComponents, after building phaseModels:
// Ensure specreview always uses the high tier (highest-quality model).
if phaseModels == nil {
    phaseModels = make(map[string]string)
}
// Don't overwrite if explicitly configured
if _, ok := phaseModels["specreview"]; !ok {
    phaseModels["specreview"] = routing.TierHigh
}
```

**Step 1: Run existing tests to establish baseline**

```bash
cd internal/v2/loop && go test ./... -v
```
Expected: PASS (all existing tests)

**Step 2: Implement the wiring**

Apply changes to `run2_components.go`.

**Step 3: Run all tests**

```bash
go test ./... -count=1
```
Expected: PASS

**Step 4: Verify the build**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add internal/v2/loop/run2_components.go
git commit -m "green: wire specreview stage into run2 components with high-tier routing"
```

**Acceptance Criteria:**
- `Run2LoopComponents.SpecReviewStage` field exists and is populated
- `review_spec_v2.md` fragment is loaded and passed to the specreview stage
- Specreview stage is wired to `WithSpecReviewStage` on the spec loop
- Routing routes "specreview" phase to `TierHigh`

---

### Task 9: --from-review flag in run2.go

**Files:**
- Modify: `cmd/gromit/run2.go`
- Create: `cmd/gromit/run2_from_review_test.go`

**What to Do:**

Add `--from-review` and `--spec` flags. When `--from-review` is set, skip normal spec resolution, query open beads with `from-review` label (optionally filtered by `spec:<specID>`), and run the bead loop directly.

In `init()`:

```go
func init() {
    run2Cmd.Flags().String("epic", "", "Run specs scoped to the specified epic")
    run2Cmd.Flags().Bool("from-review", false, "Run only beads with the from-review label")
    run2Cmd.Flags().String("spec", "", "Scope --from-review to a specific spec ID")
}
```

In `run2()`, check the flag early:

```go
fromReview, _ := cmd.Flags().GetBool("from-review")
if fromReview {
    specScope, _ := cmd.Flags().GetString("spec")
    return runFromReview(cmd.Context(), cfg, specScope, components)
}
```

Add `runFromReview` function:

```go
func runFromReview(ctx context.Context, cfg *config.Config, specScope string, components *loop.Run2LoopComponents) error {
    // Build query labels
    labels := []string{"from-review"}
    if specScope != "" {
        labels = append(labels, fmt.Sprintf("spec:%s", specScope))
    }

    // Query open from-review beads
    resp, err := taskTrackerAdapter.QueryBeads(ctx, trackertypes.TaskTrackerQueryBeadsRequest{
        Labels:   labels,
        StatusIn: []string{"open"},
    })
    if err != nil {
        return fmt.Errorf("query from-review beads: %w", err)
    }
    if resp == nil || len(resp.Beads) == 0 {
        fmt.Println("No open from-review beads found.")
        return nil
    }

    beads := make([]*bead.Bead, len(resp.Beads))
    for i, b := range resp.Beads {
        beads[i] = &bead.Bead{ID: b.ID, Title: b.Title, Description: b.Description, Priority: b.Priority, Labels: b.Labels}
    }

    // Run bead loop directly — no plan, no decompose, no accept, no review
    stopCh := make(chan struct{})
    _, err = components.BeadLoop.Run(ctx, beads, stopCh)
    return err
}
```

Note: the function needs access to `taskTrackerAdapter`. Refactor to pass it in or build it inside the function using the same adapter construction logic from `run2()`. Use the existing pattern from `run2()` to construct the task tracker adapter.

**Step 1: Write the failing test**

```go
func TestRun2FromReviewFlag_QueriesFromReviewBeads(t *testing.T) {
    // Set --from-review flag
    // Verify task tracker queried with label "from-review"
    // Verify bead loop runs with the returned beads
    // Verify no accept/review/decompose stages called
}

func TestRun2FromReviewFlag_WithSpec_ScopesLabel(t *testing.T) {
    // Set --from-review --spec my-spec
    // Verify task tracker queried with labels ["from-review", "spec:my-spec"]
}

func TestRun2FromReviewFlag_NoBeads_PrintsMessage(t *testing.T) {
    // Task tracker returns empty response
    // Verify function returns nil (no error) and prints "No open from-review beads found."
}
```

**Step 2: Run test to verify it fails**

```bash
cd cmd/gromit && go test ./... -run TestRun2FromReview -v
```
Expected: FAIL

**Step 3: Implement**

Add flags and `runFromReview` function.

**Step 4: Run tests**

```bash
cd cmd/gromit && go test ./... -v
```
Expected: PASS

**Step 5: Verify full build**

```bash
go build ./...
go test ./... -count=1
```

**Step 6: Commit**

```bash
git add cmd/gromit/run2.go cmd/gromit/run2_from_review_test.go
git commit -m "green: --from-review flag runs only from-review beads without plan/accept/review cycle"
```

**Acceptance Criteria:**
- `run2 --from-review` queries beads with label `from-review` and runs bead loop
- `run2 --from-review --spec <id>` additionally filters by `spec:<id>` label
- When no beads found, prints message and returns nil
- From-review run does NOT invoke plan, decompose, accept, or spec-level review stages
- Existing `run2` behavior unchanged when `--from-review` is not set

---

## Integration Test: Full Post-Bead-Loop Pipeline

After all tasks are complete, verify end-to-end with an integration test:

**File:** `internal/v2/loop/spec_loop_integration_test.go`

```go
func TestIntegration_AcceptPassesReviewFails_RemediatesWithFindings(t *testing.T) {
    // Wire spec loop with:
    //   - accept stage that passes
    //   - specreview stage that returns one critical finding
    //   - remediation runner that captures received findings
    // Verify: Remediate called with the critical finding from review
}

func TestIntegration_BothPassWithImprovements_SpecProceedsFromReviewBeadsCreated(t *testing.T) {
    // Wire spec loop with:
    //   - accept stage that passes
    //   - specreview stage that returns "pass" verdict with one warning finding (scope: spec)
    // Verify: Result is success (no remediation)
    // Verify: from-review bead created in tracker with labels ["from-review", "spec:<id>"]
}
```

```bash
go test ./internal/v2/loop/... -run TestIntegration -v
git add internal/v2/loop/spec_loop_integration_test.go
git commit -m "green: integration tests for full post-bead-loop pipeline"
```
