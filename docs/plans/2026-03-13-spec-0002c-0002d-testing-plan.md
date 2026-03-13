# Spec 0002c/0002d -- Testing and Verification Plan

Provider-Agnostic Adapter Layer (0002c) and Multi-Provider Routing (0002d) for Gromit Next.

New package `internal/next/llmadapter/` with shared `LLMAdapter` base, per-domain adapters (`ProviderPlanAgent`, `ProviderReviewAgent`, `ProviderAcceptAgent`, `ProviderTaskRunner`), non-LLM adapters (`SpecCompilerAdapter`, `ShellValidator`), `FallbackAdapter` for usage-limit failover, `RoutingConfig` in `execpolicy.Policy`, and wiring updates in `RealStageProvider`.

## Overview

This document defines every test required to satisfy the Spec 0002c and 0002d evidence requirements. Each evidence item maps to at least one named test function. All unit tests are deterministic -- no network calls, no timing dependencies, no flaky assertions. Contract tests run against real providers (gated by env var) and are a hard completion gate but never run in CI.

## Test Strategy

### Unit test approach

- Table-driven tests for all parsing, validation, and adapter logic.
- Test files co-located with implementation: `foo.go` -> `foo_test.go`.
- Each test function name describes the behavior under test, not the implementation detail.
- Assertions use `t.Errorf` / `t.Fatalf` with descriptive messages; no assertion libraries.
- All external boundaries are interfaces. Tests inject fakes/mocks.

### Contract test approach

- Gated by env var `GROMIT_LLM_CONTRACT=1`. Never run in CI.
- Each domain agent interface gets a shared contract test suite (`RunPlanAgentContract`, `RunReviewAgentContract`, `RunAcceptAgentContract`, `RunTaskRunnerContract`).
- Contract tests assert structural compliance only: parseable output, required fields present, valid enum values. No assertions on content quality.
- Each provider (Claude, Codex) gets its own test function that wires a real provider into the shared suite.
- A domain adapter is not considered complete until its contract test suite passes against every configured provider.

### Integration test approach

- Build tag `//go:build integration` on all integration test files.
- Integration tests wire real packages together but stub the LLM provider via mock providers.
- Integration tests use real filesystem via `t.TempDir()`.
- Run separately: `go test -tags integration ./internal/next/...`

### Fixture/fake strategy

All external boundaries are interfaces. Tests inject fakes:

| Boundary | Interface | Fake |
|----------|-----------|------|
| LLM provider | `provider.Provider` | `mockProvider` -- configurable Run result, tracks calls |
| Usage-limit provider | `provider.Provider` | `mockProviderWithUsageLimit` -- configurable `IsUsageLimitError` |
| Router | `llmadapter.RouterSelector` | `mockRouter` -- sequence-based or single-result Select |
| LLM invoker | `llmadapter.Invoker` | `mockInvoker` -- returns canned `*provider.Result` |
| Provider-aware invoker | `llmadapter.ProviderAwareInvoker` | `mockProviderAwareInvoker` -- wraps mockProvider + mockInvoker |
| Validator runner | `validator.Runner` | Real `Runner` with `FakeCmdRunner` underneath |
| Context compiler | `contextpkt.Compiler` | `mockCompiler` -- returns canned Packet |
| Artifact store | `contextpkt.ArtifactStore` | `mockArtifactStore` -- in-memory map |

---

## Package-by-Package Test Coverage

### llmadapter/ tests (0002c)

File: `internal/next/llmadapter/adapter_test.go`

**`TestInvoke_CallsProviderRun_WithCorrectTier`**
- Input: `mockProvider` with `Name()="claude"`, `Config{Tier: "high"}`.
- Call `Invoke(ctx, "hello")`.
- Assert: `mockProvider.lastTier == "high"`, `mockProvider.calls == 1`.

**`TestInvoke_ReturnsProviderResult`**
- Input: `mockProvider` returns `&provider.Result{Output: "response", CostUSD: 0.05, InputTokens: 100, OutputTokens: 50}`.
- Assert: Returned result matches provider output exactly.

**`TestInvoke_TimeoutEnforcement_CancelsContext`**
- Input: `Config{Timeout: 1 * time.Millisecond}`, `mockProvider` that blocks on `Run` until context canceled.
- Assert: Returns context deadline exceeded error.

**`TestInvoke_OnCostCallback_FiresWithResultCost`**
- Input: `Config{OnCost: func(c float64) { recorded = c }}`, provider returns `CostUSD: 0.03`.
- Assert: `recorded == 0.03`.

**`TestInvoke_OnCostCallback_NotCalledOnZeroCost`**
- Input: Provider returns `CostUSD: 0.0`.
- Assert: Callback not invoked.

**`TestInvoke_ErrorPropagation`**
- Input: Provider returns `(nil, errors.New("network failure"))`.
- Assert: Error returned, contains "network failure".

**`TestInvoke_ErrorWithResult_ReturnsBoth`**
- Input: Provider returns `(&provider.Result{Output: "partial"}, errors.New("usage limit"))`.
- Assert: Both result and error are non-nil. Callers (like FallbackAdapter) can inspect the result.
- Note: This behavior is critical for `FallbackAdapter.Invoke`, which calls `provider.IsUsageLimitError(result, err)` — if `result` were nil on error, usage-limit detection would fail. See `TestFallbackAdapter_UsageLimit_FallsBackToRouter`.

**`TestInvokeStream_CallsProviderStreamRun_WithCorrectTier`**
- Input: `mockProvider`, `Config{Tier: "medium"}`.
- Call `InvokeStream(ctx, "prompt", writer, handler, onToolCall)`.
- Assert: Provider's `StreamRun` called with tier `"medium"`.

**`TestInvokeStream_OnCostCallback_Fires`**
- Input: Provider returns streaming result with `CostUSD: 0.10`.
- Assert: OnCost callback fires with `0.10`.

**`TestInvokeStream_ErrorPropagation`**
- Input: Provider's `StreamRun` returns `(nil, errors.New("stream failure"))`.
- Assert: Error returned, contains "stream failure".

**`TestInvokeStream_TimeoutEnforcement_CancelsContext`**
- Input: `Config{Timeout: 1 * time.Millisecond}`, provider's `StreamRun` blocks until context canceled.
- Assert: Returns context deadline exceeded error.

