---
id: decompose-complexity-estimation
source_ideas: []
created: 2026-02-14
epic: codebase-health
---

# Pre-Build Complexity Estimation via File Count

## Specification

Beads that touch many files across unrelated packages fail at a higher rate than focused beads, but model selection can't account for this because complexity is unknown until after execution. This spec adds a file-count estimate to the decompose output so the runner can route high-scope beads to stronger models before they fail.

### Approach: Decompose-Time Estimation

The agent performing decomposition already understands the scope of each bead — it reads the plan, identifies files, and writes the description. We ask it to write down a number it already knows: how many files will this bead touch?

This is more reliable than parsing acceptance criteria text after the fact, because the decompose agent has the full plan context and can reason about scope. A regex over prose would be noisy and miss implicit file references.

### Change 1: Add `estimated_files` to beadDef

Extend the `beadDef` struct in `internal/pipeline/decompose.go` with an optional field:

```go
type beadDef struct {
    Title              string   `json:"title"`
    Description        string   `json:"description"`
    Priority           string   `json:"priority"`
    AcceptanceCriteria []string `json:"acceptance_criteria"`
    DependsOnIndex     []int    `json:"depends_on_index"`
    EstimatedFiles     int      `json:"estimated_files,omitempty"`
}
```

This field is optional (`omitempty`) so existing decompose outputs without it parse correctly.

### Change 2: Update the decompose skill

Add `estimated_files` to the output format in `skills/gromit-decompose/SKILL.md`. In the Bead Definition Fields section:

- `estimated_files`: Integer count of files this bead will create or modify (include test files, mock files, and any touched interfaces)

Add to the Output Format example so Claude sees the field in the JSON template. Add to the description guidelines: "Include a realistic file count — count every file that will have at least one line changed."

### Change 3: Auto-apply complexity label during bead creation

In `Decompose()` in `internal/pipeline/decompose.go`, after parsing bead definitions but before creating beads, apply a `complexity:high` label when the estimate exceeds the threshold:

```go
const highComplexityFileThreshold = 5

for i, def := range beadDefs {
    labels := []string{fmt.Sprintf("spec:%s", input.PlanName)}
    if def.EstimatedFiles > highComplexityFileThreshold {
        labels = append(labels, "complexity:high")
    }
    // ... existing creation logic using labels ...
}
```

This piggybacks on the existing `complexity:high -> opus` routing in `config.SelectModel()` without any changes to `escalation.SelectModel()` or `escalation.SelectTier()`.

### Change 4: Log predicted vs actual for calibration

After a bead completes (success or failure), the runner already has `git diff --stat` output showing actual files changed. Add the estimated file count to `IterationLog` so retro analysis can compare predicted vs actual:

```go
// In IterationLog (internal/logger/logger.go)
EstimatedFiles int `json:"estimated_files,omitempty"`
```

The runner reads `EstimatedFiles` from the bead's description or from a label like `estimated-files:7` (since bd doesn't store arbitrary metadata, the simplest path is encoding it in a label). The retro can then compute prediction accuracy.

### Label vs Description Encoding

bd beads don't have an arbitrary metadata field. Two options for persisting the estimate:

**Option A: Encode in a label** — `estimated-files:7`. Simple, queryable via `bd list --label`, but slightly abuses the label system. The label wouldn't affect model selection directly (only `complexity:high` does that), but it carries the data for logging.

**Option B: Encode in the description** — Append `[estimated-files: 7]` to the description text. The runner would parse it out. More fragile but doesn't pollute labels.

**Decision: Use a label.** Labels are structured, queryable, and already used for metadata (`spec:`, `complexity:`, `methodology:`). Adding `estimated-files:N` is consistent with the existing pattern. The decompose executor adds it alongside `spec:<name>` and `complexity:high`.

### What This Does NOT Do

- **Does not block bead creation.** A high estimate triggers a label, not a rejection. The bead is still created and can be executed.
- **Does not replace manual `complexity:high` labels.** Explicit labels set by humans or other heuristics still override via the existing precedence chain.
- **Does not change `SelectModel()` or `SelectTier()`.** The existing label-based routing handles `complexity:high` already. No new routing logic needed.
- **Does not split beads automatically.** If estimated_files > 5, it routes to opus — it doesn't decompose further. Auto-splitting is a separate, more complex feature.

