---
name: gromit-plan
description: Use when planning implementation for a specification. Reads specs, proposes architecture and test strategy with human review checkpoints, then writes a flexible implementation plan.
version: 2.0.0
---

# Gromit Plan Skill

Guides conversational planning from specifications to implementation plans. Proposes architecture and test strategies with human review checkpoints, then breaks work into logical tasks ready for decomposition into beads.

## When to Use This Skill

Use this skill when:
- You have a spec file that needs an implementation plan
- You need to design how a feature fits into existing code
- You want to plan architecture and testing before writing code
- You're ready to move from "what to build" (spec) to "how to build it" (plan)

## Methodology

This skill follows a structured conversation flow with mandatory human review checkpoints:

### 1. Read the Spec and Explore the Codebase

When the skill starts:
- Read the spec file provided as input (from `.gromit/specs/<name>.md`)
- Acknowledge what you're planning: "I'll help you plan the implementation of [spec name]. Let me explore the codebase to understand how this fits with existing patterns..."
- Use Glob to find relevant files (components, packages, modules)
- Use Grep to search for related functionality
- Read key files to understand current implementation patterns
- Identify where the new feature integrates with existing code

Share your findings:
- "I've explored the codebase and found..."
- "Similar functionality exists in..."
- "The current architecture uses..."

This exploration informs your architecture proposal.

### 2. Propose Architecture (CHECKPOINT)

Based on the spec and codebase exploration, propose the high-level architecture:

**Format:**
```
## Architecture Proposal

**Overview:**
[1-2 sentence summary of the approach]

**Key Components:**
1. **[Component/Package Name]**: [What it does and why]
2. **[Component/Package Name]**: [What it does and why]
...

**Integration Points:**
- [How this fits with existing code]
- [What existing components will be modified]
- [What new components will be created]

**Data Flow:**
[Describe how data moves through the system for key workflows]

**Files to Modify:**
- `path/to/file1.go` - [What changes]
- `path/to/file2.go` - [What changes]

**Files to Create:**
- `path/to/newfile.go` - [Purpose]

**Tradeoffs:**
- [Key decision 1]: Chose [X] over [Y] because [reason]
- [Key decision 2]: Chose [X] over [Y] because [reason]
```

**CHECKPOINT**: Present the architecture and ask: "Does this architecture approach look good? Any concerns or adjustments before I move on to the test strategy?"

Wait for user approval before proceeding. If the user wants changes, iterate on the architecture until approved.

### 3. Propose Test Strategy (CHECKPOINT)

Once architecture is approved, propose the testing approach:

**Format:**
```
## Test Strategy

**Test Levels:**
1. **Unit Tests**: [What units to test, key behaviors to cover]
2. **Integration Tests**: [What integrations to test, scenarios to cover]
3. **Manual Testing**: [What to verify manually, if applicable]

**Key Test Cases:**
- [Test case 1]: [What it verifies]
- [Test case 2]: [What it verifies]
...

**Mocking Strategy:**
- [What to mock and why]
- [What to test with real implementations and why]

**Coverage Goals:**
- [Critical paths that must be tested]
- [Edge cases to handle]

**Test Organization:**
- [Where test files will live]
- [Naming conventions to follow]
```

**CHECKPOINT**: Present the test strategy and ask: "Does this testing approach cover what we need? Any additional test cases or changes before I break this into tasks?"

Wait for user approval before proceeding. If the user wants changes, iterate on the test strategy until approved.

### 4. Break Work into Logical Tasks

Once both checkpoints pass, decompose the work into tasks. Each task should be:
- **Logically cohesive**: Related changes grouped together
- **Properly scoped**: Not too large (max 2-3 files) or too small
- **Well-specified**: Files affected, acceptance criteria, and dependencies clear
- **Ready for bead mapping**: An LLM should be able to turn this into 1-3 beads during decompose

**Task Format (flexible structure):**
```
### Task N: [Title]

**Files:**
- Modify: `path/to/file.go`
- Create: `path/to/newfile.go`
- Test: `path/to/file_test.go`

**What to Do:**
[Clear description of the work — what changes, what gets added, what behavior to implement]

**Acceptance Criteria:**
- [Concrete, testable criterion 1]
- [Concrete, testable criterion 2]
- [Concrete, testable criterion 3]

**Dependencies:**
- Task N-1 (must complete first)
- Task N-2 (provides needed types/functions)

**Notes:**
[Any implementation hints, tricky areas, or things to watch out for]
```

**Guidelines for Task Breakdown:**
- Start with foundational tasks (types, interfaces, core logic)
- Group tightly coupled code together
- Keep test tasks paired with implementation tasks when logical
- Make dependencies explicit — don't assume sequential execution
- Include 1-3 acceptance criteria per task (concrete and testable)
- Aim for tasks that could map to 1-3 beads during decompose

