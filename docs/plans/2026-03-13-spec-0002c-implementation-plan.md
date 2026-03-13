# Spec 0002c Implementation Plan — Provider-Agnostic Adapter Layer

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace noop stage implementations with real LLM-backed adapters using a shared `LLMAdapter` base, proving the pipeline end-to-end with Claude.

**Architecture:** A shared `LLMAdapter` in `internal/next/llmadapter/` wraps `provider.Provider` with timeout and cost tracking. Per-domain thin adapters in their respective packages compose `LLMAdapter` and parse output into domain types. `RealStageProvider` wires real adapters instead of noops.

**Tech Stack:** Go, `provider.Provider` interface, `internal/claude`, existing stage interfaces from `specloop/stages/`

---

### Task 1: Create `llmadapter` package — failing tests

**Files:**
- Create: `internal/next/llmadapter/adapter_test.go`

**Step 1: Write the failing tests**

```go
package llmadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/provider"
)

// mockProvider satisfies provider.Provider for unit tests.
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
func (m *mockProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	m.calls++
	m.lastTier = tier
	return m.runResult, m.runErr
}
func (m *mockProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return m.runResult, m.runErr
}
func (m *mockProvider) IsUsageLimitError(result *provider.Result, err error) bool { return false }
func (m *mockProvider) IsValidationPassed(result *provider.Result) bool           { return result != nil && result.Success }
func (m *mockProvider) IsScopeTooLarge(result *provider.Result) (bool, string)    { return false, "" }

func TestInvoke_DelegatesToProviderRun(t *testing.T) {
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{Output: "hello", CostUSD: 0.05, InputTokens: 100, OutputTokens: 50},
	}
	adapter := New(mp, Config{Tier: "medium"})
	result, err := adapter.Invoke(context.Background(), "do something")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello" {
		t.Errorf("expected output 'hello', got %q", result.Output)
	}
	if mp.calls != 1 {
		t.Errorf("expected 1 call, got %d", mp.calls)
	}
	if mp.lastTier != "medium" {
		t.Errorf("expected tier 'medium', got %q", mp.lastTier)
	}
}

func TestInvoke_PropagatesError(t *testing.T) {
	mp := &mockProvider{
		name:   "test",
		runErr: errors.New("api failure"),
	}
	adapter := New(mp, Config{Tier: "high"})
	_, err := adapter.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "api failure" {
		t.Errorf("expected 'api failure', got %q", err.Error())
	}
}

func TestInvoke_CallsOnCost(t *testing.T) {
	var captured float64
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{CostUSD: 0.12},
	}
	adapter := New(mp, Config{
		Tier:   "low",
		OnCost: func(c float64) { captured = c },
	})
	_, err := adapter.Invoke(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured != 0.12 {
		t.Errorf("expected cost 0.12, got %f", captured)
	}
}

func TestInvoke_OnCostNotCalledOnZero(t *testing.T) {
	called := false
	mp := &mockProvider{
		name:      "test",
		runResult: &provider.Result{CostUSD: 0},
	}
	adapter := New(mp, Config{
		Tier:   "low",
		OnCost: func(c float64) { called = true },
	})
	_, _ = adapter.Invoke(context.Background(), "prompt")
	if called {
		t.Error("OnCost should not be called for zero cost")
	}
}

func TestInvoke_RespectsTimeout(t *testing.T) {
	mp := &mockProvider{
		name: "test",
		runResult: &provider.Result{},
	}
	// Override Run to block until context is done
	slowProvider := &slowMockProvider{delay: 5 * time.Second, result: &provider.Result{}}
	adapter := New(slowProvider, Config{
		Tier:    "low",
		Timeout: 50 * time.Millisecond,
	})
	_, err := adapter.Invoke(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestProviderName(t *testing.T) {
	mp := &mockProvider{name: "claude"}
	adapter := New(mp, Config{Tier: "high"})
	if adapter.ProviderName() != "claude" {
		t.Errorf("expected 'claude', got %q", adapter.ProviderName())
	}
}

func TestTier(t *testing.T) {
	mp := &mockProvider{name: "test"}
	adapter := New(mp, Config{Tier: "xhigh"})
	if adapter.Tier() != "xhigh" {
		t.Errorf("expected 'xhigh', got %q", adapter.Tier())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/llmadapter/ -v -count=1`
Expected: FAIL — package does not exist

**Step 3: Commit red tests**

```bash
git add internal/next/llmadapter/adapter_test.go
git commit -m "red: llmadapter unit tests for Invoke, timeout, cost callback"
```

---

### Task 2: Implement `llmadapter.LLMAdapter`

**Files:**
- Create: `internal/next/llmadapter/adapter.go`

**Step 1: Write minimal implementation**

```go
package llmadapter

import (
	"context"
	"io"
	"time"

	"github.com/danabrams/gromit/internal/provider"
)

// Config configures an LLMAdapter instance.
type Config struct {
	Tier    string
	Timeout time.Duration
	OnCost  func(cost float64)
}

// LLMAdapter wraps a provider.Provider with timeout enforcement and cost tracking.
// It is the shared base for all per-domain adapters.
type LLMAdapter struct {
	provider provider.Provider
	cfg      Config
}

// New creates an LLMAdapter.
func New(p provider.Provider, cfg Config) *LLMAdapter {
	return &LLMAdapter{provider: p, cfg: cfg}
}

// Invoke calls provider.Run with the configured tier.
// If a timeout is configured, it is applied to the context.
// If OnCost is set and the result has non-zero cost, the callback is called.
func (a *LLMAdapter) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	if a.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.Timeout)
		defer cancel()
	}

	result, err := a.provider.Run(ctx, prompt, a.cfg.Tier)
	if err != nil {
		return nil, err
	}

	if a.cfg.OnCost != nil && result.CostUSD > 0 {
		a.cfg.OnCost(result.CostUSD)
	}

	return result, nil
}

// InvokeStream calls provider.StreamRun with the configured tier.
func (a *LLMAdapter) InvokeStream(ctx context.Context, prompt string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if a.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.Timeout)
		defer cancel()
	}

	result, err := a.provider.StreamRun(ctx, prompt, a.cfg.Tier, w, handler, onToolCall)
	if err != nil {
		return nil, err
	}

	if a.cfg.OnCost != nil && result.CostUSD > 0 {
		a.cfg.OnCost(result.CostUSD)
	}

	return result, nil
}

// ProviderName returns the name of the underlying provider.
func (a *LLMAdapter) ProviderName() string {
	return a.provider.Name()
}

// Tier returns the configured tier.
func (a *LLMAdapter) Tier() string {
	return a.cfg.Tier
}
```

