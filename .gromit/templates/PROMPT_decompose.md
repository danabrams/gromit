# Task Decomposition

A task has been identified as too large or complex to complete in a single iteration. Your job is to break it down into 2-4 smaller, more manageable sub-tasks.

## Original Task

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}
**Priority:** P{{.Bead.Priority}}

### Description

{{.Bead.Description}}

{{if .ParentBead}}
### Parent Context

This is part of: **{{.ParentBead.Title}}**
{{end}}

## Your Job

Break this task into 2-4 smaller sub-tasks that:
- Each can be completed independently (or with minimal ordering constraints)
- Are smaller in scope but maintain the original goal
- Can be executed sequentially through the Gromit loop
- Each have clear acceptance criteria

## Output Format

Respond with a JSON array containing your proposed sub-tasks. Each sub-task should have:
- `title`: Brief title (max 60 chars)
- `description`: What needs to be done
- `depends_on`: Index of previous task if dependent (null if independent)
- `acceptance_criteria`: 2-3 bullet points

Example format:
```json
[
  {
    "title": "Set up database migrations",
    "description": "Create the initial migration files and schema...",
    "depends_on": null,
    "acceptance_criteria": ["Migration files created", "Schema matches spec"]
  },
  {
    "title": "Implement user model",
    "description": "Add User model with validation...",
    "depends_on": 0,
    "acceptance_criteria": ["Model created", "Tests pass", "Validation works"]
  }
]
```

## Guidelines

- Keep tasks focused on **one deliverable behavior** — a single observable change that a caller or user could verify. Not "one file" or "one concern," but one unit of working functionality
- Soft file limit of 4-5 — if touching 6+ files across unrelated packages, consider splitting. But touching interface.go, impl.go, mock_test.go, and impl_test.go for one method addition is fine — that's one change, not four
- **Never split these natural units:**
  - **Interface + implementation + mock updates** — In Go, changing an interface requires updating all implementations and mocks to compile. This is one change, not three beads
  - **Implementation + its tests** — Claude writes tests alongside implementation. Under ATDD, they're explicitly the same workflow. Never create a separate "write tests for X" bead
  - **Companion methods in same package** — Methods that follow the same pattern in the same file are one bead. If you'd copy-paste-modify to create the second, they belong together
  - **Command flags + wiring that makes them work** — A CLI flag that does nothing isn't a deliverable. The flag, its plumbing, and its effect are one bead
  - **Template + registration** — Adding a template file and registering it in the renderer are one action, not two
- If a task has a natural prerequisite, indicate the dependency
- Avoid tasks that are just "refactoring" or "cleanup" - focus on functionality
- Each task should be demonstrable with commits/tests

### Avoiding Sibling Overlap

Each sub-task's acceptance criteria must be **unique to that task** — criteria that would NOT be satisfied by completing any sibling task. Before finalizing your decomposition, perform this cross-check:

> For each sub-task, ask: "If I completed any other sub-task instead, would this task's acceptance criteria still fail?"

If the answer is "no" for any pair, the tasks overlap. Fix this by:
- Merging the overlapping tasks into one
- Rewriting acceptance criteria to be more specific to each task's unique contribution
- Ensuring each task adds distinct, observable behavior that no sibling provides
{{if .ATDDActive}}

### ATDD Active — No Test-Only Beads

ATDD methodology is active. Do NOT create sub-tasks whose sole purpose is writing tests (e.g., "Add unit tests for X", "Write tests for Y"). ATDD handles test writing automatically as Phase 1 of each bead — creating a separate test bead leads to a logical contradiction where acceptance tests always pass before implementation.
{{end}}

Respond with ONLY the JSON array (no markdown, no explanation).
