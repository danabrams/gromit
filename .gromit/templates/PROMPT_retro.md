# Retrospective Analysis

You are analyzing accumulated learnings from gromit iterations to identify patterns, consolidate knowledge, and recommend updates to project rules.

## Current Rules

{{.Rules}}

## Current Learnings

{{.Learnings}}

## Run Statistics

{{- if .RunStats.Total }}
### Aggregate Statistics
- **Total iterations**: {{ .RunStats.Total }}
- **Succeeded**: {{ .RunStats.Succeeded }}
- **Failed**: {{ .RunStats.Failed }}
- **Failure rate**: {{ printf "%.1f%%" (mul .RunStats.FailureRate 100) }}
{{- else }}
*No iteration data available yet.*
{{- end }}

{{- if .BeadStats }}
### Stuck Beads (2+ failures)
| Bead ID | Title | Total Runs | Failures | Failure Rate |
|---------|-------|-----------|----------|--------------|
{{- range $id, $stats := .BeadStats }}
| {{ $stats.BeadID }} | {{ $stats.BeadTitle }} | {{ $stats.TotalRuns }} | {{ $stats.Failures }} | {{ printf "%.1f%%" (mul $stats.FailureRate 100) }} |
{{- end }}
{{- else if .RunStats.Total }}
*No stuck beads identified (fewer than 2 failures each).*
{{- end }}

{{- if .BeadStats }}

### Bead Comments
{{- range $id, $stats := .BeadStats }}
{{- if $stats.Comments }}

**{{ $stats.BeadID }}: {{ $stats.BeadTitle }}**
{{- range $stats.Comments }}
- {{ . }}
{{- end }}
{{- end }}
{{- end }}
{{- end }}

{{- if .Efficiency }}

## Current Run Efficiency

{{- if .Efficiency.CurrentIterations }}

### Per-Iteration Efficiency
| Bead ID | Model | Duration | Cost (USD) | Input Tokens | Output Tokens | Files |
|---------|-------|----------|------------|--------------|---------------|-------|
{{- range .Efficiency.CurrentIterations }}
| {{ .BeadID }} | {{ .Model }} | {{ .Duration }} | ${{ printf "%.4f" .CostUSD }} | {{ .InputTokens }} | {{ .OutputTokens }} | {{ .FilesTouched }} |
{{- end }}

{{- else }}

### Data Quality Warning
**Status: `insufficient_current_run_data`**

Current-run iteration data is empty. All comparative efficiency metrics below are unavailable.

{{- end }}

### Per-Model Aggregates (Current Run)
| Model | Iterations | Avg Cost | Avg Duration | Avg Input Tokens | Avg Output Tokens |
|-------|-----------|----------|--------------|------------------|-------------------|
{{- range $model, $stats := .Efficiency.CurrentModels }}
| {{ $stats.Model }} | {{ $stats.IterationCount }} | ${{ printf "%.4f" $stats.AvgCostUSD }} | {{ $stats.AvgDuration }} | {{ printf "%.0f" $stats.AvgInputTokens }} | {{ printf "%.0f" $stats.AvgOutputTokens }} |
{{- end }}

### Per-Provider-Family Aggregates (Current Run)
| Provider Family | Iterations | Avg Cost | Avg Duration | Avg Input Tokens | Avg Output Tokens |
|----------------|-----------|----------|--------------|------------------|-------------------|
{{- range $family, $stats := .Efficiency.CurrentProviderFamilies }}
| {{ $stats.Model }} | {{ $stats.IterationCount }} | ${{ printf "%.4f" $stats.AvgCostUSD }} | {{ $stats.AvgDuration }} | {{ printf "%.0f" $stats.AvgInputTokens }} | {{ printf "%.0f" $stats.AvgOutputTokens }} |
{{- end }}
{{- if .Efficiency.MixedProviderFamilies }}
| Mixed-provider aggregate | {{ len .Efficiency.CurrentIterations }} | ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }} | {{ .Efficiency.CurrentAvgDurationPerBead }} | {{ printf "%.0f" (avgInputTokens .Efficiency.CurrentIterations) }} | {{ printf "%.0f" (avgOutputTokens .Efficiency.CurrentIterations) }} |
{{- end }}

### Historical Comparison
{{- if and .Efficiency.HistoricalModels .Efficiency.CurrentIterations }}

