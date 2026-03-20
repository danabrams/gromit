# 0004 Series — Deferred Items

Items explicitly deferred from the 0004d/0004e specs. Each is a candidate for a future spec.

## Pulled into 0004d (from original deferred list)
- ~~Accept with field overrides~~ → now in 0004d as `--title`, `--change`, `--rationale` flags
- ~~Reject-after-accept supersession~~ → now in 0004d as core lifecycle

## Pulled into 0004e (from original deferred list)
- ~~Distiller rejection feedback~~ → now in 0004e as "previously rejected proposals" prompt section
- ~~Proposal grouping and --dismiss-group~~ → now in 0004e as deterministic + LLM clustering
- ~~Global/local layered resolution~~ → now in 0004e as cross-project scope

## Still deferred

### In-place proposal editing
**What:** An `--edit` flag on `review proposals accept` that opens the proposal in `$EDITOR` before materializing, allowing interactive field modifications.
**Why deferred:** 0004d now supports CLI field overrides (`--title`, `--change`, `--rationale`) which cover the common case. `$EDITOR` integration is additional sugar.
**Prerequisite:** 0004d.

### Proposal expiration and staleness detection
**What:** Automatically flag or archive proposals that have been pending beyond a configurable threshold (e.g., 30 days). Prevent unbounded accumulation of unreviewed proposals.
**Why deferred:** Not a problem until proposal volume is high enough to matter. 0004e's grouping helps manage volume in the near term.
**Prerequisite:** 0004e.

### Trend analysis and dashboards
**What:** Aggregate proposal and promotion history across runs to surface patterns — e.g., "validation_gap proposals are 3x more common than doctrine_rule proposals", "80% of planner_heuristic proposals get accepted".
**Why deferred:** Requires meaningful volume of promotion decisions to be useful. The per-run `proposal-decisions.json` files provide the raw data.
**Prerequisite:** 0004e with accumulated usage data.

### Web UI for proposal triage
**What:** A browser-based interface for reviewing, grouping, and deciding on proposals — richer than CLI for large proposal volumes.
**Why deferred:** CLI-first approach covers the core workflow. Web UI is a presentation layer change, not a data model change.
**Prerequisite:** 0004e.

### Bulk accept/reject operations
**What:** Accept or reject multiple proposals in a single command beyond `--dismiss-group` — e.g., `review proposals accept-all --type doctrine_rule --scope local` or interactive multi-select.
**Why deferred:** `--dismiss-group` (0004e) handles the most common bulk case. Broader bulk operations add CLI complexity without changing the promotion pipeline.
**Prerequisite:** 0004e.

### Automatic global promotion
**What:** Automatically promote proposals to global scope when they've been accepted locally in N or more projects.
**Why deferred:** Needs cross-project proposal visibility and frequency tracking, neither of which exists yet.
**Prerequisite:** 0004e.
