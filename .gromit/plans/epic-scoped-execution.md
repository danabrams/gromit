---
id: epic-scoped-execution
source_spec: epic-scoped-execution
created: 2026-02-08
decomposed: false
---

# Epic-Scoped Execution Implementation Plan

**Goal:** Enable focused execution, review, and retro on a single spec's or epic's beads, plus interactive problem-space exploration via `gromit explore`.

**Architecture:** A shared scope resolver package translates `--epic`/`--spec` flags into bead label sets, threaded through run, review, and retro. `gromit explore` follows the debug command's interactive session pattern with post-session artifact detection. `gromit epic status` provides visibility into epic progress with LLM gap analysis.

**Tech Stack:** Go, Cobra CLI, bd CLI, Claude Code CLI, YAML frontmatter

**Spec:** `.gromit/specs/epic-scoped-execution.md`

---

## Architecture

**Key Components:**

1. **`internal/scope/`** — Shared scope resolver. Translates `--epic <id>` or `--spec <name>` into spec label strings for bead querying. For `--spec`: returns `["spec:<name>"]`. For `--epic`: scans `.gromit/specs/*.md` frontmatter for matching `epic:` field, collects spec names, returns `["spec:<name1>", "spec:<name2>", ...]`.

2. **`internal/bead/bead.go`** — Extended with `ReadyWithLabel(label)` and `ListWithLabel(label)` methods that pass `--label` to `bd` natively.

3. **`internal/runner/`** — Runner accepts an optional label filter. When set, calls `ReadyWithLabel()` instead of `Ready()`. Iterates through multiple labels for epic scope (union of specs).

4. **`cmd/gromit/run.go`** — `--epic`/`--spec` flags with mutual exclusivity. Resolves scope and passes label filter to runner.

5. **`cmd/gromit/review.go`** — `--spec` flag added. Existing `--epic` updated to use frontmatter-based resolver (replacing parent-child resolution).

6. **`cmd/gromit/retro.go` + `internal/retro/retro.go`** — `--epic`/`--spec` flags. Bead ID filter set passed to retro logic to filter per-bead stats and log entries.

7. **`cmd/gromit/explore.go`** — Interactive explore command following debug pattern: pre-session state snapshot, Claude session with exploration prompt, post-session artifact detection (epics, specs, backlog), bd bead creation.

8. **`cmd/gromit/epic.go`** — `gromit epic status <id>` subcommand. Reads epic doc, finds linked specs, shows pipeline stages and bead progress, runs LLM gap analysis.

**Data Flow (scoped run):**
```
gromit run --epic gromit-xyz
  → scope.ResolveEpic("gromit-xyz", specsDir)
    → scan .gromit/specs/*.md for epic: gromit-xyz
    → collect spec names → return labels ["spec:init-wizard", "spec:onboarding"]
  → for each label, runner calls beads.ReadyWithLabel(label)
  → process returned beads in priority order
```

**Data Flow (explore):**
```
gromit explore "Improve onboarding"
  → snapshot existing epics, specs, backlog items
  → build exploration prompt with project context
  → launch interactive Claude session
  → user converses, decides outcome
  → Claude writes artifact (.gromit/epics/*.md or .gromit/specs/*.md)
  → session exits
  → CLI detects new files, creates bd beads as needed
```

## Test Strategy

**Unit Tests:**
- Scope resolver: `--spec` returns correct label, `--epic` scans frontmatter and returns union of labels, empty results for unlinked epics, mutual exclusivity validation
- Bead client: `ReadyWithLabel` passes `--label` flag correctly, `ListWithLabel` passes `--label` flag correctly, epic-type exclusion preserved
- Runner: no-filter uses `Ready()`, with-filter uses `ReadyWithLabel()`, empty results exit cleanly
- Retro: bead ID filtering on per-bead stats, empty filter produces empty stats, no filter = unchanged behavior

**Mocking:**
- Mock `bd` CLI via existing `commandRunner` pattern in bead tests
- Temp directories with spec files for scope resolver tests
- Real frontmatter parsing (pure Go, no mocking needed)

**Test Files:**
- `internal/scope/scope_test.go`
- `internal/bead/bead_test.go` (extend)
- `internal/runner/runner_test.go` (extend)
- `internal/retro/retro_test.go` (extend)

## Implementation Tasks

### Task 1: Scope Resolver Package

**Files:**
- Create: `internal/scope/scope.go`
- Create: `internal/scope/scope_test.go`

**What to Do:**
Create the shared scope resolver package. It provides two public functions:

