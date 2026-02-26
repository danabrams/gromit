---
epic_id: run-loop-reliability
created: 2026-02-26
---

# Run Loop Reliability

## Problem

The run loop encounters stuck beads, timeout failures, precheck loops, and deadline mismanagement. These issues burn iterations and cost without making progress, and lack clear circuit-breaking or escalation paths.

## Vision

A resilient run loop that detects stuck states early, breaks out of failure spirals, respects time budgets, and provides clear operator controls (graceful stop, between-iteration hooks, end-of-loop commands).

## Scope

- Stuck bead detection and circuit breakers
- Timeout handling and deadline management
- Precheck loop guards
- Graceful stop keystroke
- Between-iteration and end-of-loop configurable commands
- Orchestration policy decoupling
- Scope gate thresholds
