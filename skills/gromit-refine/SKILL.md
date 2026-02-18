---
name: gromit-refine
description: Use when refining backlog items or rough ideas into structured specifications. Guides codebase-aware conversation to transform vague concepts into clear specs with acceptance criteria, decisions, and context.
version: 2.0.0
---

# Gromit Refine Skill

Guides conversational refinement of backlog items or ad-hoc ideas into structured specification files following Gromit principles.

## When to Use This Skill

Use this skill when:
- You have a backlog item that needs to become a spec
- You have a rough feature idea that needs exploration and clarification
- You want to refine vague concepts into concrete, implementable specifications
- You need to understand how a feature fits into the existing codebase

## Methodology

This skill follows a structured conversation flow for transforming ideas into specs:

### 1. Introduction and Context Gathering

When the skill starts:
- Acknowledge the idea or backlog item being refined
- Explain the process: "I'll help you refine this into a structured spec through conversation. We'll explore the codebase, clarify requirements, discuss approaches, and write a spec when we reach clarity."
- If refining a backlog item, confirm what you understand from the backlog text
- If refining an ad-hoc idea, ask for the initial description

### 2. Explore the Codebase

Before asking clarifying questions, read the codebase to understand existing patterns and architecture:
- Use Glob to find relevant files (components, packages, modules)
- Use Grep to search for related functionality
- Read key files to understand current implementation patterns
- Identify where the new feature would fit in the existing structure

Share what you learned with the user:
- "I've explored the codebase and found..."
- "It looks like similar functionality exists in..."
- "The current architecture follows this pattern..."

This codebase exploration informs better questions and helps suggest approaches that align with existing patterns.

### 3. Clarify Requirements Through Conversation

Ask clarifying questions **one at a time** to understand what the feature entails:

**Purpose and Scope:**
- What problem does this solve?
- Who will use this feature or benefit from this work?
- What are the main user workflows or use cases?
- Are there any constraints or limitations?

**Requirements:**
- What are the core requirements? (Must-haves)
- What are nice-to-haves? (Should-haves)
- What's explicitly out of scope?

**Integration:**
- How does this fit with existing features?
- Are there dependencies on other systems or components?
- What data or state needs to be managed?

**Success Criteria:**
- How will we know when this is done?
- What behaviors must be demonstrable?
- What edge cases need handling?

Listen carefully to responses and ask follow-up questions to clarify ambiguous areas. Don't move on until you have clarity.

### 4. Explore Approaches (When Needed)

If the feature is complex or has multiple valid implementation paths, propose 2-3 approaches with tradeoffs:

**Format:**
```
I see a few ways we could approach this:

**Approach A: [Name]**
[Description of the approach]
Pros: [list concrete benefits]
Cons: [list concrete drawbacks]
Fits existing patterns: [yes/no + details]

**Approach B: [Name]**
[Description of the approach]
Pros: [list concrete benefits]
Cons: [list concrete drawbacks]
Fits existing patterns: [yes/no + details]

I recommend Approach [X] because [specific reasons based on simplicity, maintainability, and alignment with existing codebase patterns].
```

Discuss the recommendation with the user. They may prefer a different approach or suggest a hybrid.

### 5. Collaboratively Choose Spec Name

Once the feature is clear, propose a spec name:
- Use lowercase with hyphens (e.g., `user-profiles`, `api-rate-limiting`)
- Keep it short but descriptive
- Reflect the core feature, not implementation details
- Ask: "I'll call this spec `[name]`. Does that work or would you prefer something else?"

### 6. Write the Spec

Create the spec file at `.gromit/specs/<name>.md` with the following structure:

```markdown
---
id: <spec-name>
source_ideas: [<backlog-id>]  # If from backlog, otherwise []
created: <YYYY-MM-DD>
---

# <Title>

## Specification

[Clear description of what the feature is and how it works. Focus on behavior, not implementation. Include user-facing workflows, system interactions, and key requirements. Use tables, lists, or subsections as needed for clarity.]

## Acceptance Criteria

- [Concrete, testable criterion 1]
- [Concrete, testable criterion 2]
- [Concrete, testable criterion 3]
...

Each criterion should have an obvious pass/fail test.

## Decisions

1. **[Decision title]** [Explanation of the decision and rationale]

2. **[Decision title]** [Explanation of the decision and rationale]

...

Document key architectural choices, approach selections, and tradeoffs made during refinement.

## Research & Context

### Current State

[What exists in the codebase today that's relevant to this feature. Reference specific files, packages, or patterns.]

### [Other relevant sections as needed]

[Any additional context, research findings, external documentation references, or background that will help during planning and implementation.]
```

