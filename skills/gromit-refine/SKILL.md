---
name: ralph-refine
description: Use when refining backlog items into properly-sized beads. Guides brainstorming and decomposition of feature ideas from a backlog into bite-sized Ralph tasks with clear acceptance criteria.
version: 1.0.0
---

# Ralph Refine Skill

Guides conversational refinement of backlog items into properly-sized beads following Ralph Runner principles.

## When to Use This Skill

Use this skill when:
- You have a backlog of feature ideas that need decomposition
- You want to walk through backlog items one by one and brainstorm them into beads
- You need to refine vague feature concepts into concrete, implementable tasks
- You want to ensure backlog items follow Ralph bead sizing rules

## Methodology

This skill follows a structured conversation flow for processing backlog items:

### 1. Introduction and Setup
When the skill starts, acknowledge the backlog and explain the process:
- "I'll help you refine your backlog items into properly-sized beads"
- "We'll go through each item, brainstorm it together, and create beads once we're happy with the decomposition"
- "Let's start with the first item..."

### 2. Process Each Backlog Item Iteratively
For each backlog item, follow this flow:

#### A. Understand the Item
Ask clarifying questions one at a time to understand what the item entails:
- What problem does this item solve?
- What are the main requirements or workflows?
- Are there any constraints or dependencies?
- What's the success criteria for this item?
- Who will use this feature or benefit from this work?

Listen carefully and ask follow-up questions to clarify ambiguous areas.

#### B. Brainstorm Approaches (if needed)
If the item is complex or has multiple ways to solve it, propose 2-3 approaches:
- **Approach A**: [Description] - Pros: [list], Cons: [list]
- **Approach B**: [Description] - Pros: [list], Cons: [list]

Recommend one approach based on simplicity, maintainability, and alignment with Ralph principles.

#### C. Decompose into Beads
Break the item into properly-sized beads following Ralph Wiggum loop principles:

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
4. **Complexity**: Low/Medium/High (informs priority/model selection)
5. **Dependencies**: Any beads that must complete first

**Example Bead:**
```
Title: Create user profile page component
Acceptance Criteria:
1. React component displays user name, email, and avatar
2. Component accepts userId prop and fetches data from /api/users/{id}
3. Shows loading spinner while data is being fetched
Files: src/components/UserProfile.tsx, src/api/users.ts
Complexity: Low
Dependencies: (none, or references to other bead IDs)
```

#### D. Present Beads for Approval
Show the decomposition clearly:
- List all beads in dependency order
- Show title and acceptance criteria for each
- Include complexity level
- Ask: "Does this breakdown look good? Any changes needed?"

#### E. Create the Beads
Once approved, create beads using `bd create`:

```bash
bd create "Create user profile page component" \
  --priority 1 \
  --acceptance-criteria "React component displays user name, email, and avatar" \
  --acceptance-criteria "Component accepts userId prop and fetches data from /api/users/{id}" \
  --acceptance-criteria "Shows loading spinner while data is being fetched"
```

For beads with dependencies:
```bash
bd create "Add user profile edit form" \
  --priority 1 \
  --parent <parent-bead-id>
```

### 3. Track Progress
As you work through items:
- Keep a mental note of which backlog items have been processed
- Offer to continue with the next item after finishing one
- Summarize what's been created so far if the user asks

### 4. Wrap Up
When done with all backlog items:
- Summarize the total number of beads created
- Confirm all items have been processed
- Offer to adjust any beads if needed

## Key Principles

1. **One concern per bead** - Don't mix unrelated changes
2. **No ambiguity** - Claude should implement without asking design questions
3. **Testable criteria** - Each criterion has a clear pass/fail
4. **Dependency order** - Present beads in dependency order
5. **Right-sized chunks** - 1-2 hours of work per bead
6. **Conversational** - Ask one question at a time, listen actively

## Tips

- **Too many criteria?** Split the bead - usually a sign it's too large
- **Crossing file boundaries?** Make sure there's a tight coupling reason
- **Unclear acceptance criteria?** Rephrase as a concrete behavior
- **Vague item?** Ask more clarifying questions before decomposing
- **Item seems too small?** Consider combining with related items

## Model and Complexity

This skill uses **opus** for best results on creative brainstorming and complex decomposition decisions.

Bead complexity levels map to Ralph models:
- **Low** → P2 (haiku)
- **Medium** → P1 (sonnet)
- **High** → P0 (opus)

When creating beads, recommend appropriate priorities based on complexity.

## Example Interaction Flow

1. **User**: "I have a backlog item about adding user profiles"
2. **You**: "Great! Let me understand this better. What should the user profile page display? Just basic info or more?"
3. **User**: "Name, email, avatar, and a bio section. Plus an edit button if it's the logged-in user"
4. **You**: "Understood. A few more questions: Should the profile be editable inline or through a separate form? And where is the user data stored?"
5. **User**: "Separate edit modal form, data is in our user service"
6. **You**: [Decompose into beads - one for display, one for edit form, one for data integration]
7. **User**: "This looks good, create them"
8. **You**: [Run bd create commands for each bead]
9. **You**: "Done! I've created 3 beads. Ready for the next backlog item?"
10. **User**: "Yes, next one is..."

## Integration with Ralph Runner

These beads integrate seamlessly with Ralph Runner's loop:
- Each bead gets a fresh Claude context (`ralph run`)
- Model selection by priority/complexity
- Bead closure with `bd close <id>` after successful implementation
- Validation runs separately for cost efficiency

Beads created follow the structure expected by `.ralph/templates/` for prompt injection during runs.
