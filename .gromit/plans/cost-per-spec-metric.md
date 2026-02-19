---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T01:32:44Z"
id: cost-per-spec-metric
source_spec: cost-per-spec-metric
---

# Cost-Per-Spec Metric Implementation Plan

**Goal:** Close two remaining gaps in the existing cost-per-spec implementation to fully meet the spec's acceptance criteria.

**Architecture:** The data model, logging pipeline, aggregation function, and stats display are already implemented. Two changes needed: enhance text output format to show iteration count, bead count, and model mix; fix the unassigned label to use parentheses.

**Tech Stack:** Go

**Spec:** `.gromit/specs/cost-per-spec-metric.md`

---

## Architecture

The cost-per-spec feature is ~95% implemented. The full data flow already works:

```
bead.Labels → bead.FindSpecLabel() → bc.Result.SpecID → writeIterationLog() → JSONL spec_id
→ CostPerSpec() aggregation → gromit stats text/JSON output
```

**Already complete:**
- `SpecID` field on `IterationResult` and `IterationLog`
- `writeIterationLog()` mapping
- `setupBeadContext` extracting spec label via `bead.FindSpecLabel(b.Labels)`
- `SpecCost` struct with `TotalCostUSD`, `Iterations`, `Beads`, `ModelMix`
- `CostPerSpec()` aggregation function reading `run-*.jsonl` files
- `gromit stats` calling `CostPerSpec` with sorted text and JSON output
- 10 existing tests across `modelstats_test.go` and `stats_test.go`

**Remaining gaps:**
1. Text output only shows `spec-name: $X.XX` — spec requires iteration count, bead count, and model mix
2. Unassigned constant is `"unassigned"` — spec requires `"(unassigned)"`

## Test Strategy

- Update `TestCostPerSpec_EmptySpecIDMapsToUnassigned` to expect `"(unassigned)"`
- Update `TestStatsCmd_ShowsCostPerSpecSortedByTotalCost` to verify enhanced text format
- All other existing tests pass unchanged

## Implementation Tasks

### Task 1: Fix unassigned label and enhance text output

**Files:**
- Modify: `internal/logger/modelstats.go`
- Modify: `cmd/gromit/stats.go`
- Modify: `internal/logger/modelstats_test.go`
- Modify: `cmd/gromit/stats_test.go`

**What to Do:**

In `internal/logger/modelstats.go`, change `const unassignedSpecID = "unassigned"` to `const unassignedSpecID = "(unassigned)"`.

In `cmd/gromit/stats.go`, enhance the text output `fmt.Printf` at line 115 in the cost-per-spec loop to include iteration count, bead count, and model mix. The format should match the spec:
```
  cost-optimized-routing    $4.23  (12 iterations, 5 beads)  opus=2 sonnet=4 haiku=6
```

Build the model mix string by iterating `entry.cost.ModelMix` sorted alphabetically, formatting as `model=count` pairs separated by spaces.

Update `TestCostPerSpec_EmptySpecIDMapsToUnassigned` in `modelstats_test.go` to check for `"(unassigned)"` key. Update `TestStatsCmd_ShowsCostPerSpecSortedByTotalCost` in `stats_test.go` to verify the output contains iteration count and bead count strings.

**Acceptance Criteria:**
- `gromit stats` text output shows total cost, iteration count, bead count, and model mix per spec
- Empty spec_id entries use `"(unassigned)"` as the group key in both text and JSON output
- All existing tests pass

**Dependencies:** None

---

## Notes

- This is a very small change — most of the spec was already implemented by prior beads (gromit-szaw4, gromit-grsdc, gromit-gm8ed, gromit-5l15c and others in the open beads list).
- The JSON output already includes the full `SpecCost` struct with all fields — only the text display was incomplete.
- The model mix sort order (alphabetical) ensures deterministic output for tests.
