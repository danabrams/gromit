# Layered Failure Triage Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a programmatic triage step to the escalation handler that classifies failures into four layers (provider_transport, environment, orchestration, code) before the LLM analyzer runs, so only code-level failures pay for an LLM call.

**Architecture:** A `Triage()` function in the `escalation` package inspects `provider.Result` metadata, stderr patterns, and bead structure to classify failures. `ExecuteWithRetry` calls `Triage()` before `AnalyzeAndHandleFailure()` and short-circuits for non-code layers. Two new fields on `IterationResult` record the layer and sub-category for logging.

**Tech Stack:** Go, table-driven tests, regexp for environment pattern matching.

**Spec:** `.gromit/specs/layered-failure-triage.md`

---

### Task 1: Add FailureLayer and TriageResult types

**Files:**
- Create: `internal/runner/escalation/triage.go`
- Test: `internal/runner/escalation/triage_test.go`

**Step 1: Write the type definition test**

```go
// triage_test.go
package escalation

import "testing"

func TestFailureLayerConstants(t *testing.T) {
	// Verify all four layer constants exist and have distinct values
	layers := []FailureLayer{
		LayerProviderTransport,
		LayerEnvironment,
		LayerOrchestration,
		LayerCode,
	}
	seen := map[FailureLayer]bool{}
	for _, l := range layers {
		if l == "" {
			t.Errorf("layer constant is empty")
		}
		if seen[l] {
			t.Errorf("duplicate layer constant: %s", l)
		}
		seen[l] = true
	}
}

func TestTriageResultFields(t *testing.T) {
	tr := &TriageResult{
		Layer:       LayerEnvironment,
		SubCategory: "missing_tool",
		Detail:      "go not found in PATH",
		Retryable:   false,
	}
	if tr.Layer != LayerEnvironment {
		t.Errorf("expected LayerEnvironment, got %s", tr.Layer)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/escalation/ -run TestFailureLayerConstants -v`
Expected: FAIL — `FailureLayer` type not defined

**Step 3: Write the types**

```go
// triage.go
package escalation

// FailureLayer represents the top-level failure classification.
type FailureLayer string

const (
	LayerProviderTransport FailureLayer = "provider_transport"
	LayerEnvironment       FailureLayer = "environment"
	LayerOrchestration     FailureLayer = "orchestration"
	LayerCode              FailureLayer = "code"
)

// TriageResult holds the programmatic triage outcome.
type TriageResult struct {
	Layer       FailureLayer
	SubCategory string // e.g. "rate_limit", "missing_tool", "scope_too_large"
	Detail      string // human-readable explanation
	Retryable   bool   // whether this failure can be retried
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/runner/escalation/ -run "TestFailureLayer|TestTriageResult" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/escalation/triage.go internal/runner/escalation/triage_test.go
git commit -m "Add FailureLayer and TriageResult types for failure triage"
```

---

### Task 2: Add IterationResult fields for failure layer

**Files:**
- Modify: `internal/runner/runtypes/types.go` (add two fields to IterationResult)

**Step 1: Write the test**

Add to an existing test file or create a small one verifying the fields exist:

```go
// In a test that constructs IterationResult
func TestIterationResultHasFailureLayerFields(t *testing.T) {
	r := &runtypes.IterationResult{
		FailureLayer:  "environment",
		FailureSubCat: "missing_tool",
	}
	if r.FailureLayer != "environment" {
		t.Errorf("expected environment, got %s", r.FailureLayer)
	}
	if r.FailureSubCat != "missing_tool" {
		t.Errorf("expected missing_tool, got %s", r.FailureSubCat)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/runtypes/ -run TestIterationResultHasFailureLayerFields -v`
Expected: FAIL — fields not defined

**Step 3: Add the fields**

In `internal/runner/runtypes/types.go`, add to `IterationResult` after the diagnostic fields block:

```go
	// Failure triage classification
	FailureLayer  string // "provider_transport", "environment", "orchestration", "code", ""
	FailureSubCat string // e.g. "rate_limit", "missing_tool", "scope_too_large", "syntax"
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/runner/runtypes/ -run TestIterationResultHasFailureLayerFields -v`
Expected: PASS

**Step 5: Run full test suite to check nothing broke**

