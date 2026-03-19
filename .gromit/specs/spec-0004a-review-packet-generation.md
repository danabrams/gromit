# Spec 0004a — Review Packet Generation

## spec_id
review-packet-generation

## Depends on
- spec-0002f
- spec-0003c

## Vision

Today, a `ready_for_review` run produces technically rich artifacts — `review.json`, `acceptance.json`, `validation.json`, `review.md`, task results, and the worktree itself. These are useful for debugging the system, but they are not the right interface for product judgment.

The human reviewer wants to know two things: does the software behave the way the spec intended, and was the process that produced it trustworthy? Both are strategic questions that should be answerable from a concise, behavior-focused summary — not by spelunking through machine artifacts.

This spec adds automatic review packet generation to the finalize step. When a run reaches a terminal state, Gromit assembles a product review packet (behavior cards derived from scenarios), a process trust summary (deterministic trust level from run evidence), and a manual verification checklist — all written to the run's evidence directory. These artifacts give the reviewer a behavior-first surface for making product decisions, with technical details available but hidden by default.

## Summary

When a run reaches a terminal state, the finalize stage assembles a review packet into the run's evidence directory. For `ready_for_review` runs, the packet contains a product review with behavior cards derived from the spec's scenarios, a process trust summary with a deterministic trust level, and a manual verification checklist. For `blocked` or `needs_human` runs, the packet is a diagnostic variant that leads with blockers and omits the acceptance flow. Existing machine artifacts remain intact and unchanged.

## Goals

### Primary
- Generate a behavior-focused product review packet automatically from existing run evidence.
- Derive behavior cards from the spec's `## Scenarios` section using deterministic markdown parsing; fall back to acceptance criteria when scenarios are absent.
- Produce a deterministic process trust summary (high/medium/low) from validation, review, acceptance, and degraded-mode signals.
- Generate a manual verification checklist from the spec's `Validation > Manual` section or derived from scenarios.
- Handle both `ready_for_review` and diagnostic (`blocked`, `needs_human`) terminal states with appropriate packet variants.
- Keep review packet generation non-blocking — if it fails, run status is unchanged.

### Secondary
- Keep all new artifacts machine-readable (JSON + rendered Markdown) so later specs can build CLI and aggregation on top.
- Support degraded runs (e.g., missing diff) with a reviewable but appropriately flagged packet.

## Non-goals
- CLI commands for viewing or interacting with review packets (deferred to Spec 0004b).
- Review outcome recording and `review-outcome.json` (deferred to Spec 0004b).
- Guided interactive review flow (deferred to Spec 0004b).
- Per-scenario evidence traceability — behavior cards use coarse evidence links and overall status.
- Replacing or redefining the technical Review or Accept stages from Spec 0002b.
- Cross-run dashboards, SPC rollups, or trend analysis.
- Web UI for review workflows.

## Architecture

### Integration point

Review packet generation runs inside the existing finalize stage, after terminal status is determined but before the run state is persisted. It reads from artifacts already written by the evidence stage (review.json, acceptance.json, validation.json, task results) and from the spec content string available in the pipeline context.

If packet generation fails, the error is logged but the run's terminal status is not affected. The run proceeds as if no packet was requested.

```text
... → Evidence → Finalize [ determine status → generate review packet → persist run state ]
```

### New package

`internal/next/reviewpacket/` — responsible for building all five artifacts from inputs. No dependencies on the specloop or stage machinery; it receives plain data and returns plain data.

```go
package reviewpacket

// Generator builds review packet artifacts from run evidence.
type Generator struct{}

// Inputs holds everything needed to build a review packet.
type Inputs struct {
    RunID             string
    SpecTitle         string
    SpecContent       string                    // raw spec markdown
    TerminalState     string                    // ready_for_review, needs_human, blocked
    ValidationResult  *validator.FinalResult
    ReviewFindings    map[string][]review.Finding
    AcceptanceResult  *acceptor.AcceptanceResult
    DegradedFlags     []string
    RepairCycles      int
    RepeatedFailure   bool
}

// Outputs holds the generated artifacts ready for writing.
type Outputs struct {
    ProductReview    ProductReview
    ProcessReview    ProcessReview
    ManualChecklist  ManualChecklist
}

func (g *Generator) Generate(inputs Inputs) (Outputs, error)
```

### Scenario parsing

`internal/next/reviewpacket/scenarioparser.go` — deterministic markdown parser that extracts `### Scenario:` blocks with their Given/When/Then lines from the spec content string.

```go
type ParsedScenario struct {
    Title string
    Given string
    When  string
    Then  string
    Notes string
}

func ParseScenarios(specContent string) []ParsedScenario
```

If no scenarios are found, the generator falls back to acceptance criteria as behavior units.

### Behavior card status

Each card gets an `automatic_status` derived from overall run evidence, not per-scenario correlation:

- `proven` — validation passed, all acceptance criteria passed, no blocking review findings, and no degraded flags
- `mixed` — validation passed but some acceptance criteria unclear, non-blocking review findings exist, or degraded flags are present
- `failed` — acceptance criteria failed or blocking review findings exist
- `unclear` — insufficient evidence (missing artifacts) or run is `blocked`/`needs_human`

For `ready_for_review` runs, all cards share the same status since we lack per-scenario traceability. Evidence links point to whole artifact files.

### Surprises

The `surprises` field in `ProductReview` captures unexpected conditions detected during packet generation — for example, acceptance criteria that passed despite degraded evidence, or a mismatch between the number of scenarios in the spec and the number of task results. Surprises are informational and do not affect trust level or behavior card status. The field is omitted when empty.

### Trust level rules

Deterministic, no LLM:

- **high** — validation passed AND all acceptance passed AND no unresolved blocking findings AND no degraded flags AND no repeated-failure escalation
- **medium** — validation passed AND no blockers remain AND (degraded flags OR non-blocking concerns exist)
- **low** — run is `needs_human`/`blocked` OR validation incomplete OR unresolved blocking issues OR repeated-failure escalation fired in final cycle

### Recommended posture

Derived from trust level:
- high → `quick_accept_path`
- medium → `manual_check_carefully`
- low → `do_not_accept_without_changes`

### Product review summary

The `summary` field in `ProductReview` is a deterministic, template-generated string (no LLM). For `ready_for_review` runs it describes the overall outcome, e.g. "6 behaviors verified, all proven. 5/5 acceptance criteria passed." For diagnostic runs it summarizes the blocker situation, e.g. "Run blocked: validation failed after 3 repair cycles."

### Diagnostic packet variant

For `blocked` or `needs_human` runs, the generator produces the same artifact set but with different content rules:
- Product review leads with blocker summary instead of behavior cards
- Behavior cards are still present but marked with appropriate status
- Manual checklist is empty (items array is `[]`)
- Process trust is always `low`
- A `recommended_next_action` field is populated

### Artifact writing

The finalize stage calls the generator, then writes three JSON files (`product-review.json`, `process-review.json`, `manual-checklist.json`) and two rendered markdown files (`product-review.md`, `process-review.md`) via the existing evidence bundler pattern — five artifacts total. The markdown files are human-readable renderings of the JSON — not separate data sources.

### Types

```go
type ProductReview struct {
    RunID                 string         `json:"run_id"`
    SpecTitle             string         `json:"spec_title"`
    TerminalState         string         `json:"terminal_state"`
    Summary               string         `json:"summary"`
    BehaviorCards         []BehaviorCard `json:"behavior_cards"`
    Surprises             []string       `json:"surprises,omitempty"`
    IsDiagnostic          bool           `json:"is_diagnostic"`
    BlockerSummary        string         `json:"blocker_summary,omitempty"`
    RecommendedNextAction string         `json:"recommended_next_action,omitempty"`
}

type BehaviorCard struct {
    ID              string   `json:"id"`
    Title           string   `json:"title"`
    Given           string   `json:"given,omitempty"`
    When            string   `json:"when,omitempty"`
    Then            string   `json:"then,omitempty"`
    AutomaticStatus string   `json:"automatic_status"`
    EvidenceFiles   []string `json:"evidence_files,omitempty"`
    ManualCheckIDs  []string `json:"manual_check_ids,omitempty"`
    Notes           string   `json:"notes,omitempty"`
}

type ProcessReview struct {
    TrustLevel          string   `json:"trust_level"`
    AutomaticProof      string   `json:"automatic_proof"`      // summary of validation outcome, e.g. "all 12 checks passed"
    MachineReview       string   `json:"machine_review"`       // summary of review findings, e.g. "3 findings (0 blocking)"
    Acceptance          string   `json:"acceptance"`            // summary of acceptance outcome, e.g. "5/5 criteria passed"
    DegradedFlags       []string `json:"degraded_flags,omitempty"`
    RepairCycles        int      `json:"repair_cycles"`
    RepeatedFailureFlag bool     `json:"repeated_failure_flag"`
    RecommendedPosture  string   `json:"recommended_posture"`
}

type ManualChecklist struct {
    Items []ManualCheckItem `json:"items"`
}

type ManualCheckItem struct {
    ID              string   `json:"id"`
    Title           string   `json:"title"`
    Instructions    string   `json:"instructions"`
    ExpectedResult  string   `json:"expected_result"`
    BehaviorCardIDs []string `json:"behavior_card_ids,omitempty"`
}
```

Note: `ManualCheckItem` has no `Result` or `Notes` fields — those belong to the outcome recording spec (0004b). The checklist here is a template, not a filled-in form.

## Acceptance Criteria