**`TestProviderName_ReturnsProviderName`**
- Input: `mockProvider{name: "codex"}`.
- Assert: `adapter.ProviderName() == "codex"`.

**`TestTier_ReturnsConfiguredTier`**
- Input: `Config{Tier: "high"}`.
- Assert: `adapter.Tier() == "high"`.

**`TestLLMAdapter_SatisfiesInvoker`**
- Compile-time check: `var _ Invoker = (*LLMAdapter)(nil)`.

**`TestLLMAdapter_SatisfiesProviderAwareInvoker`**
- Compile-time check: `var _ ProviderAwareInvoker = (*LLMAdapter)(nil)`.

**`TestProvider_ReturnsUnderlyingProvider`**
- Input: `mockProvider{name: "claude"}`.
- Assert: `adapter.Provider().Name() == "claude"`.

File: `internal/next/llmadapter/extract_test.go`

**`TestExtractJSON_ObjectInProse`**
- Input: `"Here is the result: {\"key\": \"value\"} -- done"`.
- Assert: Returns `{\"key\": \"value\"}`.

**`TestExtractJSON_ArrayInProse`**
- Input: `"Results: [{\"a\":1},{\"b\":2}] end"`.
- Assert: Returns `[{\"a\":1},{\"b\":2}]`.

**`TestExtractJSON_MarkdownFence`**
- Input: `` "```json\n{\"key\": \"value\"}\n```" ``.
- Assert: Returns `{\"key\": \"value\"}`.

**`TestExtractJSON_MarkdownFence_WithLanguageTag`**
- Input: `` "```json\n[{\"a\":1}]\n```" ``.
- Assert: Returns `[{\"a\":1}]`.

**`TestExtractJSON_NestedBraces`**
- Input: `"{\"outer\": {\"inner\": 1}}"`.
- Assert: Returns the full nested object.

**`TestExtractJSON_NoJSON_ReturnsEmpty`**
- Input: `"This is plain text with no JSON"`.
- Assert: Returns `""`.

**`TestExtractJSON_EmptyString_ReturnsEmpty`**
- Input: `""`.
- Assert: Returns `""`.

**`TestExtractJSON_MultipleFences_ReturnsFirst`**
- Input: Two markdown fences with different JSON objects.
- Assert: Returns the first JSON object.

**`TestExtractJSON_MultipleBareObjects_ReturnsFirst`**
- Input: `'{"first":1} and {"second":2}'`.
- Assert: Returns `'{"first":1}'` — returns the first JSON object found.

**`TestExtractJSON_TrailingComma_ReturnsEmptyString`**
- Input: `'{"key": "value",}'`.
- Assert: Returns `""` — trailing comma is invalid JSON; ExtractJSON does not attempt repair.

**`TestExtractJSON_SingleQuotes_ReturnsEmptyString`**
- Input: `"{'key': 'value'}"`.
- Assert: Returns `""` — single quotes are not valid JSON; ExtractJSON does not attempt repair.

File: `internal/next/llmadapter/invoker_test.go`

**`TestInvokerInterface_MethodSignature`**
- Assert: `Invoker` interface has exactly one method: `Invoke(ctx context.Context, prompt string) (*provider.Result, error)`.
- Implemented as compile-time check with a mock type.

**`TestProviderAwareInvokerInterface_ExtendsInvoker`**
- Assert: `ProviderAwareInvoker` has `Invoke` + `Provider() provider.Provider`.
- Compile-time check.

### llmadapter/ tests (0002d) -- FallbackAdapter

File: `internal/next/llmadapter/fallback_test.go`

**`TestFallbackAdapter_NormalInvocation_NoFallback`**
- Input: `mockRouter` returns `mockProvider{name: "claude", runResult: {Output: "hello"}}`.
- Call `Invoke(ctx, "prompt")`.
- Assert: Result output is `"hello"`. Router `MarkUnavailable` NOT called.

**`TestFallbackAdapter_UsageLimit_FallsBackToRouter`**
- Input: Primary `mockProviderWithUsageLimit{isUsageLimit: true}` returns error. `mockRouter` second `Select` returns `mockProvider{name: "codex", runResult: {Output: "fallback worked"}}`.
- Assert: Result output is `"fallback worked"`. Router `MarkUnavailable` called with `"claude"`.

**`TestFallbackAdapter_NonUsageLimitError_NoFallback`**
- Input: Primary returns `(nil, errors.New("network timeout"))` with `isUsageLimit: false`.
- Assert: Error propagated. No fallback attempted. Router `MarkUnavailable` NOT called.

**`TestFallbackAdapter_AllProvidersExhausted_ReturnsError`**
- Input: Primary hits usage limit. `mockRouter` second `Select` returns `(nil, "")`.
- Assert: Error contains `"all providers exhausted"`.

**`TestFallbackAdapter_SatisfiesProviderAwareInvoker`**
- Compile-time check: `var _ ProviderAwareInvoker = (*FallbackAdapter)(nil)`.

**`TestFallbackAdapter_SatisfiesInvoker`**
- Compile-time check: `var _ Invoker = (*FallbackAdapter)(nil)`.

**`TestFallbackAdapter_Provider_ReturnsPrimaryProvider`**
- Input: `mockRouter` returns `mockProvider{name: "claude"}`.
- Call `Provider()` to trigger lazy init.
- Assert: `Provider().Name() == "claude"`.

**`TestFallbackAdapter_LazyResolution_DefersSelectToFirstInvoke`**
- Input: `mockRouter` with tracking.
- Assert: After `NewFallbackAdapter()`, `router.selectCalled == false`.
- Call `Invoke(ctx, "prompt")`.
- Assert: `router.selectCalled == true`.

**`TestFallbackAdapter_CostCallback_FiresOnFallback`**
- Input: Primary hits usage limit. Fallback returns result with `CostUSD: 0.05`.
- `Config{OnCost: func(c float64) { recorded = c }}`.
- Assert: `recorded == 0.05`.

**`TestFallbackAdapter_FallbackProviderError_WrapsOriginal`**
- Input: Primary hits usage limit. Fallback also returns error.
- Assert: Error message contains both primary name and fallback name.

**`TestFallbackAdapter_InvokeStream_UsageLimit_FallsBackToRouter`**
- Input: Primary `mockProviderWithUsageLimit{isUsageLimit: true}` returns usage-limit error on `StreamRun`. `mockRouter` second `Select` returns `mockProvider{name: "codex"}`.
- Call `InvokeStream(ctx, "prompt", writer, handler, onToolCall)`.
- Assert: Result from codex. Router `MarkUnavailable` called with primary name.

