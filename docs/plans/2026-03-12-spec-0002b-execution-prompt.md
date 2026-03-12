# Spec 0002b — Execution Prompt

> **REQUIRED:** Use `superpowers:executing-plans` to implement this plan task-by-task.
> **Plan document:** `docs/plans/2026-03-12-spec-0002b-execution-plan.md`
> **Testing plan:** `docs/plans/2026-03-12-spec-0002b-testing-plan.md`

---

## Project Context

- **Language/runtime:** Go 1.24, CLI built with `github.com/spf13/cobra`
- **Project root:** `/Users/dabrams/gromit`
- **Module path:** `github.com/danabrams/gromit`
- **CLI entry point:** `cmd/gromit-next/main.go`
- **Provider abstraction:** `internal/provider/` — supports tiers `low`, `medium`, `high`, `xhigh`

### Existing Packages (from Spec 0001 + 0002a)

| Package | Responsibility |
|---------|---------------|
| `internal/next/execpolicy/` | Execution policy config: always-run checks, budgets, model tiers |
| `internal/next/runstore/` | Run record CRUD, artifact layout on disk, events log |
| `internal/next/planner/` | Agent-driven plan generation, task decomposition, plan validation |
| `internal/next/executor/` | Agent invocation in worktree, inspection, result extraction |
| `internal/next/validator/` | Validation command runner (targeted, always-run, final) |
| `internal/next/evidence/` | Evidence bundle assembly (review.md, metrics.json, etc.) |
| `internal/next/specloop/` | SpecLoop + TaskLoop + stage pipeline orchestration |
| `internal/next/specloop/stages/` | Stage implementations (init, compile, plan, execute, validate, evidence, finalize) |
| `internal/next/artifact/` | Spec 0001 — artifact management |
| `internal/next/contextpkt/` | Spec 0001 — context packet compilation |
| `internal/next/projectcell/` | Spec 0001 — project cell management |
| `internal/provider/` | Provider interface, tier constants, model mapping |

### Conventions

- **Nil-field normalization:** Use exported `NormalizeNilFields()` for cross-package types; use unexported `normalizeNilFields()` for internal-only types. Both map nil slices/maps to empty values.
- **Tests:** Co-located (`foo_test.go` next to `foo.go`), table-driven, fakes over mocks.
- **TDD:** Write a failing test FIRST, then implement to make it pass, then commit.

---

## Architecture Overview

Two new packages plus extensions to existing packages:

| Package | Responsibility |
|---------|---------------|
| `internal/next/review/` | Multi-facet LLM code review: facet registry, finding types, severity, threshold, prompts, new-vs-preexisting matching |
| `internal/next/acceptor/` | Acceptance criterion evaluation: per-criterion pass/fail/unclear, evidence refs, rationale |

### Extensions to existing packages

| Package | What changes |
|---------|-------------|
| `internal/next/specloop/stages/review.go` | ReviewStage wrapper (stage interface over review/ domain logic) |
| `internal/next/specloop/stages/accept.go` | AcceptStage wrapper (stage interface over acceptor/ domain logic) |
| `internal/next/specloop/` | FailureContext extensions for review/acceptance data |
| `internal/next/execpolicy/` | Review config (facets, tiers, replan_threshold), evaluator model tier |
| `internal/next/evidence/` | `WriteReviewFindings()`, `WriteAcceptance()`, review.md sections |
| `cmd/gromit-next/` | Stage provider wiring bug fix, spec list path fix, exec list exit code fix |

### Dependency flow

```
execpolicy   runstore              (leaf — no internal/next deps)
    \          |
     \    planner  executor  validator  evidence  review  acceptor
      \      \        |         |        /         /       /
       -------\-------+---------+-------/---------/-------/
                -----       specloop       -----
```

### Extended stage pipeline

```
Init -> Compile -> Plan -> Execute -> Validate -> Review -> Accept -> Evidence -> Finalize
                    ^                    |           |        |
                    |____________________|___________|________|
                     failures loop back to Plan
```

ReviewStage and AcceptStage are inserted between ValidateStage and EvidenceStage. Both use the `evaluator` model tier from the execution policy. Both can produce `ReplanFrom`, consuming `max_spec_cycles` budget.

