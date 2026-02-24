---
id: provider-type-rename
source_spec: provider-type-rename
created: 2026-02-24
decomposed: false
---

# Provider Type Rename Implementation Plan

**Goal:** Rename provider-agnostic Claude-named interfaces/results to neutral LLM/provider names, without changing behavior.

**Architecture:** Perform a mechanical rename across pipeline and CLI adapters, then switch escalation/validation contracts to `provider.Result` so execution no longer converts provider results into Claude-specific structs.

**Tech Stack:** Go, internal pipeline/runner packages, cmd/gromit adapters, Go test tooling.

**Spec:** `.gromit/specs/provider-type-rename.md`

---

## Architecture

**Overview:**
Use a one-pass, compile-driven refactor that keeps runtime behavior unchanged while improving naming clarity and type ownership.

**Key Components:**
1. **Pipeline type surface (`internal/pipeline`)**: Rename `ClaudeClient` to `LLMClient`, `ClaudeRunResult` to `LLMRunResult`, and `Deps.ClaudeClient` to `Deps.LLMClient`.
2. **CLI adapter surface (`cmd/gromit`)**: Rename provider-generic adapter naming and helper return types to align with `LLM*` pipeline interfaces.
3. **Runner invocation contracts (`internal/runner`)**: Make escalation and validation consume `*provider.Result` directly.
4. **Execution invoker (`internal/runner/execution`)**: Remove redundant conversion from `provider.Result` to `claude.Result` used only for escalation compatibility.

**Integration Points:**
- `cmd/gromit/decompose.go` and `cmd/gromit/review.go` currently wire `pipeline.ClaudeClient`-typed dependencies.
- `internal/runner/escalation/handler.go` signatures currently expect `*claude.Result`.
- `internal/runner/validation/runner.go` currently returns `*claude.Result` from `RunDirect`.
- `internal/runner/execution/invoker.go` currently creates `claude.Result` from provider output before returning `runtypes.InvocationResult`.

**Data Flow:**
- Before: provider call returns `provider.Result` -> invoker maps to `claude.Result` -> escalation/validation consume Claude-shaped result.
- After: provider call returns `provider.Result` -> invoker exposes it directly -> escalation/validation consume `provider.Result` as canonical.

**Files to Modify:**
- `internal/pipeline/pipeline.go`
- `internal/pipeline/decompose.go`
- `internal/pipeline/*_test.go` files that reference `ClaudeClient`/`ClaudeRunResult`
- `cmd/gromit/adapters.go`
- `cmd/gromit/decompose.go`
- `cmd/gromit/review.go`
- `cmd/gromit/*_test.go` files that reference renamed pipeline types/adapters
- `internal/runner/validation/runner.go`
- `internal/runner/validation/*_test.go`
- `internal/runner/escalation/handler.go`
- `internal/runner/escalation/*_test.go`
- `internal/runner/execution/invoker.go`
- `internal/runner/execution/invoker_test.go`
- `internal/runner/methodology/*_test.go` tests with old validation callback result types
- `internal/runner/runtypes/types_test.go` (as needed for signature assertions)

**Files to Create:**
- None.

**Tradeoffs:**
- **Direct `provider.Result` reuse** avoids dual result-type drift and keeps provider metadata authoritative.
- **Single-pass rename** creates a larger diff but avoids intermediate mixed naming states.
- **Selective adapter renaming** preserves Claude-specific names only where the adapter truly wraps `claude.Client`.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** Update compile-time interface assertions and signature-level tests for renamed types and provider-result contracts.
2. **Integration Tests:** Keep existing decompose/review and runner workflow tests, but update expectations/types to neutral names.
3. **Manual Verification:** Run targeted package tests for touched areas, then broader compile/test sweep.

**Key Test Cases:**
- `pipeline.Deps` wiring uses `LLMClient` with unchanged behavior.
- Decompose and review non-interactive paths operate on `LLMRunResult`.
- `validation.RunDirect` returns `*provider.Result` and preserves success/failure output semantics.
- Escalation APIs (`HandleEscalation`, `AnalyzeAndHandleFailure`) accept `*provider.Result` and preserve retry/escalation decisions.
- Invoker no longer performs provider->Claude conversion solely for escalation compatibility.

**Mocking Strategy:**
- Reuse existing mocks/fakes and update signatures in place.
- Avoid compatibility shims; rely on compile errors to drive complete rename coverage.

**Coverage Goals:**
- Build failure path from invocation output through triage/escalation using provider-native result type.
- Non-interactive pipeline flows (decompose/review) continue to parse output and enforce validation unchanged.
- Edge cases: nil provider results, usage-limit fallback, validation non-zero exits.

**Test Organization:**
- Preserve current package-local test files.
- Keep compile-time interface assertions for adapters/interfaces after rename.

## Implementation Tasks

### Task 1: Rename Pipeline Core Types to LLM-Neutral Names

