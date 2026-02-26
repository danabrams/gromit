---
id: tracker-adapter-interface
source_ideas:
  - idea-1771591754839
created: 2026-02-20
epic: cross-project-portability
---

# Tracker Adapter Interface

## Specification

Introduce a tracker adapter layer that decouples orchestration logic from bead/`bd` implementation details, while keeping the current `bd` backend as the default path.

Today, work orchestration assumes bead semantics end-to-end (`Ready`, `ListWithLabel`, `CreateWithParentAndDescription`, `Close`, `Sync`). This makes Gromit difficult to run in repositories that use other issue/work-item systems.

This spec defines a stable internal tracker contract and migrates orchestration call-sites to that contract.

### Core Interface

Add a new tracker interface package (for example `internal/tracker/`) with provider-neutral types:

```go
type Item struct {
    ID              string
    Title           string
    Description     string
    Priority        int
    Status          string
    Labels          []string
    ParentID        string
    ExpectedOutputs []string
}

type Client interface {
    Ready(ctx context.Context, filter Filter) (*Item, error)
    List(ctx context.Context, query Query) ([]*Item, error)
    Get(ctx context.Context, id string) (*Item, error)
    Create(ctx context.Context, req CreateRequest) (*Item, error)
    Close(ctx context.Context, id string) error
    AddComment(ctx context.Context, id, comment string) error
    Sync(ctx context.Context) error
    HasOpenChildren(ctx context.Context, parentID string) (bool, error)
}
```

Existing bead-specific helpers become adapter concerns, not runner concerns.

### Adapter Implementations

Phase 1 ships:
- `BDAdapter` (maps `internal/bead.Client` to tracker `Client`)

Future adapters are out of scope for this spec, but the interface must be sufficient to support them without changing runner signatures.

### Migration Strategy

1. Add tracker interface and `BDAdapter`.
2. Introduce dual-wired constructors that accept tracker client while preserving current constructor compatibility.
3. Migrate runner/pipeline/CLI orchestration call-sites from bead-specific dependencies to tracker `Client`.
4. Keep bead package as the concrete implementation behind `BDAdapter`.

No behavior change is expected in this repository after migration.

### Error and Semantics Contract

- "No ready item" must be represented consistently (for example `nil, nil`), matching current runner expectations.
- Ordering semantics used by current workflows (priority-first where applicable) must be preserved by `BDAdapter`.
- Label filtering and parent relationships must remain compatible with existing spec/epic labeling flows.

## Acceptance Criteria

- A new internal tracker interface exists with neutral item/request/query types.
- `BDAdapter` implements the tracker interface with compile-time assertion.
- Runner and pipeline orchestration can operate through tracker `Client` without direct dependency on `internal/bead` types.
- Existing behavior for `gromit run`, `plan`, `decompose`, `review`, `epic` remains unchanged in this repository.
- Existing test coverage for bead-backed behavior remains green with adapter wiring.
- New contract tests verify adapter semantics for ready selection, label filtering, create/close/sync, and open-children checks.

## Execution Order

- Sequence position: 2
- Dependencies: `project-profiles-core`
- Unblocks: non-`bd` backend follow-up specs and adapterized orchestration work

## Decisions

1. **Introduce adapter behind existing backend first** -- this reduces risk and gives an immediate seam for future trackers.

2. **Context-aware interface methods** -- include `context.Context` in tracker methods for cancellation consistency with pipeline direction.

3. **Neutral type model, not bead aliasing** -- avoid leaking bead-specific fields and names into the long-term contract.

4. **No new external tracker in this slice** -- adding another backend now would increase risk and scope; contract-first is enough for this step.

## Research & Context

### Current Coupling Points

- Runner interface and tests are bead-centric (`internal/runner/interfaces*.go`).
- CLI flow helpers instantiate and use `bead.Client` directly in multiple commands (`cmd/gromit/review.go`, `cmd/gromit/decompose.go`, `cmd/gromit/plan.go`, `cmd/gromit/epic.go`, `cmd/gromit/main.go`).
- Pipeline adapters currently wrap bead client types (`cmd/gromit/adapters.go`).

### Files to Change

| File | Change |
|------|--------|
| `internal/tracker/*` | New interface/types/contracts package |
| `internal/bead/*` (targeted) | Add adapter implementation and conversion helpers |
| `internal/runner/interfaces.go` | Replace bead-specific dependency types with tracker client |
| `cmd/gromit/adapters.go` | Wire tracker adapter into pipeline adapters |
| `cmd/gromit/*.go` (targeted) | Use tracker client construction paths instead of direct bead assumptions |
| `test/contracts/*` | Add tracker adapter contract tests |

### Out of Scope

- Implementing non-`bd` trackers (GitHub, Linear, etc.).
- End-user configuration for selecting tracker backend (handled in follow-up spec).
- Changing decomposition/priority policy itself.
