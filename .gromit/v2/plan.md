---
id: spec-level-review-and-targeted-remediation
source_spec: spec-level-review-and-targeted-remediation
created: 2026-03-10
decomposed: false
---

# Spec-Level Review and Targeted Remediation Implementation Plan

**Goal:** Add a holistic spec-level code review after the bead loop, replace plan-based remediation with findings-based targeted remediation, and support deferred from-review bead execution.
**Architecture:** A new `specreview` stage evaluates the cumulative diff with structured findings. Accept gains structured output. Remediation receives findings instead of gap strings. A `--from-review` flag on `run2` runs only from-review beads without plan/decompose/accept/review.
**Spec:** `.gromit/specs/spec-level-review-and-targeted-remediation.md`

---

## Architecture

**Components:**
1. **`internal/v2/stage/specreview/`** — new stage implementing `stage.Stage`. Invokes highest-tier model with cumulative diff + plan + project context. Parses structured JSON findings. Verdict logic: any critical → fail, else pass.
2. **`internal/v2/review/findings.go`** — shared `Finding` type with severity/category/scope/description/affected_files, plus verdict computation. Used by both specreview and accept.
3. **Spec loop changes** — after accept, run specreview. Combined gating: both must pass. On failure, collect findings from both into a unified list for remediation.
4. **Remediation changes** — `executeRemediation` receives `[]Finding` instead of `string` gap analysis. Passes findings to decompose via new `stage.Request.Findings` field.
5. **Decompose changes** — new `findingsDecomposePromptTemplate` that creates targeted fix beads from findings.
6. **Accept changes** — produces `[]Finding` for unmet criteria (severity: critical, category: acceptance, scope: spec).
7. **From-review bead creation** — on pass-with-improvements, spec-scoped findings become beads with `from-review` + `spec:<id>` labels; general findings get `from-review` only.
8. **`--from-review` flag** — queries open from-review beads, runs them through bead loop, skips plan/decompose/accept/review.

**Data Flow:**
```
bead loop → accept(cumulative diff) → specreview(cumulative diff) 
  → if both pass (no critical findings): present + merge
  → if pass with improvements: create from-review beads, then present + merge
  → if either fail: collect all findings → remediation decompose → targeted beads → bead loop → retry
```

**Key Integration Points:**
- `stage.Request` gains `Findings []Finding` field
- `RemediationRunner` gains findings-based path alongside existing gap analysis path
- `run2_components.go` wires specreview stage with highest tier model
- `run2.go` gains `--from-review` and `--spec` flags

---

## Test Strategy

- **Unit tests** for finding types: verdict logic, severity classification, serialization
- **Unit tests** for specreview stage: structured output parsing, verdict computation, prompt assembly
- **Unit tests** for findings-based decompose: template renders findings not plan tasks, bead-to-finding mapping
- **Unit tests** for accept structured output: unmet criteria → findings conversion
- **Unit tests** for from-review bead creation: correct labeling by scope (spec vs general)
- **Unit tests** for `--from-review` flag: bead query filter, skip plan/decompose, no remediation cycle
- **Integration tests** for post-bead-loop pipeline: accept → specreview → remediation with findings → targeted beads
- **Integration tests** for pass-with-improvements: review passes with warnings, from-review beads created, spec proceeds
- **Mocking strategy**: LLM invocations mocked via existing provider interfaces; git diff mocked via existing `ExecGitAdapter` test doubles; bead tracker mocked via existing `bead.Client` test doubles

---

## Implementation Tasks

### Task 1: Define shared Finding type and verdict logic
**Files:** `internal/v2/review/findings.go`, `internal/v2/review/findings_test.go`
**What to Do:** Create the `Finding` struct with fields: Severity (critical/warning/suggestion), Category (bug/security/quality/test-gap/architecture/acceptance), Scope (spec/general), Description (string), AffectedFiles ([]string). Add `ComputeVerdict(findings []Finding) string` that returns "fail" if any critical finding exists, "pass" otherwise. Add JSON tags for serialization. Add `HasFindings(findings []Finding) bool` helper.
**Acceptance Criteria:**
- `ComputeVerdict` returns "fail" when any finding has severity "critical"
- `ComputeVerdict` returns "pass" when all findings are warning or suggestion
- Finding type serializes/deserializes to the JSON schema in the spec
**Dependencies:** None

