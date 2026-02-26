---
id: runner-pipeline
source_ideas: [gromit-5ncw5]
created: 2026-02-21
epic: multi-interface-architecture
---

# Runner Pipeline Refactor

## Specification

The Gromit runner is rebuilt as a 5-stage pipeline. The current `Runner` struct accumulates state across 40+ fields and 20+ files, with `callbacks.go` exercised by every execution path. The new design eliminates the God Object by giving each stage a hard package boundary: stages live in `internal/pipeline/<stage>/`, each receives an `Input` and returns an `Output`, and the orchestrator in `internal/runner/` holds no logic beyond sequencing.

The five stages are:

**Gate** (`internal/pipeline/prepare/`) — decides whether to attempt an iteration. Runs precheck, stuck-bead detection, scope gate, and proactive decomposition. Returns a `Decision`: `Proceed`, `Skip`, or `Block`. If `Skip` or `Block`, the orchestrator moves to the next bead without running subsequent stages.

**Build** (`internal/pipeline/execute/`) — authors code. Selects methodology (TDD, refactor, standard), constructs the prompt with bead context and recent validation failures, and invokes Claude via the provider router. Handles escalation (`haiku→sonnet→opus`) internally — the orchestrator sees success or failure only. Uses `StreamRun` for live output visibility.

**Validate** (`internal/pipeline/validate/`) — runs programmatic checks. Executes fast validation commands, auto-fix (`gofmt`/`goimports`), and periodic full validation. On failure, returns summaries that are fed into the next `Build` `Input.ValidationFailures`. Enforces mandatory command prefixes policy.

**Review** (`internal/pipeline/review/`) — optional LLM code review. Invokes Claude to review changes and creates new `[from-review]` beads from findings. Only runs when enabled in config.

**Epilogue** (`internal/pipeline/epilogue/`) — bead lifecycle and cleanup. Closes the bead, syncs, evaluates the spec gate, merges interactive worktree branches, writes `status.json`, triggers thorough review by frequency or epic completion, and runs the between-iterations command.

The structural constraint is enforced by import cycles: `internal/runner/` imports `internal/pipeline/` only. No stage package imports `internal/runner/`. A new stage cannot attach to the orchestrator without defining a `pipeline.Stage` interface implementation registered in the constructor.

The prior `~170` runner test files are replaced. Acceptance-tagged tests (`//go:build acceptance`) are preserved in `internal/runner/acceptance/` as the behavioral contract. Each stage is tested independently against its interface using fakes.

## Acceptance Criteria

- `internal/pipeline/stage.go` defines the `Stage` interface with `Run(ctx context.Context, in Input) (Output, error)`, a `Decision` type with `Proceed`/`Skip`/`Block` constants, and `Input`/`Output` structs.
- Each of the five stage sub-packages (`prepare/`, `execute/`, `validate/`, `review/`, `epilogue/`) compiles and has its own unit tests that do not import `internal/runner/`.
- `internal/runner/` contains only `orchestrator.go` and `constructor.go` plus the `acceptance/` sub-package.
- `go build ./...` passes with the new structure.
- `go test -tags acceptance ./internal/runner/acceptance/...` passes (behavioral parity with prior runner).
- No stage sub-package imports `internal/runner/` (verified by `go build` — import cycle would fail).
- `constructor.go` supports Router-only construction without Claude binary in `Deps`.
- `epilogue/` stage always runs failure-path learning extraction regardless of tier or package filters.
- `epilogue/` stage tracks touched-packages across iterations for success-learning gating.
- Iteration log writer persists the `usage_limited` field when `UsageLimited=true`.
- Orchestrator assigns monotonically-increasing iteration numbers to scope-blocked beads.
- Orchestrator `Run` reads and merges existing global stats file rather than overwriting it.
- Build stage uses `StreamRun` (not `Run`) for Claude invocations.
- Build stage takes an explicit escalation flag; methodology policy method is named `ShouldRunPostSuccess`.

## Decisions

1. **Top-level package pipeline over sub-package split** Stages live in `internal/pipeline/<stage>/`, not `internal/runner/<stage>/`. The `runner/` namespace made it psychologically easy to reach back in. Top-level placement plus import cycle enforcement makes the boundary durable.

2. **Full test reset over audit** The ~170 runner test files tested internal wiring, not behavior. Auditing them would take as long as a rebuild and would still result in deletion of ~90%. Acceptance-tagged tests are preserved as the behavioral contract. Everything else is deleted.

3. **Escalation internal to Build stage** The orchestrator sees success or failure from Build, not the escalation chain. This keeps loop-level decisions (stop on failure, L3 stop line) independent of model selection mechanics.

4. **Input/Output data flow, no shared mutable state** Stages communicate through `Input`/`Output` structs. `ValidationFailures` from `Validate` flow into the next `Build` `Input`. The orchestrator owns loop-level state; no stage holds state between iterations.
