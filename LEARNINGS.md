# LEARNINGS

## Confirmed Learnings

### 2026-02-27 | large_module_decomposition_preserves_package_boundaries | ARCHITECTURE
*Related to: gromit/review-1772054097495408438, gromit/review-1772062103155608386*
*Consolidated from: thorough_review_logic_belongs_in_dedicated_package + process_trend_split_should_keep_same_package_boundaries*

Large runner modules should be decomposed by responsibility into dedicated packages/files while preserving package boundaries and API surfaces to reduce coupling and file-size risk.

### 2026-02-25 | provider_fixture_governance_schema_first_deterministic | TEST_QUALITY
*Related to: gromit/review-1771886115282672489, gromit/review-1771908518510170783, gromit/review-1771929160626448252, gromit/review-1771897964548429202*
*Consolidated from: fixture_tests_should_assert_schema_and_records_not_prose_tokens + provider_fixture_contracts_should_be_curated_with_provenance_and_deterministic_metadata + gemini_fixture_metadata_and_artifact_requirements_are_test_enforced + gemini_schema_notes_must_reference_model_and_token_cost_evidence + real_probe_fixtures_are_canonical_and_tests_should_follow_them*

Provider fixture governance should be schema-first and deterministic: canonical real-probe fixtures with provenance metadata, explicit required artifacts, and structured assertions (never prose-token checks).

### 2026-02-24 | worktree_mergeback_conflict_ownership_and_classification | ARCHITECTURE
*Related to: gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810, gromit/review-1771878486437709843*
*Consolidated from: session_worktree_conflict_classification_and_cleanup_ownership + mergeback_requires_typed_failure_decision_and_defensive_abort*

Session worktree + MergeBack must use one cleanup owner and typed conflict/failure classification based on git output and exit status, with defensive abort only for merge state created in the current operation. Classification combines git output signals and exit codes so true merge conflicts route to conflict handoff while non-conflict git failures follow cleanup + explicit error handling. Prefer explicit conflict probes over message-fragment matching for contention retry.

### 2026-02-27 | observability_contract_completeness | ARCHITECTURE
*Related to: gromit/review-1771929519774405451, gromit/review-1771938913730053167, gromit/review-1771893007120033611, gromit/review-1772059511071909600, gromit/review-1772155497602965256*
*Consolidated from: observability_parity_telemetry_contract + schema_parity_contract_tests_catch_field_mapping_drift*

Observability completeness is one contract: runtime attribution, iteration-row persistence, and IterationLog/IterationMetric field parity must fail closed together, with per-field diagnostics for missing row/attribution/numeric values.

### 2026-02-24 | cohort_validation_must_reject_nil_lookup_payloads_before_field_access | RELIABILITY
*Related to: gromit/review-1771938913730053167*

Cohort validation paths that call external lookups must treat a nil object as invalid input and return a typed error before dereferencing fields, preventing panic-class failures from malformed integration responses.

### 2026-02-25 | review_state_should_prefer_local_json_with_repo_scoped_tag_fallback | ARCHITECTURE
*Related to: gromit/review-1772059511071909600*

When tracking last-review commit, prefer workspace-local interactive state as the authoritative value and use git tags only as repo-scoped fallback history markers to avoid stale cross-worktree state bleed.

## Provisional Learnings

*(none)*

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
