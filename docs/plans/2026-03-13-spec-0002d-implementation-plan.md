# Spec 0002d Implementation Plan — Multi-Provider Routing

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the hardcoded Claude provider in `RealStageProvider` with the existing `provider.Router`, enabling Codex (and future providers) as alternatives with automatic fallback on usage-limit errors.

**Architecture:** Add a `FallbackAdapter` to `llmadapter/` that lazily selects providers via the Router on first `Invoke` call, with transparent usage-limit failover (single-hop). Update `RealStageProvider` to wire FallbackAdapters per stage phase (provider selection deferred to invocation time). Validate Codex compatibility via contract tests.

**Tech Stack:** Go, `provider.Router`, `provider.CodexProvider`, `llmadapter.LLMAdapter`

**Depends on:** Spec 0002c (adapter layer must be complete). Specifically, 0002d depends on:
- `ProviderAwareInvoker` interface (extends `Invoker` with `Provider() provider.Provider`) defined in `llmadapter` — 0002d must NOT redefine it
- `llmadapter.New()` constructor — used by `FallbackAdapter` to wrap providers
- `*LLMAdapter` satisfies `ProviderAwareInvoker` (has `Provider() provider.Provider` method)
- `llmadapter.Config` struct — passed through by `FallbackAdapter` when constructing adapters

---

### Task 1: `FallbackAdapter` — failing tests

> **Prerequisite:** Spec 0002c must define `ProviderAwareInvoker` (extends `Invoker` with `Provider() provider.Provider`) and the `NewProviderAware()` constructor before 0002d work begins.

**Files:**
- Create: `internal/next/llmadapter/fallback_test.go`

**Step 1: Write the failing tests**

```go
package llmadapter

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestFallbackAdapter_NormalInvocation_NoFallback(t *testing.T) {
	// Pre-resolved primary — router Select returns a known provider
	primaryProv := &mockProvider{name: "claude", runResult: &provider.Result{Output: "hello", CostUSD: 0.01}}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	result, err := fa.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
}

func TestFallbackAdapter_UsageLimit_FallsBackToRouter(t *testing.T) {
	// Primary returns usage-limit error on first Select
	primaryProv := &mockProviderWithUsageLimit{name: "claude", isUsageLimit: true,
		runResult: &provider.Result{Output: "", ExitCode: 2}, runErr: errors.New("usage limit")}
	fallbackResult := &provider.Result{Output: "fallback worked", CostUSD: 0.02}
	fallbackProv := &mockProvider{name: "codex", runResult: fallbackResult}
	// Router returns primary first, then fallback on second Select
	router := &mockRouter{
		selectSequence: []mockSelectResult{
			{prov: primaryProv, model: "claude-opus"},
			{prov: fallbackProv, model: "gpt-5.3-codex"},
		},
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
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
	primaryProv := &mockProviderWithUsageLimit{name: "claude", isUsageLimit: false,
		runResult: &provider.Result{Output: ""}, runErr: errors.New("network timeout")}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	_, err := fa.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestFallbackAdapter_AllProvidersExhausted_ReturnsError(t *testing.T) {
	primaryProv := &mockProviderWithUsageLimit{name: "claude", isUsageLimit: true,
		runResult: &provider.Result{Output: "", ExitCode: 2}, runErr: errors.New("usage limit")}
	// Router returns primary first, then nil on second Select (all exhausted)
	router := &mockRouter{
		selectSequence: []mockSelectResult{
			{prov: primaryProv, model: "claude-opus"},
			{prov: nil, model: ""},
		},
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	_, err := fa.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error when all providers exhausted")
	}
	if !strings.Contains(err.Error(), "all providers exhausted") {
		t.Errorf("expected 'all providers exhausted' in error, got %q", err.Error())
	}
}

func TestFallbackAdapter_SatisfiesProviderAwareInvoker(t *testing.T) {
	// Compile-time check — FallbackAdapter must satisfy ProviderAwareInvoker
	var _ ProviderAwareInvoker = (*FallbackAdapter)(nil)
}

func TestFallbackAdapter_Provider_ReturnsPrimaryProvider(t *testing.T) {
	primaryProv := &mockProvider{name: "claude"}
	router := &mockRouter{
		selectProvider: primaryProv,
		selectModel:    "claude-opus",
	}
	fa := NewFallbackAdapter(router, "build", Config{}, "medium")
	// Force lazy init by calling Provider()
	p := fa.Provider()
	if p.Name() != "claude" {
		t.Errorf("expected provider name 'claude', got %q", p.Name())
	}
}
```

