---
id: refine-blank-idea
source_ideas: []
created: 2026-02-07
epic: developer-experience
---

# Blank Idea Refinement from the Picker

## Specification

When a user runs `gromit refine` with no arguments, the interactive picker currently only shows existing unrefined backlog items. This feature adds a "Something new" option to the picker and handles the case where the backlog is empty.

### Behavior

**When unrefined backlog items exist**, the picker displays them as before, with a new final option:

```
Select an idea to refine:

  1. [feature] Add user authentication
  2. [bug]     Login page crashes on mobile
  3. [new]     Something new...

Choice [1-3]:
```

Selecting "Something new" launches a blank Claude session with no pre-set idea text. Claude asks the user what they'd like to refine.

**When no unrefined backlog items exist** (empty backlog or all items already refined), the picker is skipped entirely. A blank Claude session launches immediately with a message like:

```
No unrefined backlog items. Starting a blank refinement session...
```

### Blank Claude Session

The system prompt for a blank session omits the "Idea to refine" section. Instead, it includes a directive telling Claude that no idea was provided and it should ask the user what they want to refine. The refine skill already handles this case — its instructions say "If refining an ad-hoc idea, ask for the initial description."

### Auto-Created Backlog Item

When a blank session produces a spec file, the CLI creates a backlog item after the session:

- **ID**: Generated via `backlog.GenerateID()` (timestamp-based)
- **Text**: Extracted from the spec's markdown title (`# Title` heading)
- **Type**: `"feature"` (default; the spec came from open-ended refinement)
- **Status**: `"refined"`
- **SpecName**: The spec's filename without `.md`

The spec's `source_ideas` frontmatter is left as `[]` — the backlog item links to the spec via `spec_name`, providing one-way traceability.

### Pipeline Chaining

Chaining (`chainAfterRefine`) works the same as today. After the session, if specs were created, the user is offered the option to continue to `plan` and `decompose`.

## Acceptance Criteria

- Running `gromit refine` with unrefined backlog items shows a "Something new" option as the last entry in the picker
- Selecting "Something new" launches a Claude session with no pre-set idea text
- Running `gromit refine` with no unrefined items skips the picker and launches a blank Claude session directly
- When a blank session creates a spec, a backlog item is auto-created with the spec title as text, status "refined", and spec_name linked

## Decisions

1. **Blank Claude session over terminal prompt** The user wanted to go straight into Claude rather than typing the idea in the terminal first. This is more natural — the user describes their idea conversationally to Claude rather than composing a one-liner.

2. **Skip picker when backlog is empty** Showing a picker with only "Something new" adds a pointless interaction. Going straight to the blank session is faster.

3. **Auto-create backlog item for traceability** Unlike ad-hoc refinement via `gromit refine "text"`, the "something new" flow creates a backlog item after the spec is written. This keeps the backlog as a complete record of all refined ideas.

4. **Use spec title as backlog item text** The spec's `# Title` heading is the most human-readable summary available. The backlog item is created already in "refined" status since it was born from a completed refinement session.

5. **Leave source_ideas empty in spec** The backlog item is created after the spec, so backfilling the spec's frontmatter would add complexity for little value. The backlog item already points to the spec via `spec_name`.

## Research & Context

### Current State

The refine command lives in `cmd/gromit/refine.go`. The interactive picker logic is at lines 63-116. When the backlog is empty, lines 78-83 print a help message and exit. The system prompt is constructed at lines 152-162 with the idea text baked in.

The backlog package at `internal/backlog/backlog.go` provides `Add()`, `List()`, `Get()`, `Update()`, and `GenerateID()` — all needed for the auto-created backlog item.

### Key Files

- `cmd/gromit/refine.go` — Main refine command, picker logic, Claude session launch, post-session spec detection
- `internal/backlog/backlog.go` — Backlog CRUD operations and `Idea` struct
- `skills/gromit-refine/SKILL.md` — Embedded skill that guides Claude's refinement conversation
- `cmd/gromit/chain.go` — Pipeline chaining after refinement

### Spec Title Extraction

The auto-created backlog item needs the spec's `# Title`. This requires reading the newly created spec file and finding the first `# ` heading. This is new functionality — no existing code parses spec file content.
