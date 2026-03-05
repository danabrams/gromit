---
id: v2-run-loop
source_spec: v2-run-loop
created: 2026-03-05
decomposed: false
---

# V2 Run Loop Implementation Plan

**Goal:** Replace the monolithic v1 orchestrator with a two-level loop (spec + bead) built on a uniform Stage interface, typed artifacts, adapter-based dependencies, and a typed event system.

**Architecture:** Clean parallel package tree at `internal/v2/` with two-level loop orchestration (SpecLoop + BeadLoop), 9 stages implementing a uniform `Run(ctx, StageRequest) -> (StageResult, error)` interface, 4 adapter interfaces for external dependencies, a layered prompt assembler, dependency DAG resolver, and typed event system with schema versioning.

**Tech Stack:** Go, Claude CLI (LLM), bd CLI (task tracker), gh CLI (presenter), git (version control)

**Spec:** `.gromit/specs/v2-run-loop.md`

---

## Architecture

**Overview:**
V2 is a clean parallel package tree at `internal/v2/` implementing a two-level loop (spec + bead) with a uniform Stage interface, typed artifacts, adapter-based external dependencies, and a typed event system. V1 remains untouched until cutover.

**Package Tree:**
```
internal/v2/
├── stage/                  # Stage interface, StageRequest, StageResult, Decision
│   ├── plan/               # Plan stage: spec + codebase -> plan.md
│   ├── decompose/          # Decompose stage: plan -> beads (adapted from v1)
│   ├── gate/               # Gate stage: skip/block/proceed checks
│   ├── build/              # Build stage: LLM code authoring + escalation (adapted from v1)
│   ├── validate/           # Validate stage: run commands, report pass/fail (adapted from v1)
│   ├── review/             # Review stage: LLM review, in/out-scope classification (adapted from v1)
│   ├── epilogue/           # Epilogue stage: close bead, emit telemetry
│   ├── accept/             # Accept stage: verify acceptance criteria, gap analysis
│   └── present/            # Present stage: invoke Presenter adapter
├── adapter/                # Adapter interfaces + concrete implementations
│   ├── llm/                # LLMProvider interface + Claude implementation
│   ├── tasktracker/        # TaskTracker interface + bd implementation
│   ├── presenter/          # Presenter interface + GitHub PR implementation
│   └── git/                # Git interface + exec-based implementation
├── loop/                   # Two-level loop orchestration
│   ├── spec_loop.go        # Outer loop: Plan -> Decompose -> BeadLoop -> Accept -> Present
│   └── bead_loop.go        # Inner loop: Gate -> Build -> Validate -> Review -> Epilogue
├── event/                  # Typed event system with schema versioning
├── prompt/                 # Layered prompt assembler (base/project/instance/fragment)
├── dep/                    # Dependency DAG resolver for bead ordering
└── spec/                   # Spec loading, parsing, dependency checking, acceptance tracking
```

**Key Types:**

```go
// stage/stage.go
type Stage interface {
    Run(ctx context.Context, req StageRequest) (StageResult, error)
}

type StageRequest struct {
    Bead         BeadInfo
    Model        string
    Iteration    int
    Config       *config.Config
    RetryContext *RetryContext   // nil on first attempt, populated on retries
}

type StageResult struct {
    Decision  Decision        // Proceed, Skip, Block, Fail
    Artifacts any             // stage-specific concrete struct, type-asserted by loop
    Events    []event.Event   // typed events for subscribers
}

type RetryContext struct {
    PriorFailures   []string
    EscalationLevel int
    Attempt         int
}

type RetryConfig struct {
    MaxRetries int
    RetryWith  []string  // stage names to rerun before retrying
}
```

**Adapter Interfaces:**

```go
// adapter/llm/llm.go - Invoke (non-streaming) + StreamInvoke (streaming)
// adapter/tasktracker/tasktracker.go - NextBead, CreateBead, CloseBead, QueryBeads
// adapter/presenter/presenter.go - Present(ctx, PresentationSummary)
// adapter/git/git.go - CreateWorktree, RemoveWorktree, Commit, Diff
```

**Data Flow:**

1. `gromit run2 <spec-file>` loads spec, checks dependency specs have `accepted: true` in frontmatter
2. SpecLoop creates worktree branched from target integration branch
3. Plan reads spec + codebase context -> LLM invocation -> writes `.gromit/v2/plan.md`
4. Decompose reads plan -> LLM invocation -> creates beads via TaskTracker with `gen:0` labels
5. BeadLoop picks beads in dependency order via DAG resolver:
   - Gate -> Build (streaming) -> Validate -> Review -> Epilogue
