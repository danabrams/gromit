---
created: 2026-02-28T00:00:00Z
decomposed: true
decomposed_at: "2026-03-01T02:31:09Z"
id: vision-metrics-pr-template-and-ci-enforcement
source_spec: vision-metrics-pr-template-and-ci-enforcement
---

# Vision Metrics PR Template and CI Enforcement Implementation Plan

**Goal:** Enforce complete, auditable `VISION.md` cycle metadata on spec-presentation PRs through a structured PR template, CI validation, and pending-aware reporting semantics.

**Architecture:** Capture metrics in a strict machine-parseable PR metadata block (LLM-friendly), validate it in CI with actionable field errors, and extend vision-metrics domain logic to support `escaped_regression_within_7d=pending` plus pending-excluded escaped-regression rate rollups.

**Tech Stack:** Go, Cobra CLI, GitHub Actions CI, Markdown/PR template contract.

**Spec:** `.gromit/specs/vision-metrics-pr-template-and-ci-enforcement.md`

---

## Architecture

**Overview:**
Use a repository PR template as the canonical metadata input surface and enforce compliance with an automated CI validator that parses only a strict structured block. Reuse the existing `internal/visionmetrics` package for field contract and rule enforcement, extending it for pending-state escaped-regression semantics.

**Key Components:**
1. **PR Template Contract (`.github/pull_request_template.md`)**: Includes required Vision Metrics metadata in a strict machine-parseable block with one known-good example for LLM copy/edit behavior.
2. **PR Metadata Parser (`internal/visionmetrics/prbody.go`)**: Extracts and parses only the structured metadata block from PR body text; ignores surrounding prose.
3. **Validation Layer (`internal/visionmetrics/validate.go`)**: Enforces required fields, enum domains, conditional `review_rationale`, and debugging subset semantics.
4. **Pending-Aware Contract (`internal/visionmetrics/contract.go`)**: Supports `escaped_regression_within_7d` values `yes|no|pending`.
5. **CI Enforcement (`.github/workflows/ci.yml` + validator command/script)**: Runs on PR updates, failing fast with actionable field-specific errors.
6. **Pending-Resolution Workflow Documentation (`README.md`)**: Documents when `pending` is acceptable and how to finalize to `yes`/`no` after the 7-day window.
7. **Pending-Aware Rollups (`internal/visionmetrics/rollup.go`)**: Excludes unresolved pending records from escaped-regression numerator/denominator while surfacing pending count.

**Integration Points:**
- Extends existing vision metrics implementation under `internal/visionmetrics/`.
- Adds a pull-request CI gate without changing push/schedule/release test behavior.
- Connects repository workflow docs to the PR metadata contract and finalization path.

**Data Flow:**
1. PR author fills structured Vision Metrics block in PR template.
2. CI extracts and parses the block on PR updates.
3. Validator enforces required/conditional/domain rules and returns field-specific failures.
4. Valid metadata feeds stored cycle records and reporting.
5. `escaped_regression_within_7d=pending` remains valid pre-window; follow-up PR edit resolves to `yes`/`no`.
6. Rollups exclude unresolved pending from escaped-regression rate math and report pending count separately.

**Files to Modify:**
- `.github/workflows/ci.yml`
- `README.md`
- `internal/visionmetrics/contract.go`
- `internal/visionmetrics/validate.go`
- `internal/visionmetrics/rollup.go`
- `internal/visionmetrics/validate_test.go`
- `internal/visionmetrics/rollup_test.go`
- `cmd/gromit/vision_metrics.go` (if reporting shape is extended)

**Files to Create:**
- `.github/pull_request_template.md`
- `internal/visionmetrics/prbody.go`
- `internal/visionmetrics/prbody_test.go`
- CI helper entrypoint for PR-metadata validation (script or command)

**Tradeoffs:**
- **Structured block vs freeform markdown fields:** structured data is significantly more robust for LLM-authored PRs and deterministic CI parsing.
- **Go parser vs regex-only shell checks:** parser code adds implementation overhead but provides precise errors and stronger testability.
- **Inline pending support vs separate delayed-tracking system:** keeping pending in the same contract preserves auditability at PR presentation time.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** parser extraction/parsing behavior and validation rules.
2. **Integration Tests:** PR-body fixture -> parser -> validator -> CI-style error aggregation.
3. **Rollup Tests:** pending exclusion from escaped-regression denominator/numerator and pending visibility.
4. **Documentation Checks:** template contract and workflow guidance coverage.

**Key Test Cases:**
- Missing required fields fail with actionable field-specific errors.
- Invalid enum values fail with expected-domain messages.
- `human_debugging_intervention=yes` requires `human_tactical_intervention=yes`.
- `review_outcome=rework_vision_change` requires non-empty `review_rationale`.
- `escaped_regression_within_7d=pending` is accepted at validation time.
- Escaped-regression rate excludes unresolved pending records from numerator/denominator.
- Reporting exposes unresolved pending count.
- Malformed/missing structured metadata block fails with correction guidance.
- Known-good template example parses and validates successfully.

