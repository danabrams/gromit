# Plan (Cycle 3)

## t-005

Replace the persistent-failures section intro text and all four audit instruction steps in buildFixPlanPrompt (lines ~174-187) with the exact wording from the spec. Fixes all 7 review findings: intro must say 'The following failures have repeated across multiple consecutive cycles.\nThis strongly suggests the contract assertion itself is wrong, not the implementation.'; directive must say 'BEFORE creating any implementation fix task for these failures:'; steps 1-4 must match spec verbatim including regex examples and 'Prefer creating a contract fix task' guidance.

