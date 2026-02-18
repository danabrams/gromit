# Routing Ratio Reliability Review — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add per-provider observability, fix Codex cost tracking, and implement a conservative circuit-breaker for transport instability.

**Architecture:** Four incremental parts, each independently testable. Schema changes land first (provider/failure_category fields), then cost-tracking fix, then per-provider metrics aggregation, then circuit-breaker in the Router.

**Tech Stack:** Go, JSONL iteration logs, process_trend.json, provider.Router

---

### Task 1: Add `Provider` and `FailureCategory` Fields to IterationResult

**Files:**
- Modify: `internal/runner/runtypes/types.go:57-107`
- Test: `internal/runner/runtypes/types_test.go`

**Step 1: Write the failing test**

```go
// In types_test.go, add:
func TestIterationResult_HasProviderAndFailureCategory(t *testing.T) {
	r := runtypes.IterationResult{
		Provider:        "codex",
		FailureCategory: "transport_disconnect",
	}
	if r.Provider != "codex" {
		t.Errorf("Provider = %q, want %q", r.Provider, "codex")
	}
	if r.FailureCategory != "transport_disconnect" {
		t.Errorf("FailureCategory = %q, want %q", r.FailureCategory, "transport_disconnect")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/runtypes/ -run TestIterationResult_HasProviderAndFailureCategory -v`
Expected: FAIL — `r.Provider` undefined

**Step 3: Add fields to IterationResult**

In `internal/runner/runtypes/types.go`, add after line 60 (`Model string`):

```go
	Provider        string
	FailureCategory string
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/runner/runtypes/ -run TestIterationResult_HasProviderAndFailureCategory -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/runtypes/types.go internal/runner/runtypes/types_test.go
git commit -m "add Provider and FailureCategory fields to IterationResult"
```

---

### Task 2: Add `Provider` and `FailureCategory` Fields to IterationLog

**Files:**
- Modify: `internal/logger/logger.go:13-61`
- Test: `internal/logger/logger_test.go`

**Step 1: Write the failing test**

```go
func TestIterationLog_HasProviderAndFailureCategory(t *testing.T) {
	log := logger.IterationLog{
		Provider:        "codex",
		FailureCategory: "transport_disconnect",
	}
	data, _ := json.Marshal(log)
	if !strings.Contains(string(data), `"provider":"codex"`) {
		t.Errorf("JSON missing provider field: %s", data)
	}
	if !strings.Contains(string(data), `"failure_category":"transport_disconnect"`) {
		t.Errorf("JSON missing failure_category field: %s", data)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/logger/ -run TestIterationLog_HasProviderAndFailureCategory -v`
Expected: FAIL — `log.Provider` undefined

**Step 3: Add fields to IterationLog**

In `internal/logger/logger.go`, add after line 18 (`Model string`):

```go
	Provider        string `json:"provider,omitempty"`
	FailureCategory string `json:"failure_category,omitempty"`
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/logger/ -run TestIterationLog_HasProviderAndFailureCategory -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/logger/logger.go internal/logger/logger_test.go
git commit -m "add provider and failure_category fields to IterationLog"
```

---

### Task 3: Wire Provider and FailureCategory Through the Runner Logging Path

**Files:**
- Modify: `internal/runner/logging.go:46-90`
- Modify: `internal/runner/callbacks.go:325-333`
- Test: `internal/runner/writeiterationlog_test.go`

**Step 1: Write the failing test**

```go
func TestWriteIterationLog_IncludesProviderAndFailureCategory(t *testing.T) {
	// Create a runner with a test logger that captures the IterationLog
	// Set result.Provider = "codex", result.FailureCategory = "transport_disconnect"
	// Call writeIterationLog()
	// Assert the captured IterationLog has Provider="codex" and FailureCategory="transport_disconnect"
}
```

Use the existing test patterns in `writeiterationlog_test.go` for the runner/logger setup.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestWriteIterationLog_IncludesProviderAndFailureCategory -v`
Expected: FAIL — Provider field not copied

**Step 3: Wire the fields**

In `internal/runner/logging.go`, add to the `IterationLog` literal at line 46:

```go
	Provider:        result.Provider,
	FailureCategory: result.FailureCategory,