Also add the `slowMockProvider` to the test file for the timeout test:

```go
// slowMockProvider blocks for a configurable delay.
type slowMockProvider struct {
	mockProvider
	delay  time.Duration
	result *provider.Result
}

func (m *slowMockProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	select {
	case <-time.After(m.delay):
		return m.result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/llmadapter/ -v -count=1`
Expected: PASS — all tests green

**Step 3: Commit**

```bash
git add internal/next/llmadapter/
git commit -m "green: implement LLMAdapter with invoke, timeout, and cost tracking"
```

---

### Task 3: `ProviderPlanAgent` — failing tests

**Files:**
- Create: `internal/next/planner/provider_agent_test.go`

**Step 1: Write the failing tests**

```go
package planner

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestProviderPlanAgent_Invoke_ReturnsAgentResult(t *testing.T) {
	result := &provider.Result{
		Output:      `{"spec_id":"test","cycle":1,"kind":"original","tasks":[{"task_id":"t-001","objective":"do thing","expected_touched_area":["pkg/"],"proof_checks":["go test"]}]}`,
		CostUSD:     0.05,
		InputTokens: 200,
		OutputTokens: 100,
		Model:       "sonnet",
	}
	agent := NewProviderPlanAgent(newMockLLMAdapter(result, nil))
	ar, err := agent.Invoke(context.Background(), "plan prompt", "medium")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ar.Output != result.Output {
		t.Errorf("output mismatch")
	}
	if ar.TokensIn != 200 {
		t.Errorf("expected TokensIn 200, got %d", ar.TokensIn)
	}
	if ar.TokensOut != 100 {
		t.Errorf("expected TokensOut 100, got %d", ar.TokensOut)
	}
	if ar.Cost != 0.05 {
		t.Errorf("expected Cost 0.05, got %f", ar.Cost)
	}
	if ar.Model != "sonnet" {
		t.Errorf("expected Model 'sonnet', got %q", ar.Model)
	}
}

func TestProviderPlanAgent_Invoke_PropagatesError(t *testing.T) {
	agent := NewProviderPlanAgent(newMockLLMAdapter(nil, errors.New("provider down")))
	_, err := agent.Invoke(context.Background(), "prompt", "high")
	if err == nil {
		t.Fatal("expected error")
	}
}
```

Note: `newMockLLMAdapter` is a test helper — Task 3 step 1 also needs a small mock for `LLMAdapter`. Since `LLMAdapter` is a concrete struct (not an interface), the `ProviderPlanAgent` should depend on an interface:

```go
// mockLLMInvoker satisfies the Invoker interface for tests.
type mockLLMInvoker struct {
	result *provider.Result
	err    error
}

func (m *mockLLMInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	return m.result, m.err
}

func newMockLLMAdapter(result *provider.Result, err error) *mockLLMInvoker {
	return &mockLLMInvoker{result: result, err: err}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/planner/ -run TestProviderPlanAgent -v -count=1`
Expected: FAIL — `NewProviderPlanAgent` undefined

**Step 3: Commit**

```bash
git add internal/next/planner/provider_agent_test.go
git commit -m "red: ProviderPlanAgent tests for invoke and error propagation"
```

---

### Task 4: Implement `ProviderPlanAgent` + `Invoker` interface

**Files:**
- Create: `internal/next/llmadapter/invoker.go`
- Create: `internal/next/planner/provider_agent.go`

**Step 1: Define Invoker interface in llmadapter**

```go
package llmadapter

import (
	"context"

	"github.com/danabrams/gromit/internal/provider"
)

// Invoker is the interface domain adapters depend on.
// LLMAdapter satisfies this, and tests can substitute mocks.
type Invoker interface {
	Invoke(ctx context.Context, prompt string) (*provider.Result, error)
}
```

Verify `LLMAdapter` satisfies `Invoker` — add compile-time check to `adapter.go`:

```go
var _ Invoker = (*LLMAdapter)(nil)
```

**Step 2: Implement ProviderPlanAgent**

```go
package planner

import (
	"context"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// ProviderPlanAgent adapts an LLM invoker to satisfy the planner.Agent interface.
type ProviderPlanAgent struct {
	llm llmadapter.Invoker
}

// NewProviderPlanAgent creates a ProviderPlanAgent.
func NewProviderPlanAgent(llm llmadapter.Invoker) *ProviderPlanAgent {
	return &ProviderPlanAgent{llm: llm}
}

// Invoke delegates to the LLM adapter and maps the result to AgentResult.
// The tier parameter is ignored — tier is configured on the LLMAdapter.
func (a *ProviderPlanAgent) Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error) {
	result, err := a.llm.Invoke(ctx, prompt)
	if err != nil {
		return AgentResult{}, err
	}
	return AgentResult{
		Output:    result.Output,
		TokensIn:  result.InputTokens,
		TokensOut: result.OutputTokens,
		Cost:      result.CostUSD,
		Model:     result.Model,
		Duration:  result.Duration.Milliseconds(),
	}, nil
}
```

Add compile-time check:

```go
var _ Agent = (*ProviderPlanAgent)(nil)
```

**Step 3: Run tests to verify they pass**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/planner/ -run TestProviderPlanAgent -v -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/next/llmadapter/invoker.go internal/next/planner/provider_agent.go internal/next/planner/provider_agent_test.go internal/next/llmadapter/adapter.go
git commit -m "green: ProviderPlanAgent + Invoker interface"
```

---

### Task 5: `ProviderReviewAgent` — failing tests

**Files:**
- Create: `internal/next/review/provider_agent_test.go`

**Step 1: Write the failing tests**

The `ReviewAgent.ReviewFacet` returns `([]Finding, error)`. The adapter must parse JSON output from the LLM into findings. The `Finding` type has custom `UnmarshalJSON` that validates required fields and returns `ParseError` on missing fields.

```go
package review

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestProviderReviewAgent_ValidFindings(t *testing.T) {
	output := `[{"severity":"warning","file":"main.go","line":10,"description":"unused import"}]`
	agent := NewProviderReviewAgent(newMockInvoker(&provider.Result{Output: output}, nil))
	findings, err := agent.ReviewFacet(context.Background(), "code_quality", "review prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].File != "main.go" {
		t.Errorf("expected file 'main.go', got %q", findings[0].File)
	}
	if findings[0].Severity != SeverityWarning {
		t.Errorf("expected severity warning, got %v", findings[0].Severity)
	}
}

