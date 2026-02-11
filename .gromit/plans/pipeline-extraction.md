---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T10:39:05-05:00"
id: pipeline-extraction
source_spec: pipeline-extraction
---

# Pipeline Extraction Implementation Plan

**Goal:** Extract workflow orchestration logic from `cmd/gromit/` command handlers into `internal/pipeline/`, creating a reusable business logic layer decoupled from the CLI.

**Architecture:** Pipeline struct with methods for each workflow. Interactive workflows return typed Session wrappers with event channels; non-interactive workflows return structured results. CLI handlers become thin adapters that resolve user input, create a Pipeline, call the workflow method, and handle chaining/display.

**Tech Stack:** Go, existing internal packages (agent, claude, bead, backlog, config, frontmatter, learnings, state, logger, review, prompt)

**Spec:** `.gromit/specs/pipeline-extraction.md`

---

## Architecture

### Pipeline Struct

Central orchestrator holding config, paths, and injected dependencies:

```go
type Pipeline struct {
    cfg   *config.Config
    paths Paths
    // Dependencies (interfaces defined in this package)
    agents    AgentResolver
    claude    ClaudeClient
    beads     BeadClient
    backlog   BacklogClient
    renderer  PromptRenderer
    learnings LearningsManager
    state     StateManager
    logWriter LogWriter
}

type Paths struct {
    GromitDir    string
    SpecsDir     string
    PlansDir     string
    TemplatesDir string
    EpicsDir     string
}

type Deps struct {
    Config    *config.Config
    Paths     Paths
    Agents    AgentResolver
    Claude    ClaudeClient
    Beads     BeadClient
    Backlog   BacklogClient
    Renderer  PromptRenderer
    Learnings LearningsManager
    State     StateManager
    Logger    LogWriter
}

func New(deps Deps) (*Pipeline, error)
```

Each workflow method validates that its specific deps are non-nil at call time. CLI handlers create a Pipeline for each command invocation.

### Session Architecture

Interactive workflows return typed session wrappers:

```go
type Session interface {
    Events() <-chan Event
    SendInput(text string) error
    Cancel()
    Wait() error
}

type baseSession struct {
    cmd         *exec.Cmd
    stdin       io.WriteCloser
    stdout      io.ReadCloser
    events      chan Event
    done        chan struct{}
    err         error
    cancelFunc  context.CancelFunc
    postProcess func() error
}

// Typed wrappers per workflow
type RefineSession struct {
    Session
    result *RefineResult
}
func (s *RefineSession) Result() *RefineResult // valid after Wait()
```

baseSession reads agent stdout in a goroutine (emitting EventOutput), accepts input via SendInput (writing to stdin pipe), and runs post-processing after process exit before emitting EventSessionEnded.

### Agent.Command() Extension

New method on Agent interface returns a configured `*exec.Cmd` without running it, letting Session manage process lifecycle and I/O piping:

```go
type Agent interface {
    Name() string
    Launch(promptPath string) error       // existing
    Command(promptPath string) (*exec.Cmd, error) // new
}
```

### Workflow API

```go
// Interactive workflows — return Session for caller to drive
func (p *Pipeline) Refine(ctx context.Context, input RefineInput) (*RefineSession, error)
func (p *Pipeline) Plan(ctx context.Context, input PlanInput) (*PlanSession, error)
func (p *Pipeline) Explore(ctx context.Context, input ExploreInput) (*ExploreSession, error)

// Dual-mode workflow
func (p *Pipeline) ReviewInteractive(ctx context.Context, input ReviewInput) (*ReviewSession, error)
func (p *Pipeline) ReviewNonInteractive(ctx context.Context, input ReviewInput) (*ReviewResult, error)

// Non-interactive only
func (p *Pipeline) Decompose(ctx context.Context, input DecomposeInput) (*DecomposeResult, error)
```

### Data Flow (interactive — Refine example)

