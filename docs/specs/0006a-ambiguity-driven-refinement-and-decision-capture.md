# Spec 0006a — Ambiguity-Driven Refinement and Decision Capture

## spec_id
ambiguity-driven-refinement-and-decision-capture

## Depends on
- spec-0004d

## Vision

By the end of the 0002-0005 series, Gromit can generate stronger proof, apply more adversarial pressure, replay regressions, and learn from review outcomes. But all of that pressure still acts mostly after the system has committed to an interpretation of the spec.

That leaves a costly upstream gap: strategically ambiguous specs. When the spec leaves a material product decision unresolved, the system can still produce code that is internally consistent, fully tested, and wrong for the creator's intent. Those failures tend to surface as late clarification or `rework_vision_change`, which is exactly the kind of human tactical intervention the vision wants to eliminate.

This spec moves that clarification earlier. It adds a pre-planning refinement stage that asks a small number of high-leverage questions only when the answer would materially change planning, implementation, validation, or review. The answers become explicit decision records, not hidden chat history. Future runs can reuse those decisions, and promoted `refinement_guidance` from 0004d finally has a first-class execution surface.

## Summary

Add a `Refine` stage after `Compile` and before `Plan`. The stage reads the compiled spec packet, active `refinement_guidance` entries, and relevant prior decision records, then determines whether materially ambiguous product choices remain. If not, it records a clear refinement report and continues. If clarification is required, it writes a machine-readable and human-readable clarification packet, returns `needs_human`, and waits for explicit answers recorded through a new `gromit-next refine` CLI flow. Answered questions are persisted both to the run and to a project-local decision ledger, then synthesized into a deterministic `RefinementContext` that downstream stages consume.

## Goals

### Primary
- Detect strategically ambiguous specs before planning starts
- Ask at most a small number of high-value clarification questions
- Persist answers as explicit, auditable decision records instead of ephemeral prompt context
- Reuse relevant prior decisions so the same strategic ambiguity is not repeatedly re-asked
- Inject resolved decisions into planning, execution, verification, and review prompts

### Secondary
- Make `refinement_guidance` from 0004d operational rather than merely stored
- Preserve a strict distinction between clarification and spec rewriting

## Non-goals

- Auto-editing the source spec file in the repo
- Free-form conversational refinement inside the specloop
- Cross-project or global decision sharing
- Semantic/fuzzy matching of prior decisions; v1 uses exact `question_key` matching
- Resolving direct contradictions between a new answer and the authored spec text; if clarification reveals that the written spec is wrong, the human must edit the spec explicitly and re-run
- Replacing the existing review or distillation flows from 0004a-0005e

## Architecture

### Pipeline position

Insert a new stage after `Compile`:

```text
Init → Compile → Refine → Plan → WriteContracts → SynthesizeCounterexamples
    → Execute → WriteScenarioTests → Validate → ReplayRegression
    → Review → AdversarialReview → Accept → Evidence → Finalize
```

`Refine` runs before any planning or contract generation so that the entire downstream proof stack works from the same clarified intent.

`Refine` is included in `dryRunStages`. A dry-run should surface clarification requirements without executing tasks.

On resume, `filterStagesForResume()` continues to skip `compile`, but it must NOT skip `refine`. Resuming a clarification-paused run should re-enter `Refine`, consume recorded answers, and then proceed to `Plan`.

### Package layout

```text
internal/next/refinement/          # NEW — assessment, packet types, decision store, context rendering
internal/next/specloop/stages/     # NEW refine.go stage
cmd/gromit-next/refine.go          # NEW — refine show, refine answer
```

`refinement` has no dependency on cobra or specloop internals. It owns the decision schemas, matching rules, and deterministic `RefinementContext` rendering.

### Core model

The design intentionally separates three artifacts:

1. `ClarificationPacket` — the unresolved questions for this run
2. `DecisionRecord` — durable answers stored in the project cell
3. `RefinementContext` — the deterministic markdown context injected into downstream prompts for this run

