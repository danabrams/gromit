# Project Rules

These rules represent high-leverage control points that have either caused repeated failures or represent confirmed best practices. Rules are enforced through code review, automated checks, and process gates.

## Architecture <!-- phases: red, build, green, refactor, review -->

### Benchmark runtime must treat manifest.modes as the sole execution source of truth
Hardcoded mode lists are forbidden in production paths. All benchmark execution must derive modes from `manifest.modes`. This prevents spec drift and ensures manifest-driven experiments are valid.

**Enforcement:** Code review for benchmark path changes; unit tests validating manifest-only mode derivation.

### Artifacts with external consumers must have a single schema writer owner
Benchmark and retro reports have external consumers (dashboards, analytics). These artifacts must have exactly one schema writer path. Dual-writer paths targeting the same output are forbidden—they cause silent overwrite and contract drift.

**Enforcement:** Architecture review for new report generation; test coverage on schema contract boundaries.

### All run termination paths must call one shared metrics-persist epilogue
Observability fields (cost/tokens/duration/model/provider/current_run_row attribution) must be produced through the same runtime execution path used in production. Pre-launch/invocation failures must still emit non-empty attribution (or explicit sentinel attribution) and a typed failure reason. Any alternate/test path must have parity contract tests proving identical field population semantics and non-empty current-run row generation. Enforce a single shared persist-epilogue invoked by every return path with tests enumerating each exit.

**Enforcement:** Parity contract tests for each exit path (success, build-fail, validation-fail, pre-launch, timeout); CI gate on persist-epilogue coverage.

### Pipeline methods: validate deps first, render, post-process for change detection, with single writer/owner
Pipeline methods follow the pattern: typed input/output structs → validate dependencies first → renderer processing → post-processing change detection. For shared artifacts, enforce one writer/owner path and add contract tests that fail on duplicate writers.

**Enforcement:** Keep pipeline tests next to command files; add contract tests before merging dual-writer changes.

## Process

### Beads touching telemetry or usage paths require telemetry contract validation
If a bead touches usage accounting, provider stream-event handling, or retro efficiency formatting, mandatory validation must also include telemetry contract suites covering provider merge semantics and iteration-row completeness before merge.

**Enforcement:** Bead scope tagging; CI gate on telemetry-tagged beads requiring contract suite pass.

### Beads touching stream events must add event-matrix contract tests
Any bead touching provider stream usage/event handling must add a stream-event matrix contract test (turn/response/result paths) and verify post-run completeness assertions still fail/pass in the expected scenarios.

**Enforcement:** Bead scope tagging for stream-event paths; CI gate requiring matrix contract test pass before merge.

### Ephemeral session worktree paths must remain untracked via repo ignore plus gitlink mode guard
Ephemeral `.-gromit-*` worktree paths must stay untracked in version control. Repository-level ignore plus explicit gitlink guard (mode `160000` fails fast before commit/CI) prevents accidental repo integrity corruption.

**Enforcement:** Local pre-commit hooks and CI entry targets; audit gitlink entries in hook output.

### Bead failure decomposition must be enforced by preflight
On bead failure: for broad/high-risk scope (cross-package, regression umbrella titles, or 6+ files), decompose after the first failure; otherwise decompose after 2 consecutive failures. If failure signature is invocation/pre-launch (duration_ms=0 and attribution unresolved), forbid same-scope retry and require immediate diagnostic decomposition. Preflight must verify decomposition children exist and parent is blocked before any retry is allowed. Missing child-bead links is a hard error (not warning), with idempotent child creation and explicit `discovered-from` linkage required. Telemetry/usage children split: (1) event-merge, (2) completeness, (3) attribution. Block parent retries until a child lands. If broad-scope signals indicate cross-package or 6+ files, planning must fail until decomposition child beads (with expected_outputs) are linked.

**Enforcement:** Preflight gate on bead retry; auditable decomposition-attempt event; broad-scope detection at first failure; block parent retries until child lands; planning validator hard-fail for broad-scope signals without linked children.

### Run status publication must derive from terminal failure classification
Any non-nil failure reason sets failed state and suppresses running/completed success states. Run completion UI must not report a running or completed state after an unsuccessful run.

**Enforcement:** Code review on run-status publication paths; CI test asserting failure-reason → failed-status mapping.

### On timeout risk, trigger decomposition earlier for high-risk scope
Apply timeout-first decomposition at >=60% elapsed budget when complexity signals are high (broad title signals, multi-type scope, or prior retry), and forbid same-scope retry before decomposition or explicit escalation is recorded. This reduces timeout/rework loops by preventing late-stage surprises.

**Enforcement:** Orchestrator/runner validates elapsed budget and complexity signals before allowing same-scope retry; telemetry gates on escalation recording.

