---
id: claude-skill
source_ideas: [idea-1770452824347]
created: 2026-02-07
---

# Claude Code Orchestrator Skill

## Specification

A single `/gromit` Claude Code skill that lets users orchestrate the full Gromit pipeline without leaving their Claude Code session. The skill acts as a lightweight dispatcher — it reads pipeline state, shows what needs doing, and launches each stage with a fresh context window.

### Pipeline Dashboard

When the user invokes `/gromit` (or Claude detects a gromit project), the skill displays a pipeline status dashboard:

```
Pipeline Status:
  Backlog: 3 unrefined ideas
  Specs:   1 ready to plan (user-profiles)
  Plans:   1 ready to decompose (api-endpoints)
  Beads:   4 ready to run

Recommended: Plan spec "user-profiles"
```

The dashboard reads from `.gromit/backlog.jsonl`, `.gromit/specs/`, `.gromit/plans/` (checking `decomposed` frontmatter), and `bd ready --json` to build the status. After showing status, the skill asks the user what they want to do, or they can state their intent directly (e.g., `/gromit refine my new idea`).

### Stage Dispatch

Each pipeline stage is dispatched differently depending on whether it needs user interaction:

**Interactive stages (refine, plan)** use the `/clear` + SessionStart hook pattern:

1. The skill gathers minimal context (which idea to refine, which spec to plan) from the user in the current session.
2. The skill writes a pipeline state file to `.gromit/pipeline-state.json` containing the stage to run, its inputs, and any relevant context.
3. The skill tells the user: "Ready. Type `/clear` to start the refine session with fresh context."
4. The user types `/clear` — context is wiped.
5. A pre-registered `SessionStart` hook fires, reads the pipeline state file, outputs the appropriate skill content (refine or plan) plus all gathered context into the fresh session.
6. The hook's script deletes the pipeline state file after reading it, so subsequent `/clear` commands behave normally.
7. Claude sees the injected skill content and runs the stage interactively with a completely clean context window.
8. When the stage completes, the user can invoke `/gromit` again to see updated status and continue the pipeline.

**Non-interactive stages (decompose, review, retro)** use Task subagents:

1. The skill launches a Task subagent with the appropriate skill content and context.
2. The subagent runs with completely fresh context, reads the codebase, and produces output (beads, review notes, etc.).
3. Results are summarized back in the main conversation.

**Simple stages (add, run, status, queue)** use direct Bash execution:

- `add`: Runs `gromit add "<idea>"` via Bash.
- `run`: Runs `gromit run` (with any flags) via Bash.
- `status`/`queue`/`board`: Runs the corresponding `gromit` CLI command and displays output.

### Pipeline State File

`.gromit/pipeline-state.json` is a transient file that bridges the `/clear` boundary:

```json
{
  "stage": "refine",
  "inputs": {
    "idea_text": "Add user profiles with avatar support",
    "backlog_id": "idea-1770452824347",
    "specs_dir": ".gromit/specs"
  },
  "created_at": "2026-02-07T10:30:00Z"
}
```

For `plan`, inputs would include `spec_name`, `spec_content`, `plans_dir`, `plan_path`, and `open_beads`.

The file is **consumed on read** — the resume script deletes it after injecting content, so the hook becomes a no-op on subsequent clears.

### SessionStart Hook