- `ResolveSpec(specName string) []string` — Returns `["spec:<specName>"]`. Simple but centralizes the label format.
- `ResolveEpic(epicID string, specsDir string) ([]string, error)` — Scans all `.md` files in `specsDir`, reads frontmatter via `internal/frontmatter`, collects files where `epic:` matches `epicID`, returns `["spec:<name1>", "spec:<name2>", ...]` using the `id` field from frontmatter.
- `ValidateFlags(epicFlag, specFlag string) error` — Returns an error if both are set.

Uses `internal/frontmatter.ReadFile()` for parsing. Returns an empty slice (not error) when no specs match an epic — the caller decides whether that's an error.

**Acceptance Criteria:**
- `ResolveEpic` correctly finds specs linked to an epic via frontmatter `epic:` field
- `ResolveEpic` ignores specs without `epic:` field or with different epic values
- `ValidateFlags` returns error when both epic and spec are set

**Dependencies:** None (foundational)

**Notes:** The frontmatter package already handles the YAML parsing. Spec `id` field in frontmatter is used as the spec name for label construction.

---

### Task 2: Bead Client Label-Based Methods

**Files:**
- Modify: `internal/bead/bead.go`
- Modify: `internal/bead/bead_test.go`

**What to Do:**
Add two new methods to `bead.Client`:

- `ReadyWithLabel(label string) (*Bead, error)` — Calls `bd ready --json --limit 10 --label <label>`, parses output excluding epic types (same as `Ready()`).
- `ListWithLabel(label string) ([]Bead, error)` — Calls `bd list --json --label <label>`, returns all matching beads.

Also add these methods to the `BeadClient` interface in `internal/runner/interfaces.go` and to any mock implementations used in tests.

**Acceptance Criteria:**
- `ReadyWithLabel("spec:foo")` passes `--label spec:foo` to `bd ready`
- `ListWithLabel("spec:foo")` passes `--label spec:foo` to `bd list`
- Both methods handle empty results gracefully

**Dependencies:** None (foundational)

---

### Task 3: Scoped Run Command

**Files:**
- Modify: `cmd/gromit/main.go` (run command flags and wiring)
- Modify: `internal/runner/runner.go` (accept label filter)
- Modify: `internal/runner/runner_test.go`

**What to Do:**
Add `--epic` and `--spec` string flags to the run command. In `runLoop()`:

1. Call `scope.ValidateFlags(epicFlag, specFlag)` — error if both set
2. If `--spec` set: resolve via `scope.ResolveSpec(specFlag)` → get label list
3. If `--epic` set: resolve via `scope.ResolveEpic(epicFlag, specsDir)` → get label list
4. Pass label list to the runner

Modify `runner.Runner` to accept an optional `[]string` of spec labels. In the main loop, if labels are set, iterate through them calling `ReadyWithLabel()` for each, collecting beads, and processing in priority order. If no labels set, call `Ready()` as today.

**Acceptance Criteria:**
- `gromit run --spec init-wizard` only processes beads with `spec:init-wizard` label
- `gromit run --epic gromit-xyz` resolves specs and processes only their beads
- `gromit run` without flags behaves identically to current behavior

**Dependencies:** Task 1, Task 2

---

### Task 4: Scoped Review Command

**Files:**
- Modify: `cmd/gromit/review.go`

**What to Do:**
Add `--spec` string flag. Update the existing `--epic` resolution to use the new scope resolver instead of parent-child bead relationships.

In `determineReviewScope()`:
1. Add mutual exclusivity check between `--epic`, `--spec`, and `--since`
2. For `--spec`: resolve to bead IDs via `scope.ResolveSpec()` + `bead.ListWithLabel()`, then find the earliest commit touching any of those beads (same git log grep pattern as current `--epic`)
3. For `--epic`: resolve via `scope.ResolveEpic()` → get spec labels → for each, `ListWithLabel()` → union bead IDs → find earliest commit

Replace the existing `getEpicBaseCommit()` implementation with the frontmatter-based resolver. The commit-finding logic (git log grep by bead ID) stays the same — only the set of bead IDs changes.

**Acceptance Criteria:**
- `gromit review --spec init-wizard` scopes review to commits for that spec's beads
- `gromit review --epic gromit-xyz` resolves via spec frontmatter (not parent-child)
- `--epic` and `--spec` are mutually exclusive with each other and with `--since`

**Dependencies:** Task 1, Task 2

---

### Task 5: Scoped Retro Command

**Files:**
- Modify: `cmd/gromit/main.go` (retro command flags)
- Modify: `internal/retro/retro.go` (accept bead ID filter)
- Modify: `internal/retro/retro_test.go`

**What to Do:**
Add `--epic` and `--spec` string flags to the retro command. Resolve scope to a set of bead IDs using the scope resolver + `bead.ListWithLabel()`.

