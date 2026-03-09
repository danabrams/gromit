# Spec-Level Review and Targeted Remediation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** After the bead loop completes, run a spec-level holistic code review using the highest-tier model, combine its findings with accept failures into a structured format, and feed that structure into a findings-based decompose to produce targeted fix beads — eliminating the duplicate-bead loop-divergence problem.

**Architecture:** A new `specreview` stage evaluates the cumulative diff holistically and returns structured `SpecFinding` records. Both `accept` and `specreview` produce findings in a shared type defined in `internal/v2/stage/stage.go`. The remediation runner is extended to run both stages in its evaluation loop and pass findings via `StageRequest.Findings` to a new findings-based decompose template. The spec loop wires specreview after accept, creates from-review beads on pass-with-improvements, and `run2` gains a `--from-review` flag to execute from-review beads directly.

**Tech Stack:** Go, existing stage/remediation/decompose/loop packages, jsonutil, coverage packages

---

### Task 1: Add SpecFinding type and Findings field to stage package

**Files:**
- Modify: `internal/v2/stage/stage.go:19-31`
- Test: `internal/v2/stage/stage_test.go` (or create if absent)

**Step 1: Add SpecFinding type and constants to stage.go**

In `internal/v2/stage/stage.go`, after the `LLMCostSummary` struct, add:

```go
// SpecFindingSeverity classifies how urgent a spec-level review finding is.
type SpecFindingSeverity string

const (
	SeverityCritical   SpecFindingSeverity = "critical"
	SeverityWarning    SpecFindingSeverity = "warning"
	SeveritySuggestion SpecFindingSeverity = "suggestion"
)

// SpecFindingCategory classifies the kind of issue found by spec-level review.
type SpecFindingCategory string

const (
	CategoryBug          SpecFindingCategory = "bug"
	CategorySecurity     SpecFindingCategory = "security"
	CategoryQuality      SpecFindingCategory = "quality"
	CategoryTestGap      SpecFindingCategory = "test-gap"
	CategoryArchitecture SpecFindingCategory = "architecture"
	CategoryAcceptance   SpecFindingCategory = "acceptance"
)

// SpecFindingScope identifies whether a finding is within this spec's changes or general.
type SpecFindingScope string

const (
	ScopeSpec    SpecFindingScope = "spec"
	ScopeGeneral SpecFindingScope = "general"
)

// SpecFinding represents a single observation from spec-level review or accept evaluation.
type SpecFinding struct {
	Severity     SpecFindingSeverity `json:"severity"`
	Category     SpecFindingCategory `json:"category"`
	Scope        SpecFindingScope    `json:"scope"`
	Description  string              `json:"description"`
	AffectedFiles []string           `json:"affected_files"`
}

// HasCritical returns true if any finding has critical severity.
func HasCritical(findings []SpecFinding) bool {
	for _, f := range findings {
		if f.Severity == SeverityCritical {
			return true
		}
	}
	return false
}
```

**Step 2: Add Findings field to StageRequest**

In the `StageRequest` struct (around line 20), add after `GapAnalysis string`:

```go
Findings []SpecFinding // structured findings for findings-based remediation decompose
```

**Step 3: Run tests to confirm no breakage**

Run: `go test ./internal/v2/stage/...`
Expected: All existing tests pass (additive changes)

**Step 4: Commit**

```bash
git add internal/v2/stage/stage.go
git commit -m "feat: add SpecFinding type and Findings field to StageRequest"
```

---

### Task 2: Create specreview stage package

**Files:**
- Create: `internal/v2/stage/specreview/specreview.go`
- Create: `internal/v2/stage/specreview/specreview_test.go`

**Step 1: Write the failing tests first**

Create `internal/v2/stage/specreview/specreview_test.go`:

```go
package specreview_test

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/specreview"
)

type stubGit struct{ diff string; err error }
func (s *stubGit) DiffFromBase(_ context.Context, _ string) (string, error) { return s.diff, s.err }

type stubLLM struct{ output string; success bool }
func (s *stubLLM) Invoke(_ context.Context, req interface{}) (interface{}, error) {
	// test via integration; use fake LLM provider type
	return nil, nil
}

func TestNew_nilConfigReturnsError(t *testing.T) {
	_, err := specreview.New(nil, &stubGit{}, &fakeLLM{}, "base", "proj", "")
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNew_nilGitReturnsError(t *testing.T) {
	cfg := &config.Config{}
	_, err := specreview.New(cfg, nil, &fakeLLM{}, "base", "proj", "")
	if err == nil {
		t.Fatal("expected error for nil git")
	}
}

func TestNew_nilLLMReturnsError(t *testing.T) {
	cfg := &config.Config{}
	_, err := specreview.New(cfg, &stubGit{}, nil, "base", "proj", "")
	if err == nil {
		t.Fatal("expected error for nil llm")
	}
}

func TestParseSpecReviewOutput_verdictPass(t *testing.T) {
	raw := `{"verdict":"pass","findings":[]}`
	verdict, findings, err := specreview.ParseSpecReviewOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != "pass" {
		t.Errorf("expected pass, got %s", verdict)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestParseSpecReviewOutput_criticalForcesFailVerdict(t *testing.T) {
	raw := `{"verdict":"pass","findings":[{"severity":"critical","category":"bug","scope":"spec","description":"nil pointer","affected_files":["foo.go"]}]}`
	verdict, findings, err := specreview.ParseSpecReviewOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Even if LLM said pass, critical finding forces fail
	if verdict != "fail" {
		t.Errorf("expected fail due to critical finding, got %s", verdict)
	}
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestParseSpecReviewOutput_warningKeepsPass(t *testing.T) {
	raw := `{"verdict":"pass","findings":[{"severity":"warning","category":"quality","scope":"general","description":"minor style","affected_files":[]}]}`
	verdict, _, err := specreview.ParseSpecReviewOutput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != "pass" {
		t.Errorf("expected pass with warning, got %s", verdict)
	}
}

func TestRun_nilRequestReturnsError(t *testing.T) {
	cfg := &config.Config{}
	cfg.Models.P0 = "claude-opus"
	s, _ := specreview.New(cfg, &stubGit{diff: ""}, &fakeLLM{output: `{"verdict":"pass","findings":[]}`}, "", "", "")
	_, err := s.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}
```

