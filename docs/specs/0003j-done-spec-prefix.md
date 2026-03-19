# Spec 0003j — Done Spec Prefix

## spec_id
done-spec-prefix

## Vision
Completed specs clutter the picker and `spec list` equally with active work. Today there's no way to mark a spec as "done" — specs that predate run tracking appear as `ready` forever, and even specs with `completed` runs still show up undifferentiated. Adding a `DONE` content prefix (mirroring the existing `DRAFT` pattern) gives a lightweight, file-local signal that a spec's work is merged and finished, keeping the picker focused on active work while preserving history in `spec list`.

## Summary
Add a `DONE YYYY-MM-DD` content prefix for spec files that excludes them from the exec picker, sorts them to the bottom of `spec list` with their completion date displayed, and is automatically applied when `exec complete` marks a run as finished. All specs before 0003h are marked as done retroactively.

## Goals
### Primary
- Parse `DONE YYYY-MM-DD` prefix in `DeriveSpecStatusFromContent()` to derive a `"done"` status
- Exclude `"done"` specs from the exec picker
- Show `"done"` specs at the bottom of `spec list` with their completion date
- Auto-prepend `DONE YYYY-MM-DD` to the spec file when `exec complete` is run
- Mark all specs before 0003h as `DONE 2026-03-19`

## Non-goals
- No changes to `DRAFT` prefix behavior
- No automated merge/branch cleanup (out of scope)
- No spec archival or deletion workflow

## Architecture
The change touches three existing files and no new files:

**`spec.go` — `DeriveSpecStatusFromContent()`**
Add a `DONE` prefix check before the existing `DRAFT` check. Parse the date from `DONE YYYY-MM-DD` format. Return `"done"` status. Extract a helper to parse the date:

```go
// ParseDoneDate extracts the completion date from a "DONE YYYY-MM-DD" prefix.
// Returns zero time if content doesn't start with "DONE".
func ParseDoneDate(content string) (time.Time, bool)
```

**`spec.go` — `spec list` command**
Sort output so `"done"` specs appear after all other statuses. Add the completion date to the status column (e.g., `done (2026-03-19)`).

**`exec_complete.go` — `exec complete` command**
After marking the run as completed, resolve the spec file path from the run's `SpecID` and prepend `DONE YYYY-MM-DD\n` to its content. Needs access to the specs directory — either from a flag or by loading the project config.

**Spec files in `docs/specs/`**
Prepend `DONE 2026-03-19` to: `0002e`, `0002f`, `0003a`, `0003b`, `0003c`, `0003d`, `0003e`, `0003f`, `0003g`.

Key decision: `DONE` takes precedence over `DRAFT` — if somehow both are present, `DONE` wins since it represents a later lifecycle state. And `DONE` takes precedence over run-derived status, same as `DRAFT` does today.

## Acceptance Criteria

1. When a spec file's content starts with `DONE YYYY-MM-DD`, `DeriveSpecStatusFromContent()` returns `"done"`
2. When a spec file starts with `DONE` followed by an unparseable or missing date, `DeriveSpecStatusFromContent()` still returns `"done"` (graceful degradation)
3. `DONE` prefix takes precedence over both `DRAFT` prefix and run-derived status
4. The exec picker excludes specs with `"done"` status
5. `spec list` displays `"done"` specs after all other statuses
6. `spec list` shows the completion date in the status column as `done (2026-03-19)`
7. When `exec complete` is run, the spec file is prepended with `DONE YYYY-MM-DD\n` using today's date
8. If `exec complete` is run on a spec that's already `DONE`, it does not add a second prefix
9. `ParseDoneDate()` correctly extracts the date from well-formed `DONE YYYY-MM-DD` prefixes
10. All specs before 0003h are prepended with `DONE 2026-03-19`
11. All existing tests continue to pass

## Scenarios

### Scenario: Spec picker skips done specs
**Given:** `docs/specs/` contains `0003a.md` starting with `DONE 2026-03-19` and `0003h.md` with no prefix
**When:** The user runs `exec` and the picker presents available specs
**Then:** Only `0003h` appears in the picker list; `0003a` is not shown

### Scenario: Spec list shows done specs at bottom with date
**Given:** Three specs — `0003a` (done 2026-03-19), `0003h` (ready), `0003i` (ready_for_review)
**When:** The user runs `spec list`
**Then:** Output shows `0003h` and `0003i` first, then `0003a` last with status `done (2026-03-19)`

### Scenario: exec complete marks spec file as done
**Given:** Run `run-abc123` has `SpecID: "0003h"` and status `ready_for_review`
**When:** The user runs `exec complete run-abc123`
**Then:** The run status is set to `completed`, and `docs/specs/0003h.md` now starts with `DONE 2026-03-19\n` followed by its original content

### Scenario: exec complete on already-done spec is idempotent
**Given:** `0003a.md` already starts with `DONE 2026-03-15` and run `run-xyz` references it
**When:** The user runs `exec complete run-xyz`
**Then:** The run is marked completed but the spec file is unchanged — no second `DONE` line added

### Scenario: DONE with malformed date still excludes from picker
**Given:** `0003b.md` starts with `DONE not-a-date`
**When:** The picker builds the spec list
**Then:** `0003b` is excluded from the picker. `spec list` shows it as `done` with no date in parentheses

## Validation
```
go test ./cmd/gromit-next/...
go test ./internal/next/...
go vet ./...
```
