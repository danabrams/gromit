# Spec 0002d Implementation Plan — Multi-Provider Routing

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the hardcoded Claude provider in `RealStageProvider` with the existing `provider.Router`, enabling Codex (and future providers) as alternatives with automatic fallback on usage-limit errors.

**Architecture:** Add a `FallbackAdapter` to `llmadapter/` that wraps the base `LLMAdapter` with transparent usage-limit failover via the Router. Update `RealStageProvider` to use Router-selected providers per stage phase. Validate Codex compatibility via contract tests.

**Tech Stack:** Go, `provider.Router`, `provider.CodexProvider`, `llmadapter.LLMAdapter`

**Depends on:** Spec 0002c (adapter layer must be complete)

---

### Task 1: `FallbackAdapter` — failing tests

**Files:**
- Create: `internal/next/llmadapter/fallback_test.go`

**Step 1: Write the failing tests**

```go
package llmadapter

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestFallbackAdapter_NormalInvocation_NoFallback(t *testing.T) {
	primary := &mockInvokerWithProvider{
		result: &provider.Result{Output: "hello", CostUSD: 0.01},
		prov:   &mockProvider{name: "claude"},
	}
	fa := NewFallbackAdapter(primary, nil, "build")
	result, err := fa.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
}

func TestFallbackAdapter_UsageLimit_FallsBackToRouter(t *testing.T) {
	// Primary returns usage-limit error
	primaryProv := &mockProviderWithUsageLimit{name: "claude", isUsageLimit: true}
	primary := &mockInvokerWithProvider{
		result: &provider.Result{Output: "", ExitCode: 2},
		err:    errors.New("usage limit"),
		prov:   primaryProv,
	}
	// Router returns fallback provider
	fallbackResult := &provider.Result{Output: "fallback worked", CostUSD: 0.02}
	fallbackProv := &mockProvider{name: "codex", runResult: fallbackResult}
	router := &mockRouter{
		selectProvider: fallbackProv,
		selectModel:    "gpt-5.3-codex",
	}
	fa := NewFallbackAdapter(primary, router, "build")
	result, err := fa.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "fallback worked" {
		t.Errorf("expected 'fallback worked', got %q", result.Output)
	}
	if !router.markUnavailableCalled {
		t.Error("expected router.MarkUnavailable to be called")
	}
}

func TestFallbackAdapter_NonUsageLimitError_NoFallback(t *testing.T) {
	primaryProv := &mockProviderWithUsageLimit{name: "claude", isUsageLimit: false}
	primary := &mockInvokerWithProvider{
		result: &provider.Result{Output: ""},
		err:    errors.New("network timeout"),
		prov:   primaryProv,
	}
	fa := NewFallbackAdapter(primary, nil, "build")
	_, err := fa.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestFallbackAdapter_SatisfiesInvoker(t *testing.T) {
	// Compile-time check is sufficient, but verify runtime too
	var _ Invoker = (*FallbackAdapter)(nil)
}
```

Mock types for tests will need to be defined (router mock, provider-with-usage-limit mock, invoker-with-provider mock).

**Step 2: Run and verify failure**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/llmadapter/ -run TestFallbackAdapter -v -count=1`
Expected: FAIL

**Step 3: Commit**

```bash
git add internal/next/llmadapter/fallback_test.go
git commit -m "red: FallbackAdapter tests for usage-limit failover"
```

---

### Task 2: Implement `FallbackAdapter`

**Files:**
- Create: `internal/next/llmadapter/fallback.go`

**Step 1: Write implementation**

```go
package llmadapter

import (
	"context"

	"github.com/danabrams/gromit/internal/provider"
)

// ProviderAwareInvoker extends Invoker with provider identity for fallback logic.
type ProviderAwareInvoker interface {
	Invoker
	Provider() provider.Provider
}

// RouterSelector abstracts the router's Select and MarkUnavailable methods.
type RouterSelector interface {
	Select(phase string, tier string) (provider.Provider, string)
	MarkUnavailable(name string)
}

// FallbackAdapter wraps an Invoker with automatic fallback on usage-limit errors.
// If the primary invocation fails with a usage-limit error, it marks the provider
// unavailable via the router and retries with the next available provider.
// Domain adapters are unaware of this fallback — same Invoker interface.
type FallbackAdapter struct {
	primary ProviderAwareInvoker
	router  RouterSelector
	phase   string
	tier    string
	cfg     Config
}

// NewFallbackAdapter creates a FallbackAdapter.
func NewFallbackAdapter(primary ProviderAwareInvoker, router RouterSelector, phase string) *FallbackAdapter {
	return &FallbackAdapter{
		primary: primary,
		router:  router,
		phase:   phase,
	}
}

// Invoke delegates to the primary invoker. On usage-limit error, falls back via router.
func (f *FallbackAdapter) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	result, err := f.primary.Invoke(ctx, prompt)
	if err != nil && f.router != nil && f.primary.Provider().IsUsageLimitError(result, err) {
		f.router.MarkUnavailable(f.primary.Provider().Name())
		fallbackProv, _ := f.router.Select(f.phase, f.tier)
		fallback := New(fallbackProv, f.cfg)
		return fallback.Invoke(ctx, prompt)
	}
	return result, err
}

