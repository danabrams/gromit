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

1. **Focus on codebase insights:**
   - Patterns: How things are structured or organized in this codebase
   - Conventions: Naming, formatting, or architectural choices
   - Gotchas: Surprising behavior, edge cases, or things to watch out for

2. **Make it actionable:**
   - Should tell what to do or avoid
   - Should be useful for similar future tasks
   - Should be concise (1-2 sentences)

3. **Skip if no learning:**
   - If the task was straightforward and revealed nothing new, return null
   - Don't force a learning from routine work

## Output Format

Respond with ONLY a JSON object (no markdown, no explanation):

{"learning": "The insight or null", "category": "conventions | gotchas | patterns"}

Examples:
- {"learning": "Config validation always happens in setDefaults() method, not in Load()", "category": "conventions"}
- {"learning": "Test files use table-driven tests with t.Run for each case", "category": "patterns"}
- {"learning": null, "category": "patterns"}