**Files:**
- Modify: `internal/pipeline/pipeline.go`
- Test: `internal/pipeline/pipeline_test.go`
- Test: `internal/pipeline/typed_interfaces_test.go`

**What to Do:**
Rename core pipeline abstractions from Claude-specific names to neutral LLM names: `ClaudeClient` -> `LLMClient`, `ClaudeRunResult` -> `LLMRunResult`, and `Deps.ClaudeClient` -> `Deps.LLMClient`. Update interface comments and compile-time checks accordingly.

**Acceptance Criteria:**
- Pipeline interfaces/types compile under `LLM*` names only.
- `Deps` uses `LLMClient` field and no stale `ClaudeClient` reference remains in `pipeline.go`.
- Existing behavior and struct fields remain unchanged.

**Dependencies:**
- None.

### Task 2: Update Pipeline Workflow Call Sites and Mocks

**Files:**
- Modify: `internal/pipeline/decompose.go`
- Test: `internal/pipeline/decompose_test.go`
- Test: `internal/pipeline/mocks_test.go`

**What to Do:**
Update workflow call sites and mock implementations to use `LLMClient`/`LLMRunResult`, including all typed mock return values and injected deps in decompose-related tests.

**Acceptance Criteria:**
- Decompose flow invokes `p.deps.LLMClient.Run(...)` and handles `LLMRunResult`.
- Decompose mocks/tests compile with renamed types and maintain existing assertions.
- No behavior changes in retry/validation/decompose output handling.

**Dependencies:**
- Task 1.

### Task 3: Rename Provider-Generic CLI Adapter Types and Wiring

**Files:**
- Modify: `cmd/gromit/adapters.go`
- Modify: `cmd/gromit/decompose.go`
- Test: `cmd/gromit/decompose_adapters_test.go`

**What to Do:**
Rename provider-generic adapter surface to neutral naming (e.g., `llmClientAdapter` where it wraps router/provider abstractions) and update helper conversion naming (`toLLMRunResult`). Keep truly Claude-specific fallback adapter naming where it directly wraps `claude.Client`.

**Acceptance Criteria:**
- Provider-generic adapter types no longer use misleading Claude names.
- Decompose client factory returns `pipeline.LLMClient`.
- Adapter tests and compile-time assertions reflect new names/types.

**Dependencies:**
- Task 1.

### Task 4: Switch Validation Direct-Run Contract to provider.Result

**Files:**
- Modify: `internal/runner/validation/runner.go`
- Test: `internal/runner/validation/validation_test.go`
- Test: `internal/runner/methodology/methodology_test.go`

**What to Do:**
Change `RunDirect` return type from `*claude.Result` to `*provider.Result` and propagate signature updates through validation callers and test callback definitions.

**Acceptance Criteria:**
- `RunDirect` returns `*provider.Result` for both pass/fail cases.
- Validation failure output formatting and exit code semantics remain unchanged.
- All direct callers and tests compile with updated signature.

**Dependencies:**
- None.

### Task 5: Switch Escalation Failure Handling to provider.Result and Remove Conversion

**Files:**
- Modify: `internal/runner/escalation/handler.go`
- Modify: `internal/runner/execution/invoker.go`
- Test: `internal/runner/execution/invoker_test.go`

**What to Do:**
Update escalation handler APIs (`HandleEscalation`, `AnalyzeAndHandleFailure`, internal helpers) to accept `*provider.Result`. In invoker, remove the `provider.Result` -> `claude.Result` conversion used for escalation compatibility, and ensure escalation call sites pass `InvocationResult.ProviderResult`.

**Acceptance Criteria:**
- Escalation handler public/internal signatures use `*provider.Result`.
- Invoker no longer constructs `claude.Result` solely for escalation flow compatibility.
- Existing escalation/retry behavior remains intact under tests.

**Dependencies:**
- Task 4 (shared runner result contract alignment).

### Task 6: Update Remaining CLI/Runner Tests and Complete Compatibility Sweep

**Files:**
- Modify: `cmd/gromit/review.go`
- Test: `cmd/gromit/review_test.go`
- Test: `internal/runner/escalation/handler_test.go`

**What to Do:**
Finish residual type/name updates across review wiring and broad test surfaces, then run targeted and full quality gates to verify no stale Claude-generic type references remain.

**Acceptance Criteria:**
- Review non-interactive client wiring compiles with renamed pipeline types.
- Escalation and review tests pass with provider-result signatures.
- Project-wide compile/test passes with no functional behavior regressions.

**Dependencies:**
- Task 2
- Task 3
- Task 5

---

## Notes

- This plan is intentionally refactor-only: no semantic behavior changes are expected.
- Keep `internal/claude` package names and genuinely Claude-specific types untouched.
- If additional stale references are discovered during implementation, they should be handled in the nearest related task rather than introducing compatibility aliases.