#### ClarificationPacket

Written to the run evidence directory as `clarification-packet.json` and rendered as `clarification-packet.md`.

```go
type ClarificationPacket struct {
    RunID             string               `json:"run_id"`
    SpecID            string               `json:"spec_id"`
    Summary           string               `json:"summary"`
    QuestionCount     int                  `json:"question_count"`
    GuidanceUsed      []string             `json:"guidance_used,omitempty"` // active refinement_guidance titles
    AppliedDecisionIDs []string            `json:"applied_decision_ids,omitempty"`
    Questions         []RefinementQuestion `json:"questions"`
    CreatedAt         time.Time            `json:"created_at"`
}

type RefinementQuestion struct {
    ID               string   `json:"id"`                 // q-001, q-002, ...
    Key              string   `json:"key"`                // stable exact-match reuse key, e.g. "editing-mode"
    Prompt           string   `json:"prompt"`
    WhyItMatters     string   `json:"why_it_matters"`     // what downstream behavior depends on the answer
    SuggestedOptions []string `json:"suggested_options,omitempty"`
}
```

The advisor may emit at most `MaxQuestions` questions. Default: 3.

The prompt contract for the advisor is strict:
- Ask a question only if the answer would materially change planning, contracts, scenario tests, review, or acceptance
- Prefer questions about user-visible behavior, irreversible tradeoffs, or deployment/business constraints
- Do not ask implementation-detail questions that the executor should decide
- Do not generate speculative "nice to know" questions
- Generate `question_key` values as short kebab-case names for the underlying decision dimension
- When an active prior decision already covers the same ambiguity, reuse its exact `question_key` rather than minting a synonym

#### DecisionRecord

Stored in the project cell at `cellPath/refinement/decisions.json`.

```go
type DecisionRecord struct {
    ID            string    `json:"id"`             // rd-<hash>
    QuestionKey   string    `json:"question_key"`
    Question      string    `json:"question"`
    Answer        string    `json:"answer"`
    Scope         string    `json:"scope"`          // "project" or "spec"
    SpecID        string    `json:"spec_id,omitempty"` // required when scope == "spec"
    Status        string    `json:"status"`         // active, superseded
    SourceRunID   string    `json:"source_run_id"`
    CreatedAt     time.Time `json:"created_at"`
    SupersededBy  string    `json:"superseded_by,omitempty"`
}
```

Deterministic ID:
- Hash `(question_key, normalized answer, scope, spec_id)` via SHA-256
- Use first 8 hex chars, prefix `rd-`

Matching rules are deterministic:
- Load only `active` decisions
- `scope: "project"` applies to any run in the same project
- `scope: "spec"` applies only when `SpecID` matches the current run's `SpecID`
- Matching uses exact `QuestionKey` equality only
- If both project-scoped and spec-scoped active decisions exist for the same key, the spec-scoped decision wins for that spec

When a new answer is recorded with the same `(question_key, scope target)` as an existing active decision:
- the old decision is marked `superseded`
- the new decision becomes `active`

This gives the ledger revision semantics without adding a separate edit command.

#### Run-local decisions

The run evidence directory also stores `refinement-decisions.json`, which records the exact decisions used by this run:

```go
type RunDecision struct {
    QuestionID   string    `json:"question_id"`
    QuestionKey  string    `json:"question_key"`
    Question     string    `json:"question"`
    Answer       string    `json:"answer"`
    Scope        string    `json:"scope"`
    DecisionID   string    `json:"decision_id"`
    RecordedAt   time.Time `json:"recorded_at"`
}
```

This file is the run-local audit trail. The project-cell decision store is the reusable memory.

### Refine stage

The stage lives in `internal/next/specloop/stages/refine.go`.

