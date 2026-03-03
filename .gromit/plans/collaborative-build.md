---
created: 2026-03-03T00:00:00Z
decomposed: true
decomposed_at: "2026-03-03T07:10:27-05:00"
id: collaborative-build
source_spec: collaborative-build
---

# Collaborative Build Implementation Plan

**Goal:** Insert a lightweight, non-blocking haiku-tier review step between Build and Validate that catches spec misalignment early, optionally triggering a single fix-build before validation runs.

**Architecture:** A new `midreview` pipeline stage examines the builder's diff against the spec and acceptance criteria. The orchestrator calls it after Build succeeds, branches on findings (zero → proceed to Validate; non-zero → one fix-build then Validate), and logs metrics to IterationLog.

**Tech Stack:** Go, existing pipeline/prompt/config/logger packages

**Spec:** `.gromit/specs/collaborative-build.md`

---

## Architecture

**Overview:**
A new `midreview` package under `internal/runner/pipeline/` implements the `pipeline.Stage` interface. After Build succeeds, the orchestrator invokes the mid-review stage with the code diff, spec, and acceptance criteria. If findings are returned, the orchestrator re-invokes Build once with findings as context (fix-build). The pipeline then proceeds to Validate regardless of whether a fix-build ran.

**Key Components:**

1. **`internal/runner/pipeline/midreview/` package**: Pipeline stage that invokes a haiku-tier LLM to review the build diff against the spec. Returns structured findings. Handles its own error recovery (LLM failures → zero findings).

2. **`internal/prompt/` extensions**: New `MidBuildReviewContext` type and `RenderMidBuildReview()` method on `Renderer`. Uses a new `PROMPT_midreview.md` template.

3. **`internal/config/config_types.go` extension**: New `MidBuildReviewConfig` struct added to the root `Config`. Fields: `Enabled` (bool), `Tier` (string, default "haiku"), `Timeout` (int, seconds).

4. **`internal/logger/logger.go` extension**: New fields on `IterationLog` for mid-review metrics.

5. **`internal/pipeline/stage.go` extension**: New `MidBuildReviewFindings` field on `Input` for fix-build context.

6. **`internal/runner/orchestrator.go` integration**: After Build success, call mid-review stage. If findings > 0, call Build again with findings on Input. Proceed to Validate.

**Integration Points:**
- Orchestrator: between Build success (line ~629) and Validate entry (line ~659)
- Input struct: new `MidBuildReviewFindings` field (parallel to existing `ValidationFailures`)
- Config: new `MidBuildReview` field on `Config` struct
- IterationLog: new mid-review metric fields
- Prompt: new render method and template

**Data Flow:**
```
Build.Run() → success
  → git diff (iteration start → HEAD)
  → RenderMidBuildReview(diff, spec, acceptance criteria)
  → haiku LLM invocation
  → parse response → []Finding
  → if len(findings) > 0:
      → set Input.MidBuildReviewFindings = findings
      → Build.Run() again (fix-build, single pass)
  → proceed to Validate.Run()
```

**Files to Modify:**
- `internal/pipeline/stage.go` — Add `MidBuildReviewFindings` field to `Input`
- `internal/config/config_types.go` — Add `MidBuildReviewConfig` struct and field on `Config`
- `internal/logger/logger.go` — Add mid-review fields to `IterationLog`
- `internal/prompt/context_types.go` — Add `MidBuildReviewContext` type
- `internal/prompt/render_methods.go` — Add `RenderMidBuildReview()` method
- `internal/runner/orchestrator.go` — Insert mid-review + fix-build between Build and Validate

**Files to Create:**
- `internal/runner/pipeline/midreview/midreview.go` — Stage implementation
- `internal/runner/pipeline/midreview/midreview_test.go` — Unit tests
- `internal/runner/pipeline/midreview/result.go` — Finding and result types
- `internal/prompt/templates/PROMPT_midreview.md` — Review prompt template

**Tradeoffs:**
- **Dedicated package vs. inline**: Chose a `midreview` package following the pipeline stage pattern (`execute/`, `validate/`, `review/`). Keeps the orchestrator thin.
- **Findings on Input vs. separate struct**: Chose `MidBuildReviewFindings []string` on `Input` (same shape as `ValidationFailures`) for uniformity. The Build stage already knows how to incorporate contextual feedback from Input fields.
- **Configurable tier with haiku default**: Follows the existing ReviewConfig pattern where tier is configurable but has a sensible default.
- **Orchestrator owns branching**: The mid-review stage returns findings; the orchestrator decides whether to re-invoke Build. This matches how validation retry branching works.

---

## Test Strategy

