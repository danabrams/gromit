---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T08:24:53-05:00"
id: model-success-tracking
source_spec: model-success-tracking
---

# Model Success Tracking Implementation Plan

**Goal:** Track per-model success/failure rates at project and global levels, and surface the data via `gromit stats` and `gromit status`.

**Architecture:** Add a ModelStats aggregation layer in the logger package that computes per-model success/failure/escalation/cost data from existing JSONL logs. Store global cross-project stats in `~/.gromit/stats.json` with atomic writes. Surface via a new `gromit stats` command and a brief section in `gromit status`.

**Tech Stack:** Go, cobra CLI, JSON file I/O

**Spec:** `.gromit/specs/model-success-tracking.md`

---

## Architecture

**Overview:**
Extend the existing logger package with per-model success aggregation computed on demand from JSONL logs. Add a global stats file at `~/.gromit/stats.json` that stores running totals across projects. Surface data through a new `gromit stats` command and a brief section in `gromit status`.

**Key Components:**
1. **`internal/logger/modelstats.go`**: Per-model stats aggregation from JSONL logs — success rates, escalation tracking, cost-per-completed-bead
2. **`internal/logger/globalstats.go`**: Global stats file management — read/write `~/.gromit/stats.json` with atomic writes
3. **`cmd/gromit/stats.go`**: New CLI command showing detailed model performance (project + global)

**Integration Points:**
- Reads existing JSONL log data via `readLogFile()` — no changes to log format
- Post-run update hooks into `Runner.Run()` end-of-loop logic
- Status display follows existing section-based formatting pattern in `format.go`
- New command follows cobra pattern (one file, `init()` registration)

**Data Flow:**
- `gromit run` → writes JSONL logs (existing) → on completion, reads run stats → updates `~/.gromit/stats.json`
- `gromit status` → reads project JSONL logs → computes per-model stats → displays brief section
- `gromit stats` → reads project JSONL logs + global stats → computes full analysis → displays detailed tables

**Files to Modify:**
- `internal/runner/runner.go` — Post-run global stats update in `Run()`, wire model stats into `Status()`
- `internal/runner/format.go` — Add `formatModelPerformance()` function
- `internal/runner/format_test.go` — Tests for new formatter

**Files to Create:**
- `internal/logger/modelstats.go` — Per-model stats aggregation
- `internal/logger/modelstats_test.go` — Tests for model stats
- `internal/logger/globalstats.go` — Global stats file management
- `internal/logger/globalstats_test.go` — Tests for global stats
- `cmd/gromit/stats.go` — New CLI command

**Tradeoffs:**
- **Compute on demand vs cache**: Compute-on-demand for project stats — JSONL files are small, caching adds complexity and staleness risk
- **Global stats in runner vs separate hook**: Inline in `Run()` — simpler, runner already knows run ID and log path
- **Separate modelstats.go vs extending efficiency.go**: New file — success tracking is a different concern from cost/duration efficiency

## Test Strategy

**Test Levels:**
1. **Unit Tests**: ModelStats aggregation, GlobalStats file I/O, format functions
2. **Manual Testing**: Run `gromit stats` and `gromit status` against real project logs

**Key Test Cases:**
- ReadModelStats aggregates correctly across multiple JSONL files
- ReadModelStats tracks escalation_from and escalation_to separately
- ReadModelStats handles empty directory, malformed files
- ReadRunModelStats filters by run ID
- CostPerCompletedBead attributes full chain cost, excludes incomplete beads
- ReadGlobalStats returns zero-value on missing file, error on corrupt file
- UpdateGlobalStats creates new file, merges with existing, uses atomic write
- formatModelPerformance handles multi-model, single-model, empty data

**Mocking Strategy:**
- No mocks for logger tests — use `t.TempDir()` with synthetic JSONL files
- Format tests are pure functions, no mocks needed

**Coverage Goals:**
- Critical: ReadModelStats aggregation, UpdateGlobalStats atomic write, CostPerCompletedBead chain attribution
- Edge cases: empty logs, missing global file, corrupt data, zero-cost iterations

## Implementation Tasks

### Task 1: Add ModelStats aggregation logic

**Files:**
- Create: `internal/logger/modelstats.go`
- Create: `internal/logger/modelstats_test.go`

