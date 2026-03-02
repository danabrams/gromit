# LEARNINGS

## Confirmed Learnings

### 2026-03-02 | subprocess_context_threading_and_platform_guards | CORRECTNESS
*Related to: gromit/review-1772480612920810000, gromit-vj714, gromit-9olfs, gromit-5ag6c*

Subprocess code has two recurring correctness gaps: (1) context threading — callers that receive a live ctx must thread it all the way to subprocess creation; using context.Background() mid-chain silently ignores cancellation and causes up to DefaultCommandTimeout of lag after user cancel; (2) platform-specific paths — any code walking /proc/<pid>/... is Linux-only; on macOS it silently no-ops; add //go:build linux tags or explicit doc comments on all affected functions. Both gaps share the same root: code that looks correct locally fails silently in a different environment.

### 2026-03-02 | package_level_var_injection_requires_cleanup | TEST_QUALITY
*Related to: gromit/review-1772480612920810000, gromit-o38nb*

Package-level var injection for test fakes (e.g. `killDescendantsOnCancelFn = procutil.KillDescendantsOnCancel`) must always be paired with a `t.Cleanup` restore. Tests that override a var without restoring it pollute all subsequent tests in the same `go test` run. Add a restore helper (e.g. `restoreXxxFns(t *testing.T)`) that captures current values and registers `t.Cleanup` to reset them. Add a comment above the var block noting the restore requirement.

### 2026-03-02 | config_accessor_receiver_nil_safety | CODE_PATTERNS
*Related to: gromit/review-1772480612920810000*

Config accessor receivers: `*bool` field accessors already guard via the pointer-nil check pattern. Plain `bool` field accessors on pointer receivers (e.g. `func (c *Config) IsXxx() bool { return c.Field }`) do not auto-guard — a nil receiver panics. Use a value receiver or add an explicit nil check. The nullable duration pattern for optional timeouts: `if field > 0 { return field }; return DefaultXxx` (see `Client.commandTimeout()`); new fields with optional durations follow this shape.

### 2026-03-02 | boundary_sensitive_runtime_behavior_fail_closed_centralized | ARCHITECTURE
*Related to: gromit/review-1772054097495408438, gromit/review-1772062103155608386, gromit-9948, gromit-9949, gromit-y7flm, gromit-z9z2k, gromit/review-1771861273153074810, gromit/review-1771878486437709843, gromit/review-1772059511071909600, gromit/review-1772199039639467769, gromit-9bhr, gromit-qs2ks, gromit/review-1771938913730053167, gromit-scrae.1, gromit-scrae.3*
*Consolidated from: runtime_execution_attribution_ownership_contract + nil_safety_boundary_centralized_guard*

Boundary-sensitive runtime behavior must be fail-closed and centralized: attribution completeness, lifecycle persistence, and nil/callback safety should be enforced through shared boundary contracts/wrappers rather than distributed call-site patches. Enforce one execution path for attribution/persistence, typed failure reasons, stream-event matrix coverage before integration wiring, and centralized safe-call wrappers that reject nil payloads before field access.

### 2026-02-25 | provider_fixture_governance_schema_first_deterministic | TEST_QUALITY
*Related to: gromit/review-1771886115282672489, gromit/review-1771908518510170783, gromit/review-1771929160626448252, gromit/review-1771897964548429202*
*Consolidated from: fixture_tests_should_assert_schema_and_records_not_prose_tokens + provider_fixture_contracts_should_be_curated_with_provenance_and_deterministic_metadata + gemini_fixture_metadata_and_artifact_requirements_are_test_enforced + gemini_schema_notes_must_reference_model_and_token_cost_evidence + real_probe_fixtures_are_canonical_and_tests_should_follow_them*

Provider fixture governance should be schema-first and deterministic: canonical real-probe fixtures with provenance metadata, explicit required artifacts, and structured assertions (never prose-token checks). Fixture contracts must include deterministic schema assertions and provenance metadata; prose-token assertions are forbidden.

## Provisional Learnings

*No provisional learnings at this time.*

## Archived Learnings
*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