func TestProviderReviewAgent_EmptyArray(t *testing.T) {
	agent := NewProviderReviewAgent(newMockInvoker(&provider.Result{Output: "[]"}, nil))
	findings, err := agent.ReviewFacet(context.Background(), "spec_alignment", "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestProviderReviewAgent_MalformedJSON_ReturnsParseError(t *testing.T) {
	agent := NewProviderReviewAgent(newMockInvoker(&provider.Result{Output: "not json at all"}, nil))
	_, err := agent.ReviewFacet(context.Background(), "code_quality", "prompt")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected ParseError, got %T: %v", err, err)
	}
}

func TestProviderReviewAgent_MissingRequiredField_ReturnsParseError(t *testing.T) {
	// Missing 'file' field
	output := `[{"severity":"error","description":"bad thing","line":1}]`
	agent := NewProviderReviewAgent(newMockInvoker(&provider.Result{Output: output}, nil))
	_, err := agent.ReviewFacet(context.Background(), "logic_gaps", "prompt")
	if err == nil {
		t.Fatal("expected error for missing field")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected ParseError, got %T: %v", err, err)
	}
}

func TestProviderReviewAgent_PropagatesProviderError(t *testing.T) {
	agent := NewProviderReviewAgent(newMockInvoker(nil, errors.New("timeout")))
	_, err := agent.ReviewFacet(context.Background(), "code_quality", "prompt")
	if err == nil {
		t.Fatal("expected error")
	}
	// Provider errors should NOT be ParseErrors (not retryable)
	var pe *ParseError
	if errors.As(err, &pe) {
		t.Error("provider errors should not be ParseError")
	}
}

