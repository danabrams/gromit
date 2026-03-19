# Spec 0004b — Review Outcome Recording and CLI

## spec_id
review-outcome-recording-and-cli

## Depends on
- spec-0004a

## Vision

Spec 0004a gives the reviewer a behavior-focused review packet, but no way to interact with it or record a decision. The reviewer must manually open JSON files and has no structured path from "I've read this" to "here's my verdict."

This spec closes the loop. It adds a review session protocol that walks the reviewer through the packet, collects manual verification results, enforces outcome validation rules, and persists a structured decision. The CLI is the first consumer; the session protocol is designed so an API can drive it with the same logic. If the review packet is missing due to a generation failure, the commands recover automatically by regenerating it from existing evidence.

## Summary

This spec adds a review session protocol and CLI commands that let a human reviewer interact with the review packet from 0004a, walk through manual verification steps, and record a structured review outcome. The session protocol is a state machine that the CLI drives via stdin and that a future API can drive with the same logic. Three CLI commands are added: `review show` to display the packet, `review guided` for step-by-step interactive review, and `review record` for non-interactive outcome recording. If the review packet is missing, commands attempt to regenerate it from existing evidence on disk.

## Goals

### Primary
- Provide a `reviewsession` package that manages review state as a step-by-step protocol, independent of I/O.
- Add `review show`, `review guided`, and `review record` CLI commands.
- Record one of three review outcomes: `accepted`, `rework_implementation_gap`, `rework_vision_change`.
- Enforce outcome validation rules: `accepted` requires no failed items (unsure items require an override note); `rework_implementation_gap` requires at least one failed/unsure item or a non-empty summary; `rework_vision_change` requires a summary note.
- Persist the review decision as `review-outcome.json` in the run's evidence directory.
- Regenerate the review packet from on-disk evidence if it's missing when a command is invoked.

### Secondary
- Keep `review show` simple (render existing markdown) with a path to richer terminal formatting later.
- Make the common "all looks good" acceptance path completable in a few minutes.

## Non-goals
- Rich terminal formatting (colors, tables, status indicators) — deferred to a follow-up.
- API handler that drives the review session — the protocol is ready for it, but the HTTP/gRPC layer is out of scope.
- Multi-reviewer approval workflows.
- Automatic promotion of review outcomes into doctrine, project memory, or execution heuristics.
- Cross-run dashboards or portfolio views.

## Architecture

### Package layout

```text
internal/next/reviewpacket/         # from 0004a — gains InputsFromEvidence loader
internal/next/reviewsession/        # NEW — review state machine and outcome validation
cmd/gromit-next/review_packet.go    # NEW — review show, guided, record subcommands
```

### Review session protocol

`internal/next/reviewsession/` manages the review as a state machine. It has no knowledge of stdin, CLI, or HTTP.

```go
package reviewsession

type Session struct {
    Packet          reviewpacket.Outputs
    Checklist       []ChecklistItemState
    CurrentStep     int
    Outcome         *ReviewOutcome
}

type ChecklistItemState struct {
    Item   reviewpacket.ManualCheckItem
    Result string  // pending, pass, fail, unsure, skipped
    Notes  string
}

// Start initializes a session from a review packet.
func Start(packet reviewpacket.Outputs) *Session

// CurrentItem returns the current checklist item, or nil if all are done.
func (s *Session) CurrentItem() *ChecklistItemState

// RecordItemResult records the result for the current item and advances.
func (s *Session) RecordItemResult(result string, notes string) error

// SkipRemaining marks all remaining items as skipped.
func (s *Session) SkipRemaining()

// CanAccept returns true if acceptance is valid given current checklist state.
// Returns false and a reason string if not.
func (s *Session) CanAccept() (bool, string)

// NeedsOverride returns true if accepting requires an override note
// (e.g., some items are unsure).
func (s *Session) NeedsOverride() bool

// RecordOutcome validates and records the final outcome.
func (s *Session) RecordOutcome(outcome string, summary string, overrideReason string) (*ReviewOutcome, error)
```

### Outcome validation rules

Enforced inside `RecordOutcome`:

- `accepted` — no checklist items with `result: "fail"`. If any items are `unsure`, an override reason is required. `pass`, `skipped`, and `pending` (for items the reviewer chose not to check) are allowed.
- `rework_implementation_gap` — at least one checklist item must be `fail` or `unsure`, OR the reviewer must provide a non-empty summary explaining the gap.
- `rework_vision_change` — summary must be non-empty.