**What to Do:**
Add a `ModelStats` struct and functions to aggregate per-model success/failure/escalation/cost data from existing JSONL log files. `ReadModelStats` reads all logs in a directory. `ReadRunModelStats` reads stats for a single run by run ID. `CostPerCompletedBead` groups iterations by bead ID and computes the total cost chain for successfully completed beads.

**Acceptance Criteria:**
- `ReadModelStats` returns correct per-model success, failure, escalation, and cost totals from multiple JSONL files
- `CostPerCompletedBead` attributes the full retry/escalation chain cost to completed beads and excludes incomplete beads
- All tests pass with synthetic JSONL data in `t.TempDir()`

**Dependencies:** None

**Notes:** Reuse the existing `readLogFile()` function. Follow the pattern in `ReadAllLogs`/`ReadPerBeadStats` for globbing and iterating log files.

### Task 2: Add global stats file management

**Files:**
- Create: `internal/logger/globalstats.go`
- Create: `internal/logger/globalstats_test.go`

**What to Do:**
Add `GlobalStats` struct matching the spec's JSON format. Implement `ReadGlobalStats` to load from a path (returning zero-value on missing file), and `UpdateGlobalStats` to read-modify-write with atomic temp-file-then-rename. The update merges a run's `map[string]*ModelStats` into the global totals.

**Acceptance Criteria:**
- `ReadGlobalStats` returns zero-value stats without error when file doesn't exist
- `UpdateGlobalStats` creates the file on first call and merges correctly on subsequent calls
- Write uses atomic temp-file + `os.Rename` pattern

**Dependencies:** Task 1 (uses `ModelStats` type)

### Task 3: Add `gromit stats` CLI command

**Files:**
- Create: `cmd/gromit/stats.go`

**What to Do:**
Add a new cobra command registered via `init()`. It loads config, reads project-level model stats from JSONL logs via `ReadModelStats`, reads global stats from `~/.gromit/stats.json` via `ReadGlobalStats`, computes cost-per-completed-bead, and displays formatted output showing per-model success rates, iteration counts, cost data, escalation frequency, and project-vs-global comparison. Add `--json` flag for machine-readable output.

**Acceptance Criteria:**
- `gromit stats` displays per-model success rate, iteration count, cost-per-completed-bead, and escalation frequency for both project and global scopes
- When `~/.gromit/stats.json` does not exist, shows project-only data without error

**Dependencies:** Task 1, Task 2

**Notes:** Follow the cobra pattern in `cmd/gromit/debug.go` for command structure. Use `resolveGromitDir(cfg)` for directory resolution. Global stats path is `~/.gromit/stats.json` via `os.UserHomeDir()`.

### Task 4: Add model performance section to `gromit status`

**Files:**
- Modify: `internal/runner/format.go`
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/format_test.go`

**What to Do:**
Add `formatModelPerformance(stats map[string]*logger.ModelStats) string` to format.go that produces a brief model performance summary (success rate + avg cost per model). Wire it into `Runner.Status()` by calling `logger.ReadModelStats(r.cfg.Paths.Logs)` and displaying the formatted section between Health and Recommendation.

**Acceptance Criteria:**
- `gromit status` includes a brief model performance summary showing per-model success rate and avg cost
- Format tests cover multi-model, single-model, and empty-data cases

**Dependencies:** Task 1

### Task 5: Wire post-run global stats update into runner

**Files:**
- Modify: `internal/runner/runner.go`

**What to Do:**
After the main loop in `Run()` completes (around line 572), if at least one iteration was processed, call `logger.ReadRunModelStats` with the current run ID to get per-model stats for this run, then call `logger.UpdateGlobalStats` to merge them into `~/.gromit/stats.json`. Log warnings on failure but don't fail the run.

**Acceptance Criteria:**
- After each run that processes at least one bead, global stats in `~/.gromit/stats.json` are updated with that run's per-model outcomes
- Global stats update failures are logged as warnings and do not cause the run to fail

**Dependencies:** Task 1, Task 2

**Notes:** Use `os.UserHomeDir()` to resolve `~/.gromit/stats.json`. The Logger's `RunID()` method provides the current run identifier. The logger's directory can be derived from `r.cfg.Paths.Logs`.

---

## Notes

- The `readLogFile()` function is unexported but lives in the same `logger` package, so all new code in `internal/logger/` can use it directly.
- Global stats use last-writer-wins semantics — concurrent runs may overwrite each other's updates, but atomic writes prevent corruption. This is acceptable given the advisory nature of the data.
- The spec explicitly states this is advisory only — no automatic routing changes based on stats.
