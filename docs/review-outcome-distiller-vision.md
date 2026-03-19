# Vision: Review Outcome Distiller

## Why this should exist

After Spec 0004b, Gromit can do something important but incomplete: it can generate a review packet, guide a human through review, and record a structured outcome. That gives the system an auditable verdict, but the verdict is still mostly terminal. The run ends, the human judgment is preserved, and the next run starts almost from scratch.

That leaves the highest-value signal in the system underused.

The review outcome is the only place where all of the following meet:

- the original product intent
- the machine's proof of correctness
- the human's judgment about whether the result was actually good enough
- the distinction between implementation failure and vision change

The next major step for Gromit should be to convert that review outcome into reusable operational knowledge. Review should stop being just an approval checkpoint and become the system's learning engine.

## Core idea

Add a `review outcome distiller`: a subsystem that reads the completed review artifacts for a run and produces structured, reviewable proposals for how the system should improve itself.

In concrete terms, the distiller would consume:

- `review-outcome.json`
- the review packet from 0004a
- validation, acceptance, and review artifacts
- the spec text and scenarios
- task results and replan history

It would then emit one or more proposed improvements in three buckets:

1. `doctrine` proposals
2. `validation` proposals
3. `execution heuristic` proposals

The key shift is this: every completed run becomes not only a delivery attempt, but also a source of system-improving evidence.

## What it would produce

The distiller should not silently rewrite project behavior. It should generate explicit proposals that a human can accept, reject, or edit.

Suggested artifacts:

- `evidence/review-distillation.json`
- `evidence/review-distillation.md`
- `doctrine/proposals/<run-id>.json`
- `validation/proposals/<run-id>.json`
- `heuristics/proposals/<run-id>.json`

Each proposal should answer four questions:

1. What happened?
2. What does that imply was missing or overly strict?
3. What durable system change would reduce recurrence?
4. How confident is the system that this change is correct?

Example outputs:

- "The run was marked `rework_implementation_gap` because the UI behavior matched the spec text but lacked a manual verification path for keyboard navigation. Propose a doctrine rule: interactive UI specs must include accessibility-oriented scenario checks."
- "A contract repeatedly failed because the assertion targeted the wrong file. Propose a validation rule: avoid file-path-specific contract assertions when behavior can be verified by scenario tests instead."
- "Three accepted runs in a row succeeded when scenario tests used package-scoped compile checks before full `go test ./...`. Propose reinforcing that as a preferred test-writing heuristic."

## How it should reason

The distiller's job is not to summarize the run. It is to classify the meaning of the review outcome and convert it into durable changes.

### If outcome is `accepted`

The question is: what proof structure was sufficient?

The distiller should look for patterns worth reinforcing:

- Which validation signals were present?
- Which review concerns were absent?
- Which manual checks consistently passed without surprises?
- Which scenario-test or contract patterns appear to be high-signal and low-noise?

Typical output:

- promote a successful pattern into doctrine
- mark a validation strategy as preferred
- strengthen trust in an execution heuristic that correlated with successful outcomes

### If outcome is `rework_implementation_gap`

The question is: what guardrail was missing?

The distiller should classify the gap:

- missing contract
- weak scenario test
- poor planner decomposition
- inadequate doctrine guidance
- insufficient review packet emphasis
- bad execution heuristic

Typical output:

- add a new doctrine rule
- require a stronger scenario-test pattern for similar specs
- adjust planning prompts to request a specific kind of task split
- add a review warning when a known weak evidence pattern appears

### If outcome is `rework_vision_change`

The question is: what changed in product direction, and how do we avoid misclassifying that as implementation failure later?

This outcome should mostly feed product memory and spec refinement guidance, not implementation doctrine. The system should learn:

- which assumptions were unstable
- which kinds of specs need earlier clarification
- which review questions should be asked before execution starts

Typical output:

- refinement guidance proposals
- spec-template improvements
- prompts that ask for unresolved product tradeoffs earlier

## Human control model

This must remain a human-supervised learning loop.

The distiller should generate proposals, not auto-merge them into doctrine or policy. The human reviewer or project owner should be able to:

- accept a proposal
- reject a proposal
- edit and then accept a proposal
- mark a proposal as local to one project or global across projects

That keeps strategic control human while still letting the machine do the expensive synthesis work.

The right operating model is:

`review outcome -> distillation proposals -> human approval -> doctrine / validation / heuristic update`

## What makes this radically useful

Most systems treat review as a gate. Gromit can do something better: treat review as a compiler from human judgment into future system behavior.

That is more than observability. It is accretive improvement.

Without this loop:

- each accepted run proves only that one run succeeded
- each rework teaches the human something, but not the system

With this loop:

- accepted runs teach the system what evidence patterns deserve more trust
- implementation-gap reworks teach the system what safeguards were missing
- vision-change reworks teach the system where refinement and planning need better clarification

This is exactly aligned with the program-level goal in `VISION.md`: increase the share of work accepted without rework. The distiller is the missing mechanism that lets review outcomes become operational leverage instead of historical records.

## Concrete first version

The first version should stay narrow and high-signal.

Scope:

- run only after a terminal review outcome exists
- generate proposals, never auto-apply them
- support a small fixed schema for proposal types
- prefer deterministic extraction where possible, with LLM synthesis only for proposal drafting

Initial proposal types:

1. `doctrine_rule`
2. `validation_gap`
3. `planner_heuristic`
4. `refinement_guidance`

Each proposal should include:

- stable ID
- source run ID
- source outcome
- evidence references
- proposal type
- proposed change text
- rationale
- confidence
- suggested scope (`local` or `global`)

## Success criteria

This addition is successful if, within a few weeks of use, it does three things:

1. produces proposals that humans frequently accept with light editing
2. reduces repeat implementation-gap rework for the same classes of failures
3. creates a visible chain from review outcome to improved future execution behavior

The important idea is simple: once Gromit can reliably capture human review, the next compounding step is to turn that review into durable system learning. The review outcome distiller is the component that closes that loop.
