# Gromit V2: What It Is, and What It Isn't

Gromit v2 is a verifiable software execution loop. It turns a written product spec into scoped implementation work, runs that work inside an isolated git worktree, validates the result, checks acceptance, and presents the outcome. The point is not to remove humans from the process. The point is to keep humans at the strategic layer while the system handles the repeatable execution mechanics with evidence, guardrails, and auditability.

## What It Is

- A spec-to-code execution system, not just a model prompt. The system owns planning, decomposition, implementation, validation, review, acceptance, presentation, and cleanup.
- A two-level orchestrator with clear control boundaries. The outer `SpecLoop` manages the lifecycle of a spec. The inner `BeadLoop` executes small work units through explicit stages: gate, build, validate, review, and epilogue.
- A bounded automation system. Stages have typed inputs and outputs, retries are explicit, generations are capped, dependencies are tracked, and failures can trigger decomposition or escalation instead of undefined behavior.
- An architecture optimized for outcomes that matter in `VISION.md`: less tactical human intervention, fewer escaped regressions, more first-pass acceptance, and higher confidence that delivered code matches product intent.
- Execution infrastructure for software creation. LLMs are only one component; git worktrees, task tracking, validation commands, review artifacts, acceptance checks, and event logs are equally important parts of the system.

## What It Isn't

- Not a free-form autonomous engineer that can improvise indefinitely. When it cannot make safe progress, it retries within limits, decomposes further, or stops and escalates.
- Not a replacement for product ownership or strategic judgment. Humans still define the vision, the spec, and whether the result matches intended direction.
- Not a black box that hides state in a giant runner object. The design intentionally separates stages, adapters, and loop control so behavior stays inspectable, replaceable, and evolvable.
- Not a velocity engine that optimizes for token output, code volume, or demo quality at the expense of correctness, safety, reversibility, or maintainability.
- Not a system that declares success because the model says it is done. Success requires validation evidence, acceptance checks, and a presentable outcome.

The simplest pitch is this: Gromit v2 is infrastructure for turning written intent into tested, reviewable, presentable software. It should automate execution details aggressively, but only inside explicit control loops that preserve human direction and stop when the system cannot prove the work is correct, aligned, and safe enough to continue.
