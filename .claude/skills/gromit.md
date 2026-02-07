---
name: gromit
description: Orchestrate the full Gromit pipeline from Claude Code. Shows pipeline dashboard, launches stages with fresh context, and dispatches work items. Use when working with Gromit projects.
version: 1.0.0
---

# Gromit Orchestrator Skill

Provides a unified `/gromit` command in Claude Code that orchestrates the full Gromit pipeline. Shows pipeline status, launches stages with fresh context, and lets users move from idea to implementation without leaving Claude Code.

## When to Use This Skill

Use this skill when:
- The user invokes `/gromit` in a Claude Code session
- The user asks about Gromit pipeline status or what to work on next
- The user wants to refine an idea, plan a spec, decompose a plan, or run beads
- Claude detects a gromit project (presence of `gromit.yaml` and `.gromit/`)

## Methodology

This skill acts as a lightweight dispatcher for the Gromit pipeline. It reads state from existing files, presents a dashboard, and launches each stage appropriately.

### 1. Display Pipeline Dashboard

When invoked, read pipeline state and display a status dashboard:

**State Sources:**
- **Backlog**: Read `.gromit/backlog.jsonl` — count lines where `status` field is empty or missing (unrefined ideas)
- **Specs**: List files in `.gromit/specs/` that don't have corresponding plans (ready to plan)
- **Plans**: List files in `.gromit/plans/` where frontmatter has `decomposed: false` (ready to decompose)
- **Beads**: Run `bd ready --json --limit 1` to check for ready work items

**Dashboard Format:**
```
Pipeline Status:
  Backlog: [N] unrefined ideas
  Specs:   [N] ready to plan ([spec-names])
  Plans:   [N] ready to decompose ([plan-names])
  Beads:   [N] ready to run

Recommended: [Next action]
```

**Recommendation Logic:**
- If unrefined ideas exist → "Refine backlog item [first-unrefined]"
- Else if specs ready to plan → "Plan spec '[first-spec]'"
- Else if plans ready to decompose → "Decompose plan '[first-plan]'"
- Else if beads ready → "Run next bead"
- Else → "Pipeline clear — use 'gromit add' to add new ideas"

After showing the dashboard, ask: "What would you like to do?" or allow the user to state intent directly (e.g., "refine idea X", "plan spec Y", "decompose Z", "run").

### 2. Stage Dispatch: Interactive Stages (refine, plan)

For **refine** and **plan** stages, use the `/clear` + SessionStart hook pattern to get fresh context:

**Refine Dispatch:**
1. Gather context from user: which backlog item to refine (by ID or text match)
2. Read the backlog entry from `.gromit/backlog.jsonl` to get full details
3. Write pipeline state file at `.gromit/pipeline-state.json`:
   ```json
   {
     "stage": "refine",
     "inputs": {
       "idea_text": "[backlog item text]",
       "backlog_id": "[backlog item id]",
       "specs_dir": ".gromit/specs"
     },
     "created_at": "[ISO 8601 timestamp]"
   }
   ```
4. Tell user: "Ready to refine '[idea summary]'. Type `/clear` to start the refine session with fresh context."
5. Wait for user to type `/clear` — the SessionStart hook will handle the rest

**Plan Dispatch:**
1. Gather context from user: which spec to plan (by name)
2. Read the spec file from `.gromit/specs/<name>.md` to get full content
3. Run `bd list --json --status open` to get open beads (for context)
4. Write pipeline state file at `.gromit/pipeline-state.json`:
   ```json
   {
     "stage": "plan",
     "inputs": {
       "spec_name": "[spec-name]",
       "spec_path": ".gromit/specs/[spec-name].md",
       "spec_content": "[full spec content]",
       "plans_dir": ".gromit/plans",
       "plan_path": ".gromit/plans/[spec-name].md",
       "open_beads": "[bd list output]"
     },
     "created_at": "[ISO 8601 timestamp]"
   }
   ```
