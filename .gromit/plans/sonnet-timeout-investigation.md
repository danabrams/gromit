---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T08:05:15-05:00"
id: sonnet-timeout-investigation
source_spec: sonnet-timeout-investigation
---

# Sonnet Timeout Investigation Implementation Plan

**Goal:** Quantify sonnet timeout root causes, add missing diagnostic data for ongoing monitoring, and implement preemptive escalation to reduce preventable timeout failures.

**Architecture:** Build a log analysis command to extract insights from existing JSONL data, extend StreamStats with rate limit recovery timing, and add scope-based preemptive model escalation before first invocation. Per-model timeout configuration already exists and just needs default values and documentation.

**Tech Stack:** Go, cobra (CLI), existing JSONL logging infrastructure

**Spec:** `.gromit/specs/sonnet-timeout-investigation.md`

---

## Architecture

**Overview:**
The investigation delivers four components: (1) a `gromit analyze-timeouts` command that parses JSONL logs to quantify timeout root causes, (2) enhanced stream diagnostics for ongoing monitoring, (3) verified per-model timeout defaults, and (4) scope-based preemptive model escalation.

**Key Finding — Existing Infrastructure:**
Much of what the spec describes as "missing" is already implemented:
- `StreamStats` tracks: `StallCount`, `StallTier`, `RateLimitHits`, `TimeToFirstEvent`, `ToolCalls`
- `IterationLog` captures all diagnostic fields in JSONL
- `TimeoutType` classifies as `"stall"`, `"bead"`, or `"invocation"`
- `ModelTimeoutOverrides` struct and `TimeoutsForModel()` method exist with full test coverage
- Rate limit detection via `isRateLimitEvent()` works

**Key Components:**

1. **`gromit analyze-timeouts` command** (`cmd/gromit/analyze_timeouts.go`): Parses JSONL log files to produce timeout statistics — breakdown by model, timeout type, rate limit correlation, bead characteristics.

2. **Log analysis engine** (`internal/logger/analyze.go`): Reads all JSONL logs and computes aggregate timeout statistics: per-model breakdown, timeout type distribution, rate limit correlation, time-to-first-event analysis, tool call counts at timeout.

3. **Enhanced stream diagnostics** (`internal/logger/stream.go`): Add `LastRateLimitTime` and `RateLimitRecoveryMs` to StreamStats for tracking rate limit recovery duration.

4. **Scope-based preemptive escalation** (`internal/runner/process.go`): When scope check returns `complexity: "high"` for a P1 bead, escalate from sonnet to opus before the first invocation rather than waiting for a timeout-then-escalate cycle.

5. **Per-model timeout defaults** (`gromit.yaml`): Document the existing `model_timeouts` config with recommended sonnet-specific values.

**Integration Points:**
- Log analysis reads from `.gromit/logs/*.jsonl` using existing `readLogFile()` in `internal/logger/logger.go`
- Enhanced diagnostics extend `StreamStats` (existing struct in `internal/logger/stream.go`)
- Preemptive escalation hooks into `setupBeadContext()` in `internal/runner/process.go` (line 82) where model is selected
- Per-model timeouts use existing `TimeoutsForModel()` path (config.go:433-456, fully tested)

**Files to Modify:**
- `internal/logger/stream.go` — Add rate limit recovery timing fields and methods
- `internal/logger/logger.go` — Add `RateLimitRecoveryMs` field to `IterationLog`
- `internal/runner/process.go` — Add scope-based model escalation in `setupBeadContext()`
- `internal/runner/runner.go` — Wire `IterationResult.RateLimitRecoveryMs` into log writing

**Files to Create:**
- `cmd/gromit/analyze_timeouts.go` — New subcommand for log analysis
- `internal/logger/analyze.go` — Log parsing and timeout aggregation logic
- `internal/logger/analyze_test.go` — Tests for analysis engine