### Usage accounting must use explicit snapshots and one canonical merge strategy
Usage accounting must use explicit before/after snapshots for every phase and one canonical merge strategy for provider stream events. Pre-launch/invocation failures must still emit non-empty attribution (or explicit sentinel attribution) and a typed failure reason; blank model/provider fields are forbidden on any iteration outcome. Treat any non-empty run with `model=unknown` or `provider=unknown` attribution as a data-quality failure: mark the run as data-quality-failed, auto-create/link a blocking bead, and block keep/revert experiment decisions until one complete current-run dataset with known attribution is recorded. Fail the iteration when usage exists but model/provider attribution is unknown. Output must remain fail-closed (`insufficient_current_run_data`, deltas `N/A`). Stratified SPC must additionally require >=10 samples and non-zero variance per stratum, otherwise emit `insufficient_stratum_data` and suppress anomaly classification.

**Enforcement:** Runtime telemetry validator; experiment decision gate; unknown-attribution detector in post-run validation; iteration-level attribution guard; experiment.json schema validator on keep/revert/extend; pre-launch attribution contract tests; stratum validity gate (sample count + variance check).

## Reliability

### Session worktree + MergeBack must use one cleanup owner and typed conflict classification
Session worktree lifecycle and MergeBack must use one explicit owner for merge-attempt and cleanup sequencing. Conflict classification must combine git output signals and exit codes so true merge conflicts route to conflict handoff while non-conflict git failures follow cleanup + explicit error handling. Defensive abort only for merge state created in the current operation.

**Enforcement:** Code review on MergeBack path changes; integration tests validating merge state detection.

### Repository hygiene checks must run in both local and CI paths
Repository hygiene checks (gitlink guards, worktree exclusions) are most effective when enforced in both local pre-commit hooks and CI entry targets so regressions are blocked regardless of contributor workflow.

**Enforcement:** Hook audit on every CI run; contributor workflow documentation.

### Runtime artifacts must stay out of git; only deterministic fixtures are versioned
Machine-local runtime state (`.dolt/`, `.doltcfg/`, `beads_gromit/`, lock files, `.gromit/state.json`, `.gromit/stats.json`, `.gromit/interactive-state.json`) and timestamped run/report outputs must be ignored and untracked. Raw run outputs belong under ignored paths (for example `.gromit/reports/runs/`). Versioned artifacts must be deterministic fixtures under designated fixture paths; for report artifacts, use curated deterministic files (for example `.gromit/reports/curated/`).

**Enforcement:** Repo ignore coverage, staged-file guards in pre-commit/CI, and review rejection for timestamped runtime output commits.

## Test Quality

### Tool-call log parsing in tests must use shared helpers to prevent cross-suite drift
Tool-call log parsing in tests must use shared helpers (for example `test/toolcalls`) rather than suite-local parsers to prevent cross-suite behavior drift on whitespace handling and command matching.

**Enforcement:** Code review on test helper changes; grep for duplicated call-log parsing outside shared helpers.

### Shared helper APIs must not expose mutable global maps or slices
Shared helper packages (especially test helpers) must not expose mutable global maps/slices. Keep mutable sources unexported and provide copy/accessor functions. This prevents order-dependent tests and hidden cross-suite coupling.

**Enforcement:** Code review on helper API changes; grep for exported `var` maps/slices in shared packages.

### End-to-end tests must execute CLI path and assert user-visible behavior
Acceptance tests must verify behavior through public API/CLI surfaces, not private helpers. Tests named `end_to_end` must execute the CLI command path and assert exit code + user-visible output/artifacts; parser- or model-state-only checks belong in focused unit tests.

**Enforcement:** Code review on test naming; CI lint for tests named `end_to_end` that don't invoke CLI.

### Profile-default changes must be coordinated across all consuming paths
Profile-default changes are cross-cutting and incomplete unless config resolution, init templates, validation policy, and validation runner wiring are updated together with parity tests.

**Enforcement:** Code review checklist on profile/default changes; CI parity tests for config resolution vs init templates vs validation policy.

### Retry-block metrics must classify by typed error categories
Retry-block metrics must classify by typed error categories, not string matching. Any new retry-gate error type must be registered in metrics classification before merge.

**Enforcement:** Code review on retry-gate additions; CI gate requiring metrics classification coverage for all retry error types.

### Issue-ledger normalization and semantic edits must be separate
Issue-ledger normalization and semantic edits must be split into separate beads/commits; mixed changes are blocked.

**Enforcement:** Code review on ledger changes; automation rejects commits mixing normalization-only and semantic edits.

### Refactor guardrail tests must validate structural declarations
Refactor guardrail tests must validate structural declarations (AST/compile-time contracts) instead of naming or string heuristics. For architecture guardrails, require AST- or type-level structural assertions when exported surface invariants are part of acceptance criteria. Source-reading tests (`os.ReadFile`+`strings.Contains` on `.go` files) break on refactoring and are insufficient.

**Enforcement:** Code review on guardrail test changes; CI lint for source-reading test patterns in guardrail suites.

### Fixture tests should assert schema and records, not prose tokens
Use structured fixture assertions (parse JSON/JSONL and ledger rows) instead of broad markdown/log token matching. Real-provider probe fixtures are canonical and should drive parser/schema updates rather than forcing fixtures back to stale assumptions.

**Enforcement:** Code review on fixture changes; require structured assertions in test review.
