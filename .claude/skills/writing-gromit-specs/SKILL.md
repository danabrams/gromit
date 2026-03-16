---
name: writing-gromit-specs
description: Co-write a Gromit Next spec interactively. Explores the idea through structured questioning, then produces a well-scoped spec with vision, architecture, acceptance criteria, and behavioral scenarios. Works in Claude Code, Codex, or mobile apps.
version: 1.0.0
---

# Writing Gromit Next Specs

Co-write a Gromit Next spec through structured dialogue. The output is a markdown spec document that Gromit Next can execute.

## When to Use This Skill

- The user wants to create a new spec for Gromit Next
- The user says "let's write a spec" or "I have an idea for a feature"
- The user invokes `/writing-gromit-specs`

## Two-Phase Flow

This skill has two phases. Phase 1 must complete before Phase 2 begins.

```
Phase 1: Exploration          Phase 2: Spec Drafting
┌─────────────────────┐       ┌──────────────────────────┐
│ Understand context   │       │ Assess scope (split?)     │
│ Ask questions (1×1)  │──────▶│ Draft sections (1×1)      │
│ Propose approaches   │       │ Present final spec        │
│ Reach agreement      │       │ Output                    │
└─────────────────────┘       └──────────────────────────┘
```

---

## Phase 1: Exploration

Understand what the user wants to build before writing anything.

### 1. Understand the context

- Ask what project or area of the codebase this is for
- If you have file access, read relevant code, existing specs, and recent commits
- Identify what exists today and what's missing

### 2. Ask clarifying questions

- **One question at a time.** Do not batch questions.
- Prefer multiple choice when possible, open-ended when needed
- Focus on: purpose, constraints, who it's for, what exists today, what success looks like
- Keep asking until you have a clear picture — don't rush to drafting

### 3. Propose 2-3 approaches

- Present different ways to solve the problem with trade-offs
- Lead with your recommendation and explain why
- Include a "do nothing" or "defer" option if appropriate

### 4. Reach agreement

- Get explicit confirmation on the chosen approach before moving to Phase 2

<HARD-GATE>
Do NOT begin Phase 2 until the user has agreed on an approach. No drafting, no section writing, no spec structure until Phase 1 is complete.
</HARD-GATE>

---

## Phase 2: Spec Drafting

### 5. Assess scope

Before drafting, evaluate whether the spec is too large.

**Signs a spec needs splitting:**
- More than ~8-10 acceptance criteria
- More than ~4-5 scenarios
- Multiple independent user-visible behaviors being introduced
- Work that would take more than a few days of focused implementation

**How to split — by end-to-end functional flow, NEVER by component:**

Each sub-spec must deliver a complete, testable behavior that works end to end. Every spec touches all layers needed for its flow.

✅ Good split: "Spec A delivers Claude as a working provider end-to-end. Spec B adds multi-provider routing end-to-end."

❌ Bad split: "Spec A builds the adapter layer. Spec B wires it up." This is a component split — it leaves Spec A delivering nothing usable on its own.

**Anti-pattern to flag:** If a proposed spec "adds infrastructure" or "builds the foundation" without delivering a working behavior, it's a component split. Push back. Every spec should produce something that works.

*Cautionary example: the 0002a/b/c/d series split by concern (execution loop, review/acceptance, adapter layer, routing) rather than by functional flow. This created specs that couldn't be tested independently and had tight coupling across the split boundary.*

**Dependency ordering between split specs:**
- Each spec must explicitly declare dependencies: `Depends on: spec-NNNN`
- The Non-goals section of spec N should reference what's deferred to spec N+1
- A dependent spec cannot run until its dependency is complete
- Make dependency ordering clear and unambiguous

If splitting is needed, agree on the split and which spec to draft first before continuing.

### 6. Draft sections incrementally

Present each section one at a time. Get approval before moving to the next.

Draft in this order:

1. **Vision** (if applicable)
2. **Summary + Goals + Non-goals**
3. **Architecture**
4. **Acceptance Criteria**
5. **Scenarios**
6. **Validation**

### 7. Present complete spec

After all sections are approved individually, present the full assembled spec for final review.

### 8. Output

- **CLI (Claude Code / Codex):** Write the spec to a file (e.g., `specs/<spec-id>.md`)
- **Mobile / chat:** Output the complete spec as a markdown code block for the user to save

---

## Spec Format

```markdown
# Spec NNNN — Title

## spec_id
kebab-case-identifier

## Depends on
spec-NNNN (if this spec depends on another — omit if standalone)

## Vision
Why this change exists. What problem it solves. The philosophy or reason
behind the change. What's wrong with the status quo.

(Omit for pure refactors, infrastructure, or specs where the "why" is
self-evident.)

## Summary
One paragraph: what this spec delivers, end to end.

## Goals
### Primary
- Goal 1
- Goal 2

### Secondary
- Nice-to-have goal (optional section)

## Non-goals
Explicit boundaries. What's deferred and to which spec.
- Not doing X (deferred to Spec NNNN+1)
- Not doing Y (out of scope entirely)

## Architecture
Key design decisions, component interactions, data flow.
Include code sketches where they clarify intent.
Keep it focused on decisions that constrain implementation,
not implementation details.

## Acceptance Criteria
Numbered, specific, testable statements. Each one should be
verifiable by a machine or a human reviewer.

1. When X, the system does Y
2. Z is persisted in format W
3. All existing tests continue to pass
4. ...

## Scenarios
Narrative use cases with concrete inputs and expected outcomes.
These are the source of truth for behavior. They should be detailed
enough that writing contract tests later is mechanical translation —
no creative interpretation needed.

### Scenario: descriptive name
**Given:** preconditions and setup
**When:** the action or trigger
**Then:** expected outcome, including observable state changes
**Notes:** edge cases, error conditions, or fixture/data needs

### Scenario: another descriptive name
...

## Validation
Commands that verify the spec is implemented correctly.
- `go test ./path/to/...`
- `go vet ./...`
- Any other verification steps
```

---

## Section Guidance

### Vision
- Write this for specs that change system behavior or introduce new capabilities
- Focus on the problem, not the solution — the solution is in Architecture
- Keep it to 2-4 sentences unless the motivation is genuinely complex
- Omit entirely for mechanical refactors or infrastructure work

### Acceptance Criteria
- Each criterion must be independently testable
- Use "When X, then Y" format when describing behavior
- Include negative criteria where important ("does NOT affect existing Z")
- If you're past ~8-10 criteria, the spec is probably too big — revisit scope

### Scenarios
- Each scenario should describe one end-to-end behavior
- Use concrete values, not abstractions ("5 tasks" not "multiple tasks")
- Include the happy path first, then error/edge cases
- Note any fixtures, dummy data, or setup needed to test this scenario
- If you're past ~4-5 scenarios, the spec may need splitting

### Architecture
- Focus on decisions that constrain implementation: interfaces, data flow, key types
- Include code sketches (type signatures, function signatures) when they clarify
- Don't over-specify — leave room for implementation decisions
- Call out what's new vs. what's being extended

---

## Key Principles

- **One question at a time** — don't overwhelm the user
- **Split by functional flow, not by component** — every spec delivers working behavior
- **Scenarios are source of truth** — detailed enough for mechanical test translation
- **Incremental validation** — approve each section before moving on
- **YAGNI** — remove anything that isn't needed for this spec's functional flow
- **Explicit dependencies** — split specs declare what they depend on and what they defer
