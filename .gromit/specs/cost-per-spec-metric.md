---
id: cost-per-spec-metric
created: 2026-02-18
epic: observability-and-diagnostics
---

# Cost-Per-Spec Metric

## Specification

Gromit tracks cost per iteration and cost per completed bead, but has no way to answer "how much did spec X cost?" Specs are the unit of planned work — each spec produces multiple beads, each bead produces multiple iterations. Without spec-level cost aggregation, there's no way to compare the real unit economics of different specs or identify which kinds of work are expensive.

Beads already carry `spec:<name>` labels. The iteration log already records `bead_id` and `cost_usd`. The missing link: the iteration log doesn't record which spec a bead belongs to. This spec adds that link and builds a retrospective cost-per-spec view in `gromit stats`.

### Data Model Changes

**IterationResult** (`internal/runner/runtypes/types.go`)

Add one field:
```
SpecID string // spec name extracted from bead's "spec:<name>" label
```

Set by the runner when the bead is fetched, using the existing `bead.FindSpecLabel(b.Labels)`.

**IterationLog** (`internal/logger/logger.go`)

Add one field:
```
SpecID string `json:"spec_id,omitempty"`
```

**Log Mapping** (`internal/runner/logging.go`)

Map `result.SpecID` to `log.SpecID` in `writeIterationLog()`.

Historical iterations without `spec_id` parse correctly (`omitempty` means the field is absent in old data). They appear under "(unassigned)" in aggregation.

### Aggregation

**New function** in `internal/logger/`:

```go
// CostPerSpec aggregates iteration costs by spec ID.
// Returns a map of spec name -> SpecCost.
// Iterations with no spec_id are grouped under "(unassigned)".
type SpecCost struct {
    TotalCostUSD float64
    Iterations   int
    Beads        int            // distinct bead IDs
    ModelMix     map[string]int // model -> iteration count
}

func CostPerSpec(logsDir string) (map[string]*SpecCost, error)
```

Reads all `run-*.jsonl` files in `logsDir` (same pattern as `ReadModelStats`). Groups each `IterationLog` entry by `spec_id`. Counts distinct `bead_id` values per spec. Tallies model usage per spec.

### Stats Output

**New section** in `gromit stats` output, after cost-per-bead:

```
Cost per spec:
  cost-optimized-routing    $4.23  (12 iterations, 5 beads)  opus=2 sonnet=4 haiku=6
  reduce-iteration-cost     $2.87  (8 iterations, 3 beads)   sonnet=3 haiku=5
  (unassigned)              $1.05  (4 iterations, 4 beads)   haiku=4
```

Sorted by total cost descending. The `--json` flag includes the same data as a `cost_per_spec` key in the JSON output.

### Where SpecID Gets Set

The runner already has the bead when building the iteration result. The spec label extraction happens in one place — the runner's iteration processing — and flows through the existing logging pipeline:

1. Runner fetches bead -> `bead.FindSpecLabel(b.Labels)` -> stores on `IterationResult.SpecID`
2. `writeIterationLog()` copies `result.SpecID` to `log.SpecID`
3. `LogIteration()` writes to JSONL with `spec_id` field
4. `CostPerSpec()` reads JSONL and aggregates

### What Does Not Change

- No new configuration fields
- No new files beyond the aggregation function
- No changes to bead labeling — beads already carry `spec:<name>` labels
- No changes to the runner's control flow (just reading an existing label)
- No backfill of historical data — old entries gracefully appear as "(unassigned)"
- The existing cost-per-bead view remains unchanged

## Acceptance Criteria

- `spec_id` appears in iteration log JSONL for iterations whose bead has a `spec:<name>` label
- `spec_id` is omitted for iterations whose bead has no spec label
- `gromit stats` displays a "Cost per spec" section showing total cost, iteration count, bead count, and model mix per spec
- Specs are sorted by total cost descending
- Beads without a spec label are grouped under "(unassigned)"
- `gromit stats --json` includes `cost_per_spec` in its JSON output
- Sum of per-spec costs equals total project cost

## Decisions

1. **Record at log time, not query time.** Embedding `spec_id` in the JSONL at write time makes stats queries fast and independent of bd. The alternative (looking up bead labels at query time) would be slow and fragile.

2. **No backfill.** Historical iterations appear as "(unassigned)". The cost of building and maintaining a migration tool outweighs the value — the "(unassigned)" bucket shrinks naturally as new iterations include `spec_id`.

3. **Model mix per spec, not just cost.** Raw cost alone doesn't explain *why* a spec was expensive. Showing the model breakdown (e.g., "this spec used 8 opus iterations") reveals whether cost came from escalation, complexity, or volume.
