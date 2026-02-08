---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T18:27:22-05:00"
id: gromit-prefix-migration
source_spec: gromit-prefix-migration
---

# Gromit Prefix Migration Implementation Plan

**Goal:** Consolidate all 395 bd issues from `ralph-runner-` and `ralph-` prefixes to `gromit-`, and update secondary files to match.

**Architecture:** Run `bd rename-prefix gromit- --repair` for the database migration, then update LEARNINGS.md, status.json, and format_test.go using the actual ID mapping output.

**Tech Stack:** bd CLI, Go (test fixtures only)

**Spec:** `.gromit/specs/gromit-prefix-migration.md`

---

## Architecture

Data migration using `bd rename-prefix --repair` to consolidate two stale prefixes into one, followed by mechanical updates to three secondary files. No new code, no structural changes.

**Files to Modify:**
- `.gromit/LEARNINGS.md` — Update ~30 bead ID headers to new mapped IDs
- `.gromit/status.json` — Update `bead_id` field to new mapped ID
- `internal/runner/format_test.go` — Replace test fixture bead IDs with `gromit-` prefixed ones
- `.gromit/specs/ralph-reference-cleanup.md` — Mark as superseded
- `.gromit/plans/ralph-reference-cleanup.md` — Mark as superseded

**Out of Scope:**
- `.gromit/state.json` — No ralph references
- `.gromit/specs/gromit-prefix-migration.md` — Documentary references in the migration spec itself
- Other plan/spec files with historical bead ID references — Already decomposed, churn with no benefit

## Test Strategy

**Test Levels:**
1. **Unit Tests**: `go test ./...` after updating format_test.go fixtures
2. **Database Verification**: `bd list --json` confirms all issues have `gromit-` prefix
3. **Grep Verification**: `grep -ri ralph .gromit/LEARNINGS.md .gromit/status.json internal/runner/format_test.go` returns no matches

**Key Test Cases:**
- `go test ./internal/runner/...` passes with updated fixture IDs
- `bd list --json | grep -c '"ralph'` returns 0
- `.beads/issues.jsonl` contains only `gromit-` prefixed IDs
- LEARNINGS.md bead ID headers all use `gromit-` prefix
- All existing tests pass

## Implementation Tasks

### Task 1: Run bd rename-prefix to migrate the database

**Files:**
- Modified by tool: `.beads/issues.jsonl`, `.beads/beads.db`

**What to Do:**
1. Run `bd rename-prefix gromit- --repair --dry-run` first to preview the migration
2. Run `bd rename-prefix gromit- --repair` to perform the actual migration
3. Capture the old→new ID mapping output — this is needed for Task 2
4. Verify with `bd list --json | head -5` that IDs now use `gromit-` prefix

**Acceptance Criteria:**
- `bd list --json` shows all issues with `gromit-` prefix
- No `ralph-runner-` or `ralph-` prefixed IDs remain in the database
- `.beads/issues.jsonl` contains only `gromit-` prefixed IDs

**Dependencies:** None

### Task 2: Update LEARNINGS.md and status.json with new bead IDs

**Files:**
- Modify: `.gromit/LEARNINGS.md`
- Modify: `.gromit/status.json`

**What to Do:**
Using the ID mapping from Task 1:
- In LEARNINGS.md, update all `ralph-runner-*` bead IDs in learning entry headers (e.g., `### 2026-02-07 | ralph-runner-4a3f | patterns` → `### 2026-02-07 | gromit-<mapped-id> | patterns`)
- In LEARNINGS.md Confirmed section, update the consolidated-from references (e.g., `consolidated from m7td, 6rao` — these are short suffixes that may or may not change depending on mapping)
- In status.json, update the `bead_id` field from `ralph-runner-hqnh` to its new `gromit-` mapped ID

**Acceptance Criteria:**
- `grep -ri ralph .gromit/LEARNINGS.md .gromit/status.json` returns no matches
- All bead IDs in LEARNINGS.md headers use `gromit-` prefix

**Dependencies:** Task 1 (needs the ID mapping output)

### Task 3: Update format_test.go test fixtures

**Files:**
- Modify: `internal/runner/format_test.go`

**What to Do:**
Replace the two hardcoded test fixture bead IDs and their expected output strings:
- `ralph-runner-abc123` → `gromit-abc123` (arbitrary, just needs `gromit-` prefix)
- `ralph-runner-ja5m` → `gromit-ja5m` (arbitrary, just needs `gromit-` prefix)
- Update the corresponding expected output lines that reference these IDs

**Acceptance Criteria:**
- `grep -ri ralph internal/runner/format_test.go` returns no matches
- `go test ./internal/runner/...` passes

**Dependencies:** None (independent of Task 1 — these are arbitrary test data)

### Task 4: Mark ralph-reference-cleanup spec and plan as superseded

**Files:**
- Modify: `.gromit/specs/ralph-reference-cleanup.md`
- Modify: `.gromit/plans/ralph-reference-cleanup.md`

**What to Do:**
Add a note at the top of both files indicating they are superseded by the `gromit-prefix-migration` spec. Decision #2 ("Leave `.beads/issues.jsonl` alone") is no longer valid since `bd rename-prefix` safely handles the database migration.

**Acceptance Criteria:**
- Both files contain a superseded notice referencing `gromit-prefix-migration`
- Decision #2 in the spec is annotated as superseded

**Dependencies:** None

---

## Notes

- **Task 1 is the critical path.** The `bd rename-prefix` command generates new short IDs that are unpredictable. Task 2 cannot start until the mapping is known.
- **Task 3 is independent** and can be done in parallel with Tasks 1+2 since the test fixtures use arbitrary IDs.
- **Task 4 is cosmetic** and can be done at any time.
- The Confirmed section in LEARNINGS.md has consolidated-from references like `consolidated from m7td, 6rao`. These are short suffixes extracted from old bead IDs. After rename, the full IDs change but these abbreviated suffixes in prose text may not have a clean mapping. Use best-effort matching from the rename output.
