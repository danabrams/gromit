---
id: ralph-reference-cleanup
source_ideas: [idea-1770402840071]
created: 2026-02-06
superseded_by: gromit-prefix-migration
superseded_at: 2026-02-07
---

# Ralph Reference Cleanup

> **⚠️ SUPERSEDED:** This spec has been superseded by `gromit-prefix-migration`. Decision #2 in this spec was made without knowledge of `bd rename-prefix`. The prefix migration was completed using `bd rename-prefix --repair`, which safely migrated all beads from `ralph-runner-*` and `ralph-*` prefixes to `gromit-*`.

## Specification

Remove stale references to the old "ralph" / "ralph-runner" naming from documentation files. The codebase rename to "gromit" is complete in all source code, but three documentation files still contain old paths and names.

Files to update:

| File | Line | Current | Replace with |
|------|------|---------|-------------|
| `demo/RECORDING.md` | 12 | `cd /path/to/ralph-runner && go build -o ralph ./cmd/gromit` | `cd /path/to/gromit && go build -o gromit ./cmd/gromit` |
| `demo_plan.md` | 9 | `Three files in /home/danabrams/ralph-runner/demo/` | `Three files in the `demo/` directory:` |
| `demo_plan.md` | 160 | `Create demo/ directory in the ralph-runner repo` | `Create `demo/` directory in the gromit repo` |

No changes to `.beads/issues.jsonl` — the `ralph-runner-*` bead ID prefix is historical bd data and is correct as-is.

## Acceptance Criteria

- No case-insensitive matches for "ralph" remain in `demo/RECORDING.md` or `demo_plan.md`
- The build command in `RECORDING.md` produces a binary named `gromit`, not `ralph`

## Decisions

1. **Clean break, no migration tooling** There are no external users and no existing projects using the old `.ralph/` naming. Migration support is unnecessary.

2. **~~Leave `.beads/issues.jsonl` alone~~ SUPERSEDED** ~~Bead IDs with the `ralph-runner-*` prefix are historical records managed by bd. Changing them would break bd's data integrity.~~ This decision was made without knowledge of `bd rename-prefix`. The prefix migration was subsequently completed in the `gromit-prefix-migration` spec using `bd rename-prefix --repair`, which safely migrated all 400 beads from `ralph-runner-*` and `ralph-*` prefixes to `gromit-*`.

## Research & Context

### Current State

The rename from "ralph" to "gromit" was completed in commit `771aab3` ("chore: update specs and templates for gromit rename") and `74a28b9` ("docs: rewrite CLAUDE.md and update design docs for gromit rename"). All Go source, config structs, defaults, templates, and primary docs use "gromit" naming. Only demo/planning docs retain stale references.
