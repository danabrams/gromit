---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T03:41:56-05:00"
id: refine-blank-idea
source_spec: refine-blank-idea
---

# Blank Idea Refinement from the Picker — Implementation Plan

**Goal:** Add a "Something new" option to the refine picker and handle empty backlogs by launching a blank Claude session, with auto-created backlog items for traceability.

**Architecture:** Modify the existing picker logic in `refine.go` to support blank sessions, add a spec-title extraction helper, and auto-create backlog items post-session. Extend the E2E fake claude to support file creation side effects.

**Tech Stack:** Go, bash (fake CLI), cobra CLI framework

**Spec:** `.gromit/specs/refine-blank-idea.md`

---

## Architecture

**Overview:**
Modify the interactive picker in `refine.go` to add a "Something new" option and handle empty backlogs, then add post-session backlog item creation for blank sessions with a small spec-title extraction helper.

**Key Components:**

1. **Modified picker logic** (`cmd/gromit/refine.go`): When unrefined items exist, append a "Something new..." option as the last entry. When the backlog is empty, skip the picker and go directly to a blank session.

2. **Blank session system prompt** (`cmd/gromit/refine.go`): An alternate prompt path that omits "Idea to refine" and instead tells Claude no idea was provided. The refine skill already handles the rest ("If refining an ad-hoc idea, ask for the initial description").

3. **Spec title extraction** (`cmd/gromit/refine.go`): A new `extractSpecTitle(path string) string` function that reads a spec file and finds the first `# ` heading. Kept in refine.go since it's only used there.

4. **Auto-create backlog item** (`cmd/gromit/refine.go`): After a blank session creates specs, create a backlog item per spec with ID from `GenerateID()`, text from spec title, type "feature", status "refined", and spec_name linked.

**Integration Points:**
- Modifies the existing picker block (lines 63-116) — no new entry points
- Adds a new `isBlankSession` bool to track whether post-session auto-backlog-creation is needed
- Reuses existing `backlog.File.Add()` for creating the backlog item
- `chainAfterRefine` works unchanged — it receives spec names regardless of how they were created

**Data Flow:**
1. User runs `gromit refine` → picker shows items + "Something new", or skips picker if empty
2. User selects "Something new" (or empty backlog auto-triggers) → `ideaText` stays empty, `isBlankSession = true`
3. System prompt constructed without "Idea to refine" section → Claude session launched
4. Claude asks user what to refine, conversation produces spec file
5. Post-session: new specs detected → for blank session, extract title from each spec → create backlog item → chain as normal

**Files to Modify:**
- `cmd/gromit/refine.go` — Picker changes, blank session prompt, spec title extraction, auto-backlog creation
- `test/fakes/claude` — Add `CLAUDE_WRITE_FILE`/`CLAUDE_WRITE_CONTENT` side effect support

**Files to Create:**
- `cmd/gromit/refine_test.go` — Unit tests for `extractSpecTitle` and `formatTypeLabel`
- `test/e2e/refine_e2e_test.go` — E2E tests for the three refine picker paths
- `test/fixtures/refine_spec.md` — Minimal spec fixture for E2E tests

**Tradeoffs:**
- **Helper in refine.go vs internal package**: `extractSpecTitle` could live in an `internal/spec` package, but it's a small function only used here. Keeping it local avoids package proliferation. Can extract later if needed.
- **One bool (`isBlankSession`) vs restructuring**: Rather than refactoring the input mode logic into an enum, a single boolean flag cleanly tracks whether post-session auto-creation is needed. Minimal change footprint.

## Test Strategy

**Test Levels:**

1. **Unit Tests** (`cmd/gromit/refine_test.go`): Test `extractSpecTitle` with various spec file formats (frontmatter, no heading, empty, missing file). Test `formatTypeLabel` for known and custom types.

2. **E2E Tests** (`test/e2e/refine_e2e_test.go`): Full integration tests using the existing E2E infrastructure with fake CLIs. Tests all three picker paths: empty backlog, "Something new" selection, existing item selection.

3. **Manual Testing**: Run `gromit refine` end-to-end with real Claude.

**Key Test Cases:**

Unit:
- `extractSpecTitle` with standard spec file (`# Title` after frontmatter) → returns title
- `extractSpecTitle` with no `#` heading → returns empty string
- `extractSpecTitle` with `##` subheading before `#` heading → returns only `#` heading
- `extractSpecTitle` with empty file → returns empty string
- `extractSpecTitle` with non-existent file → returns empty string
- `formatTypeLabel` for known types (feature, bug, chore, unknown) → correct labels
- `formatTypeLabel` for custom/unknown type → formatted bracket label

E2E:
- Empty backlog → blank session launches, spec created, backlog item auto-created
- Non-empty backlog → "Something new" selected, blank session, backlog item auto-created
- Non-empty backlog → existing item selected, existing behavior preserved

**Infrastructure Changes:**
- Extend fake claude with `CLAUDE_WRITE_FILE`/`CLAUDE_WRITE_CONTENT` env vars for spec file creation side effects
- Add `runGromitWithStdin` helper (or extend `runGromit`) for interactive command testing
- Add `test/fixtures/refine_spec.md` — minimal valid spec fixture

**Coverage Goals:**
- 100% of `extractSpecTitle` branches
- E2E coverage of all three refine picker paths
- Verify auto-backlog-creation only happens for blank sessions

## Implementation Tasks

### Task 1: Add `extractSpecTitle` helper and unit tests

**Files:**
- Create: `cmd/gromit/refine_test.go`
- Modify: `cmd/gromit/refine.go`

