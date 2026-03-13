# Spec-Level Review and Targeted Remediation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a spec-level holistic code review after the bead loop, produce structured findings from both accept and review, and use those findings to drive targeted remediation decomposition instead of re-decomposing the full plan.

**Architecture:** A new `specreview` stage evaluates the cumulative diff using the highest-tier model and emits structured `Finding` values (severity/category/scope). Accept is extended to emit findings for each failed criterion. Remediation receives a `[]Finding` slice and uses a new `findingsDecomposePromptTemplate` to create targeted beads. A `--from-review` flag on `run2` runs only `from-review`-labeled beads.

**Tech Stack:** Go 1.24+, `internal/v2/stage` interface, `internal/jsonutil` for JSON parsing, `internal/v2/routing` for tier selection, `spf13/cobra` for CLI flags.

---

## Architecture

### New `Finding` type

Defined in `internal/v2/stage/stage.go` — shared across accept, specreview, remediation, and decompose:

```go
type Finding struct {
    Severity     string   `json:"severity"`      // "critical" | "warning" | "suggestion"
    Category     string   `json:"category"`      // "bug" | "security" | "quality" | "test-gap" | "architecture" | "acceptance"
    Scope        string   `json:"scope"`         // "spec" | "general"
    Description  string   `json:"description"`
    AffectedFiles []string `json:"affected_files"`
}
```

### `StageRequest` gains `Findings []Finding`

Used by remediation → decompose to pass targeted findings as decompose input.

### Accept emits `Findings` in `AcceptArtifacts`

Each unmet criterion becomes a `Finding{Severity:"critical", Category:"acceptance", Scope:"spec"}`.

### New `internal/v2/stage/specreview/` package

Implements `stage.Stage`. Receives cumulative diff + plan + project context. Returns `SpecReviewArtifacts{Verdict, Findings}`. Verdict logic: any critical → fail; only warning/suggestion → pass.

### `spec_loop.go` changes

After bead loop: run accept → run spec-level review → if either fails, combine findings → call `remediationRunner.RunWithFindings(ctx, specID, worktree, findings)`. If review passes with findings: create from-review beads (spec-scoped findings get `from-review` + `spec:<id>` labels; general findings get `from-review` only).

### `remediationRunner` interface change

```go
// Old
type remediationRunner interface {
    Run(ctx context.Context, specID, worktree string) error
}
// New
type remediationRunner interface {
    RunWithFindings(ctx context.Context, specID, worktree string, findings []stagepkg.Finding) error
}
```

The `RemediationRunner` now: findings → decompose targeted beads → bead loop → accept + spec-level review → if either fails, new combined findings → repeat (up to generation cap).

### `decompose.go` gains findings template

When `req.Findings != nil && len(req.Findings) > 0`, uses `findingsDecomposePromptTemplate` instead of gap analysis template. Falls back to gap analysis for nil/empty findings.

### `run2.go` adds `--from-review` flag

When set: skip plan/decompose/accept/review. Query `from-review`-labeled beads (optionally scoped to `spec:<id>` via `--spec`), run through bead loop, no remediation cycle.

### `run2_components.go` wires spec-level review

Loads `review_spec_v2.md` fragment. Creates `specreview.Stage` with `req.Tier = routing.TierHigh`. Passes to spec_loop via `WithSpecReviewStage`.

---

## Test Strategy

- Unit: `Finding` type presence + `StageRequest.Findings` field — compile-time via interface assertions
- Unit: specreview stage — JSON parsing, verdict logic (critical→fail, warning→pass), nil findings safe
- Unit: accept stage — structured findings emitted for each failed criterion
- Unit: decompose stage — findings template used when `req.Findings` set, not when nil
- Unit: remediation runner — `RunWithFindings` passes findings to decompose, generation cap enforced
- Unit: spec_loop — `WithSpecReviewStage` option, from-review bead creation, combined findings path
- Integration: full post-bead-loop pipeline with findings-based remediation
- Integration: pass-with-improvements creates from-review beads and spec proceeds

---

## Implementation Tasks

---

### Task 1: Add `Finding` type and `Findings` field to stage types

**Files:**
- Modify: `internal/v2/stage/stage.go`
- Modify: `internal/v2/stage/accept/accept.go`

**Step 1: Write a failing test for `Finding` type in stage package**

In a new file `internal/v2/stage/stage_finding_test.go`:

```go
package stage_test

import (
    "testing"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestFindingTypeExists(t *testing.T) {
    f := stagepkg.Finding{
        Severity:      "critical",
        Category:      "bug",
        Scope:         "spec",
        Description:   "missing nil check",
        AffectedFiles: []string{"internal/foo/foo.go"},
    }
    if f.Severity != "critical" {
        t.Errorf("Severity = %q, want %q", f.Severity, "critical")
    }
    if len(f.AffectedFiles) != 1 {
        t.Errorf("AffectedFiles len = %d, want 1", len(f.AffectedFiles))
    }
}

func TestStageRequestHasFindingsField(t *testing.T) {
    req := stagepkg.StageRequest{}
    req.Findings = []stagepkg.Finding{{Severity: "warning", Category: "quality", Scope: "general", Description: "test"}}
    if len(req.Findings) != 1 {
        t.Errorf("Findings len = %d, want 1", len(req.Findings))
    }
}

func TestAcceptArtifactsHasFindingsField(t *testing.T) {
    // This test ensures AcceptArtifacts.Findings is present.
    // Import the accept package to access AcceptArtifacts.
    _ = stagepkg.Finding{} // type compiles
}
```

**Step 2: Run to confirm failure**

```
cd /path/to/gromit && go test ./internal/v2/stage/... -run TestFinding -v
```

Expected: FAIL with "undefined: stagepkg.Finding"

**Step 3: Add `Finding` type and `Findings` field to `stage.go`**

In `internal/v2/stage/stage.go`, add after the `LLMCostSummary` struct:

```go
// Finding represents a single issue identified by code review or acceptance evaluation.
type Finding struct {
    Severity      string   `json:"severity"`       // "critical" | "warning" | "suggestion"
    Category      string   `json:"category"`       // "bug" | "security" | "quality" | "test-gap" | "architecture" | "acceptance"
    Scope         string   `json:"scope"`          // "spec" | "general"
    Description   string   `json:"description"`
    AffectedFiles []string `json:"affected_files"`
}
```

In `StageRequest`, add `Findings []Finding` after the `GapAnalysis string` field:

```go
GapAnalysis  string
Findings     []Finding
```

**Step 4: Add `Findings` to `AcceptArtifacts`**

In `internal/v2/stage/accept/accept.go`, update `AcceptArtifacts`:

```go
type AcceptArtifacts struct {
    Results    []presentation.AcceptanceResult
    GapSummary string
    Findings   []stagepkg.Finding
}
```

**Step 5: Run tests to confirm they pass**

