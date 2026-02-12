---
id: retro-archive-learnings
source_ideas: []
created: 2026-02-12
---

# Separate Archived Learnings from LEARNINGS.md

## Specification

When learnings are archived (during retro apply), they should be moved to a separate archive file (`.gromit/LEARNINGS_ARCHIVE.md`) instead of staying in the `## Archived` section of `LEARNINGS.md`. This eliminates parsing overhead on every load, since archived entries are never used in prompts or retro analysis — only confirmed and provisional learnings are.

To preserve dedup safety (preventing re-addition of previously discarded patterns), archived learning hashes are stored in `state.json` under a new `archived_hashes` field. When adding a new learning, the dedup check consults confirmed + provisional (already loaded) plus the archived hash set (already in memory from state load). The full archive file is never loaded by the running system.

### Archive File Format

`.gromit/LEARNINGS_ARCHIVE.md` uses the same format as current archived entries in `LEARNINGS.md`: learning headers with content and archive reason annotations. Newest entries are appended at the bottom. The file is append-only and purely for human reference.

### Changed Behaviors

1. **`Archive()` method**: Instead of moving a learning to the `## Archived` section of `LEARNINGS.md`, it appends the learning to `.gromit/LEARNINGS_ARCHIVE.md`, removes it from `LEARNINGS.md`, and adds its hash to `archived_hashes` in state.json.

2. **`Add()` dedup check**: When checking if a new learning is a duplicate, check its hash against confirmed learnings, provisional learnings, and the `archived_hashes` set from state.json. No longer parses the archive file.

3. **`Save()` method**: No longer writes an `## Archived` section to `LEARNINGS.md`. The file contains only `## Confirmed` and `## Provisional` sections.

4. **`Load()` / parsing**: No longer parses an `## Archived` section. If one is found during load, it triggers one-time migration.

### One-Time Migration

The first time the updated code loads a `LEARNINGS.md` that contains a `## Archived` section:
- All archived entries are extracted and appended to `.gromit/LEARNINGS_ARCHIVE.md` (creating it if needed)
- Their hashes are added to `archived_hashes` in state.json
- The `## Archived` section is removed from `LEARNINGS.md`
- `LEARNINGS.md` is re-saved with only confirmed and provisional sections

This migration is idempotent — if the archive file already exists, entries are appended without duplication (checked by hash).

## Acceptance Criteria

- Archiving a learning writes it to `.gromit/LEARNINGS_ARCHIVE.md` and removes it from `LEARNINGS.md` (no `## Archived` section remains)
- Adding a learning that was previously archived (same hash) is correctly rejected as a duplicate via the `archived_hashes` set in state.json
- Loading a `LEARNINGS.md` with an existing `## Archived` section triggers one-time migration: entries move to archive file, hashes populate state.json, and the section is removed
- After migration, `LEARNINGS.md` contains only `## Confirmed` and `## Provisional` sections

## Decisions

1. **Hash-only index for dedup** Rather than loading and parsing the archive file for dedup checks, we store archived hashes in `state.json`. This gives O(1) lookup with zero archive file I/O. The archive file becomes purely a human-readable record.

2. **Migration during load, not a separate command** The one-time migration runs automatically when archived entries are detected in `LEARNINGS.md`. No manual step or separate CLI command needed — the system self-heals on first run.

3. **Archival happens during retro apply** The `Archive()` method (called during the retro interactive session when applying proposals) is the single code path that moves learnings to the archive file. No separate background process or retro-start cleanup.

## Research & Context

### Current State

- `internal/learnings/learnings.go`: Core parsing/saving logic. `parseLearnings()` splits by section headers and returns three slices (confirmed, provisional, archived). `Save()` writes all three sections back. `Archive()` moves between sections in memory. `Add()` checks dedup against all three.
- `internal/retro/retro.go`: Retro orchestration. `formatLearnings()` only includes confirmed + provisional in the retro prompt. Archived entries are loaded but unused.
- `internal/state/state.go`: Persistent state in `.gromit/state.json`. Already has `filtered_hashes` field — `archived_hashes` follows the same pattern.
- `.gromit/LEARNINGS.md`: Currently has ~8 confirmed, ~3 provisional, and hundreds of archived entries. The archived section is the bulk of the file.

### Files to Modify

- `internal/learnings/learnings.go` — Main changes: `Archive()`, `Add()`, `Save()`, `Load()`, migration logic
- `internal/state/state.go` — Add `archived_hashes` field to state struct
- `internal/retro/retro.go` — May need minor adjustments to pass state for archived hash lookups
