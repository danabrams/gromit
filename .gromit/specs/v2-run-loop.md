---
id: v2-run-loop
source_ideas: [gromit-v2-autonomous-product-engineer]
created: 2026-03-04
---

# V2 Run Loop

## Specification

The v2 run loop evolves the existing Gromit orchestrator in place, replacing the monolithic runner with a two-level loop built on a refined Stage abstraction. It takes a spec as input and produces code on a branch ready for product owner review. The loop operates at two levels (spec and bead) and runs in isolated git worktrees. Existing stage implementations, adapters, and infrastructure are refactored incrementally to fit the new architecture.

A cycle (as defined in VISION.md) starts when a spec enters the queue and ends when work is presented to the product owner via a Presenter adapter. The system's goal is to get it right the first time — exhausting its own ability to fix, review, and validate before presenting.

### Two-Level Loop

**Outer loop (spec level):**
- Pulls the next spec from the queue whose dependencies are satisfied
- Creates an isolated git worktree for the spec's execution
- Runs Plan and Decompose stages to generate beads from the spec against the live codebase
- Executes beads through the inner loop
- After all beads complete, checks acceptance criteria
- If acceptance criteria are not met, generates additional beads and continues (subject to generation cap)
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

**Spec dependencies:** Specs can declare dependencies on other specs. A spec cannot be planned or enqueued until all its dependency specs have been accepted by the product owner. This enables a DAG of work at the spec level.

**Bead dependencies:** Beads produced by Decompose can declare dependencies on other beads. The inner loop only picks up beads whose dependencies are satisfied. This enables parallelism — independent beads can execute concurrently (in separate worktrees or sequentially, depending on configuration), while dependent beads wait.

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

**StageResult** has three fields:
- **Decision** — Proceed, Skip, Block, or Fail. The loop uses this for control flow.
- **Artifacts** — stage-specific typed data (e.g., Review returns new beads to enqueue, Validate returns failure details). The loop collects artifacts; stages do not consume other stages' artifacts.
- **Events** — an array of typed, schema-contracted events (cost, telemetry, timing, stage transitions). The loop accumulates these and streams them to subscribers.

Stages are single-shot and stateless. They read state from git/filesystem, not from previous stage outputs.

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
4. **Review** — LLM self-review against project rules and spec. Auto-creates new beads from review findings and enqueues them via the Task Tracker adapter.
5. **Epilogue** — closes the bead via the Task Tracker adapter, emits telemetry events.

**Spec-level completion:**

6. **Accept** — after all beads complete, verifies acceptance criteria from the spec against the codebase using an LLM evaluation. If criteria are not met, generates new beads describing what's missing and the inner loop continues (subject to generation cap).
7. **Present** — on success, presents completed work to the product owner via the Presenter adapter with a summary of what was done, acceptance criteria results, and a link to the branch/diff.

### Scope Control

The loop has built-in limits to prevent infinite spirals:

- **Generation cap** — beads are tagged with a generation number. Original beads from Decompose are generation 0. Beads created by Review are generation N+1 of their parent. Beads created by Accept are also a new generation. When the generation cap is reached (configurable, default 3), the loop stops creating new beads and triggers Andon escalation. The Andon spec will define the escalation behavior; this spec defines the detection and triggering.

When a limit is hit, the loop preserves the branch with a failure summary and emits an Andon event. This is a structured failure, not a crash — the system recognizes it cannot self-correct and escalates.

### Adapter Interfaces

All external dependencies are accessed through adapter interfaces. Stages and the loop depend on interfaces, not concrete implementations. This keeps the core loop testable and allows swapping implementations.

**Required adapters:**

- **LLM Provider** — invoke an LLM with a prompt, receive a response. Supports multiple providers (Claude, Gemini, etc.). Build, Plan, Decompose, Review, and Accept stages use this.
- **Task Tracker** — create, close, query, and update beads/tasks with dependency tracking. Current implementation uses bd; the interface allows replacing it. Decompose, Review, Gate, and Epilogue stages use this.
- **Presenter** — present completed work for human review. Implementations could include GitHub PR creation, Slack notification, TUI prompt, or others. The Present stage uses this.
- **Git** — branch, commit, diff, worktree create/remove, merge operations. The loop and multiple stages use this.
- **Config** — load project configuration (validation commands, model settings, prompt paths). The loop loads config once and passes it through StageRequest.

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
- **Review** — ReviewFinding, BeadCreated
- **Scope** — GenerationCapReached, AndonTriggered
- **Telemetry** — LLMInvocation (model, tokens, cost, duration), StageTimings

**Consumers:**
- **CLI** — renders real-time progress display from the event stream
- **API** — exposes the event stream for external consumers
- **Log** — writes events to the iteration log file for post-hoc analysis