6. After all beads: Accept evaluates acceptance criteria via LLM
7. If unmet: Accept writes `.gromit/v2/gap-analysis.md` -> Decompose (remediation mode) -> BeadLoop -> Accept (subject to generation cap)
8. On success: Present creates GitHub PR
9. On failure: preserve branch, emit Andon, present failure summary

**Integration Points:**
- Imports `internal/config/` directly (no adapter)
- Imports `internal/bead/` types (behind TaskTracker adapter for operations)
- V1 untouched: `internal/runner/`, `internal/pipeline/` remain as-is
- CLI: `cmd/gromit/run2.go` during development, renamed at cutover

**Key Decisions:**
- `any`-typed Artifacts over generic/union: requires type assertions in loop, avoids 25-field grab-bag
- Retry in loop over retry in stages: more loop complexity, stages stay simple
- Frontmatter `accepted: true` for spec acceptance: simple, version-controlled
- Budget checks omitted until Andon spec lands
- Validate has no internal retry: loop handles via RetryWith config
- Build handles model escalation internally
- Review classifies in-scope vs out-of-scope findings

## Test Strategy

**Test Levels:**

1. **Unit Tests** - every package gets `_test.go` files testing public API in isolation
2. **Integration Tests** - loop orchestration tested with fake adapters executing real stage sequences
3. **CLI Tests** - `run2` and `ready` commands tested with mock wiring

**Mocking Strategy:**
- All adapter interfaces get stateful test fakes in `internal/v2/testutil/`
- Fake LLM returns canned responses keyed by prompt content
- Fake TaskTracker holds beads in memory with dependency tracking
- Fake Git operates on temp directories
- Fake Presenter records presentations for assertions
- Loop integration tests use all fakes together

**Key Test Cases:**
- Every Decision path (Proceed/Skip/Block/Fail) through the bead loop
- Retry with RetryWith config (Validate fail -> Build -> Validate)
- MaxRetries exhaustion
- Dependency DAG ordering (linear, diamond, circular detection)
- Remediation cycle (Accept fail -> Decompose -> BeadLoop -> Accept pass)
- Generation cap triggers Andon
- Event stream contains correct lifecycle events in order
- Each stage's happy and failure paths independently

**Test Organization:**
- `internal/v2/stage/*_test.go` - per-stage unit tests
- `internal/v2/loop/*_test.go` - loop unit + integration tests
- `internal/v2/adapter/*/*_test.go` - adapter unit tests
- `internal/v2/dep/*_test.go` - DAG resolver tests
- `internal/v2/event/*_test.go` - event system tests
- `internal/v2/prompt/*_test.go` - prompt assembler tests
- `internal/v2/spec/*_test.go` - spec loading/dependency tests
- `internal/v2/testutil/` - shared fake adapters

## Implementation Tasks

### Task 1: Stage Interface and Core Types

**Files:**
- Create: `internal/v2/stage/stage.go`
- Test: `internal/v2/stage/stage_test.go`

**What to Do:**
Define the core Stage interface and all supporting types: Stage (interface with Run(ctx, StageRequest) (StageResult, error)), StageRequest (Bead metadata, Model, Iteration, Config, RetryContext), StageResult (Decision, Artifacts any, Events), Decision enum (Proceed/Skip/Block/Fail), BeadInfo struct, RetryContext struct, RetryConfig struct (MaxRetries, RetryWith).

**Acceptance Criteria:**
- Stage interface compiles and is implementable by test doubles
- All types construct correctly with zero values and populated values
- Decision constants have distinct values and String() method

**Dependencies:** None

---

### Task 2: Event Types

**Files:**
- Create: `internal/v2/event/event.go`
- Test: `internal/v2/event/event_test.go`

**What to Do:**
Define all typed event structs: Lifecycle (SpecStarted, SpecCompleted, SpecFailed, BeadStarted, BeadCompleted), Stage (StageStarted, StageCompleted, StageFailed, StageRetrying), Validation (ValidationPassed, ValidationFailed, ValidationStepResult), Review (ReviewFinding with in_scope bool, BeadCreated), Scope (GenerationCapReached, AndonTriggered), Telemetry (LLMInvocation, StageTimings). All events embed a base Event struct with SchemaVersion, Timestamp, and Type fields.

**Acceptance Criteria:**
- All event types defined with schema_version field
- Events implement a common Event interface (Type() string, SchemaVersion() int)
- Event construction helpers produce valid events

**Dependencies:** None

---

### Task 3: Event Emitter

**Files:**
- Create: `internal/v2/event/emitter.go`
- Test: `internal/v2/event/emitter_test.go`

