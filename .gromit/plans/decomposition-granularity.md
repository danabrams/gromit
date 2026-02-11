---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T07:08:00-05:00"
id: decomposition-granularity
source_spec: decomposition-granularity
---

# Decomposition Granularity Implementation Plan

**Goal:** Replace file-count-based bead sizing rules with deliverable-behavior-based sizing across all four documents that govern decomposition.

**Architecture:** Documentation-only changes to four files — CLAUDE.md, RULES.md, SKILL.md, and PROMPT_decompose.md — replacing "max 2 files" philosophy with "one deliverable behavior, soft 4-5 file limit" and adding explicit never-split grouping rules.

**Tech Stack:** Markdown documentation, Go template syntax (PROMPT_decompose.md)

**Spec:** `.gromit/specs/decomposition-granularity.md`

---

## Architecture

Replace the bead sizing philosophy across all four documents from file-count-based ("one concern per bead, max 2 files") to behavior-based ("one deliverable behavior per bead, soft 4-5 file limit"). Add five explicit never-split grouping rules derived from log analysis of overlap patterns. Update splitting guidance and examples to match.

**Files to Modify:**
- `CLAUDE.md` — Replace Bead Sizing section (lines 63-70)
- `.gromit/RULES.md` — Update process rule (line 39) from "more than 2 files" to "6+ files across unrelated packages"
- `skills/gromit-decompose/SKILL.md` — Rewrite sizing rules (lines 34-50), splitting logic, and examples
- `.gromit/templates/PROMPT_decompose.md` — Update Guidelines section (lines 55-61) with new sizing philosophy and grouping rules

**Files to Create:** None

## Test Strategy

**Template Syntax Verification:** Run `go test ./...` and `go build ./cmd/gromit` to confirm PROMPT_decompose.md template still parses and renders correctly.

**Consistency Audit:** All four files use the same terminology — "deliverable behavior," "soft limit of 4-5 files," same five grouping rules.

**Preservation Check:** Anti-overlap cross-checking and ATDD test-only suppression in PROMPT_decompose.md (the `{{if .ATDDActive}}` block and "Avoiding Sibling Overlap" section) remain exactly as-is.

**Stale Reference Check:** Grep repo for any remaining "max 2 files" references after changes.

---

## Implementation Tasks

### Task 1: Update project-level docs (CLAUDE.md + RULES.md)

**Files:**
- Modify: `CLAUDE.md`
- Modify: `.gromit/RULES.md`

**What to Do:**

Replace the Bead Sizing section in CLAUDE.md (lines 63-70) with the new rules:

1. **One deliverable behavior per bead** — a single observable change that a caller or user could verify
2. **1-3 acceptance criteria** — concrete, testable criteria only; split if more than 3
3. **Soft file limit of 4-5** — if touching 6+ files across unrelated packages, consider splitting. Touching interface.go, impl.go, mock_test.go, and impl_test.go for one method addition is fine
4. **Self-contained** — understandable without reading other beads
5. **No ambiguity** — Claude implements without making design decisions
6. **Clear definition of done** — each criterion has an obvious pass/fail test

Add a Grouping Rules subsection listing the five never-split patterns:
- Interface + implementation + mock updates
- Implementation + its tests
- Companion methods in the same package
- Command flags + wiring
- Template + registration

In RULES.md line 39, change "Beads that touch more than 2 files should be split" to "Beads that touch 6+ files across unrelated packages should be split". Keep all surrounding guidance about cross-cutting refactors, scope review, and test infrastructure decomposition intact.

**Acceptance Criteria:**
- CLAUDE.md bead sizing section uses "deliverable behavior" language, lists soft 4-5 file limit, and includes grouping rules
- RULES.md process rule says "6+ files across unrelated packages" instead of "more than 2 files"

**Dependencies:** None

### Task 2: Update decompose docs (SKILL.md + PROMPT_decompose.md)

**Files:**
- Modify: `skills/gromit-decompose/SKILL.md`
- Modify: `.gromit/templates/PROMPT_decompose.md`

**What to Do:**

In SKILL.md, rewrite the Bead Sizing Rules section (lines 36-50):
- Replace "One concern per bead — A single file or two tightly coupled files" with "One deliverable behavior per bead"
- Replace "Max 2 files touched — If more, split" with "Soft file limit of 4-5 — if touching 6+ files across unrelated packages, consider splitting"
- Add a Grouping Rules subsection with the five never-split patterns

Rewrite the Splitting Logic (lines 44-48):
- Remove "If a task touches 3+ files → split by file or by logical grouping"
- Remove "If a task has both implementation and tests → consider separate beads"
- Replace with: split at 4+ acceptance criteria by distinct behaviors, split at 6+ files across unrelated packages by package boundary, split independently useful parts that don't need each other to compile, split at design decision boundaries

Rewrite Examples 1 and 3:
- Example 1: Show a task touching 4 files (interface + impl + tests + mock) as ONE bead, not split by file
- Example 3: Remove the "split impl from tests" example; replace with an example showing interface + impl + tests as one bead under the new grouping rules

Update the Key Principles section (line 213): change "Strict sizing — Enforce 1-2 files" to reflect soft 4-5 limit.

Update Common Mistakes section (lines 223-224): change "Too many files: If a bead touches 3+ files, it's too large" to reflect the new 6+ threshold.

In PROMPT_decompose.md, update the Guidelines section (lines 55-61):
- Replace "Keep tasks focused on a single concern" with guidance about one deliverable behavior per sub-task
- Add the grouping rules (never-split patterns) as a subsection
- Add the splitting rules (when to split)
- Preserve the existing "Avoiding Sibling Overlap" section (lines 63-72) and ATDD block (lines 73-78) exactly as-is

**Acceptance Criteria:**
- SKILL.md contains no "max 2 files" or "split by file" guidance, and examples show coarser beads
- PROMPT_decompose.md guidelines include new sizing philosophy and grouping rules while preserving anti-overlap and ATDD sections verbatim

**Dependencies:** None (can be done in parallel with Task 1, but consistency review afterward)

---

## Notes

- The plan skill's own bead sizing hints (in PROMPT_plan.md / the plan skill) also reference "max 2 files" — this is outside the spec's scope but worth noting for a follow-up update.
- After both tasks complete, run `go test ./...` and `go build ./cmd/gromit` to verify template syntax. Grep for "max 2 files" across the repo to catch any remaining stale references.
- The ATDD test-only suppression and anti-overlap cross-checking in PROMPT_decompose.md are complementary to this change and must not be modified.
