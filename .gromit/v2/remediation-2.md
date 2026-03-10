Now I have a complete picture. The spec is substantially implemented — the remaining work is the gap analysis remediation items. Let me produce the plan.

---

---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-10
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

**Goal:** Fix findings conversion inconsistencies (dropped Title, lost AffectedFiles, severity mapping divergence) and remove dead code in the remediation pipeline.

**Architecture:** Two parallel conversion paths exist for `SpecFinding → finding.Finding`: one in `spec_loop.go` (canonical, complete) and one in `remediation.go` (broken, drops Title/AffectedFiles, inconsistent severity mapping). The fix adds `AffectedFiles` to the `SpecFinding` type, aligns `remediation.go`'s conversion to match `spec_loop.go`'s canonical behavior, wires `AffectedFiles` through all conversion points, and removes dead code.

**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

The findings pipeline has three layers:

1. **Stage outputs** — `accept` produces `AcceptArtifacts.SpecFindings`, `specreview` produces `SpecReviewArtifacts.Findings` (type `[]SpecReviewFinding`)
2. **Spec loop conversions** — `spec_loop.go` converts both to `[]finding.Finding` via `convertSpecFindings()` and `convertSpecReviewFindings()`
3. **Remediation conversions** — `remediation.go` has its own `convertSpecFindings()` and `specReviewSpecFindings()` that convert the same types

The problem: `stage.SpecFinding` lacks `AffectedFiles`, so file info is lost when `SpecReviewFinding` converts to `SpecFinding`. Additionally, `remediation.go`'s `convertSpecFindings` drops Title and maps severity differently from `spec_loop.go`.

**Files to modify:**
- `internal/v2/stage/stage.go` — add `AffectedFiles` to `SpecFinding`, add `NormalizeNilFields`
- `internal/v2/remediation/remediation.go` — fix `convertSpecFindings`, `specReviewSpecFindings`, `mapSpecSeverity`; remove dead code
- `internal/v2/loop/spec_loop.go` — wire `AffectedFiles` through `convertSpecFindings`

**Files to test:**
- `internal/v2/stage/stage_test.go` (or existing test file)
- `internal/v2/remediation/remediation_test.go`
- `internal/v2/loop/spec_loop_test.go` or `spec_loop_specreview_test.go`

## Test Strategy

- TDD: write failing test first, then implement
- Unit tests for each conversion function change
- Run `go test ./internal/v2/remediation/... ./internal/v2/loop/... ./internal/v2/stage/...` after each task
- Final `go test ./...` for full regression check
- Mocking: not needed — conversion functions are pure transformations

---

## Implementation Tasks

### Task 1: Add AffectedFiles to SpecFinding with nil-field normalization
**Files:** `internal/v2/stage/stage.go`, stage test file
**What to Do:** Add `AffectedFiles []string` field to `SpecFinding` struct. Add `NormalizeNilFields()` method that maps nil `AffectedFiles` to empty slice. Write test verifying normalization behavior.
**Acceptance Criteria:**
- `SpecFinding` has `AffectedFiles []string` field
- `NormalizeNilFields()` maps nil to `[]string{}`
- `go build ./internal/v2/stage/...` succeeds
**Dependencies:** None

### Task 2: Wire AffectedFiles through specReviewSpecFindings in remediation.go
**Files:** `internal/v2/remediation/remediation.go`, `internal/v2/remediation/remediation_test.go`
**What to Do:** Update `specReviewSpecFindings()` to copy `AffectedFiles` from `SpecReviewFinding` to the `SpecFinding` it produces. Write test that creates a `SpecReviewFinding` with `AffectedFiles` and verifies they appear in the converted `SpecFinding`.
**Acceptance Criteria:**
- `specReviewSpecFindings` copies `AffectedFiles` via defensive clone (`append([]string(nil), ...)`)
- Test verifies round-trip preservation of file paths
**Dependencies:** Task 1

### Task 3: Fix convertSpecFindings in remediation.go — add Title, AffectedFiles, and align severity mapping
**Files:** `internal/v2/remediation/remediation.go`, `internal/v2/remediation/remediation_test.go`
**What to Do:** Rewrite `convertSpecFindings()` to preserve `Title` and `AffectedFiles` fields. Update `mapSpecSeverity()` so `High → SeverityCritical` (matching `spec_loop.go`'s `convertSeverity`). Wire `mapSpecCategory()` into the conversion (currently defined but never called). Write tests for Title preservation, AffectedFiles round-trip, and High→Critical severity mapping.
**Acceptance Criteria:**
- `convertSpecFindings` populates `Title` from `spec.Title`
- `convertSpecFindings` copies `AffectedFiles`
- `mapSpecSeverity` maps `High → SeverityCritical` (not `SeverityWarning`)
**Dependencies:** Task 1

### Task 4: Wire AffectedFiles through spec_loop.go convertSpecFindings
**Files:** `internal/v2/loop/spec_loop.go`, `internal/v2/loop/spec_loop_test.go` or `spec_loop_specreview_test.go`
**What to Do:** Change `convertSpecFindings()` in `spec_loop.go` line 879 from passing `nil` to passing `entry.AffectedFiles` in the `convertToFinding` call. Write test that creates a `SpecFinding` with `AffectedFiles` and verifies they appear in the converted `finding.Finding`.
**Acceptance Criteria:**
- `convertSpecFindings` passes `entry.AffectedFiles` to `convertToFinding`
- Test verifies round-trip preservation
**Dependencies:** Task 1

### Task 5: Remove dead code from remediation.go
**Files:** `internal/v2/remediation/remediation.go`
**What to Do:** Remove `findingsProvider` interface, `extractFindings` function, and any duplicate comment blocks. Verify no external callers reference the removed symbols via grep. Run build and tests to confirm clean removal.
**Acceptance Criteria:**
- `findingsProvider` interface removed
- `extractFindings` function removed
- `go build ./internal/v2/remediation/...` succeeds and `go test ./internal/v2/remediation/...` passes
**Dependencies:** None (can run in parallel with Tasks 1-4)

### Task 6: Full regression verification
**Files:** (read-only)
**What to Do:** Run `go build ./...` and `go test ./... -count=1 -timeout=300s`. Verify all four gap analysis items are addressed: Title preserved, AffectedFiles wired, severity mapping aligned, dead code removed. Run project rules compliance check (`grep -rn 'os\.Chdir'` in test files).
**Acceptance Criteria:**
- `go build ./...` exits 0
- `go test ./...` exits 0
- All 4 gap analysis criteria resolved
**Dependencies:** Tasks 1-5