```go
type RefinementAdvisor interface {
    Assess(ctx context.Context, input AssessInput) (Assessment, error)
}

type AssessInput struct {
    SpecPacket        string
    SpecConstraints   string
    ActiveGuidance    []playbook.Entry
    ActiveDecisions   []DecisionRecord
}

type Assessment struct {
    Status             string               `json:"status"` // clear, clarification_required
    Summary            string               `json:"summary"`
    AppliedDecisionIDs []string             `json:"applied_decision_ids,omitempty"`
    Questions          []RefinementQuestion `json:"questions,omitempty"`
}
```

`RefineStage.Run()` behavior:

1. If `rs.RefinementResolved` is true, return `Continue`
2. Read `spec-packet.md` from the run directory
3. Load active `refinement_guidance` entries from the playbook store
4. Load matching active decisions from the decision ledger
5. If `clarification-packet.json` already exists:
   - if `refinement-decisions.json` fully answers all packet questions, build `RefinementContext`, set run-state fields, emit `refinement_resolved`, return `Continue`
   - otherwise return `NeedsHuman` using the existing packet; do NOT regenerate new questions
6. If no packet exists, call `RefinementAdvisor.Assess(...)`
7. If assessment status is `clear`:
   - write `refinement-report.json`
   - write `refinement-context.md` containing any auto-applied prior decisions
   - set `rs.RefinementResolved = true`
   - set `rs.RefinementContext`
   - emit `refinement_assessed`
   - return `Continue`
8. If assessment status is `clarification_required`:
   - write `clarification-packet.json` and `.md`
   - set `rs.BlockerSummary = "clarification required: answer strategic refinement questions before planning"`
   - emit `clarification_required`
   - return `NeedsHuman`

The stage is idempotent. Once a packet exists, question churn is forbidden until that packet is either answered or the run is abandoned.

### Stage-provider wiring

`RealStageProviderConfig` gains a `CellPath string` field, resolved in `exec spec` from the project cell before `BuildStages(...)` is called. `BuildStages(...)` constructs `RefineStage` with:
- the run store for reading `spec-packet.md`
- `CellPath` for loading playbook guidance and the decision ledger
- the run evidence directory for clarification artifacts

This explicit wiring matters because later stages are instantiated before the run executes. The implementation must not rely on mutating files at refine time and expecting already-built stages to re-read them implicitly.

### RefinementContext

The stage synthesizes a deterministic markdown addendum and stores it in both:
- `evidence/refinement-context.md`
- `rs.RefinementContext`

Rendering rules:
- Title: `## Clarified Decisions`
- Sorted by `question_key`
- Each entry includes question, answer, scope, and source decision ID
- No LLM summarization; this is deterministic rendering from recorded decisions

Example:

```markdown
## Clarified Decisions

### editing-mode
- Scope: project
- Question: Should record editing be inline or modal?
- Answer: Use modal editing only. Inline editing is out of scope for this project until further notice.
- Source decision: rd-7a12c0ef
```

This is the single prompt-injection surface for resolved clarification.

### RunState and task propagation

Add these fields to `runstore.RunState`:

```go
RefinementResolved   bool     `json:"refinement_resolved"`
RefinementContext    string   `json:"refinement_context,omitempty"`
RefinementDecisionIDs []string `json:"refinement_decision_ids,omitempty"`
```

Add these fields to `planner.TaskDef` and `runstore.Task`:

```go
RefinementContext string `json:"refinement_context,omitempty"`
```

Why both levels:
- `RunState.RefinementContext` is the authoritative run-wide context
- tasks need a copied snapshot because `ProviderTaskRunner` only sees a `Task`

Plan stage changes:
- `PlanRequest` and `FixPlanRequest` gain `RefinementContext string`
- planner prompt builders render a `## Clarified Decisions` section when non-empty
- `PlanStage.Run()` copies `rs.RefinementContext` into each new task

Task execution changes:
- `ProviderTaskRunner` renders `### Clarified Product Decisions` after `### Spec Constraints`

### Downstream prompt injection

Any stage whose judgment depends on intended behavior must receive `RefinementContext`:

| Component | Injection mechanism |
|---|---|
| Planner / FixPlanner | `PlanRequest.RefinementContext`, `FixPlanRequest.RefinementContext` |
| Task execution | `Task.RefinementContext` |
| WriteContracts | include `RefinementContext` in contract-writer prompt |
| SynthesizeCounterexamples | include `RefinementContext` alongside authored scenarios |
| WriteScenarioTests | include `RefinementContext` in scenario-test writer prompt |
| Review | include `rs.RefinementContext` in `review.RunInput` |
| AdversarialReview | include `rs.RefinementContext` in adversarial run input |
| Accept | include `rs.RefinementContext` in acceptance evaluation input |

The purpose is not to create a new hidden spec, but to ensure that every stage sees the same clarified decisions instead of reinterpreting ambiguity independently.

Implementation note: stages created by `BuildStages(...)` must read `rs.RefinementContext` at `Run(...)` time, not snapshot refinement content only at construction time.

### CLI flow

Add a `gromit-next refine` command group.

**`gromit-next refine show --run <run-id>`**
- loads and renders `clarification-packet.md`
- errors clearly if the run does not currently require clarification

**`gromit-next refine answer --run <run-id>`**
- guided interactive flow over unresolved packet questions
- prompts for an answer to each unanswered question
- prompts for scope per answer: `spec` or `project` (default `spec`)
- writes/updates `refinement-decisions.json`
- writes/updates project-cell `refinement/decisions.json`
- supersedes any matching active decision when the new answer targets the same key and scope
- prints a completion message instructing the user to resume the run with the existing `exec spec --resume` flow

The command does NOT resume the run automatically. Strategic clarification and execution remain separate, auditable actions.

### Configuration

Add `RefinementConfig` to `execpolicy.Policy`:

```go
type RefinementConfig struct {
    Enabled      bool   `json:"enabled"`
    MaxQuestions int    `json:"max_questions"`
    ModelTier    string `json:"model_tier"`
}
```

Default policy:

```json
{
  "refinement": {
    "enabled": true,
    "max_questions": 3,
    "model_tier": "medium"
  }
}
```

Rules:
- `enabled: false` skips the stage entirely
- `max_questions` must be `>= 1`
- `model_tier` resolves through the existing tier-to-model mapping

### Events

Add these event types to `runstore/events.go`:

```go
type RefinementAssessedEvent struct {
    BaseEvent
    Status             string   `json:"status"` // clear or clarification_required
    QuestionCount      int      `json:"question_count"`
    AppliedDecisionIDs []string `json:"applied_decision_ids,omitempty"`
}

type ClarificationRequiredEvent struct {
    BaseEvent
    QuestionCount int    `json:"question_count"`
    Summary       string `json:"summary"`
}

type RefinementResolvedEvent struct {
    BaseEvent
    DecisionIDs []string `json:"decision_ids"`
}
```

The stage appends the first and third events. The `refine answer` CLI appends no new run event itself; resolution is logged when the resumed run consumes the answers.

## Acceptance Criteria

1. The pipeline includes a `Refine` stage after `Compile` and before `Plan`.
2. When refinement is enabled and the advisor returns `clear`, the run continues without human intervention and writes `refinement-report.json`.
3. When the advisor returns `clarification_required`, the run writes `clarification-packet.json` and `clarification-packet.md`, then transitions to `needs_human` with a clarification-specific blocker summary.
4. The clarification packet contains at most `max_questions` questions; default is 3.
5. If a clarification packet already exists and not all questions are answered, `Refine` returns `needs_human` again without regenerating or rewording the questions.
6. `gromit-next refine show --run <run-id>` renders the existing clarification packet for a clarification-paused run.
7. `gromit-next refine answer --run <run-id>` records answers to `refinement-decisions.json` in the run evidence directory.
8. Project-scoped answers are also persisted to `cellPath/refinement/decisions.json` as active `DecisionRecord` entries.
9. Recording a new answer for the same `question_key` and scope supersedes the prior active decision instead of creating two active records.
10. On resume, when all packet questions have answers, `Refine` synthesizes deterministic `refinement-context.md`, sets `rs.RefinementContext`, marks `rs.RefinementResolved = true`, and continues to `Plan`.
11. Planner and fix-planner prompts include a `Clarified Decisions` section when `RefinementContext` is non-empty.
12. New tasks inherit `RefinementContext`, and task execution prompts render it separately from `SpecConstraints`.
13. Contract writing, counterexample synthesis, scenario-test writing, review, adversarial review, and acceptance all receive the same `RefinementContext`.
14. Matching prior decisions uses exact `question_key` plus scope rules only; no semantic or fuzzy reuse is introduced.
15. The source spec file in the repo is never modified automatically by this flow.

