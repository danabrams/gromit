# Retrospective Analysis

You are analyzing accumulated learnings from ralph-runner iterations to identify patterns, consolidate knowledge, and recommend updates to project rules.

## Current Rules

{{.Rules}}

## Current Learnings

{{.Learnings}}

## Task

Analyze the learnings above and provide:

1. **Consolidation Opportunities**: Identify duplicate or related learnings that should be merged
2. **Patterns Worth Promoting**: Suggest learnings that should become rules in RULES.md
3. **Stale Learnings**: Identify learnings that may no longer be relevant
4. **Rule Updates**: Propose specific changes to RULES.md

## Output Format

Use the following format:

### Consolidation

For each set of related learnings:
- **Learnings to merge**: [List dates/IDs]
- **Consolidated version**: [Single clear statement]
- **Rationale**: [Why these should be merged]

### Promote to Rules

For learnings that should become rules:
- **Learning**: [Date | ID | Content]
- **Proposed rule**: [How it should appear in RULES.md]
- **Section**: [Which section of RULES.md: Code Style, Architecture, Safety, or Process]
- **Rationale**: [Why this should be a rule]

### Archive

For stale or obsolete learnings:
- **Learning**: [Date | ID | Content]
- **Rationale**: [Why this is no longer relevant]

### Rule Changes

For direct updates to existing rules:
- **Current rule**: [Exact text from RULES.md]
- **Proposed change**: [New text]
- **Rationale**: [Why this change is needed]

## Guidelines

- Be conservative - only promote patterns seen multiple times
- Focus on actionable, specific rules
- Ensure proposed rules align with Go idioms and project goals
- Consider whether a learning is truly a "rule" (constraint) or just good advice