func TestProviderReviewAgent_MarkdownFencedJSON(t *testing.T) {
	output := "Here are the findings:\n```json\n[{\"severity\":\"info\",\"file\":\"a.go\",\"line\":1,\"description\":\"note\"}]\n```\n"
	agent := NewProviderReviewAgent(newMockInvoker(&provider.Result{Output: output}, nil))
	findings, err := agent.ReviewFacet(context.Background(), "code_quality", "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

// mockInvoker satisfies llmadapter.Invoker for tests.
type mockInvoker struct {
	result *provider.Result
	err    error
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	return m.result, m.err
}

func newMockInvoker(result *provider.Result, err error) *mockInvoker {
	return &mockInvoker{result: result, err: err}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/review/ -run TestProviderReviewAgent -v -count=1`
Expected: FAIL — `NewProviderReviewAgent` undefined

**Step 3: Commit**

```bash
git add internal/next/review/provider_agent_test.go
git commit -m "red: ProviderReviewAgent tests for JSON parsing, ParseError, and error propagation"
```

---

### Task 6: Implement `ProviderReviewAgent`

**Files:**
- Create: `internal/next/review/provider_agent.go`

**Step 1: Write implementation**

```go
package review

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// ProviderReviewAgent adapts an LLM invoker to satisfy ReviewAgent.
// It invokes the LLM and parses the JSON output into findings.
type ProviderReviewAgent struct {
	llm llmadapter.Invoker
}

// NewProviderReviewAgent creates a ProviderReviewAgent.
func NewProviderReviewAgent(llm llmadapter.Invoker) *ProviderReviewAgent {
	return &ProviderReviewAgent{llm: llm}
}

// ReviewFacet invokes the LLM with the prompt and parses findings from JSON output.
// Returns ParseError on malformed JSON or missing required fields (retryable by Runner).
func (a *ProviderReviewAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
	result, err := a.llm.Invoke(ctx, prompt)
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSON(result.Output)
	var findings []Finding
	if err := json.Unmarshal([]byte(jsonStr), &findings); err != nil {
		return nil, &ParseError{Msg: "failed to parse findings JSON: " + err.Error()}
	}
	return findings, nil
}

// extractJSON extracts JSON from raw LLM output.
// Handles both bare JSON and markdown code fences (```json ... ```).
func extractJSON(output string) string {
	// Try to find ```json fenced block
	if idx := strings.Index(output, "```json"); idx >= 0 {
		start := idx + len("```json")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			return strings.TrimSpace(output[start : start+end])
		}
	}
	// Try bare ``` fenced block
	if idx := strings.Index(output, "```"); idx >= 0 {
		start := idx + len("```")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			return strings.TrimSpace(output[start : start+end])
		}
	}
	return strings.TrimSpace(output)
}

// Compile-time check.
var _ ReviewAgent = (*ProviderReviewAgent)(nil)
```

**Step 2: Run tests to verify they pass**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/review/ -run TestProviderReviewAgent -v -count=1`
Expected: PASS

**Step 3: Run all review tests to verify no regressions**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/review/ -v -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/next/review/provider_agent.go
git commit -m "green: ProviderReviewAgent parses LLM output into findings"
```

---

### Task 7: `ProviderAcceptAgent` — failing tests

**Files:**
- Create: `internal/next/acceptor/provider_agent_test.go`

**Step 1: Write the failing tests**

```go
package acceptor

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestProviderAcceptAgent_ValidResult(t *testing.T) {
	output := `{"criterion":"feature works","status":"pass","rationale":"tests pass","evidence_refs":["test_output.log"]}`
	agent := NewProviderAcceptAgent(newMockInvoker(&provider.Result{Output: output}, nil))
	cr, err := agent.EvaluateCriterion(context.Background(), "evaluate prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cr.Status != StatusPass {
		t.Errorf("expected status 'pass', got %q", cr.Status)
	}
	if cr.Rationale != "tests pass" {
		t.Errorf("unexpected rationale: %q", cr.Rationale)
	}
}

func TestProviderAcceptAgent_FailResult(t *testing.T) {
	output := `{"status":"fail","rationale":"missing edge case"}`
	agent := NewProviderAcceptAgent(newMockInvoker(&provider.Result{Output: output}, nil))
	cr, err := agent.EvaluateCriterion(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cr.Status != StatusFail {
		t.Errorf("expected 'fail', got %q", cr.Status)
	}
}

func TestProviderAcceptAgent_MalformedJSON(t *testing.T) {
	agent := NewProviderAcceptAgent(newMockInvoker(&provider.Result{Output: "not json"}, nil))
	_, err := agent.EvaluateCriterion(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestProviderAcceptAgent_MarkdownFencedJSON(t *testing.T) {
	output := "```json\n{\"status\":\"pass\",\"rationale\":\"ok\"}\n```"
	agent := NewProviderAcceptAgent(newMockInvoker(&provider.Result{Output: output}, nil))
	cr, err := agent.EvaluateCriterion(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cr.Status != StatusPass {
		t.Errorf("expected 'pass', got %q", cr.Status)
	}
}

func TestProviderAcceptAgent_PropagatesProviderError(t *testing.T) {
	agent := NewProviderAcceptAgent(newMockInvoker(nil, errors.New("down")))
	_, err := agent.EvaluateCriterion(context.Background(), "prompt")
	if err == nil {
		t.Fatal("expected error")
	}
}

type mockInvoker struct {
	result *provider.Result
	err    error
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	return m.result, m.err
}

func newMockInvoker(result *provider.Result, err error) *mockInvoker {
	return &mockInvoker{result: result, err: err}
}
```

**Step 2: Run and verify failure**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/acceptor/ -run TestProviderAcceptAgent -v -count=1`
Expected: FAIL

**Step 3: Commit**

```bash
git add internal/next/acceptor/provider_agent_test.go
git commit -m "red: ProviderAcceptAgent tests for JSON parsing and error handling"
```

---

### Task 8: Implement `ProviderAcceptAgent`

**Files:**
- Create: `internal/next/acceptor/provider_agent.go`

**Step 1: Write implementation**

```go
package acceptor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// ProviderAcceptAgent adapts an LLM invoker to satisfy AcceptAgent.
type ProviderAcceptAgent struct {
	llm llmadapter.Invoker
}

// NewProviderAcceptAgent creates a ProviderAcceptAgent.
func NewProviderAcceptAgent(llm llmadapter.Invoker) *ProviderAcceptAgent {
	return &ProviderAcceptAgent{llm: llm}
}

// EvaluateCriterion invokes the LLM and parses the JSON result into CriterionResult.
func (a *ProviderAcceptAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	result, err := a.llm.Invoke(ctx, prompt)
	if err != nil {
		return CriterionResult{}, err
	}

	jsonStr := extractJSON(result.Output)
	var cr CriterionResult
	if err := json.Unmarshal([]byte(jsonStr), &cr); err != nil {
		return CriterionResult{}, fmt.Errorf("failed to parse criterion result: %w", err)
	}
	cr.NormalizeNilFields()
	return cr, nil
}

// extractJSON extracts JSON from raw LLM output, handling markdown fences.
func extractJSON(output string) string {
	if idx := strings.Index(output, "```json"); idx >= 0 {
		start := idx + len("```json")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			return strings.TrimSpace(output[start : start+end])
		}
	}
	if idx := strings.Index(output, "```"); idx >= 0 {
		start := idx + len("```")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			return strings.TrimSpace(output[start : start+end])
		}
	}
	return strings.TrimSpace(output)
}

var _ AcceptAgent = (*ProviderAcceptAgent)(nil)
```

**Step 2: Run tests**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/acceptor/ -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/next/acceptor/provider_agent.go
git commit -m "green: ProviderAcceptAgent parses LLM output into CriterionResult"
```

---

### Task 9: `ProviderTaskRunner` — failing tests

**Files:**
- Create: `internal/next/specloop/provider_taskrunner_test.go`

**Step 1: Write the failing tests**

The TaskRunner must render a prompt from the task, invoke the LLM, and map the result to TaskResult. The prompt rendering is internal — the adapter decides what prompt to send. Result parsing: the LLM is an agentic coding tool that executes the task. The result captures what happened (tokens, cost, duration, files changed). The adapter doesn't parse structured JSON from the LLM — it maps `provider.Result` metadata to `TaskResult`.

```go
package specloop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/provider"
)

func TestProviderTaskRunner_RunTask(t *testing.T) {
	result := &provider.Result{
		Success:      true,
		Output:       "implemented feature X",
		InputTokens:  500,
		OutputTokens: 200,
		CostUSD:      0.03,
		Duration:     30 * time.Second,
		Model:        "sonnet",
	}
	runner := NewProviderTaskRunner(newMockInvoker(result, nil))
	task := runstore.Task{
		TaskID:    "t-001",
		Objective: "implement feature X",
	}
	tr, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Status != "done" {
		t.Errorf("expected status 'done', got %q", tr.Status)
	}
	if tr.TokensUsed != 700 {
		t.Errorf("expected 700 tokens, got %d", tr.TokensUsed)
	}
	if tr.Cost != 0.03 {
		t.Errorf("expected cost 0.03, got %f", tr.Cost)
	}
	if tr.Model != "sonnet" {
		t.Errorf("expected model 'sonnet', got %q", tr.Model)
	}
}

func TestProviderTaskRunner_RunTask_ProviderFailure(t *testing.T) {
	result := &provider.Result{Success: false, Output: "error occurred"}
	runner := NewProviderTaskRunner(newMockInvoker(result, nil))
	task := runstore.Task{TaskID: "t-002", Objective: "do thing"}
	tr, err := runner.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", tr.Status)
	}
}

func TestProviderTaskRunner_RunTask_Error(t *testing.T) {
	runner := NewProviderTaskRunner(newMockInvoker(nil, errors.New("timeout")))
	task := runstore.Task{TaskID: "t-003", Objective: "do thing"}
	_, err := runner.RunTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProviderTaskRunner_RepairTask(t *testing.T) {
	result := &provider.Result{
		Success:      true,
		Output:       "fixed the issue",
		InputTokens:  300,
		OutputTokens: 150,
		CostUSD:      0.02,
		Duration:     15 * time.Second,
	}
	invoker := &capturingInvoker{result: result}
	runner := NewProviderTaskRunner(invoker)
	task := runstore.Task{TaskID: "t-001", Objective: "implement X"}
	failures := []string{"test_foo failed: expected 1 got 2"}
	tr, err := runner.RepairTask(context.Background(), task, failures)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Status != "done" {
		t.Errorf("expected 'done', got %q", tr.Status)
	}
	// Verify repair prompt includes failure context
	if invoker.lastPrompt == "" {
		t.Fatal("expected prompt to be captured")
	}
	if !containsSubstring(invoker.lastPrompt, "test_foo failed") {
		t.Error("repair prompt should include failure details")
	}
}

func TestProviderTaskRunner_RunTask_PromptContainsObjective(t *testing.T) {
	invoker := &capturingInvoker{result: &provider.Result{Success: true}}
	runner := NewProviderTaskRunner(invoker)
	task := runstore.Task{TaskID: "t-001", Objective: "add user validation"}
	_, _ = runner.RunTask(context.Background(), task)
	if !containsSubstring(invoker.lastPrompt, "add user validation") {
		t.Error("task prompt should include the task objective")
	}
}

type mockInvoker struct {
	result *provider.Result
	err    error
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	return m.result, m.err
}

func newMockInvoker(result *provider.Result, err error) *mockInvoker {
	return &mockInvoker{result: result, err: err}
}

type capturingInvoker struct {
	result     *provider.Result
	lastPrompt string
}

func (c *capturingInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	c.lastPrompt = prompt
	return c.result, nil
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && strings.Contains(s, sub))
}
```

Add `"strings"` to imports.

**Step 2: Run and verify failure**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/specloop/ -run TestProviderTaskRunner -v -count=1`
Expected: FAIL

**Step 3: Commit**

```bash
git add internal/next/specloop/provider_taskrunner_test.go
git commit -m "red: ProviderTaskRunner tests for RunTask, RepairTask, and prompt rendering"
```

---

### Task 10: Implement `ProviderTaskRunner`

**Files:**
- Create: `internal/next/specloop/provider_taskrunner.go`

**Step 1: Write implementation**

```go
package specloop

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/next/llmadapter"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// ProviderTaskRunner adapts an LLM invoker to satisfy TaskRunner.
type ProviderTaskRunner struct {
	llm llmadapter.Invoker
}

// NewProviderTaskRunner creates a ProviderTaskRunner.
func NewProviderTaskRunner(llm llmadapter.Invoker) *ProviderTaskRunner {
	return &ProviderTaskRunner{llm: llm}
}

// RunTask renders a prompt from the task and invokes the LLM.
func (r *ProviderTaskRunner) RunTask(ctx context.Context, task runstore.Task) (TaskResult, error) {
	prompt := renderTaskPrompt(task)
	result, err := r.llm.Invoke(ctx, prompt)
	if err != nil {
		return TaskResult{}, err
	}
	return mapResult(result), nil
}

// RepairTask renders a repair prompt including failure context and invokes the LLM.
func (r *ProviderTaskRunner) RepairTask(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error) {
	prompt := renderRepairPrompt(task, failures)
	result, err := r.llm.Invoke(ctx, prompt)
	if err != nil {
		return TaskResult{}, err
	}
	return mapResult(result), nil
}

func renderTaskPrompt(task runstore.Task) string {
	var b strings.Builder
	b.WriteString("You are a coding agent. Execute the following task.\n\n")
	b.WriteString(fmt.Sprintf("## Task: %s\n\n", task.TaskID))
	b.WriteString(fmt.Sprintf("**Objective:** %s\n\n", task.Objective))
	if len(task.ExpectedTouchedArea) > 0 {
		b.WriteString("**Expected files:** ")
		b.WriteString(strings.Join(task.ExpectedTouchedArea, ", "))
		b.WriteString("\n\n")
	}
	if len(task.ProofChecks) > 0 {
		b.WriteString("**Proof checks:** ")
		b.WriteString(strings.Join(task.ProofChecks, ", "))
		b.WriteString("\n\n")
	}
	b.WriteString("Implement the objective. Run proof checks to verify your work.\n")
	return b.String()
}

func renderRepairPrompt(task runstore.Task, failures []string) string {
	var b strings.Builder
	b.WriteString("You are a coding agent. A previous attempt at this task failed. Fix the failures.\n\n")
	b.WriteString(fmt.Sprintf("## Task: %s\n\n", task.TaskID))
	b.WriteString(fmt.Sprintf("**Objective:** %s\n\n", task.Objective))
	b.WriteString("## Failures from previous attempt\n")
	for _, f := range failures {
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteString("\n")
	}
	b.WriteString("\nFix these failures. Run proof checks to verify your work.\n")
	return b.String()
}

func mapResult(result *provider.Result) TaskResult {
	status := "done"
	if !result.Success {
		status = "failed"
	}
	return TaskResult{
		Status:     status,
		TokensUsed: result.InputTokens + result.OutputTokens,
		Cost:       result.CostUSD,
		DurationMs: result.Duration.Milliseconds(),
		Model:      result.Model,
	}
}

var _ TaskRunner = (*ProviderTaskRunner)(nil)
```

Add the provider import: `"github.com/danabrams/gromit/internal/provider"`

**Step 2: Run tests**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/specloop/ -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/next/specloop/provider_taskrunner.go
git commit -m "green: ProviderTaskRunner with task/repair prompt rendering"
```

---

### Task 11: `ShellValidator` — failing tests

**Files:**
- Create: `internal/next/validator/shell_validator_test.go`

**Step 1: Write the failing tests**

```go
package validator

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestShellValidator_AllChecksPass(t *testing.T) {
	v := NewShellValidator(nil) // no LLM needed when all pass
	checks := []Check{
		{Name: "echo", Command: "echo ok", Type: "test"},
	}
	result, err := v.RunFinal(context.Background(), checks, nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected pass when all checks succeed")
	}
}

func TestShellValidator_FailingCheck_InvokesLLM(t *testing.T) {
	invoker := &mockInvoker{
		result: &provider.Result{Output: "The test failed because X was nil"},
	}
	v := NewShellValidator(invoker)
	checks := []Check{
		{Name: "fail", Command: "exit 1", Type: "test"},
	}
	result, err := v.RunFinal(context.Background(), checks, nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("expected fail when a check fails")
	}
	if invoker.calls == 0 {
		t.Error("expected LLM to be invoked for diagnosis")
	}
}

func TestShellValidator_NoLLM_FailingCheck_StillReturnsFailure(t *testing.T) {
	v := NewShellValidator(nil) // no LLM configured
	checks := []Check{
		{Name: "fail", Command: "exit 1", Type: "test"},
	}
	result, err := v.RunFinal(context.Background(), checks, nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("expected fail")
	}
}

func TestShellValidator_MultiplChecks_MixedResults(t *testing.T) {
	v := NewShellValidator(nil)
	checks := []Check{
		{Name: "pass", Command: "echo ok", Type: "test"},
		{Name: "fail", Command: "exit 1", Type: "lint"},
	}
	result, err := v.RunFinal(context.Background(), checks, nil, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("expected fail with mixed results")
	}
	passCount := 0
	failCount := 0
	for _, r := range result.AlwaysRun.Results {
		if r.Pass {
			passCount++
		} else {
			failCount++
		}
	}
	if passCount != 1 || failCount != 1 {
		t.Errorf("expected 1 pass and 1 fail, got %d pass and %d fail", passCount, failCount)
	}
}

type mockInvoker struct {
	result *provider.Result
	err    error
	calls  int
}

func (m *mockInvoker) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	m.calls++
	return m.result, m.err
}
```

**Step 2: Run and verify failure**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/validator/ -run TestShellValidator -v -count=1`
Expected: FAIL

**Step 3: Commit**

```bash
git add internal/next/validator/shell_validator_test.go
git commit -m "red: ShellValidator tests for shell-first validation with LLM diagnosis"
```

---

### Task 12: Implement `ShellValidator`

**Files:**
- Create: `internal/next/validator/shell_validator.go`

**Step 1: Write implementation**

```go
package validator

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/llmadapter"
)

