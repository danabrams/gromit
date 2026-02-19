---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T19:42:48Z"
id: prompt-token-accounting-diagnostics
source_spec: prompt-token-accounting-diagnostics
---

# Prompt Token Accounting Diagnostics Implementation Plan

**Goal:** Add per-invocation, per-section token estimation and budget diagnostics to every prompt-producing workflow so Gromit can systematically identify and reduce total token consumption.

**Architecture:** A token estimation layer captures per-section diagnostics on every prompt invocation (runtime, retro, pipeline), persists them as additive JSONL fields, reconciles estimates against provider-reported tokens, and surfaces optimization-focused summaries in `gromit stats`.

**Tech Stack:** Go, text/template, JSONL logging

**Spec:** `.gromit/specs/prompt-token-accounting-diagnostics.md`

---

## Architecture

### Token Estimator
`internal/prompt/tokencount.go` provides `EstimateTokens(text string) int` using a chars/4 heuristic. Deterministic for trend comparison; reconciliation data enables future calibration.

### PromptDiagnostics Struct
`internal/prompt/diagnostics.go` defines:
```go
type PromptDiagnostics struct {
    PromptType      string         `json:"prompt_type"`
    EstimatedTokens int            `json:"estimated_tokens"`
    SectionTokens   map[string]int `json:"section_tokens"`
    BudgetMaxChars  int            `json:"budget_max_chars,omitempty"`
    ShapeActions    []string       `json:"shape_actions,omitempty"`
    PreShapeTokens  int            `json:"pre_shape_tokens,omitempty"`
    PostShapeTokens int            `json:"post_shape_tokens,omitempty"`
    ReportedTokens  int            `json:"reported_tokens,omitempty"`
    TokenDelta      int            `json:"token_delta,omitempty"`
    TokenDeltaPct   float64        `json:"token_delta_pct,omitempty"`
}
```

### Section Taxonomy
Stable identifiers across prompt types:
- `rules`, `claude_md`, `spec`, `confirmed_learnings`, `recent_learnings`
- `task_identity`, `diff`, `failure_context`, `template_static`
- `skill_instructions`, `plan_body`, `run_stats`, `bead_stats`
- Prompt-specific sections use descriptive keys

### Renderer Instrumentation
Side-effect storage: `lastDiagnostics *PromptDiagnostics` field on Renderer. Each `Render*()` method computes and stores diagnostics. `LastDiagnostics()` method added to both Renderer and PromptRenderer interface. Non-breaking for existing callers.

### Pipeline/Retro Instrumentation
Pipeline builders (decompose, explore, review) and retro rendering compute diagnostics by measuring component strings before concatenation/rendering. Same `PromptDiagnostics` struct.

### Logging
`PromptDiagnostics` added to `IterationResult` and `IterationLog` with `json:"prompt_diagnostics,omitempty"`. Backward-compatible: old readers ignore the new field.

### Reconciliation
After provider result returns `InputTokens`, compute delta (estimated - reported) and percentage. Store on diagnostics struct before logging.

### Metrics and Stats
`process_trend.go` aggregates per-type and per-section token data. `gromit stats --prompt-tokens` surfaces top token-consuming prompt types, sections, budget reduction metrics, and reconciliation drift.

### Data Flow
```
Context fields → EstimateTokens() per section → PromptDiagnostics
  → (optional) budget shaping → pre/post token estimates
  → Render*() → stores on Renderer
  → callbacks.go → LastDiagnostics() → bc.Result.PromptDiagnostics
  → provider result → reconciliation delta
  → writeIterationLog() → JSONL
  → BuildContinuousMetrics() → process_trend.json
  → gromit stats --prompt-tokens → display
```

## Test Strategy

**Unit Tests:**
- Token estimator: determinism, edge cases (empty, Unicode), calibration samples
- PromptDiagnostics: JSON round-trip, reconciliation computation, section taxonomy uniqueness
- Renderer diagnostics: each Render* produces correct PromptType and section keys
- Budget shaping: pre/post tokens present when shaping active, absent when not

**Integration Tests:**
- Callback wiring: bc.Result.PromptDiagnostics populated after invocation
- Logging: JSONL contains prompt_diagnostics field with correct structure
- Pipeline/retro: diagnostics emitted with expected sections
- Stats: text and JSON output include prompt token summaries
- Metrics: process_trend aggregates diagnostics across iterations

**Backward Compatibility:**
- Old JSONL without prompt_diagnostics deserializes cleanly
- Full `go test ./...` passes with no regressions

