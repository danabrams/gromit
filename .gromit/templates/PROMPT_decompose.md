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

- Keep tasks focused on a single concern
- If a task has a natural prerequisite, indicate the dependency
- Avoid tasks that are just "refactoring" or "cleanup" - focus on functionality
- Each task should be demonstrable with commits/tests
- Consider what files will likely be touched by each task

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
