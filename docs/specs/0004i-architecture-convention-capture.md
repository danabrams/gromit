# Spec 0004i — Architecture Convention Capture and Propagation

## spec_id
architecture-convention-capture

## Vision

The executor makes implementation convention decisions in isolation. Each task writer resolves shared patterns — type semantics, interface signatures, naming — without knowing how sibling tasks resolved the same choice. When conventions need to be consistent across files (e.g., "Config.Tier always carries a tier label, never a resolved model name"), there's no mechanism to document that choice and propagate it. Review catches the inconsistency, triggers a replan, but the fix planner still lacks the decision, so the next fix attempt re-invents the convention or introduces a different inconsistency. The run thrashes.

This spec closes that gap. Before decomposing tasks, the planner documents cross-cutting conventions. Those conventions flow into every executor task prompt and persist through fix cycles, so once a convention is established it can't be forgotten.

## Summary

Add architecture convention capture to the planning phase. The planner documents cross-cutting implementation conventions (type semantics, interface contracts, naming patterns) as part of its initial plan output, in the same LLM call via a think-step before task decomposition. Conventions are stored in `RunState.ArchitectureConstraints`, injected into every executor task prompt, and carried forward in fix cycle prompts. When the fix planner resolves architecture drift findings, it can emit new conventions that accumulate for subsequent cycles.

## Goals

### Primary
- Planner documents conventions before decomposing tasks, in the same LLM call
- Conventions flow into executor task prompts so each task writer has them
- Fix planner receives inherited constraints and can extend them
- Constraints survive resume (persisted in RunState / run.json)

### Secondary
- Empty convention lists produce no prompt noise (no empty sections injected)

## Non-goals

- Separate LLM call for convention extraction (same call, think-step only)
- Cross-run or cross-spec convention sharing (deferred to 0006a decision ledger)
- Enforcing conventions via contract assertions (0004h territory)
- Automatically detecting convention violations — this spec only propagates; review still catches them

## Architecture

### Data flow

```
buildPlanPrompt → Plan.ArchitectureDecisions
                       ↓
              RunState.ArchitectureConstraints  ←  fix plan output (accumulates)
                       ↓                ↓
         executor task prompt    buildFixPlanPrompt
```

### New fields

`Plan` (`internal/next/planner/types.go`):
```go
ArchitectureDecisions []string `json:"architecture_decisions,omitempty"`
```

`RunState` (`internal/next/runstore`):
```go
ArchitectureConstraints []string `json:"architecture_constraints,omitempty"`
```

`FixPlanRequest` (`internal/next/planner/planner.go`):
```go
ArchitectureConstraints []string `json:"architecture_constraints,omitempty"`
```

### Prompt changes

`buildPlanPrompt` — adds a think-step before the output format instructions:

```
## Architecture Decisions (think before decomposing)
Before listing tasks, identify any cross-cutting conventions this spec
introduces: type semantics, interface signatures, naming patterns, or
behavioral contracts that multiple tasks must agree on. Document each
as a short declarative statement. If none exist, leave the array empty.
Output these as "architecture_decisions": [...] in your JSON.
```

`buildFixPlanPrompt` — adds a section when `FixPlanRequest.ArchitectureConstraints` is non-empty:

```
## Architecture Conventions (MUST follow in all fix tasks)
These conventions were established in prior cycles. All fix tasks must
comply. If this fix resolves an architecture_drift finding, add the
resolved convention to "architecture_decisions" in your output.
- <constraint>
```

Executor task prompt (`internal/next/specloop/provider_taskrunner.go`) — adds a section when non-empty:

```
### Architecture Conventions
- <constraint>
```

### RunState lifecycle

- After initial planning: `rs.ArchitectureConstraints = plan.ArchitectureDecisions`
- After fix planning: append non-duplicate entries from `fixPlan.ArchitectureDecisions` to `rs.ArchitectureConstraints` (exact-string dedup)
- `plan.go` passes `rs.ArchitectureConstraints` into `FixPlanRequest.ArchitectureConstraints`

## Acceptance Criteria

1. `Plan` struct has an `ArchitectureDecisions []string` field serialized as `"architecture_decisions"` in JSON output.

2. `buildPlanPrompt` includes a think-step section instructing the planner to document cross-cutting conventions before task decomposition, with output in `architecture_decisions`.

3. After initial planning, `RunState.ArchitectureConstraints` is populated from `plan.ArchitectureDecisions`.

4. `FixPlanRequest` has an `ArchitectureConstraints []string` field; `plan.go` populates it from `rs.ArchitectureConstraints` before invoking the fix planner.

5. `buildFixPlanPrompt` includes an "Architecture Conventions" section when `FixPlanRequest.ArchitectureConstraints` is non-empty; the section is absent when it is empty.

6. Fix plan JSON output includes `architecture_decisions`; non-empty entries are appended to `RunState.ArchitectureConstraints` (exact-string dedup).

7. Executor task prompt includes an "Architecture Conventions" section when `RunState.ArchitectureConstraints` is non-empty; the section is absent when it is empty.

8. `RunState.ArchitectureConstraints` is persisted to `run.json` and restored on resume.

9. All existing tests continue to pass.

## Scenarios

### Scenario: Planner captures a convention on initial plan

**Given:** a spec that introduces a shared type used across multiple files (e.g. `Config.Tier` used in `review_distill.go`, `review_guided.go`, and `review_packet.go`)
**When:** the planner generates the cycle 1 plan
**Then:** `plan.ArchitectureDecisions` contains an entry documenting the convention (e.g. `"Config.Tier always receives a tier label such as 'medium', never a resolved model name such as 'claude-sonnet-4-6'"`)
**And:** `RunState.ArchitectureConstraints` contains that entry after planning completes
**And:** each executor task prompt includes an "Architecture Conventions" section with that entry

### Scenario: Executor task receives conventions

**Given:** `RunState.ArchitectureConstraints` is `["Config.Tier always receives a tier label"]`
**When:** the executor builds the prompt for any task in the run
**Then:** the prompt contains `### Architecture Conventions` followed by the constraint
**Notes:** conventions are global to the run, not filtered per-file

### Scenario: Fix planner inherits and extends conventions

**Given:** cycle 1 produced `ArchitectureConstraints = ["Config.Tier always receives a tier label"]`
**And:** review finds a new architecture drift: `LLMCompleter.Complete must use context.Context as first parameter`
**When:** the fix planner generates the cycle 2 plan
**Then:** `buildFixPlanPrompt` includes the existing constraint in an "Architecture Conventions" section
**And:** the fix plan's `architecture_decisions` includes the resolved convention for the drift finding
**And:** `RunState.ArchitectureConstraints` after cycle 2 contains both entries
**And:** cycle 3 fix prompts and executor prompts contain both entries

### Scenario: No conventions — no prompt noise

**Given:** a spec that touches a single file with no cross-cutting patterns
**When:** the planner generates the plan
**Then:** `plan.ArchitectureDecisions` is empty
**And:** `RunState.ArchitectureConstraints` remains empty
**And:** executor task prompts contain no "Architecture Conventions" section
**And:** fix plan prompts contain no "Architecture Conventions" section

### Scenario: Constraints survive resume

**Given:** a run is paused mid-execution with `ArchitectureConstraints` populated in `run.json`
**When:** the run is resumed
**Then:** `RunState.ArchitectureConstraints` is restored from `run.json`
**And:** subsequent executor task prompts include the constraints

## Validation

```
go test ./internal/next/planner/...
go test ./internal/next/specloop/...
go test ./internal/next/runstore/...
go vet ./...
go build ./...
```
