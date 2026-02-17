# Success Learning Extraction

A task just succeeded. Extract any codebase patterns, conventions, or gotchas that would help future tasks.

## Task

**ID:** {{.BeadID}}
**Title:** {{.BeadTitle}}

{{.BeadDescription}}

## Summary of Work Done

{{.Summary}}

## Your Job

Extract ONE generalizable learning from this successful iteration:

1. **Choose the highest-leverage lens:**
   - architecture: boundaries, contracts, coupling, ownership, scaling, reuse
   - process: planning, validation flow, review workflow, handoff, release mechanics
   - technical: implementation details, language/library conventions, edge-case gotchas
   - Prefer architecture/process when the summary contains evidence for them. Use technical only when no higher-level learning is justified.

2. **Focus on codebase insights:**
   - Patterns: How things are structured or organized in this codebase
   - Conventions: Naming, formatting, or architectural choices
   - Gotchas: Surprising behavior, edge cases, or things to watch out for

3. **Make it actionable and durable:**
   - Should tell what to do or avoid
   - Should be useful for similar future tasks
   - Should be concise (1-2 sentences)
   - Include enough context to be reusable (specific component, workflow, or decision point)

4. **Avoid low-value output:**
   - Do not restate generic best practices ("write tests", "handle errors")
   - Do not describe only what changed; capture why the approach should repeat
   - If the candidate learning is too local to one diff, return null

5. **Skip if no learning:**
   - If the task was straightforward and revealed nothing new, return null
   - Don't force a learning from routine work

## Output Format

Respond with ONLY a JSON object (no markdown, no explanation):

{"learning": "The insight or null", "category": "conventions | gotchas | patterns"}

Examples:
- {"learning": "Config validation always happens in setDefaults() method, not in Load()", "category": "conventions"}
- {"learning": "Test files use table-driven tests with t.Run for each case", "category": "patterns"}
- {"learning": null, "category": "patterns"}
