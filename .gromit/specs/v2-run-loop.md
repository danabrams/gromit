---
id: v2-run-loop
source_ideas: [gromit-v2-autonomous-product-engineer]
created: 2026-03-04
accepted: true
---

# V2 Run Loop

## Specification

The v2 run loop replaces the monolithic Gromit orchestrator with a two-level loop built on a refined Stage abstraction. It takes a spec as input and produces code on a branch ready for product owner review. The loop operates at two levels (spec and bead) and runs in isolated git worktrees. V2 is built in a separate `internal/v2/` package tree while v1 remains operational; proven v1 logic (decomposition, validation, escalation, review) is copied and adapted to the new interfaces.

A cycle (as defined in VISION.md) starts when a spec enters the queue and ends when work is presented to the product owner via a Presenter adapter. The system's goal is to get it right the first time — exhausting its own ability to fix, review, and validate before presenting.

### Two-Level Loop

**Outer loop (spec level):**
- Pulls the next spec from the queue whose dependencies are satisfied
- Creates an isolated git worktree for the spec's execution
- Runs Plan and Decompose stages to generate beads from the spec against the live codebase
- Executes beads through the inner loop
- After all beads complete, checks acceptance criteria
- If acceptance criteria are not met, Accept produces a gap analysis and Decompose breaks it into remediation beads; the inner loop continues (subject to generation cap)
- On success: presents completed work to the product owner via the Presenter adapter
- On total failure (generation cap or retry limits exhausted): preserves the branch with a failure summary and triggers Andon escalation

**Inner loop (bead level):**
- Picks the next bead whose dependencies are satisfied
- Processes it through the stage pipeline: Gate, Build, Validate, Review, Epilogue
- Each stage is stateless; inter-stage data flows through git/filesystem, not in-memory state
- Retry policies are stage-configured and loop-enforced

### Everything Is a Spec

All work enters the system as a spec — there are no special cases for maintenance chores, bug batches, or other work types. Every spec goes through Plan and Decompose. This ensures all work items get sharpened acceptance criteria and a concrete plan before execution begins. A maintenance chore spec may be lightweight, but it still gets planned and decomposed against the live codebase.

### Dependency Model

**Spec dependencies:** Specs can declare dependencies on other specs. `gromit run <spec-file>` checks that all dependency specs have been accepted by the product owner before proceeding; if any are unsatisfied, it exits with an error listing the blocking specs. A separate query command (`gromit ready` or equivalent) scans all specs and lists those whose dependencies are satisfied and are eligible to run. Queue management and multi-spec scheduling are out of scope for this spec (see Integration Coordinator in the epic).

**Bead dependencies:** Beads produced by Decompose can declare dependencies on other beads. The inner loop picks the next bead whose dependencies are satisfied. In v2, beads execute sequentially — the dependency DAG determines ordering, not concurrency. Beads must not assume execution order relative to independent siblings; they depend only on their declared dependencies. This constraint preserves the option for future parallel execution without redesigning beads or the dependency model. Parallel bead execution (likely via sub-worktrees branched from the spec branch) is a future optimization, not a v2 concern.

### Worktree Isolation

Each spec executes in its own git worktree, branched from the target integration branch. This provides:
- **Isolation** — multiple specs can execute in parallel without interfering with each other
- **Developer freedom** — the developer's working directory is never touched
- **Clean merge point** — the worktree's branch is the unit of presentation to the product owner

### Stage Abstraction

Every stage implements a uniform interface:

```
Run(ctx, StageRequest) -> (StageResult, error)
```

**StageRequest** carries:
- Bead metadata (id, title, priority, labels, dependencies)
- Model selection
- Iteration number
- Project config
- RetryContext (optional) — populated by the loop on retry iterations. Contains prior failure information (e.g., validation failure summaries, Andon escalation level, time budget remaining). Stages read RetryContext if present and ignore it otherwise. This is the sole mechanism for passing inter-stage retry data — stages remain stateless and the loop is the stateful coordinator. RetryContext is a single structured field, not a grab-bag: it carries only what the retrying stage needs for recovery.