1. When a run reaches `ready_for_review`, the finalize stage writes `product-review.json`, `product-review.md`, `process-review.json`, `process-review.md`, and `manual-checklist.json` to the run's evidence directory.
2. When a run reaches `needs_human` or `blocked`, the same five artifacts are written, but `product-review.json` has `is_diagnostic: true` and a populated `recommended_next_action`; `manual-checklist.json` has an empty items array; and `process-review.json` has `trust_level: "low"`.
3. Behavior cards are derived from `### Scenario:` blocks in the spec content using deterministic markdown parsing; each scenario becomes one card with its Given/When/Then extracted.
4. When the spec has no `### Scenario:` blocks, behavior cards are derived from acceptance criteria — one card per criterion.
5. Each behavior card has an `automatic_status` of `proven`, `mixed`, `failed`, or `unclear` derived from overall run evidence (validation, acceptance, review findings).
6. Each behavior card links to evidence artifact files (not specific entries within them).
7. The process trust summary computes trust level using deterministic rules: `high` when all signals clean, `medium` when no blockers but degraded flags or non-blocking concerns exist, `low` when blocking issues or repeated-failure escalation present.
8. The recommended posture is derived directly from trust level: high→`quick_accept_path`, medium→`manual_check_carefully`, low→`do_not_accept_without_changes`.
9. Manual checklist items are derived from `### Manual` under `## Validation` if present, otherwise from scenarios.
10. If review packet generation fails, the run's terminal status is unchanged and the error is logged.
11. Existing evidence artifacts (`review.json`, `acceptance.json`, `validation.json`, `review.md`) remain present and unchanged.
12. The `.md` artifacts are human-readable renderings of their corresponding JSON — not independent data sources.

## Scenarios

### Scenario: clean run produces full review packet
**Given:** a run reached `ready_for_review` with all validation passed, all acceptance criteria passed, no blocking review findings, and no degraded flags
**When:** the finalize stage completes
**Then:** the evidence directory contains all five new artifacts; `product-review.json` has `is_diagnostic: false`, behavior cards with `automatic_status: "proven"`, and no surprises; `process-review.json` has `trust_level: "high"` and `recommended_posture: "quick_accept_path"`; `manual-checklist.json` has one item per scenario

### Scenario: spec with no scenarios falls back to acceptance criteria
**Given:** a spec has 3 acceptance criteria but no `### Scenario:` blocks, and the run reached `ready_for_review`
**When:** the finalize stage completes
**Then:** `product-review.json` contains 3 behavior cards, one per acceptance criterion, with titles derived from criterion text; `manual-checklist.json` has 3 items derived from the same criteria

### Scenario: degraded run produces medium-trust packet
**Given:** a run reached `ready_for_review` with validation passed and acceptance passed, but the diff was unavailable during review (degraded flag `diff_unavailable`)
**When:** the finalize stage completes
**Then:** `process-review.json` has `trust_level: "medium"`, `degraded_flags: ["diff_unavailable"]`, and `recommended_posture: "manual_check_carefully"`; behavior cards have `automatic_status: "mixed"`

### Scenario: blocked run produces diagnostic packet
**Given:** a run ended in `blocked`
**When:** the finalize stage completes
**Then:** `product-review.json` has `is_diagnostic: true`, a populated `blocker_summary`, a populated `recommended_next_action`, and behavior cards with `automatic_status: "unclear"`; `process-review.json` has `trust_level: "low"`; `manual-checklist.json` has an empty items array

### Scenario: packet generation failure does not affect run status
**Given:** a run reached `ready_for_review` but the scenario parser encounters malformed spec content that causes an error
**When:** the finalize stage completes
**Then:** the run is persisted with status `ready_for_review`, the error is logged, and the five review packet artifacts are absent from the evidence directory

### Scenario: spec with explicit manual validation section
**Given:** a spec has 2 scenarios and a `### Manual` subsection under `## Validation` with 3 explicit manual steps
**When:** the finalize stage completes for a `ready_for_review` run
**Then:** `manual-checklist.json` has 3 items matching the explicit manual steps (not 2 derived from scenarios); each item links to the relevant behavior card by ID

## Validation

### Automatic
- `go test ./internal/next/reviewpacket/...`
- `go test ./internal/next/specloop/stages/...`
- `go vet ./...`

### Manual
1. Run a fixture spec that reaches `ready_for_review` and verify `product-review.json`, `product-review.md`, `process-review.json`, `process-review.md`, and `manual-checklist.json` exist in the evidence directory.
2. Verify `product-review.md` is readable and leads with behavior cards, not raw machine findings.
3. Verify a blocked fixture run produces a diagnostic packet with `is_diagnostic: true` and no manual checklist items.

## Deferred
- CLI commands for review packet display and interaction (Spec 0004b)
- Review outcome recording (Spec 0004b)
- Guided interactive review flow (Spec 0004b)
- Per-scenario evidence traceability
- Project-level review dashboards
- SPC rollups and trend charts
- Web interface for review workflows