**`TestFallbackAdapter_InvokeStream_NormalInvocation_NoFallback`**
- Input: `mockRouter` returns `mockProvider{name: "claude"}` with successful `StreamRun`.
- Call `InvokeStream(ctx, "prompt", writer, handler, onToolCall)`.
- Assert: Result from claude. No fallback triggered.

**`TestFallbackAdapter_ChainedFallback_ThreeProviders`**
- Input: Three providers: primary `mockProviderWithUsageLimit{name: "claude", isUsageLimit: true}` (returns usage-limit error), secondary `mockProviderWithUsageLimit{name: "codex", isUsageLimit: true}` (returns usage-limit error), tertiary `mockProvider{name: "gemini", runResult: {Output: "tertiary success"}}` (succeeds). `mockRouter` returns providers in sequence: claude, codex, gemini.
- Call `Invoke(ctx, "prompt")`.
- Assert: Primary is tried first, fails with usage-limit. Router selects secondary, which also fails with usage-limit. Router selects tertiary, which succeeds. Final result output is `"tertiary success"`. Confirms the fallback chain loops through all available providers before giving up.

### planner/ tests

File: `internal/next/planner/provider_agent_test.go`

**`TestProviderPlanAgent_Invoke_ReturnsAgentResult`**
- Input: `mockInvoker` returns `&provider.Result{Output: "plan JSON", InputTokens: 100, OutputTokens: 200, CostUSD: 0.05}`.
- Call `Invoke(ctx, "prompt", "high")`.
- Assert: `AgentResult{Output: "plan JSON", TokensIn: 100, TokensOut: 200, Cost: 0.05}`.

**`TestProviderPlanAgent_Invoke_PropagatesError`**
- Input: `mockInvoker` returns error.
- Assert: Error propagated unchanged.

**`TestProviderPlanAgent_Invoke_TierMismatch_LogsWarning`**
- Input: `ProviderPlanAgent` constructed with adapter tier `"high"`. Called with `tier="medium"`.
- Assert: No error (tier param is ignored). Warning logged once via `sync.Once`. Verify by injecting a `*slog.Logger` backed by `slog.NewTextHandler(&buf, nil)` and asserting `buf` contains `"tier mismatch"`.

**`TestProviderPlanAgent_SatisfiesAgent`**
- Compile-time check: `var _ Agent = (*ProviderPlanAgent)(nil)`.

**`TestProviderPlanAgent_IntegrationWithPlanner`**
- Wire `ProviderPlanAgent` with `mockInvoker` returning valid plan JSON.
- Call `Planner.CreatePlan()`.
- Assert: Plan parsed successfully. Tasks have IDs and objectives.

### review/ tests

File: `internal/next/review/provider_agent_test.go`

**`TestProviderReviewAgent_ReviewFacet_ParsesFindings`**
- Input: `mockInvoker` returns `&provider.Result{Output: "[{\"file\":\"main.go\",\"severity\":\"warning\",\"description\":\"unused import\",\"line\":5}]"}`.
- Assert: Returns 1 finding with correct fields. `Severity` is `SeverityWarning`.

**`TestProviderReviewAgent_ReviewFacet_EmptyArray_ReturnsEmptySlice`**
- Input: Provider returns `"[]"`.
- Assert: Returns `[]Finding{}`, no error.

**`TestProviderReviewAgent_ReviewFacet_InvalidJSON_ReturnsParseError`**
- Input: Provider returns `"not json at all"`.
- Assert: Error returned. Error is wrapped as (or contains behavior of) a parse error, enabling the Runner's retry logic.

**`TestProviderReviewAgent_ReviewFacet_MarkdownFence_Handled`**
- Input: Provider returns `` "```json\n[{\"file\":\"a.go\",\"severity\":\"error\",\"description\":\"bug\"}]\n```" ``.
- Assert: Findings parsed correctly via `ExtractJSON`.

**`TestProviderReviewAgent_ReviewFacet_MissingRequiredField_ReturnsError`**
- Table-driven: missing `file`, missing `description`.
- Assert: Each returns error from `Finding.UnmarshalJSON` validation.

**`TestProviderReviewAgent_ReviewFacet_InvalidSeverity_ReturnsError`**
- Input: Finding JSON with `"severity": "bogus"`.
- Assert: Error from `ParseSeverity`.

**`TestProviderReviewAgent_SatisfiesReviewAgent`**
- Compile-time check: `var _ ReviewAgent = (*ProviderReviewAgent)(nil)`.

### acceptor/ tests

File: `internal/next/acceptor/provider_agent_test.go`

**`TestProviderAcceptAgent_EvaluateCriterion_ParsesResult`**
- Input: `mockInvoker` returns `&provider.Result{Output: "{\"status\":\"pass\",\"rationale\":\"all tests pass\",\"evidence_refs\":[\"metrics.json\"]}"}`.
- Assert: `CriterionResult{Status: "pass", Rationale: "all tests pass", EvidenceRefs: ["metrics.json"]}`.

**`TestProviderAcceptAgent_EvaluateCriterion_FailStatus`**
- Input: Provider returns `"{\"status\":\"fail\",\"rationale\":\"missing test coverage\"}"`.
- Assert: `Status == "fail"`, `Rationale == "missing test coverage"`.

**`TestProviderAcceptAgent_EvaluateCriterion_UnclearStatus`**
- Input: Provider returns `"{\"status\":\"unclear\",\"rationale\":\"cannot determine\"}"`.
- Assert: `Status == "unclear"`.

**`TestProviderAcceptAgent_EvaluateCriterion_InvalidJSON_ReturnsError`**
- Input: Provider returns unparseable output.
- Assert: Error returned.

**`TestProviderAcceptAgent_EvaluateCriterion_MarkdownFence`**
- Input: Provider returns result wrapped in markdown fences.
- Assert: Parsed correctly via `ExtractJSON`.

**`TestProviderAcceptAgent_EvaluateCriterion_MissingStatus_ReturnsError`**
- Input: Provider returns `"{\"rationale\":\"good\"}"` (no status field).
- Assert: Error from validation (status empty string).

