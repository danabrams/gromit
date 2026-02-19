---
id: dynamic-code-map
source_ideas: []
created: 2026-02-19
---

# Dynamic Code Map

## Specification

Build prompts currently include the full CLAUDE.md file, which contains a static architecture listing of all packages plus key behavioral principles. For beads that only touch 1-2 packages, most of the architecture section is irrelevant token waste.

This feature replaces the architecture section of CLAUDE.md with a dynamically generated, bead-scoped code map — a minimal listing of only the packages relevant to the current bead. The key principles section of CLAUDE.md is always included regardless.

### Package Discovery

Relevant packages are identified through two layers, applied in order:

**Layer 1 — Spec and bead description parsing:** Extract Go package paths from the bead's linked spec file and bead description using pattern matching (e.g., paths matching `internal/...`, `cmd/...`). Specs produced by the refine skill typically reference specific files and packages in their "Research & Context" section, so this covers most cases at zero cost.

**Layer 2 — Sibling bead enrichment:** When the bead has a parent epic and sibling beads have already completed iterations, union their `TouchedPackages` data (already computed from git diff) into the relevant set. This captures the actual working set for the epic and handles drift from what the spec predicted.

### Code Map Format

The code map is a minimal listing: each relevant package as a path with a one-line description. For example:

```
- `internal/prompt/` — prompt template rendering
- `internal/runner/` — core loop orchestration
```

Descriptions are sourced from the existing CLAUDE.md architecture section or from the package's Go doc comment.

### Prompt Assembly

When at least one relevant package is identified:
- The architecture section of CLAUDE.md is replaced with the scoped code map
- The key principles section of CLAUDE.md is always included

When no relevant packages are identified (neither layer produces results):
- Fall back to including the full CLAUDE.md as today

This is strictly additive — the prompt is never worse than the current behavior.

## Acceptance Criteria

- When a bead has a linked spec that mentions Go package paths, those packages appear in the code map
- When a bead's description mentions Go package paths, those packages appear in the code map
- When sibling beads under the same parent have completed iterations, their TouchedPackages are included in the code map
- When relevant packages are identified, the build prompt includes only those packages (not the full architecture listing) plus the key principles section
- When no relevant packages are identified, the build prompt includes the full CLAUDE.md unchanged
- The code map for a bead with 2 relevant packages uses fewer tokens than the full architecture section

## Decisions

1. **Minimal detail level for code maps.** Package path + one-line description only. No exported symbols, file lists, or dependency graphs. This minimizes token usage and keeps the feature simple. Can be upgraded to medium detail (adding key exported types/functions) later if minimal proves insufficient.

2. **Two discovery layers, not three.** Spec/description parsing (Layer 1) and sibling bead enrichment (Layer 2) are both free — no additional LLM calls or latency. A third layer (extending the scope gate to output relevant packages) is deferred to the backlog as a future enhancement.

3. **Replace architecture section only, not all of CLAUDE.md.** The key principles section contains universal behavioral guidelines (fresh context, state in files, escalation chain) that are always relevant regardless of which packages a bead touches. Only the package listing is swapped out.

4. **Full CLAUDE.md as fallback.** When neither discovery layer produces packages, the prompt includes CLAUDE.md unchanged. This ensures the feature is strictly additive — never worse than today.

## Research & Context

### Current State

**CLAUDE.md structure** (`CLAUDE.md`): Two sections — "Architecture" (package directory listing, ~12 lines) and "Key Principles" (5 behavioral rules). The architecture section is the target for replacement.

**Prompt rendering** (`internal/prompt/prompt.go`): `BuildContext()` loads CLAUDE.md via `LoadClaudeMD()` and stores it in `Context.ClaudeMD`. Templates reference it as `{{.ClaudeMD}}`. The content is cached after first load.

**Budget trimming** (`internal/prompt/budget.go`): CLAUDE.md is already the first field trimmed when context exceeds the token budget, confirming the system treats it as lower-priority context.

**TouchedPackages** (`internal/runner/runtypes/types.go`): Already tracked on `BeadContext` and populated via `DetectTouchedPackages()` in `internal/runner/methodology/diff.go`. Currently used only for test command scoping and learning extraction filtering.

**Spec loading** (`internal/prompt/prompt.go`): Specs are loaded via `LoadSpec()` using the `spec:<name>` label on the bead or its parent. The spec content is available as `Context.Spec`.

**Scope gate** (`internal/prompt/prompt.go`): The `ScopeEstimate` struct exists but does not currently output relevant packages. This is the deferred Layer 3 enhancement.

### Backlog: Scope Gate Enhancement (Layer 3)

The existing scope gate makes a cheap LLM call to estimate complexity. It could be extended to also output 2-5 relevant package paths at near-zero incremental cost. This would catch cases where specs don't mention specific paths. Deferred until Layers 1 and 2 are validated in practice.