**What to Do:**
Implement an Emitter that accepts subscribers (via Subscribe(func(Event))) and fans out emitted events to all subscribers. Must be thread-safe. Subscribers that panic or take too long should not block other subscribers.

**Acceptance Criteria:**
- Emitter fans out events to multiple subscribers
- Thread-safe concurrent Emit and Subscribe
- Unknown event types don't crash subscribers

**Dependencies:** Task 2

---

### Task 4: Adapter Interfaces

**Files:**
- Create: `internal/v2/adapter/llm/llm.go`
- Create: `internal/v2/adapter/tasktracker/tasktracker.go`
- Create: `internal/v2/adapter/presenter/presenter.go`
- Create: `internal/v2/adapter/git/git.go`

**What to Do:**
Define the four adapter interfaces with their request/response types. LLMProvider: Invoke (non-streaming) and StreamInvoke (streaming with io.Writer). TaskTracker: NextBead, CreateBead, CloseBead, QueryBeads. Presenter: Present(ctx, PresentationSummary). Git: CreateWorktree, RemoveWorktree, Commit, Diff. Include all supporting types (LLMResponse, CreateBeadRequest, BeadFilter, PresentationSummary).

**Acceptance Criteria:**
- All four interfaces defined and documented
- Request/response types for each method compile
- No implementation code - interfaces only

**Dependencies:** None

---

### Task 5: Shared Test Fakes

**Files:**
- Create: `internal/v2/testutil/fake_llm.go`
- Create: `internal/v2/testutil/fake_tasktracker.go`
- Create: `internal/v2/testutil/fake_presenter.go`
- Create: `internal/v2/testutil/fake_git.go`
- Test: `internal/v2/testutil/fakes_test.go`

**What to Do:**
Implement fake adapters for all four interfaces. FakeLLM: returns canned responses keyed by prompt substring, records all invocations. FakeTaskTracker: holds beads in memory, supports dependency queries, tracks create/close calls. FakePresenter: records PresentationSummary for assertion. FakeGit: operates on temp directories, tracks operations.

**Acceptance Criteria:**
- All fakes implement their adapter interfaces (compile-time check)
- FakeTaskTracker supports in-memory dependency resolution
- All fakes record calls for test assertions

**Dependencies:** Task 4

---

### Task 6: Spec Loading and Acceptance

**Files:**
- Create: `internal/v2/spec/spec.go`
- Test: `internal/v2/spec/spec_test.go`

**What to Do:**
Implement spec file loading: parse YAML frontmatter for id, dependencies (list of spec IDs), accepted (bool), plus Architecture Direction and Test Strategy sections. Implement CheckDependencies(specsDir) that reads all dependency specs and returns an error listing any that don't have accepted: true. Implement ListReady(specsDir) that returns specs whose dependencies are all satisfied and are not yet accepted.

**Acceptance Criteria:**
- Parses spec frontmatter correctly (id, dependencies, accepted, architecture direction, test strategy)
- CheckDependencies returns descriptive error listing all unsatisfied deps
- ListReady returns only specs with all deps satisfied and not yet accepted

**Dependencies:** None

---

### Task 7: Dependency DAG Resolver

**Files:**
- Create: `internal/v2/dep/resolver.go`
- Test: `internal/v2/dep/resolver_test.go`

**What to Do:**
Implement a DAG resolver that tracks beads and their declared dependencies. Add(beadID, dependsOn []string) registers a bead. Next(completed []string) (string, error) returns the next eligible bead (all dependencies in completed set). Detect cycles returns an error if the graph has cycles. Topological ordering for deterministic selection among equally eligible beads.

**Acceptance Criteria:**
- Linear chains resolve in order
- Diamond dependencies handled correctly (both paths must complete)
- Circular dependencies detected and returned as an error

**Dependencies:** None

---

### Task 8: Prompt Assembler

**Files:**
- Create: `internal/v2/prompt/assembler.go`
- Test: `internal/v2/prompt/assembler_test.go`

**What to Do:**
Implement a layered prompt assembler that composes prompts from four sources: base layer (stage-specific instructions from embedded templates), project layer (CLAUDE.md, RULES.md from project), instance layer (bead description, prior failures, acceptance criteria), and fragment layer (optional cross-cutting concerns from config). Each layer is a string; the assembler concatenates with clear section markers. Missing layers are skipped gracefully.

**Acceptance Criteria:**
- Composes all four layers with section delimiters
- Missing/empty layers are skipped without error
- Base templates are embeddable (go:embed or loaded from filesystem)

**Dependencies:** None

---

### Task 9: Gate Stage

**Files:**
- Create: `internal/v2/stage/gate/gate.go`
- Test: `internal/v2/stage/gate/gate_test.go`

