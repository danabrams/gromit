DONE 2026-03-23
# Spec 0004e — Cross-Project Promotion and Smart Triage

## spec_id
0004e-cross-project-promotion-and-smart-triage

## Depends on
- spec-0004d

## Vision

After 0004d, promoted proposals improve the project they came from — but each project learns in isolation. A validation gap discovered in one project is invisible to another, even when both share the same codebase patterns. And as runs accumulate, the flat proposal list grows without structure, making triage tedious.

This spec extends the learning loop in two directions. First, it adds global/local layered stores so knowledge can be shared across projects while still allowing per-project overrides. Second, it adds proposal grouping — deterministic hash matching followed by automatic LLM semantic clustering — so the reviewer sees patterns across runs instead of a flat list of individual proposals. Accepting the best proposal in a group dismisses its siblings in one action.

Finally, it feeds rejection history into the distiller so that future runs stop proposing things the human has already rejected.

## Summary

This spec adds three capabilities on top of 0004d's local promotion pipeline. (1) A global store at `.gromit-next/global/` with layered resolution: global entries provide cross-project defaults, local entries override or mask them with local-wins semantics. (2) Proposal grouping in `review proposals list`: exact content-hash matching first, then automatic LLM semantic clustering for remaining proposals, with `--dismiss-group` on accept to clear siblings. (3) Distiller rejection feedback: when distillation runs, previously rejected proposals for the same project are included in the prompt context to suppress re-proposals.

## Goals

### Primary
- Add a global store at `.gromit-next/global/` for both doctrine and playbook, parallel to project-local stores.
- Implement layered resolution that merges global and local stores with local-wins semantics at prompt assembly time.
- Add `--scope global|local` to `review proposals accept` (default `local`).
- Group similar proposals in `review proposals list` via deterministic content-hash matching, then automatic LLM semantic clustering.
- Add `--dismiss-group` to `review proposals accept` to record `dismissed` decisions for sibling proposals.
- Feed rejection history into 0004c's distiller prompts to suppress re-proposals of previously rejected guidance.

### Secondary
- LLM clustering produces human-readable group descriptions that explain why proposals were clustered.
- Grouping is transparent: each group shows its reason (`exact_match` or LLM-generated description).

## Non-goals
- Fuzzy matching or edit-distance-based deduplication (LLM clustering handles the semantic case).
- Cross-project proposal discovery (each project's `list` shows only its own proposals; global promotion copies into the global store, not across project lists).
- Automatic global promotion based on acceptance frequency.
- Proposal expiration or staleness detection.
- Web UI for proposal triage.
- Bulk operations beyond `--dismiss-group`.

## Architecture

### Global/local layered resolution

```text
.gromit-next/
  global/
    doctrine/rules.json       # global doctrine rules
    playbook/entries.json      # global playbook entries
  cells/
    <project>/
      doctrine/rules.json     # project-local doctrine rules
      playbook/entries.json    # project-local playbook entries
```

Resolution at prompt assembly time:

```go
// MergedDoctrine returns global rules + local rules, with local-wins on matching rule IDs.
func MergedDoctrine(globalDir, localDir string) ([]doctrine.Rule, error)

// MergedPlaybook returns global entries + local entries, with local-wins on matching entry IDs.
func MergedPlaybook(globalDir, localDir string) ([]playbook.Entry, error)
```

"Local-wins" means: if a local entry has the same ID as a global entry, the local entry is used. If a local entry has status `superseded`, it masks the global entry (effectively a local rejection of a global rule). This allows a project to opt out of a global rule without deleting it for other projects.

The `--scope` flag on accept determines the target directory:
- `--scope local` (default) → project cell directory (unchanged from 0004d)
- `--scope global` → `.gromit-next/global/` directory

Global entries include a `scope: "global"` field; local entries have `scope: "local"`.

### Proposal grouping

Grouping is added to the `proposaltriage` package and runs as part of `review proposals list`.

```go
type PendingProposal struct {
    Proposal    reviewdistiller.Proposal
    RunID       string
    SpecID      string
    GroupID     string    // assigned after grouping
}

type ProposalGroup struct {
    GroupID     string
    Proposals   []PendingProposal
    GroupReason string   // "exact_match" or LLM-generated description
}
```

**Grouping pipeline:**
1. Compute content hash from `(type, proposed_change)` using the same whitespace-normalization and SHA-256 algorithm as materialization IDs — group proposals with identical hashes.
2. For ungrouped proposals, invoke LLM with the list of proposal summaries (type, title, proposed_change) and ask it to cluster semantically similar ones. Each cluster becomes a group with an LLM-generated description.
3. Remaining singletons form their own single-member groups.

The LLM clustering uses the project's configured `distiller_tier` (from `project.json`). If the LLM call fails, grouping degrades gracefully: all ungrouped proposals become singletons, and a warning is logged.

### Dismiss-group

`review proposals accept <id> --dismiss-group` performs:
1. Normal accept + materialization (per 0004d)
2. Look up the accepted proposal's group
3. For each sibling in the group, record a `dismissed` decision in that sibling's run's `proposal-decisions.json`

The `dismissed` action is a new terminal state alongside `accepted` and `rejected`. Dismissed proposals don't appear in default `list` output but appear with `--all`.

```go
type Decision struct {
    // ... existing fields from 0004d ...
    Action           string    `json:"action"`           // accepted, rejected, dismissed
    DismissedBy      string    `json:"dismissed_by,omitempty"` // proposal ID that triggered dismissal
}
```

### Distiller rejection feedback

When the distiller runs (automatically after outcome recording or via `review distill`), the calling code loads `proposal-decisions.json` files from recent runs for the same project, collects rejected decisions, and includes them in the distiller's prompt as a "previously rejected proposals" section.

The prompt addition is a markdown list of rejected proposals with their type, title, proposed change, and rejection reason. This is appended to the shared preamble in the distiller's prompt (0004c's `DistillerInputs` gains an optional `RejectedProposals` field of type `json.RawMessage`).