```
CLI: parse flags, run picker → RefineInput{IdeaText, AgentName, ...}
     ↓
Pipeline.Refine(ctx, input)
  1. Scan existing specs (pre-snapshot)
  2. Build system prompt from idea + skill content
  3. Write prompt to temp file
  4. Resolve agent by name → Agent.Command(promptPath)
  5. Create baseSession with pipes + post-processing
  6. Start process, emit EventSessionStarted
  7. Return RefineSession
     ↓
CLI: drain Events() → stdout, pipe stdin → SendInput()
     ↓
Agent exits → Session post-processing:
  - Scan specs dir (post-snapshot), diff against pre
  - Update backlog if from backlog item
  - Create backlog item if blank session produced spec
  - Populate RefineResult
  - Emit EventSessionEnded
     ↓
CLI: session.Result() → display summary, offer chaining
```

### What Stays in CLI

- Cobra command definitions and flag parsing
- Interactive pickers (backlog item picker, spec picker, plan picker)
- User prompts and confirmations
- Chaining orchestration (chain.go)
- Output formatting and display
- stdin/stdout wiring for Session events

### What Moves to Pipeline

- Prompt building and context assembly
- Agent resolution and process launching (via Session)
- All post-processing: spec detection, bead creation, frontmatter updates, backlog mutations, learning persistence, state updates, logging
- Scope resolution for review
- Structured result construction

---

## Test Strategy

### Unit Tests (per workflow file)

- `pipeline_test.go` — Constructor validates required deps, rejects nil config
- `session_test.go` — Event emission from stdout, SendInput to stdin, Cancel kills process, Wait blocks until post-processing, handles process crash
- `refine_test.go` — Spec detection (before/after diff), backlog updates, blank session item creation
- `plan_test.go` — Plan detection, spec loading, duplicate rejection with force override
- `decompose_test.go` — JSON parsing, dependency mapping, partial failure, frontmatter update
- `review_test.go` — Both modes: scope resolution, result parsing, bead/backlog creation, learning persistence, state update
- `explore_test.go` — Prompt building with/without topic, agent normalization
- `helpers_test.go` — File scanning, spec title extraction

### Mocking Strategy

- Mock all external clients via pipeline-defined interfaces
- Compile-time satisfaction checks (`var _ BeadClient = (*mockBeadClient)(nil)`)
- Use `t.TempDir()` for real file system operations
- Do NOT mock: JSON parsing, path manipulation, frontmatter parsing

### Coverage Goals

- All post-processing paths tested
- Error handling: nil deps, Claude failure, JSON parse failure, mid-batch bead creation failure
- Session lifecycle fully tested (start → output → input → exit → post-process)
- Edge cases: empty dirs, no new files, zero beads, empty diff

---

## Implementation Tasks

### Task 1: Pipeline foundation — types, interfaces, and constructor

**Files:**
- Create: `internal/pipeline/pipeline.go`
- Create: `internal/pipeline/types.go`
- Test: `internal/pipeline/pipeline_test.go`

**What to Do:**

Create the Pipeline struct with Deps-based constructor following the runner's NewWithDeps pattern. Define narrow interfaces for all dependencies: AgentResolver (resolves agent by name), ClaudeClient (Run method), BeadClient (Create, CreateWithDepsAndDescription, List, ListWithLabel), BacklogClient (List, Get, Add, Update), PromptRenderer (RenderThoroughReview, LoadClaudeMD, LoadRules, GetLearningsFile), LearningsManager (Load, Add, SetFilter), StateManager (Load, LastReviewCommit, RecordReview), LogWriter (LogReview, Close). Add compile-time interface satisfaction checks against concrete types.

In types.go, define all input/output structs (RefineInput, RefineResult, PlanInput, PlanResult, DecomposeInput, DecomposeResult, ReviewInput, ReviewResult, ExploreInput, ExploreResult), the Session interface (Events, SendInput, Cancel, Wait), EventType constants (EventOutput, EventSessionStarted, EventSessionEnded, EventError), and Event struct. Define typed session wrappers (RefineSession, PlanSession, ReviewSession, ExploreSession) with Result() methods.

