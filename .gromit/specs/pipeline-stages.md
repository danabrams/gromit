---
id: pipeline-stages
source_ideas: []
created: 2026-02-06
epic: multi-interface-architecture
---

# Pipeline Stages

## Specification

Gromit manages code work through a four-stage pipeline, each producing a durable artifact. The stages are: Capture, Refine, Plan, and Decompose.

| Stage | Command | Input | Output | Mode |
|-------|---------|-------|--------|------|
| Capture | `gromit add` | raw idea | `backlog.jsonl` entry | CLI prompts |
| Refine | `gromit refine` | backlog item or ad-hoc idea | `.gromit/specs/<name>.md` | Interactive, human-heavy |
| Plan | `gromit plan` | spec name | `.gromit/plans/<name>.md` | Interactive, Claude-heavy |
| Decompose | `gromit decompose` | plan name | bd beads | Automated (`--review` for checkpoint) |

Each stage is a separate command invocation. Running the command is the quality gate — no status fields gate transitions. Artifacts use YAML frontmatter for metadata linking. The spec name is the primary key that flows through the pipeline: `specs/oauth.md` produces `plans/oauth.md` produces beads with `spec:oauth` labels.

### Capture (gromit add — no changes)

`gromit add` works as it does today. Ideas are captured to `.gromit/backlog.jsonl` with auto-categorization (feature/bug/chore).

### Refine (gromit refine — rewrite)

Refine is a human-heavy interactive session where rough ideas become specs through codebase-aware conversation. Claude reads the codebase, asks clarifying questions one at a time, explores approaches with tradeoffs, and writes the spec when the conversation reaches clarity.

**Command variants:**
- `gromit refine` — lists unrefined backlog items, lets you pick one
- `gromit refine <backlog-id>` — refines a specific backlog item
- `gromit refine "some idea"` — refines an ad-hoc idea not in the backlog

**Behavior:**
1. Claude reads the codebase to understand existing patterns and architecture
2. Asks clarifying questions one at a time — purpose, constraints, scope, edge cases
3. Explores approaches with tradeoffs, recommends one
4. Writes the spec to `.gromit/specs/<name>.md`
5. If the input was a backlog item, marks it `status: refined` with the spec name linked

The spec name is chosen collaboratively — Claude proposes a slug, human approves or adjusts. External research (web search, docs) is available ad-hoc when the user asks for it.

### Plan (gromit plan — rewrite)

Plan is a Claude-heavy interactive session with human review checkpoints on architecture and test strategy. Claude reads the spec and codebase, proposes architecture and test plans, and writes the plan after human approval.

**Command variants:**
- `gromit plan <spec-name>` — reads the spec, launches interactive session
- `gromit plan` (no args) — lists available specs, lets you pick

**Behavior:**
1. Claude reads the spec and explores the codebase
2. Proposes architecture — how the feature fits into existing code, what changes where
3. **Checkpoint:** human reviews architecture before proceeding
4. Proposes test strategy — what to test, what level (unit/integration), what mocks
5. **Checkpoint:** human reviews test plan
6. Breaks the work into logical tasks with files, dependencies, and acceptance criteria
7. Writes the plan to `.gromit/plans/<name>.md`

Plan refuses to run if a plan already exists for that spec: "Plan already exists. Use `--force` to regenerate." The plan format is natural and flexible — structured enough for human readability, but not rigidly templated. Each task must include files affected, acceptance criteria, and dependencies, since an LLM will consume the plan during decompose.

### Decompose (gromit decompose — new command)

Decompose is fully automated. It runs Claude non-interactively with the plan as context. Claude extracts tasks, maps each to a bead following bead sizing rules, and creates them via `bd create`.

**Command:**
- `gromit decompose <plan-name>` — reads the plan, creates beads
- `gromit decompose <plan-name> --review` — shows proposed beads for approval before creating

**Behavior (default):**
1. Runs Claude non-interactively with the plan as context
2. Claude extracts tasks, maps each to a bead following bead sizing rules
3. Creates beads via `bd create` with title, description, acceptance criteria, priority, `spec:<name>` label, and dependencies
4. Prints a summary of what was created