**What to Do:**
Implement the Gate stage. Checks whether the bead is still relevant (not already closed), already completed, or blocked. Uses TaskTracker adapter to query bead status. Returns Skip for completed/closed beads, Block for beads with unsatisfied dependencies or other blockers, Proceed to continue pipeline.

**Acceptance Criteria:**
- Returns Skip for completed bead
- Returns Block for bead with unsatisfied dependencies
- Returns Proceed for active, eligible bead

**Dependencies:** Task 1, Task 4

---

### Task 10: Plan Stage

**Files:**
- Create: `internal/v2/stage/plan/plan.go`
- Test: `internal/v2/stage/plan/plan_test.go`

**What to Do:**
Implement the Plan stage. Reads the spec content and assembles a prompt using the prompt assembler (base: plan instructions, project: CLAUDE.md, instance: spec content + codebase file listing). Invokes LLM (non-streaming, opus model). Writes result to .gromit/v2/plan.md in the worktree. Returns Proceed with plan content as artifact.

**Acceptance Criteria:**
- Invokes LLM with spec content and codebase context
- Writes plan to .gromit/v2/plan.md
- Returns Proceed with PlanArtifacts containing plan content

**Dependencies:** Task 1, Task 4, Task 8

---

### Task 11: Decompose Stage

**Files:**
- Create: `internal/v2/stage/decompose/decompose.go`
- Test: `internal/v2/stage/decompose/decompose_test.go`

**What to Do:**
Adapt the v1 decompose logic (from internal/pipeline/decompose.go) to the v2 Stage interface. Reads .gromit/v2/plan.md (or .gromit/v2/gap-analysis.md when running in remediation mode - detected via StageRequest context). Invokes LLM to parse plan into bead definitions with dependencies. Validates bead sizing (max files, acceptance criteria count). Retry loop for violation reduction. Creates beads via TaskTracker adapter with generation labels. Remove CLI-command scaffolding from v1; keep the core parse-validate-create logic.

**Acceptance Criteria:**
- Reads plan or gap-analysis and produces bead definitions via LLM
- Creates beads via TaskTracker with correct generation labels and dependency declarations
- Violation retry loop reduces complexity issues before creating beads

**Dependencies:** Task 1, Task 4, Task 8

**Notes:** Largest stage - adapted from 735 lines of v1. Decompose may split into 2-3 beads.

---

### Task 12: Build Stage

**Files:**
- Create: `internal/v2/stage/build/build.go`
- Test: `internal/v2/stage/build/build_test.go`

**What to Do:**
Adapt v1 Build logic to v2 Stage interface. Select methodology (TDD/Refactor/Standard) from bead labels and config. Render prompt using prompt assembler (base: build instructions for methodology, project: CLAUDE.md, instance: bead description + prior validation failures from RetryContext). Invoke LLM via StreamInvoke for live output. Model escalation on failure: follow configured escalation chain (e.g., haiku->sonnet->opus) internally. Remove MidReview coupling from v1. Return Proceed with BuildArtifacts (model, tokens, cost, duration).

**Acceptance Criteria:**
- Renders methodology-appropriate prompt via assembler
- Uses StreamInvoke for live output
- Model escalation retries with next tier on failure

**Dependencies:** Task 1, Task 4, Task 8

---

### Task 13: Validate Stage

**Files:**
- Create: `internal/v2/stage/validate/validate.go`
- Test: `internal/v2/stage/validate/validate_test.go`

**What to Do:**
Adapt v1 Validate logic to v2 Stage interface. Runs configured validation commands from project config (build, test, lint, format). Executes sequentially, stops at first failure. Returns Proceed on all pass, Fail with ValidateArtifacts containing failure details (command, stdout, stderr, step name). No internal auto-fix retry - the loop handles retry via RetryWith config. Pure pass/fail reporter.

**Acceptance Criteria:**
- Runs all configured validation commands in sequence
- Returns Proceed when all pass, Fail with details on first failure
- No internal retry logic - stage is single-shot

**Dependencies:** Task 1

---

### Task 14: Review Stage

**Files:**
- Create: `internal/v2/stage/review/review.go`
- Test: `internal/v2/stage/review/review_test.go`

**What to Do:**
Adapt v1 Review logic to v2 Stage interface. Loads diff via Git adapter. Renders review prompt using prompt assembler (base: review instructions, project: rules, instance: diff + spec acceptance criteria). Invokes LLM (non-streaming). Parses findings. Classifies each finding as in-scope (affects spec acceptance criteria) or out-of-scope (tangential). Creates beads for in-scope findings via TaskTracker (generation N+1). Emits ReviewFinding events for out-of-scope findings. Returns Proceed with ReviewArtifacts (created bead IDs, out-of-scope findings).