**`TestProviderAcceptAgent_SatisfiesAcceptAgent`**
- Compile-time check: `var _ AcceptAgent = (*ProviderAcceptAgent)(nil)`.

**`TestParseCriterionResult_ValidJSON`**
- Input: `"{\"status\":\"pass\",\"rationale\":\"ok\"}"`.
- Assert: Parsed correctly.

**`TestParseCriterionResult_ExtractsFromProse`**
- Input: `"The result is: {\"status\":\"pass\",\"rationale\":\"ok\"} -- done"`.
- Assert: Parsed correctly via `ExtractJSON`.

### specloop/ tests

File: `internal/next/specloop/provider_taskrunner_test.go`

**`TestProviderTaskRunner_RunTask_ReturnsTaskResult`**
- Input: `mockInvoker` returns `&provider.Result{Output: "implementation", InputTokens: 500, OutputTokens: 1000, CostUSD: 0.10}`.
- Call `RunTask(ctx, task)`.
- Assert: `TaskResult{Status: "done", TokensUsed: 1500, Cost: 0.10}`.

**`TestProviderTaskRunner_RunTask_ProviderError_ReturnsFailed`**
- Input: Provider returns error.
- Assert: Error propagated. Caller (task loop) marks task as failed.

**`TestProviderTaskRunner_RepairTask_IncludesFailuresInPrompt`**
- Input: `task` with `TaskID: "t-003"`, `failures: ["test X failed", "lint error in Y"]`.
- Record prompt passed to `mockInvoker`.
- Assert: Prompt contains `"test X failed"` and `"lint error in Y"`.

**`TestProviderTaskRunner_RepairTask_ReturnsTaskResult`**
- Input: `mockInvoker` returns successful result.
- Assert: TaskResult returned with status `"done"`.

**`TestProviderTaskRunner_RunTask_RendersTaskPrompt`**
- Input: `task` with `Objective: "add helper function"`, `ExpectedTouchedArea: ["pkg/util/"]`.
- Record prompt passed to `mockInvoker`.
- Assert: Prompt contains `"add helper function"` and `"pkg/util/"`.

**`TestProviderTaskRunner_SatisfiesTaskRunner`**
- Compile-time check: `var _ TaskRunner = (*ProviderTaskRunner)(nil)`.

### validator/ tests

File: `internal/next/validator/shell_validator_test.go`

**`TestShellValidator_RunFinal_DelegatesToRunner`**
- Input: `ShellValidator` wrapping real `Runner`. Checks that pass via shell.
- Assert: `FinalResult.Pass == true`. Delegation verified by Runner executing the commands.

**`TestShellValidator_RunFinal_FailingCheck_ReportsFailure`**
- Input: Always-run check with command `"exit 1"`.
- Assert: `FinalResult.Pass == false`. Failed check identified.

**`TestShellValidator_RunFinal_MixedChecks`**
- Input: 2 always-run (1 pass, 1 fail), 1 project check (pass).
- Assert: `FinalResult.Pass == false` (due to failing always-run).

**`TestShellValidator_SatisfiesFinalValidator`**
- Compile-time check verifying `ShellValidator` satisfies the `stages.FinalValidator` interface.

### contextpkt/ tests

File: `internal/next/contextpkt/compiler_adapter_test.go`

**`TestSpecCompilerAdapter_Compile_DelegatesToCompiler`**
- Input: `mockCompiler` returns `Packet{Level: LevelSpec, Sections: [...]}`; `SpecCompilerAdapter` wraps it.
- Call `Compile(ctx)`.
- Assert: Result is rendered packet string. Compiler called with correct cell, level, and opts.

**`TestSpecCompilerAdapter_Compile_ResolvesCell`**
- Input: `SpecCompilerAdapter` constructed with `specPath: "/path/to/spec.md"`.
- Assert: Correct cell resolved from spec path and passed to underlying compiler.

**`TestSpecCompilerAdapter_Compile_PassesTokenBudget`**
- Input: `SpecCompilerAdapter` constructed with `tokenBudget: 5000`.
- Assert: Underlying compiler receives `CompileOpts{TokenBudget: 5000}`.

**`TestSpecCompilerAdapter_Compile_PropagatesError`**
- Input: Underlying compiler returns error.
- Assert: Error propagated from `Compile()`.

**`TestSpecCompilerAdapter_SatisfiesSpecCompiler`**
- Compile-time check verifying `SpecCompilerAdapter` satisfies `stages.SpecCompiler` interface (which has `Compile(ctx context.Context) (string, error)`).

### execpolicy/ tests (0002d)

File: `internal/next/execpolicy/policy_test.go` (additions)

**`TestPolicy_Validate_RoutingRatioSumsTo100`**
- Input: `DefaultPolicy()` with `Routing.Ratio = map[string]int{"claude": 70, "codex": 20}` (sums to 90).
- Assert: Validation error contains `"sum to 100"`.

**`TestPolicy_Validate_RoutingRatioValid`**
- Input: `Routing.Ratio = map[string]int{"claude": 70, "codex": 30}`.
- Assert: No validation error.

**`TestPolicy_Validate_RoutingRatioEmpty_NoError`**
- Input: `Routing.Ratio = map[string]int{}`.
- Assert: No validation error. Design decision: empty ratio bypasses the sum-to-100 check because it signals single-provider mode (provider selected by preference or default, not by weighted ratio). This is distinct from a ratio with entries that don't sum to 100.

**`TestRouter_EmptyRatios_FallsBackToPreferencesOnly`**
- Input: Router constructed with an empty ratio map (`map[string]int{}`) and provider preferences `["claude", "codex"]`.
- Call `SelectProvider()` — returns `"claude"` (first preference).
- Assert: Confirms that empty ratios signal single-provider mode using preferences, not round-robin.

**`TestPolicy_Validate_RoutingRatioSingleProvider`**
- Input: `Routing.Ratio = map[string]int{"claude": 100}`.
- Assert: No validation error.

**`TestPolicy_NormalizeNilFields_RoutingMaps`**
- Input: Policy with nil `Routing.Preferences` and nil `Routing.Ratio`.
- Assert: After `NormalizeNilFields()`, both are non-nil empty maps.

**`TestPolicy_Defaults_IncludesRouting`**
- Assert: `DefaultPolicy().Routing.Preferences` is non-nil.
- Assert: `DefaultPolicy().Routing.Ratio` is non-nil.
- Assert: `DefaultPolicy().Routing.CooldownSeconds > 0`.