**Per-Model Aggregates (Historical)**
| Model | Iterations | Avg Cost | Avg Duration | Avg Input Tokens | Avg Output Tokens |
|-------|-----------|----------|--------------|------------------|-------------------|
{{- range $model, $stats := .Efficiency.HistoricalModels }}
| {{ $stats.Model }} | {{ $stats.IterationCount }} | ${{ printf "%.4f" $stats.AvgCostUSD }} | {{ $stats.AvgDuration }} | {{ printf "%.0f" $stats.AvgInputTokens }} | {{ printf "%.0f" $stats.AvgOutputTokens }} |
{{- end }}

**Per-Provider-Family Aggregates (Historical)**
| Provider Family | Iterations | Avg Cost | Avg Duration | Avg Input Tokens | Avg Output Tokens |
|----------------|-----------|----------|--------------|------------------|-------------------|
{{- range $family, $stats := .Efficiency.HistoricalProviderFamilies }}
| {{ $stats.Model }} | {{ $stats.IterationCount }} | ${{ printf "%.4f" $stats.AvgCostUSD }} | {{ $stats.AvgDuration }} | {{ printf "%.0f" $stats.AvgInputTokens }} | {{ printf "%.0f" $stats.AvgOutputTokens }} |
{{- end }}

**Per-Model Deltas (Current vs Historical)**
{{- range $model, $currentStats := .Efficiency.CurrentModels }}
{{- if index $.Efficiency.HistoricalModels $model }}
{{- $historicalStats := index $.Efficiency.HistoricalModels $model }}
{{- $costDelta := sub $currentStats.AvgCostUSD $historicalStats.AvgCostUSD }}
{{- $durationDelta := sub (durationMs $currentStats.AvgDuration) (durationMs $historicalStats.AvgDuration) }}
{{- $inputDelta := sub $currentStats.AvgInputTokens $historicalStats.AvgInputTokens }}
{{- $outputDelta := sub $currentStats.AvgOutputTokens $historicalStats.AvgOutputTokens }}

*{{ $model }}:*
- Cost: ${{ printf "%.4f" $currentStats.AvgCostUSD }} vs ${{ printf "%.4f" $historicalStats.AvgCostUSD }} ({{ if gt $costDelta 0.0 }}↑ +{{ printf "%.1f%%" (mul (div $costDelta $historicalStats.AvgCostUSD) 100) }}{{ else if lt $costDelta 0.0 }}↓ {{ printf "%.1f%%" (mul (div $costDelta $historicalStats.AvgCostUSD) 100) }}{{ else }}→ no change{{ end }})
- Duration: {{ $currentStats.AvgDuration }} vs {{ $historicalStats.AvgDuration }} ({{ if gt $durationDelta 0.0 }}↑ +{{ printf "%.1f%%" (mul (div $durationDelta (durationMs $historicalStats.AvgDuration)) 100) }}{{ else if lt $durationDelta 0.0 }}↓ {{ printf "%.1f%%" (mul (div $durationDelta (durationMs $historicalStats.AvgDuration)) 100) }}{{ else }}→ no change{{ end }})
- Input tokens: {{ printf "%.0f" $currentStats.AvgInputTokens }} vs {{ printf "%.0f" $historicalStats.AvgInputTokens }} ({{ if gt $inputDelta 0.0 }}↑ +{{ printf "%.1f%%" (mul (div $inputDelta $historicalStats.AvgInputTokens) 100) }}{{ else if lt $inputDelta 0.0 }}↓ {{ printf "%.1f%%" (mul (div $inputDelta $historicalStats.AvgInputTokens) 100) }}{{ else }}→ no change{{ end }})
- Output tokens: {{ printf "%.0f" $currentStats.AvgOutputTokens }} vs {{ printf "%.0f" $historicalStats.AvgOutputTokens }} ({{ if gt $outputDelta 0.0 }}↑ +{{ printf "%.1f%%" (mul (div $outputDelta $historicalStats.AvgOutputTokens) 100) }}{{ else if lt $outputDelta 0.0 }}↓ {{ printf "%.1f%%" (mul (div $outputDelta $historicalStats.AvgOutputTokens) 100) }}{{ else }}→ no change{{ end }})
{{- end }}
{{- end }}

