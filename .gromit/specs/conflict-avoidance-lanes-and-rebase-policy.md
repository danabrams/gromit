---
id: conflict-avoidance-lanes-and-rebase-policy
source_ideas: []
created: 2026-02-28
epic: codebase-health
accepted: true
---

# Conflict Avoidance Lanes And Rebase Policy

## Specification

Define a conflict-minimizing integration policy that allows code-changing interactive commands (`debug`, `review`, `retro`) while maximizing safe automation. Lane checks are strict only for hard safety violations; normal source overlap is handled through rebase+gate automation.

## Problem

Command-based assumptions ("interactive branches are always safe metadata edits") are false in practice. Interactive sessions often modify production code. Overly strict command-level blocking would reduce usefulness, while no policy leads to avoidable conflicts and integration churn.

## Goals

1. Support code edits from any command when safe.
2. Minimize conflicts through automated rebase and validation.
3. Enforce strict blocking only where safety requires it.
4. Keep queue throughput even when individual branches fail.

## Non-Goals

- Guaranteeing zero conflicts for all overlapping source edits.
- Full ownership-locking across all source paths in phase 1.
- Manual gate execution as default behavior.

## Design

### 1. File-based lane classification

Classify branches by changed files, not command type:

- `safe_lane`: `.gromit/**`, docs/spec/epic/backlog artifacts, non-runtime project metadata.
- `code_lane`: any source, tests, configs, build files, or mixed changes.

### 2. Hard safety policy (strict blocking)

Block integration (`lane_violation`) only for prohibited paths/artifacts defined by project safety rules, including runtime/local-state artifacts that must never be committed.

Strict blocking does not apply simply because a branch touches source code.

### 3. Rebase and gate automation

For `code_lane` branches:
1. Rebase onto latest `origin/main`.
2. Run touched-package validation gates.
3. Retry once after a fresh rebase on gate failure.
4. Mark `failed_gates` if still failing.

For `safe_lane` branches:
- Use fast path with minimal required validation, still respecting hard safety checks.

### 4. Conflict handling

- If rebase/merge conflict occurs, mark branch `conflict`, preserve branch for manual resolution, continue processing next FIFO entry.
- Queue must not stall on a single conflict.

## Acceptance Criteria

- Branch classification uses changed-file analysis, not command allowlists.
- `debug`, `review`, and `retro` branches that touch source code are processed through `code_lane` automation, not automatically blocked.
- Hard-safety artifact violations are blocked with explicit `lane_violation` state.
- `code_lane` branches receive rebase + touched-package gates + one retry.
- On conflict or terminal failure, coordinator continues with remaining queued branches.

## Decisions

1. Strict blocking is narrow and safety-focused.
2. Code edits from interactive commands are first-class and automated.
3. Automation is fail-closed per branch, fail-open for queue progress.

## Research & Context

- Existing rules already identify prohibited runtime artifacts and validation expectations.
- This spec aligns automation with real command behavior while preserving safety boundaries.

