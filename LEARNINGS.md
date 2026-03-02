# LEARNINGS

## Confirmed Learnings

### 2026-03-02 | boundary_sensitive_runtime_behavior_fail_closed_centralized | ARCHITECTURE
*Related to: gromit/review-1772054097495408438, gromit/review-1772062103155608386, gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810, gromit/review-1771878486437709843, gromit/review-1772059511071909600, gromit/review-1772199039639467769, gromit-9bhr, gromit-qs2ks, gromit/review-1771938913730053167, gromit-scrae.1, gromit-scrae.3*
*Consolidated from: runtime_execution_attribution_ownership_contract + nil_safety_boundary_centralized_guard*

Boundary-sensitive runtime behavior must be fail-closed and centralized: attribution completeness, lifecycle persistence, and nil/callback safety should be enforced through shared boundary contracts/wrappers rather than distributed call-site patches. Enforce one execution path for attribution/persistence, typed failure reasons, stream-event matrix coverage before integration wiring, and centralized safe-call wrappers that reject nil payloads before field access.

### 2026-02-25 | provider_fixture_governance_schema_first_deterministic | TEST_QUALITY
*Related to: gromit/review-1771886115282672489, gromit/review-1771908518510170783, gromit/review-1771929160626448252, gromit/review-1771897964548429202*
*Consolidated from: fixture_tests_should_assert_schema_and_records_not_prose_tokens + provider_fixture_contracts_should_be_curated_with_provenance_and_deterministic_metadata + gemini_fixture_metadata_and_artifact_requirements_are_test_enforced + gemini_schema_notes_must_reference_model_and_token_cost_evidence + real_probe_fixtures_are_canonical_and_tests_should_follow_them*

Provider fixture governance should be schema-first and deterministic: canonical real-probe fixtures with provenance metadata, explicit required artifacts, and structured assertions (never prose-token checks). Fixture contracts must include deterministic schema assertions and provenance metadata; prose-token assertions are forbidden.

## Provisional Learnings

*No provisional learnings at this time.*

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