**Unit Tests:**
- `midreview/midreview_test.go`: Zero findings path, findings-present path, LLM failure graceful degradation, timeout handling, result parsing (valid and malformed)
- `prompt/midreview_test.go` (or additions to `prompt_test.go`): `RenderMidBuildReview()` includes diff/spec/criteria, handles empty diff, diagnostics populated
- `config/config_test.go` additions: `MidBuildReviewConfig` defaults, YAML deserialization with and without the section
- `logger/logger_test.go` additions: Mid-review fields serialize/omit correctly in JSONL

**Integration Tests:**
- Build + zero findings → Validate (no fix-build)
- Build + findings → fix-build once → Validate
- Build + review failure → Validate (graceful skip)
- Fix-build runs exactly once (no loop)
- Mid-review metrics appear in IterationLog

**Mocking Strategy:**
- Mock LLM provider/invoker (canned responses) — same pattern as existing build/review tests
- Mock git diff retrieval (canned diffs)
- Real config parsing, prompt rendering, result parsing (deterministic)

**Test Organization:**
- `internal/runner/pipeline/midreview/midreview_test.go` — stage unit tests
- `internal/runner/pipeline/midreview/result_test.go` — result parsing tests
- Existing test files extended for config/logger/prompt changes

---

## Implementation Tasks

### Task 1: Define finding/result types and config

**Files:**
- Create: `internal/runner/pipeline/midreview/result.go`
- Modify: `internal/config/config_types.go`
- Test: `internal/runner/pipeline/midreview/result_test.go`

**What to Do:**
Define the `Finding` struct (File, Line, Description, Severity string fields) and `MidBuildReviewResult` struct (Findings []Finding). Add a `ParseMidBuildReviewResult(raw string) MidBuildReviewResult` function that extracts findings from LLM output (expect a simple structured format — JSON array or markdown list). Handle malformed input by returning zero findings.

Add `MidBuildReviewConfig` to `config_types.go` with fields: `Enabled bool`, `Tier string`, `Timeout int`. Add `MidBuildReview MidBuildReviewConfig` field to the root `Config` struct with yaml tag `mid_build_review`. Include a `NormalizeNilFields()` method on `MidBuildReviewResult`.

**Acceptance Criteria:**
- `Finding` and `MidBuildReviewResult` types exist with proper JSON tags
- `ParseMidBuildReviewResult` returns correct findings for valid input and zero findings for malformed input
- `MidBuildReviewConfig` deserializes from YAML with correct defaults

**Dependencies:** None (foundational types)

### Task 2: Add mid-review prompt rendering

**Files:**
- Modify: `internal/prompt/context_types.go`
- Modify: `internal/prompt/render_methods.go`
- Create: `internal/prompt/templates/PROMPT_midreview.md`
- Test: additions to `internal/prompt/prompt_test.go`

**What to Do:**
Define `MidBuildReviewContext` in `context_types.go` with fields: `Diff string`, `Spec string`, `AcceptanceCriteria string`, `BeadTitle string`, `BeadDescription string`. Add `normalizeNilFields()` method (convention).

Add `RenderMidBuildReview(ctx *MidBuildReviewContext) (string, error)` to `render_methods.go`. Follow the pattern of `RenderReview()` — normalize nil fields, compute diagnostics, render template.

Create `PROMPT_midreview.md` template that instructs the LLM to review the diff against the spec and acceptance criteria, outputting findings in a parseable format (JSON array of {file, line, description, severity}).

**Acceptance Criteria:**
- `RenderMidBuildReview()` produces a prompt containing the diff, spec, and acceptance criteria
- Template instructs LLM to output structured findings
- Diagnostics are populated after rendering

**Dependencies:** Task 1 (uses Finding type in template output format)

### Task 3: Extend Input and IterationLog for mid-review data

**Files:**
- Modify: `internal/pipeline/stage.go`
- Modify: `internal/logger/logger.go`
- Test: additions to existing test files

**What to Do:**
Add `MidBuildReviewFindings []string` to `pipeline.Input`. This carries formatted finding descriptions into the fix-build, parallel to how `ValidationFailures` works.

Add these fields to `logger.IterationLog`:
- `MidBuildReviewRan bool` (`json:"mid_build_review_ran,omitempty"`)
- `MidBuildReviewFindings int` (`json:"mid_build_review_findings,omitempty"`)
- `MidBuildReviewFixBuild bool` (`json:"mid_build_review_fix_build,omitempty"`)
- `MidBuildReviewDurationMs int64` (`json:"mid_build_review_duration_ms,omitempty"`)
- `MidBuildReviewCostUSD float64` (`json:"mid_build_review_cost_usd,omitempty"`)
- `MidBuildReviewInputTokens int` (`json:"mid_build_review_input_tokens,omitempty"`)
- `MidBuildReviewOutputTokens int` (`json:"mid_build_review_output_tokens,omitempty"`)
- `FixBuildDurationMs int64` (`json:"fix_build_duration_ms,omitempty"`)
- `FixBuildCostUSD float64` (`json:"fix_build_cost_usd,omitempty"`)
- `FixBuildInputTokens int` (`json:"fix_build_input_tokens,omitempty"`)
- `FixBuildOutputTokens int` (`json:"fix_build_output_tokens,omitempty"`)

