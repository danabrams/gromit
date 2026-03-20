# Spec 0004c — Review Outcome Distiller

## spec_id
0004c-review-outcome-distiller

## Depends on
- spec-0004b

## Vision

After 0004b, Gromit captures human review judgments as structured outcomes — but those judgments are terminal. Each run ends. The verdict is preserved, and the next run starts with no memory of what went wrong or right.

The review outcome is the only place where product intent, machine proof, and human judgment all meet. That makes it the highest-value signal in the system, and right now it's underused.

This spec adds a review outcome distiller: a subsystem that reads the completed review artifacts for a run and produces structured, human-reviewable proposals for how the system should improve. Accepted runs teach the system which evidence patterns deserve trust. Implementation-gap reworks reveal missing guardrails. Vision-change reworks expose where refinement needs earlier clarification. Every completed run becomes not just a delivery attempt, but a source of system-improving evidence.

The distiller generates proposals — it never auto-applies them. Strategic control stays human.

## Summary

When a review outcome is recorded for a run (via `review guided`, `review record`, or a future API), the distiller first ensures the review packet exists, regenerating it from on-disk evidence when needed via the same recovery path defined in 0004b. It then reads the run's review artifacts plus selected run metadata, classifies the outcome's meaning using deterministic logic, and invokes an LLM to draft 3-5 structured improvement proposals. Proposals are written as `distillation-proposals.json` and `distillation-proposals.md` in the run's evidence directory. A standalone `review distill` CLI command allows re-running distillation or running it against older runs. If the distiller encounters an outcome type other than the three defined types, it returns an error.

The distiller uses outcome-specific prompt templates to produce four proposal types: `doctrine_rule`, `validation_gap`, `planner_heuristic`, and `refinement_guidance`.

## Goals

