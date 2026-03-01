---
id: human-ai-learning-loop
source_ideas: []
created: 2026-03-01
---

# Human-AI Learning Loop

## Specification

Extend `gromit retro` to include a **Workmanship Report** section that closes the loop between AI-observed codebase friction and human-led improvement decisions. This formalizes the human's role as manager in Deming's Theory of Knowledge: the system surfaces actionable knowledge with options and tradeoffs, the human allocates improvement effort, the AI executes, and the system confirms whether the friction was resolved.

### How It Works

After retro completes its existing analysis (efficiency trends, learning consolidation, rule proposals), it performs an additional pass:

1. **Friction Detection**: Cluster confirmed and recent learnings by codebase area (package, module, or cross-cutting concern). Identify areas where learnings are accumulating — repeated observations about the same structural issue indicate friction that the AI is working around rather than resolving.

2. **Evidence Assembly**: For each friction cluster, gather supporting evidence: number of related learnings, timespan, associated rework cycles, and any convergence or stability anomalies from the same area.

3. **Option Generation**: For each friction area, generate 2-4 options ranging from structural refactoring to deferral. Each option includes estimated investment (in spec cycles), expected impact on the friction, and risk of choosing or deferring.

4. **Workmanship Report Output**: Present all friction areas with their options as a section of the retro output. Always include this section — when no friction clusters are detected, report that the codebase is healthy.

5. **Decision Capture**: When the human selects an option, it becomes a spec that flows through the normal pipeline (refine, plan, decompose, run). The AI executes the refactoring.

6. **Confirmation**: On subsequent retros, the system checks whether learnings in previously-identified friction areas have decreased. This closes the Theory of Knowledge loop: the prediction (this area causes friction) was acted on, and the outcome is measured.

### The Human's Role

The human is the manager, not the implementer. Their responsibilities are:

- Review the Workmanship Report during retro
- Choose which friction areas to invest in (and which to defer)
- Select among the presented options or propose an alternative
- Validate that completed refactoring actually reduced friction

The AI handles detection, evidence gathering, option generation, and execution of the chosen refactoring.

### Relationship to Pride of Workmanship

This addresses Deming's Point 12 (Remove barriers to Pride of Workmanship) in two directions:

- **For the AI**: Structural friction in the codebase prevents the AI from producing clean work. Surfacing that friction and resolving it removes barriers to quality output.
- **For the human**: Without structured signals about where improvement effort has the highest leverage, the human cannot take pride in the system's trajectory. The Workmanship Report gives the human clear, evidence-based management decisions to make.

## Acceptance Criteria

- `gromit retro` output includes a Workmanship Report section after the existing retro analysis.
- The Workmanship Report clusters learnings by codebase area and identifies friction areas where learnings are accumulating.
- Each friction area includes supporting evidence (learning count, timespan, associated rework cycles).
- Each friction area presents 2-4 options with investment estimates, expected impact, and risk of deferral.
- When no friction clusters are detected, the Workmanship Report states that the codebase is healthy.
- Subsequent retros check whether previously-identified friction areas show reduced learning accumulation after refactoring was executed.
- A selected option can be captured as a spec for execution through the normal pipeline.

## Decisions

1. **Part of retro, not a separate command** The Workmanship Report is a natural extension of retro's existing analysis. Retro already examines learnings and trends; adding friction clustering and option generation keeps the workflow unified rather than introducing a new command the human must remember to run.

2. **Always include the section** Even when no friction is detected, reporting a healthy codebase is valuable signal. This establishes the habit of reviewing workmanship and avoids the section becoming invisible when things are going well. Can be made conditional later once the pattern is proven.

3. **Options with tradeoffs, not single recommendations** The human is a manager making resource allocation decisions. Presenting options with tradeoffs respects their judgment and gives them the information needed to make good decisions. A single recommendation removes the management function.

4. **High-level friction areas, not prescriptive refactoring steps** The report identifies *what* areas have friction and *what kind* of improvement options exist. The detailed implementation planning happens downstream when the chosen option becomes a spec. This keeps the Workmanship Report at the right level of abstraction for a management decision.

5. **AI executes the refactoring** Consistent with VISION.md principle 3 (preserve human strategic control, automate execution details). The human decides what to improve; the AI does the work through the normal spec-plan-run pipeline.

6. **Confirmation loop measures learning reduction** The Theory of Knowledge requires that predictions be tested. By checking whether learnings in a friction area decrease after refactoring, the system validates that the intervention worked — or flags that deeper issues remain.

## Research & Context

### Current State

- `internal/retro/` performs post-run analysis: efficiency trends, learning consolidation, rule proposals
- `internal/learnings/` manages the three-tier learning hierarchy (confirmed, provisional, archived) with fuzzy-match promotion and category tagging
- Learnings are date-stamped, bead-ID-tracked, and categorized (ARCHITECTURE, RELIABILITY, TEST_QUALITY)
- `internal/logger/process_trend.go` already implements SPC (UCL/LCL/anomalies) for success rate, duration, and cost
- No existing mechanism clusters learnings by codebase area or surfaces them as human action items

### Deming Alignment

- **Point 1 (Constancy of Purpose)**: Already addressed by VISION.md
- **Point 3 (Cease dependence on inspection)**: Addressed by process-stability-governance spec
- **Point 8 (Drive out fear)**: Addressed by process-stability-governance spec
- **Point 12 (Remove barriers to Pride of Workmanship)**: This spec — surfaces friction so it can be resolved rather than worked around
- **Theory of Knowledge**: The confirmation loop (predict friction, act, measure outcome) operationalizes Deming's requirement that knowledge be predictive and testable

### Relevant Files

- `internal/retro/` — retro analysis, where the Workmanship Report will be added
- `internal/learnings/` — learning storage, querying, and fuzzy-match promotion
- `internal/learnings/query.go` — filtered learning retrieval (will need area-based clustering)
- `internal/logger/process_trend.go` — SPC metrics that can provide supporting evidence
- `VISION.md` — strategic alignment reference
- `.gromit/specs/process-stability-governance.md` — related Deming-aligned spec for SPC-based governance
