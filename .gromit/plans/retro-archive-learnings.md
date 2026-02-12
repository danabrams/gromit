---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T07:28:16-05:00"
id: retro-archive-learnings
source_spec: retro-archive-learnings
---

# Separate Archived Learnings Implementation Plan

**Goal:** Move archived learnings to a separate file (`.gromit/LEARNINGS_ARCHIVE.md`) and track their hashes in `state.json` for O(1) dedup, eliminating archive parsing overhead on every load.

**Architecture:** Archived learnings are written to an append-only archive file and their hashes stored in state.json. The `learnings.File` struct gains `archivePath` and `archivedHashes` fields, wired by callers via setter methods (matching the existing `SetFilter()` pattern). Migration of existing `## Archived` sections happens automatically during `Load()`.

**Tech Stack:** Go, file I/O, JSON state persistence

**Spec:** `.gromit/specs/retro-archive-learnings.md`

---

## Architecture

**Key Components:**
1. **`internal/state/state.go`** — New `ArchivedLearningHashes` field on State struct with getter/setter methods, mirroring the existing `FilteredLearningHashes` pattern
2. **`internal/learnings/learnings.go`** — Core changes: new fields (`archivePath`, `archivedHashes`), changed methods (`Archive`, `Add`, `Save`, `Load`, `FilterProvisional`), new helpers (`appendToArchiveFile`, `migrateArchived`)

**Integration Points:**
- `Archive()` appends to `.gromit/LEARNINGS_ARCHIVE.md` and adds hash to in-memory `archivedHashes` set
- `hashExists()` checks `archivedHashes` map (from state.json) in addition to confirmed/provisional slices
- `Save()` writes only Confirmed and Provisional sections
- `Load()` detects `## Archived` section and triggers one-time migration
- `FilterProvisional()` archives generic learnings to archive file instead of in-memory slice
- Runner and Retro wire archived hashes from state.json into learnings via `SetArchivedHashes()`

**State Dependency (Decoupled):**
- `learnings.File` has no import dependency on `state` package
- Callers set archived hashes via `SetArchivedHashes(map[string]bool)` and read them back via `GetArchivedHashes() map[string]bool`
- Callers persist to state.json — same pattern as `SetFilter()`

## Test Strategy

**Unit Tests:**
- `internal/state/state_test.go`: ArchivedLearningHashes persistence, getter returns map, setter deduplicates, NormalizeNilFields, nil-safety
- `internal/learnings/learnings_test.go`: Archive writes to file, Add rejects archived duplicates, Save omits Archived section, migration triggers on Load, migration is idempotent, FilterProvisional uses archive file, round-trip archive-then-dedup

**Mocking Strategy:**
- No mocks — all tests use real file I/O with `t.TempDir()`, matching existing patterns in both packages

**Coverage Goals:**
- All 4 spec acceptance criteria directly covered
- Edge cases: empty archive file, missing archive file (created on first write), migration with empty Archived section, backward compatibility (no SetArchivedHashes call)
- Existing tests updated to reflect new behavior (no more `f.archived` in-memory state after archive operations)

## Implementation Tasks

### Task 1: Add ArchivedLearningHashes to state

**Files:**
- Modify: `internal/state/state.go`
- Test: `internal/state/state_test.go`

**What to Do:**
Add `ArchivedLearningHashes []string` field to the `State` struct with JSON tag `archived_learning_hashes,omitempty`. Add `GetArchivedHashes() map[string]bool` method (returns map for O(1) lookups, matching `GetFilteredHashes` pattern). Add `AddArchivedHashes(hashes []string)` method (merges and deduplicates, matching `AddFilteredHashes` pattern). Update `NormalizeNilFields()` to initialize `ArchivedLearningHashes` to empty slice if nil.

**Acceptance Criteria:**
- ArchivedLearningHashes persists through save/load round-trip
- GetArchivedHashes returns map with O(1) lookup, nil-safe on nil receiver
- AddArchivedHashes merges without duplicates

**Dependencies:** None

### Task 2: Core archive behavior — Archive(), Save(), Add(), hashExists()

**Files:**
- Modify: `internal/learnings/learnings.go`
- Test: `internal/learnings/learnings_test.go`

**What to Do:**
Add `archivePath string` and `archivedHashes map[string]bool` fields to the `File` struct. Set `archivePath` in `NewFile()` to `filepath.Join(dir, "LEARNINGS_ARCHIVE.md")`. Add `SetArchivedHashes(hashes map[string]bool)` and `GetArchivedHashes() map[string]bool` methods. Add `appendToArchiveFile(learning Learning) error` helper that formats the learning and appends to the archive file (creating it if needed).

Change `Archive()`: after removing learning from confirmed/provisional and annotating content, call `appendToArchiveFile()` instead of appending to `f.archived`. Add hash to `f.archivedHashes`. Then call `Save()` (which no longer writes archived section).

Change `hashExists()`: after checking confirmed and provisional slices, check `f.archivedHashes[hash]` instead of iterating `f.archived`.

