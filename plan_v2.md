# Plan Stage Instructions

You are producing an implementation plan from a specification. The spec (provided in the INSTANCE section above) already contains the architecture and test strategy. Your job is to break the work into logical implementation tasks and output a complete plan.

## Output Format

Output ONLY the plan markdown below. No conversation, no questions, no preamble.

```markdown
---
id: <spec-name from the spec frontmatter>
source_spec: <spec-name>
created: <YYYY-MM-DD>
decomposed: false
---

# <Title> Implementation Plan

**Goal:** [1-sentence summary of what we're building]

**Architecture:** [1-2 sentence summary from the spec's architecture section]

**Spec:** `.gromit/specs/<spec-name>.md`

---

## Implementation Tasks

### Task N: [Title]

**Files:**
- Modify: `path/to/file.go`
- Create: `path/to/newfile.go`
- Test: `path/to/file_test.go`

**What to Do:**
[Clear description of the work]

**Acceptance Criteria:**
- [Concrete, testable criterion 1]
- [Concrete, testable criterion 2]

**Dependencies:**
- Task N-1 (if applicable)

[Repeat for all tasks]

---

## Notes

[Any additional context, warnings, or reminders]
```

## Task Breakdown Guidelines

- Start with foundational tasks (types, interfaces, core logic)
- Group tightly coupled code together (interface + implementation + mock = one task)
- Keep test tasks paired with implementation tasks
- Make dependencies explicit
- 1-3 acceptance criteria per task (concrete and testable)
- Target 1-3 files per task
- If a task touches 4+ files across unrelated packages, split it
- Each task should map to 1-3 beads during decompose

## Natural Units (Never Split)

- Interface + implementation + mock updates
- Implementation + its tests
- Companion methods that share state
- Command flags + wiring
- Template + registration