Note: The `fakeLLM` in tests should implement `llmtypes.LLMProvider`. Create a test helper that returns canned JSON.

**Step 2: Run tests to see them fail**

Run: `go test ./internal/v2/stage/specreview/...`
Expected: compilation failure (package doesn't exist yet)

**Step 3: Create the implementation**

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
	"github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

const defaultSpecReviewFragment = `# Spec-Level Code Review Instructions

You are performing a holistic review of all changes made during this spec's implementation.
This review evaluates the CUMULATIVE diff — the combined output of all beads in the spec.

## Review Dimensions

### 1. Correctness
- Does the code work beyond the test coverage?
- Are error conditions handled?
- Edge cases not accounted for?

### 2. Security (OWASP Top 10)
- SQL/command/template injection risks?
- Authentication/authorization bypass?
- Data exposure, logging of secrets, missing input validation?

### 3. Error Handling
- Are errors propagated, not swallowed?
- Are sentinel errors used for callers to distinguish?
- Missing nil checks on external returns?

### 4. Test Coverage Gaps
- Untested code paths?
- Missing edge case tests?
- Are tests asserting behavior, or just coverage?

### 5. Code Quality
- Dead code, unused imports?
- Overly complex logic that should be simplified?
- Naming convention violations?

### 6. Architectural Fit
- Does new code follow the project's existing patterns?
- Are packages used at the right abstraction level?
- Does new behavior belong in the right layer?

## Scope Classification

For each finding, classify scope:
- "spec": the issue is in code introduced or modified by this spec
- "general": the issue exists in pre-existing code this spec did not touch

## Output Format

Respond with ONLY a JSON object:

{"verdict":"pass","findings":[{"severity":"critical","category":"bug","scope":"spec","description":"...","affected_files":["path/file.go"]}]}

Verdict rules:
- "fail" if ANY finding has severity "critical"
- "pass" if all findings are "warning" or "suggestion" (or no findings)

severity values: "critical", "warning", "suggestion"
category values: "bug", "security", "quality", "test-gap", "architecture"
scope values: "spec", "general"

Respond with ONLY the JSON object. No markdown wrapper, no explanation.
`

// GitDiffer provides the DiffFromBase capability needed by specreview.
type GitDiffer interface {
	DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// SpecReviewArtifacts captures the output of the spec-level review stage.
type SpecReviewArtifacts struct {
	Verdict  string               // "pass" or "fail"
	Findings []stagepkg.SpecFinding
}

// Stage executes the spec-level review after the bead loop completes.
type Stage struct {
	name     string
	cfg      *config.Config
	git      GitDiffer
	llm      llmtypes.LLMProvider
	base     string
	project  string
	fragment string
}

var _ stagepkg.Stage = (*Stage)(nil)

// New constructs a specreview stage.
func New(cfg *config.Config, git GitDiffer, llm llmtypes.LLMProvider, base, project, fragment string) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if git == nil {
		return nil, fmt.Errorf("git adapter required")
	}
	if llm == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	if strings.TrimSpace(fragment) == "" {
		fragment = defaultSpecReviewFragment
	}
	return &Stage{
		name:     stagedesc.Describe("specreview", cfg),
		cfg:      cfg,
		git:      git,
		llm:      llm,
		base:     base,
		project:  project,
		fragment: fragment,
	}, nil
}

// Name returns the canonical stage identifier.
func (s *Stage) Name() string { return s.name }

// Run invokes the spec-level review LLM and returns structured findings.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	cfg := req.Config
	if cfg == nil {
		cfg = s.cfg
	}

	root := strings.TrimSpace(req.Worktree)
	if root == "" {
		root = cfg.ProjectRoot
	}
	if root == "" {
		root = "."
	}

	diff, err := s.git.DiffFromBase(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("specreview: git diff: %w", err)
	}

	instance := buildInstance(diff)
	promptText := prompt.NewPromptAssembler(s.base, s.project, instance, s.fragment).
		Assemble("specreview", prompt.BeadInfo{Title: req.Bead.ID})

	provider := s.llm
	if req.Provider != nil {
		provider = req.Provider
	}

	model := s.selectModel(cfg, req)
	resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: promptText, Model: model, Dir: req.Worktree})
	if err != nil {
		return nil, fmt.Errorf("specreview: invoke: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("specreview: nil response")
	}
	if !resp.Success {
		return nil, fmt.Errorf("specreview: invocation failed: %s", resp.Output)
	}

	verdict, findings, err := ParseSpecReviewOutput(resp.Output)
	if err != nil {
		return nil, fmt.Errorf("specreview: parse output: %w", err)
	}

	artifacts := &SpecReviewArtifacts{Verdict: verdict, Findings: findings}
	if verdict == "fail" {
		return &stagepkg.Result{Decision: stagepkg.DecisionFail, Artifacts: artifacts}, nil
	}
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed, Artifacts: artifacts}, nil
}