5. Tell user: "Ready to plan '[spec-name]'. Type `/clear` to start the plan session with fresh context."
6. Wait for user to type `/clear` — the SessionStart hook will handle the rest

**CRITICAL**: Do NOT output the skill content yourself. The SessionStart hook will inject the appropriate skill content (refine or plan) after `/clear` is typed. Your job is to gather context and write the pipeline state file.

### 3. Stage Dispatch: Non-Interactive Stages (decompose)

For **decompose**, launch a Task subagent with fresh context:

**Decompose Dispatch:**
1. Gather context from user: which plan to decompose (by name)
2. Verify the plan exists at `.gromit/plans/<name>.md`
3. Launch Task subagent with:
   - **subagent_type**: "general-purpose"
   - **prompt**: Build a prompt containing:
     - The decompose skill content (see placeholder marker below)
     - The plan file path to read
     - Instructions to output JSON only (no explanations)
4. When the subagent returns, parse the JSON output
5. For each bead in the JSON:
   - Run `bd create "[title]" --priority [priority] --description "[description]" --accept "[criterion]"` for each acceptance criterion
   - Add `--depends-on [bead-id]` flags based on dependency mapping
   - Add `--label spec:[spec-name]` to track which spec this came from
6. Update the plan file frontmatter: set `decomposed: true`
7. Summarize results to user: "Created [N] beads for '[plan-name]'. Run `bd list` or `/gromit` to see status."

**IMPORTANT**: The decompose stage is fully automated — no user interaction during execution. The Task subagent reads the plan, applies sizing rules, and returns JSON. You handle bead creation.

### 4. Stage Dispatch: Simple Commands (add, run, status)

For simple operations, use direct Bash execution:

**Add:**
- Run `gromit add "[idea text]"` via Bash
- Show confirmation: "Added idea to backlog: [idea summary]"

**Run:**
- Run `gromit run [flags]` via Bash (pass any user-specified flags like `-n 5` or `--time-budget 30`)
- Stream output to user
- Show completion summary

**Status/Queue/Board:**
- Run `gromit status`, `gromit queue`, or equivalent `bd` commands via Bash
- Display output to user

### 5. Pipeline State File Format

The pipeline state file (`.gromit/pipeline-state.json`) is a transient file that bridges the `/clear` boundary. It MUST be consumed (deleted) by the SessionStart hook after reading.

**Format:**
```json
{
  "stage": "refine" | "plan",
  "inputs": {
    // Stage-specific inputs (see dispatch instructions above)
  },
  "created_at": "2026-02-07T10:30:00Z"
}
```

**Lifecycle:**
1. Orchestrator skill writes the file when preparing an interactive stage
2. User types `/clear` — conversation context is wiped
3. SessionStart hook fires, detects the file, reads it
4. Hook outputs skill content + context to stdout (injected into fresh session)
5. Hook deletes the file
6. Subsequent `/clear` commands are no-ops (no file exists)

### 6. SessionStart Hook Integration

The SessionStart hook (`pipeline-resume.sh`) is installed by `gromit install-skill` and registered in `.claude/settings.json`. The hook script:

1. Checks if `.gromit/pipeline-state.json` exists
2. If no — exits silently (exit code 0, no output)
3. If yes:
   - Reads the JSON file
   - Determines which stage to resume (from `stage` field)
   - Outputs the appropriate skill content (refine or plan) plus the gathered inputs
   - Deletes the pipeline state file
   - Exits with code 0

**Hook Output Format:**

For **refine** stage:
```
[GROMIT REFINE SKILL CONTENT - see placeholder marker below]

---

## Context for This Session

You are refining the following backlog item:

**ID:** [backlog_id]
**Idea:** [idea_text]

Please follow the refine methodology to transform this into a structured spec at `.gromit/specs/`.
```

