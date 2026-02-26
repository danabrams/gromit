---
created: 2026-02-25T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T00:58:19Z"
id: gemini-provider-adapter
source_spec: gemini-provider-adapter
---

# Gemini Provider Adapter Implementation Plan

**Goal:** Add a production-ready `GeminiProvider` that implements `provider.Provider` and participates in configured multi-provider routing with correct parsing, retry, and cost attribution behavior.

**Architecture:** Implement Gemini using the existing Codex-style subprocess/provider pattern, with Gemini-specific JSON and JSONL parsers plus conservative failure classification, then wire provider construction and agent preset defaults to match spike findings (stdin-first).

**Tech Stack:** Go, table-driven unit tests, contract tests, fixture-backed parser tests

**Spec:** `.gromit/specs/gemini-provider-adapter.md`

---

## Architecture

**Overview:**  
Implement Gemini as a third first-class `provider.Provider` by mirroring Codex’s subprocess + retry architecture, with Gemini-specific JSON/JSONL parsing helpers and config-driven model/cost wiring.

**Key Components:**
1. **`internal/provider/gemini.go`**: New `GeminiProvider` implementing all 8 `Provider` methods, including `Run`, `StreamRun`, `RunValidation`, retry policy, and usage/scope/validation checks.
2. **`internal/provider/gemini_helpers.go`**: Parsing and classification helpers (`classifyGeminiError`, stream event parser, JSON parser, text/usage extraction).
3. **`internal/provider/build.go`**: Extend `BuildProvidersFromConfig` to recognize `gemini` and build it from `providers.gemini` (no default tier map fallback).
4. **`internal/provider/provider.go`**: Extend `legacyModelToTier` for Gemini model aliases.
5. **`internal/agent/resolve.go`**: Change Gemini preset to `Stdin` delivery by default (with config override compatibility retained).
6. **Tests + fixtures**: Provider unit tests with injected subprocess fn, contract tests for invocation shape, and fixture-backed parser/classifier tests.

**Integration Points:**
- Existing provider architecture already supports multi-provider routing via `BuildProvidersFromConfig` + router ratios.
- Existing config schema (`ProviderDef`) already supports `binary`, `flags`, `models`, `reasoning_effort`, and cost fields, so Gemini can plug in with minimal schema churn.
- Existing agent prompt-delivery modes already include `Stdin`, making Gemini preset alignment straightforward.

**Data Flow:**
- **Run**: select model from tier -> build `gemini -m <model> --output-format json` command -> send prompt via stdin -> parse JSON response/tokens -> compute/propagate cost.
- **StreamRun**: select model -> run with `--output-format stream-json` -> line-by-line JSONL parse -> emit handler/tool events -> accumulate assistant text + usage -> classify/retry transient failures.
- **RunValidation**: reuse provider-standard numbered command prompt format and parse pass/fail markers with shared helpers.

**Files to Modify:**
- `internal/provider/build.go` - add Gemini provider constructor wiring and recognition.
- `internal/provider/provider.go` - add Gemini legacy model mappings.
- `internal/agent/resolve.go` - set Gemini preset default delivery to stdin.
- `internal/agent/resolve_test.go` - update preset expectations.
- `internal/provider/build_test.go` and related constructor tests - cover Gemini config wiring and failure cases.

**Files to Create:**
- `internal/provider/gemini.go` - provider implementation.
- `internal/provider/gemini_helpers.go` - parsing/error helper surface.
- `internal/provider/gemini_test.go` - provider method tests (including retry behavior).
- `test/contracts/gemini_contract_test.go` - invocation contract tests.
- `test/fixtures/gemini_stream_success.jsonl`, `test/fixtures/gemini_stream_failure.jsonl`, `test/fixtures/gemini_success.txt` - canonical fixture set from spike captures.

**Tradeoffs:**
- **Codex-style subprocess pattern vs Claude-client wrapper:** choose Codex-style for Gemini because output handling and retry/error semantics are CLI-event driven and closer to Codex.
- **Stdin-first delivery vs `-p` default:** choose stdin-first for reliability from spike findings; keep `-p` as fallback path for diagnostics/short prompts.
- **Conservative error classification:** implement auth/rate/transport detection with conservative matching and default to `other` for uncertain signatures until more live failures are captured.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Validate `GeminiProvider` behavior in isolation with injected subprocess function (`runFn`) and deterministic stdout/stderr fixtures.
2. **Contract Tests**: Validate end-to-end CLI invocation shape (args, prompt delivery mode, model selection) with the existing contract harness.
3. **Manual/Smoke Verification**: Optional local CLI-authenticated run to confirm unverified live error signatures (auth/rate/transport), without blocking core implementation.