The hook is registered in the project's `.claude/settings.json` (or user-level settings). It fires on every `/clear` but only acts when a pipeline state file exists:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "clear",
        "hooks": [
          {
            "type": "command",
            "command": ".gromit/hooks/pipeline-resume.sh"
          }
        ]
      }
    ]
  }
}
```

The `pipeline-resume.sh` script:
1. Checks if `.gromit/pipeline-state.json` exists.
2. If no — exits silently (no-op).
3. If yes — reads the state, outputs the appropriate skill content and context to stdout, then deletes the state file.

### Installation

`gromit install-skill` is an opt-in command that:

1. Creates the `.gromit/hooks/` directory and writes `pipeline-resume.sh`.
2. Writes the `/gromit` skill file to `.claude/skills/gromit.md` (project-level).
3. Registers the `SessionStart` hook in `.claude/settings.json` (merging with existing hooks if present).
4. Prints confirmation and usage instructions.

The command is idempotent — running it again updates files to the latest version without duplicating hooks.

### Skill Content

The installed skill file (`.claude/skills/gromit.md`) contains:

- The orchestrator logic (dashboard, stage dispatch, state file writing).
- Embedded copies of the refine, plan, and decompose skill content (so the resume script can inject them without needing the `gromit` binary).
- Instructions for launching Task subagents with the correct prompts for non-interactive stages.

### Pipeline Flow Example

1. User starts Claude Code in a gromit project, types `/gromit`.
2. Skill shows dashboard: "Backlog: 2 unrefined ideas. Recommended: Refine idea 'user profiles'."
3. User says "refine user profiles."
4. Skill confirms the idea, writes pipeline state, says "Type `/clear` to start."
5. User types `/clear`. Hook fires, injects refine skill + idea context. Fresh session.
6. Claude runs the full refine flow (explore codebase, ask questions, write spec).
7. User types `/gromit` again. Dashboard shows: "Specs: 1 ready to plan (user-profiles)."
8. User says "plan it."
9. Skill writes pipeline state for plan stage, says "Type `/clear` to start."
10. User types `/clear`. Hook fires, injects plan skill + spec content. Fresh session.
11. Claude runs the plan flow (architecture checkpoint, test checkpoint, task breakdown).
12. User types `/gromit` again. Dashboard shows: "Plans: 1 ready to decompose."
13. User says "decompose it." Skill launches Task subagent — fresh context, runs autonomously.
14. Subagent returns: "Created 6 beads for user-profiles." User sees summary.
15. User says "run." Skill executes `gromit run` via Bash.

## Acceptance Criteria

- Invoking `/gromit` in a gromit project displays a pipeline dashboard showing counts of unrefined backlog items, unplanned specs, undecomposed plans, and ready beads, with a recommended next action.
- Interactive stages (refine, plan) write a pipeline state file and instruct the user to `/clear`; after clearing, the SessionStart hook injects the correct skill content and context into a fresh session.
- The pipeline state file is deleted after the hook consumes it, so subsequent `/clear` commands behave normally.
- Non-interactive stages (decompose, review, retro) run as Task subagents with fresh context and return summarized results to the main conversation.
- `gromit install-skill` creates the hook script, skill file, and hook registration, and is idempotent.
- The `/gromit` skill can dispatch `add` and `run` via direct Bash execution of the `gromit` CLI.

## Decisions

1. **Single orchestrator skill, not one skill per stage.** Multiple skills would fragment the UX and require the user to remember which skill to invoke. A single `/gromit` entry point with a dashboard guides the user to the right action.

2. **`/clear` + hook for interactive stages, Task subagents for non-interactive.** Interactive stages (refine, plan) need back-and-forth with the user, which Task subagents can't provide. The `/clear` + hook pattern gives truly fresh context while preserving full interactivity. Non-interactive stages (decompose, review, retro) don't need interaction, so Task subagents are simpler and require no user action.

3. **Pipeline state file is consumed (deleted) on read.** This ensures the hook is a no-op when no pipeline is active. The user can `/clear` freely without triggering unintended stage launches.

4. **Opt-in installation via `gromit install-skill`.** Not all gromit users use Claude Code, and the skill requires writing to `.claude/` which is Claude Code's territory. Keeping it opt-in avoids surprising users who don't want the integration.

5. **Skill file embeds all stage skill content.** The resume script needs to inject skill content (refine, plan, decompose) without requiring the `gromit` binary to be invoked during the hook. Embedding the content in the installed skill file makes the hook self-contained.

6. **Dashboard reads existing gromit state files directly.** Rather than introducing new state tracking, the dashboard derives status from what already exists: `backlog.jsonl` entries and their `status` field, spec files in `.gromit/specs/`, plan files in `.gromit/plans/` with their `decomposed` frontmatter, and `bd ready --json` for bead counts.

## Research & Context

### Current State

The pipeline stages already exist as embedded Go skills in `skills/`:
- `skills/gromit-refine/SKILL.md` — conversational refinement (232 lines)
- `skills/gromit-plan/SKILL.md` — implementation planning with checkpoints (266 lines)
- `skills/gromit-decompose/SKILL.md` — JSON-only bead extraction (241 lines)

Each CLI command (`cmd/gromit/refine.go`, `cmd/gromit/plan.go`, `cmd/gromit/decompose.go`) builds a system prompt, injects the skill content via `--append-system-prompt`, and launches `claude` as a subprocess. The skill content is embedded at compile time via `skills/embed.go`.

Pipeline chaining (`cmd/gromit/chain.go`) already offers to run the next stage after each completes, but this operates at the terminal level between Claude sessions.

### Pipeline State Sources

- **Backlog**: `.gromit/backlog.jsonl` — each line has `status` ("" or "refined") and `spec_name`
- **Specs**: `.gromit/specs/*.md` — existence means "refined"
- **Plans**: `.gromit/plans/*.md` — frontmatter `decomposed: true/false` tracks decomposition
- **Beads**: `bd ready --json --limit 1` for next ready bead, `bd list --json` for counts
- **Run state**: `.gromit/state.json` tracks last review and retro timestamps

### Claude Code Integration Points

- Skills are installed at `.claude/skills/<name>.md` (project-level) or `~/.claude/skills/` (user-level)
- Hooks are configured in `.claude/settings.json` under the `hooks` key
- `SessionStart` hooks with `"matcher": "clear"` fire after `/clear` commands
- Hook commands output to stdout, which is injected into the session context
- The `/clear` command wipes conversation history but reloads CLAUDE.md and hooks
