---
created: 2026-02-06T00:00:00Z
decomposed: true
decomposed_at: "2026-02-06T22:25:40-05:00"
id: pipeline-chaining
source_spec: pipeline-chaining
---

# Pipeline Chaining Implementation Plan

**Goal:** After each pipeline stage exits, the CLI offers to run the next stage automatically, keeping the user in flow.

**Architecture:** A new `cmd/gromit/chain.go` file provides shared confirm/exec utilities and multi-spec orchestration. Each command (refine, plan, decompose) gets chaining logic inserted after its success path. A hidden `--no-chain` flag on plan and decompose prevents nested chaining when refine orchestrates the full multi-spec pipeline.

**Tech Stack:** Go, Cobra (CLI framework), bufio (stdin reading), os/exec (subprocess invocation)

**Spec:** `.gromit/specs/pipeline-chaining.md`

---

## Architecture

### Key Components

1. **`cmd/gromit/chain.go`** — New file with shared utilities:
   - `confirmPrompt(reader *bufio.Reader, prompt string, defaultYes bool) bool` — Y/n or y/N prompt using bufio (consistent with triage.go pattern)
   - `execGromit(args ...string) error` — finds current binary via `os.Executable()`, runs as subprocess with stdio connected
   - `chainAfterRefine(specNames []string, plansDir string)` — three-phase multi-spec orchestration, passes `--no-chain` to subprocesses

2. **Modified `cmd/gromit/decompose.go`** — Hidden `--no-chain` flag. After bead creation success, offer `gromit run` (default: no).

3. **Modified `cmd/gromit/plan.go`** — Hidden `--no-chain` flag. After plan creation confirmed, offer `gromit decompose <name>` (default: yes). Does NOT pass `--no-chain` to decompose, so it naturally cascades to the run offer.

4. **Modified `cmd/gromit/refine.go`** — After spec listing, call `chainAfterRefine(specNames, plansDir)`.

### Data Flow

**Standalone plan:**
```
plan exits → plan file exists → "Run gromit decompose <name>? [Y/n]" →
  exec gromit decompose <name> (no --no-chain) →
    decompose creates beads → "Run gromit run? [y/N]" → user decides
```

**Standalone decompose:**
```
decompose exits → beads created → "Run gromit run? [y/N]" → user decides
```

**Refine (multi-spec orchestration):**
```
refine exits → chainAfterRefine(["specA", "specB"], plansDir) →
  Phase 1 (Planning):
    "Run gromit plan specA? [Y/n]" → exec gromit plan specA --no-chain → check plan exists → planned=["specA"]
    "Run gromit plan specB? [Y/n]" → exec gromit plan specB --no-chain → check plan exists → planned=["specA","specB"]
    (declining skips remaining plans)
  Phase 2 (Decomposition):
    "Run gromit decompose specA? [Y/n]" → exec gromit decompose specA --no-chain → exit 0 → decomposed++
    "Run gromit decompose specB? [Y/n]" → exec gromit decompose specB --no-chain → exit 0 → decomposed++
    (declining skips remaining decomposes)
  Phase 3 (Run):
    "Run gromit run? [y/N]" → user decides
    (only offered if decomposed > 0)
```

### Nested Chaining Prevention

When refine orchestrates the pipeline, it passes `--no-chain` to plan and decompose subprocesses. This prevents:
- Plan from chaining to decompose (refine manages decompose phase separately)
- Decompose from chaining to run (refine manages run offer after all decomposes)

When plan is invoked standalone, it does NOT pass `--no-chain` to decompose, allowing natural cascade: plan → decompose → run.

### Tradeoffs

- **Hidden `--no-chain` flag vs env var**: Flag is more Go-idiomatic, testable, and traceable.
- **`os.Executable()` vs `os.Args[0]`**: `os.Executable()` resolves the actual binary path. Fallback to `os.Args[0]`.
- **File check vs exit code for plan**: Plan exits 0 even when no plan is created. File existence is ground truth.
- **Exit code for decompose**: Decompose returns error on failure — exit code is reliable.

## Test Strategy

### Unit Tests
- `TestConfirmPrompt` — table-driven tests for `confirmPrompt` with injected `*bufio.Reader` via `strings.NewReader`
- Cases: "y", "n", empty with default-yes, empty with default-no, uppercase, whitespace

### Manual Testing
- Single spec: refine → plan → decompose → run offer
- Multi spec: refine → plan all → decompose all → run offer
- Decline at each point: verify clean exit, phase skipping
- `--no-chain` flag: verify it suppresses chaining

### Mocking
- `confirmPrompt` takes `*bufio.Reader` parameter for test injection
- `execGromit` is not unit tested (thin exec wrapper)

### Test Files
- `cmd/gromit/chain_test.go`

## Implementation Tasks

### Task 1: Create chain utilities

**Files:**
- Create: `cmd/gromit/chain.go`
- Create: `cmd/gromit/chain_test.go`

**What to Do:**
Create `cmd/gromit/chain.go` with three functions:

1. `confirmPrompt(reader *bufio.Reader, prompt string, defaultYes bool) bool` — Prints the prompt, reads a line from reader, trims whitespace, checks for y/yes/n/no (case-insensitive). Empty input uses the default. Returns true for yes, false for no.

2. `execGromit(args ...string) error` — Uses `os.Executable()` to find the current binary (fallback to `os.Args[0]`). Creates `exec.Command` with the args, connects Stdin/Stdout/Stderr to `os.Stdin`/`os.Stdout`/`os.Stderr`. Runs and returns error. Treats `*exec.ExitError` as nil (same pattern as existing Claude invocations — the subprocess printed its own errors).

3. `chainAfterRefine(specNames []string, plansDir string)` — Orchestrates the three-phase multi-spec flow:
   - Creates a `bufio.NewReader(os.Stdin)`
   - Phase 1: Loop through specNames, offer `gromit plan <name> --no-chain` for each. After subprocess exits, check if `.gromit/plans/<name>.md` exists. Collect planned names. If user declines, break.
   - Phase 2: Loop through planned names, offer `gromit decompose <name> --no-chain` for each. Track how many succeeded (exit code 0). If user declines, break.
   - Phase 3: If any decomposed, offer `gromit run` with default-no.

Create `cmd/gromit/chain_test.go` with table-driven `TestConfirmPrompt` covering: "y", "n", "Y", "N", "yes", "no", empty with default-yes, empty with default-no, whitespace-padded inputs.

**Acceptance Criteria:**
- `confirmPrompt` correctly returns true/false for all input variants and defaults
- `execGromit` finds the current binary and runs a subprocess with stdio connected
- `chainAfterRefine` implements the three-phase flow with decline-skips-rest behavior

**Dependencies:** None

**Notes:**
- Use `bufio.NewReader(os.Stdin)` pattern from triage.go
- `confirmPrompt` takes a reader parameter so tests can inject `strings.NewReader`
- Prompt format: `Run 'gromit plan <name>'? [Y/n]: ` or `Run 'gromit run'? [y/N]: `

### Task 2: Add chaining to decompose command

**Files:**
- Modify: `cmd/gromit/decompose.go`

**What to Do:**
Add a hidden `--no-chain` flag (`decomposeNoChain bool`). After the success message at line 197 (`fmt.Printf("\n✓ Created %d bead(s)...")`), if `!decomposeNoChain`, create a `bufio.NewReader(os.Stdin)`, call `confirmPrompt(reader, "Run 'gromit run'?", false)`. If confirmed, call `execGromit("run")`.

**Acceptance Criteria:**
- After successful bead creation, prompts `Run 'gromit run'? [y/N]: ` with no as default
- `--no-chain` flag suppresses the prompt entirely
- Declining exits cleanly with no error

**Dependencies:** Task 1

### Task 3: Add chaining to plan command

**Files:**
- Modify: `cmd/gromit/plan.go`

**What to Do:**
Add a hidden `--no-chain` flag (`planNoChain bool`). After the plan-exists check at line 197-198, if `!planNoChain` and plan file exists, create a `bufio.NewReader(os.Stdin)`, call `confirmPrompt(reader, "Run 'gromit decompose <specName>'?", true)`. If confirmed, call `execGromit("decompose", specName)` — note: NO `--no-chain`, so decompose naturally cascades to its own run offer.

**Acceptance Criteria:**
- After plan creation, prompts `Run 'gromit decompose <name>'? [Y/n]: ` with yes as default
- `--no-chain` flag suppresses the prompt entirely
- Declining exits cleanly with no error

**Dependencies:** Task 1

### Task 4: Add chaining to refine command

**Files:**
- Modify: `cmd/gromit/refine.go`

**What to Do:**
After the spec listing block (line 213-218), if `len(createdSpecs) > 0`, extract spec names from paths (using `strings.TrimSuffix(filepath.Base(path), ".md")`), resolve plansDir from config, and call `chainAfterRefine(specNames, plansDir)`.

**Acceptance Criteria:**
- After specs are created, the multi-spec chaining flow is initiated
- Single spec follows the same flow (just one iteration per phase)
- No chaining if no specs were created

**Dependencies:** Task 1

### Task 5: Verify compilation and run tests

**Files:** (none modified)

**What to Do:**
Run `go build ./cmd/gromit`, `go test ./...`, and `golangci-lint run ./...` to verify no compilation errors, test failures, or lint issues.

**Acceptance Criteria:**
- `go build ./cmd/gromit` succeeds
- `go test ./...` passes (including new chain_test.go)
- `golangci-lint run ./...` reports no issues

**Dependencies:** Tasks 1-4

---

## Notes

- The `--no-chain` flag is hidden (not shown in `--help`) but exists for internal use by the chaining mechanism and for debugging.
- Subprocess errors from chained commands are not propagated to the parent command. The parent already succeeded (created its artifact). Chaining failures are printed but don't affect the parent's exit code.
- The `confirmPrompt` function takes a `*bufio.Reader` parameter rather than reading from `os.Stdin` directly, enabling unit testing with injected input. Callers create their own `bufio.NewReader(os.Stdin)`.