// ParseSpecReviewOutput parses the LLM JSON response and enforces verdict logic.
// Exported for testing.
func ParseSpecReviewOutput(raw string) (verdict string, findings []stagepkg.SpecFinding, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil, fmt.Errorf("empty output")
	}

	var out struct {
		Verdict  string `json:"verdict"`
		Findings []struct {
			Severity      string   `json:"severity"`
			Category      string   `json:"category"`
			Scope         string   `json:"scope"`
			Description   string   `json:"description"`
			AffectedFiles []string `json:"affected_files"`
		} `json:"findings"`
	}
	if err := jsonutil.ExtractObject(trimmed, &out); err != nil {
		return "", nil, fmt.Errorf("parse spec review output: %w", err)
	}

	findings = make([]stagepkg.SpecFinding, 0, len(out.Findings))
	for _, f := range out.Findings {
		findings = append(findings, stagepkg.SpecFinding{
			Severity:      stagepkg.SpecFindingSeverity(f.Severity),
			Category:      stagepkg.SpecFindingCategory(f.Category),
			Scope:         stagepkg.SpecFindingScope(f.Scope),
			Description:   f.Description,
			AffectedFiles: append([]string(nil), f.AffectedFiles...),
		})
	}

	// Enforce: any critical finding forces fail regardless of LLM-stated verdict.
	if stagepkg.HasCritical(findings) {
		return "fail", findings, nil
	}
	verdict = strings.TrimSpace(out.Verdict)
	if verdict != "pass" && verdict != "fail" {
		verdict = "fail" // default to fail on unrecognized verdict
	}
	return verdict, findings, nil
}

func (s *Stage) selectModel(cfg *config.Config, req *stagepkg.Request) string {
	if req != nil {
		if m := strings.TrimSpace(req.Model); m != "" {
			return m
		}
	}
	// Always use highest-tier model for spec-level review.
	if m := strings.TrimSpace(cfg.Models.P0); m != "" {
		return m
	}
	return config.ModelOpus
}

func buildInstance(diff string) string {
	if trimmed := strings.TrimSpace(diff); trimmed != "" {
		return fmt.Sprintf("## Cumulative Diff\n\n%s", trimmed)
	}
	return "## Cumulative Diff\n\n(no changes)"
}
```

**Step 4: Run tests**

Run: `go test ./internal/v2/stage/specreview/...`
Expected: Tests pass

**Step 5: Run full suite to check for regressions**

Run: `go test ./internal/v2/...`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/v2/stage/specreview/
git commit -m "feat: add specreview stage with structured findings and verdict logic"
```

---

### Task 3: Create review_spec_v2.md prompt fragment

**Files:**
- Create: `review_spec_v2.md` (project root)

**Step 1: Create the file**

The `defaultSpecReviewFragment` in the specreview stage is used when no file is provided.
The file at project root overrides it for production runs. Create `review_spec_v2.md` with the same content as `defaultSpecReviewFragment` so operators can customize it:

```markdown
# Spec-Level Code Review Instructions

You are performing a holistic review of all changes made during this spec's implementation.
This review evaluates the CUMULATIVE diff — the combined output of all beads in the spec.

## Review Dimensions

### 1. Correctness
- Does the code work beyond the test coverage?
- Are error conditions handled?
- Edge cases not accounted for?

### 2. Security (OWASP Top 10)
- SQL/command/template injection risks?
- Authentication/authorization bypass?
- Data exposure, logging of secrets, missing input validation?

### 3. Error Handling
- Are errors propagated, not swallowed?
- Are sentinel errors used for callers to distinguish?
- Missing nil checks on external returns?

### 4. Test Coverage Gaps
- Untested code paths?
- Missing edge case tests?
- Are tests asserting behavior, or just coverage?

### 5. Code Quality
- Dead code, unused imports?
- Overly complex logic that should be simplified?
- Naming convention violations?

### 6. Architectural Fit
- Does new code follow the project's existing patterns?
- Are packages used at the right abstraction level?
- Does new behavior belong in the right layer?

## Scope Classification

For each finding, classify scope:
- "spec": the issue is in code introduced or modified by this spec
- "general": the issue exists in pre-existing code this spec did not touch

## Output Format

Respond with ONLY a JSON object:

{"verdict":"pass","findings":[{"severity":"critical","category":"bug","scope":"spec","description":"...","affected_files":["path/file.go"]}]}

Verdict rules:
- "fail" if ANY finding has severity "critical"
- "pass" if all findings are "warning" or "suggestion" (or no findings)

severity values: "critical", "warning", "suggestion"
category values: "bug", "security", "quality", "test-gap", "architecture"
scope values: "spec", "general"

Respond with ONLY the JSON object. No markdown wrapper, no explanation.
```

**Step 2: Verify it loads (no test needed — loadFragment already handles missing gracefully)**

Run: `go build ./...`
Expected: Clean build

**Step 3: Commit**

```bash
git add review_spec_v2.md
git commit -m "feat: add review_spec_v2.md prompt fragment for spec-level review"
```

---

### Task 4: Add structured findings to accept stage

**Files:**
- Modify: `internal/v2/stage/accept/accept.go:60-72` (AcceptArtifacts struct)
- Modify: `internal/v2/stage/accept/accept.go:210-232` (failure collection)
- Test: `internal/v2/stage/accept/accept_test.go`

**Step 1: Write the failing tests**

In `accept_test.go`, add tests that verify findings are populated on failure. Look for existing tests that mock the LLM returning `{"pass":false,"summary":"missing implementation"}` and add assertions:

```go
func TestRun_failedCriterion_populatesFindings(t *testing.T) {
    // Set up stage with LLM that returns fail for one criterion
    // Run stage
    // Extract AcceptArtifacts
    artifacts, ok := result.Artifacts.(*accept.AcceptArtifacts)
    if !ok {
        t.Fatal("expected AcceptArtifacts")
    }
    if len(artifacts.Findings) == 0 {
        t.Error("expected findings for failed criterion")
    }
    if artifacts.Findings[0].Severity != stagepkg.SeverityCritical {
        t.Errorf("expected critical severity, got %s", artifacts.Findings[0].Severity)
    }
    if artifacts.Findings[0].Category != stagepkg.CategoryAcceptance {
        t.Errorf("expected acceptance category, got %s", artifacts.Findings[0].Category)
    }
}
```

**Step 2: Run tests to see them fail**

Run: `go test ./internal/v2/stage/accept/...`
Expected: FAIL (Findings field doesn't exist yet)

**Step 3: Modify AcceptArtifacts**

In `internal/v2/stage/accept/accept.go`, update `AcceptArtifacts`:

```go
// AcceptArtifacts captures acceptance evaluation results produced by the stage.
type AcceptArtifacts struct {
	Results    []presentation.AcceptanceResult
	GapSummary string
	Findings   []stagepkg.SpecFinding // structured findings for findings-based remediation
}
```

**Step 4: Populate findings in Run()**

In `internal/v2/stage/accept/accept.go`, in the `Run()` method where failures are collected (around line 215-232), after the existing `failures` slice append, also append to a new findings slice:

```go
// In the Run() method, after declaring failures:
findings := make([]stagepkg.SpecFinding, 0)

// Inside the loop where criterion fails (where score == "FAIL"):
findings = append(findings, stagepkg.SpecFinding{
    Severity:    stagepkg.SeverityCritical,
    Category:    stagepkg.CategoryAcceptance,
    Scope:       stagepkg.ScopeSpec,
    Description: fmt.Sprintf("Criterion %d: %s — %s", criterion.Number, trimmed, summaryOrDefault(summary)),
})
```

Then set `artifacts.Findings = findings` before the `DecisionFail` return.

**Step 5: Run tests**

Run: `go test ./internal/v2/stage/accept/...`
Expected: All tests pass including new finding assertion

**Step 6: Commit**

```bash
git add internal/v2/stage/accept/accept.go internal/v2/stage/accept/accept_test.go
git commit -m "feat: accept stage produces structured SpecFinding records for failed criteria"
```

---

### Task 5: Add findings-based decompose template

**Files:**
- Modify: `internal/v2/stage/decompose/decompose.go:68-105` (after remediationDecomposePromptTemplate)
- Modify: `internal/v2/stage/decompose/decompose.go:160-188` (Run() prompt selection)
- Test: `internal/v2/stage/decompose/decompose_test.go`

**Step 1: Write failing tests**

In `decompose_test.go`, add:

```go
func TestRun_withFindings_usesFindingsTemplate(t *testing.T) {
    // Set up stage with stub LLM that captures the prompt text
    // Set req.Findings = []stagepkg.SpecFinding{...}
    // Run stage
    // Assert the captured prompt contains "targeted fix beads" or findings JSON
    // Assert it does NOT contain the full plan decompose instructions
}
```

**Step 2: Run tests to see them fail**

Run: `go test ./internal/v2/stage/decompose/...`
Expected: FAIL (findings logic not implemented)

**Step 3: Add the findings decompose prompt template**

In `decompose.go`, after `remediationDecomposePromptTemplate`, add:

```go
var findingsDecomposePromptTemplate = `# Targeted Fix Decompose: %s

You are creating TARGETED beads to address specific review findings. Do NOT re-implement work that already exists.

## Full Plan (architectural context only — do NOT re-decompose it)

%s

## Specific Findings to Fix (create beads ONLY for these)

%s

## Skill Instructions

%s

## Rules

- Create one bead per finding or per tightly-coupled group of findings.
- Do NOT create beads for work not listed in the findings above.
- Each bead acceptance_criteria must describe observable behavior, NOT file paths.
- depends_on_index: 0-based index of prerequisite beads in THIS output array.

## Output

Output ONLY a JSON array of bead definitions. No markdown, no explanations.
Each bead: title, description, priority, acceptance_criteria, expected_outputs, covers_tasks, depends_on_index.

The spec label will be added automatically: spec:%s
`
```

**Step 4: Update Run() to route on findings**

In `decompose.go`'s `Run()` method, update the prompt selection block (around line 183-188):

```go
gapAnalysis := s.resolveGapAnalysis(req)
switch {
case len(req.Findings) > 0:
    findingsJSON := formatFindingsForPrompt(req.Findings)
    promptText = fmt.Sprintf(findingsDecomposePromptTemplate, specID, string(planBody), findingsJSON, skills.DecomposeSkill, specID)
case req.Remediation && gapAnalysis != "":
    promptText = fmt.Sprintf(remediationDecomposePromptTemplate, specID, string(planBody), gapAnalysis, skills.DecomposeSkill, specID)
default:
    promptText = fmt.Sprintf(s.promptTemplate, specID, string(planBody), skills.DecomposeSkill, specID)
}
```

Add the helper function:

```go
func formatFindingsForPrompt(findings []stagepkg.SpecFinding) string {
    if len(findings) == 0 {
        return "(no findings)"
    }
    var sb strings.Builder
    for i, f := range findings {
        sb.WriteString(fmt.Sprintf("%d. [%s/%s/%s] %s", i+1, f.Severity, f.Category, f.Scope, f.Description))
        if len(f.AffectedFiles) > 0 {
            sb.WriteString(fmt.Sprintf(" (files: %s)", strings.Join(f.AffectedFiles, ", ")))
        }
        sb.WriteString("\n")
    }
    return sb.String()
}
```

**Step 5: Run tests**

Run: `go test ./internal/v2/stage/decompose/...`
Expected: All tests pass including new findings routing test

**Step 6: Commit**

```bash
git add internal/v2/stage/decompose/decompose.go internal/v2/stage/decompose/decompose_test.go
git commit -m "feat: decompose stage routes to findings-based template when SpecFindings present"
```

---

### Task 6: Update remediation runner to use SpecReviewStage and structured findings

**Files:**
- Modify: `internal/v2/remediation/remediation.go` (all key sections)
- Modify: `internal/v2/remediation/remediation_test.go`

**Step 1: Write failing tests**

In `remediation_test.go`, add:

```go
func TestRemediationRunner_passesWhenBothAcceptAndReviewPass(t *testing.T) {
    // specReviewStage returns DecisionProceed
    // acceptStage returns DecisionProceed
    // Run() should return nil without calling bead runner
}

func TestRemediationRunner_decomposeReceivesFindings_whenAcceptFails(t *testing.T) {
    // acceptStage returns DecisionFail with AcceptArtifacts.Findings populated
    // specReviewStage returns DecisionProceed with no critical findings
    // decomposeStage captures the request
    // Assert req.Findings contains the accept findings
}

func TestRemediationRunner_combinesAcceptAndReviewFindings(t *testing.T) {
    // acceptStage fails with 1 finding
    // specReviewStage fails with 1 critical finding
    // Decompose receives 2 findings total
}
```

**Step 2: Run tests to see them fail**

Run: `go test ./internal/v2/remediation/...`
Expected: FAIL

**Step 3: Add SpecReviewStage to RemediationRunnerConfig**

In `remediation.go`, update `RemediationRunnerConfig`:

```go
type RemediationRunnerConfig struct {
    AcceptStage     stage.Stage
    SpecReviewStage stage.Stage // spec-level review; optional, run alongside accept
    GapStage        stage.Stage
    DecomposeStage  stage.Stage
    BeadRunner      BeadRunner
    GenerationCap   int
    Presenter       adapter.PresenterAdapter
    Emitter         *events.Emitter
    WorktreeCleaner WorktreeCleaner
}
```

**Step 4: Update Run() to evaluate both accept and specreview**

Replace the current `Run()` loop body:

```go
func (r *RemediationRunner) Run(ctx context.Context, specID, worktree string) error {
    r.generationCount = 0
    if specID == "" {
        return ErrSpecIDRequired
    }
    if r.cfg.AcceptStage == nil {
        return ErrAcceptStageRequired
    }

    reqTemplate := stage.Request{Bead: stage.BeadInfo{ID: specID}, Worktree: worktree}
    for {
        req := reqTemplate

        // Run accept stage.
        acceptRes, err := r.cfg.AcceptStage.Run(ctx, &req)
        if err != nil {
            return err
        }

        // Run spec-level review stage (if configured).
        var reviewRes *stage.Result
        if r.cfg.SpecReviewStage != nil {
            reviewRes, err = r.cfg.SpecReviewStage.Run(ctx, &req)
            if err != nil {
                return err
            }
        }

        // Collect all critical findings.
        findings := collectFindings(acceptRes, reviewRes)
        if !stage.HasCritical(findings) {
            // Both passed (or only non-critical findings) — done.
            if err := r.cleanup(ctx, specID); err != nil {
                return err
            }
            return nil
        }

        if err := r.executeRemediation(ctx, &req, findings); err != nil {
            return err
        }
    }
}
```

**Step 5: Add collectFindings helper**

```go
func collectFindings(acceptRes, reviewRes *stage.Result) []stage.SpecFinding {
    var findings []stage.SpecFinding
    if acceptRes != nil {
        if arts, ok := acceptRes.Artifacts.(findingsProvider); ok {
            findings = append(findings, arts.GetFindings()...)
        }
    }
    if reviewRes != nil {
        if arts, ok := reviewRes.Artifacts.(findingsProvider); ok {
            findings = append(findings, arts.GetFindings()...)
        }
    }
    return findings
}

type findingsProvider interface {
    GetFindings() []stage.SpecFinding
}
```

Add `GetFindings()` to `AcceptArtifacts` in accept package (do this in the same commit or note as a dependency):

In `internal/v2/stage/accept/accept.go`:
```go
func (a *AcceptArtifacts) GetFindings() []stagepkg.SpecFinding {
    if a == nil {
        return nil
    }
    return a.Findings
}
```

Add `GetFindings()` to `SpecReviewArtifacts` in specreview package:
```go
func (a *SpecReviewArtifacts) GetFindings() []stagepkg.SpecFinding {
    if a == nil {
        return nil
    }
    return a.Findings
}
```

**Step 6: Update executeRemediation to take findings**

```go
func (r *RemediationRunner) executeRemediation(ctx context.Context, req *stage.Request, findings []stage.SpecFinding) error {
    specID := req.Bead.ID
    if !r.canRemediate() {
        return r.handleGenerationCap(ctx, specID)
    }

    req.Remediation = true
    req.Findings = findings  // pass structured findings to decompose

    if r.cfg.GapStage != nil {
        if _, err := r.cfg.GapStage.Run(ctx, req); err != nil {
            return err
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

**Step 7: Run tests**

Run: `go test ./internal/v2/remediation/...`
Expected: All tests pass

Run: `go test ./internal/v2/...`
Expected: All tests pass

**Step 8: Commit**

```bash
git add internal/v2/remediation/remediation.go internal/v2/remediation/remediation_test.go \
    internal/v2/stage/accept/accept.go internal/v2/stage/specreview/specreview.go
git commit -m "feat: remediation runner runs specreview alongside accept, decomposes from structured findings"
```

---

### Task 7: Wire specreview into spec_loop

**Files:**
- Modify: `internal/v2/loop/spec_loop.go` (SpecLoop struct, ensureAcceptance, StageSequence)
- Modify: `internal/v2/loop/spec_loop_test.go`

**Step 1: Write failing tests**

In `spec_loop_test.go` or a new `spec_loop_specreview_test.go`:

```go
func TestSpecLoop_runsSpecReviewAfterBeadLoop(t *testing.T) {
    // Install stub specReviewStage that records it was called
    // Run spec loop with stubs
    // Assert specReviewStage.Run was called
}

func TestSpecLoop_failsWhenSpecReviewReturnsCritical(t *testing.T) {
    // Install specReviewStage that returns DecisionFail
    // Install acceptStage that returns DecisionProceed
    // Run spec loop
    // Expect error (combined failure triggers remediation or terminal fail)
}

func TestSpecLoop_createsFromReviewBeads_onPassWithImprovements(t *testing.T) {
    // Install specReviewStage returning pass with warning findings
    // Install stub task tracker
    // Run spec loop
    // Assert tracker.CreateBead was called for spec-scoped findings with "from-review" label
}
```

**Step 2: Run tests to see them fail**

Run: `go test ./internal/v2/loop/...`
Expected: FAIL

**Step 3: Add specReviewStage to SpecLoop**

In `spec_loop.go`, in the `SpecLoop` struct (around line 197-218), add:

```go
specReviewStage stagepkg.Stage
```

Add the option:

```go
// WithSpecReviewStage configures the spec-level review stage.
func WithSpecReviewStage(stage stagepkg.Stage) SpecLoopOption {
    return func(s *SpecLoop) {
        s.specReviewStage = stage
    }
}
```

Add "specreview" to `StageSequence` after "accept":

```go
var StageSequence = []string{
    "plan",
    "decompose",
    "gate",
    "build",
    "validate",
    "review",
    "epilogue",
    "accept",
    "specreview",
    "present",
}
```

**Step 4: Update ensureAcceptance to also call specreview**

Rename `ensureAcceptance` to `ensureAcceptanceAndReview` (update all call sites) or update in place. The method now:
1. Runs accept
2. Runs specreview (if configured)
3. Combines findings
4. If any critical findings → call remediationRunner
5. If no critical but review has warnings/suggestions → create from-review beads (after remediation loop exits successfully)

```go
func (s *SpecLoop) ensureAcceptance(ctx context.Context, req *stagepkg.Request, specID string) (*stagepkg.Result, error) {
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

        s.recordStage("specreview")
        reviewRes, err := s.runSpecReviewStage(ctx, req)
        if err != nil {
            return nil, err
        }

        // Both passed — collect non-critical findings for from-review beads.
        acceptFailed := s.acceptFailed(acceptRes)
        reviewFailed := reviewRes != nil && reviewRes.Decision == stagepkg.DecisionFail

        if !acceptFailed && !reviewFailed {
            // Create from-review beads for non-critical review findings.
            if reviewRes != nil {
                s.createFromReviewBeads(ctx, specID, reviewRes)
            }
            return acceptRes, nil
        }

        if s.remediationRunner == nil {
            return acceptRes, fmt.Errorf("accept/review failed")
        }
        if retriesRemaining <= 0 {
            return acceptRes, fmt.Errorf("%w: limit %d reached", ErrAcceptanceRetriesExceeded, maxAcceptanceRetries)
        }
        if err := s.remediationRunner.Run(ctx, specID, req.Worktree); err != nil {
            return acceptRes, err
        }
        retriesRemaining--
    }
}

