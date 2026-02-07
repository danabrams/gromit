---
id: pipeline-chaining
source_ideas: []
created: 2026-02-06
---

# Pipeline Chaining

## Specification

After each interactive pipeline stage exits, the Go CLI offers to run the next stage automatically. This keeps the user in flow without requiring them to manually type the next command, while still giving them the option to stop at any point.

### Chaining Points

1. **refine → plan**: After `gromit refine` exits and one or more specs were created, offer to run `gromit plan` for each new spec sequentially. Default: yes (`[Y/n]`).

2. **plan → decompose**: After `gromit plan` exits and a plan was created, offer to run `gromit decompose` for the plan. Default: yes (`[Y/n]`).

3. **decompose → run**: After `gromit decompose` exits and beads were created, offer to run `gromit run`. Default: no (`[y/N]`).

### Multi-Spec Flow

When `gromit refine` creates multiple specs (A, B, C), the chaining follows a two-phase pattern:

**Phase 1 — Planning (interactive, sequential):**
- Offer to plan spec A → plan session runs → exits
- Offer to plan spec B → plan session runs → exits
- Offer to plan spec C → plan session runs → exits

**Phase 2 — Decomposition (non-interactive, sequential):**
- Offer to decompose plan A → runs
- Offer to decompose plan B → runs
- Offer to decompose plan C → runs

**Phase 3 — Run (only after all decomposition is complete):**
- Offer to run `gromit run` (`[y/N]`)

If the user declines at any point (e.g., declines to plan spec B), the remaining plans in that phase are skipped, and the flow moves to decomposition of whatever plans were successfully created. Similarly, if decomposition is declined for one plan, remaining decomposes are skipped and the run offer is made (or skipped if nothing was decomposed).

### Invocation Mechanism

Each chained stage is invoked as a fresh process via the Go CLI. This ensures:
- Fresh Claude context for each stage (no accumulated conversation bleeding through)
- Correct skill injection (handled by each command's Go code via `--append-system-prompt`)
- Interactive sessions work naturally (sequential, not nested)

### Single-Spec Flow (Common Case)

The most common path is a single spec through the full pipeline:

1. `gromit refine` → spec created
2. "Run `gromit plan <name>`? [Y/n]" → plan session
3. "Run `gromit decompose <name>`? [Y/n]" → beads created
4. "Run `gromit run`? [y/N]" → user decides

## Acceptance Criteria

- After `gromit refine` creates a spec, the CLI prompts to run `gromit plan <name>` with yes as default; answering yes launches the plan session.
- After `gromit plan` creates a plan, the CLI prompts to run `gromit decompose <name>` with yes as default; answering yes runs decompose.
- After `gromit decompose` creates beads, the CLI prompts to run `gromit run` with no as default.
- When refine creates multiple specs, all plans are offered sequentially before any decomposes, and the run offer only appears after all decomposition is complete.
- Declining a chaining prompt does not error — it exits cleanly.

## Decisions

1. **Chaining lives in Go code, not in skills/prompts.** The offer happens after the Claude session exits, at the CLI level. This avoids the problem of nesting interactive Claude sessions inside Bash tool calls, and keeps each stage's context clean.

2. **Yes-default for plan and decompose, no-default for run.** Plan and decompose are natural continuations of the pipeline. Run is different — the user may want to review beads, adjust priorities, or wait before starting the build loop.

3. **All plans before any decomposes in multi-spec flow.** Planning is interactive and may inform subsequent plans. Batching all planning first, then all decomposition, keeps the user in the right headspace for each phase.

4. **Declining skips remaining items in the current phase.** If the user declines to plan spec B, they probably don't want to plan spec C either. The flow gracefully moves to decomposing whatever plans were successfully created.

## Research & Context

### Current State

- `refine.go` already detects new spec files after Claude exits (lines ~213-220). It scans the specs directory before and after the session to find newly created files.
- `plan.go` already checks if the plan file exists after Claude exits (line ~196). It uses `os.Stat(planPath)` to verify creation.
- `decompose.go` prints a success message with bead count after creation. The plan name is known from the command argument.
- All three commands return `nil` after their post-session checks, so the chaining logic can be inserted just before the return.

### Implementation Notes

- The prompt/confirm logic can be a small shared utility (e.g., `internal/prompt/confirm.go`) that reads a single line from stdin with a default.
- Invoking the next stage can use `exec.Command("gromit", "plan", specName)` with stdin/stdout/stderr connected to the parent process, or call the command's `Run` function directly via Cobra.
- For the multi-spec case, `refine.go` already has `createdSpecs []string` — this list drives the sequential plan offers. The resulting plan names need to be collected to drive the decompose phase.
