# LEARNINGS

## Confirmed Learnings

### 2026-03-02 | runtime_execution_attribution_ownership_contract | ARCHITECTURE
*Related to: gromit/review-1772054097495408438, gromit/review-1772062103155608386, gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810, gromit/review-1771878486437709843, gromit/review-1772059511071909600, gromit/review-1772199039639467769, gromit-9bhr, gromit-qs2ks*
*Consolidated from: execution_boundary_ownership_contract + codex_usage_attribution_contract_first*

Runtime execution ownership and telemetry attribution must be contract-first: enforce one execution path for attribution/persistence, typed failure reasons, typed merge conflict handling, and stream-event matrix coverage before integration wiring.

### 2026-02-25 | provider_fixture_governance_schema_first_deterministic | TEST_QUALITY
*Related to: gromit/review-1771886115282672489, gromit/review-1771908518510170783, gromit/review-1771929160626448252, gromit/review-1771897964548429202*
*Consolidated from: fixture_tests_should_assert_schema_and_records_not_prose_tokens + provider_fixture_contracts_should_be_curated_with_provenance_and_deterministic_metadata + gemini_fixture_metadata_and_artifact_requirements_are_test_enforced + gemini_schema_notes_must_reference_model_and_token_cost_evidence + real_probe_fixtures_are_canonical_and_tests_should_follow_them*

Provider fixture governance should be schema-first and deterministic: canonical real-probe fixtures with provenance metadata, explicit required artifacts, and structured assertions (never prose-token checks).

### 2026-03-02 | nil_safety_boundary_centralized_guard | RELIABILITY
*Related to: gromit/review-1771938913730053167, gromit-scrae.1, gromit-scrae.3*
*Consolidated from: cohort_validation_must_reject_nil_lookup_payloads_before_field_access + nil_guard_centralized_wrapper*

Nil-safety must be enforced at boundaries and through centralized safe-call wrappers; reject nil lookup payloads before field access and avoid scattered call-site nil-guard patches.

## Provisional Learnings

*No provisional learnings at this time.*

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
