---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T09:09:56-05:00"
id: expanded-status
source_spec: expanded-status
---

# Expanded Status Display Implementation Plan

**Goal:** Make `gromit status` show a rich four-section display (Pipeline, Run, Health, Recommendation) instead of just the next bead.

**Architecture:** Rewrite `Runner.Status()` to gather data from three sources — `status.json` (run state), `pipeline.ReadStatus()` (pipeline counts), and `state.json` (health) — and format them into a human-readable display. Change `StatusWriter` to persist on clean exit instead of deleting the file.

**Tech Stack:** Go

**Spec:** `.gromit/specs/expanded-status.md`

---

## Architecture

### Overview

Rewrite `Runner.Status()` to produce a rich four-section display by wiring together existing data sources. Change `StatusWriter` to persist on clean exit with `running: false` instead of deleting the file.

### Key Components

1. **`internal/runner/status.go`** — Add `MaxIterations` and `TimeBudgetMinutes` fields to `Status` struct. Add `ReadStatus()` function to read/parse `status.json`. Add `WriteFinal()` method for clean exit persistence.

2. **`internal/runner/format.go`** (new) — Pure display formatting functions: `formatPipeline()`, `formatRun()`, `formatHealth()`, `formatRecommendation()`.

3. **`internal/runner/runner.go`** — Rewrite `Status()` to gather + format all four sections. Change `Run()` exit to call `WriteFinal()` instead of `Delete()`.

### Data Flow

```
gromit status → showStatus() → Runner.Status()
  ├── ReadStatus(gromitDir)         → Status{running, iteration, elapsed, bead, model, limits}
  ├── pipeline.ReadStatus(...)      → PipelineStatus{backlog, specs, plans, beads, recommendation}
  ├── state.File.Load()             → State{LastRetro, IterationsSinceReview}
  └── format + print to r.output
```

### Integration Points

- `Runner.Status()` is called from `showStatus()` in `cmd/gromit/main.go` — no CLI changes needed
- `pipeline.ReadStatus()` already exists — just call it
- `state.File` already has `LastRetro()`, `IterationsSinceReview()` — just call them
- `StatusWriter.Write()` needs `MaxIterations` and `TimeBudgetMinutes` threaded through from `Runner.Run()`

### Tradeoffs

- **Formatting in `runner/format.go`**: Keeps display logic close to data sources, CLI stays thin
- **`WriteFinal()` as separate method**: Cleaner than adding optional parameters to `Write()`
- **"Up to 3 items" is a display concern**: Handled in format functions, not in data layer

## Test Strategy

### Unit Tests (`format_test.go`)
- Table-driven tests for each format function (pure functions, easy to test)
- Cover: zero items, under 3, exactly 3, more than 3, time formatting, all run states

### Unit Tests (`status_test.go`)
- `ReadStatus`: valid file, missing file, corrupt JSON
- `WriteFinal`: correct JSON with `running: false`
- `Write` with new limit fields

### Integration Tests (`status_test.go`)
- `Runner.Status()` with `NewRunnerWithDeps` — verify full formatted output

### Mocking
- Mock `BeadClient` via `NewRunnerWithDeps` (existing pattern)
- Real temp directories with fixture files for `ReadStatus`/`WriteFinal`
- No mocking for pure format functions

## Implementation Tasks

### Task 1: Add fields to Status struct and implement ReadStatus/WriteFinal

**Files:**
- Modify: `internal/runner/status.go`
- Test: `internal/runner/status_test.go`

**What to Do:**
1. Add `MaxIterations int` (json: `max_iterations`) and `TimeBudgetMinutes int` (json: `time_budget_minutes`) fields to the `Status` struct.
2. Update `StatusWriter.Write()` signature to accept `maxIterations` and `timeBudgetMinutes` params, and populate the new fields.
3. Add `ReadStatus(gromitDir string) (*Status, error)` function that reads `.gromit/status.json`, returns `nil, nil` when file doesn't exist.
4. Add `WriteFinal(iteration int) error` method to `StatusWriter` that writes a final status with `running: false`, empty bead fields, and the completed iteration count.
5. Add tests for: `ReadStatus` with valid file, missing file, corrupt JSON; `WriteFinal` output; `Write` with limit fields populated; round-trip (Write then ReadStatus).