func (s *SpecLoop) runSpecReviewStage(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
    if s.specReviewStage == nil {
        return nil, nil
    }
    return s.specReviewStage.Run(ctx, req)
}
```

**Step 5: Add createFromReviewBeads**

```go
func (s *SpecLoop) createFromReviewBeads(ctx context.Context, specID string, reviewRes *stagepkg.Result) {
    if s.adapters.TaskTracker == nil || reviewRes == nil {
        return
    }
    arts, ok := reviewRes.Artifacts.(*specreview.SpecReviewArtifacts)
    if !ok || arts == nil {
        return
    }
    for _, finding := range arts.Findings {
        if finding.Severity == stagepkg.SeverityCritical {
            continue // critical findings were remediated; skip
        }
        labels := []string{"from-review"}
        if finding.Scope == stagepkg.ScopeSpec {
            labels = append(labels, "spec:"+specID)
        }
        _, _ = s.adapters.TaskTracker.CreateBead(ctx, trackertypes.TaskTrackerCreateBeadRequest{
            Title:       fmt.Sprintf("[from-review] %s: %s", finding.Category, shortDescription(finding.Description)),
            Description: finding.Description,
            Priority:    1,
            Labels:      labels,
        })
    }
}

func shortDescription(desc string) string {
    if len(desc) > 80 {
        return desc[:80] + "..."
    }
    return desc
}
```

Note: import `specreview "github.com/danabrams/gromit/internal/v2/stage/specreview"` in spec_loop.go.

**Step 6: Run tests**

Run: `go test ./internal/v2/loop/...`
Expected: All tests pass

**Step 7: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/loop/spec_loop_test.go
git commit -m "feat: spec_loop runs specreview after accept, creates from-review beads on pass-with-improvements"
```