The event schema is a contract: consumers depend on it, and changes require versioning.

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

## Acceptance Criteria

- The run loop processes a spec end-to-end: Plan, Decompose, then execute beads through Gate, Build, Validate, Review, Epilogue, then Accept, then Present
- Everything enters as a spec — no special batch or chore handling
- Planning and decomposition run just-in-time when a spec becomes current, not ahead of time
- Each spec executes in an isolated git worktree
- Specs with unsatisfied dependencies are not planned or executed
- Beads with unsatisfied dependencies are not picked up by the inner loop
- After all beads in a spec complete, acceptance criteria are checked; if not met, new beads are generated and the loop continues
- Generation cap stops bead creation and triggers Andon when reached
- On success, work is presented to the product owner via the Presenter adapter
- On failure, the branch is preserved with a failure summary and Andon is triggered
- All stages implement the uniform Stage interface (Run with StageRequest/StageResult)
- StageResult carries Decision, typed Artifacts, and typed Events array
- Validate stage runs configurable external commands (not hardcoded to any language)
- Retry is configured per-stage (MaxRetries, RetryWith) and enforced by the loop
- Build stage handles model escalation internally
- Review stage auto-creates beads from findings
- Stages are stateless; inter-stage data flows through git/filesystem
- Prompts are composed from base, project, instance, and fragment layers
- The loop emits typed events with a schema contract
- Events are consumable by CLI, API, and log subscribers
- All external dependencies are accessed through adapter interfaces (LLM Provider, Task Tracker, Presenter, Git, Config)
- Adapters can be swapped without changing stage or loop code

## Decisions

1. **Two-level loop over monolithic orchestrator.** The v1 orchestrator mixes spec-level concerns (acceptance, planning) with bead-level concerns (retry, stage sequencing) in 700+ lines. Separating these into outer (spec) and inner (bead) loops makes each level independently testable and the control flow explicit.

2. **Everything is a spec.** All work enters the system as a spec — no special cases for batches, chores, or pre-decomposed bead lists. This forces all work through Plan and Decompose, ensuring sharpened acceptance criteria and concrete plans. Simplifies the loop to one input type and one flow.

3. **Just-in-time planning.** Planning and decomposition run when a spec becomes current, not when it's enqueued. This ensures plans reflect the actual codebase state at execution time, avoiding stale plans.

4. **Worktree isolation.** Each spec runs in its own git worktree. This enables parallel execution, keeps the developer's working directory clean, and provides a clean branch as the unit of presentation.

5. **DAG dependencies for specs and beads.** Both specs and beads support dependency declarations, enabling a DAG of work rather than a flat queue. Specs wait for dependency specs to be accepted; beads wait for dependency beads to complete. This supports parallelism and correct ordering.

6. **Uniform Stage interface with typed Artifacts.** Stages return a uniform StageResult with Decision (for control flow), typed Artifacts (for stage-specific data), and Events (for telemetry). Stages do not consume other stages' artifacts — the loop collects them. This keeps stages decoupled.

7. **Paired retry over retry groups.** Stages declare RetryWith (stages to rerun before retrying) rather than the loop defining explicit stage groups. This keeps retry semantics co-located with the stage that needs them and avoids a separate grouping concept.

8. **Escalation internal to Build.** Model escalation (haiku to sonnet to opus) is Build's concern, not the loop's. This keeps the loop's retry logic simple (rerun stages) and escalation logic encapsulated.

9. **Git as ground truth.** Stages read state from git/filesystem, not from in-memory structs passed between stages. StageRequest carries only bead metadata, model selection, iteration number, and config. This aligns with the principle "state in files, not memory" and makes stages independently testable.

10. **Language-agnostic validation.** Validate runs configurable commands from project config. The loop does not parse or interpret command output. Prompts provide language/project context to the LLM. This allows gromit to work with any language or toolchain.

11. **Plan and Decompose become loop stages.** In v2, planning and decomposition are stages of the run loop rather than separate CLI commands. The existing `gromit plan` and `gromit decompose` commands are removed. This simplifies the interface and ensures planning always happens against the live codebase.

12. **Review auto-creates beads.** When Review finds issues, it creates new beads and enqueues them via the Task Tracker adapter, rather than returning findings for the loop to interpret. This keeps the Review stage self-contained and the loop simple.

13. **Adapter interfaces for all external dependencies.** LLM Provider, Task Tracker, Presenter, Git, and Config are all accessed through interfaces. This makes the core loop testable with mocks and allows swapping implementations (e.g., replacing bd with a different tracker, or presenting via Slack instead of GitHub PRs).

