---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T12:35:32Z"
id: extract-status-display-package
source_spec: extract-status-display-package
---

# Extract Status Display Package Implementation Plan

**Goal:** Extract runner status-display formatting from `internal/runner/format.go` into a dedicated `internal/runner/display` package while preserving behavior and test coverage.

**Architecture:** Move formatting logic into a cycle-safe `display` package with a display-local run status type, and keep `internal/runner/format.go` as a thin compatibility shim/adaptor during transition.

**Tech Stack:** Go, standard library (`fmt`, `strings`, `time`, `sort`, `math`), existing internal packages (`internal/logger`, `internal/pipeline`, `internal/runner`).

**Spec:** `.gromit/specs/extract-status-display-package.md`

---

## Architecture

## Architecture Proposal

**Overview:**  
Extract all status-display formatting into `internal/runner/display`, while keeping `internal/runner/format.go` as a thin compatibility shim that adapts `runner.Status` to a display-local type and forwards calls.

**Key Components:**
1. **`internal/runner/display` package**: Owns all extracted formatter logic and tests.
2. **Display-local run input type**: New struct in `display` (e.g., `RunStatus`) mirrors only fields formatting needs, avoiding `display -> runner` dependency.
3. **`internal/runner/format.go` shim**: Minimal wrappers (`formatRun`, `formatHealth`, etc.) calling exported display functions to preserve existing runner call sites/tests during transition.

**Integration Points:**
- Update `internal/runner/print_status.go` to call `display` directly where clean.
- Keep `runner` compatibility wrappers for tests and any residual internal callers.
- Keep non-targeted function `formatCompatibility` in `runner` (not in extraction list).

**Data Flow:**
- `PrintStatus` gathers status/pipeline/log data.
- It passes status through an adapter `toDisplayRunStatus(*Status) display.RunStatus`.
- `display` returns formatted strings for run/health/pipeline/SPC/recommendation/model sections.
- Output composition remains in `PrintStatus`.

**Files to Modify:**
- `internal/runner/format.go` - shrink to wrappers/adapters.
- `internal/runner/print_status.go` - import/use `display`.
- `internal/runner/format_test.go` - migrate tests out (or leave small wrapper smoke tests if needed).

**Files to Create:**
- `internal/runner/display/display.go` - extracted functions.
- `internal/runner/display/types.go` - `RunStatus` and related minimal types.
- `internal/runner/display/display_test.go` - moved formatter tests.

**Tradeoffs:**
- **Shim-first migration vs full call-site rewrite**: chose shim-first to minimize regression risk and keep incremental compatibility.
- **Display-local type vs importing `runner.Status`**: chose display-local type to avoid package cycles and keep display reusable.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** Move formatter-focused tests to `internal/runner/display/display_test.go` and run against exported `display` APIs.
2. **Integration Tests:** Keep runner integration tests (especially status output paths) validating `PrintStatus` behavior after import/wrapper changes.
3. **Manual/spot verification:** Quick `gromit status` style output sanity in a fixture path if needed, but primary validation is automated tests/build.

**Key Test Cases:**
- `FormatRun` behavior for nil/not-running/running with reliability/escalation/recurrence.
- SPC formatting coverage: control limits, EWMA, Nelson violations, anomaly summary, deterministic ordering.
- `FormatPipeline`, `FormatRecommendation`, `FormatHealth`, `FormatModelPerformance`, and helper formatting edge cases (rounding, clamping, list overflow).
- Compatibility shim behavior in `runner` (if retained): wrapper output matches display output for adapted status data.

**Mocking Strategy:**
- No heavy mocking needed for display unit tests (pure formatting).
- Use real structs (`logger.ProcessTrend`, `pipeline.PipelineStatus`, model stats maps) as current tests do.
- Runner integration tests continue to use current test fixtures/helpers.

**Coverage Goals:**
- Preserve all existing formatter coverage currently in `internal/runner/format_test.go`.
- Add/keep targeted tests for adapter correctness (`runner.Status` -> `display.RunStatus`) if a custom adapter function is introduced.
- Ensure deterministic output ordering remains covered (maps/sorted lines).

**Test Organization:**
- Primary formatter tests live in `internal/runner/display/display_test.go`.
- Optional small `internal/runner/format_test.go` remains only for wrapper/adapter smoke tests (or remove if no longer needed).
- Continue standard `TestFormatXxx` naming.

## Implementation Tasks

### Task 1: Create Display Package Surface and Types

**Files:**
- Create: `internal/runner/display/types.go`
- Create: `internal/runner/display/display.go`

**What to Do:**
Define the new `display` package with exported formatter API and a cycle-safe `RunStatus` input type containing only fields needed for display rendering. Move SPC metric constants and shared helpers into this package as the canonical implementation.

**Acceptance Criteria:**
- `internal/runner/display` builds with exported formatter functions matching runner needs.
- `display.RunStatus` (or equivalent) removes any dependency on `runner.Status`.
- All extracted helper/metric constants used by SPC formatting compile in the new package.

**Dependencies:**
- None.

**Notes:**
- Keep helper visibility minimal; export only functions required by external callers/tests.

### Task 2: Extract Formatting Logic and Add Runner Compatibility Shim

**Files:**
- Modify: `internal/runner/format.go`
- Modify: `internal/runner/print_status.go`
- Modify: `internal/runner/format_bead_breakdown.go` (only if bead breakdown visibility must change)

**What to Do:**
Move formatting implementations from `runner` into `display`, then reduce `runner/format.go` to thin wrappers/adapters. Update `PrintStatus` to call `display` functions directly (or wrapper functions), using an explicit adapter from `*runner.Status` to `display.RunStatus`.

**Acceptance Criteria:**
- `internal/runner/format.go` is under 550 lines.
- Formatting behavior in `PrintStatus` output remains unchanged.
- No import cycle exists between `runner` and `display`.

**Dependencies:**
- Task 1.

**Notes:**
- Keep `formatCompatibility` in `runner` unless intentionally relocated by a separate change.

### Task 3: Move and Rehome Formatter Unit Tests

**Files:**
- Create: `internal/runner/display/display_test.go`
- Modify: `internal/runner/format_test.go` (remove moved tests; optionally keep shim tests)

**What to Do:**
Move formatter-focused tests from `runner` to `display`, updating package/import references and test fixtures to use new exported APIs/types. Retain only wrapper/adapter tests in `runner` if useful for backward-compatibility guarantees.

**Acceptance Criteria:**
- Existing formatter test coverage runs in `internal/runner/display/display_test.go`.
- `internal/runner/format_test.go` no longer duplicates moved formatter tests.
- All moved tests pass without behavior changes.

**Dependencies:**
- Task 2.

**Notes:**
- Prefer preserving test names and assertions to reduce migration risk.

### Task 4: Validate Build and Runner Test Targets

**Files:**
- Modify: any touched files from Tasks 1-3 to fix compile/test issues.

**What to Do:**
Run required quality gates and resolve remaining compile/test regressions caused by extraction.

**Acceptance Criteria:**
- `go test ./internal/runner/...` passes.
- `go test ./internal/runner/display/...` passes.
- `go build ./...` succeeds with no compilation errors.

**Dependencies:**
- Task 3.

**Notes:**
- If failures reveal additional follow-up work, capture with linked beads during implementation.

---

## Notes

- The highest-risk area is `formatRun` due to dependency on `runner.Status`; the explicit adapter and display-local type are mandatory to avoid cycles.
- Keep changes behavior-preserving first; avoid opportunistic formatting tweaks during extraction.
- After implementation, ensure no stale references remain to moved private helpers in `runner` tests/callers.