---

## Key Design Decisions

Read the full design spec carefully before starting: `docs/plans/2026-03-11-spec-0002b-review-acceptance-design.md`

### Stage interface (existing — do not modify)

```go
// specloop/stage.go
type Stage interface {
    Name() string
    Run(ctx context.Context, run *runstore.RunState) (NextAction, error)
}

type NextAction struct {
    Kind    ActionKind      // Continue, ReplanFrom, NeedsHuman, Blocked
    Context *FailureContext // populated on ReplanFrom only
}

type FailureContext struct {
    Failures []string `json:"failures"`
    Cycle    int      `json:"cycle"`
    Diff     string   `json:"diff,omitempty"`
}
```

`FailureContext.Failures` remains `[]string`. Helper functions serialize structured data into formatted strings:
- `ReviewFailuresToStrings(findings []Finding, threshold Severity) []string` — formats blocking findings as `review:<facet>:<severity>: <file>:<line> — <description> (suggested fix: <fix>)`
- `AcceptanceFailuresToStrings(results []CriterionResult) []string` — formats failed/unclear criteria as `acceptance:<status>: "<criterion>" — <rationale> [<evidence_refs>]`. Differentiates `fail` (implement the missing behavior) from `unclear` (add tests or evidence) in the formatted string so the planner produces appropriately targeted fix tasks.

```go
// These helpers are NOT on FailureContext — they are free functions used by
// ReviewStage and AcceptStage to build the Failures slice before constructing FailureContext.
```

### Severity levels

```go
// review/severity.go
type Severity int // JSON representation is a string (e.g., "error", "warning")

const (
    SeverityError      Severity = ... // "error"
    SeverityWarning    Severity = ... // "warning"
    SeveritySuggestion Severity = ... // "suggestion"
    SeverityInfo       Severity = ... // "info"
)

func (s Severity) Rank() int          // numeric rank for comparison
func (s Severity) String() string     // string representation
func ParseSeverity(s string) (Severity, error) // parse from string, returns error for unknown values
```

Severity ordering for threshold comparison: `error > warning > suggestion > info`.

### Finding format

```go
// review/finding.go
type Finding struct {
    Facet        string   `json:"facet"`
    Severity     Severity `json:"severity"`
    File         string   `json:"file"`
    Line         int      `json:"line,omitempty"`
    Description  string   `json:"description"`
    SuggestedFix string   `json:"suggested_fix,omitempty"`
    Cycle        int      `json:"cycle"`
    Disposition  string   `json:"disposition"` // "new" or "pre-existing"
}
```

### Facet review output (per-facet LLM response)

```json
{
  "facet": "code_quality",
  "findings": [
    {
      "severity": "suggestion",
      "file": "internal/next/validator/runner.go",
      "line": 42,
      "description": "nil pointer if commands list is empty",
      "suggested_fix": "add empty check before iteration"
    }
  ]
}
```

### Facet registry

```go
// review/registry.go
type FacetDef struct {
    Name           string
    PromptTemplate string // Go text/template with {{.Diff}}, {{.SpecPacket}}, etc.
    DefaultTier    string // provider tier (e.g., "high", "medium")
}

type Registry struct {
    facets map[string]FacetDef
}

func NewRegistry() *Registry { /* pre-populated with built-in facets */ }
func (r *Registry) Get(name string) (FacetDef, bool)
func (r *Registry) ListNames() []string
```

Built-in facets (all pre-registered, first two enabled by default):

| Facet | What it checks | Default tier |
|-------|---------------|-------------|
| `spec_alignment` | Does the diff implement what the spec asked for? | high |
| `code_quality` | Naming, structure, duplication, readability | medium |
| `logic_gaps` | Off-by-one, nil handling, missing error paths | high |
| `test_coverage` | Are new code paths tested? Missing edge cases? | medium |
| `architecture_drift` | Does the change respect boundaries from the project cell? | medium |