### Task 2: Add Findings field to stage.Request
**Files:** `internal/v2/stage/stage.go`
**What to Do:** Add `Findings []Finding` field to `StageRequest`. Import the findings type from `internal/v2/review`. This field carries structured findings from accept/specreview into remediation decompose.
**Acceptance Criteria:**
- `StageRequest` has a `Findings` field of type `[]review.Finding`
- Existing tests continue to pass (zero-value nil slice is backward compatible)
**Dependencies:** Task 1

### Task 3: Create spec-level review stage
**Files:** `internal/v2/stage/specreview/specreview.go`, `internal/v2/stage/specreview/specreview_test.go`
**What to Do:** Implement `SpecReviewStage` satisfying `stage.Stage`. In `Run`: get cumulative diff via `git.DiffFromBase`, assemble prompt with diff + plan + project context using `PromptAssembler`, invoke LLM, parse structured JSON response into `[]Finding`, compute verdict via `ComputeVerdict`. Return `SpecReviewArtifacts{Verdict string, Findings []Finding}`. Stage name: `"spec-review"`. The stage always uses the model/tier passed in the request (wiring sets it to highest tier).
**Acceptance Criteria:**
- Stage parses valid JSON response into findings with correct severity/category/scope
- Verdict is "fail" when any critical finding exists
- Stage returns `DecisionFail` on fail verdict, `DecisionProceed` on pass
**Dependencies:** Task 1

### Task 4: Create spec-level review prompt fragment
**Files:** `review_spec_v2.md` (project root)
**What to Do:** Write the prompt fragment for spec-level review. It should instruct the model to evaluate the cumulative diff holistically, covering: correctness, security (OWASP top 10), error handling, test coverage gaps, code quality, architectural fit. Specify the exact JSON output schema matching the `Finding` type. Include examples of critical vs warning vs suggestion findings. Instruct that scope should be "spec" for issues in changed code and "general" for pre-existing issues noticed in the diff context.
**Acceptance Criteria:**
- Fragment specifies the exact JSON output schema
- Fragment covers all six review dimensions from the spec
- Fragment explains severity and scope classification rules
**Dependencies:** None

### Task 5: Wire spec-level review into run2 components
**Files:** `internal/v2/loop/run2_components.go`
**What to Do:** In `NewRun2LoopComponents`: load `review_spec_v2.md` fragment, create `SpecReviewStage` with highest-tier model configuration (opus tier), add it to the components struct. The spec-level review stage should be configured with the same prompt assembler pattern as other stages but forced to use the highest tier regardless of bead priority.
**Acceptance Criteria:**
- SpecReviewStage is created with the highest configured tier
- Fragment is loaded and passed to the stage
- Components struct exposes the spec review stage for use by spec loop
**Dependencies:** Task 3, Task 4

### Task 6: Add structured findings output to accept stage
**Files:** `internal/v2/stage/accept/accept.go`, `internal/v2/stage/accept/accept_test.go`
**What to Do:** Modify `AcceptArtifacts` to include a `Findings []Finding` field alongside the existing `GapSummary`. When criteria fail, create a `Finding` for each unmet criterion with severity "critical", category "acceptance", scope "spec", description from the criterion text + failure summary. Populate both `GapSummary` (for backward compat) and `Findings`.
**Acceptance Criteria:**
- Failed criteria produce findings with severity "critical" and category "acceptance"
- `AcceptArtifacts.Findings` is populated alongside `GapSummary`
- Existing gap analysis behavior is preserved (backward compatible)
**Dependencies:** Task 1

### Task 7: Integrate spec-level review into spec loop
**Files:** `internal/v2/loop/spec_loop.go`
**What to Do:** After the accept stage succeeds (DecisionProceed), run the spec-level review stage. Combined gating: if accept fails, collect accept findings and skip review. If accept passes but review fails, collect review findings. If both pass, check for pass-with-improvements (review passed but has findings). On failure: merge all findings into a single list and pass to remediation. On pass-with-improvements: create from-review beads (Task 9), then proceed to present.
**Acceptance Criteria:**
- Spec-level review runs after successful accept
- Spec fails when review verdict is "fail" even if accept passed
- Combined findings from both stages are passed to remediation on failure
**Dependencies:** Task 3, Task 5, Task 6