**Key Test Cases:**
- `Name()` returns `gemini`.
- `ModelForTier()` returns configured model; returns empty/unchanged fallback per implementation decision when tier missing.
- `Run()`:
  - invokes `--output-format json`.
  - parses output text + token fields from JSON response.
  - computes `CostUSD` from provider config when direct cost absent.
  - classifies failure category from stderr.
- `StreamRun()`:
  - invokes `--output-format stream-json`.
  - parses JSONL lines by event type.
  - accumulates assistant content from `message` events.
  - captures usage from `result.stats`.
  - returns failure result on error events/exit failures.
- Retry behavior:
  - retries exactly up to max for `rate_limited` and `transport_disconnect`.
  - applies expected backoff sequence.
  - does not retry `auth` or `other`.
- `RunValidation()`:
  - renders numbered command block.
  - detects `VALIDATION_PASSED` marker.
  - detects `SCOPE_TOO_LARGE:` marker + reason extraction.
- `IsUsageLimitError()`:
  - true for known Gemini rate/quota signatures.
  - false for unrelated failures.
- Constructor wiring:
  - `providers.gemini` creates Gemini provider.
  - missing/invalid Gemini config fails clearly.
  - no regressions in Claude/Codex provider creation.
- Agent preset:
  - Gemini preset defaults to `Stdin`.
  - config/custom agent overrides remain functional.

**Mocking Strategy:**
- Mock subprocess execution via injectable `runFn` in provider tests for deterministic outputs and error paths.
- Use fixture files for representative JSON and stream-json payloads instead of hand-assembling long inline strings.
- Keep contract tests using existing fake binary/call-log harness style used by Claude/Codex contract tests.

**Coverage Goals:**
- All 8 `Provider` methods on `GeminiProvider`.
- All helper functions in `gemini_helpers.go`.
- Retry gate correctness (category + attempt bound + backoff schedule).
- Parser resilience for malformed/partial JSONL lines.
- Cost/token propagation to `provider.Result` fields.

**Test Organization:**
- Primary unit tests in `internal/provider/gemini_test.go` (table-driven where practical).
- Helper-focused tests co-located or split into `internal/provider/gemini_helpers_test.go`.
- Contract tests in `test/contracts/gemini_contract_test.go` with `//go:build contract`.
- Reuse `test/fixtures/gemini/*` live-capture assets; add top-level compatibility fixtures only where required by acceptance criteria tooling.

## Implementation Tasks

### Task 1: Build Gemini parser and classifier helpers

**Files:**
- Create: `internal/provider/gemini_helpers.go`
- Create: `internal/provider/gemini_helpers_test.go`
- Reuse/Read fixtures from: `test/fixtures/gemini/*.json`, `test/fixtures/gemini/*.jsonl`, `test/fixtures/gemini/errors/*.txt`

**What to Do:**
Implement helper functions for JSON result parsing, JSONL event parsing, assistant text extraction, token/cost extraction, and stderr-based failure classification. Cover setup failures (`command not found`) as `other` for now, with conservative placeholders for auth/rate/transport patterns from spike findings.

**Acceptance Criteria:**
- JSON and stream-json fixture payloads parse into stable structures without panics.
- `extractGeminiUsage` populates input/output/cached token counts and computed cost when config rates exist.
- `classifyGeminiError` maps known signatures to configured categories and defaults to `other`.

**Dependencies:** None

### Task 2: Implement `GeminiProvider` core run paths with bounded retry

**Files:**
- Create: `internal/provider/gemini.go`
- Create/Modify: `internal/provider/gemini_test.go`

**What to Do:**
Add `GeminiProvider` struct and implement `Name`, `ModelForTier`, `Run`, and `StreamRun` using subprocess invocation with stdin-first prompt delivery and configurable fallback behavior for inline prompt mode. Add bounded transient retry (max 2 retries; 250ms, 750ms, 1500ms backoff) for `rate_limited` and `transport_disconnect` only.

**Acceptance Criteria:**
- Compile-time assertion confirms `GeminiProvider` satisfies `provider.Provider`.
- `Run` invokes Gemini with `--output-format json` and returns parsed output/tokens/cost.
- `StreamRun` parses JSONL events, accumulates assistant output, and retries only transient failure categories.

