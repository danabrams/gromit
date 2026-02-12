---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T12:00:40-05:00"
id: refine-epic-context
source_spec: refine-epic-context
---

# Epic Context Injection for Refine — Implementation Plan

**Goal:** When `gromit refine` processes an idea whose context references a known epic, enrich the refine prompt with the full epic document, sibling spec summaries, and a frontmatter instruction.

**Architecture:** Add three helper functions to `refine.go` (epic detection, acceptance criteria extraction, prompt section builder) and wire them into the existing prompt assembly. Add `resolveEpicsDir` to `resolve.go` for consistency with existing patterns.

**Tech Stack:** Go, YAML frontmatter parsing via `internal/frontmatter`

**Spec:** `.gromit/specs/refine-epic-context.md`

---

## Architecture

All new logic lives in `cmd/gromit/refine.go` as helper functions, with one small addition to `cmd/gromit/resolve.go`. No new packages or files.

**Key Functions:**
1. `resolveEpicsDir(cfg)` — returns epics directory path from config or default `.gromit/epics`
2. `detectEpicFromContext(context, epicsDir)` — scans epic files for IDs, returns longest substring match
3. `extractAcceptanceCriteria(body)` — parses `## Acceptance Criteria` bullet list from spec markdown
4. `buildEpicContextSection(epicID, epicTitle, epicContent, specs, specsDir)` — builds the `## Epic Context` prompt section

**Integration:** In `runRefine()`, after determining the idea text (and before building the system prompt), detect epic from context, and if found, build and insert the epic context section.

**Reuse:** `findLinkedSpecs()` from `epic.go` finds sibling specs. `frontmatter.Parse()` handles YAML extraction.

## Test Strategy

All tests in `cmd/gromit/refine_test.go`. No mocks — use `t.TempDir()` with temporary epic/spec files.

**Key test cases:**
- Detection: exact match, substring in sentence, truncated input, no match, empty inputs, longest-match-wins, bad frontmatter skipped
- Extraction: standard bullet list, stops at next heading, no section present, empty body
- Builder: with sibling specs, without sibling specs, epic ID instruction present

## Implementation Tasks

### Task 1: Add resolveEpicsDir and detectEpicFromContext with tests

**Files:**
- Modify: `cmd/gromit/resolve.go`
- Modify: `cmd/gromit/refine.go`
- Modify: `cmd/gromit/refine_test.go`

**What to Do:**
Add `resolveEpicsDir(cfg *config.Config) string` to `resolve.go`, following the exact pattern of `resolveSpecsDir` but returning `.gromit/epics` as default.

Add `detectEpicFromContext(context string, epicsDir string) (epicID string, epicPath string, epicTitle string)` to `refine.go`. This function:
1. Lists `.md` files in `epicsDir`
2. For each, parses frontmatter and extracts `epic_id` (string)
3. Checks if `epic_id` appears as a substring in `context`
4. Tracks the longest matching `epic_id`
5. Returns the winning epic's ID, file path, and title (from first `# ` heading in body)
6. Returns empty strings if no match or epicsDir doesn't exist

Add unit tests covering: exact match, substring match, truncated context, no match, empty context, empty epics dir, multiple epics with longest-match-wins, epic with missing/invalid frontmatter skipped, epic with non-string epic_id skipped.

**Acceptance Criteria:**
- `resolveEpicsDir` returns config value when set, `.gromit/epics` when not
- `detectEpicFromContext` returns the longest-matching epic ID from the context string
- When no epic matches or epics directory is missing, returns empty strings

**Dependencies:** None

### Task 2: Add extractAcceptanceCriteria and buildEpicContextSection with tests

**Files:**
- Modify: `cmd/gromit/refine.go`
- Modify: `cmd/gromit/refine_test.go`

**What to Do:**
Add `extractAcceptanceCriteria(body string) []string` to `refine.go`. This function:
1. Scans lines for `## Acceptance Criteria` heading
2. Collects subsequent lines starting with `- ` (trimmed)
3. Stops at the next `##` heading or end of content
4. Returns the collected criteria as a string slice

Add `buildEpicContextSection(epicID, epicTitle, epicContent string, siblingSpecs []spec, specsDir string) string` to `refine.go`. This function:
1. Starts with `## Epic Context` header
2. Adds "This idea is part of the **<title>** epic (`<id>`)."
3. Adds "Include `epic: <id>` in the spec frontmatter."
4. Adds `### Epic Document` with full epic content
5. Adds `### Sibling Specs` section:
   - If no siblings: "No other specs have been created for this epic yet."
   - If siblings: for each, reads spec file from specsDir, extracts body via `frontmatter.Parse`, calls `extractAcceptanceCriteria`, formats as `**<title>** (<id>)` with criteria bullets
6. Returns the assembled section string

Add unit tests for `extractAcceptanceCriteria`: standard list, stops at next heading, no section, empty body, criteria with varied formatting.

Add unit tests for `buildEpicContextSection`: with sibling specs (verifies title, id, criteria, epic instruction), without siblings (verifies "No other specs" message), verifies epic document content is included.

**Acceptance Criteria:**
- `extractAcceptanceCriteria` returns bullet items from the `## Acceptance Criteria` section
- `buildEpicContextSection` produces the full `## Epic Context` prompt section per spec format
- Sibling specs show title and acceptance criteria only

**Dependencies:** Task 1 (uses `spec` type from `epic.go`)

### Task 3: Wire epic context into refine prompt assembly

**Files:**
- Modify: `cmd/gromit/refine.go`

**What to Do:**
Modify `runRefine()` to detect and inject epic context into the system prompt:

1. After determining `ideaText` and `backlogID` (around line 156), extract the raw context string. For backlog items, this is `idea.Context`. For ad-hoc text, context is empty (no epic detection). For blank sessions, context is empty.

2. After resolving `specsDir` and before building the system prompt, call `detectEpicFromContext(rawContext, epicsDir)` where `epicsDir = resolveEpicsDir(cfg)`.

3. If an epic is detected (non-empty epicID):
   - Read the epic file content
   - Call `findLinkedSpecs(epicID, specsDir, cfg)` to get sibling specs
   - Call `buildEpicContextSection(epicID, epicTitle, epicContent, siblings, specsDir)`
   - Insert the resulting section into the system prompt between the idea text and the `## Context` section

4. If no epic is detected, the prompt is unchanged from current behavior.

Track `rawContext` as a separate variable from `ideaText` so the detection has the clean context string rather than the full "Text\n\nContext: ..." concatenation.

**Acceptance Criteria:**
- When a backlog item's context contains a known epic ID, the refine prompt includes the `## Epic Context` section
- When no epic ID matches, the refine prompt is identical to current behavior
- Ad-hoc ideas and blank sessions do not trigger epic detection

**Dependencies:** Task 1, Task 2

---

## Notes

- The `spec` struct and `findLinkedSpecs` function in `epic.go` are package-level (unexported but accessible within `package main`), so they can be called directly from `refine.go`.
- The `config.Config.Paths` struct does not have an `Epics` field. The `resolveEpicsDir` helper uses `GromitDir` + "epics" as default, matching how `epic.go` currently resolves the path.
- Epic detection only runs when there's a non-empty context string, so ad-hoc ideas passed as plain text won't trigger filesystem scanning.
