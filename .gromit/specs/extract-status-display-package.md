---
id: extract-status-display-package
source_ideas: []
created: 2026-02-26
epic: codebase-health
---

# Extract Status Display Formatting into Own Package

## Problem

`internal/runner/format.go` is over the 550-line production file limit (currently ~577 lines) and contains formatting logic for status display (SPC control charts, model performance stats, health metrics, run status, pipeline status, next-action recommendations) that is orthogonal to runner orchestration. As phase-cost-optimization adds more display sections, the file will grow further.

## Approach

- Create a new package `internal/runner/display/` (preferred) or `internal/status/display/`
- Move all formatting functions from `internal/runner/format.go` to the new package: `formatRun`, `formatHealth`, `formatSPCSummary`, `formatModelPerformance`, `formatRecommendation`, `formatPipeline`, `formatDuration`, `formatSPCLine`, `formatSPCValue`, `simplifySPCMetric`, `formatEscalationBreakdown`, `formatRecurrenceBreakdown`, `formatReliabilityLine`, `formatRunningLine`, `formatElapsedSuffix`, `formatIterationPrefix`, `formatItems`
- Export the functions that `internal/runner/` callers need (capitalize names as required)
- Leave `internal/runner/format.go` as a thin re-export file if needed to avoid breaking callers during transition, or update all call sites directly if the set of callers is small
- Move associated tests from `internal/runner/format_test.go` to the new package's test file
- Verify `internal/runner/format.go` is under 550 lines after extraction

## Files to Change

- `internal/runner/display/` — new package with extracted formatting functions and tests
- `internal/runner/format.go` — reduced to re-exports or removed if all callers are updated
- `internal/runner/format_test.go` — moved to `internal/runner/display/display_test.go`
- Any files in `internal/runner/` that call the formatting functions — update import paths

## Acceptance Criteria

- `internal/runner/format.go` is under 550 lines after extraction
- All formatting functions are in the new `display` package
- All existing format tests pass in the new package location
- No test failures in `go test ./internal/runner/...` or `go test ./internal/runner/display/...`
- `go build ./...` succeeds with no compilation errors
