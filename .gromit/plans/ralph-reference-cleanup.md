---
id: ralph-reference-cleanup
source_spec: ralph-reference-cleanup
created: 2026-02-06
decomposed: false
---

# Ralph Reference Cleanup Implementation Plan

**Goal:** Remove the last three stale "ralph" / "ralph-runner" references from demo documentation files.

**Architecture:** Direct string replacements in two documentation files — no code changes, no tests, no structural impact.

**Tech Stack:** N/A (documentation only)

**Spec:** `.gromit/specs/ralph-reference-cleanup.md`

---

## Architecture

Three mechanical text replacements across two files. No design decisions, no integration points, no tradeoffs.

**Files to Modify:**
- `demo/RECORDING.md` — Fix build command on line 12
- `demo_plan.md` — Fix two path references on lines 9 and 160

**Files to Create:** None

**Out of Scope:**
- `.beads/issues.jsonl` — bead IDs with `ralph-runner-*` prefix are historical bd data and must not be changed

## Test Strategy

- After edits, run case-insensitive grep for "ralph" across both files to confirm zero matches
- Visually confirm the build command in `RECORDING.md` produces a binary named `gromit`

## Implementation Tasks

### Task 1: Replace ralph references in demo docs

**Files:**
- Modify: `demo/RECORDING.md`
- Modify: `demo_plan.md`

**What to Do:**
- Line 12 of `demo/RECORDING.md`: change `cd /path/to/ralph-runner && go build -o ralph ./cmd/gromit` to `cd /path/to/gromit && go build -o gromit ./cmd/gromit`
- Line 9 of `demo_plan.md`: change `` Three files in `/home/danabrams/ralph-runner/demo/`: `` to `` Three files in the `demo/` directory: ``
- Line 160 of `demo_plan.md`: change `` Create `demo/` directory in the ralph-runner repo `` to `` Create `demo/` directory in the gromit repo ``

**Acceptance Criteria:**
- Case-insensitive grep for "ralph" in `demo/RECORDING.md` and `demo_plan.md` returns zero matches
- Build command in `RECORDING.md` references `gromit` binary, not `ralph`

**Dependencies:** None

---

## Notes

- This is the smallest possible plan — one task, one bead. No splitting needed.
- The `.beads/issues.jsonl` file contains `ralph-runner-*` bead ID prefixes that are intentionally preserved as historical bd data.