### Task 8: Add findings-based decompose prompt template
**Files:** `internal/v2/stage/decompose/decompose.go`, `internal/v2/stage/decompose/decompose_test.go`
**What to Do:** Add `findingsDecomposePromptTemplate` that takes a list of findings and produces targeted fix beads. Each finding should map to one or more beads. The template should instruct: "Create targeted fix beads for the following specific findings. Do NOT re-implement already completed work. Each bead should address one or a few related findings." When `req.Findings` is non-empty, use this template instead of the remediation template. Format findings as a numbered list with severity, category, description, and affected files.
**Acceptance Criteria:**
- When `req.Findings` is populated, findings template is used instead of remediation template
- Template renders each finding with its severity, category, and affected files
- Generated beads reference specific findings, not the original plan
**Dependencies:** Task 1, Task 2

### Task 9: Implement from-review bead creation on pass-with-improvements
**Files:** `internal/v2/loop/spec_loop.go`
**What to Do:** When spec-level review passes with findings (verdict "pass" but findings list non-empty): for each finding, create a bead via the task tracker. Spec-scoped findings (scope "spec") get labels `["from-review", "spec:<specID>"]`. General findings (scope "general") get label `["from-review"]`. Bead title derived from finding description (truncated to reasonable length). Bead description includes full finding detail. Log the created beads. Spec proceeds to present stage after bead creation.
**Acceptance Criteria:**
- Spec-scoped findings become beads with both `from-review` and `spec:<id>` labels
- General findings become beads with only `from-review` label
- Spec proceeds to present and merge after creating from-review beads
**Dependencies:** Task 7

### Task 10: Update remediation runner to use findings
**Files:** `internal/v2/remediation/remediation.go`
**What to Do:** Modify `Run` to accept findings (via the stage result artifacts). In `executeRemediation`, populate `req.Findings` from the collected findings. Keep backward compatibility: if findings are empty but gap analysis exists, fall back to the existing gap-analysis-based remediation path. The remediation loop collects findings from both accept and specreview on each iteration.
**Acceptance Criteria:**
- Remediation passes findings to decompose via `req.Findings`
- Falls back to gap-analysis path when findings are empty (backward compat)
- Remediation loop re-runs both accept and specreview, collecting fresh findings each iteration
**Dependencies:** Task 2, Task 7, Task 8

### Task 11: Add `--from-review` flag to run2
**Files:** `cmd/gromit/run2.go`
**What to Do:** Add `--from-review` boolean flag and optional `--spec <id>` string flag for scoping. When `--from-review` is set: skip plan/decompose stages. Query open beads with `from-review` label (and optionally `spec:<id>` label). Pass beads directly to bead loop. Skip accept and spec-level review after bead loop. No remediation cycle. If no from-review beads found, log and exit cleanly.
**Acceptance Criteria:**
- `--from-review` queries only beads with the `from-review` label
- `--from-review --spec <id>` further filters to beads with `spec:<id>` label
- From-review run skips plan, decompose, accept, and spec-level review stages
**Dependencies:** Task 9

### Task 12: Integration tests for spec-level review pipeline
**Files:** `internal/v2/loop/spec_loop_specreview_test.go`
**What to Do:** Write integration tests covering: (1) bead loop → accept pass → review pass → spec succeeds, (2) accept pass → review fail with critical finding → remediation triggered with findings → targeted beads created, (3) accept fail → remediation triggered with accept findings (review skipped), (4) pass-with-improvements → from-review beads created → spec proceeds to present. Use test doubles for LLM (return canned JSON), git (canned diffs), and task tracker (in-memory bead store).
**Acceptance Criteria:**
- All four scenarios are tested with assertions on stage sequencing and bead creation
- Findings flow correctly from accept/review through remediation to decompose
- From-review beads have correct labels based on finding scope
**Dependencies:** Task 7, Task 9, Task 10

### Task 13: Integration test for `--from-review` execution
**Files:** `internal/v2/loop/from_review_test.go`
**What to Do:** Write integration test for from-review execution: pre-create beads with `from-review` label, run with `--from-review` flag, verify only from-review beads are executed, verify no plan/decompose/accept/review stages run. Test scoping with `--spec` flag. Test empty result (no from-review beads found).
**Acceptance Criteria:**
- From-review run executes only beads with `from-review` label
- Scoped run filters correctly by spec label
- No accept/review/remediation cycle triggered
**Dependencies:** Task 11