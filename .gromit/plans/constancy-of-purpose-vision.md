---
created: 2026-02-28T00:00:00Z
decomposed: true
decomposed_at: "2026-02-28T03:55:15Z"
id: constancy-of-purpose-vision
source_spec: constancy-of-purpose-vision
---

# Constancy of Purpose Vision Artifact Implementation Plan

**Goal:** Establish `VISION.md` as the durable two-year strategic north star and wire operational guidance to it for alignment checks.

**Architecture:** Docs-only implementation: strengthen `VISION.md` as a clear strategy contract and add an explicit cross-reference from `RULES.md` so guidance changes are evaluated against vision.

**Tech Stack:** Markdown documentation, repository documentation conventions

**Spec:** `.gromit/specs/constancy-of-purpose-vision.md`

---

## Architecture

**Overview:**
Use a lightweight documentation architecture where `VISION.md` serves as the canonical long-horizon contract and `RULES.md` references it as the strategic alignment source for day-to-day governance updates.

**Key Components:**
1. **Vision Contract (`VISION.md`)**: Defines mission and intended outcomes, observable two-year system characteristics, non-negotiable guardrails, and alignment evaluation criteria.
2. **Operational Link (`RULES.md`)**: Explicitly references `VISION.md` as the source used to evaluate whether proposed rules/learnings/spec changes align with long-term direction.
3. **Scope Boundary Language (`VISION.md`)**: Clarifies that vision content is strategic and durable, not sprint planning, implementation checklists, or release roadmaps.

**Integration Points:**
- `VISION.md` remains the strategic document at repo root.
- `RULES.md` becomes the explicit operational entry point that points maintainers to `VISION.md` before guidance updates.
- Existing specs in `.gromit/specs/` and learnings in `LEARNINGS.md` are covered by the alignment contract language.

**Data Flow:**
Maintainer proposes a rules/learnings/spec change -> maintainer checks alignment against `VISION.md` principles and guardrails -> if aligned, proceed -> if misaligned, include explicit written justification and scope rationale.

**Files to Modify:**
- `VISION.md` - ensure all required sections are explicit and that strategic-vs-operational boundary language is unambiguous.
- `RULES.md` - add explicit reference to `VISION.md` as strategic alignment source for guidance updates.

**Files to Create:**
- None.

**Tradeoffs:**
- **Reference placement**: Choose `RULES.md` over `README.md` for the required reference because `RULES.md` is the most direct policy surface for maintainers making guidance changes.
- **Change scope**: Choose focused edits over broad rewrites to preserve useful current vision content while meeting acceptance criteria exactly.
- **Enforcement mode**: Choose an explicit documented alignment check (soft gate with justification path) over hard process blocking to maintain velocity while preventing unexamined local optimization.

## Test Strategy

**Test Levels:**
1. **Content Verification (document-level):** Verify `VISION.md` explicitly contains mission/outcomes, desired system characteristics, non-negotiable guardrails, and alignment assessment guidance.
2. **Cross-Document Integration Verification:** Verify at least one existing guidance artifact explicitly references `VISION.md` as the strategic alignment source.
3. **Change Integrity Verification:** Verify only intended docs are changed and wording distinguishes strategic vision from near-term planning.

**Key Test Cases:**
- `VISION.md` states a two-year target state for the development system.
- `VISION.md` includes explicit mission/outcomes, desired characteristics, and non-negotiable guardrails sections.
- `VISION.md` defines a concrete alignment-evaluation method for rules/learnings/spec updates.
- `VISION.md` explicitly states it is not a sprint plan, implementation checklist, or release roadmap.
- `RULES.md` contains explicit strategic alignment reference to `VISION.md`.

**Mocking Strategy:**
- No mocks; all checks are against real repository artifacts.

**Coverage Goals:**
- 100% acceptance criteria coverage through direct file inspection.
- Explicit verification of at least one cross-artifact reference path to `VISION.md`.

**Test Organization:**
- Use direct file inspection and search checks (`rg -n "VISION\\.md|VISION"`).
- Validate final deltas with `git diff -- VISION.md RULES.md .gromit/plans/constancy-of-purpose-vision.md`.

## Implementation Tasks

### Task 1: Align `VISION.md` with Required Strategic Contract Structure

**Files:**
- Modify: `VISION.md`

**What to Do:**
Refine `VISION.md` to ensure required strategic sections are explicit and easy to scan: mission/outcomes, desired two-year characteristics, non-negotiable guardrails, and alignment assessment method. Ensure language clearly distinguishes durable vision from near-term planning artifacts.

**Acceptance Criteria:**
- `VISION.md` explicitly defines a two-year target state and mission/outcomes.
- `VISION.md` explicitly includes desired system characteristics and non-negotiable guardrails.
- `VISION.md` explicitly defines how maintainers assess alignment of new rules/learnings/specs.
- `VISION.md` explicitly states strategic scope boundaries (not sprint plan/checklist/roadmap).

**Dependencies:**
- None.

**Notes:**
- Existing content already covers much of this; prefer refinement and structure tightening over full rewrite.

### Task 2: Add Strategic Alignment Reference in Existing Guidance Artifact

**Files:**
- Modify: `RULES.md`

**What to Do:**
Add concise, explicit guidance in `RULES.md` pointing maintainers to `VISION.md` as the strategic alignment source when introducing or updating rules/learnings/spec-related behavior.

**Acceptance Criteria:**
- `RULES.md` contains an explicit reference to `VISION.md` as the strategic alignment source.
- Reference language explains when the alignment check applies (guidance/spec/learnings updates).
- Reference does not duplicate tactical instructions better kept in operational artifacts.

**Dependencies:**
- Task 1 (references should match finalized `VISION.md` terminology and structure).

**Notes:**
- Keep wording short and directive to avoid bloating operational rules text.

### Task 3: Verify Acceptance Criteria and Finalize Documentation Delta

**Files:**
- Verify: `VISION.md`
- Verify: `RULES.md`

**What to Do:**
Run final document checks to confirm acceptance criteria coverage, verify the explicit cross-reference exists, and ensure the final change set is scoped to intended documentation updates only.

**Acceptance Criteria:**
- All spec acceptance criteria are directly traceable to final document text.
- `rg`/diff checks confirm explicit `VISION.md` reference and intended edits only.
- Plan remains decompose-ready with clear dependency order and concrete outcomes.

**Dependencies:**
- Task 2.

**Notes:**
- This is a validation/finalization step, not new content creation.

---

## Notes

- Repository state already includes a root `VISION.md`; this implementation is a conformance and integration update, not a net-new artifact creation from scratch.
- Prefer `RULES.md` as the first reference point because it governs live behavioral policy; optionally mirror a lightweight mention in `README.md` later if discoverability needs improvement.
