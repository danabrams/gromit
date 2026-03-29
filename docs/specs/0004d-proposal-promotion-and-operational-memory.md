DONE 2026-03-29
# Spec 0004d — Proposal Promotion and Operational Memory

## spec_id
0004d-proposal-promotion-and-operational-memory

## Depends on
- spec-0004c

## Vision

After 0004c, Gromit can synthesize human review judgments into structured improvement proposals — but the proposals are write-only. They accumulate in evidence directories, read once and forgotten. No proposal ever changes how the next run behaves.

This spec closes the learning loop. It adds a proposal triage CLI, a promotion pipeline that materializes accepted proposals into durable operational stores, and prompt injection wiring that feeds those stores back into the planner, validator, and refinement stages. A proposal accepted today makes tomorrow's run smarter.

Two stores hold operational knowledge: doctrine (normative rules, already established in 0004a-c) and a new playbook (advisory heuristics, validation gaps, and refinement guidance). Doctrine remains authoritative; playbook entries are advisory guidance that can be overridden or superseded.

The human remains in full control. Proposals are never auto-applied. Every promotion requires explicit acceptance, and every acceptance is reversible via rejection that supersedes the materialized entry.

## Summary

This spec adds a `review proposals` CLI command group that lets a human reviewer triage distillation proposals from 0004c, promote accepted proposals into project-local operational stores, and wire those stores into future run prompts. `doctrine_rule` proposals materialize into the existing doctrine store. `validation_gap`, `planner_heuristic`, and `refinement_guidance` proposals materialize into a new playbook store. The reviewer can override proposal fields (title, change text, rationale) at accept time for minor wording fixes. Rejecting a previously accepted proposal supersedes the materialized entry. Promoted knowledge is injected into type-specific prompt injection points: planner heuristics into the planner, validation gaps into the validator, and refinement guidance into the pre-execution refinement stage.

## Goals

### Primary
- Add `review proposals list`, `show`, `accept`, and `reject` CLI commands for triaging distillation proposals.
- Materialize accepted `doctrine_rule` proposals into the existing doctrine store (`cellPath/doctrine/rules.json`).
- Materialize accepted `validation_gap`, `planner_heuristic`, and `refinement_guidance` proposals into a new playbook store (`cellPath/playbook/entries.json`).
- Inject promoted doctrine rules into task executor prompts (extending existing wiring).
- Inject promoted planner heuristics into planner prompts.
- Inject promoted validation gaps into validator prompts.
- Inject promoted refinement guidance into pre-execution refinement prompts.
- Support field overrides on accept (`--title`, `--change`, `--rationale`) for minor wording adjustments without reject-and-redistill.
- Support reject-after-accept to supersede materialized entries when promoted knowledge turns out to be wrong.
- Record rejections with reasons for future use.

### Secondary
- Preserve full provenance: every materialized rule/entry traces back to its source proposal, run, and spec.
- Deterministic content-hash IDs so duplicate proposals across runs resolve to the same materialized entry.

## Non-goals
- Auto-applying proposals without human approval.
- Global or cross-project proposal scope (deferred to Spec 0004e).
- Global/local layered store resolution (deferred to Spec 0004e).
- Proposal grouping and LLM-based semantic clustering (deferred to Spec 0004e).
- `--dismiss-group` for clearing sibling proposals (deferred to Spec 0004e).
- Feeding rejection history back into the distiller's prompts (deferred to Spec 0004e).
- In-place `$EDITOR` flow for editing proposals.
- Web UI for proposal triage.
- Cross-run trend analysis or dashboards over proposal history.
- Proposal `suggested_scope` field in the distiller output.

## Architecture

### Package layout

```text
internal/next/playbook/              # NEW — playbook store (entries.json), load/save/merge
internal/next/proposaltriage/        # NEW — proposal discovery, decision validation, promotion logic
cmd/gromit-next/review_proposals.go  # NEW — review proposals list/show/accept/reject
```

### Playbook store

The playbook is the non-doctrine operational memory store. It holds three entry types: `validation_gap`, `planner_heuristic`, and `refinement_guidance`. It follows the same patterns as the doctrine store and enrichment fact store.