---

### Task 8: Wire specreview into run2_components and RemediationRunner

**Files:**
- Modify: `internal/v2/loop/run2_components.go`
- Modify: `internal/v2/loop/run2_components_test.go` (if it tests component wiring)

**Step 1: Add SpecReviewStage to Run2LoopComponents**

In `run2_components.go`, update `Run2LoopComponents`:

```go
type Run2LoopComponents struct {
    PlanStage             stagepkg.Stage
    PresentStage          stagepkg.Stage
    PresentSummaryContext *present.SummaryContext
    DecomposeStage        stagepkg.Stage
    BeadLoop              *BeadLoop
    AcceptStage           stagepkg.Stage
    SpecReviewStage       stagepkg.Stage // new
    RemediationRunner     remediationRunner
    Emitter               Run2LoopEmitter
    StageCommitter        StageCommitter
    TypedEmitter          *event.Emitter
}
```

**Step 2: Load review_spec_v2.md fragment and create specreview stage**

In `NewRun2LoopComponents()`, after loading `acceptFragment`, add:

```go
specReviewFragment, err := loadFragment(cfg.ProjectRoot, "review_spec_v2.md")
if err != nil {
    cleanup()
    return nil, err
}
```

After creating `acceptStage`, add:

```go
specReviewStage, err := specreviewstage.New(cfg, adapters.Git, adapters.LLM, baseInstructions, projectContext, specReviewFragment)
if err != nil {
    cleanup()
    return nil, err
}
```