**Tradeoffs:**
- **Separate command vs inline reporting**: Chose `analyze-timeouts` over adding to `gromit status` because analysis is ad-hoc investigation, not needed every run
- **Preemptive escalation vs longer timeouts**: Chose escalation because longer timeouts burn more money/time on work that will likely fail
- **Extend existing structs vs new types**: Chose extending `StreamStats` to avoid fragmenting the diagnostic data path

---

## Test Strategy

**Test Levels:**

1. **Unit Tests**: Log analysis aggregation, rate limit recovery timing, escalation decisions
2. **Integration Tests**: End-to-end log parsing with realistic JSONL, per-model timeout resolution (already covered)
3. **Manual Testing**: Run `gromit analyze-timeouts` against real project logs

**Key Test Cases:**

- Log analysis correctly aggregates timeout counts by model, type, and rate limit correlation from multi-line JSONL
- Empty/missing log directories handled gracefully (no crash, zero counts)
- Malformed JSON lines in log files are skipped without breaking analysis
- Rate limit recovery timing: `StreamStats` correctly measures time between rate limit hit and next successful event
- Preemptive escalation: P1 bead with `complexity: "high"` scope estimate escalates to opus
- Escalation skip: P0 beads (already opus), beads without scope estimates, and disabled scope check are not affected
- DiagnosticSnapshot returns new rate limit recovery field correctly

**Mocking Strategy:**
- Use `t.TempDir()` with synthetic JSONL files for log analysis tests
- Use real `StreamStats` instances for diagnostic tests (pure data, no mocking needed)
- Existing `selectModel` tests cover model selection; escalation tests mock scope estimate

**Coverage Goals:**
- All timeout type classifications in analysis output
- All edge cases: zero-tool-call timeouts, rate limits with no recovery, single-iteration logs, logs with only successes
- Escalation decision logic: all priority levels, with/without scope check, with/without complexity label

**Test Organization:**
- `internal/logger/analyze_test.go` — Log analysis unit tests
- `internal/logger/stream_test.go` — Extended with rate limit recovery tests
- `internal/runner/process_test.go` — Preemptive escalation tests

---

## Implementation Tasks

### Task 1: Build log analysis engine

**Files:**
- Create: `internal/logger/analyze.go`
- Create: `internal/logger/analyze_test.go`

**What to Do:**
Implement a `TimeoutAnalysis` struct and `AnalyzeTimeouts(logsDir string) (*TimeoutAnalysis, error)` function that reads all JSONL log files using the existing `readLogFile()` helper. Aggregate statistics into per-model breakdowns:
- Total iterations, successes, failures, timeouts by type (stall/bead/invocation)
- Rate limit hit counts correlated with timeout outcomes
- Average time-to-first-event for timed-out vs successful iterations
- Average tool call count at timeout vs completion
- Per-model summary (sonnet vs opus vs haiku)

The `TimeoutAnalysis` struct should include a `ModelStats` map keyed by model name, each containing counts and averages. Include a `Summary()` method that returns a formatted string for CLI output.

**Acceptance Criteria:**
- `AnalyzeTimeouts` parses all `run-*.jsonl` files in the logs directory and returns correct aggregate counts
- Returns zero-value analysis (no error) when logs directory is empty or missing
- Malformed JSON lines are skipped without error

**Dependencies:** None

### Task 2: Add `gromit analyze-timeouts` command

**Files:**
- Create: `cmd/gromit/analyze_timeouts.go`

**What to Do:**
Add a cobra subcommand `analyze-timeouts` following the pattern in `cmd/gromit/debug.go`. The command loads config, resolves the logs directory, calls `logger.AnalyzeTimeouts()`, and prints the summary to stdout. Add an optional `--json` flag for machine-readable output. Register the command in `init()` with `rootCmd.AddCommand()`.

**Acceptance Criteria:**
- `gromit analyze-timeouts` prints human-readable timeout breakdown by model and type
- `gromit analyze-timeouts --json` outputs the analysis struct as JSON
- Handles missing config or logs directory gracefully with informative message

**Dependencies:** Task 1

### Task 3: Add rate limit recovery timing to StreamStats

