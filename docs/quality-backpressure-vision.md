# Vision: Quality Backpressure

## Why this should exist

Gromit already applies meaningful backpressure: independent validation, contract assertions, scenario tests, review, acceptance, and human review outcomes. That is a strong base, but it is still possible for a run to clear the pipeline and then fail a fresh manual LLM review in ways that feel obvious in retrospect. When that happens regularly, the issue is not just that the model made mistakes. It is that the system's existing proof structure is still weaker than the judgment the human is applying after the fact.

The next step is to raise the standard of proof before a run is considered done. That does not mean making every run maximally expensive. It means adding higher-signal forms of backpressure that better match the ways bugs actually escape: missing edge cases, shallow happy-path-only review, weak regression memory, and insufficient adversarial checking. The goal is simple: make the default pipeline catch the kinds of issues that are currently only caught in the post-run cleanup pass.

## Adversarial Second-Pass Review

The system should add an explicitly adversarial review pass whose job is to disprove the success of the run, not merely summarize it. This review should operate with fresh context and a different posture from the ordinary review stage. Instead of asking whether the implementation appears aligned, it should ask where the implementation is likely brittle, under-specified, misleading, or accidentally passing because the validation surface is too narrow.

This matters because many serious misses are not "code is obviously broken" failures. They are failures of pressure. A feature can satisfy the written happy path, pass deterministic checks, and still be wrong in the ways a skeptical reviewer notices immediately. An adversarial second pass turns that skepticism into a productized stage. If a human repeatedly finds the same class of issues with a final manual review, the system should absorb that review style and make it part of the loop.

## Counterexample Scenario Synthesis

Scenario writing today is strongest when the original spec already contains the right examples. The problem is that many bugs live in the missing examples: empty input, invalid input, duplicate input, off-by-one boundaries, stale state, conflicting state, or behavior after retries and partial failure. A system that only translates the scenarios it was given will usually under-test exactly the places where implementations drift.

The answer is to synthesize counterexample scenarios from the original scenarios and acceptance criteria. For every claimed behavior, the system should ask what nearby behavior would fail if the implementation were only superficially correct. That does not require open-ended creativity. It requires disciplined expansion around boundaries, negations, reversals, and persistence edges. This is one of the highest-leverage ways to make the validation surface feel less optimistic.

## Guardrail Promotion

Every issue that a human catches after a run should have a path to becoming durable system pressure. If the same kind of mistake can be spotted manually but is never converted into a test, contract, doctrine rule, playbook heuristic, or review heuristic, the system is not actually learning. It is just paying the same quality tax over and over.

The right model is that manual catches are raw material for guardrails. Some will become scenario tests. Some will become contract assertions. Some will become output snapshots. Some will become planner or reviewer heuristics. The important shift is cultural and architectural: a manually discovered failure mode should be treated as incomplete systemization, not as an unavoidable cost of shipping.

## Regression Corpus

A project should accumulate a living corpus of real escaped bugs, near-misses, and review catches, then replay them automatically. This creates a high-value regression surface grounded in the actual history of the system rather than an imagined catalog of what might go wrong. Unlike generic validation, a regression corpus has already proved its relevance.

This corpus is especially useful for user-visible behavior, data migrations, serializers, diff/output rendering, and state-machine transitions. It gives the system memory where generic review and testing are often too abstract. If a run fails in a way the team cares about once, future runs should have to prove that they do not fail that way again.

## Targeted Mutation Testing

Mutation testing is a strong tool when used selectively and a poor default when applied indiscriminately. Its value is not that it "adds more tests." Its value is that it measures whether the current tests are strong enough to notice meaningfully wrong code. If a mutant survives in a core logic path, the test suite may be structurally present but semantically weak.

The right vision is targeted mutation pressure, not universal mutation pressure. Apply it to touched packages, high-value pure logic, parsers, transforms, core business rules, and other deterministic code where the signal is high and the runtime is manageable. This makes mutation testing a quality amplifier instead of an expensive ritual. It should tell the system where tests are non-discriminating, not drown the project in compute cost.

## Property and Invariant Testing

Some domains are better defended by invariants than by examples. Planners, state machines, graph operations, counters, reconciliation logic, serialization round-trips, and financial or scheduling computations often have rules that should always hold regardless of the specific example. When those invariants are explicit, the system gains a much deeper kind of proof than ordinary scenario coverage.

Property and invariant testing matter because they catch classes of bugs rather than instances of bugs. They also pair well with mutation testing: a suite with strong invariants tends to kill far more meaningful mutants. If the project wants stronger backpressure without specifying every single case by hand, invariants are one of the most scalable ways to get there.

## Golden and Differential Testing

A large share of important defects are not deep algorithmic failures. They are output defects: a CLI field missing, a JSON shape drifting, a markdown artifact omitting a warning, or two commands disagreeing about the same state. These bugs often survive unit tests because each code path is locally correct while the outward behavior is globally inconsistent.

Golden and differential tests apply pressure exactly where users feel the product. Golden tests lock down important rendered outputs. Differential tests make multiple views of the same underlying state agree with each other. This is particularly valuable in systems like Gromit where evidence artifacts, summaries, review packets, and CLI surfaces all need to remain coherent under continuous change.

## Risk-Weighted Review Rigor

Not every run deserves the same level of scrutiny. Applying the heaviest gates to every change raises cost without necessarily improving outcomes. But applying the same moderate scrutiny to every run also leaves the system blind to risk spikes. A quality system should increase pressure when the change is riskier: larger diffs, broader surface area, persistence changes, concurrency, auth, degraded evidence, or low process trust.

Risk-weighted rigor lets the project spend quality budget where it matters most. Cheap runs stay cheap. Expensive review, stronger mutation gates, deeper adversarial passes, or broader regression replays can be triggered only when the risk profile justifies them. This keeps the pipeline economically sane while still making it much harder for the highest-cost failures to slip through.

## What success looks like

This direction succeeds when the final manual LLM review stops being a routine second validation lane and becomes an occasional exception path. The human should still be able to review strategically, but they should no longer be doing repetitive bug harvesting that the system could have done itself. That is the clearest sign that the backpressure is becoming real rather than ceremonial.

The deeper success condition is compounding quality. A bug found today should make tomorrow's runs harder to fake. A manual catch should narrow future failure modes. A risky change should face proportionally stronger proof demands. When that is true, the project is not merely testing more. It is learning how to require better evidence before it believes itself.
