---
epic_id: test-quality
created: 2026-02-26
---

# Test Quality

## Problem

Test suite has runtime issues, documentary tests that don't call real code, fragile picker index assumptions, skipped tests that may be runnable, and missing coverage in key areas (decompose cancellation, stats merge, explore delegation, timeout classification).

## Vision

A fast, trustworthy test suite where every test exercises real behavior, slow tests are tagged for CI separation, and critical paths have explicit coverage.

## Scope

- Test runtime reduction (bead and runner packages)
- Eliminate documentary tests for real function calls
- Robust picker tests (not hardcoded indices)
- t.Skip() audit across all test files
- Missing unit test coverage (runner, decompose, stats, explore, timeout, learnings)
- Shared MockProvider extraction
- Gate integration test realism
- Gemini fixture testing strategy
- Acceptance-tag build failures
