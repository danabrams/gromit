# Review System Design

Two-layer review system that catches issues early and prevents accumulation.

## Problem

Issues accumulate silently across iterations. Manual reviews conducted every few iterations find massive numbers of problems that are expensive to fix. By catching issues per-iteration and running periodic deep reviews, problems stay small and cheap to address.

## Layer 1: Light Post-Iteration Review

### Position in the Loop

```
pick bead -> build -> validate -> REVIEW -> (re-validate if fixes) -> close bead
```

Runs after every successful validation. One pass only -- no review-of-the-review loops.

### Execution

1. Render `PROMPT_review.md` with:
   - Bead details (title, description, acceptance criteria, spec)
   - Git diff from this iteration
   - Project CLAUDE.md and RULES.md
   - Validation commands already run

2. Call Claude with **sonnet** (or **opus** if the build used opus).

3. Reviewer returns structured JSON:
   ```json
   {
     "passed": true,
     "fixes_applied": ["added missing error check in handler.go"],
     "beads_to_create": [
       {"title": "...", "description": "...", "priority": 1, "labels": ["from-review"]}
     ],
     "backlog_items": [
       {"title": "...", "description": "...", "reason": "needs product owner decision"}
     ],
     "summary": "Implementation matches spec but missing input validation on email field"
   }
   ```

4. If `fixes_applied` is non-empty, re-run validation. If re-validation fails, treat as a build failure and enter retry/escalation.

5. Create beads and backlog items via `bd create`.

### Review Dimensions

1. **Intent & Spec Drift** -- Do changes fulfill the bead's intent, not just pass tests?
2. **Correctness** -- Does the code actually work beyond test coverage?
3. **Security** -- Injection, auth bypass, data exposure, OWASP concerns?
4. **Test Gaps** -- Untested code paths or edge cases?
5. **Consistency** -- Does new code match project conventions?
6. **Code Quality** -- Dead code, poor naming, missing error handling?

### Issue Triage

- **Fix immediately**: Trivial issues (missing error check, naming, dead code). Reviewer applies the fix directly.
- **Create bead**: Significant issues needing dedicated work. Reviewer assigns priority.
- **Backlog**: Issues needing design discussion or product owner input. Created at P2 with `backlog` label.

All review-created beads get a `from-review` label for traceability.

## Layer 2: Thorough Periodic Review

### Triggers

1. **Every N iterations** -- Configurable (default: 5). Counter tracked in `state.json`.
2. **After epic completion** -- When the last child bead of an epic closes.
3. **Manual** -- `ralph review` command.

### Scope

Diff-based with context:
- **N-iteration trigger**: Git diff from last thorough review commit to HEAD.
- **Epic trigger**: All files touched by the epic's child beads (from iteration logs).
- **Manual**: Since last review commit, or `--since <commit>`, or `--epic <id>`.

The reviewer reads surrounding files as needed for architectural and consistency assessment.

### Execution (Automated / Non-Interactive)

1. Determine scope and gather the diff.
2. Render `PROMPT_thorough_review.md` with:
   - Full diff within scope
   - List of beads completed since last review (titles, descriptions, specs)
   - Project CLAUDE.md, RULES.md
   - All review dimensions plus architectural assessment and cross-cutting analysis
3. Call Claude with **opus**. Longer timeout (default: 900s).
4. Same JSON response format as light review.
5. Fix minor issues, re-validate, create beads/backlog items.
6. Update `state.json` with new `last_review_commit` and `last_review_iteration`.

### Additional Thorough Review Dimensions

Beyond the light review dimensions:
- Architectural assessment across all changes
- Cross-cutting concern analysis (are the beads coherent together?)
- Pattern detection across multiple changes

## `ralph review` Command

### Interactive Mode (Default)

Launches an interactive Claude Code session with the review prompt pre-loaded:

1. Gather scope (since last review commit, or per flags).
2. Render review prompt to a temp file.
3. Execute `claude --prompt-file <path>` to start an interactive session.
4. Claude presents findings organized by severity, waits for user input on each before acting.
5. User can discuss, fix, dismiss, or create beads interactively.

### Non-Interactive Mode (`--non-interactive`)

Same as automated thorough review -- opus runs autonomously, fixes minor issues, creates beads/backlog items, re-validates.

### Flags

```
ralph review                          # interactive, since last review
ralph review --non-interactive        # automated, since last review
ralph review --epic abc-123           # scoped to epic
ralph review --since a1b2c3d          # from specific commit
ralph review --dry-run                # preview scope
```

## Configuration

```yaml
review:
  enabled: true                # light post-iteration review
  model: sonnet
  match_build_model: true      # use opus if build used opus
  timeout: 120                 # light review timeout (seconds)

  thorough:
    enabled: true
    every_n_iterations: 5
    on_epic_complete: true
    model: opus
    timeout: 900
```

## State

Additions to `.ralph/state.json`:

```json
{
  "last_review_commit": "a1b2c3d",
  "last_review_iteration": 12,
  "iterations_since_review": 3
}
```

Light reviews don't affect the thorough review counter.

## Logging

Review results logged to the run JSONL log:

```json
{
  "timestamp": "...",
  "type": "review",
  "review_type": "light",
  "iteration": 5,
  "bead_id": "abc-123",
  "model": "sonnet",
  "passed": true,
  "fixes_applied": 1,
  "beads_created": 0,
  "backlog_created": 0,
  "duration_ms": 25000
}
```

## Failure Modes

- **Review fixes break validation**: Treat as build failure, enter retry/escalation. Retry prompt includes context that the review's fix caused the failure.
- **No review-of-review loops**: One pass only. Issues the reviewer can't fix trivially become beads.
- **Empty diff**: Skip the review step, log as skipped.
- **Thorough review creates P0 beads during `ralph run`**: They get picked up next by normal `bd ready` priority ordering.
- **Disabling**: Both review types independently toggleable.

## Templates

Two new templates in `.ralph/templates/`:

- `PROMPT_review.md` -- Light post-iteration review
- `PROMPT_thorough_review.md` -- Thorough periodic review

## Architecture

New package: `internal/review/`

- `review.go` -- Core review logic (run light review, run thorough review, parse results)
- `prompt.go` -- Review prompt rendering (if needed beyond existing prompt package)

Integration points:
- `internal/runner/runner.go` -- Add review step after validation, add thorough review triggers
- `internal/state/` -- Track review state (last commit, iteration counter)
- `cmd/ralph/main.go` -- Add `review` subcommand

Single-agent execution for both review types. No parallel subagents -- cross-cutting context matters more than parallelism. Revisit if thorough reviews hit quality or timeout issues.
