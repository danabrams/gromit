---
id: tdd-fresh-context-per-cycle
source_ideas: []
created: 2026-02-19
supersedes_decisions:
  - tdd-methodology#1
  - tdd-methodology#6
epic: token-efficiency-program
---

# Fresh Context Per TDD Cycle with Structured Handoffs

## Specification

Each red-green-refactor micro-cycle in TDD gets its own fresh Claude invocation. Between phases, the runner assembles a structured handoff that carries forward the exact file contents the next phase needs, so it can start working immediately without discovery overhead.

This replaces the current single-invocation approach where all red-green cycles happen in one Claude session. The current approach causes context pollution — late cycles work with a large, noisy context from earlier cycles, increasing failure rates and wasting tokens on stale information.

### Phase Invocations

Each TDD cycle consists of three separate Claude invocations:

1. **Red** — Write one failing test
2. **Green** — Make the failing test pass with minimal code
3. **Refactor** — Clean up without changing behavior

After each cycle completes (all three phases), the runner decides whether to start another cycle based on spec coverage.

### Structured Handoff

Between phases, the runner builds a handoff struct containing the actual file contents the next phase needs. The next Claude invocation receives this content directly in its prompt — no file discovery, no re-reading.

#### Red phase receives:

- Spec excerpt: the current requirement to test (not the full spec)
- Current test file contents (if any exist)
- Public API surface of the code under test (signatures, types)
- Cycle summary: "Cycle N. Tests for X and Y pass. Next: write a test for Z."

#### Green phase receives:

- The failing test (verbatim from red phase output or git)
- The test failure output (error messages, stack trace)
- Current implementation file contents
- Brief instruction: "Make this test pass with minimal code."

#### Refactor phase receives:

- Current implementation file contents
- Current test file contents
- Brief instruction: "Clean up without changing behavior. All tests must still pass."

#### Cycle-to-cycle handoff (red phase of cycle N+1):

- Updated test file contents (reflects all passing tests so far)
- Updated implementation file contents
- Updated cycle summary with remaining spec requirements

### Handoff Assembly

The runner gathers handoff content between phases using:

1. **Git diff / git show** — to identify which files changed
2. **File reads** — to grab current contents of touched files
3. **Test output capture** — to pass failure messages to the green phase
4. **Spec tracking** — to determine which requirements remain uncovered

This is cheap Go code running between invocations, not LLM work.

### Prompt Structure

Each phase prompt follows this structure:

```
## Role
[One sentence: what this phase does]

## Rules
[Non-negotiable constraints from RULES.md — kept short]

## Context
[Handoff content: file contents, test output, cycle state]

## Task
[Specific instruction for this phase]
```

No learnings, no full spec, no historical context, no bead metadata beyond what's needed. Each prompt is as small as possible while containing everything needed to act.

### Cycle Orchestration

The runner manages the cycle loop:

```
for each cycle:
    1. Assemble red handoff (spec excerpt, existing files, cycle state)
    2. Invoke Claude: red phase → produces new test
    3. Run test → capture failure output
    4. Assemble green handoff (failing test, failure output, impl file)
    5. Invoke Claude: green phase → produces implementation
    6. Run test → verify pass
    7. Assemble refactor handoff (impl file, test file)
    8. Invoke Claude: refactor phase → produces cleanup
    9. Run test → verify still passes
    10. Update cycle state (what's covered, what remains)
    11. If spec requirements remain → next cycle
    12. Else → done, proceed to final validation
```

Steps 3, 6, and 9 are lightweight validation runs (touched packages only), not full validation. Full validation runs once after all cycles complete.

### Cycle Termination

The runner stops cycling when:

