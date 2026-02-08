---
id: epic-scoped-execution
source_ideas: []
created: 2026-02-08
---

# Epic-Scoped Execution

## Problem

Gromit treats all beads as a flat queue. When multiple specs are in flight, `gromit run` interleaves their beads by priority. There is no way to focus on completing one specification's work, review it, retro on it, and then move to the next. There is also no concept of an "epic" — a higher-level exploration that spawns multiple specs over time.

Users want to:
- Run, review, and retro on a single spec's beads without touching others
- Group related specs under an epic (a problem space / exploration)
- Run, review, and retro on an entire epic's beads
- Track what parts of an epic have been specified and what hasn't
- Explore a problem space interactively before committing to any artifacts

## Solution

### Hierarchy

```
Epic (problem space, deliberately fuzzy)
  ├── Spec A (concrete feature, code-ready)
  │    └── Plan A → Beads
  ├── Spec B (another feature)
  │    └── Plan B → Beads
  └── (more specs discovered over time)
```

Epics are explorations. Specs are implementations. Beads are tasks. The linkage is loose — resolved at query time, not enforced at creation.

### Epic Documents

Epic documents live in `.gromit/epics/`. They are free-form markdown capturing the problem, research, possible solutions, and scope. Unlike specs, they are deliberately unstructured — they describe a problem space, not an implementation.

Each epic document has frontmatter with the bd bead ID:

```yaml
---
epic_id: gromit-xyz
created: 2026-02-08
---
```

### Spec-to-Epic Linkage

Specs reference their parent epic via an optional `epic:` field in frontmatter:

```yaml
---
id: init-wizard
epic: gromit-xyz
created: 2026-02-08
---
```

Beads do NOT carry an `epic:` label. The resolution chain is: epic → find specs with that epic in frontmatter → find beads with those `spec:` labels. The spec is the single source of truth for which epic it belongs to.

### `gromit explore` Command

Top-level command for exploring a problem space interactively:

```
gromit explore "Improve developer onboarding"
gromit explore                                   # Describe interactively
```

Flow:
1. Launches an interactive Claude session framed for exploration — understand the problem, research approaches, identify scope, weigh tradeoffs
2. Claude has full project context (CLAUDE.md, RULES.md, LEARNINGS.md)
3. The user converses freely. May decide the idea isn't worth pursuing — just exit, nothing is created
4. When the exploration concludes, Claude asks what to do with the findings: create an epic, create a spec directly, add to backlog, or discard
5. Claude writes the appropriate artifact (epic doc to `.gromit/epics/`, spec to `.gromit/specs/`, or backlog item)
6. After the session exits, the CLI detects new files and creates the corresponding bd bead (epic type for epic docs, or follows existing patterns for specs/backlog)

No bead or file is created until the exploration concludes successfully. The user decides the outcome inside the conversation, not via CLI prompts.

### `--epic` and `--spec` Flags

Three commands gain `--epic` and `--spec` flags. They are mutually exclusive.

**`gromit run`**

```
gromit run --epic gromit-xyz          # Only beads under this epic's specs
gromit run --spec init-wizard         # Only beads with spec:init-wizard label
gromit run --spec init-wizard -n 5    # Existing flags (iteration/time limits) still work
```

Without `--epic` or `--spec`, `gromit run` behaves exactly as today — pure priority ordering across all beads. Scoping is opt-in only.

**`gromit review`**

```
gromit review --epic gromit-xyz       # Already exists, keeps working
gromit review --spec init-wizard      # Scopes review to one spec's commits
```

**`gromit retro`**

```
gromit retro --epic gromit-xyz        # Retro on epic's beads only
gromit retro --spec init-wizard       # Retro on one spec's beads only
```

### Resolution Logic

A shared resolver produces a set of bead IDs from either flag:

- `--spec init-wizard` → call `bd ready --label spec:init-wizard` (for run) or `bd list --label spec:init-wizard` (for review/retro) → bead ID set
- `--epic gromit-xyz` → scan `.gromit/specs/*.md` frontmatter for `epic: gromit-xyz` → collect spec names → for each, query beads by `spec:` label → union of all bead IDs

For `gromit run`, use `bd ready --label spec:<name>` to let bd handle filtering natively. For review/retro, filter iteration log entries by bead ID membership in the resolved set.

### `gromit epic status` Command

Shows epic coverage and progress:

```
gromit epic status gromit-xyz
```

