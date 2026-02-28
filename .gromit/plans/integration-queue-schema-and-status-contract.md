---
id: integration-queue-schema-and-status-contract
source_spec: integration-queue-schema-and-status-contract
created: 2026-02-28
decomposed: false
---

# Integration Queue Schema And Status Contract Implementation Plan

**Goal:** Implement a durable, versioned integration queue schema and deterministic `gromit status` text/JSON contracts for single-writer integration visibility.

**Architecture:** Add a dedicated `internal/integrationqueue` package for schema, validation, transitions, ordering, and atomic persistence, then consume it from status rendering paths to produce stable human-readable and JSON output.

**Tech Stack:** Go, Cobra CLI, standard library file I/O (`os`, `encoding/json`, atomic rename), existing runner/display pipeline.

**Spec:** `.gromit/specs/integration-queue-schema-and-status-contract.md`

---

## Architecture

**Overview:**
Add a new `internal/integrationqueue` package that owns schema v1, validation, transition rules, deterministic ordering, and atomic persistence for `.gromit/integration-queue.json`. Integrate this package into the status command path so `gromit status` gains a stable Integration Queue text section and a JSON output mode includes the `integration_queue` payload.

**Key Components:**
1. **`internal/integrationqueue/types.go`**: Queue/entry structs, enum constants, and canonical error code constants for schema version 1.
2. **`internal/integrationqueue/validate.go`**: Hard validation for required fields, enum values, and error-code/message contract.
3. **`internal/integrationqueue/transitions.go`**: Transition matrix and fail-closed state transition checks.
4. **`internal/integrationqueue/store.go`**: Load/save using atomic write-then-rename semantics and durable on-disk updates.
5. **`internal/integrationqueue/status.go`**: Deterministic projection for queue counts and ordered status entries.
6. **`internal/runner/display/integration_queue.go`**: Formatting for the Integration Queue section in human-readable `gromit status`.
7. **`internal/runner/status_json.go`**: Structured status JSON payload builder including `integration_queue`.
8. **`cmd/gromit/main.go` updates**: `status --json` support while preserving current default text output.

**Integration Points:**
- `internal/runner/print_status.go` loads queue snapshot and appends Integration Queue section.
- `internal/runner/display` keeps rendering concerns separated from loading/validation.
- `cmd/gromit` status command routes to text or JSON output using shared queue projection logic.
- Coordinator code can later reuse transition and persistence APIs unchanged.

**Data Flow:**
1. `gromit status` starts and resolves config/gromit paths.
2. Runner reads run status and pipeline status (existing path).
3. Runner loads and validates `.gromit/integration-queue.json` through `integrationqueue` store.
4. Queue snapshot is projected into deterministic summary + entries.
5. Output surfaces:
   - text: Integration Queue section with summary and up to 10 entries
   - JSON: `integration_queue` payload with required counts and ordering
6. Validation/parse failures fail closed and surface `queue_schema_invalid`.

**Files to Modify:**
- `internal/runner/print_status.go` - load queue snapshot and include it in status output.
- `cmd/gromit/main.go` - support `gromit status --json`.
- `cmd/gromit/status_test.go` - CLI coverage for queue section and JSON output.
- `internal/runner/status_test.go` - queue section integration and error-path tests.

**Files to Create:**
- `internal/integrationqueue/types.go` - schema model and constants.
- `internal/integrationqueue/validate.go` - schema validation.
- `internal/integrationqueue/transitions.go` - transition rules.
- `internal/integrationqueue/store.go` - atomic persistence and reload.
- `internal/integrationqueue/status.go` - queue status projection/sorting.
- `internal/integrationqueue/types_test.go`
- `internal/integrationqueue/validate_test.go`
- `internal/integrationqueue/transitions_test.go`
- `internal/integrationqueue/store_test.go`
- `internal/integrationqueue/status_test.go`
- `internal/runner/display/integration_queue.go`
- `internal/runner/status_json.go`

