# LEARNINGS

## Confirmed Learnings

### 2026-02-25 | agent_test_setup_should_be_shared_helpers | TEST_QUALITY
*Related to: gromit/review-1772054097495408438*

When multiple agent-related tests need temporary config/backlog scaffolding, centralize the setup in shared helpers to avoid drift and simplify new acceptance tests.

### 2026-02-25 | thorough_review_logic_belongs_in_dedicated_package | ARCHITECTURE
*Related to: gromit/review-1772054097495408438*

Thorough review orchestration should live in its own package so runner wiring stays thin and review-specific dependencies remain isolated.

### 2026-02-25 | provider_fixture_governance_schema_first_deterministic | TEST_QUALITY
*Related to: gromit/review-1771886115282672489, gromit/review-1771908518510170783, gromit/review-1771929160626448252, gromit/review-1771897964548429202*
*Consolidated from: fixture_tests_should_assert_schema_and_records_not_prose_tokens + provider_fixture_contracts_should_be_curated_with_provenance_and_deterministic_metadata + gemini_fixture_metadata_and_artifact_requirements_are_test_enforced + gemini_schema_notes_must_reference_model_and_token_cost_evidence + real_probe_fixtures_are_canonical_and_tests_should_follow_them*

Provider fixture governance should be schema-first and deterministic: canonical real-probe fixtures with provenance metadata, explicit required artifacts, and structured assertions (never prose-token checks).

### 2026-02-25 | repo_boundary_governance_local_ci_and_runtime_artifacts | PROCESS
*Related to: gromit/review-1771986309252130811, gromit/review-1771906824758942325, gromit/review-1771933220448456983, gromit/review-1771936368864075181*
*Consolidated from: repo_hygiene_policy_enforced_local_and_ci_with_gitlink_guards + runtime_artifacts_outside_versioned_paths_canonical_fixtures_only*

Repository boundary governance should be enforced as one policy: local+CI guards must block gitlinks/ephemeral runtime artifacts, and only deterministic curated fixtures are allowed in versioned paths.

### 2026-02-25 | end_to_end_tests_must_assert_cli_surface_not_only_internal_calls | TEST_QUALITY
*Related to: gromit-lqqvs*

Tests named end-to-end should execute the CLI path and assert user-visible behavior (output/exit code/artifacts); parser- or model-state-only checks belong in focused unit tests.

### 2026-02-24 | worktree_mergeback_conflict_ownership_and_classification | ARCHITECTURE
*Related to: gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810, gromit/review-1771878486437709843*
*Consolidated from: session_worktree_conflict_classification_and_cleanup_ownership + mergeback_requires_typed_failure_decision_and_defensive_abort*

Session worktree + MergeBack must use one cleanup owner and typed conflict/failure classification based on git output and exit status, with defensive abort only for merge state created in the current operation. Classification combines git output signals and exit codes so true merge conflicts route to conflict handoff while non-conflict git failures follow cleanup + explicit error handling. Prefer explicit conflict probes over message-fragment matching for contention retry.

### 2026-02-25 | runtime_path_parity_for_observability | ARCHITECTURE
*Related to: gromit/review-1771929519774405451, gromit/review-1771938913730053167, gromit/review-1771893007120033611*
*Consolidated from: token_efficiency_telemetry_requires_single_runtime_wiring_path + time_injection_only_helps_when_runtime_paths_consume_the_injected_clock + harness_requires_real_worktree_execution_and_log_wiring*

Runtime path parity is mandatory for observability: injected clocks/harnesses/telemetry fields only matter if production execution paths consume them end-to-end, with log wiring validated by acceptance/contract tests.

### 2026-02-24 | token_efficiency_routing_needs_strict_category_and_tier_validation | RELIABILITY
*Related to: gromit/review-1771929519774405451*

`token_efficiency.routing` config must validate both override categories and tier values (`low|medium|high`) after normalization, otherwise invalid entries can silently bypass intended utility-routing guardrails.

### 2026-02-24 | issue_ledger_normalization_should_be_isolated_from_semantic_edits | CODE_QUALITY
*Related to: gromit/review-1771936368864075181*

When issue-ledger normalization (ordering/canonical encoding) and semantic issue edits land in the same change, reviewability and merge safety degrade. Keep normalization-only rewrites separate from content changes and enforce that separation in automation.

### 2026-02-24 | cohort_validation_must_reject_nil_lookup_payloads_before_field_access | RELIABILITY
*Related to: gromit/review-1771938913730053167*

Cohort validation paths that call external lookups must treat a nil object as invalid input and return a typed error before dereferencing fields, preventing panic-class failures from malformed integration responses.

## Provisional Learnings

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
