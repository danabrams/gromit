---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T12:39:49Z"
id: tracker-adapter-interface
source_spec: tracker-adapter-interface
---

# Tracker Adapter Interface Implementation Plan

**Goal:** Introduce a provider-neutral tracker interface with a BD-backed adapter and migrate orchestration to use it without changing behavior.

**Architecture:** Add `internal/tracker` types and a `BDAdapter` over `internal/bead.Client`, then update runner/pipeline/CLI wiring to depend on `tracker.Client` and `tracker.Item` instead of bead-specific types.

**Tech Stack:** Go, existing `internal/bead` CLI wrapper, existing pipeline/runner architecture.

**Spec:** `.gromit/specs/tracker-adapter-interface.md`

---

## Architecture

**Overview:**  
Introduce a provider-neutral `internal/tracker` interface with typed requests and items, implement a `BDAdapter` over `internal/bead.Client`, then migrate runner/pipeline/CLI orchestration to depend on `tracker.Client` and `tracker.Item` instead of bead types.

**Key Components:**
1. **`internal/tracker`**: Owns neutral `Item`, `Filter`, `Query`, `CreateRequest`, and `Client` interface (context-aware).
2. **`BDAdapter`**: Implements `tracker.Client` by delegating to `*bead.Client` and converting between `bead.Bead` and `tracker.Item`.
3. **Runner/Pipeline/CLI wiring**: Use `tracker.Client` in constructors and orchestration, with thin adapters where needed.

**Integration Points:**
- Replace bead-specific client usage in runner constructors and interfaces with `tracker.Client`.
- Update pipeline to accept `tracker.Client` (or a minimal tracker-shaped interface) instead of bead-specific `BeadClient`.
- Replace direct `bead.NewClient()` calls in CLI with `tracker.NewClient()` or equivalent factory that returns a `tracker.Client` (backed by BDAdapter).

**Data Flow:**
- CLI/runner/pipeline request a `tracker.Client` from a backend resolver.
- `BDAdapter` translates `tracker.Filter`/`Query`/`CreateRequest` to `bead.Client` calls and converts results to `tracker.Item`.
- Orchestration logic uses `tracker.Item` fields (`ID`, `Title`, `Labels`, `Status`, `ParentID`, `ExpectedOutputs`) without importing `internal/bead`.

**Files to Modify:**
- `internal/runner/interfaces.go` - replace bead-specific types with tracker types for orchestration.
- `internal/runner/constructor.go` - `newTrackerClient` returns `tracker.Client`; update usage of `Ready`, `ReadyWithLabel`, `Show`, `ListWithLabel` to tracker equivalents.
- `internal/runner/constructor_adapters.go` - adapters accept tracker client, translate to stage interfaces; remove direct `*bead.Client` dependencies.
- `cmd/gromit/adapters.go` - pipeline adapters consume tracker client and convert `tracker.Item` to `pipeline.BeadInfo`.
- `cmd/gromit/*.go` (targeted) - replace `bead.NewClient()` and `ListWithLabel` usage with `tracker.Client` and `Query`/`Filter`.
- `internal/pipeline/pipeline.go` (and related tests) - update `BeadClient` dependency to tracker-shaped interface and adjust mocks.

**Files to Create:**
- `internal/tracker/types.go` - `Item`, `Filter`, `Query`, `CreateRequest`, optional status constants.
- `internal/tracker/client.go` - `Client` interface and helper constructors.
- `internal/bead/tracker_adapter.go` - `BDAdapter` implementing `tracker.Client`.
- `internal/bead/tracker_convert.go` - conversion helpers between bead and tracker types (if needed).
- `test/contracts/tracker_adapter_contract_test.go` - contract tests for `BDAdapter` semantics.

**Tradeoffs:**
- **Adapter in `internal/bead` vs `internal/tracker`**: Place in `internal/bead` to align with “bead-specific helpers become adapter concerns” and minimize cross-package dependencies.
- **Tracker `Query`/`Filter` scope**: Keep minimal fields required by current call sites (labels, status, exclude IDs, limit), avoiding premature generalization.
- **Pipeline dependency change**: Update pipeline to use tracker client rather than bead client so orchestration truly decouples from `internal/bead`.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: adapter conversion and method semantics for `BDAdapter` in `internal/bead`.
2. **Integration/Contract Tests**: `test/contracts/tracker_adapter_contract_test.go` against fake bd harness for ready/list/create/close/sync/has-open-children.
3. **Regression Coverage**: existing bead-backed tests in runner/pipeline/CLI remain green; update typed/mock tests for tracker types.

**Key Test Cases:**
- `BDAdapter.Ready` returns `nil, nil` when no ready item.
- `BDAdapter.List` with label filters returns items in priority order; behavior matches bead semantics.
- `BDAdapter.Create` passes expected outputs and respects parent/deps semantics where used.
- `BDAdapter.HasOpenChildren` validates parent ID and matches bead behavior.
- `BDAdapter.Sync` and `Close` propagate errors.

**Mocking Strategy:**
- Use bead `RunFn` injection for unit tests of adapter argument formation.
- For contract tests, reuse fake bd harness in `test/contracts` for end-to-end behavior.

**Coverage Goals:**
- All adapter methods used by orchestration call-sites.
- Edge cases: empty results, invalid IDs, empty labels, nil client.