```

In `internal/runner/callbacks.go`, after line 329 (`bc.Model = modelName`), add:

```go
	bc.Result.Provider = p.Name()
```

After the invocation completes (near the end of the build callback where `result` is available), add:

```go
	if result != nil {
		bc.Result.FailureCategory = result.FailureCategory
	}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/runner/ -run TestWriteIterationLog_IncludesProviderAndFailureCategory -v`
Expected: PASS

**Step 5: Run full test suite to check backward compatibility**

Run: `go test ./internal/runner/ -count=1`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/runner/logging.go internal/runner/callbacks.go internal/runner/writeiterationlog_test.go
git commit -m "wire provider and failure_category through iteration logging"
```

---

### Task 4: Add `Provider` and `FailureCategory` to IterationMetric

**Files:**
- Modify: `internal/logger/process_trend.go:19-39` (IterationMetric struct)
- Modify: `internal/logger/process_trend.go:245-265` (buildIterationMetrics)
- Test: `internal/logger/process_trend_test.go`

**Step 1: Write the failing test**

```go
func TestBuildIterationMetrics_IncludesProviderAndFailureCategory(t *testing.T) {
	entries := []logger.IterationLog{
		{
			Timestamp: time.Now(),
			Model:     "gpt-5.2-codex",
			Provider:  "codex",
			FailureCategory: "transport_disconnect",
			Success:   false,
			DurationMs: 1000,
		},
	}
	// Call buildIterationMetrics (or use BuildContinuousMetrics with temp files)
	// Assert the resulting IterationMetric has Provider="codex" and FailureCategory="transport_disconnect"
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/logger/ -run TestBuildIterationMetrics_IncludesProviderAndFailureCategory -v`
Expected: FAIL — IterationMetric has no Provider field

**Step 3: Add fields to IterationMetric and wire in buildIterationMetrics**

In `internal/logger/process_trend.go`, add to `IterationMetric` struct (after `Model` field at line 23):

```go
	Provider        string  `json:"provider,omitempty"`
	FailureCategory string  `json:"failure_category,omitempty"`
```

In `buildIterationMetrics` (line 245), add to the `IterationMetric` literal:

```go
	Provider:        entry.Provider,
	FailureCategory: entry.FailureCategory,
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/logger/ -run TestBuildIterationMetrics_IncludesProviderAndFailureCategory -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/logger/process_trend.go internal/logger/process_trend_test.go
git commit -m "add provider and failure_category to IterationMetric"
```

---

### Task 5: Investigate and Fix Codex Token Propagation

This task requires investigation. The `processCodexStream` function captures `codexUsage` but all Codex iterations log `input_tokens: 0`. Trace where the data is lost.

**Files:**
- Investigate: `internal/provider/codex.go:146` (processCodexStream return)
- Investigate: `internal/runner/callbacks.go:86-91` (stats.CostData())
- Investigate: `internal/logger/stream.go` (StreamStats.CostData)
- Test: `internal/provider/codex_cost_tracking_test.go` (existing)

**Step 1: Write a test that verifies tokens propagate from Codex StreamRun Result**

```go
func TestCodexStreamRun_PropagatesTokensToResult(t *testing.T) {
	// Set up a fake codex binary that emits JSONL with turn.completed usage data
	// Call StreamRun
	// Assert result.InputTokens > 0 and result.OutputTokens > 0
}
```

Check `internal/provider/codex_cost_tracking_test.go` for existing patterns.

**Step 2: Run the test and determine if it passes or fails**