```
cd /path/to/gromit && go test ./internal/v2/stage/... -run TestFinding -v
```

Expected: PASS

```
go build ./...
```

Expected: no errors (Findings is a new zero-value field, no callers break)

**Step 6: Commit**

```bash
git add internal/v2/stage/stage.go internal/v2/stage/accept/accept.go internal/v2/stage/stage_finding_test.go
git commit -m "feat: add Finding type and Findings field to stage types"
```

---

### Task 2: Emit structured findings from accept stage for failed criteria

**Files:**
- Modify: `internal/v2/stage/accept/accept.go`
- Modify: `internal/v2/stage/accept/accept_test.go` (add test for findings)

**Step 1: Write the failing test**

In `accept_test.go`, add a test (find the test file first to understand existing patterns):

```go
func TestAcceptStage_EmitsStructuredFindings_OnCriterionFailure(t *testing.T) {
    // Setup: two criteria, one fails
    // Verify: AcceptArtifacts.Findings has one Finding with severity="critical", category="acceptance", scope="spec"
    // This test should fail because Findings is not yet populated.
    cfg := &config.Config{}
    cfg.Paths.Specs = t.TempDir()
    // write a spec with one acceptance criterion
    specContent := "## Acceptance Criteria\n- The system does X"
    specPath := filepath.Join(cfg.Paths.Specs, "my-spec.md")
    os.WriteFile(specPath, []byte(specContent), 0o644)

    mockGit := &mockGitDiffer{diff: "some changes"}
    mockLLM := &mockLLMProvider{
        responses: []llmtypes.LLMInvokeResponse{
            {Success: true, Output: `{"pass": false, "summary": "criterion not met"}`},
        },
    }
    stage, _ := accept.New(cfg, mockGit, mockLLM, "", "", "")
    req := &stagepkg.StageRequest{
        Bead:     stagepkg.BeadInfo{ID: "my-spec"},
        Worktree: t.TempDir(),
    }

    res, err := stage.Run(context.Background(), req)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    arts, ok := res.Artifacts.(*accept.AcceptArtifacts)
    if !ok {
        t.Fatal("expected *AcceptArtifacts")
    }
    if len(arts.Findings) != 1 {
        t.Fatalf("Findings len = %d, want 1", len(arts.Findings))
    }
    f := arts.Findings[0]
    if f.Severity != "critical" {
        t.Errorf("Severity = %q, want %q", f.Severity, "critical")
    }
    if f.Category != "acceptance" {
        t.Errorf("Category = %q, want %q", f.Category, "acceptance")
    }
    if f.Scope != "spec" {
        t.Errorf("Scope = %q, want %q", f.Scope, "spec")
    }
}
```

**Step 2: Run to confirm failure**

```
go test ./internal/v2/stage/accept/... -run TestAcceptStage_EmitsStructuredFindings -v
```

Expected: FAIL — `Findings len = 0, want 1`

**Step 3: Implement findings emission in `accept.go`**

In the `Run` method, find the block that builds `failures` and constructs `AcceptArtifacts`. After building `failures`, also build `findings`:

```go
// After the for loop over criteria:
artifacts := &AcceptArtifacts{Results: results}
if len(failures) > 0 {
    gapSummary := strings.Join(failures, "\n")
    artifacts.GapSummary = gapSummary
    artifacts.Findings = buildAcceptFindings(failures, specID)
    // ... existing writeGapAnalysis call ...
}
```

Add helper `buildAcceptFindings`:

```go
func buildAcceptFindings(failures []string, specID string) []stagepkg.Finding {
    findings := make([]stagepkg.Finding, 0, len(failures))
    for _, f := range failures {
        findings = append(findings, stagepkg.Finding{
            Severity:    "critical",
            Category:    "acceptance",
            Scope:       "spec",
            Description: f,
        })
    }
    return findings
}
```

**Step 4: Run tests**

```
go test ./internal/v2/stage/accept/... -v
```

Expected: all pass including new test

**Step 5: Commit**

```bash
git add internal/v2/stage/accept/accept.go internal/v2/stage/accept/accept_test.go
git commit -m "feat: emit structured Finding slice from accept stage for failed criteria"
```

---

### Task 3: Create `review_spec_v2.md` prompt fragment

**Files:**
- Create: `review_spec_v2.md` (project root, at `cfg.ProjectRoot`)

**Step 1: Write the fragment file**

```bash
cat > /path/to/gromit/review_spec_v2.md << 'EOF'
# Spec-Level Code Review Instructions

You are performing a holistic code review of the cumulative diff for an entire spec implementation. This is distinct from the per-bead review — you evaluate the complete output as a whole.

## Context Provided

- **Cumulative diff**: All changes made for this spec from the base branch
- **Original plan**: The intent and architecture the implementation was targeting
- **Project context**: CLAUDE.md and rules

## Review Dimensions

### 1. Correctness
- Does the implementation work end-to-end, not just in isolation?
- Are there gaps between components (e.g., stage wired but prompt layer empty)?
- Are error paths handled consistently across the diff?

### 2. Security (OWASP top 10)
- Command injection, SQL injection, XSS?
- Authentication/authorization bypass?
- Secrets logged or exposed?

### 3. Error Handling
- Are all error returns from new functions checked at call sites?
- Are there silent failures that swallow errors?

### 4. Test Coverage Gaps
- Are there new code paths that have no test coverage?
- Are tests behavioral (asserting outcomes) vs structural (asserting file names)?

### 5. Code Quality
- Dead code or unused variables/imports?
- Overly complex logic with simpler alternatives?
- Naming inconsistencies with the existing codebase?

### 6. Architectural Fit
- Do new packages/types follow established patterns?
- Are abstractions introduced only where they serve multiple callers?
- Are nil-safety conventions followed (NormalizeNilFields where needed)?

## Verdict Rules

- `"verdict": "fail"` — if ANY finding has `"severity": "critical"`
- `"verdict": "pass"` — if findings are only `"warning"` or `"suggestion"` (or none)

## Scope Classification

- `"scope": "spec"` — the issue is in code changed by this spec
- `"scope": "general"` — the issue exists in pre-existing code unrelated to this spec

## Output Format

Output ONLY a JSON object. No markdown wrapper, no explanation.

```json
{
  "verdict": "pass",
  "findings": [
    {
      "severity": "critical",
      "category": "bug",
      "scope": "spec",
      "description": "Stage.Run does not check for nil req before dereferencing",
      "affected_files": ["internal/v2/stage/specreview/specreview.go"]
    }
  ]
}
```

`findings` is an empty array when there are no issues.
EOF
```

**Step 2: Verify the file exists**

```bash
ls -la /path/to/gromit/review_spec_v2.md
```

Expected: file present

**Step 3: Commit**

```bash
git add review_spec_v2.md
git commit -m "feat: add review_spec_v2.md prompt fragment for spec-level review"
```

