---
id: decomposition-granularity
source_ideas: [idea-1770482150013]
created: 2026-02-11
---

# Decomposition Granularity

## Specification

Gromit's bead sizing rules are too fine-grained for how Claude naturally works. The current rules ("one concern per bead, max 2 files, 1-3 acceptance criteria") treat files as the unit of complexity, but Claude thinks in terms of working features. This mismatch causes sibling beads to overlap: the first bead implements the complete working unit, and subsequent siblings find their work already done.

Log analysis from Feb 5-10 2026 shows 50+ wasted iterations from this problem, costing an estimated $15-30 in API fees. The worst case was epic-scoped execution, where 6 sibling beads all found work already done.

The fix is to redefine bead sizing around **natural implementation units** — the smallest change that compiles, passes tests, and delivers observable behavior — rather than strict file counts.

### New Bead Sizing Rules

Replace the current sizing rules across all documents (CLAUDE.md, SKILL.md, RULES.md, PROMPT_decompose.md) with:

1. **One deliverable behavior per bead** — a single observable change that a caller or user could verify. Not "one file" or "one concern," but one unit of working functionality.

2. **1-3 acceptance criteria** — concrete, testable criteria only. This rule stays unchanged; it constrains scope effectively without constraining files.

3. **Soft file limit of 4-5** — if touching 6+ files across unrelated packages, consider splitting. But touching interface.go, impl.go, mock_test.go, and impl_test.go for one method addition is fine — that's one change, not four.

4. **Self-contained** — understandable without reading other beads.

5. **No ambiguity** — Claude implements without making design decisions.

6. **Clear definition of done** — each criterion has an obvious pass/fail test.

### Grouping Rules (Never Split These)

Based on the five overlap patterns observed in logs:

- **Interface + implementation + mock updates** — In Go, changing an interface requires updating all implementations and mocks to compile. This is one change, not three beads.

- **Implementation + its tests** — Claude writes tests alongside implementation. Under ATDD, they're explicitly the same workflow. Never create a separate "write tests for X" bead.

- **Companion methods in the same package** — Methods that follow the same pattern in the same file (e.g., `ReadyWithLabel` and `ListWithLabel`) are one bead. If you'd copy-paste-modify to create the second, they belong together.

- **Command flags + the wiring that makes them work** — A CLI flag that does nothing isn't a deliverable. The flag, its plumbing through to the runner, and its effect are one bead.

- **Template + its registration** — Adding a template file and registering it in the renderer are one action, not two.

### Splitting Rules (When to Split)

- If a bead has 4+ acceptance criteria, split by distinct behaviors
- If a bead touches 6+ files across unrelated packages, split by package boundary
- If two parts of a bead are independently useful and don't need each other to compile, they can be separate beads
- If a bead requires design decisions that would benefit from being settled first (e.g., define the data model, then build the API), split at the decision boundary

### Changes to Decompose Skill (SKILL.md)

Update the bead sizing rules section, splitting logic, and examples to reflect the new philosophy. Specifically:

- Remove "max 2 files touched" and "split by file" guidance
- Remove "if a task has both implementation and tests, consider separate beads"
- Add the grouping rules (never-split patterns)
- Update examples to show coarser beads (e.g., interface + impl + tests as one bead)

### Changes to Decompose Template (PROMPT_decompose.md)

Update the Guidelines section to use the new sizing philosophy. The existing anti-overlap guidance stays but is reinforced by the grouping rules.

### Changes to Project Docs (CLAUDE.md, RULES.md)

Update the "Bead Sizing" section in CLAUDE.md and the process section in RULES.md to reflect the new file limit and grouping rules.

## Acceptance Criteria

- CLAUDE.md bead sizing section reflects the new rules: deliverable behavior (not file count), soft limit of 4-5 files, and grouping rules for never-split patterns
- RULES.md process section updated from "more than 2 files" to "6+ files across unrelated packages"
- SKILL.md sizing rules, splitting logic, and examples updated to match the new philosophy — no "max 2 files," no "separate beads for implementation and tests"
- PROMPT_decompose.md guidelines updated with the new sizing philosophy and grouping rules
- Existing anti-overlap guidance and ATDD test-only suppression in PROMPT_decompose.md remain intact

## Decisions

1. **Deliverable behavior over file count** — Files are the wrong unit of complexity. A method addition that touches 4 files (interface, impl, mock, test) is simpler than a 1-file change that introduces a new subsystem. The new rules optimize for Claude's natural work units rather than filesystem structure.

2. **Soft file limit, not hard** — The old "max 2 files" was treated as a hard rule and caused aggressive splitting. The new "4-5, consider splitting at 6+" is a guideline. The real constraint is acceptance criteria count (1-3), which effectively caps scope without micromanaging file touches.

3. **Explicit never-split groupings** — Rather than relying on the decomposer's judgment alone, we enumerate the five patterns that must never be split. These come directly from log evidence of the most common overlap failures.

4. **Documentation-only changes** — This spec changes prompt templates and documentation, not Go code. The decomposition logic itself doesn't enforce file counts — it follows prompt instructions. Changing the instructions is sufficient.

5. **Builds on overlap guard, doesn't replace it** — The decompose-overlap-guard spec added detection guidance (cross-check sibling criteria) and ATDD test-only suppression. This spec addresses the root cause by changing the sizing philosophy that produces the overlap in the first place. Both are complementary.

## Research & Context

### Current State

Bead sizing rules exist in four places, all with the same "max 2 files" philosophy:
- `CLAUDE.md` lines 63-69 — project-wide bead sizing section
- `.gromit/RULES.md` line 39 — "Beads that touch more than 2 files should be split"
- `skills/gromit-decompose/SKILL.md` lines 34-50 — decompose skill sizing rules and splitting logic
- `.gromit/templates/PROMPT_decompose.md` lines 55-78 — runtime decompose template guidelines

The decompose-overlap-guard spec (implemented Feb 10) added anti-overlap cross-checking and ATDD test-only bead suppression to PROMPT_decompose.md. These address detection but not root cause.

### Evidence from Logs (Feb 5-10 2026)

| Overlap Pattern | Instances | Cost Per Instance | Example |
|---|---|---|---|
| Test-only sibling | ~15 | 10-20 min (ATDD) | ReconcileFilteredHashes impl + separate test bead |
| Tightly-coupled feature split | ~10 | 8-20 min each | Epic-scoped execution: 6 beads all already done |
| Companion method split | ~5 | 15-30s (precheck) | ReadyWithLabel + ListWithLabel as separate beads |
| Interface + mock + impl cascade | ~5 | 8-20 min each | BeadClient interface + mock + impl as 3 beads |
| Template + registration | ~3 | 15-30s (precheck) | Add template file + register in renderer |

Total: ~50+ wasted iterations, estimated $15-30 in API costs.

### Relationship to Other Specs

- **decompose-overlap-guard** — Complementary. That spec handles detection; this spec handles prevention.
- **precheck-already-done** — That spec adds a haiku pre-check to skip beads whose work is already done. With better granularity, fewer beads will need pre-checking in the first place.
