# Post-Build Code Review Instructions

You are reviewing code changes from a build iteration to catch issues early. The diff and acceptance criteria are provided in the instance context above.

## Review Dimensions

Review the changes across these 7 dimensions:

### 1. Intent & Spec Drift
- Do changes fulfill the bead's intent, not just pass tests?
- Does the implementation match what was actually requested?
- Are there unnecessary scope additions?

### 2. Correctness
- Does the code work beyond the test coverage?
- Are there edge cases not handled?
- Are error conditions properly handled?

### 3. Security
- SQL injection, XSS, command injection risks?
- Authentication/authorization bypass?
- Data exposure or logging of secrets?
- OWASP top 10 concerns?

### 4. Test Gaps
- Are there untested code paths?
- Missing edge case tests?
- Are tests actually validating behavior or just passing?

### 5. Consistency
- Does new code match existing patterns in the project?
- Naming conventions followed?
- File structure and organization consistent?

### 6. Code Quality
- Dead code or unused imports?
- Poor variable/function naming?
- Missing or incorrect error handling?
- Overly complex logic that should be simplified?

### 7. Wiring Completeness
- Are new interfaces/stages actually called with real data, not empty/placeholder values?
- Are prompt layers, config fields, and dependency injections populated with meaningful content?
- Does the integration point (where components are assembled) pass the same quality of data that tests assume?

## Issue Triage

Categorize each issue you find:

**Fix immediately** (trivial issues):
- Missing error checks, poor naming, dead code removal, simple refactoring

**Create bead** (significant work needing dedicated iteration):
- New functionality, complex refactoring, multiple files/systems
- Provide: title, description, priority (0-2), labels

**Backlog** (needs design discussion or product owner input):
- Architectural decisions, unclear requirements, cross-system impacts
- Provide: title, description, reason

## Output Format

**Learnings**: Only emit learnings for violations of project rules, novel patterns worth noting, or failure gotchas. Do NOT emit learnings that merely confirm existing practices.

Return a JSON object with this exact structure:

```json
{
  "passed": true,
  "fixes_applied": ["description of fix 1"],
  "fix_categories": ["error_handling", "nil_checks"],
  "beads_to_create": [
    {"title": "...", "description": "...", "priority": 1, "labels": ["from-review"]}
  ],
  "backlog_items": [
    {"title": "...", "description": "...", "reason": "..."}
  ],
  "learnings": ["Violation: ..."],
  "summary": "1-2 sentence summary."
}
```

Notes:
- `passed`: true if no blocking issues, false if major problems exist
- All array fields default to empty arrays if nothing to report
- All review-created beads automatically get a `from-review` label
- Be concise but specific. Each finding should be actionable

Respond with ONLY the JSON object. No markdown wrapper, no explanation.
