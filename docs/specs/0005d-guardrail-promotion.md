# Spec 0005d — Guardrail Promotion

## spec_id
guardrail-promotion

## Depends on
spec-0004c, spec-0004d, spec-0005a, spec-0005b, spec-0005c

## Vision
Every issue that a human or the system catches after a run should have a path to becoming durable system pressure. Today the distiller only consumes manual review outcomes, and proposals only materialize as prompt guidance. That leaves two gaps: automated pipeline signals (adversarial findings, counterexample failures, repeated-failure patterns) never feed the learning loop, and promoted knowledge never becomes structured config that directly strengthens future runs. This spec widens both ends — more inputs into distillation, and new promotion targets that compound with the rest of the quality backpressure pipeline.

## Summary
This spec extends the 0004c distiller to consume three new automated input sources (adversarial review findings, counterexample contract failures, repeated-failure patterns) via an end-of-run batch distillation pass. It adds three new proposal types that promote into structured targets: counterexample seeds (fed into 0005a's synthesis stage), review facet heuristics (injected into review and adversarial review prompts), and sensitive area tags (fed into 0005c's risk scoring). Review heuristics extend the existing playbook store; counterexample seeds and sensitive area tags get their own dedicated stores. The existing 0004d triage CLI is extended to handle the new proposal types.

## Goals
### Primary
- Feed adversarial review findings, counterexample contract failures, and repeated-failure patterns into the distiller as new input sources
- Batch-distill automated signals at end of run to avoid redundant LLM calls
- Add three new proposal types: `counterexample_seed`, `review_heuristic`, `sensitive_area`
- Promote counterexample seeds into a dedicated store consumed by 0005a's synthesis stage
- Promote review heuristics into the playbook store, injected into review and adversarial review prompts
- Promote sensitive area tags into a dedicated store consumed by 0005c's risk assessor

### Secondary
- Preserve full provenance from automated signal → proposal → promoted guardrail
- Keep manual review outcome distillation unchanged (per-event, as defined in 0004c)