var _ Invoker = (*FallbackAdapter)(nil)
```

**Step 2: Run tests**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/llmadapter/ -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/next/llmadapter/fallback.go
git commit -m "green: FallbackAdapter with transparent usage-limit failover"
```

---

### Task 3: Update `RealStageProvider` to use Router

**Files:**
- Modify: `cmd/gromit-next/stage_provider.go`
- Modify: `cmd/gromit-next/exec.go` (add `--provider` flag or routing config)

**Step 1: Update BuildStages to accept Router**

Replace the hardcoded Claude provider (from 0002c) with Router-based selection:

```go
func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget) ([]specloop.Stage, error) {
    router := p.buildRouter(policy)
    costCallback := func(c float64) { budget.AddCost(c) }

    // Select provider per stage phase
    planProv, _ := router.Select("plan", policy.Models.Planner)
    execProv, _ := router.Select("execute", policy.Models.DefaultTier)
    reviewProv, _ := router.Select("review", policy.Models.Evaluator)
    validateProv, _ := router.Select("validate", provider.TierLow)

    // Build adapters with fallback
    planAdapter := llmadapter.NewFallbackAdapter(
        llmadapter.NewProviderAware(planProv, llmadapter.Config{Tier: policy.Models.Planner, OnCost: costCallback}),
        router, "plan")
    // ... same pattern for exec, review, validate
}
```

**Step 2: Add router construction**

```go
func (p *RealStageProvider) buildRouter(policy execpolicy.Policy) *provider.Router {
    // Build providers from policy config
    // For now: Claude always available, Codex if configured
    // This will be extended as provider config matures
}
```

**Step 3: Run tests**

Run: `cd /Users/dabrams/gromit && go test ./cmd/gromit-next/ -v -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/gromit-next/stage_provider.go cmd/gromit-next/exec.go
git commit -m "feat: wire Router into RealStageProvider for multi-provider routing"
```

---

### Task 4: Codex contract tests

**Files:**
- Modify: `internal/next/planner/agent_contract_test.go` — add Codex test
- Modify: `internal/next/review/agent_contract_test.go` — add Codex test
- Modify: `internal/next/acceptor/agent_contract_test.go` — add Codex test
- Modify: `internal/next/specloop/taskrunner_contract_test.go` — add Codex test

**Step 1: Add Codex contract tests alongside Claude**

Each contract test file gets a second test function that wires the real Codex provider:

```go
func TestContract_ProviderPlanAgent_Codex(t *testing.T) {
    if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
        t.Skip("set GROMIT_LLM_CONTRACT=1")
    }
    agent := buildRealPlanAgentCodex(t)
    RunPlanAgentContract(t, agent)
}

func buildRealPlanAgentCodex(t *testing.T) Agent {
    t.Helper()
    // Wire: codex binary → CodexProvider → LLMAdapter → ProviderPlanAgent
    // Uses provider.NewCodexProvider(...)
}
```

**Step 2: Run contract tests against Codex locally**

Run: `cd /Users/dabrams/gromit && GROMIT_LLM_CONTRACT=1 go test -tags llmcontract ./internal/next/planner/ -run TestContract_ProviderPlanAgent_Codex -v -count=1 -timeout 120s`
Expected: PASS — Codex satisfies structural contract

**Step 3: If any contract fails — investigate prompt compatibility**

Codex may need different JSON output instructions. If findings/plans don't parse:
- Add prompt wrappers in the domain adapter that add explicit "respond with JSON" instructions
- OR add provider-specific prompt suffixes in the LLMAdapter

**Step 4: Commit**

```bash
git add internal/next/planner/agent_contract_test.go internal/next/review/agent_contract_test.go internal/next/acceptor/agent_contract_test.go internal/next/specloop/taskrunner_contract_test.go
git commit -m "feat: Codex contract tests — validate provider compatibility"
```

---

### Task 5: Integration scenarios for multi-provider routing

**Files:**
- Modify: `internal/next/specloop/integration_test.go`

**Step 1: Add routing-specific scenarios**

```go
func TestIntegration_ProviderFallbackOnUsageLimit(t *testing.T) {
    if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
        t.Skip("set GROMIT_LLM_CONTRACT=1")
    }
    t.Skip("TODO: requires simulated usage-limit — manual test")
}

func TestIntegration_RouterPhasePreferences(t *testing.T) {
    if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
        t.Skip("set GROMIT_LLM_CONTRACT=1")
    }
    t.Skip("TODO: verify correct provider selected per phase preference")
}
```

**Step 2: Commit**

```bash
git add internal/next/specloop/integration_test.go
git commit -m "feat: integration test scaffolds for multi-provider routing scenarios"
```

---

### Task 6: Final verification

**Step 1: Run all unit tests**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/... ./cmd/gromit-next/ -v -count=1`
Expected: PASS

**Step 2: Run contract tests against both providers**

Run: `cd /Users/dabrams/gromit && GROMIT_LLM_CONTRACT=1 go test -tags llmcontract ./internal/next/... -run TestContract -v -count=1 -timeout 300s`
Expected: PASS for both Claude and Codex

**Step 3: Commit**

```bash
git commit -m "chore: final verification for spec 0002d"
```
