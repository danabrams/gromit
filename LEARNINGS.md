# LEARNINGS

## Confirmed Learnings

### 2026-02-24 | worktree_mergeback_conflict_ownership_and_classification | ARCHITECTURE
*Related to: gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810, gromit/review-1771878486437709843*
*Consolidated from: session_worktree_conflict_classification_and_cleanup_ownership + mergeback_requires_typed_failure_decision_and_defensive_abort*

Session worktree + MergeBack must use one cleanup owner and typed conflict/failure classification based on git output and exit status, with defensive abort only for merge state created in the current operation. Classification combines git output signals and exit codes so true merge conflicts route to conflict handoff while non-conflict git failures follow cleanup + explicit error handling. Prefer explicit conflict probes over message-fragment matching for contention retry.

### 2026-02-23 | fixture_tests_should_assert_schema_and_records_not_prose_tokens | TEST_QUALITY
*Related to: gromit/review-1771886115282672489*

Use structured fixture assertions (parse JSON/JSONL and ledger rows) instead of broad markdown/log token matching.

### 2026-02-24 | gitlink_entries_should_be_blocked_for_ephemeral_worktree_paths | RELIABILITY
*Related to: gromit/review-1771906824758942325*

Ephemeral `.-gromit-*` worktree paths must stay untracked. Add repository-level ignore plus an explicit gitlink guard so mode `160000` entries fail fast before commit/CI.

### 2026-02-24 | repo_hygiene_guards_should_run_in_local_and_ci_paths | PROCESS
*Related to: gromit/review-1771906824758942325*

Repository hygiene checks are most effective when enforced in both local pre-commit hooks and CI entry targets so regressions are blocked regardless of contributor workflow.

### 2026-02-24 | provider_fixture_contracts_should_be_curated_with_provenance_and_deterministic_metadata | TEST_QUALITY
*Related to: gromit/review-1771908518510170783*

Provider fixture suites should commit deterministic, curated artifacts with explicit provenance/refresh metadata and structured schema assertions; avoid brittle prose-token checks and normalize generated ledgers before commit to reduce merge churn.

### 2026-02-24 | gemini_fixture_metadata_and_artifact_requirements_are_test_enforced | TEST_QUALITY
*Related to: gromit/review-1771929160626448252*

Gemini fixture tests require provenance/refresh headers plus specific prompt-delivery artifacts (inline, stdin pipe, file ref). Missing any artifact or header causes failures across the suite.

### 2026-02-24 | gemini_schema_notes_must_reference_model_and_token_cost_evidence | TEST_QUALITY
*Related to: gromit/review-1771929160626448252*

Schema notes fixtures are validated for explicit references to model/token-cost evidence files and concrete prompt-mode comparisons, making those references part of the fixture contract.

### 2026-02-24 | token_efficiency_telemetry_requires_single_runtime_wiring_path | ARCHITECTURE
*Related to: gromit/review-1771929519774405451*

Adding telemetry fields to runtypes/logger schemas is not sufficient by itself; production invocation/stage outputs must carry those values through to orchestrator iteration-log writes or observability will silently drift.

### 2026-02-24 | token_efficiency_routing_needs_strict_category_and_tier_validation | RELIABILITY
*Related to: gromit/review-1771929519774405451*

`token_efficiency.routing` config must validate both override categories and tier values (`low|medium|high`) after normalization, otherwise invalid entries can silently bypass intended utility-routing guardrails.
## Provisional Learnings

### 2026-02-24 | benchmark_execution_and_reporting_require_single_source_truth_and_owner | ARCHITECTURE
*Related to: gromit/review-1771893007120033611, gromit/review-1771897964548429202*
*Consolidated from: benchmark_cli_should_reuse_internal_benchmark_pipeline + benchmark_harness_should_execute_manifest_modes_not_hardcoded_modes + benchmark_report_schema_must_have_single_writer_owner*

Benchmark execution and reporting must have single source-of-truth ownership to prevent drift and overwrite instability. Execution should treat `manifest.modes` as the sole source of truth (hardcoded mode lists are forbidden in production paths). Report generation should have one schema owner/writer path (dual writers targeting the same artifact cause silent overwrite risk). CLI must route through internal pipeline to prevent spec drift from duplicated manifest/selection logic.

### 2026-02-24 | harness_requires_real_worktree_execution_and_log_wiring | RELIABILITY
*Related to: gromit/review-1771893007120033611*

Harness abstractions alone are insufficient; acceptance requires real worktree execution plus log wiring so metrics/reporting reflect actual runs.

### 2026-02-24 | real_probe_fixtures_are_canonical_and_tests_should_follow_them | TEST_QUALITY
*Related to: gromit/review-1771897964548429202*

When fixture artifacts come from a fresh real-provider probe, treat them as canonical evidence. Tests and parsing contracts should evolve to the observed schema rather than forcing fixtures back to stale assumptions.

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
