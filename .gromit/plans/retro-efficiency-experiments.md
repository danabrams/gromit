---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T23:32:15-05:00"
id: retro-efficiency-experiments
source_spec: retro-efficiency-experiments
---

# Retro Efficiency Analysis & Experiments Implementation Plan

**Goal:** Extend the retro system to analyze execution efficiency (cost/time/tokens) and introduce a structured experiment loop for continuous process improvement.

**Architecture:** Extend the data pipeline from Claude stream events through iteration logs to retro analysis — capturing cost/tokens at the stream level, surfacing them through the runner, adding efficiency aggregation over JSONL files in Go, and building experiment lifecycle management on top of the retro system.

**Tech Stack:** Go, Go text/template, JSON/JSONL file I/O

**Spec:** `.gromit/specs/retro-efficiency-experiments.md`

---

## Architecture

### Data Flow (New Metrics)

```
Claude CLI result event (total_cost_usd, input_tokens, output_tokens)
  → StreamEvent (new fields) → ParseAndLogEvent → StreamStats (new fields)
  → executeClaudeInvocation returns StreamStats
  → IterationResult (new fields) → writeIterationLog → IterationLog (new fields)
  → JSONL files on disk
  → ReadEfficiencyReport() computes aggregates in Go
  → TemplateContext.Efficiency → retro prompt (summary tables + deltas)
```

### Key Design Decisions

- **Aggregate in Go, summarize in prompt**: Historical data is reduced to averages/deltas in Go code. Only the current run's per-iteration table and pre-computed summaries go into the retro prompt, keeping context window usage bounded.
- **StreamStats returned from executeClaudeInvocation**: Avoids coupling the claude package to logging concerns. The runner bridges stream data to iteration logs.
- **Efficiency aggregation in logger package**: Pure data aggregation over JSONL files — no retro-specific logic.
- **Experiment management in retro package**: Experiments are tightly coupled to the retro workflow.

### Files Modified

- `internal/logger/stream.go` — Add token fields to StreamEvent, cost/token fields to StreamStats, capture in ParseAndLogEvent
- `internal/logger/logger.go` — Add cost/token fields to IterationLog
- `internal/runner/process.go` — Return StreamStats from executeClaudeInvocation
- `internal/runner/runner.go` — Add fields to IterationResult, populate in writeIterationLog
- `internal/retro/retro.go` — Extend TemplateContext, wire data loading, update LaunchClaudeCode
- `.gromit/templates/PROMPT_retro.md` — Add efficiency and experiment sections

### Files Created

- `internal/logger/efficiency.go` — Efficiency aggregation structs and functions
- `internal/logger/efficiency_test.go` — Tests for efficiency aggregation
- `internal/retro/experiment.go` — Experiment struct, Load/Save/Delete, baseline metrics
- `internal/retro/experiment_test.go` — Tests for experiment management

---

## Test Strategy

### Unit Tests

1. **StreamStats cost/token capture** (`stream_test.go`): ParseAndLogEvent with result event JSON populates StreamStats; non-result events leave fields zero; backward compat with events missing token fields.

2. **IterationLog serialization** (`logger_test.go`): New fields round-trip through JSONL; old-format logs deserialize with zero values.

3. **Efficiency aggregation** (`efficiency_test.go`): Per-iteration stats, per-model aggregates, historical deltas, trend detection, context window flags, empty/zero-cost edge cases.

4. **Experiment management** (`experiment_test.go`): Save/Load/Delete lifecycle, missing file returns nil, baseline metrics computation.

### Mocking Strategy

- Efficiency and experiment tests use temp directories with crafted files
- Runner changes rely on existing mock interfaces — no new mocks needed

### Coverage Goals

- Critical: cost/token capture from stream events, aggregation math, experiment file lifecycle
- Edge cases: old-format logs, empty runs, missing experiment file

---

## Implementation Tasks

### Task 1: Add cost/token fields to StreamEvent and StreamStats

**Files:**
- Modify: `internal/logger/stream.go`
- Test: `internal/logger/stream_test.go`

**What to Do:**
Add `InputTokens int` and `OutputTokens int` fields to `StreamEvent` (json tags: `input_tokens`, `output_tokens`). Add `TotalCost float64`, `InputTokens int`, `OutputTokens int` fields to `StreamStats`. In `ParseAndLogEvent`, when processing a `result` event, copy these values from the parsed `StreamEvent` into `StreamStats` (guarded by the existing mutex). Add a public accessor method on StreamStats to retrieve cost/token data thread-safely.

**Acceptance Criteria:**
- StreamEvent has InputTokens and OutputTokens fields that deserialize from Claude's result event JSON
- StreamStats captures TotalCost, InputTokens, OutputTokens from result events via ParseAndLogEvent
- Unit tests verify capture from a result event and zero values from non-result events

