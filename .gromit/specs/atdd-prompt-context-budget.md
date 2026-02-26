---
id: atdd-prompt-context-budget
source_ideas: []
created: 2026-02-15
epic: spec-first-atdd-execution
---

# ATDD Prompt Context Budget

## Specification

ATDD acceptance-test generation is often slow because the acceptance prompt carries too much static context. A recent dry-run measurement for bead `gromit-2s2q` showed:

- Baseline acceptance prompt: `38,544` chars
- Rules contribution: `~12,897` chars
- Spec contribution: `~10,059` chars
- CLAUDE.md contribution: `~5,343` chars
- Confirmed learnings contribution: `~2,995` chars

Existing work (`gromit-kse3`) removes explicit CLAUDE.md from templates. That helps, but does not address the largest contributors (rules + spec), and does not define a hard budget mechanism.

This spec adds explicit ATDD prompt budgeting and context selection rules so acceptance generation remains fast and predictable while preserving correctness.

## Goals

1. Reduce ATDD acceptance prompt size materially (target: 30-50% reduction on large-context beads).
2. Keep acceptance generation behaviorally correct (no loss of required task/scenario coverage).
3. Make prompt size/selection observable so regressions are easy to detect.

## Non-Goals

1. Changing build-phase prompt behavior (this spec is ATDD-acceptance focused).
2. Rewriting RULES.md authoring model across all phases.
3. LLM-based summarization/compression at runtime.

## Design

### 1) Add ATDD context controls in config

Add a dedicated config block (names can vary, behavior is required):

- `methodology.atdd.prompt.max_chars` (0 = unlimited)
- `methodology.atdd.prompt.include_rules` (default true)
- `methodology.atdd.prompt.include_spec` (default true)
- `methodology.atdd.prompt.include_claude_md` (default false once kse3 lands)
- `methodology.atdd.prompt.max_confirmed_learning_chars` (default small cap, e.g. 2000-3000)

These controls apply only to acceptance-test prompt rendering.

### 2) Add an ATDD-specific context shaper before RenderAcceptanceTests

Before rendering `PROMPT_acceptance_tests.md`, create a shaped copy of `prompt.Context`:

1. Start with full context.
2. Apply include toggles (rules/spec/claude/learnings).
3. Apply learning cap.
4. If `max_chars > 0` and rendered size exceeds budget, trim in deterministic priority order:
   - Drop recent learnings
   - Reduce confirmed learnings to fit cap / then drop
   - Replace full rules with short ATDD rule subset
   - Truncate spec with clear marker + retain head/tail
   - As last resort, omit rules entirely
5. Never drop current bead ID/title/description.

The trim order must be deterministic and testable.

### 3) Provide ATDD rules subset

For acceptance-test writing, include only rules relevant to:

- test quality
- no implementation edits during acceptance phase
- expected-failure discipline

Do not inject the full general build ruleset unless explicitly configured.

### 4) Observability

Emit per-attempt ATDD context stats in normal logs:

- `prompt_chars_before`
- `prompt_chars_after`
- `trim_actions` applied
- key section sizes (rules/spec/claude/learnings)

This should appear once per ATDD acceptance invocation (not only under debug mode).

## Acceptance Criteria

1. ATDD acceptance prompt rendering supports config-driven inclusion/exclusion for rules/spec/CLAUDE/learnings.
2. ATDD acceptance rendering supports a character budget and deterministic trim order.
3. For a large-context bead fixture, rendered acceptance prompt size is reduced by at least 30% under default ATDD budget settings.
4. Logs include ATDD prompt sizing and trim-action metadata for each acceptance attempt.
5. Existing `gromit-kse3` behavior remains compatible (explicit CLAUDE.md removal still valid).
6. Unit tests cover:
   - include/exclude toggles
   - trim priority order
   - budget enforcement
   - no-loss of mandatory bead identity fields

## Implementation Notes

- Primary touchpoints:
  - `internal/prompt/prompt.go` (context building / rendering helpers)
  - `internal/runner/callbacks.go` (ATDD invoke path)
  - `internal/config/config.go` (new config fields + defaults)
  - `.gromit/templates/PROMPT_acceptance_tests.md` (aligned with shaped context)
- Keep trimming mechanical; avoid adding extra model calls.

