---
name: ralph-plan
description: Use when the user wants to plan a feature by breaking it into bite-sized beads. Helps decompose features into properly-sized Ralph tasks with acceptance criteria, proposing approaches and creating beads via bd.
version: 1.0.0
---

# Ralph Plan Skill

Guides conversational feature decomposition into properly-sized beads following Ralph Runner principles.

## When to Use This Skill

Use this skill when:
- The user describes a new feature or large task that needs decomposition
- They want to break down work into Ralph beads
- They ask "how should I structure this" or "help me plan this feature"
- They mention needing properly-sized tasks for the Ralph loop

## Methodology

This skill follows a structured conversation flow:

### 1. Understand the Feature (Ask Questions)
Start by understanding the feature deeply. Ask one question at a time:
- What problem does this feature solve?
- Who are the users/consumers?
- What are the key workflows or happy paths?
- Are there important edge cases or constraints?
- What are the success criteria?

Listen carefully to responses and ask follow-up questions to clarify ambiguous areas.

### 2. Propose Approaches
Once you understand the feature, propose 2-3 different implementation approaches with tradeoffs:
- **Approach A**: [Description] - Pros: [list], Cons: [list]
- **Approach B**: [Description] - Pros: [list], Cons: [list]
- **Approach C**: [Description] - Pros: [list], Cons: [list]

Recommend one approach based on complexity, maintainability, and alignment with Ralph principles.

### 3. Decompose into Beads
Break the feature into properly-sized beads following Ralph Wiggum loop principles:

**Bead Sizing Rules:**
- **One concern per bead** - A single file or two tightly coupled files
- **1-3 acceptance criteria** - Concrete, testable criteria only; split if more than 3
- **Self-contained** - Understandable without reading other beads
- **No ambiguity** - Implementation clear without design decisions
- **Max 2 files touched** - If more, reconsider the split
- **Clear definition of done** - Each criterion has an obvious pass/fail test

For each bead, provide:
1. **Title**: Clear, specific description of the single concern
2. **Acceptance Criteria**: 1-3 concrete, testable criteria
3. **Files Involved**: 1-2 files that will be touched
4. **Dependencies**: Any beads that must complete first

**Example Bead:**
```
Title: Add JWT token validation endpoint
Acceptance Criteria:
1. POST /api/validate-token endpoint accepts JWT token
2. Returns 200 with decoded claims if valid
3. Returns 401 with error message if invalid or expired
Files: internal/auth/validator.go, cmd/ralph/main.go
Dependencies: (none, or references to other bead IDs)
```

### 4. Present Bead List for Approval
Show the user the complete bead list in a clear format:
- List all beads in dependency order
- Show the title and acceptance criteria for each
- Include estimated priority/complexity
- Ask for feedback: "Does this decomposition look good? Any changes or additions?"

### 5. Create Beads
Once the user approves, create beads using `bd create`:

```bash
bd create "Add JWT token validation endpoint" \
  --priority 1 \
  --acceptance-criteria "POST /api/validate-token endpoint accepts JWT token" \
  --acceptance-criteria "Returns 200 with decoded claims if valid" \
  --acceptance-criteria "Returns 401 with error message if invalid or expired"
```

For subsequent beads with dependencies:
```bash
bd create "Add token refresh endpoint" \
  --priority 1 \
  --parent <parent-bead-id>
```

## Key Principles

1. **One concern per bead** - Don't mix unrelated changes
2. **No ambiguity** - Claude should implement without asking design questions
3. **Testable criteria** - Each criterion has a clear pass/fail
4. **Dependency order** - Present beads in creation order
5. **Right-sized chunks** - 1-2 hours of work per bead

## Tips

- **Too many criteria?** Split the bead - usually a sign it's too large
- **Crossing file boundaries?** Make sure there's a tight coupling reason
- **Unclear acceptance criteria?** Rephrase as a concrete behavior
- **Missing dependencies?** Ask the user about build/test requirements

## Example Interaction Flow

1. **User**: "I want to add user authentication"
2. **You**: "Great! Let me understand this better. What authentication mechanism were you thinking - JWT, sessions, OAuth, or are you open to suggestions?"
3. **User**: "JWT would be good"
4. **You**: "Understood. A few more questions: Where will tokens be stored on the client? And do you need refresh tokens or just access tokens?"
5. **User**: "Client-side storage, and we need refresh tokens for session persistence"
6. **You**: [Propose 2-3 approaches with tradeoffs, recommend one]
7. **User**: "Approach B looks good"
8. **You**: [Decompose into bite-sized beads]
9. **User**: "This looks great, let's create them"
10. **You**: [Run bd create commands for each bead]

## Integration with Ralph Runner

These beads integrate seamlessly with Ralph Runner's loop:
- Each bead gets a fresh Claude context (`ralph run`)
- Model selection by priority/complexity
- Bead closure with `bd close <id>` after successful implementation
- Validation runs separately for cost efficiency

Beads created follow the structure expected by `.ralph/templates/` for prompt injection during runs.