Each facet has a Go `text/template` prompt that receives `{{.Diff}}`, `{{.SpecPacket}}`, and other context variables. The 5 built-in facet prompts evaluate:
- **spec_alignment**: Compares the diff against the spec's in-scope requirements, checking for missing, incomplete, or divergent implementations.
- **code_quality**: Evaluates naming conventions, code structure, duplication, and readability against project conventions.
- **logic_gaps**: Checks for off-by-one errors, nil handling issues, missing error paths, and incomplete logic branches.
- **test_coverage**: Assesses whether new code paths have corresponding tests and flags missing edge-case coverage.
- **architecture_drift**: Reviews whether the change respects package boundaries, dependency direction, and architectural constraints from the project cell.

### Configurable threshold

There is no standalone `Threshold` type. Threshold comparison uses a free function:

```go
// review/threshold.go
// IsBlocking returns true if findingSeverity is at or above the threshold.
// Both threshold and findingSeverity are Severity values.
func IsBlocking(threshold, findingSeverity Severity) bool
```

| Threshold value | Blocks on |
|----------------|-----------|
| `SeverityError` | error only |
| `SeverityWarning` | error + warning |
| `SeveritySuggestion` (default) | error + warning + suggestion |

### New-vs-preexisting matching

On fix cycles, the review stage must distinguish new findings from pre-existing ones. Only **new** findings at or above threshold trigger replanning.

The ReviewStage instance holds prior findings (`[]Finding`) in-memory for disposition matching across SpecLoop cycles. Because the stage object persists across cycles (it is constructed once and reused), no serialization to RunState is needed for matching purposes. The RunState `ReviewFindings []string` field serves a different purpose: planner consumption and evidence.

Matching algorithm (v1): A finding is **pre-existing** if a prior finding exists with:
1. Same file path, AND
2. Current finding's description contains prior finding's description as a substring, OR vice versa

```go
// review/matching.go
func ClassifyFindings(current []Finding, prior []Finding) []Finding
// Returns current findings with Disposition set to "new" or "pre-existing"
```

### Acceptance evaluation output

```go
// acceptor/result.go
type CriterionResult struct {
    Criterion    string   `json:"criterion"`
    Status       string   `json:"status"` // "pass", "fail", "unclear"
    Rationale    string   `json:"rationale"`
    EvidenceRefs []string `json:"evidence_refs"`
}

type AcceptanceResult struct {
    Results          []CriterionResult `json:"results"`
    AllPass          bool              `json:"all_pass"`
    HasFailOrUnclear bool              `json:"has_fail_or_unclear"`
}
```

### Acceptance rules

- Any `fail` triggers replan (within budget). Planner gets: "This criterion failed. Implement the missing behavior."
- Any `unclear` triggers replan (within budget). Planner gets: "This criterion could not be evaluated. Add tests or observable evidence that proves or disproves it."
- Budget exhausted with remaining failures/unclear -> `needs_human`.
- All `pass` -> continue to evidence stage.

### ReviewStage FailureContext

When ReviewStage triggers replanning, it provides blocking findings as structured context:

```json
{
  "failures": [
    "review:spec_alignment:error: internal/next/validator/runner.go:42 — nil pointer if commands list is empty (suggested fix: add empty check before iteration)"
  ],
  "cycle": 2
}
```

Each failure string is formatted as `review:<facet>:<severity>: <file>:<line> — <description> (suggested fix: <fix>)` so the planner can produce targeted fix tasks.

### AcceptStage FailureContext

When AcceptStage triggers replanning, it provides failed/unclear criteria:

```json
{
  "failures": [
    "acceptance:fail: \"audit log entry created\" — Implementation calls audit service but no test verifies the call succeeds. [evidence/acceptance.json, evidence/test-results.json]"
  ],
  "cycle": 2
}
```

Each failure string is formatted as `acceptance:<status>: "<criterion>" — <rationale> [<evidence_refs>]`.

### Budget sharing

Validation, review, and acceptance fix cycles all consume from the same `max_spec_cycles` budget. The SpecLoop already handles this — ReviewStage and AcceptStage return `ReplanFrom` just like ValidateStage does, and the SpecLoop's cycle counter applies uniformly.

### FinalizeStage three-gate condition