```go
package playbook

type Entry struct {
    ID               string    `json:"id"`
    Type             string    `json:"type"`              // validation_gap, planner_heuristic, refinement_guidance
    Title            string    `json:"title"`
    Content          string    `json:"content"`           // the actionable guidance text
    Rationale        string    `json:"rationale"`
    Status           string    `json:"status"`            // active, superseded
    SourceProposalID string    `json:"source_proposal_id"`
    SourceRunID      string    `json:"source_run_id"`
    SourceSpecID     string    `json:"source_spec_id"`
    CreatedAt        time.Time `json:"created_at"`
    SupersededBy     string    `json:"superseded_by,omitempty"`
}

type Store struct{}

func (s *Store) Load(dir string) ([]Entry, error)
func (s *Store) Save(dir string, entries []Entry) error
```

Entry IDs are computed from `(type, content)` via SHA-256, first 8 hex characters, prefixed with `pb-`. This makes IDs stable for deduplication. The `content` field is whitespace-normalized (trimmed, internal whitespace collapsed to single spaces) before hashing.

### Proposal discovery

`proposaltriage` scans run evidence directories for `distillation-proposals.json` files, filters out proposals that have already been decided (by checking for `proposal-decisions.json` per run), and builds the list of pending proposals.

```go
package proposaltriage

type PendingProposal struct {
    Proposal    reviewdistiller.Proposal
    RunID       string
    SpecID      string
}

type Decision struct {
    ProposalID       string    `json:"proposal_id"`
    Action           string    `json:"action"`           // accepted, rejected
    Reason           string    `json:"reason,omitempty"` // required for rejected
    ApprovedTitle    string    `json:"approved_title,omitempty"`
    ApprovedChange   string    `json:"approved_change,omitempty"`
    ApprovedRationale string   `json:"approved_rationale,omitempty"`
    MaterializedID   string    `json:"materialized_id,omitempty"`
    DuplicateOf      string    `json:"duplicate_of,omitempty"`
    DecidedAt        time.Time `json:"decided_at"`
}
```

Discovery behavior:
1. List runs for the current project via the run store
2. Skip runs without `distillation-proposals.json`
3. Join with `proposal-decisions.json` by proposal ID when present
4. Proposals with no decision are `pending`
5. Results sorted by creation time descending

### Promotion pipeline

When a proposal is accepted:

1. Apply field overrides: use `--title`, `--change`, `--rationale` values if provided, otherwise default from the original proposal
2. Determine target store: `doctrine_rule` → doctrine store, others → playbook store
3. Compute deterministic materialized ID from `(type, approved_change)` — `promoted-<hash>` for doctrine, `pb-<hash>` for playbook
4. Check for duplicates: if an entry with that ID already exists and is `active`, record the decision with `duplicate_of` set and skip materialization
5. Materialize: create a `doctrine.Rule` or `playbook.Entry` from the approved fields
6. Save to project cell store
7. Record a `Decision` in `proposal-decisions.json` in the source run's evidence directory

### Reject-after-accept

When `reject` targets a proposal that was previously accepted:

1. Look up the `materialized_id` from the existing accepted decision
2. Find the corresponding entry in the doctrine or playbook store
3. Set its status to `superseded`, with `superseded_by` pointing to the rejection decision
4. Overwrite the decision in `proposal-decisions.json` with the new `rejected` action and reason

This makes promotion fully reversible without needing a separate retraction command.

### Decision persistence

Each run's evidence directory gains an optional `proposal-decisions.json` — an array of `Decision` objects. New decisions are added; existing decisions for the same proposal ID are overwritten (enabling reject-after-accept).

### Prompt injection points

At prompt assembly time, the project's doctrine and playbook stores are loaded and injected at type-specific points:

| Entry type | Injection point | Mechanism |
|---|---|---|
| `doctrine_rule` | Task executor packet | Extend existing `Doctrine` field in `TaskPacketInput` (already wired) |
| `planner_heuristic` | Planner prompt | Add `PlaybookHeuristics` field to `buildPlanPrompt()` and `buildFixPlanPrompt()` |
| `validation_gap` | Validator prompt | Add `KnownGaps` field to validation prompt assembly |
| `refinement_guidance` | Refinement/pre-execution prompt | Add `RefinementGuidance` field to spec refinement prompt |

Each injection renders entries as a markdown list within the prompt section — title, content, and rationale per entry. Only `active` entries are included; `superseded` entries are excluded.

### CLI commands

**`review proposals list [--type <type>] [--run <run-id>] [--all]`**
- Discovers pending proposals for the current project
- Shows proposal ID, type, source run, confidence, and title
- Defaults to pending proposals only; `--all` includes accepted and rejected
- Filters by type or source run

**`review proposals show <proposal-id>`**
- Displays full proposal detail: all fields, source run context, existing decision if any

