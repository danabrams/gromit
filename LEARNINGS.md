# LEARNINGS

## Confirmed Learnings

### 2026-02-28 | execution_boundary_ownership_contract | ARCHITECTURE
*Related to: gromit/review-1772054097495408438, gromit/review-1772062103155608386, gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810, gromit/review-1771878486437709843, gromit/review-1772059511071909600, gromit/review-1772199039639467769*
*Consolidated from: large_module_decomposition_preserves_package_boundaries + worktree_mergeback_conflict_ownership_and_classification + review_state_should_prefer_local_json_with_repo_scoped_tag_fallback + emitter_logging_migration_must_preserve_fallback_path*

Execution-boundary ownership includes diagnostic survivability: decompose by responsibility while preserving package boundaries, keep single mergeback cleanup ownership with typed conflict handling, use workspace-local state as source of truth with repo-scoped fallback only, and preserve fallback emission paths so failures remain observable.

### 2026-02-25 | provider_fixture_governance_schema_first_deterministic | TEST_QUALITY
*Related to: gromit/review-1771886115282672489, gromit/review-1771908518510170783, gromit/review-1771929160626448252, gromit/review-1771897964548429202*
*Consolidated from: fixture_tests_should_assert_schema_and_records_not_prose_tokens + provider_fixture_contracts_should_be_curated_with_provenance_and_deterministic_metadata + gemini_fixture_metadata_and_artifact_requirements_are_test_enforced + gemini_schema_notes_must_reference_model_and_token_cost_evidence + real_probe_fixtures_are_canonical_and_tests_should_follow_them*

Provider fixture governance should be schema-first and deterministic: canonical real-probe fixtures with provenance metadata, explicit required artifacts, and structured assertions (never prose-token checks).

### 2026-02-24 | cohort_validation_must_reject_nil_lookup_payloads_before_field_access | RELIABILITY
*Related to: gromit/review-1771938913730053167*

Cohort validation paths that call external lookups must treat a nil object as invalid input and return a typed error before dereferencing fields, preventing panic-class failures from malformed integration responses.

### 2026-02-28 | codex_usage_attribution_contract_first | ARCHITECTURE
*Related to: gromit-9bhr, gromit-qs2ks*

Codex usage attribution must be contract-first: define normalized usage schema, implement a pure event reducer with precedence rules, and require stream-event matrix tests before integration wiring.

### 2026-02-28 | nil_guard_centralized_wrapper | RELIABILITY
*Related to: gromit-scrae.1, gromit-scrae.3*

Nil-guard fixes must be centralized in one safe-call wrapper; avoid bead scopes that patch every call site directly.

## Provisional Learnings

*No provisional learnings at this time.*

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
