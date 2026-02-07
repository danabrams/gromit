---
id: precheck-already-done
source_ideas: []
created: 2026-02-07
priority: high
---

# Pre-Check: Skip Already-Completed Beads

## Specification

Before processing a bead, the runner performs a lightweight pre-check using Claude haiku to determine whether the bead's acceptance criteria are already satisfied by the current codebase. If they are, the bead is auto-closed and the iteration is skipped entirely.

### Flow

1. Runner picks up the next ready bead from `bd ready` (existing behavior).
2. **New step**: Runner invokes Claude haiku with a pre-check prompt containing the bead's title, description, and acceptance criteria.
3. Claude examines the codebase against the acceptance criteria and responds with a clear `PRECHECK_PASSED` or `PRECHECK_NOT_MET` signal.
4. If `PRECHECK_PASSED`: the runner logs `"Pre-check: acceptance criteria already met, auto-closing bead <id>"`, closes the bead via `bd close`, and continues to the next bead without counting it as a full iteration.
5. If `PRECHECK_NOT_MET`: the runner proceeds with normal `processBead` flow (existing behavior, no change).

### Pre-Check Prompt

A new template `PROMPT_precheck.md` is added. It receives the bead's title, description, acceptance criteria, and parent context (if any). The prompt instructs Claude to:

- Read relevant files in the codebase to check each acceptance criterion
- Not make any changes — read-only inspection
- Output `PRECHECK_PASSED` if ALL criteria are already satisfied
- Output `PRECHECK_NOT_MET` if ANY criterion is not yet satisfied
- Err on the side of `PRECHECK_NOT_MET` when uncertain — a false negative just means normal processing, while a false positive would skip needed work

### Integration Point

The pre-check runs in `runner.go` between the stuck-bead check and the call to `processBead`. It uses the same Claude client but always with the haiku model regardless of bead priority or complexity labels.

### Logging

- Skipped beads are logged to the console: `"Pre-check: acceptance criteria already met, auto-closing bead <id>"`
- Skipped beads are recorded in the iteration log with a distinct outcome (e.g., `precheck_skipped`) so users can see how many iterations were saved
- The pre-check result (passed/not-met) is logged even when not skipping, for observability

### Always On

The pre-check runs for every bead unconditionally. There is no configuration toggle. The cost of a haiku invocation (~$0.01-0.05) is negligible compared to the cost of a wasted full iteration ($0.50-1.00+).

## Acceptance Criteria

- Before each bead is processed, a haiku pre-check runs that examines the codebase against the bead's acceptance criteria
- If all acceptance criteria are already met, the bead is auto-closed and skipped without running a full build iteration
- Skipped beads produce a clear log message: `"Pre-check: acceptance criteria already met, auto-closing bead <id>"`
- Skipped beads are recorded in the iteration log with outcome `precheck_skipped`
- The pre-check never makes code changes — it is read-only
- A new prompt template `PROMPT_precheck.md` is created and registered in `gromit init`

## Decisions

1. **Always on, no config toggle.** The cost ratio makes it a no-brainer — a haiku check costs ~1-5% of the iteration it could save. There's no scenario where you'd want to skip it.

2. **Haiku model always.** The pre-check is a simple "do these criteria match the code" question. Haiku is sufficient and keeps cost minimal. No escalation or model selection needed.

3. **Err on the side of NOT_MET.** A false negative (says not done when it is) just means we run the normal iteration — the current behavior. A false positive (says done when it isn't) would skip needed work. The prompt explicitly instructs Claude to be conservative.

4. **Don't count pre-check skips as iterations.** If a user runs `gromit run -n 5`, they expect 5 beads worth of real work. Skipped beads shouldn't consume that budget.

5. **Pre-check skips still close the bead.** The bead's work is done — it should be closed via `bd close` and synced, just like a successful iteration.

## Research & Context

### Current State

The runner loop lives in `internal/runner/runner.go` (main loop at line ~240) and `internal/runner/process.go` (bead processing). The integration point is `runner.go:338`, after the stuck-bead check and before `processBead`.

The existing scope check (`runner.go:102-114`, `process.go:92-123`) provides a pattern for a similar pre-processing step — it also invokes Claude with a lightweight prompt before the main build.

Claude invocation happens through `internal/claude/claude.go`. The `Run` method (non-streaming) would be appropriate for the pre-check since we don't need streaming or heartbeat monitoring for a quick haiku call.

Prompt rendering lives in `internal/prompt/` — a new `RenderPrecheck` method and template will follow the same pattern as existing `RenderBuild`, `RenderValidate`, etc.

Templates are registered in `cmd/gromit/init.go` and live in `.gromit/templates/`.

### Problem Evidence

Log analysis shows ~50% of runs contain "already completed" iterations. The most common cause is sibling beads from the same decomposition — the first bead implements multiple concerns, and subsequent siblings find their work already done. Each wasted iteration costs 60-180 seconds and $0.50-1.00+ in API costs.