**`TestRouter_CooldownReenablesProvider`**
- Input: Router with `CooldownSeconds: 60` and an injectable `Clock` interface (`type Clock interface { Now() time.Time }`). `mockProvider{name: "claude"}` and `mockProvider{name: "codex"}`. Inject a `fakeClock` whose `Now()` return value is controlled by the test.
- Call `MarkUnavailable("claude")`.
- Call `Select("plan", "high")` immediately — returns codex (claude is cooling down).
- Advance `fakeClock` by 61 seconds (past the cooldown duration).
- Call `Select("plan", "high")` again — returns claude (cooldown expired).
- Assert: Provider re-enabled after cooldown period.
- Note: This test lives in the router package, not in FallbackAdapter. FallbackAdapter delegates cooldown to the Router via `MarkUnavailable`.
- Note: The Router accepts a `Clock` interface to avoid real-time waits in tests. Production code passes a `realClock` that delegates to `time.Now()`. Tests pass a `fakeClock` whose time is advanced explicitly, ensuring determinism with no timing dependencies.

**`TestPolicy_LoadPolicy_PartialRouting`**
- Input: JSON with only `routing.cooldown_seconds: 600` set.
- Assert: `CooldownSeconds == 600`. Other routing fields take defaults.

### cmd/gromit-next/ tests

File: `cmd/gromit-next/stage_provider_test.go` (additions)

**`TestBuildStages_ReplacesNoopsWithRealAdapters`**
- Input: `RealStageProvider` with mock providers wired.
- Call `BuildStages(policy, rs, budget)`.
- Assert: Returned stages list has same count as current (9 stages). Stage names match expected pipeline order: init, compile, plan, execute, validate, review, accept, evidence, finalize.

**`TestBuildStages_CostCallback_FeedsBudget`**
- Input: Mock provider returns `CostUSD: 0.15`.
- Build stages and invoke the plan adapter.
- Assert: `budget.AccumulatedCost()` increases by `0.15`.

**`TestBuildStages_NilCodexProvider_SingleProviderMode`** (0002d)
- Input: `RealStageProvider` with `codexProvider: nil`.
- Call `BuildStages`.
- Assert: Stages build successfully. Single-provider mode (Claude only).

**`TestBuildStages_BothProviders_FallbackAdaptersUsed`** (0002d)
- Input: `RealStageProvider` with both `claudeProvider` and `codexProvider`.
- Call `BuildStages`.
- Assert: Stages build successfully. `FallbackAdapter` wired for LLM stages.

**`TestBuildRouter_ReturnsConfiguredRouter`** (0002d)
- Input: `RealStageProvider` with both providers. `policy.Routing.Ratio = {"claude": 70, "codex": 30}`.
- Call `buildRouter(policy)`.
- Assert: Router non-nil. `Select("plan", "high")` returns a provider.

---

## Contract Test Suites

### Planner contract

File: `internal/next/planner/agent_contract_test.go`

```go
// RunPlanAgentContract runs structural contract tests against any Agent implementation.
func RunPlanAgentContract(t *testing.T, agent Agent) {
    t.Run("returns parseable plan for well-formed prompt", func(t *testing.T) {
        result, err := agent.Invoke(ctx, samplePlanPrompt, "high")
        if err != nil {
            t.Fatalf("invoke: %v", err)
        }
        plan, err := ParsePlan(result.Output)
        if err != nil {
            t.Fatalf("parse plan: %v", err)
        }
        if len(plan.Tasks) == 0 {
            t.Error("plan must contain at least one task")
        }
    })
    t.Run("tasks have required fields", func(t *testing.T) {
        result, _ := agent.Invoke(ctx, samplePlanPrompt, "high")
        plan, _ := ParsePlan(result.Output)
        for _, task := range plan.Tasks {
            if task.TaskID == "" {
                t.Error("task missing task_id")
            }
            if task.Objective == "" {
                t.Error("task missing objective")
            }
        }
    })
    t.Run("respects context cancellation", func(t *testing.T) {
        ctx, cancel := context.WithCancel(context.Background())
        cancel()
        _, err := agent.Invoke(ctx, samplePlanPrompt, "high")
        if err == nil {
            t.Error("expected error on cancelled context")
        }
    })
}
```

**`TestContract_ProviderPlanAgent_Claude`**
- Gated: `GROMIT_LLM_CONTRACT=1`.
- Wires: Claude provider -> LLMAdapter -> ProviderPlanAgent.
- Calls `RunPlanAgentContract(t, agent)`.

**`TestContract_ProviderPlanAgent_Codex`** (0002d)
- Gated: `GROMIT_LLM_CONTRACT=1`.
- Wires: Codex provider -> LLMAdapter -> ProviderPlanAgent.
- Calls `RunPlanAgentContract(t, agent)`.

### Review contract

File: `internal/next/review/agent_contract_test.go`

```go
func RunReviewAgentContract(t *testing.T, agent ReviewAgent) {
    t.Run("returns valid findings for well-formed prompt", func(t *testing.T) {
        findings, err := agent.ReviewFacet(ctx, "code_quality", sampleReviewPrompt)
        if err != nil {
            t.Fatalf("review facet: %v", err)
        }
        // Findings may be empty for clean code -- that is valid
        for _, f := range findings {
            if f.File == "" {
                t.Error("finding missing file field")
            }
            if f.Description == "" {
                t.Error("finding missing description field")
            }
            if f.Severity == 0 {
                t.Error("finding has zero severity (unset)")
            }
        }
    })
    t.Run("returns non-nil findings slice for clean code", func(t *testing.T) {
        findings, err := agent.ReviewFacet(ctx, "code_quality", sampleCleanCodePrompt)
        if err != nil {
            t.Fatalf("review facet: %v", err)
        }
        if findings == nil {
            t.Error("expected non-nil findings slice")
        }
        // Note: An LLM may still return style suggestions for clean code,
        // so we assert only that the slice is non-nil, not that it is empty.
    })
    t.Run("handles empty prompt gracefully", func(t *testing.T) {
        _, err := agent.ReviewFacet(ctx, "code_quality", "")
        // Should not panic; error is acceptable
        _ = err
    })
    t.Run("respects context cancellation", func(t *testing.T) {
        ctx, cancel := context.WithCancel(context.Background())
        cancel()
        _, err := agent.ReviewFacet(ctx, "code_quality", sampleReviewPrompt)
        if err == nil {
            t.Error("expected error on cancelled context")
        }
    })
}
```

