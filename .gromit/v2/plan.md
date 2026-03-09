---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-09
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

**Goal:** Replace the broken remediation re-decomposition loop with a post-bead-loop spec-level review that produces structured findings, enabling targeted fix beads instead of plan-level re-decomposition.
**Architecture:** New `specreview` stage runs after accept in `spec_loop.go`; structured `Finding` type unifies accept and review output; remediation receives findings slice instead of gap-analysis string; decompose gains a findings-based prompt template; `--from-review` flag enables isolated review-bead runs.
**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

**Components:**
- `internal/v2/findings/` — shared `Finding` type and verdict logic used by accept, specreview, and remediation
- `internal/v2/stage/specreview/` — new stage: invokes highest-tier model with cumulative diff + plan + project context; parses structured JSON findings; returns verdict + findings
- `internal/v2/stage/accept/` — gains `Findings []findings.Finding` in artifacts alongside existing `GapSummary`; each unmet criterion becomes a finding with `severity:critical, category:acceptance`
- `internal/v2/stage/decompose/` — gains `findingsDecomposePromptTemplate`; receives findings slice via `stage.Request`
- `internal/v2/remediation/` — `executeRemediation` receives `[]findings.Finding` instead of `gapAnalysis string`
- `internal/v2/loop/spec_loop.go` — calls specreview after accept; combines findings from both; gates on both verdicts; passes findings to remediation
- `internal/v2/loop/run2_components.go` — wires specreview with highest-tier model
- `cmd/gromit/run2.go` — adds `--from-review` and `--spec` scoping flag

**Data flow:**
1. Bead loop completes → accept runs → produces `[]Finding` + pass/fail
2. Spec-level review runs → produces `[]Finding` + verdict
3. If both pass: spec-scoped findings → from-review beads labeled `from-review spec:<id>`; general findings → `from-review` beads; proceed to present
4. If either fails: collect critical findings → pass to remediation → findings-based decompose → targeted beads → retry

**Integration points:**
- `stage.Request` gains `Findings []findings.Finding` field (used for findings-based decompose)
- Specreview stage reads the same `DiffFromBase` used by accept
- Review fragment loaded from `review_spec_v2.md` at repo root

---

## Test Strategy

- Unit tests in `internal/v2/findings/`: verdict logic (critical→fail, warning/suggestion→pass), finding serialization
- Unit tests in `internal/v2/stage/specreview/`: output parsing for all severity/category/scope variants; verdict derivation; error on malformed JSON with retry
- Unit tests in `internal/v2/stage/accept/`: finding generation for each unmet criterion; correct severity/category/scope fields
- Unit tests in `internal/v2/stage/decompose/`: findings prompt template produces beads mapped to findings, not plan tasks
- Unit tests in `internal/v2/remediation/`: accepts findings slice, constructs correct decompose request
- Integration test: full post-bead-loop pipeline with fake LLM — accept fail → review fail → remediation with targeted findings → second accept pass
- Integration test: pass-with-improvements — review verdict pass with warning findings → from-review beads created with correct labels → spec proceeds
- Unit tests for `--from-review` flag: queries by label, skips plan/decompose/accept/review, no remediation cycle

---

## Implementation Tasks

### Task 1: Shared Finding Types
**Files:** `internal/v2/findings/findings.go`, `internal/v2/findings/findings_test.go`
**What to Do:** Define `Finding` struct with fields: `Severity` (critical/warning/suggestion), `Category` (bug/security/quality/test-gap/architecture/acceptance), `Scope` (spec/general), `Description`, `AffectedFiles []string`. Define `Verdict` type and `DeriveVerdict(findings []Finding) Verdict` — returns fail if any critical finding exists, pass otherwise. Define `ReviewResult` struct with `Verdict` and `Findings []Finding`. Export JSON tags matching the spec schema.
**Acceptance Criteria:**
1. `DeriveVerdict` returns fail for any input containing a critical finding, pass otherwise
2. `Finding` round-trips through JSON without field loss
3. No dependencies on other internal packages (leaf package)
**Dependencies:** None

### Task 2: Structured Findings Output in Accept Stage
**Files:** `internal/v2/stage/accept/accept.go`, `internal/v2/stage/accept/accept_test.go`
**What to Do:** Add `Findings []findings.Finding` to `AcceptArtifacts`. After evaluating each criterion, convert each failed criterion to a `Finding{Severity: "critical", Category: "acceptance", Scope: "spec", Description: criterion text + gap reason, AffectedFiles: nil}`. Store findings in artifacts alongside existing `GapSummary`. Keep existing `GapSummary` population for backward compatibility during transition.
**Acceptance Criteria:**
1. Each failed criterion produces exactly one Finding with severity=critical, category=acceptance, scope=spec
2. Passed criteria produce no findings
3. Existing `GapSummary` behavior unchanged
**Dependencies:** Task 1

