---
id: auto-generate-acceptance-criteria
source_ideas: []
created: 2026-03-02
---

# Auto-Generate Acceptance Criteria for Beads

## Specification

Beads can enter the ready queue without acceptance criteria (empty `ExpectedOutputs`). Today there is no mechanism to detect which beads are missing criteria and no way to add criteria after creation. This feature adds two capabilities:

1. **Gate-level auto-generation**: When a bead reaches the gate (`prepare/gate.go`) with empty `ExpectedOutputs`, an LLM generates acceptance criteria before the bead proceeds. The generation uses the richest available context: the linked spec (via `spec:name` label) or plan if one exists, falling back to the bead's own title and description.

2. **Queue visibility**: `gromit status` reports a count of beads missing acceptance criteria, and a query flag (e.g., `--missing-criteria`) lists them by ID so operators can see queue health at a glance.

### Gate Behavior

The auto-generation step runs inside the Gate stage, after precheck and before stuck detection. When triggered:
- Look up the bead's `spec:` label to find the linked spec file (`.gromit/specs/<name>.md`)
- If a plan exists for that spec (`.gromit/plans/<name>.md`), include it as additional context
- If no spec is linked, use the bead's title and description as the sole context
- Call an LLM (using the validation-tier model) with the context and ask it to produce 1–5 concrete, testable acceptance criteria
- Write the generated criteria back to the bead's `ExpectedOutputs` field (via `bd` CLI or equivalent mutation)
- Log the generation event and proceed with normal gate flow

If generation fails (LLM error, timeout), log a warning and allow the bead to proceed without criteria — do not block the pipeline on generation failure.

### Configuration

A single boolean toggle in the config controls the feature:

```yaml
gate:
  auto_generate_criteria: true  # default: true
```

When `false`, beads with missing criteria pass through the gate unchanged (current behavior).

### Status Visibility

`gromit status` includes a line showing how many open/ready beads lack acceptance criteria. A `--missing-criteria` flag on list-style commands filters to only those beads.

## Acceptance Criteria

- When `gate.auto_generate_criteria` is `true` (default) and a bead has empty `ExpectedOutputs`, the gate generates criteria from the linked spec/plan before proceeding
- When `gate.auto_generate_criteria` is `false`, beads with empty criteria pass through unchanged
- Context resolution prefers spec > plan > bead title+description, using the richest source available
- Generated criteria are written back to the bead so subsequent gate passes don't regenerate
- If criteria generation fails (LLM error, timeout), the bead proceeds with a logged warning — no pipeline block
- `gromit status` displays a count of beads missing acceptance criteria
- A query mechanism (flag or subcommand) lists bead IDs that are missing acceptance criteria
- Generated criteria contain 1–5 concrete, testable items (matching the existing max-5 validation constraint)

## Decisions

1. **Gate-level, not standalone command** — Auto-generation happens at the gate rather than as a separate `gromit enrich` command. This keeps the pipeline seamless: beads are enriched just-in-time without requiring operator intervention. The gate already has the right extension points (`DataQualityBlocker` pattern, builder-style wiring).

2. **Simple on/off config, automatic context resolution** — Rather than letting users configure context sources, the system automatically picks the best available context (spec → plan → title). This avoids config complexity while producing the best results. Users who don't want auto-generation can disable it entirely.

3. **Fail-open on generation errors** — If the LLM fails to generate criteria, the bead proceeds anyway. Blocking the pipeline on a best-effort enrichment step would be worse than running without criteria. The warning log gives operators visibility.

4. **Validation-tier model** — Uses the validation model (typically haiku) for generation since this is a utility task, not a creative one. Keeps cost low.

5. **Write-back to bead** — Generated criteria are persisted to the bead via `bd` so they're visible in the tracker and don't need regeneration on retry.

## Research & Context

### Current State

- **Bead struct** (`internal/bead/bead.go:21-42`): Has `ExpectedOutputs []string` and legacy `AcceptanceCriteria string`. No minimum-count validation exists.
- **Gate stage** (`internal/pipeline/prepare/gate.go`): Runs precheck → readiness → stuck → data quality → spec SPC → scope checks. The auto-generation step would slot in early (after precheck, before readiness) since readiness assessment benefits from having criteria present.
- **DataQualityBlocker interface** (`gate.go:42-44`): Existing pattern for pluggable gate checks. Auto-generation could follow a similar wiring pattern but with a different interface (enricher rather than blocker).
- **Config types** (`internal/config/config_types.go`): No `GateConfig` section exists yet — this would be a new top-level config section.
- **Bead helpers** (`internal/bead/bead_helpers.go`): `FindSpecLabel()` already extracts spec names from labels, providing the spec-lookup mechanism needed for context resolution.
- **Status command** (`internal/pipeline/status.go`): Currently reads bead counts by status. Would need a new query for beads with empty `ExpectedOutputs`.
- **Bead query client** (`internal/bead/bead_query.go`): Has `ListByStatus`, `CountByStatus` etc. — a new `ListMissingCriteria` or filter could be added here.