Initialize all slice fields to empty slices (nil-safe normalization).

**Acceptance Criteria:**
- All types compile and are importable from other packages
- Pipeline constructor validates required fields (Config non-nil) and returns error for invalid deps
- Compile-time interface checks pass against concrete implementations

**Dependencies:** None

---

### Task 2: Add Command() method to Agent interface

**Files:**
- Modify: `internal/agent/agent.go`
- Test: `internal/agent/agent_test.go`

**What to Do:**

Add `Command(promptPath string) (*exec.Cmd, error)` to the Agent interface. Implement in cliAgent: verify prompt file exists, build command args based on promptDelivery mode (same logic as Launch but without cmd.Run), return the configured *exec.Cmd without starting it. Set cmd.Dir if needed. For Stdin delivery, do NOT set up the pipe — let the caller (Session) manage I/O.

Update agent_test.go with tests for Command(): returns *exec.Cmd with correct binary and args for each delivery mode (FileRef, PromptFileArg, Stdin), returns error for missing prompt file.

**Acceptance Criteria:**
- Command() returns a configured *exec.Cmd without running the process
- Launch() continues to work unchanged (existing tests pass)
- Command() respects all three prompt delivery modes

**Dependencies:** None

---

### Task 3: Session implementation

**Files:**
- Create: `internal/pipeline/session.go`
- Test: `internal/pipeline/session_test.go`

**What to Do:**

Implement baseSession struct that manages an agent subprocess via piped I/O. The constructor takes a context, an *exec.Cmd (from Agent.Command), and a post-processing function. On creation:
1. Create pipes for stdin and stdout
2. Wire pipes to cmd.Stdin and cmd.Stdout (set cmd.Stderr to a buffer or discard)
3. Start the process
4. Launch goroutine to read stdout line-by-line, emit EventOutput events
5. Emit EventSessionStarted on the events channel

Events() returns the read-only event channel. SendInput(text) writes to stdin pipe. Cancel() kills the process via context cancellation. Wait() blocks until the process exits, runs the post-processing function, emits EventSessionEnded (or EventError on failure), and closes the events channel.

Implement typed session wrappers (RefineSession, PlanSession, etc.) that embed baseSession and add Result() methods. The post-processing function populates the result before returning.

Test with a real subprocess (e.g., `echo "hello"` or `cat`) to verify event channel behavior, SendInput, Cancel, and Wait lifecycle.

**Acceptance Criteria:**
- Session correctly reads subprocess stdout and emits EventOutput events
- SendInput writes to subprocess stdin
- Wait blocks until process exits, runs post-processing, then returns

**Dependencies:** Task 1 (types), Task 2 (Agent.Command)

**Notes:** The Session implementation is the most novel piece — it bridges the gap between the current "launch and wait" model and the event-driven model needed for TUI/web adapters. Keep the implementation simple; don't optimize for edge cases beyond clean shutdown.

---

### Task 4: Shared helpers

**Files:**
- Create: `internal/pipeline/helpers.go`
- Test: `internal/pipeline/helpers_test.go`

**What to Do:**

Extract shared utility functions from cmd/gromit/ into the pipeline package:
- `listMarkdownFiles(dir string) ([]string, error)` — scans directory for .md files, creates dir if needed
- `diffFiles(before, after []string) []string` — returns files in `after` not in `before`
- `extractSpecTitle(path string) string` — reads first # heading from markdown, handles frontmatter
- `writeTempPrompt(gromitDir, prefix, content string) (path string, cleanup func(), err error)` — writes prompt to temp file, returns cleanup function

These are currently defined in refine.go (listMarkdownFiles, containsSpec, extractSpecTitle) and repeated across handlers (temp file writing). The pipeline versions are exported for use by workflow methods.

Test with t.TempDir(): create temp directories with .md files, verify scanning, title extraction, and diff logic.

**Acceptance Criteria:**
- listMarkdownFiles matches behavior of existing getSpecFiles/listMarkdownFiles in refine.go
- extractSpecTitle handles frontmatter blocks and returns first # heading
- writeTempPrompt creates file in gromitDir/tmp and cleanup removes it