### ReviewOutcome type

```go
type ReviewOutcome struct {
    RunID          string              `json:"run_id"`
    ReviewedAt     time.Time           `json:"reviewed_at"`
    Outcome        string              `json:"outcome"`
    Summary        string              `json:"summary"`
    ManualResults  []ManualCheckResult `json:"manual_results,omitempty"`
    OverrideReason string              `json:"override_reason,omitempty"`
}

type ManualCheckResult struct {
    ID     string `json:"id"`
    Result string `json:"result"`
    Notes  string `json:"notes,omitempty"`
}
```

### InputsFromEvidence loader

Added to `internal/next/reviewpacket/`:

```go
// InputsFromEvidence reconstructs generator inputs by reading
// existing artifacts from the evidence directory, spec file, and run state.
func InputsFromEvidence(evidenceDir string, specPath string, run *runstore.RunState) (Inputs, error)
```

Reads `review.json`, `acceptance.json`, and `validation.json` from `evidenceDir`, spec content from `specPath`, and run metadata (run ID, terminal state, cycle/failure history) from the `run` parameter. Returns an error if required evidence files are missing (not a silent fallback — the reviewer needs to know).

### CLI commands

All three commands share a common preamble: load the run, check for review packet, regenerate if missing.

**`review show --run <run-id>`**
- Print `product-review.md` and a one-line trust banner from `process-review.json`
- With `--details`: also print linked technical artifacts

**`review guided --run <run-id>`**
- Display the product review packet
- Create a `Session`, walk each checklist item via stdin prompts
- After checklist: prompt for outcome (`accepted` / `rework_implementation_gap` / `rework_vision_change`)
- Prompt for summary and, if needed, override reason
- Validate and persist `review-outcome.json`; if validation fails, re-prompt for outcome

**`review record --run <run-id> --outcome <outcome> [--summary "..."] [--override "..."]`**
- Non-interactive: validate the outcome against the checklist state (all items default to `skipped` if not walked interactively)
- Persist `review-outcome.json`

### Outcome idempotency

If `review-outcome.json` already exists when a command writes a new outcome, the existing file is overwritten. The most recent review decision wins. A future spec may add outcome history tracking.

### Packet regeneration

When any review command finds the packet artifacts missing:
1. Call `InputsFromEvidence` to reconstruct inputs
2. Call `Generator.Generate` to produce the packet
3. Write the artifacts to the evidence directory
4. Proceed normally

If regeneration fails, the command exits with an error explaining what's missing.

## Acceptance Criteria

1. `gromit-next review show --run <run-id>` prints the product review markdown and a one-line trust banner to stdout.
2. `gromit-next review show --run <run-id> --details` additionally prints linked technical artifact content.
3. `gromit-next review guided --run <run-id>` displays the product review packet, walks each manual checklist item with stdin prompts, prompts for an outcome, and writes `review-outcome.json` to the evidence directory.
4. `gromit-next review record --run <run-id> --outcome accepted --summary "..."` writes `review-outcome.json` without interactive prompts.
5. The `reviewsession` package manages review state as a step-by-step protocol with no dependency on I/O or CLI packages.
6. `accepted` outcome is rejected if any checklist item has `result: "fail"`; if any item is `unsure`, an override reason is required.
7. `rework_implementation_gap` outcome is rejected unless at least one checklist item is `fail` or `unsure`, or the summary is non-empty.
8. `rework_vision_change` outcome is rejected if the summary is empty.
9. `review-outcome.json` contains run ID, timestamp, outcome, summary, manual results, and any override reason.
10. When review packet artifacts are missing, all three commands attempt to regenerate them from on-disk evidence via `InputsFromEvidence` before proceeding.
11. When regeneration fails (required evidence files missing), the command exits with a clear error message listing what's missing.
12. All three commands refuse to proceed if the run has no terminal state.
13. `review record` treats unwalked checklist items as `skipped`.

## Scenarios

### Scenario: guided review accepts a clean run
**Given:** a run with ID `run-001` reached `ready_for_review` and all 5 review packet artifacts exist with 2 manual checklist items
**When:** the reviewer runs `gromit-next review guided --run run-001` and marks both items as `pass`, then selects `accepted` with summary "Looks good"
**Then:** `review-outcome.json` is written with `outcome: "accepted"`, 2 manual results both `pass`, and no override reason