- All spec requirements appear covered (heuristic: Claude's red phase says "all requirements tested")
- A configured max cycle count is reached (default: 10)
- A cycle fails after retry/escalation

### Escalation Within Cycles

If a phase fails:

- **Red fails** (can't write a test): retry once with analysis, then escalate model
- **Green fails** (can't make test pass): retry once with analysis, then escalate model
- **Refactor fails** (breaks tests): revert refactor, skip to next cycle

Escalation applies to the failing phase's invocation, not the whole cycle.

### Relationship to Existing Specs

- **`tdd-methodology`**: This spec preserves TDD's overall shape (red-green-refactor, methodology toggles, label overrides, inheritance) but changes the execution model from single-invocation to per-phase invocations with handoffs.
- **`phase-isolated-methodology-contexts`**: That spec handles Go `context.Context` isolation for timeouts. This spec handles Claude invocation isolation. They are complementary — each phase invocation here should use its own Go context per that spec.
- **`atdd-methodology`**: ATDD's red phase (write acceptance tests) and green phase (build to make them pass) could adopt the same handoff pattern in future. This spec focuses on TDD only.

### Out of Scope

- Changing ATDD to use per-phase invocations (future work)
- Redesigning escalation tiers or provider routing
- Changes to non-methodology build flow

## Acceptance Criteria

- Each TDD red-green-refactor phase is a separate Claude invocation with a fresh context
- Phase prompts contain only the content needed for that phase (no full spec, no learnings, no history)
- Structured handoff carries actual file contents and test output between phases
- Green phase receives the exact failing test and failure output without re-discovery
- Refactor phase receives implementation and test files without re-discovery
- Runner assembles handoff content between phases using git/filesystem reads
- Lightweight validation (touched packages) runs between phases; full validation runs after all cycles
- Cycle loop terminates on spec coverage, max cycles, or unrecoverable failure
- Phase-level escalation: a failing phase retries/escalates independently
- Refactor failure reverts and skips rather than failing the cycle
- Feature gated behind `fresh_context_per_cycle: true` in methodology config (default: false, preserving existing single-invocation behavior)
- Existing TDD config toggles and label overrides continue to work
- Unit tests cover:
  - handoff assembly between each phase transition (red→green, green→refactor, refactor→next-red)
  - cycle termination conditions (coverage complete, max cycles, failure)
  - phase-level escalation within a cycle
  - refactor failure revert behavior

## Decisions

1. **Separate invocations per phase, not per cycle.** Three invocations per cycle (red, green, refactor) gives each phase the smallest possible context. An alternative — one invocation per full cycle — would still accumulate context within a cycle.

2. **Runner assembles handoff, not Claude.** The runner reads files and test output between phases using Go code. This is faster, cheaper, and more reliable than asking Claude to summarize state for the next phase.

3. **Handoff carries content, not pointers.** The handoff includes actual file contents, not file paths. This eliminates the "discovery tax" where each invocation spends tokens reading files it already knows about.

4. **Lightweight validation between phases, full validation at the end.** Running `go test ./touched-package` between phases is fast and catches regressions immediately. Full `go test ./...` runs once at the end.

5. **Phase-level escalation.** If the green phase can't make a test pass, escalate that phase's model. Don't restart the whole cycle at a higher tier.

6. **Feature flag gated.** The fresh-context-per-cycle behavior is behind `fresh_context_per_cycle: true` in methodology config. Default is `false` — existing single-invocation TDD remains the default until this feature is validated in production. When the flag is off, TDD works exactly as before.

## Research & Context

### Why Change From Single Invocation

The original TDD spec chose single invocation to "preserve the decomposition thread." In practice:

- Late cycles in a long session produce lower-quality code because the context is polluted with earlier iterations
- Token usage scales quadratically — each cycle re-processes all prior cycles in the context
- A structured handoff preserves the decomposition thread more reliably than hoping Claude remembers what it did 5 cycles ago
- The runner can track spec coverage explicitly rather than relying on Claude's self-assessment

### Relevant Code Areas

- `internal/runner/process_methodology.go` — methodology phase orchestration (primary change target)
- `internal/runner/methodology/refactor.go` — refactor execution (adapt to handoff model)
- `internal/runner/execution/invoker.go` — invocation execution (called per phase)
- `internal/prompt/prompt.go` — prompt rendering (new phase-specific renderers)
- `.gromit/templates/` — new per-phase templates replacing `PROMPT_tdd_build.md`
