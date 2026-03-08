# V2 Run Loop: Clean Implementation Design

## Decision

Build v2 in a separate `internal/v2/` package tree while v1 remains untouched. This replaces Decision #15 ("in-place evolution") from the v2-run-loop spec.

### Rationale

Three factors favor a clean implementation over in-place evolution:

1. **Bootstrapping constraint.** Gromit v1 builds v2. In-place migration risks breaking the tool that produces the code. A separate package tree eliminates this risk.

2. **No throwaway adapters.** The in-place strategy wraps v1 stages in translation adapters, then rewrites them one by one. Clean implementation builds v2 stages natively from the start — no intermediate wrappers.

3. **Simpler mental model.** No half-v1/half-v2 state. V1 works. V2 grows alongside it. When v2 is ready, v1 is deleted.

## Package Structure

```
internal/v2/
  loop/         # two-level loop (spec_loop.go, bead_loop.go)
  stage/        # Stage interface, Request, Result, Decision types
  stages/
    plan/       # spec-level: generate implementation plan
    decompose/  # spec-level: break plan into beads with deps
    gate/       # bead-level: relevance check
    build/      # bead-level: LLM invocation with model escalation
    validate/   # bead-level: run configured validation commands
    review/     # bead-level: self-review, auto-create fix beads
    epilogue/   # bead-level: close bead, emit telemetry
    accept/     # spec-level: verify acceptance criteria
    present/    # spec-level: surface work to product owner
  adapter/      # interfaces: LLM, TaskTracker, Presenter, Git
  event/        # typed event system with versioned schema
  prompt/       # composable assembler (base/project/instance/fragment)
  dep/          # dependency DAG resolution for specs and beads
```

## Stage Interface

```go
type Stage interface {
    Name() string
    Run(ctx context.Context, req Request) (Result, error)
}

type Request struct {
    Bead      BeadMeta
    Model     string
    Iteration int
    Config    *config.Config
    SpecDir   string
}

type Decision int
const (
    Proceed Decision = iota
    Skip
    Block
    Fail
)

type Result struct {
    Decision  Decision
    Artifacts any       // stage-specific typed data
    Events    []Event
}
```

Stages are stateless. They read from git/filesystem via `SpecDir`. The loop type-asserts `Artifacts` where needed (Review and Accept return new beads). `Events` accumulate in the loop and stream to subscribers.

## Two-Level Loop

**Outer loop** (`spec_loop.go`):
1. Load spec, verify dependency specs are satisfied
2. Create worktree
3. Plan stage: generate implementation plan
4. Decompose stage: produce ordered beads with dependency declarations
5. Execute beads via inner loop
6. Accept stage: verify acceptance criteria against codebase
   - Not met + under generation cap: generate more beads, return to step 5
   - Cap reached: Andon escalation, preserve branch with failure summary
7. Present stage: surface completed work to product owner
8. Clean up worktree

**Inner loop** (`bead_loop.go`):
Pick next bead whose dependencies are satisfied, run: Gate, Build, Validate, Review, Epilogue.

Retry is loop-enforced. Stages declare `MaxRetries` and `RetryWith`. On Validate failure, rerun Build then Validate. Build handles model escalation internally.

Generation tracking: Decompose beads are gen 0, Review-created beads are parent gen+1, Accept-created beads are a new generation. Default cap: 3.

## Reuse Strategy

**Import from v1** (use existing packages directly):
- `internal/config/` — YAML config loading
- `internal/bead/` — bd CLI integration, behind TaskTracker adapter

**Copy and adapt from v1** (take the logic, rewrite the interface):
- Decomposition logic from `internal/pipeline/decompose/` — works well, copy into v2 Decompose stage
- Validation command execution from `internal/pipeline/validate/`
- Model escalation logic from `internal/pipeline/execute/`
- Review prompt logic from `internal/pipeline/review/`

**Build fresh:**
- Plan stage (new)
- Accept and Present stages (new)
- Composable prompt assembler (replaces monolithic templates)
- Typed event system (replaces current approach)
- Dependency DAG resolver
- Two-level loop orchestration

Principle: import infrastructure, copy proven logic, build new architecture.

## CLI and Cutover

**During development:** Add `gromit run2 <spec-file>` command (`cmd/gromit/run2.go`). V1's `gromit run` stays untouched.

**Cutover:** When v2 passes acceptance:
1. Delete `internal/runner/` and `internal/pipeline/`
2. Rename `run2` command to `run`
3. Remove old CLI wiring
4. Single PR

## Migration Strategy (Revised)

1. **Scaffold v2 package tree** — interfaces, types, empty stage stubs
2. **Build the two-level loop** — spec_loop and bead_loop with test doubles
3. **Implement stages** — copy proven logic from v1, build new stages fresh
4. **Wire CLI** — `gromit run2` entry point
5. **End-to-end testing** — run v2 on a real spec
6. **Cutover** — delete v1, rename command

Each step produces a testable increment. V1 runs throughout.

## Appendix: Stale Bead Prevention (2026-03-08)

Three fixes prevent the run loop from rebuilding beads whose work is already done:

1. **Cumulative diff (P0):** Accept stage uses `DiffFromBase` instead of `git diff HEAD`. Branch base SHA stored in `.gromit/v2/branch-base` at worktree creation. Diffs capture all committed + uncommitted changes since the branch point.

2. **Gate satisfaction check (P1):** Before proceeding, gate evaluates bead acceptance criteria against the cumulative diff via LLM. Tier escalates by generation: gen0=skip, gen1=haiku, gen2=sonnet, gen3+=opus. Structural beads (refactor, test, rename, etc.) bypass the check to avoid false positives.

3. **Behavioral criteria (P2):** Decompose prompts require acceptance criteria to describe observable behavior, not file paths or code structure. Validation rule flags criteria containing file paths as `criteria_structural` violations.

Design doc: `docs/plans/2026-03-08-stale-bead-prevention-design.md`