{{- if .Efficiency.CurrentIterations }}

{{- if .Efficiency.MixedProviderFamilies }}
**Overall Metrics (Mixed providers: Claude + Codex)**
{{- else }}
**Overall Metrics**
{{- end }}
- Current avg cost per bead: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }}
- Historical avg cost per bead: ${{ printf "%.4f" .Efficiency.HistoricalAvgCostPerBead }}
{{- if and .Efficiency.HistoricalModels (ne .Efficiency.CostDelta 0.0) }}
- Cost delta: ${{ printf "%.4f" .Efficiency.CostDelta }} ({{ if gt .Efficiency.CostDelta 0.0 }}+{{ printf "%.1f%%" (mul (div .Efficiency.CostDelta .Efficiency.HistoricalAvgCostPerBead) 100) }} more expensive{{ else }}{{ printf "%.1f%%" (mul (div .Efficiency.CostDelta .Efficiency.HistoricalAvgCostPerBead) 100) }} cheaper{{ end }})
{{- else if .Efficiency.HistoricalModels }}
- Cost delta: N/A
{{- end }}

- Current avg duration per bead: {{ .Efficiency.CurrentAvgDurationPerBead }}
- Historical avg duration per bead: {{ .Efficiency.HistoricalAvgDurationPerBead }}
{{- if and .Efficiency.HistoricalModels (ne .Efficiency.DurationDelta 0) }}
- Duration delta: {{ .Efficiency.DurationDelta }} ({{ if gt .Efficiency.DurationDelta 0 }}+{{ printf "%.1f%%" (mul (div (durationMs .Efficiency.DurationDelta) (durationMs .Efficiency.HistoricalAvgDurationPerBead)) 100) }} slower{{ else }}{{ printf "%.1f%%" (mul (div (durationMs .Efficiency.DurationDelta) (durationMs .Efficiency.HistoricalAvgDurationPerBead)) 100) }} faster{{ end }})
{{- else if .Efficiency.HistoricalModels }}
- Duration delta: N/A
{{- end }}

{{- else }}

{{- if .Efficiency.HistoricalModels }}
**Overall Metrics**
- Current avg cost per bead: N/A (insufficient current-run data)
- Historical avg cost per bead: ${{ printf "%.4f" .Efficiency.HistoricalAvgCostPerBead }}
- Cost delta: N/A
- Current avg duration per bead: N/A (insufficient current-run data)
- Historical avg duration per bead: {{ .Efficiency.HistoricalAvgDurationPerBead }}
- Duration delta: N/A
{{- end }}

{{- end }}
{{- else }}
*No historical data available for comparison.*
{{- end }}

{{- if and .Efficiency.CurrentIterations .Efficiency.HighContextIterations }}

### Context Window Utilization Flags
The following iterations exceeded 80% of their model's context window:
{{- range .Efficiency.HighContextIterations }}
- **{{ .BeadID }}** ({{ .Model }}): {{ .InputTokens }} tokens ({{ printf "%.1f%%" (mul .ContextWindowUsed 100) }} of context window)
{{- end }}
{{- end }}

{{- end }}

{{- if .ProcessTrend }}

## Continuous Process Trend

- Generated at: {{ .ProcessTrend.GeneratedAt.Format "2006-01-02T15:04:05Z07:00" }}
- Total iterations observed: {{ .ProcessTrend.TotalIterations }}
- Rolling window size: {{ .ProcessTrend.WindowSize }}

### Latest Rolling Window
- Success rate: {{ printf "%.1f%%" (mul .ProcessTrend.LatestWindow.SuccessRate 100) }}
- Failure rate: {{ printf "%.1f%%" (mul .ProcessTrend.LatestWindow.FailureRate 100) }}
- First-pass success rate: {{ printf "%.1f%%" (mul .ProcessTrend.LatestWindow.FirstPassSuccess 100) }}
- Escalation rate: {{ printf "%.1f%%" (mul .ProcessTrend.LatestWindow.EscalationRate 100) }}
- Avg duration: {{ printf "%.0f" .ProcessTrend.LatestWindow.AvgDurationMs }}ms
- P95 duration: {{ printf "%.0f" .ProcessTrend.LatestWindow.P95DurationMs }}ms
- Avg cost: ${{ printf "%.4f" .ProcessTrend.LatestWindow.AvgCostUSD }}
- Avg MTTR proxy: {{ printf "%.0f" .ProcessTrend.LatestWindow.AvgMTTRProxyMs }}ms

