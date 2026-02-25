# LEARNINGS

## Confirmed Learnings

### 2026-02-25 | provider_fixture_governance_schema_first_deterministic | TEST_QUALITY
*Related to: gromit/review-1771886115282672489, gromit/review-1771908518510170783, gromit/review-1771929160626448252, gromit/review-1771897964548429202*
*Consolidated from: fixture_tests_should_assert_schema_and_records_not_prose_tokens + provider_fixture_contracts_should_be_curated_with_provenance_and_deterministic_metadata + gemini_fixture_metadata_and_artifact_requirements_are_test_enforced + gemini_schema_notes_must_reference_model_and_token_cost_evidence + real_probe_fixtures_are_canonical_and_tests_should_follow_them*

Provider fixture governance should be schema-first and deterministic: canonical real-probe fixtures with provenance metadata, explicit required artifacts, and structured assertions (never prose-token checks).

### 2026-02-25 | repo_hygiene_policy_enforced_local_and_ci_with_gitlink_guards | PROCESS
*Related to: gromit/review-1771986309252130811, gromit/review-1771906824758942325*
*Consolidated from: beads_policy_checks_need_ci_enforcement + gitlink_entries_should_be_blocked_for_ephemeral_worktree_paths + repo_hygiene_guards_should_run_in_local_and_ci_paths*

Repository hygiene/policy checks must be enforced in both local hooks and CI, with explicit guards for high-risk git states (for example gitlinks in ephemeral worktree paths).

### 2026-02-25 | runtime_artifacts_outside_versioned_paths_canonical_fixtures_only | ARCHITECTURE
*Related to: gromit/review-1771933220448456983, gromit/review-1771936368864075181*
*Consolidated from: runtime_artifacts_and_fixture_validation_need_explicit_repo_boundaries + runtime_state_and_timestamped_report_artifacts_must_be_blocked_from_commits + provider_fixture_storage_requires_single_canonical_location*

Runtime/state artifacts and raw captures must remain outside versioned source paths; only deterministic curated fixtures in canonical directories should be committed.

### 2026-02-25 | cli_call_filters_should_match_command_token_not_string_prefix | TEST_QUALITY
*Related to: gromit/review-1772019888577324268*

Call-log assertions should match the first argv token (for example `codex`) after trim, not raw string prefixes, to avoid false positives like `codex-helper`.

### 2026-02-24 | worktree_mergeback_conflict_ownership_and_classification | ARCHITECTURE
*Related to: gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810, gromit/review-1771878486437709843*
*Consolidated from: session_worktree_conflict_classification_and_cleanup_ownership + mergeback_requires_typed_failure_decision_and_defensive_abort*

Session worktree + MergeBack must use one cleanup owner and typed conflict/failure classification based on git output and exit status, with defensive abort only for merge state created in the current operation. Classification combines git output signals and exit codes so true merge conflicts route to conflict handoff while non-conflict git failures follow cleanup + explicit error handling. Prefer explicit conflict probes over message-fragment matching for contention retry.

### 2026-02-24 | token_efficiency_telemetry_requires_single_runtime_wiring_path | ARCHITECTURE
*Related to: gromit/review-1771929519774405451*

Adding telemetry fields to runtypes/logger schemas is not sufficient by itself; production invocation/stage outputs must carry those values through to orchestrator iteration-log writes or observability will silently drift.

### 2026-02-24 | token_efficiency_routing_needs_strict_category_and_tier_validation | RELIABILITY
*Related to: gromit/review-1771929519774405451*

`token_efficiency.routing` config must validate both override categories and tier values (`low|medium|high`) after normalization, otherwise invalid entries can silently bypass intended utility-routing guardrails.

### 2026-02-24 | issue_ledger_normalization_should_be_isolated_from_semantic_edits | CODE_QUALITY
*Related to: gromit/review-1771936368864075181*

When issue-ledger normalization (ordering/canonical encoding) and semantic issue edits land in the same change, reviewability and merge safety degrade. Keep normalization-only rewrites separate from content changes and enforce that separation in automation.

### 2026-02-24 | cohort_validation_must_reject_nil_lookup_payloads_before_field_access | RELIABILITY
*Related to: gromit/review-1771938913730053167*

Cohort validation paths that call external lookups must treat a nil object as invalid input and return a typed error before dereferencing fields, preventing panic-class failures from malformed integration responses.

### 2026-02-24 | time_injection_only_helps_when_runtime_paths_consume_the_injected_clock | TEST_QUALITY
*Related to: gromit/review-1771938913730053167*

Deterministic clock injection should be wired through runtime code paths (not just declared for tests); otherwise tests can configure a fake clock that production paths ignore, creating false confidence.

## Provisional Learnings

### 2026-02-24 | harness_requires_real_worktree_execution_and_log_wiring | RELIABILITY
*Related to: gromit/review-1771893007120033611*

Harness abstractions alone are insufficient; acceptance requires real worktree execution plus log wiring so metrics/reporting reflect actual runs.

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
