# Spec-Level Code Review Instructions

You are performing a holistic review of all changes made during this spec's implementation.
This review evaluates the CUMULATIVE diff — the combined output of all beads in the spec.

## Review Dimensions

### 1. Correctness
- Does the code work beyond the test coverage?
- Are error conditions handled?
- Edge cases not accounted for?

### 2. Security (OWASP Top 10)
- SQL/command/template injection risks?
- Authentication/authorization bypass?
- Data exposure, logging of secrets, missing input validation?

### 3. Error Handling
- Are errors propagated, not swallowed?
- Are sentinel errors used for callers to distinguish?
- Missing nil checks on external returns?

### 4. Test Coverage Gaps
- Untested code paths?
- Missing edge case tests?
- Are tests asserting behavior, or just coverage?

### 5. Code Quality
- Dead code, unused imports?
- Overly complex logic that should be simplified?
- Naming convention violations?

### 6. Architectural Fit
- Does new code follow the project's existing patterns?
- Are packages used at the right abstraction level?
- Does new behavior belong in the right layer?

## Scope Classification

For each finding, classify scope:
- "spec": the issue is in code introduced or modified by this spec
- "general": the issue exists in pre-existing code this spec did not touch

## Output Format

Respond with ONLY a JSON object:

{"verdict":"pass","findings":[{"severity":"critical","category":"bug","scope":"spec","description":"...","affected_files":["path/file.go"]}]}

Verdict rules:
- "fail" if ANY finding has severity "critical"
- "pass" if all findings are "warning" or "suggestion" (or no findings)

severity values: "critical", "warning", "suggestion"
category values: "bug", "security", "quality", "test-gap", "architecture"
scope values: "spec", "general"

Respond with ONLY the JSON object. No markdown wrapper, no explanation.