**`TestContract_ProviderReviewAgent_Claude`** / **`TestContract_ProviderReviewAgent_Codex`**

### Acceptor contract

File: `internal/next/acceptor/agent_contract_test.go`

```go
func RunAcceptAgentContract(t *testing.T, agent AcceptAgent) {
    t.Run("returns valid criterion result", func(t *testing.T) {
        cr, err := agent.EvaluateCriterion(ctx, sampleAcceptPrompt)
        if err != nil {
            t.Fatalf("evaluate: %v", err)
        }
        switch cr.Status {
        case StatusPass, StatusFail, StatusUnclear:
            // valid
        default:
            t.Errorf("invalid status %q", cr.Status)
        }
    })
    t.Run("fail and unclear include rationale", func(t *testing.T) {
        cr, err := agent.EvaluateCriterion(ctx, sampleFailAcceptPrompt)
        if err != nil {
            t.Fatalf("evaluate: %v", err)
        }
        if cr.Status != StatusPass && cr.Rationale == "" {
            t.Error("non-pass result must include rationale")
        }
    })
    t.Run("respects context cancellation", func(t *testing.T) {
        ctx, cancel := context.WithCancel(context.Background())
        cancel()
        _, err := agent.EvaluateCriterion(ctx, sampleAcceptPrompt)
        if err == nil {
            t.Error("expected error on cancelled context")
        }
    })
}
```

**`TestContract_ProviderAcceptAgent_Claude`** / **`TestContract_ProviderAcceptAgent_Codex`**

### TaskRunner contract

File: `internal/next/specloop/taskrunner_contract_test.go`

```go
func RunTaskRunnerContract(t *testing.T, runner TaskRunner) {
    t.Run("RunTask returns status", func(t *testing.T) {
        result, err := runner.RunTask(ctx, sampleTask)
        if err != nil {
            t.Fatalf("run task: %v", err)
        }
        switch result.Status {
        case "done", "failed", "needs_split":
            // valid
        default:
            t.Errorf("invalid status %q", result.Status)
        }
    })
    t.Run("RepairTask includes failure context", func(t *testing.T) {
        result, err := runner.RepairTask(ctx, sampleTask, []string{"test_X failed"})
        if err != nil {
            t.Fatalf("repair task: %v", err)
        }
        if result.Status == "" {
            t.Error("repair result must have a status")
        }
    })
    t.Run("respects context cancellation", func(t *testing.T) {
        ctx, cancel := context.WithCancel(context.Background())
        cancel()
        _, err := runner.RunTask(ctx, sampleTask)
        if err == nil {
            t.Error("expected error on cancelled context")
        }
    })
}
```

**`TestContract_ProviderTaskRunner_Claude`** / **`TestContract_ProviderTaskRunner_Codex`**

### ShellValidator contract

File: `internal/next/validator/shell_validator_contract_test.go`

```go
func RunShellValidatorContract(t *testing.T, v FinalValidator) {
    t.Run("passing checks produce pass result", func(t *testing.T) {
        checks := []Check{{Name: "echo", Command: "echo ok", Type: "test"}}
        result, err := v.RunFinal(ctx, checks, nil, t.TempDir())
        if err != nil {
            t.Fatalf("run final: %v", err)
        }
        if !result.Pass {
            t.Error("expected pass for echo command")
        }
    })
    t.Run("failing check produces fail result", func(t *testing.T) {
        checks := []Check{{Name: "fail", Command: "exit 1", Type: "test"}}
        result, err := v.RunFinal(ctx, checks, nil, t.TempDir())
        if err != nil {
            t.Fatalf("run final: %v", err)
        }
        if result.Pass {
            t.Error("expected fail for exit 1 command")
        }
    })
}
```

---

## Integration Test Scenarios

All integration tests in files with build tag `//go:build integration`.

### Scenario 1: Happy path through all stages with mock provider

File: `internal/next/specloop/adapter_integration_test.go`

**`TestIntegration_AdapterLayer_HappyPath`**

Setup:
- Create fixture project in `t.TempDir()` with `go.mod`, `main.go`, passing test.
- Wire `mockProvider` with phase-aware responses: `mockProvider.Run` inspects the prompt for phase markers (e.g., `"## Phase: plan"`) and returns the corresponding canned response. Alternatively, use a call-sequence `mockProvider` that returns responses in order: first call → plan JSON, subsequent calls → implementation output, then review findings, then acceptance result. The sequence approach is simpler and preferred.
- Construct real adapters: `ProviderPlanAgent`, `ProviderTaskRunner`, `ProviderReviewAgent`, `ProviderAcceptAgent`.
- Wire into real stages. Use `ShellValidator` with `Runner` for validation.

Assertions:
- Pipeline completes. Terminal state is `ready_for_review` or `completed`.
- Cost tracked: `budget.AccumulatedCost() > 0`.
- Each adapter invoked at least once (verified via `mockProvider.calls`).
- Per-domain output correctly parsed into domain types.

### Scenario 2: Provider fallback on usage-limit (0002d)

File: `internal/next/llmadapter/fallback_integration_test.go`

**`TestIntegration_FallbackAdapter_UsageLimitFallback_ThroughRouter`**

Setup:
- `mockProviderWithUsageLimit{name: "claude", isUsageLimit: true}` returns usage-limit error.
- `mockProvider{name: "codex"}` returns valid result.
- Build a real `provider.Router` with both providers.
- Construct `FallbackAdapter` wrapping the router.

Assertions:
- `Invoke` succeeds with codex result.
- Claude marked unavailable in router.
- Result from codex returned to caller.

### Scenario 3: Router phase preferences (0002d)

File: `cmd/gromit-next/stage_provider_integration_test.go`

**`TestIntegration_RouterPhasePreferences`**

Setup:
- `policy.Routing.Preferences = {"plan": "claude", "execute": "codex", "review": "any"}`.
- `policy.Routing.Ratio = {"claude": 50, "codex": 50}`.
- Both mock providers wired.

Assertions:
- Plan adapter resolves to claude provider.
- Execute adapter resolves to codex provider.
- Review adapter resolves to either (based on ratio).

### Scenario 4: Cost tracking through adapter layer

File: `internal/next/llmadapter/cost_integration_test.go`

