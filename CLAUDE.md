# Gromit

A Go CLI tool that runs the Gromit loop correctly - with fresh context on each iteration.

## Quick Start

```bash
gromit init                        # Creates gromit.yaml + .gromit/
bd create "Implement feature X" --priority 1
gromit run                         # Run until no work
gromit run -n 5 --time-budget 30   # Max 5 beads, 30-min budget
gromit status                      # Show next bead + model
```

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

## Project Structure (after `gromit init`)

```
your-project/
├── gromit.yaml              # Configuration (see gromit.yaml in repo for full reference)
└── .gromit/
    ├── templates/           # Prompt templates (PROMPT_*.md) — one per phase
    ├── specs/               # Spec files for complex features
    ├── RULES.md             # Project constraints (non-negotiable)
    ├── LEARNINGS.md         # Accumulated knowledge
    └── logs/                # Iteration logs (JSONL + streaming)
```

Templates are created by `gromit init` and cover: build, validate, analyze, retro, scope, decompose, review, and thorough review.

## Development Commands

```bash
go fmt ./... && go build ./cmd/gromit   # Format + build
go test ./...                           # Test
golangci-lint run ./...                 # Lint
```

## Key Principles

1. **Fresh context each iteration** — each Claude invocation is a new process
2. **State in files, not memory** — bd beads + git commits are the memory
3. **Model selection by complexity** — P0→opus, P1→sonnet, P2→haiku
4. **Escalation on failure** — haiku→sonnet→opus retry chain
5. **Separate validation** — tests/lint run as separate haiku invocation

## Bead Sizing

- **One deliverable behavior per bead** — a single observable change that a caller or user could verify, not "one file" or "one concern," but one unit of working functionality
- **1-3 acceptance criteria** — concrete, testable criteria only; split if more than 3
- **Soft file limit of 4-5** — if touching 6+ files across unrelated packages, consider splitting. Touching interface.go, impl.go, mock_test.go, and impl_test.go for one method addition is fine — that's one change, not four
- **Self-contained** — understandable without reading other beads
- **No ambiguity** — Claude implements without making design decisions
- **Clear definition of done** — each criterion has an obvious pass/fail test

### Grouping Rules (Never Split These)

- **Interface + implementation + mock updates** — changing an interface requires updating all implementations and mocks to compile. This is one change, not three beads
- **Implementation + its tests** — Claude writes tests alongside implementation. Under ATDD, they're explicitly the same workflow. Never create a separate "write tests for X" bead
- **Companion methods in the same package** — methods that follow the same pattern in the same file (e.g., `ReadyWithLabel` and `ListWithLabel`) are one bead. If you'd copy-paste-modify to create the second, they belong together
- **Command flags + the wiring that makes them work** — a CLI flag that does nothing isn't a deliverable. The flag, its plumbing through to the runner, and its effect are one bead
- **Template + its registration** — adding a template file and registering it in the renderer are one action, not two

## Capturing Ideas vs Creating Beads

When asked to add something to the backlog, use `gromit add "<idea>"` — not `bd create`. The backlog is for rough ideas that flow through the refine → plan → decompose pipeline. Only use `bd create` when you have a fully scoped, ready-to-implement task with clear acceptance criteria.

## bd Integration

- `bd ready --json --limit 1` — get next unblocked bead
- `bd show <id> --json` — get bead details + parent info
- `bd close <id>` — mark bead complete
- Labels: `complexity:high`, `complexity:low`, `spec:<name>`

## Model Selection

Priority-based: P0→opus, P1→sonnet, P2→haiku. Label overrides beat priority (`complexity:high`→opus, `complexity:low`→haiku). Validation always uses haiku.

## Configuration

See `gromit.yaml` in the repo root for the full annotated config reference. Key sections: `models`, `escalation`, `loop`, `scope_check`, `validation`, `review`, `preflight`, `claude`, `paths`.

## Keeping Docs Current

CLAUDE.md describes patterns and conventions, not exhaustive file lists. When adding new commands or packages, the architecture section above should stay accurate without edits — it points to directories, not individual files. Update this file only when the project's *principles* or *conventions* change, not when files are added.