Run: `go test ./internal/runner/... -count=1`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/runner/runtypes/types.go
git commit -m "Add FailureLayer and FailureSubCat fields to IterationResult"
```

---

### Task 3: Implement Triage() for provider_transport layer

**Files:**
- Modify: `internal/runner/escalation/triage.go`
- Modify: `internal/runner/escalation/triage_test.go`

**Step 1: Write failing tests for provider_transport detection**

```go
func TestTriage_ProviderTransport_Disconnect(t *testing.T) {
	result := Triage(nil, &provider.Result{
		FailureCategory: provider.FailureCategoryTransportDisconnect,
	}, nil)
	if result.Layer != LayerProviderTransport {
		t.Errorf("expected provider_transport, got %s", result.Layer)
	}
	if result.SubCategory != "disconnect" {
		t.Errorf("expected disconnect, got %s", result.SubCategory)
	}
	if !result.Retryable {
		t.Error("disconnect should be retryable")
	}
}

func TestTriage_ProviderTransport_RateLimited(t *testing.T) {
	result := Triage(nil, &provider.Result{
		FailureCategory: provider.FailureCategoryRateLimited,
	}, nil)
	if result.Layer != LayerProviderTransport {
		t.Errorf("expected provider_transport, got %s", result.Layer)
	}
	if result.SubCategory != "rate_limit" {
		t.Errorf("expected rate_limit, got %s", result.SubCategory)
	}
	if !result.Retryable {
		t.Error("rate_limit should be retryable")
	}
}

