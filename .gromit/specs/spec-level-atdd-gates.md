---
id: spec-level-atdd-gates
source_ideas: []
created: 2026-02-18
updates: [spec-acceptance-verification-loop, spec-level-atdd-execution]
epic: spec-first-atdd-execution
---

# Spec-Level ATDD Gates

## Specification

Gromit's ATDD methodology currently operates per bead: each bead gets its own acceptance test authoring, red-phase verification, green-phase build, and refactor. This ceremony adds latency and token cost to every bead, even when multiple beads implement a single spec whose acceptance criteria define the real contract.

This spec introduces a `methodology.granularity` config field that shifts ATDD from per-bead to per-spec. When set to `"spec"`, the runner:

1. Authors acceptance tests once, before the first bead of a spec executes
2. Suppresses per-bead ATDD entirely
3. Runs a spec gate after the last bead closes to verify acceptance criteria
4. Synthesizes fix beads from failures and re-gates, up to a configurable limit

Per-bead TDD remains active regardless of granularity.

### Granularity Mode

A new `methodology.granularity` field accepts `"bead"` (default) or `"spec"`. When `"bead"`, nothing changes. When `"spec"`:

- `IsMethodologyActive()` returns `false` for ATDD on any bead with a `spec:` label. Per-bead ATDD phases are skipped.
- The runner tracks which specs have had acceptance tests authored during the current run (in-memory state, resets each run).
- Beads without a `spec:` label still follow the global `methodology.atdd` setting.

### Per-Spec Acceptance Test Authoring

Before the first bead of a spec executes, the runner loads `.gromit/specs/<name>.md`, extracts the acceptance criteria, and invokes an LLM to write acceptance tests. The LLM uses the existing `PROMPT_acceptance_tests.md` template (or a spec-scoped variant) with the full spec context. Tests are tagged (`-tags acceptance`), committed to git, and the spec is marked as "tests authored" in run state.

This happens once per spec per run. Subsequent beads for the same spec skip authoring.

### Spec Gate Verification

When the last open bead for a `spec:<name>` closes, the runner triggers the spec gate. The gate:

1. Runs acceptance tests scoped to packages touched by the spec's beads
2. Collects the cumulative git diff across all spec beads
3. Renders `PROMPT_spec_gate.md` with test output, diff, and acceptance criteria
4. Sends to an LLM, which returns a structured JSON verdict: pass/fail per criterion with evidence

The existing `spec-acceptance-verification-loop` spec defines gate internals, package structure, and verdict format.

### Fix Bead Synthesis

Each failing criterion produces one fix bead labeled `spec:<name>`. The runner picks these up on subsequent iterations. After fix beads close, the gate re-triggers. This loop repeats up to `spec_gate.max_cycles` times. If the limit is reached, the runner logs the failure and moves on.

### CLI

`gromit verify-spec <name>` runs the gate standalone. `--create-beads` controls whether fix beads are created on failure.

`gromit run --spec <name>` filters beads to `spec:<name>` labels, enabling focused spec execution. Without this flag, the runner processes all ready beads but still tracks spec completeness and triggers gates.

## Config

```yaml
methodology:
  atdd: true
  tdd: true
  granularity: "spec"   # "bead" (default) | "spec"

spec_gate:
  enabled: true
  max_cycles: 3          # max gate-fix-re-gate cycles per spec per run
  model: "sonnet"
  auto_trigger: true     # fire gate when last spec bead closes
```

`methodology.granularity: "spec"` requires `methodology.atdd: true` to have effect. If `atdd` is false, granularity is irrelevant since no ATDD runs at any level.

## Acceptance Criteria

- `methodology.granularity` accepts `"bead"` (default) or `"spec"`
- When `granularity: "spec"`, per-bead ATDD phases are suppressed for beads with `spec:` labels
- Beads without `spec:` labels follow the global `methodology.atdd` setting regardless of granularity
- Acceptance tests are authored once per spec before the first bead executes
- The spec gate auto-triggers when the last bead for a spec closes (if `auto_trigger` is true)
- Failed criteria produce fix beads labeled `spec:<name>` with inherited priority
- The gate-fix-re-gate loop respects `max_cycles`
- `gromit verify-spec <name>` runs the gate standalone with optional `--create-beads`
- `gromit run --spec <name>` filters bead selection to the named spec
- Per-bead TDD is unaffected by granularity setting
- When `granularity: "bead"` (default), all behavior is unchanged

## Decisions

1. **Mode switch, not layering.** Spec granularity replaces per-bead ATDD rather than adding a second acceptance layer. Running both wastes tokens on redundant verification.

2. **Suppression by label presence.** The runner checks for `spec:` labels to decide suppression. Beads not linked to a spec still get per-bead ATDD if globally enabled. This avoids an all-or-nothing switch.

3. **Separate `spec_gate` config section.** Gate configuration (cycles, model, auto-trigger) is operationally distinct from methodology selection (ATDD/TDD/granularity). Separating them keeps each section focused.

4. **One authoring pass per spec per run.** Re-authoring acceptance tests mid-run wastes tokens and risks inconsistency. The spec's criteria are stable within a run.

5. **Umbrella over existing specs.** This spec defines the full lifecycle. `spec-acceptance-verification-loop` defines gate internals. `spec-level-atdd-execution` defines the broader execution model. Both remain as implementation-level detail.

## Lifecycle Flow

```
gromit run [--spec my-feature]
  |
  +- getNextBead() -> bead with spec:my-feature label
  |
  +- First bead for this spec?
  |   +- YES: runSpecAcceptanceAuthoring(specName)
  |       -> Load .gromit/specs/my-feature.md
  |       -> LLM writes acceptance tests (tagged, committed)
  |       -> Mark spec as "tests authored" in run state
  |
  +- processBead() -- normal build + validation
  |   +- Per-bead ATDD suppressed (atddActive = false)
  |   +- TDD still active if configured
  |
  +- handleSuccessfulIteration()
  |   +- Last bead for spec:my-feature?
  |       +- YES: maybeRunSpecGate("my-feature")
  |           -> Run acceptance tests
  |           -> Collect cumulative diff
  |           -> Render PROMPT_spec_gate.md -> LLM
  |           -> Parse JSON verdict
  |           -> PASS: spec complete
  |           -> FAIL + cycles < max:
  |               -> SynthesizeFixBeads()
  |               -> Runner picks them up next iteration
  |               -> Gate re-triggers after fix beads close
  |           -> FAIL + cycles >= max:
  |               -> Log failure, stop processing this spec
  |
  +- Loop continues with next bead
```

## Related Specs

- `spec-acceptance-verification-loop` (gate internals, phases 3-4)
- `spec-level-atdd-execution` (broader execution model)
- `atdd-simplification` (per-bead ATDD simplification)
- `run-scope-flags` (--spec/--epic CLI flags)
