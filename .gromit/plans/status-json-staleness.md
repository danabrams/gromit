---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T08:38:37-05:00"
id: status-json-staleness
source_spec: status-json-staleness
---

# Status JSON Staleness Detection Implementation Plan

**Goal:** Detect stale `status.json` files left by crashed `gromit run` processes using PID liveness checks, and surface this info in `gromit status`.

**Architecture:** Add a `pid` field to the existing `Status` struct, a `ReadStatus()` function, and an `IsProcessAlive()` helper — all in `internal/runner/status.go`. Then modify `Runner.Status()` to read/validate the status file before showing the normal bead queue.

**Tech Stack:** Go, `os`/`syscall` packages for PID liveness checks

**Spec:** `.gromit/specs/status-json-staleness.md`

---

## Architecture

**Key Components:**

1. **`internal/runner/status.go`**: Add `PID int` field to `Status` struct. Write `os.Getpid()` in `StatusWriter.Write()`. Add `ReadStatus(gromitDir string) (*Status, error)` to load and parse the file. Add `IsProcessAlive(pid int) bool` using signal-0 technique.

2. **`internal/runner/runner.go`**: Modify `Runner.Status()` to call `ReadStatus(r.gromitDir)` first. If status file exists, check PID liveness and either display run-in-progress info or warn + delete stale file.

**Integration Points:**
- `Status` struct gets a new `PID` field (additive, backward-compatible JSON)
- `StatusWriter.Write()` gains one line: `PID: os.Getpid()`
- `Runner.Status()` adds ~20 lines before the existing `r.beads.Ready()` call
- `showStatus` in `cmd/gromit/main.go` stays unchanged

**Tradeoffs:**
- PID check lives in `status.go` since it's tightly coupled to the status file — no separate package needed
- `IsProcessAlive` is a package-level function for independent testability

## Test Strategy

**Unit Tests (`internal/runner/status_test.go`):**
- `TestStatusWriter_Write_IncludesPID`: Write status, verify `pid` field in JSON
- `TestReadStatus_Valid`: Write file, call ReadStatus, verify all fields
- `TestReadStatus_NoFile`: Empty dir, verify nil return
- `TestIsProcessAlive_Self`: Current PID returns true
- `TestIsProcessAlive_Dead`: Dead PID returns false

**Integration Tests (Runner.Status behavior):**
- `TestStatus_NoStatusFile`: No file, normal bead queue output
- `TestStatus_RunInProgress`: Status file with live PID, shows iteration/bead/model
- `TestStatus_StaleRun`: Status file with dead PID, warns and deletes file

**Mocking:** Use `NewRunnerWithDeps` with mock `BeadClient` for Runner.Status tests. `IsProcessAlive` and `ReadStatus` use real syscalls and file I/O with `t.TempDir()`.

## Implementation Tasks

### Task 1: Add PID field to Status struct and StatusWriter.Write

**Files:**
- Modify: `internal/runner/status.go`
- Test: `internal/runner/status_test.go`

**What to Do:**
Add `PID int \`json:"pid"\`` field to the `Status` struct. In `StatusWriter.Write()`, set `PID: os.Getpid()` when constructing the status value. Update the existing `TestStatusWriter_Write` test to also verify the `pid` field, and add a dedicated `TestStatusWriter_Write_IncludesPID` test.

**Acceptance Criteria:**
- `Status` struct has a `PID int` field with JSON tag `"pid"`
- `StatusWriter.Write()` sets `PID` to `os.Getpid()`
- Test verifies the written JSON contains the correct PID value

**Dependencies:** None

### Task 2: Add ReadStatus and IsProcessAlive functions with tests

**Files:**
- Modify: `internal/runner/status.go`
- Test: `internal/runner/status_test.go`

**What to Do:**
Add `ReadStatus(gromitDir string) (*Status, error)` that reads and parses `.gromit/status.json`. Return `nil, nil` when the file doesn't exist (not an error — just means no run). Add `IsProcessAlive(pid int) bool` using `os.FindProcess(pid)` + `process.Signal(syscall.Signal(0))`. Add tests for both functions.

**Acceptance Criteria:**
- `ReadStatus` returns parsed `*Status` for a valid file
- `ReadStatus` returns `nil, nil` when file doesn't exist
- `IsProcessAlive(os.Getpid())` returns true
- `IsProcessAlive(<dead-pid>)` returns false

**Dependencies:** Task 1 (needs PID field in Status struct)

### Task 3: Modify Runner.Status to read and validate status.json

**Files:**
- Modify: `internal/runner/runner.go`

**What to Do:**
At the top of `Runner.Status()`, call `ReadStatus(r.gromitDir)`. If a status is returned:
- Call `IsProcessAlive(status.PID)`. If alive, print run-in-progress info (iteration, bead ID, bead title, model, elapsed time) before continuing to normal queue output.
- If dead, print a warning (when the run started, which bead, how long ago), delete the stale file via `os.Remove`, then continue to normal queue output.
If no status file, proceed with existing behavior unchanged.

**Acceptance Criteria:**
- Live PID: output includes "Run in progress" with iteration, bead, model, elapsed
- Dead PID: output includes "Warning: stale run" info and file is deleted
- No file: existing output unchanged

**Dependencies:** Task 2 (needs ReadStatus and IsProcessAlive)

### Task 4: Add Runner.Status integration tests

**Files:**
- Test: `internal/runner/status_test.go` (or a new section using NewRunnerWithDeps)

**What to Do:**
Write tests for the three `Status()` scenarios using `NewRunnerWithDeps` with a mock `BeadClient`. For the "run in progress" test, write a status file with `os.Getpid()`. For the "stale run" test, write a status file with a dead PID (use a subprocess that has exited). For the "no file" test, use an empty gromit dir.

**Acceptance Criteria:**
- Test verifies run-in-progress output when PID is alive
- Test verifies warning output and file deletion when PID is dead
- Test verifies normal output when no status file exists

**Dependencies:** Task 3 (needs the modified Status method)

---

## Notes

- The `status.json` on disk right now is stale (iteration 64, started hours ago, no PID). Once this is implemented, `gromit status` will detect and clean it up.
- `IsProcessAlive` is Unix-specific (signal 0). This is fine — gromit targets Unix systems.
- The `ReadStatus` returning `nil, nil` for missing file is intentional — "no status file" is a normal state, not an error.
