---
id: gromit-prefix-migration
source_ideas: []
created: 2026-02-07
---

# Gromit Prefix Migration

## Specification

Rename the bd issue prefix from `ralph-runner-` / `ralph-` to `gromit-` and update all files that reference bead IDs with the old prefixes.

The project was renamed from "ralph-runner" to "gromit" but the bd issue prefix was never updated. bd auto-detected the prefix from the directory name at init time, and the database now contains 395 issues across two stale prefixes (`ralph-runner-`: 310, `ralph-`: 85). The `bd rename-prefix` command exists specifically for this operation and handles ID remapping safely.

### Steps

1. **Run `bd rename-prefix gromit- --repair`** to consolidate all 395 issues under the `gromit-` prefix. This updates IDs and all text references within bd's database.

2. **Update `.gromit/LEARNINGS.md`** — bead IDs in learning entry headers (e.g., `ralph-runner-4a3f` → new gromit ID) need to match the renamed IDs.

3. **Update `.gromit/status.json`** — the `bead_id` field references a `ralph-runner-*` ID that will have been renamed.

4. **Update `internal/runner/format_test.go`** — test fixtures use hardcoded `ralph-runner-abc123` and `ralph-runner-ja5m` bead IDs. These are arbitrary test data and should be changed to use `gromit-` prefixed IDs for consistency.

5. **Update the existing `ralph-reference-cleanup` spec** — its Decision #2 ("Leave `.beads/issues.jsonl` alone") is now superseded. Either update or delete the spec.

### ID Mapping

`bd rename-prefix` generates new short IDs (e.g., `ralph-runner-9u1c` → `gromit-xe0`). The exact mapping is determined at rename time. Secondary file updates must use the actual mapping output, not guessed IDs.

## Acceptance Criteria

- `bd list --json` shows all issues with `gromit-` prefix, no `ralph-runner-` or `ralph-` prefixes remain
- `grep -ri ralph .gromit/LEARNINGS.md .gromit/status.json` returns no matches
- `grep -ri ralph internal/runner/format_test.go` returns no matches
- All existing tests pass after the rename (`go test ./...`)
- `.beads/issues.jsonl` contains only `gromit-` prefixed IDs

## Decisions

1. **Use `bd rename-prefix --repair`** The `--repair` flag is needed because there are two different prefixes (`ralph-runner-` and `ralph-`). Normal rename requires a single source prefix; `--repair` consolidates multiple prefixes into one.

2. **Update secondary files** Unlike the earlier `ralph-reference-cleanup` spec which left bead IDs alone, this spec explicitly updates LEARNINGS.md and status.json to match the new IDs. Since the IDs are changing in the database, dangling references would be confusing and broken.

3. **Test fixtures use arbitrary IDs** The bead IDs in `format_test.go` are test data, not real bead references. They can be freely changed to `gromit-*` for consistency without needing to match any real bead.

4. **Supersedes Decision #2 in `ralph-reference-cleanup`** That spec's decision to leave bead IDs alone was made without knowledge of `bd rename-prefix`. Now that a safe rename tool exists, there's no reason to keep stale prefixes.

## Research & Context

### Current State

The rename from "ralph" to "gromit" was completed in source code (commits `771aab3` and `74a28b9`), but the bd database prefix was never migrated. bd stores the prefix in its database, auto-detected from the directory name at `bd init` time.

### bd rename-prefix behavior

- `bd rename-prefix <new-prefix>` renames all issues from the current prefix to the new one
- `--repair` flag handles databases with multiple prefixes by consolidating them
- `--dry-run` flag previews changes without applying
- The command updates IDs and all text references across all fields in the database
- New IDs are generated with short alphanumeric suffixes (e.g., `gromit-xe0`)

### Files containing ralph references

| File | Type of reference |
|------|------------------|
| `.beads/issues.jsonl` | All 395 bead IDs (primary — fixed by bd rename-prefix) |
| `.beads/beads.db` | SQLite database (primary — fixed by bd rename-prefix) |
| `.gromit/LEARNINGS.md` | Bead IDs in learning entry headers |
| `.gromit/status.json` | Current bead_id field |
| `.gromit/state.json` | May contain bead references |
| `internal/runner/format_test.go` | Hardcoded test fixture bead IDs |
| `.gromit/specs/ralph-reference-cleanup.md` | Spec with now-superseded decision |
