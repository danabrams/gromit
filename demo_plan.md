# Gromit Demo Plan: Recorded Terminal Walkthrough

## Context

We need a reusable demo script that creates a fresh project, installs gromit, and walks through the complete workflow (add ideas, refine, plan, decompose, run, review, retro). The demo will be recorded with asciinema and kept as a script for re-recording in the future.

## What We'll Create

Three files in the `demo/` directory:

### 1. `demo.sh` — The main demo script

A sourceable bash script with numbered `demo_*` functions. The user sources it (`source demo.sh`) and calls functions one at a time. This allows:
- Re-running individual sections if something goes wrong
- Pausing between sections for narration
- Skipping interactive sections with `--skip` flag (uses pre-seeded content)

**Functions:**

| Function | Mode | Duration | What it does |
|----------|------|----------|-------------|
| `demo_setup` | Scripted | ~1 min | Create Go project, git init, bd init, gromit init, initial commit |
| `demo_add` | Semi-scripted | ~30s | Add 2 ideas to backlog (pipes answers to avoid interactive prompts) |
| `demo_backlog` | Read-only | ~10s | Show the backlog |
| `demo_refine` | Interactive | ~5 min | Launch `gromit refine` to create a spec |
| `demo_plan` | Interactive | ~5 min | Launch `gromit plan` to create an implementation plan |
| `demo_decompose` | Autonomous | ~1 min | Run `gromit decompose --review` to create beads |
| `demo_queue` | Read-only | ~10s | Show bead queue with model assignments |
| `demo_run` | Autonomous | ~5 min | Run `gromit run -n 2` to execute 2 beads |
| `demo_board` | Read-only | ~10s | Show results on the board |
| `demo_review` | Interactive | ~3 min | Launch `gromit review` for code review |
| `demo_retro` | Interactive | ~3 min | Launch `gromit retro` for retrospective |
| `demo_cleanup` | Scripted | ~5s | Remove demo directory |

**Key design decisions:**
- The demo project is a simple Go CLI todo app (matches gromit's Go validation defaults, familiar domain)
- `gromit add` prompts for type and context interactively — the script will pipe answers via stdin (`printf "1\n\n" | gromit add "..."`) to keep it smooth
- Each function prints a section header explaining what's about to happen
- A `DEMO_DIR` variable controls the demo location (default: `/tmp/gromit-demo`)
- The default `gromit.yaml` from `gromit init` uses `pnpm` commands for validation — the script will `sed` it to use `go test/vet/build` after init

### 2. `demo-content.sh` — Pre-seeded project content + fallbacks

Sourced by `demo.sh`. Contains:
- **Heredocs for the starter project**: `main.go`, `todo/todo.go`, `todo/todo_test.go`, `go.mod`, `CLAUDE.md`, `.gitignore`
- **Fallback spec** (`due-dates.md`): Used if `demo_refine --skip` is called
- **Fallback plan** (`due-dates.md`): Used if `demo_plan --skip` is called
- **Seed learnings**: Pre-seeded learnings for `demo_retro` in case `gromit run` didn't generate any

### 3. `RECORDING.md` — Recording instructions

Brief instructions for:
- Installing asciinema (`sudo apt install asciinema` or `pip install asciinema`)
- Recording settings (`asciinema rec --idle-time-limit 3 demo.cast`)
- Tips for a clean recording (terminal size, font, colors)
- How to upload/share recordings

---

## Demo Project: todo-cli

A minimal Go CLI todo manager that gives Claude something to work with:

```
/tmp/gromit-demo/
├── main.go           # cobra CLI with "add" and "list" commands
├── todo/
│   ├── todo.go       # Todo struct, file-based JSON storage
│   └── todo_test.go  # Basic tests for add/list
├── go.mod
├── CLAUDE.md         # Project description for Claude context
└── .gitignore
```

This provides existing patterns for Claude to follow. The feature we'll refine and build is **"add due dates with natural language parsing"** — it's a clear feature that naturally decomposes into 3-5 beads (date parser, struct changes, CLI flags, sorting, tests).

---

## Demo Flow (What the Audience Sees)

### Act 1: Setup (~2 min, scripted)

```bash
source demo/demo.sh
demo_setup
```

Creates the todo-cli project, initializes git, bd, and gromit. Shows the project structure and gromit's scaffolding (templates, rules, learnings, config).

### Act 2: Capture Ideas (~1 min, scripted)

```bash
demo_add
demo_backlog
```

Adds two ideas to the backlog. Shows how gromit auto-categorizes ideas (feature vs bug). Displays the backlog.

### Act 3: Refine (~5 min, interactive)

```bash
demo_refine
```

Launches Claude Code to refine the "due dates" idea into a structured spec. Claude explores the codebase, asks questions, and writes a spec to `.gromit/specs/due-dates.md`.

**Talking points during the session:**
- "Support relative dates like 'tomorrow', '3 days', 'next friday', plus ISO dates"
- "Add a DueDate field to the Todo struct, store as RFC3339"
- "Use a separate date parsing package for testability"

### Act 4: Plan (~5 min, interactive)

```bash
demo_plan
```

Claude reads the spec, explores existing code, and creates an implementation plan with architecture decisions and test strategy.

### Act 5: Decompose + Queue (~2 min, autonomous)

```bash
demo_decompose
demo_queue
```

Gromit invokes Claude non-interactively to convert the plan into beads. Shows the queue with model assignments (P1 -> sonnet).

### Act 6: Run (~5 min, autonomous)

```bash
demo_run
```

`gromit run -n 2` — watches the loop execute: model selection, Claude building code, validation (go test, go vet, go build), bead closure. This is the payoff moment.

### Act 7: Results (~1 min, read-only)

```bash
demo_board
```

Shows the board with completed beads, git log with commits, tests passing.

### Act 8: Review + Retro (~5 min, interactive)

```bash
demo_review
demo_retro
```

Quick code review session, then retrospective analysis of learnings.

**Total estimated time: 25-30 minutes**

---

## Implementation Steps

1. Create `demo/` directory in the gromit repo
2. Write `demo/demo-content.sh` with all heredoc content (starter project + fallback spec/plan/learnings)
3. Write `demo/demo.sh` with all `demo_*` functions
4. Write `demo/RECORDING.md` with recording instructions
5. Test `demo_setup` end-to-end: verify the project scaffolds, builds, and tests pass
6. Test `demo_refine --skip` + `demo_decompose` to verify the fallback path works

---

## Notes

- `bd init -p demo-todo` initializes bd with "demo-todo" prefix for bead IDs
- After `gromit init`, validation commands need to be changed from pnpm to go (`go test ./...`, `go vet ./...`, `go build ./...`)
- `gromit add` always prompts for type + context — we pipe stdin to automate this during the demo
- The skills (gromit-refine, gromit-plan, gromit-decompose) are installed as Claude Code skills in the gromit repo and will be available when Claude Code launches
- `gromit run` uses `--dangerously-skip-permissions` by default (configured in gromit.yaml)
