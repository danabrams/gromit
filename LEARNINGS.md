# LEARNINGS

## Confirmed Learnings

### 2026-02-23 | session_worktree_conflict_classification_and_cleanup_ownership | ARCHITECTURE
*Related to: gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810*

Session worktree lifecycle should use one explicit owner for merge-attempt and cleanup sequencing. Conflict classification must combine git output signals and exit codes so true merge conflicts route to conflict handoff while non-conflict git failures follow cleanup + explicit error handling. Prefer explicit conflict probes over message-fragment matching for contention retry.

### 2026-02-23 | mergeback_requires_typed_failure_decision_and_defensive_abort | RELIABILITY
*Related to: gromit/review-1771878486437709843*

`MergeBack` should classify failures through a typed decision (including exit-code capture when available), then apply one cleanup owner path. For non-conflict failures, probe merge state and defensively abort only when `MERGE_HEAD` exists so stale merge state does not leak while non-merge failures are not mislabeled.

### 2026-02-23 | fixture_tests_should_assert_schema_and_records_not_prose_tokens | TEST_QUALITY
*Related to: gromit/review-1771886115282672489*

Use structured fixture assertions (parse JSON/JSONL and ledger rows) instead of broad markdown/log token matching.

## Provisional Learnings

### 2026-02-24 | benchmark_cli_should_reuse_internal_benchmark_pipeline | ARCHITECTURE
*Related to: gromit/review-1771893007120033611*

Prefer routing benchmark CLI through the internal benchmark pipeline to prevent spec drift from duplicated manifest/selection logic.

### 2026-02-24 | harness_requires_real_worktree_execution_and_log_wiring | RELIABILITY
*Related to: gromit/review-1771893007120033611*

Harness abstractions alone are insufficient; acceptance requires real worktree execution plus log wiring so metrics/reporting reflect actual runs.

### 2026-02-24 | benchmark_harness_should_execute_manifest_modes_not_hardcoded_modes | CORRECTNESS
*Related to: gromit/review-1771897964548429202*

Benchmark execution should treat `manifest.modes` as the source of truth. Hardcoding mode lists in runtime paths creates spec drift and invalidates manifest-driven experiments.

### 2026-02-24 | benchmark_report_schema_must_have_single_writer_owner | ARCHITECTURE
*Related to: gromit/review-1771897964548429202*

Report generation should have one schema owner/writer path. Dual writers targeting the same artifact path cause silent overwrite risk and unstable downstream contracts.

### 2026-02-24 | real_probe_fixtures_are_canonical_and_tests_should_follow_them | TEST_QUALITY
*Related to: gromit/review-1771897964548429202*

When fixture artifacts come from a fresh real-provider probe, treat them as canonical evidence. Tests and parsing contracts should evolve to the observed schema rather than forcing fixtures back to stale assumptions.

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
