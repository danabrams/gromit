# Spec 0004c — Review Outcome Distiller

## spec_id
0004c-review-outcome-distiller

## Depends on
- spec-0004b

## Vision

After 0004b, Gromit captures human review judgments as structured outcomes — but those judgments are terminal. Each run ends, the verdict is preserved, and the next run starts with no memory of what went wrong or right.

The review outcome is the only place where product intent, machine proof, and human judgment all meet. That makes it the highest-value signal in the system, and right now it's underused.

This spec adds a review outcome distiller: a subsystem that reads the completed review artifacts for a run and produces structured, human-reviewable proposals for how the system should improve. Accepted runs teach the system which evidence patterns deserve trust. Implementation-gap reworks reveal missing guardrails. Vision-change reworks expose where refinement needs earlier clarification. Every completed run becomes not just a delivery attempt, but a source of system-improving evidence.

The distiller generates proposals — it never auto-applies them. Strategic control stays human.

## Summary

When a review outcome is recorded for a run (via `review guided`, `review record`, or a future API), the distiller reads the run's review artifacts, classifies the outcome's meaning using deterministic logic, then invokes an LLM to draft 3-5 structured improvement proposals. Proposals are written as `distillation-proposals.json` and `distillation-proposals.md` in the run's evidence directory. A standalone `review distill` CLI command allows re-running distillation or running it against older runs.

The distiller uses outcome-specific prompt templates to produce four proposal types: `doctrine_rule`, `validation_gap`, `planner_heuristic`, and `refinement_guidance`.

## Goals

### Primary
- Produce structured improvement proposals from completed review outcomes using LLM-assisted synthesis.
- Use outcome-specific prompt templates with a shared preamble for `accepted`, `rework_implementation_gap`, and `rework_vision_change` outcomes.
- Support four proposal types: `doctrine_rule`, `validation_gap`, `planner_heuristic`, `refinement_guidance` — with soft guidance per outcome type, not hard restrictions.
- Write `distillation-proposals.json` and `distillation-proposals.md` to the run's evidence directory.
- Run automatically after review outcome recording and via a standalone `review distill --run <run-id>` CLI command.
- Include categorical confidence (high/medium/low) with a one-sentence rationale per proposal.
- Use a configurable LLM model, defaulting to sonnet.
- Cap proposals at 3-5 per run via prompt instruction.

### Secondary
- Include rich evidence references in each proposal to support future cross-run deduplication and pattern detection.
- Keep distillation non-blocking — if it fails, the review outcome is still persisted.

## Non-goals
- Auto-applying proposals to doctrine, validation rules, or heuristics (human approval required — deferred).
- Cross-run deduplication or pattern detection across proposals from multiple runs.
- CLI commands for accepting, rejecting, or editing proposals.
- Proposal routing to per-type directories (`doctrine/proposals/`, etc.).
- Multi-project proposal scoping (local vs. global).
- Proposal `suggested_scope` field — deferred until multi-project scoping is designed.

## Architecture

### Package layout

```text
internal/next/reviewdistiller/          # NEW — distillation logic, prompt templates, proposal types
cmd/gromit-next/review_distill.go       # NEW — review distill subcommand
```

### Integration point

The distiller runs after `review-outcome.json` is written. In the `review guided` and `review record` code paths, after outcome persistence succeeds, the distiller is invoked. If distillation fails, the error is logged but the outcome remains persisted — distillation is non-blocking.

```text
review guided / review record
  → persist review-outcome.json
  → invoke distiller (non-blocking on failure)
  → write distillation-proposals.json + .md
```

### Distillation pipeline

```text
1. Load artifacts    — review-outcome.json, review packet (product-review.json,
                       process-review.json, manual-checklist.json), spec content,
                       validation.json, acceptance.json, review.json,
                       task-results.json, replan-history.json
2. Classify          — deterministic: map outcome to reasoning path
3. Assemble prompt   — shared preamble (all artifacts) + outcome-specific instructions
4. Invoke LLM        — configurable model (default sonnet), structured output
5. Parse & validate  — enforce proposal schema, cap at 5 proposals
6. Return result     — caller writes distillation-proposals.json + distillation-proposals.md
```

### Prompt design

Three outcome-specific templates share a common preamble:

**Shared preamble** — spec content, review outcome, product review summary, process trust summary, behavior card statuses (from product-review.json), manual checklist template (from review packet) and reviewer verdicts (from review outcome), validation summary, acceptance summary, machine review (review.json), task results, replan history.

**`accepted` instructions** — focus on reinforcement: which evidence patterns were sufficient, which validation strategies correlated with success, which heuristics should be promoted.

**`rework_implementation_gap` instructions** — focus on missing guardrails: classify the gap (missing contract, weak scenario test, poor decomposition, inadequate doctrine, bad heuristic), propose what would have caught the issue.

**`rework_vision_change` instructions** — focus on refinement: which assumptions were unstable, which specs need earlier clarification, which questions should be asked before execution starts.