**Acceptance Criteria:**
- Classifies findings as in-scope vs out-of-scope
- Creates beads only for in-scope findings with correct generation label
- Emits ReviewFinding events for out-of-scope findings

**Dependencies:** Task 1, Task 2, Task 4, Task 8

---

### Task 15: Epilogue Stage

**Files:**
- Create: `internal/v2/stage/epilogue/epilogue.go`
- Test: `internal/v2/stage/epilogue/epilogue_test.go`

**What to Do:**
Implement the Epilogue stage. Closes the bead via TaskTracker adapter. Emits telemetry events: BeadCompleted with timing, LLM cost summary accumulated from prior stage events. Simplified from v1 - no FailureLearner, no ThoroughReviewer, no StatusWriter. Returns Proceed with EpilogueArtifacts.

**Acceptance Criteria:**
- Closes bead via TaskTracker on success path
- Emits BeadCompleted telemetry event
- Handles both success and failure paths (close vs leave open)

**Dependencies:** Task 1, Task 2, Task 4

---

### Task 16: Accept Stage

**Files:**
- Create: `internal/v2/stage/accept/accept.go`
- Test: `internal/v2/stage/accept/accept_test.go`

**What to Do:**
Implement the Accept stage. Reads the spec's acceptance criteria. Invokes LLM (non-streaming) to evaluate each criterion against the current codebase state (via file listing and targeted reads). Returns Proceed with AcceptArtifacts if all criteria pass. On failure: writes gap analysis to .gromit/v2/gap-analysis.md describing what's missing, returns Fail with AcceptArtifacts containing the gap analysis summary.

**Acceptance Criteria:**
- Evaluates acceptance criteria against codebase via LLM
- Returns Proceed when all criteria met
- Writes gap-analysis.md and returns Fail when criteria unmet

**Dependencies:** Task 1, Task 4, Task 8

---

### Task 17: Present Stage

**Files:**
- Create: `internal/v2/stage/present/present.go`
- Test: `internal/v2/stage/present/present_test.go`

**What to Do:**
Implement the Present stage. Builds a PresentationSummary from accumulated data: what was done (bead summaries), acceptance criteria results, out-of-scope review findings, branch/diff link. Invokes the Presenter adapter. Returns Proceed. On presenter error, returns Fail.

**Acceptance Criteria:**
- Builds PresentationSummary with all required fields
- Invokes Presenter adapter
- Includes out-of-scope review findings in summary

**Dependencies:** Task 1, Task 4

---

### Task 18: Bead Loop - Core Stage Sequencing

**Files:**
- Create: `internal/v2/loop/bead_loop.go`
- Test: `internal/v2/loop/bead_loop_test.go`

**What to Do:**
Implement the inner bead loop. Picks next bead whose dependencies are satisfied (via dep resolver). Runs the stage pipeline: Gate -> Build -> Validate -> Review -> Epilogue. Handles Decision-based control flow: Skip/Block from Gate skips to Epilogue (failure path), Fail from Validate/Build triggers Epilogue (failure path). Emits BeadStarted/BeadCompleted lifecycle events. Loops until no more eligible beads.

**Acceptance Criteria:**
- Picks beads in dependency order using DAG resolver
- Runs Gate->Build->Validate->Review->Epilogue per bead
- Skip/Block/Fail decisions drive correct control flow

**Dependencies:** Task 1, Task 2, Task 3, Task 5, Task 7

---

### Task 19: Bead Loop - Retry Logic

**Files:**
- Modify: `internal/v2/loop/bead_loop.go`
- Modify: `internal/v2/loop/bead_loop_test.go`

**What to Do:**
Add retry logic to the bead loop. Each stage has a RetryConfig (MaxRetries, RetryWith). When a stage returns Fail: check if retries remain. If RetryWith is configured (e.g., Validate has RetryWith: ["build"]), rerun those stages first, then retry the failed stage. Populate RetryContext on StageRequest with prior failure information. Emit StageRetrying events. Stop retrying when MaxRetries exhausted (proceed to Epilogue failure path).

**Acceptance Criteria:**
- RetryWith reruns specified stages before retrying failed stage
- MaxRetries respected - exhaustion proceeds to failure path
- RetryContext populated with prior failure details on retry attempts

**Dependencies:** Task 18

---

### Task 20: Bead Loop - Generation Cap

**Files:**
- Modify: `internal/v2/loop/bead_loop.go`
- Modify: `internal/v2/loop/bead_loop_test.go`

