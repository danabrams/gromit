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

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