**`review proposals accept <proposal-id> [--title "..."] [--change "..."] [--rationale "..."]`**
- Materializes into project-local target store
- Records decision with approved fields (overridden or defaulted from proposal)
- Reports materialized entry ID and target store

**`review proposals reject <proposal-id> --reason "..."`**
- Records rejection decision with reason
- If the proposal was previously accepted, supersedes the materialized entry in the store

## Acceptance Criteria

1. `gromit-next review proposals list` discovers pending proposals from `distillation-proposals.json` files across runs for the current project.
2. `review proposals list --all` includes accepted and rejected proposals with their recorded status.
3. `review proposals list` supports `--type` and `--run` filters.
4. `review proposals show <proposal-id>` displays the full proposal including all fields, source run context, and existing decision if any.
5. `review proposals accept <proposal-id>` materializes the proposal into the appropriate project-local store: `doctrine_rule` → doctrine store, others → playbook store.
6. `review proposals accept` supports `--title`, `--change`, and `--rationale` overrides; omitted fields default from the original proposal.
7. `review proposals reject <proposal-id> --reason "..."` records a rejection decision with the given reason.
8. Rejecting a previously accepted proposal supersedes the materialized entry in the target store (sets status to `superseded`).
9. Decisions are persisted in `proposal-decisions.json` in the source run's evidence directory; re-deciding overwrites the previous decision for the same proposal ID.
10. The playbook store (`cellPath/playbook/entries.json`) supports `validation_gap`, `planner_heuristic`, and `refinement_guidance` entry types with the `Entry` schema defined in Architecture.
11. Accepting a proposal whose deterministic materialized ID already exists as an `active` entry does not create a duplicate; the decision is recorded with `duplicate_of` set to the existing entry ID.
12. Promoted `doctrine_rule` entries are injected into task executor prompts via the existing `Doctrine` field.
13. Promoted `planner_heuristic` entries are injected into planner prompts (`buildPlanPrompt` and `buildFixPlanPrompt`).
14. Promoted `validation_gap` entries are injected into validator prompts.
15. Promoted `refinement_guidance` entries are injected into refinement/pre-execution prompts.
16. Only `active` entries are injected into prompts; `superseded` entries are excluded.
17. Playbook entry IDs are computed from `(type, whitespace-normalized content)` via SHA-256, first 8 hex characters, prefixed with `pb-`.
18. Doctrine rule IDs from promotion are computed the same way, prefixed with `promoted-`.
19. Every materialized rule/entry includes provenance fields: `source_proposal_id`, `source_run_id`, `source_spec_id`.
20. The `proposaltriage` package has no dependency on CLI or specloop machinery — it receives paths and returns data.
21. The `playbook` package has no dependency on CLI, specloop, or proposaltriage — it is a pure store.

## Scenarios

### Scenario: accept a doctrine_rule proposal into project-local store
**Given:** a run with ID `run-201` has `distillation-proposals.json` containing 4 proposals, one of which is a `doctrine_rule` with ID `run-201-proposal-a1b2c3d4` and title "Interactive UI specs must include accessibility scenario checks"
**When:** the reviewer runs `gromit-next review proposals accept run-201-proposal-a1b2c3d4`
**Then:** a new rule appears in the project cell's `doctrine/rules.json` with ID `promoted-<hash>`, the proposal's title as summary, and source `promoted:run-201-proposal-a1b2c3d4`; `proposal-decisions.json` in run-201's evidence directory contains an `accepted` decision; the proposal no longer appears in `review proposals list`

### Scenario: accept a planner_heuristic proposal into playbook
**Given:** a run with ID `run-202` has a `planner_heuristic` proposal with ID `run-202-proposal-e5f6a7b8` and content "Prefer package-scoped compile checks before full test suite"
**When:** the reviewer runs `gromit-next review proposals accept run-202-proposal-e5f6a7b8`
**Then:** a new entry appears in `cellPath/playbook/entries.json` with type `planner_heuristic`, status `active`, and the proposal's content; provenance fields trace back to run-202

### Scenario: accept with field overrides
**Given:** a run with ID `run-203` has a `validation_gap` proposal with ID `run-203-proposal-f1f2f3f4`, title "Contract assertions target wrong file", and proposed change "Avoid file-path-specific contract assertions when behavior can be verified by scenario tests"
**When:** the reviewer runs `gromit-next review proposals accept run-203-proposal-f1f2f3f4 --title "Prefer scenario tests over file-path contracts" --change "When a behavior can be verified by a scenario test, prefer that over file-path-specific contract assertions which break on refactoring"`
**Then:** the materialized playbook entry uses the overridden title and content, not the original proposal text; the decision records the approved overrides

