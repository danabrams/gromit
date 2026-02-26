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

## Task

Analyze the learnings above and provide:

1. **Learning Taxonomy (required)**: Classify key insights into `technical`, `architecture`, and `process`
2. **Consolidation Opportunities**: Identify duplicate or related learnings that should be merged
3. **Patterns Worth Promoting**: Suggest learnings that should become rules in RULES.md
4. **Stale Learnings**: Identify learnings that may no longer be relevant
5. **Rule Updates**: Propose specific changes to RULES.md
6. **Depth Quota (required)**:
   - At least one architecture learning
   - At least one process learning
   - At least one repeated-pattern callout

{{- if .BeadStats }}

7. **Stuck Beads Analysis**: For each stuck bead (with 2+ failures) above, suggest:
   - Root cause hypothesis (based on the failures and learnings)
   - Recommended decomposition strategy (how to break it into smaller tasks)
   - Specific next steps to unblock it
{{- end }}

8. **System Actions (required for top 1-2 highest-impact findings)**:
   - `local_fix`: Immediate patch/change
   - `system_fix`: Architecture or process change that prevents recurrence
   - `owner`: Responsible role/person
   - `due_date`: Concrete date (YYYY-MM-DD)
   - `leading_indicator`: Early metric/signal of improvement

9. **Selective Five Whys (required)**:
   - Run Five Whys on top 1-2 highest-impact findings (failure or success), not all items
   - Stop when you reach a system-level cause

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
    "repeated_patterns": ["what repeats and where"]
  },
  "system_actions": [
    {
      "finding": "high-impact issue or success",
      "type": "architecture | process",
      "local_fix": "Immediate action",
      "system_fix": "Structural prevention change",
      "owner": "team or person",
      "due_date": "YYYY-MM-DD",
      "leading_indicator": "metric/signal to watch"
    }
  ],
  "five_whys": [
    {
      "item": "finding analyzed",
      "impact": "why this matters",
      "why_chain": [
        {"why": 1, "because": "surface cause with evidence"},
        {"why": 2, "because": "deeper cause with evidence"},
        {"why": 3, "because": "system-level cause"}
      ],
      "root_cause_type": "architecture | process | technical",
      "stopping_reason": "why to stop at this depth"
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
- Prioritize architecture/process leverage in conclusions, not only local technical fixes
- `system_actions` must include concrete owners and due dates (no TBD)
- Run Five Whys selectively for top 1-2 impact items only
