# Gromit

**Autonomous task execution for Claude Code, with built-in learning.**

Gromit takes a queue of tasks (beads), hands them to Claude one at a time with fresh context, validates the results, and escalates on failure. Along the way, it builds a persistent knowledge base of what works and what doesn't — so each iteration gets smarter than the last.

```
                         ┌─────────────┐
                         │  bd ready   │ ← Get next task
                         └──────┬──────┘
                                │
                         ┌──────▼──────┐
                         │ Select Model│ ← P0→opus, P1→sonnet, P2→haiku
                         └──────┬──────┘
                                │
                    ┌───────────▼───────────┐
                    │  Claude Code (fresh)  │ ← New process, full context
                    │  + rules + learnings  │
                    └───────────┬───────────┘
                                │
                         ┌──────▼──────┐
                    ┌────│  Validate   │────┐
                    │    └─────────────┘    │
                  pass                    fail
                    │                      │
             ┌──────▼──────┐     ┌─────────▼─────────┐
             │  bd close   │     │ Analyze + Escalate │
             └─────────────┘     └─────────┬─────────┘
                                           │
                                    ┌──────▼──────┐
                                    │   Learning  │ ← Extract insight
                                    │   recorded  │   for next time
                                    └─────────────┘
```

## Why Gromit?

AI coding agents lose context between sessions. They make the same mistakes. They can't learn from experience.

Gromit fixes this:

- **Fresh context every iteration** — no stale state, no confused models
- **Automatic failure analysis** — when something breaks, Gromit figures out why
- **Persistent learnings** — mistakes become knowledge that feeds into future prompts
- **Model escalation** — starts cheap, upgrades only when needed
- **Separate validation** — tests and lints run in isolation, not by the build model

