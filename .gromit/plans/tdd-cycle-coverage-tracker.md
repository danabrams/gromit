---
id: tdd-cycle-coverage-tracker
source_spec: tdd-cycle-coverage-tracker
created: 2026-02-19
decomposed: false
---

# Hybrid Spec Coverage Tracker for TDD Cycles — Implementation Plan

**Goal:** Replace the heuristic "Claude says it's done" cycle termination with verified, runner-controlled coverage tracking using parsed acceptance criteria, self-reports, and haiku validation.

**Architecture:** A new `internal/coverage/` package provides pure domain logic (parsing, state tracking, validation response parsing). The runner's TDD cycle loop consumes the tracker to inject coverage state into red phase prompts, validate coverage claims via haiku after green phases, and control cycle termination. The tracker — not Claude — is authoritative.

**Tech Stack:** Go, text/template, JSON parsing

**Spec:** `.gromit/specs/tdd-cycle-coverage-tracker.md`

**Dependency:** `tdd-fresh-context-per-cycle` — provides the per-phase invocation loop and structured handoffs that this spec builds on. Tasks 1–6 below are independent of that spec; Tasks 7–9 wire into the cycle loop it introduces.

---

## Architecture

### Package Structure

```
internal/coverage/
├── parser.go           # Parse spec acceptance criteria into numbered checklist
├── parser_test.go
├── tracker.go          # Coverage state machine across TDD cycles
├── tracker_test.go
├── validator.go        # Self-report + haiku validation response parsing
└── validator_test.go
```

### Key Types

**Criterion** — A single parsed acceptance criterion:
```go
type Criterion struct {
    Number int
    Text   string
}
```

**CriterionState** — Per-criterion tracking:
```go
type CriterionState struct {
    Criterion
    Status         Status // Unchecked, Covered, Untestable
    RejectionCount int
}
```

**CoverageTracker** — Stateful checklist:
```go
type CoverageTracker struct {
    criteria       []CriterionState
    maxRejections  int // default: 2
}
```

**SelfReport** — Parsed from Claude's red phase output:
```go
type SelfReport struct {
    Targeting int   `json:"targeting"`
    Remaining []int `json:"remaining"`
}
```

**ValidationResponse** — Parsed from haiku's response:
```go
type ValidationResponse struct {
    Covers bool   `json:"covers"`
    Reason string `json:"reason"`
}
```

### Integration Points

1. **Criteria parsing** — Called once at TDD cycle start, reads spec content string
2. **Red phase prompt** — Tracker formats coverage state string for injection into handoff
3. **Red phase output** — Parse self-report JSON from Claude's output
4. **Green phase validation** — Invoke haiku with test code + criterion, parse response
5. **Cycle termination** — Tracker's `IsComplete()` replaces heuristic
6. **Bead reporting** — Uncovered/untestable criteria added to bead comment

### Data Flow

```
Spec content → ParseCriteria() → []Criterion
                                       ↓
                              NewTracker(criteria) → CoverageTracker
                                       ↓
                              FOR EACH TDD CYCLE:
                                tracker.FormatCoverageState() → prompt string
                                       ↓
                                Red phase: Claude writes test targeting criterion
                                       ↓
                                ParseSelfReport(output) → SelfReport
                                       ↓
                                Green phase: tests pass
                                       ↓
                                RenderCoverageValidation() → haiku prompt
                                       ↓
                                Haiku: {"covers": true/false, "reason": "..."}
                                       ↓
                                ParseValidationResponse() → ValidationResponse
                                       ↓
                                tracker.MarkCovered(n) or tracker.RecordRejection(n)
                                       ↓
                                tracker.IsComplete() → stop or continue
                                       ↓
                              END LOOP
                                       ↓
                              tracker.Summary() → bead comment
```

## Test Strategy

### Unit Tests (pure logic, no mocks)
- Criteria parsing: bullet extraction, numbering, edge cases
- Tracker state machine: all state transitions, termination conditions
- Self-report parsing: valid/invalid JSON
- Validation response parsing: covers true/false, malformed

