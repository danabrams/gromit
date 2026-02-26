# Your Project

<!-- Update this with your project's name and description -->

## Quick Start

```bash
bd create "Task title" --priority 1
gromit run                         # Run until no work
gromit run -n 5 --time-budget 30   # Max 5 beads, 30-min budget
gromit status                      # Show next bead + model
```

## Bead Sizing

- **One concern per bead** — a single file or two tightly coupled files
- **1-3 acceptance criteria** — concrete, testable criteria only; split if more than 3
- **Self-contained** — understandable without reading other beads
- **No ambiguity** — The implementation agent executes without making design decisions
- **Max 2 files touched** — if more, consider splitting the bead
- **Clear definition of done** — each criterion has an obvious pass/fail test

## Capturing Ideas vs Creating Beads

When asked to add something to the backlog, use `gromit add "<idea>"` — not `bd create`. The backlog is for rough ideas that flow through the refine → plan → decompose pipeline. Only use `bd create` when you have a fully scoped, ready-to-implement task with clear acceptance criteria.

## Rules

See `.gromit/RULES.md` for project-specific constraints and best practices.

## Learnings

See `.gromit/LEARNINGS.md` for accumulated patterns and conventions from this project's iterations.
