---
epic_id: codebase-health
created: 2026-02-26
---

# Codebase Health

## Problem

The runner package is 24K lines across 52 files. Internal interfaces use interface{} returns, dual fields exist on bead.Client, shell injection surfaces remain, and several abstractions need consolidation (SpecGate, normalizeNilFields, RunFn/runFn).

## Vision

Clean, well-factored internals where each package has a focused responsibility, interfaces return concrete types, and known tech debt items are resolved — making the codebase easier to extend and maintain.

## Scope

- Runner sub-package split
- Pipeline interface{} → concrete types
- bead.Client RunFn/runFn consolidation
- Shell injection surface elimination
- SpecGate unification
- normalizeNilFields standardization
- Result.Output vs Result.Stdout cleanup
- context.Background() audit
- Review.go git ops injection
- Pipeline nil check fixes
- Invocation result struct consolidation
- JSON tag casing fixes