**Acceptance Criteria:**
- `Input.MidBuildReviewFindings` field exists and can be populated
- `IterationLog` mid-review fields serialize to JSONL correctly
- Fields omit cleanly when review didn't run (omitempty)

**Dependencies:** None (type extensions only)

### Task 4: Implement mid-review pipeline stage

**Files:**
- Create: `internal/runner/pipeline/midreview/midreview.go`
- Test: `internal/runner/pipeline/midreview/midreview_test.go`

**What to Do:**
Implement the `Stage` struct with dependencies: an LLM invoker interface (or provider), a prompt renderer, and a git diff function. Implement `Run(ctx, Input) (Output, error)`:

1. Check `Config.MidBuildReview.Enabled` — if false, return `Output{Decision: Proceed}` immediately.
2. Get the git diff from iteration start commit to HEAD.
3. Build `MidBuildReviewContext` from the diff, bead spec, and acceptance criteria.
4. Call `Renderer.RenderMidBuildReview()` to get the prompt.
5. Invoke the LLM at the configured tier (default haiku) with a timeout.
6. Parse the response with `ParseMidBuildReviewResult()`.
7. Return `Output{Decision: Proceed}` with findings accessible for the orchestrator.

On any error (LLM failure, timeout, diff error): log the error, return `Output{Decision: Proceed}` with zero findings. Never return an error from `Run()`.

Store findings on a custom output field or use a convention the orchestrator can read (e.g., attach to `Output` via a new field, or return via a channel/callback — follow whichever pattern is cleanest given `Output` extensibility).

**Acceptance Criteria:**
- Stage returns Proceed with findings when review succeeds
- Stage returns Proceed with zero findings when LLM fails or times out
- Stage skips entirely when `Config.MidBuildReview.Enabled` is false
- Unit tests cover all three branches (zero findings, findings present, failure)

**Dependencies:** Task 1 (result types), Task 2 (prompt rendering), Task 3 (Input/Output types)

### Task 5: Wire mid-review into orchestrator

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Test: integration tests (new file or additions to existing orchestrator tests)

**What to Do:**
In the orchestrator, after Build succeeds (around line 629) and before Validate entry (around line 659):

1. If `o.cfg.MidReview != nil` (the mid-review stage is wired):
   a. Call `o.cfg.MidReview.Run(ctx, baseIn)`.
   b. Extract findings from the output.
   c. Populate `baseIn.Result` mid-review metric fields (ran, findings count, duration, cost, tokens).
   d. If findings > 0:
      - Set `baseIn.MidBuildReviewFindings` to formatted finding strings.
      - Call `o.cfg.Build.Run(ctx, baseIn)` for the fix-build.
      - Populate `baseIn.Result` fix-build metric fields.
      - Set `baseIn.MidBuildReviewFindings = nil` after fix-build (don't carry into Validate).
2. Proceed to Validate as before.

Add `MidReview pipeline.Stage` field to `OrchestratorConfig`. Wire it in the orchestrator constructor or wherever stages are assembled.

**Acceptance Criteria:**
- Mid-review runs between Build and Validate when wired
- Zero findings → no fix-build invoked, proceeds to Validate
- Findings present → fix-build invoked once, then Validate
- Mid-review failure → logged, proceeds to Validate
- IterationLog contains mid-review metrics after iteration completes

**Dependencies:** Task 3 (Input fields), Task 4 (mid-review stage)

**Notes:**
- The fix-build is just a normal Build.Run() call with `MidBuildReviewFindings` populated on Input. The Build stage needs to incorporate these findings in its prompt (similar to how it uses `ValidationFailures`). This may require a small addition to the build prompt rendering to include mid-review findings when present.

---

## Notes

- The Build stage's prompt rendering will need to be aware of `Input.MidBuildReviewFindings` to include them in the fix-build prompt. This is a small change in the build prompt template/rendering, not a separate task — handle it as part of Task 5 or as a sub-task of Task 4.
- The `PROMPT_midreview.md` template should instruct the LLM to output findings in a JSON array format for reliable parsing. Include a fallback parser for markdown-list format.
- The mid-review stage needs access to git operations for the diff. Follow the same pattern used by the existing reviewer (`reviewpkg/reviewer.go`) for getting diffs.
- Consider adding events (`MidBuildReviewStartEvent`, `MidBuildReviewCompleteEvent`) to the event system for observability, following the pattern of `ReviewStartEvent`/`ReviewCompleteEvent`.