---

### Task 4: Create the `specreview` stage

**Files:**
- Create: `internal/v2/stage/specreview/specreview.go`
- Create: `internal/v2/stage/specreview/specreview_test.go`

**Step 1: Write failing tests first**

Create `internal/v2/stage/specreview/specreview_test.go`:

```go
package specreview_test

import (
    "context"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/v2/llmtypes"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    "github.com/danabrams/gromit/internal/v2/stage/specreview"
)

// mockLLMProvider for tests.
type mockLLMProvider struct {
    response llmtypes.LLMInvokeResponse
    err      error
}

func (m *mockLLMProvider) Invoke(_ context.Context, _ llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
    return &m.response, m.err
}

type mockGitDiffer struct {
    diff string
    err  error
}

func (m *mockGitDiffer) DiffFromBase(_ context.Context, _ string) (string, error) {
    return m.diff, m.err
}

func TestSpecReview_CriticalFindingForcesFailVerdict(t *testing.T) {
    cfg := &config.Config{}
    llmResp := llmtypes.LLMInvokeResponse{
        Success: true,
        Output: `{"verdict": "fail", "findings": [{"severity": "critical", "category": "bug", "scope": "spec", "description": "nil dereference", "affected_files": ["foo.go"]}]}`,
    }
    stage, err := specreview.New(cfg, &mockGitDiffer{diff: "some diff"}, &mockLLMProvider{response: llmResp}, "", "", "")
    if err != nil {
        t.Fatalf("New: %v", err)
    }
    req := &stagepkg.StageRequest{
        Bead:     stagepkg.BeadInfo{ID: "my-spec"},
        Worktree: t.TempDir(),
    }
    res, err := stage.Run(context.Background(), req)
    if err != nil {
        t.Fatalf("Run: %v", err)
    }
    if res.Decision != stagepkg.DecisionFail {
        t.Errorf("Decision = %v, want DecisionFail", res.Decision)
    }
    arts, ok := res.Artifacts.(*specreview.SpecReviewArtifacts)
    if !ok {
        t.Fatal("expected *SpecReviewArtifacts")
    }
    if arts.Verdict != "fail" {
        t.Errorf("Verdict = %q, want fail", arts.Verdict)
    }
    if len(arts.Findings) != 1 {
        t.Fatalf("Findings len = %d, want 1", len(arts.Findings))
    }
    if arts.Findings[0].Severity != "critical" {
        t.Errorf("Findings[0].Severity = %q, want critical", arts.Findings[0].Severity)
    }
}

func TestSpecReview_WarningOnlyProducesPassVerdict(t *testing.T) {
    cfg := &config.Config{}
    llmResp := llmtypes.LLMInvokeResponse{
        Success: true,
        Output: `{"verdict": "pass", "findings": [{"severity": "warning", "category": "quality", "scope": "general", "description": "naming inconsistency", "affected_files": []}]}`,
    }
    stage, err := specreview.New(cfg, &mockGitDiffer{diff: "diff"}, &mockLLMProvider{response: llmResp}, "", "", "")
    if err != nil {
        t.Fatalf("New: %v", err)
    }
    req := &stagepkg.StageRequest{
        Bead:     stagepkg.BeadInfo{ID: "spec-id"},
        Worktree: t.TempDir(),
    }
    res, err := stage.Run(context.Background(), req)
    if err != nil {
        t.Fatalf("Run: %v", err)
    }
    if res.Decision != stagepkg.DecisionProceed {
        t.Errorf("Decision = %v, want DecisionProceed", res.Decision)
    }
    arts := res.Artifacts.(*specreview.SpecReviewArtifacts)
    if arts.Verdict != "pass" {
        t.Errorf("Verdict = %q, want pass", arts.Verdict)
    }
    if len(arts.Findings) != 1 {
        t.Errorf("Findings len = %d, want 1", len(arts.Findings))
    }
}

func TestSpecReview_NilConfigReturnsError(t *testing.T) {
    _, err := specreview.New(nil, &mockGitDiffer{}, &mockLLMProvider{}, "", "", "")
    if err == nil {
        t.Fatal("expected error for nil config")
    }
}

func TestSpecReview_EmptyFindingsProducesPassVerdict(t *testing.T) {
    cfg := &config.Config{}
    llmResp := llmtypes.LLMInvokeResponse{
        Success: true,
        Output:  `{"verdict": "pass", "findings": []}`,
    }
    stage, _ := specreview.New(cfg, &mockGitDiffer{diff: "diff"}, &mockLLMProvider{response: llmResp}, "", "", "")
    req := &stagepkg.StageRequest{Bead: stagepkg.BeadInfo{ID: "s"}, Worktree: t.TempDir()}
    res, err := stage.Run(context.Background(), req)
    if err != nil {
        t.Fatalf("Run: %v", err)
    }
    if res.Decision != stagepkg.DecisionProceed {
        t.Errorf("Decision = %v, want DecisionProceed", res.Decision)
    }
}

func TestSpecReview_VerdictOverriddenByCriticalFinding(t *testing.T) {
    // Even if LLM says "pass", a critical finding must flip to fail.
    cfg := &config.Config{}
    llmResp := llmtypes.LLMInvokeResponse{
        Success: true,
        Output:  `{"verdict": "pass", "findings": [{"severity": "critical", "category": "security", "scope": "spec", "description": "XSS", "affected_files": []}]}`,
    }
    stage, _ := specreview.New(cfg, &mockGitDiffer{diff: "d"}, &mockLLMProvider{response: llmResp}, "", "", "")
    req := &stagepkg.StageRequest{Bead: stagepkg.BeadInfo{ID: "s"}, Worktree: t.TempDir()}
    res, _ := stage.Run(context.Background(), req)
    if res.Decision != stagepkg.DecisionFail {
        t.Errorf("Decision = %v, want DecisionFail (critical finding overrides pass verdict)", res.Decision)
    }
}
```

**Step 2: Run to confirm failure**

```
go test ./internal/v2/stage/specreview/... -v
```

Expected: FAIL — package does not exist yet

**Step 3: Implement `specreview.go`**

Create `internal/v2/stage/specreview/specreview.go`:

```go
package specreview

import (
    "context"
    "fmt"
    "strings"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/jsonutil"
    "github.com/danabrams/gromit/internal/v2/llmtypes"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

const defaultSpecReviewFragment = `# Spec-Level Code Review

You are reviewing the cumulative diff for a spec. Output ONLY a JSON object:
{"verdict": "pass|fail", "findings": [{"severity": "critical|warning|suggestion", "category": "bug|security|quality|test-gap|architecture", "scope": "spec|general", "description": "...", "affected_files": [...]}]}