### Failure Breakdown
- Preflight failures: {{ printf "%.1f%%" (mul .ProcessTrend.LatestWindow.PreflightFailureRate 100) }}
- Build failures: {{ printf "%.1f%%" (mul .ProcessTrend.LatestWindow.BuildFailureRate 100) }}
- Validation failures: {{ printf "%.1f%%" (mul .ProcessTrend.LatestWindow.ValidationFailureRate 100) }}
- Timeout failures: {{ printf "%.1f%%" (mul .ProcessTrend.LatestWindow.TimeoutFailureRate 100) }}

{{- if .ProcessTrend.ControlLimits }}
### Control Limits
| Metric | Latest | Mean | Std Dev | LCL | UCL |
|--------|--------|------|---------|-----|-----|
{{- range .ProcessTrend.ControlLimits }}
| {{ .Metric }} | {{ printf "%.4f" .Latest }} | {{ printf "%.4f" .Mean }} | {{ printf "%.4f" .StdDev }} | {{ printf "%.4f" .LCL }} | {{ printf "%.4f" .UCL }} |
{{- end }}
{{- end }}

{{- if .ProcessTrend.Anomalies }}
### Out-Of-Control Signals
{{- range .ProcessTrend.Anomalies }}
- **{{ .Metric }}** ({{ .Severity }}): {{ .Message }}
{{- end }}
{{- else }}
*No out-of-control signals detected in latest metrics.*
{{- end }}

{{- if .ProcessTrend.StratifiedAnomalies }}
### Stratified Out-Of-Control Signals
| Stratum | Metric | Severity | Message |
|---------|--------|----------|---------|
{{- range $stratum, $anomalies := .ProcessTrend.StratifiedAnomalies }}
{{- range $anomalies }}
| {{ $stratum }} | {{ .Metric }} | {{ .Severity }} | {{ .Message }} |
{{- end }}
{{- end }}
{{- end }}

{{- if .ProcessTrend.EWMAAnomalies }}
### EWMA Signals
{{- range .ProcessTrend.EWMAAnomalies }}
- **{{ .Metric }}** ({{ .Severity }}): {{ .Message }}
{{- end }}
{{- else }}
*No EWMA signals detected in latest metrics.*
{{- end }}

{{- if .ProcessTrend.PatternViolations }}
### Pattern Signals
{{- range .ProcessTrend.PatternViolations }}
- **{{ .Metric }}** ({{ .Rule }}, {{ .Direction }}, run {{ .RunLength }}): {{ .Message }}
{{- end }}
{{- else }}
*No Nelson-rule pattern signals detected in latest metrics.*
{{- end }}

{{- end }}

{{- if .Experiment }}

## Active Experiment Evaluation

An experiment is currently active. Evaluate its results against the baseline metrics.

**Experiment Details:**
- **Name**: {{ .Experiment.Name }}
- **Hypothesis**: {{ .Experiment.Hypothesis }}
- **Change**: {{ .Experiment.Change }}
- **Measurement**: {{ .Experiment.Measurement }}
- **Risk**: {{ .Experiment.Risk }}
- **Started**: {{ .Experiment.StartedAt.Format "2006-01-02" }}

**Baseline Metrics (at experiment start):**
- Avg cost per bead: ${{ printf "%.4f" .Experiment.BaselineMetrics.AvgCostPerBead }}
- Avg duration per bead: {{ .Experiment.BaselineMetrics.AvgDurationMs }}ms
- Avg input tokens: {{ printf "%.0f" .Experiment.BaselineMetrics.AvgInputTokens }}
- Avg output tokens: {{ printf "%.0f" .Experiment.BaselineMetrics.AvgOutputTokens }}
- Failure rate: {{ printf "%.1f%%" (mul .Experiment.BaselineMetrics.FailureRate 100) }}