**Dependencies:** None

---

### Task 5: Refine workflow + CLI adapter

**Files:**
- Create: `internal/pipeline/refine.go`
- Test: `internal/pipeline/refine_test.go`
- Modify: `cmd/gromit/refine.go`

**What to Do:**

Implement `Pipeline.Refine(ctx, RefineInput) (*RefineSession, error)`:
1. Validate deps (backlog, agents non-nil)
2. Scan existing specs in SpecsDir (pre-snapshot)
3. Build system prompt: idea text (or blank) + specs dir + embedded skill content
4. Write prompt to temp file via writeTempPrompt
5. Resolve agent: `p.agents.ResolveByName(input.AgentName)`
6. Get command: `agent.Command(promptPath)`
7. Define post-processing function:
   - Scan specs dir (post-snapshot), diff against pre-snapshot
   - If from backlog and spec created: update backlog item status to "refined"
   - If blank session and spec created: create backlog item with title from spec
   - Populate RefineResult{CreatedSpecs, RefinedItems}
8. Create RefineSession with baseSession + post-processing
9. Return RefineSession

Refactor cmd/gromit/refine.go to thin adapter:
- Keep: Cobra command, flag parsing, interactive picker (backlog item selection), output formatting, chaining
- Replace: All business logic after picker with Pipeline.Refine() call
- Wire Session: drain Events() to stdout, pipe stdin to SendInput, call Wait(), use Result() for display and chaining

Test refine.go with mocked deps: verify spec detection, backlog updates, prompt building.

**Acceptance Criteria:**
- `gromit refine` produces identical behavior for all three input modes
- Business logic (spec detection, backlog updates) is in pipeline package
- CLI handler is ~50 lines of adapter code plus picker/chaining

**Dependencies:** Task 1, Task 3, Task 4

---

### Task 6: Plan workflow + CLI adapter

**Files:**
- Create: `internal/pipeline/plan.go`
- Test: `internal/pipeline/plan_test.go`
- Modify: `cmd/gromit/plan.go`

**What to Do:**

Implement `Pipeline.Plan(ctx, PlanInput) (*PlanSession, error)`:
1. Validate deps (agents, beads non-nil)
2. Check spec file exists at SpecsDir/specName.md
3. Check plan doesn't already exist (unless Force=true)
4. Load spec content via frontmatter.ReadFile
5. Gather open beads context via p.beads.List()
6. Build system prompt: spec name + content + open beads + plans dir + plan path + embedded skill
7. Write prompt to temp file
8. Resolve agent, get command
9. Define post-processing: check if plan file was created, populate PlanResult{PlanPath, Created}
10. Create PlanSession, return

PlanInput includes: SpecName, Force, AgentName.
PlanResult includes: PlanPath string, Created bool.

Refactor cmd/gromit/plan.go: keep picker, flag parsing, chaining (chainAfterPlan). Replace business logic with Pipeline.Plan() call.

**Acceptance Criteria:**
- `gromit plan` produces identical behavior for both input modes
- Plan existence check and force override work correctly in pipeline
- CLI handler is thin adapter with picker and chaining

**Dependencies:** Task 1, Task 3, Task 4

---

### Task 7: Decompose workflow + CLI adapter

**Files:**
- Create: `internal/pipeline/decompose.go`
- Test: `internal/pipeline/decompose_test.go`
- Modify: `cmd/gromit/decompose.go`

**What to Do:**

Implement `Pipeline.Decompose(ctx, DecomposeInput) (*DecomposeResult, error)`:
1. Validate deps (claude, beads non-nil)
2. Check plan file exists at PlansDir/planName.md
3. Read plan frontmatter/body; check decomposed field (unless Force)
4. Build decompose prompt via buildDecomposePrompt (moved to pipeline)
5. Call p.claude.Run(ctx, prompt, model)
6. Parse JSON array of bead definitions via jsonutil.ExtractJSON
7. If Review mode: return proposed beads in result without creating (let CLI handle confirmation)
8. Create beads: loop through definitions, map priority, build labels (spec:planName), resolve dependency indices to created IDs, call p.beads.CreateWithDepsAndDescription
9. Update plan frontmatter: decomposed=true, decomposed_at=timestamp
10. Return DecomposeResult{CreatedBeads, PlanUpdated, ProposedBeads (for review mode)}

