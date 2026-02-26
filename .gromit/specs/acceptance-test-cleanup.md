---
id: acceptance-test-cleanup
created: 2026-02-10
epic: test-quality
---

# Acceptance Test Cleanup: Proper ATDD Hygiene

## Problem

The project has 24 `_acceptance_test.go` files (~7,363 lines) that misuse the "acceptance" label. Most are unit tests with verbose, duplicated setup. This wastes tokens during gromit's verification phase — Claude reads thousands of lines of boilerplate when checking test results. The test suite works, but it's 2-3x larger than it needs to be.

## Goals

1. Correctly classify tests: rename files that are actually unit tests, keep "acceptance" only for tests that exercise end-to-end behavior through the command surface
2. Eliminate setup duplication via shared test helpers
3. Convert repetitive test functions to table-driven tests
4. Remove dead weight (skipped tests, stdlib verification)
5. Preserve all meaningful test coverage — no behavior should lose its test

## Specification

### Phase 1: Audit and Reclassify

Examine every `*_acceptance_test.go` file. A test is a true acceptance test if it:
- Exercises behavior through the public command/API surface (not internal helpers)
- Tests a user-visible outcome, not an implementation detail
- Could serve as a "definition of done" for a bead

Tests that call internal functions directly (like `getEpicFiles`, `buildExplorePrompt`, `getSpecFiles`) are **unit tests** and should be renamed from `*_acceptance_test.go` to `*_test.go` (merging into existing test files where appropriate).

The one file with `//go:build acceptance` (`internal/retro/filtered_hash_eviction_acceptance_test.go`) is correctly tagged and should keep the acceptance label.

Files that need reclassification (based on audit):

- `cmd/gromit/explore_acceptance_test.go` — calls `getEpicFiles()` and `getSpecFiles()` directly; these are unit tests for helper functions
- `cmd/gromit/explore_prompt_acceptance_test.go` — calls `buildExplorePrompt()` directly; unit tests
- `cmd/gromit/explore_session_acceptance_test.go` — inspect whether this exercises the command or internal functions; reclassify accordingly
- All other `*_acceptance_test.go` files without `//go:build acceptance` — audit each one using the criteria above

**Rule of thumb:** If the test creates mocks and calls internal functions, it's a unit test. If it builds a binary and runs a command, it's an acceptance/integration test.

### Phase 2: Extract Shared Setup Helpers

Many test files repeat the same 10-20 line setup blocks. Extract these into focused helpers in the same package (not a separate testutil package — keep them co-located).

**Pattern to extract — explore tests:**
```go
// Every explore test repeats:
tmpDir := t.TempDir()
gromitDir := filepath.Join(tmpDir, ".gromit")
templatesDir := filepath.Join(gromitDir, "templates")
os.MkdirAll(templatesDir, 0755)
claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
os.WriteFile(claudeMDPath, []byte("# Project"), 0644)
cfg := &config.Config{Paths: config.PathsConfig{...}}

// Extract to:
func setupExploreTest(t *testing.T) (cfg *config.Config, gromitDir string) { ... }
```

**Pattern to extract — runner label filtering tests:**
```go
// Every test repeats mockBeads, mockClaude, cfg, deps, NewRunnerWithDeps setup
// Extract the common parts, let each test only specify what differs

func setupLabelFilterTest(t *testing.T, opts labelFilterTestOpts) (*Runner, *labelFilterTestState) { ... }
```

**Pattern to extract — retro hash eviction tests:**
```go
// Each test creates config, writes LEARNINGS.md, loads learnings, creates state
// Extract to a helper that takes the learnings content and initial hashes
```

### Phase 3: Convert to Table-Driven Tests

`runner_label_filtering_acceptance_test.go` (1026 lines) has ~10 test functions that vary only in:
- Which label filters are configured
- What beads exist with what labels
- Which beads should be processed
- Whether `Ready()` or `ReadyWithLabel()` is called

This is a textbook candidate for a single table-driven test. Target: ~300 lines.

Similarly, `explore_prompt_acceptance_test.go` has 7 test functions for `buildExplorePrompt` that share setup and differ only in inputs and which `strings.Contains` checks to run. Convert to table-driven.

### Phase 4: Delete Dead Weight

1. **Delete `TestExploreCommand_E2E_EnsuresEpicsDirExists`** (`explore_acceptance_test.go:131-164`). It tests that `os.MkdirAll` creates a directory. That's Go stdlib behavior.

2. **Delete `TestExploreCommand_WritesPromptToTempFile`** (`explore_prompt_acceptance_test.go:127-179`). It tests that `os.CreateTemp` + `WriteString` works. Stdlib behavior.

3. **Remove or convert skipped tests.** `filtered_hash_eviction_acceptance_test.go` has two `t.Skip()` tests (~250 lines combined):
   - `TestFilteredHashEviction_SingleSaveWhenBothAddAndPrune`
   - `TestFilteredHashEviction_HandlesArchivedLearnings`

   If these describe desired behavior, capture them as backlog ideas via `gromit add`. Then delete the test code — skipped tests that can never run in the current architecture are misleading.

### Phase 5: Verify

After all changes:
- `go test ./...` passes (all non-tagged tests)
- `go test -tags acceptance ./...` passes (tagged acceptance tests)
- No test coverage is lost for actual behavior (deletions should only remove stdlib-testing and dead code)
- `go vet ./...` clean
- `golangci-lint run ./...` clean

## Acceptance Criteria

- No `*_acceptance_test.go` file exists without `//go:build acceptance` unless it genuinely tests end-to-end command behavior
- Every `*_acceptance_test.go` file that was reclassified has been renamed/merged into the appropriate `*_test.go` file
- Shared setup helpers exist for explore tests, runner label filtering tests, and retro hash eviction tests
- `runner_label_filtering_acceptance_test.go` (or its renamed equivalent) uses table-driven tests and is under 400 lines
- Tests verifying stdlib behavior (`os.MkdirAll`, `os.CreateTemp`) are deleted
- Skipped tests are either made runnable or deleted (with ideas captured in backlog)
- `go test ./...` passes
- `go test -tags acceptance ./...` passes
- Total acceptance/unit test line count is reduced by at least 30%

## Constraints

- Do NOT change any production code. This is a test-only refactoring.
- Do NOT delete tests that verify actual gromit behavior — only delete tests that verify Go stdlib behavior or are permanently skipped.
- When merging acceptance test functions into existing `_test.go` files, place them in a clearly commented section or use a descriptive test name prefix.
- Helpers should be in `_test.go` files (not exported), co-located with the tests that use them.

## Sizing Notes

This spec touches many files but each change is mechanical. Recommend decomposing by package:
1. `cmd/gromit/explore_*` tests (3 files → helpers + table-driven + reclassify)
2. `internal/runner/runner_label_filtering_*` tests (1 file → table-driven + helpers + reclassify)
3. `internal/retro/filtered_hash_eviction_*` tests (1 file → helpers + delete skipped)
4. Audit remaining `*_acceptance_test.go` files across all packages
5. Final verification pass
