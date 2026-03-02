# Learnings Archive

---

## Archived

### 2026-02-24 | cohort_validation_must_reject_nil_lookup_payloads_before_field_access | RELIABILITY
*Related to: gromit/review-1771938913730053167*
*Archived: 2026-03-02 — consolidated into nil_safety_boundary_centralized_guard; incident-specific detail with no recent recurrence.*

Cohort validation paths that call external lookups must treat a nil object as invalid input and return a typed error before dereferencing fields, preventing panic-class failures from malformed integration responses.

### 2026-02-28 | codex_usage_attribution_contract_first | ARCHITECTURE
*Related to: gromit-9bhr, gromit-qs2ks*
*Archived: 2026-03-02 — consolidated into runtime_execution_attribution_ownership_contract.*

Codex usage attribution must be contract-first: define normalized usage schema, implement a pure event reducer with precedence rules, and require stream-event matrix tests before integration wiring.

### 2026-02-28 | execution_boundary_ownership_contract | ARCHITECTURE
*Related to: gromit/review-1772054097495408438, gromit/review-1772062103155608386, gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k*
*Archived: 2026-03-02 — consolidated into runtime_execution_attribution_ownership_contract.*

Execution-boundary ownership includes diagnostic survivability: decompose by responsibility while preserving package boundaries, keep single mergeback cleanup ownership with typed conflict handling, use workspace-local state as source of truth with repo-scoped fallback only, and preserve fallback emission paths so failures remain observable.

### 2026-02-28 | nil_guard_centralized_wrapper | RELIABILITY
*Related to: gromit-scrae.1, gromit-scrae.3*
*Archived: 2026-03-02 — consolidated into nil_safety_boundary_centralized_guard.*

Nil-guard fixes must be centralized in one safe-call wrapper; avoid bead scopes that patch every call site directly.

### 2026-02-27 | select_break_only_exits_select_not_enclosing_loop | CODE_QUALITY
*Related to: gromit/review-1772199039639467769*
*Archived: 2026-02-28 — generic language-level behavior; not project-specific under anti-generic archival policy.*

Go's bare `break` inside a `select` block only exits the select, not an enclosing for loop. Use labeled breaks or flag variables for select-in-loop drain patterns to avoid infinite loops on timeout.

### 2026-02-27 | emitter_wiring_boilerplate_suggests_embeddable_mixin | ARCHITECTURE
*Related to: gromit/review-1772199039639467769*
*Archived: 2026-02-28 — speculative refactor idea without repeated cross-bead evidence.*

The WithEmitter/SetEmitter dual-method pattern (builder return vs void setter) appears across Gate, Build, Epilogue, and Review stages — this is a candidate for an embeddable mixin to reduce boilerplate and ensure consistent emitter wiring.

### 2026-02-26 | profile_defaults_are_cross_cutting | ARCHITECTURE
*Related to: gromit/review-1772071010321620531*
*Archived: 2026-02-27 — promoted to rule.*

Profile defaults now affect config resolution, init templates, validation policy, and the validation runner, so changes must be coordinated across these paths.

### 2026-02-26 | timeout_retry_block_metrics_must_cover_all_gate_errors | RELIABILITY
*Related to: gromit/review-1772066330959789077*
*Archived: 2026-02-27 — promoted to rule.*

Retry-block metrics currently depend on error-string matching, so new retry-gate errors (like partial decomposition state) must be included or the timeout retry-block rate will silently undercount.

### 2026-02-25 | agent_test_setup_should_be_shared_helpers | TEST_QUALITY
*Related to: gromit/review-1772054097495408438*
*Archived: 2026-02-27 — redundant with explicit Test Quality rules.*

When multiple agent-related tests need temporary config/backlog scaffolding, centralize the setup in shared helpers to avoid drift and simplify new acceptance tests.

### 2026-02-24 | issue_ledger_normalization_should_be_isolated_from_semantic_edits | CODE_QUALITY
*Related to: gromit/review-1771936368864075181*
*Archived: 2026-02-27 — promoted to rule.*

When issue-ledger normalization (ordering/canonical encoding) and semantic issue edits land in the same change, reviewability and merge safety degrade. Keep normalization-only rewrites separate from content changes and enforce that separation in automation.

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

### 2026-02-25 | cli_call_filters_should_match_command_token_not_string_prefix | TEST_QUALITY
*Related to: gromit/review-1772019888577324268*