**What to Do:**
Add generation cap enforcement to the bead loop. Track the start generation (highest generation at loop entry) and the cap (configurable, default 3). When any bead's generation reaches start_generation + cap, stop creating new beads (Review and Decompose are prevented from enqueueing). Emit GenerationCapReached event. Return a signal to the spec loop indicating the cap was hit.

**Acceptance Criteria:**
- Stops bead creation when generation cap window is reached
- Emits GenerationCapReached event
- Returns cap-hit signal to spec loop

**Dependencies:** Task 18, Task 2

---

### Task 21: Spec Loop - Happy Path

**Files:**
- Create: `internal/v2/loop/spec_loop.go`
- Test: `internal/v2/loop/spec_loop_test.go`

**What to Do:**
Implement the outer spec loop happy path. Creates worktree via Git adapter. Runs Plan stage, then Decompose stage, then BeadLoop, then Accept stage. If Accept returns Proceed, runs Present stage. Cleans up worktree on completion. Emits SpecStarted/SpecCompleted lifecycle events.

**Acceptance Criteria:**
- Plan->Decompose->BeadLoop->Accept(pass)->Present executes in sequence
- Worktree created at start and cleaned up on completion
- SpecStarted and SpecCompleted events emitted

**Dependencies:** Task 18, Task 5

---

### Task 22: Spec Loop - Remediation and Failure

**Files:**
- Modify: `internal/v2/loop/spec_loop.go`
- Modify: `internal/v2/loop/spec_loop_test.go`

**What to Do:**
Add remediation and failure paths to the spec loop. When Accept returns Fail: read gap analysis from .gromit/v2/gap-analysis.md, run Decompose in remediation mode (reads gap analysis instead of plan), run BeadLoop again, run Accept again. Loop until Accept passes or generation cap is hit. On generation cap or total failure: preserve the branch, emit AndonTriggered and SpecFailed events, run Present with failure summary.

**Acceptance Criteria:**
- Accept(fail)->Decompose(gap analysis)->BeadLoop->Accept cycle works
- Generation cap across remediation cycles triggers failure path
- Failure path preserves branch and emits Andon event

**Dependencies:** Task 21, Task 20

---

### Task 23: Claude LLM Adapter

**Files:**
- Create: `internal/v2/adapter/llm/claude.go`
- Test: `internal/v2/adapter/llm/claude_test.go`

**What to Do:**
Implement the LLMProvider interface using the Claude CLI (similar to internal/claude/claude.go). Invoke constructs and runs the Claude CLI command with the prompt and model, parses the JSON result. StreamInvoke uses --output-format stream-json and streams to the writer. Both return LLMResponse with success, output, tokens, cost, duration.

**Acceptance Criteria:**
- Invoke sends prompt to Claude CLI and parses response
- StreamInvoke streams output to writer
- Both return correctly populated LLMResponse

**Dependencies:** Task 4

---

### Task 24: bd TaskTracker Adapter

**Files:**
- Create: `internal/v2/adapter/tasktracker/bd.go`
- Test: `internal/v2/adapter/tasktracker/bd_test.go`

**What to Do:**
Implement the TaskTracker interface using bd CLI (adapting patterns from internal/bead/). NextBead queries bd for the next open bead. CreateBead creates a bead with title, description, priority, labels (including gen:N), and dependency declarations. CloseBead marks a bead as closed. QueryBeads supports filtering by labels, status, parent.

**Acceptance Criteria:**
- CreateBead sets generation labels correctly
- NextBead returns beads with dependency info
- CloseBead marks bead as closed via bd CLI

**Dependencies:** Task 4

---

### Task 25: GitHub Presenter Adapter

**Files:**
- Create: `internal/v2/adapter/presenter/github.go`
- Test: `internal/v2/adapter/presenter/github_test.go`

**What to Do:**
Implement the Presenter interface using gh CLI. Creates a pull request from the spec's worktree branch to the target integration branch. PR title from spec name, body from PresentationSummary (what was done, acceptance criteria results, out-of-scope findings, bead count, cost summary). On failure presentation, body includes failure summary and what remains.

**Acceptance Criteria:**
- Creates PR via gh pr create with correct branch, title, and body
- Success presentation includes acceptance results and out-of-scope findings
- Failure presentation includes failure summary and remaining work

**Dependencies:** Task 4

---

### Task 26: Git Exec Adapter

**Files:**
- Create: `internal/v2/adapter/git/exec.go`
- Test: `internal/v2/adapter/git/exec_test.go`

**What to Do:**
Implement the Git interface using exec.Command("git", ...). CreateWorktree: git worktree add. RemoveWorktree: git worktree remove. Commit: git -C <worktree> add -A && git -C <worktree> commit -m <msg>. Diff: git -C <worktree> diff <base>.