**Mocking:** Mock PromptRenderer.LastDiagnostics() in callback tests. Use t.TempDir() with real JSONL for logger/stats tests. Existing template fixtures for renderer tests.

## Implementation Tasks

### Task 1: Add token estimation utility

**Files:**
- Create: `internal/prompt/tokencount.go`
- Test: `internal/prompt/tokencount_test.go`

**What to Do:**
Implement `EstimateTokens(text string) int` that returns an estimated token count for a text string. Use chars/4 heuristic (length of UTF-8 byte representation divided by 4, rounded up). Export for use across packages. Add `EstimateSectionTokens(sections map[string]string) map[string]int` convenience function that applies EstimateTokens to each value.

**Acceptance Criteria:**
- EstimateTokens returns 0 for empty string, positive int for non-empty text
- EstimateTokens is deterministic: same input always produces same output
- EstimateSectionTokens returns a map with the same keys as input, each value being the EstimateTokens result for that section's text

**Dependencies:** None

### Task 2: Add PromptDiagnostics types and section taxonomy

**Files:**
- Create: `internal/prompt/diagnostics.go`
- Test: `internal/prompt/diagnostics_test.go`

**What to Do:**
Define the `PromptDiagnostics` struct with JSON tags. Define section taxonomy constants (SectionRules, SectionClaudeMD, SectionSpec, SectionConfirmedLearnings, SectionRecentLearnings, SectionTaskIdentity, SectionDiff, SectionFailureContext, SectionTemplateStatic, SectionSkillInstructions, SectionPlanBody, SectionRunStats, SectionBeadStats). Add `Reconcile(reportedTokens int)` method that computes TokenDelta and TokenDeltaPct. Add `NewDiagnostics(promptType string, sectionTokens map[string]int) *PromptDiagnostics` constructor that sums section tokens into EstimatedTokens.

**Acceptance Criteria:**
- PromptDiagnostics JSON round-trips cleanly with all fields
- NewDiagnostics computes EstimatedTokens as sum of all section token values
- Reconcile sets TokenDelta (estimated - reported) and TokenDeltaPct correctly, handling zero reported gracefully

**Dependencies:** Task 1 (uses EstimateTokens)

### Task 3: Wire diagnostics into Renderer for runtime prompts

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/interfaces.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Add `lastDiagnostics *PromptDiagnostics` field to Renderer. Add `LastDiagnostics() *PromptDiagnostics` method. Add `computeBuildDiagnostics(ctx *Context, promptType string) *PromptDiagnostics` helper that extracts section strings from the context struct (ClaudeMD, Rules, Spec, ConfirmedLearnings content, RecentLearnings content, bead identity, FailureContext) and calls EstimateSectionTokens + NewDiagnostics. Wire into every Render* method: compute diagnostics from the context, store in lastDiagnostics, then proceed with existing rendering. For specialized contexts (ReviewContext, ThoroughReviewContext, etc.), add prompt-type-specific helpers that extract the relevant sections. Add `LastDiagnostics()` to the `PromptRenderer` interface in interfaces.go. Update mock implementations to return nil.

**Acceptance Criteria:**
- After calling any Render* method, LastDiagnostics() returns non-nil with correct PromptType and non-zero EstimatedTokens
- SectionTokens map contains expected keys for the prompt type (e.g., RenderBuild includes rules, spec, task_identity; RenderReview includes diff)
- Adding LastDiagnostics to PromptRenderer interface compiles with all existing implementations

**Dependencies:** Task 1, Task 2

**Notes:** This is the largest task. The key insight is that each Render* method already receives a typed context struct whose fields map directly to sections. Compute diagnostics from the context *before* template rendering (section measurement is on inputs, not the rendered output). Store the final rendered prompt's token estimate as EstimatedTokens on the diagnostics as well, but keep section-level attribution from inputs.

### Task 4: Extend budget shaping with token estimates

**Files:**
- Modify: `internal/prompt/budget.go`
- Test: `internal/prompt/budget_test.go`

**What to Do:**
Add `PreShapeTokens int` and `PostShapeTokens int` fields to `ShapeReport`. In `ShapeContextForBudget`, `ShapeReviewContextForBudget`, and `ShapeThoroughReviewContextForBudget`, compute token estimates from the before-shape context and after-shape context using `EstimateTokens(measureContext(...))`. Store on ShapeReport. In the Render* methods that call shaping (RenderBuild, RenderReview, RenderThoroughReview), copy ShapeReport's PreShapeTokens, PostShapeTokens, and TrimActions into the PromptDiagnostics. Also handle ShapeRetroForBudget for retro path.

