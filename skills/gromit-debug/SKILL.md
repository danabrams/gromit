---
name: gromit-debug
description: Use when investigating bugs in the target codebase. Guides free-form investigation to identify root cause, triage severity, and produce appropriate outcomes (direct fix, plan, or backlog item).
version: 1.0.0
---

# Gromit Debug Skill

Guides free-form bug investigation in the target codebase. Claude investigates unexpected behavior, error messages, or failing tests, identifies root cause, and produces graduated outcomes based on triage.

## When to Use This Skill

Use this skill when:
- The user reports unexpected behavior or bugs
- Tests are failing and the cause is unknown
- Error messages appear without clear root cause
- The user asks to debug, investigate, or diagnose an issue
- The user invokes `gromit debug` from the command line

## Methodology

This skill follows a free-form investigation process with graduated outcomes based on triage. There is no rigid methodology — adapt your approach to the specific bug.

### 1. Understand the Symptom

When the skill starts:
- Acknowledge the bug report or symptom description
- If the user provided a description via the command line, confirm what you understand
- If no description was provided, ask: "What behavior are you observing? What did you expect to happen?"
- Clarify any ambiguous details about when/how the bug occurs

### 2. Investigate Freely

Use whatever approach makes sense for this specific bug:

**Common Investigation Techniques:**
- Read the relevant code files to understand current implementation
- Trace execution paths to identify where behavior deviates
- Search for related error messages or log statements
- Check test files to understand expected behavior
- Reproduce the issue if possible (run tests, execute commands)
- Look for recent changes (git log, git blame) that might have introduced the bug
- Check for edge cases or input validation issues
- Review error handling and failure paths
- Search for similar patterns elsewhere in the codebase

**Flexible Approach:**
- Start with the most obvious place to look based on the symptom
- Follow the trail of evidence wherever it leads
- Ask the user for clarification if you need more context
- Share your findings as you go ("I see X is happening in file.go:123")
- Don't feel constrained to follow a specific sequence

### 3. Identify Root Cause

Once you've investigated, determine the root cause:
- What is the actual bug in the code? (Missing validation, incorrect logic, wrong assumption, etc.)
- Where exactly is the problem? (Specific files, functions, lines)
- Why is it happening? (What condition triggers it?)
- What's the impact? (Does it break core functionality, edge case, or just cosmetic?)

If you can't identify root cause with the information available, that's okay — move to outcome #3 (see below).

### 4. Triage and Choose Outcome

Based on what you found, choose the appropriate outcome:

#### Outcome 1: Trivial Fix (Apply Directly)

**When to use:**
- Root cause is clear
- Fix is small (1-10 lines changed)
- No design decisions required
- No architectural implications
- High confidence the fix won't break anything

**Actions:**
1. Make the fix directly in the session using the Edit tool
2. Explain what you changed and why
3. Run validation commands from gromit.yaml to confirm tests pass:
   ```bash
   go test ./...
   go vet ./...
   go build ./cmd/gromit
   ```
4. If validation passes, summarize the fix:
   ```
   Fixed: [Brief description]
   Files changed: [file1.go, file2.go]
   Root cause: [What was wrong]
   Solution: [What you changed]
   Validation: All tests pass ✓
   ```
5. Suggest the user commit the fix

**Important:** Only use this outcome for genuinely trivial fixes. When in doubt, choose outcome 2 or 3.

#### Outcome 2: Clear Fix, Needs Planning (Create Investigation Report + Plan)

**When to use:**
- Root cause is clear
- Fix is well understood
- Implementation is straightforward but non-trivial (10+ lines, multiple functions, or cross-cutting)
- No major design decisions required
- Medium confidence level

**Actions:**
1. Write an investigation report to `.gromit/reports/debug-<timestamp>.md` (format below)
2. Create an implementation plan at `.gromit/plans/<descriptive-name>.md` following the standard plan format:
   - Include tasks breakdown
   - Note dependencies between tasks
   - Reference the investigation report in the Research & Context section