**Dependencies:** None

**Notes:**
Claude CLI result events include `total_cost_usd` (already in StreamEvent), plus token count fields. Check the actual JSON field names from Claude CLI output — they may be `input_tokens_used` / `output_tokens_used` or similar. The stream log file already logs cost; token data should be logged there too for debugging.

---

### Task 2: Add cost/token fields to IterationLog and IterationResult

**Files:**
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/runner.go` (IterationResult struct only)

**What to Do:**
Add `CostUSD float64` (json: `cost_usd`), `InputTokens int` (json: `input_tokens`), `OutputTokens int` (json: `output_tokens`) to `IterationLog`. Add matching fields to `IterationResult` in runner.go. No behavioral changes — just struct field additions.

**Acceptance Criteria:**
- IterationLog serializes/deserializes cost and token fields in JSONL
- Old-format JSONL entries (without these fields) deserialize with zero values, no errors
- IterationResult has matching fields for the runner to populate

**Dependencies:** None

---

### Task 3: Surface StreamStats from runner and populate iteration logs

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/runner.go`

**What to Do:**
Change `executeClaudeInvocation` to return `*logger.StreamStats` as an additional return value. Update all callers in the execution chain (executeWithRetry, processBead) to thread StreamStats through. In the code that builds `IterationResult` after a Claude invocation, populate CostUSD, InputTokens, OutputTokens from StreamStats. In `writeIterationLog`, copy these fields from IterationResult to IterationLog.

**Acceptance Criteria:**
- executeClaudeInvocation returns StreamStats alongside existing returns
- IterationResult is populated with cost/token data from StreamStats after each invocation
- writeIterationLog writes cost/token data to the JSONL log

**Dependencies:** Task 1, Task 2

**Notes:**
The existing mock interfaces (`ClaudeClient`) in tests won't be affected since StreamStats is created inside executeClaudeInvocation, not passed through the ClaudeClient interface. Update runner tests that call writeIterationLog to verify new fields are written.

---

### Task 4: Implement efficiency aggregation

**Files:**
- Create: `internal/logger/efficiency.go`
- Create: `internal/logger/efficiency_test.go`

**What to Do:**
Define structs:
- `IterationEfficiency`: BeadID, BeadTitle, Model, DurationMs, CostUSD, InputTokens, OutputTokens
- `ModelEfficiency`: Model, IterationCount, AvgCostUSD, AvgDurationMs, AvgInputTokens, AvgOutputTokens
- `EfficiencyReport`: CurrentRun ([]IterationEfficiency + []ModelEfficiency), Historical ([]ModelEfficiency), Deltas (per metric: value, direction), ContextWindowFlags ([]string for iterations exceeding 80% of model context)

Implement `ReadEfficiencyReport(logsDir string, currentRunID string) (*EfficiencyReport, error)`:
- Read current run's JSONL → populate per-iteration table and per-model aggregates
- Read all other runs' JSONL → compute historical per-model aggregates
- Compute deltas between current and historical
- Detect trend direction (improving/stable/degrading) based on delta magnitude
- Flag iterations where input tokens > 80% of model context window (use known context sizes: opus=200k, sonnet=200k, haiku=200k)

**Acceptance Criteria:**
- Per-iteration and per-model aggregates computed correctly from JSONL files
- Historical comparison produces correct deltas and trend directions
- Empty logs and old-format logs (zero cost/tokens) handled gracefully without errors

**Dependencies:** Task 2 (needs IterationLog with new fields)

---

### Task 5: Implement experiment management

**Files:**
- Create: `internal/retro/experiment.go`
- Create: `internal/retro/experiment_test.go`

**What to Do:**
Define `Experiment` struct matching the spec's JSON format:
```go
type Experiment struct {
    Name            string          `json:"name"`
    Hypothesis      string          `json:"hypothesis"`
    Change          string          `json:"change"`
    Measurement     string          `json:"measurement"`
    Risk            string          `json:"risk"`
    StartedAt       time.Time       `json:"started_at"`
    BaselineMetrics BaselineMetrics `json:"baseline_metrics"`
}

type BaselineMetrics struct {
    AvgCostPerBead    float64 `json:"avg_cost_per_bead"`
    AvgDurationMs     float64 `json:"avg_duration_ms"`
    AvgInputTokens    float64 `json:"avg_input_tokens"`
    AvgOutputTokens   float64 `json:"avg_output_tokens"`
    FailureRate       float64 `json:"failure_rate"`
}
```

Implement:
- `LoadExperiment(gromitDir string) (*Experiment, error)` — returns nil, nil if file doesn't exist
- `SaveExperiment(gromitDir string, exp *Experiment) error` — writes to `.gromit/experiment.json`
- `DeleteExperiment(gromitDir string) error` — removes the file
- `ComputeBaselineMetrics(logsDir string) (*BaselineMetrics, error)` — aggregates from all JSONL logs