**Dependencies:** Task 1

### Task 3: Implement validation and provider-level detection helpers

**Files:**
- Modify: `internal/provider/gemini.go`
- Modify: `internal/provider/gemini_test.go`
- Optional shared helper reference: `internal/provider/helpers.go`

**What to Do:**
Implement `RunValidation`, `IsUsageLimitError`, `IsValidationPassed`, and `IsScopeTooLarge` for Gemini. Reuse existing shared marker checks where possible and ensure validation command formatting matches existing Claude/Codex provider behavior.

**Acceptance Criteria:**
- `RunValidation` builds numbered command prompts and executes against configured Gemini tier/model.
- Validation pass and scope-too-large detection use existing marker conventions.
- Usage-limit detection matches known Gemini rate/quota signatures from spike notes.

**Dependencies:** Task 2

### Task 4: Wire Gemini into provider construction and legacy tier mapping

**Files:**
- Modify: `internal/provider/build.go`
- Modify: `internal/provider/provider.go`
- Modify: `internal/provider/build_test.go`
- Modify (if needed): `internal/runner/constructor_test.go`

**What to Do:**
Extend provider builder logic to recognize `gemini` in `cfg.Providers`, construct `GeminiProvider` from `ProviderDef`, and preserve existing Claude/Codex behavior unchanged. Add Gemini entries to `legacyModelToTier` for compatibility (`gemini-3.1-pro`, `gemini-3-pro`, `gemini-3-flash`).

**Acceptance Criteria:**
- `BuildProvidersFromConfig` instantiates Gemini when configured and errors clearly for invalid Gemini definitions.
- Router selection continues to work with Gemini ratios.
- Existing Claude/Codex builder tests remain green.

**Dependencies:** Task 2

### Task 5: Align Gemini agent preset with stdin-first invocation

**Files:**
- Modify: `internal/agent/resolve.go`
- Modify: `internal/agent/resolve_test.go`

**What to Do:**
Update `resolveByName("gemini")` preset to use `Stdin` prompt delivery by default, while preserving custom-agent override semantics and compatibility with explicit prompt-file-arg configurations via `agents.definitions`.

**Acceptance Criteria:**
- Built-in Gemini preset resolves to binary `gemini` with `Stdin` delivery.
- Existing custom agent definition override behavior is unchanged.
- Picker and phase-resolution paths continue to include Gemini.

**Dependencies:** None

### Task 6: Add Gemini contract tests and canonical fixtures

**Files:**
- Create: `test/contracts/gemini_contract_test.go`
- Create: `test/fixtures/gemini_stream_success.jsonl`
- Create: `test/fixtures/gemini_stream_failure.jsonl`
- Create: `test/fixtures/gemini_success.txt`
- Reconcile with existing directory: `test/fixtures/gemini/*`

**What to Do:**
Add contract tests for Gemini invocation format and fixture shape parity with current provider contract style. Prefer leveraging existing shared contract harness helpers and call-log filtering pattern; avoid introducing a parallel bespoke harness.

**Acceptance Criteria:**
- Contract tests verify expected Gemini CLI args for run/stream paths.
- Fixture files are deterministic and include provenance/refresh metadata when required by fixture contract checks.
- Gemini contract tests run under `-tags=contract` alongside existing provider contracts.

**Dependencies:** Task 2, Task 4

### Task 7: Final verification and acceptance sweep

**Files:**
- No new source files; run verification commands and adjust tests as needed.

**What to Do:**
Run targeted provider and agent unit tests, then contract subset relevant to Gemini/provider fixtures. Ensure all acceptance criteria from spec are covered by tests or documented follow-up where live-signature gaps remain.

**Acceptance Criteria:**
- `go test ./internal/provider ./internal/agent ./internal/runner` passes.
- `go test -tags=contract ./test/contracts` passes (or known pre-existing failures are isolated and documented).
- No behavior regressions in Claude/Codex provider tests.

**Dependencies:** Tasks 1-6

---

## Notes

- Existing contract testing is provider-specific plus shared harness helpers, not a single consolidated provider contract matrix. Gemini should follow that established style now and can later be folded into a unified provider-contract framework.
- Keep auth/rate-limit/transport signature matching conservative until additional live Gemini failure captures are available.
- Preserve configuration-driven behavior: no hardcoded Gemini tier defaults; model and cost settings must come from `providers.gemini` config.
- Start with routing ratio disabled or near-zero in production configs until live confidence is established.