Move beadDef struct and buildDecomposePrompt to pipeline package. Move parsePriority helper.

DecomposeInput: PlanName, Force, Review bool.
DecomposeResult: CreatedBeads []CreatedBead, PlanUpdated bool, ProposedBeads []ProposedBead (for review mode).
CreatedBead: ID, Title.
ProposedBead: Title, Priority, Description, AcceptanceCriteria, DependsOnIndex.

Refactor cmd/gromit/decompose.go: keep picker, "decompose all" flow, review confirmation prompt, chaining. Replace decomposeSinglePlan core with Pipeline.Decompose().

**Acceptance Criteria:**
- `gromit decompose` produces identical bead creation behavior
- Dependency index mapping handles self-deps and out-of-range indices (warn and skip)
- Frontmatter updated after successful decomposition

**Dependencies:** Task 1, Task 4

**Notes:** This is the most complex non-interactive workflow. The review mode returns proposed beads without creating them — the CLI handler prompts the user and calls Decompose again without Review if confirmed, or a separate CreateBeads method.

---

### Task 8: Review workflow + CLI adapter

**Files:**
- Create: `internal/pipeline/review.go`
- Test: `internal/pipeline/review_test.go`
- Modify: `cmd/gromit/review.go`

**What to Do:**

Implement two methods:

`Pipeline.ReviewInteractive(ctx, ReviewInput) (*ReviewSession, error)`:
1. Validate deps (agents, renderer non-nil)
2. Build ThoroughReviewContext with diff, model, CLAUDE.md, rules
3. Render prompt via p.renderer.RenderThoroughReview
4. Write prompt to temp file
5. Resolve agent, get command
6. Post-processing: none needed for interactive (user drives review)
7. Return ReviewSession

`Pipeline.ReviewNonInteractive(ctx, ReviewInput) (*ReviewResult, error)`:
1. Validate deps (claude, beads, renderer, learnings, state, logWriter non-nil)
2. Build and render prompt (same as interactive)
3. Call p.claude.Run(ctx, prompt, model) with timeout
4. Parse result via review.ParseReviewResult
5. Create beads from findings via p.beads.Create (with "from-review" label)
6. Create backlog items via p.beads.Create (with "from-review", "backlog" labels)
7. Persist learnings via p.learnings
8. Log review via p.logWriter.LogReview
9. Update state: record review commit via p.state.RecordReview
10. Return ReviewResult{Passed, FixesApplied, BeadsCreated, BacklogCreated, Summary, Learnings}

Move scope resolution logic (determineReviewScope, getSpecBaseCommit, getEpicBaseCommit, findEarliestCommitFromBeads) into pipeline package as helper methods or a separate scope.go in pipeline. These use git commands and bead client.

ReviewInput: FromCommit, Diff, Model, Timeout, AgentName.
ReviewResult: Passed, Summary, FixesApplied, BeadsCreated int, BacklogCreated int.

Note: Scope resolution (--since/--spec/--epic/state) stays in CLI because it involves flag parsing. The CLI resolves the scope to a commit and diff, then passes both in ReviewInput. Git diff retrieval can stay in CLI since it's simple exec.Command calls.

Refactor cmd/gromit/review.go: keep flag parsing, scope resolution, dry-run, mode dispatch, output formatting. Replace runReviewInteractive/runReviewNonInteractive with Pipeline calls.

**Acceptance Criteria:**
- Both `gromit review` and `gromit review --non-interactive` produce identical behavior
- Non-interactive mode creates beads, persists learnings, updates state, and logs
- Interactive mode launches agent session with rendered review prompt

