# LEARNINGS

## Confirmed Learnings

### 2026-03-02 | package_level_var_injection_requires_cleanup | TEST_QUALITY
*Related to: gromit/review-1772480612920810000, gromit-o38nb*

Package-level var injection for test fakes (e.g. `killDescendantsOnCancelFn = procutil.KillDescendantsOnCancel`) must always be paired with a `t.Cleanup` restore. Tests that override a var without restoring it pollute all subsequent tests in the same `go test` run. Add a restore helper (e.g. `restoreXxxFns(t *testing.T)`) that captures current values and registers `t.Cleanup` to reset them. Add a comment above the var block noting the restore requirement.

### 2026-03-02 | boundary_sensitive_runtime_behavior_fail_closed_centralized | ARCHITECTURE
*Related to: gromit/review-1772480612920810000, gromit-vj714, gromit-9olfs, gromit-5ag6c, gromit/review-1772054097495408438, gromit/review-1772062103155608386, gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810, gromit/review-1771878486437709843, gromit/review-1772059511071909600, gromit/review-1772199039639467769, gromit-9bhr, gromit-qs2ks, gromit/review-1771938913730053167, gromit-scrae.1, gromit-scrae.3*
*Consolidated from: runtime_execution_attribution_ownership_contract + nil_safety_boundary_centralized_guard + subprocess_context_threading_and_platform_guards*

Boundary-sensitive runtime behavior must be centralized and fail-closed: subprocess/context propagation, platform-specific guards, attribution completeness, and lifecycle persistence must be enforced through shared boundary contracts rather than call-site patches. Specific recurring failure modes: (1) context threading — callers that receive a live ctx must thread it all the way to subprocess creation; using context.Background() mid-chain silently ignores cancellation and causes up to DefaultCommandTimeout of lag after user cancel; (2) platform-specific /proc paths are Linux-only — add //go:build linux tags or explicit typed not-supported behavior on macOS; (3) attribution completeness, lifecycle persistence, and nil/callback safety must use centralized safe-call wrappers that reject nil payloads before field access. All three failure modes share the same root: boundary ownership spread across call sites causes silent correctness and observability regressions.

### 2026-02-25 | provider_fixture_governance_schema_first_deterministic | TEST_QUALITY
*Related to: gromit/review-1771886115282672489, gromit/review-1771908518510170783, gromit/review-1771929160626448252, gromit/review-1771897964548429202*
*Consolidated from: fixture_tests_should_assert_schema_and_records_not_prose_tokens + provider_fixture_contracts_should_be_curated_with_provenance_and_deterministic_metadata + gemini_fixture_metadata_and_artifact_requirements_are_test_enforced + gemini_schema_notes_must_reference_model_and_token_cost_evidence + real_probe_fixtures_are_canonical_and_tests_should_follow_them*

Provider fixture governance should be schema-first and deterministic: canonical real-probe fixtures with provenance metadata, explicit required artifacts, and structured assertions (never prose-token checks). Fixture contracts must include deterministic schema assertions and provenance metadata; prose-token assertions are forbidden.

### 2026-03-03 | timemixin_embedded_struct_literal_initialization | CODE_PATTERN
*Related to: gromit/review-1772533894632263000*

When event structs embed TimeMixin (or any struct with fields), struct literals must initialize the embedded type by name: `TimeMixin: events.TimeMixin{Time: time.Now()}`, not `Time: time.Now()` directly. Go does not allow setting fields of an embedded struct without naming the embedded type in a composite literal. This was missed across 30+ call sites when TimeMixin was introduced, causing build failures in pipeline/review, pipeline/validate, pipeline/epilogue, pipeline/execute, pipeline/prepare, and runner/orchestrator.

### 2026-03-03 | integration_queue_session_architecture | ARCHITECTURE
*Related to: gromit/review-1772533894632263000*

Sessions now produce branches that flow through a state-machine-based integration queue (draft→ready→integrating→merged/conflict/failed_gates) with file-based JSON persistence and atomic writes, rather than merging directly back to main. This avoids merge conflicts during session execution and is a safer model for single-machine session orchestration. The tracker.Client interface decomposition into ItemReader/ItemWriter/ItemQuery follows Interface Segregation Principle well.

## Provisional Learnings

*No provisional learnings at this time.*

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