Change `Save()`: remove the `## Archived` section writing block entirely. File now contains only `## Confirmed` and `## Provisional`.

Change `Add()`: the filter-as-generic path currently appends to `f.archived` — change it to call `appendToArchiveFile()` and add hash to `f.archivedHashes` instead.

Update existing tests: `TestArchive`, `TestArchiveFromConfirmed`, `TestLoadAndSaveWithArchived`, `TestFilterFuncGeneric`, `TestAddArchivedDuplicateReturnsNil`, `TestAddArchivedDuplicateDoesNotCallFilter` — these reference `f.archived` or expect `## Archived` in saved output. Update to verify archive file contents and archivedHashes map instead. Update `TestParseArchivedSection` to test that archived entries are still parsed (needed for migration) but no longer persisted to the archived slice after migration runs.

**Acceptance Criteria:**
- Archiving a learning writes it to `LEARNINGS_ARCHIVE.md` and removes it from `LEARNINGS.md` (no `## Archived` section remains)
- Adding a learning that was previously archived (same hash in archivedHashes) is rejected as duplicate
- Save() produces a file with only `## Confirmed` and `## Provisional` sections

**Dependencies:** None (state wiring happens in Task 4)

**Notes:**
- `GetByHash()` and `Remove()` currently search `f.archived` — update to no longer search it (archived entries are in a separate file, not in memory). These methods are used by `Replace()` during retro consolidation, which operates on confirmed/provisional learnings only.
- The `archived` slice on the struct is retained but only used transiently during Load() for migration detection (Task 3). After migration, it's empty.
- `normalizeNilFields()` still initializes `archived` to empty slice for backward compatibility.

### Task 3: Migration logic in Load() and FilterProvisional() update

**Files:**
- Modify: `internal/learnings/learnings.go`
- Test: `internal/learnings/learnings_test.go`

**What to Do:**
Change `Load()`: after `parseLearnings()` populates the `archived` slice, check if it's non-empty. If so, call new `migrateArchived() error` method, then clear `f.archived` to empty slice.

Implement `migrateArchived()`: iterate archived entries. For each, check if hash already exists in `f.archivedHashes` (skip if duplicate — idempotency). If not, call `appendToArchiveFile()` and add hash to `f.archivedHashes`. After all entries processed, call `Save()` to rewrite LEARNINGS.md without the archived section. Return nil on success.

Change `FilterProvisional()`: the archival loop currently appends to `f.archived`. Change it to call `appendToArchiveFile()` and add hash to `f.archivedHashes` instead.

Add tests: migration with existing Archived section (entries move to file, hashes populated, section removed); migration is idempotent (run twice, no duplicate entries in archive file); migration with empty Archived section header; FilterProvisional writes to archive file; round-trip test (archive via FilterProvisional, create new File, set archived hashes, add same content — rejected).

**Acceptance Criteria:**
- Loading LEARNINGS.md with `## Archived` section triggers migration: entries appear in archive file, hashes in archivedHashes, section removed from LEARNINGS.md
- Migration is idempotent — running again with pre-existing archive file doesn't duplicate entries
- FilterProvisional archives generic learnings to archive file, not in-memory slice

**Dependencies:** Task 2

### Task 4: Wire archived hashes in runner and retro

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/retro/retro.go`

**What to Do:**
In `runner.go` (around line 108, after `lf := renderer.GetLearningsFile()`): load archived hashes from state via `stateFile.GetArchivedHashes()` and pass to learnings via `lf.SetArchivedHashes()`. After the run loop completes (or at save points), persist updated hashes back via `stateFile.AddArchivedHashes()` from `lf.GetArchivedHashes()`.

In `retro.go` `Run()` method (around line 128, after state is loaded): call `r.learningsFile.SetArchivedHashes(stateFile.GetArchivedHashes())`. After filtering/analysis, persist any new archived hashes back: `stateFile.AddArchivedHashes(...)` and save state.

**Acceptance Criteria:**
- Runner loads archived hashes from state and passes to learnings file before any Add/Archive operations
- Retro loads archived hashes from state and passes to learnings file before filtering
- Updated archived hashes are persisted back to state.json

**Dependencies:** Task 1, Task 2

---

## Notes

- The interactive retro session (launched by `LaunchClaudeCode`) may still move entries to `## Archived` in LEARNINGS.md directly (since Claude Code edits the file as text). The migration-on-Load mechanism in Task 3 serves as a safety net for this — entries will be migrated on the next `gromit run`.
- Consider updating the `LaunchClaudeCode` prompt in a follow-up to instruct Claude Code to write directly to `LEARNINGS_ARCHIVE.md` instead of the `## Archived` section.
- The `archived` field on the `File` struct is retained as a transient field used only during Load/migration. It is always empty after Load completes.
- Existing `archived_dedup_acceptance_test.go` tests will need updates since they test the current in-memory archival behavior. These should be updated to verify archive file contents and archivedHashes map state.
