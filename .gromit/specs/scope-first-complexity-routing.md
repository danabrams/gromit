---
id: scope-first-complexity-routing
created: 2026-02-22
epic: provider-ecosystem
---

# Scope-First Complexity Routing

## Specification

Gromit should choose the initial model tier from complexity, not priority. To make that reliable, scope-check must run first and publish a normalized complexity result (`low|medium|high`) that routing consumes directly.

Today, this cannot be achieved with `gromit.yaml` alone:
- Initial tier selection still flows through `cfg.SelectTier(priority, labels)` in active execution paths.
- Scope gate currently uses file-count blocking/decomposition logic, but does not set routing tier from scope estimate.
- `ScopeEstimate.Complexity` exists as a type, but is not wired into initial tier selection in the live build path.

### Goals

1. Scope-check happens before build-tier selection for every bead.
2. Initial tier is complexity-driven:
   - `complexity:low -> low`
   - `complexity:medium -> medium`
   - `complexity:high -> high`
3. Priority is removed from initial model-tier selection.
4. Escalation remains enabled as the safety net.
5. Priority may still influence escalation policy (caps/aggressiveness), but not initial tier.

### Non-Goals

- Do not change bead ordering semantics (priority can continue to control ordering).
- Do not remove escalation.
- Do not require decomposition-time complexity labels to be present.

### Required Code Changes

1. Add a scope-first tier resolver in runner execution flow:
   - Resolve/compute complexity before first invocation.
   - Store complexity in `BeadContext` and iteration logs.

2. Wire scope complexity into initial tier selection:
   - Introduce a dedicated selector for initial tier based on complexity.
   - Stop using priority as initial tier input in active runner/build paths.

3. Define fallback behavior when scope estimate is unavailable:
   - Recommended fallback: `medium`.
   - Emit explicit log marker when fallback is used.

4. Normalize complexity source of truth:
   - Prefer scope-check output.
   - If unavailable, optionally accept explicit complexity label override.
   - Ensure exactly one effective complexity value is used for routing.

5. Preserve escalation behavior:
   - Keep `NextEscalationTier` chain.
   - Ensure escalation can move beyond initial complexity tier on failure.

6. Add observability:
   - Log `complexity`, `original_tier`, `actual_tier`, and fallback reason.
   - Keep this visible in iteration JSONL for tuning.

### Config Surface

`gromit.yaml` can express mappings, but needs code support to be authoritative:

```yaml
models:
  labels:
    "complexity:low": low
    "complexity:medium": medium
    "complexity:high": high
```

Optional (future): a dedicated `routing.complexity_map` block to avoid overloading labels.

### Acceptance Criteria

- Initial build invocation tier is determined from scope complexity, not bead priority.
- Scope-check executes before initial tier decision.
- `complexity:medium` is supported as a first-class mapping.
- When scope-check cannot provide complexity, system falls back deterministically and logs why.
- Escalation behavior remains functional (`low -> medium -> high` by configured chain).
- Existing priority-based ordering behavior remains unchanged.
- Unit + integration tests cover:
  - low/medium/high complexity mapping
  - missing complexity fallback
  - escalation from each starting tier
  - priority not affecting initial tier

## Decisions

1. **Complexity owns initial tier.**
   Priority expresses urgency, not technical difficulty.

2. **Scope-check is the complexity producer.**
   This avoids relying on title heuristics or stale/manual labels for initial routing.

3. **Escalation stays as the reliability guardrail.**
   Misclassification must recover by tier escalation without manual intervention.

4. **Fallback defaults to medium.**
   Medium is the safest neutral tier when complexity cannot be determined.

## Research & Context

- `internal/config/config_accessors.go` currently selects tier via priority + label overrides.
- `internal/pipeline/execute/build.go` currently calls `in.Config.SelectTier(in.Bead.Priority, in.Bead.Labels)`.
- `internal/pipeline/prepare/gate.go` scope gate currently blocks/decomposes by file count; it does not set initial build tier.
- `internal/prompt/context_types.go` defines `ScopeEstimate{Complexity}`, indicating partial plumbing exists but is not yet the active routing input.