**Dependencies:** Task 1, Task 3, Task 4

**Notes:** Review is the most complex workflow. Scope resolution stays in CLI because it requires flag values and state file access that's CLI-specific. The pipeline receives the resolved diff and fromCommit.

---

### Task 9: Explore workflow + CLI adapter + agent normalization

**Files:**
- Create: `internal/pipeline/explore.go`
- Test: `internal/pipeline/explore_test.go`
- Modify: `cmd/gromit/explore.go`

**What to Do:**

Implement `Pipeline.Explore(ctx, ExploreInput) (*ExploreSession, error)`:
1. Validate deps (agents, renderer non-nil)
2. Record existing artifacts (epics, specs, backlog items) — pre-snapshots
3. Build explore prompt using renderer: load CLAUDE.md, rules, learnings, format instructions
4. Include topic if provided in input
5. Write prompt to temp file
6. **Resolve agent** (instead of direct exec.Command): `p.agents.ResolveByName(input.AgentName)` defaulting to "claude"
7. Get command via agent.Command(promptPath)
8. Define post-processing: scan for new epics/specs/backlog items, diff against pre-snapshots, populate ExploreResult
9. Create ExploreSession, return

ExploreInput: Topic (optional), AgentName, Model.
ExploreResult: CreatedSpecs, CreatedEpics, CreatedBacklogItems []string.

The key change: explore currently uses direct `exec.Command(claudeBinary, cmdArgs...)` bypassing the agent abstraction. This task normalizes it to use `agent.Resolve()`/`Agent.Command()`, making all interactive workflows consistent.

For the `--model` flag: the model is passed to the agent via config or as an extra arg. Since Agent already accepts flags, the model can be an extra flag passed through.

Refactor cmd/gromit/explore.go: keep Cobra command, flag parsing. Replace direct exec.Command with Pipeline.Explore().

**Acceptance Criteria:**
- `gromit explore` produces identical behavior (with and without topic)
- Explore uses agent abstraction instead of direct exec.Command
- Post-session artifact detection is implemented (currently a TODO in explore.go)

**Dependencies:** Task 1, Task 3, Task 4

---

## Notes

### Dependency Graph

```
Task 1 (types/interfaces) ──┐
                             ├──→ Task 3 (Session) ──┐
Task 2 (Agent.Command) ─────┘                        │
                                                      ├──→ Task 5 (Refine)
Task 4 (helpers) ─────────────────────────────────────├──→ Task 6 (Plan)
                                                      ├──→ Task 7 (Decompose)
                                                      ├──→ Task 8 (Review)
                                                      └──→ Task 9 (Explore)
```

Tasks 1, 2, and 4 have no dependencies and can be done in parallel.
Task 3 depends on Tasks 1 and 2.
Tasks 5-9 depend on Tasks 1, 3, and 4, but are independent of each other.

### Risk Areas

- **Session implementation**: This is the most novel piece. The event-channel-based process management hasn't been done in this codebase before. Keep it simple — don't optimize for edge cases beyond clean shutdown.
- **Review scope resolution**: Moving scope resolution to pipeline vs keeping in CLI is a judgment call. The plan keeps it in CLI because it involves flag values, but this may need revisiting if other interfaces (TUI) need scope resolution.
- **Explore model flag**: Currently passed as `--model` to the exec.Command. The agent abstraction doesn't have a model concept. May need to pass it as an extra flag through the agent.
- **Backward compatibility**: Each CLI refactoring must preserve exact output formatting. Test manually after each workflow extraction.

### Implementation Order Recommendation

1. Tasks 1, 2, 4 in parallel (foundation)
2. Task 3 (Session — needed by interactive workflows)
3. Task 7 (Decompose — simplest, non-interactive only, good first workflow to validate pattern)
4. Task 5 (Refine — straightforward interactive workflow)
5. Task 6 (Plan — similar to Refine)
6. Task 8 (Review — most complex, dual-mode)
7. Task 9 (Explore — includes agent normalization, implement last)
