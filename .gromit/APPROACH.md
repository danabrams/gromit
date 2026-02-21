# Gromit Approach Validation

*Recorded 2026-02-21 from exploration session.*

## The Goal

Specify in the morning, come back to working code at the end of the day. Review through behavior, not code. Only engage with architectural decisions.

## Why This Approach (Not Specs + Subagents)

Claude Code sessions aren't designed for all-day autonomous execution. They timeout, hit context limits, crash. A subagent-based approach loses all state when the session dies.

Gromit's value for this use case:
- State persists in beads + git — a crash is a pause, not a loss
- Fresh context per iteration prevents compounding errors
- Escalation and retry handle failures without supervision
- Accumulated learnings (rules/retro) improve output quality over time

## Validated Signals (Early Days)

- 76 beads closed overnight — the loop produces real output
- Firefighting ratio ~1/3, trending down — acceptable for early stage
- Clear path to improvement — spec discipline, not infrastructure changes

## The Owner Split

| Domain | Owner |
|--------|-------|
| What to build | Human (specs) |
| Architectural decisions | Human (review during planning) |
| How to build it | System |
| Code quality | System (rules + validation + review) |
| Behavioral correctness | Acceptance tests |

## Known Failure Mode: Complexity Accumulation

LLMs default to adding rather than refactoring. Each bead is locally reasonable but cumulative effect is tangled, oversized packages. The system doesn't drift *away* from architecture — it grows *toward* complexity.

**Fix**: Lead more on module design in specs. State not just what to build but where it lives and what it must not touch.

Complexity-as-review-signal is a good idea but should surface as a digest/report, not auto-created beads (avoids backlog pollution).

## Leverage Points

1. **Spec discipline** — architectural detail in specs prevents complexity accumulation downstream
2. **Acceptance test coverage** — behavioral review only works if tests actually cover the behavior
3. **Rules** — turn architectural intuitions into enforceable constraints; each firefighting session is a candidate rule

## What Would Falsify the Approach

- Spec writing takes longer than just coding
- Firefighting ratio increases or holds steady (not decreasing)
- Architectural rework consumes more time than forward progress
- Building gromit infrastructure crowds out building the actual product