### Scenario: guided review rejects acceptance with failed item
**Given:** a run with ID `run-002` reached `ready_for_review` with 3 manual checklist items
**When:** the reviewer runs `gromit-next review guided --run run-002`, marks item 1 as `pass`, item 2 as `fail` with notes "Button does nothing", item 3 as `pass`, and selects `accepted`
**Then:** the session rejects the outcome and explains that acceptance is not allowed with failed items; the reviewer is re-prompted for an outcome

### Scenario: acceptance with unsure item requires override
**Given:** a run with ID `run-003` reached `ready_for_review` with 2 manual checklist items
**When:** the reviewer runs `gromit-next review guided --run run-003`, marks item 1 as `pass`, item 2 as `unsure`, selects `accepted`, and provides override reason "Verified manually outside checklist"
**Then:** `review-outcome.json` is written with `outcome: "accepted"`, `override_reason: "Verified manually outside checklist"`

### Scenario: rework_implementation_gap requires a flagged item or summary
**Given:** a run with ID `run-004` reached `ready_for_review` with 2 manual checklist items
**When:** the reviewer runs `gromit-next review guided --run run-004`, marks both items as `pass`, and selects `rework_implementation_gap` with an empty summary
**Then:** the session rejects the outcome and explains that at least one failed/unsure item or a non-empty summary is required

### Scenario: rework_vision_change requires a summary
**Given:** a run with ID `run-004b` reached `ready_for_review` with 2 manual checklist items
**When:** the reviewer runs `gromit-next review guided --run run-004b`, marks both items as `pass`, and selects `rework_vision_change` with an empty summary
**Then:** the session rejects the outcome and explains that a non-empty summary is required for vision change rework

### Scenario: non-interactive record with outcome
**Given:** a run with ID `run-005` reached `ready_for_review` with review packet artifacts present
**When:** the reviewer runs `gromit-next review record --run run-005 --outcome accepted --summary "Reviewed offline"`
**Then:** `review-outcome.json` is written with `outcome: "accepted"`, all checklist items as `skipped`, and summary "Reviewed offline"

### Scenario: missing packet triggers regeneration
**Given:** a run with ID `run-006` reached `ready_for_review` with evidence artifacts (`review.json`, `acceptance.json`, `validation.json`) present but review packet artifacts missing
**When:** the reviewer runs `gromit-next review show --run run-006`
**Then:** the command regenerates the review packet from on-disk evidence, writes all 5 artifacts, and displays the product review markdown

### Scenario: regeneration fails with clear error
**Given:** a run with ID `run-007` reached `ready_for_review` but `acceptance.json` is missing from the evidence directory and review packet artifacts are also missing
**When:** the reviewer runs `gromit-next review show --run run-007`
**Then:** the command exits with an error message that lists `acceptance.json` as missing and does not write partial artifacts

### Scenario: command refuses non-terminal run
**Given:** a run with ID `run-008` is still in state `running`
**When:** the reviewer runs `gromit-next review show --run run-008`
**Then:** the command exits with an error explaining that the run has not reached a terminal state

## Validation

### Automatic
- `go test ./internal/next/reviewsession/...`
- `go test ./internal/next/reviewpacket/...`
- `go test ./cmd/gromit-next/...`
- `go vet ./...`

### Manual
1. Run `gromit-next review show --run <run-id>` on a completed run and verify the product review and trust banner are printed.
2. Run `gromit-next review show --run <run-id> --details` and verify technical artifacts are included.
3. Run `gromit-next review guided --run <run-id>`, walk through checklist items, accept the run, and verify `review-outcome.json` is written correctly.
4. Attempt to accept a run with a failed checklist item and verify the session rejects it.
5. Run `gromit-next review record --run <run-id> --outcome rework_vision_change --summary "Changed direction"` and verify the outcome is persisted.
6. Delete the review packet artifacts from a run's evidence directory and run `review show` — verify regeneration succeeds.

## Deferred
- Rich terminal formatting for review commands
- API handler for review session protocol
- Multi-reviewer approval workflows
- Automatic promotion of review outcomes into doctrine or execution heuristics
- Cross-run dashboards and portfolio views