**Acceptance Criteria:**
- `Status` struct has `MaxIterations` and `TimeBudgetMinutes` with correct JSON tags
- `ReadStatus` returns nil/nil for missing file, error for corrupt file, correct struct for valid file
- `WriteFinal` writes JSON with `running: false` and correct iteration count

**Dependencies:** None

### Task 2: Create display formatting functions

**Files:**
- Create: `internal/runner/format.go`
- Create: `internal/runner/format_test.go`

**What to Do:**
1. Implement `formatPipeline(ps *pipeline.PipelineStatus) string` — formats pipeline section with counts and up to 3 item names per category, "and X more" when >3. Zero-count categories show count but no items.
2. Implement `formatRun(s *Status) string` — formats run section. When `s` is nil or `s.Running` is false with zero iteration, show just "not running". When not running with iteration > 0, show "not running" plus "Last run: Xh ago, N iterations completed". When running, show iteration (with /max if MaxIterations > 0), elapsed time (with "of Xm" if TimeBudgetMinutes > 0), current bead, and model.
3. Implement `formatHealth(lastRetro time.Time, iterationsSinceReview int) string` — formats health section. Use "never" for zero-value times. Show iterations since review.
4. Implement `formatRecommendation(rec string) string` — formats the recommendation line with command hint.
5. Implement helper `formatTimeAgo(t time.Time) string` for human-readable relative time ("2h ago", "3d ago", etc.).
6. Write table-driven tests covering all edge cases from the test strategy.

**Acceptance Criteria:**
- Pipeline section shows up to 3 items per category with "and X more" for overflow
- Run section handles all four variants: running with limits, running without limits, not running with history, not running without history
- Health section shows "never" for zero-value times
- Time formatting produces human-readable relative times

**Dependencies:** None (uses `pipeline.PipelineStatus` type but only needs the import)

### Task 3: Rewrite Runner.Status and change Run exit path

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/status_test.go`

**What to Do:**
1. Rewrite `Runner.Status()` to:
   - Call `ReadStatus(r.gromitDir)` to get run state from `status.json`
   - Call `pipeline.ReadStatus(r.gromitDir, r.cfg.Paths.Specs, r.cfg.Paths.Plans)` for pipeline data
   - Load `state.File` for health data (LastRetro, IterationsSinceReview)
   - Call the four format functions and print to `r.output`
2. In `Runner.Run()`:
   - Thread `maxIterations` and `timeBudgetMinutes` (computed from deadline) to `StatusWriter` — either store on the writer or pass to each `Write()` call.
   - Replace `defer statusWriter.Delete()` with a deferred `statusWriter.WriteFinal(iteration)` call.
   - Update existing `statusWriter.Write()` calls to pass limit values.
3. Add integration tests using `NewRunnerWithDeps` that verify the full formatted output of `Runner.Status()` for different scenarios (active run, idle with history, idle without history).

**Acceptance Criteria:**
- `gromit status` prints all four sections (Pipeline, Run, Health, Next action)
- `status.json` is not deleted on clean exit; a final `running: false` entry is written instead
- `status.json` includes `max_iterations` and `time_budget_minutes` when limits are set

**Dependencies:** Task 1 (Status struct changes, ReadStatus, WriteFinal), Task 2 (format functions)

**Notes:**
- The `Paths.Plans` field exists in config (`internal/config/config.go:82`) but may default to empty. Check `config.go` defaults — if Plans isn't defaulted, use `filepath.Join(gromitDir, "plans")` as fallback.
- `StatusWriter.Write()` is called in two places in `Run()` (lines 317 and 357) — both need the limit params.
- The `cfg.Paths.Plans` default may need to be set in `config.go` if not already present. Check before implementing.

---

## Notes

- The `status-json-staleness` spec (PID-based stale detection) is complementary work. It will eventually add PID tracking and stale file cleanup. This plan does not conflict — `WriteFinal()` producing `running: false` is compatible with PID-based staleness detection for crash cases.
- The `countReadyBeads` function in `pipeline/status.go` has a known bug (open bead ralph-runner-8i7k) — it calls `client.List()` instead of `client.Ready()`. This plan uses `pipeline.ReadStatus()` as-is; that bug should be fixed separately.
- The `generateRecommendation` function currently returns raw strings. The spec wants command hints like "(gromit refine)". The `formatRecommendation` function in Task 2 can append the appropriate command hint based on the recommendation content, or we can update `generateRecommendation` — either approach works. Prefer keeping the change in the format layer.