**Tradeoffs:**
- **Dedicated package vs embedding queue logic in runner**: Dedicated package keeps schema/transition logic reusable and testable independent of CLI rendering.
- **Strict validation vs tolerant parsing**: Strict fail-closed behavior prevents silent drift and enforces explicit contract errors.
- **Status JSON command output vs mutating run `status.json` file format**: Command-output JSON avoids coupling with run-loop process-liveness state file semantics.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: schema field/enum validation, transition matrix enforcement, deterministic ordering, and atomic persistence behaviors.
2. **Integration Tests**: runner/CLI status output includes Integration Queue section and `integration_queue` JSON payload under mixed queue states.
3. **Manual Verification**: `gromit status` and `gromit status --json` with crafted queue fixtures to validate readability and automation contract.

**Key Test Cases:**
- Valid schema v1 file loads and re-saves without violating required fields.
- Unknown `state`/`lane` values fail validation.
- Disallowed transitions fail with `invalid_transition`.
- Allowed manual requeue transitions (`failed_gates -> ready`, `conflict -> ready`) succeed.
- Blocked count includes only `conflict`, `failed_gates`, and `lane_violation`.
- Entry ordering: integrating first, ready by `fifo_seq`, blocked by descending `updated_at`.
- Text output prints queue length, summary, and up to first 10 non-merged entries with diagnostics.
- JSON output includes `integration_queue` object with deterministic ordering and required keys.
- Parse/validation failures surface `queue_schema_invalid` and keep status command resilient.

**Mocking Strategy:**
- Use real temp-dir filesystem writes for store durability tests.
- Use fixture queue snapshots and lightweight loader seams for runner/display tests.
- Keep rendering tests substring/order based to avoid brittle formatting snapshots.

**Coverage Goals:**
- Full transition matrix and enum validation coverage.
- Contract paths for empty queue, mixed queue, and invalid queue.
- Text and JSON status parity for queue counts and entry ordering.

**Test Organization:**
- Queue contract tests in `internal/integrationqueue/*_test.go`.
- Runner/display integration tests in `internal/runner/status_test.go` and display tests.
- CLI status command contract tests in `cmd/gromit/status_test.go`.

## Implementation Tasks

### Task 1: Define Queue Schema Types And Constants

**Files:**
- Create: `internal/integrationqueue/types.go`
- Test: `internal/integrationqueue/types_test.go`

**What to Do:**
Define schema version 1 data structures for queue file and entries, enumerations for state/lane, and standardized error code constants. Ensure JSON tags and optionality match the spec contract (including optional `changed_files`).

**Acceptance Criteria:**
- Queue and entry structs include all required fields from the spec.
- State/lane/error-code constants match contract names exactly.
- Unit tests verify JSON marshal/unmarshal shape for required/optional fields.

**Dependencies:**
- None

**Notes:**
- Keep schema-version constant explicit for future migrations.

### Task 2: Implement Schema Validation Rules

**Files:**
- Create: `internal/integrationqueue/validate.go`
- Test: `internal/integrationqueue/validate_test.go`

**What to Do:**
Implement strict validation for required fields, unknown enums, and error-code/message rules. Return typed/inspectable validation errors that status/coordinator paths can classify as `queue_schema_invalid`.

**Acceptance Criteria:**
- Unknown `state` and `lane` values fail validation hard.
- Missing required fields fail validation with field-specific diagnostics.
- Entries with non-empty error message but missing error code fail validation.

**Dependencies:**
- Task 1

**Notes:**
- Validation should not mutate input structures.

### Task 3: Encode Transition Contract

**Files:**
- Create: `internal/integrationqueue/transitions.go`
- Test: `internal/integrationqueue/transitions_test.go`

**What to Do:**
Implement the allowed state transition matrix and helper APIs for transition checks/apply operations. Disallowed transitions must fail closed and classify as `invalid_transition`.

**Acceptance Criteria:**
- All allowed transitions in spec pass.
- All unspecified transitions fail with `invalid_transition`.
- Transition helper updates `updated_at` and transition reason only on success.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
- Keep transition table explicit and table-driven for readability.

