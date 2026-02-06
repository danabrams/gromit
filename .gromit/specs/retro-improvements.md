# Retro Improvements Spec

## Problem

The `ralph retro` command has two issues:

1. **Blind analysis** — stuck bead data only includes failure counts, not current status or comments. This causes wrong root cause hypotheses (e.g., analyzing closed beads as stuck, hypothesizing about missing features that exist).

2. **Weak interaction model** — interactive mode is just y/n prompts. User can't discuss, modify, or explore alternatives.

## Solution

### Data Gathering Fix

Enrich stuck bead data by calling `bd show <id> --json` for each stuck bead to get:
- Current status (open/closed)
- Close reason (if closed)
- Comments

Filter out closed beads from the stuck list entirely.

### New Interaction Model

Replace the y/n interactive review with launching Claude Code:
1. Run analysis via Claude API (opus) — same as today
2. Launch `claude` with analysis results as prompt argument
3. User interactively decides what to apply in Claude Code session
4. Claude Code can: edit RULES.md, edit LEARNINGS.md, run bd commands, create spec files

The `--non-interactive` flag keeps its current behavior (write proposals to file).

### What Gets Removed

- `internal/retro/interactive.go` — deleted entirely
- `ApplyAccepted()` and all `apply*()` methods in `retro.go` — Claude Code edits files directly
- Proposal parsing from the default (interactive) path in `cmd/ralph/main.go`

### What Stays

- `proposals.go` — still needed for `--non-interactive` mode
- Claude API analysis call — still generates the analysis
- `--non-interactive` flag and its file-writing behavior

## Files Affected

- `internal/retro/retro.go` — add bead enrichment, remove apply methods, add Claude Code launch
- `internal/retro/interactive.go` — delete
- `internal/retro/proposals.go` — keep, no changes
- `internal/logger/logger.go` — extend BeadStats struct
- `.ralph/templates/PROMPT_retro.md` — filter closed beads, show comments
- `cmd/ralph/main.go` — simplify runRetro()
