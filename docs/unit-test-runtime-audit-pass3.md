# Unit Test Runtime Audit (Pass 3)

Date: 2026-02-17

Scope:
- Included: `*_test.go` files treated as unit tests.
- Excluded: `*acceptance_test.go`, `*integration_test.go`, `*contract*_test.go`, and `test/e2e/**`.
- Patterns searched: `time.Sleep(...)`, recursive helper/test logic, and subprocess execution via `exec.Command(...)` / `exec.CommandContext(...)`.

## 1) Unit Tests Using Sleep

| File | Lines | Notes |
|---|---|---|
| `internal/logger/stream_rate_limit_recovery_test.go` | 18, 41, 58, 113, 123, 153, 155, 159, 190, 200, 215, 241 | heavy timing-based assertions |
| `internal/logger/stream_test.go` | 138, 149, 159, 172, 368, 551, 560, 618 | repeated timing waits |
| `internal/runner/runner_test.go` | 488, 534, 575, 606, 630, 636, 664 | heartbeat/timing windows |
| `internal/runner/status_test.go` | 131, 573, 640 | elapsed-time assertions |
| `internal/runner/execution/heartbeat_test.go` | 171, 181, 220 | async heartbeat timing |
| `internal/runner/validation/validation_test.go` | 188 | bounded concurrency timing |
| `internal/runner/rate_limit_recovery_logging_test.go` | 30 | recovery timing |
| `internal/runner/process_test.go` | 1648 | delayed goroutine completion |
| `internal/pipeline/decompose_test.go` | 736 | long sleep (`5s`) |
| `internal/logger/diagnostic_snapshot_test.go` | 15 | snapshot timing |
| `internal/backlog/backlog_test.go` | 113 | short sleep |

## 2) Unit Tests With Recursive Logic

| File | Lines | Recursive logic |
|---|---|---|
| `internal/retro/experiment_test.go` | 361, 362 | `contains(s, substr)` recursively calls `contains(s[1:], substr)` |
| `internal/runner/runner_split_phase1_test.go` | 66, 69 | `receiverName` recursively unwraps `*ast.StarExpr` |
| `internal/runner/runner_split_phase1_reclassified_test.go` | 76, 79 | `receiverTypeName` recursively unwraps `*ast.StarExpr` |

Notes:
- Recursion found in the unit-test scope is helper-level and bounded (string length / AST pointer depth).
- No other direct recursive self-calls were found in unit test functions.

## 3) Unit Tests That Shell Out to Executables

| File | Lines | Executable(s) |
|---|---|---|
| `cmd/gromit/epic_test.go` | 392 | `bd` |
| `cmd/gromit/repo_hygiene_test.go` | 21 | `git` |
| `internal/bead/bead_test.go` | 1029 | `bd` |
| `internal/runner/runner_split_final_verification_lint_test.go` | 20 | `golangci-lint` |
| `internal/runner/runner_split_final_verification_reclassified_test.go` | 244, 276 | `go`, `wc` |
| `internal/runner/runner_split_phase4_reclassified_test.go` | 173 | `wc` |
| `internal/runner/runner_split_phase_build_checks_test.go` | 27 | variable executable (`args[0]`) |
| `internal/runner/golangci_lint_acceptance_helpers_test.go` | 34, 54 | `go`, candidate `golangci-lint` binaries |

## Scan Notes

- Some unit tests assert source text contains `exec.Command(...)` strings; these were not counted as shell-outs unless a subprocess is actually launched.
- Acceptance/integration/contract/e2e tests contain additional sleep and subprocess usage not listed here by design.
