# Spec 0002b Review & Acceptance — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend the Spec 0002a execution loop with LLM-driven multi-facet code review, per-criterion acceptance evaluation, fix-cycle replanning from review/acceptance failures, and configurable severity thresholds.

**Architecture:** Two new domain packages (`internal/next/review/`, `internal/next/acceptor/`), two new stage wrappers (`internal/next/specloop/stages/review.go`, `internal/next/specloop/stages/accept.go`), extensions to `execpolicy`, `evidence`, and CLI wiring. Stage wrappers are thin adapters; domain logic lives in the domain packages.

**Tech Stack:** Go 1.24, cobra CLI, existing provider/specloop/evidence packages.

**Module:** `github.com/danabrams/gromit`

**Existing packages used:**
- `internal/provider` — Provider interface, tier constants (`TierXHigh`, `TierHigh`, `TierMedium`, `TierLow`)
- `internal/next/specloop` — Stage interface, NextAction, FailureContext, ActionKind constants
- `internal/next/specloop/stages` — ValidateStage (pattern for ReviewStage/AcceptStage)
- `internal/next/execpolicy` — Policy, Models, Budgets, Check types
- `internal/next/evidence` — Bundler, ReviewInput, Metrics
- `internal/next/runstore` — RunState, Task, Store, EventLog
- `internal/next/workspace` — Root, EnvResolver for `~/.local/share/gromit/`

**Pipeline after this spec:**
```
Init -> Compile -> Plan -> Execute -> Validate -> Review -> Accept -> Evidence -> Finalize
                   ^                    |           |        |
                   |____________________|___________|________|
                    failures loop back to Plan
```

---

## Phase 1: Bug Fixes from 0002a Manual Testing

### Task 1: Fix `exec list` exit code on empty results

- **Files:**
  - Modify: `cmd/gromit-next/exec_list.go`
  - Modify: `cmd/gromit-next/exec_test.go`

- **Step 1: Write failing test** in `exec_test.go`:
```go
func TestExecList_EmptyResults_ExitCodeZero(t *testing.T) {
	tmpDir := t.TempDir()
	store := runstore.NewStore(tmpDir)

	output, err := execList("nonexistent-project", store)
	if err != nil {
		t.Fatalf("execList returned error for empty results: %v", err)
	}
	// Should contain header but no data rows
	if !strings.Contains(output, "RUN ID") {
		t.Errorf("expected header row, got: %s", output)
	}
}
```

- **Step 2:** `go test ./cmd/gromit-next/ -run TestExecList_EmptyResults -v` — expect FAIL

- **Step 3: Implement** — Modify `execList` in `exec_list.go` to return the header-only table (no error) when `store.List` returns an empty slice. If `store.List` returns an error that indicates no runs directory, treat it as empty rather than propagating the error.

- **Step 4:** `go test ./cmd/gromit-next/ -run TestExecList_EmptyResults -v` — expect PASS
- **Step 5:** Commit `"fix(next): exec list returns exit 0 on empty results (Bug 3)"`

---

### Task 2: Fix `spec list` path resolution from `--project` flag

- **Files:**
  - Modify: `cmd/gromit-next/spec.go`
  - Modify or create: `cmd/gromit-next/spec_test.go`

- **Step 1: Write failing test** in `spec_test.go`:
```go
func TestSpecList_ResolvesProjectCellPath(t *testing.T) {
	// Set up a fake workspace root with a project cell containing project.json
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "projects", "myapp")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	specsDir := filepath.Join(projectDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]string{"specs_dir": specsDir}
	cfgData, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(projectDir, "project.json"), cfgData, 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a dummy spec
	if err := os.WriteFile(filepath.Join(specsDir, "my-spec.md"), []byte("# My Spec"), 0o644); err != nil {
		t.Fatal(err)
	}

	// LoadProjectConfig should resolve from workspace root + project name
	resolved, err := ResolveProjectConfigPath(workspace.Root(tmpDir), "myapp")
	if err != nil {
		t.Fatalf("ResolveProjectConfigPath failed: %v", err)
	}
	if resolved != projectDir {
		t.Errorf("got %q, want %q", resolved, projectDir)
	}
}
```

- **Step 2:** `go test ./cmd/gromit-next/ -run TestSpecList_ResolvesProjectCellPath -v` — expect FAIL

- **Step 3: Implement** — Add `ResolveProjectConfigPath(root workspace.Root, projectName string) (string, error)` that builds the path as `root.ProjectCell(projectName)`. Update `newSpecListCmd` to resolve the workspace root via `workspace.NewEnvResolver()` when `--specs-dir` is not explicitly provided, then load `project.json` from the resolved project cell path instead of `"."`.

- **Step 4:** `go test ./cmd/gromit-next/ -run TestSpecList_ResolvesProjectCellPath -v` — expect PASS
- **Step 5:** Commit `"fix(next): spec list resolves project.json from workspace root (Bug 2)"`

---

### Task 3: Fix agent provider wiring in `exec spec`

- **Files:**
  - Modify: `cmd/gromit-next/exec.go`
  - Create: `cmd/gromit-next/stage_provider.go`
  - Modify or create: `cmd/gromit-next/stage_provider_test.go`

- **Step 1: Write failing test** in `stage_provider_test.go`:
```go
package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestRealStageProvider_BuildStages_ReturnsStages(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	stages, err := provider.BuildStages(policy, rs)
	if err != nil {
		t.Fatalf("BuildStages returned error: %v", err)
	}
	if len(stages) == 0 {
		t.Fatal("expected at least one stage, got 0")
	}

	// Verify expected stage names in order
	expectedNames := []string{"init", "compile", "plan", "execute", "validate", "evidence", "finalize"}
	for i, name := range expectedNames {
		if i >= len(stages) {
			t.Fatalf("missing stage %q at index %d", name, i)
		}
		if stages[i].Name() != name {
			t.Errorf("stage[%d].Name() = %q, want %q", i, stages[i].Name(), name)
		}
	}
}

func TestRealStageProvider_BuildStages_NoStubError(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	_, err := provider.BuildStages(policy, rs)
	if err != nil {
		t.Fatalf("BuildStages should not return stub error, got: %v", err)
	}
}
```

- **Step 2:** `go test ./cmd/gromit-next/ -run TestRealStageProvider -v` — expect FAIL

- **Step 3: Implement** — Create `RealStageProvider` struct in `stage_provider.go` that implements `StageProvider`. `BuildStages` instantiates real stages: `InitStage`, `CompileStage`, `PlanStage`, `ExecuteStage`, `ValidateStage`, `EvidenceStage`, `FinalizeStage` using their constructors from `specloop/stages`. For stages that require an Agent (Plan, Execute), use a stub/noop agent initially — the real LLM wiring is an integration concern. Update `newExecSpecCmd` to use `NewRealStageProvider` instead of `defaultStageProvider`. Remove the stub `defaultStageProvider`.

- **Step 4:** `go test ./cmd/gromit-next/ -run TestRealStageProvider -v` — expect PASS
- **Step 5:** Commit `"fix(next): replace stub StageProvider with real stage wiring (Bug 1)"`

---

## Phase 2: `review/` Package — Facet Registry, Finding Types, Severity, Threshold

### Task 4: Severity type and ordering

- **Files:**
  - Create: `internal/next/review/severity.go`
  - Create: `internal/next/review/severity_test.go`

- **Step 1: Write failing test** in `severity_test.go`:
```go
package review

import "testing"

func TestSeverity_Ordering(t *testing.T) {
	if SeverityError.Rank() <= SeverityWarning.Rank() {
		t.Error("error should rank higher than warning")
	}
	if SeverityWarning.Rank() <= SeveritySuggestion.Rank() {
		t.Error("warning should rank higher than suggestion")
	}
	if SeveritySuggestion.Rank() <= SeverityInfo.Rank() {
		t.Error("suggestion should rank higher than info")
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		sev  Severity
		want string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
		{SeveritySuggestion, "suggestion"},
		{SeverityInfo, "info"},
	}
	for _, tt := range tests {
		if got := tt.sev.String(); got != tt.want {
			t.Errorf("Severity.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestParseSeverity_Valid(t *testing.T) {
	for _, name := range []string{"error", "warning", "suggestion", "info"} {
		sev, err := ParseSeverity(name)
		if err != nil {
			t.Errorf("ParseSeverity(%q) error: %v", name, err)
		}
		if sev.String() != name {
			t.Errorf("ParseSeverity(%q).String() = %q", name, sev.String())
		}
	}
}

func TestParseSeverity_Invalid(t *testing.T) {
	_, err := ParseSeverity("critical")
	if err == nil {
		t.Error("ParseSeverity(\"critical\") should return error")
	}
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestSeverity -v` — expect FAIL

- **Step 3: Implement** — Define `Severity` type (int) with constants `SeverityError`, `SeverityWarning`, `SeveritySuggestion`, `SeverityInfo`. Add `Rank() int` method (error=4, warning=3, suggestion=2, info=1), `String() string`, and `ParseSeverity(s string) (Severity, error)`.

- **Step 4:** `go test ./internal/next/review/ -run TestSeverity -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review severity type with ordering and parsing"`

---

### Task 5: Finding type

- **Files:**
  - Create: `internal/next/review/finding.go`
  - Create: `internal/next/review/finding_test.go`

- **Step 1: Write failing test** in `finding_test.go`:
```go
package review

import (
	"encoding/json"
	"testing"
)

func TestFinding_JSONRoundTrip(t *testing.T) {
	f := Finding{
		Facet:        "code_quality",
		Severity:     SeverityWarning,
		File:         "internal/handler.go",
		Line:         42,
		Description:  "nil pointer if commands list is empty",
		SuggestedFix: "add empty check before iteration",
		Cycle:        1,
		Disposition:  DispositionNew,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Finding
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Facet != f.Facet {
		t.Errorf("Facet = %q, want %q", got.Facet, f.Facet)
	}
	if got.Severity != f.Severity {
		t.Errorf("Severity = %v, want %v", got.Severity, f.Severity)
	}
	if got.File != f.File {
		t.Errorf("File = %q, want %q", got.File, f.File)
	}
	if got.Line != f.Line {
		t.Errorf("Line = %d, want %d", got.Line, f.Line)
	}
	if got.Disposition != DispositionNew {
		t.Errorf("Disposition = %q, want %q", got.Disposition, DispositionNew)
	}
}

func TestFinding_NormalizeNilFields(t *testing.T) {
	f := Finding{}
	f.NormalizeNilFields()
	// No slices to normalize in Finding, but method should exist for convention
}

func TestFindingSet_NormalizeNilFields(t *testing.T) {
	fs := FindingSet{}
	fs.NormalizeNilFields()
	if fs.Findings == nil {
		t.Error("NormalizeNilFields should set nil Findings to empty slice")
	}
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestFinding -v` — expect FAIL

- **Step 3: Implement** — Define `Finding` struct with fields: `Facet string`, `Severity Severity`, `File string`, `Line int`, `Description string`, `SuggestedFix string`, `Cycle int`, `Disposition string`. Define `DispositionNew = "new"` and `DispositionPreExisting = "pre-existing"` constants. Define `FindingSet` struct with `Facet string` and `Findings []Finding`. Add `NormalizeNilFields()` on both types. Implement custom JSON marshal/unmarshal for `Severity` to use string representation.

- **Step 4:** `go test ./internal/next/review/ -run TestFinding -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review Finding and FindingSet types with JSON support"`

---

### Task 6: Threshold logic — blocking determination

- **Files:**
  - Create: `internal/next/review/threshold.go`
  - Create: `internal/next/review/threshold_test.go`

- **Step 1: Write failing test** in `threshold_test.go`:
```go
package review

import "testing"

func TestThreshold_IsBlocking(t *testing.T) {
	tests := []struct {
		name      string
		threshold Severity
		finding   Severity
		want      bool
	}{
		{"error blocks at error threshold", SeverityError, SeverityError, true},
		{"warning does not block at error threshold", SeverityError, SeverityWarning, false},
		{"suggestion does not block at error threshold", SeverityError, SeveritySuggestion, false},
		{"info never blocks", SeverityError, SeverityInfo, false},
		{"warning blocks at warning threshold", SeverityWarning, SeverityWarning, true},
		{"error blocks at warning threshold", SeverityWarning, SeverityError, true},
		{"suggestion does not block at warning threshold", SeverityWarning, SeveritySuggestion, false},
		{"suggestion blocks at suggestion threshold", SeveritySuggestion, SeveritySuggestion, true},
		{"error blocks at suggestion threshold", SeveritySuggestion, SeverityError, true},
		{"info never blocks at any threshold", SeveritySuggestion, SeverityInfo, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBlocking(tt.threshold, tt.finding)
			if got != tt.want {
				t.Errorf("IsBlocking(%v, %v) = %v, want %v", tt.threshold, tt.finding, got, tt.want)
			}
		})
	}
}

func TestFilterBlockingFindings(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityError, Description: "error finding"},
		{Severity: SeverityWarning, Description: "warning finding"},
		{Severity: SeveritySuggestion, Description: "suggestion finding"},
		{Severity: SeverityInfo, Description: "info finding"},
	}

	blocking := FilterBlockingFindings(findings, SeverityWarning)
	if len(blocking) != 2 {
		t.Fatalf("expected 2 blocking findings at warning threshold, got %d", len(blocking))
	}
	if blocking[0].Description != "error finding" {
		t.Errorf("first blocking should be error, got %q", blocking[0].Description)
	}
	if blocking[1].Description != "warning finding" {
		t.Errorf("second blocking should be warning, got %q", blocking[1].Description)
	}
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestThreshold -v` — expect FAIL

- **Step 3: Implement** — `IsBlocking(threshold, findingSeverity Severity) bool` returns true if the finding's rank is >= the threshold's rank AND the finding is not `SeverityInfo` (info never blocks). `FilterBlockingFindings(findings []Finding, threshold Severity) []Finding` returns the subset of findings where `IsBlocking` is true.

- **Step 4:** `go test ./internal/next/review/ -run TestThreshold -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review threshold logic for blocking determination"`

---

### Task 7: Facet registry — built-in facet definitions