**Mock type definitions** (add to `fallback_test.go` before the test functions):

```go
// mockProvider satisfies provider.Provider with configurable results.
type mockProvider struct {
	name      string
	runResult *provider.Result
	runErr    error
}

func (m *mockProvider) Name() string                                              { return m.name }
func (m *mockProvider) ModelForTier(tier string) string                           { return tier }
func (m *mockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return m.runResult, m.runErr
}
func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return m.runResult, m.runErr
}
func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return m.runResult, m.runErr
}
func (m *mockProvider) IsUsageLimitError(r *provider.Result, err error) bool      { return false }
func (m *mockProvider) IsValidationPassed(r *provider.Result) bool                { return true }
func (m *mockProvider) IsScopeTooLarge(r *provider.Result) (bool, string)         { return false, "" }

// mockProviderWithUsageLimit is like mockProvider but with configurable IsUsageLimitError.
type mockProviderWithUsageLimit struct {
	name         string
	runResult    *provider.Result
	runErr       error
	isUsageLimit bool
}

func (m *mockProviderWithUsageLimit) Name() string                                              { return m.name }
func (m *mockProviderWithUsageLimit) ModelForTier(tier string) string                           { return tier }
func (m *mockProviderWithUsageLimit) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return m.runResult, m.runErr
}
func (m *mockProviderWithUsageLimit) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return m.runResult, m.runErr
}
func (m *mockProviderWithUsageLimit) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return m.runResult, m.runErr
}
func (m *mockProviderWithUsageLimit) IsUsageLimitError(r *provider.Result, err error) bool      { return m.isUsageLimit }
func (m *mockProviderWithUsageLimit) IsValidationPassed(r *provider.Result) bool                { return true }
func (m *mockProviderWithUsageLimit) IsScopeTooLarge(r *provider.Result) (bool, string)         { return false, "" }

// mockSelectResult holds a provider/model pair for sequence-based mock router.
type mockSelectResult struct {
	prov  provider.Provider
	model string
}

// mockRouter satisfies RouterSelector with two modes:
// - Single-result mode: selectProvider/selectModel returned on every Select call
// - Sequence mode: selectSequence consumed in order (for testing fallback chains)
type mockRouter struct {
	// Single-result mode
	selectProvider provider.Provider
	selectModel    string

	// Sequence mode (takes priority over single-result if non-empty)
	selectSequence       []mockSelectResult
	selectIdx            int
	markUnavailableCalled bool
}

func (m *mockRouter) Select(phase string, tier string) (provider.Provider, string) {
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
}
```

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
	"fmt"
	"log"
	"sync"

	"github.com/danabrams/gromit/internal/provider"
)

// NOTE: ProviderAwareInvoker is defined by 0002c in the llmadapter package.
// Do NOT redefine it here — import and use it from llmadapter.
// The interface extends Invoker with Provider() provider.Provider.

// RouterSelector abstracts the router's Select and MarkUnavailable methods.
type RouterSelector interface {
	Select(phase string, tier string) (provider.Provider, string)
	MarkUnavailable(name string)
}