3. Ask the user: "I've created a plan for this fix at `.gromit/plans/<name>.md`. Would you like me to decompose it into beads now so you can run it?"
4. If the user says yes, invoke the decompose stage (use the same pattern as the orchestrator skill)
5. If the user says no, summarize what was created and exit

**Investigation Report Format (for outcomes 2 and 3):**
```markdown
# Bug Investigation: <brief title>

## Symptom
[What was observed/reported by the user. Include error messages, unexpected behavior, or test failures.]

## Root Cause
[Detailed explanation of what was found. If not yet determined, write "Under investigation" and document what's known so far.]

## Affected Code
[List the specific files, functions, and line ranges involved. Use file:line format for easy navigation.]

## Suggested Fix
[Describe the approach for fixing. If root cause is unclear, describe what investigation is still needed.]

## Evidence
[Key observations that support the analysis:
- Test results (pass/fail, specific assertions)
- Log excerpts
- Code excerpts showing the problem
- git blame context (when the code was introduced)
- Related issues or patterns elsewhere in the codebase]
```

**Plan Format:**
Follow the same plan format used by the `gromit plan` stage (see gromit-plan skill for reference). Include:
- Frontmatter with `id`, `spec`, `created`, `decomposed: false`
- Architecture section if relevant
- Tasks section with concrete, sized tasks
- Dependencies section if tasks have ordering constraints
- Testing Strategy section describing how to verify the fix

#### Outcome 3: Needs More Investigation or Design Decisions (Create Investigation Report + Backlog Item)

**When to use:**
- Root cause is unclear or only partially understood
- Fix requires design decisions or user input
- Multiple valid approaches exist
- Architectural implications
- Low confidence in solution

**Actions:**
1. Write an investigation report to `.gromit/reports/debug-<timestamp>.md` (format above)
2. Add the item to the backlog via Bash:
   ```bash
   gromit add "Fix: [Brief description]. See investigation report at .gromit/reports/debug-<timestamp>.md"
   ```
3. Summarize what was created:
   ```
   Investigation complete. Root cause is [partially determined / unclear].

   Report: .gromit/reports/debug-<timestamp>.md
   Backlog item: [ID from gromit add output]

   Next steps: [What investigation or decisions are still needed]
   ```

### 5. File Naming Conventions

**Investigation Reports:**
- Location: `.gromit/reports/`
- Naming: `debug-<timestamp>.md` where timestamp is `YYYYMMDD-HHMMSS` (e.g., `debug-20260208-143022.md`)
- Use ISO 8601 format for timestamp to ensure chronological sorting

**Plans Created from Debug:**
- Location: `.gromit/plans/`
- Naming: Use descriptive names that reflect the fix, not generic debug names
  - Good: `fix-login-plus-sign.md`, `repair-validation-edge-case.md`
  - Bad: `debug-fix.md`, `bug-repair.md`
- Follow existing plan naming conventions (lowercase with hyphens)

## Key Principles

1. **Free-form investigation** — No rigid methodology; adapt to the specific bug
2. **Evidence-based** — Support conclusions with concrete observations
3. **Graduated outcomes** — Match the response to the situation (trivial fix vs plan vs backlog)
4. **Validation after fixes** — Always run tests when applying direct fixes
5. **Clear documentation** — Investigation reports are reference material for future work
6. **User collaboration** — Ask clarifying questions when needed

## Validation Commands

When applying a direct fix (outcome 1), run the validation commands defined in `gromit.yaml`. For this project, use:

```bash
go test ./...    # Run all tests
go vet ./...     # Run static analysis
go build ./cmd/gromit  # Ensure the binary builds
```

If any validation command fails:
- Show the failure output to the user
- Ask if they want you to investigate the failure or revert the change
- DO NOT mark the fix as complete if validation fails

## Context Provided