**Files:**
- Modify: `internal/logger/stream.go`
- Modify: `internal/logger/stream_test.go` (if exists, otherwise test in analyze_test.go)

**What to Do:**
Add `LastRateLimitTime time.Time` and `RateLimitRecoveryMs int64` fields to `StreamStats`. Update `RecordRateLimitHit()` to set `LastRateLimitTime = time.Now()`. Update `RecordEvent()` to compute recovery time: when `LastRateLimitTime` is non-zero and a new event arrives, set `RateLimitRecoveryMs` to the elapsed milliseconds and clear `LastRateLimitTime`. Update `DiagnosticSnapshot()` to return the new field.

**Acceptance Criteria:**
- `RateLimitRecoveryMs` correctly measures time between rate limit hit and next successful stream event
- `DiagnosticSnapshot()` returns the recovery time
- Multiple rate limit hits record the most recent recovery time

**Dependencies:** None

### Task 4: Wire rate limit recovery into iteration logging

**Files:**
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/runner.go`

**What to Do:**
Add `RateLimitRecoveryMs int64` field to `IterationLog` (json tag: `rate_limit_recovery_ms,omitempty`). Add matching field to `IterationResult`. In `executeClaudeInvocation()` (process.go), capture the new field from `DiagnosticSnapshot()` into `bc.result.RateLimitRecoveryMs`. In `writeIterationLog()` (runner.go), propagate the field to the log entry.

**Acceptance Criteria:**
- JSONL log entries include `rate_limit_recovery_ms` when rate limiting was detected
- Field is omitted (omitempty) when no rate limiting occurred
- DiagnosticSnapshot signature updated to return the additional value

**Dependencies:** Task 3

### Task 5: Add scope-based preemptive model escalation

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/process_test.go` (or existing test file)

**What to Do:**
In `setupBeadContext()` (process.go:82), after `model := r.selectModel(b)`, add a check: if `scopeEstimate != nil && scopeEstimate.Complexity == "high" && model == "sonnet"`, escalate to opus. Log the escalation: `r.log("Preemptive escalation: %s → opus (scope complexity: high)", model)`. Set `bc.result.Escalated = true` and `bc.result.EscalatedTo = "opus"`. Only apply when the scope check is enabled (`r.cfg.ScopeCheck.Enabled`).

**Acceptance Criteria:**
- P1 bead with high-complexity scope estimate uses opus instead of sonnet
- P0 beads (already opus) are not affected
- When scope check is disabled or scope estimate is nil, no escalation occurs

**Dependencies:** None

### Task 6: Add recommended per-model timeout defaults to gromit.yaml

**Files:**
- Modify: `gromit.yaml` (config reference)

**What to Do:**
Add a commented-out `model_timeouts` section to the `claude:` block in `gromit.yaml` showing recommended sonnet-specific values. Include comments explaining the rationale:
- Sonnet stall timeout: 90s (shorter than default 120s — sonnet responds faster when not rate-limited)
- Sonnet stall timeout active: 180s (shorter than default 300s — same rationale)
- Opus stall timeout active: 420s (longer than default — opus legitimately takes longer on complex reasoning)

**Acceptance Criteria:**
- `gromit.yaml` includes commented `model_timeouts` section with sonnet and opus entries
- Each value has a comment explaining why it differs from the default

**Dependencies:** None

---

## Notes

- The per-model timeout system (`ModelTimeoutOverrides`, `TimeoutsForModel()`) is already fully implemented and tested (6 test cases in config_test.go). No code changes needed — just documentation and recommended defaults.
- The `readLogFile()` function in logger.go is unexported but lives in the same package, so `analyze.go` can call it directly.
- The preemptive escalation in Task 5 is conservative — it only fires for P1 beads where sonnet was selected by priority AND the scope check flagged high complexity. Label overrides (e.g., `complexity:low`) take precedence because `selectModel` processes them first.
- Task 1 (analysis engine) is the most important deliverable for answering the spec's key questions. Running it against real logs will determine whether further tuning is needed or if the investigation can be closed with data.
