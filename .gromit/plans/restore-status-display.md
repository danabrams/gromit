---
id: restore-status-display
spec: null
created: 2026-02-22
decomposed: false
---

# Plan: Restore rich status display lost during architecture migration

## Research & Context
- Investigation report: `.gromit/reports/debug-20260222-120033.md`
- Reference implementation: `.-gromit-debug-1771735150064710899/internal/runner/format.go` and `lifecycle.go`
- Acceptance tests: `internal/runner/acceptance/status_acceptance_test.go` and `status_integration_acceptance_test.go`

## Architecture

The fix creates a standalone `PrintStatus` function that replaces the old `Runner.Status()` method.
It reads from the same data sources (status.json, pipeline status, state files, model stats,
process trend) but doesn't require a Runner instance — just gromitDir, config, and an io.Writer.

Data flow:
```
PrintStatus(gromitDir, cfg, writer, processChecker)
  ├── ReadStatus(gromitDir)           → Status (run state)
  ├── pipeline.ReadStatus(...)        → PipelineStatus (backlog/specs/plans/beads)
  ├── state.NewFile + Load            → IterationsSinceReview
  ├── state.NewInteractiveFile + Load → LastRetro
  ├── logger.ReadModelStats(logsDir)  → map[string]ModelStats
  └── logger.ReadProcessTrend(path)   → ProcessTrend (SPC)
```

## Tasks

### Task 1: Restore formatting functions to format.go
**Size:** M (restore ~250 lines from reference)
**Files:** `internal/runner/format.go`

Restore these functions from the old format.go:
- `formatRun(s *Status) string`
- `formatRunningLine(s *Status) string`
- `formatDuration(d time.Duration) string`
- `formatHealth(lastRetro time.Time, iterationsSinceReview int) string`
- `formatModelPerformance(stats map[string]logger.ModelStats) string`
- `formatRecommendation(rec string) string`
- `formatSPCSummary(trend *logger.ProcessTrend) string`
- `formatSPCLine`, `formatSPCValue`, `simplifySPCMetric` helpers
- `formatEscalationBreakdown`, `formatRecurrenceBreakdown` helpers
- SPC metric constants

These are pure formatting functions with no dependencies on Runner.

### Task 2: Implement PrintStatus standalone function
**Size:** M (~80 lines)
**Files:** `internal/runner/print_status.go` (new file)
**Depends on:** Task 1

Create `PrintStatus(gromitDir string, cfg *config.Config, w io.Writer, processChecker func(int) bool) error`:
- Adapted from old `Runner.Status()` in lifecycle.go:122-213
- Handles stale PID detection and status file cleanup
- Reads all data sources (pipeline status, state, interactive state, model stats, process trend)
- Formats and writes all sections: Pipeline, Run, SPC, Health, Model Performance, Next Action

### Task 3: Fix StatusWriter data gaps
**Size:** S (~5 lines)
**Files:** `internal/pipeline/epilogue/epilogue.go`, `internal/runner/constructor.go`

- In epilogue.go:196, pass `in.Result.Model` (if non-nil) instead of `""`
- In constructor.go:182, compute timeBudgetMinutes from deadline (similar to epilogue's computeTimeBudgetMinutes)

### Task 4: Update CLI to use PrintStatus
**Size:** S (~15 lines)
**Files:** `cmd/gromit/main.go`

Replace the `showStatus` function body with a call to `runner.PrintStatus(gromitDir, cfg, os.Stdout, nil)`.

### Task 5: Verify acceptance tests pass
**Size:** S
**Depends on:** Tasks 1-4

Run acceptance tests: `go test -tags acceptance ./internal/runner/acceptance/ -run Status`
Verify all `TestOrchestratorHelper_Status*` tests pass.

## Dependencies
- Task 1 → Task 2 (formatting needed before PrintStatus)
- Tasks 1-3 → Task 4 (all pieces needed before CLI wiring)
- Tasks 1-4 → Task 5 (verify everything works)

## Testing Strategy
- Unit tests for restored formatting functions (already exist in old `format_test.go` — restore if needed)
- Acceptance tests already written and expecting the rich output
- Run `go test ./internal/runner/...` and `go test ./cmd/gromit/...`
- Run `go test -tags acceptance ./internal/runner/acceptance/ -run Status`
