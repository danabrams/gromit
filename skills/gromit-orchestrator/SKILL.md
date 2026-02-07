---
name: gromit
description: Orchestrate the full Gromit pipeline from Claude Code. Shows pipeline dashboard, launches stages with fresh context, and dispatches work items. Use when working with Gromit projects.
version: 1.0.0
---

# Gromit Orchestrator Skill

Provides a unified `/gromit` command in Claude Code that orchestrates the full Gromit pipeline. Shows pipeline status, launches stages with fresh context, and lets users move from idea to implementation without leaving Claude Code.

## When to Use This Skill

Use this skill when:
- The user invokes `/gromit` in a Claude Code session
- The user asks about Gromit pipeline status or what to work on next
- The user wants to refine an idea, plan a spec, decompose a plan, or run beads
- Claude detects a gromit project (presence of `gromit.yaml` and `.gromit/`)

## Methodology

This skill acts as a lightweight dispatcher for the Gromit pipeline. It reads state from existing files, presents a dashboard, and launches each stage appropriately.

### 1. Display Pipeline Dashboard

When invoked, read pipeline state and display a status dashboard:

**State Sources:**
- **Backlog**: Read `.gromit/backlog.jsonl` — count lines where `status` field is empty or missing (unrefined ideas)
- **Specs**: List files in `.gromit/specs/` that don't have corresponding plans (ready to plan)
- **Plans**: List files in `.gromit/plans/` where frontmatter has `decomposed: false` (ready to decompose)
- **Beads**: Run `bd ready --json --limit 1` to check for ready work items

**Dashboard Format:**
```
Pipeline Status:
  Backlog: [N] unrefined ideas
  Specs:   [N] ready to plan ([spec-names])
  Plans:   [N] ready to decompose ([plan-names])
  Beads:   [N] ready to run

Recommended: [Next action]
```

**Recommendation Logic:**
- If unrefined ideas exist → "Refine backlog item [first-unrefined]"
- Else if specs ready to plan → "Plan spec '[first-spec]'"
- Else if plans ready to decompose → "Decompose plan '[first-plan]'"
- Else if beads ready → "Run next bead"
- Else → "Pipeline clear — use 'gromit add' to add new ideas"

After showing the dashboard, ask: "What would you like to do?" or allow the user to state intent directly (e.g., "refine idea X", "plan spec Y", "decompose Z", "run").

### 2. Stage Dispatch: Interactive Stages (refine, plan)

For **refine** and **plan** stages, use the `/clear` + SessionStart hook pattern to get fresh context:

**Refine Dispatch:**
1. Gather context from user: which backlog item to refine (by ID or text match)
2. Read the backlog entry from `.gromit/backlog.jsonl` to get full details
3. Write pipeline state file at `.gromit/pipeline-state.json`:
   ```json
   {
     "stage": "refine",
     "inputs": {
       "idea_text": "[backlog item text]",
       "backlog_id": "[backlog item id]",
       "specs_dir": ".gromit/specs"
     },
     "created_at": "[ISO 8601 timestamp]"
   }
   ```
4. Tell user: "Ready to refine '[idea summary]'. Type `/clear` to start the refine session with fresh context."
5. Wait for user to type `/clear` — the SessionStart hook will handle the rest

**Plan Dispatch:**
1. Gather context from user: which spec to plan (by name)
2. Read the spec file from `.gromit/specs/<name>.md` to get full content
3. Run `bd list --json --status open` to get open beads (for context)
4. Write pipeline state file at `.gromit/pipeline-state.json`:
   ```json
   {
     "stage": "plan",
     "inputs": {
       "spec_name": "[spec-name]",
       "spec_path": ".gromit/specs/[spec-name].md",
       "spec_content": "[full spec content]",
       "plans_dir": ".gromit/plans",
       "plan_path": ".gromit/plans/[spec-name].md",
       "open_beads": "[bd list output]"
     },
     "created_at": "[ISO 8601 timestamp]"
   }
   ```
5. Tell user: "Ready to plan '[spec-name]'. Type `/clear` to start the plan session with fresh context."
6. Wait for user to type `/clear` — the SessionStart hook will handle the rest

**CRITICAL**: Do NOT output the skill content yourself. The SessionStart hook will inject the appropriate skill content (refine or plan) after `/clear` is typed. Your job is to gather context and write the pipeline state file.

### 3. Stage Dispatch: Non-Interactive Stages (decompose)

For **decompose**, launch a Task subagent with fresh context:

**Decompose Dispatch:**
1. Gather context from user: which plan to decompose (by name)
2. Verify the plan exists at `.gromit/plans/<name>.md`
3. Launch Task subagent with:
   - **subagent_type**: "general-purpose"
   - **prompt**: Build a prompt containing:
     - The decompose skill content (see placeholder marker below)
     - The plan file path to read
     - Instructions to output JSON only (no explanations)