If the test passes, the bug is upstream (in the runner's stats extraction, not in the provider). If it fails, the bug is in `processCodexStream` or `StreamRun`.

**Step 3: Follow the data to the break point and fix**

Likely the `StreamStats.CostData()` path (used by callbacks.go:87) reads from Claude-specific stream events, not from the provider.Result. Check whether the runner prefers `stats.CostData()` over `result.CostUSD` — if stats returns zeros for Codex, those zeros overwrite the nonzero values from the Result.

Fix: After `stats.CostData()` at `callbacks.go:87-91`, check if stats returned zeros but the result has nonzero values, and prefer the result's values.

**Step 4: Verify the fix**

Run: `go test ./internal/provider/ -run TestCodexStreamRun_PropagatesTokensToResult -v`
Run: `go test ./internal/runner/ -count=1`
Expected: All pass

**Step 5: Commit**

```bash
git add <modified files>
git commit -m "fix: codex token data propagation to iteration logs"
```

---

### Task 6: Add ProviderMetrics Struct and Aggregation

**Files:**
- Modify: `internal/logger/process_trend.go:74-81` (ProcessTrend struct)
- Create: (add ProviderMetrics type in same file or in `internal/logger/reliability.go`)
- Test: `internal/logger/process_trend_test.go`

**Step 1: Write the failing test**

```go
func TestBuildProcessTrend_IncludesProviderMetrics(t *testing.T) {
	// Create IterationMetric entries with mixed providers
	// 5 codex entries (3 success, 2 transport_disconnect)
	// 5 claude entries (4 success, 1 other failure)
	// Build trend
	// Assert trend.ProviderMetrics has 2 entries
	// Assert codex entry: SuccessRate=0.6, TransportFailureRate=0.4
	// Assert claude entry: SuccessRate=0.8, TransportFailureRate=0.0
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/logger/ -run TestBuildProcessTrend_IncludesProviderMetrics -v`
Expected: FAIL — ProcessTrend has no ProviderMetrics field

**Step 3: Add ProviderMetrics struct and aggregation**

Add to `internal/logger/process_trend.go`:

```go
type ProviderMetrics struct {
	Name                 string  `json:"name"`
	TotalInvocations     int     `json:"total_invocations"`
	Successes            int     `json:"successes"`
	SuccessRate          float64 `json:"success_rate"`
	TransportFailures    int     `json:"transport_failures"`
	TransportFailureRate float64 `json:"transport_failure_rate"`
	FallbacksTriggered   int     `json:"fallbacks_triggered"`
	AvgDurationMs        int64   `json:"avg_duration_ms"`
	TotalCostUSD         float64 `json:"total_cost_usd"`
	TotalInputTokens     int     `json:"total_input_tokens"`
	TotalOutputTokens    int     `json:"total_output_tokens"`
}
```

Add to `ProcessTrend` struct (line 80):

```go
	ProviderMetrics []ProviderMetrics `json:"provider_metrics,omitempty"`
```

Add a `computeProviderMetrics(metrics []IterationMetric, windowSize int) []ProviderMetrics` function that:
1. Takes the latest `windowSize` metrics
2. Groups by `Provider` field
3. Computes per-provider success rate, transport failure rate, avg duration, total cost, total tokens

Wire it into `buildProcessTrend()` at line 271.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/logger/ -run TestBuildProcessTrend_IncludesProviderMetrics -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/logger/process_trend.go internal/logger/process_trend_test.go
git commit -m "add per-provider reliability metrics to process_trend.json"
```

---

### Task 7: Add CircuitBreakerConfig to Config

**Files:**
- Modify: `internal/config/config.go:291-295` (RoutingConfig struct)
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test**

```go
func TestConfig_CircuitBreakerParsing(t *testing.T) {
	yaml := `
routing:
  circuit_breaker:
    enabled: true
    window_size: 10
    failure_threshold: 0.3
    degraded_floor: 20
    recovery_successes: 5
`
	cfg, err := config.Parse([]byte(yaml))
	// Assert no error
	// Assert cfg.Routing.CircuitBreaker.Enabled == true
	// Assert cfg.Routing.CircuitBreaker.WindowSize == 10
	// Assert cfg.Routing.CircuitBreaker.FailureThreshold == 0.3
	// Assert cfg.Routing.CircuitBreaker.DegradedFloor == 20
	// Assert cfg.Routing.CircuitBreaker.RecoverySuccesses == 5
}

func TestConfig_CircuitBreakerDefaults(t *testing.T) {
	yaml := `routing: {}`
	cfg, err := config.Parse([]byte(yaml))
	// Assert cfg.Routing.CircuitBreaker.Enabled == false (default)
	// Assert cfg.Routing.CircuitBreaker.WindowSize == 0 (zero value, router uses default)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestConfig_CircuitBreaker -v`
Expected: FAIL — CircuitBreaker field undefined

**Step 3: Add CircuitBreakerConfig**

In `internal/config/config.go`, add after `FallbackConfig` (line 300):

```go
type CircuitBreakerConfig struct {
	Enabled           bool    `yaml:"enabled"`
	WindowSize        int     `yaml:"window_size"`
	FailureThreshold  float64 `yaml:"failure_threshold"`
	DegradedFloor     int     `yaml:"degraded_floor"`
	RecoverySuccesses int     `yaml:"recovery_successes"`
}
```

Add to `RoutingConfig` struct (line 294):

```go
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestConfig_CircuitBreaker -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "add circuit_breaker config to routing section"
```

---

### Task 8: Implement CircuitBreaker

**Files:**
- Create: `internal/provider/circuit_breaker.go`
- Test: `internal/provider/circuit_breaker_test.go`

**Step 1: Write the failing tests**

```go
func TestCircuitBreaker_StartsHealthy(t *testing.T) {
	cb := provider.NewCircuitBreaker(10, 0.3, 20, 5)
	if cb.IsDegraded("codex") {
		t.Error("new circuit breaker should not be degraded")
	}
	if ratio := cb.EffectiveRatio("codex", 60); ratio != 60 {
		t.Errorf("effective ratio = %d, want 60", ratio)
	}
}

func TestCircuitBreaker_DegradedAfterThreshold(t *testing.T) {
	cb := provider.NewCircuitBreaker(10, 0.3, 20, 5)
	// Record 7 successes and 3 transport_disconnect failures
	for i := 0; i < 7; i++ {
		cb.RecordOutcome("codex", "")
	}
	for i := 0; i < 3; i++ {
		cb.RecordOutcome("codex", "transport_disconnect")
	}
	if !cb.IsDegraded("codex") {
		t.Error("should be degraded at 30% transport failure rate")
	}
	if ratio := cb.EffectiveRatio("codex", 60); ratio != 20 {
		t.Errorf("degraded effective ratio = %d, want 20 (floor)", ratio)
	}
}

func TestCircuitBreaker_RecoveryAfterConsecutiveSuccesses(t *testing.T) {
	cb := provider.NewCircuitBreaker(10, 0.3, 20, 5)
	// Degrade it
	for i := 0; i < 7; i++ {
		cb.RecordOutcome("codex", "")
	}
	for i := 0; i < 3; i++ {
		cb.RecordOutcome("codex", "transport_disconnect")
	}
	// 5 consecutive successes should recover
	for i := 0; i < 5; i++ {
		cb.RecordOutcome("codex", "")
	}
	if cb.IsDegraded("codex") {
		t.Error("should have recovered after 5 consecutive successes")
	}
	if ratio := cb.EffectiveRatio("codex", 60); ratio != 60 {
		t.Errorf("recovered effective ratio = %d, want 60", ratio)
	}
}

func TestCircuitBreaker_SlidingWindowDropsOldEntries(t *testing.T) {
	cb := provider.NewCircuitBreaker(5, 0.3, 20, 3)
	// Fill window with 2 failures + 3 successes (40% failure -> degraded)
	cb.RecordOutcome("codex", "transport_disconnect")
	cb.RecordOutcome("codex", "transport_disconnect")
	cb.RecordOutcome("codex", "")
	cb.RecordOutcome("codex", "")
	cb.RecordOutcome("codex", "")
	// Window is now [fail, fail, ok, ok, ok] = 40% -> degraded
	if !cb.IsDegraded("codex") {
		t.Error("should be degraded at 40%")
	}
	// Add 2 more successes, pushing out the 2 failures
	cb.RecordOutcome("codex", "")
	cb.RecordOutcome("codex", "")
	// Window is now [ok, ok, ok, ok, ok] = 0% -> recovered via window
	if cb.IsDegraded("codex") {
		t.Error("should have recovered as failures slid out of window")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider/ -run TestCircuitBreaker -v`
Expected: FAIL — NewCircuitBreaker undefined

**Step 3: Implement CircuitBreaker**

Create `internal/provider/circuit_breaker.go`:

```go
package provider

// CircuitBreaker tracks per-provider transport failure rates using a sliding
// window and temporarily reduces a provider's effective ratio when failures
// exceed a threshold.
type CircuitBreaker struct {
	windowSize        int
	failureThreshold  float64
	degradedFloor     int
	recoverySuccesses int
	windows           map[string]*slidingWindow
}

type slidingWindow struct {
	outcomes           []bool // true = transport failure, false = success/other
	consecutiveSuccess int
	degraded           bool
}

func NewCircuitBreaker(windowSize int, failureThreshold float64, degradedFloor int, recoverySuccesses int) *CircuitBreaker {
	if windowSize <= 0 {
		windowSize = 10
	}
	if failureThreshold <= 0 {
		failureThreshold = 0.3
	}
	if degradedFloor <= 0 {
		degradedFloor = 20
	}
	if recoverySuccesses <= 0 {
		recoverySuccesses = 5
	}
	return &CircuitBreaker{
		windowSize:        windowSize,
		failureThreshold:  failureThreshold,
		degradedFloor:     degradedFloor,
		recoverySuccesses: recoverySuccesses,
		windows:           make(map[string]*slidingWindow),
	}
}

func (cb *CircuitBreaker) RecordOutcome(providerName string, failureCategory string) {
	if cb == nil {
		return
	}
	w := cb.getOrCreate(providerName)
	isTransportFailure := failureCategory == FailureCategoryTransportDisconnect

	// Append to sliding window
	w.outcomes = append(w.outcomes, isTransportFailure)
	if len(w.outcomes) > cb.windowSize {
		w.outcomes = w.outcomes[len(w.outcomes)-cb.windowSize:]
	}

	// Track consecutive successes
	if isTransportFailure {
		w.consecutiveSuccess = 0
	} else {
		w.consecutiveSuccess++
	}

	// Check for recovery
	if w.degraded && w.consecutiveSuccess >= cb.recoverySuccesses {
		w.degraded = false
		return
	}

	// Check for degradation
	if !w.degraded && len(w.outcomes) >= cb.windowSize {
		failures := 0
		for _, f := range w.outcomes {
			if f {
				failures++
			}
		}
		rate := float64(failures) / float64(len(w.outcomes))
		if rate >= cb.failureThreshold {
			w.degraded = true
		}
	}
}

func (cb *CircuitBreaker) IsDegraded(providerName string) bool {
	if cb == nil {
		return false
	}
	w, ok := cb.windows[providerName]
	if !ok {
		return false
	}
	return w.degraded
}

func (cb *CircuitBreaker) EffectiveRatio(providerName string, configuredRatio int) int {
	if cb == nil || !cb.IsDegraded(providerName) {
		return configuredRatio
	}
	if configuredRatio < cb.degradedFloor {
		return configuredRatio // already below floor
	}
	return cb.degradedFloor
}

func (cb *CircuitBreaker) getOrCreate(providerName string) *slidingWindow {
	w, ok := cb.windows[providerName]
	if !ok {
		w = &slidingWindow{
			outcomes: make([]bool, 0, cb.windowSize),
		}
		cb.windows[providerName] = w
	}
	return w
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/provider/ -run TestCircuitBreaker -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/provider/circuit_breaker.go internal/provider/circuit_breaker_test.go
git commit -m "implement sliding-window circuit breaker for provider transport failures"
```

---

### Task 9: Wire CircuitBreaker into Router

**Files:**
- Modify: `internal/provider/router.go:8-16` (Router struct)
- Modify: `internal/provider/router.go:121-157` (selectByRatio)
- Test: `internal/provider/router_test.go`

**Step 1: Write the failing test**

```go
func TestRouter_CircuitBreakerReducesRatio(t *testing.T) {
	// Create router with 2 providers, 60/40 ratio, and a circuit breaker
	// Record enough transport failures on codex to trigger degradation
	// Assert that Select() for an "any" phase now favors claude
}

func TestRouter_CircuitBreakerRecovery(t *testing.T) {
	// Degrade codex, then record enough successes to recover
	// Assert that Select() returns to normal ratio behavior
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/provider/ -run TestRouter_CircuitBreaker -v`
Expected: FAIL — Router has no circuit breaker field

**Step 3: Wire the circuit breaker**

Add `circuitBreaker *CircuitBreaker` field to `Router` struct (line 15).

Update `NewRouter` to accept an optional `*CircuitBreaker` parameter (or add a `WithCircuitBreaker` method).

Update `selectByRatio()` (line 121) to use `cb.EffectiveRatio(name, targetRatio)` instead of raw `targetRatio` when computing the gap.

Add a `RecordOutcome(providerName, failureCategory string)` method on Router that delegates to the circuit breaker.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/provider/ -run TestRouter_CircuitBreaker -v`
Expected: PASS

**Step 5: Run full provider test suite**

Run: `go test ./internal/provider/ -count=1`
Expected: All pass (backward compat — nil circuit breaker means no-op)

**Step 6: Commit**

```bash
git add internal/provider/router.go internal/provider/router_test.go
git commit -m "wire circuit breaker into router ratio selection"
```

---

### Task 10: Wire CircuitBreaker Construction from Config

**Files:**
- Modify: `internal/runner/constructor.go` or `internal/runner/run_init.go` (where Router is built from config)
- Modify: `internal/runner/callbacks.go` (record outcome after each invocation)
- Test: `internal/runner/wire_router_from_config_test.go`

**Step 1: Write the failing test**

```go
func TestNewRunner_WithCircuitBreakerConfig(t *testing.T) {
	// Create config with routing.circuit_breaker.enabled = true
	// Create runner
	// Assert the router has a non-nil circuit breaker
}
```

**Step 2: Run test to verify it fails**

**Step 3: Wire circuit breaker construction**

In the `HasProviders()` branch of the runner constructor, after building the router:
- If `cfg.Routing.CircuitBreaker.Enabled`, create a `NewCircuitBreaker(...)` and attach to the router.

In `callbacks.go`, after each invocation completes (where `result` is available), call:
```go
r.router.RecordOutcome(p.Name(), result.FailureCategory)
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/runner/ -run TestNewRunner_WithCircuitBreakerConfig -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./... -count=1 -p 4`
Expected: All pass

**Step 6: Commit**

```bash
git add <modified files>
git commit -m "wire circuit breaker construction from routing config"
```

---

### Task 11: Add Provider Metrics to `gromit stats` Output

**Files:**
- Modify: `cmd/gromit/stats.go`
- Test: `cmd/gromit/stats_test.go`

**Step 1: Update stats command to read and display provider metrics**

The `gromit stats` command currently reads `ModelStats` and `GlobalStats`. Add a path to read `ProcessTrend` and display `ProviderMetrics`.

In `runStats()`, add after project stats:
```go
	// Read process trend for provider metrics
	metricsDir := filepath.Join(gromitDir, "metrics")
	trend, err := logger.ReadProcessTrend(metricsDir)
	if err == nil && len(trend.ProviderMetrics) > 0 {
		// Display provider metrics
	}
```

For text output, add a `printProviderMetrics(metrics []logger.ProviderMetrics)` function:
```
Provider Reliability (last 30 iterations):
  codex   80% success  (16/20)  transport_fail=10%  avg 542ms
  claude  90% success  (9/10)   transport_fail=0%   avg 650ms
```

For JSON output, add `"provider_metrics"` to the output map.

**Step 2: Write a test for the display**

**Step 3: Run tests**

Run: `go test ./cmd/gromit/ -run TestStats -v`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/gromit/stats.go cmd/gromit/stats_test.go
git commit -m "display per-provider reliability metrics in gromit stats"
```

---

### Task 12: Final Integration Verification

**Step 1: Run full test suite**

Run: `go test ./... -count=1 -p 4`
Expected: All pass

**Step 2: Run linter**

Run: `go vet ./...`
Expected: No issues

**Step 3: Build**

Run: `go build ./...`
Expected: Clean build

**Step 4: Manual smoke test with current gromit.yaml**

Run: `gromit stats`
Expected: Shows provider metrics section (may be empty if no recent runs with the new fields)

**Step 5: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "routing ratio reliability review: final cleanup"
```