verdict="fail" if any finding has severity="critical". verdict="pass" otherwise.
`

// GitDiffer provides the cumulative diff capability needed by this stage.
type GitDiffer interface {
    DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// SpecReviewArtifacts captures the structured output of the spec-level review.
type SpecReviewArtifacts struct {
    Verdict  string           // "pass" | "fail"
    Findings []stagepkg.Finding
}

// Stage executes the spec-level holistic code review.
type Stage struct {
    name     string
    cfg      *config.Config
    git      GitDiffer
    llm      llmtypes.LLMProvider
    base     string
    project  string
    fragment string
}

// New constructs a specreview stage.
func New(cfg *config.Config, git GitDiffer, provider llmtypes.LLMProvider, base, project, fragment string) (*Stage, error) {
    if cfg == nil {
        return nil, fmt.Errorf("config required")
    }
    if git == nil {
        return nil, fmt.Errorf("git adapter required")
    }
    if provider == nil {
        return nil, fmt.Errorf("llm provider required")
    }
    if strings.TrimSpace(fragment) == "" {
        fragment = defaultSpecReviewFragment
    }
    return &Stage{
        name:     stagedesc.Describe("spec-review", cfg),
        cfg:      cfg,
        git:      git,
        llm:      provider,
        base:     base,
        project:  project,
        fragment: fragment,
    }, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the stage identifier.
func (s *Stage) Name() string {
    if s == nil {
        return ""
    }
    return s.name
}

// Run invokes the spec-level review and emits structured findings.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
    if req == nil {
        return nil, fmt.Errorf("request required")
    }
    specID := strings.TrimSpace(req.Bead.ID)
    if specID == "" {
        return nil, fmt.Errorf("spec ID required")
    }

    root := s.resolveRoot(req)
    diff, err := s.git.DiffFromBase(ctx, root)
    if err != nil {
        return nil, fmt.Errorf("spec-review: git diff: %w", err)
    }

    promptText := s.buildPrompt(specID, diff, req)
    provider := s.llm
    if req.Provider != nil {
        provider = req.Provider
    }
    model := strings.TrimSpace(req.Model)
    if model == "" {
        model = s.defaultModel()
    }

    resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: promptText, Model: model, Dir: root})
    if err != nil {
        return nil, fmt.Errorf("spec-review: invoke: %w", err)
    }
    if resp == nil {
        return nil, fmt.Errorf("spec-review: nil response")
    }
    if !resp.Success {
        return nil, fmt.Errorf("spec-review: provider failed: %s", resp.Output)
    }

    artifacts, err := parseReviewOutput(resp.Output)
    if err != nil {
        return nil, fmt.Errorf("spec-review: parse: %w", err)
    }

    // Enforce: any critical finding forces fail verdict regardless of what LLM said.
    for _, f := range artifacts.Findings {
        if f.Severity == "critical" {
            artifacts.Verdict = "fail"
            break
        }
    }

    decision := stagepkg.DecisionProceed
    if artifacts.Verdict == "fail" {
        decision = stagepkg.DecisionFail
    }

    return &stagepkg.Result{Decision: decision, Artifacts: artifacts}, nil
}

func parseReviewOutput(output string) (*SpecReviewArtifacts, error) {
    trimmed := strings.TrimSpace(output)
    if trimmed == "" {
        return nil, fmt.Errorf("empty output")
    }
    var raw struct {
        Verdict  string           `json:"verdict"`
        Findings []stagepkg.Finding `json:"findings"`
    }
    if err := jsonutil.ExtractObject(trimmed, &raw); err != nil {
        return nil, fmt.Errorf("parse review output: %w", err)
    }
    if raw.Findings == nil {
        raw.Findings = []stagepkg.Finding{}
    }
    return &SpecReviewArtifacts{Verdict: raw.Verdict, Findings: raw.Findings}, nil
}

func (s *Stage) buildPrompt(specID, diff string, req *stagepkg.Request) string {
    diffText := strings.TrimSpace(diff)
    if diffText == "" {
        diffText = "(no diff)"
    }
    instance := fmt.Sprintf("Spec: %s\n\n## Cumulative Diff\n%s", specID, diffText)
    parts := []string{}
    if s.base != "" {
        parts = append(parts, s.base)
    }
    if s.project != "" {
        parts = append(parts, s.project)
    }
    parts = append(parts, instance, s.fragment)
    return strings.Join(parts, "\n\n")
}

func (s *Stage) resolveRoot(req *stagepkg.Request) string {
    if req != nil {
        if t := strings.TrimSpace(req.Worktree); t != "" {
            return t
        }
    }
    if s.cfg != nil && strings.TrimSpace(s.cfg.ProjectRoot) != "" {
        return s.cfg.ProjectRoot
    }
    return "."
}

func (s *Stage) defaultModel() string {
    if s.cfg != nil && strings.TrimSpace(s.cfg.Models.P0) != "" {
        return s.cfg.Models.P0
    }
    return config.ModelOpus
}
```

**Step 4: Run tests**

```
go test ./internal/v2/stage/specreview/... -v
```

Expected: all pass

**Step 5: Commit**

```bash
git add internal/v2/stage/specreview/
git commit -m "feat: implement specreview stage with verdict logic and structured findings"
```

---