FinalizeStage determines `ready_for_review` only when ALL three gates pass:

```
allDone && FinalValidationPassed && FinalReviewPassed && FinalAcceptancePassed
```

If any gate fails and budget is exhausted, the terminal state is `needs_human`. FinalizeStage preserves worktrees for ALL terminal states (including `blocked`). InitStage auto-cleans blocked worktrees from prior runs of the same spec.

### RunState extensions

RunState gains four new fields to carry review and acceptance results across cycles:

```go
type RunState struct {
    // ... existing fields ...
    FinalReviewPassed     bool                `json:"final_review_passed"`
    FinalAcceptancePassed bool                `json:"final_acceptance_passed"`
    ReviewFindings        []string `json:"review_findings,omitempty"`
    AcceptanceResults     []string `json:"acceptance_results,omitempty"`
}
```

- `FinalReviewPassed` — set to true when the last review cycle produced no new blocking findings.
- `FinalAcceptancePassed` — set to true when the last acceptance evaluation produced all-pass results.
- `ReviewFindings` — human-readable review finding strings for planner consumption and evidence. ReviewStage holds structured `[]Finding` internally for disposition matching across cycles.
- `AcceptanceResults` — human-readable acceptance result strings for planner consumption and evidence.

### AcceptanceResult field naming

`AcceptanceResult` uses `Results` (not `Criteria`) as the field name:

```go
type AcceptanceResult struct {
    Results         []CriterionResult `json:"results"`
    AllPass         bool              `json:"all_pass"`
    HasFailOrUnclear bool            `json:"has_fail_or_unclear"`
}
```

`AllPass` and `HasFailOrUnclear` are computed convenience fields set during evaluation.

### ParseAcceptanceCriteria

A helper in the `acceptor` package parses `## Acceptance Criteria` from spec markdown:

```go
// acceptor/parse.go
func ParseAcceptanceCriteria(specMarkdown string) ([]string, error)
```

This extracts the numbered list items under the `## Acceptance Criteria` heading, returning them as individual criterion strings for per-criterion evaluation.

### Evaluator model tier

The execution policy gains an `evaluator` field in `Models`:

```go
type Models struct {
    Planner   string `json:"planner"`
    Executor  string `json:"executor"`
    Evaluator string `json:"evaluator"` // NEW — used by ReviewStage and AcceptStage
}
```

Default: `"high"`. Both ReviewStage and AcceptStage use `policy.Models.Evaluator` to select the provider tier.

### Execution policy review config

```go
// execpolicy/policy.go — additions
type ReviewConfig struct {
    Facets           []string          `json:"facets"`
    Tiers            map[string]string `json:"tiers"`
    ReplanThreshold  string            `json:"replan_threshold"`
}
```

Default:
```json
{
  "review": {
    "facets": ["spec_alignment", "code_quality"],
    "tiers": {
      "spec_alignment": "high",
      "code_quality": "medium"
    },
    "replan_threshold": "suggestion"
  }
}
```

Validation: all facet names in `review.facets` must exist in the built-in registry. Unknown facet names are rejected at policy validation time.

---

## Existing Patterns to Follow

Study these files for patterns before implementing. They show the conventions for stages, types, and bundles:

| File | What to learn |
|------|--------------|
| `internal/next/specloop/stage.go` | Stage interface, NextAction, FailureContext, ActionKind |
| `internal/next/specloop/specloop.go` | SpecLoop orchestration, replan loop, budget checks, evidence on failure |
| `internal/next/specloop/stages/validate.go` | Stage implementation pattern: interface dep, config struct, constructor, Name(), Run() returning ReplanFrom |
| `internal/next/specloop/stages/evidence.go` | EvidenceStage: how it reads RunState and writes evidence bundle |
| `internal/next/specloop/stages/finalize.go` | FinalizeStage: terminal state determination |
| `internal/next/evidence/bundle.go` | Bundler pattern: Init(), WriteX() methods, writeJSON helper |
| `internal/next/execpolicy/policy.go` | Policy loading: unmarshal-into-defaults, Validate(), NormalizeNilFields() |
| `internal/provider/provider.go` | Provider interface, tier constants, Result type |
| `internal/next/executor/executor.go` | Agent interface pattern for LLM invocations |
| `internal/next/planner/planner.go` | Agent interface pattern for plan generation |