Decompose checks the plan frontmatter for `decomposed: true`. If set, refuses unless `--force` is passed. After successful decomposition, sets `decomposed: true` and `decomposed_at` in the plan frontmatter.

## Acceptance Criteria

- `gromit refine` produces a spec file in `.gromit/specs/` with frontmatter metadata and the four sections (Specification, Acceptance Criteria, Decisions, Research & Context)
- `gromit refine` accepts no args (interactive picker), a backlog ID, or an ad-hoc string
- `gromit refine` marks backlog items as `status: refined` when a spec is produced
- `gromit plan` reads a spec and produces a plan file in `.gromit/plans/` with frontmatter linking to the source spec
- `gromit plan` refuses if a plan already exists unless `--force` is passed
- `gromit plan` accepts a spec name or no args (interactive picker)
- `gromit decompose` reads a plan and creates beads via `bd create` fully automatically
- `gromit decompose --review` shows proposed beads and waits for confirmation
- `gromit decompose` checks `decomposed: true` in plan frontmatter and refuses unless `--force`
- `gromit decompose` sets `decomposed: true` and `decomposed_at` in plan frontmatter after success
- Each stage has its own Claude skill: `gromit-refine`, `gromit-plan`, `gromit-decompose`
- Existing skill content (bead sizing rules, conversational style, complexity assessment) is preserved in rewritten skills

## Decisions

1. **Four explicit commands over combined workflows.** Each stage has genuinely different behavior and a different human/Claude balance. Separate commands create natural quality gates.

2. **Specs and plans in `.gromit/` not `docs/`.** These are pipeline artifacts, not documentation. `.gromit/specs/` and `.gromit/plans/` are parallel directories for parallel artifacts.

3. **Naming as primary key.** The spec name flows through the entire pipeline. No separate ID tracking system needed — `oauth` is `specs/oauth.md` is `plans/oauth.md` is `spec:oauth` on beads.

4. **YAML frontmatter for metadata.** Lightweight, human-readable, machine-parseable. Links artifacts across stages without a database.

5. **Spec format ordered by consumer priority.** Specification and Acceptance Criteria at top, then Decisions, then Research & Context. Tools reading the spec can stop early if they have what they need.

6. **Plan format is flexible, not rigid.** An LLM writes the plan and an LLM reads it for decompose. Rigid templating adds friction without value. The skill instructs Claude to include files, acceptance criteria, and dependencies per task.

7. **Decompose is fully automated by default.** The quality gate is reviewing the plan, not the beads. `--review` flag exists as an escape hatch for when you want to double-check.

8. **Refuse-with-force over silent overwrite.** Both `plan` and `decompose` refuse to run if their output already exists. `--force` is the explicit override. Prevents accidental duplication.

9. **Backlog items marked refined, not deleted.** Preserves history of what you started with. Status field prevents re-processing without losing the original idea.

10. **Evolve existing skills, don't rewrite from scratch.** The bead sizing rules, one-question-at-a-time approach, and complexity assessment are proven. Change the output targets, keep the conversational methodology.

## Research & Context

### Current State

The following already exists:
- `gromit add` captures ideas to `.gromit/backlog.jsonl` with auto-categorization
- `gromit backlog` lists/filters/deletes backlog items
- `gromit refine` launches Claude with backlog context and gromit-refine skill — currently produces beads directly
- `gromit plan` launches Claude with a feature description and gromit-plan skill — currently produces beads directly
- Specs live in `.gromit/specs/` and are referenced via `spec:<name>` labels on beads
- Plans live in `docs/plans/` as hand-created design documents
- Skills (`gromit-refine`, `gromit-plan`) have well-developed conversational methodology

### What Changes

- `gromit refine` stops producing beads, produces specs instead
- `gromit plan` stops accepting a feature string, accepts a spec name instead
- `gromit plan` stops producing beads, produces plans instead
- Plans move from `docs/plans/` to `.gromit/plans/`
- New `gromit decompose` command created for the beads stage
- New `gromit-decompose` skill created
- Backlog items gain a `status` field (default empty, set to `refined` when processed)
- Both existing skills rewritten to target new output artifacts while preserving conversational methodology