For **plan** stage:
```
[GROMIT PLAN SKILL CONTENT - see placeholder marker below]

---

## Context for This Session

You are planning implementation for the following spec:

**Spec:** [spec_name]
**Path:** [spec_path]

**Spec Content:**
[spec_content]

**Open Beads in Project:**
[open_beads or "No open beads"]

Please follow the plan methodology to create an implementation plan at `.gromit/plans/[spec_name].md`.
```

The hook script will read these skill contents from embedded markers in the installed skill file (see below).

## Embedded Skill Content (Placeholder Markers)

The orchestrator skill file includes the full content of the refine, plan, and decompose skills so the SessionStart hook and Task subagents can use them without invoking the `gromit` binary.

**When this skill is installed** (via `gromit install-skill`), the command will:
1. Read the existing `skills/gromit-refine/SKILL.md` file
2. Read the existing `skills/gromit-plan/SKILL.md` file
3. Read the existing `skills/gromit-decompose/SKILL.md` file
4. Replace the placeholder markers below with the actual skill content
5. Write the result to `.claude/skills/gromit.md`

### Refine Skill Content Placeholder

```
<!-- BEGIN GROMIT-REFINE-SKILL -->
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

<!-- END GROMIT-REFINE-SKILL -->
```

### Plan Skill Content Placeholder

```
<!-- BEGIN GROMIT-PLAN-SKILL -->
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

<!-- END GROMIT-PLAN-SKILL -->
```

### Decompose Skill Content Placeholder

