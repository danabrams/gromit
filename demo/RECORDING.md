# Recording the Gromit Demo

## Prerequisites

- **asciinema** installed:
  ```bash
  pip install asciinema    # or: sudo apt install asciinema
  ```
- **bd** on PATH (`bd --version` works)
- **gromit** built:
  ```bash
  cd /path/to/gromit && go build -o gromit ./cmd/gromit
  ```
- A valid Claude API key (or Claude Code configured)

## Terminal Setup

- **Size:** 120 columns x 35 rows (`printf '\e[8;35;120t'` or resize manually)
- **Font:** Monospace, 14-16pt — large enough to read in a recording
- **Background:** Dark background, light text
- **Prompt:** Keep it short — e.g. `export PS1='$ '`
- **Clear history:** `clear` before starting

## Recording

```bash
asciinema rec --idle-time-limit 3 demo.cast
```

The `--idle-time-limit 3` flag caps any pause at 3 seconds, keeping the recording tight during long Claude invocations.

## Demo Sequence

Source the script, then call functions in order:

```bash
source demo/demo.sh
```

| # | Function | Mode | ~Duration | Notes |
|---|----------|------|-----------|-------|
| 1 | `demo_setup` | Scripted | ~1 min | Creates Go project, git/bd/gromit init |
| 2 | `demo_add` | Scripted | ~30s | Adds 2 ideas to backlog |
| 3 | `demo_backlog` | Read-only | ~10s | Shows the backlog |
| 4 | `demo_refine` | Interactive | ~5 min | Refines "due dates" idea into a spec |
| 5 | `demo_plan` | Interactive | ~5 min | Creates implementation plan from spec |
| 6 | `demo_decompose` | Autonomous | ~1 min | Decomposes plan into beads |
| 7 | `demo_queue` | Read-only | ~10s | Shows bead queue with model assignments |
| 8 | `demo_run` | Autonomous | ~5 min | Runs `gromit run -n 2` on 2 beads |
| 9 | `demo_board` | Read-only | ~10s | Shows board + recent git commits |
| 10 | `demo_review` | Interactive | ~3 min | Code review of changes |
| 11 | `demo_retro` | Interactive | ~3 min | Retrospective on learnings |
| 12 | `demo_cleanup` | Scripted | ~5s | Removes demo directory |

**Total: ~25-30 minutes**

### Talking Points for Interactive Sections

**demo_refine** — When Claude asks about the feature:
- "Support relative dates like 'tomorrow', '3 days', 'next friday', plus ISO dates"
- "Add a DueDate field to the Todo struct, store as RFC3339"
- "Use a separate date parsing package for testability"

**demo_plan** — Claude reads the spec and creates an implementation plan. Let it run, then review the plan it writes.

**demo_review / demo_retro** — Let Claude drive; comment on anything interesting it flags.

## Quick Test Run

Use `--skip` on interactive steps to do a dry run without Claude API calls:

```bash
source demo/demo.sh
demo_setup
demo_add
demo_backlog
demo_refine --skip     # Uses fallback spec instead of Claude
demo_plan --skip       # Uses fallback plan instead of Claude
demo_decompose
demo_queue
# demo_run             # Still requires Claude — skip for dry run
# demo_review          # Still requires Claude — skip for dry run
# demo_retro           # Still requires Claude — skip for dry run
demo_cleanup
```

This verifies the scaffolding, content, and bd integration work end-to-end without spending API credits.

## Sharing

Upload to asciinema.org:

```bash
asciinema upload demo.cast
```

Embed in a page:

```html
<script src="https://asciinema.org/a/YOUR_ID.js" id="asciicast-YOUR_ID" async></script>
```

Or use the standalone player:

```html
<asciinema-player src="demo.cast" speed="1.5" theme="monokai"></asciinema-player>
```
