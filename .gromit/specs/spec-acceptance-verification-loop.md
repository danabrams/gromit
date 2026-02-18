---
id: spec-acceptance-verification-loop
source_ideas: []
created: 2026-02-18
updates: [spec-level-atdd-execution]
---

# Spec Acceptance Verification Loop

## Specification

After all beads for a spec close, run a hybrid acceptance gate that checks executable acceptance tests and then uses an LLM to review remaining non-executable acceptance criteria. Failing criteria become targeted fix beads that feed back into the same scoped run. The loop repeats until the gate passes or a configurable retry limit is exhausted.

This implements phases 3 and 4 of `spec-level-atdd-execution`: the spec acceptance gate and failure-to-bead synthesis.

## Workflow

1. Detect that the last open bead for a `spec:<name>` scope has closed.
2. Run acceptance tests (`go test -tags acceptance ./...`) scoped to packages touched by the spec's beads.
3. Feed acceptance criteria, test output, and cumulative diff to a sonnet-tier LLM. LLM returns a structured verdict: pass/fail per criterion with evidence.
4. If all criteria pass, the spec gate passes. Done.
5. If any criteria fail, create one fix bead per failing criterion with `spec:<name>` label.
6. The scoped run loop picks up fix beads naturally via `getNextBead()`.
7. After fix beads close, re-trigger the gate (step 1). Repeat up to `max_cycles`.

## Architecture

New package `internal/specgate/` encapsulates gate logic. Two consumers:

- **Runner**: auto-triggers in `handleSuccessfulIteration()` when the last spec bead closes and `spec_gate.auto_trigger` is enabled.
- **CLI**: `gromit verify-spec <name>` runs the gate once and prints the verdict. `--create-beads` flag controls whether fix beads are created.

### Package structure

```
internal/specgate/
  gate.go          — Gate struct, Run() orchestration
  verdict.go       — CriterionResult, GateVerdict types
  synthesize.go    — fix bead creation from failing criteria
```

### Config

```yaml
spec_gate:
  enabled: true
  max_cycles: 3
  model: "sonnet"
  auto_trigger: true
```

`SpecGateConfig` added to the top-level `Config` struct.

### Prompt template

Extend existing `PROMPT_spec_gate.md` to request structured JSON output covering both test results and non-executable criteria. Add a `SpecGateVerificationContext` that carries test output, cumulative diff, and the full acceptance criteria text.

### Fix bead synthesis

For each failing criterion, call `beads.CreateWithParentAndDescription()` with:
- Title: `"Fix: <criterion summary>"`
- Labels: `spec:<name>`
- Priority: inherited from the spec's existing beads
- Description: criterion text, evidence of failure, and LLM-suggested fix direction

### Runner integration

In `handleSuccessfulIteration()`, after closing a bead:
1. If `spec_gate.auto_trigger` is enabled and this is a scoped run (`--spec`), check `beads.ReadyWithLabel("spec:<name>")`.
2. If no open beads remain, call `specgate.Run()`.
3. If the gate fails, fix beads are already created. The loop continues.
4. Track cycle count in per-run state. If `>= max_cycles`, log the failure and stop processing this spec.

## Acceptance Criteria

- A `spec_gate` config section exists with `enabled`, `max_cycles` (default 3), `model` (default sonnet), and `auto_trigger` (default true) fields.
- When the last bead for a spec closes during a scoped run, the spec acceptance gate fires automatically if `auto_trigger` is true.
- The gate runs acceptance tests scoped to touched packages and produces hard pass/fail from exit codes.
- The gate feeds acceptance criteria + test output + diff to an LLM that returns a structured per-criterion verdict.
- Each failing criterion produces exactly one fix bead with `spec:<name>` label, inherited priority, and descriptive title/body.
- Fix beads are picked up by subsequent loop iterations without special orchestration.
- The gate re-triggers after fix beads close, up to `max_cycles` times.
- `gromit verify-spec <name>` runs the gate standalone with optional `--create-beads`.
- Existing non-scoped runs are unaffected when `spec_gate` is disabled or no `--spec` flag is set.

## Decisions

1. Hybrid verification: acceptance tests for executable criteria, LLM review for the rest. Neither alone is sufficient.
2. One fix bead per failing criterion. Keeps beads focused and independently closeable.
3. Sonnet tier for the LLM gate review. Adequate for structured judgment; opus is overkill, haiku too weak.
4. Configurable retry limit rather than unbounded looping. Default 3 cycles balances convergence with cost.
5. Separate `internal/specgate/` package rather than inline runner logic. Enables the standalone CLI command and independent testing.

## Related Specs

- `spec-level-atdd-execution` (parent spec, phases 3+4)
- `run-scope-flags`
- `atdd-simplification`