Output includes:
- Epic title and status (open / fully-specified / complete)
- Each linked spec with its pipeline stage (unplanned / planned / decomposed / done) and bead progress (open/closed counts)
- LLM-assisted coverage gap analysis: feeds Claude the epic document plus the list of linked specs and asks what areas of the epic aren't covered by specs yet

### Epic Lifecycle

1. **Open** — actively being explored, may spawn new specs
2. **Fully specified** — user has decided all needed specs exist. Marked manually: `bd update gromit-xyz --label fully-specified`
3. **Complete** — all linked specs' beads are closed. User closes manually: `bd close gromit-xyz`

No automation for transitions — epics are a human-judgment boundary. Gromit provides visibility; the user makes the call.

### No Changes to Decompose

`gromit decompose` does not change. It continues to create beads with `spec:<name>` labels and dependency chains. The epic linkage lives on the spec, not on beads, so decompose doesn't need to know about epics.

## Acceptance Criteria

- `gromit explore` launches an interactive Claude session for problem-space exploration with full project context
- `gromit explore` creates no artifacts until the exploration concludes; outcome is determined inside the conversation
- Epic documents are written to `.gromit/epics/` with frontmatter containing the bd bead ID
- Spec frontmatter supports an optional `epic:` field linking to an epic bead ID
- `gromit run --spec <name>` only processes beads with the matching `spec:` label
- `gromit run --epic <id>` resolves to all specs for that epic and only processes their beads
- `gromit review --spec <name>` scopes review to commits related to that spec's beads
- `gromit retro --spec <name>` and `gromit retro --epic <id>` filter iteration logs to matching beads
- `gromit epic status <id>` displays linked specs, their pipeline stages, bead progress, and LLM-identified coverage gaps
- `--epic` and `--spec` are mutually exclusive on all commands
- `gromit run` without either flag behaves identically to current behavior

## Decisions

1. **Epics are explorations, specs are implementations.** Epics capture problem spaces and research. They are deliberately fuzzy and may spawn zero or more specs. Specs are code-ready and go through the plan → decompose → run pipeline. This separation prevents half-baked ideas from entering the implementation pipeline.

2. **Linkage lives on the spec, not on beads.** The `epic:` field in spec frontmatter is the single source of truth. Beads inherit their spec association via `spec:` labels (already exists). Adding `epic:` labels to beads would create redundant state to keep in sync with no practical benefit.

3. **Resolution at query time, not creation time.** The epic → specs → beads chain is resolved when you run a command with `--epic`, not when beads are created. This keeps decompose unchanged and allows specs to be reassigned between epics by editing one line of frontmatter.

4. **No automated epic lifecycle transitions.** Deciding when an epic is "fully specified" requires understanding whether the problem space is covered — a human judgment. Gromit provides the `epic status` view (including LLM gap analysis) to inform that judgment but doesn't automate it.

5. **Exploration creates artifacts only on success.** `gromit explore` doesn't create a bead or file until the user decides to proceed. This encourages low-cost exploration without cluttering the tracker with abandoned ideas.

6. **LLM-assisted coverage analysis over structured checklists.** Epic documents are free-form, so structured cross-referencing is impractical. Feeding the epic doc and linked specs to Claude for gap analysis matches the exploratory nature of epics.

7. **Scoped execution is opt-in.** `gromit run` without flags runs everything by priority, same as today. Batching by epic/spec only happens when explicitly requested.

## Research & Context

### Current State

- Beads already carry `spec:<name>` labels from decompose
- `bd ready` supports `--label` and `--parent` flags for filtering
- `gromit review --epic` already exists and resolves scope via parent-child bead relationships and git log searches
- Iteration logs capture `bead_id` on every entry, enabling log filtering by bead set
- `gromit debug` provides the implementation pattern for interactive sessions with post-session artifact detection

### Key Files

- `cmd/gromit/run.go` — Run command, needs `--epic`/`--spec` flags and filtered bead fetching
- `cmd/gromit/review.go` — Review command, already has `--epic`, needs `--spec`
- `cmd/gromit/retro.go` — Retro command, needs both flags and log filtering
- `internal/retro/retro.go` — Retro logic, needs to accept a bead ID filter set
- `internal/runner/runner.go` — Core loop, `Ready()` calls need label-based filtering
- `internal/bead/bead.go` — Bead client, may need `ReadyWithLabel()` or similar
- `cmd/gromit/decompose.go` — No changes needed
- `cmd/gromit/debug.go` — Pattern for interactive session + post-session detection
- `internal/frontmatter/` — Frontmatter reading for spec `epic:` field