14. **Cycle ends at presentation.** Aligned with VISION.md, a cycle ends when work is presented to the product owner. Post-presentation feedback (if it's an implementation gap) is a new cycle. The loop's job is to get it right the first time.

15. **In-place evolution over clean start.** v2 evolves the existing codebase rather than rewriting from scratch. The orchestrator is replaced, the Stage interface is refined (StageResult with Decision/Artifacts/Events), and existing stage implementations are adapted incrementally. This preserves battle-tested edge case handling, working adapters, CLI wiring, and config infrastructure while achieving all architectural goals. The risk of half-v1-half-v2 limbo is managed by replacing the orchestrator first (the core problem), then migrating stages one at a time.

16. **Generation cap with Andon trigger.** Scope control uses a generation cap on beads to prevent review/accept spirals. When the cap is reached, the loop triggers Andon escalation rather than silently stopping. The Andon spec defines what happens next; this spec defines detection and triggering.

17. **Composable prompt system.** Prompts are assembled from four layers (base, project, instance, fragment) rather than monolithic per-stage templates. This makes individual concerns independently editable, supports cross-cutting concerns via fragments, and separates what gromit provides from what the project provides.

18. **Typed event stream with schema contract.** The loop emits typed events (not freeform logs) with a versioned schema. Consumers (CLI, API, log) subscribe to the stream. This enables real-time observation, structured post-hoc analysis, and guarantees consumer compatibility across versions.

## Research & Context

### Current State

The v1 orchestrator lives in `internal/runner/orchestrator.go` and is ~700+ lines with hand-rolled retry loops for Validate, WiringGate, and RegressionGate tangled into the main Run method. The stage interface (`internal/pipeline/stage.go`) is clean — `Run(ctx, Input) (Output, error)` — but the Output struct is a grab-bag with fields most stages don't use.

**Replace:**
- `internal/runner/orchestrator.go` — the monolithic orchestrator is replaced with the two-level loop
- `internal/pipeline/stage.go` — the Stage interface is refined (StageResult with Decision/Artifacts/Events)
- `internal/pipeline/types.go` — Input/Output types replaced with StageRequest/StageResult

**Refactor and adapt:**
- `internal/pipeline/execute/` — becomes the Build stage, adapted to new interface; escalation logic preserved
- `internal/pipeline/validate/` — becomes the Validate stage, adapted to new interface; validation step logic preserved
- `internal/pipeline/review/` — becomes the Review stage, adapted to new interface; review logic preserved, bead auto-creation added
- `internal/pipeline/prepare/` — becomes the Gate stage, adapted to new interface
- `internal/pipeline/epilogue/` — becomes the Epilogue stage, adapted to new interface
- `internal/runner/adapters.go` — adapter pattern preserved, interfaces formalized for LLM Provider, Task Tracker, Presenter, Git

**Preserve as-is (initially):**
- `internal/config/` — YAML config loading
- `internal/bead/` — bd CLI integration (behind Task Tracker interface)
- `internal/events/` — event system (evolved to typed schema)
- `internal/prompt/` — prompt rendering (evolved to composable layers)
- `cmd/gromit/` — CLI wiring

**Add new:**
- Plan and Decompose stages (spec-level)
- Accept and Present stages (spec-level completion)
- Spec dependency resolution
- Bead dependency DAG and scheduling
- Generation tracking and cap enforcement
- Presenter adapter interface and initial implementation

The v1 pipeline also has a separate `Pipeline` type for interactive workflows (plan, review, decompose) which is unrelated to the build-loop stages — a naming confusion that v2 resolves.

### VISION.md Alignment

This spec directly supports the VISION.md outcomes:
- **<=10% human tactical intervention**: The loop self-corrects through Review, retry, acceptance criteria verification, and Andon escalation before requiring human help
- **>=95% first integration pass**: Just-in-time planning, exhaustive validation, and self-review maximize the chance of getting it right the first time
- **>=90% accepted without rework**: Acceptance criteria checking ensures the spec is met before presentation

### Migration Strategy

The migration follows this order:
1. **Replace the orchestrator** — write the new two-level loop with v2 Stage interface, initially wrapping existing stage implementations in adapters that translate between v1 Input/Output and v2 StageRequest/StageResult
2. **Migrate stages one at a time** — refactor each stage to implement the v2 interface natively, removing the translation adapter
3. **Add new stages** — Plan, Decompose, Accept, Present
4. **Evolve infrastructure** — prompt system (composable layers), event system (typed schema), dependency DAG
5. **Remove v1 artifacts** — old orchestrator, grab-bag Output type, Pipeline type for interactive workflows

Each step produces a working system. The loop works end-to-end after step 1, even though individual stages are still v1 internally.

### Epic Context

This spec implements the "Goal-Oriented Orchestration" and foundational infrastructure from the `gromit-v2-autonomous-product-engineer` epic. Other epic outcomes (Immutable Pipeline, Recursive Quality/Andon escalation, Integration Coordinator) will be separate specs layered on top of this core loop.
