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

### Per-Iteration Efficiency
| Bead ID | Model | Duration | Cost (USD) | Input Tokens | Output Tokens |
|---------|-------|----------|------------|--------------|---------------|
{{- range .Efficiency.CurrentIterations }}
| {{ .BeadID }} | {{ .Model }} | {{ .Duration }} | ${{ printf "%.4f" .CostUSD }} | {{ .InputTokens }} | {{ .OutputTokens }} |
{{- end }}

### Per-Model Aggregates (Current Run)
| Model | Iterations | Avg Cost | Avg Duration | Avg Input Tokens | Avg Output Tokens |
|-------|-----------|----------|--------------|------------------|-------------------|
{{- range $model, $stats := .Efficiency.CurrentModels }}
| {{ $stats.Model }} | {{ $stats.IterationCount }} | ${{ printf "%.4f" $stats.AvgCostUSD }} | {{ $stats.AvgDuration }} | {{ printf "%.0f" $stats.AvgInputTokens }} | {{ printf "%.0f" $stats.AvgOutputTokens }} |
{{- end }}

### Historical Comparison
{{- if .Efficiency.HistoricalModels }}

**Per-Model Aggregates (Historical)**
| Model | Iterations | Avg Cost | Avg Duration | Avg Input Tokens | Avg Output Tokens |
|-------|-----------|----------|--------------|------------------|-------------------|
{{- range $model, $stats := .Efficiency.HistoricalModels }}
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

**Overall Metrics**
- Current avg cost per bead: ${{ printf "%.4f" .Efficiency.CurrentAvgCostPerBead }}
- Historical avg cost per bead: ${{ printf "%.4f" .Efficiency.HistoricalAvgCostPerBead }}
{{- if ne .Efficiency.CostDelta 0.0 }}
- Cost delta: ${{ printf "%.4f" .Efficiency.CostDelta }} ({{ if gt .Efficiency.CostDelta 0.0 }}+{{ printf "%.1f%%" (mul (div .Efficiency.CostDelta .Efficiency.HistoricalAvgCostPerBead) 100) }} more expensive{{ else }}{{ printf "%.1f%%" (mul (div .Efficiency.CostDelta .Efficiency.HistoricalAvgCostPerBead) 100) }} cheaper{{ end }})
{{- end }}

- Current avg duration per bead: {{ .Efficiency.CurrentAvgDurationPerBead }}
- Historical avg duration per bead: {{ .Efficiency.HistoricalAvgDurationPerBead }}
{{- if ne .Efficiency.DurationDelta 0 }}
- Duration delta: {{ .Efficiency.DurationDelta }} ({{ if gt .Efficiency.DurationDelta 0 }}+{{ printf "%.1f%%" (mul (div (durationMs .Efficiency.DurationDelta) (durationMs .Efficiency.HistoricalAvgDurationPerBead)) 100) }} slower{{ else }}{{ printf "%.1f%%" (mul (div (durationMs .Efficiency.DurationDelta) (durationMs .Efficiency.HistoricalAvgDurationPerBead)) 100) }} faster{{ end }})
{{- end }}
{{- else }}
*No historical data available for comparison.*
{{- end }}

{{- if .Efficiency.HighContextIterations }}

### Context Window Utilization Flags
The following iterations exceeded 80% of their model's context window:
{{- range .Efficiency.HighContextIterations }}
- **{{ .BeadID }}** ({{ .Model }}): {{ .InputTokens }} tokens ({{ printf "%.1f%%" (mul .ContextWindowUsed 100) }} of context window)
{{- end }}
{{- end }}

{{- end }}

## Task

Analyze the learnings above and provide:

1. **Consolidation Opportunities**: Identify duplicate or related learnings that should be merged
2. **Patterns Worth Promoting**: Suggest learnings that should become rules in RULES.md
3. **Stale Learnings**: Identify learnings that may no longer be relevant
4. **Rule Updates**: Propose specific changes to RULES.md

{{- if .BeadStats }}

5. **Stuck Beads Analysis**: For each stuck bead (with 2+ failures) above, suggest:
   - Root cause hypothesis (based on the failures and learnings)
   - Recommended decomposition strategy (how to break it into smaller tasks)
   - Specific next steps to unblock it
{{- end }}

{{- if .Efficiency }}

6. **Efficiency Analysis**: Identify cost or time anomalies in the Current Run Efficiency data above. When anomalies are found, apply Five Whys analysis to trace surface symptoms (e.g., "this bead cost $3") to root causes (e.g., "the acceptance criteria were ambiguous, causing opus escalation"). Produce efficiency-related learnings that identify patterns to avoid or improve.
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
  ]
}
```

**Important**: Use the learning hashes (shown as `Hash: xxxx` in the learnings above) to reference specific learnings in your proposals. This ensures the correct learnings are updated.

## Guidelines

- Be conservative - only promote patterns seen multiple times
- Focus on actionable, specific rules
- Ensure proposed rules align with Go idioms and project goals
- Consider whether a learning is truly a "rule" (constraint) or just good advice
- Use the learning hashes from above to reference learnings in your JSON proposals
