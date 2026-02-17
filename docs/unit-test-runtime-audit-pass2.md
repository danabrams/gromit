# Unit Test Runtime Audit (Pass 2)

Date: 2026-02-17

Scope:
- Included: `*_test.go` files that look like unit tests.
- Excluded from the primary list: `*acceptance_test.go`, `*integration_test.go`, contract tests, and `test/e2e/**`.
- Patterns searched: sleep usage, recursive logic in test code/helpers, and subprocess execution (`exec.Command`).

## 1) Tests Using Sleep

| File | Lines | Notes |
|---|---|---|
| `internal/logger/stream_rate_limit_recovery_test.go` | 18,41,58,113,123,153,155,159,190,200,215,241 | heavy timing-based assertions |
| `internal/logger/stream_test.go` | 138,149,159,172,368,551,560,618 | repeated timing waits |
| `internal/runner/runner_test.go` | 488,534,575,606,630,636,664 | heartbeat/timing windows |
| `internal/runner/status_test.go` | 132,574,641 | async polling windows |
| `internal/runner/execution/heartbeat_test.go` | 171,181,220 | timing-sensitive behavior |
| `internal/provider/codex_test.go` | 398,538,540 | sleep calls inside test scripts |
| `internal/runner/validation/validation_test.go` | 188 | single sleep |
| `internal/runner/rate_limit_recovery_logging_test.go` | 30 | single sleep |
| `internal/runner/process_test.go` | 1648 | single sleep |
| `internal/pipeline/decompose_test.go` | 736 | long sleep (`5s`) |
| `internal/logger/diagnostic_snapshot_test.go` | 15 | single sleep |
| `internal/backlog/backlog_test.go` | 113 | short sleep |

## 2) Tests With Recursive Logic

| File | Lines | Recursive logic |
|---|---|---|
| `internal/retro/experiment_test.go` | 361,362 | `contains(s, substr)` recursively calls `contains(s[1:], substr)` |
| `internal/runner/runner_split_phase1_test.go` | 66,69 | `receiverName` recursively unwraps `*ast.StarExpr` |
| `internal/runner/runner_split_phase1_reclassified_test.go` | 76,79 | `receiverTypeName` recursively unwraps `*ast.StarExpr` |

Notes:
- No broad/accidental recursion patterns were found in the unit-test set beyond these helpers.
- The two AST helpers are bounded recursion (pointer depth), not likely major runtime drivers.

## 3) Tests That Shell Out to Other Executables

| File | Lines | Executable(s) | Status |
|---|---|---|---|
| `cmd/gromit/install_skill_test.go` | 580,592,721,733,778,837,849,883,934,990 | `go`, built `gromit` binary | Done (converted integration tests to in-process `runInstallSkill`) |
| `cmd/gromit/epic_test.go` | 392 | `bd` | In progress |
| `cmd/gromit/repo_hygiene_test.go` | 21 | `git` | In progress |
| `cmd/gromit/test_binary_helpers_test.go` | 28 | `go` | Done (removed `go build`; uses test-helper process harness) |
| `internal/agent/agent_test.go` | 24 | `echo` | Done (replaced with mocked command/run hooks) |
| `internal/agent/agent_launch_in_dir_test.go` | 23 | `echo` | Done (replaced with mocked command/run hooks) |
| `internal/bead/bead_test.go` | 1029 | `bd` | In progress |
| `internal/tmux/tmux_test.go` | 80 | `tmux` | Done (converted to mock command runner) |
| `internal/runner/status_test.go` | 804 | `true` | Done (dead PID no longer spawned via subprocess) |
| `internal/runner/runner_split_final_verification_lint_test.go` | 20 | `golangci-lint` | In progress |
| `internal/runner/runner_split_final_verification_reclassified_test.go` | 244,276 | `go`, `wc` | In progress |
| `internal/runner/runner_split_phase4_reclassified_test.go` | 173 | `wc` | In progress |
| `internal/runner/runner_split_phase_build_checks_test.go` | 27 | variable command (`args[0]`) | In progress |
| `internal/runner/golangci_lint_acceptance_helpers_test.go` | 34,54 | `go`, candidate `golangci-lint` binaries | In progress |

## Out-of-Scope But Notable

The broader codebase (acceptance/e2e) also includes additional sleep/subprocess patterns (for example `test/e2e/e2e_test.go` recursion via `copyScaffold` and several acceptance tests that shell out).
