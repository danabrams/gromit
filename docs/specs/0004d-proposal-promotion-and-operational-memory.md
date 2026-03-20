# Spec 0004d — Proposal Promotion and Operational Memory

## spec_id
0004d-proposal-promotion-and-operational-memory

## Depends on
- spec-0004c

## Vision

Spec 0004c gives Gromit something strategically valuable: structured proposals about how the system should improve. But those proposals are inert. They live in run evidence, a human can read them, and then the system goes back to work unchanged.

That leaves the highest-value learning signal in the project stranded at the edge of the loop.

If accepted review outcomes and implementation-gap reworks are going to compound, Gromit needs a human-gated path from proposal to durable project memory. The machine should draft improvements, a human should approve or edit them, and accepted improvements should become visible in future prompts automatically. Without that last step, review distillation is analysis without leverage.

## Summary

This spec adds a proposal promotion workflow for distillation outputs. New `review proposals` CLI commands let a human list, inspect, accept, edit, or reject proposals produced by spec 0004c. Accepting a proposal writes a decision record to the source run and materializes the approved guidance into project memory:

- `doctrine_rule` proposals become doctrine rules
- `validation_gap`, `planner_heuristic`, and `refinement_guidance` proposals become entries in a new `playbook` artifact

The context compiler is extended to inject `playbook` alongside `doctrine` into future project/spec/task packets, so accepted proposals affect future execution behavior. Materialization is deterministic and duplicate-safe: identical approved guidance maps to the same target ID, so repeated proposals reinforce the same memory instead of bloating prompts.

## Goals

### Primary
- Provide a human-gated workflow for accepting, editing, or rejecting distillation proposals
- Materialize accepted proposals into prompt-visible project memory that future runs automatically receive
- Deduplicate repeated accepted proposals across runs using deterministic IDs
- Persist proposal decisions with provenance back to the source run

### Secondary
- Support project-wide proposal review without introducing a separate database or background indexer
- Keep doctrine normative and playbook guidance advisory, with explicit precedence rules
- Make repeated accept/reject commands idempotent when the requested decision matches the existing one

## Non-goals

- Automatically accepting or applying proposals without human action
- Semantic or LLM-based deduplication across differently worded proposals
- Global or cross-project proposal scope
- Retraction, editing, or removal of already-accepted project memory entries
- Automatic repo-wide backfill of missing `distillation-proposals.json` artifacts
- Writing to the target repository instead of the project cell / run evidence
- Replacing or changing the proposal-generation behavior from spec 0004c

## Architecture

### Package layout

```text
internal/next/reviewpromotion/      # proposal discovery, decision validation, materialization
internal/next/playbook/             # approved non-doctrine operational memory store
cmd/gromit-next/review_proposals.go # review proposals list/show/apply/record subcommands
```

### Proposal discovery

V1 does not add a persistent project-level inbox or index. The source of truth remains the run evidence written by spec 0004c.

`review proposals` commands discover proposals by scanning runs in the store for:

- `distillation-proposals.json`
- optional `proposal-decisions.json`

This keeps the design aligned with the existing run-centric evidence model and avoids introducing a second store just to surface pending work.

```go
package reviewpromotion

type DiscoveredProposal struct {
    ProposalID   string
    RunID        string
    SpecID       string
    Outcome      string
    ProposalType string
    Title        string
    Confidence   string
    Status       string // pending, accepted, rejected
    CreatedAt    time.Time
}

func Discover(store *runstore.Store, project string) ([]DiscoveredProposal, error)
func LoadProposal(store *runstore.Store, project string, proposalID string) (*LoadedProposal, error)
```

Discovery behavior:

1. `store.List(project)` provides candidate runs
2. Runs without `distillation-proposals.json` are skipped silently
3. If `proposal-decisions.json` exists, it is joined by `proposal_id`
4. Proposals with no decision are `pending`
5. Results are sorted by proposal creation time descending

This means older runs only appear if they have already been distilled. Explicit backfill remains the job of `review distill`.

### Decision artifact

Each source run gets a new evidence artifact: `proposal-decisions.json`.