## Scenarios

### Scenario: Clear spec proceeds without clarification
**Given:** a spec packet with explicit scenarios, clear acceptance criteria, no active `refinement_guidance` requiring follow-up, and no unresolved strategic tradeoffs
**When:** the `Refine` stage runs
**Then:** it writes `refinement-report.json` with status `clear`, sets `rs.RefinementResolved = true`, emits `refinement_assessed`, and continues directly to `Plan`

### Scenario: Ambiguous product choice pauses the run
**Given:** a spec packet that defines data import behavior but does not say whether partial failures should block the whole import or allow per-row success
**When:** the `Refine` stage runs
**Then:** it writes a clarification packet with a question such as `partial-import-failure-mode`, returns `needs_human`, and no planning occurs in that run until the question is answered

### Scenario: Resume with incomplete answers preserves the packet
**Given:** a run has `clarification-packet.json` with 3 questions, and `refinement-decisions.json` contains answers for only 2 of them
**When:** the run is resumed
**Then:** `Refine` does not call the advisor again, does not alter the packet, and returns `needs_human` again referencing the remaining unanswered question

### Scenario: Answered questions unlock planning and propagate downstream
**Given:** a clarification-paused run has all packet questions answered via `gromit-next refine answer`
**When:** the run is resumed
**Then:** `Refine` writes `refinement-context.md`, stores the same content in `rs.RefinementContext`, and `Plan`, `WriteContracts`, `WriteScenarioTests`, `Review`, and `Accept` all receive that context in their prompts or inputs

### Scenario: Prior project decision suppresses a repeated question
**Given:** the project decision ledger contains an active project-scoped decision with `question_key: "editing-mode"`
**And:** a new spec in the same project would otherwise trigger the same ambiguity
**When:** the `Refine` stage runs for the new spec
**Then:** the advisor sees the active decision, the question is not re-emitted as unresolved, and the resulting `RefinementContext` includes the reused decision

### Scenario: New project answer supersedes an old one
**Given:** the project decision ledger contains an active project-scoped decision for `question_key: "editing-mode"` with answer `inline`
**When:** the user answers a new clarification packet question for the same `question_key` with project scope and answer `modal`
**Then:** the old decision is marked `superseded`, the new decision becomes `active`, and future runs in that project see only the `modal` answer

## Validation

- `go test ./cmd/gromit-next/... ./internal/next/refinement/... ./internal/next/specloop/...`
- `go test ./internal/next/planner/... ./internal/next/contract/... ./internal/next/review/... ./internal/next/acceptor/...`
- `go vet ./cmd/gromit-next/... ./internal/next/...`
- Manual:
  1. Run `gromit-next exec spec --project <project> --spec <spec> --dry-run` with an intentionally ambiguous spec and verify that the run stops after `Refine` with a clarification packet.
  2. Run `gromit-next refine show --run <run-id>` and confirm the packet renders cleanly.
  3. Run `gromit-next refine answer --run <run-id>`, answer all questions, then resume via `gromit-next exec spec --project <project> --resume <run-id>`.
  4. Inspect the resumed run artifacts and verify that `refinement-context.md` exists and the downstream evidence reflects the clarified choice.
