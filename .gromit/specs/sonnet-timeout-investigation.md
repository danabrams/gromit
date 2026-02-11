# Sonnet Timeout Investigation

## Problem

Sonnet experiences 15x more timeouts than opus in gromit iterations. This causes iteration failures that may be preventable through better timeout tuning, model selection, or rate limiting mitigation.

## Key Questions

### 1. Root Cause: Rate Limiting vs Complexity

- Are sonnet timeouts caused by Anthropic API rate limiting (429s, throttling) or by the model genuinely struggling with task complexity?
- Does the stall detection system (`runner.go:841-954`, `startHeartbeat()`) distinguish between rate-limit stalls and complexity stalls?
- Are there patterns in the stream stats (`stream.go:68-164`) that differentiate the two? (e.g., rate-limited stalls show no events at all, while complexity stalls show intermittent events)

### 2. Bead Characteristics of Timed-Out Beads

- What priority levels (P0/P1/P2) are most affected?
- What are the acceptance criteria counts for timed-out beads vs successful ones?
- Are timed-out beads associated with specific spec labels or complexity labels?
- Are certain types of work (refactoring, new features, bug fixes) more prone to timeout?

### 3. Model Selection Appropriateness

- Are some P1 beads too complex for sonnet and should be auto-escalated to opus?
- Does the scope check (`config.go:63-67`, `ScopeCheckConfig`) provide useful signal for preemptive escalation?
- Would a complexity-based timeout threshold (higher timeout for complex beads) reduce false timeouts?

### 4. Timeout Threshold Tuning

- Current timeout config (`config.go:80-88`):
  - `timeout`: 600s (10 min) — overall Claude invocation timeout
  - `stall_timeout`: 120s (2 min) — initial stall detection
  - `stall_timeout_active`: 300s (5 min) — stall detection after tool activity
  - `bead_timeout`: 1200s (20 min) — max per bead
- Should timeouts be per-model? (e.g., sonnet gets shorter timeout, opus gets longer)
- Is the two-tier stall detection (initial vs active) properly calibrated for sonnet's response patterns?
- Stall retry logic (`process.go:174-208`) — does the current retry strategy work for sonnet or just burn time?

### 5. Data Collection Gaps

- Current JSONL logs capture: success/failure, duration, model, cost, tokens
- Missing data that would help diagnosis:
  - Stall detection events (which tier fired, how many stalls per invocation)
  - Rate limit indicators (if detectable from stream events)
  - Time-to-first-event (distinguishes startup delay from mid-run stall)
  - Tool call count and types before timeout (was the model making progress?)
  - Whether the timeout was a stall timeout vs bead timeout vs invocation timeout

## Proposed Investigation Steps

1. **Analyze existing logs**: Parse JSONL logs to identify patterns in timed-out sonnet iterations
2. **Add diagnostic logging**: Capture stall events, time-to-first-event, and tool activity metrics
3. **Test per-model timeouts**: Configure different stall/bead timeouts for sonnet vs opus
4. **Evaluate preemptive escalation**: Use scope check results to skip sonnet for complex beads
5. **Monitor rate limiting**: Add stream-level detection for rate limit responses

## Code Paths to Investigate

- Stall detection: `runner.go:841-954` (`startHeartbeat()`)
- Stream stats: `stream.go:68-164` (event parsing, timing)
- Timeout config: `config.go:80-88` (`ClaudeConfig`)
- Stall retry: `process.go:174-208` (`handleStallTimeout`)
- Model selection: `config.go:334-357` (`SelectModel`)
- Scope check: `runner.go:1482-1535` (`checkScope`)

## Success Criteria

Investigation complete when:
- Root cause distribution is quantified (rate limiting vs complexity vs other)
- Per-model timeout configuration is either implemented or ruled out with data
- Diagnostic logging captures enough data for ongoing monitoring
- Preemptive escalation rules are defined based on bead characteristics
