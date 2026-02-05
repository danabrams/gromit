# Ralph Runner

A Go CLI tool that runs the Ralph Wiggum loop correctly - with fresh context on each iteration.

## Architecture

```
cmd/ralph/          # CLI entry point
internal/
  config/           # YAML configuration loading
  bead/             # bd CLI integration (beads issue tracker)
  runner/           # Core loop orchestration
  prompt/           # Prompt template rendering
  claude/           # Claude CLI invocation
templates/          # Prompt templates (PROMPT_build.md, etc.)
specs/              # Specification files referenced by beads
```

## Key Principles

1. **Fresh context each iteration** - Each Claude invocation is a new process
2. **State in files, not memory** - bd beads + git commits are the memory
3. **Model selection by complexity** - P0→opus, P1→sonnet, P2→haiku
4. **Escalation on failure** - haiku→sonnet→opus retry chain
5. **Separate validation** - Tests/lint run as separate haiku invocation

## bd Integration

- `bd ready --json --limit 1` - Get next unblocked bead
- `bd show <id> --json` - Get bead details + parent info
- `bd close <id>` - Mark bead complete
- Labels: `complexity:high`, `complexity:low`, `spec:<name>`

## Running

```bash
ralph run                    # Run loop until no work
ralph run --max-iterations 5 # Limit iterations
ralph run --dry-run          # Show what would run without executing
ralph status                 # Show current queue state
```

## Configuration

See `config.yaml` for model selection rules and escalation settings.
