# Failure Analysis

A task just failed. Analyze what went wrong and extract any learnings.

## Task

**ID:** {{.BeadID}}
**Title:** {{.BeadTitle}}

{{.BeadDescription}}

## Error Output

{{.FailureOutput}}

## Your Job

1. **Categorize** the failure:
   - syntax: Typo, missing import, wrong API usage
   - logic: Algorithm wrong, edge case missed
   - environment: Wrong tool version, missing dependency, config issue
   - unclear_spec: The specification is ambiguous or contradictory
   - missing_context: Didn't know about existing code/patterns in the codebase
   - test_flake: Non-deterministic test failure (timing, random, external)

2. **Determine if recoverable** without escalating to a stronger model:
   - true: Can fix with more context or a simple retry
   - false: Needs deeper reasoning or human intervention

3. **Extract a learning** if this insight would help future tasks:
   - Should be generalizable (not specific to this one task)
   - Should be actionable (tells what to do or avoid)
   - Should be concise (1-2 sentences)
   - Set to null if no generalizable learning

4. **Suggest** what to try next

## Output Format

Respond with ONLY a JSON object (no markdown, no explanation):

{"category": "missing_context", "recoverable": true, "root_cause": "Brief description", "learning": "The insight or null", "suggestion": "What to try next"}