// FallbackAdapter wraps provider selection with automatic fallback on usage-limit errors.
// Provider selection is deferred to first Invoke call (lazy Select) so that
// provider availability is evaluated at invocation time, not at pipeline build time.
// Current design supports single-hop fallback only (primary -> one fallback).
// If all providers fail, the last error is returned. N-hop fallback can be
// added later by looping through available providers.
// Domain adapters are unaware of this fallback — same ProviderAwareInvoker interface.
//
// Known limitation: Router.Select increments the provider invocation count as a
// side effect. On the fallback path, the failed primary's count is already
// incremented. A future iteration should split Select into a read-only selection
// method and a separate RecordInvocation method to avoid double-counting.
type FallbackAdapter struct {
	router  RouterSelector
	phase   string
	tier    string
	cfg     Config

	mu      sync.Mutex
	primary ProviderAwareInvoker  // resolved lazily on first Invoke; re-checked if nil
}

// NewFallbackAdapter creates a FallbackAdapter that lazily resolves the primary
// provider via Router.Select on first Invoke call.
// cfg and tier are passed through so the fallback adapter can construct a
// properly configured LLMAdapter when falling back to a different provider.
func NewFallbackAdapter(router RouterSelector, phase string, cfg Config, tier string) *FallbackAdapter {
	return &FallbackAdapter{
		router: router,
		phase:  phase,
		cfg:    cfg,
		tier:   tier,
	}
}

// resolvePrimary selects the primary provider from the router.
func (f *FallbackAdapter) resolvePrimary() ProviderAwareInvoker {
	prov, _ := f.router.Select(f.phase, f.tier)
	if prov == nil {
		return nil
	}
	return New(prov, f.cfg)
}

// Provider returns the primary adapter's provider (satisfies ProviderAwareInvoker).
// Note: triggers lazy initialization if not yet resolved. Re-resolves if primary
// was previously nil (recovery-after-cooldown semantics).
func (f *FallbackAdapter) Provider() provider.Provider {
	f.mu.Lock()
	if f.primary == nil {
		f.primary = f.resolvePrimary()
	}
	p := f.primary
	f.mu.Unlock()
	if p == nil {
		return nil
	}
	return p.Provider()
}

// Invoke delegates to the primary invoker, resolved lazily on first call.
// On usage-limit error, logs the primary error and falls back via router.
// Each call re-checks if primary is nil under the mutex, allowing
// recovery after a provider's cooldown expires.
func (f *FallbackAdapter) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	f.mu.Lock()
	if f.primary == nil {
		f.primary = f.resolvePrimary()
	}
	f.mu.Unlock()
	if f.primary == nil {
		return nil, fmt.Errorf("no providers available for phase %q tier %q", f.phase, f.tier)
	}
	result, err := f.primary.Invoke(ctx, prompt)
	// Nil guard: primary.Provider() can be nil if the provider was constructed
	// without a backing provider (e.g., in certain test scenarios).
	prov := f.primary.Provider()
	if err != nil && prov != nil && prov.IsUsageLimitError(result, err) {
		primaryName := prov.Name()
		// Log primary error before attempting fallback
		log.Printf("provider %s hit usage limit, attempting fallback: %v", primaryName, err)
		f.router.MarkUnavailable(primaryName)
		// Known limitation: Router.Select increments the provider invocation count
		// as a side effect. On this fallback path, the failed primary's count was
		// already incremented by the first Select call. A future iteration should
		// split Select into a read-only selection method and a separate
		// RecordInvocation method to avoid double-counting.
		fallbackProv, _ := f.router.Select(f.phase, f.tier)
		if fallbackProv == nil {
			return result, fmt.Errorf("all providers exhausted after %s usage limit: %w", primaryName, err)
		}
		fallback := New(fallbackProv, f.cfg)
		fallbackResult, fallbackErr := fallback.Invoke(ctx, prompt)
		if fallbackErr != nil {
			return fallbackResult, fmt.Errorf("fallback provider %s also failed (primary was %s): %w", fallbackProv.Name(), primaryName, fallbackErr)
		}
		log.Printf("provider fallback: %s (usage limit) -> %s (success)", primaryName, fallbackProv.Name())
		return fallbackResult, nil
	}
	return result, err
}