**Acceptance Criteria:**
- CreateWorktree and RemoveWorktree execute correct git commands
- Commit stages and commits all changes in worktree
- Diff returns diff against specified base

**Dependencies:** Task 4

---

### Task 27: gromit run2 CLI Command

**Files:**
- Create: `cmd/gromit/run2.go`
- Modify: `cmd/gromit/main.go`

**What to Do:**
Implement the run2 subcommand. Accepts a spec file path as argument. Loads the spec, checks dependencies via spec.CheckDependencies. Loads config. Wires all concrete adapters (Claude LLM, bd TaskTracker, GitHub Presenter, exec Git). Creates SpecLoop with adapters. Subscribes event consumers (CLI progress display, log writer). Runs the spec loop. Returns exit code 0 on success, non-zero on failure.

**Acceptance Criteria:**
- gromit run2 <spec-file> runs full loop end-to-end
- Exits with error listing blockers when spec dependencies unsatisfied
- Streams events to CLI display during execution

**Dependencies:** Task 6, Task 21, Task 22, Task 23, Task 24, Task 25, Task 26

---

### Task 28: gromit ready CLI Command

**Files:**
- Create: `cmd/gromit/ready.go`
- Modify: `cmd/gromit/main.go`

**What to Do:**
Implement the ready subcommand. Scans the specs directory, calls spec.ListReady to find specs with all dependencies satisfied. Prints eligible spec names and their file paths. No execution - query only.

**Acceptance Criteria:**
- Lists specs with all dependencies satisfied and not yet accepted
- Shows spec name and file path for each eligible spec
- Returns empty list gracefully when no specs are ready

**Dependencies:** Task 6

---

### Task 29: End-to-End Integration Tests

**Files:**
- Create: `internal/v2/loop/integration_test.go`

**What to Do:**
Write integration tests that exercise the full spec loop with all fake adapters. Happy path: spec with 2-3 beads, all pass, Accept succeeds, Present called. Remediation path: Accept fails first time, gap analysis produces remediation bead, second Accept passes. Failure path: generation cap hit, Andon emitted, failure presented. Verify event stream contains expected lifecycle events in correct order.

**Acceptance Criteria:**
- Happy path: Plan->Decompose->BeadLoop->Accept->Present with correct events
- Remediation: Accept(fail)->Decompose->BeadLoop->Accept(pass) cycle verified
- Failure: generation cap triggers Andon and failure presentation

**Dependencies:** Task 5, Task 22

---

## Acceptance Tests

### Task 30: AT - End-to-End Spec Processing

**Files:**
- Create: `internal/v2/acceptance_test.go`

**What to Do:**
Write acceptance tests verifying the core end-to-end flow using fake adapters. A spec goes through Plan -> Decompose -> bead execution (Gate->Build->Validate->Review->Epilogue) -> Accept -> Present. Verify that everything enters as a spec (no special batch/chore paths exist). Verify planning runs just-in-time (plan.md does not exist before SpecLoop starts, exists after Plan stage). Verify all stages implement the uniform Stage interface (Run with StageRequest/StageResult). Verify StageResult carries Decision, typed Artifacts, and typed Events array.

**Acceptance Criteria:**
- Full spec-to-presentation flow completes with fake adapters
- No alternative entry points exist for batches or chores
- Plan stage writes plan.md just-in-time, not ahead of execution

**Dependencies:** Task 22, Task 5

---

### Task 31: AT - Dependency Gating

**Files:**
- Create: `internal/v2/acceptance_dep_test.go`

**What to Do:**
Write acceptance tests for dependency gating at both spec and bead levels. Spec level: gromit run2 with unsatisfied spec dependencies returns error listing all blocking specs. gromit ready lists only specs whose dependencies are all accepted. Bead level: beads with unsatisfied dependencies are never picked up by the inner loop; only beads with all deps completed are executed.

**Acceptance Criteria:**
- Spec with unsatisfied deps produces error listing all blockers
- gromit ready returns only eligible specs
- Bead loop never executes a bead whose dependencies are incomplete

**Dependencies:** Task 6, Task 18, Task 7

---

### Task 32: AT - Remediation and Scope Control

**Files:**
- Create: `internal/v2/acceptance_remediation_test.go`

**What to Do:**
Write acceptance tests for the remediation cycle and scope control. When Accept finds unmet criteria, it produces gap analysis -> Decompose creates remediation beads -> inner loop processes them -> Accept re-evaluates. Generation cap (configurable, default 3) stops bead creation when highest generation reaches start_generation + cap. On cap hit or total failure: branch preserved with failure summary, AndonTriggered event emitted, failure presented via Presenter.