**`TestIntegration_CostTracking_AcrossAdapters`**

Setup:
- Single shared `costTotal` accumulator via `OnCost` callback.
- 3 adapters (plan, execute, review) each with `Config{OnCost: func(c) { costTotal += c }}`.
- Each invocation costs `0.05`.

Assertions:
- After 3 invocations (one per adapter), `costTotal == 0.15`.
- Budget reflects accumulated cost.

### Scenario 5: End-to-end with FallbackAdapter in pipeline (0002d)

File: `internal/next/specloop/routing_integration_test.go`

**`TestIntegration_FullPipeline_WithFallbackAdapters`**

Setup:
- Wire `FallbackAdapter` for each LLM stage (plan, execute, review, accept).
- Router with claude (primary) and codex (fallback).
- Claude provider returns valid results (no usage-limit).

Assertions:
- Pipeline completes normally using claude for all stages.
- No fallback triggered.
- Cost tracked through FallbackAdapter -> LLMAdapter -> OnCost callback chain.

---

## Evidence Checklist

Mapping spec requirements to test functions.

| # | Requirement (0002c) | Test(s) |
|---|---------------------|---------|
| C1 | LLMAdapter wraps provider.Provider with timeout and cost | `TestInvoke_CallsProviderRun_WithCorrectTier`, `TestInvoke_TimeoutEnforcement_CancelsContext`, `TestInvoke_OnCostCallback_FiresWithResultCost` |
| C2 | Invoker interface enables mock substitution | `TestLLMAdapter_SatisfiesInvoker`, `TestInvokerInterface_MethodSignature` |
| C3 | ProviderPlanAgent satisfies planner.Agent | `TestProviderPlanAgent_SatisfiesAgent`, `TestProviderPlanAgent_IntegrationWithPlanner` |
| C4 | ProviderReviewAgent satisfies review.ReviewAgent | `TestProviderReviewAgent_SatisfiesReviewAgent`, `TestProviderReviewAgent_ReviewFacet_ParsesFindings` |
| C5 | ProviderAcceptAgent satisfies acceptor.AcceptAgent | `TestProviderAcceptAgent_SatisfiesAcceptAgent`, `TestProviderAcceptAgent_EvaluateCriterion_ParsesResult` |
| C6 | ProviderTaskRunner satisfies specloop.TaskRunner | `TestProviderTaskRunner_SatisfiesTaskRunner`, `TestProviderTaskRunner_RunTask_ReturnsTaskResult` |
| C7 | ShellValidator delegates to validator.Runner | `TestShellValidator_RunFinal_DelegatesToRunner`, `TestShellValidator_SatisfiesFinalValidator` |
| C8 | SpecCompilerAdapter delegates to contextpkt.Compiler | `TestSpecCompilerAdapter_Compile_DelegatesToCompiler`, `TestSpecCompilerAdapter_SatisfiesSpecCompiler` |
| C9 | RealStageProvider wires real adapters replacing noops | `TestBuildStages_ReplacesNoopsWithRealAdapters` |
| C10 | ExtractJSON handles markdown fences | `TestExtractJSON_MarkdownFence`, `TestExtractJSON_ObjectInProse` |
| C11 | Cost feeds through adapter to Budget | `TestBuildStages_CostCallback_FeedsBudget`, `TestIntegration_CostTracking_AcrossAdapters` |
| C12 | Contract tests pass against Claude | `TestContract_ProviderPlanAgent_Claude`, `TestContract_ProviderReviewAgent_Claude`, `TestContract_ProviderAcceptAgent_Claude`, `TestContract_ProviderTaskRunner_Claude` |

| # | Requirement (0002d) | Test(s) |
|---|---------------------|---------|
| D1 | FallbackAdapter provides transparent usage-limit failover | `TestFallbackAdapter_UsageLimit_FallsBackToRouter`, `TestFallbackAdapter_InvokeStream_UsageLimit_FallsBackToRouter`, `TestIntegration_FallbackAdapter_UsageLimitFallback_ThroughRouter` |
| D2 | Non-usage-limit errors propagate without fallback | `TestFallbackAdapter_NonUsageLimitError_NoFallback` |
| D3 | All providers exhausted returns descriptive error | `TestFallbackAdapter_AllProvidersExhausted_ReturnsError` |
| D4 | FallbackAdapter satisfies ProviderAwareInvoker | `TestFallbackAdapter_SatisfiesProviderAwareInvoker` |
| D5 | Lazy provider resolution defers Select to first Invoke | `TestFallbackAdapter_LazyResolution_DefersSelectToFirstInvoke` |
| D6 | RoutingConfig validated (ratio sums to 100) | `TestPolicy_Validate_RoutingRatioSumsTo100`, `TestPolicy_Validate_RoutingRatioValid` |
| D7 | RealStageProvider wires Router for multi-provider | `TestBuildStages_BothProviders_FallbackAdaptersUsed`, `TestBuildRouter_ReturnsConfiguredRouter` |
| D8 | Router phase preferences respected | `TestIntegration_RouterPhasePreferences` |
| D9 | Codex contract tests pass | `TestContract_ProviderPlanAgent_Codex`, `TestContract_ProviderReviewAgent_Codex`, `TestContract_ProviderAcceptAgent_Codex`, `TestContract_ProviderTaskRunner_Codex` |
| D10 | Cost callback fires on fallback path | `TestFallbackAdapter_CostCallback_FiresOnFallback` |

---

## Test Fixtures and Fakes

### Mock definitions

All mocks live alongside their test files (package-internal). Shared test utilities live in `internal/next/testutil/`.