---

## Implementation Phases

### Phase 1: Bug fixes from 0002a

Fix three bugs discovered during manual testing:
1. Agent provider wiring in `cmd/gromit-next/exec.go` — implement real `StageProvider`
2. `spec list` default path resolution in `cmd/gromit-next/spec.go`
3. `exec list` exit code on empty results

### Phase 2: `review/` package

Build the multi-facet LLM review package:
- Severity type and ordering
- Finding type with NormalizeNilFields
- IsBlocking(threshold, findingSeverity) threshold comparison function
- Facet registry with built-in facets and prompt templates
- Facet reviewer (interface for LLM invocation per facet)
- New-vs-preexisting finding matching (ClassifyFindings)
- Aggregation: run all enabled facets, collect findings, apply threshold

### Phase 3: `acceptor/` package

Build the acceptance evaluation package:
- CriterionResult and AcceptanceResult types
- Evaluator interface (LLM invocation for per-criterion evaluation)
- Input assembly (spec criteria + validation results + review findings + diff summary + task results)
- Output parsing and validation
- NormalizeNilFields for result types

### Phase 4: `ReviewStage` and `AcceptStage`

Implement stage interface for both:
- ReviewStage: invokes review package, checks threshold, returns Continue or ReplanFrom
- AcceptStage: invokes acceptor package, returns Continue, ReplanFrom, or NeedsHuman
- Pipeline insertion: stages registered between validate and evidence
- FailureContext formatting for planner consumption

### Phase 5: Fix-cycle extensions

Extend the fix-cycle replan loop:
- ReviewStage FailureContext contract (blocking findings as structured failure strings)
- AcceptStage FailureContext contract (failed/unclear criteria as structured failure strings)
- New-vs-preexisting in fix cycles: review stage reads prior findings from run state, classifies current findings
- Replan triggers: only new findings above threshold or failed/unclear criteria
- RunState extensions: fields to carry review findings and acceptance results across cycles

### Phase 6: Execution policy extensions

Extend the execution policy:
- `ReviewConfig` type with facets, tiers, replan_threshold
- `Models.Evaluator` field with default "high"
- Policy validation: facet names must exist in registry, threshold must be valid
- NormalizeNilFields for ReviewConfig (nil map/slice handling)
- DefaultPolicy update with review defaults

### Phase 7: Evidence bundle extensions

Extend the evidence bundle:
- `WriteReviewFindings()` — writes `review.json` (aggregated findings by facet)
- `WriteAcceptance()` — writes `acceptance.json` (per-criterion evaluation)
- `WriteReview()` update — add review findings by facet section and per-criterion acceptance table to `review.md`
- EvidenceStage update: reads review findings and acceptance results from RunState, writes new artifacts

### Phase 8: Integration wiring + CLI updates

- Wire ReviewStage and AcceptStage into the stage pipeline in the stage provider
- Ensure SpecLoop handles the extended pipeline (review and accept between validate and evidence)
- CLI updates if needed for new config options
- End-to-end integration tests

---

## Phase-by-Phase Workflow

**This is critical.** Each phase must be completed, reviewed, and verified before moving to the next. Context is cleared between phases to prevent degradation.

After completing each phase:

1. **Run all tests:**
   ```bash
   go test ./internal/next/... -v
   go vet ./internal/next/...
   gofmt -l internal/next/
   ```
2. **Request code review** — the reviewer will do 2 rounds of review
3. **Fix any issues** found in review
4. **Signal:** "Phase N complete, ready to clear context"
5. **Context will be cleared** before starting the next phase

### Why this matters

Spec 0002a ran all phases in a single long session, which led to context degradation, accumulated confusion, and lower quality in later phases. The phase-by-phase workflow ensures:
- Fresh context for each phase
- Focused review at phase boundaries
- Issues caught early instead of compounding
- Clear checkpoints for progress tracking

### Phase dependencies

