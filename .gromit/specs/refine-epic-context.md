---
id: refine-epic-context
source_ideas: []
created: 2026-02-11
---

# Epic Context Injection for Refine

## Specification

When `gromit refine` processes a backlog item (or receives an ad-hoc idea with context), it parses the context field to detect references to known epics. If an epic is detected, the refine prompt is enriched with:

1. **The full epic document** — the contents of `.gromit/epics/<epic-id>.md`, giving the refine agent visibility into the problem space, vision, architecture, and design decisions.

2. **Sibling spec summaries** — for each spec already linked to the epic (via `epic:` frontmatter field), include the spec's title and acceptance criteria. This prevents the refine agent from producing overlapping specs and helps it understand what ground has already been covered.

3. **Epic frontmatter instruction** — the prompt tells Claude to include `epic: <epic-id>` in the new spec's frontmatter, so the spec is automatically linked to the epic without manual editing.

### Epic Detection

Detection works by substring matching the backlog item's `context` field against known epic IDs:

1. List all `.md` files in `.gromit/epics/`
2. Extract `epic_id` from each file's YAML frontmatter
3. Check if any epic ID appears as a substring in the context field
4. If multiple match, use the longest match (most specific)

This handles freeform context like "Part of multi-interface-architecture epic", "f multi-interface-architecture epic" (truncated input), or just "multi-interface-architecture".

### Prompt Enrichment

When an epic is detected, a new `## Epic Context` section is inserted into the system prompt between the idea text and the specs directory line. The section contains:

```
## Epic Context

This idea is part of the **<epic-title>** epic (`<epic-id>`).
Include `epic: <epic-id>` in the spec frontmatter.

### Epic Document

<full epic document content>

### Sibling Specs

**<spec-title>** (`<spec-id>`)
Acceptance Criteria:
- <criterion 1>
- <criterion 2>
...

**<spec-title>** (`<spec-id>`)
Acceptance Criteria:
- <criterion 1>
...
```

If no sibling specs exist yet, the section says "No other specs have been created for this epic yet."

### No Epic Detected

When no epic is detected from the context field, the refine prompt is unchanged — no epic section is added. This is the common case for standalone ideas.

## Acceptance Criteria

- When a backlog item's context field contains a known epic ID, the refine prompt includes the full epic document and sibling spec summaries
- When no epic ID matches, the refine prompt is unchanged from current behavior
- Sibling spec summaries include only the title and acceptance criteria, not full spec content
- The prompt instructs Claude to set `epic: <epic-id>` in the new spec's frontmatter
- Detection handles substring matching (e.g., "Part of X epic", "X", truncated strings containing X)

## Decisions

1. **Substring matching over structured field** — Rather than adding an `epic` field to backlog items, we parse the freeform `context` field. This works retroactively with existing backlog items and doesn't require changing the `gromit add` workflow. The context field already contains epic references naturally.

2. **Longest match wins** — If multiple epic IDs appear in the context, the longest matching ID is used. This avoids false positives from short IDs that might be substrings of other words.

3. **Title + acceptance criteria only for siblings** — Full spec content could make the prompt very large with many siblings. Title and acceptance criteria give the refine agent enough to avoid overlap without bloating the context.

4. **Explicit frontmatter instruction** — Rather than hoping Claude adds `epic:` on its own, we explicitly instruct it in the prompt. This closes the loop so specs are properly linked automatically.

## Research & Context

### Current State

The system prompt is built in `cmd/gromit/refine.go:164-188`. It currently includes only the idea text, specs directory path, and the RefineSkill content. There is no epic awareness.

Epic documents live in `.gromit/epics/` with `epic_id` in YAML frontmatter. The `findLinkedSpecs` function in `cmd/gromit/epic.go:157-209` already resolves epic → specs by checking `fm["epic"] == epicID` across all spec files. This logic can be reused.

The `frontmatter.Parse` function in `internal/frontmatter/frontmatter.go` handles YAML frontmatter extraction and is used by both epic.go and other commands.

Backlog items use the `Idea` struct in `internal/backlog/backlog.go:14-22` with a `Context` string field that contains freeform text like "Part of multi-interface-architecture epic".

### Extraction of Acceptance Criteria

Acceptance criteria need to be extracted from spec markdown. They appear under a `## Acceptance Criteria` heading as a bulleted list. A simple approach: find the heading, collect lines starting with `- ` until the next `##` heading or end of file.
