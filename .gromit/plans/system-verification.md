---
created: 2026-02-06T00:00:00Z
decomposed: true
decomposed_at: "2026-02-06T22:43:29-05:00"
id: system-verification
source_spec: system-verification
---

# System Verification Implementation Plan

**Goal:** Add three layers of system-level tests — CLI contract tests, external tool contract tests with fake CLIs, and E2E smoke tests — to catch regressions that the existing 519 unit/integration tests miss.

**Architecture:** Build the real gromit binary in TestMain, test the CLI surface with golden files and flag structs, intercept external tool calls with shell script fakes via PATH manipulation, and run multi-step workflow scenarios behind a build tag.

**Tech Stack:** Go testing, shell scripts (bash), Cobra CLI framework

**Spec:** `.gromit/specs/system-verification.md`

---

## Architecture

### Layer 1: CLI Contract Tests (`cmd/gromit/cli_contract_test.go`)

A single test file that freezes the CLI interface. `TestMain` builds the real `gromit` binary once into a temp directory. Table-driven subtests cover every command:

- **Help text snapshots** — `gromit <cmd> --help` output compared against golden files in `cmd/gromit/testdata/golden/`. A `-update` test flag regenerates them.
- **Flag contracts** — Go structs declaring expected flags with name, type, default, and shorthand. Tests iterate the struct and assert each flag exists with the right properties. Adding or removing a flag breaks the test.
- **Exit code assertions** — Commands with bad input (missing required args, invalid flags) assert specific non-zero exit codes.

Commands to cover (14 total): root, run, init, status, retro, add, backlog, backlog-delete, board, queue, triage, refine, plan, review, decompose.

### Layer 2: External Tool Contract Tests

Shell script fakes in `test/fakes/` intercept `bd`, `claude`, and `git` via PATH prepending.

**Fake bd** (stateful): Keeps bead state in `$TEST_DIR/.fake_bd_state.json`. Supports `ready`, `show`, `close`, `create`, `comment`, `comments`, `list`, `sync` subcommands. `ready` returns the next unclosed bead. `close <id>` marks it closed. `BD_FAIL=1` triggers errors. All invocations recorded to `$TEST_CALL_LOG`.

**Fake claude** (canned): Returns content from fixture file specified by `$CLAUDE_FIXTURE` env var. `CLAUDE_FAIL=1` triggers non-zero exit. Records invocations including all arguments to `$TEST_CALL_LOG`.

**Fake git** (passthrough): Calls real git (found via `$REAL_GIT` env var, resolved before PATH manipulation) but records all invocations to `$TEST_CALL_LOG`.

Contract tests in `test/contracts/` build the binary, set up fakes on PATH, run gromit commands, then assert:
- The exact arguments gromit passed to each tool
- That gromit correctly parsed the canned responses
- That gromit handled error responses appropriately

### Layer 3: E2E Smoke Tests (`test/e2e/`)

Behind `//go:build e2e`. `TestMain` builds the binary, creates a temp directory with `git init` + `gromit init` to scaffold a project. Each test gets a copy of this scaffolded directory with fakes on PATH.

Scenarios: happy path (multi-bead run), escalation (haiku→sonnet retry), time budget, validation failure, empty queue, review flow, status/board/queue output.

### File Structure

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
        ├── backlog-delete.help.txt
        ├── board.help.txt
        ├── queue.help.txt
        ├── triage.help.txt
        ├── refine.help.txt
        ├── plan.help.txt
        ├── review.help.txt
        └── decompose.help.txt
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
├── contracts/
│   ├── helpers_test.go
│   ├── bd_contract_test.go
│   ├── claude_contract_test.go
│   └── git_contract_test.go
└── e2e/
    ├── e2e_test.go
    └── testdata/
        └── gromit.yaml