Note: `adapters.Git` must implement `specreviewstage.GitDiffer` (needs `DiffFromBase`). Check that the git adapter already has this method — it does, since `accept.go` already calls `DiffFromBase`.

**Step 3: Pass highest-tier model to specreview**

The specreview stage already selects `cfg.Models.P0` internally in `selectModel()`. No additional wiring needed for model selection.

**Step 4: Wire specreview into RemediationRunner**

Update the `remediationRunner` construction:

```go
remediationRunner := v2remediation.NewRemediationRunner(v2remediation.RemediationRunnerConfig{
    AcceptStage:     acceptStage,
    SpecReviewStage: specReviewStage, // add this
    DecomposeStage:  decomposeStage,
    BeadRunner:      &remediationBeadRunner{loop: beadLoop},
    GenerationCap:   v2remediation.DefaultGenerationCap,
    Emitter:         legacyEmitter,
    Presenter:       adapters.Presenter,
})
```

**Step 5: Pass specreview to SpecLoop via option**

In `run2.go` where the spec loop is constructed, add `loop.WithSpecReviewStage(components.SpecReviewStage)`. (Alternatively, this is done in SpecLoop construction in run2.go — see next task.)

Actually, `SpecLoop` construction happens in `run2.go`. We just need to add the `SpecReviewStage` to `Run2LoopComponents` and pass it when constructing the spec loop. Update `run2_components.go` to set:

```go
return &Run2LoopComponents{
    ...
    SpecReviewStage:  specReviewStage,
    ...
}, nil
```

**Step 6: Run tests**

Run: `go test ./internal/v2/loop/...`
Expected: All tests pass

Run: `go test ./...`
Expected: All tests pass

**Step 7: Commit**

```bash
git add internal/v2/loop/run2_components.go
git commit -m "feat: wire specreview stage into run2 components and remediation runner"
```

---

### Task 9: Wire SpecReviewStage into SpecLoop construction in run2.go

**Files:**
- Modify: `cmd/gromit/run2.go` (SpecLoop construction call)

**Step 1: Add WithSpecReviewStage option to SpecLoop construction**

In `run2.go`, find where `NewSpecLoop` is called (via `newSpecLoopFn`) and where spec loop options are assembled. Add `loop.WithSpecReviewStage(components.SpecReviewStage)` to the options list.

Look for the section that builds loop options (search for `WithAcceptStage`, `WithDecomposeStage`, etc.) and add:

```go
loop.WithSpecReviewStage(components.SpecReviewStage),
```

**Step 2: Run build and tests**

Run: `go build ./cmd/gromit/`
Expected: Clean build

Run: `go test ./cmd/gromit/...`
Expected: All tests pass

**Step 3: Commit**

```bash
git add cmd/gromit/run2.go
git commit -m "feat: pass SpecReviewStage option to SpecLoop in run2 command"
```

---

### Task 10: Add --from-review flag to run2

**Files:**
- Modify: `cmd/gromit/run2.go`

**Step 1: Write tests for from-review flag behavior**

Examine existing run2 tests for patterns, then add:

```go
func TestRun2_fromReviewFlag_queriesFromReviewBeads(t *testing.T) {
    // Set --from-review flag
    // Stub task tracker to return beads with "from-review" label
    // Assert: plan stage NOT called, decompose NOT called, accept NOT called
    // Assert: bead loop WAS called with the from-review beads
}

func TestRun2_fromReviewFlag_withSpecScope(t *testing.T) {
    // Set --from-review --spec myspec
    // Stub tracker to return beads with "from-review" AND "spec:myspec" labels
    // Assert bead loop called with only those beads
}
```

**Step 2: Run tests to see them fail**