**Current Metrics (since experiment started):**
{{- if .ExperimentMetrics }}
- Provider family: {{ .ExperimentMetrics.ProviderFamily }}
- Avg cost per bead: ${{ printf "%.4f" .ExperimentMetrics.CurrentAvgCostPerBead }} ({{ if gt .ExperimentMetrics.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead }}↑ +{{ printf "%.1f%%" (mul (div (sub .ExperimentMetrics.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead) .Experiment.BaselineMetrics.AvgCostPerBead) 100) }}{{ else if lt .ExperimentMetrics.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead }}↓ {{ printf "%.1f%%" (mul (div (sub .ExperimentMetrics.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead) .Experiment.BaselineMetrics.AvgCostPerBead) 100) }}{{ else }}→ no change{{ end }})
- Avg duration per bead: {{ .ExperimentMetrics.CurrentAvgDurationPerBead }} ({{ if gt (durationMs .ExperimentMetrics.CurrentAvgDurationPerBead) .Experiment.BaselineMetrics.AvgDurationMs }}↑ +{{ printf "%.1f%%" (mul (div (sub (durationMs .ExperimentMetrics.CurrentAvgDurationPerBead) .Experiment.BaselineMetrics.AvgDurationMs) .Experiment.BaselineMetrics.AvgDurationMs) 100) }}{{ else if lt (durationMs .ExperimentMetrics.CurrentAvgDurationPerBead) .Experiment.BaselineMetrics.AvgDurationMs }}↓ {{ printf "%.1f%%" (mul (div (sub (durationMs .ExperimentMetrics.CurrentAvgDurationPerBead) .Experiment.BaselineMetrics.AvgDurationMs) .Experiment.BaselineMetrics.AvgDurationMs) 100) }}{{ else }}→ no change{{ end }})
{{- else if .Efficiency }}
- Avg cost per bead: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }} ({{ if gt .Efficiency.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead }}↑ +{{ printf "%.1f%%" (mul (div (sub .Efficiency.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead) .Experiment.BaselineMetrics.AvgCostPerBead) 100) }}{{ else if lt .Efficiency.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead }}↓ {{ printf "%.1f%%" (mul (div (sub .Efficiency.CurrentAvgCostPerBead .Experiment.BaselineMetrics.AvgCostPerBead) .Experiment.BaselineMetrics.AvgCostPerBead) 100) }}{{ else }}→ no change{{ end }})
- Avg duration per bead: {{ .Efficiency.CurrentAvgDurationPerBead }} ({{ if gt (durationMs .Efficiency.CurrentAvgDurationPerBead) .Experiment.BaselineMetrics.AvgDurationMs }}↑ +{{ printf "%.1f%%" (mul (div (sub (durationMs .Efficiency.CurrentAvgDurationPerBead) .Experiment.BaselineMetrics.AvgDurationMs) .Experiment.BaselineMetrics.AvgDurationMs) 100) }}{{ else if lt (durationMs .Efficiency.CurrentAvgDurationPerBead) .Experiment.BaselineMetrics.AvgDurationMs }}↓ {{ printf "%.1f%%" (mul (div (sub (durationMs .Efficiency.CurrentAvgDurationPerBead) .Experiment.BaselineMetrics.AvgDurationMs) .Experiment.BaselineMetrics.AvgDurationMs) 100) }}{{ else }}→ no change{{ end }})
{{- end }}
{{- if .RunStats.Total }}
- Failure rate: {{ printf "%.1f%%" (mul .RunStats.FailureRate 100) }} ({{ if gt .RunStats.FailureRate .Experiment.BaselineMetrics.FailureRate }}↑ +{{ printf "%.1f%%" (mul (div (sub .RunStats.FailureRate .Experiment.BaselineMetrics.FailureRate) .Experiment.BaselineMetrics.FailureRate) 100) }}{{ else if lt .RunStats.FailureRate .Experiment.BaselineMetrics.FailureRate }}↓ {{ printf "%.1f%%" (mul (div (sub .RunStats.FailureRate .Experiment.BaselineMetrics.FailureRate) .Experiment.BaselineMetrics.FailureRate) 100) }}{{ else }}→ no change{{ end }})
{{- end }}

**Your Task for this Experiment:**

Analyze the metrics comparison above and provide:
1. **Observations**: What changed? Did the hypothesis hold? Were there unexpected side effects?
2. **Analysis**: Apply Five Whys if anomalies occurred. What patterns emerged?
3. **Recommendation**: Should we keep the change (integrate it as standard practice), revert it (undo the experiment), or extend the experiment (gather more data)?