**StageResult** has three fields:
- **Decision** — Proceed, Skip, Block, or Fail. The loop uses this for control flow.
- **Artifacts** — `any`-typed stage-specific data (e.g., Review returns new beads to enqueue, Validate returns failure details). Each stage defines its own concrete artifact struct containing only the data it produces. The loop is the sole consumer of artifacts: it always knows which stage just ran and does a single type-assertion to the expected concrete type. This solves the v1 grab-bag problem (stages forced to populate irrelevant Output fields) while keeping the interface uniform. The type-assertion is safe because it is centralized in the loop, not scattered across stages.
- **Events** — an array of typed, schema-contracted events (cost, telemetry, timing, stage transitions). The loop accumulates these and streams them to subscribers.

Stages are single-shot and stateless. They read codebase state from git/filesystem, not from previous stage outputs. The one exception is RetryContext on StageRequest, which the loop populates with prior failure information on retry iterations. This is control-flow data (what failed and why), not codebase state — the loop is the stateful coordinator and RetryContext is how it communicates retry intent to stages.

### Filesystem Conventions

Stages that produce or consume artifacts beyond git and the Task Tracker follow these conventions within the spec's worktree:

- **Plan output** — `.gromit/v2/plan.md`. Written by Plan, read by Decompose.
- **Accept gap analysis** — `.gromit/v2/gap-analysis.md`. Written by Accept when criteria are not met, read by Decompose to produce remediation beads.

Bead metadata (descriptions, dependencies, status) lives in the Task Tracker (bd), not the filesystem. Build reads the bead's task description from the Task Tracker via the adapter. Code changes are committed to git in the worktree. Validation commands run against the worktree's working tree.

### Retry

Stages declare their retry semantics via configuration:

- **MaxRetries** — how many times the loop may retry this stage
- **RetryWith** — an ordered list of stages to rerun before retrying the failed stage (e.g., Validate retries with Build: on Validate failure, rerun Build then Validate, up to MaxRetries)

The loop enforces retry. Stages themselves do not retry. This keeps stage implementations simple and retry logic centralized and testable.

### Stages

**Spec-level stages** (run once when a spec becomes current):

1. **Plan** — generates an implementation plan from the spec and the live codebase. The spec's Architecture Direction and Test Strategy sections inform the plan.
2. **Decompose** — breaks the plan into ordered beads with dependency declarations and enqueues them via the Task Tracker adapter.

**Bead-level stages** (run per bead):