Pass the bead ID set into the retro logic. In retro, filter:
- `ReadPerBeadStats()` results — only include stats for beads in the filter set
- `ReadAllLogs()` results — only include log entries where `BeadID` is in the filter set
- Run stats derived from filtered logs

When no filter is set, behavior is unchanged (all logs included).

**Acceptance Criteria:**
- `gromit retro --spec init-wizard` only includes that spec's bead stats in the retro
- `gromit retro --epic gromit-xyz` resolves to all epic's beads and filters accordingly
- Retro without flags behaves identically to current behavior

**Dependencies:** Task 1, Task 2

---

### Task 6: Explore Command

**Files:**
- Create: `cmd/gromit/explore.go`

**What to Do:**
Implement `gromit explore [topic]` following the debug command pattern:

1. **Pre-session snapshot**: Record existing files in `.gromit/epics/`, `.gromit/specs/`, and backlog items (via backlog file)
2. **Ensure `.gromit/epics/` exists**: Create directory if missing
3. **Build exploration prompt**: Include project context (CLAUDE.md, RULES.md, LEARNINGS.md), the topic argument, and instructions framing the session for exploration. The prompt should instruct Claude to:
   - Explore the problem space, research approaches, weigh tradeoffs
   - When the user is ready to conclude, ask what to do: create epic doc, create spec, add to backlog, or discard
   - Write the chosen artifact to the correct location with proper frontmatter
4. **Write prompt to temp file** (avoid ARG_MAX, same as debug)
5. **Launch interactive Claude session** via `exec.Command` (not `claude.Client`)
6. **Post-session detection**: Compare before/after snapshots for `.gromit/epics/` and `.gromit/specs/`. For new epic files, read frontmatter and create a bd bead with type=epic. For new spec files, report them. For new backlog items, report them.

Add `--model` flag (optional, same pattern as debug).

**Acceptance Criteria:**
- `gromit explore "topic"` launches an interactive Claude session with exploration framing
- No artifacts are created until the session concludes
- New epic files in `.gromit/epics/` are detected and bd beads created post-session

**Dependencies:** None (can be built independently, but uses frontmatter package)

**Notes:** Follow `cmd/gromit/debug.go` closely. The explore prompt is the main differentiator — it frames the conversation for exploration rather than debugging. The post-session detection is nearly identical to debug's `detectAndReportArtifacts()`, extended to handle epics.

---

### Task 7: Epic Status Command

**Files:**
- Create: `cmd/gromit/epic.go`

**What to Do:**
Implement `gromit epic status <id>` as a subcommand:

1. **Read epic document**: Find the epic file in `.gromit/epics/` by scanning frontmatter for matching `epic_id`. Read the epic title and content.
2. **Find linked specs**: Use `scope.ResolveEpic(epicID, specsDir)` to get spec names. For each spec, determine its pipeline stage:
   - Check if plan exists in `.gromit/plans/<spec>.md` → planned
   - Check if plan has `decomposed: true` → decomposed
   - Check bead progress via `bd list --label spec:<name>` → count open/closed
   - If all beads closed → done
   - Otherwise → unplanned
3. **Display progress**: Print epic title, status, and a table of specs with stages and bead counts.
4. **LLM gap analysis**: Feed the epic document content plus the list of linked spec names/summaries to Claude (non-interactive, via `claude.Client.Run()`). Ask what areas of the epic aren't covered by existing specs. Print the analysis.

Register as `epicCmd` with `statusCmd` as a subcommand: `gromit epic status <id>`.

**Acceptance Criteria:**
- `gromit epic status <id>` displays linked specs with pipeline stages and bead progress
- Gap analysis calls Claude with epic doc + spec list and prints the response
- Handles epic with no linked specs gracefully

**Dependencies:** Task 1 (uses scope resolver)

**Notes:** The LLM gap analysis is the most novel part. Keep the prompt simple — feed the epic doc and spec names, ask what's missing. Use haiku for cost efficiency since this is an informational query.

---

## Notes

- **Flag naming consistency**: All commands use `--epic` and `--spec` as string flags (not `--epic-id` or `--spec-name`). The values are the epic bead ID and spec name respectively.
- **Review `--epic` behavior change**: The existing parent-child resolution is being replaced with frontmatter-based resolution. This is a deliberate behavior change approved during planning.
- **Explore prompt quality**: The exploration prompt is critical to the `gromit explore` experience. It should encourage open-ended thinking while guiding toward actionable outcomes. Worth iterating on the prompt text during implementation.
- **Epic directory initialization**: `gromit init` should be updated to create `.gromit/epics/` alongside other directories, but this is a minor followup, not a task here.