```

## Test Strategy

**Layer 1** runs with `go test ./cmd/gromit/ -run TestCLIContract` — fast, no external deps, < 2s.

**Layer 2** runs with `go test ./test/contracts/` — needs built binary + fakes, < 10s.

**Layer 3** runs with `go test ./test/e2e/ -tags=e2e` — needs binary + fakes + real git, < 30s.

**Mocking:** No Go-level mocks. All three layers test the real compiled binary as a subprocess. External tools are faked at the process boundary with shell scripts and PATH manipulation. Git is real (passthrough) in a temp repo.

**Fixture data:** Bead fixtures match the `bead.Bead` JSON struct (`id`, `title`, `description`, `priority`, `labels`, `parent`, `issue_type`, `status`, `owner`, `expected_outputs`). Claude fixtures match the output formats gromit parses: stream-json events for `StreamRun`, plain text for `Run`.

**Coverage goals:** Every subcommand's help text and flags frozen. Every bd subcommand gromit uses has argument verification. Every claude invocation mode has argument verification. Core workflows (happy path, escalation, empty queue) tested end-to-end.

## Implementation Tasks

### Task 1: CLI Contract Test Infrastructure + Golden Files

**Files:**
- Create: `cmd/gromit/cli_contract_test.go`
- Create: `cmd/gromit/testdata/golden/*.help.txt` (15 files, generated)

**What to Do:**

Create `cli_contract_test.go` with:

1. A package-level `var binaryPath string` to hold the built binary location.
2. A `var update = flag.Bool("update", false, "update golden files")` flag.
3. `TestMain(m *testing.M)` that builds `./cmd/gromit` to a temp directory via `go build -o <path> ./cmd/gromit`, stores the path in `binaryPath`, runs `m.Run()`, then cleans up.
4. A `runGromit(t *testing.T, args ...string) (stdout string, stderr string, exitCode int)` helper that runs the binary with the given args and captures output.
5. A `TestCLIContract_HelpText(t *testing.T)` function with a table of all 15 commands. Each entry has the command name and the args to invoke help (e.g., `[]string{"run", "--help"}`). For each: run the command, compare stdout against `testdata/golden/<name>.help.txt`. If `-update` is set, write the output to the golden file instead of comparing. Use `t.Run(name, ...)` for subtests.
6. A `goldenPath(name string) string` helper that returns the golden file path.

Run the test once with `-update` to generate all golden files, then verify it passes without `-update`.

**Acceptance Criteria:**
- `TestMain` builds the binary and tests can invoke it
- `go test ./cmd/gromit/ -run TestCLIContract_HelpText -update` generates 15 golden files
- `go test ./cmd/gromit/ -run TestCLIContract_HelpText` passes by comparing against golden files

**Dependencies:** None

**Notes:**
- The 15 commands are: root (no subcommand), run, init, status, retro, add, backlog, `backlog delete`, board, queue, triage, refine, plan, review, decompose
- For `backlog delete`, use args `[]string{"backlog", "delete", "--help"}` and golden file name `backlog-delete.help.txt`
- Root help uses just `[]string{"--help"}`

---

### Task 2: Flag Contracts + Exit Code Tests

**Files:**
- Modify: `cmd/gromit/cli_contract_test.go`

**What to Do:**

Add to the existing test file:

1. A `flagContract` struct type: `type flagContract struct { Name, Shorthand, Type, Default string }`.
2. A `TestCLIContract_Flags(t *testing.T)` function with a table mapping command name to `[]flagContract`. Each entry declares the expected flags for that command. Example entries:
   - `"root"`: `{Name: "config", Shorthand: "c", Type: "string", Default: "gromit.yaml"}`
   - `"run"`: `{Name: "max-iterations", Shorthand: "n", Type: "int", Default: "0"}`, `{Name: "dry-run", Type: "bool", Default: "false"}`, `{Name: "time-budget", Shorthand: "t", Type: "int", Default: "0"}`, `{Name: "time-budget-hours", Shorthand: "H", Type: "int", Default: "0"}`
   - `"init"`: `{Name: "force", Shorthand: "f", Type: "bool", Default: "false"}`
   - `"decompose"`: `{Name: "review", Type: "bool", Default: "false"}`, `{Name: "force", Type: "bool", Default: "false"}`
   - `"plan"`: `{Name: "force", Type: "bool", Default: "false"}`
   - `"review"`: `{Name: "non-interactive", Type: "bool", Default: "false"}`, `{Name: "since", Type: "string", Default: ""}`, `{Name: "epic", Type: "string", Default: ""}`, `{Name: "dry-run", Type: "bool", Default: "false"}`
   - `"retro"`: `{Name: "non-interactive", Type: "bool", Default: "false"}`
   - `"backlog"`: `{Name: "type", Type: "string", Default: ""}`, `{Name: "recent", Type: "int", Default: "0"}`
   - Commands with no flags (`add`, `board`, `queue`, `triage`, `refine`, `backlog-delete`, `status`): empty slice
3. The test parses help text output to extract flags and asserts they match the struct. Alternatively, use `gromit <cmd> --help` and parse the Cobra flags section. The simplest approach: assert that each declared flag name appears in the help text, and that no unexpected flags appear (compare the set of flag names in help output against the struct).
4. A `TestCLIContract_ExitCodes(t *testing.T)` function with table-driven cases:
   - `gromit add` (missing required arg) → non-zero exit
   - `gromit decompose` (missing required arg) → non-zero exit
   - `gromit run --max-iterations -1` (invalid value) → non-zero exit
   - `gromit --help` → exit 0
   - `gromit run --help` → exit 0

**Acceptance Criteria:**
- Flag contract struct exists for every command with all expected flags declared
- Test fails if a flag is added to or removed from a command without updating the struct
- Exit code tests verify non-zero exits for bad input and zero exits for --help

**Dependencies:** Task 1

**Notes:**
- Hidden flags (like `--no-chain` on decompose and plan, per open beads) should be excluded from the contract — they're hidden for a reason and may be transient. Only test visible flags.
- Parse flags from help text by looking for lines starting with `  -` or `      --` in Cobra's help format.
- The flag type can be inferred from the default value format in help text, or from the flag usage line.

---

### Task 3: Fake BD Script + BD Fixtures

**Files:**
- Create: `test/fakes/bd`
- Create: `test/fixtures/bd_ready.json`
- Create: `test/fixtures/bd_show.json`
- Create: `test/fixtures/bd_show_parent.json`

**What to Do:**

Create `test/fakes/bd` as an executable bash script that:

1. **Records invocations**: Appends `"bd $*"` to `$TEST_CALL_LOG` (one line per invocation).
2. **Checks for failure mode**: If `$BD_FAIL` is set, prints an error message to stderr and exits with code 1.
3. **Routes by subcommand** (first positional arg):
   - `ready`: Read `$TEST_DIR/.fake_bd_state.json`. Find the first bead with `status != "closed"`. Output it as a JSON array with one element. If no beads are unclosed, output `[]`.
   - `show <id>`: Read state file, find bead by ID, output as JSON object. If `$BD_SHOW_FIXTURE` is set, cat that file instead (for parent bead lookups).
   - `close <id>`: Read state file, set the matching bead's status to `"closed"`, write state back.
   - `create`: Echo a fake bead ID. If `$BD_CREATE_FIXTURE` is set, use that.
   - `comment <id> <message>`: Record the comment in the state file under the bead. No stdout output needed.
   - `comments <id> --json`: Output `[]` (empty comments array).
   - `list --json`: Output all beads from state file.
   - `sync`: No-op, exit 0.
4. **State file format** (`$TEST_DIR/.fake_bd_state.json`):
   ```json
   [
     {"id": "test-bead-1", "title": "Test bead 1", "priority": 1, "status": "open", ...},
     {"id": "test-bead-2", "title": "Test bead 2", "priority": 1, "status": "open", ...}
   ]
   ```
   This is a JSON array of bead objects matching the `bead.Bead` struct fields.

Create fixture files with realistic bead JSON:

- `bd_ready.json`: Array with one bead (`id: "test-bead-abc1"`, `title: "Implement feature X"`, `priority: 1`, `labels: []`, `status: "open"`, `issue_type: "task"`)
- `bd_show.json`: Single bead object (same bead, with full fields including `description`, `expected_outputs`)
- `bd_show_parent.json`: A bead object representing a parent/epic with `issue_type: "epic"`

**Acceptance Criteria:**
- `bd ready --json --limit 1` returns the next unclosed bead from state
- `bd close <id>` marks the bead closed in state; subsequent `bd ready` skips it
- All invocations are recorded in `$TEST_CALL_LOG`

**Dependencies:** None

**Notes:**
- Use `jq` for JSON manipulation in the script if available, but fall back to simple sed/awk for environments without jq. Actually, prefer Python one-liners or keep it simple with a helper approach — the state file is small.
- The script must be executable (`chmod +x`).
- Bead IDs should use the `[a-zA-Z0-9._-]+` format that passes `validBeadID` regex validation.

---

### Task 4: Fake Claude + Fake Git Scripts + Claude Fixtures

**Files:**
- Create: `test/fakes/claude`
- Create: `test/fakes/git`
- Create: `test/fixtures/claude_build_success.txt`
- Create: `test/fixtures/claude_build_fail.txt`
- Create: `test/fixtures/claude_validate_success.txt`
- Create: `test/fixtures/claude_validate_fail.txt`

**What to Do:**

**Fake claude** (`test/fakes/claude`): Executable bash script that:
1. Records `"claude $*"` to `$TEST_CALL_LOG`.
2. If `$CLAUDE_FAIL` is set, exit with code 1.
3. Reads stdin (gromit pipes the prompt to stdin) and discards it.
4. Checks `$CLAUDE_FIXTURE` env var for the fixture file path to cat. If not set, output a default success message.
5. For stream-json mode (detected by checking if args contain `--output-format stream-json`): output the fixture as stream-json events. For plain mode: output the fixture as plain text.
6. Exit 0 (unless `$CLAUDE_FAIL` is set).

**Fake git** (`test/fakes/git`): Executable bash script that:
1. Records `"git $*"` to `$TEST_CALL_LOG`.
2. Calls `$REAL_GIT "$@"` to pass through to real git. `$REAL_GIT` must be set by the test harness to the absolute path of the real git binary (resolved before PATH manipulation).
3. Exits with real git's exit code.

**Claude fixture files:**
- `claude_build_success.txt`: Stream-json formatted output simulating a successful build. Include `{"type":"assistant","message":{"content":[{"type":"text","text":"Build completed successfully."}]}}` and a result event.
- `claude_build_fail.txt`: Output simulating a failed build with error text.
- `claude_validate_success.txt`: Plain text output containing `VALIDATION_PASSED`.
- `claude_validate_fail.txt`: Plain text output containing `VALIDATION_FAILED` followed by error details.

**Acceptance Criteria:**
- Claude fake returns the fixture specified by `$CLAUDE_FIXTURE` and records args
- Git fake passes through to real git while recording every invocation
- `$CLAUDE_FAIL=1` causes claude fake to exit with code 1

**Dependencies:** None

**Notes:**
- The claude fake needs to handle both `Run` mode (plain text, prompt on stdin) and `StreamRun` mode (stream-json events). Detect mode by checking for `--output-format` in args.
- For stream-json fixtures, look at `internal/claude/claude_test.go` for the expected event format (e.g., `{"type":"assistant","message":{"content":[...]}}`, `{"type":"result",...}`).
- The git fake must resolve `$REAL_GIT` before the test prepends fakes to PATH. The test harness sets this via `REAL_GIT=$(which git)` before modifying PATH.

---

### Task 5: Contract Test Infrastructure + BD Contract Tests

**Files:**
- Create: `test/contracts/helpers_test.go`
- Create: `test/contracts/bd_contract_test.go`

**What to Do:**

**helpers_test.go**: Package `contracts` test infrastructure:
1. `var binaryPath string` — path to built gromit binary.
2. `TestMain(m *testing.M)` — builds `./cmd/gromit` to a temp dir, resolves real git path via `exec.LookPath("git")`, stores both in package vars, runs `m.Run()`, cleans up.
3. `setupTestEnv(t *testing.T) (testDir string, callLog string, env []string)` helper:
   - Creates `t.TempDir()` for the test.
   - Creates a `TEST_CALL_LOG` file path in the temp dir.
   - Initializes a git repo in the temp dir (`git init`, initial commit).
   - Creates a minimal `gromit.yaml` in the temp dir.
   - Runs `gromit init` in the temp dir to scaffold `.gromit/`.
   - Builds environment slice: `PATH=<repo>/test/fakes:$PATH`, `TEST_DIR=<testDir>`, `TEST_CALL_LOG=<logFile>`, `REAL_GIT=<realGitPath>`, plus inherited env vars.
   - Returns testDir, callLog path, and env slice.
4. `runGromitWithEnv(t *testing.T, env []string, dir string, args ...string) (stdout, stderr string, exitCode int)` — runs the gromit binary with the given environment and working directory.
5. `readCallLog(t *testing.T, path string) []string` — reads the call log file and returns lines.
6. `filterCalls(lines []string, prefix string) []string` — filters call log lines by prefix (e.g., `"bd"`, `"claude"`, `"git"`).

**bd_contract_test.go**: Tests that verify gromit passes correct arguments to bd:
1. `TestBDContract_ReadyArgs(t *testing.T)`: Set up a test env with one bead in the fake bd state. Run `gromit run -n 1`. Assert the call log contains `bd ready --json --limit 1`.
2. `TestBDContract_ShowArgs(t *testing.T)`: Same setup. After run, assert call log contains `bd show test-bead-abc1 --json` (the ID from the ready response).
3. `TestBDContract_CloseArgs(t *testing.T)`: Same setup with a successful claude fixture. After run, assert call log contains `bd close test-bead-abc1`.
4. `TestBDContract_SyncArgs(t *testing.T)`: Assert call log contains `bd sync` at the start of the run.

**Acceptance Criteria:**
- TestMain builds the binary and setupTestEnv scaffolds a working test environment
- bd ready/show/close argument assertions pass for a single-bead gromit run
- Call log correctly captures all bd invocations in order

**Dependencies:** Task 3 (fake bd script + fixtures), Task 4 (fake claude + git scripts)

**Notes:**
- The gromit binary needs a valid `gromit.yaml` in the working directory. The setup helper must create one with appropriate config (models, validation commands, etc.).
- The fake bd state file must be pre-populated with test beads before running gromit.
- Set `CLAUDE_FIXTURE` to the build success fixture and validation success fixture paths so the full build+validate+close flow completes.

---

### Task 6: Claude + Git Contract Tests

**Files:**
- Create: `test/contracts/claude_contract_test.go`
- Create: `test/contracts/git_contract_test.go`

**What to Do:**

**claude_contract_test.go**: Tests that verify gromit passes correct arguments to claude:
1. `TestClaudeContract_BuildArgs(t *testing.T)`: Set up test env with one bead. Run `gromit run -n 1`. Filter call log for `claude` lines. Assert the build invocation includes `-p`, `--model`, `--output-format stream-json`, `--verbose`.
2. `TestClaudeContract_ValidateArgs(t *testing.T)`: Same setup with validation enabled in config. Assert a second claude invocation exists with `-p`, `--model haiku` (validation model).
3. `TestClaudeContract_BuildFailureTriggersEscalation(t *testing.T)`: Set `CLAUDE_FAIL=1` for the first invocation (or use a fixture that simulates failure). Assert that a second claude call appears with the escalated model.

**git_contract_test.go**: Tests that verify gromit runs expected git commands:
1. `TestGitContract_RevParseBeforeBuild(t *testing.T)`: Run `gromit run -n 1` with successful fakes. Filter call log for `git` lines. Assert `git rev-parse HEAD` appears before the build.
2. `TestGitContract_DiffAfterBuild(t *testing.T)`: Same setup. Assert `git diff --stat` and `git diff` appear after the build phase.

**Acceptance Criteria:**
- Claude build invocation arguments match expected format (flags, model, output format)
- Claude validation invocation uses the configured validation model
- Git rev-parse and diff invocations are verified in the call log

**Dependencies:** Task 5 (shared helpers_test.go infrastructure)

**Notes:**
- For the escalation test, you may need to control the claude fake's behavior per-invocation. One approach: have the fake check a counter file (increment on each call) and fail on call N=1 but succeed on call N=2. Alternatively, use `CLAUDE_FAIL_ONCE=1` that auto-clears after the first failure.
- The exact model names in claude args depend on the gromit.yaml config. Use the defaults: P1→sonnet for build, haiku for validation.

---

### Task 7: E2E Infrastructure + Happy Path + Empty Queue

**Files:**
- Create: `test/e2e/e2e_test.go`
- Create: `test/e2e/testdata/gromit.yaml`

**What to Do:**

Create `e2e_test.go` with `//go:build e2e` tag:

1. Package-level vars: `binaryPath`, `realGitPath`, `scaffoldDir` (the template scaffolded directory).
2. `TestMain(m *testing.M)`: Build gromit binary. Resolve real git path. Create a temp directory, init a git repo, copy `test/e2e/testdata/gromit.yaml` into it, run `gromit init` to scaffold. Store as `scaffoldDir`. Run `m.Run()`. Clean up.
3. `copyScaffold(t *testing.T) string` helper: Copies `scaffoldDir` to a fresh `t.TempDir()` so each test gets isolation.
4. `setupE2E(t *testing.T) (testDir, callLog string, runGromit func(args ...string) (string, string, int))` helper: Calls `copyScaffold`, sets up environment with fakes on PATH, returns a convenience function for running gromit.
5. `TestE2E_HappyPath(t *testing.T)`:
   - Create fake bd state with 2 beads.
   - Set `CLAUDE_FIXTURE` to build success + validate success.
   - Run `gromit run`.
   - Assert: both beads are closed in the state file, call log shows 2x (ready→show→build→validate→close) sequences, exit code 0.
6. `TestE2E_EmptyQueue(t *testing.T)`:
   - Create fake bd state with no beads (empty array).
   - Run `gromit run`.
   - Assert: exit code 0, stdout contains a message about no work/no beads, no claude invocations in call log.

Create `test/e2e/testdata/gromit.yaml` with minimal config:
- Models: haiku for all priorities (fast), haiku for validation
- Escalation: enabled, chain [haiku, sonnet]
- Validation: enabled, commands ["go test ./..."]
- Loop: maxIterations 10, stopOnFailure false
- Paths: defaults

**Acceptance Criteria:**
- TestMain builds binary, scaffolds a template directory with gromit init
- Happy path test closes 2 beads and verifies the full ready→build→validate→close cycle
- Empty queue test exits cleanly with no claude invocations

**Dependencies:** Task 3 (fake bd), Task 4 (fake claude + git)

**Notes:**
- The scaffolded directory needs a real git repo (so git passthrough works) with at least one initial commit.
- The `gromit.yaml` config should use short timeouts for claude to keep tests fast.
- For the happy path, the claude fake needs to produce output that makes gromit consider the build successful. Check what `runner.go` looks for in the claude output (likely checks `Result.Success` which is based on exit code 0).

---

### Task 8: E2E Escalation + Validation Failure + Time Budget Scenarios

**Files:**
- Modify: `test/e2e/e2e_test.go`

**What to Do:**

Add three more test functions:

1. `TestE2E_Escalation(t *testing.T)`:
   - Create fake bd state with 1 bead.
   - Configure claude fake to fail on first invocation, succeed on second. Use a counter-file approach: the claude fake increments a counter in `$TEST_DIR/.claude_call_count` and checks `CLAUDE_FAIL_UNTIL` env var (fail if counter <= N).
   - Run `gromit run -n 1`.
   - Assert: call log shows two claude build invocations — first with the initial model (haiku), second with escalated model (sonnet). Bead is eventually closed.

2. `TestE2E_ValidationFailure(t *testing.T)`:
   - Create fake bd state with 1 bead.
   - Set build fixture to success, validation fixture to failure (`VALIDATION_FAILED`).
   - Run `gromit run -n 1`.
   - Assert: call log shows build claude call then validation claude call. Bead is NOT closed (validation failed). Exit code reflects the failure mode (depends on `stopOnFailure` config).

3. `TestE2E_TimeBudget(t *testing.T)`:
   - Create fake bd state with 5 beads.
   - Set build + validate fixtures to success.
   - Configure claude fake to sleep for 2 seconds per invocation (`CLAUDE_DELAY=2`).
   - Run `gromit run --time-budget 1` (1 minute budget, but each bead takes ~4s for build+validate, so it should process some but not all).
   - Assert: fewer than 5 beads processed, gromit exited cleanly (exit 0), stdout mentions time budget.

**Acceptance Criteria:**
- Escalation test verifies the haiku→sonnet model upgrade in claude args
- Validation failure test verifies bead stays open when validation fails
- Time budget test verifies gromit stops processing before exhausting all beads

**Dependencies:** Task 7 (E2E infrastructure + helpers)

**Notes:**
- The escalation test requires the claude fake to support per-invocation behavior. Add a `CLAUDE_FAIL_UNTIL=N` env var that causes failures for the first N calls, tracked via a file counter in `$TEST_DIR`.
- The time budget test is inherently timing-sensitive. Use generous margins — the point is that gromit stops before processing all 5 beads, not that it processes exactly N.
- The validation failure flow depends on how `runner.go` handles it. Based on the integration tests, validation failure means the bead stays open and the runner may continue to the next bead or stop (depending on `stopOnFailure`).

---

## Notes

- **Go module path**: Contract and E2E test packages need to be under the module's go.mod. Since the module is `github.com/danabrams/gromit`, the test packages will be `github.com/danabrams/gromit/test/contracts` and `github.com/danabrams/gromit/test/e2e`. No go.mod changes needed — they're subdirectories of the module root.
- **CI integration**: Layer 1 and 2 run with normal `go test ./...`. Layer 3 needs a separate CI step: `go test ./test/e2e/ -tags=e2e -timeout 120s`. Note: Layer 2 contract tests will also run with `go test ./...` since they have no build tag — consider whether this is desired or if they should also be behind a build tag. The spec says only E2E is tagged.
- **Fixture maintenance**: When bd or claude output formats change, fixtures must be updated. Keep fixtures minimal — only the fields gromit actually parses.
- **Platform**: Shell scripts assume bash. This is fine for Linux/macOS CI. Windows support is not a concern for gromit.
- **Hidden flags**: The open beads show `--no-chain` being added to decompose and plan as hidden flags. These should NOT be included in the flag contract structs — hidden flags are internal and may change. Only test visible flags.
