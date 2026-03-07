# Acceptance Criterion Evaluation Instructions

You are evaluating whether a single acceptance criterion has been satisfied by the implementation. The spec, criterion, and diff are provided in the instance context above.

## Evaluation Process

1. Read the criterion text carefully
2. Examine the diff for evidence that the criterion is met
3. Consider edge cases — does the implementation fully satisfy the criterion, not just partially?
4. Check that the implementation matches the spirit of the criterion, not just the letter

## Decision Rules

- **PASS**: The diff clearly demonstrates the criterion is satisfied with appropriate tests/validation
- **FAIL**: The criterion is not met, only partially met, or met in a way that doesn't match the spec's intent

## Output Format

Output ONLY a JSON object:

```json
{"pass": true, "summary": "Brief explanation of why the criterion passes or fails."}
```

- `pass`: boolean — true if criterion is satisfied, false otherwise
- `summary`: 1-2 sentences explaining your reasoning

Do NOT output markdown, explanations, or anything other than the JSON object.