### 5. Write the Plan

Create the plan file at `.gromit/plans/<name>.md` with the following structure:

```markdown
---
id: <plan-name>
source_spec: <spec-name>
created: <YYYY-MM-DD>
decomposed: false
---

# <Title> Implementation Plan

**Goal:** [1-sentence summary of what we're building]

**Architecture:** [1-2 sentence summary of the approach]

**Tech Stack:** [Languages, frameworks, libraries involved]

**Spec:** `.gromit/specs/<spec-name>.md`

---

## Architecture

[The approved architecture proposal from checkpoint 1]

## Test Strategy

[The approved test strategy from checkpoint 2]

## Implementation Tasks

[The task breakdown with all tasks in dependency order]

---

## Notes

[Any additional context, warnings, or reminders for whoever implements this]
```

**Key Guidelines:**
- **Frontmatter**: `id` matches spec name, `source_spec` links back, `decomposed: false` gates the decompose stage
- **Natural structure**: Not rigidly templated — adapt sections as needed for the feature
- **LLM-consumable**: An LLM will read this during `gromit decompose`, so be clear and complete
- **Human-readable**: This gets reviewed before decompose, so make it scannable

### 6. Confirm and Finalize

After writing the plan:
- Use the Write tool to create the file at `.gromit/plans/<name>.md`
- Show the user the path and summarize what was created
- Confirm: "I've created the implementation plan at `.gromit/plans/<name>.md`. It includes [X] tasks covering [high-level summary]. Ready to run `gromit decompose <name>` to create beads, or would you like any adjustments?"

## Key Principles

1. **Spec-driven** - Start from the spec, not a vague description
2. **Codebase-aware** - Explore existing code to inform architecture
3. **Human checkpoints** - Get approval on architecture and tests before task breakdown
4. **Flexible format** - Natural structure, not rigid templates
5. **Decompose-ready** - Tasks must have files, acceptance criteria, and dependencies for LLM consumption
6. **One conversation** - Complete the full plan in one interactive session

## Preventing Duplication

Before writing the plan, check if a plan already exists:
- The CLI command (`gromit plan`) will prevent duplicate plans unless `--force` is used
- If you encounter a situation where a plan exists, inform the user: "A plan already exists at `.gromit/plans/<name>.md`. Please use `gromit plan <name> --force` if you want to regenerate it."

## Task Sizing Hints for Decompose

During task breakdown, keep in mind that each task will become 1-3 beads during decompose. Use these hints to size tasks appropriately:

**Bead Sizing Rules (for decompose reference):**
- **One concern per bead** - A single file or two tightly coupled files
- **1-3 acceptance criteria** - Concrete, testable criteria only; split if more than 3
- **Max 2 files touched** - If more, the task should be split
- **Self-contained** - Understandable without reading other beads
- **No ambiguity** - Implementation clear without design decisions

If a task has more than 3 acceptance criteria or touches more than 2-3 files, consider splitting it into multiple tasks.

## Model and Complexity

This skill uses **opus** for best results on architecture design, codebase analysis, and complex planning decisions. The checkpoint-based flow ensures human oversight at critical decision points.

## Example Interaction Flow

1. **You**: "I'll help you plan the implementation of `user-authentication`. Let me explore the codebase to understand how this fits with existing patterns..."
2. **You**: [Read spec, explore codebase with Glob/Grep/Read]
3. **You**: "I found that the current app uses a simple token in localStorage. Based on the spec requirements for JWT and refresh tokens, here's my architecture proposal: [detailed architecture]. Does this approach look good?"
4. **User**: "Yes, but can we store refresh tokens in httpOnly cookies instead of localStorage?"
5. **You**: "Good point for security. I'll update the architecture to use httpOnly cookies for refresh tokens. [Updated architecture]. Better?"
6. **User**: "Perfect"
7. **You**: "Great! Now for testing: [detailed test strategy]. Does this testing approach cover what we need?"
8. **User**: "Looks good"
9. **You**: "Excellent! I'll break this into tasks..." [Writes plan with task breakdown]
10. **You**: "I've created the implementation plan at `.gromit/plans/user-authentication.md`. It includes 8 tasks covering the auth middleware, token generation, refresh flow, and tests. Ready to run `gromit decompose user-authentication` to create beads?"

## Integration with Gromit Pipeline

This skill is the **Plan** stage in Gromit's four-stage pipeline:
1. **Capture** (`gromit add`) - Raw ideas → backlog entries
2. **Refine** (`gromit refine`) - Backlog items or ad-hoc ideas → specs
3. **Plan** (`gromit plan`) - Specs → implementation plans (THIS SKILL)
4. **Decompose** (`gromit decompose`) - Plans → bd beads

The plan you produce becomes the input to `gromit decompose`, which will automatically create beads following the task breakdown and bead sizing rules.