**Test Organization:**
- Unit tests colocated with adapter (`internal/bead/*_test.go` or `internal/tracker/*_test.go`).
- Contract tests in `test/contracts/tracker_adapter_contract_test.go`, build tag `contract`.

## Implementation Tasks

### Task 1: Add Tracker Interface Types

**Files:**
- Create: `internal/tracker/types.go`
- Create: `internal/tracker/client.go`

**What to Do:**
Define `Item`, `Filter`, `Query`, `CreateRequest`, and the `Client` interface with `context.Context` methods per spec. Keep types minimal and provider-neutral; include fields currently required by orchestration (labels, status, parent, expected outputs). Add optional constants for known statuses to avoid string scatter.

**Acceptance Criteria:**
- `internal/tracker` compiles with `Item`, `Filter`, `Query`, `CreateRequest`, and `Client` definitions.
- All methods use `context.Context` and align with spec signatures.
- No dependency on `internal/bead` from `internal/tracker`.

**Dependencies:**
- None

**Notes:**
Keep the type set minimal so later adapters can extend without changing runner signatures.

### Task 2: Implement BDAdapter for tracker.Client

**Files:**
- Create: `internal/bead/tracker_adapter.go`
- Create: `internal/bead/tracker_convert.go`
- Modify: `internal/bead/*` as needed for helper wiring

**What to Do:**
Implement `BDAdapter` with a compile-time interface assertion. Map tracker `Filter`/`Query` to existing bead operations (`Ready`, `ListWithLabel`, `Show`, `CreateWithParentAndDescription`, `CreateWithDepsAndDescription`, `Close`, `Sync`, `AddComment`, `HasOpenChildren`). Add conversion helpers between `bead.Bead` and `tracker.Item` and ensure nil input yields `nil, nil` for “no ready item.”

**Acceptance Criteria:**
- `BDAdapter` implements `tracker.Client` with compile-time assertion.
- Conversion preserves `ID`, `Title`, `Description`, `Priority`, `Status`, `Labels`, `Parent`, `ExpectedOutputs`.
- Ready semantics preserve “no ready item” as `nil, nil`.

**Dependencies:**
- Task 1

**Notes:**
Avoid leaking bead types into tracker package; keep conversions in `internal/bead`.

### Task 3: Migrate Runner to tracker.Client

**Files:**
- Modify: `internal/runner/interfaces.go`
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/constructor_adapters.go`

**What to Do:**
Replace bead-specific interfaces with tracker-based ones. Update `newTrackerClient` to return `tracker.Client` via `BDAdapter`. Update all orchestration call-sites (`Ready`, `ReadyWithLabel`, `Show`, `ListWithLabel`, `Create*`, `HasOpenChildren`) to use tracker types and adapters. Keep compatibility behavior unchanged.

**Acceptance Criteria:**
- Runner and stages compile without direct dependency on `internal/bead` types.
- Runtime behavior unchanged for `gromit run` and related flows.
- Status/progress computations still use the same ordering/filters as before.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
Be careful to preserve `ReadyWithLabel` behavior; if tracker interface uses filter/query, implement equivalent logic in runner.

### Task 4: Migrate Pipeline and CLI Orchestration

**Files:**
- Modify: `internal/pipeline/pipeline.go`
- Modify: `cmd/gromit/adapters.go`
- Modify: `cmd/gromit/main.go`
- Modify: `cmd/gromit/decompose.go`
- Modify: `cmd/gromit/review.go`
- Modify: `cmd/gromit/epic.go`
- Modify: `cmd/gromit/queue.go`
- Modify: `cmd/gromit/verify_spec.go`

**What to Do:**
Update pipeline `BeadClient` dependency to tracker-shaped interface (or rename to `TrackerClient`). Adjust pipeline adapters to consume `tracker.Client` and convert `tracker.Item` into `pipeline.BeadInfo`. Replace direct `bead.NewClient()` usage in CLI with tracker client construction. Update label filtering utilities to call tracker `List` with query filters.

**Acceptance Criteria:**
- Pipeline and CLI compile with no direct `internal/bead` usage in orchestration.
- Existing behavior for `plan`, `decompose`, `review`, `epic`, `queue` remains unchanged.
- All unit/typed interface tests updated to the new tracker client types.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
Keep `pipeline.BeadInfo` as-is; it remains the pipeline’s minimal view of tracker items.

### Task 5: Add Tracker Adapter Contract Tests

**Files:**
- Create: `test/contracts/tracker_adapter_contract_test.go`
- Modify: `test/contracts/doc.go` or helpers if needed

**What to Do:**
Add contract tests validating BDAdapter behavior against the fake bd harness. Mirror existing bd contract patterns for readiness, label filtering, create/close/sync, and open-children checks.

**Acceptance Criteria:**
- Contract tests pass under `-tags contract` with fake bd harness.
- Tests assert ready nil semantics, label filtering, and create/close/sync flows.

**Dependencies:**
- Task 2
- Task 4

**Notes:**
Use existing contract helpers to avoid duplicating harness setup.

---

## Notes

- Keep behavior parity: label filtering, priority ordering, epic exclusion, and nil-ready semantics must match existing bead behavior.
- Ensure any new tracker query fields are sufficient but minimal; defer future tracker-specific fields.
- Avoid introducing non-`bd` backend selection in this change; keep adapter behind existing default.