### Scenario: reject a proposal with reason
**Given:** a run with ID `run-204` has a `validation_gap` proposal with ID `run-204-proposal-c9d0e1f2`
**When:** the reviewer runs `gromit-next review proposals reject run-204-proposal-c9d0e1f2 --reason "Too specific to this one-off migration"`
**Then:** `proposal-decisions.json` in run-204's evidence directory contains a `rejected` decision with the given reason; the proposal no longer appears in `review proposals list`; neither doctrine nor playbook is modified

### Scenario: reject a previously accepted proposal supersedes the materialized entry
**Given:** proposal `run-205-proposal-aabbccdd` was previously accepted as a local `planner_heuristic`, and an active entry with the corresponding ID exists in `cellPath/playbook/entries.json`
**When:** the reviewer runs `gromit-next review proposals reject run-205-proposal-aabbccdd --reason "Turned out to cause worse task splits"`
**Then:** the entry in `playbook/entries.json` has status `superseded`; `proposal-decisions.json` is updated with a `rejected` decision replacing the previous `accepted` decision

### Scenario: accepting a duplicate proposal does not create duplicate memory
**Given:** proposal `run-206-proposal-1111aaaa` was previously accepted and materialized into playbook entry `pb-9f81c3a2`; a later run produces proposal `run-207-proposal-2222bbbb` whose type and approved change text hash to the same `pb-9f81c3a2`
**When:** the reviewer accepts the later proposal
**Then:** no second playbook entry is created; the decision records `materialized_id: "pb-9f81c3a2"` and `duplicate_of: "pb-9f81c3a2"`

### Scenario: promoted heuristic appears in planner prompt
**Given:** a `planner_heuristic` entry with title "Prefer compile checks before full test suite" is active in the project's playbook
**When:** the planner builds a plan for a new spec in this project
**Then:** the planner prompt includes the heuristic title and content in its guidance section

### Scenario: promoted validation gap appears in validator prompt
**Given:** a `validation_gap` entry with title "File-path-specific contract assertions are fragile" is active in the project's playbook
**When:** the validator runs for a spec in this project
**Then:** the validator prompt includes the gap title and content as a known issue to watch for

### Scenario: promoted refinement guidance appears in refinement prompt
**Given:** a `refinement_guidance` entry with title "Ask about deployment constraints before designing infrastructure tasks" is active in the project's playbook
**When:** a spec enters the refinement stage in this project
**Then:** the refinement prompt includes the guidance title and content

### Scenario: superseded entries excluded from prompts
**Given:** a `planner_heuristic` entry exists in the playbook with status `superseded`
**When:** the planner builds a prompt for this project
**Then:** the superseded entry does not appear in the planner prompt

### Scenario: list with --all shows decided proposals
**Given:** 4 proposals exist across runs: 2 pending, 1 accepted, 1 rejected
**When:** the reviewer runs `gromit-next review proposals list --all`
**Then:** all 4 proposals are shown with their respective statuses

## Deferred
- Global or cross-project proposal scope and `--scope global|local` (Spec 0004e).
- Global/local layered store resolution with local-wins semantics (Spec 0004e).
- Proposal grouping via deterministic content-hash matching and LLM semantic clustering (Spec 0004e).
- `--dismiss-group` for clearing sibling proposals on accept (Spec 0004e).
- Feeding rejection history into the distiller's prompts to suppress re-proposals (Spec 0004e).
- In-place `$EDITOR` flow for editing proposals before acceptance.
- Proposal expiration or staleness detection.
- Trend analysis and dashboards over proposal/promotion history.
- Web UI for proposal triage.
- Bulk accept/reject operations.

## Validation

### Automatic
- `go test ./internal/next/playbook/...`
- `go test ./internal/next/proposaltriage/...`
- `go test ./cmd/gromit-next/...`
- `go vet ./...`

### Manual
1. Run a spec through to review, record an outcome, distill it, then run `review proposals list` and verify proposals appear.
2. Accept a `doctrine_rule` proposal and verify it appears in `cellPath/doctrine/rules.json` with correct provenance.
3. Accept a `planner_heuristic` proposal with `--change` override and verify the playbook entry uses the overridden text.
4. Run a new spec and verify the promoted heuristic appears in the planner prompt output.
5. Reject a previously accepted proposal and verify the materialized entry is superseded.
6. Accept a duplicate proposal (same type and change text as an existing entry) and verify no duplicate entry is created.
