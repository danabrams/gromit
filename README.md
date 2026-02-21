# Gromit

**Autonomous task execution for AI coding agents, with built-in learning.**

Gromit processes a queue of coding tasks, handing each one to a configured provider with fresh context, validating the results independently, and escalating on failure. Along the way, it builds a persistent knowledge base of what works and what doesn't -- so each iteration gets smarter than the last.

## Why Gromit?

Long-running agent sessions drift. Context bloats, the model latches onto stale assumptions, and partial attempts pollute the next attempt. Validation is inconsistent or optimistic.

Gromit fixes this:

- **Fresh context every iteration** -- each task gets a new provider process with exactly the context it needs, nothing more
- **Independent validation** -- tests and lints run in a separate step, not by the build model
- **Automatic failure analysis** -- when something breaks, Gromit figures out why and decides whether to retry, escalate, or skip
- **Persistent learnings** -- mistakes become knowledge that feeds into future prompts
- **Model escalation** -- starts cheap, upgrades only when needed

Named after [Gromit](https://en.wikipedia.org/wiki/Wallace_and_Gromit) -- the silent, competent one who actually makes everything work while Wallace tinkers with the grand designs. You bring the ideas, Gromit makes sure they don't end up like one of Wallace's contraptions.

## Quick Start

### Install

```bash
go install github.com/danabrams/gromit/cmd/gromit@latest
```

Or build from source:

```bash
git clone https://github.com/danabrams/gromit.git
cd gromit
go build -o gromit ./cmd/gromit
```

### Initialize Your Project

```bash
cd your-project
gromit init
```

This creates `gromit.yaml` (configuration) and a `.gromit/` directory with prompt templates, a rules file, a learnings file, and a logs directory. Edit `gromit.yaml` to set your validation commands (tests, linting, build).

### Add Work

Capture ideas to the backlog:

```bash
gromit add "Add user authentication"
gromit add "Fix the date formatting bug"
gromit add "Refactor database layer"
```

Then move them through the pipeline:

```bash
gromit refine              # Turn ideas into structured specs (interactive)
gromit plan <spec-name>    # Create an implementation plan from a spec (interactive)
gromit decompose <plan>    # Break the plan into executable tasks (automatic)
```

### Run

```bash
gromit run                       # Process all tasks until queue is empty
gromit run -n 5                  # Process at most 5 tasks
gromit run --time-budget 30      # Run for at most 30 minutes
gromit run --time-budget-hours 2 # Run for at most 2 hours
gromit run --dry-run             # Preview what would run, without executing
gromit status                    # Show the next task and which model it would use
```

### Linting And Hooks

Gromit pins `golangci-lint` in `.golangci-version` and enforces that version in `make lint`.

```bash
GOBIN="$(go env GOPATH)/bin" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(cat .golangci-version)
make lint
make install-hooks
```

`make install-hooks` configures `core.hooksPath=.githooks`, so pre-commit runs `make lint`.

### Test Loops

```bash
make test-touched  # Run go test only for packages touched in current git diff
make test-timing   # Run go test -json, print slowest tests/packages, enforce package budgets
make test-parallel-safe-top5  # Guard shared-state calls and run top-5 packages in shuffled parallel mode
```

`make test-timing` uses `scripts/test_package_budgets.txt` and defaults to a 45s budget for packages without an explicit override.

### Test Tiers

```bash
make test-unit        # Fast deterministic tests (default go test tags)
make test-acceptance  # Deterministic acceptance tests (-tags acceptance)
make test-contract    # Tagged contract harness suite (-tags=contract ./test/contracts)
make test-e2e         # Tagged e2e harness suite (-tags=e2e ./test/e2e)
make test-tagged-harness # Fail-fast smoke command that runs contract + e2e tagged harness suites
make test-e2e-live    # Live external integration tests in e2e_live-tagged packages only
```

Run `test-e2e-live` for integration validation (release checks, integration changes, nightly), not on every local edit loop.

### Example Output

```
Gromit v0.1
  Config: gromit.yaml
  Streaming log: .gromit/logs/stream-20260205-143022.log

[1] Processing: Add user authentication (abc-123)
  Model: opus (P0)
  Building...
  [1m elapsed] Still working. 4 files modified, 12 tool calls.
  [2m elapsed] Still working. 7 files modified, 28 tool calls.
  Build complete (142s)
  Validating...
  Validation passed
  Completed abc-123

[2] Processing: Refactor database layer (def-456)
  Model: sonnet (P1)
  Building...
  Build complete (87s)
  Validating...
  Validation passed
  Completed def-456

No more tasks ready. 2 iterations completed.
```

## Core Concepts

| Concept | What it is |
|---------|------------|
| **Task** | A unit of work Gromit executes -- one focused change with clear acceptance criteria |
| **Spec** | A structured description of a feature: overview, constraints, acceptance criteria |
| **Plan** | An implementation strategy breaking a spec into ordered tasks with dependencies |
| **Rules** | Non-negotiable project constraints, always included in every prompt |
| **Learnings** | Facts discovered while working in the repo -- accumulated automatically, reviewed by humans |
| **Validation** | Your test/lint/build commands, run independently after each task |
| **Escalation** | Automatic upgrade to a stronger model when a task fails |
| **Retro** | Periodic review of accumulated learnings to consolidate knowledge and update rules |

## The Pipeline

Gromit separates **planning** (human-guided) from **execution** (autonomous):

```
 Capture        Refine         Plan          Decompose       Execute
┌────────┐   ┌──────────┐   ┌───────────┐   ┌───────────┐   ┌─────────┐
│ Ideas  │──▶│  Specs   │──▶│  Plans    │──▶│  Tasks    │──▶│  Run    │
│ (rough)│   │(structured)  │(ordered)  │   │(executable)   │(auto)   │
└────────┘   └──────────┘   └───────────┘   └───────────┘   └─────────┘
 gromit add   gromit refine  gromit plan     gromit decompose gromit run
```

**Capture** -- `gromit add "idea"` saves rough ideas to the backlog. No structure needed.

**Refine** -- `gromit refine` launches an interactive agent session to turn an idea into a proper spec with acceptance criteria. Specs are saved to `.gromit/specs/`.

**Plan** -- `gromit plan <spec>` creates an implementation plan: what to build, in what order, with what dependencies. Plans are saved to `.gromit/plans/`.

**Decompose** -- `gromit decompose <plan>` reads the plan and automatically creates executable tasks with priorities, dependencies, and acceptance criteria.

**Execute** -- `gromit run` processes the task queue autonomously. Each task gets fresh context, independent validation, and failure handling.

For simple projects, you can also manage tasks directly and skip straight to `gromit run`.

### Interactive Agent Selection

Interactive commands (`refine`, `plan`, `review`, `explore`, `debug`) support agent selection by CLI flag or config phase defaults.

```bash
# One-off override for a debug session
gromit debug --agent codex "Cache miss flapping in production"
```

```yaml
agents:
  phases:
    debug: codex
```

Use `--choose-agent` on interactive commands to open a picker at launch time.

### Non-Interactive Review Provider Selection

`gromit review --non-interactive` uses provider routing (`providers` + `routing.phase_preferences.review`), not `agents.phases.review`.

```yaml
providers:
  claude:
    binary: claude
    models: {high: opus, medium: sonnet, low: haiku}
  openai:
    binary: codex
    models: {high: gpt-5.3-codex, medium: gpt-5.3-codex, low: gpt-5.3-codex}

routing:
  phase_preferences:
    review: openai   # force Codex for non-interactive review

review:
  thorough:
    model: sonnet    # tier hint: low|medium|high by legacy model alias

agents:
  phases:
    review: claude   # interactive `gromit review` default only
```

## How It Works

### The Execution Loop

```
                         ┌─────────────┐
                         │  Get next   │ ── highest priority, unblocked
                         │    task     │
                         └──────┬──────┘
                                │
                         ┌──────▼──────┐
                         │ Select Model│ ── P0→opus, P1→sonnet, P2→haiku
                         └──────┬──────┘
                                │
                    ┌───────────▼───────────┐
                    │   Agent CLI (fresh)   │ ── new process, full context
                    │  + rules + learnings  │
                    └───────────┬───────────┘
                                │
                         ┌──────▼──────┐
                    ┌────│  Validate   │────┐
                    │    └─────────────┘    │
                  pass                    fail
                    │                      │
             ┌──────▼──────┐     ┌─────────▼─────────┐
             │  Complete   │     │ Analyze + Escalate │
             │  task       │     └─────────┬─────────┘
             └─────────────┘               │
                                    ┌──────▼──────┐
                                    │   Learning  │ ── extract insight
                                    │   recorded  │    for next time
                                    └─────────────┘
```

For each task in the queue:

1. **Get next task** -- returns the highest-priority unblocked task
2. **Select model** -- based on priority and labels (see [Model Selection](#model-selection))
3. **Build prompt** -- assembles rules, learnings, spec, task details, and any failure context
4. **Run provider** -- fresh process with full context, streamed output
5. **Validate** -- runs your test/lint commands via a separate haiku invocation
6. **On success** -- closes the task, moves to next
7. **On failure** -- analyzes the failure, extracts a learning, and either retries or escalates

### Model Selection

Priority determines the default model:

| Priority | Model | Use For |
|----------|-------|---------|
| P0 | opus | Critical, complex tasks |
| P1 | sonnet | Normal tasks |
| P2 | haiku | Simple, low-risk tasks |

Complexity labels override priority -- a P2 task labeled `complexity:high` gets opus, a P0 labeled `complexity:low` gets haiku. Validation always uses haiku for cost efficiency.

### Escalation

When a task fails and the failure isn't recoverable with the current model, Gromit escalates:

```
haiku (failed) → sonnet (retry) → opus (retry) → give up
```

The prompt for retries includes the previous failure output and analysis, so the stronger model knows what went wrong and can try a different approach.

### Fresh Context

This is the key design decision. Each provider invocation is a **new process**. There's no conversation history carrying forward, no confused context from previous iterations.

State lives in files:

- **Git commits** -- the actual code changes
- **Task queue** -- what to do next and what's done
- **LEARNINGS.md** -- accumulated knowledge
- **RULES.md** -- project constraints

The prompt template assembles everything the active provider needs for each task, from scratch, every time.

## Self-Improvement

Gromit doesn't just execute tasks -- it learns from them.

### Learnings

When a task fails, Gromit runs failure analysis and extracts generalizable insights. These go into `.gromit/LEARNINGS.md` with two tiers:

**Provisional** -- seen once, might be specific to one task:
```markdown
### 2026-02-05 | abc-123 | gotchas
Validation failures due to missing tools are environment issues, not code issues.
Check tool availability before assuming code is broken.
```

**Confirmed** -- seen multiple times, high confidence:
```markdown
### conventions
Before implementing, always verify actual file and code state. Explore existing
commands to understand patterns before building new ones.
```

When a new learning matches an existing provisional one, it gets promoted to confirmed. Confirmed learnings are included in every build prompt.

### Rules

`.gromit/RULES.md` contains non-negotiable project constraints. Rules are always included at the top of every prompt.

```markdown
# Rules

## Code Style
- This is a Go project - use idiomatic Go patterns
- Use `fmt.Errorf("context: %w", err)` for error wrapping

## Safety
- Never commit secrets or API keys
- Always handle errors - no silent failures
```

### Retrospective

Periodically, consolidate what Gromit has learned:

```bash
gromit retro
```

This invokes opus to analyze all accumulated learnings and:

- **Merge** related or duplicate learnings
- **Promote** patterns to rules (when warranted)
- **Archive** stale or obsolete learnings
- **Suggest** rule changes

By default, retro runs interactively -- it launches an agent session so you can discuss and apply changes together. Use `gromit retro --non-interactive` to write proposals to a file instead.

Gromit also tells you when it's time for a retro. At the end of each run, it checks for conditions like many unreviewed learnings, high failure rates, or time since last retro, and suggests running one.

## Configuration

### gromit.yaml

After `gromit init`, edit `gromit.yaml` for your project. The key sections:

```yaml
# Model selection by task priority
models:
  p0: opus                     # Critical tasks → strongest model
  p1: sonnet                   # Normal tasks → balanced
  p2: haiku                    # Simple tasks → fastest/cheapest
  validation: haiku            # Validation always uses haiku
  labels:                      # Label overrides (beat priority)
    "complexity:high": opus
    "complexity:low": haiku

# Escalation on failure
escalation:
  enabled: true
  chain: [haiku, sonnet, opus]
  max_retries_per_model: 1

# Loop behavior
loop:
  max_iterations: 0            # 0 = unlimited
  stop_on_failure: false       # true = stop on first failure
  stuck_bead_threshold: 3      # Skip task if it fails this many times

# Validation commands -- customize for your project
validation:
  enabled: true
  commands:
    - "go test ./..."
    - "go vet ./..."
    - "go build ./cmd/myapp"

# Provider CLI settings (legacy-compatible `claude` key)
claude:
  timeout: 600                 # Seconds per invocation
  stall_timeout: 120           # Seconds of silence before auto-retry
  flags:
    - "--dangerously-skip-permissions"
```

### Validation Commands

Customize the `validation.commands` list for your stack:

```yaml
# Node/TypeScript
validation:
  commands:
    - "pnpm run test"
    - "pnpm run lint:check"
    - "pnpm run build"

# Go
validation:
  commands:
    - "go test ./..."
    - "go vet ./..."
    - "go build ./cmd/myapp"

# Python
validation:
  commands:
    - "pytest"
    - "ruff check ."
    - "mypy ."
```

## Reliability Features

### Stall Detection

If the provider goes silent for a configurable period, Gromit automatically kills the stalled process and retries or escalates. No manual Ctrl+C needed. Stall retries count against `max_retries_per_model` -- once exhausted, Gromit escalates to the next model.

### Time Budgets

Prevent runaway sessions:

```bash
gromit run --time-budget 30          # 30 minutes max
gromit run --time-budget-hours 2     # 2 hours max
gromit run -t 30 -H 2               # Flags are additive: 150 minutes
```

When the deadline passes, Gromit finishes the current task and exits gracefully.

### Failure Analysis

Every failure gets analyzed by a separate provider call that categorizes it:

| Category | Meaning | Action |
|----------|---------|--------|
| `syntax` | Typo, import, API misuse | Retry with context |
| `logic` | Algorithm wrong, edge case | Retry or escalate |
| `environment` | Missing tool, version issue | Skip or fix env |
| `unclear_spec` | Ambiguous specification | Skip for human |
| `missing_context` | Didn't know about existing code | Retry with hint |
| `test_flake` | Non-deterministic failure | Retry |
| `task_too_complex` | Scope too large for one iteration | Skip for human |

### Partial Progress Detection

When a task fails, Gromit shows what was accomplished before the failure using a git checkpoint captured before work started:

```
Changes detected (partial progress):
 internal/auth/jwt.go     | 120 ++++++++++
 internal/auth/jwt_test.go | 85 ++++++++
 2 files changed, 205 insertions(+)
```

### Streaming and Heartbeat

During long-running invocations, Gromit provides visibility with a streaming log (watch with `tail -f .gromit/logs/stream-*.log`) and a heartbeat every 30 seconds showing elapsed time, files modified, and tool calls made.
Provider-native terminal streaming (color/layout) is enabled by default via `stream.preserve_provider_output: true` in `gromit.yaml`. You can override at runtime with `GROMIT_PRESERVE_PROVIDER_STREAM=0` or `1`.

### Pre-flight Checks

Before validation, Gromit checks that required tools are available (e.g., `pnpm`, `go`, `pytest`). This catches environment issues before they look like code failures.

## Safety and Permissions

Gromit runs the configured provider CLI with `--dangerously-skip-permissions` by default, meaning the agent can modify files and run commands without asking. This is configurable in `gromit.yaml` under `claude.flags`.

Recommended practices:

- **Start with `--dry-run`** to preview what Gromit would do before executing
- **Run in a dedicated branch** so you can review changes before merging
- **Keep validation commands deterministic** -- flaky tests cause unnecessary retries and escalation
- **Don't put secrets in specs, rules, or learnings** -- these are injected into prompts
- **Review learnings periodically** with `gromit retro` -- bad learnings degrade future performance

Gromit auto-pushes to the remote after each successful task by default. Disable this with `git.auto_push: false` in `gromit.yaml`.

## When NOT to Use Gromit

- **Interactive pairing** -- if you want a back-and-forth coding conversation, use your agent CLI directly
- **No local test suite** -- Gromit's value depends on independent validation; without tests, it's just a loop
- **One-off exploration** -- if you're exploring an approach rather than executing a defined task, Gromit adds overhead
- **Tasks requiring human judgment mid-execution** -- Gromit runs autonomously; it can't pause and ask you questions during a task

## Commands Reference

| Command | Description |
|---------|-------------|
| `gromit init` | Initialize gromit.yaml and .gromit/ directory |
| `gromit add <idea>` | Capture an idea to the backlog |
| `gromit backlog` | View and manage the idea backlog |
| `gromit refine` | Turn ideas into structured specs (interactive) |
| `gromit plan <spec>` | Create an implementation plan from a spec (interactive) |
| `gromit decompose <plan>` | Break a plan into executable tasks |
| `gromit run` | Process tasks until queue is empty |
| `gromit run -n 5` | Process at most 5 tasks |
| `gromit run -t 30` | Run with a 30-minute time budget |
| `gromit run -H 2` | Run with a 2-hour time budget (flags stack) |
| `gromit run --dry-run` | Preview without executing |
| `gromit status` | Show next task and selected model |
| `gromit queue` | Display processing queue with model assignments |
| `gromit board` | Show all tasks grouped by status |
| `gromit triage` | Interactively triage open tasks |
| `gromit review` | Run a thorough code review |
| `gromit retro` | Run retrospective analysis |

## License

MIT