### Task 5: Add findings-based decompose template

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go`
- Modify: `internal/v2/stage/decompose/decompose_test.go`

**Step 1: Write the failing test**

Add to `decompose_test.go` a test verifying that when `req.Findings` is non-empty, the findings template is used:

```go
func TestDecompose_UsesFindingsTemplateWhenFindingsPresent(t *testing.T) {
    // Setup: a decompose stage where the LLM captures the prompt.
    // When req.Findings is non-empty, prompt must contain "Targeted findings" not "Unmet Acceptance Criteria".
    // This test will fail until findingsDecomposePromptTemplate is added.
    var capturedPrompt string
    mockLLM := &mockCapturingLLM{
        capture: &capturedPrompt,
        resp:    `[{"title":"fix nil check","description":"...","priority":"P1","acceptance_criteria":["nil safe"],"depends_on_index":[]}]`,
    }
    cfg := minimalConfig(t)
    tracker := &mockTracker{}
    stage, _ := decompose.New(cfg, mockLLM, tracker)

    req := &stagepkg.StageRequest{
        Bead:     stagepkg.BeadInfo{ID: "my-spec"},
        Worktree: setupPlanFile(t, cfg, "my-spec"),
        Config:   cfg,
        Findings: []stagepkg.Finding{
            {Severity: "critical", Category: "bug", Scope: "spec", Description: "nil panic in Run"},
        },
    }
    _, err := stage.Run(context.Background(), req)
    if err != nil {
        t.Fatalf("Run: %v", err)
    }
    if !strings.Contains(capturedPrompt, "Targeted Findings") {
        t.Errorf("expected prompt to contain 'Targeted Findings', got:\n%s", capturedPrompt[:min(200, len(capturedPrompt))])
    }
    if strings.Contains(capturedPrompt, "Unmet Acceptance Criteria") {
        t.Errorf("prompt should not contain 'Unmet Acceptance Criteria' when Findings is set")
    }
}
```

**Step 2: Run to confirm failure**

```
go test ./internal/v2/stage/decompose/... -run TestDecompose_UsesFindingsTemplate -v
```

Expected: FAIL

**Step 3: Add `findingsDecomposePromptTemplate` constant to `decompose.go`**

After `remediationDecomposePromptTemplate`, add:

```go
var findingsDecomposePromptTemplate = `# Targeted Remediation Decompose: %s

You are creating TARGETED beads to address specific code review findings. Do NOT re-implement work that already exists.

## Full Plan (architectural context only)

%s

## Targeted Findings (create beads ONLY for these)

%s

## Skill Instructions

%s

## Acceptance Criteria Rules

acceptance_criteria: each criterion MUST describe an observable behavior or capability, NOT a file path or code structure.

## Output

ONLY create beads that directly address the findings listed above. Map each finding to one or more targeted fix beads.

Output ONLY a JSON array. No markdown, no explanations.
Each bead must include: title, description, priority, acceptance_criteria, expected_outputs, covers_tasks, depends_on_index.

The spec label will be added automatically: spec:%s
`
```

**Step 4: Modify `Run` in `decompose.go` to select template**

In the `Run` method, find the prompt-selection block:

```go
// Existing:
if req.Remediation && gapAnalysis != "" {
    promptText = fmt.Sprintf(remediationDecomposePromptTemplate, ...)
} else {
    promptText = fmt.Sprintf(s.promptTemplate, ...)
}
```

Change to:

```go
if len(req.Findings) > 0 {
    findingsText := formatFindings(req.Findings)
    promptText = fmt.Sprintf(findingsDecomposePromptTemplate, specID, string(planBody), findingsText, skills.DecomposeSkill, specID)
} else if req.Remediation && gapAnalysis != "" {
    promptText = fmt.Sprintf(remediationDecomposePromptTemplate, specID, string(planBody), gapAnalysis, skills.DecomposeSkill, specID)
} else {
    promptText = fmt.Sprintf(s.promptTemplate, specID, string(planBody), skills.DecomposeSkill, specID)
}
```

Add helper:

```go
func formatFindings(findings []stagepkg.Finding) string {
    var b strings.Builder
    for i, f := range findings {
        b.WriteString(fmt.Sprintf("%d. [%s/%s/%s] %s", i+1, f.Severity, f.Category, f.Scope, f.Description))
        if len(f.AffectedFiles) > 0 {
            b.WriteString(fmt.Sprintf("\n   Files: %s", strings.Join(f.AffectedFiles, ", ")))
        }
        b.WriteString("\n")
    }
    return b.String()
}
```

**Step 5: Run tests**

```
go test ./internal/v2/stage/decompose/... -v
```

Expected: all pass

**Step 6: Commit**

```bash
git add internal/v2/stage/decompose/decompose.go internal/v2/stage/decompose/decompose_test.go
git commit -m "feat: add findings-based decompose template for targeted remediation"
```

---

### Task 6: Modify `RemediationRunner` to accept findings and call spec-level review

**Files:**
- Modify: `internal/v2/remediation/remediation.go`
- Modify: `internal/v2/remediation/remediation_test.go`

**Step 1: Write failing tests**

Add to `remediation_test.go`:

```go
func TestRemediationRunner_RunWithFindings_PassesToDecompose(t *testing.T) {
    // Verifies that RunWithFindings sets req.Findings before calling decompose.
    var capturedFindings []stagepkg.Finding
    mockDecompose := &capturingDecomposeStage{
        captureFindings: &capturedFindings,
        result:          emptyDecomposeResult(),
    }
    runner := remediation.NewRemediationRunner(remediation.RemediationRunnerConfig{
        AcceptStage:    alwaysPassAcceptStage(),
        DecomposeStage: mockDecompose,
        BeadRunner:     noopBeadRunner(),
        GenerationCap:  1,
    })
    findings := []stagepkg.Finding{
        {Severity: "critical", Category: "bug", Scope: "spec", Description: "nil panic"},
    }
    err := runner.RunWithFindings(context.Background(), "spec-id", t.TempDir(), findings)
    if err != nil {
        t.Fatalf("RunWithFindings: %v", err)
    }
    if len(capturedFindings) != 1 {
        t.Errorf("Findings passed to decompose: %d, want 1", len(capturedFindings))
    }
}

func TestRemediationRunner_RunWithFindings_RespectsGenerationCap(t *testing.T) {
    // Accept always fails, so remediation keeps looping. Generation cap stops it.
    callCount := 0
    mockAccept := &countingAcceptStage{count: &callCount, alwaysFail: true}
    runner := remediation.NewRemediationRunner(remediation.RemediationRunnerConfig{
        AcceptStage:    mockAccept,
        DecomposeStage: alwaysSucceedDecompose(),
        BeadRunner:     noopBeadRunner(),
        GenerationCap:  2,
    })
    err := runner.RunWithFindings(context.Background(), "s", t.TempDir(), []stagepkg.Finding{
        {Severity: "critical", Category: "acceptance", Scope: "spec", Description: "failed"},
    })
    if err == nil {
        t.Fatal("expected error when generation cap reached")
    }
}
```

**Step 2: Run to confirm failure**

```
go test ./internal/v2/remediation/... -run TestRemediationRunner_RunWithFindings -v
```

Expected: FAIL — `RunWithFindings` does not exist yet

**Step 3: Add `RunWithFindings` to `RemediationRunner`**

In `remediation.go`:

1. Add `SpecReviewStage` to `RemediationRunnerConfig`:

```go
type RemediationRunnerConfig struct {
    AcceptStage      stage.Stage
    SpecReviewStage  stage.Stage   // optional; used to gate each remediation cycle
    GapStage         stage.Stage
    DecomposeStage   stage.Stage
    BeadRunner       BeadRunner
    GenerationCap    int
    Presenter        adapter.PresenterAdapter
    Emitter          *events.Emitter
    WorktreeCleaner  WorktreeCleaner
}
```

2. Add `RunWithFindings` method:

```go
// RunWithFindings executes the remediation cycle starting from pre-computed findings.
// It decomposes targeted beads from the findings, runs the bead loop, then re-evaluates
// accept (and optionally spec-level review) to get findings for the next cycle.
func (r *RemediationRunner) RunWithFindings(ctx context.Context, specID, worktree string, initialFindings []stage.Finding) error {
    r.generationCount = 0
    if specID == "" {
        return ErrSpecIDRequired
    }
    if r.cfg.AcceptStage == nil {
        return ErrAcceptStageRequired
    }

    findings := initialFindings
    for {
        if !r.canRemediate() {
            return r.handleGenerationCap(ctx, specID)
        }

        req := stage.Request{Bead: stage.BeadInfo{ID: specID}, Worktree: worktree}
        req.Remediation = true
        req.Findings = findings

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

        // Re-evaluate accept to collect new findings for the next cycle.
        acceptReq := stage.Request{Bead: stage.BeadInfo{ID: specID}, Worktree: worktree}
        acceptRes, err := r.cfg.AcceptStage.Run(ctx, &acceptReq)
        if err != nil {
            return err
        }

        nextFindings := extractFindings(acceptRes)
        acceptPassed := acceptRes == nil || acceptRes.Decision != stage.DecisionFail

        // Run spec-level review if available and accept passed.
        if acceptPassed && r.cfg.SpecReviewStage != nil {
            reviewRes, err := r.cfg.SpecReviewStage.Run(ctx, &acceptReq)
            if err != nil {
                return err
            }
            if reviewRes != nil && reviewRes.Decision == stage.DecisionFail {
                nextFindings = append(nextFindings, extractFindings(reviewRes)...)
                acceptPassed = false
            }
        }

        if acceptPassed && len(nextFindings) == 0 {
            return r.cleanup(ctx, specID)
        }
        if acceptPassed {
            // Pass with improvements — return nil (caller handles from-review beads).
            return r.cleanup(ctx, specID)
        }

        findings = nextFindings
    }
}
```

3. Add `extractFindings` helper:

```go
type findingsProvider interface {
    GetFindings() []stage.Finding
}