## Non-goals
- Generating actual Go test code or contract assertion YAML from proposals (deferred — requires code generation)
- Auto-applying proposals without human triage (0004d's human-in-the-loop model is preserved)
- Cross-project guardrail sharing (deferred to 0004e)
- Weighting or prioritizing proposals from different input sources differently

## Architecture

### New Input Sources

Three new input adapters feed the existing distiller pipeline:

```go
package reviewdistiller

// AutomatedSignals bundles end-of-run signals for batch distillation.
type AutomatedSignals struct {
    AdversarialFindings      json.RawMessage // Content of adversarial-review.json as defined in 0005b (array of Finding structs as defined in 0005b)
    CounterexampleFailures   json.RawMessage // Array of failed synthesized scenario contract results: [{scenario_name, contract_id, failure_message, spec_id}]
    RepeatedFailurePatterns  json.RawMessage // Array of repeated-failure escalation records from the runner: [{file_path, cycle_count, error_categories, escalation_action}]
}
```

**Trigger:** After finalization, if any automated signals are present, a single batch distillation pass runs. This is independent of the per-event manual review outcome distillation from 0004c.

**Prompt template:** A new `automated_signals` prompt template joins the existing three outcome-specific templates. It receives the shared preamble plus the automated signals and is instructed to produce proposals across all seven types (the four existing types plus the three new ones).

**Validation rules for batch output:** Automated distillation may produce any of the seven proposal types. However, `doctrine_rule` proposals are unlikely to arise from automated signals and should be treated as exceptional — the distiller prompt instructs the LLM to prefer the three new types (`counterexample_seed`, `review_heuristic`, `sensitive_area`) and the contextually relevant existing types (`validation_gap`, `planner_heuristic`, `refinement_guidance`) over `doctrine_rule`, which is better suited to manual review outcomes. No hard filter is applied; all seven types are accepted from batch output.

### New Proposal Types

| Type | Promotion target | Store |
|------|-----------------|-------|
| `counterexample_seed` | Seed scenario for 0005a synthesis | `cellPath/counterexample-seeds.json` |
| `review_heuristic` | Prompt guidance for review/adversarial facets | `cellPath/playbook/entries.json` (existing) |
| `sensitive_area` | Risk signal for 0005c assessor | `cellPath/sensitive-areas.json` |

### Counterexample Seed Store

```go
package counterexampleseeds

type Seed struct {
    ID               string    `json:"id"`
    ScenarioName     string    `json:"scenario_name"`
    Category         string    `json:"category"`        // boundary, negation, error_path, state_edge, persistence
    Description      string    `json:"description"`     // natural language scenario description
    Status           string    `json:"status"`           // active, superseded
    SourceProposalID string    `json:"source_proposal_id"`
    SourceRunID      string    `json:"source_run_id"`
    SourceSpecID     string    `json:"source_spec_id"`
    CreatedAt        time.Time `json:"created_at"`
    SupersededBy     string    `json:"superseded_by,omitempty"`
}

type Store struct{}

func (s *Store) Load(dir string) ([]Seed, error)
func (s *Store) Save(dir string, seeds []Seed) error
```

Seed IDs computed from `(scenario_name, description)` via SHA-256, first 8 hex characters, prefixed with `cs-`.

**Integration with 0005a:** The SynthesizeCounterexamples stage loads active seeds from the store and includes them as additional input to the LLM alongside the spec's authored scenarios. Seeds act as "must consider" hints — the synthesis LLM is instructed to generate counterexamples that cover the seed's described scenario if the spec's domain is relevant.

**Note:** 0005a does not currently define a seed input mechanism for its SynthesizeCounterexamples stage. This spec extends 0005a's stage interface to accept seed inputs as additional LLM context. The stage's `Run` method (or equivalent) must be updated to accept an optional `[]Seed` parameter that is appended to the synthesis prompt.

### Sensitive Area Store

```go
package sensitiveareas

type Tag struct {
    ID               string    `json:"id"`
    Pattern          string    `json:"pattern"`          // file glob or package path
    Reason           string    `json:"reason"`
    Status           string    `json:"status"`           // active, superseded
    SourceProposalID string    `json:"source_proposal_id"`
    SourceRunID      string    `json:"source_run_id"`
    SourceSpecID     string    `json:"source_spec_id"`
    CreatedAt        time.Time `json:"created_at"`
    SupersededBy     string    `json:"superseded_by,omitempty"`
}

type Store struct{}

func (s *Store) Load(dir string) ([]Tag, error)
func (s *Store) Save(dir string, tags []Tag) error
```

Tag IDs computed from `(pattern, reason)` via SHA-256, first 8 hex characters, prefixed with `sa-`.

**Integration with 0005c:** The RiskAssessor loads active sensitive area tags from the store. When any file in the diff matches a tag's pattern, the tag's reason is added to `RiskSignals.SensitiveAreas`, contributing to risk level computation.

### Review Heuristic in Playbook

`review_heuristic` is a new entry type in the existing playbook store. No new store needed.

**Schema change:** This extends 0004d's playbook `Entry` type enum to include `review_heuristic` as a valid entry type. The playbook store's validation must accept this new type alongside the existing `validation_gap`, `planner_heuristic`, and `refinement_guidance` types.

**Integration with 0005b and existing review:** At prompt assembly time, active `review_heuristic` entries are injected into both normal review facet prompts and adversarial review facet prompts as additional guidance to watch for.

### Batch Distillation Trigger

```
Finalize
  → write evidence artifacts
  → check for automated signals (adversarial-review.json, counterexample failures, repeated-failure data)
  → if any present: invoke batch distillation
  → write automated-distillation-proposals.json + .md to evidence directory
```

Automated distillation proposals are written to `automated-distillation-proposals.json` (separate from manual review `distillation-proposals.json`) to keep the two sources distinct.

### Extended Triage CLI

The existing `review proposals` commands from 0004d are extended:

- `review proposals list` discovers proposals from both `distillation-proposals.json` and `automated-distillation-proposals.json`. Source is determined by which file the proposal was loaded from: proposals from `distillation-proposals.json` are labeled "manual", proposals from `automated-distillation-proposals.json` are labeled "automated".
- `review proposals accept <id>` handles all seven proposal types, routing to the correct store
- `review proposals accept <id> --to-corpus` routes the accepted proposal to the regression corpus (0005e) instead of its default store. This creates a forward dependency on 0005e.
- `review proposals reject` handles all seven proposal types, including reject-after-accept superseding

### Promotion Pipeline Extension

When a proposal is accepted, the target store is determined by type:

| Proposal type | Target store | ID prefix |
|--------------|-------------|-----------|
| `doctrine_rule` | doctrine store | `promoted-` |
| `validation_gap` | playbook | `pb-` |
| `planner_heuristic` | playbook | `pb-` |
| `refinement_guidance` | playbook | `pb-` |
| `counterexample_seed` | counterexample seed store | `cs-` |
| `review_heuristic` | playbook | `pb-` |
| `sensitive_area` | sensitive area store | `sa-` |

### Configuration

```yaml
guardrail_promotion:
  batch_distillation: true    # default: true — run batch distillation at end of run
  model_tier: medium          # default: medium — model tier for batch distillation
```

## Acceptance Criteria

1. After finalization, if adversarial review findings, counterexample contract failures, or repeated-failure patterns are present in evidence, a batch distillation pass runs and writes `automated-distillation-proposals.json` and `automated-distillation-proposals.md`
2. Batch distillation is a single LLM invocation regardless of how many automated signals are present
3. Manual review outcome distillation (0004c) is unchanged — still per-event, still writes to `distillation-proposals.json`
4. The distiller can produce proposals of types `counterexample_seed`, `review_heuristic`, and `sensitive_area` in addition to the four existing types
5. `review proposals list` discovers proposals from both `distillation-proposals.json` and `automated-distillation-proposals.json`
6. Accepting a `counterexample_seed` proposal materializes it into `cellPath/counterexample-seeds.json` with scenario name, category, description, and provenance
7. Accepting a `review_heuristic` proposal materializes it into the playbook store with type `review_heuristic`
8. Accepting a `sensitive_area` proposal materializes it into `cellPath/sensitive-areas.json` with file glob pattern, reason, and provenance
9. The SynthesizeCounterexamples stage (0005a) loads active counterexample seeds and includes them as input to the synthesis LLM
10. The RiskAssessor (0005c) loads active sensitive area tags and matches them against diff file paths
11. Active `review_heuristic` entries are injected into both normal review and adversarial review facet prompts
12. Reject-after-accept supersedes the materialized entry in the correct store (counterexample seed, playbook, or sensitive area)
13. Batch distillation is configurable on/off and uses a configurable model tier (default medium)
14. If batch distillation fails, finalization still completes and evidence is preserved
15. All existing tests continue to pass

## Scenarios

### Scenario: Adversarial finding produces counterexample seed proposal

**Given:** A run where the adversarial `red_team` facet found an error-severity finding about unhandled empty input in a parser
**When:** Batch distillation runs at end of run
**Then:** `automated-distillation-proposals.json` contains a `counterexample_seed` proposal with a scenario description covering empty input to the parser, category `boundary`, and evidence references pointing to the adversarial finding

### Scenario: Repeated failure produces sensitive area proposal

**Given:** A run where repeated-failure escalation fired for `internal/next/specloop/worktree_guard.go` across 3 cycles
**When:** Batch distillation runs at end of run
**Then:** `automated-distillation-proposals.json` contains a `sensitive_area` proposal with pattern `internal/next/specloop/worktree_guard*` and a reason referencing the repeated failure pattern

### Scenario: Counterexample failure produces review heuristic proposal

**Given:** A run where 2 synthesized counterexamples failed contract verification, both related to nil map handling
**When:** Batch distillation runs at end of run
**Then:** `automated-distillation-proposals.json` contains a `review_heuristic` proposal about nil map initialization patterns, with evidence references to both failed counterexamples

### Scenario: Promoted counterexample seed influences future synthesis

**Given:** An active counterexample seed with scenario name "empty input to parser" and category "boundary" exists in `cellPath/counterexample-seeds.json`
**When:** The SynthesizeCounterexamples stage runs for a spec that includes a parser scenario
**Then:** The synthesis prompt includes the seed as a "must consider" hint, instructing the LLM to generate counterexamples covering empty parser input if relevant to the spec's domain

### Scenario: Promoted sensitive area tag raises risk level

**Given:** An active sensitive area tag with pattern `internal/auth/**` exists in `cellPath/sensitive-areas.json`
**When:** A run produces a diff touching `internal/auth/token.go`
**Then:** The RiskAssessor includes the tag's reason in `RiskSignals.SensitiveAreas`, contributing to a higher risk level

### Scenario: Promoted review heuristic appears in adversarial review

**Given:** An active `review_heuristic` playbook entry with content "Watch for nil map writes in hot paths" exists
**When:** The AdversarialReview stage runs for a spec in this project
**Then:** All enabled adversarial facet prompts include the heuristic as additional guidance

### Scenario: Batch distillation skipped when no automated signals

**Given:** A run that completed with no adversarial review (low risk, skipped per 0005c), no counterexample failures, and no repeated-failure escalation
**When:** Finalization completes
**Then:** No batch distillation runs, `automated-distillation-proposals.json` is not written

### Scenario: Triage CLI shows proposals from both sources

**Given:** A run with `distillation-proposals.json` (from manual review, 3 proposals) and `automated-distillation-proposals.json` (from batch distillation, 4 proposals)
**When:** The reviewer runs `gromit-next review proposals list`
**Then:** All 7 pending proposals are listed, with source indicated (manual vs automated)

### Scenario: Reject-after-accept supersedes a counterexample seed

**Given:** A `counterexample_seed` proposal was previously accepted and materialized into `cellPath/counterexample-seeds.json` with status `active`
**When:** The reviewer runs `gromit-next review proposals reject <id>`
**Then:** The seed entry in `cellPath/counterexample-seeds.json` is updated to status `superseded`, and the proposal status is updated to `rejected`

### Scenario: Reject-after-accept supersedes a sensitive area tag

**Given:** A `sensitive_area` proposal was previously accepted and materialized into `cellPath/sensitive-areas.json` with status `active`
**When:** The reviewer runs `gromit-next review proposals reject <id>`
**Then:** The tag entry in `cellPath/sensitive-areas.json` is updated to status `superseded`, and the proposal status is updated to `rejected`

### Scenario: Reject-after-accept supersedes review heuristic in playbook

**Given:** A previously accepted `review_heuristic` proposal materialized as a playbook entry with ID `pb-abc123`
**When:** The reviewer runs `review proposals reject <proposal-id> --reason "too noisy, caused false positives"`
**Then:** The playbook entry `pb-abc123` is superseded in the playbook store, and the rejection reason is recorded in the proposal's disposition

### Scenario: Batch distillation disabled via configuration

**Given:** A project with `guardrail_promotion.batch_distillation: false` in config, and adversarial findings present in evidence
**When:** Finalization completes
**Then:** No batch distillation runs, `automated-distillation-proposals.json` is not written, and no LLM invocation occurs for distillation

### Scenario: Batch distillation uses custom model tier

**Given:** A project with `guardrail_promotion.model_tier: high` in config
**When:** Batch distillation runs at end of run
**Then:** The distillation LLM invocation uses the high model tier instead of the default medium tier

### Scenario: Batch distillation failure does not block finalization

**Given:** A run with adversarial findings present, but the LLM endpoint is unreachable
**When:** Finalization attempts batch distillation
**Then:** The distillation error is logged, `automated-distillation-proposals.json` is not written, but all other evidence artifacts are preserved and the run finalizes normally

## Validation

```
go test ./internal/next/reviewdistiller/...
go test ./internal/next/counterexampleseeds/...
go test ./internal/next/sensitiveareas/...
go test ./internal/next/playbook/...
go test ./internal/next/proposaltriage/...
go test ./internal/next/specloop/stages/...
go test ./internal/next/risk/...
go test ./cmd/gromit-next/...
go vet ./...
```