```go
type ProposalDecisionFile struct {
    Decisions []ProposalDecision `json:"decisions"`
}

type ProposalDecision struct {
    ProposalID          string    `json:"proposal_id"`
    RunID               string    `json:"run_id"`
    Decision            string    `json:"decision"` // accepted, rejected
    DecidedAt           time.Time `json:"decided_at"`
    Reason              string    `json:"reason,omitempty"`               // required for rejected
    ApprovedTitle       string    `json:"approved_title,omitempty"`
    ApprovedChange      string    `json:"approved_change,omitempty"`
    ApprovedRationale   string    `json:"approved_rationale,omitempty"`
    Scope               string    `json:"scope,omitempty"`                // doctrine only; defaults to "*"
    MaterializedEntryID string    `json:"materialized_entry_id,omitempty"`
    DuplicateOf         string    `json:"duplicate_of,omitempty"`
}
```

Decision rules:

- `accepted` and `rejected` are terminal
- Recording the same terminal decision twice with the same effective payload is idempotent
- Recording a different decision for an already-decided proposal returns an error
- `rejected` requires a non-empty `reason`
- `accepted` defaults `approved_*` fields from the original proposal when omitted

This keeps materialization simple and avoids the need to retract already-promoted memory in V1.

### Deterministic materialization IDs

Accepted proposals are materialized by hashing the approved content, not the source run.

Fingerprint algorithm:

1. Trim leading/trailing whitespace from `approved_change`
2. Collapse internal whitespace to single spaces (`strings.Fields`)
3. Concatenate `proposal_type + "\x00" + normalized_approved_change`
4. Compute SHA-256
5. Take the first 8 hex characters

Materialized IDs:

- doctrine rules: `promoted-<short-hash>`
- playbook entries: `pb-<short-hash>`

This is intentionally deterministic and narrow. If two accepted proposals approve the same change text for the same type, they resolve to the same target entry. Differently worded proposals are treated as distinct unless a human edits them to the same approved wording.

### Materialization targets

#### Doctrine

Accepted `doctrine_rule` proposals are converted into `doctrine.Rule` values and persisted via the existing doctrine store.

```go
// existing type with new usage contract, not a new struct
type Rule struct {
    ID        string    `json:"id"`
    Summary   string    `json:"summary"`
    Scope     string    `json:"scope"`
    Source    string    `json:"source"`
    CreatedAt time.Time `json:"created_at"`
}
```

Materialization mapping:

- `ID` = deterministic `promoted-<short-hash>`
- `Summary` = `approved_change`
- `Scope` = accepted scope, defaulting to `*`
- `Source` = `promoted:<proposal-id>`
- `CreatedAt` = decision timestamp

When the rule ID already exists, no duplicate rule is added. The decision is still recorded, with `materialized_entry_id` set to the existing rule ID and `duplicate_of` pointing to that same ID.

After doctrine materialization, the project cell's doctrine artifact is refreshed so context compilation sees the updated rule set immediately.

#### Playbook

Accepted non-doctrine proposals are materialized into a new `playbook` store.

```go
package playbook

type Entry struct {
    ID                 string    `json:"id"`
    Type               string    `json:"type"` // validation_gap, planner_heuristic, refinement_guidance
    Title              string    `json:"title"`
    Guidance           string    `json:"guidance"`
    Rationale          string    `json:"rationale"`
    EvidenceReferences []string  `json:"evidence_references,omitempty"`
    SourceProposalID   string    `json:"source_proposal_id"`
    SourceRunID        string    `json:"source_run_id"`
    CreatedAt          time.Time `json:"created_at"`
}

type Playbook struct {
    Entries []Entry `json:"entries"`
}

type Store interface {
    Save(playbookDir string, pb Playbook) error
    Load(playbookDir string) (Playbook, error)
}
```

Materialization mapping:

- `ID` = deterministic `pb-<short-hash>`
- `Type` = proposal type
- `Title` = `approved_title`
- `Guidance` = `approved_change`
- `Rationale` = `approved_rationale`
- `EvidenceReferences` = proposal evidence references
- `SourceProposalID` = proposal ID
- `SourceRunID` = source run ID
- `CreatedAt` = decision timestamp

The playbook store lives in the project cell and is mirrored into an artifact (`playbook.json`) so the context compiler can read it without importing the concrete package.

When the playbook entry ID already exists, no duplicate entry is added. The decision is still recorded with `duplicate_of` set to the existing entry ID.

### Knowledge precedence

Doctrine remains the highest-authority instruction surface. Playbook entries are approved operational guidance, not declared law.