// ShellValidator runs checks via shell and uses an LLM for failure diagnosis.
type ShellValidator struct {
	llm llmadapter.Invoker // nil if no LLM diagnosis desired
}

// NewShellValidator creates a ShellValidator. llm may be nil.
func NewShellValidator(llm llmadapter.Invoker) *ShellValidator {
	return &ShellValidator{llm: llm}
}

// RunFinal executes all checks via shell. On failure, optionally invokes LLM for diagnosis.
func (v *ShellValidator) RunFinal(ctx context.Context, alwaysRun []Check, projectChecks []Check, workDir string) (FinalResult, error) {
	alwaysResults := v.runChecks(ctx, alwaysRun, workDir)
	projectResults := v.runChecks(ctx, projectChecks, workDir)

	allPass := true
	var failures []CheckResult
	for _, r := range alwaysResults.Results {
		if !r.Pass {
			allPass = false
			failures = append(failures, r)
		}
	}
	for _, r := range projectResults.Results {
		if !r.Pass {
			allPass = false
			failures = append(failures, r)
		}
	}

	if !allPass && v.llm != nil {
		// Feed failures to LLM for diagnosis (best-effort, don't fail on LLM error)
		prompt := renderDiagnosisPrompt(failures)
		_, _ = v.llm.Invoke(ctx, prompt)
	}

	return FinalResult{
		Pass:          allPass,
		AlwaysRun:     alwaysResults,
		ProjectChecks: projectResults,
	}, nil
}

