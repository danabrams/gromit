---
id: tdd-cycle-coverage-tracker
source_ideas: []
created: 2026-02-19
depends_on:
  - tdd-fresh-context-per-cycle
epic: observability-and-diagnostics
---

# Hybrid Spec Coverage Tracker for TDD Cycles

## Specification

The runner tracks which acceptance criteria have been covered during TDD cycles using a hybrid approach: Claude self-reports which criteria it targets during each red phase, and a lightweight haiku invocation validates the mapping after each green phase. Disagreements between Claude's self-report and the runner's checklist trigger additional cycles.

This determines when to stop cycling — replacing the current heuristic of "Claude says it's done" with verified coverage tracking.

### Criteria Parsing

At the start of TDD cycles, the runner parses the spec's `## Acceptance Criteria` section into a numbered checklist:

1. Read the spec file
2. Extract lines between `## Acceptance Criteria` and the next `##` header
3. Split on `- ` prefix (single indentation level — no nesting exists in current specs)
4. Assign each criterion a sequential number
5. Initialize all as unchecked

This is pure string parsing — no LLM call needed. The format is consistent across all specs (bullet points, single level).

### Red Phase Integration

The red phase prompt includes the current coverage state:

```
## Coverage State
Targeting criterion #3: "Users can reset their password via email."
Remaining uncovered: #3, #5, #7

Write a test that verifies criterion #3.
```

The red phase output must include a structured block:

```json
{"targeting": 3, "remaining": [5, 7]}
```

The runner parses this to track Claude's self-reported progress.

### Green Phase Validation

After each green phase passes (test is green), the runner sends a lightweight haiku invocation:

```
Here is a test:
[test code]

Here is the criterion it claims to cover:
#3: "Users can reset their password via email."

Does this test actually verify this criterion? Answer with:
{"covers": true/false, "reason": "one sentence explanation"}
```

If `covers: true`, the runner checks off criterion #3. If `covers: false`, the criterion stays unchecked and the runner notes the gap for the next red cycle.

This validation call is cheap (haiku, small prompt, structured output) and catches cases where Claude writes a tangentially related test instead of one that actually verifies the criterion.

### Disagreement Handling

When Claude's red phase says "nothing remaining" but the runner's checklist still has unchecked items:

1. Runner injects another red cycle with: "These criteria are still uncovered: #5, #7. Write a test for #5."
2. Claude cannot self-terminate the cycle — only the runner's checklist determines completion.

When the haiku validator repeatedly rejects coverage claims for a criterion (2+ rejections):

1. Runner flags the criterion as "possibly untestable" in the cycle log
2. Continues to the next criterion rather than looping forever
3. Reports uncovered criteria in the bead result for human review

### Cycle Termination

The runner stops cycling when any of:

- All criteria are checked off (both self-reported and haiku-validated)
- Max cycle count reached (default: 10, configurable)
- A cycle fails after retry/escalation
- All remaining criteria are flagged as "possibly untestable"

### Compound Criteria

Some acceptance criteria contain multiple testable statements joined by semicolons or commas (e.g., "`Emitter.Subscribe()` returns a buffered channel; `Emit()` sends to all subscribers non-blocking; `Close()` closes all channels"). The parser treats these as single criteria. The haiku validator assesses whether the test covers the criterion as a whole. If the test only covers part, the validator can report `covers: false` with a reason, and the criterion stays unchecked for another cycle.

### Out of Scope

- Automatic spec rewriting when criteria are untestable
- Coverage tracking for non-TDD builds
- Integration with external test coverage tools (line/branch coverage)

## Acceptance Criteria

- Runner parses spec acceptance criteria into a numbered checklist at the start of TDD cycles
- Red phase prompt includes current coverage state (targeting, remaining)
- Red phase output includes structured coverage self-report
- After each green phase, a haiku invocation validates whether the test covers the claimed criterion
- Runner checklist determines cycle termination, not Claude's self-report alone
- When Claude says "done" but checklist has unchecked items, runner injects another red cycle
- Criteria rejected by haiku validator 2+ times are flagged as "possibly untestable" and skipped
- Uncovered/untestable criteria are reported in the bead result
- Cycles stop when all criteria are checked, max cycles reached, or all remaining are untestable
- Unit tests cover:
  - spec criteria parsing from bullet-point format
  - coverage state tracking across cycles
  - disagreement handling (Claude says done, runner says not)
  - haiku validator rejection and untestable flagging
  - cycle termination conditions

## Decisions

1. **Hybrid over pure self-report.** Claude's self-assessment is unreliable for coverage completeness. The haiku validation step catches false positives cheaply. The runner's checklist is authoritative.

2. **Haiku for validation, not the build model.** Validation is a simple yes/no classification task. Haiku is fast, cheap, and sufficient. No need to use the build model's tier for this.

3. **Parser treats compound criteria as single items.** Splitting semicolon-separated statements would require heuristics that break on edge cases. Treating them as units and letting the validator assess holistic coverage is simpler and more robust.

4. **"Possibly untestable" escape hatch.** Some criteria may describe non-functional requirements, documentation, or config defaults that don't map to unit tests. Rather than looping forever, flag and move on. Human review catches genuine gaps.

5. **Runner controls termination.** Claude cannot end the cycle loop — only the runner's checklist and max-cycle limit can. This prevents premature termination from optimistic self-reports.
