---
id: bead-scope-validation
source_ideas: [1]
created: 2026-02-12
epic: run-loop-reliability
---

# Bead Scope Validation

## Specification

After Claude returns bead definitions during decomposition — but before beads are created via `bd create` — a validation layer checks each proposed bead against sizing rules and flags violations. When violations are found, the system feeds them back to Claude and asks it to re-decompose the offending beads, up to a configurable retry limit.

This applies to both decomposition paths:
- **CLI decompose** (`gromit decompose`): Validates bead definitions returned by sonnet before creating beads
- **Runtime auto-decompose** (`DecomposeTask()` in the runner): Validates sub-task definitions returned by opus before calling `CreateSubBeads()`

### Validation Rules

**Structural checks** (deterministic, zero LLM cost):
1. **Criteria count**: Flag beads with more than 3 acceptance criteria
2. **Sibling overlap**: Flag beads whose acceptance criteria substantially overlap with a sibling's criteria (simple substring/similarity check)

**Heuristic checks** (keyword-based, zero LLM cost):
3. **Scope signals in title/description**: Flag beads containing phrases that historically correlate with over-scoping (e.g., "refactor entire", "update all", "across all packages", "and also"). These are soft signals — the re-prompt lets Claude decide whether the flag is valid.

### Re-prompt Flow

When violations are detected:
1. Collect all violations into a structured report
2. Send the original bead definitions plus violations back to Claude
3. Ask Claude to re-decompose only the flagged beads, keeping valid beads unchanged
4. Re-validate the new output
5. Repeat up to N times (configurable, default: 2)
6. If violations persist after retries, warn and create beads anyway (don't block the pipeline permanently)

### Bypass

- `gromit decompose --skip-validation` skips all checks
- Runtime auto-decompose respects a config flag (`decompose.validate: false`) but defaults to enabled

## Acceptance Criteria

- After Claude returns bead definitions during decompose, each bead is checked against structural rules (criteria count > 3, sibling criteria overlap) and heuristic rules (scope-signal keywords) before `bd create` is called
- When violations are found, the system re-prompts Claude with the violations and asks for re-decomposition of flagged beads, retrying up to the configured limit
- Validation runs in both the CLI `gromit decompose` path and the runner's runtime `DecomposeTask`/`CreateSubBeads` path
- `--skip-validation` flag on `gromit decompose` bypasses all checks

## Decisions

1. **Post-decompose, not pre-execution.** The original idea proposed a circuit breaker on tool call count during execution. Analysis of iteration logs showed that high tool call counts (50-84) correlated with complex but successful work, not thrashing. The 0% success rate at 100+ calls was caused by timeout limits, not scope issues. Catching bad beads at decompose time — before any compute is spent — is cheaper and more effective.

2. **Structural + heuristic, not LLM review.** An LLM-based review of bead quality was considered but rejected for V1. The re-prompt itself serves as the judgment layer: when a heuristic flags a keyword like "refactor", Claude decides during re-decomposition whether the bead is actually over-scoped. This avoids paying for a screening LLM call.

3. **Auto re-prompt, not warn-and-block.** The pipeline is designed for automation. Blocking on violations and requiring manual intervention defeats the purpose. Re-prompting keeps the pipeline moving while still enforcing quality. After exhausting retries, beads are created with a warning to avoid permanently blocking forward progress.

4. **Both decompose paths.** Runtime auto-decompose (triggered when beads fail after exhausting retries) creates sub-beads with the same lack of validation as CLI decompose. Applying validation to both paths ensures consistent enforcement regardless of how beads are created.

5. **Conservative heuristics.** Heuristic keyword lists should err on the side of fewer false positives. Each false positive triggers a full re-decompose LLM call, which is expensive. Better to miss edge cases than to re-prompt unnecessarily. The keyword list can be tuned over time based on observed false positive rates.

## Research & Context

### Current State

**CLI decompose** (`cmd/gromit/decompose.go`): Parses Claude's JSON output into `[]beadDef` structs and immediately creates beads via `bd create`. No validation between parse and create.

**Runtime decompose** (`internal/runner/runner.go:1382-1529`): `DecomposeTask()` returns `[]SubTask`, then `CreateSubBeads()` creates child beads. No validation between the two calls.

**Decompose prompt** (`PROMPT_decompose.md`): Already includes sizing guidelines (1-3 criteria, 4-5 file soft limit, grouping rules). Violations happen when Claude doesn't follow these guidelines perfectly.

**Scope gate** (`runner.go:468-502`): Existing runtime scope check that uses haiku to estimate complexity. Disabled by default. Separate concern — it estimates difficulty, not sizing rule compliance. Could be improved later but is not part of this spec.

### Evidence: Over-Scoped Beads in Practice

Analysis of iteration logs found one bead (`gromit-mz3m`: "Update learnings adapter and analyzer for Provider") that burned $10.21 across 3 failed attempts (122, 99, and 24 tool calls). This bead touched learnings, analyzer, and runner in incompatible ways — a decomposition failure that structural validation would likely have caught via criteria count or scope-signal keywords.

Meanwhile, beads with 50-84 tool calls that succeeded were doing legitimate large refactors (router wiring, stats infrastructure). The problem isn't complexity — it's badly-scoped beads that should have been split.

### Validation Integration Points

The validation function needs to be callable from two locations:
1. `cmd/gromit/decompose.go` — between JSON parse and `bd create` loop
2. `internal/runner/runner.go` — between `DecomposeTask()` return and `CreateSubBeads()` call

This suggests the validation logic should live in a shared package (e.g., `internal/decompose/` or `internal/validate/`) rather than being duplicated.