Call-log assertions should match the first argv token (for example `codex`) after trim, not raw string prefixes, to avoid false positives like `codex-helper`.

*Archived: narrow implementation detail now subsumed by broader shared tool-call parsing rule; low incremental guidance value.*

### 2026-02-25 | end_to_end_tests_must_assert_cli_surface_not_only_internal_calls | TEST_QUALITY
*Related to: gromit-lqqvs*
*Archived: 2026-02-27 — already captured by explicit test-quality rule requiring end-to-end CLI-surface assertions.*

Tests named end-to-end should execute the CLI path and assert user-visible behavior (output/exit code/artifacts); parser- or model-state-only checks belong in focused unit tests.

### 2026-02-24 | token_efficiency_routing_needs_strict_category_and_tier_validation | RELIABILITY
*Related to: gromit/review-1771929519774405451*
*Archived: 2026-02-27 — already codified in build-process validation rules for token_efficiency routing normalization.*

`token_efficiency.routing` config must validate both override categories and tier values (`low|medium|high`) after normalization, otherwise invalid entries can silently bypass intended utility-routing guardrails.

### 2026-02-25 | repo_boundary_governance_local_ci_and_runtime_artifacts | PROCESS
*Related to: gromit/review-1771986309252130811, gromit/review-1771906824758942325, gromit/review-1771933220448456983, gromit/review-1771936368864075181*
*Archived: 2026-02-27 — already covered by runtime-artifact and local+CI hygiene guard rules.*

Repository boundary governance should be enforced as one policy: local+CI guards must block gitlinks/ephemeral runtime artifacts, and only deterministic curated fixtures are allowed in versioned paths.

### 2026-02-27 | json_encoding_helpers_should_consolidate_to_single_source | CODE_QUALITY
*Related to: gromit/review-1772155497602965256*
*Archived: 2026-02-27 — implementation-specific refactor guidance; not a durable cross-bead rule.*

Consolidating duplicate JSON-encoding helpers (marshalJSONList, encodeJSONStrings) into a single canonical function (tracker.EncodeMetadataJSONList) eliminates behavioral drift between callers and simplifies metadata encoding across packages.

### 2026-02-27 | observability_contract_completeness | ARCHITECTURE
*Related to: gromit/review-1771929519774405451, gromit/review-1771938913730053167, gromit/review-1771893007120033611, gromit/review-1772059511071909600, gromit/review-1772155497602965256*
*Promoted to Architecture rule: repeated unknown-attribution failures block efficiency analysis and experiment decisions; systemic reliability issue.*

Observability completeness is one contract: runtime attribution, iteration-row persistence, and IterationLog/IterationMetric field parity must fail closed together, with per-field diagnostics for missing row/attribution/numeric values.

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

### 2026-02-25 | shared_test_helpers_should_not_export_mutable_global_maps | RELIABILITY
*Related to: gromit-ay3oy, gromit-jm77m*

Shared helper packages should avoid exposing mutable global maps because external mutation creates hidden coupling and order-dependent test behavior; prefer immutable internals with copy/accessor APIs.

*Promoted to Test Quality rule: prevents order-dependent tests and hidden cross-suite coupling.*

### 2026-02-25 | stream_event_matrix_contract_test_required | PROCESS
*Related to: runtime_path_parity_for_observability*

Any bead touching provider stream usage/event handling must add a stream-event matrix contract test and verify post-run completeness assertions.

*Promoted to Process rule: directly targets repeated telemetry-attribution regressions and stuck-bead recurrence.*

### 2026-02-27 | run_completion_status_should_reflect_failure_reason | RELIABILITY
*Related to: gromit/review-1772191027352243303*

Run completion UI state should map failure reasons to a failed status so dashboards do not report a running or completed state after an unsuccessful run.

*Promoted to Process rule: incorrect success/running status corrupts operational dashboards and masks true failure rates.*

### 2026-02-25 | refactor_guardrail_tests_should_validate_structure_directly | TEST_QUALITY
*Related to: gromit/review-1772062103155608386*

Refactor guardrail tests should parse and validate actual declarations (for example via AST) rather than rely on naming heuristics that can pass even when required exported surface drifts.

*Promoted to Test Quality rule: heuristic tests have allowed surface drift; structural assertions are more stable under refactor.*