- **Files:**
  - Create: `internal/next/review/facet.go`
  - Create: `internal/next/review/facet_test.go`

- **Step 1: Write failing test** in `facet_test.go`:
```go
package review

import "testing"

func TestRegistry_DefaultFacets(t *testing.T) {
	reg := NewRegistry()

	// Must have the two default facets
	sa, ok := reg.Get("spec_alignment")
	if !ok {
		t.Fatal("registry missing spec_alignment")
	}
	if sa.DefaultTier != "high" {
		t.Errorf("spec_alignment.DefaultTier = %q, want %q", sa.DefaultTier, "high")
	}

	cq, ok := reg.Get("code_quality")
	if !ok {
		t.Fatal("registry missing code_quality")
	}
	if cq.DefaultTier != "medium" {
		t.Errorf("code_quality.DefaultTier = %q, want %q", cq.DefaultTier, "medium")
	}
}

func TestRegistry_AdditionalFacets(t *testing.T) {
	reg := NewRegistry()
	for _, name := range []string{"logic_gaps", "test_coverage", "architecture_drift"} {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("registry missing additional facet %q", name)
		}
	}
}

func TestRegistry_ListNames(t *testing.T) {
	reg := NewRegistry()
	names := reg.ListNames()
	if len(names) < 5 {
		t.Errorf("expected at least 5 facets, got %d", len(names))
	}
}

func TestRegistry_UnknownFacet(t *testing.T) {
	reg := NewRegistry()
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("Get(\"nonexistent\") should return false")
	}
}

func TestFacetDef_HasPromptTemplate(t *testing.T) {
	reg := NewRegistry()
	for _, name := range reg.ListNames() {
		facet, _ := reg.Get(name)
		if facet.PromptTemplate == "" {
			t.Errorf("facet %q has empty PromptTemplate", name)
		}
	}
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestRegistry -v` — expect FAIL

- **Step 3: Implement** — Define `FacetDef` struct with `Name string`, `Description string`, `DefaultTier string`, `PromptTemplate string`. Define `Registry` struct holding a `map[string]FacetDef`. `NewRegistry()` populates it with 5 built-in facets: `spec_alignment` (high), `code_quality` (medium), `logic_gaps` (high), `test_coverage` (medium), `architecture_drift` (medium). Each has a prompt template string that instructs the LLM what to review. Add `Get(name string) (FacetDef, bool)` and `ListNames() []string` (sorted).

- **Step 4:** `go test ./internal/next/review/ -run TestRegistry -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review facet registry with 5 built-in facets"`

---

### Task 8: Prompt template rendering for facets

- **Files:**
  - Create: `internal/next/review/prompt.go`
  - Create: `internal/next/review/prompt_test.go`

- **Step 1: Write failing test** in `prompt_test.go`:
```go
package review

import "testing"

func TestRenderReviewPrompt_ContainsFacetName(t *testing.T) {
	reg := NewRegistry()
	facet, _ := reg.Get("spec_alignment")

	input := ReviewPromptInput{
		FacetDef:    facet,
		DiffSummary: "Added refund handler in internal/handler/refund.go",
		SpecContent: "## Acceptance Criteria\n- refund endpoint returns 200",
	}

	prompt, err := RenderReviewPrompt(input)
	if err != nil {
		t.Fatalf("RenderReviewPrompt: %v", err)
	}
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	if !containsSubstring(prompt, "spec_alignment") {
		t.Error("prompt should contain facet name")
	}
	if !containsSubstring(prompt, "refund handler") {
		t.Error("prompt should contain diff summary")
	}
}

func TestRenderReviewPrompt_IncludesPriorFindings(t *testing.T) {
	reg := NewRegistry()
	facet, _ := reg.Get("code_quality")

	input := ReviewPromptInput{
		FacetDef:      facet,
		DiffSummary:   "Modified handler.go",
		PriorFindings: []Finding{{Severity: SeverityWarning, File: "handler.go", Description: "duplicate logic"}},
	}

	prompt, err := RenderReviewPrompt(input)
	if err != nil {
		t.Fatalf("RenderReviewPrompt: %v", err)
	}
	if !containsSubstring(prompt, "duplicate logic") {
		t.Error("prompt should include prior finding descriptions for disposition labeling")
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAny(s, sub))
}

func containsAny(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestRenderReviewPrompt -v` — expect FAIL

- **Step 3: Implement** — Define `ReviewPromptInput` struct with `FacetDef`, `DiffSummary string`, `SpecContent string`, `PriorFindings []Finding`. `RenderReviewPrompt(input ReviewPromptInput) (string, error)` uses `text/template` to render the facet's `PromptTemplate` with the input data. The template includes a section for prior findings when `PriorFindings` is non-empty, instructing the LLM to label each current finding as new or pre-existing.

- **Step 4:** `go test ./internal/next/review/ -run TestRenderReviewPrompt -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review prompt template rendering"`

---

### Task 9: New-vs-preexisting finding matching

- **Files:**
  - Create: `internal/next/review/matching.go`
  - Create: `internal/next/review/matching_test.go`

- **Step 1: Write failing test** in `matching_test.go`:
```go
package review

import "testing"

func TestMatchFindings_ExactMatch(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}

	labeled := LabelDispositions(current, prior)
	if len(labeled) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(labeled))
	}
	if labeled[0].Disposition != DispositionPreExisting {
		t.Errorf("expected pre-existing, got %q", labeled[0].Disposition)
	}
}

func TestMatchFindings_SubstringMatch(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty; line shifted to 45"},
	}

	labeled := LabelDispositions(current, prior)
	if labeled[0].Disposition != DispositionPreExisting {
		t.Errorf("substring match should yield pre-existing, got %q", labeled[0].Disposition)
	}
}

func TestMatchFindings_DifferentFile_IsNew(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "router.go", Description: "nil pointer if commands list is empty"},
	}

	labeled := LabelDispositions(current, prior)
	if labeled[0].Disposition != DispositionNew {
		t.Errorf("different file should be new, got %q", labeled[0].Disposition)
	}
}

func TestMatchFindings_DifferentDescription_IsNew(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "handler.go", Description: "duplicated logic in refund calculation"},
	}

	labeled := LabelDispositions(current, prior)
	if labeled[0].Disposition != DispositionNew {
		t.Errorf("different description should be new, got %q", labeled[0].Disposition)
	}
}

func TestMatchFindings_NoPrior_AllNew(t *testing.T) {
	current := []Finding{
		{File: "handler.go", Description: "missing error check"},
		{File: "router.go", Description: "unused variable"},
	}

	labeled := LabelDispositions(current, nil)
	for i, f := range labeled {
		if f.Disposition != DispositionNew {
			t.Errorf("finding[%d]: expected new with no prior, got %q", i, f.Disposition)
		}
	}
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestMatchFindings -v` — expect FAIL

- **Step 3: Implement** — `LabelDispositions(current, prior []Finding) []Finding` returns a copy of `current` with `Disposition` set. Matching strategy (v1): a current finding matches a prior finding if they share the same `File` path AND either the prior `Description` is a substring of the current `Description` or vice versa. Matched findings get `DispositionPreExisting`; unmatched get `DispositionNew`.

- **Step 4:** `go test ./internal/next/review/ -run TestMatchFindings -v` — expect PASS
- **Step 5:** Commit `"feat(next): add new-vs-preexisting finding matching logic"`

---

### Task 10: Review runner — orchestrates per-facet review

- **Files:**
  - Create: `internal/next/review/runner.go`
  - Create: `internal/next/review/runner_test.go`

- **Step 1: Write failing test** in `runner_test.go`:
```go
package review

import (
	"context"
	"testing"
)

// mockReviewAgent is a test double for the LLM review agent.
type mockReviewAgent struct {
	findings map[string][]Finding // facet name -> findings
}

func (m *mockReviewAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
	return m.findings[facetName], nil
}

func TestRunner_RunAllFacets(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"spec_alignment": {{Severity: SeverityError, File: "handler.go", Description: "missing validation"}},
			"code_quality":   {},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"spec_alignment", "code_quality"},
		Threshold: SeveritySuggestion,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(result.AllFindings) != 1 {
		t.Fatalf("expected 1 finding total, got %d", len(result.AllFindings))
	}
	if len(result.BlockingFindings) != 1 {
		t.Fatalf("expected 1 blocking finding, got %d", len(result.BlockingFindings))
	}
	if !result.HasBlockingFindings {
		t.Error("HasBlockingFindings should be true")
	}
}

func TestRunner_InfoFindingsNeverBlock(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"code_quality": {{Severity: SeverityInfo, File: "handler.go", Description: "consider extracting helper"}},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"code_quality"},
		Threshold: SeveritySuggestion,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.HasBlockingFindings {
		t.Error("info findings should not block")
	}
	if len(result.AllFindings) != 1 {
		t.Error("info finding should still appear in AllFindings")
	}
}

func TestRunner_FixCycle_LabelsDispositions(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"spec_alignment": {{Severity: SeverityWarning, File: "handler.go", Description: "missing validation"}},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"spec_alignment"},
		Threshold: SeveritySuggestion,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Fixed handler",
		SpecContent: "# Spec",
		Cycle:       2,
		PriorFindings: []Finding{
			{Severity: SeverityWarning, File: "handler.go", Description: "missing validation"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The finding matches a prior finding, so it should be pre-existing
	if result.AllFindings[0].Disposition != DispositionPreExisting {
		t.Errorf("expected pre-existing disposition, got %q", result.AllFindings[0].Disposition)
	}
	// Pre-existing findings should NOT block
	if result.HasBlockingFindings {
		t.Error("pre-existing findings should not trigger blocking")
	}
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestRunner -v` — expect FAIL

- **Step 3: Implement** — Define `ReviewAgent` interface with `ReviewFacet(ctx, facetName, prompt string) ([]Finding, error)`. Define `RunnerConfig` with `Facets []string`, `Threshold Severity`, `FacetTiers map[string]string`. Define `RunInput` with `DiffSummary`, `SpecContent`, `Cycle int`, `PriorFindings []Finding`. Define `RunResult` with `AllFindings []Finding`, `BlockingFindings []Finding`, `HasBlockingFindings bool`, `FindingsByFacet map[string][]Finding`. `Runner.Run` iterates enabled facets, renders prompts, calls the agent, labels dispositions on fix cycles (cycle > 1), filters for blocking (only new findings above threshold), and assembles the result.

- **Step 4:** `go test ./internal/next/review/ -run TestRunner -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review runner with per-facet orchestration and fix-cycle support"`

---

## Phase 3: `acceptor/` Package — Criterion Evaluation

### Task 11: Criterion and evaluation result types

- **Files:**
  - Create: `internal/next/acceptor/types.go`
  - Create: `internal/next/acceptor/types_test.go`

- **Step 1: Write failing test** in `types_test.go`:
```go
package acceptor

import (
	"encoding/json"
	"testing"
)

func TestCriterionResult_JSONRoundTrip(t *testing.T) {
	cr := CriterionResult{
		Criterion:    "Zero repo pollution",
		Status:       StatusPass,
		Rationale:    "No gromit files found in target repo.",
		EvidenceRefs: []string{"evidence/diff-summary.md", "evidence/worktree-info.json"},
	}

	data, err := json.Marshal(cr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got CriterionResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Status != StatusPass {
		t.Errorf("Status = %q, want %q", got.Status, StatusPass)
	}
	if len(got.EvidenceRefs) != 2 {
		t.Errorf("EvidenceRefs len = %d, want 2", len(got.EvidenceRefs))
	}
}

func TestCriterionResult_NormalizeNilFields(t *testing.T) {
	cr := CriterionResult{}
	cr.NormalizeNilFields()
	if cr.EvidenceRefs == nil {
		t.Error("NormalizeNilFields should set nil EvidenceRefs to empty slice")
	}
}

func TestAcceptanceResult_NormalizeNilFields(t *testing.T) {
	ar := AcceptanceResult{}
	ar.NormalizeNilFields()
	if ar.Results == nil {
		t.Error("NormalizeNilFields should set nil Results to empty slice")
	}
}

func TestStatus_Constants(t *testing.T) {
	if StatusPass != "pass" {
		t.Errorf("StatusPass = %q, want %q", StatusPass, "pass")
	}
	if StatusFail != "fail" {
		t.Errorf("StatusFail = %q, want %q", StatusFail, "fail")
	}
	if StatusUnclear != "unclear" {
		t.Errorf("StatusUnclear = %q, want %q", StatusUnclear, "unclear")
	}
}
```

- **Step 2:** `go test ./internal/next/acceptor/ -run TestCriterionResult -v` — expect FAIL

- **Step 3: Implement** — Define status constants `StatusPass = "pass"`, `StatusFail = "fail"`, `StatusUnclear = "unclear"`. Define `CriterionResult` struct with `Criterion string`, `Status string`, `Rationale string`, `EvidenceRefs []string`. Define `AcceptanceResult` struct with:
  - `Results []CriterionResult` (field name `Results`, not `Criteria`)
  - `AllPass bool`
  - `HasFailOrUnclear bool`
  - JSON serialization: `{"results": [...], "all_pass": true, "has_fail_or_unclear": false}`
  Add `NormalizeNilFields()` on both types.

- **Step 4:** `go test ./internal/next/acceptor/ -run TestCriterionResult -v` — expect PASS
- **Step 5:** Commit `"feat(next): add acceptor types for criterion evaluation results"`

---

### Task 12: Acceptance evaluator — per-criterion LLM evaluation

- **Files:**
  - Create: `internal/next/acceptor/evaluator.go`
  - Create: `internal/next/acceptor/evaluator_test.go`