All templates instruct the LLM to emit 3-5 proposals, each conforming to the `Proposal` schema, with soft guidance toward the proposal types most relevant to the given outcome.

### LLM interface

```go
package reviewdistiller

// LLM abstracts the model call so the distiller is testable.
type LLM interface {
    Complete(ctx context.Context, prompt string) (string, error)
}
```

The distiller accepts an `LLM` interface — production wires in the configured model, tests use a stub.

### Types

```go
package reviewdistiller
// imports omitted for brevity

type DistillationResult struct {
    RunID      string     `json:"run_id"`
    SpecID     string     `json:"spec_id"`
    Outcome    string     `json:"outcome"`
    Model      string     `json:"model"`
    Proposals  []Proposal `json:"proposals"`
    CreatedAt  time.Time  `json:"created_at"`
}

type Proposal struct {
    ID                  string   `json:"id"`
    Type                string   `json:"type"`        // doctrine_rule, validation_gap, planner_heuristic, refinement_guidance
    Title               string   `json:"title"`
    WhatHappened        string   `json:"what_happened"`
    WhatWasMissing      string   `json:"what_was_missing"`
    ProposedChange      string   `json:"proposed_change"`
    Rationale           string   `json:"rationale"`
    Confidence          string   `json:"confidence"`          // high, medium, low
    ConfidenceRationale string   `json:"confidence_rationale"`
    EvidenceReferences  []string `json:"evidence_references"`
}

// DistillerInputs bundles all artifacts the distiller needs.
// The caller loads files; the distiller never touches the filesystem.
type DistillerInputs struct {
    RunID           string
    SpecID          string
    SpecContent     string
    ReviewOutcome   json.RawMessage   // review-outcome.json
    ProductReview   json.RawMessage   // product-review.json
    ProcessReview   json.RawMessage   // process-review.json
    ManualChecklist json.RawMessage   // manual-checklist.json (generated template — reviewer verdicts are in ReviewOutcome)
    Validation      json.RawMessage   // validation.json
    Acceptance      json.RawMessage   // acceptance.json
    MachineReview   json.RawMessage   // review.json
    TaskResults     json.RawMessage   // task-results.json (if available)
    ReplanHistory   json.RawMessage   // replan-history.json (if available)
}
```

All artifact fields use `json.RawMessage` to avoid coupling the `reviewdistiller` package to upstream types like `reviewsession.ReviewOutcome` or `reviewpacket` types — the distiller treats artifacts as opaque JSON injected into prompts.

Proposal IDs are generated as `<run_id>-proposal-<index>` where index is the 1-based position in the response. IDs are deterministic per-position but may map to different proposals if the LLM produces different output on re-run.

Note: the `Proposal` fields map directly to the vision's four questions — what happened, what was missing/overly strict, what durable change would help, and how confident (split into level + rationale). The `rationale` field provides the overall justification for the proposal, while `confidence_rationale` explains the confidence level specifically.

### CLI command

**`review distill --run <run-id> [--model <model>]`**
- Loads run, verifies `review-outcome.json` exists
- Runs the distillation pipeline
- Writes `distillation-proposals.json` and `distillation-proposals.md` to evidence directory
- Overwrites existing distillation artifacts if present (idempotent re-run)
- `--model` overrides the configured default for this invocation

The project config gains a `distiller_model` field (string, default `sonnet`) that specifies which LLM model the distiller uses.

### Markdown rendering

`distillation-proposals.md` is a human-readable rendering of the JSON — one section per proposal with title, type badge, confidence, the four narrative fields (what_happened, what_was_missing, proposed_change, rationale), and evidence references. Not an independent data source.

## Acceptance Criteria

1. When a review outcome is recorded via `review guided` or `review record`, the distiller runs automatically and writes `distillation-proposals.json` and `distillation-proposals.md` to the run's evidence directory.
2. If distillation fails, the review outcome remains persisted and the error is logged — distillation is non-blocking.
3. `gromit-next review distill --run <run-id>` runs distillation as a standalone command, requiring `review-outcome.json` to exist.
4. `review distill --model <model>` overrides the configured default model for that invocation.
5. The distiller uses outcome-specific prompt templates with a shared preamble for `accepted`, `rework_implementation_gap`, and `rework_vision_change`.
6. Each proposal conforms to the `Proposal` schema: ID, type, title, what_happened, what_was_missing, proposed_change, rationale, confidence, confidence_rationale, and evidence_references.
7. Proposals are one of four types: `doctrine_rule`, `validation_gap`, `planner_heuristic`, `refinement_guidance`.
8. Confidence is categorical (`high`, `medium`, `low`) with a one-sentence rationale.
9. The distiller produces 3-5 proposals per run via prompt instruction; the parse-and-validate step silently truncates to 5 if the LLM returns more. Fewer than 3 proposals is accepted as-is — the lower bound is advisory, not enforced.
10. `distillation-proposals.md` is a human-readable rendering of the JSON, not an independent data source.
11. Re-running `review distill` overwrites existing distillation artifacts (idempotent).
12. The distiller model is configurable in project config, defaulting to sonnet.
13. The `reviewdistiller` package has no dependency on CLI, specloop, or stage machinery — it receives plain data and an `LLM` interface — verified by import analysis, not behavioral scenario.

