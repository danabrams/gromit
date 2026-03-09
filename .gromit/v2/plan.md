---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-09
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

**Goal:** Add a holistic post-bead-loop spec-level code review stage, replace plan re-decomposition with findings-based targeted remediation, and add a `--from-review` execution mode for deferred improvement beads.

**Architecture:** New `internal/v2/stage/specreview/` package implements the spec-level review stage following the existing `Stage` interface; accept stage gains structured `Finding` output; remediation runner receives findings instead of gap text; decompose stage gains a findings-based prompt template; spec loop orchestrates the combined verdict; run2 gains `--from-review` flag.

**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

**New package:** `internal/v2/stage/specreview/` — implements the `Stage` interface. Takes cumulative diff (DiffFromBase), original plan, and project context. Invokes highest-tier model with structured review prompt. Parses JSON `{ verdict, findings[] }` response. Returns `SpecReviewArtifacts` with verdict and typed findings.

**Shared Finding type:** `internal/v2/stage/finding/` (or within specreview) — `Finding{Severity, Category, Scope, Description, AffectedFiles}` shared by accept and specreview so both can contribute to the combined findings list passed to remediation.

**Accept stage changes:** Unmet criteria become `Finding{severity:critical, category:acceptance, scope:spec}`. Accept emits structured `[]Finding` in addition to the current `GapSummary` string.

**Spec loop changes:** After bead loop, run accept then specreview. Combined verdict: fail if accept fails OR review verdict is fail. On any failure, merge accept findings + review findings and pass to remediation. On pass with review findings, create from-review beads labeled by scope.

**Remediation changes:** `executeRemediation` receives `[]Finding` instead of a gap analysis string. Serializes findings to pass to decompose stage via `StageRequest`. Decompose gains `findingsDecomposePromptTemplate` that maps findings to targeted fix beads.

**run2.go:** `--from-review` flag skips plan/decompose/accept/review; queries open beads with `from-review` label (optionally scoped by `--spec`) and runs them through the bead loop directly.

**Prompt fragment:** `review_spec_v2.md` loaded from project root, provides review instructions covering correctness, security (OWASP), error handling, test coverage, quality, and architectural fit.

## Test Strategy

- Unit tests for specreview: JSON parsing, verdict logic (critical→fail, warning+suggestion→pass), finding classification, model tier enforcement
- Unit tests for findings-based decompose template: verify beads reference findings, not original plan tasks
- Unit tests for accept finding output: each unmet criterion produces a critical/acceptance/spec finding
- Unit tests for from-review bead labeling: spec-scoped vs general based on `scope` field
- Unit tests for `--from-review` flag: bead query filter, skips plan/decompose, no remediation cycle
- Integration test: bead loop → accept (fail) → specreview → findings merge → targeted decompose → second accept (pass)
- Integration test: pass-with-improvements — specreview passes with warnings, from-review beads created, spec proceeds to present

## Implementation Tasks

### Task 1: Define shared Finding type
**Files:** `internal/v2/stage/finding/finding.go`, `internal/v2/stage/finding/finding_test.go`
**What to Do:** Define the `Finding` struct with fields `Severity`, `Category`, `Scope`, `Description`, `AffectedFiles []string`. Define string constants for severity (`SeverityCritical`, `SeverityWarning`, `SeveritySuggestion`), category (`CategoryBug`, `CategorySecurity`, `CategoryQuality`, `CategoryTestGap`, `CategoryArchitecture`, `CategoryAcceptance`), and scope (`ScopeSpec`, `ScopeGeneral`). Add `HasCritical([]Finding) bool` helper. Add `NormalizeNilFields()` to zero-value nil slices.
**Acceptance Criteria:**
- `HasCritical` returns true iff any finding has `Severity == SeverityCritical`
- All nil slice fields normalize to empty via `NormalizeNilFields()`
- Constants compile and match spec values

### Task 2: Implement spec-level review stage
**Files:** `internal/v2/stage/specreview/specreview.go`, `internal/v2/stage/specreview/specreview_test.go`
**What to Do:** Create `SpecReviewStage` implementing `Stage`. `Run` method: build prompt from `review_spec_v2.md` fragment + cumulative diff (call `DiffFromBase`) + plan text (read from `.gromit/v2/plan.md`) + project context. Invoke LLM (always using `TierHigh` / highest tier). Parse JSON response `{"verdict":"pass|fail","findings":[...]}`. Retry once with repair prompt on parse failure. Return `SpecReviewArtifacts{Verdict string, Findings []finding.Finding}`. Decision: `DecisionFail` if verdict is "fail", `DecisionProceed` otherwise. Emit a typed `SpecReviewCompletedEvent`.
**Acceptance Criteria:**
- Any `critical` finding in parsed output sets `Verdict = "fail"` and `Decision = DecisionFail`
- `warning` and `suggestion`-only findings set `Verdict = "pass"` and `Decision = DecisionProceed`
- LLM is always invoked with `TierHigh` regardless of the stage request tier
- Parse failure triggers one repair-prompt retry; second failure returns error
**Dependencies:** Task 1

### Task 3: Create spec-level review prompt fragment
**Files:** `review_spec_v2.md`
**What to Do:** Write the review prompt fragment (loaded from project root). Sections: (1) Role — you are a senior reviewer evaluating the cumulative diff of a completed spec. (2) Inputs — diff, plan, project context. (3) Review dimensions — correctness, security (OWASP Top 10), error handling, test coverage gaps, code quality, architectural fit. (4) Output format — strict JSON matching the findings schema with verdict logic. (5) Verdict rule — `fail` if any finding is critical, `pass` otherwise.
**Acceptance Criteria:**
- Fragment is loadable as a plain text file from the project root
- JSON output schema in the fragment matches the `Finding` struct fields exactly (snake_case)
- Verdict rule is stated unambiguously