Precedence rules:

- Doctrine overrides playbook on conflict
- Playbook is advisory and should be rendered as "preferred guidance", not "must"
- Proposal promotion never mutates or removes declared doctrine authored outside this workflow

This preserves the design rule from the existing system: human-declared doctrine stays authoritative.

### Context compilation

The context compiler gains a `playbookSection()` reader parallel to `doctrineSection()`. The new section is included when the `playbook` artifact exists and contains entries.

Updated packet composition:

```text
Project: architecture + doctrine + playbook + glossary + validation
Spec:    architecture + doctrine + playbook + spec-text
Task:    doctrine + playbook + spec-text + proof-requirements
```

Rendering rules:

- Omit the section entirely when playbook is empty
- Group entries by `Type`
- Sort deterministically by `Type`, then `ID`
- Render guidance text, not raw JSON
- Keep the section small and compatible with the existing token-budget trimming in `contextpkt`

This is the key operational change in the spec: accepted proposals stop being archival evidence and become future prompt context.

### CLI commands

#### `review proposals list [--all]`

- Scans proposals for the current project
- Prints proposal ID, type, status, source run, outcome, confidence, and title
- Defaults to `pending` proposals only
- `--all` includes accepted and rejected proposals

#### `review proposals show --proposal <proposal-id>`

- Loads and prints the full proposal
- Includes existing decision data if present

#### `review proposals apply --proposal <proposal-id>`

Interactive path:

1. Show the proposal
2. Prompt for `accept` or `reject`
3. If `accept`, allow edits to title/change/rationale
4. If `accept` and type is `doctrine_rule`, prompt for scope (default `*`)
5. Persist `proposal-decisions.json`
6. Materialize doctrine/playbook entry if needed

#### `review proposals record --proposal <proposal-id> --decision accepted|rejected`

Non-interactive path:

- `accepted` accepts optional `--title`, `--change`, `--rationale`, `--scope`
- `rejected` requires `--reason`
- Uses the same validation and materialization path as `apply`

### Missing artifacts and backfill behavior

Proposal promotion commands do not automatically re-run distillation across all runs.

Behavior:

- `list` skips runs with no `distillation-proposals.json`
- `show` / `apply` / `record` return a clear error when the proposal cannot be found
- If the user targets a run with review outcome but no distillation artifacts, the error should suggest `review distill --run <run-id>`

This keeps promotion narrowly scoped to proposal handling rather than turning it into a second distillation orchestration path.

## Acceptance Criteria

1. `gromit-next review proposals list` discovers distilled proposals by scanning run evidence and shows pending proposals by default.
2. `gromit-next review proposals list --all` includes accepted and rejected proposals with their recorded status.
3. `gromit-next review proposals show --proposal <proposal-id>` prints the full proposal and any existing decision.
4. `gromit-next review proposals apply --proposal <proposal-id>` supports an interactive accept/reject flow and writes `proposal-decisions.json` to the source run's evidence directory.
5. `gromit-next review proposals record --proposal <proposal-id> --decision accepted|rejected ...` supports non-interactive decision recording using the same validation/materialization path as `apply`.
6. Accepted `doctrine_rule` proposals are materialized into the doctrine store using deterministic rule IDs and the doctrine artifact is refreshed.
7. Accepted `validation_gap`, `planner_heuristic`, and `refinement_guidance` proposals are materialized into the playbook store and mirrored artifact using deterministic entry IDs.
8. Accepting a proposal whose deterministic target ID already exists does not create a duplicate doctrine rule or playbook entry; the decision is still recorded and points to the existing entry ID.
9. Rejected proposals require a non-empty reason; accepted proposals may override title/change/rationale and default omitted fields from the original proposal.
10. Proposal decisions are terminal: repeating the same decision is idempotent, but attempting to change an existing accepted/rejected decision returns an error.
11. Future project/spec/task context packets include the playbook section when playbook entries exist, and omit it when empty.
12. Doctrine remains authoritative over playbook guidance; this precedence is documented in the rendering and materialization rules.
13. Promotion writes only to run evidence and the project cell (doctrine/playbook artifacts), never to the target repository.

## Scenarios

### Scenario: list shows pending proposals across multiple runs