4. When the subagent returns, parse the JSON output
5. For each bead in the JSON:
   - Run `bd create "[title]" --priority [priority] --description "[description]" --accept "[criterion]"` for each acceptance criterion
   - Add `--depends-on [bead-id]` flags based on dependency mapping
   - Add `--label spec:[spec-name]` to track which spec this came from
6. Update the plan file frontmatter: set `decomposed: true`
7. Summarize results to user: "Created [N] beads for '[plan-name]'. Run `bd list` or `/gromit` to see status."

**IMPORTANT**: The decompose stage is fully automated — no user interaction during execution. The Task subagent reads the plan, applies sizing rules, and returns JSON. You handle bead creation.

### 4. Stage Dispatch: Simple Commands (add, run, status)

For simple operations, use direct Bash execution:

**Add:**
- Run `gromit add "[idea text]"` via Bash
- Show confirmation: "Added idea to backlog: [idea summary]"

**Run:**
- Run `gromit run [flags]` via Bash (pass any user-specified flags like `-n 5` or `--time-budget 30`)
- Stream output to user
- Show completion summary

**Status/Queue/Board:**
- Run `gromit status`, `gromit queue`, or equivalent `bd` commands via Bash
- Display output to user

### 5. Pipeline State File Format

The pipeline state file (`.gromit/pipeline-state.json`) is a transient file that bridges the `/clear` boundary. It MUST be consumed (deleted) by the SessionStart hook after reading.

**Format:**
```json
{
  "stage": "refine" | "plan",
  "inputs": {
    // Stage-specific inputs (see dispatch instructions above)
  },
  "created_at": "2026-02-07T10:30:00Z"
}
```

**Lifecycle:**
1. Orchestrator skill writes the file when preparing an interactive stage
2. User types `/clear` — conversation context is wiped
3. SessionStart hook fires, detects the file, reads it
4. Hook outputs skill content + context to stdout (injected into fresh session)
5. Hook deletes the file
6. Subsequent `/clear` commands are no-ops (no file exists)

### 6. SessionStart Hook Integration

The SessionStart hook (`pipeline-resume.sh`) is installed by `gromit install-skill` and registered in `.claude/settings.json`. The hook script:

1. Checks if `.gromit/pipeline-state.json` exists
2. If no — exits silently (exit code 0, no output)
3. If yes:
   - Reads the JSON file
   - Determines which stage to resume (from `stage` field)
   - Outputs the appropriate skill content (refine or plan) plus the gathered inputs
   - Deletes the pipeline state file
   - Exits with code 0

**Hook Output Format:**

For **refine** stage:
```
[GROMIT REFINE SKILL CONTENT - see placeholder marker below]

---

## Context for This Session

You are refining the following backlog item:

**ID:** [backlog_id]
**Idea:** [idea_text]

Please follow the refine methodology to transform this into a structured spec at `.gromit/specs/`.
```

For **plan** stage:
```
[GROMIT PLAN SKILL CONTENT - see placeholder marker below]

---

## Context for This Session

You are planning implementation for the following spec:

**Spec:** [spec_name]
**Path:** [spec_path]

**Spec Content:**
[spec_content]

**Open Beads in Project:**
[open_beads or "No open beads"]

Please follow the plan methodology to create an implementation plan at `.gromit/plans/[spec_name].md`.
```

The hook script will read these skill contents from embedded markers in the installed skill file (see below).

## Embedded Skill Content (Placeholder Markers)

The orchestrator skill file includes the full content of the refine, plan, and decompose skills so the SessionStart hook and Task subagents can use them without invoking the `gromit` binary.

**When this skill is installed** (via `gromit install-skill`), the command will:
1. Read the existing `skills/gromit-refine/SKILL.md` file
2. Read the existing `skills/gromit-plan/SKILL.md` file
3. Read the existing `skills/gromit-decompose/SKILL.md` file
4. Replace the placeholder markers below with the actual skill content
5. Write the result to `.claude/skills/gromit.md`

### Refine Skill Content Placeholder

```
<!-- BEGIN GROMIT-REFINE-SKILL -->
[Content of skills/gromit-refine/SKILL.md will be inlined here]
<!-- END GROMIT-REFINE-SKILL -->
```

### Plan Skill Content Placeholder

```
<!-- BEGIN GROMIT-PLAN-SKILL -->
[Content of skills/gromit-plan/SKILL.md will be inlined here]
<!-- END GROMIT-PLAN-SKILL -->
```

### Decompose Skill Content Placeholder