**Acceptance Criteria:**
- ShapeReport includes PreShapeTokens and PostShapeTokens after shaping
- When budget shaping trims content, PostShapeTokens < PreShapeTokens
- PromptDiagnostics for shaped prompts includes PreShapeTokens, PostShapeTokens, ShapeActions, and BudgetMaxChars

**Dependencies:** Task 1, Task 2, Task 3

### Task 5: Add PromptDiagnostics to IterationResult and IterationLog

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/logger/logger.go`
- Test: `internal/logger/logger_test.go`

**What to Do:**
Add `PromptDiagnostics *prompt.PromptDiagnostics` field to IterationResult in runtypes/types.go. Add `PromptDiagnostics *prompt.PromptDiagnostics` with JSON tag `json:"prompt_diagnostics,omitempty"` to IterationLog in logger.go. Test JSONL serialization: verify the field is present when set and absent when nil. Test deserialization of old entries without the field (should deserialize cleanly with nil).

**Acceptance Criteria:**
- IterationResult and IterationLog define PromptDiagnostics fields
- JSONL serialization includes prompt_diagnostics when non-nil and omits it when nil
- Deserializing old JSONL entries without prompt_diagnostics produces nil field without error

**Dependencies:** Task 2

### Task 6: Wire diagnostics through runner callbacks to logging with reconciliation

**Files:**
- Modify: `internal/runner/callbacks.go`
- Modify: `internal/runner/logging.go`
- Test: `internal/runner/callbacks_test.go`, `internal/runner/logging_test.go`

**What to Do:**
In `makeInvokeFn()` in callbacks.go, after the render call that builds the prompt, call `r.renderer.LastDiagnostics()` and store result on `bc.Result.PromptDiagnostics`. After the provider returns results with InputTokens, call `bc.Result.PromptDiagnostics.Reconcile(bc.Result.InputTokens)` to compute the estimated-vs-reported delta. In `writeIterationLog()` in logging.go, map `result.PromptDiagnostics` to `log.PromptDiagnostics`. Wire the same pattern for ATDD and methodology invocation paths in callbacks.go.

**Acceptance Criteria:**
- After a build invocation, bc.Result.PromptDiagnostics is populated with diagnostics from the renderer
- When provider reports InputTokens > 0, reconciliation fields (ReportedTokens, TokenDelta, TokenDeltaPct) are set on diagnostics
- writeIterationLog produces a JSONL entry with prompt_diagnostics containing the expected structure

**Dependencies:** Task 3, Task 5

### Task 7: Instrument pipeline decompose prompts

**Files:**
- Modify: `internal/pipeline/decompose.go`
- Test: `internal/pipeline/decompose_test.go`

**What to Do:**
In `buildDecomposePrompt()`, compute diagnostics by measuring each component (plan body, skill instructions, static template text) using `EstimateTokens`, build a `PromptDiagnostics` with PromptType "decompose" and section tokens for `plan_body`, `skill_instructions`, `template_static`. Return diagnostics alongside the prompt string (either as additional return value or store on the Pipeline struct). Log diagnostics when the decompose result comes back from the provider.

**Acceptance Criteria:**
- Decompose prompt construction produces PromptDiagnostics with PromptType "decompose"
- SectionTokens includes plan_body and skill_instructions keys with non-zero values
- Diagnostics are logged or made available for aggregation

**Dependencies:** Task 1, Task 2

**Notes:** Pipeline prompts don't go through the runner's iteration logging. Either add a lightweight log call via the LogWriter interface, or store diagnostics on the pipeline result for the CLI layer to log.

### Task 8: Instrument pipeline explore and review prompts

**Files:**
- Modify: `cmd/gromit/explore.go`
- Modify: `cmd/gromit/review.go`
- Test: corresponding test files

**What to Do:**
In the explore prompt builder (`explorePromptRenderer.RenderExplore`), measure each section (topic, claude_md, rules, learnings, instructions) using EstimateTokens and construct PromptDiagnostics with PromptType "explore". In the review non-interactive path (which uses `RenderThoroughReview` via adapter), the diagnostics are already captured by Task 3's Renderer instrumentation — verify they flow through. Log diagnostics for explore similar to Task 7.

**Acceptance Criteria:**
- Explore prompt construction produces PromptDiagnostics with PromptType "explore" and section tokens for claude_md, rules, confirmed_learnings
- Non-interactive review diagnostics flow through from Renderer.LastDiagnostics()
- At least one test verifies explore diagnostics contain expected sections

**Dependencies:** Task 3, Task 7

### Task 9: Instrument retro prompts

**Files:**
- Modify: `internal/retro/retro.go`
- Test: `internal/retro/retro_test.go`

**What to Do:**
In `renderPrompt()`, after assembling the template context but before executing the template, compute diagnostics by measuring each context field (rules, learnings, run_stats, bead_stats, efficiency, process_trend) using EstimateTokens. Build PromptDiagnostics with PromptType "retro". If budget shaping was applied (via ShapeRetroForBudget), include pre/post token estimates and shape actions. Store diagnostics on the Retro struct for logging.

**Acceptance Criteria:**
- Retro prompt rendering produces PromptDiagnostics with PromptType "retro"
- SectionTokens includes rules, confirmed_learnings, run_stats, bead_stats keys
- When retro budget shaping is active, PreShapeTokens and PostShapeTokens are populated

**Dependencies:** Task 1, Task 2

### Task 10: Aggregate prompt token metrics in process_trend

**Files:**
- Modify: `internal/logger/process_trend.go`
- Test: `internal/logger/process_trend_test.go`

**What to Do:**
Add `PromptTokenSummary` struct to process_trend.go with fields: ByPromptType (map of prompt type to average estimated tokens and invocation count), BySectionTop10 (top 10 sections by total estimated tokens across all invocations), BudgetActionFrequency (map of action string to count), ReconciliationDrift (mean and p95 of absolute TokenDeltaPct across invocations with reconciliation data). Add PromptTokenSummary field to ProcessTrend struct. In `buildProcessTrend()`, iterate over iteration metrics that have PromptDiagnostics, aggregate into the summary. Add `prompt_diagnostics` field to IterationMetric populated from IterationLog.

**Acceptance Criteria:**
- ProcessTrend includes PromptTokenSummary with per-type averages and section-level totals
- BudgetActionFrequency counts shape actions from diagnostics across the window
- ReconciliationDrift computed from entries that have non-zero ReportedTokens

**Dependencies:** Task 5, Task 6

### Task 11: Add prompt-token stats to gromit stats

**Files:**
- Modify: `cmd/gromit/stats.go`
- Test: `cmd/gromit/stats_test.go`

**What to Do:**
Add `--prompt-tokens` bool flag to the stats cobra command. When set, read PromptTokenSummary from process_trend.json and display: (1) top prompt types by estimated input tokens (type, avg tokens, count), (2) top sections by total tokens, (3) budget action frequency and reduction percentages, (4) reconciliation drift summary (mean delta %, p95 delta %). In JSON output mode, include a `prompt_token_diagnostics` key with the structured PromptTokenSummary. Handle gracefully when no diagnostics data exists (print "No prompt token diagnostics available").

**Acceptance Criteria:**
- `gromit stats --prompt-tokens` displays prompt type rankings, top sections, budget actions, and reconciliation drift
- JSON output includes prompt_token_diagnostics key when --prompt-tokens and --json are both set
- Graceful handling when process_trend.json has no PromptTokenSummary (empty state)

**Dependencies:** Task 10

### Task 12: Final verification and backward compatibility

**Files:**
- No new files

**What to Do:**
Run `go test ./...`, `go vet ./...`, and `go build ./...` to confirm all quality gates pass. Verify that existing JSONL files without prompt_diagnostics deserialize cleanly. Verify prompt rendering produces identical output content (same rendered strings) with diagnostics enabled. Run `gromit stats` (without --prompt-tokens) and confirm output is unchanged.

**Acceptance Criteria:**
- All tests pass, no compilation errors, no vet warnings
- Existing JSONL files parse without errors
- Prompt rendering output is byte-identical with and without diagnostics capture

**Dependencies:** All previous tasks

---

## Notes

- **Thread safety**: The `lastDiagnostics` field on Renderer is safe because the Renderer is used sequentially within a single runner iteration. If this changes in the future, add a mutex.
- **Token estimation calibration**: The chars/4 heuristic is a starting point. Once reconciliation data accumulates, consider adjusting the ratio. The reconciliation drift metrics in stats will show when calibration is needed.
- **Pipeline logging gap**: Pipeline commands (decompose, explore, plan, refine) don't use the runner's iteration logging. Diagnostics for these flows need either a dedicated log call or storage on the pipeline result. The simplest approach is to emit a one-line JSONL entry via the existing LogWriter interface.
- **Retro is template-based**: Unlike pipeline string-concatenation prompts, retro uses Go templates. Measure sections from the template context *before* template execution, since the template adds formatting/structure.
- **Section taxonomy is extensible**: New prompt types can add custom section keys beyond the defined constants. The taxonomy constants are for cross-prompt aggregation; prompt-specific keys are fine for detailed analysis.
