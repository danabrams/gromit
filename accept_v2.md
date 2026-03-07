# Acceptance Criterion Evaluation

You are evaluating a single acceptance criterion against an implementation diff.

## Task

Using the spec, criterion, and diff provided in the INSTANCE section above, determine whether the implementation satisfies the criterion.

## Evaluation Guidelines

- Focus ONLY on the specific criterion being evaluated
- The diff shows all code changes made to implement the spec
- Consider whether the criterion's requirements are fully met, not just partially
- Look for both positive evidence (criterion met) and negative evidence (criterion violated)
- If the diff does not address the criterion at all, it fails

## Output Format

Respond with ONLY a JSON object containing two fields:

```json
{"pass": true, "summary": "The implementation correctly handles X by doing Y, satisfying the criterion."}
```

- `pass`: true if the implementation satisfies the criterion, false otherwise
- `summary`: Brief explanation of your reasoning (1-2 sentences)

Output only the JSON object. No markdown, no explanation, no preamble.
