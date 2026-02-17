# Prompt Context Budget Design

**Date:** 2026-02-17
**Spec:** `.gromit/specs/prompt-context-budget.md`

## Principle

Prompts should be as small and specific to the task at hand as possible.

## Problem

LLM invocations are slow because prompts carry too much context. A typical build prompt reaches 26K chars before adding specs. RULES.md (13.6K) and CLAUDE.md (5.3K) dominate the baseline.

## Approach: Two layers

### Layer 1: Trim source files

Reduce CLAUDE.md from 5,341 to ~1,200 chars by keeping only Architecture and Key Principles. Tighten RULES.md from 13,655 to ~9,000-10,000 chars by condensing verbose rules and removing duplication. This drops baseline from ~26K to ~18K.

### Layer 2: Dynamic budget mechanism

A unified `ShapeContextForBudget()` function applies a deterministic trim order when context exceeds a configurable budget (default 25K chars). Trim order:

1. Drop recent learnings
2. Drop CLAUDE.md
3. Cap confirmed learnings
4. Drop confirmed learnings
5. Phase-filter rules
6. Truncate spec (last resort)

Never fully drop rules or spec.

### Config

```yaml
prompt:
  budget:
    max_chars: 20000
    learning_cap_chars: 2000
```

### Observability

Log trim actions when they fire. Silent when under budget.

## Decision log

- All phases covered (build, review, retro, refactor, TDD/ATDD build)
- Single global budget, no per-phase overrides (retro and build are close enough)
- Both source trimming and dynamic budgeting (defense in depth)
- ATDD acceptance phase excluded (separate spec)