The distiller's outcome-specific instructions are updated to include: "Do not re-propose guidance that matches previously rejected proposals unless circumstances have materially changed. If proposing something similar, explain what is different."

### Updated CLI commands

**`review proposals list [--type <type>] [--run <run-id>] [--all]`** — updated:
- After discovery, runs the grouping pipeline
- Displays proposals organized by group with group descriptions
- Shows group size and reason (`exact_match` or semantic description)

**`review proposals accept <proposal-id> [--scope local|global] [--title "..."] [--change "..."] [--rationale "..."] [--dismiss-group]`** — updated:
- `--scope global` materializes into `.gromit-next/global/` store
- `--dismiss-group` dismisses sibling proposals in the same group

### Prompt assembly update

The prompt injection points defined in 0004d now load merged (global + local) stores instead of local-only stores. The injection mechanism is unchanged; only the data source is widened.

## Acceptance Criteria

1. `review proposals accept <id> --scope global` materializes the entry into `.gromit-next/global/doctrine/` or `.gromit-next/global/playbook/`; `--scope local` (default) materializes into the project cell (same as 0004d).
2. Layered resolution merges global and local stores with local-wins semantics: matching IDs use the local version; a local `superseded` entry masks the corresponding global entry.
3. Prompt injection loads merged (global + local) stores so global entries are visible to all projects.
4. A new project with no local entries inherits all global entries automatically.
5. `review proposals list` groups proposals: proposals with identical `(type, proposed_change)` content hashes are grouped with reason `exact_match`.
6. After deterministic grouping, remaining ungrouped proposals are automatically clustered by an LLM call; each cluster has an LLM-generated group description.
7. If LLM clustering fails, ungrouped proposals appear as singletons and a warning is displayed.
8. `review proposals accept <id> --dismiss-group` records `dismissed` decisions for all sibling proposals in the same group.
9. Dismissed proposals do not appear in default `list` output but appear with `--all`.
10. When distillation runs, previously rejected proposals for the same project are included in the distiller's prompt context.
11. The distiller's instructions direct it to avoid re-proposing previously rejected guidance unless circumstances have materially changed.
12. The `dismissed` action is a terminal decision state: dismissed proposals cannot be re-decided.

## Scenarios

### Scenario: accept a proposal into global store
**Given:** a run with ID `run-301` has a `planner_heuristic` proposal with ID `run-301-proposal-e5f6a7b8` and content "Prefer package-scoped compile checks before full test suite"
**When:** the reviewer runs `gromit-next review proposals accept run-301-proposal-e5f6a7b8 --scope global`
**Then:** a new entry appears in `.gromit-next/global/playbook/entries.json` with type `planner_heuristic`, scope `global`, and the proposal's content; provenance fields trace back to run-301