```
<!-- BEGIN GROMIT-DECOMPOSE-SKILL -->
[Content of skills/gromit-decompose/SKILL.md will be inlined here]
<!-- END GROMIT-DECOMPOSE-SKILL -->
```

These markers allow the `gromit install-skill` command to build a self-contained skill file that the SessionStart hook and Task subagents can consume.

## Key Principles

1. **Dashboard-first** — Always show pipeline status before asking what to do
2. **Fresh context for interactive stages** — Use `/clear` + hook pattern for refine and plan
3. **Autonomous execution for non-interactive stages** — Use Task subagents for decompose
4. **Direct execution for simple commands** — Use Bash for add, run, status
5. **State in files** — Pipeline state file is the only bridge across `/clear`
6. **Consume state on read** — Hook deletes pipeline state file after injecting content
7. **Embedded skill content** — All stage skills are inlined so hook doesn't need `gromit` binary

## User Experience Flow

### Example: Refining an Idea

1. **User**: `/gromit`
2. **Skill**: Shows dashboard: "Backlog: 2 unrefined ideas. Recommended: Refine idea 'Add user profiles'"
3. **User**: "refine user profiles"
4. **Skill**: Writes pipeline state file, says "Ready to refine 'Add user profiles'. Type `/clear` to start."
5. **User**: `/clear` (context wiped)
6. **Hook**: Fires, reads pipeline state, outputs refine skill + idea context, deletes state file
7. **Claude**: Sees refine skill in fresh context, runs the refine flow interactively
8. **Result**: User collaborates with Claude to create `.gromit/specs/user-profiles.md`

### Example: Planning a Spec

1. **User**: `/gromit`
2. **Skill**: Shows dashboard: "Specs: 1 ready to plan (user-profiles). Recommended: Plan spec 'user-profiles'"
3. **User**: "plan it"
4. **Skill**: Writes pipeline state file, says "Ready to plan 'user-profiles'. Type `/clear` to start."
5. **User**: `/clear` (context wiped)
6. **Hook**: Fires, reads pipeline state, outputs plan skill + spec content, deletes state file
7. **Claude**: Sees plan skill in fresh context, runs the plan flow with checkpoints
8. **Result**: User approves architecture and tests, Claude writes `.gromit/plans/user-profiles.md`

### Example: Decomposing a Plan

1. **User**: `/gromit`
2. **Skill**: Shows dashboard: "Plans: 1 ready to decompose (user-profiles). Recommended: Decompose plan 'user-profiles'"
3. **User**: "decompose it"
4. **Skill**: Launches Task subagent with decompose skill + plan path
5. **Subagent**: Reads plan, applies sizing rules, returns JSON array of beads
6. **Skill**: Parses JSON, creates beads via `bd create` commands, updates plan frontmatter
7. **Skill**: "Created 6 beads for 'user-profiles'. Run `/gromit` to see updated status."

### Example: Running Beads

1. **User**: `/gromit`
2. **Skill**: Shows dashboard: "Beads: 4 ready to run. Recommended: Run next bead"
3. **User**: "run"
4. **Skill**: Executes `gromit run` via Bash
5. **Result**: Gromit CLI runs beads with fresh Claude sessions per the normal flow

## Model and Complexity

This skill uses **sonnet** for cost-effective orchestration. The skill doesn't do heavy codebase analysis — it reads state files, displays a dashboard, and dispatches to specialized stage skills that do the real work.

## Integration with Gromit Pipeline

This skill is the **orchestrator** for all pipeline stages:
1. **Capture** (`gromit add`) - Dispatched via Bash
2. **Refine** (interactive) - Dispatched via `/clear` + hook
3. **Plan** (interactive) - Dispatched via `/clear` + hook
4. **Decompose** (automated) - Dispatched via Task subagent
5. **Run** (`gromit run`) - Dispatched via Bash

The orchestrator is installed via `gromit install-skill` and registered as `/gromit` in Claude Code's skill system.

## Installation Notes

The `gromit install-skill` command handles:
- Creating `.gromit/hooks/` directory
- Writing `pipeline-resume.sh` hook script with embedded skill content extraction logic
- Writing `.claude/skills/gromit.md` with inlined skill content
- Registering SessionStart hook in `.claude/settings.json`
- Making hook script executable (`chmod +x`)

The command is idempotent — running it multiple times updates files to the latest version without breaking existing configuration.

## Tips

- **First time using `/gromit`?** The dashboard shows you exactly what's ready to work on
- **Pipeline seems empty?** Use `gromit add "your idea"` to capture something new
- **Stuck in a stage?** You can always `/clear` to start fresh (if no pipeline state exists)
- **Want to bypass the orchestrator?** Use `gromit` CLI commands directly in the terminal
- **Not sure what to do next?** The dashboard's recommendation follows the natural pipeline flow
