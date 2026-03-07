# Plan Generation Instructions

You are generating an implementation plan for a specification. You are running in non-interactive mode — produce the plan directly without asking questions or requesting confirmation.

## Output Format

Output ONLY the plan content in this structure:

```markdown
---
id: <spec-name>
source_spec: <spec-name>
created: <YYYY-MM-DD>
decomposed: false
---

# <Title> Implementation Plan

**Goal:** [1-sentence summary]
**Architecture:** [1-2 sentence approach summary]
**Spec:** `.gromit/specs/<spec-name>.md`

---

## Architecture

[High-level architecture: components, integration points, data flow, files to modify/create, tradeoffs]

## Test Strategy

[Test levels, key test cases, mocking strategy, coverage goals]

## Implementation Tasks

### Task 1: [Title]
**Files:** [files to modify/create/test]
**What to Do:** [clear description]
**Acceptance Criteria:** [1-3 concrete, testable criteria]
**Dependencies:** [other tasks this depends on, if any]

### Task 2: [Title]
...
```

## Task Sizing Rules

- One concern per task — a single file or two tightly coupled files
- 1-3 acceptance criteria per task — split if more than 3
- Max 2-3 files touched per task
- Start with foundational tasks (types, interfaces, core logic)
- Never split natural units: interface + implementation + mock, implementation + tests
- Make dependencies explicit

## Important

- Do NOT ask questions or request confirmation
- Do NOT suggest next steps or ask to execute
- Output ONLY the plan markdown content
- Explore the spec thoroughly and produce a complete, actionable plan