**Acceptance Criteria:**
- Accept(fail) -> gap analysis -> Decompose -> BeadLoop -> Accept(pass) cycle verified
- Generation cap stops new bead creation and emits GenerationCapReached
- Failure path preserves branch, emits Andon, presents failure summary

**Dependencies:** Task 22, Task 5

---

### Task 33: AT - Retry and Escalation

**Files:**
- Create: `internal/v2/acceptance_retry_test.go`

**What to Do:**
Write acceptance tests for retry semantics and model escalation. Retry is configured per-stage (MaxRetries, RetryWith) and enforced by the loop, not by stages. Validate with RetryWith: ["build"] causes Build to rerun before Validate retries. Build handles model escalation internally (e.g., haiku->sonnet->opus). Stages are stateless; inter-stage retry data flows only through RetryContext on StageRequest. Verify RetryContext carries prior failure information.

**Acceptance Criteria:**
- Validate failure triggers Build rerun then Validate retry (RetryWith)
- MaxRetries exhaustion proceeds to failure path without infinite loop
- RetryContext populated with prior failures on retry attempts

**Dependencies:** Task 19, Task 5

---

### Task 34: AT - Review Scoping

**Files:**
- Create: `internal/v2/acceptance_review_test.go`

**What to Do:**
Write acceptance tests for review finding classification. Review stage classifies findings as in-scope (affects current spec's acceptance criteria) or out-of-scope (tangential). Only in-scope findings become new beads (with generation N+1). Out-of-scope findings are emitted as ReviewFinding events with in_scope: false. Out-of-scope findings are included in the Present stage summary for the product owner.

**Acceptance Criteria:**
- In-scope findings create beads with correct generation label
- Out-of-scope findings emitted as events, not converted to beads
- Present stage summary includes out-of-scope findings

**Dependencies:** Task 14, Task 17, Task 5

---

### Task 35: AT - Event System Contract

**Files:**
- Create: `internal/v2/acceptance_events_test.go`

**What to Do:**
Write acceptance tests for the typed event system. All events carry a schema_version field. Consumers handle unknown event types gracefully (ignore, don't crash). The event stream is consumable by CLI, API, and log subscribers simultaneously. Verify correct event ordering: SpecStarted -> BeadStarted -> StageStarted/Completed -> BeadCompleted -> SpecCompleted. Verify validation events (ValidationPassed/Failed/StepResult) and telemetry events (LLMInvocation, StageTimings) are emitted at correct points.

**Acceptance Criteria:**
- All events carry schema_version field
- Unknown event types handled gracefully by subscribers
- Event ordering matches lifecycle (Spec -> Bead -> Stage hierarchy)

**Dependencies:** Task 22, Task 5

---

### Task 36: AT - Adapter Swappability

**Files:**
- Create: `internal/v2/acceptance_adapter_test.go`

**What to Do:**
Write acceptance tests proving adapter swappability. Run the full spec loop with one set of fake adapters, then swap each adapter independently and verify the loop still works. Verify that no stage or loop code imports concrete adapter implementations directly - only interfaces. Verify config is loaded from internal/config/ directly (not through an adapter). This test is primarily a compile-time and structural verification.

**Acceptance Criteria:**
- Full loop runs with swapped adapter implementations
- No stage package imports concrete adapter packages
- Config loaded directly, not via adapter

**Dependencies:** Task 22, Task 5

---

### Task 37: AT - Worktree Isolation and Language Agnosticism

**Files:**
- Create: `internal/v2/acceptance_isolation_test.go`

**What to Do:**
Write acceptance tests for worktree isolation and language-agnostic validation. Each spec executes in its own git worktree (verify via Git adapter calls). Validate stage runs configurable external commands from project config (not hardcoded to any language). Verify validate commands are read from config, not hard-coded. Verify prompts compose from base, project, instance, and fragment layers.

**Acceptance Criteria:**
- Spec execution creates and uses isolated worktree
- Validate runs commands from config, not hardcoded
- Prompt assembler composes all four layers

**Dependencies:** Task 22, Task 5, Task 8

---

## Notes

- V1 must remain stable throughout - it builds v2 (bootstrapping constraint)
- At cutover: delete `internal/runner/`, `internal/pipeline/`, rename `run2` to `run`
- Budget integration points will be added when the Andon spec lands
- The prompt assembler's base layer templates will need iteration as stages are built - start simple, refine
- Generation labels on beads use format `gen:N` (e.g., `gen:0`, `gen:1`, `gen:2`)
- The decompose stage is the most complex adaptation from v1 - expect 2-3 beads from it