**Given:** Run A and Run B both contain `distillation-proposals.json`; Run A has no `proposal-decisions.json`, while Run B has one accepted proposal decision
**When:** the reviewer runs `gromit-next review proposals list`
**Then:** only Run A's undecided proposals are shown
**And:** `gromit-next review proposals list --all` shows both Run A's pending proposals and Run B's accepted proposal with its status

### Scenario: accepting a doctrine proposal materializes a rule

**Given:** proposal `run-101-proposal-abcd1234` has type `doctrine_rule`, approved change text `Interactive UI specs must include keyboard-navigation scenarios`, and no existing decision
**When:** the reviewer runs `gromit-next review proposals record --proposal run-101-proposal-abcd1234 --decision accepted --scope ui`
**Then:** `proposal-decisions.json` records an accepted decision for that proposal
**And:** the doctrine store contains a rule with ID `promoted-<hash>`, summary equal to the approved change text, scope `ui`, and source `promoted:run-101-proposal-abcd1234`
**And:** the doctrine artifact is refreshed so future context packets include the new rule

### Scenario: accepting a planner heuristic proposal materializes playbook guidance

**Given:** proposal `run-102-proposal-efgh5678` has type `planner_heuristic`, title `Split proof-writing from implementation`, and proposed change text `When scenario tests and implementation both need to be written, prefer separate tasks so validation can localize failures`
**When:** the reviewer accepts the proposal
**Then:** the playbook store contains an entry with type `planner_heuristic`, the approved title, the approved guidance text, and evidence references from the proposal
**And:** a subsequent task-level context packet includes a `playbook` section containing that guidance

### Scenario: accepting a duplicate proposal does not create duplicate memory

**Given:** proposal `run-103-proposal-1111aaaa` is accepted and materialized into playbook entry `pb-9f81c3a2`
**And:** a later run produces proposal `run-104-proposal-2222bbbb` whose accepted type and approved change text hash to the same `pb-9f81c3a2`
**When:** the reviewer accepts the later proposal
**Then:** no second playbook entry is created
**And:** the later run's `proposal-decisions.json` records `materialized_entry_id: "pb-9f81c3a2"` and `duplicate_of: "pb-9f81c3a2"`

### Scenario: rejecting a proposal requires a reason

**Given:** proposal `run-105-proposal-cccc3333` is pending
**When:** the reviewer runs `gromit-next review proposals record --proposal run-105-proposal-cccc3333 --decision rejected --reason "Too project-specific to codify"`
**Then:** `proposal-decisions.json` records a rejected decision with that reason
**And:** neither doctrine nor playbook is modified

### Scenario: repeated record with same decision is idempotent

**Given:** proposal `run-106-proposal-dddd4444` already has an accepted decision with the same approved fields and target entry ID
**When:** the reviewer repeats the same `review proposals record` command
**Then:** the command succeeds without changing doctrine or playbook
**And:** no duplicate decision or duplicate materialized entry is created

### Scenario: attempting to change a terminal decision is rejected

**Given:** proposal `run-107-proposal-eeee5555` already has a rejected decision
**When:** the reviewer attempts to accept it with `review proposals record`
**Then:** the command returns an error explaining that the proposal already has a terminal decision
**And:** no doctrine or playbook state changes

## Validation

### Automatic
- `go test ./internal/next/reviewpromotion/...`
- `go test ./internal/next/playbook/...`
- `go test ./internal/next/contextpkt/...`
- `go test ./cmd/gromit-next/...`
- `go vet ./...`

### Manual
1. Record a review outcome, run distillation, then run `gromit-next review proposals list` and verify the new proposal appears as pending.
2. Accept a `doctrine_rule` proposal and verify the doctrine artifact changes and future context packets include the promoted rule.
3. Accept a `planner_heuristic` proposal and verify the playbook artifact changes and future task packets include the new guidance.
4. Accept an equivalent proposal from a second run and verify no duplicate doctrine/playbook entry is created.
5. Reject a proposal and verify only `proposal-decisions.json` changes.

## Deferred

- Global / cross-project proposal scope
- Proposal retirement, retraction, or editing after acceptance
- Automatic repo-wide backfill for runs that have review outcomes but no distillation artifacts
- Semantic deduplication across similar but differently worded proposals
- Ordering or weighting playbook entries by reinforcement count
- Rendering playbook content in `project guide` or other human-facing summary surfaces
- Impact tracking that measures whether accepted proposals improve future review outcomes