### Unit Tests (with mocks)
- Haiku validation invocation: mock provider, verify prompt and response handling
- Cycle termination wiring: mock tracker, verify runner stops/continues correctly
- Disagreement handling: tracker not complete when self-report says done
- Bead comment generation: formatting of uncovered/untestable criteria

### Integration Tests
- Template rendering with real context
- Coverage state formatting matches expected prompt structure

### Mocking Strategy
- Mock `InvokeFn` for haiku calls
- Mock `ValidateDirectFn` for green phase execution
- Mock `BeadClient` for comment verification
- Real `CoverageTracker` everywhere — it's pure state

## Implementation Tasks

### Task 1: Criteria Parser

**Files:**
- Create: `internal/coverage/parser.go`
- Create: `internal/coverage/parser_test.go`

**What to Do:**
Implement `ParseCriteria(specContent string) ([]Criterion, error)` that extracts acceptance criteria from a spec's `## Acceptance Criteria` section. Find lines between `## Acceptance Criteria` and the next `##` header. Split on `- ` prefix (single indentation level). Assign sequential 1-based numbers. Return empty slice (not error) when no criteria section found. Define the `Criterion` struct with `Number int` and `Text string` fields.

**Acceptance Criteria:**
- Parses bullet-point criteria from a spec string into numbered `Criterion` structs
- Returns empty slice when no `## Acceptance Criteria` section exists
- Handles compound criteria (semicolons/commas) as single items without splitting

**Dependencies:** None

**Notes:** Pure string parsing — no regexp needed, just line scanning. The format is consistent across all specs (single-level bullets starting with `- `). Trim whitespace from extracted text.

### Task 2: Coverage Tracker

**Files:**
- Create: `internal/coverage/tracker.go`
- Create: `internal/coverage/tracker_test.go`

**What to Do:**
Implement `CoverageTracker` struct that manages a checklist of `CriterionState` entries. Each criterion starts as `Unchecked`. Provide methods: `MarkCovered(n int)` transitions to `Covered`, `RecordRejection(n int)` increments rejection count and transitions to `Untestable` at threshold (default 2), `NextUncovered() *Criterion` returns the lowest-numbered unchecked criterion (nil when none remain), `IsComplete() bool` returns true when all criteria are Covered or Untestable, `UncoveredCriteria() []CriterionState` returns remaining unchecked items, `UntestableCriteria() []CriterionState` returns items flagged untestable, `FormatCoverageState(targeting int) string` renders the coverage state block for prompt injection, `Summary() string` renders a human-readable summary for bead comments. Constructor: `NewTracker(criteria []Criterion, maxRejections int)`.

**Acceptance Criteria:**
- Tracks per-criterion state transitions (Unchecked → Covered, Unchecked → Untestable after 2 rejections)
- `IsComplete()` returns true when all criteria are covered or untestable
- `FormatCoverageState()` renders targeting + remaining in the format specified by the spec

**Dependencies:** Task 1 (uses `Criterion` type)

**Notes:** The `FormatCoverageState` output format must match what the red phase template expects:
```
## Coverage State
Targeting criterion #3: "criterion text"
Remaining uncovered: #3, #5, #7
```

### Task 3: Validation Types and Parsing

**Files:**
- Create: `internal/coverage/validator.go`
- Create: `internal/coverage/validator_test.go`

**What to Do:**
Define `SelfReport` struct (`Targeting int`, `Remaining []int`) and `ParseSelfReport(output string) (*SelfReport, error)` that scans Claude's red phase output for a JSON block matching `{"targeting": N, "remaining": [...]}`. Define `ValidationResponse` struct (`Covers bool`, `Reason string`) and `ParseValidationResponse(output string) (*ValidationResponse, error)` that parses haiku's JSON response. Both parsers should be lenient — scan for the JSON block within surrounding text, not require the entire output to be JSON.

**Acceptance Criteria:**
- `ParseSelfReport` extracts targeting/remaining from JSON embedded in Claude's output
- `ParseValidationResponse` extracts covers/reason from haiku's JSON response
- Both parsers return errors on malformed JSON without panicking

**Dependencies:** None