```
<!-- BEGIN GROMIT-DECOMPOSE-SKILL -->
---
name: gromit-decompose
description: Use when decomposing implementation plans into bd beads. Reads plans, extracts tasks, and outputs JSON array of bead definitions following strict sizing rules. Non-interactive, produces machine-readable output only.
version: 1.0.0
---

# Gromit Decompose Skill

Transforms implementation plans into structured bead definitions for automated creation via `bd create`. This is a non-interactive skill that reads plans and outputs **JSON only** — no markdown, no conversation, no explanations.

## When to Use This Skill

Use this skill when:
- You have an implementation plan that needs to be broken into beads
- You're ready to create work items from a completed plan
- You need to map logical tasks to executable beads following sizing rules

## Methodology

This skill follows a strict non-interactive process:

### 1. Read the Plan

When the skill starts:
- Read the plan file from `.gromit/plans/<name>.md`
- Extract the plan frontmatter (especially `id` and `source_spec`)
- Read the Implementation Tasks section
- Understand task dependencies and files affected

**DO NOT** output any text at this stage. Work silently.

### 2. Apply Bead Sizing Rules

Each task in the plan must be mapped to 1-3 beads following these **strict sizing rules**:

**Bead Sizing Rules:**
- **One concern per bead** — A single file or two tightly coupled files
- **1-3 acceptance criteria** — Concrete, testable criteria only; split if more than 3
- **Max 2 files touched** — If more, split the task into multiple beads
- **Self-contained** — Understandable without reading other beads
- **No ambiguity** — Claude implements without making design decisions
- **Clear definition of done** — Each criterion has an obvious pass/fail test

**Splitting Logic:**
- If a task touches 3+ files → split by file or by logical grouping
- If a task has 4+ acceptance criteria → split into multiple beads with 1-3 criteria each
- If a task has both implementation and tests → consider separate beads (implementation first, then tests)
- If a task has multiple concerns (e.g., types + logic + API) → split by concern

**DO NOT** create beads that are too small (e.g., single-line changes) unless they're genuinely independent concerns.

### 3. Map Tasks to Beads

For each task in the plan:

1. **Assess complexity**: Count files, count acceptance criteria, identify concerns
2. **Decide split strategy**: If the task exceeds sizing rules, determine how to split
3. **Create bead definitions**: Generate 1-3 beads per task with proper metadata

**Bead Definition Fields:**
- `title`: Clear, concise title (imperative form: "Add X", "Implement Y", "Fix Z")
- `description`: 2-4 sentences describing what needs to be done, including files to touch
- `priority`: `P1` (use plan's default priority — P1 for most features)
- `acceptance_criteria`: Array of 1-3 concrete, testable criteria
- `depends_on_index`: Array of zero-based indices of beads this depends on (or empty array if no dependencies)

**Title Guidelines:**
- Start with a verb: "Add", "Implement", "Create", "Update", "Fix", "Refactor"
- Be specific: "Add JWT token validation" not "Add auth"
- Keep it under 60 characters
- Reflect the bead's single concern

**Description Guidelines:**
- First sentence: What to do
- Second sentence: Which files to modify/create
- Optional third sentence: Key implementation notes
- Reference specific functions, types, or APIs if helpful
- DO NOT include rationale or background — keep it action-oriented

**Acceptance Criteria Guidelines:**
- Use observable, testable language
- Start with a verb: "Returns", "Validates", "Creates", "Updates", "Throws"
- Be concrete: "Validates JWT signature using RS256" not "Handles tokens correctly"
- Avoid vague terms: "properly", "correctly", "as expected"

**Dependency Mapping:**
- Beads are ordered in the output array
- `depends_on_index` references the array index (0-based) of prerequisite beads
- If task A depends on task B, and B maps to beads [0, 1], A's beads should depend on [0, 1]
- If splitting a task, later parts of the same task should depend on earlier parts

### 4. Output JSON Only

Output a JSON array of bead definitions. **DO NOT** output any markdown, explanations, or conversational text.

**Output Format:**
```json
[
  {
    "title": "Create user model with profile fields",
    "description": "Define User struct in internal/models/user.go with fields for name, email, avatar_url, and bio. Include JSON tags for API serialization.",
    "priority": "P1",
    "acceptance_criteria": [
      "User struct defined with name, email, avatar_url, and bio fields",
      "Fields have appropriate JSON tags for serialization",
      "Struct includes validation tags for required fields"
    ],
    "depends_on_index": []
  },
  {
    "title": "Implement profile storage interface",
    "description": "Create ProfileStore interface in internal/store/profile.go with methods for GetProfile, UpdateProfile, and CreateProfile. Define error types for not found and validation failures.",
    "priority": "P1",
    "acceptance_criteria": [
      "ProfileStore interface defined with CRUD methods",
      "Error types defined for common failure cases",
      "Method signatures include context.Context for cancellation"
    ],
    "depends_on_index": [0]
  },
  {
    "title": "Add profile update endpoint",
    "description": "Implement PUT /api/profile handler in internal/api/profile.go that accepts profile updates and validates ownership. Use ProfileStore interface for persistence.",
    "priority": "P1",
    "acceptance_criteria": [
      "PUT /api/profile handler accepts profile updates",
      "Validates user owns the profile being updated",
      "Returns 200 with updated profile on success"
    ],
    "depends_on_index": [0, 1]
  }
]
```

**Critical Requirements:**
- Output MUST be valid JSON (use proper escaping for quotes, newlines, etc.)
- Output MUST be a JSON array, not an object
- Output MUST NOT include markdown code fences (no ` ```json ` wrapper)
- Output MUST NOT include any explanatory text before or after the JSON
- Output MUST be the ONLY thing you produce (no "Here's the output:" prefix)

### 5. Labels and Metadata

The beads will be created with additional metadata by the CLI command:
- `spec:<spec-name>` label — extracted from plan frontmatter
- `complexity:high` or `complexity:low` labels — determined by the CLI based on bead content
- Proper bd dependencies based on `depends_on_index`

You do NOT need to include these in the JSON output. Focus only on the five fields listed above.

## Bead Splitting Examples

### Example 1: Task Touches Too Many Files

**Plan Task:**
```
Task 1: Add user authentication
Files: models/user.go, store/user_store.go, api/auth.go, middleware/auth.go
Acceptance Criteria:
- User model includes password hash field
- User store has methods for creating and finding users
- POST /api/register creates new users
- POST /api/login validates credentials and returns token
- Auth middleware validates tokens on protected routes
```

