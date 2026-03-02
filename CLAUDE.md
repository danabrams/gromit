# Gromit

A Go CLI tool that runs the Gromit loop correctly - with fresh context on each iteration.

## Architecture

CLI commands live in `cmd/gromit/` — one file per subcommand.

Internal packages live in `internal/` — each directory is a focused package. Key ones:
- `runner/` — core loop orchestration
- `config/` — YAML config loading
- `bead/` — bd CLI integration
- `prompt/` — prompt template rendering
- `review/` — post-build code review

## Key Principles

1. **Fresh context each iteration** — each Claude invocation is a new process
2. **State in files, not memory** — bd beads + git commits are the memory
3. **Model selection by complexity** — P0→opus, P1→sonnet, P2→haiku
4. **Escalation on failure** — haiku→sonnet→opus retry chain
5. **Separate validation** — tests/lint run as separate haiku invocation

## Code Patterns

Nil-field normalization: use Exported `NormalizeNilFields()` for cross-package types; use unexported `normalizeNilFields()` for internal-only types. Both map nil slices/maps to empty values.
