# Spec-Level Review: Findings Conversion Remediation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the findings conversion inconsistencies identified in the gap analysis: Title field dropped in `remediation.go`'s `convertSpecFindings`, AffectedFiles lost for spec-review findings, severity mapping inconsistency between `spec_loop.go` and `remediation.go`, and dead code removal.

**Architecture:** The remediation runner has its own `convertSpecFindings` function that predates the spec_loop version and is missing fields. The fix aligns remediation.go's conversion to match spec_loop.go's canonical `convertToFinding` pattern, adds `AffectedFiles` to `SpecFinding` so it flows through the entire pipeline, and removes dead code.

**Tech Stack:** Go 1.24+, TDD with `go test`

---

## Architecture

Two conversion paths exist for `SpecFinding → finding.Finding`:

1. **spec_loop.go** — `convertSpecFindings()` (line 873) calls `convertToFinding()` which preserves Title, maps `high→critical`, and passes AffectedFiles (currently nil because `SpecFinding` has no `AffectedFiles` field).

2. **remediation.go** — `convertSpecFindings()` (line 355) builds `finding.Finding` inline, drops Title, groups `high+medium→warning`, never includes AffectedFiles.

**Root cause:** `stage.SpecFinding` lacks `AffectedFiles []string`. The specreview stage produces `SpecReviewFinding` (which has `AffectedFiles`) but converts to `SpecFinding` (which doesn't), silently dropping file info.

**Fix strategy:**
1. Add `AffectedFiles` to `SpecFinding` with nil-field normalization
2. Align `remediation.go`'s `convertSpecFindings` to preserve Title and use consistent severity mapping
3. Wire `AffectedFiles` through all conversion points
4. Remove dead code (`findingsProvider`, `extractFindings`, duplicate comment)

## Test Strategy

- Unit tests for each conversion function change in `remediation_test.go`
- Unit tests for `SpecFinding.NormalizeNilFields()` in existing stage test file
- Run `go test ./internal/v2/remediation/... ./internal/v2/loop/... ./internal/v2/stage/...` after each task
- Final `go test ./...` to confirm no regressions

---

## Implementation Tasks

### Task 1: Add AffectedFiles to SpecFinding and normalize nil fields

**Files:**
- Modify: `internal/v2/stage/stage.go:154-161`
- Test: `internal/v2/stage/stage_test.go` (or `internal/v2/loop/spec_loop_test.go` if stage_test.go doesn't exist)

**Step 1: Write the failing test**

Create or add to a test file for the stage package:

```go
func TestSpecFinding_NormalizeNilFields(t *testing.T) {
	t.Parallel()
	f := SpecFinding{Title: "test", Description: "desc"}
	f.NormalizeNilFields()
	if f.AffectedFiles == nil {
		t.Fatal("AffectedFiles should be non-nil empty slice after normalization")
	}
	if len(f.AffectedFiles) != 0 {
		t.Fatalf("AffectedFiles length = %d, want 0", len(f.AffectedFiles))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go build ./internal/v2/stage/...`
Expected: FAIL — `SpecFinding` has no field `AffectedFiles` and no method `NormalizeNilFields`

**Step 3: Add AffectedFiles field and NormalizeNilFields method**

In `stage.go`, add `AffectedFiles` to `SpecFinding`:

```go
type SpecFinding struct {
	Title         string
	Description   string
	Severity      SpecFindingSeverity
	Category      SpecFindingCategory
	Scope         SpecFindingScope
	AffectedFiles []string
}

// NormalizeNilFields ensures slice fields are non-nil.
func (f *SpecFinding) NormalizeNilFields() {
	if f == nil {
		return
	}
	if f.AffectedFiles == nil {
		f.AffectedFiles = []string{}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/stage/... -v -run TestSpecFinding`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/stage/stage.go internal/v2/stage/stage_test.go
git commit -m "feat: add AffectedFiles field and NormalizeNilFields to SpecFinding"
```

**Acceptance Criteria:**
- `SpecFinding` has `AffectedFiles []string` field
- `NormalizeNilFields` maps nil to empty slice
- `go build ./internal/v2/stage/...` succeeds

---

### Task 2: Wire AffectedFiles through specReviewSpecFindings in remediation.go

**Files:**
- Modify: `internal/v2/remediation/remediation.go:428-447`
- Test: `internal/v2/remediation/remediation_test.go`

**Step 1: Write the failing test**

```go
func TestSpecReviewSpecFindingsPreservesAffectedFiles(t *testing.T) {
	t.Parallel()
	runner := NewRemediationRunner(RemediationRunnerConfig{})
	reviewFinding := specreview.SpecReviewFinding{
		Title:         "dead code",
		Description:   "Remove unused function",
		Severity:      stage.SpecFindingSeverityWarning,
		Category:      stage.SpecFindingCategoryQuality,
		Scope:         stage.SpecFindingScopeSpec,
		AffectedFiles: []string{"internal/v2/foo.go", "internal/v2/bar.go"},
	}
	res := &stage.Result{
		Decision: stage.DecisionFail,
		Artifacts: &specreview.SpecReviewArtifacts{
			Findings: []specreview.SpecReviewFinding{reviewFinding},
		},
	}
	got := runner.specReviewSpecFindings(res)
	if len(got) != 1 {
		t.Fatalf("findings count = %d, want 1", len(got))
	}
	if len(got[0].AffectedFiles) != 2 {
		t.Fatalf("affected files count = %d, want 2", len(got[0].AffectedFiles))
	}
	if got[0].AffectedFiles[0] != "internal/v2/foo.go" {
		t.Fatalf("affected file[0] = %q, want %q", got[0].AffectedFiles[0], "internal/v2/foo.go")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/remediation/... -v -run TestSpecReviewSpecFindingsPreservesAffectedFiles`
Expected: FAIL — `AffectedFiles` not populated in the conversion

**Step 3: Add AffectedFiles to the conversion**

In `remediation.go`, update `specReviewSpecFindings` (line 438-444):

```go
result = append(result, stage.SpecFinding{
	Title:         strings.TrimSpace(raw.Title),
	Description:   strings.TrimSpace(raw.Description),
	Severity:      raw.Severity,
	Category:      raw.Category,
	Scope:         raw.Scope,
	AffectedFiles: append([]string(nil), raw.AffectedFiles...),
})
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/remediation/... -v -run TestSpecReviewSpecFindingsPreservesAffectedFiles`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/remediation/remediation.go internal/v2/remediation/remediation_test.go
git commit -m "fix: preserve AffectedFiles in specReviewSpecFindings conversion"
```

**Acceptance Criteria:**
- `specReviewSpecFindings` copies `AffectedFiles` from `SpecReviewFinding` to `SpecFinding`
- Test verifies round-trip preservation

---

### Task 3: Fix convertSpecFindings in remediation.go — add Title and align severity mapping

**Files:**
- Modify: `internal/v2/remediation/remediation.go:355-395`
- Test: `internal/v2/remediation/remediation_test.go`

**Step 1: Write the failing test for Title preservation**

```go
func TestConvertSpecFindingsPreservesTitle(t *testing.T) {
	t.Parallel()
	src := []stage.SpecFinding{{
		Title:         "missing error check",
		Description:   "Function returns error but caller ignores it",
		Severity:      stage.SpecFindingSeverityCritical,
		Category:      stage.SpecFindingCategoryQuality,
		Scope:         stage.SpecFindingScopeSpec,
		AffectedFiles: []string{"internal/handler.go"},
	}}
	got := convertSpecFindings(src)
	if len(got) != 1 {
		t.Fatalf("findings count = %d, want 1", len(got))
	}
	if got[0].Title != "missing error check" {
		t.Fatalf("title = %q, want %q", got[0].Title, "missing error check")
	}
	if len(got[0].AffectedFiles) != 1 {
		t.Fatalf("affected files = %d, want 1", len(got[0].AffectedFiles))
	}
	if got[0].AffectedFiles[0] != "internal/handler.go" {
		t.Fatalf("affected file = %q, want %q", got[0].AffectedFiles[0], "internal/handler.go")
	}
}
```

**Step 2: Write the failing test for severity mapping alignment**

```go
func TestConvertSpecFindingsHighSeverityMapsToCritical(t *testing.T) {
	t.Parallel()
	src := []stage.SpecFinding{{
		Title:    "high sev",
		Severity: stage.SpecFindingSeverityHigh,
		Category: stage.SpecFindingCategoryQuality,
	}}
	got := convertSpecFindings(src)
	if len(got) != 1 {
		t.Fatalf("count = %d, want 1", len(got))
	}
	if got[0].Severity != finding.SeverityCritical {
		t.Fatalf("severity = %q, want %q", got[0].Severity, finding.SeverityCritical)
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `go test ./internal/v2/remediation/... -v -run "TestConvertSpecFindings"`
Expected: FAIL — Title is empty, High maps to Warning not Critical

**Step 4: Rewrite convertSpecFindings and mapSpecSeverity**

Replace `convertSpecFindings` (lines 355-369):

```go
func convertSpecFindings(src []stage.SpecFinding) []finding.Finding {
	if len(src) == 0 {
		return nil
	}
	converted := make([]finding.Finding, 0, len(src))
	for _, spec := range src {
		converted = append(converted, finding.Finding{
			Title:         strings.TrimSpace(spec.Title),
			Severity:      mapSpecSeverity(spec.Severity),
			Category:      mapSpecCategory(spec.Category),
			Scope:         strings.TrimSpace(string(spec.Scope)),
			Description:   strings.TrimSpace(spec.Description),
			AffectedFiles: append([]string(nil), spec.AffectedFiles...),
		})
	}
	return converted
}
```

Replace `mapSpecSeverity` (lines 371-380) to align with `spec_loop.go`'s `convertSeverity`:

```go
func mapSpecSeverity(severity stage.SpecFindingSeverity) finding.Severity {
	switch severity {
	case stage.SpecFindingSeverityCritical:
		return finding.SeverityCritical
	case stage.SpecFindingSeverityHigh:
		return finding.SeverityCritical
	case stage.SpecFindingSeverityMedium:
		return finding.SeverityWarning
	case stage.SpecFindingSeverityLow:
		return finding.SeveritySuggestion
	default:
		return finding.SeveritySuggestion
	}
}
```

Also update `mapSpecCategory` (lines 382-395) to handle the new category constants:

```go
func mapSpecCategory(category stage.SpecFindingCategory) finding.Category {
	switch category {
	case stage.SpecFindingCategoryAcceptance:
		return finding.CategoryAcceptance
	case stage.SpecFindingCategoryScope, stage.SpecFindingCategoryArchitecture:
		return finding.CategoryArchitecture
	case stage.SpecFindingCategoryQuality:
		return finding.CategoryQuality
	case stage.SpecFindingCategorySafety, stage.SpecFindingCategorySecurity:
		return finding.CategorySecurity
	case stage.SpecFindingCategoryTestGap:
		return finding.CategoryTestGap
	default:
		return finding.CategoryQuality
	}
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/v2/remediation/... -v -run "TestConvertSpecFindings"`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/v2/remediation/remediation.go internal/v2/remediation/remediation_test.go
git commit -m "fix: preserve Title and AffectedFiles in remediation convertSpecFindings, align severity mapping"
```

**Acceptance Criteria:**
- `convertSpecFindings` populates Title from `spec.Title`
- `convertSpecFindings` copies AffectedFiles
- `mapSpecSeverity` maps High→Critical (matching spec_loop.go)
- `mapSpecCategory` handles Security, TestGap, Architecture categories
- All existing remediation tests still pass

---

### Task 4: Wire AffectedFiles through spec_loop.go convertSpecFindings

**Files:**
- Modify: `internal/v2/loop/spec_loop.go:873-882`
- Test: `internal/v2/loop/spec_loop_test.go`

**Step 1: Write the failing test**

Add to `spec_loop_test.go` or `spec_loop_specreview_test.go`:

```go
func TestConvertSpecFindingsPreservesAffectedFiles(t *testing.T) {
	t.Parallel()
	src := []stagepkg.SpecFinding{{
		Title:         "test issue",
		Description:   "desc",
		Severity:      stagepkg.SpecFindingSeverityCritical,
		Category:      stagepkg.SpecFindingCategoryQuality,
		Scope:         stagepkg.SpecFindingScopeSpec,
		AffectedFiles: []string{"file1.go", "file2.go"},
	}}
	got := convertSpecFindings(src)
	if len(got) != 1 {
		t.Fatalf("count = %d, want 1", len(got))
	}
	if len(got[0].AffectedFiles) != 2 {
		t.Fatalf("affected files = %d, want 2", len(got[0].AffectedFiles))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/v2/loop/... -v -run TestConvertSpecFindingsPreservesAffectedFiles`
Expected: FAIL — AffectedFiles is nil (line 879 passes `nil`)

**Step 3: Pass AffectedFiles through convertToFinding**

Change line 879 in `spec_loop.go`:

From:
```go
out = append(out, convertToFinding(entry.Title, entry.Severity, entry.Category, entry.Scope, entry.Description, nil))
```
To:
```go
out = append(out, convertToFinding(entry.Title, entry.Severity, entry.Category, entry.Scope, entry.Description, entry.AffectedFiles))
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/v2/loop/... -v -run TestConvertSpecFindingsPreservesAffectedFiles`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/loop/spec_loop.go internal/v2/loop/spec_loop_test.go
git commit -m "fix: pass AffectedFiles through spec_loop convertSpecFindings"
```

**Acceptance Criteria:**
- `convertSpecFindings` in spec_loop.go passes `entry.AffectedFiles` to `convertToFinding`
- Test verifies round-trip

---

### Task 5: Remove dead code from remediation.go

**Files:**
- Modify: `internal/v2/remediation/remediation.go`

**Step 1: Verify the dead code is unused**

Run: `grep -n 'findingsProvider\|extractFindings' internal/v2/remediation/remediation.go`
Expected: Only the definition lines, no callers

Run: `grep -rn 'findingsProvider\|extractFindings' internal/v2/` to confirm no external callers exist.

**Step 2: Remove dead code**

Delete the following from `remediation.go`:

1. **`findingsProvider` interface** (lines 56-58):
```go
type findingsProvider interface {
	GetFindings() []stage.Finding
}
```

2. **Duplicate comment block** (lines 60-63) — the `planContentProvider` comment is duplicated from lines 48-51:
```go
// planContentProvider is implemented by artifacts that carry generated plan text.
// The plan stage's PlanArtifacts satisfies this interface when a GetPlanContent
// method is added, allowing the remediation runner to persist the actual
// remediation plan rather than just the gap analysis.
```

3. **`extractFindings` function** (lines 210-218):
```go
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

**Step 3: Verify compilation**

Run: `go build ./internal/v2/remediation/...`
Expected: PASS

**Step 4: Run tests**

Run: `go test ./internal/v2/remediation/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/v2/remediation/remediation.go
git commit -m "cleanup: remove unused findingsProvider, extractFindings, and duplicate comment"
```

**Acceptance Criteria:**
- `findingsProvider` interface removed
- `extractFindings` function removed
- Duplicate comment removed
- `go build ./internal/v2/remediation/...` succeeds
- `go test ./internal/v2/remediation/...` passes

---

### Task 6: Full regression verification

**Files:** (read-only verification)

**Step 1: Run full build**

Run: `go build ./...`
Expected: PASS

**Step 2: Run full test suite**

Run: `go test ./... -count=1 -timeout=300s`
Expected: PASS

**Step 3: Verify gap analysis criteria**

Cross-reference the gap analysis items:

1. **"Title field dropped in RemediationRunner.convertSpecFindings"** — Fixed in Task 3: Title now populated from `spec.Title`
2. **"AffectedFiles silently lost for spec-review findings"** — Fixed in Task 2 (`specReviewSpecFindings`) and Task 1 (`SpecFinding.AffectedFiles` field added)
3. **"severity mapping inconsistency"** — Fixed in Task 3: `mapSpecSeverity` now maps High→Critical matching `spec_loop.go`
4. **"dead code that should be removed"** — Fixed in Task 5: `findingsProvider`, `extractFindings`, duplicate comment removed

**Step 4: Run project rules compliance checks**

Run: `grep -rn 'os\.Chdir' cmd/ internal/ --include='*_test.go'` — must return zero hits outside production code.

**Acceptance Criteria:**
- `go build ./...` exits 0
- `go test ./...` exits 0
- All 4 gap analysis items addressed
- No project rules violations
