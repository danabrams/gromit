# Thorough Code Review

You are performing a comprehensive review of multiple completed iterations to assess architectural quality, cross-cutting concerns, and accumulated technical debt.

{{if .Rules}}
## Project Rules

{{.Rules}}
{{end}}

## Review Scope

This review covers changes from {{len .CompletedBeads}} completed beads:

{{range .CompletedBeads}}
### {{.Title}} ({{.ID}})
{{if .Description}}
{{.Description}}
{{end}}

{{end}}

## All Changes

```diff
{{.Diff}}
```

## Your Job

Review the cumulative changes above across **9 dimensions**:

### 1. Intent & Spec Drift
- Do changes fulfill the original intents across all beads?
- Is there scope creep or unnecessary additions?
- Do the beads work coherently together?

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

### 7. Architectural Assessment
- Do the changes maintain good separation of concerns?
- Are there new coupling issues introduced?
- Are abstractions appropriate or over/under-engineered?
- Does the architecture still make sense with these additions?

### 8. Cross-Cutting Concerns
- Are there patterns that should be extracted into shared utilities?
- Are there inconsistent approaches to similar problems?
- Are configuration, logging, and error handling uniform?

### 9. Pattern Detection
- Are there repeated code patterns that suggest missing abstractions?
- Are there anti-patterns emerging across changes?
- Are there opportunities for refactoring that span multiple beads?

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
      "title": "Refactor auth middleware for consistency",
      "description": "Auth checks are implemented differently in handlers A, B, and C. Extract common pattern.",
      "priority": 1,
      "labels": ["refactor", "from-review"]
    }
  ],
  "backlog_items": [
    {
      "title": "Consider event sourcing for audit trail",
      "description": "Current audit approach is fragmented across beads. Event sourcing could unify it.",
      "reason": "Needs architectural discussion and product owner buy-in"
    }
  ],
  "summary": "Architecture is sound. Found opportunities for extracting shared patterns and identified minor security gap."
}
```

**Notes:**
- `passed`: true if no blocking issues found, false if major problems exist
- `fixes_applied`: List of fixes you made directly (empty array if none)
- `beads_to_create`: Issues that need dedicated work (empty array if none)
- `backlog_items`: Issues needing discussion/decision (empty array if none)
- `summary`: 1-2 sentence summary of your review

**Important:**
- Fix trivial issues directly. If you fix something, re-validation will run automatically.
- Only create beads for issues requiring significant work.
- Use backlog for issues blocked on decisions or unclear requirements.
- All review-created beads automatically get a `from-review` label added.
- Focus on cross-cutting patterns and architectural concerns — this is your chance to see the bigger picture.
- Be concise but specific. Each finding should be actionable.

Respond with ONLY the JSON object. No markdown, no explanation, just the JSON.