**Acceptance Criteria:**
- Experiment round-trips through Save/Load with correct JSON structure
- LoadExperiment returns nil (not error) when no experiment file exists
- DeleteExperiment removes the file; no error if already absent

**Dependencies:** None

---

### Task 6: Update retro prompt template

**Files:**
- Modify: `.gromit/templates/PROMPT_retro.md`

**What to Do:**
Add three new sections to the template after the existing Run Statistics section:

1. **Current Run Efficiency** (conditional on `.Efficiency`):
   - Per-iteration table: Bead ID | Model | Duration | Cost | Input Tokens | Output Tokens
   - Per-model aggregates: avg cost, avg duration, avg tokens
   - Context window utilization flags

2. **Historical Comparison** (conditional on `.Efficiency.HasHistorical`):
   - Per-model deltas: "Cost per bead: $X this run vs $Y historical (±Z%)"
   - Trend indicators

3. **Active Experiment Evaluation** (conditional on `.Experiment`):
   - Experiment details (name, hypothesis, change, measurement, risk)
   - Baseline metrics vs current metrics
   - Prompt to analyze whether the experiment succeeded

Update the Task section to add item 6: Efficiency Analysis with Five Whys guidance.

Add experiment recommendations instruction: "Generate 2-4 experiment recommendations with name, hypothesis, change, measurement, and risk."

**Acceptance Criteria:**
- Template renders efficiency table when data is present, omits when absent
- Template renders experiment evaluation when active experiment exists
- Template includes experiment recommendation instructions in the task section

**Dependencies:** Task 4 (struct names for template fields), Task 5 (experiment struct for template fields)

---

### Task 7: Wire efficiency and experiment data into retro.Run()

**Files:**
- Modify: `internal/retro/retro.go`

**What to Do:**
Extend `TemplateContext` with:
- `Efficiency *logger.EfficiencyReport`
- `Experiment *Experiment`

In `Run()`, after reading RunStats and BeadStats:
- Determine the current run ID (from the logger or by finding the most recent run file)
- Call `logger.ReadEfficiencyReport(logsDir, currentRunID)` to get efficiency data
- Call `LoadExperiment(gromitDir)` to check for an active experiment
- Pass both to TemplateContext

**Acceptance Criteria:**
- TemplateContext includes efficiency data and active experiment when available
- Retro prompt renders with efficiency tables and experiment section
- Retro works correctly when no efficiency data exists (old logs) or no experiment is active

**Dependencies:** Task 4, Task 5, Task 6

**Notes:**
The runner's Logger needs to expose its current run ID so the retro can distinguish "current run" from "historical runs". This may require adding a `RunID() string` method to the Logger or passing the run ID through another channel. Check how the logger generates run file names (timestamp-based) and whether the retro can identify the latest one.

---

### Task 8: Update LaunchClaudeCode for experiment management

**Files:**
- Modify: `internal/retro/retro.go`

**What to Do:**
Update `LaunchClaudeCode` to accept efficiency analysis and experiment data alongside the existing analysis text. Build a prompt that includes:
- The standard retro analysis (existing)
- Efficiency analysis summary
- Experiment recommendations from the retro
- If active experiment: evaluation results + keep/revert/extend decision prompt
- Guidance to use Five Whys for efficiency anomalies
- Constraint: pick at most one new experiment
- Instructions for saving selected experiment to `.gromit/experiment.json`

Update the function signature to accept the additional data (or bundle it into a struct).

**Acceptance Criteria:**
- Interactive review session receives efficiency and experiment context
- When an active experiment exists, the session includes evaluation and keep/revert/extend options
- The prompt instructs selection of at most one new experiment

**Dependencies:** Task 5, Task 7

---

## Notes

- **Backward compatibility**: Old JSONL logs without cost/token fields will deserialize with zero values. Efficiency aggregation should handle this gracefully (skip zero-cost entries from averages, or note "no cost data available for N iterations").
- **Claude CLI token field names**: Need to verify the exact JSON field names for input/output tokens in Claude CLI's stream-json result events. The spec says they exist but aren't parsed yet.
- **Run ID identification**: The retro needs to know which JSONL file is the "current run" vs historical. The logger creates files named `run-YYYYMMDD-HHMMSS.jsonl`. The retro could receive the run ID from the runner, or identify the most recent file by timestamp. Consider adding a `RunID() string` accessor to Logger.
- **Model context window sizes**: Hard-code known sizes (opus=200k, sonnet=200k, haiku=200k) for the 80% utilization flag. These could live as constants in the efficiency package.
