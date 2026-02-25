# Learnings Archive

---

## Archived

### 2026-02-24 | repo_hygiene_guards_should_run_in_local_and_ci_paths | PROCESS
*Related to: gromit/review-1771906824758942325*

Repository hygiene checks are most effective when enforced in both local pre-commit hooks and CI entry targets so regressions are blocked regardless of contributor workflow.

*Archived from confirmed: generic process advice; superseded by explicit project-specific gitlink/worktree guard learning.*

### 2026-02-25 | policy_guard_tests_should_cover_pass_and_fail_paths | TEST_QUALITY
*Related to: gromit/review-1771989652251291587*

Repository policy guards are safest when tests cover both positive and negative paths (canonical semantic edits pass, normalization-only rewrites pass, non-canonical and mixed semantic+normalization edits fail with actionable messages).

*Archived: generic testing advice (cover pass/fail paths) without project-specific mechanics.*

### 2026-02-25 | centralized_policy_scripts_pair_well_with_scenario_matrix_tests | RELIABILITY
*Related to: gromit/review-1771989652251291587*

Centralized shell policy scripts are most regression-resistant when Go tests exercise a full scenario matrix (pass and fail cases) against the script's real process behavior and user-facing guidance text.

*Archived: generic scenario-matrix testing guidance; useful but not specific enough to this codebase.*

### 2026-02-24 | post_fix_reviews_should_re_run_touched_package_suites | PROCESS
*Related to: gromit/review-1771938913730053167*

After landing targeted fixes, re-running tests for touched packages is a lightweight regression gate that quickly confirms behavioral integrity without waiting for full-suite signal.

*Archived: generic post-fix test rerun advice; already reflected by existing process rules.*

### 2026-02-23 | fixture_tests_should_assert_schema_and_records_not_prose_tokens | TEST_QUALITY
*Related to: gromit/review-1771886115282672489*

Use structured fixture assertions (parse JSON/JSONL and ledger rows) instead of broad markdown/log token matching.

*Archived: already codified in current rules (schema/records over prose-token assertions); keep rules as source of truth.*

## Promoted to Rules

### 2026-02-24 | benchmark_execution_and_reporting_require_single_source_truth_and_owner | ARCHITECTURE
*Related to: gromit/review-1771893007120033611, gromit/review-1771897964548429202*
*Consolidated from: benchmark_cli_should_reuse_internal_benchmark_pipeline + benchmark_harness_should_execute_manifest_modes_not_hardcoded_modes + benchmark_report_schema_must_have_single_writer_owner*

Benchmark execution and reporting must have single source-of-truth ownership to prevent drift and overwrite instability. Execution should treat `manifest.modes` as the sole source of truth (hardcoded mode lists are forbidden in production paths). Report generation should have one schema owner/writer path (dual writers targeting the same artifact cause silent overwrite risk). CLI must route through internal pipeline to prevent spec drift from duplicated manifest/selection logic.

*Promoted to Architecture rule: repeated benchmark drift/overwrite risk; architectural and prevents silent contract divergence.*

### 2026-02-25 | shared_tool_call_filtering_prevents_cross_suite_behavior_drift | ARCHITECTURE
*Related to: gromit/review-1772019888577324268*

Tool-call log parsing should live in one shared helper (`test/toolcalls`) and be reused by contract/e2e wrappers; duplicated per-suite parsing quickly drifts on whitespace handling and command matching.

*Promoted to Process rule: concrete repeated testing drift mode with high regression risk once suites fork helper logic.*