Named after [Gromit](https://en.wikipedia.org/wiki/Wallace_and_Gromit) — the silent, competent one who actually makes everything work while Wallace tinkers with the grand designs. You bring the ideas, Gromit makes sure they don't end up like one of Wallace's contraptions.

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

### Set Up Your Project

```bash
cd your-project
gromit init
```

This creates:

```
your-project/
├── gromit.yaml                     # Configuration
└── .gromit/
    ├── templates/                  # Prompt templates (customizable)
    │   ├── PROMPT_build.md         # Main task execution prompt
    │   ├── PROMPT_validate.md      # Validation runner prompt
    │   ├── PROMPT_analyze.md       # Failure analysis prompt
    │   ├── PROMPT_scope.md         # Scope/complexity estimation prompt
    │   ├── PROMPT_decompose.md     # Task decomposition prompt
    │   ├── PROMPT_review.md        # Light code review prompt
    │   ├── PROMPT_thorough_review.md  # Thorough code review prompt
    │   └── PROMPT_retro.md         # Retrospective analysis prompt
    ├── specs/                      # Spec files for complex features
    ├── RULES.md                    # Project constraints (non-negotiable)
    ├── LEARNINGS.md                # Accumulated knowledge
    └── logs/                       # Iteration logs (JSONL + streaming)
```

### Create Tasks

Gromit uses [Beads](https://github.com/steveyegge/beads) (`bd`) for task tracking:

```bash
bd create "Add user authentication" --priority 0    # P0 → uses opus
bd create "Refactor database layer" --priority 1     # P1 → uses sonnet
bd create "Fix typo in footer" --priority 2          # P2 → uses haiku
```

### Run

```bash
gromit run                           # Process all beads until queue is empty
gromit run -n 5                      # Process at most 5 beads
gromit run --time-budget 30          # Run for at most 30 minutes
gromit run --time-budget-hours 2     # Run for at most 2 hours (flags are additive)
gromit run --dry-run                 # Preview what would run, without executing
gromit status                        # Show the next bead and which model it would use
```

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
  Closed bead abc-123

[2] Processing: Refactor database layer (def-456)
  Model: sonnet (P1)
  Building...
  Build complete (87s)
  Validating...
  Validation passed
  Closed bead def-456

No more beads ready. 2 iterations completed.
```

## Architecture

### Directory Structure

CLI commands live in `cmd/gromit/` — one file per subcommand. Run `gromit --help` for the full list.

Internal packages live in `internal/` — each directory is a focused package:

- `runner/` — core loop orchestration and iteration management
- `config/` — YAML config loading and validation
- `bead/` — bd CLI integration and task retrieval
- `claude/` — Claude CLI invocation and subprocess management
- `prompt/` — prompt template rendering and context assembly
- `analyzer/` — failure analysis and categorization
- `review/` — post-build code review (light and thorough)
- `learnings/` — accumulated knowledge base management
- `rules/` — non-negotiable project constraints
- `retro/` — retrospective analysis and learning consolidation
- `preflight/` — environment checks before validation
- `state/` — persistent state across runs
- `logger/` — JSONL iteration logging and streaming
- `backlog/` — backlog management (ideas to tasks)
- `jsonutil/` — JSON utilities and helpers
- `tmux/` — tmux integration for interactive sessions

## How It Works

### The Loop

For each bead in the queue:

1. **Get next bead** — `bd ready` returns the highest-priority unblocked task
2. **Select model** — based on priority and labels (see [Model Selection](#model-selection))
3. **Build prompt** — assembles rules, learnings, spec, task details, and any failure context
4. **Run Claude** — fresh process with full context, streamed output
5. **Validate** — runs your test/lint commands via a separate haiku invocation
6. **On success** — closes the bead, moves to next
7. **On failure** — analyzes the failure, extracts a learning, and either retries or escalates

### Model Selection

Priority determines the default model:

| Priority | Model | Use For |
|----------|-------|---------|
| P0 | opus | Critical, complex tasks |
| P1 | sonnet | Normal tasks |
| P2 | haiku | Simple, low-risk tasks |

Labels override priority:

```bash
bd create "Complex migration" --priority 2 --label complexity:high  # → opus (label wins)
bd create "Quick rename" --priority 0 --label complexity:low        # → haiku (label wins)
```

Validation always uses haiku for cost efficiency.

### Escalation

When a task fails and the failure isn't recoverable with the current model, Gromit escalates:

```
haiku (failed) → sonnet (retry) → opus (retry) → give up
```

The prompt for retries includes the previous failure output and analysis, so the stronger model knows what went wrong and can try a different approach.

Configure the chain in `gromit.yaml`:

```yaml
escalation:
  enabled: true
  chain: [haiku, sonnet, opus]
  max_retries_per_model: 1
```

### Review and Refinement

After validation succeeds, Gromit can optionally run a **light review** to examine the completed work and suggest refinements or follow-up tasks. This is separate from validation — it's a code review step that looks for quality, consistency, and opportunities for improvement.

Configure light review in `gromit.yaml`:

```yaml
review:
  enabled: true                  # Enable light review after each bead
  model: sonnet                  # Which model reviews (sonnet is good for quality)
  match_build_model: true        # Or use the same model that built (opus if build used opus)
  timeout: 120                   # Seconds per review invocation
```

When enabled, after each successful validation:

1. **Light review** runs — examines the changes and provides feedback
2. **Review findings** are extracted — any suggested beads are created automatically
3. **Chain continues** — next bead is processed immediately

For periodic deep analysis, enable **thorough review**:

```yaml
review:
  thorough:
    enabled: true                # Periodic thorough code review
    every_n_iterations: 5        # After every N completed beads
    on_epic_complete: true       # Also review when epic tasks complete
    model: opus                  # Use strongest model for thorough review
    timeout: 900                 # Thorough reviews take longer
```

Thorough reviews are more comprehensive and generate multiple follow-up tasks. They're triggered either periodically (every N iterations) or when epic-level tasks complete.

### Fresh Context

This is the key design decision. Each Claude invocation is a **new process**. There's no conversation history carrying forward, no confused context from previous iterations.

Instead, state lives in files:

- **Git commits** — the actual code changes
- **Beads** — the task queue and status
- **LEARNINGS.md** — accumulated knowledge
- **RULES.md** — project constraints

The prompt template assembles everything Claude needs to know for each task, from scratch, every time.

## Self-Improvement System

Gromit doesn't just execute tasks — it learns from them.

### Learnings

When a task fails, Gromit runs failure analysis (via a separate Claude call) and extracts generalizable insights. These go into `.gromit/LEARNINGS.md` with two tiers:

**Provisional** — seen once, might be specific to one task:
```markdown
### 2026-02-05 | abc-123 | gotchas
Validation failures due to missing tools are environment issues, not code issues.
Check tool availability before assuming code is broken.
```

**Confirmed** — seen multiple times, high confidence:
```markdown
### conventions
Before implementing, always verify actual file and code state. Explore existing
commands to understand patterns before building new ones.
```

When a new learning is similar (>70% match) to an existing provisional one, it gets automatically promoted to confirmed. Confirmed learnings are included in every build prompt, so Claude benefits from accumulated project knowledge.

### Rules

`.gromit/RULES.md` contains non-negotiable project constraints — things that should never be violated. Rules are always included at the top of every prompt.

```markdown
# Rules

## Code Style
- This is a Go project - use idiomatic Go patterns
- Use `fmt.Errorf("context: %w", err)` for error wrapping

## Safety
- Never commit secrets or API keys
- Always handle errors - no silent failures

## Process
- Run `go build ./cmd/gromit` before committing
- Run `go test ./...` to verify tests pass
```

### Retrospective

Periodically, run a retrospective to consolidate knowledge:

```bash
gromit retro
```

This invokes opus to analyze all accumulated learnings and:

- **Merge** related or duplicate learnings
- **Promote** patterns to rules (when warranted)
- **Archive** stale or obsolete learnings
- **Suggest** rule changes

By default, retro runs interactively — it launches Claude Code with the analysis so you can discuss and apply changes together. Use `gromit retro --non-interactive` to write proposals to `.gromit/RETRO_PROPOSED_CHANGES.md` instead.

### Smart Retro Suggestions

Gromit tells you when it's time for a retro. At the end of each run, it checks:

- More than 10 provisional learnings accumulated
- More than 7 days since last retro
- Failure rate above 30%
- Total learnings exceeding 20

If any trigger fires, you'll see:

```
Retro suggested: 14 provisional learnings, 3 confirmed patterns
  (many unreviewed provisional learnings). Run: gromit retro
```

## Configuration

### gromit.yaml Reference

```yaml
# Model selection
models:
  p0: opus                     # P0 (critical) → strongest model
  p1: sonnet                   # P1 (normal) → balanced
  p2: haiku                    # P2 (low) → fastest/cheapest
  validation: haiku            # Validation model (always cheap)
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
  stuck_bead_threshold: 3      # Skip bead if it fails this many times

# Scope checking - estimate task complexity before work
scope_check:
  enabled: true
  model: haiku                 # haiku is fast and cheap for estimation

# Validation commands
validation:
  enabled: true
  commands:
    - "pnpm run test"
    - "pnpm run lint:check"
    - "pnpm run build"

# Post-work review and refinement
review:
  enabled: false               # Enable light review after each bead
  model: sonnet                # Review model (or match_build_model: true)
  match_build_model: true      # Use opus if build used opus
  timeout: 120                 # Seconds per review

  thorough:
    enabled: false             # Periodic thorough reviews
    every_n_iterations: 5      # After every N completed beads
    on_epic_complete: true     # Also on epic task completion
    model: opus
    timeout: 900

# Pre-flight environment checks
preflight:
  auto_install: ask            # ask | always | never
  tools: []                    # Explicit tool list (auto-detected if empty)

# Claude CLI settings
claude:
  binary: "claude"
  timeout: 600                 # Seconds per invocation
  stall_timeout: 120           # Seconds of silence before auto-retry (initial)
  stall_timeout_active: 300    # Seconds of silence after tool activity (allows thinking)
  bead_timeout: 1200           # Seconds max per bead (all retries + analysis + validation)
  analysis_timeout: 120        # Seconds max per failure analysis
  flags:
    - "--dangerously-skip-permissions"

# File paths (relative to project root)
paths:
  templates: ".gromit/templates"
  specs: ".gromit/specs"
  logs: ".gromit/logs"
  project_claude_md: "CLAUDE.md"
```

### Validation Commands

Customize the `validation.commands` list for your project. These run after every successful build in a separate Claude session:

```yaml
# Node/TypeScript project
validation:
  commands:
    - "pnpm run test"
    - "pnpm run lint:check"
    - "pnpm run build"

# Go project
validation:
  commands:
    - "go test ./..."
    - "go vet ./..."
    - "go build ./cmd/myapp"

# Python project
validation:
  commands:
    - "pytest"
    - "ruff check ."
    - "mypy ."
```

### Pre-flight Checks

Before running validation, Gromit checks that all required tools are available (e.g., `pnpm`, `go`, `pytest`). This catches environment issues before they look like code failures.

```yaml
preflight:
  auto_install: ask     # Prompt before installing missing tools
  # auto_install: always  # Install automatically
  # auto_install: never   # Fail if tools missing
```

When a tool is missing, you'll see:

```
Pre-flight check: missing tools: [pnpm]
  [1] Try to install automatically
  [2] Skip validation and continue
  [3] Abort
Choice [2]:
```

## Reliability Features

### Validation Output Capture

When validation fails, Gromit saves the full output to `.gromit/logs/validation-YYYYMMDD-HHMMSS.log` and displays it in the terminal. No more guessing why tests failed.

### Streaming and Heartbeat

During long-running Claude invocations, Gromit provides visibility:

- **Streaming log** — all Claude events written to `.gromit/logs/stream-*.log` in real time. Watch with `tail -f`.
- **Heartbeat** — every 30 seconds, prints progress: elapsed time, files modified, tool calls made.

```
[2m00s] 7 files modified, 28 tool calls
```

### Stall Detection

If Claude goes silent (no stream events for a configurable period), Gromit automatically kills the stalled process and retries or escalates — no manual Ctrl+C needed.

```yaml
claude:
  timeout: 600        # Global max per invocation (seconds)
  stall_timeout: 120  # Kill after this many seconds of silence (0 = disable)
```

Stall retries count against `max_retries_per_model`. Once exhausted, Gromit escalates to the next model in the chain. This prevents a single hung invocation from burning your entire timeout budget.

### Time Budgets

Run Gromit with a time limit to prevent runaway sessions:

```bash
# Run for at most 30 minutes
gromit run --time-budget 30

# Run for at most 2 hours
gromit run --time-budget-hours 2

# Combine: 30 minutes + 2 hours = 150 minutes
gromit run -t 30 -H 2
```

When the deadline approaches, Gromit checks it between iterations. If the deadline has passed, it:

1. Stops accepting new beads
2. Completes the current bead
3. Exits gracefully

Time budgets are useful for:
- **CI/CD pipelines** — prevent run jobs from timing out
- **Development sessions** — "run Gromit for the next 2 hours, then switch to code review"
- **Cost control** — limit how many expensive (opus) invocations happen in one run

### Partial Progress Detection

When a task fails, Gromit shows what was accomplished before the failure:

```
Changes detected (partial progress):
 internal/auth/jwt.go     | 120 ++++++++++
 internal/auth/jwt_test.go | 85 ++++++++
 2 files changed, 205 insertions(+)

Expected outputs:
  ✓ internal/auth/jwt.go (exists)
  ✗ internal/auth/middleware.go (not found)
```

This uses a git checkpoint captured before work starts and the bead's `expected_outputs` field to show exactly what got done and what's still missing.

### Failure Analysis

Every failure gets analyzed by a separate Claude call that categorizes it:

| Category | Meaning | Action |
|----------|---------|--------|
| `syntax` | Typo, import, API misuse | Retry with context |
| `logic` | Algorithm wrong, edge case | Retry or escalate |
| `environment` | Missing tool, version issue | Skip or fix env |
| `unclear_spec` | Ambiguous specification | Skip bead for human |
| `missing_context` | Didn't know about existing code | Retry with hint |
| `test_flake` | Non-deterministic failure | Retry |
| `task_too_complex` | Scope too large for one iteration | Skip bead for human |

The analysis determines whether to retry (same model), escalate (stronger model), or skip (needs human attention). Extractable learnings are added to LEARNINGS.md.

## Integration with Beads

Gromit uses [Beads](https://github.com/steveyegge/beads) for task management. Beads is git-native issue tracking — tasks live in your repo as data, not on a web UI.

### Creating Tasks for Gromit

```bash
# Simple tasks
bd create "Add logout button to navbar" --priority 1
bd create "Fix date formatting bug" --priority 0

# Complex features with specs
bd create "Authentication system" --type epic --label spec:auth
bd create "Add JWT validation" --parent <epic-id> --priority 1
bd create "Add refresh token support" --parent <epic-id> --priority 1
```

### Spec Files

For complex features, write a spec in `.gromit/specs/`:

```markdown
# specs/auth.md

## Overview
JWT-based authentication with refresh tokens.

## Acceptance Criteria
- Login endpoint returns access + refresh tokens
- Access tokens expire after 15 minutes
- Refresh tokens expire after 7 days
- Invalid tokens return 401
```

Link it to an epic with the `spec:auth` label. All child tasks inherit the spec automatically — so each subtask gets the full requirements in its prompt.

### Expected Outputs

If a bead has an `expected_outputs` field, Gromit uses it for partial progress tracking on failure:

```bash
bd create "Add auth middleware" --priority 1 \
  --expected-outputs "internal/auth/middleware.go,internal/auth/middleware_test.go"
```

### Labels

Labels control model selection and spec loading:

| Label | Effect |
|-------|--------|
| `complexity:high` | Forces opus regardless of priority |
| `complexity:low` | Forces haiku regardless of priority |
| `spec:<name>` | Loads `.gromit/specs/<name>.md` into the prompt |

## Philosophy

### Why Fresh Context?

Long-running agent sessions accumulate stale assumptions. A model that's been "thinking" for 20 iterations has a context window full of old diffs, abandoned approaches, and resolved errors. It makes worse decisions, not better ones.

Gromit throws all of that away. Each task gets a fresh Claude process with exactly the context it needs: rules, learnings, the spec, and (if retrying) what went wrong last time. Nothing else.

### Why Learnings Beat Fine-Tuning

Fine-tuning is expensive, slow, and hard to inspect. Learnings are a markdown file you can read, edit, and version control. They're injected into prompts where they have immediate effect. And they're curated by the retro process — bad learnings get archived, good ones get promoted to rules.

### The Human-in-the-Loop Retro

Gromit automates execution, but humans still drive strategy. The retro is where you review what Gromit has learned, decide what becomes a rule, and course-correct. It's the supervisory layer that keeps the loop aligned with your actual goals.

```
Execution (automated)     →  gromit run
Learning (automated)      →  failure analysis + LEARNINGS.md
Consolidation (assisted)  →  gromit retro
Decision (human)          →  interactive review during gromit retro
```

## Commands Reference

| Command | Description |
|---------|-------------|
| `gromit init` | Bootstrap gromit.yaml and .gromit/ directory |
| `gromit init --force` | Overwrite existing configuration |
| `gromit run` | Process beads until queue is empty |
| `gromit run -n 5` | Process at most 5 beads |
| `gromit run -t 30` | Process with 30-minute time budget |
| `gromit run -H 2` | Process with 2-hour time budget (flags stack) |
| `gromit run --dry-run` | Preview without executing |
| `gromit run -c path/to/config.yaml` | Use alternate config file |
| `gromit status` | Show next bead and selected model |
| `gromit board` | Show all beads grouped by status (open/closed) |
| `gromit queue` | Display processing queue with model assignments |
| `gromit triage` | Interactively triage open beads |
| `gromit plan <feature>` | Launch Claude Code session for feature planning |
| `gromit review` | Run a thorough code review |
| `gromit review --non-interactive` | Run review autonomously without interactive session |
| `gromit review --since <commit>` | Review from a specific commit |
| `gromit review --epic <id>` | Review changes from an epic's child beads |
| `gromit review --dry-run` | Preview what would be reviewed |
| `gromit retro` | Run retrospective analysis |
| `gromit retro --non-interactive` | Write analysis to file without launching editor |
| `gromit add` | Capture ideas to backlog |
| `gromit backlog` | View/manage backlog |
| `gromit refine` | Turn ideas into tasks |

## License

MIT
