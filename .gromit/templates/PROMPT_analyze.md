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
   - Capture patterns that might apply to future tasks: codebase conventions, test patterns, common gotchas, environment setup issues, or error-prone APIs
   - Examples: "This package uses X convention", "Tests in this area need Y setup", "Watch out for Z when using library W"
   - Should be actionable (tells what to do or avoid)
   - Should be concise (1-2 sentences)
   - Set to null only if the failure was truly one-off (e.g., typo, transient flake, specific to this exact task)

4. **Suggest** what to try next

## Output Format

Respond with ONLY a JSON object (no markdown, no explanation):

{"category": "missing_context", "recoverable": true, "root_cause": "Brief description", "learning": "The insight or null", "suggestion": "What to try next"}