func TestTriage_ProviderTransport_Auth(t *testing.T) {
	result := Triage(nil, &provider.Result{
		FailureCategory: provider.FailureCategoryAuth,
	}, nil)
	if result.Layer != LayerProviderTransport {
		t.Errorf("expected provider_transport, got %s", result.Layer)
	}
	if result.SubCategory != "auth" {
		t.Errorf("expected auth, got %s", result.SubCategory)
	}
	if result.Retryable {
		t.Error("auth should not be retryable")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/escalation/ -run "TestTriage_ProviderTransport" -v`
Expected: FAIL — `Triage` function not defined

**Step 3: Implement the Triage function (provider_transport layer only)**

```go
// Triage classifies a failure into a layer using only programmatic signals.
// No LLM calls. Inspects provider result metadata, stderr patterns, and bead structure.
//
// Parameters:
//   - invResult: the escalation InvocationResult (may be nil for non-invocation errors)
//   - provResult: the provider.Result with FailureCategory, Stderr, Output
//   - b: the bead being processed (used for orchestration checks)
func Triage(invResult *InvocationResult, provResult *provider.Result, b *bead.Bead) *TriageResult {
	if provResult == nil {
		return &TriageResult{Layer: LayerCode, SubCategory: "", Detail: "no provider result", Retryable: false}
	}

	// Layer 1: Provider transport
	switch provResult.FailureCategory {
	case provider.FailureCategoryTransportDisconnect:
		return &TriageResult{
			Layer:       LayerProviderTransport,
			SubCategory: "disconnect",
			Detail:      "transport disconnected from provider",
			Retryable:   true,
		}
	case provider.FailureCategoryRateLimited:
		return &TriageResult{
			Layer:       LayerProviderTransport,
			SubCategory: "rate_limit",
			Detail:      "provider rate limited",
			Retryable:   true,
		}
	case provider.FailureCategoryAuth:
		return &TriageResult{
			Layer:       LayerProviderTransport,
			SubCategory: "auth",
			Detail:      "authentication failed — check API key / credentials",
			Retryable:   false,
		}
	}

	// Fall through to code layer (other layers added in subsequent tasks)
	return &TriageResult{Layer: LayerCode, SubCategory: "", Detail: "", Retryable: false}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/runner/escalation/ -run "TestTriage_ProviderTransport" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/escalation/triage.go internal/runner/escalation/triage_test.go
git commit -m "Implement Triage() for provider_transport layer"
```

---

### Task 4: Implement Triage() for environment layer

**Files:**
- Modify: `internal/runner/escalation/triage.go`
- Modify: `internal/runner/escalation/triage_test.go`

**Step 1: Write failing tests for environment detection**

```go
func TestTriage_Environment_MissingTool(t *testing.T) {
	result := Triage(nil, &provider.Result{
		Stderr: `exec: "go": executable file not found in $PATH`,
	}, nil)
	if result.Layer != LayerEnvironment {
		t.Errorf("expected environment, got %s", result.Layer)
	}
	if result.SubCategory != "missing_tool" {
		t.Errorf("expected missing_tool, got %s", result.SubCategory)
	}
	if result.Retryable {
		t.Error("environment errors should not be retryable")
	}
}

func TestTriage_Environment_VersionMismatch(t *testing.T) {
	result := Triage(nil, &provider.Result{
		Stderr: `go: go.mod requires go >= 1.22.0 (running go 1.21.3)`,
	}, nil)
	if result.Layer != LayerEnvironment {
		t.Errorf("expected environment, got %s", result.Layer)
	}
	if result.SubCategory != "version_mismatch" {
		t.Errorf("expected version_mismatch, got %s", result.SubCategory)
	}
}

func TestTriage_Environment_DiskFull(t *testing.T) {
	result := Triage(nil, &provider.Result{
		Stderr: `write /tmp/build123: no space left on device`,
	}, nil)
	if result.Layer != LayerEnvironment {
		t.Errorf("expected environment, got %s", result.Layer)
	}
	if result.SubCategory != "resource_exhausted" {
		t.Errorf("expected resource_exhausted, got %s", result.SubCategory)
	}
}

func TestTriage_Environment_PermissionDenied(t *testing.T) {
	result := Triage(nil, &provider.Result{
		Stderr: `open /etc/passwd: permission denied`,
	}, nil)
	if result.Layer != LayerEnvironment {
		t.Errorf("expected environment, got %s", result.Layer)
	}
	if result.SubCategory != "permission" {
		t.Errorf("expected permission, got %s", result.SubCategory)
	}
}

func TestTriage_Environment_FallsBackToOutput(t *testing.T) {
	// When Stderr is empty, check Output
	result := Triage(nil, &provider.Result{
		Output: `exec: "node": executable file not found in $PATH`,
	}, nil)
	if result.Layer != LayerEnvironment {
		t.Errorf("expected environment from Output fallback, got %s", result.Layer)
	}
}

func TestTriage_Environment_NoFalsePositive(t *testing.T) {
	// Normal test failure output should NOT trigger environment detection
	result := Triage(nil, &provider.Result{
		Stderr: `--- FAIL: TestFoo (0.01s)\n    foo_test.go:10: expected 42, got 0`,
	}, nil)
	if result.Layer == LayerEnvironment {
		t.Error("normal test failure should not be classified as environment")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/escalation/ -run "TestTriage_Environment" -v`
Expected: FAIL — environment detection not implemented

**Step 3: Add environment pattern matching to Triage()**

Add after the provider_transport switch block in `Triage()`:

```go
	// Layer 2: Environment
	// Check stderr first, fall back to Output if stderr is empty.
	text := provResult.Stderr
	if text == "" {
		text = provResult.Output
	}
	if envResult := matchEnvironmentPatterns(text); envResult != nil {
		return envResult
	}
```

And add the pattern matching function:

```go
import "regexp"

// envPattern maps a compiled regexp to a sub-category and detail message.
type envPattern struct {
	re          *regexp.Regexp
	subCategory string
	detail      string
}

// envPatterns is the ordered list of environment error patterns.
// Conservative: only high-confidence patterns that cannot be confused with code errors.
var envPatterns = []envPattern{
	{
		re:          regexp.MustCompile(`exec: .+: executable file not found`),
		subCategory: "missing_tool",
		detail:      "required tool not found in PATH",
	},
	{
		re:          regexp.MustCompile(`go: go\.mod requires go >=`),
		subCategory: "version_mismatch",
		detail:      "Go version too old for go.mod requirement",
	},
	{
		re:          regexp.MustCompile(`(?i)no space left on device`),
		subCategory: "resource_exhausted",
		detail:      "disk full",
	},
	{
		re:          regexp.MustCompile(`(?i)permission denied`),
		subCategory: "permission",
		detail:      "file or directory permission denied",
	},
}

// matchEnvironmentPatterns checks text against known environment error patterns.
// Returns nil if no pattern matches.
func matchEnvironmentPatterns(text string) *TriageResult {
	for _, p := range envPatterns {
		if p.re.MatchString(text) {
			return &TriageResult{
				Layer:       LayerEnvironment,
				SubCategory: p.subCategory,
				Detail:      p.detail,
				Retryable:   false,
			}
		}
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/runner/escalation/ -run "TestTriage_Environment" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/escalation/triage.go internal/runner/escalation/triage_test.go
git commit -m "Add environment layer detection with conservative stderr patterns"
```

---

### Task 5: Implement Triage() for orchestration layer

**Files:**
- Modify: `internal/runner/escalation/triage.go`
- Modify: `internal/runner/escalation/triage_test.go`

**Step 1: Write failing tests for orchestration detection**

```go
func TestTriage_Orchestration_BadPrompt(t *testing.T) {
	result := Triage(&InvocationResult{}, &provider.Result{}, &bead.Bead{
		Description: "some description",
	})
	// With a non-empty bead, this should fall through to code
	if result.Layer != LayerCode {
		t.Errorf("expected code, got %s", result.Layer)
	}
}

func TestTriage_Orchestration_EmptyBead(t *testing.T) {
	result := Triage(nil, &provider.Result{}, &bead.Bead{
		ID:    "beads-123",
		Title: "Do something",
		// No description, no expected_outputs (proxy for acceptance criteria)
	})
	if result.Layer != LayerOrchestration {
		t.Errorf("expected orchestration, got %s", result.Layer)
	}
	if result.SubCategory != "bad_bead" {
		t.Errorf("expected bad_bead, got %s", result.SubCategory)
	}
}

func TestTriage_Orchestration_BeadWithDescription(t *testing.T) {
	// A bead with a description should NOT trigger bad_bead
	result := Triage(nil, &provider.Result{}, &bead.Bead{
		ID:          "beads-123",
		Title:       "Do something",
		Description: "Here is what to do",
	})
	if result.Layer == LayerOrchestration {
		t.Error("bead with description should not be classified as orchestration")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/runner/escalation/ -run "TestTriage_Orchestration" -v`
Expected: FAIL — orchestration detection not implemented

**Step 3: Add orchestration checks to Triage()**

Add after the environment check, before the code fallthrough:

```go
	// Layer 3: Orchestration
	if b != nil && b.Description == "" && len(b.ExpectedOutputs) == 0 {
		return &TriageResult{
			Layer:       LayerOrchestration,
			SubCategory: "bad_bead",
			Detail:      fmt.Sprintf("bead %s has no description and no acceptance criteria", b.ID),
			Retryable:   false,
		}
	}
```

Note: `scope_too_large` is already detected upstream in the runner via `provider.IsScopeTooLarge()` and handled before `ExecuteWithRetry` is called. We do not duplicate that check here. If a future caller needs it, it can be added.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/runner/escalation/ -run "TestTriage_Orchestration" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/escalation/triage.go internal/runner/escalation/triage_test.go
git commit -m "Add orchestration layer detection for malformed beads"
```

---

### Task 6: Integrate Triage into ExecuteWithRetry

**Files:**
- Modify: `internal/runner/escalation/handler.go`
- Modify: `internal/runner/escalation/handler_test.go`

This is the key integration task. `ExecuteWithRetry` must call `Triage()` after `showPartialProgress` and before `AnalyzeAndHandleFailure`, short-circuiting for non-code layers.

**Step 1: Write failing test — transport failure skips analyzer**

```go
func TestExecuteWithRetry_TransportFailureSkipsAnalyzer(t *testing.T) {
	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			Chain:            []string{"low", "medium", "high"},
			MaxRetriesPerModel: 1,
			MaxRetriesPerBead:  3,
		},
	}

	analyzerCalled := false
	mockAnalyzer := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			analyzerCalled = true
			return &analyzer.Analysis{Category: analyzer.CategoryLogic, Recoverable: false}, nil
		},
	}

	h := NewHandler(cfg, mockAnalyzer, &mockBeadClient{}, nil, nil, nil, nil)

	attempt := 0
	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error) {
		attempt++
		if attempt == 1 {
			// First attempt: transport disconnect
			return &InvocationResult{
				Result: &claude.Result{Success: false, Output: "connection reset"},
				ProviderResult: &provider.Result{
					FailureCategory: provider.FailureCategoryTransportDisconnect,
				},
			}, nil
		}
		// Second attempt: success
		return &InvocationResult{
			Result: &claude.Result{Success: true, Output: "done"},
		}, nil
	}

	bc := &runtypes.BeadContext{
		Bead:              &bead.Bead{ID: "test-1", Title: "Test"},
		Result:            &runtypes.IterationResult{},
		Tier:              "low",
		MaxRetries:        3,
		MaxRetriesPerBead: 5,
		BuildPrompt:       "build it",
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if !success {
		t.Error("expected success after transport retry")
	}
	if analyzerCalled {
		t.Error("analyzer should NOT be called for transport failures")
	}
	if bc.Result.FailureLayer != "" {
		// On success the layer should be empty (only set on final failure)
		// But during the retry the layer was set — check it was cleared or verify logs
	}
}
```

**Step 2: Write failing test — environment failure fails fast**

```go
func TestExecuteWithRetry_EnvironmentFailureFailsFast(t *testing.T) {
	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			Chain:            []string{"low", "medium", "high"},
			MaxRetriesPerModel: 3,
			MaxRetriesPerBead:  5,
		},
	}

	analyzerCalled := false
	mockAnalyzer := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			analyzerCalled = true
			return nil, nil
		},
	}

	h := NewHandler(cfg, mockAnalyzer, &mockBeadClient{}, nil, nil, nil, nil)

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error) {
		return &InvocationResult{
			Result: &claude.Result{Success: false, Output: ""},
			ProviderResult: &provider.Result{
				Stderr: `exec: "go": executable file not found in $PATH`,
			},
		}, nil
	}

	bc := &runtypes.BeadContext{
		Bead:              &bead.Bead{ID: "test-1", Title: "Test", Description: "desc"},
		Result:            &runtypes.IterationResult{},
		Tier:              "low",
		MaxRetries:        3,
		MaxRetriesPerBead: 5,
		BuildPrompt:       "build it",
	}

	success := h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if success {
		t.Error("expected failure for environment error")
	}
	if analyzerCalled {
		t.Error("analyzer should NOT be called for environment failures")
	}
	if bc.Result.FailureLayer != string(LayerEnvironment) {
		t.Errorf("expected FailureLayer=environment, got %s", bc.Result.FailureLayer)
	}
	if bc.Result.FailureSubCat != "missing_tool" {
		t.Errorf("expected FailureSubCat=missing_tool, got %s", bc.Result.FailureSubCat)
	}
}
```

**Step 3: Write failing test — code failure still calls analyzer**

```go
func TestExecuteWithRetry_CodeFailureCallsAnalyzer(t *testing.T) {
	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			Chain:            []string{"low", "medium", "high"},
			MaxRetriesPerModel: 1,
			MaxRetriesPerBead:  3,
		},
	}

	analyzerCalled := false
	mockAnalyzer := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, output string) (*analyzer.Analysis, error) {
			analyzerCalled = true
			return &analyzer.Analysis{Category: analyzer.CategoryLogic, Recoverable: false}, nil
		},
	}

	h := NewHandler(cfg, mockAnalyzer, &mockBeadClient{}, nil, nil, nil, nil)

	invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*InvocationResult, error) {
		return &InvocationResult{
			Result: &claude.Result{Success: false, Output: "test failed"},
			ProviderResult: &provider.Result{
				// No FailureCategory, no env patterns — falls through to code
			},
		}, nil
	}

	bc := &runtypes.BeadContext{
		Bead:              &bead.Bead{ID: "test-1", Title: "Test", Description: "desc"},
		Result:            &runtypes.IterationResult{},
		Tier:              "low",
		MaxRetries:        1,
		MaxRetriesPerBead: 3,
		BuildPrompt:       "build it",
	}

	_ = h.ExecuteWithRetry(context.Background(), bc, invokeFn)
	if !analyzerCalled {
		t.Error("analyzer SHOULD be called for code-layer failures")
	}
}
```

**Step 4: Run tests to verify they fail**

Run: `go test ./internal/runner/escalation/ -run "TestExecuteWithRetry_(Transport|Environment|Code)Failure" -v`
Expected: FAIL

**Step 5: Add ProviderResult field to InvocationResult**

The `InvocationResult` struct in `handler.go` needs a `ProviderResult` field so triage can access `FailureCategory` and `Stderr`:

```go
type InvocationResult struct {
	Result         *claude.Result
	ProviderResult *provider.Result  // NEW: raw provider result for triage
	StallFired     bool
	TimeoutType    string
}
```

**Step 6: Integrate triage into ExecuteWithRetry**

In `handler.go`, in `ExecuteWithRetry()`, replace the block after `showPartialProgress` and before `AnalyzeAndHandleFailure`:

```go
		// Triage: classify failure layer before expensive LLM analysis
		triageResult := Triage(invResult, invResult.ProviderResult, bc.Bead)
		bc.Result.FailureLayer = string(triageResult.Layer)
		bc.Result.FailureSubCat = triageResult.SubCategory

		switch triageResult.Layer {
		case LayerProviderTransport:
			if !triageResult.Retryable {
				h.log("Triage: %s/%s — %s (not retryable)", triageResult.Layer, triageResult.SubCategory, triageResult.Detail)
				bc.Result.Error = fmt.Errorf("%s: %s", triageResult.SubCategory, triageResult.Detail)
				return false
			}
			h.log("Triage: %s/%s — %s (retrying)", triageResult.Layer, triageResult.SubCategory, triageResult.Detail)
			bc.RetriesThisModel++
			bc.TotalRetriesThisBead++
			continue

		case LayerEnvironment:
			h.log("Triage: environment error — %s: %s", triageResult.SubCategory, triageResult.Detail)
			bc.Result.Error = fmt.Errorf("environment error: %s — %s", triageResult.SubCategory, triageResult.Detail)
			return false

		case LayerOrchestration:
			h.log("Triage: orchestration error — %s: %s", triageResult.SubCategory, triageResult.Detail)
			if triageResult.SubCategory == "scope_too_large" {
				return h.AttemptDecomposition(ctx, bc, "scope too large")
			}
			bc.Result.Error = fmt.Errorf("orchestration error: %s — %s", triageResult.SubCategory, triageResult.Detail)
			return false

		case LayerCode:
			// Fall through to existing LLM analyzer
		}

		// Analyze failure and decide: retry, escalate, or stop
		if h.AnalyzeAndHandleFailure(ctx, bc, claudeResult) {
			continue
		}
		return false
```

**Step 7: Run tests to verify they pass**

Run: `go test ./internal/runner/escalation/ -run "TestExecuteWithRetry_(Transport|Environment|Code)Failure" -v`
Expected: PASS

**Step 8: Run the full escalation test suite**

Run: `go test ./internal/runner/escalation/ -v -count=1`
Expected: All PASS (existing tests should still work because their `InvocationResult.ProviderResult` is nil, which makes `Triage()` return `LayerCode`, falling through to the existing path)

**Step 9: Commit**

```bash
git add internal/runner/escalation/handler.go internal/runner/escalation/handler_test.go
git commit -m "Integrate triage into ExecuteWithRetry, short-circuit non-code layers"
```

---

### Task 7: Wire ProviderResult through the invocation chain

**Files:**
- Modify: `internal/runner/callbacks.go` (where `InvokeFn` is constructed)
- Possibly modify: `internal/runner/execution/invoker.go` (if it doesn't already return `provider.Result`)

The `InvokeFn` closure in `callbacks.go` creates `escalation.InvocationResult`. It must now populate the `ProviderResult` field from the provider's raw result. Check how the invoker returns data and wire the `provider.Result` through.

**Step 1: Read `callbacks.go` to find the InvokeFn construction**

Read `internal/runner/callbacks.go` to find where `escalation.InvocationResult` is created. Identify where `provider.Result` is available.

**Step 2: Wire the ProviderResult field**

Set `ProviderResult: providerResult` in the `InvocationResult` construction. The exact field depends on what the invoker returns — it may already be on the `claude.Result` or may need to be threaded through from `execution.Invoker`.

**Step 3: Run full test suite**

Run: `go test ./internal/runner/... -count=1`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/runner/callbacks.go
git commit -m "Wire provider.Result through InvocationResult for triage"
```

---

### Task 8: End-to-end verification and cleanup

**Files:**
- Review: all modified files
- Run: full test suite

**Step 1: Run full project tests**

Run: `go test ./... -count=1`
Expected: All PASS

**Step 2: Run linter**

Run: `golangci-lint run ./...`
Expected: No issues

**Step 3: Format**

Run: `go fmt ./...`
Expected: No changes (already formatted)

**Step 4: Verify triage is called on real failure paths**

Review the integration manually:
- `ExecuteWithRetry` calls `Triage()` before `AnalyzeAndHandleFailure()` — confirmed
- `ProviderResult` is populated in the `InvokeFn` — confirmed
- `IterationResult.FailureLayer` and `FailureSubCat` are set — confirmed
- Existing tests still pass (backward compatible) — confirmed

**Step 5: Final commit if any cleanup was needed**

```bash
git add -A
git commit -m "Clean up layered failure triage integration"
```