**Mocking Strategy:**
- Table-driven parser/validator tests with local string fixtures.
- CI helper tested as pure command behavior (no external API dependency).
- Rollup tests run with in-memory records and JSONL fixtures.

**Coverage Goals:**
- Full branch coverage for new conditional and pending logic.
- Explicit tests for every acceptance-rule path in the spec.
- At least one integration-style test validating actionable CI error output.

**Test Organization:**
- `internal/visionmetrics/prbody_test.go`
- `internal/visionmetrics/validate_test.go`
- `internal/visionmetrics/rollup_test.go`
- Optional command-level tests under `cmd/gromit/`
- Documentation verification by repository text assertions

## Implementation Tasks

### Task 1: Add LLM-safe PR template contract

**Files:**
- Create: `.github/pull_request_template.md`

**What to Do:**
Define a mandatory Vision Metrics section with a strict structured block (machine-parseable), list all required fields and allowed values inline, and include one known-good example that LLM authors can reliably copy and edit.

**Acceptance Criteria:**
- Template includes all required fields from the spec.
- Allowed values are explicit and unambiguous.
- Conditional rule for `review_rationale` is documented inline.
- Template format is parseable deterministically by CI.

**Dependencies:**
- None.

### Task 2: Implement PR-body parser for structured metrics block

**Files:**
- Create: `internal/visionmetrics/prbody.go`
- Create: `internal/visionmetrics/prbody_test.go`

**What to Do:**
Build parser logic that extracts only the structured Vision Metrics block from PR body text and maps it into a cycle record shape suitable for validation.

**Acceptance Criteria:**
- Parser ignores non-contract prose and comments.
- Missing/malformed block produces actionable parse errors.
- Known-good example block parses successfully.

**Dependencies:**
- Task 1.

### Task 3: Extend contract and validation for pending escaped-regression semantics

**Files:**
- Modify: `internal/visionmetrics/contract.go`
- Modify: `internal/visionmetrics/validate.go`
- Modify: `internal/visionmetrics/validate_test.go`

**What to Do:**
Extend escaped-regression field domain to accept `pending` while preserving existing rules for required fields, subset constraints, and conditional rationale.

**Acceptance Criteria:**
- `escaped_regression_within_7d` accepts `yes|no|pending`.
- Existing validation invariants still hold.
- Validation errors identify the field and allowed domain.

**Dependencies:**
- Task 2.

### Task 4: Add CI enforcement for PR metadata compliance

**Files:**
- Modify: `.github/workflows/ci.yml`
- Create: CI helper entrypoint (script or command) used by workflow

**What to Do:**
Wire pull-request CI to read PR body content, run parser+validator, and fail with actionable errors when metadata is missing or invalid.

**Acceptance Criteria:**
- CI runs on PR updates and enforces the metadata contract.
- Failure output identifies exact invalid fields and expected values.
- CI passes with the known-good metadata example.

**Dependencies:**
- Task 2.
- Task 3.

### Task 5: Implement pending-aware escaped-regression rollup behavior

**Files:**
- Modify: `internal/visionmetrics/rollup.go`
- Modify: `internal/visionmetrics/rollup_test.go`
- Modify: `cmd/gromit/vision_metrics.go` (if output needs pending count surfacing)

**What to Do:**
Update rollup computation so escaped-regression rates exclude unresolved pending records from numerator/denominator while tracking pending count for transparency.

**Acceptance Criteria:**
- Escaped-regression rate denominator includes only resolved (`yes|no`) records.
- Pending count is visible in reporting outputs.
- Existing KPI calculations remain correct for other metrics.

**Dependencies:**
- Task 3.

### Task 6: Document pending-resolution and workflow expectations

**Files:**
- Modify: `README.md` (or docs workflow equivalent)

**What to Do:**
Add contributor guidance for where to provide Vision Metrics PR metadata, when `pending` is valid, and how to finalize unresolved escaped-regression values after the 7-day window.

**Acceptance Criteria:**
- Documentation points to PR template contract and CI validation behavior.
- Documentation defines the follow-up path from `pending` to `yes|no`.
- Documentation clarifies that unresolved pending is excluded from escaped-regression rate calculations but counted separately.

**Dependencies:**
- Task 4.
- Task 5.

### Task 7: End-to-end validation and reporting coverage

**Files:**
- Modify/Create tests under `internal/visionmetrics/` and possibly `cmd/gromit/`

**What to Do:**
Add an end-to-end fixture-driven test path proving parser+validator+rollup behavior for valid, invalid, and pending-resolution cases.

**Acceptance Criteria:**
- CI-style failures are deterministic and actionable.
- Pending handling is verified both pre-resolution and post-resolution.
- Acceptance criteria in spec are directly covered by tests.

**Dependencies:**
- Task 2.
- Task 3.
- Task 4.
- Task 5.
- Task 6.

---

## Notes

- The parser+template contract should optimize for deterministic LLM output, not manual prose flexibility.
- Keep validation logic centralized in `internal/visionmetrics` so CLI, CI, and future tooling share one source of truth.
- Prefer explicit counts in reporting to preserve auditability and make pending backlog visible.