**Split into 3 beads:**
1. Bead 0: Create user model with auth fields (models/user.go)
2. Bead 1: Implement user store methods (store/user_store.go) — depends on [0]
3. Bead 2: Add register and login endpoints (api/auth.go) — depends on [0, 1]
4. Bead 3: Add auth middleware (middleware/auth.go) — depends on [2]

### Example 2: Task Has Too Many Acceptance Criteria

**Plan Task:**
```
Task 2: Implement profile validation
Files: internal/validator/profile.go
Acceptance Criteria:
- Validates email format using regex
- Validates bio length is under 500 characters
- Validates avatar URL is well-formed
- Validates name is not empty
- Returns structured error messages
- Handles nil pointers gracefully
```

**Split into 2 beads:**
1. Bead 0: Implement field validators (email, bio, avatar, name) — 3 criteria
2. Bead 1: Add validation error handling — 2 criteria, depends on [0]

### Example 3: Implementation + Tests

**Plan Task:**
```
Task 3: Build JWT token generator
Files: internal/auth/token.go, internal/auth/token_test.go
Acceptance Criteria:
- Generates JWT tokens with user ID and expiration
- Signs tokens with RS256 algorithm
- Includes refresh token in response
- Unit tests cover token generation
- Unit tests cover signature verification
```

**Split into 2 beads:**
1. Bead 0: Implement JWT token generation and signing — 3 criteria (implementation only)
2. Bead 1: Add token generator tests — 2 criteria (tests), depends on [0]

## Key Principles

1. **Non-interactive** — No conversation, no questions, no explanations
2. **JSON only** — Output must be valid JSON, nothing else
3. **Strict sizing** — Enforce 1-2 files, 1-3 criteria per bead
4. **Self-contained beads** — Each bead is understandable on its own
5. **Concrete criteria** — Every acceptance criterion must be testable
6. **Dependency integrity** — Preserve task dependencies when splitting

## Common Mistakes to Avoid

1. **Outputting markdown**: DO NOT wrap JSON in code fences or add explanatory text
2. **Too many criteria**: If you're creating a bead with 4+ criteria, you need to split it
3. **Too many files**: If a bead touches 3+ files, it's too large
4. **Vague criteria**: "Works correctly" is not testable; "Returns 404 when user not found" is
5. **Missing dependencies**: If bead B uses types from bead A, B must depend on A
6. **Breaking atomicity**: Each bead should be independently committable

## Model and Complexity

This skill uses **haiku** for cost-effective, fast processing. The task is well-defined and mechanical — no complex decisions required. The plan has already been reviewed and approved by a human, so decompose is purely execution.

## Integration with Gromit Pipeline

This skill is the **Decompose** stage in Gromit's four-stage pipeline:
1. **Capture** (`gromit add`) - Raw ideas → backlog entries
2. **Refine** (`gromit refine`) - Backlog items or ad-hoc ideas → specs
3. **Plan** (`gromit plan`) - Specs → implementation plans
4. **Decompose** (`gromit decompose`) - Plans → bd beads (THIS SKILL)

The JSON you produce is consumed by the CLI, which creates beads via `bd create` and adds labels.