func (v *ShellValidator) runChecks(ctx context.Context, checks []Check, workDir string) CheckResults {
	var results []CheckResult
	for _, c := range checks {
		start := time.Now()
		cmd := exec.CommandContext(ctx, "sh", "-c", c.Command)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		duration := time.Since(start)
		pass := err == nil
		results = append(results, CheckResult{
			Name:     c.Name,
			Pass:     pass,
			Output:   string(output),
			Duration: duration,
			Type:     c.Type,
		})
	}
	return CheckResults{Results: results}
}

func renderDiagnosisPrompt(failures []CheckResult) string {
	var b strings.Builder
	b.WriteString("The following validation checks failed. Diagnose the root cause.\n\n")
	for _, f := range failures {
		b.WriteString(fmt.Sprintf("## %s (%s)\n", f.Name, f.Type))
		b.WriteString(f.Output)
		b.WriteString("\n\n")
	}
	return b.String()
}
```

**Step 2: Run tests**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/validator/ -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/next/validator/shell_validator.go
git commit -m "green: ShellValidator runs checks via shell, LLM diagnosis on failure"
```

---

### Task 13: `ContextPktCompiler` adapter — failing tests

**Files:**
- Create: `internal/next/contextpkt/spec_compiler_test.go`

**Step 1: Write the failing tests**

The `stages.SpecCompiler` interface requires `Compile(ctx) (string, error)`. We need a thin adapter that wraps `contextpkt.DefaultCompiler` (which has a different signature: `Compile(ctx, cell, level, opts) (Packet, error)`) and returns the packet as a string.

```go
package contextpkt

import (
	"context"
	"testing"
)

func TestSpecCompilerAdapter_ReturnsPacketAsString(t *testing.T) {
	store := &inMemoryStore{
		data: map[string]map[string]string{},
	}
	adapter := NewSpecCompilerAdapter(SpecCompilerAdapterConfig{
		Store:    store,
		CellPath: t.TempDir(),
		CellName: "test-project",
		SpecPath: "specs/test.md",
	})

	result, err := adapter.Compile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty compiled output")
	}
}

// inMemoryStore satisfies ArtifactStore for tests.
type inMemoryStore struct {
	data map[string]map[string]string
}

func (s *inMemoryStore) Read(cellPath string, artifact string, dest any) error {
	return fmt.Errorf("not found")
}

func (s *inMemoryStore) Write(cellPath string, artifact string, src any) error {
	return nil
}

func (s *inMemoryStore) Exists(cellPath string, artifact string) bool {
	return false
}
```

**Step 2: Run and verify failure**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/contextpkt/ -run TestSpecCompilerAdapter -v -count=1`
Expected: FAIL

**Step 3: Commit**

```bash
git add internal/next/contextpkt/spec_compiler_test.go
git commit -m "red: SpecCompilerAdapter test for stages.SpecCompiler interface"
```

---

### Task 14: Implement `SpecCompilerAdapter`

**Files:**
- Create: `internal/next/contextpkt/spec_compiler_adapter.go`

**Step 1: Write implementation**

```go
package contextpkt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SpecCompilerAdapterConfig holds configuration for the adapter.
type SpecCompilerAdapterConfig struct {
	Store       ArtifactStore
	CellPath    string
	CellName    string
	SpecPath    string
	TokenBudget int
}

// SpecCompilerAdapter wraps DefaultCompiler to satisfy the stages.SpecCompiler interface.
type SpecCompilerAdapter struct {
	compiler *DefaultCompiler
	cfg      SpecCompilerAdapterConfig
}

// NewSpecCompilerAdapter creates a SpecCompilerAdapter.
func NewSpecCompilerAdapter(cfg SpecCompilerAdapterConfig) *SpecCompilerAdapter {
	return &SpecCompilerAdapter{
		compiler: NewCompiler(cfg.Store),
		cfg:      cfg,
	}
}