// Compile-time interface checks.
var _ Invoker = (*FallbackAdapter)(nil)
var _ ProviderAwareInvoker = (*FallbackAdapter)(nil)
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

**Step 0a: Add provider fields to `RealStageProvider`**

`RealStageProvider` currently has no provider fields. Add the following to `cmd/gromit-next/stage_provider.go`:

```go
// RealStageProvider fields needed for multi-provider routing:
type RealStageProvider struct {
    // ... existing fields ...
    claudeProvider  provider.Provider        // required — always present
    codexProvider   provider.Provider        // optional — nil in single-provider mode
    stateFn         provider.StateFile       // optional — for stateful routing
    circuitBreaker  *provider.CircuitBreaker // optional — for circuit-breaking
}
```

Update `RealStageProviderConfig` to accept these:

```go
type RealStageProviderConfig struct {
    // ... existing fields ...
    ClaudeProvider  provider.Provider        // required
    CodexProvider   provider.Provider        // optional
    StateFn         provider.StateFile       // optional
    CircuitBreaker  *provider.CircuitBreaker // optional
}
```

Update the constructor to wire them:

```go
func NewRealStageProvider(cfg RealStageProviderConfig) *RealStageProvider {
    return &RealStageProvider{
        // ... existing wiring ...
        claudeProvider:  cfg.ClaudeProvider,
        codexProvider:   cfg.CodexProvider,
        stateFn:         cfg.StateFn,
        circuitBreaker:  cfg.CircuitBreaker,
    }
}
```

**Step 0b: Add `Routing` config to `execpolicy.Policy`**

The `Policy` struct currently has no `Routing` field. Add it to `internal/next/execpolicy/policy.go`:

```go
// RoutingConfig defines multi-provider routing preferences.
type RoutingConfig struct {
    Preferences    map[string]string `json:"preferences"`      // phase -> provider name or "any"
    Ratio          map[string]int    `json:"ratio"`             // provider name -> percentage (must sum to 100)
    CooldownSeconds int              `json:"cooldown_seconds"`  // how long (in seconds) to mark a provider unavailable after usage-limit
}

// NormalizeNilFields maps nil slices/maps to empty values.
func (rc *RoutingConfig) NormalizeNilFields() {
    if rc.Preferences == nil {
        rc.Preferences = map[string]string{}
    }
    if rc.Ratio == nil {
        rc.Ratio = map[string]int{}
    }
}
```

Add the field to `Policy`:
```go
Routing RoutingConfig `json:"routing"`
```

Add defaults in `DefaultPolicy()`:
```go
Routing: RoutingConfig{
    Preferences:    map[string]string{"plan": "any", "execute": "any", "review": "any", "validate": "any"},
    Ratio:          map[string]int{"claude": 100},
    CooldownSeconds: 300, // 5 minutes
},
```

Add nil-field normalization for the new maps in `Policy.NormalizeNilFields()`:
```go
func (p *Policy) NormalizeNilFields() {
    // ... existing normalization ...
    p.Routing.NormalizeNilFields()
}
```

Add validation in `Policy.Validate()`:
```go
// Validate ratio values sum to 100.
if len(p.Routing.Ratio) > 0 {
    sum := 0
    for _, v := range p.Routing.Ratio {
        sum += v
    }
    if sum != 100 {
        return fmt.Errorf("routing.ratio values must sum to 100, got %d", sum)
    }
}
// Provider name validation is deferred to router construction time,
// where the actual provider map is known. Policy.Validate() only checks
// structural invariants (ratio sums to 100). The router constructor will
// reject unknown provider names when it receives the providers map.
```