func extractFindings(res *stage.Result) []stage.Finding {
    if res == nil || res.Artifacts == nil {
        return nil
    }
    if fp, ok := res.Artifacts.(findingsProvider); ok {
        return fp.GetFindings()
    }
    return nil
}
```

4. Add `GetFindings()` to `AcceptArtifacts` in `accept.go`:

```go
// GetFindings returns the findings slice, or nil if the receiver is nil.
func (a *AcceptArtifacts) GetFindings() []stagepkg.Finding {
    if a == nil {
        return nil
    }
    return a.Findings
}
```

Similarly add to `SpecReviewArtifacts`:

```go
// GetFindings returns the findings from the spec-level review.
func (a *SpecReviewArtifacts) GetFindings() []stagepkg.Finding {
    if a == nil {
        return nil
    }
    return a.Findings
}
```

**Step 4: Update `remediationRunner` interface in `spec_loop.go`**

Change the private interface:

```go
// Old:
type remediationRunner interface {
    Run(ctx context.Context, specID, worktree string) error
}
// New:
type remediationRunner interface {
    RunWithFindings(ctx context.Context, specID, worktree string, findings []stagepkg.Finding) error
}
```

**Step 5: Run tests**

```
go test ./internal/v2/remediation/... -v
go build ./...
```

Expected: remediation tests pass; build succeeds (we'll fix spec_loop callers in Task 7)

**Step 6: Commit**

```bash
git add internal/v2/remediation/remediation.go internal/v2/remediation/remediation_test.go
git add internal/v2/stage/accept/accept.go internal/v2/stage/specreview/specreview.go
git add internal/v2/loop/spec_loop.go
git commit -m "feat: add RunWithFindings to RemediationRunner; wire findings through accept/specreview/decompose"
```

---

### Task 7: Add spec-level review to `spec_loop.go`

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`

This task wires the specreview stage into the spec loop, changes the remediation call to pass findings, and creates from-review beads when review passes with improvements.

**Step 1: Add `specReviewStage` field and `WithSpecReviewStage` option**

In `SpecLoop` struct, add:

```go
specReviewStage  stagepkg.Stage
```

Add option:

```go
// WithSpecReviewStage installs the spec-level review stage.
func WithSpecReviewStage(stage stagepkg.Stage) SpecLoopOption {
    return func(s *SpecLoop) {
        s.specReviewStage = stage
    }
}
```

**Step 2: Write failing test for the combined accept+review path**

Add to `spec_loop_test.go` (or wherever spec loop tests live):

```go
func TestSpecLoop_RunsSpecReviewAfterAccept(t *testing.T) {
    // When specReviewStage is wired and accept passes, spec review must run.
    // Verify by checking that specReviewStage.Run is called.
    reviewed := false
    mockReview := &trackingStage{name: "spec-review", onRun: func() { reviewed = true }}
    // ... setup spec loop with mockReview as specReviewStage, accept always passes
    // ... run loop
    if !reviewed {
        t.Error("expected specReviewStage.Run to be called after accept passes")
    }
}
```

**Step 3: Modify `ensureAcceptance` to combine accept + spec-level review**

Replace the current `ensureAcceptance` method with a new approach. Rename it to `runAcceptAndReview` and change `Run` to call it:

```go
// runAcceptAndReview calls accept, then spec-level review (if configured).
// Returns combined findings and whether either failed.
func (s *SpecLoop) runAcceptAndReview(ctx context.Context, req *stagepkg.Request) (findings []stagepkg.Finding, failed bool, err error) {
    s.applyRouting(req, "accept")
    acceptRes, err := s.runAcceptStage(ctx, req)
    if err != nil {
        return nil, false, err
    }

    if s.acceptFailed(acceptRes) {
        findings = append(findings, extractFindings(acceptRes)...)
        failed = true
    }

    // Run spec-level review even if accept passed (to check holistic quality).
    if s.specReviewStage != nil {
        s.applyRouting(req, "spec-review")
        reviewReq := *req
        reviewRes, err := s.specReviewStage.Run(ctx, &reviewReq)
        if err != nil {
            return nil, false, err
        }
        if reviewRes != nil && reviewRes.Decision == stagepkg.DecisionFail {
            findings = append(findings, extractFindings(reviewRes)...)
            failed = true
        } else if reviewRes != nil {
            // Pass with improvements — collect findings for from-review beads.
            findings = append(findings, extractFindings(reviewRes)...)
        }
    }

    return findings, failed, nil
}
```

Where `extractFindings` is a package-level helper:

```go
type findingsProvider interface {
    GetFindings() []stagepkg.Finding
}

func extractFindings(res *stagepkg.Result) []stagepkg.Finding {
    if res == nil || res.Artifacts == nil {
        return nil
    }
    if fp, ok := res.Artifacts.(findingsProvider); ok {
        return fp.GetFindings()
    }
    return nil
}
```

**Step 4: Update `Run` to call new method and handle from-review beads**

In the main `Run` method, replace the `ensureAcceptance` call section:

```go
// Old:
s.recordStage("accept")
acceptRes, err := s.ensureAcceptance(ctx, &req, specID)
if err != nil {
    handleFailureCleaned = true
    return s.handleFailure(ctx, specID, baseSummary, err)
}

// New:
s.recordStage("accept")
findings, failed, err := s.runAcceptAndReview(ctx, &req)
if err != nil {
    handleFailureCleaned = true
    return s.handleFailure(ctx, specID, baseSummary, err)
}
if failed {
    if s.remediationRunner == nil {
        handleFailureCleaned = true
        return s.handleFailure(ctx, specID, baseSummary, fmt.Errorf("accept/review failed"))
    }
    retriesRemaining := maxAcceptanceRetries
    for failed && retriesRemaining > 0 {
        if err := s.remediationRunner.RunWithFindings(ctx, specID, req.Worktree, findings); err != nil {
            handleFailureCleaned = true
            return s.handleFailure(ctx, specID, baseSummary, err)
        }
        retriesRemaining--
        findings, failed, err = s.runAcceptAndReview(ctx, &req)
        if err != nil {
            handleFailureCleaned = true
            return s.handleFailure(ctx, specID, baseSummary, err)
        }
    }
    if failed {
        handleFailureCleaned = true
        return s.handleFailure(ctx, specID, baseSummary, fmt.Errorf("%w: limit %d reached", ErrAcceptanceRetriesExceeded, maxAcceptanceRetries))
    }
}

// Create from-review beads for pass-with-improvements findings.
if len(findings) > 0 {
    if err := s.createFromReviewBeads(ctx, specID, findings); err != nil {
        // Log but don't fail the spec run.
        log.Printf("WARNING: creating from-review beads for spec %s: %v", specID, err)
    }
}
```

**Step 5: Implement `createFromReviewBeads`**

```go
const (
    labelFromReview = "from-review"
)

func (s *SpecLoop) createFromReviewBeads(ctx context.Context, specID string, findings []stagepkg.Finding) error {
    if s.adapters.TaskTracker == nil {
        return nil
    }
    for _, f := range findings {
        if f.Severity == "critical" {
            continue // critical findings drove remediation; not deferred
        }
        labels := []string{labelFromReview}
        if f.Scope == "spec" {
            labels = append(labels, fmt.Sprintf("spec:%s", specID))
        }
        priority := 1
        if f.Severity == "warning" {
            priority = 1
        } else {
            priority = 2 // suggestion
        }
        _, err := s.adapters.TaskTracker.CreateBead(ctx, trackertypes.TaskTrackerCreateBeadRequest{
            Title:       fmt.Sprintf("[%s] %s", f.Category, truncate(f.Description, 80)),
            Description: f.Description,
            Priority:    priority,
            Labels:      labels,
        })
        if err != nil {
            return fmt.Errorf("create from-review bead: %w", err)
        }
    }
    return nil
}

func truncate(s string, n int) string {
    if len(s) <= n {
        return s
    }
    return s[:n-3] + "..."
}
```

**Step 6: Run tests**

```
go test ./internal/v2/loop/... -v
go build ./...
```

Expected: all pass; build clean

**Step 7: Commit**

```bash
git add internal/v2/loop/spec_loop.go
git commit -m "feat: add spec-level review to spec loop; wire findings into remediation; create from-review beads on pass-with-improvements"
```

---

### Task 8: Wire spec-level review in `run2_components.go`

**Files:**
- Modify: `internal/v2/loop/run2_components.go`
- Modify: `cmd/gromit/run2.go`

**Step 1: Load `review_spec_v2.md` in `NewRun2LoopComponents`**

In `NewRun2LoopComponents`, after loading `planFragment`:

```go
specReviewFragment, err := loadFragment(cfg.ProjectRoot, "review_spec_v2.md")
if err != nil {
    cleanup()
    return nil, err
}
```

**Step 2: Create the specreview stage with highest tier**

After creating `acceptStage`:

```go
import specreviewstage "github.com/danabrams/gromit/internal/v2/stage/specreview"

specReviewStage, err := specreviewstage.New(cfg, adapters.Git, adapters.LLM, baseInstructions, projectContext, specReviewFragment)
if err != nil {
    cleanup()
    return nil, err
}
```

**Step 3: Add `SpecReviewStage` to `Run2LoopComponents`**

In the `Run2LoopComponents` struct:

```go
type Run2LoopComponents struct {
    PlanStage             stagepkg.Stage
    PresentStage          stagepkg.Stage
    PresentSummaryContext *present.SummaryContext
    DecomposeStage        stagepkg.Stage
    BeadLoop              *BeadLoop
    AcceptStage           stagepkg.Stage
    SpecReviewStage       stagepkg.Stage  // new
    RemediationRunner     remediationRunner
    Emitter               Run2LoopEmitter
    StageCommitter        StageCommitter
    TypedEmitter          *event.Emitter
}
```

**Step 4: Pass spec-level review stage in routing as highest tier**

The `applyRouting` call in `spec_loop.go` uses `routing.TierForPhase("spec-review", s.phaseModels, routing.TierHigh)`. This defaults to `TierHigh` when no override is present — which is the desired behavior (always use highest tier for spec review).

Wire the `RemediationRunner` to also receive `SpecReviewStage`:

```go
remediationRunner := v2remediation.NewRemediationRunner(v2remediation.RemediationRunnerConfig{
    AcceptStage:     acceptStage,
    SpecReviewStage: specReviewStage,  // new
    DecomposeStage:  decomposeStage,
    BeadRunner:      &remediationBeadRunner{loop: beadLoop},
    GenerationCap:   v2remediation.DefaultGenerationCap,
    Emitter:         legacyEmitter,
    Presenter:       adapters.Presenter,
})
```

Return `specReviewStage` in the components struct:

```go
return &Run2LoopComponents{
    // ... existing fields ...
    SpecReviewStage: specReviewStage,
    // ...
}, nil
```

**Step 5: Wire `WithSpecReviewStage` in `run2.go`**

In the `baseOpts` slice in `run2`:

```go
baseOpts := []loop.SpecLoopOption{
    // ... existing options ...
    loop.WithSpecReviewStage(components.SpecReviewStage),  // new
}
```

**Step 6: Build and run all tests**

```
go build ./...
go test ./internal/v2/loop/... -v
go test ./cmd/gromit/... -v
```

Expected: all pass

**Step 7: Commit**

```bash
git add internal/v2/loop/run2_components.go cmd/gromit/run2.go
git commit -m "feat: wire spec-level review stage into run2 components and spec loop"
```

---

### Task 9: Add `--from-review` flag to `run2.go`

**Files:**
- Modify: `cmd/gromit/run2.go`

This adds a separate execution path that skips plan/decompose/accept/review and runs only `from-review`-labeled beads through the bead loop.

**Step 1: Write failing test for the new flag**

Add a test in `cmd/gromit/run2_test.go` (or create it) that verifies `--from-review` causes the spec loop to not run plan/decompose stages. Since `run2` is a cobra command, test via the `run2` function directly with a mock loop.

**Step 2: Register the flag**

In `init()`:

```go
func init() {
    run2Cmd.Flags().String("epic", "", "Run specs scoped to the specified epic")
    run2Cmd.Flags().Bool("from-review", false, "Run only beads with the from-review label")
    run2Cmd.Flags().String("spec", "", "Scope --from-review to a specific spec ID")
}
```

**Step 3: Implement the from-review path**

In `run2`, after building `components`, check the flag:

```go
fromReview, err := cmd.Flags().GetBool("from-review")
if err != nil {
    return fmt.Errorf("reading from-review flag: %w", err)
}

if fromReview {
    scopeSpec, _ := cmd.Flags().GetString("spec")
    return runFromReview(ctx, components, adapters, cfg, scopeSpec, stopCh)
}
```

Implement `runFromReview`:

```go
func runFromReview(ctx context.Context, components *loop.Run2LoopComponents, adapters adapter.AdapterSet, cfg *config.Config, scopeSpec string, stopCh <-chan struct{}) error {
    if adapters.TaskTracker == nil {
        return fmt.Errorf("task tracker required for --from-review")
    }
    queryLabels := []string{"from-review"}
    if trimmed := strings.TrimSpace(scopeSpec); trimmed != "" {
        queryLabels = append(queryLabels, fmt.Sprintf("spec:%s", trimmed))
    }
    resp, err := adapters.TaskTracker.QueryBeads(ctx, trackertypes.TaskTrackerQueryBeadsRequest{
        Labels: queryLabels,
        Status: "open",
    })
    if err != nil {
        return fmt.Errorf("query from-review beads: %w", err)
    }
    if resp == nil || len(resp.Beads) == 0 {
        return nil // nothing to run
    }
    beads := make([]*bead.Bead, 0, len(resp.Beads))
    for _, b := range resp.Beads {
        beads = append(beads, &bead.Bead{
            ID:          b.ID,
            Title:       b.Title,
            Description: b.Description,
            Priority:    b.Priority,
            Labels:      b.Labels,
            Status:      b.Status,
        })
    }
    // Run through bead loop only — no plan, decompose, accept, review, remediation.
    _, err = components.BeadLoop.Run(ctx, beads, stopCh)
    return err
}
```

**Step 4: Update `run2Args` to allow no positional args when `--from-review` is set**

```go
func run2Args(cmd *cobra.Command, args []string) error {
    fromReview, _ := cmd.Flags().GetBool("from-review")
    if fromReview {
        if len(args) > 0 {
            return fmt.Errorf("--from-review cannot be combined with a spec file argument")
        }
        return nil
    }
    epicID, err := cmd.Flags().GetString("epic")
    if err != nil {
        return fmt.Errorf("reading epic flag: %w", err)
    }
    if strings.TrimSpace(epicID) != "" {
        if len(args) > 0 {
            return fmt.Errorf("the --epic flag cannot be combined with a spec file")
        }
        return nil
    }
    if len(args) != 1 {
        return fmt.Errorf("spec file argument required")
    }
    return nil
}
```

**Step 5: Run tests**

```
go build ./...
go test ./cmd/gromit/... -v
```

Expected: all pass

**Step 6: Commit**

```bash
git add cmd/gromit/run2.go
git commit -m "feat: add --from-review flag to run2 for running only from-review-labeled beads"
```

---

### Task 10: Integration tests

**Files:**
- Create: `internal/v2/loop/spec_loop_specreview_integration_test.go`

**Step 1: Write integration tests**

```go
package loop_test

// TestIntegration_SpecLevelReview_FullPipeline verifies the full post-bead-loop pipeline:
// bead loop completes → accept passes → spec-level review fails with critical finding
// → remediation receives findings → targeted decompose creates beads → accept + review pass.

// TestIntegration_SpecLevelReview_PassWithImprovements verifies that when review passes
// with warning/suggestion findings, from-review beads are created and the spec proceeds.

// TestIntegration_SpecLevelReview_NotCalledWhenAcceptFails verifies that spec-level review
// does not run when accept has already failed (findings from accept are sufficient).
```

Implement using the existing test patterns in `spec_loop_test.go`. Key assertions:
1. Full pipeline: verify `remediationRunner.RunWithFindings` is called with findings from spec review
2. Pass-with-improvements: verify from-review beads are created via `adapters.TaskTracker.CreateBead` with `from-review` label
3. From-review beads scoped to spec get `spec:<id>` label; general findings do not

**Step 2: Run the integration tests**

```
go test ./internal/v2/loop/... -run TestIntegration_SpecLevel -v
```

Expected: all pass

**Step 3: Run the full test suite**

```
go test ./... -count=1
```

Expected: all pass, no regressions

**Step 4: Commit**

```bash
git add internal/v2/loop/spec_loop_specreview_integration_test.go
git commit -m "test: integration tests for spec-level review pipeline and pass-with-improvements"
```

---

## Notes for Implementer

### Import paths

The Go module is `github.com/danabrams/gromit`. All internal imports follow this pattern:
- Stage types: `github.com/danabrams/gromit/internal/v2/stage`
- Accept stage: `github.com/danabrams/gromit/internal/v2/stage/accept`
- Specreview stage: `github.com/danabrams/gromit/internal/v2/stage/specreview` (new)
- Routing tiers: `github.com/danabrams/gromit/internal/v2/routing` — use `routing.TierHigh`
- JSON parsing: `github.com/danabrams/gromit/internal/jsonutil` — use `jsonutil.ExtractObject`

### `config.ModelOpus`

Check `internal/config/config.go` for the exact constant name. It may be `config.ModelOpus` or similar. The `specreview` stage's `defaultModel()` should return the highest-tier model constant.

### Test patterns

- Use `t.TempDir()` for all temp directories (not `os.MkdirTemp`)
- Use `t.Chdir()` instead of `os.Chdir()` in any test that needs to change directory
- Package-level injectable vars overridden in tests must use `t.Cleanup` for restoration
- Shared helper packages must not expose mutable global maps or slices

### Nil-safety

The specreview stage follows the same nil-receiver patterns as other stages: `Name()`, `GetFindings()`, and `GetGapSummary()` all guard with `if s == nil { return "" }` / `if a == nil { return nil }`.

### `phaseModels` for `spec-review`

Add to `phaseModelsFromConfig` in `run2.go`:

```go
if pm.SpecReview != "" {
    m["spec-review"] = pm.SpecReview
}
```

And to `config.PhaseModelsConfig` in `internal/config/`:

```go
SpecReview string `yaml:"spec_review"`
```

Check `internal/config/config.go` for the exact struct before adding.

### `GitDiffer` in specreview

The `GitDiffer` interface in specreview needs `DiffFromBase` (not `Diff`). The `gitadapter.ExecGitAdapter` already implements `DiffFromBase` — no adapter changes needed. Use `adapter.GitAdapter` or create a minimal interface as shown in the accept stage.
