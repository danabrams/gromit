---
id: process-stability-governance
source_ideas: ["idea-1772245470239"]
created: 2026-02-28
---

# Process Stability Governance

## Specification

Gromit's run loop currently relies on arbitrary numerical quotas—hard limits like `max_tdd_cycles` or the 6,000-line test budget—to prevent resource exhaustion and infinite loops. This specification replaces these static caps with **Process Stability Metrics** derived from Statistical Process Control (SPC).

The goal is to move from "inspection at the end of the line" (hitting a budget) to "quality built-in" by detecting and responding to process instability (e.g., loop churn or bloat) before it consumes resources unproductively.

### 1. TDD Loop: Convergence Monitoring
Instead of stopping after a fixed number of cycles (e.g., `max_tdd_cycles: 10`), the TDD loop must monitor the **convergence velocity** of the red-green-refactor cycles.

- **Convergence Metric**: Track the **Complexity Delta** (a weighted score of diff size, failing test counts, and token usage) over the last 3 cycles.
- **Instability Detection**: 
    - **Deadlock**: Near-zero Complexity Delta over 3 cycles (no progress being made).
    - **Oscillation**: High Complexity Delta that repeats a previous state or fluctuates wildly without resolving failures.
- **Andon Response**: When instability is detected, the loop triggers an **Andon Cord event** that stops the bead and categorizes the failure as `NOT_READY_SCOPE` or `NOT_READY_CRITERIA`. It recommends an interactive `refine` session rather than a retry.

### 2. Test Bloat: Validation Time UCL
Instead of a fixed line-count budget for tests, the system uses the **Validation Time Trend** to identify bloat.

- **Process Baseline**: Use the 3-sigma Upper Control Limit (UCL) for a package's validation time (already tracked in `internal/logger`).
- **Bloat Detection**: A package is flagged for **"Process Bloat"** if its validation duration exceeds the UCL for 3 consecutive iterations.
- **Response**: The system continues to allow new tests but marks the package as **"High Maintenance Cost."** The `gromit status` output and the `retro` phase must highlight these packages for manual refactoring or "test debt" cleanup.

### 3. Systemic Andon Cord (Self-Correction)
Leverage existing `TrendAnomaly` logic to trigger automated system corrections across the pipeline:

- **Rework Rate Spike**: If the `RollingReworkRate` exceeds its UCL, the **Gate Phase** must reject new beads belonging to the same `spec` as the failing beads, requiring a `refine` or `plan` review.
- **Duration/Token Anomaly**: If a bead's estimated tokens or duration exceeds the 3-sigma UCL for its tier, the system must trigger **Proactive Decomposition** automatically, breaking the bead into smaller sub-tasks before execution.

## Acceptance Criteria

- [ ] `internal/logger/process_trend.go` includes a `ConvergenceScore` that tracks complexity deltas over 3 cycles.
- [ ] TDD Loop (`internal/runner/tdd/orchestrator.go`) terminates when `ConvergenceScore` indicates a deadlock or oscillation, emitting an `AndonEvent`.
- [ ] The `Gate` stage (`internal/pipeline/prepare/`) rejects beads when the associated spec's `RollingReworkRate` is in a high-severity anomaly state.
- [ ] `gromit status` and `gromit retro` report "High Maintenance Cost" warnings for packages exceeding their Validation Time UCL.
- [ ] The `max_tdd_cycles` configuration is deprecated and ignored in favor of Convergence Monitoring.

## Decisions

1. **Why replace hard caps with SPC?** Hard caps incentivize sub-optimal workarounds (e.g., reducing coverage to stay under a line budget). SPC incentivizes process improvement by identifying the *instability* that causes the bloat or churn.
2. **Why use a 3-cycle window for convergence?** A 3-cycle window is the minimum necessary to distinguish between a single difficult cycle and a systemic failure to converge (deadlock or oscillation).
3. **Andon over failure?** Treating these as Andon Cord events (requiring human intervention via `refine`) rather than simple "failures" prevents the loop from retrying a task that the current system cannot solve without better requirements.

## Research & Context

### Current State
- `internal/logger/process_trend.go`: Already implements SPC (UCL/LCL/Anomalies/Pattern Violations) for success rate, duration, and cost.
- `internal/runner/tdd/orchestrator.go`: Currently relies on `CycleState.IsComplete()` which checks `MaxCycles`.
- `LEARNINGS_ARCHIVE.md`: Documents the 6,000-line hard budget for acceptance tests as a current "hard constraint."

### Theoretical Background: W. Edwards Deming
This approach aligns with Deming’s 14 points, specifically:
- **Point 3 (Cease dependence on inspection)**: Instead of measuring at the end (quota), we measure the process (stability).
- **Point 8 (Drive out fear)**: Developers (and LLMs) shouldn't fear hitting a budget; they should be notified when the *system* is unstable.