### Task 4: Build Durable Queue Store With Atomic Writes

**Files:**
- Create: `internal/integrationqueue/store.go`
- Test: `internal/integrationqueue/store_test.go`

**What to Do:**
Implement load/save APIs for `.gromit/integration-queue.json` using write-to-temp + rename semantics. Validate on load and save, and ensure durable updates of top-level `updated_at`.

**Acceptance Criteria:**
- Save path uses atomic rename and never partially overwrites target file.
- Load rejects invalid schema with classification-friendly error wrapping.
- Restart-style read-after-write tests show no entry loss.

**Dependencies:**
- Task 1
- Task 2
- Task 3

**Notes:**
- Keep file path helper centralized to avoid duplicated path logic.

### Task 5: Implement Queue Status Projection And Ordering

**Files:**
- Create: `internal/integrationqueue/status.go`
- Test: `internal/integrationqueue/status_test.go`

**What to Do:**
Add projection logic that computes queue summary counts and entry list ordering per contract (integrating first, ready by FIFO, blocked by recent updates). Include `fifo_position` derivation for ready entries.

**Acceptance Criteria:**
- Counts for queue length, ready, integrating, blocked, and merged-this-run are correct.
- Output entries are deterministically sorted per spec.
- Projection excludes merged entries from the “first 10 non-merged entries” view.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
- Keep this pure (no filesystem access) to simplify reuse and tests.

### Task 6: Add Integration Queue Section To Human-Readable Status

**Files:**
- Create: `internal/runner/display/integration_queue.go`
- Modify: `internal/runner/print_status.go`
- Test: `internal/runner/status_test.go`

**What to Do:**
Wire queue loading/projection into `PrintStatus` and render a new Integration Queue section with summary lines and entry diagnostics. Preserve existing status output sections and ordering.

**Acceptance Criteria:**
- `gromit status` output includes Integration Queue summary lines per contract.
- Up to 10 non-merged entries render with branch/state/lane/ready position/error summary.
- Queue parse/validation failures surface `queue_schema_invalid` diagnostics without panics.

**Dependencies:**
- Task 4
- Task 5

**Notes:**
- Keep section formatting stable for test determinism.

### Task 7: Add Status JSON Surface With integration_queue Payload

**Files:**
- Create: `internal/runner/status_json.go`
- Modify: `cmd/gromit/main.go`
- Test: `cmd/gromit/status_test.go`

**What to Do:**
Introduce `gromit status --json` and emit structured status including existing run/pipeline context plus the new `integration_queue` object. Reuse queue projection logic to guarantee parity with text counts/order.

**Acceptance Criteria:**
- `gromit status --json` emits valid JSON with `integration_queue` fields from the spec.
- `integration_queue.entries` ordering follows contract deterministically.
- Default `gromit status` text output remains backward compatible.

**Dependencies:**
- Task 5
- Task 6

**Notes:**
- Keep JSON schema additive to avoid breaking existing consumers.

### Task 8: Contract Hardening And End-to-End Status Coverage

**Files:**
- Modify: `internal/runner/acceptance/status_acceptance_test.go`
- Modify: `internal/runner/acceptance/status_integration_acceptance_test.go`
- Modify: `cmd/gromit/cli_contract_test.go`

**What to Do:**
Add acceptance/contract tests that lock down queue contract behavior across process restarts and command invocations, including invalid schema fail-closed handling and stable output surfaces.

**Acceptance Criteria:**
- Acceptance tests verify queue data survives restart and remains visible in status.
- Contract tests verify stable queue section presence and no unintended status mutation behavior.
- Invalid queue schema path exercises surfaced `queue_schema_invalid` handling.

**Dependencies:**
- Task 6
- Task 7

**Notes:**
- Keep fixtures small and deterministic to avoid flakiness.

---

## Notes

- This plan intentionally isolates schema/transition logic from coordinator execution logic so future single-writer integration work can reuse the same contract package.
- If an integration queue already exists in a different format, add a schema migration task before enabling strict validation in production paths.
