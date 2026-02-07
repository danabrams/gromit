# Post-Iteration Review

You are reviewing code changes from a single iteration to catch issues early.

{{if .Rules}}
## Project Rules

{{.Rules}}
{{end}}

## Task Context

**Bead:** {{.Bead.Title}}
{{if .Bead.Description}}
**Description:** {{.Bead.Description}}
{{end}}

{{if .Spec}}
## Specification

{{.Spec}}
{{end}}

## Changes This Iteration

```diff
{{.Diff}}
```

{{if .ValidationCommands}}
## Validation Already Run

These commands passed:
{{range .ValidationCommands}}
- `{{.}}`
{{end}}
{{end}}

## Your Job

Review the changes above across **6 dimensions**:

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

## Issue Triage

Categorize each issue you find:

**Fix immediately** (trivial issues you can fix right now):
- Missing error checks
- Poor naming
- Dead code removal
- Simple refactoring

**Create bead** (significant work needing dedicated iteration):
- New functionality to add
- Complex refactoring
- Multiple files or systems involved
- Provide: title, description, priority (0-2), labels

**Backlog** (needs design discussion or product owner input):
- Architectural decisions
- Unclear requirements
- Cross-system impacts
- Provide: title, description, reason (why it's blocked)

## Output Format

Return a JSON object with this exact structure:

```json
{
  "passed": true,
  "fixes_applied": [
    "Added nil check in handler.go line 45",
    "Removed unused import from service.go"
  ],
  "beads_to_create": [
    {
      "title": "Add input validation for email field",
      "description": "Email field accepts invalid formats. Need regex validation and error messages.",
      "priority": 1,
      "labels": ["validation", "from-review"]
    }
  ],
  "backlog_items": [
    {
      "title": "Consider rate limiting for auth endpoints",
      "description": "No rate limiting on login/signup. Vulnerable to brute force.",
      "reason": "Needs infra decision on rate limit backend (Redis vs in-memory)"
    }
  ],
  "learnings": [
    "Error handling pattern in service.go is cleaner than older code in handler.go",
    "Test naming convention followed consistently across new test cases"
  ],
  "summary": "Implementation matches spec. Fixed 2 minor issues. Created 1 bead for validation gap."
}
```

**Notes:**
- `passed`: true if no blocking issues found, false if major problems exist
- `fixes_applied`: List of fixes you made directly (empty array if none)
- `beads_to_create`: Issues that need dedicated work (empty array if none)
- `backlog_items`: Issues needing discussion/decision (empty array if none)
- `learnings`: Codebase patterns, conventions, or gotchas observed during review (empty array if none)
- `summary`: 1-2 sentence summary of your review

**Important:**
- Fix trivial issues directly. If you fix something, re-validation will run automatically.
- Only create beads for issues requiring significant work.
- Use backlog for issues blocked on decisions or unclear requirements.
- All review-created beads automatically get a `from-review` label added.
- Be concise but specific. Each finding should be actionable.

Respond with ONLY the JSON object. No markdown, no explanation, just the JSON.
