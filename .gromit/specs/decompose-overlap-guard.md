---
id: decompose-overlap-guard
source_ideas: []
created: 2026-02-10
---

# Decompose Overlap Guard

## Specification

When Gromit decomposes a task into sub-beads, sibling beads frequently overlap — one bead implements functionality that makes another bead's acceptance tests pass immediately. This wastes iterations: the ATDD pre-pass spends 8-20 minutes per bead writing acceptance tests, verifying they fail, retrying, and ultimately marking the bead as "already done."

The decompose template (PROMPT_decompose.md) currently has no guidance about avoiding overlap. It says "each can be completed independently" but doesn't warn about the specific failure mode where one bead's implementation satisfies another bead's criteria.

Two changes are needed:

1. **Decompose template guidance**: Add explicit anti-overlap rules to PROMPT_decompose.md. Each sub-task must have acceptance criteria that are unique to that task — criteria that would NOT be satisfied by completing any sibling task. The template should instruct Claude to verify this by asking: "If I completed task N, would task M's criteria still fail?"

2. **Test-only bead suppression under ATDD**: When ATDD is active, the decompose step should NOT create standalone "write tests" beads. ATDD already handles test writing as Phase 1 of each bead. Creating a separate test bead under ATDD leads to the logical contradiction where acceptance tests for a test bead always pass (the tests ARE the implementation). The template should say: "When ATDD methodology is active, do NOT create beads whose sole purpose is writing tests — ATDD handles test writing automatically."

## Acceptance Criteria

- PROMPT_decompose.md includes anti-overlap guidance: each sub-task's acceptance criteria must not be satisfiable by completing a sibling task
- PROMPT_decompose.md includes ATDD-aware guidance: when ATDD is active, do not create test-only beads
- The ATDD-active flag is passed to the decompose template context so the template can conditionally include the guidance
- Existing decompose tests still pass

## Decisions

1. **Template guidance over code enforcement** — Preventing overlap is a judgment call best handled by prompt guidance rather than automated detection. A code-based overlap detector would need to understand acceptance criteria semantics, which is impractical. Clear instructions in the template are simpler and more maintainable.

2. **Conditional ATDD section** — The test-only bead suppression guidance should only appear when ATDD is active (using a template conditional). When ATDD is disabled, test-only beads are fine and the decomposer should be free to create them.

3. **Sibling overlap check as self-verification** — Rather than trying to detect overlap algorithmically, instruct the decomposer to perform a mental cross-check: "For each sub-task, verify that completing any other sub-task would NOT satisfy this one's acceptance criteria." This is cheap, effective, and doesn't require new code.

## Research & Context

### Current State

- `PROMPT_decompose.md` — The decompose template. Currently 63 lines with no anti-overlap guidance. The "Guidelines" section (lines 56-62) mentions "single concern" and "what files will likely be touched" but doesn't address overlap.
- `internal/prompt/prompt.go` — Renders prompts. The decompose context would need an `ATDDActive` field added to `PromptContext` or passed through the existing template data.
- `internal/runner/runner.go` — Where decomposition is triggered. The ATDD-active check already exists at line 651 using `bead.IsMethodologyActive()`.

### Evidence from Logs

From Feb 8-10 2026 runs, 9 iterations were wasted on "atdd_already_done" beads whose work was completed by siblings, and 8+ iterations failed because test-only beads under ATDD always have passing tests. Affected epics: ReadyWithLabel/label filters, epic/spec flags, and scoped run/retro commands.
