---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T05:16:37-05:00"
id: decompose-picker
source_spec: decompose-picker
---

# Decompose Picker Implementation Plan

**Goal:** Add an interactive picker to `gromit decompose` so it works without arguments, listing undecomposed plans for selection with a "Decompose all" batch option.

**Architecture:** Add a no-args picker mode to the decompose command following the established `plan.go`/`refine.go` picker pattern. Extract the core single-plan decomposition logic into a helper function that both picker-selected and batch paths call.

**Tech Stack:** Go, Cobra CLI, frontmatter package (YAML parsing)

**Spec:** `.gromit/specs/decompose-picker.md`

---

## Architecture

### Overview

All changes live in `cmd/gromit/decompose.go` plus tests and golden file updates. The approach:

1. Change `cobra.ExactArgs(1)` to `cobra.MaximumNArgs(1)`
2. Add `filterUndecomposedPlans()` helper that scans plans dir and reads frontmatter
3. Extract core decompose logic into `decomposeSinglePlan()` callable by both paths
4. Add picker UI (numbered list, "Decompose all" option for 2+ plans)
5. Add batch mode loop with progress and error-continue semantics

### Key Components

1. **`filterUndecomposedPlans(plansDir string, force bool)`** — Scans `.gromit/plans/`, reads frontmatter from each `.md` file, returns those where `decomposed` is false/missing. When `force` is true, returns all plans. Returns `[]planInfo` structs containing name, title, and path.

2. **`planInfo` struct** — Lightweight struct with `Name`, `Title`, `Path` fields for picker display.

3. **Picker UI** — Numbered list showing `name — title` for each plan. "Decompose all" as last entry when 2+ plans. Uses `bufio.Reader` pattern matching `plan.go` and `refine.go`.

4. **`decomposeSinglePlan()`** — Extracted from current `runDecompose`. Takes plan name, config, and flags. Returns error. Handles: file read, frontmatter check, Claude invocation, bead creation, frontmatter update.

5. **Batch mode** — Alphabetical order, progress output `[1/3] Decomposing api-endpoints...`, error-continue, summary at end.

### Integration Points

- `runDecompose` becomes a dispatcher: 0 args → picker, 1 arg → direct
- Reuses `extractSpecTitle()` from `refine.go` (same package, no import needed)
- `--force` threads into `filterUndecomposedPlans()` to show all plans
- `--review` respected per-plan in batch mode
- `--no-chain` suppresses chaining; in batch mode, chaining only offered after all plans complete

### Data Flow

1. No args → `filterUndecomposedPlans(plansDir, force)` → display picker → user selects → `decomposeSinglePlan()` or batch loop
2. With arg → `decomposeSinglePlan()` directly (current behavior, unchanged)

### Tradeoffs

- **All in decompose.go**: Follows existing pattern where each command file contains its own picker logic.
- **Manual picker over promptui**: The codebase uses `bufio.Reader` pickers, not `promptui.Select`. Staying consistent.
- **Reuse extractSpecTitle**: Already handles frontmatter skipping and `# Title` extraction.

## Test Strategy

### Unit Tests (`cmd/gromit/decompose_test.go`)

- `TestFilterUndecomposedPlans` — Table-driven: empty dir, all decomposed, none decomposed, mixed, force=true returns all, missing frontmatter = undecomposed, non-.md files ignored
- Plan title extraction already tested via `TestExtractSpecTitle` in `refine_test.go`

### E2E Tests (`cmd/gromit/cli_contract_test.go`)

- No-args with no undecomposed plans: prints helpful message, exits 0
- No-args with undecomposed plans: shows picker with plan names (use `runGromitWithStdin`)

### Golden File

- Regenerate `decompose.help.txt` since `Use` line changes

### Mocking Strategy

- Unit tests use real temp dirs with fixture `.md` files (same as `TestFilterUnplannedSpecs`)
- E2E tests use built binary + temp dirs (same as existing contract tests)
- No Claude/bead mocking needed — picker/filter logic is independently testable

---

## Implementation Tasks

### Task 1: Add `filterUndecomposedPlans` helper and unit tests

**Files:**
- Modify: `cmd/gromit/decompose.go`
- Create: `cmd/gromit/decompose_test.go`

**What to Do:**
Add a `planInfo` struct with `Name`, `Title`, `Path` fields. Add `filterUndecomposedPlans(plansDir string, force bool) ([]planInfo, error)` that:
- Reads all `.md` files from plansDir using `os.ReadDir`
- For each, reads frontmatter via `frontmatter.ReadFile()`
- Filters to `decomposed: false` or missing (unless `force` is true)
- Extracts title using `extractSpecTitle()` (reuse from refine.go)
- Returns sorted by name alphabetically