```
Phase 1 (bug fixes)           — independent, do first
Phase 2 (review/) \
                    > — independent of each other, CAN run in parallel
Phase 3 (acceptor/) /
Phase 4 (stages)              — depends on Phase 2 + 3
Phase 5 (fix-cycle)           — depends on Phase 4
Phase 6 (exec policy)         — depends on Phase 2 (for facet registry validation)
Phase 7 (evidence)            — depends on Phase 2 + 3 (for types)
Phase 8 (integration)         — depends on all previous phases
```

---

## Subagent Dispatch Guidance

For phases where tasks are independent, subagents can work in parallel:

### Phase 2 (review/ package)
- **Parallel group A:** Severity type + Finding type + Threshold type (pure types, no deps)
- **Parallel group B:** Facet registry + prompt templates (depends on types from group A)
- **Sequential:** New-vs-preexisting matching, aggregation (depends on all above)

### Phase 3 (acceptor/ package)
- **Parallel group A:** CriterionResult type + AcceptanceResult type
- **Sequential:** Evaluator interface + input assembly + output parsing

### Phases 2 and 3 themselves can run in parallel
The `review/` and `acceptor/` packages have no dependencies on each other. Two subagents can implement them simultaneously.

### Phase 6 (exec policy extensions)
- **Parallel:** ReviewConfig type, Models.Evaluator field, policy validation updates, NormalizeNilFields
- These are all modifications to the same package but touch different parts of the code

### Phases that must be sequential
- Phase 4 depends on Phase 2 + 3 outputs
- Phase 5 depends on Phase 4
- Phase 8 depends on everything

---

## Execution Rules

1. **TDD strictly.** Write a failing test first, then implement to pass it, then commit. No implementation without a covering test.
2. **Commit frequently.** After each green test or small logical unit.
3. **DRY, YAGNI.** No speculative abstractions. Build only what the spec requires.
4. **Follow existing patterns** from `internal/next/specloop/stages/` for stage implementations. Study `validate.go` and `evidence.go` as templates.
5. **Run tests:** `go test ./internal/next/... -v`
6. **Build:** `go build ./cmd/gromit-next/`
7. **Interfaces for dependencies.** Use interfaces for LLM providers, git operations, and filesystem access so tests use fakes.
8. **No global state.** All state flows through RunState or function parameters.
9. **NormalizeNilFields convention.** Per project convention, add `NormalizeNilFields()` to types with slice/map fields. Exported for cross-package types, unexported for internal.
10. **Budget sharing is automatic.** ReviewStage and AcceptStage return `ReplanFrom` just like ValidateStage. The SpecLoop's existing cycle counter handles budget consumption. Do not add separate budget tracking.
11. **Evaluator tier.** Both ReviewStage and AcceptStage read `policy.Models.Evaluator` for the provider tier. Do not hardcode model names.
12. **FailureContext formatting.** Format failure strings so the planner can parse them into targeted fix tasks. Use the prefix conventions defined in this document (`review:<facet>:<severity>:` and `acceptance:<status>:`).

---

## What NOT to Do

- **Don't add VISION.md review outcome labels** (`accepted`, `rework_implementation_gap`, `rework_vision_change`). Those are Spec 0003.
- **Don't add custom user-defined review facets.** Only built-in facets from the registry. Custom facets may be added in a later spec.
- **Don't add parallel spec execution.**
- **Don't modify existing Spec 0001 packages** (`internal/next/artifact/`, `projectcell/`, etc.) unless absolutely required for integration. Prefer wrapping.
- **Don't create Gromit state files in target repos.** All run artifacts live in the external workspace (`~/.local/share/gromit/projects/`).
- **Don't add fuzzy matching for finding deduplication.** V1 uses exact substring match on description text. Cosine similarity may be added later.
- **Don't add PR creation or merge automation.**
- **Don't add resume/recovery** beyond artifact preservation.
- **Don't over-engineer.** Simple implementations that pass tests are better than clever abstractions.
- **Don't add indefinite retries.** All retry loops are bounded by budget config.

---

## Run Storage Layout

All run artifacts go under the external workspace. Never in the target repo.