- **Step 1: Write failing test** in `evaluator_test.go`:
```go
package acceptor

import (
	"context"
	"testing"
)

type mockAcceptAgent struct {
	results map[string]CriterionResult
}

func (m *mockAcceptAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	// Extract criterion from prompt (simplified: return based on first match)
	for criterion, result := range m.results {
		if containsSubstring(prompt, criterion) {
			return result, nil
		}
	}
	return CriterionResult{Status: StatusPass, Rationale: "default pass"}, nil
}

func TestEvaluator_AllPass(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"returns 200":  {Criterion: "returns 200", Status: StatusPass, Rationale: "test proves it"},
			"handles errors": {Criterion: "handles errors", Status: StatusPass, Rationale: "error tests exist"},
		},
	}

	eval := NewEvaluator(agent)
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"returns 200", "handles errors"},
		DiffSummary: "Added handler",
		TaskResults: "all passed",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.AllPass {
		t.Error("expected AllPass=true")
	}
	if result.HasFailOrUnclear {
		t.Error("expected HasFailOrUnclear=false")
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
}

func TestEvaluator_FailTriggersReplan(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"multi-currency": {Criterion: "multi-currency", Status: StatusFail, Rationale: "only USD implemented"},
		},
	}

	eval := NewEvaluator(agent)
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"multi-currency"},
		DiffSummary: "Added handler",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.AllPass {
		t.Error("expected AllPass=false")
	}
	if !result.HasFailOrUnclear {
		t.Error("expected HasFailOrUnclear=true")
	}
}

func TestEvaluator_UnclearTriggersReplan(t *testing.T) {
	agent := &mockAcceptAgent{
		results: map[string]CriterionResult{
			"audit log": {Criterion: "audit log", Status: StatusUnclear, Rationale: "no test verifies audit call"},
		},
	}

	eval := NewEvaluator(agent)
	result, err := eval.Evaluate(context.Background(), EvaluateInput{
		Criteria:    []string{"audit log"},
		DiffSummary: "Added handler",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.HasFailOrUnclear {
		t.Error("unclear should set HasFailOrUnclear=true")
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- **Step 2:** `go test ./internal/next/acceptor/ -run TestEvaluator -v` — expect FAIL

- **Step 3: Implement** — Define `AcceptAgent` interface with `EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error)`. Define `EvaluateInput` struct with `Criteria []string`, `DiffSummary string`, `TaskResults string`, `ValidationResults string`, `ReviewFindings string`. `Evaluator` struct wraps an `AcceptAgent`. `Evaluate` iterates each criterion, renders a prompt containing the criterion text plus all context (diff, tasks, validation, review findings), calls the agent, and assembles `AcceptanceResult`. Sets `AllPass` and `HasFailOrUnclear` flags.

- **Step 4:** `go test ./internal/next/acceptor/ -run TestEvaluator -v` — expect PASS
- **Step 5:** Commit `"feat(next): add acceptance evaluator with per-criterion LLM evaluation"`

---

### Task 13: Acceptance prompt rendering

- **Files:**
  - Create: `internal/next/acceptor/prompt.go`
  - Create: `internal/next/acceptor/prompt_test.go`

- **Step 1: Write failing test** in `prompt_test.go`:
```go
package acceptor

import "testing"

func TestRenderAcceptancePrompt_ContainsCriterion(t *testing.T) {
	prompt, err := RenderAcceptancePrompt(AcceptancePromptInput{
		Criterion:         "refund endpoint returns 200",
		DiffSummary:       "Added refund handler",
		TaskResults:       "4/4 tasks passed",
		ValidationResults: "6/6 checks passed",
		ReviewFindings:    "0 findings",
	})
	if err != nil {
		t.Fatalf("RenderAcceptancePrompt: %v", err)
	}
	if !containsSubstring(prompt, "refund endpoint returns 200") {
		t.Error("prompt should contain the criterion text")
	}
	if !containsSubstring(prompt, "pass") && !containsSubstring(prompt, "fail") && !containsSubstring(prompt, "unclear") {
		t.Error("prompt should mention the three possible statuses")
	}
}

func TestRenderAcceptancePrompt_UnclearGuidance(t *testing.T) {
	prompt, err := RenderAcceptancePrompt(AcceptancePromptInput{
		Criterion:   "audit log entry created",
		DiffSummary: "Added handler",
	})
	if err != nil {
		t.Fatalf("RenderAcceptancePrompt: %v", err)
	}
	// Prompt should instruct the LLM about "unclear" meaning
	if !containsSubstring(prompt, "unclear") {
		t.Error("prompt should explain the unclear status")
	}
}
```

- **Step 2:** `go test ./internal/next/acceptor/ -run TestRenderAcceptancePrompt -v` — expect FAIL

- **Step 3: Implement** — Define `AcceptancePromptInput` struct with `Criterion string`, `DiffSummary string`, `TaskResults string`, `ValidationResults string`, `ReviewFindings string`. `RenderAcceptancePrompt(input AcceptancePromptInput) (string, error)` uses `text/template` to produce a prompt that asks the LLM to evaluate the criterion against the evidence and respond with pass/fail/unclear, rationale, and evidence references.

- **Step 4:** `go test ./internal/next/acceptor/ -run TestRenderAcceptancePrompt -v` — expect PASS
- **Step 5:** Commit `"feat(next): add acceptance prompt rendering"`

---

### Task 14: Acceptance failure context for planner

- **Files:**
  - Modify: `internal/next/acceptor/types.go`
  - Create: `internal/next/acceptor/failctx.go`
  - Create: `internal/next/acceptor/failctx_test.go`

- **Step 1: Write failing test** in `failctx_test.go`:
```go
package acceptor

import "testing"

func TestBuildFailureContext_FailAndUnclear(t *testing.T) {
	results := AcceptanceResult{
		Results: []CriterionResult{
			{Criterion: "returns 200", Status: StatusPass, Rationale: "test proves it"},
			{Criterion: "multi-currency", Status: StatusFail, Rationale: "only USD"},
			{Criterion: "audit log", Status: StatusUnclear, Rationale: "no test"},
		},
	}

	failures := BuildFailureContext(results, 2)
	if len(failures) != 2 {
		t.Fatalf("expected 2 failure contexts, got %d", len(failures))
	}
	if failures[0].Criterion != "multi-currency" {
		t.Errorf("first failure should be multi-currency, got %q", failures[0].Criterion)
	}
	if failures[0].Status != StatusFail {
		t.Errorf("first failure status = %q, want %q", failures[0].Status, StatusFail)
	}
}

func TestBuildFailureContext_AllPass_Empty(t *testing.T) {
	results := AcceptanceResult{
		Results: []CriterionResult{
			{Criterion: "returns 200", Status: StatusPass},
		},
	}

	failures := BuildFailureContext(results, 1)
	if len(failures) != 0 {
		t.Errorf("expected 0 failures for all-pass, got %d", len(failures))
	}
}

func TestAcceptanceFailuresToStrings_FailAndUnclear(t *testing.T) {
	results := []CriterionResult{
		{Criterion: "multi-currency", Status: StatusFail, Rationale: "only USD"},
		{Criterion: "audit log", Status: StatusUnclear, Rationale: "no test"},
		{Criterion: "returns 200", Status: StatusPass, Rationale: "test proves it"},
	}
	strs := AcceptanceFailuresToStrings(results)
	if len(strs) != 2 {
		t.Fatalf("expected 2 strings (skip pass), got %d", len(strs))
	}
	// fail format: "acceptance:fail: <criterion> — implement missing behavior"
	if !containsSubstring(strs[0], "acceptance:fail:") || !containsSubstring(strs[0], "multi-currency") {
		t.Errorf("fail string format wrong: %q", strs[0])
	}
	// unclear format: "acceptance:unclear: <criterion> — add tests or evidence to prove/disprove"
	if !containsSubstring(strs[1], "acceptance:unclear:") || !containsSubstring(strs[1], "audit log") {
		t.Errorf("unclear string format wrong: %q", strs[1])
	}
}
```

- **Step 2:** `go test ./internal/next/acceptor/ -run TestBuildFailureContext -v` — expect FAIL

- **Step 3: Implement** — Define `AcceptanceFailure` struct with `Criterion string`, `Status string`, `Rationale string`, `EvidenceRefs []string`. `BuildFailureContext(result AcceptanceResult, cycle int) []AcceptanceFailure` filters for non-pass results. `AcceptanceFailuresToStrings(results []CriterionResult) []string` converts each non-pass result to a human-readable string suitable for `FailureContext.Failures` with fail/unclear differentiation:
  - fail: `"acceptance:fail: <criterion> — implement missing behavior"`
  - unclear: `"acceptance:unclear: <criterion> — add tests or evidence to prove/disprove"`

  Also add `ReviewFailuresToStrings(findings []Finding) []string` helper (in `review/failctx.go` — see Task 19) that converts review findings to `FailureContext.Failures` strings. Note: `FailureContext.Failures` remains `[]string` — these helpers produce the string representations that populate it.

- **Step 4:** `go test ./internal/next/acceptor/ -run TestBuildFailureContext -v` — expect PASS
- **Step 5:** Commit `"feat(next): add acceptance failure context for planner replanning"`

---

## Phase 4: ReviewStage and AcceptStage — Stage Implementations

**Package layout:** Stage wrappers live in `internal/next/specloop/stages/review.go` and `internal/next/specloop/stages/accept.go`. These are thin adapters that call into the domain logic packages: `internal/next/review/` (facet registry, findings, matching, threshold, runner) and `internal/next/acceptor/` (criterion types, evaluator, prompt rendering). The stages handle RunState mutation and NextAction decisions; the domain packages handle the actual review/acceptance logic.

### Task 15: ReviewStage implementation

- **Files:**
  - Create: `internal/next/specloop/stages/review.go`
  - Create: `internal/next/specloop/stages/review_test.go`

- **Step 1: Write failing test** in `review_test.go`:
```go
package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type mockReviewRunner struct {
	result review.RunResult
	err    error
}

func (m *mockReviewRunner) Run(ctx context.Context, input review.RunInput) (review.RunResult, error) {
	return m.result, m.err
}

func TestReviewStage_Name(t *testing.T) {
	s := NewReviewStage(nil, ReviewStageConfig{}, nil)
	if s.Name() != "review" {
		t.Errorf("Name() = %q, want %q", s.Name(), "review")
	}
}

