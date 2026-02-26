---
id: touched-package-unit-tests
source_ideas: []
created: 2026-02-26
epic: test-quality
---

# Runner Touched-Package Tracking Unit Tests

## Problem

`mergeTouchedPackages` and `normalizeTouchedPackages` in `internal/runner/orchestrator.go` are pure functions with no dedicated unit tests. Coverage for touched-package tracking relies on integration-level tests that exercise many other code paths, making regressions hard to isolate.

## Approach

- Add table-driven unit tests for `mergeTouchedPackages` directly in `internal/runner/orchestrator_test.go` (or a new `touched_packages_test.go` in the same package)
- Test cases: empty inputs, single non-overlapping maps, overlapping keys with count accumulation, nil map inputs
- Add table-driven unit tests for `normalizeTouchedPackages`: nil input returns empty map, empty map returns empty map, non-empty map passes through unchanged
- Tests should be pure in-memory with no file I/O or subprocess calls

## Files to Change

- `internal/runner/orchestrator_test.go` — add `TestMergeTouchedPackages` and `TestNormalizeTouchedPackages` table-driven tests (or new `touched_packages_test.go`)

## Acceptance Criteria

- `TestMergeTouchedPackages` covers: both inputs empty, one input empty, disjoint keys, overlapping keys with correct count addition, nil map input
- `TestNormalizeTouchedPackages` covers: nil input, empty map, non-empty map
- No file I/O or subprocess dependencies in the new tests
- Tests pass with `go test ./internal/runner/...`
- No permanently skipped test cases
