# Ralph Runner

A Go CLI tool that runs the Ralph Wiggum loop correctly - with fresh context on each iteration.

## Installation

```bash
# From source
go install github.com/danabrams/ralph-runner/cmd/ralph@latest

# Or build locally
cd ralph-runner
go build -o ralph ./cmd/ralph
```

## Quick Start

```bash
# In your project directory
ralph init                    # Creates ralph.yaml + .ralph/

# Create beads (tasks) using bd
bd create "Implement feature X" --priority 1

# Run the loop
ralph run                     # Run until no work
ralph run -n 5                # Run max 5 iterations
ralph run --dry-run           # Preview without executing
ralph status                  # Show next bead + model
```

## Architecture

```
cmd/ralph/              # CLI entry point
  main.go               # Root command + run/status
  init.go               # Init command
internal/
  config/               # YAML configuration loading
  bead/                 # bd CLI integration
  runner/               # Core loop orchestration
  prompt/               # Prompt template rendering
  claude/               # Claude CLI invocation
  logger/               # JSONL iteration logging
```

## Project Structure (after `ralph init`)

```
your-project/
├── ralph.yaml              # Configuration
├── CLAUDE.md               # Your project's Claude instructions
└── .ralph/
    ├── templates/
    │   ├── PROMPT_build.md     # Build prompt template
    │   └── PROMPT_validate.md  # Validation prompt template
    ├── specs/                  # Spec files for complex features
    └── logs/                   # Iteration logs (JSONL)
```

## Key Principles

1. **Fresh context each iteration** - Each Claude invocation is a new process
2. **State in files, not memory** - bd beads + git commits are the memory
3. **Model selection by complexity** - P0→opus, P1→sonnet, P2→haiku
4. **Escalation on failure** - haiku→sonnet→opus retry chain
5. **Separate validation** - Tests/lint run as separate haiku invocation

## Bead Sizing

Properly sized beads are the foundation of the Ralph Wiggum loop. Follow these rules:

- **One concern per bead** - A single file or two tightly coupled files
- **1-3 acceptance criteria** - Concrete, testable criteria only; split if more than 3
- **Self-contained** - Understandable without reading other beads
- **No ambiguity** - Claude implements without making design decisions
- **Max 2 files touched** - If more, consider splitting the bead
- **Clear definition of done** - Each criterion has an obvious pass/fail test

## bd Integration

- `bd ready --json --limit 1` - Get next unblocked bead
- `bd show <id> --json` - Get bead details + parent info
- `bd close <id>` - Mark bead complete
- Labels: `complexity:high`, `complexity:low`, `spec:<name>`

## Model Selection

Priority-based:
- P0 (critical) → opus
- P1 (normal) → sonnet
- P2 (low) → haiku

Label overrides (higher precedence):
- `complexity:high` → opus
- `complexity:low` → haiku

Validation always uses haiku for cost efficiency.

## Spec Files

For complex features, create a spec file in `.ralph/specs/`:

```markdown
# specs/auth.md
## Acceptance Criteria
- JWT-based authentication
- Refresh token support
...
```

Then reference it from beads via label:
- Epic bead: `bd create "Auth system" --type epic --label spec:auth`
- Child tasks: `bd create "Add JWT validation" --parent <epic-id>`

Child tasks inherit the spec from their parent epic.

## Configuration (ralph.yaml)

```yaml
models:
  p0: opus
  p1: sonnet
  p2: haiku
  validation: haiku
  labels:
    "complexity:high": opus

escalation:
  enabled: true
  chain: [haiku, sonnet, opus]

validation:
  enabled: true
  commands:
    - "pnpm run test"
    - "pnpm run lint:check"

claude:
  timeout: 600
  flags:
    - "--dangerously-skip-permissions"
```

## Logs

Iteration results are logged to `.ralph/logs/run-YYYYMMDD-HHMMSS.jsonl`:

```json
{"timestamp":"...","iteration":1,"bead_id":"abc-123","model":"sonnet","success":true,"duration_ms":45000}
```