### Task 4: Add structured findings output to accept stage
**Files:** `internal/v2/stage/accept/accept.go`, `internal/v2/stage/accept/accept_test.go`
**What to Do:** Extend `AcceptArtifacts` with `Findings []finding.Finding`. For each unmet acceptance criterion, append a `Finding{Severity: SeverityCritical, Category: CategoryAcceptance, Scope: ScopeSpec, Description: criterion summary, AffectedFiles: nil}`. Keep existing `GapSummary` and `Results` fields unchanged (backwards compatibility). The GapSummary string remains the concatenation for legacy display.
**Acceptance Criteria:**
- Each failing criterion produces exactly one `Finding` with severity critical and category acceptance
- Passing runs produce zero findings
- Existing accept tests continue to pass
**Dependencies:** Task 1

### Task 5: Add findings field to StageRequest and wire decompose
**Files:** `internal/v2/stage/stage.go`, `internal/v2/stage/decompose/decompose.go`, `internal/v2/stage/decompose/decompose_test.go`
**What to Do:** Add `Findings []finding.Finding` to `StageRequest`. In decompose, add `findingsDecomposePromptTemplate` that takes serialized findings (JSON array) instead of a gap analysis string. Selection: if `len(req.Findings) > 0` use findings template; else if `req.Remediation && req.GapAnalysis != ""` use existing remediation template; else use default template. The findings template instructs the LLM to create one targeted fix bead per finding (or group related findings), reference specific affected files, and link dependencies to existing closed beads.
**Acceptance Criteria:**
- When `req.Findings` is non-empty, findings template is selected
- Findings template prompt includes serialized findings JSON
- Existing template selection logic is unchanged when `Findings` is nil/empty
**Dependencies:** Task 1, Task 4

### Task 6: Update remediation runner to use findings
**Files:** `internal/v2/remediation/remediation.go`, `internal/v2/remediation/remediation_test.go`
**What to Do:** Change `RemediationRunnerConfig` to add `Findings []finding.Finding`. Change `executeRemediation` to accept `findings []finding.Finding` and pass them to the decompose stage via `req.Findings`. Remove the plan re-decomposition step from the remediation path: when findings are non-empty, skip re-running the plan stage and go directly to decompose with findings. Keep the plan stage call path when findings are empty (fallback for callers that haven't migrated). Update event emission to include finding count.
**Acceptance Criteria:**
- When findings are provided, plan stage is skipped in remediation
- Decompose stage receives `req.Findings` set to the provided findings
- When findings are empty, existing behavior (plan + gap-analysis decompose) is preserved
**Dependencies:** Task 1, Task 4, Task 5

### Task 7: Update spec loop to call specreview and combine verdicts
**Files:** `internal/v2/loop/spec_loop.go`, `internal/v2/loop/spec_loop_test.go`
**What to Do:** After the bead loop completes, run accept then specreview (in that order). Collect combined findings: accept failures become findings (from Task 4), specreview findings come from `SpecReviewArtifacts`. Gate: spec succeeds only when `accept.Decision == Proceed AND specreview.Decision == Proceed`. On any failure: merge accept.Findings + specreview.Findings, pass to remediation runner. On specreview pass with findings (warnings/suggestions): create from-review beads before calling present — spec-scoped findings (`scope == ScopeSpec`) get label `from-review spec:<specID>`, general findings (`scope == ScopeGeneral`) get label `from-review`. Then proceed to present.
**Acceptance Criteria:**
- Spec fails when accept fails, even if specreview passes
- Spec fails when specreview verdict is fail, even if accept passes
- Spec passes only when both pass
- From-review beads are created with correct labels before present is called
- Remediation receives merged findings list from both stages
**Dependencies:** Task 2, Task 4, Task 6

### Task 8: Wire spec-level review stage in run2 components
**Files:** `internal/v2/loop/run2_components.go`
**What to Do:** In `NewRun2LoopComponents`, load `review_spec_v2.md` fragment from project root (similar to other fragment loads). Construct `specreview.SpecReviewStage` with the loaded fragment, highest-tier model selection (hardcode `TierHigh`), and the same router and git adapter used by other stages. Add `SpecReviewStage` field to `Run2LoopComponents` struct. Pass it to `specLoop` construction.
**Acceptance Criteria:**
- `review_spec_v2.md` is loaded at startup; missing file returns an error
- `SpecReviewStage` is constructed with `TierHigh` explicitly
- The stage is reachable via `Run2LoopComponents.SpecReviewStage`
**Dependencies:** Task 2, Task 3, Task 7

### Task 9: Add `--from-review` flag to run2 command
**Files:** `cmd/gromit/run2.go`
**What to Do:** Add `--from-review` boolean flag. Add `--spec` string flag (already may exist for other uses; add if absent). When `--from-review` is set: skip plan/decompose/accept/specreview; query open beads with label `from-review` from the task tracker; if `--spec <id>` is also set, add label filter `spec:<id>`; run queried beads through the bead loop directly; exit without remediation cycle. The bead loop result is the final result — no accept/review after from-review execution.
**Acceptance Criteria:**
- `--from-review` without `--spec` runs all open beads labeled `from-review`
- `--from-review --spec foo` runs only beads labeled `from-review` AND `spec:foo`
- Plan, decompose, accept, and specreview stages are not called in from-review mode
- From-review bead failures do not trigger remediation; beads remain open for next run
**Dependencies:** Task 7, Task 8