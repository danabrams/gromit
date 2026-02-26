---
id: debug-command
source_ideas: []
created: 2026-02-08
epic: observability-and-diagnostics
---

# Debug Command

## Specification

`gromit debug` launches an interactive Claude Code session for investigating bugs in the target codebase. The user describes a bug — unexpected behavior, error messages, failing tests — and Claude investigates freely, reading code, tracing logic, and identifying root cause.

### Input

The command accepts an optional text argument describing the bug:

```
gromit debug                              # Blank session, describe bug interactively
gromit debug "login fails with + in email" # Pre-seeded bug description
```

### Model Selection

Default model is opus. Override with `--model`:

```
gromit debug --model sonnet "API returns 500"
```

### Context Provided

The session receives full project context at launch:
- CLAUDE.md (project conventions)
- RULES.md (constraints and non-negotiables)
- LEARNINGS.md (confirmed and recent learnings)
- Working directory path

This is the same context the build phase provides, minus bead-specific data.

### Investigation

Claude investigates the bug freely — no rigid methodology. It reads files, traces code paths, reproduces issues, checks logs, runs tests, whatever it deems appropriate. The session is interactive so the user can steer, provide additional context, or answer questions.

### Graduated Outcome

After investigation, Claude triages and takes the appropriate action:

1. **Trivial fix** — If the root cause is clear and the fix is small (a few lines), Claude fixes it directly in the session and runs the project's validation commands (tests/lint from gromit.yaml) to confirm the fix doesn't break anything.

2. **Clear fix, no design decisions** — If the root cause is well understood and the fix is straightforward but non-trivial, Claude writes an investigation report to `.gromit/reports/debug-<timestamp>.md`, creates a plan, and asks the user if they want to decompose it into beads immediately.

3. **Needs more investigation or design decisions** — If the root cause is unclear, or the fix requires design decisions, Claude writes an investigation report and adds the item to the backlog via `gromit add` with enriched context pointing to the report.

### Investigation Report Format

When Claude writes a report (outcomes 2 and 3), it follows this structure:

```markdown
# Bug Investigation: <brief title>

## Symptom
What was observed / reported.

## Root Cause
What was found (or "Under investigation" if not yet determined).

## Affected Code
Files and functions involved.

## Suggested Fix
Approach for fixing, if known.

## Evidence
Key observations, test results, log excerpts that support the analysis.
```

### Post-Session Chaining

After the session exits:
- If a plan was created, offer to decompose (same pattern as `gromit refine` chaining).
- If a backlog item was created, display its ID.
- If a fix was applied directly, show a summary of what changed.

## Acceptance Criteria

- `gromit debug` launches an interactive Claude Code session with full project context (CLAUDE.md, RULES.md, LEARNINGS.md)
- `gromit debug "description"` pre-seeds the session with the bug description
- `--model` flag overrides the default model (opus)
- When Claude fixes a bug directly, validation commands from gromit.yaml are run before the session ends
- Investigation reports are written to `.gromit/reports/` when the fix is non-trivial
- Backlog items created from investigation include a reference to the report file

## Decisions

1. **Free-form investigation over structured methodology.** Bug investigation is too varied for a rigid reproduce-isolate-fix pipeline. Claude adapts its approach to the specific bug. This keeps the prompt simple and the command flexible.

2. **Graduated outcomes based on triage.** Rather than always producing the same artifact, the command adapts its output to the situation. Trivial bugs get fixed immediately; complex bugs get properly tracked. This avoids ceremony for simple issues and ensures complex ones don't get lost.

3. **Full project context at launch.** Even though investigation is exploratory, providing RULES, LEARNINGS, and CLAUDE.md upfront means Claude already knows project conventions and known gotchas before it starts reading code. This prevents re-discovering things that are already documented.

4. **Validation after direct fixes.** When Claude fixes a bug in-session, it must run the same validation commands that `gromit run` uses. This maintains the same quality bar as automated fixes.

5. **Reports directory for investigation artifacts.** Using `.gromit/reports/` (not specs or templates) keeps investigation outputs separate from the spec/plan pipeline. Reports are reference material, not pipeline inputs.

## Research & Context

### Current State

The closest existing functionality is `internal/analyzer/`, which categorizes build failures during `gromit run`. However, the analyzer operates on Claude's failed output — not on bugs in the user's codebase. It produces structured JSON (category, root_cause, suggestion) for the retry/escalation loop, not for human consumption.

The `gromit refine` command (`cmd/gromit/refine.go`) provides the implementation pattern: it launches an interactive Claude Code session with a system prompt written to a temp file, then processes artifacts after the session exits. The debug command follows this same pattern.

### Key Files

- `cmd/gromit/refine.go` — Template for interactive session commands (prompt → temp file → claude launch → post-processing)
- `cmd/gromit/chain.go` — Chaining logic for plan → decompose → run flow
- `internal/prompt/prompt.go` — Context building (CLAUDE.md, RULES.md, LEARNINGS.md loading)
- `internal/config/` — Config loading including validation commands
- `internal/backlog/` — Backlog item creation for outcome 3
- `gromit.yaml` — Validation commands referenced for direct-fix validation
