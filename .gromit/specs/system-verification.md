---
id: system-verification
source_ideas: []
created: 2026-02-06
epic: developer-experience
---

# System Verification

## Specification

Gromit has 519 unit and integration tests, but nothing that verifies the system works end-to-end. The CLI interface, external tool interactions, and multi-step workflows are all untested at the system level. This spec adds three layers of verification to catch regressions that unit tests miss.

### Layer 1: CLI Contract Tests

A test file `cmd/gromit/cli_contract_test.go` freezes the CLI interface. It builds the real `gromit` binary once via `TestMain`, then runs table-driven subtests for every subcommand.

Each subcommand gets:
- **Help text snapshot** — runs `gromit <cmd> --help`, compares output against a golden file in `cmd/gromit/testdata/golden/<cmd>.help.txt`. A `-update` test flag regenerates golden files for intentional changes.
- **Flag contract** — a Go struct per command declaring expected flags, types, and defaults. Adding or removing a flag requires updating the struct. This is the hard contract.
- **Exit code assertions** — commands run with bad input assert specific exit codes.
- **Output format assertions** — commands with `--json` output assert the JSON schema.

File structure:
```
cmd/gromit/
├── cli_contract_test.go
└── testdata/
    └── golden/
        ├── root.help.txt
        ├── run.help.txt
        ├── init.help.txt
        ├── status.help.txt
        ├── retro.help.txt
        ├── add.help.txt
        ├── backlog.help.txt
        ├── board.help.txt
        ├── queue.help.txt
        ├── triage.help.txt
        ├── refine.help.txt
        ├── plan.help.txt
        ├── review.help.txt
        └── decompose.help.txt
```

Runs with `go test ./cmd/gromit/` — fast, no external dependencies.

### Layer 2: External Tool Contract Tests (Fake CLIs)

Shell script fakes for `bd`, `claude`, and `git` that simulate the real tools. Gromit runs against these via `PATH` manipulation.

Each fake does three things:
1. **Records invocations** — appends the full command line to `$TEST_CALL_LOG`. Tests assert the exact commands gromit sent.
2. **Returns canned responses** — based on subcommand/args, returns the appropriate fixture file. E.g., `bd ready --json --limit 1` returns `bd_ready.json`.
3. **Simulates failures** — environment variables toggle error scenarios (`BD_FAIL=1` makes bd return exit code 1).

The `bd` fake is **stateful**: it keeps a JSON file at `$TEST_DIR/.fake_bd_state.json` tracking which beads exist and their status. `bd ready` returns the next unclosed bead. `bd close <id>` marks it closed. This supports testing multi-bead flows where closing one bead makes the next one ready.

The `git` fake is a **passthrough wrapper**: it calls real git but records all invocations. Gromit does real git operations in a temp repo while tests verify what commands were run.

The `claude` fake returns canned output from fixture files, with environment variables controlling which fixture to return and whether to simulate failure.

File structure:
```
test/
├── fakes/
│   ├── bd
│   ├── claude
│   └── git
├── fixtures/
│   ├── bd_ready.json
│   ├── bd_show.json
│   ├── bd_show_parent.json
│   ├── claude_build_success.txt
│   ├── claude_build_fail.txt
│   ├── claude_validate_success.txt
│   └── claude_validate_fail.txt
└── contracts/
    ├── bd_contract_test.go
    ├── claude_contract_test.go
    └── git_contract_test.go
```

Contract tests build the real gromit binary, prepend `test/fakes/` to `PATH`, run a gromit command, then assert the arguments gromit passed to each external tool and that gromit correctly parsed the canned responses.

Runs with `go test ./test/contracts/`.

### Layer 3: E2E Smoke Tests

Workflow-based tests that run realistic multi-step scenarios against the real binary with fake CLIs. Behind a build tag so they don't run on every `go test ./...`.

`TestMain` builds the gromit binary, creates a temp directory with a git repo, and runs `gromit init` to scaffold the project. Each test gets a fresh copy of this scaffolded directory.

Scenarios:
1. **Happy path** — init, create beads via fake bd state, `gromit run`, verify beads closed, commits made, logs written
2. **Escalation** — haiku build fails, verify sonnet retry triggered with correct arguments
3. **Time budget** — `gromit run --time-budget 1`, verify graceful stop after budget exceeded
4. **Validation failure** — claude build succeeds but validation fails, verify failure analysis runs
5. **Empty queue** — no ready beads, verify clean exit with appropriate message
6. **Review flow** — `gromit review`, verify review prompt rendered and output captured
7. **Status/board/queue output** — verify human-readable output formats are stable

File structure:
```
test/
└── e2e/
    ├── e2e_test.go
    └── testdata/
        └── gromit.yaml
```

Runs with `go test ./test/e2e/ -tags=e2e`.

## Acceptance Criteria

- `go test ./cmd/gromit/ -run TestCLIContract` passes and covers all 11 subcommands plus the root command
- Golden files exist for every subcommand's help output
- Flag contracts exist for every subcommand declaring expected flags, types, and defaults
- `-update` flag regenerates golden files without failing
- Fake `bd` script is stateful: tracks beads, supports `ready`, `show`, `close`, `create`, and `comment` subcommands
- Fake `claude` script returns configurable canned output and supports failure simulation
- Fake `git` script passes through to real git while recording invocations
- Contract tests verify the exact arguments gromit passes to `bd`, `claude`, and `git` for a single-bead `gromit run`
- E2E happy path test runs `gromit init` + `gromit run` with fake CLIs and verifies beads are closed
- E2E escalation test verifies the haiku-to-sonnet retry chain
- E2E empty queue test verifies clean exit
- All new tests pass in CI

## Decisions

1. **Three layers, not one.** Contract tests are fast and catch flag drift. Fake CLIs catch argument regressions. E2E catches wiring bugs. Each layer has a distinct purpose.

2. **Golden files for help text, Go structs for flags.** Help text changes are cosmetic and should be easy to update. Flag changes are breaking and should require explicit code changes.

3. **Stateful bd fake over stateless.** Multi-bead processing is core to gromit's loop. A stateless fake can't test the flow where closing bead 1 makes bead 2 ready.

4. **Git passthrough over full fake.** Git operations are complex and faking them fully is fragile. Passthrough with recording gives real git behavior plus invocation verification.

5. **Build tag for E2E tests.** They're slower and require the full binary build. `go test ./...` should stay fast for development. E2E runs separately in CI and before pushing.

6. **Test directory at repo root, not under internal/.** These tests cross package boundaries and test the whole system. They don't belong to any single internal package.

7. **Shell scripts for fakes, not Go binaries.** Simpler to write, easier to read, trivial to modify. The fakes don't need Go's type system — they just need to echo the right output for the right input.

## Research & Context

### Current State

- 519 unit and integration tests across 24 test files, all passing
- Integration tests in `internal/runner/integration_test.go` use Go-level mocks (mock structs implementing interfaces), not real binaries
- Zero tests in `cmd/gromit/` — the Cobra command wiring is untested
- External tool interactions (`bd`, `claude`, `git`) are mocked at the Go interface level but never tested at the process boundary
- No golden files, no test fixtures directory, no E2E harness

### External Tool Interfaces to Freeze

- `bd ready --json --limit 1` — returns next unblocked bead as JSON
- `bd show <id> --json` — returns bead details with parent info
- `bd close <id>` — marks bead complete
- `bd create` — creates new beads
- `bd comment <id> <message>` — adds comment to bead
- `claude -p <prompt> --model <model> --output-format stream-json` — runs Claude non-interactively
- `git add`, `git commit`, `git diff`, `git status` — standard git operations