```go
// internal/next/llmadapter/adapter_test.go (or fallback_test.go)

// mockProvider satisfies provider.Provider for unit tests.
// Thread safety: mockProvider is designed for single-goroutine use only.
// If subtests use t.Parallel(), add sync.Mutex protection around calls and lastTier fields.
type mockProvider struct {
    name      string
    runResult *provider.Result
    runErr    error
    calls     int
    lastTier  string
}

func (m *mockProvider) Name() string                    { return m.name }
func (m *mockProvider) ModelForTier(tier string) string  { return "mock-" + tier }
func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
    m.calls++
    m.lastTier = tier
    return m.runResult, m.runErr
}
func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
    handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
    m.calls++
    m.lastTier = tier
    return m.runResult, m.runErr
}
func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
    return m.runResult, m.runErr
}
func (m *mockProvider) IsUsageLimitError(r *provider.Result, err error) bool      { return false }
func (m *mockProvider) IsValidationPassed(r *provider.Result) bool                { return true }
func (m *mockProvider) IsScopeTooLarge(r *provider.Result) (bool, string)         { return false, "" }

// mockProviderWithUsageLimit extends mockProvider with configurable IsUsageLimitError.
type mockProviderWithUsageLimit struct {
    mockProvider
    isUsageLimit bool
}

func (m *mockProviderWithUsageLimit) IsUsageLimitError(r *provider.Result, err error) bool {
    return m.isUsageLimit
}

// mockRouter satisfies RouterSelector for FallbackAdapter tests.
type mockRouter struct {
    selectProvider         provider.Provider
    selectModel            string
    selectSequence         []mockSelectResult
    selectIdx              int
    selectCalled           bool
    markUnavailableCalled  bool
    markUnavailableName    string
}

type mockSelectResult struct {
    prov  provider.Provider
    model string
}

func (m *mockRouter) Select(phase string, tier string) (provider.Provider, string) {
    m.selectCalled = true
    if len(m.selectSequence) > 0 {
        if m.selectIdx >= len(m.selectSequence) {
            return nil, ""
        }
        r := m.selectSequence[m.selectIdx]
        m.selectIdx++
        return r.prov, r.model
    }
    return m.selectProvider, m.selectModel
}

func (m *mockRouter) MarkUnavailable(name string) {
    m.markUnavailableCalled = true
    m.markUnavailableName = name
}
```

```go
// internal/next/llmadapter/ (shared across domain adapter tests)

// mockInvoker satisfies llmadapter.Invoker for per-domain adapter tests.
type mockInvoker struct {
    result    *provider.Result
    err       error
    calls     int
    lastPrompt string
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
    m.calls++
    m.lastPrompt = prompt
    return m.result, m.err
}
```

### Canned responses

Located in `internal/next/testutil/responses/`. JSON files with canned LLM outputs:

```
testutil/responses/
  valid_plan_2tasks.json            # Plan with 2 tasks, valid structure
  valid_findings_warning.json       # Array of 2 findings (severity warning)
  valid_findings_empty.json         # Empty array []
  valid_criterion_pass.json         # {status: "pass", rationale: "..."}
  valid_criterion_fail.json         # {status: "fail", rationale: "..."}
  valid_task_result.json            # Implementation output for task runner
  invalid_json_malformed.json       # Broken JSON for error path testing
  markdown_fenced_findings.json     # Findings wrapped in ```json fences
```

### Fixture: sample prompts for contract tests

```go
// internal/next/testutil/contract_prompts.go

var SamplePlanPrompt = `You are a planning agent. Generate an execution plan as JSON.
## Spec Packet
Add a helper function Sum(a, b int) int to pkg/util/math.go.
## Cycle: 1
Respond with a JSON object containing spec_id, cycle, kind, and tasks array.`

var SampleReviewPrompt = `Review the following diff for code quality issues.
## Diff
+func Sum(a, b int) int { return a + b }
Respond with a JSON array of findings.`

var SampleAcceptPrompt = `Evaluate whether this criterion is met:
Criterion: "Sum function exists and has a unit test"
## Diff Summary
Added Sum function with test.
Respond with JSON: {status, rationale, evidence_refs}`
```

---

## Running Tests

```bash
# Unit tests -- llmadapter package
go test ./internal/next/llmadapter/... -v -count=1

# Unit tests -- per-domain adapter tests
go test ./internal/next/planner/ -run TestProviderPlanAgent -v -count=1
go test ./internal/next/review/ -run TestProviderReviewAgent -v -count=1
go test ./internal/next/acceptor/ -run TestProviderAcceptAgent -v -count=1
go test ./internal/next/specloop/ -run TestProviderTaskRunner -v -count=1

# Unit tests -- non-LLM adapters
go test ./internal/next/validator/ -run TestShellValidator -v -count=1
go test ./internal/next/contextpkt/ -run TestSpecCompilerAdapter -v -count=1

# Unit tests -- execpolicy routing config
go test ./internal/next/execpolicy/ -run TestPolicy_Validate_Routing -v -count=1

# Unit tests -- RealStageProvider wiring
go test ./cmd/gromit-next/ -run TestBuildStages -v -count=1

# All unit tests across affected packages
go test ./internal/next/llmadapter/... ./internal/next/planner/... \
       ./internal/next/review/... ./internal/next/acceptor/... \
       ./internal/next/specloop/... ./internal/next/validator/... \
       ./internal/next/contextpkt/... ./internal/next/execpolicy/... \
       ./cmd/gromit-next/... -v -count=1

# Integration tests (mock provider, real filesystem)
go test -tags integration ./internal/next/... -v -count=1

# Contract tests -- Claude (local only, costs money)
GROMIT_LLM_CONTRACT=1 go test ./internal/next/planner/ -run TestContract.*Claude -v -count=1 -timeout 120s
GROMIT_LLM_CONTRACT=1 go test ./internal/next/review/ -run TestContract.*Claude -v -count=1 -timeout 120s
GROMIT_LLM_CONTRACT=1 go test ./internal/next/acceptor/ -run TestContract.*Claude -v -count=1 -timeout 120s
GROMIT_LLM_CONTRACT=1 go test ./internal/next/specloop/ -run TestContract.*Claude -v -count=1 -timeout 120s

# Contract tests -- Codex (local only, costs money)
GROMIT_LLM_CONTRACT=1 go test ./internal/next/planner/ -run TestContract.*Codex -v -count=1 -timeout 120s
GROMIT_LLM_CONTRACT=1 go test ./internal/next/review/ -run TestContract.*Codex -v -count=1 -timeout 120s
GROMIT_LLM_CONTRACT=1 go test ./internal/next/acceptor/ -run TestContract.*Codex -v -count=1 -timeout 120s
GROMIT_LLM_CONTRACT=1 go test ./internal/next/specloop/ -run TestContract.*Codex -v -count=1 -timeout 120s

# All contract tests -- both providers
GROMIT_LLM_CONTRACT=1 go test ./internal/next/... -run TestContract -v -count=1 -timeout 300s

# Full test suite (unit + integration, no contract)
go test -tags integration ./internal/next/... ./cmd/gromit-next/... -v -count=1
```
