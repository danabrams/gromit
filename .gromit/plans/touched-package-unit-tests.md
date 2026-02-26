---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T19:18:44Z"
id: touched-package-unit-tests
source_spec: touched-package-unit-tests
---

# Touched Package Unit Tests Implementation Plan

**Goal:** Add focused unit test coverage for touched-package merge and normalization helpers in the runner orchestrator.

**Architecture:** Add table-driven tests directly in the existing orchestrator test file to validate pure helper behavior in isolation from stage orchestration.

**Tech Stack:** Go (`testing` package), existing runner test conventions.

**Spec:** `.gromit/specs/touched-package-unit-tests.md`

---

## Architecture

**Overview:**
Add direct table-driven unit tests in `internal/runner/orchestrator_test.go` for `mergeTouchedPackages` and `normalizeTouchedPackages`, focusing exclusively on in-memory inputs and deterministic outputs.

**Key Components:**
1. **`TestMergeTouchedPackages`**: Verifies merge behavior and normalized final output across nil/empty/disjoint/overlapping inputs.
2. **`TestNormalizeTouchedPackages`**: Verifies normalization rules and edge handling (`nil`, empty, already-normalized inputs).

**Integration Points:**
- Extend the existing runner orchestrator test suite in `internal/runner/orchestrator_test.go`.
- Keep production code unchanged.
- Complement current integration-style touched-package accumulation coverage with direct helper-level assertions.

**Data Flow:**
- `mergeTouchedPackages(existing, incoming)` combines slices, then delegates to normalization.
- `normalizeTouchedPackages(touchedPackages)` trims whitespace and path prefixes/slashes, preserves `"."`, removes empty entries, and deduplicates in first-seen order.
- Unit tests assert final slices from representative inputs, including nil and mixed-format values.

**Files to Modify:**
- `internal/runner/orchestrator_test.go` - add table-driven tests for helper behavior.

**Files to Create:**
- None.

**Tradeoffs:**
- **Existing test file vs new test file**: Keep tests in `orchestrator_test.go` to align with existing helper-adjacent orchestration tests and minimize churn.
- **Direct helper tests vs orchestration-only tests**: Add direct tests to isolate regressions without requiring full stage wiring.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Table-driven tests for `mergeTouchedPackages` and `normalizeTouchedPackages`.
2. **Integration Tests**: Not required for this spec; existing orchestrator integration tests remain as supporting coverage.
3. **Manual Testing**: Not required.

**Key Test Cases:**
- `mergeTouchedPackages`:
  - both inputs empty
  - one input empty (existing-only and incoming-only)
  - disjoint keys combine in order
  - overlapping keys dedupe after merge
  - nil input handling
- `normalizeTouchedPackages`:
  - nil input
  - empty input
  - non-empty already-normalized input
  - mixed formatting normalization (`./`, trailing slashes, whitespace, duplicates, empty values, `"."`)

**Mocking Strategy:**
- No mocks; tests call pure functions directly.
- No filesystem I/O, subprocess execution, or stage dependencies.

**Coverage Goals:**
- Protect normalization and merge semantics with deterministic helper-level assertions.
- Cover edge conditions likely to regress during refactors (`nil`, duplicates, formatting variants).

**Test Organization:**
- Place `TestMergeTouchedPackages` and `TestNormalizeTouchedPackages` in `internal/runner/orchestrator_test.go`.
- Use table-driven subtests with descriptive case names.

## Implementation Tasks

### Task 1: Add direct helper unit tests

**Files:**
- Modify: `internal/runner/orchestrator_test.go`

**What to Do:**
Add table-driven `TestMergeTouchedPackages` and `TestNormalizeTouchedPackages` covering empty, nil, disjoint, overlapping, and normalization-formatting scenarios. Keep tests pure and independent of orchestrator stage setup.

**Acceptance Criteria:**
- `TestMergeTouchedPackages` covers both-empty, one-empty, disjoint, overlapping, and nil input cases.
- `TestNormalizeTouchedPackages` covers nil, empty, and non-empty pass-through plus normalization edge behavior.
- No file I/O or subprocess dependencies are introduced.

**Dependencies:**
- None.

**Notes:**
- Preserve expected order based on first appearance after normalization/deduplication.

### Task 2: Validate runner package tests

**Files:**
- Test: `internal/runner/orchestrator_test.go` (execution target is package-level)

**What to Do:**
Run package tests to verify the new helper tests pass and do not regress existing runner behavior.

**Acceptance Criteria:**
- `go test ./internal/runner/...` passes.
- No permanently skipped cases are added for this spec.

**Dependencies:**
- Task 1.

**Notes:**
- Keep failures actionable by using explicit expected slices in table-driven assertions.

---

## Notes

This plan intentionally keeps scope narrow: test-only changes in the orchestrator test suite with no production behavior changes. The task breakdown is decompose-ready and should map cleanly to one implementation bead plus one quality-gate verification bead.