## Acceptance Criteria

- `beadDef` struct includes `EstimatedFiles int` field with `json:"estimated_files,omitempty"` tag
- Decompose skill instructions include `estimated_files` in the field list, output example, and description guidelines
- `Decompose()` adds `complexity:high` label when `EstimatedFiles > 5`
- `Decompose()` adds `estimated-files:N` label for all beads where `EstimatedFiles > 0`
- Existing decompose outputs without `estimated_files` parse correctly (backward compatible)
- `IterationLog` includes `estimated_files` field, populated from the bead's label when available

## Decisions

1. **Decompose-time estimation, not post-hoc text parsing.** The decompose agent already knows the scope — asking it to output a number is cheaper and more accurate than regex over acceptance criteria prose. Acceptance criteria describe behavior ("Users can log in"), not code structure ("internal/auth"), so text parsing would miss most references.

2. **Threshold of 5, not 3.** The bead sizing rules say "soft file limit of 4-5" and "consider splitting" at 6+. A threshold of 5 catches beads at the boundary — they're allowed by sizing rules but benefit from a stronger model. Setting it at 3 would over-escalate: touching interface.go, impl.go, mock_test.go, and impl_test.go for one method is 4 files and routine.

3. **Label, not description, for persistence.** Labels are structured and queryable. The `estimated-files:N` label follows the existing `key:value` label pattern used by `spec:`, `complexity:`, and `methodology:`.

4. **Route to opus, not split.** Auto-splitting is appealing but complex — it would need to re-invoke the decompose agent, manage new dependencies, and handle partial completion. Routing to opus is a single-line label addition that leverages existing infrastructure. If opus still fails on a high-estimate bead, the existing "escalate to splitting" rule in RULES.md applies.

5. **Experiment-compatible.** This change can be deployed as an experiment: add the field and labels but only activate the `complexity:high` auto-labeling behind a config flag. Measure prediction accuracy first, then enable routing.

## Research & Context

### Current State

- **Decompose output** (`internal/pipeline/decompose.go:17-23`): `beadDef` struct has 5 fields. Adding `EstimatedFiles` is a one-line struct change.
- **Bead creation** (`internal/pipeline/decompose.go:86-126`): Labels are built per-bead starting with `spec:<name>`. Adding conditional labels is straightforward.
- **Decompose skill** (`skills/gromit-decompose/SKILL.md`): Field list at line 68-72, output example at lines 104-139. Both need the new field added.
- **Model selection** (`internal/runner/escalation/tierselect.go:24-41`): `SelectModel()` delegates to `cfg.SelectModel()` which checks labels first. `complexity:high` already routes to opus — no changes needed.
- **Label conventions** (`internal/config/config.go:40-45`): Default labels map includes `complexity:high -> opus`, `complexity:low -> haiku`.
- **Iteration logging** (`internal/logger/logger.go`): `IterationLog` struct would need one new field.
- **Bead struct** (`internal/bead/bead.go:15-28`): Has `Labels []string` — the `estimated-files:N` label can be parsed from here at runtime.

### Failure Rate Analysis

The retro identified a 31.4% overall failure rate (22/70 beads). Five Whys analysis traced this to imperfect complexity proxies:
1. Some beads exceed model capability at their assigned tier
2. Scope is sometimes too broad or acceptance criteria ambiguous
3. Decomposition doesn't always catch complexity hidden in "simple" titles
4. Title-based routing and priority-based model selection are imperfect proxies
5. True complexity estimation requires analyzing scope, which only the decompose agent has

This spec addresses root cause #4 by making the decompose agent's scope knowledge explicit and actionable.

### Risk

- **Estimate accuracy unknown.** The decompose agent may systematically over- or under-estimate. The `estimated-files:N` label + `IterationLog` field enable calibration, but the first cycle is an experiment.
- **Over-escalation cost.** If the threshold is too low, too many beads route to opus ($1.51/bead vs $0.48 for sonnet). The threshold of 5 is conservative — most beads should stay below it.
- **Skill compliance.** Claude may ignore or hallucinate the `estimated_files` field. The `omitempty` tag and the existing validation-free approach to bead definitions mean a missing field is harmless (defaults to 0, no label applied).