**What to Do:**
Add an `extractSpecTitle(path string) string` function to `refine.go` that reads a spec file and returns the text of the first `# ` heading (level-1 markdown heading). Must handle frontmatter blocks (`---`), files with no heading, empty files, and non-existent files. Also add unit tests for this function and for the existing `formatTypeLabel` function.

**Acceptance Criteria:**
- `extractSpecTitle` returns the heading text for a valid spec with `# Title` after frontmatter
- `extractSpecTitle` returns empty string for missing file, empty file, or no `#` heading
- `formatTypeLabel` tests cover all known types and a custom type

**Dependencies:** None

### Task 2: Modify picker to add "Something new" and handle empty backlog

**Files:**
- Modify: `cmd/gromit/refine.go` (lines 63-116)

**What to Do:**
Change the interactive picker block:
- When `len(unrefined) == 0`: instead of printing help and returning, print "No unrefined backlog items. Starting a blank refinement session..." and fall through to launch a blank Claude session. Set `isBlankSession = true`, leave `ideaText` empty.
- When `len(unrefined) > 0`: after displaying existing items, append a final `[new] Something new...` option. If the user selects that option (number == len(unrefined)+1), set `isBlankSession = true`, leave `ideaText` empty. Otherwise, existing selection behavior.

Add `var isBlankSession bool` alongside the existing `ideaText`, `backlogID`, `fromBacklog` vars.

**Acceptance Criteria:**
- Empty backlog skips picker and launches blank session
- Non-empty backlog shows "Something new..." as last option
- Selecting "Something new" sets blank session mode with no idea text

**Dependencies:** None

### Task 3: Build blank session system prompt

**Files:**
- Modify: `cmd/gromit/refine.go` (lines 151-162)

**What to Do:**
When `isBlankSession` is true (or `ideaText` is empty), construct the system prompt without the "Idea to refine" section. Instead, include a directive like: "No idea was provided. Ask the user what they'd like to refine." The refine skill already handles this case. When `ideaText` is non-empty, keep the existing prompt construction.

**Acceptance Criteria:**
- Blank session prompt omits "Idea to refine" section
- Blank session prompt includes directive for Claude to ask the user
- Non-blank sessions use existing prompt format unchanged

**Dependencies:** Task 2 (needs `isBlankSession` flag)

### Task 4: Auto-create backlog item after blank session

**Files:**
- Modify: `cmd/gromit/refine.go` (post-session logic, lines 205-238)

**What to Do:**
After detecting new spec files, if `isBlankSession` is true and specs were created, create a backlog item for each new spec:
- ID: `backlog.GenerateID()`
- Text: `extractSpecTitle(specPath)` (fall back to spec filename if title is empty)
- Type: `"feature"`
- Status: `"refined"`
- SpecName: spec filename without `.md`
- CreatedAt: `time.Now()`

Use `bf.Add()` to persist. Print confirmation. Then proceed to `chainAfterRefine` as before.

**Acceptance Criteria:**
- Blank session that creates a spec produces a backlog item with correct fields
- Backlog item text comes from spec `# Title` heading
- Existing backlog-item-from-picker flow is unchanged

**Dependencies:** Task 1 (`extractSpecTitle`), Task 2 (`isBlankSession`), Task 3

### Task 5: Extend fake claude and add E2E tests

**Files:**
- Modify: `test/fakes/claude` (add `CLAUDE_WRITE_FILE` / `CLAUDE_WRITE_CONTENT` support)
- Create: `test/fixtures/refine_spec.md` (minimal spec fixture)
- Create: `test/e2e/refine_e2e_test.go`

**What to Do:**
1. Extend the fake claude script: if `CLAUDE_WRITE_FILE` and `CLAUDE_WRITE_CONTENT` are set, create parent dirs and write the content to the file path before outputting the fixture and exiting.
2. Create a minimal spec fixture with frontmatter and a `# Blank Idea Title` heading.
3. Add a `runGromitWithStdin` helper that accepts an `stdin string` parameter and pipes it to the command.
4. Add E2E tests:
   - **TestE2E_RefineEmptyBacklog**: no unrefined items, verify stdout contains "No unrefined backlog items", verify claude was invoked (call log), verify backlog item created with spec title and status "refined".
   - **TestE2E_RefineSomethingNew**: add unrefined items, pipe stdin to select last option (the "Something new" choice), verify claude invoked without "Idea to refine" in system prompt, verify backlog item created.
   - **TestE2E_RefineExistingItem**: add unrefined items, pipe stdin to select first item, verify claude invoked with idea text in system prompt, verify existing backlog item updated (not new one created).

**Acceptance Criteria:**
- Fake claude creates spec file when `CLAUDE_WRITE_FILE`/`CLAUDE_WRITE_CONTENT` are set
- E2E test for empty backlog passes
- E2E test for "Something new" selection passes
- E2E test for existing item selection passes

**Dependencies:** Tasks 1-4 (implementation must be complete)

---

## Notes

- The refine command uses `--append-system-prompt` (plain/interactive mode), not `stream-json`. The fake claude's plain mode path (`cat "$fixture_file"`) applies.
- The picker reads from stdin via `bufio.NewReader(os.Stdin)`, then Claude inherits the same stdin. For E2E tests, the stdin string must contain the picker choice followed by a newline, then anything else gets consumed by the fake claude's `cat > /dev/null`.
- The `isBlankSession` flag is intentionally separate from `fromBacklog` — a blank session is neither "from backlog" (no pre-existing item) nor "ad-hoc text" (no text provided). It's a third mode that creates a backlog item *after* the session.
- The initial message sent to Claude should also change for blank sessions. Instead of "Begin refining this idea into a structured spec following the instructions above." it should be something like "Begin a blank refinement session following the instructions above." to avoid implying an idea was provided.