// Compile assembles the spec context packet and returns it as a string.
func (a *SpecCompilerAdapter) Compile(ctx context.Context) (string, error) {
	cell := Cell{Name: a.cfg.CellName, CellPath: a.cfg.CellPath}
	opts := CompileOpts{
		SpecPath:    a.cfg.SpecPath,
		TokenBudget: a.cfg.TokenBudget,
	}

	packet, err := a.compiler.Compile(ctx, cell, LevelSpec, opts)
	if err != nil {
		return "", fmt.Errorf("compile spec packet: %w", err)
	}

	// Render packet sections as readable text
	var b strings.Builder
	for _, s := range packet.Sections {
		b.WriteString(fmt.Sprintf("## %s\n\n", s.Name))
		b.WriteString(s.Content)
		b.WriteString("\n\n")
	}

	return b.String(), nil
}
```

**Step 2: Run tests**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/contextpkt/ -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/next/contextpkt/spec_compiler_adapter.go
git commit -m "green: SpecCompilerAdapter satisfies stages.SpecCompiler"
```

---

### Task 15: Refactor `extractJSON` into shared utility

**Files:**
- Create: `internal/next/llmadapter/parse.go`
- Modify: `internal/next/review/provider_agent.go` — use shared `extractJSON`
- Modify: `internal/next/acceptor/provider_agent.go` — use shared `extractJSON`

Both `ProviderReviewAgent` and `ProviderAcceptAgent` duplicate `extractJSON`. Move it to `llmadapter`.

**Step 1: Create shared utility**

```go
package llmadapter

import "strings"

// ExtractJSON extracts JSON from raw LLM output.
// Handles both bare JSON and markdown code fences (```json ... ```).
func ExtractJSON(output string) string {
	if idx := strings.Index(output, "```json"); idx >= 0 {
		start := idx + len("```json")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			return strings.TrimSpace(output[start : start+end])
		}
	}
	if idx := strings.Index(output, "```"); idx >= 0 {
		start := idx + len("```")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			return strings.TrimSpace(output[start : start+end])
		}
	}
	return strings.TrimSpace(output)
}
```

**Step 2: Update review and acceptor adapters to use `llmadapter.ExtractJSON`**

Remove local `extractJSON` from both files. Replace calls with `llmadapter.ExtractJSON`.

**Step 3: Run all tests**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/llmadapter/ ./internal/next/review/ ./internal/next/acceptor/ -v -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/next/llmadapter/parse.go internal/next/review/provider_agent.go internal/next/acceptor/provider_agent.go
git commit -m "refactor: extract shared ExtractJSON utility to llmadapter"
```

---

### Task 16: Wire real adapters in `RealStageProvider`

**Files:**
- Modify: `cmd/gromit-next/stage_provider.go`

**Step 1: Write a test for real adapter wiring**

Add to `cmd/gromit-next/exec_test.go` or create a new test file:

```go
func TestRealStageProvider_BuildStages_ReturnsRealAdapters(t *testing.T) {
	// Verify that BuildStages no longer returns noop implementations.
	// Use a mock provider to avoid real LLM calls.
	// This test verifies wiring, not LLM behavior.
}
```

This test should verify that calling `BuildStages` produces stages backed by real adapters. The exact assertion depends on whether stage types expose their internal dependencies (they may not). At minimum, verify it doesn't panic and returns the expected number of stages.

**Step 2: Update BuildStages to wire real adapters**

Replace noop implementations in `BuildStages` with:
- `contextpkt.NewSpecCompilerAdapter(...)` for Compile
- `planner.NewProviderPlanAgent(planAdapter)` wrapped in `planner.NewPlanner(agent, tier)` for Plan (note: PlanStage takes `PlanCreator` — check if `Planner` satisfies it)
- `specloop.NewProviderTaskRunner(execAdapter)` for Execute
- `validator.NewShellValidator(validateAdapter)` for Validate
- `review.NewProviderReviewAgent(reviewAdapter)` wrapped in `review.NewRunner(agent, config)` for Review
- `acceptor.NewProviderAcceptAgent(reviewAdapter)` wrapped in `acceptor.NewEvaluator(agent)` for Accept

The `RealStageProvider` will need a `provider.Provider` — for 0002c, hardcode Claude:

```go
claudeClient, err := claude.NewClient("claude", []string{"--no-input"}, 300)
claudeProvider := provider.NewClaudeProvider(claudeClient, map[string]string{
    provider.TierXHigh:  "opus",
    provider.TierHigh:   "opus",
    provider.TierMedium: "sonnet",
    provider.TierLow:    "haiku",
})
```

**Important:** Check what `stages.NewPlanStage` accepts — it takes `PlanCreator` interface, not `planner.Planner` directly. Verify that `planner.Planner` satisfies `PlanCreator`:

```go
// In stages/plan.go:
type PlanCreator interface {
    CreatePlan(ctx context.Context, req planner.PlanRequest) (planner.Plan, error)
}
```

`planner.Planner.CreatePlan` matches this signature.

Similarly for ReviewStage — it takes `ReviewRunner` (which has `Run(ctx, RunInput) (*RunResult, error)`). `review.Runner.Run` satisfies this.

And AcceptStage takes `AcceptEvaluator` (which has `Evaluate(ctx, EvaluateInput) (AcceptanceResult, error)`). `acceptor.Evaluator.Evaluate` satisfies this.

**Step 3: Run tests**

Run: `cd /Users/dabrams/gromit && go test ./cmd/gromit-next/ -v -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/gromit-next/stage_provider.go
git commit -m "feat: wire real LLM adapters in RealStageProvider, replacing noops"
```

---

### Task 17: Contract test framework

**Files:**
- Create: `internal/next/planner/agent_contract_test.go`
- Create: `internal/next/review/agent_contract_test.go`
- Create: `internal/next/acceptor/agent_contract_test.go`
- Create: `internal/next/specloop/taskrunner_contract_test.go`

**Step 1: Write contract test suites**

These are gated by build tag `//go:build llmcontract` and env var `GROMIT_LLM_CONTRACT=1`. They run against real providers (configured via env) and assert only structural contracts.

Example for planner:

```go
//go:build llmcontract

package planner

import (
	"context"
	"os"
	"testing"
)

// RunPlanAgentContract runs the agent contract suite against any Agent implementation.
func RunPlanAgentContract(t *testing.T, agent Agent) {
	t.Run("returns parseable plan", func(t *testing.T) {
		prompt := buildPlanPrompt(PlanRequest{
			SpecPacket: "Implement a function that adds two numbers and returns the result.",
			Cycle:      1,
		})
		result, err := agent.Invoke(context.Background(), prompt, "medium")
		if err != nil {
			t.Fatalf("agent invocation failed: %v", err)
		}
		if result.Output == "" {
			t.Fatal("expected non-empty output")
		}
		plan, err := ParsePlan(result.Output)
		if err != nil {
			t.Fatalf("output not parseable as Plan: %v", err)
		}
		if len(plan.Tasks) == 0 {
			t.Error("plan should have at least one task")
		}
		for _, task := range plan.Tasks {
			if task.TaskID == "" {
				t.Error("task missing task_id")
			}
			if task.Objective == "" {
				t.Error("task missing objective")
			}
		}
	})

	t.Run("returns token counts", func(t *testing.T) {
		result, err := agent.Invoke(context.Background(), "Generate a plan for: add logging", "low")
		if err != nil {
			t.Fatalf("agent invocation failed: %v", err)
		}
		if result.TokensIn == 0 {
			t.Error("expected non-zero TokensIn")
		}
		if result.TokensOut == 0 {
			t.Error("expected non-zero TokensOut")
		}
	})
}

func TestContract_ProviderPlanAgent(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1 to run contract tests")
	}
	// Build real Claude provider from env
	agent := buildRealPlanAgent(t)
	RunPlanAgentContract(t, agent)
}

func buildRealPlanAgent(t *testing.T) Agent {
	t.Helper()
	// Wire up real Claude provider → LLMAdapter → ProviderPlanAgent
	// Implementation depends on how Claude client is configured.
	// This is intentionally left for the implementor to wire based on
	// the local environment (claude binary must be available).
	t.Fatal("TODO: wire real provider for contract tests")
	return nil
}
```

Same pattern for review, acceptor, and taskrunner contracts. Key assertions per domain:

**Review contract:**
- Returns parseable `[]Finding` (valid JSON array)
- Each finding has severity (valid enum), file (non-empty), description (non-empty)
- Empty findings (clean code prompt) returns `[]Finding{}` not nil

**Acceptor contract:**
- Returns parseable `CriterionResult`
- Status is one of pass/fail/unclear
- Fail/unclear includes non-empty rationale

**TaskRunner contract:**
- RunTask returns TaskResult with non-empty status
- RepairTask returns TaskResult with non-empty status
- TokensUsed > 0

**Step 2: Verify they compile but skip without env var**

Run: `cd /Users/dabrams/gromit && go test -tags llmcontract ./internal/next/planner/ ./internal/next/review/ ./internal/next/acceptor/ ./internal/next/specloop/ -run TestContract -v -count=1`
Expected: All SKIP (env var not set)

**Step 3: Commit**

```bash
git add internal/next/planner/agent_contract_test.go internal/next/review/agent_contract_test.go internal/next/acceptor/agent_contract_test.go internal/next/specloop/taskrunner_contract_test.go
git commit -m "feat: contract test suites for all domain agents (gated by build tag + env var)"
```

---

### Task 18: Wire contract tests to real Claude provider

**Files:**
- Modify: `internal/next/planner/agent_contract_test.go` — implement `buildRealPlanAgent`
- Modify: `internal/next/review/agent_contract_test.go` — implement `buildRealReviewAgent`
- Modify: `internal/next/acceptor/agent_contract_test.go` — implement `buildRealAcceptAgent`
- Modify: `internal/next/specloop/taskrunner_contract_test.go` — implement `buildRealTaskRunner`

**Step 1: Implement real provider wiring**

Each `buildReal*` function:
1. Creates a `claude.NewClient("claude", []string{"--no-input"}, 120)` (2min timeout for contract tests)
2. Creates a `provider.NewClaudeProvider(client, tierMap)`
3. Creates an `llmadapter.New(provider, llmadapter.Config{Tier: "low"})` (use cheapest tier)
4. Creates the domain adapter (`NewProviderPlanAgent`, `NewProviderReviewAgent`, etc.)

**Step 2: Run contract tests locally**

Run: `cd /Users/dabrams/gromit && GROMIT_LLM_CONTRACT=1 go test -tags llmcontract ./internal/next/planner/ -run TestContract -v -count=1 -timeout 120s`
Expected: PASS — contract assertions satisfied by Claude

Repeat for each domain.

**Step 3: Commit**

```bash
git add internal/next/planner/agent_contract_test.go internal/next/review/agent_contract_test.go internal/next/acceptor/agent_contract_test.go internal/next/specloop/taskrunner_contract_test.go
git commit -m "feat: wire contract tests to real Claude provider"
```

---

### Task 19: Integration scenario scaffolds

**Files:**
- Create: `internal/next/specloop/integration_test.go`

**Step 1: Write scenario scaffolds**

```go
//go:build llmcontract

package specloop

import (
	"os"
	"testing"
)

func TestIntegration_HappyPath(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: implement happy path scenario")
}

func TestIntegration_ValidationFailureTriggersRepair(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: implement validation failure scenario")
}

func TestIntegration_ReviewTriggersReplan(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: implement review replan scenario")
}

func TestIntegration_BudgetExhaustion(t *testing.T) {
	if os.Getenv("GROMIT_LLM_CONTRACT") != "1" {
		t.Skip("set GROMIT_LLM_CONTRACT=1")
	}
	t.Skip("TODO: implement budget exhaustion scenario")
}
```

**Step 2: Commit**

```bash
git add internal/next/specloop/integration_test.go
git commit -m "feat: integration test scaffolds for pipeline scenarios"
```

---

### Task 20: Final verification — all tests pass

**Step 1: Run all gromit-next tests**

Run: `cd /Users/dabrams/gromit && go test ./internal/next/... ./cmd/gromit-next/ -v -count=1`
Expected: PASS — all unit tests green, contract tests skipped

**Step 2: Run linter**

Run: `cd /Users/dabrams/gromit && gofmt -l ./internal/next/llmadapter/ ./internal/next/planner/provider_agent.go ./internal/next/review/provider_agent.go ./internal/next/acceptor/provider_agent.go ./internal/next/specloop/provider_taskrunner.go ./internal/next/validator/shell_validator.go ./internal/next/contextpkt/spec_compiler_adapter.go`
Expected: No output (all formatted)

**Step 3: Verify contract tests locally**

Run: `cd /Users/dabrams/gromit && GROMIT_LLM_CONTRACT=1 go test -tags llmcontract ./internal/next/... -run TestContract -v -count=1 -timeout 300s`
Expected: PASS for all contract tests against Claude

**Step 4: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore: final cleanup for spec 0002c"
```
