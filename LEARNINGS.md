# LEARNINGS

## Confirmed Learnings

### 2026-02-23 | review | ARCHITECTURE
*Related to: gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k*

Session worktree lifecycle behavior needs a single, explicit ownership model for merge-attempt and cleanup sequencing. Error taxonomy should distinguish true merge conflicts from generic git failures, and contention retry should prefer explicit probes over message-fragment matching.

### 2026-02-23 | merge_conflict_classification_uses_git_output | CORRECTNESS
*Related to: gromit/review-1771861273153074810*

Git merge conflicts can appear in command output while errors remain generic (`exit status 1`). Classify conflicts from output plus error so merge cleanup runs reliably.

## Provisional Learnings

## Archived Learnings