Write table-driven unit tests covering: empty dir, all decomposed, none decomposed, mixed, force flag, missing frontmatter, non-.md files ignored.

**Acceptance Criteria:**
- `filterUndecomposedPlans` returns only undecomposed plans when force=false
- `filterUndecomposedPlans` returns all plans when force=true
- Plans with missing `decomposed` field are treated as undecomposed

**Dependencies:** None

### Task 2: Extract `decomposeSinglePlan` and wire picker into `runDecompose`

**Files:**
- Modify: `cmd/gromit/decompose.go`

**What to Do:**
1. Change `Args: cobra.ExactArgs(1)` to `Args: cobra.MaximumNArgs(1)`.
2. Extract the body of `runDecompose` (from plan name resolution through chaining) into `decomposeSinglePlan(planName string, cfg *config.Config) error`. This function should respect the package-level `decomposeReview`, `decomposeForce`, and `decomposeNoChain` flags.
3. Update `runDecompose` to dispatch:
   - If `len(args) == 1`: call `decomposeSinglePlan(args[0], cfg)` directly (preserves current behavior).
   - If `len(args) == 0`: call `filterUndecomposedPlans()`, then show the picker (next task adds picker UI).

**Acceptance Criteria:**
- `gromit decompose <plan-name>` works identically to before (no behavior change for 1-arg path)
- `gromit decompose` with no args loads config and calls `filterUndecomposedPlans`

**Dependencies:** Task 1

### Task 3: Add picker UI and "Decompose all" batch mode

**Files:**
- Modify: `cmd/gromit/decompose.go`

**What to Do:**
Add the picker UI in the no-args path of `runDecompose`:
1. If no undecomposed plans: print "No undecomposed plans found. Create one with `gromit plan`." and return nil.
2. If exactly 1 plan: show picker with just that plan (no "Decompose all").
3. If 2+ plans: show numbered list + "Decompose all" as last entry.
4. Display format: `  1. plan-name — Plan Title` (use planInfo.Name and planInfo.Title).
5. Read user choice via `bufio.Reader`, validate input.
6. Single selection: call `decomposeSinglePlan()`.
7. "Decompose all" selection: loop over plans alphabetically, print progress `[1/3] Decomposing plan-name...`, call `decomposeSinglePlan()` for each, continue on error, print summary. Suppress `--no-chain` per-plan during batch (only offer chain after all complete).

**Acceptance Criteria:**
- Empty plan list shows helpful message and exits 0
- Single plan shows picker without "Decompose all" option
- 2+ plans shows picker with "Decompose all" as last option

**Dependencies:** Task 2

### Task 4: Update CLI contract tests and golden files

**Files:**
- Modify: `cmd/gromit/cli_contract_test.go`
- Modify: `cmd/gromit/testdata/golden/decompose.help.txt`

**What to Do:**
1. Update the golden file for decompose help text (the `Use` line changes from `decompose <plan-name>` to `decompose [plan-name]`, and the Long description should mention the picker mode).
2. Remove the "decompose missing argument" exit code test if present (no-args is now valid).
3. Add E2E test: `gromit decompose` in a temp dir with no plans dir → helpful message, exit 0.
4. Add E2E test: `gromit decompose` in a temp dir with undecomposed plan fixtures → picker output contains plan names (use `runGromitWithStdin` with selection input). This test verifies picker display, not full decompose (Claude isn't available).
5. Update the flag contract if the `--no-chain` flag needs to remain hidden or any new flags are added (none expected).

**Acceptance Criteria:**
- Golden file matches new help output
- E2E test confirms no-args empty case prints message and exits 0
- CLI contract tests pass with updated golden file

**Dependencies:** Task 3

---

## Notes

- The `--no-chain` flag behavior in batch mode: during "Decompose all", each individual `decomposeSinglePlan` call should suppress chaining (as if `--no-chain` were set), and chaining should only be offered once after all plans are processed. This avoids prompting the user to run `gromit run` after each plan.
- The `extractSpecTitle()` function works for plan files too since it just looks for `# Title` after skipping frontmatter.
- The picker input validation should match the existing pattern in `plan.go` — `fmt.Sscanf` with range check, returning `fmt.Errorf("invalid choice")` on bad input.