**Notes:** Use `strings.Index` to find `{"targeting"` or `{"covers"` markers, then extract and unmarshal. This handles the common case where Claude wraps JSON in explanation text.

### Task 4: Coverage Validation Template and Render Method

**Files:**
- Create: `.gromit/templates/PROMPT_coverage_validation.md`
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/interfaces.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Create `CoverageValidationContext` struct with `TestCode string`, `CriterionNumber int`, `CriterionText string` fields. Add `RenderCoverageValidation(ctx *CoverageValidationContext) (string, error)` to `Renderer` and `PromptRenderer` interface. Create the template that instructs haiku to evaluate whether the test actually verifies the criterion and respond with `{"covers": true/false, "reason": "one sentence"}`. Keep the template minimal — haiku prompt should be under 500 tokens plus the test code.

**Acceptance Criteria:**
- `CoverageValidationContext` struct defined with test code and criterion fields
- `RenderCoverageValidation` renders the haiku validation prompt from template
- Template instructs structured JSON response matching `ValidationResponse` schema

**Dependencies:** Task 3 (defines the response types the template requests)

**Notes:** Follow existing template patterns (see `PROMPT_scope.md` for a similar lightweight prompt). The template should be direct and specific — haiku works best with clear instructions and no ambiguity.

### Task 5: Coverage State Prompt Integration

**Files:**
- Modify: `internal/prompt/prompt.go` (add coverage fields to `Context`)
- Modify: `.gromit/templates/PROMPT_tdd_build.md` (add conditional coverage section)
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Add `CoverageState string` and `TargetCriterion string` fields to `prompt.Context`. In the TDD build template, add a conditional section after the task description that renders coverage state when present: `{{if .CoverageState}}## Coverage State\n{{.CoverageState}}{{end}}`. This allows the existing TDD build flow to include coverage information when available, and is a no-op when coverage tracking is not active.

**Acceptance Criteria:**
- `prompt.Context` has `CoverageState` and `TargetCriterion` fields
- TDD build template renders coverage state section when fields are populated
- Template renders identically to before when coverage fields are empty

**Dependencies:** Task 1 (coverage state format), Task 2 (FormatCoverageState output)

**Notes:** When `tdd-fresh-context-per-cycle` introduces per-phase red templates, the coverage section may move there. For now, adding it conditionally to the existing TDD build template is forward-compatible — the section simply won't render when coverage is not active.

