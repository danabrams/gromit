---
epic_id: observability-and-diagnostics
created: 2026-02-26
---

# Observability and Diagnostics

## Problem

When iterations fail or costs spike, diagnosing root causes is difficult. Metrics lack granularity (missing failure reasons, phase-level observability, cost-per-spec tracking), logging has formatting issues, and debugging requires manual investigation.

## Vision

Rich per-phase, per-spec, and per-provider observability with structured failure classification, a debug command, and deterministic runbooks — so operators can quickly understand what went wrong and why.

## Scope

- Failure reason fields and layered failure classification
- Per-phase observability contracts
- Cost-per-spec and haiku cost/token tracking
- Debug command and deterministic runbook
- Newline and log formatting fixes
- Git failure classification
- Phase-specific failure rate analysis