**Key Guidelines:**
- **Specification section**: Describe what and how, not implementation details. Focus on observable behavior.
- **Acceptance Criteria**: Make them concrete and testable. Each should be verifiable through tests, manual verification, or observation.
- **Decisions section**: Capture the "why" behind choices made during refinement. This prevents relitigating decisions later.
- **Research & Context**: Provide background that will help during planning. Include relevant file paths, existing patterns, or external documentation.

### 7. Confirm and Finalize

After writing the spec:
- Use the Write tool to create the file at `.gromit/specs/<name>.md`
- Show the user the path and summarize what was created
- If this was a backlog item, note that it should be marked `status: refined` with the spec name (the CLI command will handle this)
- Ask if any adjustments are needed

### 8. External Research (Ad-Hoc)

If during conversation the user asks for external research (e.g., "What do other tools do?" or "Check the docs for..."):
- Use WebSearch or WebFetch as appropriate
- Summarize findings and integrate them into the conversation
- Don't do research proactively — wait for the user to request it

## Key Principles

1. **Codebase-aware** - Always explore existing code before asking questions
2. **One question at a time** - Don't overwhelm with multiple questions at once
3. **Collaborative naming** - Let the user approve the spec name
4. **Concrete criteria** - Acceptance criteria must be testable
5. **Document decisions** - Capture the "why" behind choices
6. **Conversational** - Listen actively, follow up on answers, adjust based on responses
7. **Specs only, never plans or code** - Refine produces specs (the *what*). Do NOT write implementation plans, task breakdowns, file lists, architecture proposals, or implementation code — planning is `gromit plan`'s job, implementation is `gromit run`'s job. If you catch yourself writing "Implementation Tasks", "Architecture" sections, or actual code, stop — you've left refine territory. Only write code or plans if the user explicitly asks you to

## Output Format Reference

The spec format is designed for downstream consumers:
- **Frontmatter**: Machine-parseable metadata (id, source, created date)
- **Specification**: Primary content — what the feature is and how it works
- **Acceptance Criteria**: Clear pass/fail tests for completion
- **Decisions**: Rationale for choices made during refinement
- **Research & Context**: Supporting information for planning and implementation

Consumers (like `gromit plan`) can stop reading early if they only need the Specification and Acceptance Criteria.

## Tips

- **Too many acceptance criteria?** Consider whether the feature should be split into multiple specs
- **Unclear criteria?** Rephrase as concrete, observable behavior
- **Vague requirements?** Ask more clarifying questions before writing the spec
- **Feature seems too small?** That's fine — small specs are easier to plan and implement
- **Lost in details?** Step back and focus on the core problem and user workflow

## Model and Complexity

This skill uses **opus** for best results on codebase exploration, creative brainstorming, and complex refinement decisions.

## Example Interaction Flow

1. **You**: "I'll help you refine this into a structured spec. Let me first explore the codebase to understand how this fits with existing patterns..."
2. **You**: [Read relevant files, search for related functionality]
3. **You**: "I found that authentication currently uses JWT tokens in `internal/auth/`. Let me understand your requirements better. What specific user information needs to be stored in the profile?"
4. **User**: "Name, email, avatar URL, and a bio section"
5. **You**: "Got it. And should profiles be editable by the user themselves, by admins, or both?"
6. **User**: "Users can edit their own profiles only"
7. **You**: [Continue clarifying questions]
8. **You**: "I see two approaches for this: [Approach A details vs Approach B details]. I recommend Approach A because it aligns with how settings are currently handled. What do you think?"
9. **User**: "Approach A sounds good"
10. **You**: "Great! I'll call this spec `user-profiles`. Does that work?"
11. **User**: "Yes"
12. **You**: [Write the spec file]
13. **You**: "I've created the spec at `.gromit/specs/user-profiles.md`. The spec captures the profile display and editing, and includes 4 acceptance criteria. Ready for any adjustments or should we move forward?"
14. **User**: "Looks good, thanks!"

## Integration with Gromit Pipeline

This skill is the **Refine** stage in Gromit's four-stage pipeline:
1. **Capture** (`gromit add`) - Raw ideas → backlog entries
2. **Refine** (`gromit refine`) - Backlog items or ad-hoc ideas → specs (THIS SKILL)
3. **Plan** (`gromit plan`) - Specs → implementation plans
4. **Decompose** (`gromit decompose`) - Plans → bd beads

The spec you produce becomes the input to `gromit plan`, which will break it into an implementation plan with tasks, dependencies, and architecture decisions.
