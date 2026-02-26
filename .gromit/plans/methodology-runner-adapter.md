---
id: methodology-runner-adapter
source_spec: methodology-runner-adapter
created: 2026-02-26
decomposed: false
---

# Methodology Runner Adapter Implementation Plan

**Goal:** Introduce a profile-selected methodology runner adapter seam so ATDD/TDD command behavior is no longer hard-coded to Go while preserving existing Go behavior.

**Architecture:** Add a `RunnerAdapter` interface in the methodology package, implement `go` and `passthrough` adapters, and route executor command generation through adapter selection derived from resolved profile/compatibility context.

**Tech Stack:** Go, existing gromit runner/methodology/config packages, existing Go test suite.

**Spec:** `.gromit/specs/methodology-runner-adapter.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Introduce a `RunnerAdapter` seam in `internal/runner/methodology` and route all methodology command generation through it. Keep Go behavior identical via a Go adapter, and use passthrough for non-Go profiles.

**Key Components:**
1. **`RunnerAdapter` interface** (`internal/runner/methodology/adapter.go`): Contract for acceptance/TDD command generation and validation.
2. **Go adapter** (`internal/runner/methodology/adapter_go.go`): Moves current `AcceptanceCommands` behavior unchanged (tag injection + touched package scoping for `go test ./...`).
3. **Passthrough adapter** (`internal/runner/methodology/adapter_passthrough.go`): Returns commands unchanged; basic validation only.
4. **Adapter resolver/selector** (`internal/runner/methodology/adapter_resolver.go`): Selects adapter from resolved project profile (`go` => go adapter; `node|python|custom` => passthrough).
5. **Executor wiring updates** (`internal/runner/methodology/executor.go`): Replace direct `AcceptanceCommands(...)` calls with adapter methods, and log selected adapter + resulting commands in debug logs.

**Integration Points:**
- `Executor` gets/constructs a `RunnerAdapter` once (from config-resolved profile/adapter context).
- Existing helper logic for Go scoping remains but moves behind Go adapter.
- Config validation expands to permit supported adapters (at least `go`, `passthrough`) or enforces profile-driven defaults while still honoring explicit values.

**Data Flow:**
- `Config` resolves profile/adapter selector.
- Methodology executor resolves adapter at runtime.
- ATDD/TDD phases request command sets via adapter methods.
- Validator runs adapter-produced commands.
- Debug logs emit adapter name + final command list.

**Files to Modify:**
- `internal/runner/methodology/executor.go` - replace direct rewrite calls with adapter usage and logging.
- `internal/runner/methodology/methodology_test.go` - preserve current Go behavior assertions; add adapter-driven behavior tests.
- `internal/config/config.go` - relax/adjust `methodology.adapter` validation to support non-go pathway.
- `internal/config/compatibility_resolution.go` - ensure profile defaults map to adapter selection consistent with spec.
- `internal/config/profile_defaults.go` (if needed) - carry explicit profile->adapter default mapping metadata.

**Files to Create:**
- `internal/runner/methodology/adapter.go`
- `internal/runner/methodology/adapter_go.go`
- `internal/runner/methodology/adapter_passthrough.go`
- `internal/runner/methodology/adapter_resolver.go`
- `internal/runner/methodology/adapter_test.go` (or split per adapter)

**Tradeoffs:**
- **Explicit adapter seam vs inline branching:** chose seam to isolate language-specific behavior and avoid profile conditionals spreading through executor logic.
- **Passthrough for non-Go now vs partial Node/Python heuristics now:** chose passthrough to avoid incorrect assumptions and keep this slice low-risk.
- **Profile-driven default with explicit override:** preserves backward compatibility while supporting future adapters incrementally.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** adapter behavior (go vs passthrough), profile-driven selection, and command generation correctness.
2. **Integration Tests:** executor ATDD/TDD paths still produce current Go commands and do not mutate non-Go commands.
3. **Manual Verification:** optional run with `project.profile: go` and `project.profile: python` to confirm logged adapter/commands.

**Key Test Cases:**
- Go adapter injects `-tags acceptance` for `go test` and preserves existing scoping behavior.
- Go adapter does not double-inject tags.
- Passthrough adapter leaves commands unchanged.
- Adapter resolver maps profiles: `go -> go`, `node|python|custom -> passthrough`.
- Executor uses adapter output (not hardcoded rewrite function).
- Non-Go profile avoids Go-specific rewrites in methodology flows.
- Existing Go methodology integration tests remain green (regression guard).

**Mocking Strategy:**
- Mock `validateFn` in executor tests to capture commands passed for verification.
- Use real adapter implementations in unit tests (no heavy mocking) to lock behavior contracts.

**Coverage Goals:**
- Critical path: adapter selection + ATDD validation command generation.
- Edge cases: empty commands, pre-tagged Go commands, touched package scoping, non-Go profile behavior.

**Test Organization:**
- New adapter tests under `internal/runner/methodology/*_test.go`.
- Extend existing executor/methodology tests in `internal/runner/methodology/methodology_test.go`.
- Extend config compatibility/profile tests in `internal/config/*_test.go` for selector defaults/validation.

## Implementation Tasks

### Task 1: Define RunnerAdapter Contract and Built-In Adapters

**Files:**
- Create: `internal/runner/methodology/adapter.go`
- Create: `internal/runner/methodology/adapter_go.go`
- Create: `internal/runner/methodology/adapter_passthrough.go`
- Test: `internal/runner/methodology/adapter_test.go`

**What to Do:**
Add the `RunnerAdapter` interface and implement two first-party adapters:
- `go` adapter encapsulating current Go-specific acceptance command mutation and scoped package behavior.
- `passthrough` adapter returning commands as-is.
Include adapter-level validation behavior appropriate for methodology mode.

**Acceptance Criteria:**
- `RunnerAdapter` interface exists with acceptance, TDD, and validation methods.
- Go adapter reproduces current `AcceptanceCommands` behavior exactly.
- Passthrough adapter performs no command rewriting.

**Dependencies:**
- None.

**Notes:**
Keep Go command mutation helpers private to the Go adapter implementation so the seam is clearly language-scoped.

### Task 2: Add Profile-Driven Adapter Resolution

**Files:**
- Create: `internal/runner/methodology/adapter_resolver.go`
- Modify: `internal/config/compatibility_resolution.go`
- Modify: `internal/config/config.go`
- Test: `internal/config/profile_resolution_test.go`
- Test: `internal/config/compatibility_resolution_test.go`

**What to Do:**
Add resolver logic that maps resolved project profile to adapter choice (`go` => `go`, `node|python|custom` => `passthrough`) and reconcile explicit `methodology.adapter` handling with this model. Update config validation to allow supported adapter values used by this phase.

**Acceptance Criteria:**
- Resolved non-Go profiles select passthrough adapter by default.
- Explicit supported adapter values pass config validation.
- Existing legacy fallback behavior for unspecified selectors remains backward compatible.

**Dependencies:**
- Task 1 (adapter type names and constructors).

**Notes:**
Preserve compatibility metadata (`source`, `deprecation_marker`) semantics while updating selected value defaults.

### Task 3: Wire Executor and Methodology Flows to RunnerAdapter

**Files:**
- Modify: `internal/runner/methodology/executor.go`
- Modify: `internal/runner/process_methodology.go` (if TDD command building path needs adapter access)
- Modify: `internal/runner/process_methodology_atdd.go` (if required for ATDD flow plumbing)
- Test: `internal/runner/methodology/methodology_test.go`
- Test: `internal/runner/process_methodology_test.go`

**What to Do:**
Replace direct calls to hard-coded Go command transformation with adapter method calls in ATDD/TDD methodology execution paths. Ensure adapter selection is done once per execution context and surfaced through debug logs alongside resulting command lists.

**Acceptance Criteria:**
- ATDD verification and red-phase checks obtain commands via `RunnerAdapter`.
- Go profile behavior remains unchanged in existing tests.
- Non-Go profiles do not receive Go-specific command rewrites.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
Prefer narrow wiring changes (Executor-owned adapter resolution) to avoid broad runner constructor churn unless tests show a cleaner injection seam is needed.

### Task 4: Regression and Coverage Hardening

**Files:**
- Modify: `internal/runner/methodology/methodology_test.go`
- Create/Modify: `internal/runner/methodology/adapter_selection_test.go`
- Modify: `internal/runner/acceptance/*` tests as needed for updated logging assertions

**What to Do:**
Add and update tests to assert profile-based adapter selection and end-to-end command-generation behavior. Keep existing Go flow tests intact as regression locks and add non-Go coverage to prevent silent reintroduction of Go-specific rewriting.

**Acceptance Criteria:**
- Unit tests cover adapter selection matrix by profile.
- Unit/integration tests prove non-Go commands run unchanged in methodology mode.
- Existing Go ATDD/TDD integration behavior remains green.

**Dependencies:**
- Task 3.

**Notes:**
Treat command list assertions as behavioral contracts; keep them explicit and stable.

---

## Notes

- This plan intentionally establishes an adapter seam first and defers Node/Python-specific command heuristics to follow-up specs.
- Backward compatibility for this repository is critical: Go behavior must remain unchanged while non-Go profiles stop receiving implicit Go command rewrites.
- Debug visibility of selected adapter and final commands is required for diagnosability and future multi-language support.
