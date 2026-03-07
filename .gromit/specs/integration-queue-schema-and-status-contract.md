---
id: integration-queue-schema-and-status-contract
source_ideas: []
created: 2026-02-28
epic: codebase-health
accepted: true
---

# Integration Queue Schema And Status Contract

## Specification

Define the on-disk schema for integration queue state and the user-visible `gromit status` output contract for single-writer integration.

This spec is the contract layer for:
- `.gromit/integration-queue.json`
- status rendering and JSON output surfaces
- forward-compatible schema evolution

## Problem

Without an explicit schema and status contract, implementation can drift across commands and tests. Queue behavior becomes hard to reason about, and users cannot reliably diagnose why a branch is pending or blocked.

## Goals

1. Stable, versioned queue schema.
2. Explicit per-branch state and diagnostics fields.
3. Deterministic `gromit status` text and JSON surfaces.
4. Backward-compatible evolution rules.

## Non-Goals

- Defining storage for unrelated run metrics.
- Full TUI design for queue visualization.
- Network-distributed queue coordination.

## Queue Schema

### File path

- `.gromit/integration-queue.json`

### Top-level object

```json
{
  "schema_version": 1,
  "updated_at": "2026-02-28T00:00:00Z",
  "entries": []
}
```

### Entry object

```json
{
  "branch": "gromit/review-1772240000000000000",
  "session_id": "review-1772240000000000000",
  "origin_command": "review",
  "state": "ready",
  "lane": "code_lane",
  "created_at": "2026-02-28T00:00:00Z",
  "updated_at": "2026-02-28T00:00:00Z",
  "attempt_count": 0,
  "retry_count": 0,
  "fifo_seq": 42,
  "base_ref": "origin/main",
  "head_sha": "abc123",
  "changed_files": [
    "cmd/gromit/review.go"
  ],
  "changed_files_hash": "sha256:...",
  "last_error_code": "",
  "last_error_message": "",
  "last_transition_reason": "session_committed"
}
```

### Required fields

- `branch`
- `session_id`
- `origin_command`
- `state`
- `lane`
- `created_at`
- `updated_at`
- `attempt_count`
- `retry_count`
- `fifo_seq`
- `base_ref`
- `head_sha`
- `changed_files_hash`

`changed_files` may be omitted for very large diffs; `changed_files_hash` remains required.

### State enum

- `draft`
- `ready`
- `integrating`
- `merged`
- `conflict`
- `failed_gates`
- `lane_violation`

Unknown state values are hard validation errors.

### Lane enum

- `safe_lane`
- `code_lane`

Unknown lane values are hard validation errors.

## Transition Contract

Allowed transitions:

- `draft -> ready`
- `ready -> integrating`
- `integrating -> merged`
- `integrating -> conflict`
- `integrating -> failed_gates`
- `integrating -> lane_violation`
- `failed_gates -> ready` (manual requeue)
- `conflict -> ready` (manual requeue after conflict resolution)

Disallowed transitions must fail closed and record a coordinator error.

## Status Contract

### Human-readable `gromit status` additions

Add an **Integration Queue** section with:

- `Queue length: <n>`
- `Ready: <n> | Integrating: <n> | Blocked: <n> | Merged this run: <n>`
- Up to first 10 non-merged entries in FIFO order showing:
  - branch
  - state
  - lane
  - queue position (if `ready`)
  - short last error code/message (if present)

Blocked count includes `conflict`, `failed_gates`, and `lane_violation`.

### JSON status surface

If status JSON output exists, include:

```json
{
  "integration_queue": {
    "schema_version": 1,
    "queue_length": 3,
    "ready_count": 1,
    "integrating_count": 1,
    "blocked_count": 1,
    "entries": [
      {
        "branch": "gromit/debug-...",
        "state": "ready",
        "lane": "code_lane",
        "fifo_position": 1,
        "last_error_code": "",
        "last_error_message": ""
      }
    ]
  }
}
```

Entries must be sorted by:
1. active `integrating` first
2. `ready` by ascending `fifo_seq`
3. blocked by descending `updated_at`

## Error Code Contract

Standardize `last_error_code`:

- `rebase_conflict`
- `merge_conflict`
- `gate_failure`
- `lane_violation_artifact`
- `invalid_transition`
- `queue_schema_invalid`
- `push_failed`

Freeform `last_error_message` is allowed, but code is required when an error exists.

## Concurrency & Durability Rules

1. Queue writes use atomic write-then-rename semantics.
2. Coordinator reloads queue from disk before each integration attempt.
3. On parse/validation failure, integration pauses and surfaces `queue_schema_invalid`.
4. No in-memory-only mutations are considered committed until durable write succeeds.

## Acceptance Criteria

- Queue file validates against schema version 1.
- Invalid enum values or transitions fail closed with explicit error code.
- `gromit status` consistently shows queue summary and entry-level diagnostics.
- Status JSON includes `integration_queue` payload with deterministic ordering.
- Queue survives process restart with no entry loss.

## Decisions

1. Single JSON file with explicit schema version for simplicity and migration control.
2. Deterministic ordering and explicit error codes to support automation and tests.
3. Fail-closed semantics for schema and transition violations.

## Research & Context

- Prior specs define single-writer coordinator, FIFO lifecycle, and lane/rebase policy.
- This spec formalizes the data and output contracts needed to implement and verify those behaviors.