### Task 3: Spec-Level Review Stage
**Files:** `internal/v2/stage/specreview/specreview.go`, `internal/v2/stage/specreview/specreview_test.go`, `review_spec_v2.md`
**What to Do:** Create `Stage` struct implementing the stage interface. `Run` method: (1) load cumulative diff via `req.GitDiffer.DiffFromBase()`; (2) load plan from worktree `plan.md`; (3) assemble prompt using project context + diff + plan + review_spec_v2.md fragment; (4) invoke LLM with highest-tier model (passed via config, not hardcoded); (5) parse JSON response into `findings.ReviewResult`; (6) retry once on parse failure. Return `DecisionProceed` with findings in artifacts. Write `review_spec_v2.md` covering correctness, security (OWASP top 10), error handling, test coverage gaps, code quality, architectural fit — instructing the model to emit structured JSON matching the `Finding` schema.
**Acceptance Criteria:**
1. Any critical finding in parsed output produces verdict=fail in result artifacts
2. Parse failure triggers one retry with reprompt; second failure returns error
3. Stage returns `DecisionProceed` in all non-error cases (verdict embedded in artifacts, not in Decision)
**Dependencies:** Task 1

### Task 4: Wire Specreview in run2_components.go
**Files:** `internal/v2/loop/run2_components.go`
**What to Do:** Add `SpecReview *specreview.Stage` field to `Run2LoopComponents`. In `NewRun2LoopComponents`, load `review_spec_v2.md` fragment, determine highest-tier model from router (phase key `"specreview"` or fallback to `"review"` top tier), instantiate `specreview.Stage` with provider, model, project context, and fragment. Expose via accessor or direct field access.
**Acceptance Criteria:**
1. `SpecReview` field populated in constructed components
2. Highest-tier model resolved from router without hardcoding model strings
3. Fragment load failure returns error from constructor
**Dependencies:** Task 3

### Task 5: Findings-Based Decompose Prompt Template
**Files:** `internal/v2/stage/decompose/decompose.go`, `internal/v2/stage/decompose/decompose_test.go`
**What to Do:** Add `findingsDecomposePromptTemplate` constant. The template receives a JSON findings list and instructs the model to create one or more targeted fix beads per finding — not to re-decompose the original plan. Add `Findings []findings.Finding` field to decompose `Options` or use `stage.Request`. When `len(req.Findings) > 0`, use findings template instead of remediation template.
**Acceptance Criteria:**
1. Non-empty `req.Findings` selects findings template over remediation template
2. Findings template produces beads with titles referencing finding descriptions, not original plan tasks
3. Empty findings continues to use existing template selection logic
**Dependencies:** Task 1, Task 2

### Task 6: Update Remediation to Use Findings
**Files:** `internal/v2/remediation/remediation.go`, `internal/v2/remediation/remediation_test.go`
**What to Do:** Change `executeRemediation` signature to accept `findings []findings.Finding` instead of `gapAnalysis string`. Populate `req.Findings` on the stage request before calling decompose. Remove (or gate) gap stage invocation when findings are present — skip the gap stage since findings are already structured. Update the caller in `spec_loop.go` (Task 7). Keep `gapAnalysis` fallback for callers that haven't yet migrated (guarded by `len(findings) == 0`).
**Acceptance Criteria:**
1. Non-empty findings slice populates `req.Findings` before decompose call
2. Gap stage skipped when findings provided
3. Empty findings falls back to existing gap-analysis path (backward compatible)
**Dependencies:** Task 1, Task 5

### Task 7: Integrate Specreview into Spec Loop
**Files:** `internal/v2/loop/spec_loop.go`
**What to Do:** In `ensureAcceptance` (or equivalent), after accept runs: (1) run specreview stage; (2) collect accept findings from artifacts; (3) collect specreview findings from artifacts; (4) combine into single `[]findings.Finding`; (5) gate: spec succeeds only if accept=pass AND review.verdict=pass; (6) on failure, pass combined findings to `executeRemediation`; (7) on pass-with-improvements: iterate findings, create from-review beads via tracker — spec-scoped (scope=spec) get label `from-review spec:<specID>`, general get `from-review`; then proceed to present.
**Acceptance Criteria:**
1. Spec proceeds to present only when both accept passes and review verdict is pass
2. Critical findings from either source trigger remediation with findings
3. Pass-with-improvements: from-review beads created with correct labels before present
**Dependencies:** Task 2, Task 3, Task 4, Task 6

### Task 8: Add --from-review Flag to run2.go
**Files:** `cmd/gromit/run2.go`
**What to Do:** Add `--from-review` boolean flag and `--spec` string flag (for scoping). When `--from-review` set: skip plan, skip decompose, skip accept, skip specreview — query tracker for open beads with label `from-review` (optionally filtered by `spec:<id>` label if `--spec` given), run bead loop directly, no remediation cycle. Return after bead loop; if any bead fails it stays open for next `--from-review` run.
**Acceptance Criteria:**
1. `--from-review` skips plan/decompose/accept/specreview stages entirely
2. `--from-review --spec foo` queries only beads with both `from-review` and `spec:foo` labels
3. Failed from-review beads stay open; no remediation cycle triggered
**Dependencies:** Task 7

### Task 9: Integration Tests
**Files:** `internal/v2/loop/spec_loop_specreview_integration_test.go`
**What to Do:** Write two integration tests using fake LLM providers: (1) Full post-bead-loop pipeline — accept fails (missing criterion) + review fails (critical finding) → remediation receives combined findings → targeted decompose → second accept passes → spec completes. (2) Pass-with-improvements — accept passes + review passes with warning findings → from-review beads created with correct labels → spec proceeds to present without remediation. Use existing fake/stub patterns from the test suite.
**Acceptance Criteria:**
1. Test 1 demonstrates targeted beads reference finding descriptions, not original plan sections
2. Test 2 asserts from-review beads exist in tracker with correct labels after spec completes
3. Both tests run without external dependencies (pure fake LLM)
**Dependencies:** Task 7, Task 8