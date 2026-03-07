---
id: constancy-of-purpose-vision
source_ideas: []
created: 2026-02-28
accepted: true
---

# Constancy of Purpose Vision Artifact

## Specification

Add a first-class `VISION.md` artifact at the repository root that defines the long-term target state of the development system (time horizon: two years). The artifact establishes a stable "constancy of purpose" reference so day-to-day rules, learnings, and specification decisions can be evaluated against a shared direction.

### Vision Artifact Requirements

`VISION.md` must describe the desired future system in concrete behavioral terms, including:

- The core mission and intended outcomes of the development system
- Observable characteristics of the "ideal" system after two years (for example reliability, cycle time, quality, autonomy boundaries, operator experience, and cost efficiency)
- Explicit guardrails that must remain true while improving the system
- How to evaluate whether a proposed rule, learning, or spec aligns with the long-term direction

The document is strategic and durable. It is not a sprint plan, implementation checklist, or release roadmap.

### Alignment Contract

When updating guidance artifacts (such as `RULES.md`, `LEARNINGS.md`, and new specs in `.gromit/specs/`), maintainers must be able to map the change to one or more principles in `VISION.md`, or identify it as intentionally out of scope.

This introduces a simple consistency check: short-term optimizations that conflict with long-term system outcomes are treated as misaligned unless explicitly justified.

### Maintenance Expectations

`VISION.md` is updated infrequently and only when strategic direction changes. Routine iteration output (task execution details, temporary incidents, one-off process adjustments) should remain in operational artifacts and not be embedded into the vision document.

## Acceptance Criteria

- A root-level `VISION.md` file exists and defines a two-year target state for the development system.
- `VISION.md` includes explicit sections for mission/outcomes, desired system characteristics, and non-negotiable guardrails.
- `VISION.md` defines how maintainers should assess whether new rules/learnings/specs align with the long-term direction.
- The document explicitly distinguishes long-term vision from near-term implementation planning.
- At least one existing guidance artifact (`README.md`, `RULES.md`, or `LEARNINGS.md`) references `VISION.md` as the strategic alignment source.

## Decisions

1. **Use a dedicated root artifact (`VISION.md`)** A standalone file makes the strategic north star explicit, stable, and easy to reference from operational documents.

2. **Set a two-year horizon** Two years is far enough to force system-level thinking and near enough to remain actionable for architecture and process decisions.

3. **Treat vision as an alignment contract, not a backlog** This keeps the artifact durable and prevents it from becoming a transient task list that loses strategic value.

4. **Require explicit alignment checks for guidance updates** The value of vision comes from ongoing decision pressure, not from the document's existence alone.

## Research & Context

### Current State

- The repository contains operational and tactical guidance (`README.md`, `LEARNINGS.md`, `.gromit/specs/*.md`) but no dedicated long-horizon vision artifact.
- Existing documentation describes workflows, execution loops, and learnings management, but does not provide a single strategic purpose document to evaluate tradeoffs over time.

### Relevant Files

- `README.md` — primary operator-facing system description and workflow documentation
- `LEARNINGS.md` — confirmed/provisional learning store used for iterative improvement
- `.gromit/specs/` — refined feature and behavior specifications that drive planning and execution

### Problem Framing

Without an explicit long-term vision, local optimizations can accumulate without a clear check against the intended future system. A dedicated `VISION.md` creates a consistent strategic frame for rule evolution, learning promotion, and spec refinement.