Run: `go test ./cmd/gromit/...`
Expected: FAIL (flag doesn't exist yet)

**Step 3: Add the flag in init()**

In `run2.go`'s `init()`:

```go
func init() {
    run2Cmd.Flags().String("epic", "", "Run specs scoped to the specified epic")
    run2Cmd.Flags().Bool("from-review", false, "Run only from-review beads (skip plan/decompose/accept/review)")
    run2Cmd.Flags().String("spec", "", "Scope --from-review to a specific spec ID")
}
```

**Step 4: Handle --from-review in run2()**

At the top of `run2()`, after loading config:

```go
fromReview, _ := cmd.Flags().GetBool("from-review")
if fromReview {
    specScope, _ := cmd.Flags().GetString("spec")
    return runFromReview(cmd.Context(), cfg, specScope)
}
```

**Step 5: Implement runFromReview()**

```go
func runFromReview(ctx context.Context, cfg *config.Config, specScope string) error {
    // Build minimal adapters: task tracker + bead loop (no LLM needed for querying)
    gromitDir := resolveGromitDir(cfg)
    ttAdapter, err := tasktracker.NewBdAdapter(gromitDir)
    if err != nil {
        return fmt.Errorf("task tracker: %w", err)
    }

    labels := []string{"from-review"}
    if specScope != "" {
        labels = append(labels, "spec:"+specScope)
    }

    beads, err := ttAdapter.QueryBeads(ctx, trackertypes.TaskTrackerQueryBeadsRequest{
        Labels: labels,
        Status: "open",
    })
    if err != nil {
        return fmt.Errorf("query from-review beads: %w", err)
    }
    if len(beads) == 0 {
        fmt.Println("No open from-review beads found.")
        return nil
    }

    // Build full adapters for bead loop execution.
    // ... (same adapter setup as regular run2, excluding plan/accept/specreview)
    // Run beads through bead loop.
    // No accept, no specreview, no remediation cycle.
    fmt.Printf("Running %d from-review beads...\n", len(beads))
    // ... bead loop execution
    return nil
}
```

Note: The full adapter setup is complex — reuse `NewRun2LoopComponents` but only invoke `components.BeadLoop.Run(ctx, beads, stopCh)` directly, skipping plan/accept/specreview.

**Step 6: Run tests**

Run: `go test ./cmd/gromit/...`
Expected: All tests pass

Run: `go test ./...`
Expected: All tests pass

**Step 7: Run build**

Run: `go build ./cmd/gromit/`
Expected: Clean build

**Step 8: Commit**

```bash
git add cmd/gromit/run2.go
git commit -m "feat: add --from-review flag to run2 for executing from-review beads directly"
```

---

### Task 11: Integration test — full post-bead-loop pipeline

**Files:**
- Create: `internal/v2/loop/spec_loop_specreview_integration_test.go`

**Step 1: Write integration test**

```go
func TestIntegration_SpecLoop_AcceptPassReviewPass_Succeeds(t *testing.T) {
    // Stubs: accept → pass, specreview → pass
    // Run spec loop through the full pipeline
    // Assert: no error, no remediation
}

func TestIntegration_SpecLoop_ReviewFailCritical_TriggersRemediation(t *testing.T) {
    // Stubs: accept → pass, specreview → fail (critical)
    // First remediation pass: accept → pass, specreview → pass
    // Assert: bead loop ran once for remediation beads
}

func TestIntegration_SpecLoop_AcceptFailReviewFail_CombinesFindings(t *testing.T) {
    // Stubs: accept → fail with 1 finding, specreview → fail with 1 critical
    // Assert: decompose receives 2 findings
}

func TestIntegration_SpecLoop_PassWithImprovements_CreatesFromReviewBeads(t *testing.T) {
    // Stubs: accept → pass, specreview → pass with 2 warning findings (1 spec, 1 general)
    // Assert: tracker.CreateBead called twice
    // Assert: spec-scoped bead has "from-review" and "spec:<id>" labels
    // Assert: general bead has only "from-review" label
}
```

**Step 2: Run integration tests**

Run: `go test ./internal/v2/loop/... -run TestIntegration_SpecLoop`
Expected: All pass

**Step 3: Commit**

```bash
git add internal/v2/loop/spec_loop_specreview_integration_test.go
git commit -m "test: integration tests for post-bead-loop specreview pipeline"
```

---

## Architecture Summary

```
SpecLoop.Run()
  ├── plan stage
  ├── decompose stage (default template)
  ├── bead loop (gate → build → validate → review → epilogue)
  └── ensureAcceptance()
        ├── accept stage → AcceptArtifacts{Findings: [...SpecFinding]}
        ├── specreview stage → SpecReviewArtifacts{Verdict, Findings: [...SpecFinding]}
        ├── if both pass && review has non-critical findings:
        │     └── createFromReviewBeads() → tracker.CreateBead("from-review"/"spec:<id>")
        ├── if either has critical findings:
        │     └── remediationRunner.Run()
        │           ├── accept + specreview loop
        │           ├── collectFindings()
        │           └── executeRemediation(findings)
        │                 └── decompose (findingsDecomposePromptTemplate)
        │                       └── bead loop → repeat
        └── on generation cap → fail

run2 --from-review [--spec <id>]
  └── query open beads with "from-review" label → bead loop (no accept/specreview/remediation)
```

## Key Invariants

1. **specreview always uses P0 model** — hardcoded in `selectModel()`, not routed
2. **critical findings always force fail verdict** — enforced in `ParseSpecReviewOutput()`, not trusted from LLM
3. **findings-based decompose fires when `req.Findings` non-empty** — takes priority over `req.Remediation`+gap-analysis
4. **from-review beads never trigger accept/specreview** — `runFromReview` calls bead loop directly
5. **GetFindings() is nil-safe** — `AcceptArtifacts.GetFindings()` and `SpecReviewArtifacts.GetFindings()` return nil on nil receiver