### Primary
- Produce structured improvement proposals from completed review outcomes using LLM-assisted synthesis.
- Use outcome-specific prompt templates with a shared preamble for `accepted`, `rework_implementation_gap`, and `rework_vision_change` outcomes.
- Support four proposal types: `doctrine_rule`, `validation_gap`, `planner_heuristic`, `refinement_guidance` — with soft guidance per outcome type in the prompt, backed by minimal post-hoc validation (see Outcome-specific validation).
- Write `distillation-proposals.json` and `distillation-proposals.md` to the run's evidence directory.
- Run automatically after review outcome recording and via a standalone `review distill --run <run-id>` CLI command.
- Reuse 0004b's review packet regeneration path when packet artifacts are missing.
- Include categorical confidence (high/medium/low) with a one-sentence rationale per proposal.
- Use a configurable LLM model tier, defaulting to `medium` (provider-agnostic; resolved to a concrete model by the provider's tier-to-model mapping).
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
- Proposal `suggested_scope` field — listed in the vision doc's initial schema but deferred until multi-project scoping is designed.

## Architecture

### Package layout

```text
internal/next/reviewdistiller/          # NEW — distillation logic, prompt templates, proposal types
cmd/gromit-next/review_distill.go       # NEW — review distill subcommand
```

### Integration point

The distiller runs after `review-outcome.json` is written. In the `review guided` and `review record` code paths (introduced by 0004b), after outcome persistence succeeds, the calling code path invokes the distiller. Before loading distillation inputs, the calling code path must ensure the review packet exists. If `product-review.json`, `process-review.json`, or `manual-checklist.json` are missing, it reuses 0004b's regeneration path: call `reviewpacket.InputsFromEvidence`, then `reviewpacket.Generator.Generate`, then write the packet artifacts to the evidence directory. If packet regeneration fails, the error is logged and distillation is skipped; the outcome remains persisted.

```text
review guided / review record
  → persist review-outcome.json
  → ensure review packet exists (regenerate if missing)
  → invoke distiller (non-blocking on failure)
  → write distillation-proposals.json + .md
```

`review distill --run <run-id>` follows the same setup sequence: load run, verify `review-outcome.json` exists, ensure the review packet exists (regenerate if missing), then invoke the distiller. This keeps standalone distillation behavior aligned with the review commands from 0004b.

### Distillation pipeline

```text
1. Load artifacts    — review-outcome.json, review packet (product-review.json,
                       process-review.json, manual-checklist.json), spec content,
                       validation.json, acceptance.json, review.json,
                       task-results.json, run metadata
2. Classify          — deterministic: map outcome to reasoning path
3. Assemble prompt   — shared preamble (all artifacts) + outcome-specific instructions
4. Invoke LLM        — configurable model tier (default medium), structured output
5. Parse & validate  — enforce proposal schema, apply outcome-specific validation, cap at 5 proposals
6. Return result     — caller writes distillation-proposals.json + distillation-proposals.md
```

### Input recovery and run metadata

The distiller does not require a `replan-history.json` artifact. No such artifact is defined elsewhere, and this spec should not introduce an implicit dependency on an undefined file.

Instead, callers provide a serialized `run metadata` input derived from the existing run state. This input contains only fields that are already persisted and useful for distillation, for example:

- `status`
- `cycle`
- `blocker_summary`
- `replan_context`
- `failure_history`
- `task_lineage`

This preserves the useful repair history signal without forcing the implementation to invent a new artifact just for distillation.

### Prompt design

Three outcome-specific templates share a common preamble:

**Shared preamble** — includes these artifacts, each injected as raw JSON:
- Spec content
- Review outcome (`review-outcome.json`)
- Product review (`product-review.json`) — behavior card statuses and product review summary
- Process review (`process-review.json`) — process trust summary
- Manual checklist template (`manual-checklist.json` from review packet; reviewer verdicts are in the review outcome)
- Validation results (`validation.json`)
- Acceptance results (`acceptance.json`)
- Machine review (`review.json`)
- Task results (`task-results.json`, omitted from prompt if absent)
- Run metadata — serialized subset of run state including any available replan context and failure history

**`accepted` instructions** — focus on reinforcement: which evidence patterns were sufficient, which validation strategies correlated with success, which heuristics should be promoted.

**`rework_implementation_gap` instructions** — focus on missing guardrails: classify the gap (missing contract, weak scenario test, poor decomposition, inadequate doctrine, bad heuristic), propose what would have caught the issue.

**`rework_vision_change` instructions** — focus on refinement: which assumptions were unstable, which specs need earlier clarification, which questions should be asked before execution starts.

All templates instruct the LLM to emit 3-5 proposals, each conforming to the `Proposal` schema, with soft guidance toward the proposal types most relevant to the given outcome.

### Outcome-specific validation

Prompt guidance alone is not enough; the parse & validate step must enforce a small amount of outcome-aware structure so acceptance criteria are testable without overconstraining the model.

Validation rules:

- `accepted` — at least one proposal must have type `doctrine_rule` or `planner_heuristic`
- `rework_implementation_gap` — at least one proposal must have type `validation_gap`, `doctrine_rule`, or `planner_heuristic`
- `rework_vision_change` — at least one proposal must have type `refinement_guidance`

If the parsed output violates these rules, the distiller returns an error for that invocation rather than silently writing low-signal proposals. This error is non-blocking in the automatic path — outcome recording still succeeds.

### LLM interface

The codebase already defines `llmadapter.Invoker` (which includes `Invoke`, `InvokeInDir`, among other methods). The distiller uses a narrower adapter interface that wraps `Invoker` and returns plain text, keeping the distiller decoupled from `provider.Result`:

```go
package reviewdistiller

// LLMCompleter abstracts the model call so the distiller is testable.
// Production wraps llmadapter.Invoker, extracting the text from provider.Result.
// Tests use a stub returning canned JSON.
type LLMCompleter interface {
    Complete(ctx context.Context, prompt string) (string, error)
}
```

The distiller accepts an `LLMCompleter` interface. A thin adapter in the calling code converts `llmadapter.Invoker` to `LLMCompleter`.

### Types

```go
package reviewdistiller
// imports omitted for brevity

type DistillationResult struct {
    RunID      string     `json:"run_id"`
    SpecID     string     `json:"spec_id"`
    Outcome    string     `json:"outcome"`
    ModelTier  string     `json:"model_tier"`
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
// This is an in-memory data transfer object — it is not serialized.
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
    RunMetadata     json.RawMessage   // serialized subset of run state (status, cycle, blocker_summary, replan_context, failure_history, task_lineage)
}
```

All artifact fields use `json.RawMessage` to avoid coupling the `reviewdistiller` package to upstream types like `reviewsession.ReviewOutcome` or `reviewpacket` types — the distiller treats artifacts as opaque JSON injected into prompts.

Proposal IDs are generated from proposal content, not response position. Concatenate `type`, `title`, and `proposed_change` separated by `\x00`, compute SHA-256, take the first 8 hex characters, and format as `<run_id>-proposal-<short-hash>`. No normalization beyond trimming whitespace. This makes IDs stable when the same proposal is regenerated and naturally produces new IDs when the proposal meaning changes.

Note: the `Proposal` fields map directly to the vision's four questions — what happened, what was missing/overly strict, what durable change would help, and how confident (split into level + rationale). The `rationale` field provides the overall justification for the proposal, while `confidence_rationale` explains the confidence level specifically.

### CLI command

**`review distill --run <run-id> [--tier <tier>]`**
- Loads run, verifies `review-outcome.json` exists
- Ensures the review packet exists, regenerating it via the 0004b path if needed
- Runs the distillation pipeline
- Writes `distillation-proposals.json` and `distillation-proposals.md` to evidence directory
- Overwrites existing distillation artifacts if present; the most recent distillation result wins
- `--tier` overrides the configured default model tier for this invocation (accepts `high`, `medium`, `low`)

The `project.json` config loaded by `cmd/gromit-next/spec.go` gains an optional `distiller_tier` field (string, default `medium`) that specifies which model tier the distiller uses. The provider's tier-to-model mapping resolves this to a concrete model name. The CLI command reads this field and passes the tier to the distiller as a plain string — the `reviewdistiller` package does not import the config package.

### Markdown rendering

`distillation-proposals.md` is a human-readable rendering of the JSON — one section per proposal with title, type badge, confidence, the four narrative fields (what_happened, what_was_missing, proposed_change, rationale), and evidence references. Not an independent data source.

## Acceptance Criteria

1. When a review outcome is recorded via `review guided` or `review record`, the distiller runs automatically and writes `distillation-proposals.json` and `distillation-proposals.md` to the run's evidence directory.
2. If distillation fails, the review outcome remains persisted and the error is logged — distillation is non-blocking.
3. `gromit-next review distill --run <run-id>` runs distillation as a standalone command, requiring `review-outcome.json` to exist.
4. `review distill --tier <tier>` overrides the configured default model tier for that invocation.
5. The distiller uses outcome-specific prompt templates with a shared preamble for `accepted`, `rework_implementation_gap`, and `rework_vision_change`.
6. Each proposal conforms to the `Proposal` schema: ID, type, title, what_happened, what_was_missing, proposed_change, rationale, confidence, confidence_rationale, and evidence_references.
7. Proposals are one of four types: `doctrine_rule`, `validation_gap`, `planner_heuristic`, `refinement_guidance`.
8. Confidence is categorical (`high`, `medium`, `low`) with a one-sentence rationale.
9. If required review packet artifacts are missing, both the automatic path and `review distill` attempt regeneration using the same `InputsFromEvidence` + `Generator.Generate` path defined in 0004b. If regeneration fails, the automatic path logs the error and skips distillation; the CLI command exits with an error.
10. The distiller consumes run metadata from stored run state; it does not depend on an undefined `replan-history.json` artifact.
11. The distiller produces 3-5 proposals per run via prompt instruction; the parse & validate step silently truncates to 5 if the LLM returns more. Fewer than 3 proposals is accepted as-is — the lower bound is advisory, not enforced.
12. Parsed proposals are validated against outcome-specific minimal type requirements: `accepted` must contain at least one `doctrine_rule` or `planner_heuristic`; `rework_implementation_gap` must contain at least one `validation_gap`, `doctrine_rule`, or `planner_heuristic`; `rework_vision_change` must contain at least one `refinement_guidance`.
13. Proposal IDs are generated from proposal content (whitespace-trimmed) rather than response position — verified by unit test, not behavioral scenario.
14. `distillation-proposals.md` is a human-readable rendering of the JSON, not an independent data source.
15. Re-running `review distill` overwrites existing distillation artifacts; the latest distillation result wins.
16. The distiller model tier is configurable in `project.json` via `distiller_tier`, defaulting to `medium`.
17. The `reviewdistiller` package has no dependency on CLI, specloop, or stage machinery — it receives plain data and an `LLMCompleter` interface — verified by import analysis, not behavioral scenario.
18. `DistillationResult` metadata fields (`RunID`, `SpecID`, `Outcome`, `ModelTier`, `CreatedAt`) are populated in the output JSON.
19. The distiller returns an error for unrecognized outcome types (anything other than `accepted`, `rework_implementation_gap`, `rework_vision_change`).

## Scenarios

### Scenario: accepted run produces reinforcement proposals
**Given:** a run with ID `run-101` has `review-outcome.json` with `outcome: "accepted"`, a full review packet, and all validation/acceptance passing
**When:** the distiller runs (automatically after outcome recording or via `review distill --run run-101`)
**Then:** `distillation-proposals.json` is written with 3-5 proposals; the result includes `run_id: "run-101"`, a non-empty `spec_id`, `outcome: "accepted"`, `model_tier`, and a non-zero `created_at`; at least one proposal is `doctrine_rule` or `planner_heuristic`; each proposal has all schema fields populated including confidence and confidence_rationale; `distillation-proposals.md` is also written

### Scenario: rework_implementation_gap produces guardrail proposals
**Given:** a run with ID `run-102` has `review-outcome.json` with `outcome: "rework_implementation_gap"`, manual results showing 1 failed item with notes "Keyboard nav broken", and the process review shows trust level "medium"
**When:** the distiller runs
**Then:** `distillation-proposals.json` contains 3-5 proposals; at least one is `validation_gap`, `doctrine_rule`, or `planner_heuristic` type; proposals reference the specific failed manual check item and relevant evidence files in `evidence_references`

### Scenario: rework_vision_change produces refinement proposals
**Given:** a run with ID `run-103` has `review-outcome.json` with `outcome: "rework_vision_change"` and summary "Product direction shifted — we no longer want inline editing"
**When:** the distiller runs
**Then:** `distillation-proposals.json` contains at least one proposal of type `refinement_guidance`; proposals reference the outcome summary and spec content in their rationale

### Scenario: distillation failure does not block outcome recording
**Given:** a run with ID `run-104` has just completed `review guided` with outcome `accepted`, but the configured LLM endpoint is unreachable
**When:** outcome recording completes and automatic distillation is attempted
**Then:** `review-outcome.json` is persisted successfully; the distillation error is logged; `distillation-proposals.json` and `distillation-proposals.md` are absent from the evidence directory

### Scenario: missing review packet is regenerated before distillation
**Given:** a run with ID `run-105` has `review-outcome.json`, `validation.json`, `acceptance.json`, and `review.json`, but `product-review.json`, `process-review.json`, and `manual-checklist.json` are missing because packet generation previously failed
**When:** the reviewer runs `gromit-next review distill --run run-105`
**Then:** the command regenerates the review packet using the 0004b path, then runs distillation and writes `distillation-proposals.json` and `distillation-proposals.md`

### Scenario: standalone CLI re-runs distillation with tier override
**Given:** a run with ID `run-106` has `review-outcome.json` and existing `distillation-proposals.json` from a prior automatic distillation
**When:** the reviewer runs `gromit-next review distill --run run-106 --tier high`
**Then:** the distiller re-runs using the `high` tier; `distillation-proposals.json` and `distillation-proposals.md` are overwritten with new proposals; the `model_tier` field in the JSON reflects `high`

### Scenario: distill command refuses run without outcome
**Given:** a run with ID `run-107` has review packet artifacts but no `review-outcome.json`
**When:** the reviewer runs `gromit-next review distill --run run-107`
**Then:** the command exits with an error explaining that no review outcome has been recorded for this run

### Scenario: distiller accepts fewer than 3 proposals
**Given:** a run with ID `run-108` has `review-outcome.json` with `outcome: "accepted"` for a trivially simple spec, and the LLM returns only 2 proposals
**When:** the distiller runs
**Then:** `distillation-proposals.json` is written with 2 proposals — the lower bound is advisory and not enforced

### Scenario: distillation uses configured default tier
**Given:** a `project.json` config with `distiller_tier: low` and a run with ID `run-109` that has `review-outcome.json` with `outcome: "accepted"`
**When:** the distiller runs automatically after outcome recording (no `--tier` override)
**Then:** `distillation-proposals.json` is written with the `model_tier` field set to `low`

### Scenario: outcome-specific validation rejects non-conforming proposals
**Given:** a run with ID `run-110` has `review-outcome.json` with `outcome: "rework_vision_change"`, but the LLM returns 4 proposals, none of which is type `refinement_guidance`
**When:** the distiller runs
**Then:** the distiller returns a validation error; `distillation-proposals.json` and `distillation-proposals.md` are not written; the error message identifies the missing required proposal type

### Scenario: distiller truncates excess proposals
**Given:** a run with ID `run-111` has `review-outcome.json` with `outcome: "accepted"`, and the LLM returns 7 proposals
**When:** the distiller runs
**Then:** `distillation-proposals.json` contains exactly 5 proposals — the first 5 from the LLM response are kept

### Scenario: unrecognized outcome type returns error
**Given:** a run with ID `run-112` has `review-outcome.json` with `outcome: "rejected"`
**When:** the reviewer runs `gromit-next review distill --run run-112`
**Then:** the command exits with an error explaining that outcome type `rejected` is not supported for distillation

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
3. Run `gromit-next review distill --run <run-id> --tier low` and verify the tier override is reflected in the output.
4. Disconnect the LLM endpoint, record an outcome, and verify the outcome persists despite distillation failure.
