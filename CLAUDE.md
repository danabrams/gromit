# Gromit

A Go CLI tool that runs the Gromit loop correctly - with fresh context on each iteration.

## Architecture

CLI commands live in `cmd/gromit/` — one file per subcommand. Run `gromit --help` for the full list.

Internal packages live in `internal/` — each directory is a focused package. Key ones:
- `runner/` — core loop orchestration
- `config/` — YAML config loading
- `bead/` — bd CLI integration
- `claude/` — Claude CLI invocation
- `prompt/` — prompt template rendering
- `analyzer/` — failure analysis
- `review/` — post-build code review
- `learnings/`, `rules/`, `retro/` — self-improvement system
- `preflight/` — environment checks before validation
- `state/` — persistent state across runs
- `logger/` — JSONL iteration logging

## Key Principles

1. **Fresh context each iteration** — each Claude invocation is a new process
2. **State in files, not memory** — bd beads + git commits are the memory
3. **Model selection by complexity** — P0→opus, P1→sonnet, P2→haiku
4. **Escalation on failure** — haiku→sonnet→opus retry chain
5. **Separate validation** — tests/lint run as separate haiku invocation
