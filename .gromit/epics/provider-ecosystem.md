---
epic_id: provider-ecosystem
created: 2026-02-26
---

# Provider Ecosystem

## Problem

Gromit supports multiple LLM providers (Claude, Codex, Gemini) but routing, cost tracking, and provider-specific capabilities are evolving quickly. Model selection heuristics, escalation thresholds, and provider feature parity need ongoing tuning and experimentation.

## Vision

A robust multi-provider routing layer with per-provider health monitoring, configurable escalation policies, and evidence-based routing ratios — so each bead runs on the best provider for its complexity and cost profile.

## Scope

- Model routing and tier assignment evaluation
- Codex CLI integration and streaming parity
- Gemini provider capabilities (cross-review, context window, deep think)
- Provider health canary and usage-limit detection
- Multi-agent interactive phases
- Provider-neutral wording and abstractions
