# Project Rules

These rules represent high-leverage control points that have either caused repeated failures or represent confirmed best practices. Rules are enforced through code review, automated checks, and process gates.

## Architecture

### Benchmark runtime must treat manifest.modes as the sole execution source of truth
Hardcoded mode lists are forbidden in production paths. All benchmark execution must derive modes from `manifest.modes`. This prevents spec drift and ensures manifest-driven experiments are valid.

**Enforcement:** Code review for benchmark path changes; unit tests validating manifest-only mode derivation.

### Artifacts with external consumers must have a single schema writer owner
Benchmark and retro reports have external consumers (dashboards, analytics). These artifacts must have exactly one schema writer path. Dual-writer paths targeting the same output are forbidden—they cause silent overwrite and contract drift.

**Enforcement:** Architecture review for new report generation; test coverage on schema contract boundaries.

### Pipeline methods: validate deps first, render, post-process for change detection, with single writer/owner
Pipeline methods follow the pattern: typed input/output structs → validate dependencies first → renderer processing → post-processing change detection. For shared artifacts, enforce one writer/owner path and add contract tests that fail on duplicate writers.

**Enforcement:** Keep pipeline tests next to command files; add contract tests before merging dual-writer changes.

## Process

### Ephemeral session worktree paths must remain untracked via repo ignore plus gitlink mode guard
Ephemeral `.-gromit-*` worktree paths must stay untracked in version control. Repository-level ignore plus explicit gitlink guard (mode `160000` fails fast before commit/CI) prevents accidental repo integrity corruption.

**Enforcement:** Local pre-commit hooks and CI entry targets; audit gitlink entries in hook output.

### On timeout risk, trigger decomposition earlier for high-risk scope
Apply timeout-first decomposition at >=60% elapsed budget when complexity signals are high (broad title signals, multi-type scope, or prior retry), and forbid same-scope retry before decomposition or explicit escalation is recorded. This reduces timeout/rework loops by preventing late-stage surprises.

**Enforcement:** Orchestrator/runner validates elapsed budget and complexity signals before allowing same-scope retry; telemetry gates on escalation recording.

### Usage accounting must use explicit snapshots and fail-closed on missing data
Usage accounting must use explicit before/after snapshots for every phase (red/green/refactor/validate) and a single merge strategy for provider stream events. Mixing raw totals and deltas in one run is forbidden. If completeness checks fail (missing rows/snapshots with iterations present), mark the run `data_quality_blocked`, auto-create/link a blocking bead, and block keep/revert experiment decisions until one complete current-run dataset is recorded. Output must remain fail-closed (`insufficient_current_run_data`, deltas `N/A`).

**Enforcement:** Runtime telemetry validator; experiment decision gate; experiment.json schema validator on keep/revert/extend.

## Reliability

### Session worktree + MergeBack must use one cleanup owner and typed conflict classification
Session worktree lifecycle and MergeBack must use one explicit owner for merge-attempt and cleanup sequencing. Conflict classification must combine git output signals and exit codes so true merge conflicts route to conflict handoff while non-conflict git failures follow cleanup + explicit error handling. Defensive abort only for merge state created in the current operation.

**Enforcement:** Code review on MergeBack path changes; integration tests validating merge state detection.

### Repository hygiene checks must run in both local and CI paths
Repository hygiene checks (gitlink guards, worktree exclusions) are most effective when enforced in both local pre-commit hooks and CI entry targets so regressions are blocked regardless of contributor workflow.

**Enforcement:** Hook audit on every CI run; contributor workflow documentation.

## Test Quality

### Fixture tests should assert schema and records, not prose tokens
Use structured fixture assertions (parse JSON/JSONL and ledger rows) instead of broad markdown/log token matching. Real-provider probe fixtures are canonical and should drive parser/schema updates rather than forcing fixtures back to stale assumptions.

**Enforcement:** Code review on fixture changes; require structured assertions in test review.