Corresponding test cases for validation:
```go
func TestPolicy_Validate_RoutingRatioSumsTo100(t *testing.T) {
    p := DefaultPolicy()
    p.Routing.Ratio = map[string]int{"claude": 70, "codex": 20}
    err := p.Validate()
    if err == nil || !strings.Contains(err.Error(), "sum to 100") {
        t.Errorf("expected ratio sum validation error, got %v", err)
    }
}

// NOTE: Unknown provider name validation is NOT tested here because it is
// deferred to router construction time (where the actual provider map is known).
// See provider.NewRouter tests for unknown-name rejection.

func TestPolicy_Validate_RoutingRatioValid(t *testing.T) {
    p := DefaultPolicy()
    p.Routing.Ratio = map[string]int{"claude": 70, "codex": 30}
    err := p.Validate()
    if err != nil {
        t.Errorf("expected no error for valid ratio, got %v", err)
    }
}
```

**Step 0c: Update `exec.go` to construct providers and pass them to `RealStageProviderConfig`**

In `cmd/gromit-next/exec.go`, inside the `RunE` closure where `NewRealStageProvider` is called,
add provider construction before the `RealStageProviderConfig`:

```go
// Construct the Claude provider (always required).
claudeClient := claude.NewClient() // or however the claude client is created
claudeTierMap := map[string]string{
    provider.TierXHigh:  "opus",
    provider.TierHigh:   "opus",
    provider.TierMedium: "sonnet",
    provider.TierLow:    "haiku",
}
claudeProv := provider.NewClaudeProvider(claudeClient, claudeTierMap)

// Optionally construct the Codex provider (only if codex binary is available).
var codexProv provider.Provider
codexBinary, err := exec.LookPath("codex")
if err == nil {
    codexTierMap := map[string]string{
        provider.TierMedium: "gpt-5.3-codex",
        provider.TierLow:    "gpt-5.1-codex-mini",
    }
    codexProv = provider.NewCodexProvider(codexBinary, nil, codexTierMap)
}

p = NewRealStageProvider(RealStageProviderConfig{
    WorkDir:        workDir,
    StoreDir:       storeDir,
    SpecPath:       specPath,
    PolicyPath:     policyPath,
    ClaudeProvider: claudeProv,
    CodexProvider:  codexProv,
})
```

Note: The exact `claude.NewClient()` constructor depends on 0002c's adapter wiring.
The key point is that `exec.go` constructs both providers and passes them into
`RealStageProviderConfig`, which then forwards them to `buildRouter`.

**Step 1: Update BuildStages to use Router with lazy selection**

Replace the hardcoded Claude provider (from 0002c) with FallbackAdapter wrapping the Router.
Provider selection is deferred to first Invoke (lazy Select), so BuildStages does NOT
call `router.Select` — it passes the Router to FallbackAdapter instead:

```go
func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget) ([]specloop.Stage, error) {
    router := p.buildRouter(policy)
    costCallback := func(c float64) { budget.AddCost(c) }

    // Build adapters with lazy provider selection via FallbackAdapter.
    // Provider is resolved on first Invoke, not here.
    planCfg := llmadapter.Config{Tier: policy.Models.Planner, OnCost: costCallback}
    planAdapter := llmadapter.NewFallbackAdapter(
        router, "plan", planCfg, policy.Models.Planner)

    execCfg := llmadapter.Config{Tier: policy.Models.Executor, OnCost: costCallback}
    execAdapter := llmadapter.NewFallbackAdapter(
        router, "execute", execCfg, policy.Models.Executor)

    reviewCfg := llmadapter.Config{Tier: policy.Models.Evaluator, OnCost: costCallback}
    reviewAdapter := llmadapter.NewFallbackAdapter(
        router, "review", reviewCfg, policy.Models.Evaluator)

    acceptCfg := llmadapter.Config{Tier: policy.Models.Evaluator, OnCost: costCallback}
    acceptAdapter := llmadapter.NewFallbackAdapter(
        router, "accept", acceptCfg, policy.Models.Evaluator)

    // ShellValidator does not use LLM — no adapter needed for validate phase.
    // ... wire adapters into stages (planAdapter, execAdapter, reviewAdapter, acceptAdapter)
}
```