1. **Gate** — checks whether the bead is still relevant, already done, or out of scope. Returns Skip or Block to short-circuit, Proceed to continue.
2. **Build** — invokes the LLM to do the work. Prompt is composed from base, project, and instance layers. Model escalation (e.g., haiku to sonnet to opus) is internal to the Build stage.
3. **Validate** — runs configured validation commands (build, test, lint, format). Language-agnostic; commands come from project config. Returns pass/fail per step.
4. **Review** — LLM self-review against project rules and spec. Findings are classified as in-scope (affects the current spec's acceptance criteria) or out-of-scope (tangential issues noticed during review). In-scope findings become new beads, enqueued via the Task Tracker adapter. Out-of-scope findings are emitted as `ReviewFinding` events with `in_scope: false` and are not acted on — they surface in the Present stage summary for the product owner.
5. **Epilogue** — closes the bead via the Task Tracker adapter, emits telemetry events.

**Spec-level completion:**

6. **Accept** — after all beads complete, verifies acceptance criteria from the spec against the codebase using an LLM evaluation. If criteria are not met, Accept produces a gap analysis describing what's missing, then Decompose runs against the gap analysis to produce properly scoped, dependency-ordered remediation beads. Plan is skipped because Accept's gap analysis serves the same role — it already identifies what needs to change against the live codebase. The inner loop then continues with the new beads (subject to generation cap).
7. **Present** — on success, presents completed work to the product owner via the Presenter adapter with a summary of what was done, acceptance criteria results, out-of-scope review findings, and a link to the branch/diff.

### Budget Integration

The loop checks time and cost budgets at two points: before starting each bead (outer/inner loop boundary) and before retrying a stage. Budget policy (limits, escalation thresholds) is defined by the Andon spec, not this spec. The loop's responsibility is to check remaining budget at control points and trigger Andon escalation when exhausted. RetryContext carries remaining budget so stages have awareness of runway. When budget is low at a bead boundary, the loop may skip remaining lower-priority beads and proceed directly to Accept to evaluate whether completed work is sufficient.

### Scope Control

The loop has built-in limits to prevent infinite spirals:

- **Generation cap** — beads are tagged with a generation number. Original beads from Decompose are generation 0. Beads created by Review are generation N+1 of their parent. Beads created by Accept are generation `max(current bead generations) + 1`. The generation cap (configurable, default 3) is a *window*, not an absolute ceiling: when the highest generation reaches `start_generation + cap`, the loop stops creating new beads and pauses for human review. If the human re-runs `gromit run <spec-file>`, the loop resumes from the current state — the new start generation is whatever the existing beads are at, and the cap window resets. This means generations can grow indefinitely across human checkpoints, but never more than `cap` generations without review.

When the cap is hit, the loop preserves the branch with a summary of what was attempted and what remains, and emits an Andon event. This is a structured pause, not a crash — the system recognizes it needs human review before continuing.

### Adapter Interfaces

All external dependencies are accessed through adapter interfaces. Stages and the loop depend on interfaces, not concrete implementations. This keeps the core loop testable and allows swapping implementations.

**Required adapters:**

- **LLM Provider** — invoke an LLM with a prompt, receive a response. Supports multiple providers (Claude, Gemini, etc.). Build, Plan, Decompose, Review, and Accept stages use this.
- **Task Tracker** — create, close, query, and update beads/tasks with dependency tracking. Current implementation uses bd; the interface allows replacing it. Decompose, Review, Gate, and Epilogue stages use this.
- **Presenter** — present completed work for human review. Implementations could include GitHub PR creation, Slack notification, TUI prompt, or others. The Present stage uses this.
- **Git** — branch, commit, diff, worktree create/remove, merge operations. The loop and multiple stages use this.

Config is not an adapter — it is loaded once at startup from `internal/config/` and passed through StageRequest. There is no need to swap config implementations at runtime.

### Prompt System

Prompts are composed from layered sources rather than monolithic per-stage templates:

- **Base layer** — stage-specific instructions (what the stage does, its constraints, output format). Defined by gromit, same across all projects.
- **Project layer** — project-specific context (rules, conventions, language, architecture patterns). Defined by the project (e.g., CLAUDE.md, RULES.md).
- **Instance layer** — per-invocation context (this bead's task description, prior validation failures, acceptance criteria, relevant code context). Assembled by the loop from the current state.
- **Fragment layer** — optional cross-cutting concerns that can be mixed in (security checklist, performance guidelines, accessibility requirements). Configured per project.

The prompt assembler composes these layers for the target model. Each layer is a separate template/file, making individual concerns independently editable and testable.

### Event System

The loop emits typed, schema-contracted events as it runs. Events follow a defined schema with a versioned contract. Consumers subscribe to the event stream for real-time observation.

**Event categories:**
- **Lifecycle** — SpecStarted, SpecCompleted, SpecFailed, BeadStarted, BeadCompleted
- **Stage** — StageStarted, StageCompleted, StageFailed, StageRetrying
- **Validation** — ValidationPassed, ValidationFailed, ValidationStepResult
- **Review** — ReviewFinding (with `in_scope` boolean, description, affected files; in-scope findings also produce BeadCreated events), BeadCreated
- **Scope** — GenerationCapReached, AndonTriggered
- **Telemetry** — LLMInvocation (model, tokens, cost, duration), StageTimings

**Consumers:**
- **CLI** — renders real-time progress display from the event stream
- **API** — exposes the event stream for external consumers
- **Log** — writes events to the iteration log file for post-hoc analysis

Every event carries a `schema_version` field. Consumers must handle unknown event types gracefully (ignore, don't crash). This enables forward compatibility — new event types can be added without breaking existing consumers.

### Language Agnosticism

The run loop is not coupled to any programming language. Stages that execute external commands (Validate, and potentially others) use configurable commands from project config:

```yaml
validate:
  steps:
    - name: build
      command: "go build ./..."
    - name: test
      command: "go test ./..."
    - name: lint
      command: "golangci-lint run"
    - name: format
      command: "gofmt -l ."
```

The loop does not interpret command output; it passes raw output to the LLM for analysis when needed. Prompts provide language/project context to the LLM via the project and instance layers.

### Completion and Failure

**Success path:** All beads executed → all validation passes → all review findings resolved → acceptance criteria verified → work presented to product owner via Presenter adapter.

**Failure path:** When the loop hits a scope control limit (generation cap) or exhausts all retries, it:
- Preserves the branch in its current state (partial work)
- Emits an AndonTriggered event with failure context
- Generates a failure summary describing what was attempted, what failed, and why
- Presents the failure via the Presenter adapter for human diagnosis

This counts as a cycle that required human tactical intervention per VISION.md metrics. The Andon spec will define escalation levels and recovery procedures.

### Enriched Spec Format

Specs include additional high-level sections beyond the current format:

- **Architecture Direction** — high-level structural guidance (e.g., "this should be a new package," "reuse the existing Stage interface," "must support future concurrency"). Informs the just-in-time planner without prescribing implementation details.
- **Test Strategy** — high-level testing approach (e.g., "needs integration tests for the full loop without a real LLM," "unit tests for retry logic"). Guides the planner on what test coverage to target.
- **Dependencies** — specs this spec depends on (must be accepted before this spec can be planned).

These sections capture intent and constraints. The Plan stage fills in concrete details when it runs against the live codebase.

### CLI Entry Point

The primary CLI command is `gromit run <spec-file>`. This:
1. Loads the spec and checks dependency specs are satisfied
2. Creates a worktree
3. Runs the full loop (Plan → Decompose → bead execution → Accept → Present)
4. Streams events to CLI display and log
5. Returns exit code 0 on success (presented to PO), non-zero on failure

`gromit ready` lists specs whose dependencies are satisfied and are eligible to run. This is a query, not an execution command.

## Acceptance Criteria

- The run loop processes a spec end-to-end: Plan, Decompose, then execute beads through Gate, Build, Validate, Review, Epilogue, then Accept, then Present
- Everything enters as a spec — no special batch or chore handling
- Planning and decomposition run just-in-time when a spec becomes current, not ahead of time
- Each spec executes in an isolated git worktree
- `gromit run` exits with an error listing blocking specs when dependencies are unsatisfied
- `gromit ready` lists specs eligible to run (all dependencies satisfied)
- Beads with unsatisfied dependencies are not picked up by the inner loop
- After all beads in a spec complete, acceptance criteria are checked; if not met, Accept produces a gap analysis and Decompose breaks it into remediation beads, then the loop continues
- Generation cap stops bead creation and triggers Andon when reached
- The loop checks time/cost budgets at bead boundaries and before stage retries, triggering Andon when exhausted
- On success, work is presented to the product owner via the Presenter adapter
- On failure, the branch is preserved with a failure summary and Andon is triggered
- All stages implement the uniform Stage interface (Run with StageRequest/StageResult)
- StageResult carries Decision, typed Artifacts, and typed Events array
- Validate stage runs configurable external commands (not hardcoded to any language)
- Retry is configured per-stage (MaxRetries, RetryWith) and enforced by the loop
- Build stage handles model escalation internally
- Review stage classifies findings as in-scope or out-of-scope; only in-scope findings become beads
- Out-of-scope review findings are emitted as events and included in the Present stage summary
- Stages are stateless; inter-stage data flows through git/filesystem
- Prompts are composed from base, project, instance, and fragment layers
- The loop emits typed events with a schema contract
- Events are consumable by CLI, API, and log subscribers
- Events carry a `schema_version` field; consumers handle unknown event types gracefully
- All external dependencies are accessed through adapter interfaces (LLM Provider, Task Tracker, Presenter, Git)
- Adapters can be swapped without changing stage or loop code
- Config is loaded directly from `internal/config/`, not through an adapter

## Decisions

1. **Two-level loop over monolithic orchestrator.** The v1 orchestrator mixes spec-level concerns (acceptance, planning) with bead-level concerns (retry, stage sequencing) in 700+ lines. Separating these into outer (spec) and inner (bead) loops makes each level independently testable and the control flow explicit.

2. **Everything is a spec.** All work enters the system as a spec — no special cases for batches, chores, or pre-decomposed bead lists. This forces all work through Plan and Decompose, ensuring sharpened acceptance criteria and concrete plans. Simplifies the loop to one input type and one flow.

3. **Just-in-time planning.** Planning and decomposition run when a spec becomes current, not when it's enqueued. This ensures plans reflect the actual codebase state at execution time, avoiding stale plans.

4. **Worktree isolation.** Each spec runs in its own git worktree. This enables parallel execution, keeps the developer's working directory clean, and provides a clean branch as the unit of presentation.

5. **DAG dependencies for specs and beads.** Both specs and beads support dependency declarations, enabling a DAG of work rather than a flat queue. Specs wait for dependency specs to be accepted; beads wait for dependency beads to complete. This supports parallelism and correct ordering.

6. **Uniform Stage interface with `any`-typed Artifacts.** Stages return a uniform StageResult with Decision (for control flow), `any`-typed Artifacts (for stage-specific data), and Events (for telemetry). Each stage defines its own concrete artifact struct; the loop is the sole consumer and does a single type-assertion per stage. This solves the v1 grab-bag problem (monolith Output struct with 25+ fields most stages ignore) while keeping the Stage interface uniform. Stages do not consume other stages' artifacts — the loop collects them.

7. **Paired retry over retry groups.** Stages declare RetryWith (stages to rerun before retrying) rather than the loop defining explicit stage groups. This keeps retry semantics co-located with the stage that needs them and avoids a separate grouping concept.

8. **Escalation internal to Build.** Model escalation (haiku to sonnet to opus) is Build's concern, not the loop's. This keeps the loop's retry logic simple (rerun stages) and escalation logic encapsulated.

9. **Git as ground truth for codebase state.** Stages read codebase state from git/filesystem, not from in-memory structs passed between stages. StageRequest carries bead metadata, model selection, iteration number, config, and RetryContext. RetryContext is the sole exception: it carries control-flow data (prior failure summaries, escalation level, time budget) that the loop populates on retry iterations. This distinguishes codebase state (git) from retry coordination (in-memory via the loop), keeping stages independently testable while giving retries the context they need.

10. **Language-agnostic validation.** Validate runs configurable commands from project config. The loop does not parse or interpret command output. Prompts provide language/project context to the LLM. This allows gromit to work with any language or toolchain.

11. **Plan and Decompose become loop stages.** In v2, planning and decomposition are stages of the run loop rather than separate CLI commands. The existing `gromit plan` and `gromit decompose` commands are removed. This simplifies the interface and ensures planning always happens against the live codebase.

12. **Review auto-creates beads.** When Review finds issues, it creates new beads and enqueues them via the Task Tracker adapter, rather than returning findings for the loop to interpret. This keeps the Review stage self-contained and the loop simple.

13. **Adapter interfaces for external dependencies, not config.** LLM Provider, Task Tracker, Presenter, and Git are accessed through interfaces. Config is loaded once from `internal/config/` and passed through StageRequest — no adapter needed since there is no runtime-swap use case. This makes the core loop testable with mocks while avoiding unnecessary abstraction.

14. **Cycle ends at presentation.** Aligned with VISION.md, a cycle ends when work is presented to the product owner. Post-presentation feedback (if it's an implementation gap) is a new cycle. The loop's job is to get it right the first time.

15. **Clean implementation in parallel package tree.** V2 is built in `internal/v2/` while v1 stays untouched in `internal/runner/` and `internal/pipeline/`. This is driven by three factors: (a) v1 must remain stable because it builds v2 (bootstrapping constraint), (b) wrapping v1 stages in translation adapters adds throwaway complexity, (c) a separate tree avoids half-v1/half-v2 limbo. Proven v1 logic (decomposition, validation, model escalation, review) is copied and adapted to v2 interfaces. Infrastructure packages (`internal/config/`, `internal/bead/`) are imported directly. At cutover, v1 packages are deleted and the CLI command is renamed. See `docs/plans/2026-03-04-v2-run-loop-clean-implementation-design.md`.

16. **Generation cap as checkpoint window, not absolute ceiling.** The generation cap (default 3) limits consecutive generations within a single run. When hit, the loop pauses for human review. Re-running `gromit run` resets the window from the current generation — generations can grow indefinitely across human checkpoints. This balances autonomy (the system works without supervision for `cap` generations) with safety (humans review before unbounded continuation).

17. **Composable prompt system.** Prompts are assembled from four layers (base, project, instance, fragment) rather than monolithic per-stage templates. This makes individual concerns independently editable, supports cross-cutting concerns via fragments, and separates what gromit provides from what the project provides.

18. **Accept feeds Decompose, skips Plan.** When Accept finds unmet acceptance criteria, it produces a gap analysis that feeds into Decompose to produce properly scoped remediation beads. Plan is skipped because Accept's gap analysis already identifies what needs to change against the live codebase — running Plan would redundantly re-analyze the same state. This preserves decomposition granularity for remediation work while avoiding unnecessary planning overhead. Review-created beads remain direct (single-bead fixes from code review findings), which is appropriate for their narrower scope.

19. **Budget integration points, not budget policy.** The v2 loop defines where budgets are checked (bead boundaries, stage retries) and what happens when they're exhausted (Andon escalation), but budget policy (time limits, cost caps, escalation thresholds) is owned by the Andon spec. This avoids duplicating concerns and keeps budget policy in one place.

20. **Sequential bead execution in v2, parallel-ready design.** The inner loop executes beads one at a time; the dependency DAG determines order. Beads must not assume execution order relative to independent siblings — they depend only on declared dependencies. This constraint makes future parallel execution possible (likely via sub-worktrees) without redesigning the bead model. Parallelism is deferred until v2 runs smoothly.

21. **Minimal filesystem conventions for new artifacts only.** Plan and Accept gap analysis are the only new filesystem artifacts — they live at known paths in the worktree (`.gromit/v2/`). Bead metadata stays in the Task Tracker (bd). Code state stays in git. This keeps the filesystem layout small and avoids duplicating what bd and git already manage.

22. **Review scoped to spec acceptance criteria.** Review only creates beads for findings that affect the current spec's acceptance criteria. Tangential findings are emitted as `ReviewFinding` events with `in_scope: false` and surfaced in the Present stage summary for the product owner. This prevents scope creep within the loop while ensuring nothing is silently dropped.

23. **Single-spec execution with dependency gate, not queue management.** `gromit run` executes one spec and fails fast if dependencies aren't met. `gromit ready` queries which specs are eligible. Queue management and multi-spec scheduling belong to the Integration Coordinator (separate epic concern). This keeps the v2 run loop focused on execution.

24. **RetryContext on StageRequest over filesystem-mediated retry data.** Retry context (validation failures, escalation level, time budget remaining) flows through a structured RetryContext field on StageRequest rather than through filesystem files. The loop is the stateful coordinator — it captures stage results, builds RetryContext, and passes it on retry. Writing retry data to the filesystem just to have stages read it back would be ceremony without benefit. This is compatible with the immutable pipeline (iteration snapshots can include RetryContext for debugging) and Andon escalation (RetryContext carries escalation level and time budget).

25. **Event schema versioning via field, not framework.** Every event carries a `schema_version` field. Consumers must handle unknown event types gracefully (ignore, don't crash). This enables forward compatibility without a versioning framework.

19. **Typed event stream with schema contract.** The loop emits typed events (not freeform logs) with a versioned schema. Consumers (CLI, API, log) subscribe to the stream. This enables real-time observation, structured post-hoc analysis, and guarantees consumer compatibility across versions.

## Research & Context

### Current State

The v1 orchestrator lives in `internal/runner/orchestrator.go` and is ~700+ lines with hand-rolled retry loops for Validate, WiringGate, and RegressionGate tangled into the main Run method. The stage interface (`internal/pipeline/stage.go`) is clean — `Run(ctx, Input) (Output, error)` — but the Output struct is a grab-bag with fields most stages don't use.

V2 is built in `internal/v2/` as a parallel package tree. V1 packages remain untouched until cutover.

**Import from v1** (use directly):
- `internal/config/` — YAML config loading
- `internal/bead/` — bd CLI integration (behind TaskTracker adapter)

**Copy and adapt from v1** (proven logic, rewritten to v2 interfaces):
- `internal/pipeline/decompose/` — decomposition logic, works well
- `internal/pipeline/execute/` — model escalation logic for Build stage
- `internal/pipeline/validate/` — validation command execution for Validate stage
- `internal/pipeline/review/` — review prompt logic for Review stage

**Build fresh in `internal/v2/`:**
- Two-level loop orchestration (`loop/`)
- Stage interface and types (`stage/`)
- Plan, Gate, Epilogue, Accept, Present stages
- Adapter interfaces (`adapter/`): LLM Provider, TaskTracker, Presenter, Git, Config
- Composable prompt assembler (`prompt/`)
- Typed event system (`event/`)
- Dependency DAG resolver (`dep/`)

**CLI:** `gromit run2` during development (`cmd/gromit/run2.go`), renamed to `gromit run` at cutover.

**Delete at cutover:** `internal/runner/`, `internal/pipeline/`, old `cmd/gromit/run.go`.

### VISION.md Alignment

This spec directly supports the VISION.md outcomes:
- **<=10% human tactical intervention**: The loop self-corrects through Review, retry, acceptance criteria verification, and Andon escalation before requiring human help
- **>=95% first integration pass**: Just-in-time planning, exhaustive validation, and self-review maximize the chance of getting it right the first time
- **>=90% accepted without rework**: Acceptance criteria checking ensures the spec is met before presentation

### Migration Strategy

The migration builds v2 in `internal/v2/` while v1 stays operational:

1. **Scaffold v2 package tree** — interfaces, types, empty stage stubs
2. **Build the two-level loop** — spec_loop and bead_loop with test doubles
3. **Implement stages** — copy proven logic from v1 (decompose, validate, build, review), build new stages fresh (plan, gate, epilogue, accept, present)
4. **Wire CLI** — `gromit run2` entry point
5. **End-to-end testing** — run v2 on a real spec
6. **Cutover** — delete v1 packages, rename `run2` to `run`

Each step produces a testable increment. V1 runs throughout, including as the tool that builds v2.

### Epic Context

This spec implements the "Goal-Oriented Orchestration" and foundational infrastructure from the `gromit-v2-autonomous-product-engineer` epic. Other epic outcomes (Immutable Pipeline, Recursive Quality/Andon escalation, Integration Coordinator) will be separate specs layered on top of this core loop.