{{- end }}

## Task

Analyze the learnings above and provide:

1. **Learning Taxonomy (required)**: Classify key insights into `technical`, `architecture`, and `process`.
2. **Consolidation Opportunities**: Identify duplicate or related learnings that should be merged.
3. **Patterns Worth Promoting**: Suggest learnings that should become rules in RULES.md.
4. **Stale Learnings**: Identify learnings that may no longer be relevant.
5. **Rule Updates**: Propose specific changes to RULES.md.
6. **Depth Quota (required)**:
   - At least one architecture learning.
   - At least one process learning.
   - At least one repeated-pattern callout (recurring signal across multiple beads/learnings/runs).
7. **System Actions (required for top 1-2 highest-impact findings)**:
   - `local_fix`: Immediate patch/change.
   - `system_fix`: Architecture or process change that prevents recurrence.
   - `owner`: Responsible role/person.
   - `due_date`: Concrete date (YYYY-MM-DD).
   - `leading_indicator`: Early signal/metric that improvement is working.

{{- if .BeadStats }}

8. **Stuck Beads Analysis**: For each stuck bead (with 2+ failures) above, suggest:
   - Root cause hypothesis (based on the failures and learnings)
   - Recommended decomposition strategy (how to break it into smaller tasks)
   - Specific next steps to unblock it
{{- end }}

{{- if .Efficiency }}

9. **Efficiency Analysis**: Identify cost or time anomalies in the Current Run Efficiency data above. When anomalies are found, apply Five Whys analysis to trace surface symptoms (e.g., "this bead cost $3") to root causes (e.g., "the acceptance criteria were ambiguous, causing opus escalation"). Produce efficiency-related learnings that identify patterns to avoid or improve.

10. **Experiment Recommendations**: Based on the efficiency analysis, generate 2-4 concrete experiment recommendations. Each experiment should have:
   - **Name**: Short descriptive label (e.g., "Use haiku for test-only beads")
   - **Hypothesis**: What you expect to happen (e.g., "Beads that only modify test files can succeed with haiku, reducing cost by ~60% for those beads")
   - **Change**: What to do differently (e.g., "Add label `complexity:low` to beads whose title contains 'test'")
   - **Measurement**: How to evaluate success (e.g., "Compare success rate and cost of test-only beads before vs after")
   - **Risk**: What could go wrong (e.g., "Test-only beads may fail more on haiku, increasing retries")

11. **Selective Five Whys (required)**:
   - Run Five Whys on the top 1-2 highest-impact anomalies (failure or success), not all items.
   - Include clear evidence at each why-step; stop when the cause moves from local symptom to system behavior.
{{- end }}

{{- if .Experiment }}

12. **PDSA Update**: Update the active experiment in explicit PDSA terms:
   - Evaluate whether the hypothesis held (Study)
   - Decide Act outcome: `keep`, `revert`, or `extend`
   - Provide an implementation-ready summary that can be persisted back into `experiment.json`
{{- end }}

## Output Format

Provide your analysis in two parts:

1. **Freeform Analysis**: Write a narrative summary of your findings, patterns you've noticed, and reasoning behind your recommendations. Use markdown formatting.

2. **Structured Proposals**: After your analysis, include a JSON code block with structured proposals using this schema:

```json
{
  "consolidations": [
    {
      "learning_hashes": ["hash1", "hash2"],
      "consolidated_text": "Merged learning content",
      "rationale": "Why these should be merged"
    }
  ],
  "promotions": [
    {
      "learning_hash": "hash",
      "proposed_rule": "How it should appear in RULES.md",
      "section": "Code Style | Architecture | Safety | Process",
      "rationale": "Why this should be a rule"
    }
  ],
  "archives": [
    {
      "learning_hash": "hash",
      "rationale": "Why this is no longer relevant"
    }
  ],
  "rule_changes": [
    {
      "current_rule": "Exact text from RULES.md",
      "proposed_rule": "New text",
      "rationale": "Why this change is needed"
    }
  ],
  "taxonomy": {
    "technical": ["short insight"],
    "architecture": ["short insight"],
    "process": ["short insight"],
    "repeated_patterns": ["what repeats and where it appears"]
  },
  "system_actions": [
    {
      "finding": "High-impact issue or success to reinforce",
      "type": "architecture | process",
      "local_fix": "Immediate action",
      "system_fix": "Structural prevention/enablement change",
      "owner": "team or person",
      "due_date": "YYYY-MM-DD",
      "leading_indicator": "metric/signal to watch"
    }
  ],
  "five_whys": [
    {
      "item": "anomaly or success being analyzed",
      "impact": "why this matters",
      "why_chain": [
        {"why": 1, "because": "surface cause with evidence"},
        {"why": 2, "because": "deeper cause with evidence"},
        {"why": 3, "because": "system-level cause"}
      ],
      "root_cause_type": "architecture | process | technical",
      "stopping_reason": "why further whys would not add value"
    }
  ],
  "experiments": [
    {
      "name": "Short descriptive label",
      "hypothesis": "What you expect to happen",
      "change": "What to do differently",
      "measurement": "How to evaluate success",
      "risk": "What could go wrong"
    }
  ],
  "pdsa_updates": [
    {
      "experiment_id": "id-or-name",
      "status": "study | act | completed",
      "study_summary": "Concise findings and effect size where possible",
      "act_decision": "keep | revert | extend"
    }
  ]
}
```

**Important**: Use the learning hashes (shown as `Hash: xxxx` in the learnings above) to reference specific learnings in your proposals. This ensures the correct learnings are updated.

## Post-Analysis Housekeeping

After applying retro changes to LEARNINGS.md (archives, consolidations, promotions), **always move all entries from the Archived section to LEARNINGS_ARCHIVE.md**. The Archived section in LEARNINGS.md should only contain the header line `*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*` — never actual entries. Append new archived entries to the top of LEARNINGS_ARCHIVE.md (after the header/separator), preserving existing content.

## Guidelines

- Be conservative - only promote patterns seen multiple times
- Focus on actionable, specific rules
- Ensure proposed rules align with Go idioms and project goals
- Consider whether a learning is truly a "rule" (constraint) or just good advice
- Use the learning hashes from above to reference learnings in your JSON proposals
- Keep technical detail grounded, but prioritize architecture/process leverage in conclusions
- For `system_actions`, include owners and concrete due dates (no "TBD" or relative dates)
- If no architecture/process insight is justified by evidence, explicitly say why (rare)
{{- if .Efficiency }}
- Generate 2-4 experiment recommendations based on efficiency data (not more, not less)
- Each experiment must be concrete, testable, and have clear measurement criteria
- During interactive review, the user will select at most one experiment to run (never multiple)
- Before setting or changing any experiment decision, get explicit user approval; if the session retries, re-confirm approval before finalizing
- Run Five Whys only for top 1-2 highest-impact items; do not apply mechanically to all findings
{{- end }}
{{- if .Experiment }}
- Include exactly one `pdsa_updates` entry for the active experiment
- `study_summary` should explicitly reference process trend/control-limit signals when available
{{- end }}

### Anti-Generic Archival Rules

Archive any learning that meets these criteria:

- **Restates standard engineering principles**: DRY, SRP, SOLID, YAGNI, error handling, code review, test coverage, etc., unless it references a project-specific pattern, file, or convention.
- **Describes basic language features**: Standard library behavior, language syntax, or features (e.g., "Go interfaces are satisfied implicitly", "use defer for cleanup", "channels are thread-safe").
- **Could apply to any project**: The learning lacks specificity — it could be copy-pasted into any software project without modification.
- **Generic process advice**: "Always verify tests pass", "commit early and often", "read documentation before implementing", unless it references a specific failure mode or workflow in this codebase.

**When in doubt, archive**. Project-specific learnings reference concrete files, packages, functions, bead patterns, error messages, or failure modes unique to this codebase. Examples of project-specific learnings:

- "The runner's escalation chain skips haiku when the bead has `complexity:high` label" (references specific code behavior)
- "LEARNINGS.md entries must follow strict pipe-delimited header format: `### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME`" (references specific file format)
- "Use `go test -count=1` in validation to avoid cached results" (references specific project validation command)

Examples of generic learnings to archive:

- "Always verify tests pass before marking a bead complete" (universal advice)
- "Use single responsibility principle" (standard principle)
- "Handle errors gracefully" (no specific pattern or file reference)