<!-- END GROMIT-DECOMPOSE-SKILL -->
```

These markers allow the `gromit install-skill` command to build a self-contained skill file that the SessionStart hook and Task subagents can consume.

## Key Principles

1. **Dashboard-first** — Always show pipeline status before asking what to do
2. **Fresh context for interactive stages** — Use `/clear` + hook pattern for refine and plan
3. **Autonomous execution for non-interactive stages** — Use Task subagents for decompose
4. **Direct execution for simple commands** — Use Bash for add, run, status
5. **State in files** — Pipeline state file is the only bridge across `/clear`
6. **Consume state on read** — Hook deletes pipeline state file after injecting content
7. **Embedded skill content** — All stage skills are inlined so hook doesn't need `gromit` binary

## User Experience Flow

### Example: Refining an Idea

1. **User**: `/gromit`
2. **Skill**: Shows dashboard: "Backlog: 2 unrefined ideas. Recommended: Refine idea 'Add user profiles'"
3. **User**: "refine user profiles"
4. **Skill**: Writes pipeline state file, says "Ready to refine 'Add user profiles'. Type `/clear` to start."
5. **User**: `/clear` (context wiped)
6. **Hook**: Fires, reads pipeline state, outputs refine skill + idea context, deletes state file
7. **Claude**: Sees refine skill in fresh context, runs the refine flow interactively
8. **Result**: User collaborates with Claude to create `.gromit/specs/user-profiles.md`

### Example: Planning a Spec

1. **User**: `/gromit`
2. **Skill**: Shows dashboard: "Specs: 1 ready to plan (user-profiles). Recommended: Plan spec 'user-profiles'"
3. **User**: "plan it"
4. **Skill**: Writes pipeline state file, says "Ready to plan 'user-profiles'. Type `/clear` to start."
5. **User**: `/clear` (context wiped)
6. **Hook**: Fires, reads pipeline state, outputs plan skill + spec content, deletes state file
7. **Claude**: Sees plan skill in fresh context, runs the plan flow with checkpoints
8. **Result**: User approves architecture and tests, Claude writes `.gromit/plans/user-profiles.md`

### Example: Decomposing a Plan

1. **User**: `/gromit`
2. **Skill**: Shows dashboard: "Plans: 1 ready to decompose (user-profiles). Recommended: Decompose plan 'user-profiles'"
3. **User**: "decompose it"
4. **Skill**: Launches Task subagent with decompose skill + plan path
5. **Subagent**: Reads plan, applies sizing rules, returns JSON array of beads
6. **Skill**: Parses JSON, creates beads via `bd create` commands, updates plan frontmatter
7. **Skill**: "Created 6 beads for 'user-profiles'. Run `/gromit` to see updated status."

### Example: Running Beads

1. **User**: `/gromit`
2. **Skill**: Shows dashboard: "Beads: 4 ready to run. Recommended: Run next bead"
3. **User**: "run"
4. **Skill**: Executes `gromit run` via Bash
5. **Result**: Gromit CLI runs beads with fresh Claude sessions per the normal flow

## Model and Complexity

This skill uses **sonnet** for cost-effective orchestration. The skill doesn't do heavy codebase analysis — it reads state files, displays a dashboard, and dispatches to specialized stage skills that do the real work.

## Integration with Gromit Pipeline

This skill is the **orchestrator** for all pipeline stages:
1. **Capture** (`gromit add`) - Dispatched via Bash
2. **Refine** (interactive) - Dispatched via `/clear` + hook
3. **Plan** (interactive) - Dispatched via `/clear` + hook
4. **Decompose** (automated) - Dispatched via Task subagent
5. **Run** (`gromit run`) - Dispatched via Bash

The orchestrator is installed via `gromit install-skill` and registered as `/gromit` in Claude Code's skill system.

## Installation Notes

The `gromit install-skill` command handles:
- Creating `.gromit/hooks/` directory
- Writing `pipeline-resume.sh` hook script with embedded skill content extraction logic
- Writing `.claude/skills/gromit.md` with inlined skill content
- Registering SessionStart hook in `.claude/settings.json`
- Making hook script executable (`chmod +x`)

The command is idempotent — running it multiple times updates files to the latest version without breaking existing configuration.

## Tips

- **First time using `/gromit`?** The dashboard shows you exactly what's ready to work on
- **Pipeline seems empty?** Use `gromit add "your idea"` to capture something new
- **Stuck in a stage?** You can always `/clear` to start fresh (if no pipeline state exists)
- **Want to bypass the orchestrator?** Use `gromit` CLI commands directly in the terminal
- **Not sure what to do next?** The dashboard's recommendation follows the natural pipeline flow