```
~/.local/share/gromit/projects/<project-id>/
  runs/
    <run-id>/
      run.json                # Canonical run record
      spec.md                 # Copy of approved spec
      spec-packet.md          # Compiled spec context
      plan.md                 # Human-readable plan
      tasks.json              # Machine-readable task list (all cycles)
      events.jsonl            # Append-only event log
      execution-policy.json   # Snapshot of policy used
      tasks/
        <task-id>/
          task-packet.md      # Compiled task context (0002b executor integration)
          result.json         # Task outcome + metrics
          agent-output.txt    # Raw agent stdout (0002b executor integration)
      evidence/
        summary.md            # Human-facing run summary
        diff-summary.md       # What changed
        task-results.json     # Aggregated task outcomes
        validation.json       # Final validation results
        review.json           # NEW — Aggregated review findings (all facets)
        acceptance.json       # NEW — Per-criterion evaluation results
        review.md             # Decision sheet (updated with review + acceptance sections)
        metrics.json          # Raw signal: per-invocation records
```

### Evidence file authorship

ReviewStage and AcceptStage write structured JSON evidence files (`review.json`, `acceptance.json`) directly as side effects of their `Run()` method, using the structured types from `review/` and `acceptor/` packages respectively. The RunState `[]string` fields (`ReviewFindings`, `AcceptanceResults`) are NOT the source for evidence files — they carry human-readable summaries for planner consumption. The EvidenceStage does NOT re-derive evidence files from RunState string fields.

### review.json schema

Object keyed by facet name, each containing an array of findings:

```json
{
  "spec_alignment": [
    {
      "severity": "error",
      "file": "internal/handler/refund.go",
      "line": 87,
      "description": "Spec requires idempotency key validation",
      "suggested_fix": "Add idempotency key lookup before processing",
      "cycle": 1,
      "disposition": "new"
    }
  ],
  "code_quality": []
}
```

### acceptance.json schema

```json
{
  "results": [
    {
      "criterion": "Zero repo pollution",
      "status": "pass",
      "rationale": "No gromit files found in target repo tracked files.",
      "evidence_refs": ["evidence/diff-summary.md", "evidence/worktree-info.json"]
    }
  ],
  "all_pass": true,
  "has_fail_or_unclear": false
}
```

---

## Success Criteria

All of these must be satisfied before the implementation is complete:

1. **Review gate** — `ready_for_review` is impossible if review finds findings above the configured threshold.
2. **Acceptance evidence** — Every acceptance criterion has an explicit pass, fail, or unclear result with rationale and evidence references.
3. **Configurable threshold** — The `review.replan_threshold` setting controls which severity levels trigger replanning.
4. **Fix-cycle from review** — Review findings above threshold trigger a fix-plan cycle that targets the specific findings.
5. **Fix-cycle from acceptance** — Acceptance `fail` or `unclear` results trigger a fix-plan cycle targeting the specific gaps.
6. **Facet configurability** — Review facets are selected from a built-in registry and can be enabled or disabled via execution policy without code changes.
7. **Severity levels** — Findings are categorized as error/warning/suggestion/info with distinct blocking behavior.
8. **VISION label deferral** — The system does not auto-label work as `accepted`. VISION review outcome labels are explicitly deferred to Spec 0003.
9. **Budget sharing** — Validation, review, and acceptance fix cycles all consume from the same `max_spec_cycles` budget. A run using cycle 1 for initial execution, cycle 2 for validation fix, and cycle 3 for review fix correctly exhausts the budget.

---

## Checkpoints

After each phase, verify:

```bash
go test ./internal/next/... -v
go vet ./internal/next/...
gofmt -l internal/next/
```

After Phase 1 and Phase 8, also verify:

```bash
go build ./cmd/gromit-next/
go vet ./cmd/gromit-next/...
```

---

## Final Verification

After all phases are complete, run the full verification:

```bash
go test ./internal/next/... -v
go test -race ./internal/next/...
go vet ./internal/next/... && go vet ./cmd/gromit-next/...
go mod tidy && git diff --exit-code go.mod go.sum
go build ./cmd/gromit-next/
gofmt -l internal/next/
```

Only push after all checks pass.