## Scenarios

### Scenario: accepted run produces reinforcement proposals
**Given:** a run with ID `run-101` has `review-outcome.json` with `outcome: "accepted"`, a full review packet, and all validation/acceptance passing
**When:** the distiller runs (automatically after outcome recording or via `review distill --run run-101`)
**Then:** `distillation-proposals.json` is written with 3-5 proposals; proposals are predominantly `doctrine_rule` or `planner_heuristic` type; each proposal has all schema fields populated including confidence and confidence_rationale; `distillation-proposals.md` is also written

### Scenario: rework_implementation_gap produces guardrail proposals
**Given:** a run with ID `run-102` has `review-outcome.json` with `outcome: "rework_implementation_gap"`, manual results showing 1 failed item with notes "Keyboard nav broken", and the process review shows trust level "medium"
**When:** the distiller runs
**Then:** `distillation-proposals.json` contains 3-5 proposals; at least one is `validation_gap` type; proposals reference the specific failed manual check item and relevant evidence files in `evidence_references`

### Scenario: rework_vision_change produces refinement proposals
**Given:** a run with ID `run-103` has `review-outcome.json` with `outcome: "rework_vision_change"` and summary "Product direction shifted — we no longer want inline editing"
**When:** the distiller runs
**Then:** `distillation-proposals.json` contains proposals predominantly of type `refinement_guidance`; proposals reference the outcome summary and spec content in their rationale

### Scenario: distillation failure does not block outcome recording
**Given:** a run with ID `run-104` has just completed `review guided` with outcome `accepted`, but the configured LLM endpoint is unreachable
**When:** outcome recording completes and automatic distillation is attempted
**Then:** `review-outcome.json` is persisted successfully; the distillation error is logged; `distillation-proposals.json` and `distillation-proposals.md` are absent from the evidence directory

### Scenario: standalone CLI re-runs distillation with model override
**Given:** a run with ID `run-105` has `review-outcome.json` and existing `distillation-proposals.json` from a prior automatic distillation
**When:** the reviewer runs `gromit-next review distill --run run-105 --model opus`
**Then:** the distiller re-runs using opus; `distillation-proposals.json` and `distillation-proposals.md` are overwritten with new proposals; the `model` field in the JSON reflects `opus`

### Scenario: distill command refuses run without outcome
**Given:** a run with ID `run-106` has review packet artifacts but no `review-outcome.json`
**When:** the reviewer runs `gromit-next review distill --run run-106`
**Then:** the command exits with an error explaining that no review outcome has been recorded for this run

### Scenario: distiller accepts fewer than 3 proposals
**Given:** a run with ID `run-107` has `review-outcome.json` with `outcome: "accepted"` for a trivially simple spec, and the LLM returns only 2 proposals
**When:** the distiller runs
**Then:** `distillation-proposals.json` is written with 2 proposals — the lower bound is advisory and not enforced

### Scenario: distillation uses configured default model
**Given:** a project config with `distiller_model: haiku` and a run with ID `run-108` that has `review-outcome.json` with `outcome: "accepted"`
**When:** the distiller runs automatically after outcome recording (no `--model` override)
**Then:** `distillation-proposals.json` is written with the `model` field set to `haiku`

### Scenario: distiller truncates excess proposals
**Given:** a run with ID `run-109` has `review-outcome.json` with `outcome: "accepted"`, and the LLM returns 7 proposals
**When:** the distiller runs
**Then:** `distillation-proposals.json` contains exactly 5 proposals — the first 5 from the LLM response are kept

## Deferred
- Proposal acceptance/rejection CLI and lifecycle management.
- Cross-run deduplication and pattern detection across proposals.
- Auto-application pipeline for promoting accepted proposals into doctrine, validation rules, or heuristics.
- Proposal `suggested_scope` field for multi-project scoping.
- Retry or fallback on partial LLM responses.

## Validation

### Automatic
- `go test ./internal/next/reviewdistiller/...`
- `go test ./cmd/gromit-next/...`
- `go vet ./...`

### Manual
1. Run a spec through to review, record an `accepted` outcome, and verify `distillation-proposals.json` and `distillation-proposals.md` appear in the evidence directory with 3-5 well-formed proposals.
2. Record a `rework_implementation_gap` outcome and verify proposals focus on missing guardrails.
3. Run `gromit-next review distill --run <run-id> --model haiku` and verify the model override is reflected in the output.
4. Disconnect the LLM endpoint, record an outcome, and verify the outcome persists despite distillation failure.
