# Run Monitoring Debug Log

## Date
- 2026-02-15

## Reported Symptoms
- `./gromit run -n 1` appeared stuck during ATDD.
- Stream log files were often empty (`.gromit/logs/stream-*.log`).
- `status.json` showed `elapsed_s: 0` even while the process was active.

## What We Did
1. Reproduced and monitored multiple live runs.
2. Confirmed runs were active (gromit + codex child processes alive) even when no visible output appeared.
3. Added ATDD invocation diagnostics in `internal/runner/callbacks.go`.
4. Added provider stream diagnostics in `internal/provider/codex.go` (behind `GROMIT_CODEX_DEBUG=1`) around:
   - command start
   - stream scanner lifecycle
   - event flow
   - wait boundaries
5. Reran with debug enabled and observed steady event flow (events continued increasing over time).
6. Implemented product changes for visibility and status correctness:
   - ATDD progress heartbeat (non-debug) in `internal/runner/callbacks.go`
   - periodic `status.json` refresh during bead execution in `internal/runner/runner.go`
   - thread-safe `StatusWriter` in `internal/runner/status.go`

## Findings
- Codex stream was not deadlocked in the observed runs; machine events were still arriving.
- The apparent “hang” was mostly lack of user-visible progress during long ATDD phases.
- `elapsed_s` staying at `0` was a real UX/state bug (status file only updated at iteration boundaries).

## Root Cause
- Two issues combined:
  1. **Insufficient runtime progress signaling** during ATDD streaming (users saw silence and inferred a stall).
  2. **Stale status reporting** (`elapsed_s` not refreshed while an iteration was running).

## Fixes Applied
- `internal/runner/callbacks.go`
  - Added ATDD stream lifecycle logs and periodic heartbeat:
    - `ATDD stream started ...`
    - `ATDD stream connected ...`
    - `ATDD progress: elapsed=... events=... idle=...`
    - completion/error summaries
- `internal/runner/runner.go`
  - Added 5-second periodic status writer updates during each iteration.
- `internal/runner/status.go`
  - Added mutex protection for concurrent status writes/deletes/final writes.

## Verification
- Live smoke run confirmed:
  - `ATDD progress` logs appear during long ATDD phase.
  - `.gromit/status.json` now increments `elapsed_s` while running.

## Notes
- Additional debug instrumentation remains in `internal/provider/codex.go` and can be toggled with `GROMIT_CODEX_DEBUG=1` for future stream diagnostics.