func TestReviewStage_Clean_Continue(t *testing.T) {
	runner := &mockReviewRunner{
		result: review.RunResult{
			AllFindings:        []review.Finding{},
			BlockingFindings:   []review.Finding{},
			HasBlockingFindings: false,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}
}

func TestReviewStage_BlockingFindings_ReplanFrom(t *testing.T) {
	runner := &mockReviewRunner{
		result: review.RunResult{
			AllFindings: []review.Finding{
				{Severity: review.SeverityError, File: "handler.go", Description: "missing validation"},
			},
			BlockingFindings: []review.Finding{
				{Severity: review.SeverityError, File: "handler.go", Description: "missing validation"},
			},
			HasBlockingFindings: true,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Errorf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext")
	}
	if len(action.Context.Failures) == 0 {
		t.Error("expected at least one failure message")
	}
}

func TestReviewStage_InfoOnly_Continue(t *testing.T) {
	runner := &mockReviewRunner{
		result: review.RunResult{
			AllFindings: []review.Finding{
				{Severity: review.SeverityInfo, File: "handler.go", Description: "consider helper"},
			},
			BlockingFindings:   []review.Finding{},
			HasBlockingFindings: false,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("info-only findings should Continue, got %v", action.Kind)
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestReviewStage -v` — expect FAIL

- **Step 3: Implement** — Define `ReviewRunner` interface matching `review.Runner.Run` signature. Define `ReviewStageConfig` with `DiffSummary string`, `SpecContent string`, `EvidenceDir string`. `ReviewStage` struct holds a `ReviewRunner`, config, optional `*runstore.EventLog`, and a `priorFindings []review.Finding` field (stage-local state for disposition matching across cycles). `Name()` returns `"review"`. `Run` calls the runner with `RunInput` assembled from config and run state. If `result.HasBlockingFindings`, set `rs.FinalReviewPassed = false`, populate `rs.ReviewFindings` with `ReviewFailuresToStrings(result.BlockingFindings)` (for planner/FailureContext), and return `ReplanFrom` with `FailureContext` containing the same strings. If no blocking findings, set `rs.FinalReviewPassed = true` and return `Continue`. After each run, append `result.AllFindings` to the stage's internal `priorFindings` slice, and write structured `review.json` via `Bundler.WriteReviewFindings`.

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestReviewStage -v` — expect PASS
- **Step 5:** Commit `"feat(next): add ReviewStage with blocking/continue logic"`

---

### Task 16: AcceptStage implementation

- **Files:**
  - Create: `internal/next/specloop/stages/accept.go`
  - Create: `internal/next/specloop/stages/accept_test.go`

- **Step 1: Write failing test** in `accept_test.go`:
```go
package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type mockAcceptEvaluator struct {
	result acceptor.AcceptanceResult
	err    error
}

func (m *mockAcceptEvaluator) Evaluate(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
	return m.result, m.err
}

func TestAcceptStage_Name(t *testing.T) {
	s := NewAcceptStage(nil, AcceptStageConfig{}, nil)
	if s.Name() != "accept" {
		t.Errorf("Name() = %q, want %q", s.Name(), "accept")
	}
}

func TestAcceptStage_AllPass_Continue(t *testing.T) {
	eval := &mockAcceptEvaluator{
		result: acceptor.AcceptanceResult{
			Results: []acceptor.CriterionResult{
				{Criterion: "returns 200", Status: acceptor.StatusPass},
			},
			AllPass:          true,
			HasFailOrUnclear: false,
		},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"returns 200"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}
}

func TestAcceptStage_Fail_ReplanFrom(t *testing.T) {
	eval := &mockAcceptEvaluator{
		result: acceptor.AcceptanceResult{
			Results: []acceptor.CriterionResult{
				{Criterion: "multi-currency", Status: acceptor.StatusFail, Rationale: "only USD"},
			},
			AllPass:          false,
			HasFailOrUnclear: true,
		},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"multi-currency"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Errorf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext")
	}
}

func TestAcceptStage_Unclear_ReplanFrom(t *testing.T) {
	eval := &mockAcceptEvaluator{
		result: acceptor.AcceptanceResult{
			Results: []acceptor.CriterionResult{
				{Criterion: "audit log", Status: acceptor.StatusUnclear, Rationale: "no test verifies it"},
			},
			AllPass:          false,
			HasFailOrUnclear: true,
		},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"audit log"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Errorf("unclear should trigger ReplanFrom, got %v", action.Kind)
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestAcceptStage -v` — expect FAIL

- **Step 3: Implement** — Define `AcceptEvaluator` interface matching `acceptor.Evaluator.Evaluate` signature. Define `AcceptStageConfig` with `Criteria []string`, `DiffSummary string`, `EvidenceDir string`. `AcceptStage` struct holds an `AcceptEvaluator`, config, and optional `*runstore.EventLog`. `Name()` returns `"accept"`. `Run` calls the evaluator with `EvaluateInput` assembled from config and run state. If `result.HasFailOrUnclear`, return `ReplanFrom` with `FailureContext` built from `acceptor.BuildFailureContext` (using `AcceptanceFailuresToStrings` with fail/unclear differentiation). Otherwise return `Continue`. On success, set `rs.FinalAcceptancePassed = true`. On failure, set `rs.FinalAcceptancePassed = false` and populate `rs.AcceptanceResults` with the string representations (for planner/FailureContext). After each run, write structured `acceptance.json` via `Bundler.WriteAcceptanceResults(result)` as a side effect.

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestAcceptStage -v` — expect PASS
- **Step 5:** Commit `"feat(next): add AcceptStage with fail/unclear replanning"`

---

### Task 16a: ParseAcceptanceCriteria helper

- **Files:**
  - Create: `internal/next/acceptor/parse.go`
  - Create: `internal/next/acceptor/parse_test.go`

- **Step 1: Write failing test** in `parse_test.go`:
```go
package acceptor

import "testing"

func TestParseAcceptanceCriteria_BasicSection(t *testing.T) {
	specMD := `# My Spec

## Description
Some description here.

## Acceptance Criteria
- Refund endpoint returns 200
- Multi-currency support for USD and EUR
- Audit log entry created on refund

## Implementation Notes
Some notes.
`
	criteria, err := ParseAcceptanceCriteria(specMD)
	if err != nil {
		t.Fatalf("ParseAcceptanceCriteria: %v", err)
	}
	if len(criteria) != 3 {
		t.Fatalf("expected 3 criteria, got %d: %v", len(criteria), criteria)
	}
	if criteria[0] != "Refund endpoint returns 200" {
		t.Errorf("criteria[0] = %q", criteria[0])
	}
}

func TestParseAcceptanceCriteria_NoSection(t *testing.T) {
	specMD := `# My Spec

## Description
No acceptance criteria section here.
`
	_, err := ParseAcceptanceCriteria(specMD)
	if err == nil {
		t.Error("expected error when no Acceptance Criteria section found")
	}
}

func TestParseAcceptanceCriteria_EmptySection(t *testing.T) {
	specMD := `# My Spec

## Acceptance Criteria

## Next Section
`
	criteria, err := ParseAcceptanceCriteria(specMD)
	if err != nil {
		t.Fatalf("ParseAcceptanceCriteria: %v", err)
	}
	if len(criteria) != 0 {
		t.Errorf("expected 0 criteria for empty section, got %d", len(criteria))
	}
}
```

- **Step 2:** `go test ./internal/next/acceptor/ -run TestParseAcceptanceCriteria -v` — expect FAIL

- **Step 3: Implement** — `ParseAcceptanceCriteria(specMarkdown string) ([]string, error)` parses the `## Acceptance Criteria` section from the spec markdown. Scans for the heading, collects bullet points (`- ` prefix) until the next `##` heading or EOF. Strips the `- ` prefix and trims whitespace. Returns error if the section is not found. Lives in `internal/next/acceptor/parse.go`.

- **Step 4:** `go test ./internal/next/acceptor/ -run TestParseAcceptanceCriteria -v` — expect PASS
- **Step 5:** Commit `"feat(next): add ParseAcceptanceCriteria markdown parser"`

---

### Task 17: Pipeline insertion — review and accept stages in correct order

- **Files:**
  - Modify: `cmd/gromit-next/stage_provider.go`
  - Modify: `cmd/gromit-next/stage_provider_test.go`

- **Step 1: Write failing test** in `stage_provider_test.go`:
```go
func TestRealStageProvider_BuildStages_IncludesReviewAndAccept(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	stages, err := provider.BuildStages(policy, rs)
	if err != nil {
		t.Fatalf("BuildStages: %v", err)
	}

	expectedOrder := []string{"init", "compile", "plan", "execute", "validate", "review", "accept", "evidence", "finalize"}
	if len(stages) != len(expectedOrder) {
		t.Fatalf("expected %d stages, got %d", len(expectedOrder), len(stages))
	}
	for i, name := range expectedOrder {
		if stages[i].Name() != name {
			t.Errorf("stage[%d].Name() = %q, want %q", i, stages[i].Name(), name)
		}
	}
}

func TestRealStageProvider_ReviewBeforeAccept(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	stages, err := provider.BuildStages(policy, rs)
	if err != nil {
		t.Fatalf("BuildStages: %v", err)
	}

	var reviewIdx, acceptIdx int
	for i, s := range stages {
		if s.Name() == "review" {
			reviewIdx = i
		}
		if s.Name() == "accept" {
			acceptIdx = i
		}
	}
	if reviewIdx >= acceptIdx {
		t.Errorf("review (idx %d) must come before accept (idx %d)", reviewIdx, acceptIdx)
	}
}
```

- **Step 2:** `go test ./cmd/gromit-next/ -run TestRealStageProvider_BuildStages_IncludesReviewAndAccept -v` — expect FAIL

- **Step 3: Implement** — Update `RealStageProvider.BuildStages` to insert `ReviewStage` and `AcceptStage` between `ValidateStage` and `EvidenceStage` in the pipeline. Wire them with appropriate configs from the policy and run state. The stage order becomes: init, compile, plan, execute, validate, review, accept, evidence, finalize.

- **Step 4:** `go test ./cmd/gromit-next/ -run TestRealStageProvider_BuildStages_Includes -v` — expect PASS
- **Step 5:** Commit `"feat(next): insert ReviewStage and AcceptStage into execution pipeline"`

---

### Task 18: Dry-run filter excludes review and accept stages

- **Files:**
  - Modify: `cmd/gromit-next/exec.go`
  - Modify: `cmd/gromit-next/exec_test.go`

- **Step 1: Write failing test** in `exec_test.go`:
```go
func TestFilterStagesForDryRun_ExcludesReviewAndAccept(t *testing.T) {
	stages := []specloop.Stage{
		&namedStage{name: "init"},
		&namedStage{name: "compile"},
		&namedStage{name: "plan"},
		&namedStage{name: "execute"},
		&namedStage{name: "validate"},
		&namedStage{name: "review"},
		&namedStage{name: "accept"},
		&namedStage{name: "evidence"},
		&namedStage{name: "finalize"},
	}

	filtered := filterStagesForDryRun(stages, true)
	for _, s := range filtered {
		if s.Name() == "review" || s.Name() == "accept" {
			t.Errorf("dry-run should not include %q stage", s.Name())
		}
	}
	// Should still include init, compile, plan
	if len(filtered) != 3 {
		t.Errorf("expected 3 dry-run stages, got %d", len(filtered))
	}
}
```

- **Step 2:** `go test ./cmd/gromit-next/ -run TestFilterStagesForDryRun_ExcludesReviewAndAccept -v` — expect PASS (already excluded since dryRunStages only allows init/compile/plan)

- **Step 3: Implement** — Verify that the existing `dryRunStages` map (init, compile, plan) already excludes review and accept. No code change needed if the test passes. Add the `namedStage` test helper.

- **Step 4:** `go test ./cmd/gromit-next/ -run TestFilterStagesForDryRun -v` — expect PASS
- **Step 5:** Commit `"test(next): verify dry-run filter excludes review and accept stages"`

---

## Phase 5: Fix-Cycle Extensions — Review/Acceptance Failure Context and Replan Triggers

### Task 19: Review failure context — structured finding data for planner

- **Files:**
  - Create: `internal/next/review/failctx.go`
  - Create: `internal/next/review/failctx_test.go`

- **Step 1: Write failing test** in `failctx_test.go`:
```go
package review

import "testing"

func TestBuildReviewFailureContext_BlockingOnly(t *testing.T) {
	result := RunResult{
		AllFindings: []Finding{
			{Facet: "spec_alignment", Severity: SeverityError, File: "handler.go", Description: "missing validation", SuggestedFix: "add check"},
			{Facet: "code_quality", Severity: SeverityInfo, File: "handler.go", Description: "consider extracting helper"},
		},
		BlockingFindings: []Finding{
			{Facet: "spec_alignment", Severity: SeverityError, File: "handler.go", Description: "missing validation", SuggestedFix: "add check"},
		},
	}

	strs := BuildFailureStrings(result)
	if len(strs) != 1 {
		t.Fatalf("expected 1 failure string (blocking only), got %d", len(strs))
	}
	if !containsAny(strs[0], "missing validation") {
		t.Errorf("failure string should contain description, got %q", strs[0])
	}
	if !containsAny(strs[0], "handler.go") {
		t.Errorf("failure string should contain file, got %q", strs[0])
	}
	if !strings.HasPrefix(strs[0], "review:") {
		t.Errorf("failure string should have review: prefix, got %q", strs[0])
	}
}

func TestBuildReviewFailureContext_Empty(t *testing.T) {
	result := RunResult{
		AllFindings:      []Finding{},
		BlockingFindings: []Finding{},
	}

	strs := BuildFailureStrings(result)
	if len(strs) != 0 {
		t.Errorf("expected 0 failure strings, got %d", len(strs))
	}
}

func TestBuildReviewFailureContext_IncludesSuggestedFix(t *testing.T) {
	result := RunResult{
		BlockingFindings: []Finding{
			{Facet: "spec_alignment", Severity: SeverityError, File: "handler.go", Description: "missing check", SuggestedFix: "add nil guard"},
		},
	}

	strs := BuildFailureStrings(result)
	if len(strs) != 1 {
		t.Fatalf("expected 1 string, got %d", len(strs))
	}
	if !containsAny(strs[0], "add nil guard") {
		t.Error("failure string should include suggested fix")
	}
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestBuildReviewFailureContext -v` — expect FAIL

- **Step 3: Implement** — `BuildFailureStrings(result RunResult) []string` iterates `result.BlockingFindings` and formats each as a human-readable string: `"review:<facet>:<severity>: <file>:<line> — <description> (suggested fix: <fix>)"`. These strings become the `FailureContext.Failures` slice that the planner receives for targeted fix-task generation. Also add `ReviewFailuresToStrings(findings []Finding) []string` helper that converts a slice of `Finding` to human-readable failure strings for `FailureContext.Failures`. This is the review-side counterpart to `AcceptanceFailuresToStrings` in the acceptor package.

- **Step 4:** `go test ./internal/next/review/ -run TestBuildReviewFailureContext -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review failure context builder for planner replanning"`

---

### Task 20: RunState extensions — store review and acceptance results

- **Files:**
  - Modify: `internal/next/runstore/types.go`
  - Modify or create: `internal/next/runstore/types_test.go`

- **New fields to add to RunState:**
```go
FinalReviewPassed     bool     `json:"final_review_passed"`
FinalAcceptancePassed bool     `json:"final_acceptance_passed"`
ReviewFindings        []string `json:"review_findings,omitempty"`
AcceptanceResults     []string `json:"acceptance_results,omitempty"`
```

- **Step 1: Write failing test** in `types_test.go`:
```go
package runstore

import (
	"encoding/json"
	"testing"
)

func TestRunState_ReviewAndAcceptanceFields(t *testing.T) {
	rs := NewRunState("test-spec", "test-project")
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = false
	rs.ReviewFindings = []string{"[spec_alignment] error: handler.go:42 — missing validation"}
	rs.AcceptanceResults = []string{"acceptance:fail: multi-currency — implement missing behavior"}

	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RunState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.FinalReviewPassed {
		t.Error("FinalReviewPassed should round-trip")
	}
	if got.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should round-trip as false")
	}
	if len(got.ReviewFindings) != 1 {
		t.Errorf("ReviewFindings should round-trip, got %d", len(got.ReviewFindings))
	}
	if len(got.AcceptanceResults) != 1 {
		t.Errorf("AcceptanceResults should round-trip, got %d", len(got.AcceptanceResults))
	}
}

func TestRunState_NormalizeNilFields_IncludesNewFields(t *testing.T) {
	rs := &RunState{}
	rs.NormalizeNilFields()
	// Existing fields
	if rs.Tasks == nil {
		t.Error("Tasks should not be nil after NormalizeNilFields")
	}
	if rs.ReplanContext == nil {
		t.Error("ReplanContext should not be nil after NormalizeNilFields")
	}
	if rs.ReviewFindings == nil {
		t.Error("ReviewFindings should not be nil after NormalizeNilFields")
	}
	if rs.AcceptanceResults == nil {
		t.Error("AcceptanceResults should not be nil after NormalizeNilFields")
	}
}
```

- **Step 2:** `go test ./internal/next/runstore/ -run TestRunState_ReviewAndAcceptance -v` — expect FAIL

- **Step 3: Implement** — Add four new fields to `RunState`: `FinalReviewPassed bool`, `FinalAcceptancePassed bool`, `ReviewFindings []string` (omitempty), `AcceptanceResults []string` (omitempty). `ReviewFindings` stores human-readable review finding strings (produced by `ReviewFailuresToStrings`). `AcceptanceResults` stores human-readable acceptance result strings (produced by `AcceptanceFailuresToStrings`). These `[]string` fields exist **solely for planner/FailureContext consumption** — they are NOT used for evidence files (which are written as structured JSON by ReviewStage/AcceptStage directly) and NOT used for disposition matching (which uses stage-local `[]review.Finding` state). The `bool` flags indicate terminal pass/fail state for FinalizeStage's `ready_for_review` determination. Update `NormalizeNilFields()` to map nil `ReviewFindings` and `AcceptanceResults` to empty slices.

- **Step 4:** `go test ./internal/next/runstore/ -run TestRunState_ReviewAndAcceptance -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review/acceptance fields to RunState"`

---

### Task 21: ReviewStage stores findings in RunState

- **Files:**
  - Modify: `internal/next/specloop/stages/review.go`
  - Modify: `internal/next/specloop/stages/review_test.go`

- **Step 1: Write failing test** in `review_test.go`:
```go
func TestReviewStage_StoresFindingsInRunState(t *testing.T) {
	runner := &mockReviewRunner{
		result: review.RunResult{
			AllFindings: []review.Finding{
				{Facet: "code_quality", Severity: review.SeverityInfo, File: "handler.go", Description: "consider helper"},
			},
			FindingsByFacet: map[string][]review.Finding{
				"code_quality": {{Severity: review.SeverityInfo, File: "handler.go", Description: "consider helper"}},
			},
			BlockingFindings:   []review.Finding{},
			HasBlockingFindings: false,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(rs.ReviewFindings) == 0 {
		t.Fatal("ReviewFindings should be populated in RunState after review")
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestReviewStage_StoresFindings -v` — expect FAIL

- **Step 3: Implement** — After running the review, convert all findings to string representations using `ReviewFailuresToStrings` and store in `rs.ReviewFindings` (for planner/FailureContext consumption only). Also set `rs.FinalReviewPassed` based on `result.HasBlockingFindings`. Additionally, the stage appends all `result.AllFindings` to its internal `priorFindings []review.Finding` slice (stage-local state for disposition matching on fix cycles). The stage also writes the structured `review.json` evidence file as a side effect, using `Bundler.WriteReviewFindings(result.FindingsByFacet)` with the facet-keyed structured data. This ensures evidence files contain full structured Finding objects matching the spec schema.

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestReviewStage_StoresFindings -v` — expect PASS
- **Step 5:** Commit `"feat(next): ReviewStage stores findings as strings in RunState"`

---

### Task 22: AcceptStage stores results in RunState

- **Files:**
  - Modify: `internal/next/specloop/stages/accept.go`
  - Modify: `internal/next/specloop/stages/accept_test.go`

- **Step 1: Write failing test** in `accept_test.go`:
```go
func TestAcceptStage_StoresResultsInRunState(t *testing.T) {
	eval := &mockAcceptEvaluator{
		result: acceptor.AcceptanceResult{
			Results: []acceptor.CriterionResult{
				{Criterion: "returns 200", Status: acceptor.StatusPass, Rationale: "test proves it"},
			},
			AllPass:          true,
			HasFailOrUnclear: false,
		},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"returns 200"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !rs.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be true when all pass")
	}
	// AcceptanceResults may be empty when all pass (no failures to report)
}

func TestAcceptStage_StoresFailuresInRunState(t *testing.T) {
	eval := &mockAcceptEvaluator{
		result: acceptor.AcceptanceResult{
			Results: []acceptor.CriterionResult{
				{Criterion: "multi-currency", Status: acceptor.StatusFail, Rationale: "only USD"},
			},
			HasFailOrUnclear: true,
		},
	}

	stage := NewAcceptStage(eval, AcceptStageConfig{
		Criteria: []string{"multi-currency"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rs.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should be false on failure")
	}
	if len(rs.AcceptanceResults) == 0 {
		t.Error("AcceptanceResults should contain failure strings")
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestAcceptStage_Stores -v` — expect FAIL

- **Step 3: Implement** — After running the evaluator, set `rs.FinalAcceptancePassed = result.AllPass`. If `result.HasFailOrUnclear`, populate `rs.AcceptanceResults` with `AcceptanceFailuresToStrings(result.Results)` using the fail/unclear differentiation format (for planner/FailureContext consumption only). The stage also writes the structured `acceptance.json` evidence file as a side effect, using `Bundler.WriteAcceptanceResults(result)` with the full `AcceptanceResult` struct. This ensures the evidence file contains the spec-schema-compliant structure (`{results: [...], all_pass, has_fail_or_unclear}`).

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestAcceptStage_Stores -v` — expect PASS
- **Step 5:** Commit `"feat(next): AcceptStage stores results and flags in RunState"`

---

### Task 23: ReviewStage passes prior findings on fix cycles

- **Files:**
  - Modify: `internal/next/specloop/stages/review.go`
  - Modify: `internal/next/specloop/stages/review_test.go`

- **Design note:** ReviewStage holds `[]review.Finding` as **stage-local state** (a field on the ReviewStage struct, not on RunState). On each cycle, after the review runner returns, the stage appends all findings from the current cycle to its internal `priorFindings` slice. On fix cycles (cycle > 1), the stage passes its internal prior findings to `RunInput.PriorFindings` for disposition matching (new vs. pre-existing). This avoids any need to parse `rs.ReviewFindings` strings back into structured `Finding` objects.

- **Step 1: Write failing test** in `review_test.go`:
```go
func TestReviewStage_FixCycle_PassesPriorFindings(t *testing.T) {
	cycle1Findings := []review.Finding{
		{Facet: "spec_alignment", Severity: "warning", File: "handler.go", Message: "missing check"},
	}

	var capturedInput review.RunInput
	callCount := 0
	runner := &capturingReviewRunner{
		resultFn: func() review.RunResult {
			callCount++
			if callCount == 1 {
				return review.RunResult{
					AllFindings:         cycle1Findings,
					BlockingFindings:    []review.Finding{},
					HasBlockingFindings: false,
				}
			}
			return review.RunResult{
				AllFindings:         []review.Finding{},
				BlockingFindings:    []review.Finding{},
				HasBlockingFindings: false,
			}
		},
		capture: func(input review.RunInput) {
			capturedInput = input
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{}, nil)

	// Cycle 1: produces findings, stage stores them internally
	rs1 := runstore.NewRunState("test-spec", "test-project")
	rs1.Cycle = 1
	stage.Run(context.Background(), rs1)

	// Cycle 2: stage should pass prior findings from its internal state
	rs2 := runstore.NewRunState("test-spec", "test-project")
	rs2.Cycle = 2
	_, err := stage.Run(context.Background(), rs2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(capturedInput.PriorFindings) == 0 {
		t.Error("on fix cycle, prior findings should be passed to runner from stage-local state")
	}
	if capturedInput.PriorFindings[0].Facet != "spec_alignment" {
		t.Errorf("expected spec_alignment facet, got %s", capturedInput.PriorFindings[0].Facet)
	}
}

type capturingReviewRunner struct {
	resultFn func() review.RunResult
	capture  func(review.RunInput)
}

func (c *capturingReviewRunner) Run(ctx context.Context, input review.RunInput) (review.RunResult, error) {
	if c.capture != nil {
		c.capture(input)
	}
	return c.resultFn(), nil
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestReviewStage_FixCycle -v` — expect FAIL

- **Step 3: Implement** — Add a `priorFindings []review.Finding` field to the `ReviewStage` struct. In `ReviewStage.Run`: (1) on fix cycles (cycle > 1), populate `RunInput.PriorFindings` from the stage's internal `priorFindings` slice; (2) after the review runner returns, append all `RunResult.AllFindings` to the stage's `priorFindings` slice. The stage also writes `rs.ReviewFindings` (the `[]string` form) for planner/FailureContext consumption, but disposition matching uses only the structured stage-local state.

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestReviewStage_FixCycle -v` — expect PASS
- **Step 5:** Commit `"feat(next): ReviewStage passes prior findings to runner on fix cycles"`

---

## Phase 6: Execution Policy Extensions

### Task 24: Add review config section to Policy

- **Files:**
  - Modify: `internal/next/execpolicy/policy.go`
  - Modify: `internal/next/execpolicy/policy_test.go`

- **Step 1: Write failing test** in `policy_test.go`:
```go
func TestDefaultPolicy_HasReviewConfig(t *testing.T) {
	p := DefaultPolicy()
	if len(p.Review.Facets) != 2 {
		t.Fatalf("default review should have 2 facets, got %d", len(p.Review.Facets))
	}
	if p.Review.Facets[0] != "spec_alignment" {
		t.Errorf("first facet = %q, want spec_alignment", p.Review.Facets[0])
	}
	if p.Review.Facets[1] != "code_quality" {
		t.Errorf("second facet = %q, want code_quality", p.Review.Facets[1])
	}
	if p.Review.ReplanThreshold != "suggestion" {
		t.Errorf("ReplanThreshold = %q, want suggestion", p.Review.ReplanThreshold)
	}
}

func TestDefaultPolicy_ReviewTiers(t *testing.T) {
	p := DefaultPolicy()
	if p.Review.Tiers["spec_alignment"] != "high" {
		t.Errorf("spec_alignment tier = %q, want high", p.Review.Tiers["spec_alignment"])
	}
	if p.Review.Tiers["code_quality"] != "medium" {
		t.Errorf("code_quality tier = %q, want medium", p.Review.Tiers["code_quality"])
	}
}
```

- **Step 2:** `go test ./internal/next/execpolicy/ -run TestDefaultPolicy_HasReviewConfig -v` — expect FAIL

- **Step 3: Implement** — Add `ReviewConfig` struct with `Facets []string`, `Tiers map[string]string`, `ReplanThreshold string`. Add `Review ReviewConfig` field to `Policy`. Update `DefaultPolicy()` to include default review config: facets `["spec_alignment", "code_quality"]`, tiers `{"spec_alignment": "high", "code_quality": "medium"}`, threshold `"suggestion"`. Update `NormalizeNilFields()` to handle `Review.Facets` and `Review.Tiers`.

- **Step 4:** `go test ./internal/next/execpolicy/ -run TestDefaultPolicy_HasReview -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review config section to execution policy"`

---

### Task 25: Add evaluator model tier to Policy

- **Files:**
  - Modify: `internal/next/execpolicy/policy.go`
  - Modify: `internal/next/execpolicy/policy_test.go`

- **Step 1: Write failing test** in `policy_test.go`:
```go
func TestDefaultPolicy_HasEvaluatorTier(t *testing.T) {
	p := DefaultPolicy()
	if p.Models.Evaluator != "high" {
		t.Errorf("Evaluator tier = %q, want high", p.Models.Evaluator)
	}
}

func TestPolicy_Validate_EvaluatorRequired(t *testing.T) {
	p := DefaultPolicy()
	p.Models.Evaluator = ""
	err := p.Validate()
	if err == nil {
		t.Error("Validate should fail when Evaluator is empty")
	}
}
```

- **Step 2:** `go test ./internal/next/execpolicy/ -run TestDefaultPolicy_HasEvaluatorTier -v` — expect FAIL

- **Step 3: Implement** — Add `Evaluator string` field to `Models` struct. Set default to `"high"` in `DefaultPolicy()`. Add validation in `Validate()` for non-empty `Models.Evaluator`.

- **Step 4:** `go test ./internal/next/execpolicy/ -run TestDefaultPolicy_HasEvaluatorTier -v` — expect PASS
- **Step 5:** Commit `"feat(next): add evaluator model tier to execution policy"`

---

### Task 26: Policy validation — facet list validation against registry

- **Files:**
  - Modify: `internal/next/execpolicy/policy.go`
  - Modify: `internal/next/execpolicy/policy_test.go`

- **Step 1: Write failing test** in `policy_test.go`:
```go
func TestPolicy_ValidateReviewFacets_ValidFacets(t *testing.T) {
	p := DefaultPolicy()
	err := p.ValidateReviewFacets([]string{"spec_alignment", "code_quality", "logic_gaps", "test_coverage", "architecture_drift"})
	if err != nil {
		t.Errorf("valid facets should not error: %v", err)
	}
}

func TestPolicy_ValidateReviewFacets_UnknownFacet(t *testing.T) {
	p := DefaultPolicy()
	p.Review.Facets = []string{"spec_alignment", "nonexistent_facet"}
	err := p.ValidateReviewFacets([]string{"spec_alignment", "code_quality"})
	if err == nil {
		t.Error("unknown facet should produce validation error")
	}
}

func TestPolicy_ValidateReviewThreshold_Valid(t *testing.T) {
	p := DefaultPolicy()
	for _, threshold := range []string{"error", "warning", "suggestion"} {
		p.Review.ReplanThreshold = threshold
		if err := p.ValidateReviewConfig(); err != nil {
			t.Errorf("threshold %q should be valid: %v", threshold, err)
		}
	}
}

func TestPolicy_ValidateReviewThreshold_Invalid(t *testing.T) {
	p := DefaultPolicy()
	p.Review.ReplanThreshold = "critical"
	if err := p.ValidateReviewConfig(); err == nil {
		t.Error("invalid threshold should produce error")
	}
}
```

- **Step 2:** `go test ./internal/next/execpolicy/ -run TestPolicy_ValidateReview -v` — expect FAIL

- **Step 3: Implement** — Add `ValidateReviewFacets(knownFacets []string) error` that checks each facet in `p.Review.Facets` is in the known list. Add `ValidateReviewConfig() error` that validates `ReplanThreshold` is one of `"error"`, `"warning"`, `"suggestion"`. Consider calling these from within `Validate()` or as separate validation methods that callers invoke.

- **Step 4:** `go test ./internal/next/execpolicy/ -run TestPolicy_ValidateReview -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review facet and threshold validation to policy"`

---

### Task 27: Policy JSON loading — review config section

- **Files:**
  - Modify: `internal/next/execpolicy/policy_test.go`

- **Step 1: Write failing test** in `policy_test.go`:
```go
func TestLoadPolicy_ReviewConfigFromJSON(t *testing.T) {
	tmpDir := t.TempDir()
	policyJSON := `{
		"review": {
			"facets": ["spec_alignment", "code_quality", "logic_gaps"],
			"tiers": {
				"spec_alignment": "high",
				"code_quality": "medium",
				"logic_gaps": "high"
			},
			"replan_threshold": "error"
		}
	}`
	path := filepath.Join(tmpDir, "policy.json")
	if err := os.WriteFile(path, []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if len(p.Review.Facets) != 3 {
		t.Errorf("expected 3 facets, got %d", len(p.Review.Facets))
	}
	if p.Review.ReplanThreshold != "error" {
		t.Errorf("threshold = %q, want error", p.Review.ReplanThreshold)
	}
	if p.Review.Tiers["logic_gaps"] != "high" {
		t.Errorf("logic_gaps tier = %q, want high", p.Review.Tiers["logic_gaps"])
	}
}

func TestLoadPolicy_ReviewDefaultsPreservedOnPartialJSON(t *testing.T) {
	tmpDir := t.TempDir()
	// Partial JSON: only override threshold, keep default facets
	policyJSON := `{
		"review": {
			"replan_threshold": "warning"
		}
	}`
	path := filepath.Join(tmpDir, "policy.json")
	if err := os.WriteFile(path, []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.Review.ReplanThreshold != "warning" {
		t.Errorf("threshold should be overridden to warning, got %q", p.Review.ReplanThreshold)
	}
	// Facets should be defaults since they weren't overridden
	if len(p.Review.Facets) != 2 {
		t.Errorf("facets should be default (2), got %d", len(p.Review.Facets))
	}
}
```

- **Step 2:** `go test ./internal/next/execpolicy/ -run TestLoadPolicy_ReviewConfig -v` — expect FAIL (or PASS if JSON unmarshalling already works via struct embedding)

- **Step 3: Implement** — The existing `LoadPolicy` unmarshal-into-defaults approach should handle the new `Review` field automatically. If tests pass without changes, this is a verification-only task. If the `Tiers` map does not unmarshal correctly (because maps in defaults get replaced, not merged), add post-unmarshal fixup to merge tier defaults for facets that were not overridden.

- **Step 4:** `go test ./internal/next/execpolicy/ -run TestLoadPolicy_Review -v` — expect PASS
- **Step 5:** Commit `"test(next): verify review config loads from JSON with defaults"`

---

## Phase 7: Evidence Bundle Extensions

### Task 28: WriteReviewFindings method on Bundler

- **Files:**
  - Modify: `internal/next/evidence/bundle.go`
  - Modify or create: `internal/next/evidence/bundle_test.go`

- **Design note:** `WriteReviewFindings` accepts structured `map[string][]review.Finding` (facet-keyed) and writes it as structured JSON matching the spec schema. ReviewStage calls this directly as a side effect of its `Run` method, passing the structured findings it already holds. The Bundler method does NOT accept `[]string` — the `[]string` form on RunState is only for planner/FailureContext.

- **Step 1: Write failing test** in `bundle_test.go`:
```go
package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
)

func TestBundler_WriteReviewFindings(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	findings := map[string][]review.Finding{
		"spec_alignment": {
			{Facet: "spec_alignment", Severity: "error", File: "handler.go", Line: 42, Message: "missing validation"},
		},
		"code_quality": {
			{Facet: "code_quality", Severity: "warning", File: "router.go", Line: 10, Message: "long function"},
		},
	}

	if err := b.WriteReviewFindings(findings); err != nil {
		t.Fatalf("WriteReviewFindings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}

	var parsed map[string][]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("review.json should be a facet-keyed JSON object: %v", err)
	}
	if len(parsed["spec_alignment"]) != 1 {
		t.Errorf("expected 1 spec_alignment finding, got %d", len(parsed["spec_alignment"]))
	}
	if len(parsed["code_quality"]) != 1 {
		t.Errorf("expected 1 code_quality finding, got %d", len(parsed["code_quality"]))
	}
}
```

- **Step 2:** `go test ./internal/next/evidence/ -run TestBundler_WriteReviewFindings -v` — expect FAIL

- **Step 3: Implement** — Add `WriteReviewFindings(findings map[string][]review.Finding) error` to `Bundler`. Write the facet-keyed findings object as structured JSON to `review.json` in the evidence directory using `json.MarshalIndent`. This matches the spec schema where review.json is a facet-keyed object with Finding arrays as values.

- **Step 4:** `go test ./internal/next/evidence/ -run TestBundler_WriteReviewFindings -v` — expect PASS
- **Step 5:** Commit `"feat(next): add WriteReviewFindings to evidence Bundler"`

---

### Task 29: WriteAcceptanceResults method on Bundler

- **Files:**
  - Modify: `internal/next/evidence/bundle.go`
  - Modify: `internal/next/evidence/bundle_test.go`

- **Design note:** `WriteAcceptanceResults` accepts a structured `acceptor.AcceptanceResult` and writes it as structured JSON matching the spec schema (`{results: [...], all_pass: bool, has_fail_or_unclear: bool}`). AcceptStage calls this directly as a side effect of its `Run` method, passing the structured result it already holds. The `[]string` form on RunState is only for planner/FailureContext.

- **Step 1: Write failing test** in `bundle_test.go`:
```go
func TestBundler_WriteAcceptanceResults(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	result := acceptor.AcceptanceResult{
		Results: []acceptor.CriterionResult{
			{Criterion: "multi-currency", Status: "fail", Reason: "implement missing behavior"},
			{Criterion: "audit log", Status: "unclear", Reason: "add tests or evidence to prove/disprove"},
		},
		AllPass:           false,
		HasFailOrUnclear:  true,
	}

	if err := b.WriteAcceptanceResults(result); err != nil {
		t.Fatalf("WriteAcceptanceResults: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "acceptance.json"))
	if err != nil {
		t.Fatalf("read acceptance.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("acceptance.json should be a valid JSON object: %v", err)
	}
	results, ok := parsed["results"].([]interface{})
	if !ok || len(results) != 2 {
		t.Errorf("expected 2 results in results array, got %v", parsed["results"])
	}
	if parsed["all_pass"] != false {
		t.Error("all_pass should be false")
	}
	if parsed["has_fail_or_unclear"] != true {
		t.Error("has_fail_or_unclear should be true")
	}
}
```

- **Step 2:** `go test ./internal/next/evidence/ -run TestBundler_WriteAcceptanceResults -v` — expect FAIL

- **Step 3: Implement** — Add `WriteAcceptanceResults(result acceptor.AcceptanceResult) error` to `Bundler`. Write the structured acceptance result as JSON to `acceptance.json` in the evidence directory using `json.MarshalIndent`. The output matches the spec schema: `{results: [{criterion, status, reason}, ...], all_pass: bool, has_fail_or_unclear: bool}`.

- **Step 4:** `go test ./internal/next/evidence/ -run TestBundler_WriteAcceptanceResults -v` — expect PASS
- **Step 5:** Commit `"feat(next): add WriteAcceptanceResults to evidence Bundler"`

---

### Task 30: Extended review.md — review findings and acceptance table sections

- **Files:**
  - Modify: `internal/next/evidence/bundle.go`
  - Modify: `internal/next/evidence/bundle_test.go`

- **Step 1: Write failing test** in `bundle_test.go`:
```go
func TestBundler_WriteReview_IncludesReviewFindings(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	input := ReviewInput{
		TerminalState:     "ready_for_review",
		WhatChanged:       "Added refund handler",
		CycleHistory:      []CycleRecord{{Cycle: 1, TaskCount: 4, PassCount: 4}},
		ValidationResults: "6/6 passed",
		KnownRisks:        []string{},
		RecommendedAction: "approve",
		ReviewFindings: []ReviewFindingSummary{
			{Facet: "spec_alignment", Count: 0, Severities: "none"},
			{Facet: "code_quality", Count: 1, Severities: "1 info"},
		},
		AcceptanceCriteria: []AcceptanceCriterionSummary{
			{Criterion: "returns 200", Status: "pass", Rationale: "test proves it"},
			{Criterion: "handles errors", Status: "pass", Rationale: "error tests exist"},
		},
	}

	if err := b.WriteReview(input); err != nil {
		t.Fatalf("WriteReview: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "review.md"))
	if err != nil {
		t.Fatalf("read review.md: %v", err)
	}

	content := string(data)
	if !containsSubstring(content, "Review Findings") {
		t.Error("review.md should contain Review Findings section")
	}
	if !containsSubstring(content, "spec_alignment") {
		t.Error("review.md should list spec_alignment facet")
	}
	if !containsSubstring(content, "Acceptance Criteria") {
		t.Error("review.md should contain Acceptance Criteria section")
	}
	if !containsSubstring(content, "returns 200") {
		t.Error("review.md should list acceptance criteria")
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- **Step 2:** `go test ./internal/next/evidence/ -run TestBundler_WriteReview_IncludesReviewFindings -v` — expect FAIL

- **Step 3: Implement** — Add `ReviewFindingSummary` struct with `Facet`, `Count`, `Severities` fields. Add `AcceptanceCriterionSummary` struct with `Criterion`, `Status`, `Rationale` fields. Extend `ReviewInput` with `ReviewFindings []ReviewFindingSummary` and `AcceptanceCriteria []AcceptanceCriterionSummary`. Update `WriteReview` to render additional markdown sections: "## Review Findings" (table with facet/count/severities) and "## Acceptance Criteria" (table with criterion/status/rationale). Update `NormalizeNilFields` on `ReviewInput` to handle the new slices.

- **Step 4:** `go test ./internal/next/evidence/ -run TestBundler_WriteReview_IncludesReview -v` — expect PASS
- **Step 5:** Commit `"feat(next): extend review.md with review findings and acceptance criteria sections"`

---

### Task 31: EvidenceStage writes review.md summary from RunState strings

- **Files:**
  - Modify: `internal/next/specloop/stages/evidence.go`
  - Modify or create: `internal/next/specloop/stages/evidence_test.go`

- **Design note:** The structured evidence files (review.json, acceptance.json) are written directly by ReviewStage and AcceptStage as side effects of their `Run` methods, using the structured domain types they already hold. EvidenceStage does NOT write review.json or acceptance.json. Instead, EvidenceStage uses the `[]string` data from `rs.ReviewFindings` and `rs.AcceptanceResults` only for generating the human-readable `review.md` summary. This cleanly separates concerns: stages that have structured data write structured evidence; EvidenceStage assembles the human-readable summary.

- **Step 1: Write failing test** in `evidence_test.go`:
```go
package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestEvidenceStage_WritesReviewMDFromRunState(t *testing.T) {
	dir := t.TempDir()
	evidenceDir := filepath.Join(dir, "evidence")

	stage := NewEvidenceStage(EvidenceStageConfig{
		EvidenceDir: evidenceDir,
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.ReviewFindings = []string{"[spec_alignment] error: handler.go:42 — missing validation"}
	rs.AcceptanceResults = []string{"acceptance:fail: multi-currency — implement missing behavior"}

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Check review.md exists and contains finding/acceptance summary
	data, err := os.ReadFile(filepath.Join(evidenceDir, "review.md"))
	if err != nil {
		t.Fatalf("read review.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "spec_alignment") {
		t.Error("review.md should contain review findings")
	}
	if !strings.Contains(content, "multi-currency") {
		t.Error("review.md should contain acceptance results")
	}
}

func TestEvidenceStage_NoReviewData_SkipsReviewSections(t *testing.T) {
	dir := t.TempDir()
	evidenceDir := filepath.Join(dir, "evidence")

	stage := NewEvidenceStage(EvidenceStageConfig{
		EvidenceDir: evidenceDir,
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	// No ReviewFindings or AcceptanceResults set

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// review.md may exist but should not contain review/acceptance sections
	data, _ := os.ReadFile(filepath.Join(evidenceDir, "review.md"))
	content := string(data)
	if strings.Contains(content, "Review Findings") {
		t.Error("review.md should not have review findings section when no data")
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestEvidenceStage -v` — expect FAIL

- **Step 3: Implement** — Update `EvidenceStage.Run` to check `rs.ReviewFindings` and `rs.AcceptanceResults`. If non-empty, include them as sections in the `review.md` human-readable summary. EvidenceStage does NOT write review.json or acceptance.json — those are written by ReviewStage and AcceptStage directly using structured domain types. Skip review/acceptance sections in review.md if the slices are empty (backward compatible with 0002a runs that have no review/acceptance data).

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestEvidenceStage -v` — expect PASS
- **Step 5:** Commit `"feat(next): EvidenceStage writes review.json and acceptance.json artifacts"`

---

## Phase 8: Integration Wiring and CLI Updates

### Task 32: Wire review config from policy into ReviewStage

- **Files:**
  - Modify: `cmd/gromit-next/stage_provider.go`
  - Modify: `cmd/gromit-next/stage_provider_test.go`

- **Step 1: Write failing test** in `stage_provider_test.go`:
```go
func TestRealStageProvider_ReviewStageUsesPolicy(t *testing.T) {
	policy := execpolicy.DefaultPolicy()
	policy.Review.Facets = []string{"spec_alignment", "code_quality", "logic_gaps"}
	policy.Review.ReplanThreshold = "error"

	rs := runstore.NewRunState("test-spec", "test-project")

	provider := NewRealStageProvider(RealStageProviderConfig{
		WorkDir:  t.TempDir(),
		StoreDir: t.TempDir(),
		SpecPath: "test-spec.md",
	})

	stages, err := provider.BuildStages(policy, rs)
	if err != nil {
		t.Fatalf("BuildStages: %v", err)
	}

	// Find the review stage and verify it was wired
	var foundReview bool
	for _, s := range stages {
		if s.Name() == "review" {
			foundReview = true
		}
	}
	if !foundReview {
		t.Error("expected review stage to be present")
	}
}
```

- **Step 2:** `go test ./cmd/gromit-next/ -run TestRealStageProvider_ReviewStageUsesPolicy -v` — expect FAIL or PASS depending on Task 17 state

- **Step 3: Implement** — Update `RealStageProvider.BuildStages` to read `policy.Review` config and pass facets, threshold, and tier config when constructing the `ReviewStage`. Parse the threshold string to `review.Severity` using `review.ParseSeverity`. Configure `RunnerConfig` with the policy's facet list and threshold.

- **Step 4:** `go test ./cmd/gromit-next/ -run TestRealStageProvider_ReviewStageUsesPolicy -v` — expect PASS
- **Step 5:** Commit `"feat(next): wire review policy config into ReviewStage construction"`

---

### Task 33: Wire acceptance criteria from spec into AcceptStage

AcceptStage reads criteria at Run time from `<runDir>/spec.md` — the spec markdown that InitStage copies into the run directory (init.go line 71). At Run time, AcceptStage calls `acceptor.ParseAcceptanceCriteria(specMarkdown)` (from Task 16a) to extract criteria, then passes them to the evaluator. This avoids the problem of criteria not being available at construction time.

AcceptStageConfig needs a `Store` reference (or the run dir path) so it can locate `spec.md`. If `ParseAcceptanceCriteria` returns an error (e.g., the spec has no acceptance criteria section), AcceptStage returns `Blocked` with a descriptive message.

- **Files:**
  - Modify: `internal/next/specloop/stages/accept.go`
  - Modify or create: `internal/next/specloop/stages/accept_test.go`

- **Step 1: Write failing test** in `accept_test.go`:
```go
func TestAcceptStage_ReadsSpecFromRunDir(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	store.Save(rs)

	// Write spec.md into the run directory with an acceptance criteria section
	runDir := store.RunDir(rs.RunID)
	specContent := "# Spec\n\n## Acceptance Criteria\n\n- Feature X works\n- Feature Y returns correct output\n"
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec.md"), []byte(specContent), 0o644)

	stage := NewAcceptStage(AcceptStageConfig{
		Store: store,
		// ... other config (evaluator can be a mock/stub)
	})

	result, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// AcceptStage should have parsed criteria from spec.md
	// Verify criteria were extracted and passed to the evaluator
	if result.Status == stages.StageBlocked {
		t.Error("expected non-blocked result when spec.md has acceptance criteria")
	}
}

func TestAcceptStage_NoCriteriaSection_ReturnsBlocked(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	rs := runstore.NewRunState("test-spec", "test-project")
	store.Save(rs)

	// Write spec.md WITHOUT an acceptance criteria section
	runDir := store.RunDir(rs.RunID)
	specContent := "# Spec\n\nThis spec has no acceptance criteria section.\n"
	os.MkdirAll(runDir, 0o755)
	os.WriteFile(filepath.Join(runDir, "spec.md"), []byte(specContent), 0o644)

	stage := NewAcceptStage(AcceptStageConfig{
		Store: store,
	})

	result, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Status != stages.StageBlocked {
		t.Errorf("Status = %v, want StageBlocked when spec has no acceptance criteria", result.Status)
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestAcceptStage -v` — expect FAIL

- **Step 3: Implement** — Update `AcceptStageConfig` to include a `Store` field (type `*runstore.Store`). In `AcceptStage.Run`:
  1. Compute `runDir := cfg.Store.RunDir(rs.RunID)`
  2. Read `filepath.Join(runDir, "spec.md")`
  3. Call `acceptor.ParseAcceptanceCriteria(specMarkdown)` to extract criteria
  4. If `ParseAcceptanceCriteria` returns an error, return `StageBlocked` with the error message
  5. Pass the extracted criteria to the evaluator for acceptance checking

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestAcceptStage -v` — expect PASS
- **Step 5:** Commit `"feat(next): AcceptStage reads spec.md from run dir and parses acceptance criteria at Run time"`

---

### Task 34: Event log entries for review and acceptance

- **Files:**
  - Modify: `internal/next/runstore/events.go` (or wherever events are defined)
  - Create or modify: `internal/next/runstore/events_test.go`

- **Step 1: Write failing test** in `events_test.go`:
```go
package runstore

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReviewResultEvent_JSON(t *testing.T) {
	evt := ReviewResultEvent{
		BaseEvent:        BaseEvent{Type: "review_result", Timestamp: time.Now()},
		TotalFindings:    3,
		BlockingFindings: 1,
		FacetsReviewed:   []string{"spec_alignment", "code_quality"},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "review_result" {
		t.Errorf("type = %v, want review_result", got["type"])
	}
}

func TestAcceptanceResultEvent_JSON(t *testing.T) {
	evt := AcceptanceResultEvent{
		BaseEvent:    BaseEvent{Type: "acceptance_result", Timestamp: time.Now()},
		TotalCriteria: 5,
		PassCount:     4,
		FailCount:     1,
		UnclearCount:  0,
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "acceptance_result" {
		t.Errorf("type = %v, want acceptance_result", got["type"])
	}
}
```

- **Step 2:** `go test ./internal/next/runstore/ -run TestReviewResultEvent -v` — expect FAIL

- **Step 3: Implement** — Define `ReviewResultEvent` struct with `BaseEvent`, `TotalFindings int`, `BlockingFindings int`, `FacetsReviewed []string`. Define `AcceptanceResultEvent` struct with `BaseEvent`, `TotalCriteria int`, `PassCount int`, `FailCount int`, `UnclearCount int`. These events are emitted by `ReviewStage` and `AcceptStage` respectively, following the existing event pattern used by `FinalValidationResultEvent`. Note: the `unmarshalEvent` switch cases for `review_result` and `acceptance_result` (plus `blocked_worktree_cleaned`) are handled in Task 44.

- **Step 4:** `go test ./internal/next/runstore/ -run TestReviewResultEvent -v` — expect PASS
- **Step 5:** Commit `"feat(next): add review and acceptance event types for event log"`

---

### Task 35: ReviewStage and AcceptStage emit events

- **Files:**
  - Modify: `internal/next/specloop/stages/review.go`
  - Modify: `internal/next/specloop/stages/accept.go`
  - Modify: `internal/next/specloop/stages/review_test.go`
  - Modify: `internal/next/specloop/stages/accept_test.go`

- **Step 1: Write failing test** in `review_test.go`:
```go
func TestReviewStage_EmitsEvent(t *testing.T) {
	runner := &mockReviewRunner{
		result: review.RunResult{
			AllFindings: []review.Finding{
				{Severity: review.SeverityInfo, File: "handler.go", Description: "info note"},
			},
			BlockingFindings:   []review.Finding{},
			HasBlockingFindings: false,
			FindingsByFacet: map[string][]review.Finding{
				"code_quality": {{Severity: review.SeverityInfo}},
			},
		},
	}

	eventLog := runstore.NewEventLog(filepath.Join(t.TempDir(), "events.jsonl"))
	stage := NewReviewStage(runner, ReviewStageConfig{}, eventLog)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	events := eventLog.ReadAll()
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
}
```

And in `accept_test.go`:
```go
func TestAcceptStage_EmitsEvent(t *testing.T) {
	eval := &mockAcceptEvaluator{
		result: acceptor.AcceptanceResult{
			Results: []acceptor.CriterionResult{
				{Criterion: "x", Status: acceptor.StatusPass},
			},
			AllPass: true,
		},
	}

	eventLog := runstore.NewEventLog(filepath.Join(t.TempDir(), "events.jsonl"))
	stage := NewAcceptStage(eval, AcceptStageConfig{Criteria: []string{"x"}}, eventLog)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	events := eventLog.ReadAll()
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run "TestReviewStage_EmitsEvent|TestAcceptStage_EmitsEvent" -v` — expect FAIL

- **Step 3: Implement** — In `ReviewStage.Run`, after computing results, emit a `ReviewResultEvent` via `eventLog.Append()`. In `AcceptStage.Run`, after computing results, emit an `AcceptanceResultEvent`. Follow the existing pattern from `ValidateStage` which emits `FinalValidationResultEvent`.

- **Step 4:** `go test ./internal/next/specloop/stages/ -run "TestReviewStage_EmitsEvent|TestAcceptStage_EmitsEvent" -v` — expect PASS
- **Step 5:** Commit `"feat(next): ReviewStage and AcceptStage emit event log entries"`

---

### Task 36: Integration test — review finding blocks ready_for_review

- **Files:**
  - Create: `internal/next/specloop/stages/review_integration_test.go`

- **Step 1: Write failing test** in `review_integration_test.go`:
```go
package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestIntegration_ReviewBlocksReadyForReview(t *testing.T) {
	// Build a minimal pipeline: validate (pass) -> review (blocking finding) -> finalize
	validateStage := &passStage{name: "validate"}
	reviewRunner := &mockReviewRunner{
		result: review.RunResult{
			AllFindings: []review.Finding{
				{Facet: "spec_alignment", Severity: review.SeverityError, File: "handler.go", Description: "missing validation"},
			},
			BlockingFindings: []review.Finding{
				{Facet: "spec_alignment", Severity: review.SeverityError, File: "handler.go", Description: "missing validation"},
			},
			HasBlockingFindings: true,
			FindingsByFacet: map[string][]review.Finding{
				"spec_alignment": {{Severity: review.SeverityError, File: "handler.go", Description: "missing validation"}},
			},
		},
	}

	reviewStage := NewReviewStage(reviewRunner, ReviewStageConfig{}, nil)
	planStage := &passStage{name: "plan"}

	stages := []specloop.Stage{planStage, validateStage, reviewStage}

	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0}) // maxSpecCycles=1 so it exhausts
	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("loop.Run: %v", err)
	}

	if rs.Status == runstore.StatusReadyForReview {
		t.Error("review blocking findings should prevent ready_for_review")
	}
	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("expected needs_human when budget exhausted with blocking findings, got %q", rs.Status)
	}
}

type passStage struct {
	name string
}

func (s *passStage) Name() string { return s.name }
func (s *passStage) Run(_ context.Context, _ *runstore.RunState) (specloop.NextAction, error) {
	return specloop.NextAction{Kind: specloop.Continue}, nil
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestIntegration_ReviewBlocksReadyForReview -v` — expect FAIL

- **Step 3: Implement** — This is an integration test. If it fails, the issue is likely in how the SpecLoop handles `ReplanFrom` from ReviewStage combined with budget exhaustion. Fix any gaps in the loop's handling of review-triggered replans.

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestIntegration_ReviewBlocksReadyForReview -v` — expect PASS
- **Step 5:** Commit `"test(next): integration test verifying review blocks ready_for_review"`

---

### Task 37: Integration test — acceptance fail triggers fix cycle

- **Files:**
  - Create: `internal/next/specloop/stages/accept_integration_test.go`

- **Step 1: Write failing test** in `accept_integration_test.go`:
```go
package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestIntegration_AcceptFailTriggersReplan(t *testing.T) {
	planCallCount := 0
	planStage := &callbackStageInteg{
		name: "plan",
		fn: func(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
			planCallCount++
			return specloop.NextAction{Kind: specloop.Continue}, nil
		},
	}

	validateStage := &passStage{name: "validate"}
	reviewStage := &passStage{name: "review"}

	// First call: fail. Second call: pass.
	acceptCallCount := 0
	acceptEval := &callbackAcceptEvaluator{
		fn: func(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
			acceptCallCount++
			if acceptCallCount == 1 {
				return acceptor.AcceptanceResult{
					Results:          []acceptor.CriterionResult{{Criterion: "x", Status: acceptor.StatusFail, Rationale: "not done"}},
					HasFailOrUnclear: true,
				}, nil
			}
			return acceptor.AcceptanceResult{
				Results: []acceptor.CriterionResult{{Criterion: "x", Status: acceptor.StatusPass}},
				AllPass: true,
			}, nil
		},
	}

	acceptStage := NewAcceptStage(acceptEval, AcceptStageConfig{Criteria: []string{"x"}}, nil)

	stages := []specloop.Stage{planStage, validateStage, reviewStage, acceptStage}
	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 3, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0})
	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("loop.Run: %v", err)
	}

	if planCallCount < 2 {
		t.Errorf("expected plan to be called at least 2 times (initial + replan), got %d", planCallCount)
	}
	if rs.Status != runstore.StatusReadyForReview {
		t.Errorf("expected ready_for_review after fix cycle, got %q", rs.Status)
	}
}

type callbackStageInteg struct {
	name string
	fn   func(context.Context, *runstore.RunState) (specloop.NextAction, error)
}

func (s *callbackStageInteg) Name() string { return s.name }
func (s *callbackStageInteg) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	return s.fn(ctx, rs)
}

type callbackAcceptEvaluator struct {
	fn func(context.Context, acceptor.EvaluateInput) (acceptor.AcceptanceResult, error)
}

func (c *callbackAcceptEvaluator) Evaluate(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
	return c.fn(ctx, input)
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestIntegration_AcceptFailTriggersReplan -v` — expect FAIL

- **Step 3: Implement** — This is an integration test validating the end-to-end flow. Fix any issues in how SpecLoop handles `ReplanFrom` from `AcceptStage` — it should loop back to the plan stage, re-execute, re-validate, re-review, and re-accept. The second acceptance should pass, resulting in `ready_for_review`.

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestIntegration_AcceptFailTriggersReplan -v` — expect PASS
- **Step 5:** Commit `"test(next): integration test verifying acceptance fail triggers fix cycle"`

---

### Task 38: Integration test — budget exhaustion with acceptance unclear

- **Files:**
  - Modify: `internal/next/specloop/stages/accept_integration_test.go`

- **Step 1: Write failing test** in `accept_integration_test.go`:
```go
func TestIntegration_AcceptUnclear_BudgetExhaustion_NeedsHuman(t *testing.T) {
	planStage := &passStage{name: "plan"}
	validateStage := &passStage{name: "validate"}
	reviewStage := &passStage{name: "review"}

	// Always returns unclear
	acceptEval := &callbackAcceptEvaluator{
		fn: func(ctx context.Context, input acceptor.EvaluateInput) (acceptor.AcceptanceResult, error) {
			return acceptor.AcceptanceResult{
				Results: []acceptor.CriterionResult{
					{Criterion: "audit log", Status: acceptor.StatusUnclear, Rationale: "no test verifies it"},
				},
				HasFailOrUnclear: true,
			}, nil
		},
	}

	acceptStage := NewAcceptStage(acceptEval, AcceptStageConfig{Criteria: []string{"audit log"}}, nil)

	stages := []specloop.Stage{planStage, validateStage, reviewStage, acceptStage}
	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 2, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0}) // 2 cycles max
	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("loop.Run: %v", err)
	}

	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("expected needs_human after budget exhaustion with unclear, got %q", rs.Status)
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestIntegration_AcceptUnclear_BudgetExhaustion -v` — expect FAIL

- **Step 3: Implement** — This is an integration test. Budget exhaustion with remaining failures should result in `needs_human`. Fix any issues in how the SpecLoop transitions to `needs_human` when budget is exhausted and acceptance criteria remain unclear.

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestIntegration_AcceptUnclear_BudgetExhaustion -v` — expect PASS
- **Step 5:** Commit `"test(next): integration test verifying unclear acceptance + budget exhaustion = needs_human"`

---

### Task 39: Integration test — configurable threshold changes blocking behavior

- **Files:**
  - Create: `internal/next/review/threshold_integration_test.go`

- **Step 1: Write failing test** in `threshold_integration_test.go`:
```go
package review

import (
	"context"
	"testing"
)

func TestIntegration_ThresholdError_WarningsNonBlocking(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"code_quality": {
				{Severity: SeverityWarning, File: "handler.go", Description: "duplicate logic"},
				{Severity: SeverityWarning, File: "router.go", Description: "long function"},
			},
		},
	}

	// Threshold set to "error" — only errors block
	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"code_quality"},
		Threshold: SeverityError,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.HasBlockingFindings {
		t.Error("warnings should NOT block at error threshold")
	}
	if len(result.AllFindings) != 2 {
		t.Errorf("all findings should still be recorded, got %d", len(result.AllFindings))
	}
}

func TestIntegration_ThresholdSuggestion_WarningsBlock(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"code_quality": {
				{Severity: SeverityWarning, File: "handler.go", Description: "duplicate logic"},
			},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"code_quality"},
		Threshold: SeveritySuggestion,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !result.HasBlockingFindings {
		t.Error("warnings SHOULD block at suggestion threshold")
	}
}

type mockReviewAgent struct {
	findings map[string][]Finding
}

func (m *mockReviewAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
	return m.findings[facetName], nil
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestIntegration_Threshold -v` — expect FAIL

- **Step 3: Implement** — This is an integration test verifying the threshold logic works end-to-end through the runner. If the runner correctly uses `FilterBlockingFindings` with the configured threshold, both tests should pass.

- **Step 4:** `go test ./internal/next/review/ -run TestIntegration_Threshold -v` — expect PASS
- **Step 5:** Commit `"test(next): integration test verifying configurable threshold behavior"`

---

### Task 40: Integration test — adding facet via config (no code change)

- **Files:**
  - Create: `internal/next/review/facet_integration_test.go`

- **Step 1: Write failing test** in `facet_integration_test.go`:
```go
package review

import (
	"context"
	"testing"
)

func TestIntegration_EnableFacetViaConfig(t *testing.T) {
	// Simulate enabling logic_gaps facet (not in default 2)
	agent := &facetCapturingAgent{
		reviewedFacets: make(map[string]bool),
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"spec_alignment", "code_quality", "logic_gaps"},
		Threshold: SeveritySuggestion,
	})

	_, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, facet := range []string{"spec_alignment", "code_quality", "logic_gaps"} {
		if !agent.reviewedFacets[facet] {
			t.Errorf("facet %q should have been reviewed", facet)
		}
	}
}

type facetCapturingAgent struct {
	reviewedFacets map[string]bool
}

func (a *facetCapturingAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
	a.reviewedFacets[facetName] = true
	return nil, nil
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestIntegration_EnableFacetViaConfig -v` — expect FAIL

- **Step 3: Implement** — This is an integration test verifying that the runner invokes all configured facets from the registry. If the runner iterates `config.Facets` and calls the agent for each, this should pass without additional implementation.

- **Step 4:** `go test ./internal/next/review/ -run TestIntegration_EnableFacetViaConfig -v` — expect PASS
- **Step 5:** Commit `"test(next): integration test verifying facet enablement via config"`

---

### Task 41: Integration test — new-vs-preexisting finding matching in fix cycles

- **Files:**
  - Create: `internal/next/review/matching_integration_test.go`

- **Step 1: Write failing test** in `matching_integration_test.go`:
```go
package review

import (
	"context"
	"testing"
)

func TestIntegration_FixCycle_PreExistingFindingsNotBlocking(t *testing.T) {
	// Cycle 2: agent returns same finding as cycle 1
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"spec_alignment": {{Severity: SeverityWarning, File: "handler.go", Description: "missing validation check"}},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"spec_alignment"},
		Threshold: SeveritySuggestion,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Fixed handler",
		SpecContent: "# Spec",
		Cycle:       2,
		PriorFindings: []Finding{
			{Severity: SeverityWarning, File: "handler.go", Description: "missing validation check", Cycle: 1},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Pre-existing finding should not block
	if result.HasBlockingFindings {
		t.Error("pre-existing finding should not trigger blocking on fix cycle")
	}
	// But should still be recorded
	if len(result.AllFindings) != 1 {
		t.Errorf("expected 1 finding recorded, got %d", len(result.AllFindings))
	}
	if result.AllFindings[0].Disposition != DispositionPreExisting {
		t.Errorf("disposition should be pre-existing, got %q", result.AllFindings[0].Disposition)
	}
}

func TestIntegration_FixCycle_NewFindingStillBlocks(t *testing.T) {
	// Cycle 2: agent returns a new finding not seen in cycle 1
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"code_quality": {{Severity: SeverityWarning, File: "handler.go", Description: "duplicated error handling"}},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"code_quality"},
		Threshold: SeveritySuggestion,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Fixed handler",
		SpecContent: "# Spec",
		Cycle:       2,
		PriorFindings: []Finding{
			{Severity: SeverityError, File: "handler.go", Description: "missing nil check", Cycle: 1},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !result.HasBlockingFindings {
		t.Error("new finding on fix cycle should still block")
	}
	if result.AllFindings[0].Disposition != DispositionNew {
		t.Errorf("disposition should be new, got %q", result.AllFindings[0].Disposition)
	}
}
```

- **Step 2:** `go test ./internal/next/review/ -run TestIntegration_FixCycle -v` — expect FAIL

- **Step 3: Implement** — This is an integration test verifying the full flow: runner receives prior findings, labels dispositions, and only counts new findings as blocking. If the runner's fix-cycle logic (from Task 10) correctly uses `LabelDispositions` and filters blocking findings to exclude pre-existing ones, these tests should pass.

- **Step 4:** `go test ./internal/next/review/ -run TestIntegration_FixCycle -v` — expect PASS
- **Step 5:** Commit `"test(next): integration test verifying new-vs-preexisting matching in fix cycles"`

---

## Phase 9: FinalizeStage Updates and Blocked Worktree Lifecycle

### Task 42: FinalizeStage — updated ready_for_review condition

- **Files:**
  - Modify: `internal/next/specloop/stages/finalize.go`
  - Modify or create: `internal/next/specloop/stages/finalize_test.go`

- **Step 1: Write failing test** in `finalize_test.go`:
```go
func TestFinalizeStage_ReadyForReview_RequiresAllGates(t *testing.T) {
	tests := []struct {
		name       string
		rs         *runstore.RunState
		wantStatus string
	}{
		{
			name: "all pass -> ready_for_review",
			rs: &runstore.RunState{
				// Set up RunState so FinalizeStage computes allDone=true
				// (add completed tasks to rs.Tasks or set appropriate fields)
				FinalValidationPassed: true,
				FinalReviewPassed:     true,
				FinalAcceptancePassed: true,
			},
			wantStatus: runstore.StatusReadyForReview,
		},
		{
			name: "review failed -> needs_human",
			rs: &runstore.RunState{
				// Set up RunState so FinalizeStage computes allDone=true
				// (add completed tasks to rs.Tasks or set appropriate fields)
				FinalValidationPassed: true,
				FinalReviewPassed:     false,
				FinalAcceptancePassed: true,
			},
			wantStatus: runstore.StatusNeedsHuman,
		},
		{
			name: "acceptance failed -> needs_human",
			rs: &runstore.RunState{
				// Set up RunState so FinalizeStage computes allDone=true
				// (add completed tasks to rs.Tasks or set appropriate fields)
				FinalValidationPassed: true,
				FinalReviewPassed:     true,
				FinalAcceptancePassed: false,
			},
			wantStatus: runstore.StatusNeedsHuman,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Constructor: NewFinalizeStage(gitOps GitOps, store *runstore.Store, eventLog *runstore.EventLog)
			stage := NewFinalizeStage(nil, nil, nil) // nil deps acceptable for unit test
			_, err := stage.Run(context.Background(), tt.rs)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if tt.rs.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", tt.rs.Status, tt.wantStatus)
			}
		})
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestFinalizeStage_ReadyForReview_RequiresAllGates -v` — expect FAIL

- **Step 3: Implement** — Update FinalizeStage to require all four conditions for `ready_for_review`: `allDone && rs.FinalValidationPassed && rs.FinalReviewPassed && rs.FinalAcceptancePassed`. When any gate fails, set status to `needs_human`. Preserve worktrees for ALL terminal states including `needs_human` — remove the `RemoveWorktree` call for needs_human runs. Only remove worktrees on `ready_for_review` (successful completion).

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestFinalizeStage_ReadyForReview -v` — expect PASS
- **Step 5:** Commit `"feat(next): FinalizeStage requires review+acceptance gates for ready_for_review"`

---

### Task 43: InitStage — clean up prior blocked worktrees

- **Files:**
  - Modify: `internal/next/specloop/stages/init.go`
  - Modify or create: `internal/next/specloop/stages/init_test.go`

- **Step 1: Write failing test** in `init_test.go`:
```go
func TestInitStage_CleansBlockedWorktrees(t *testing.T) {
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)

	// Create a prior run with the SAME project ID so store.List finds it
	priorRS := runstore.NewRunState("test-spec", "test-project")
	priorRS.Status = runstore.StatusBlocked
	priorRS.WorktreePath = filepath.Join(t.TempDir(), "old-worktree")
	os.MkdirAll(priorRS.WorktreePath, 0o755)
	store.Save(priorRS)

	stage := NewInitStage(InitStageConfig{
		Store: store,
	})
	newRS := runstore.NewRunState("test-spec", "test-project")

	_, err := stage.Run(context.Background(), newRS)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Prior blocked run's worktree should be removed
	if _, err := os.Stat(priorRS.WorktreePath); !os.IsNotExist(err) {
		t.Error("blocked worktree should have been removed")
	}

	// Prior run's worktree_path should be cleared in store
	reloaded, _ := store.Load(priorRS.RunID)
	if reloaded.WorktreePath != "" {
		t.Error("worktree_path should be cleared in run.json")
	}
}
```

- **Step 2:** `go test ./internal/next/specloop/stages/ -run TestInitStage_CleansBlockedWorktrees -v` — expect FAIL

- **Step 3: Implement** — In InitStage, when creating a new run, call `store.List(rs.ProjectID)` to get all runs for the project. Iterate the results and filter client-side for runs where `run.SpecID == rs.SpecID && run.Status == StatusBlocked && run.WorktreePath != ""`. No new Store method is needed — client-side filtering is sufficient since the number of runs per project is small. For each matching run: remove the worktree directory, clear `worktree_path` in the run state, save the updated run, and emit a `blocked_worktree_cleaned` event.

- **Step 4:** `go test ./internal/next/specloop/stages/ -run TestInitStage_CleansBlockedWorktrees -v` — expect PASS
- **Step 5:** Commit `"feat(next): InitStage cleans up prior blocked worktrees on new run"`

---

### Task 44: Event types — blocked_worktree_cleaned and unmarshalEvent switch cases

- **Files:**
  - Modify: `internal/next/runstore/events.go`
  - Modify: `internal/next/runstore/events_test.go`

- **Step 1: Write failing test** in `events_test.go`:
```go
func TestBlockedWorktreeCleanedEvent_JSON(t *testing.T) {
	evt := BlockedWorktreeCleanedEvent{
		BaseEvent:      BaseEvent{Type: "blocked_worktree_cleaned", Timestamp: time.Now()},
		PriorRunID:     "run-abc-123",
		WorktreePath:   "/path/to/old-worktree",
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "blocked_worktree_cleaned" {
		t.Errorf("type = %v, want blocked_worktree_cleaned", got["type"])
	}
}

func TestUnmarshalEvent_ReviewAndAcceptanceTypes(t *testing.T) {
	reviewJSON := `{"type":"review_result","timestamp":"2026-03-12T00:00:00Z","total_findings":3,"blocking_findings":1}`
	evt, err := unmarshalEvent([]byte(reviewJSON))
	if err != nil {
		t.Fatalf("unmarshalEvent review_result: %v", err)
	}
	if _, ok := evt.(*ReviewResultEvent); !ok {
		t.Errorf("expected *ReviewResultEvent, got %T", evt)
	}

	acceptJSON := `{"type":"acceptance_result","timestamp":"2026-03-12T00:00:00Z","total_criteria":5,"pass_count":4}`
	evt2, err := unmarshalEvent([]byte(acceptJSON))
	if err != nil {
		t.Fatalf("unmarshalEvent acceptance_result: %v", err)
	}
	if _, ok := evt2.(*AcceptanceResultEvent); !ok {
		t.Errorf("expected *AcceptanceResultEvent, got %T", evt2)
	}
}
```

- **Step 2:** `go test ./internal/next/runstore/ -run "TestBlockedWorktreeCleanedEvent|TestUnmarshalEvent_ReviewAndAcceptance" -v` — expect FAIL

- **Step 3: Implement** — Define `BlockedWorktreeCleanedEvent` struct with `BaseEvent`, `PriorRunID string`, `WorktreePath string`. Add `unmarshalEvent` switch cases for `"review_result"` (-> `*ReviewResultEvent`), `"acceptance_result"` (-> `*AcceptanceResultEvent`), and `"blocked_worktree_cleaned"` (-> `*BlockedWorktreeCleanedEvent`).

- **Step 4:** `go test ./internal/next/runstore/ -run "TestBlockedWorktreeCleanedEvent|TestUnmarshalEvent" -v` — expect PASS
- **Step 5:** Commit `"feat(next): add blocked_worktree_cleaned event and unmarshalEvent switch cases"`

---

### Task 45: Per-task artifact files

- **Files:**
  - Modify: `internal/next/specloop/stages/execute.go` (or relevant executor integration)
  - Create or modify: `internal/next/specloop/stages/execute_test.go`

- **Note:** As part of the executor integration, each task execution should write per-task artifacts to the run's evidence directory:
  - `task-packet.md` — the rendered task prompt sent to the agent
  - `agent-output.txt` — the raw agent response

  These files live under `<evidence-dir>/tasks/<task-id>/` and provide debugging traceability. The `EvidenceStage` does not need to be modified — artifacts are written inline during execution. This is a wiring concern for the `ExecuteStage` or whichever stage invokes the agent per-task.