This skill receives full project context at launch (same as the build phase):
- CLAUDE.md (project conventions)
- RULES.md (constraints and non-negotiables)
- LEARNINGS.md (confirmed and recent learnings)
- Working directory path

This context is already loaded when the skill starts — you don't need to read these files manually.

## Model and Complexity

This skill uses **opus** by default for best results on complex investigation and root cause analysis. The user can override with `--model sonnet` or `--model haiku` via the command line.

## Example Interaction Flows

### Example 1: Trivial Fix

1. **User**: `gromit debug "login fails when email contains + sign"`
2. **You**: "I'll investigate the login failure with + signs in email addresses. Let me start by looking at the authentication code..."
3. **You**: [Read auth code, trace email handling]
4. **You**: "Found it! The email validation regex in `internal/auth/validator.go:45` doesn't allow + signs. This is a one-line fix. I'll apply it now."
5. **You**: [Edit the regex pattern]
6. **You**: "Applied fix. Running validation commands..."
7. **You**: [Run go test, go vet, go build]
8. **You**: "Fixed: Email validation now allows + signs in email addresses. Files changed: internal/auth/validator.go. Root cause: Regex pattern was too restrictive. Solution: Updated regex to include + in allowed characters. Validation: All tests pass ✓"

### Example 2: Clear Fix, Needs Planning

1. **User**: `gromit debug "API returns 500 when rate limit exceeded"`
2. **You**: "I'll investigate the 500 error when hitting rate limits..."
3. **You**: [Read API middleware, trace rate limiter]
4. **You**: "I've identified the root cause. The rate limiter middleware panics instead of returning a proper HTTP error when limits are exceeded. The fix is straightforward but involves updating error handling across multiple middleware functions. Let me create an investigation report and plan."
5. **You**: [Write `.gromit/reports/debug-20260208-143022.md`]
6. **You**: [Write `.gromit/plans/fix-rate-limit-panic.md`]
7. **You**: "I've created a plan for this fix at `.gromit/plans/fix-rate-limit-panic.md`. Would you like me to decompose it into beads now so you can run it?"

### Example 3: Needs More Investigation

1. **User**: `gromit debug "intermittent test failures in CI"`
2. **You**: "I'll investigate the intermittent test failures..."
3. **You**: [Read test code, check for timing issues, look at CI logs]
4. **You**: "I've found evidence of a race condition in the state manager tests, but I need to understand the intended concurrency model before proposing a fix. The tests use goroutines without synchronization in several places. Let me document what I've found."
5. **You**: [Write `.gromit/reports/debug-20260208-150033.md` with partial findings]
6. **You**: [Run `gromit add "Fix: Race condition in state manager tests. See investigation report at .gromit/reports/debug-20260208-150033.md"`]
7. **You**: "Investigation complete. Root cause is partially determined (race condition in state manager), but needs design decisions about the concurrency model. Report: .gromit/reports/debug-20260208-150033.md. Backlog item: [ID]. Next steps: Clarify intended concurrency model for state manager."

## Tips

- **Start with the symptom location** — Error messages often point directly to the problem
- **Use git blame** — Recent changes near the bug are often the cause
- **Check test files** — Tests document expected behavior
- **Look for similar patterns** — If X is broken, check if Y has the same issue
- **Don't overthink trivial fixes** — If it's a typo or obvious logic error, just fix it
- **Use investigation reports liberally** — Better to document findings than lose them
- **Validate before declaring victory** — Tests must pass for direct fixes

## Integration with Gromit Pipeline

This skill operates outside the main pipeline but can feed into it:
- Outcome 1 (trivial fix) → user commits directly
- Outcome 2 (clear fix) → creates plan → decompose → run (same as refine → plan → decompose → run)
- Outcome 3 (needs investigation) → adds to backlog → refine → plan → decompose → run

The debug skill is a lateral entry point that can inject work into any stage of the pipeline depending on triage outcome.