### Task 6: Coverage Result Fields

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/runner.go` (writeIterationLog wiring)

**What to Do:**
Add to `IterationResult`: `CriteriaTotal int`, `CriteriaCovered int`, `CriteriaUntestable int`, `UncoveredCriteria []string`. Add corresponding fields to `IterationLog` with `json:",omitempty"` tags. Wire through `writeIterationLog`. Zero values are safe — no behavior change for non-TDD builds.

**Acceptance Criteria:**
- `IterationResult` has coverage tracking fields (total, covered, untestable, uncovered list)
- `IterationLog` includes coverage fields with omitempty for backward compatibility
- Fields are wired through `writeIterationLog`

**Dependencies:** None

**Notes:** Follow the additive field pattern used by existing fields like `EstimatedFiles`, `FailureLayer`, etc. All fields zero-valued by default.

### Task 7: Wire Haiku Validation Callback into Executor

**Files:**
- Modify: `internal/runner/methodology/executor.go`
- Test: `internal/runner/methodology/methodology_test.go`

**What to Do:**
Add `CoverageValidateFn` type: `func(ctx context.Context, testCode string, criterion Criterion) (*coverage.ValidationResponse, error)`. Add `coverageValidateFn` field and `SetCoverageValidateFn` setter to `Executor`. Implement `ValidateCoverage(ctx context.Context, bc *runtypes.BeadContext, testCode string, criterion coverage.Criterion) (*coverage.ValidationResponse, error)` that calls `coverageValidateFn`. This is the executor-side plumbing — the runner will construct the actual callback that renders the prompt and invokes haiku.

**Acceptance Criteria:**
- Executor accepts a coverage validation callback via setter
- `ValidateCoverage` delegates to the callback and returns the parsed response
- Tests verify callback invocation with correct arguments and response propagation

**Dependencies:** Task 3 (uses `Criterion`, `ValidationResponse` types)

**Notes:** Follow the existing pattern of `SetRefactorDeps` / `SetAnalyzeFn` for dependency injection. The actual callback construction happens in Task 8 at the runner level.

### Task 8: Wire Coverage Tracker into TDD Cycle Loop

**Files:**
- Modify: `internal/runner/process_methodology.go`
- Modify: `internal/runner/callbacks.go` (construct haiku validation callback)
- Test: `internal/runner/process_methodology_test.go` or `internal/runner/process_test.go`

**What to Do:**
In the TDD cycle preparation path (`prepareMethodologyForBead` or the cycle loop from `tdd-fresh-context-per-cycle`):
1. When TDD is active and bead has a spec label, parse criteria from spec via `coverage.ParseCriteria`
2. Create `coverage.NewTracker(criteria, 2)` and store on BeadContext or runner state
3. Before each red phase: call `tracker.FormatCoverageState()`, inject into prompt context
4. After red phase: call `coverage.ParseSelfReport()` on Claude's output
5. After green phase passes: construct haiku validation callback (render prompt via `RenderCoverageValidation`, invoke via router at `TierLow`, parse response), call `executor.ValidateCoverage()`
6. Based on validation: call `tracker.MarkCovered()` or `tracker.RecordRejection()`
7. Check `tracker.IsComplete()` for cycle termination
8. When Claude self-reports "nothing remaining" but tracker disagrees: inject another cycle with explicit target

**Acceptance Criteria:**
- Coverage tracker initialized from spec at TDD cycle start
- Cycle termination uses tracker state, not Claude's self-report alone
- Disagreement handling injects additional cycles when tracker has unchecked items

**Dependencies:** Tasks 1, 2, 3, 4, 5, 7; also depends on `tdd-fresh-context-per-cycle` for the cycle loop structure

**Notes:** This is the main integration task. The cycle loop structure from `tdd-fresh-context-per-cycle` determines exactly where these hooks go. If that spec introduces a `CycleState` struct for handoffs, the coverage tracker should integrate with it. If the cycle loop doesn't exist yet, this task should be deferred until it does.

### Task 9: Bead Result Reporting

**Files:**
- Modify: `internal/runner/process_methodology.go` or `internal/runner/run_iteration.go`
- Test: corresponding test file

**What to Do:**
After the TDD cycle loop completes (whether by coverage completion, max cycles, or failure):
1. Populate `bc.Result.CriteriaTotal`, `CriteriaCovered`, `CriteriaUntestable`, `UncoveredCriteria` from tracker state
2. If there are uncovered or untestable criteria, add a bead comment via `r.beads.AddComment()` with `tracker.Summary()` describing what was covered, what wasn't, and what was flagged untestable
3. Log coverage summary to runner output

**Acceptance Criteria:**
- IterationResult populated with final coverage state after cycle loop
- Bead comment added when criteria are uncovered or untestable
- Coverage summary logged to runner output

**Dependencies:** Tasks 2, 6, 8

**Notes:** Follow the existing bead comment pattern from stuck bead handling in `run_iteration.go`. Comments are best-effort (log warning on failure, don't fail the build).

---

## Notes

- **Dependency ordering**: Tasks 1–6 can be implemented and tested independently of `tdd-fresh-context-per-cycle`. Tasks 7–9 integrate with the cycle loop that spec introduces. Decompose should respect this ordering.
- **Backward compatibility**: All new fields use zero values / omitempty. Non-TDD builds are completely unaffected.
- **Spec format assumption**: The parser assumes `## Acceptance Criteria` with `- ` bullets at a single indentation level. This matches all current specs. If spec format changes, only `parser.go` needs updating.
- **Max rejections**: Hardcoded default of 2 matches the spec. Exposed as a constructor parameter for testability but not surfaced in config (YAGNI).
- **JSON parsing leniency**: Both `ParseSelfReport` and `ParseValidationResponse` scan for JSON within surrounding text. Claude and haiku often wrap structured output in explanation. Strict parsing would cause false failures.
