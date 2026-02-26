# Task Scope Estimation

You are reviewing a task to quickly estimate its complexity and whether it can be completed in a single iteration.

## Task Details

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}
**Priority:** P{{.Bead.Priority}}

{{if .Bead.Labels}}**Labels:** {{join .Bead.Labels ", "}}{{end}}

### Description

{{.Bead.Description}}

{{if .ParentBead}}## Parent Context

This task is part of: **{{.ParentBead.Title}}**{{if .ParentBead.Description}}

{{.ParentBead.Description}}{{end}}{{end}}

## Your Job

Estimate the scope of this task and determine if it can be completed in a single iteration. Consider:

1. **Codebase familiarity** - How much existing code needs to be understood?
2. **Number of files** - How many files will likely need changes?
3. **Complexity** - Are there intricate algorithms, architectural changes, or cross-system dependencies?
4. **Testing** - How thorough must testing be to validate completion?
5. **Unknowns** - Are there architectural decisions or dependencies that are unclear?

## Output Format

Respond with ONLY a JSON object (no markdown, no explanation):

```json
{
  "complexity": "low|medium|high",
  "estimated_iterations": 1,
  "rationale": "Brief explanation of scope assessment",
  "can_complete_in_single_iteration": true,
  "blockers": ["List of", "potential blockers if any"]
}
```

### Complexity Levels

- **low**: Straightforward changes to 1-2 files, minimal testing, clear requirements
- **medium**: Changes to 3-5 files, moderate testing, some architectural consideration
- **high**: Changes to 6+ files, extensive testing, complex architecture, unclear requirements, or cross-system dependencies

### Iteration Estimates

- 1-2 iterations: Task is achievable with current context
- 3+ iterations: Task likely too large and should be decomposed

Tasks that cannot be completed in a single iteration should return `can_complete_in_single_iteration: false` and recommend breakdown in blockers.