**Step 2: Add router construction**

```go
func (p *RealStageProvider) buildRouter(policy execpolicy.Policy) *provider.Router {
    // Build providers map from policy config
    providers := map[string]provider.Provider{
        "claude": p.claudeProvider,
    }
    if p.codexProvider != nil {
        providers["codex"] = p.codexProvider
    }

    // Phase preferences and ratio from policy routing config
    preferences := policy.Routing.Preferences     // e.g. {"plan": "claude", "execute": "any"}
    ratio := policy.Routing.Ratio                  // e.g. {"claude": 70, "codex": 30}
    // Convert CooldownSeconds (int) to time.Duration internally
    cooldown := time.Duration(policy.Routing.CooldownSeconds) * time.Second

    // NewRouter signature (from provider/router.go):
    //   NewRouter(providers, preferences, ratio, cooldown, stateFn, circuitBreaker)
    return provider.NewRouter(providers, preferences, ratio, cooldown, p.stateFn, p.circuitBreaker)
}
```

**Step 2b: Add tests for `buildRouter` construction**

```go
func TestBuildRouter_ReturnsConfiguredRouter(t *testing.T) {
    p := &RealStageProvider{
        claudeProvider: &mockProvider{name: "claude"},
        codexProvider:  &mockProvider{name: "codex"},
        stateFn:        nil,
        circuitBreaker: nil,
    }
    policy := execpolicy.DefaultPolicy()
    policy.Routing.Ratio = map[string]int{"claude": 70, "codex": 30}
    router := p.buildRouter(policy)
    // Router should be non-nil and usable
    prov, _ := router.Select("plan", "high")
    if prov == nil {
        t.Fatal("expected router to return a provider")
    }
}

func TestBuildStages_SelectsDifferentProvidersPerPhase(t *testing.T) {
    p := &RealStageProvider{
        claudeProvider: &mockProvider{name: "claude"},
        codexProvider:  &mockProvider{name: "codex"},
    }
    policy := execpolicy.DefaultPolicy()
    policy.Routing.Preferences = map[string]string{
        "plan": "claude", "execute": "codex", "review": "any", "validate": "any",
    }
    policy.Routing.Ratio = map[string]int{"claude": 50, "codex": 50}
    // Build stages and verify different providers are selected per phase.
    // FallbackAdapter.Provider() triggers lazy Select, which respects preferences.
    t.Skip("TODO: complete assertions after BuildStages returns inspectable adapters — call Provider() on the FallbackAdapters and check names match phase preferences")
}

func TestBuildStages_NilCodexProvider_SingleProviderMode(t *testing.T) {
    p := &RealStageProvider{
        claudeProvider: &mockProvider{name: "claude"},
        codexProvider:  nil, // single-provider mode
    }
    policy := execpolicy.DefaultPolicy()
    policy.Routing.Ratio = map[string]int{"claude": 100}
    router := p.buildRouter(policy)
    prov, _ := router.Select("plan", "high")
    if prov == nil {
        t.Fatal("expected claude provider in single-provider mode")
    }
    if prov.Name() != "claude" {
        t.Errorf("expected claude, got %q", prov.Name())
    }
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

> **Cross-plan dependency:** This task requires the following 0002c tasks to be complete before work can begin:
> - 0002c contract test suites must exist (`RunPlanAgentContract`, `RunReviewAgentContract`, `RunAcceptAgentContract`, `RunTaskRunnerContract`)
> - 0002c `LLMAdapter` and `NewProviderAware` constructor must be implemented
> - 0002c per-domain provider agents (`ProviderPlanAgent`, `ProviderReviewAgent`, `ProviderAcceptAgent`, `ProviderTaskRunner`) must be implemented
>
> The Codex provider itself (`provider.NewCodexProvider`) already exists from 0001.

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
- Modify: `cmd/gromit-next/stage_provider_test.go` (for BuildStages wiring test — must be in package main)
- Modify: `internal/next/specloop/integration_test.go` (for FallbackAdapter-through-Router test — uses only importable packages)

**Step 1a: Add BuildStages wiring test to `cmd/gromit-next/stage_provider_test.go`**

This test must live in `cmd/gromit-next/` (package main) because it references
`NewRealStageProvider` and `RealStageProviderConfig`, which are unexportable from
package main. Placing it in `internal/next/specloop/` would fail to compile.

```go
func TestIntegration_BuildStages_FallbackAdapter_RouterWiring(t *testing.T) {
    // Build a real RealStageProvider with mock providers
    claudeProv := &mockProvider{name: "claude", runResult: &provider.Result{Output: "ok"}}
    codexProv := &mockProviderWithUsageLimit{
        name: "codex", isUsageLimit: false,
        runResult: &provider.Result{Output: "codex-ok"}, runErr: nil,
    }
    sp := NewRealStageProvider(RealStageProviderConfig{
        ClaudeProvider: claudeProv,
        CodexProvider:  codexProv,
    })

    policy := execpolicy.DefaultPolicy()
    policy.Routing.Preferences = map[string]string{
        "plan": "claude", "execute": "codex", "review": "any",
    }
    policy.Routing.Ratio = map[string]int{"claude": 50, "codex": 50}

    budget := specloop.NewBudget(execpolicy.DefaultPolicy().Budgets)
    rs := &runstore.RunState{}
    stages, err := sp.BuildStages(policy, rs, budget)
    if err != nil {
        t.Fatalf("BuildStages failed: %v", err)
    }

    // Verify stages are wired correctly by invoking them and checking which
    // mock provider was called (behavioral verification). The Stage interface
    // does not expose Invoker() directly, so we verify through invocation.
    // TODO: verify wiring through stage invocation — invoke each stage with a
    // test prompt and assert the correct mock provider's Run method was called.
    // For now, verify stages were created successfully.
    if len(stages) == 0 {
        t.Fatal("expected at least one stage from BuildStages")
    }
    t.Skip("TODO: verify wiring through stage invocation — Stage interface does not expose Invoker() directly; invoke stages and check which mock provider was called")
}
```

**Step 1b: Add FallbackAdapter-through-Router test to `internal/next/specloop/integration_test.go`**

This test uses only importable packages (`provider`, `llmadapter`) and validates the
fallback path through a real Router without depending on package main types.

```go
func TestIntegration_FallbackAdapter_UsageLimitFallback_ThroughRouter(t *testing.T) {
    // End-to-end: primary hits usage limit, Router routes to fallback
    claudeProv := &mockProviderWithUsageLimit{
        name: "claude", isUsageLimit: true,
        runResult: &provider.Result{Output: "", ExitCode: 2},
        runErr: errors.New("usage limit"),
    }
    codexProv := &mockProvider{name: "codex", runResult: &provider.Result{Output: "codex-ok"}}

    // Build a real Router (not a mock) to test full integration path
    router := provider.NewRouter(
        map[string]provider.Provider{"claude": claudeProv, "codex": codexProv},
        map[string]string{"build": "any"},
        map[string]int{"claude": 50, "codex": 50},
        0, nil, nil,
    )
    fa := llmadapter.NewFallbackAdapter(router, "build", llmadapter.Config{}, "medium")
    result, err := fa.Invoke(context.Background(), "prompt")
    if err != nil {
        t.Fatalf("expected fallback to succeed, got error: %v", err)
    }
    if result.Output != "codex-ok" {
        t.Errorf("expected codex fallback output, got %q", result.Output)
    }
}
```

**Step 2: Add routing-specific integration scaffolds (skipped)**

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

**Step 3: Commit**

```bash
git add cmd/gromit-next/stage_provider_test.go internal/next/specloop/integration_test.go
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

No commit needed — this task only runs verification.