### Scenario: local entry overrides global entry
**Given:** a global playbook entry with ID `pb-12345678` exists with content "Always split UI tasks by component", and the reviewer accepts a local proposal that produces the same entry ID with content "Split UI tasks by user flow, not component"
**When:** prompt assembly resolves the merged playbook for this project
**Then:** the planner prompt contains "Split UI tasks by user flow, not component" (the local version), not the global version

### Scenario: local superseded entry masks global entry
**Given:** a global doctrine rule with ID `promoted-abcd1234` is active, and the local project has a `superseded` entry with the same ID
**When:** prompt assembly resolves the merged doctrine for this project
**Then:** the global rule does not appear in the prompt — the local superseded entry masks it

### Scenario: new project inherits global entries
**Given:** 3 global playbook entries exist (2 `planner_heuristic`, 1 `validation_gap`) and a new project has no local playbook
**When:** the planner builds a prompt for the new project
**Then:** all 3 global entries appear in the prompt

### Scenario: list groups proposals by deterministic hash and LLM clustering
**Given:** 6 pending proposals across 4 runs: 2 have identical `(type, proposed_change)` content, and 2 others are semantically similar but not identical
**When:** the reviewer runs `gromit-next review proposals list`
**Then:** output shows the 2 identical proposals as one group with reason `exact_match`; the 2 semantically similar proposals as another group with an LLM-generated description; the remaining 2 as singleton groups

### Scenario: accept with --dismiss-group clears siblings
**Given:** `review proposals list` shows a group of 3 proposals (from runs 302, 303, 304) all suggesting similar doctrine rules about test isolation; the group was formed by LLM clustering with description "Test isolation doctrine proposals"
**When:** the reviewer runs `gromit-next review proposals accept run-302-proposal-11223344 --scope local --dismiss-group`
**Then:** the run-302 proposal is materialized into doctrine; the run-303 and run-304 proposals are recorded as `dismissed` in their respective `proposal-decisions.json` files; none of the three appear in future default `list` output

### Scenario: LLM clustering failure degrades gracefully
**Given:** 5 pending proposals and the LLM endpoint is unreachable
**When:** the reviewer runs `gromit-next review proposals list`
**Then:** deterministic hash grouping still works for exact matches; remaining proposals appear as singletons; a warning about clustering failure is displayed

### Scenario: distiller avoids re-proposing rejected guidance
**Given:** a `validation_gap` proposal with title "Avoid file-path contracts" was rejected with reason "We specifically need path-based contracts for our migration tooling"; this rejection is recorded in `proposal-decisions.json` for run-305
**When:** a new run for the same project reaches distillation
**Then:** the distiller prompt includes the rejected proposal and reason in its "previously rejected" section; the distiller does not produce a proposal with the same title and change text

### Scenario: list filters by type within groups
**Given:** 5 pending proposals in 2 groups: group A has 2 `doctrine_rule` + 1 `planner_heuristic`, group B has 2 `validation_gap`
**When:** the reviewer runs `gromit-next review proposals list --type validation_gap`
**Then:** only group B's 2 `validation_gap` proposals are shown

## Deferred
- Automatic global promotion based on acceptance frequency across projects.
- Cross-project proposal discovery (browsing proposals from other projects).
- Proposal expiration or staleness detection.
- Trend analysis and dashboards over proposal/promotion history.
- Web UI for proposal triage.
- Bulk accept/reject operations beyond `--dismiss-group`.

## Validation

### Automatic
- `go test ./internal/next/playbook/...`
- `go test ./internal/next/proposaltriage/...`
- `go test ./internal/next/reviewdistiller/...`
- `go test ./cmd/gromit-next/...`
- `go vet ./...`

### Manual
1. Accept a `planner_heuristic` proposal with `--scope global` and verify it appears in `.gromit-next/global/playbook/entries.json`.
2. Create a local entry with the same ID as a global entry and verify local-wins resolution in the prompt.
3. Supersede a local entry and verify it masks the global entry.
4. Run `review proposals list` with proposals across multiple runs and verify grouping output (exact-match and LLM-clustered groups).
5. Accept a proposal with `--dismiss-group` and verify siblings are marked `dismissed`.
6. Reject a proposal, then run distillation on a new run for the same project and verify the rejected proposal appears in the distiller's prompt context.
