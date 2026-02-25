---
created: 2026-02-20T00:00:00Z
decomposed: true
decomposed_at: "2026-02-25T23:09:17Z"
id: prompt-ab-framework
source_spec: prompt-ab-framework
---

# Prompt A/B Testing Framework Implementation Plan

**Goal:** Enable continuous experimentation on prompt variants per-phase using Thompson sampling, with persistent bandit state, default success criteria, and a CLI reporting command.

**Architecture:** A new `internal/experiment/` package owns bandit math, experiment loading, and state persistence. The runner hooks variant selection into `setupBeadContext`, applies overrides (model, budget, template, gate), scores outcomes after each invocation, and logs experiment/variant IDs to JSONL. A `gromit experiments` CLI command surfaces per-variant metrics and convergence status.

**Tech Stack:** Go, YAML (gopkg.in/yaml.v3), Thompson sampling with Beta distributions, JSONL logging

**Spec:** `.gromit/specs/prompt-ab-framework.md`

---

## Architecture

### Key Components

1. **`internal/experiment/` package**: Core bandit logic — experiment loading/validation, Thompson sampling arm selection, Beta distribution state management, convergence detection. Self-contained with no runner dependencies.

2. **`ExperimentConfig` in config**: Global experiment settings (enabled, min_sample_size, confidence_threshold) added to the existing config system following the established pattern.

3. **Experiment loader**: Reads `.gromit/experiments/*.yaml` at run start, validates them (known phases, well-formed variants, control present), returns map of phase -> active experiment.

4. **Bandit state persistence**: Per-experiment state files in `.gromit/experiments/state/` storing Beta distribution parameters and cost accumulators per variant.

5. **Runner integration**: In `setupBeadContext()`, query experiment manager for active variant. Override model/tier/budget/gate per variant spec. Pass variant ID through to logging. Score outcome after invocation.

6. **Logging integration**: `ExperimentID` and `VariantID` fields on `IterationLog` and `IterationResult`, flowing into JSONL and metrics pipeline.

7. **`gromit experiments` CLI command**: Reads bandit state and displays per-variant metrics, convergence status, cost comparison.

### Data Flow

```
Run() start -> loadExperiments(".gromit/experiments/") -> map[phase]*Experiment
  |
processSingleBead() -> for each phase:
  |
  experiment.SelectVariant(phase) -> Thompson sample -> variantID
  |
  applyVariantOverrides(bc, variant) -> override model/budget/template/gate
  |
  execute phase -> capture result
  |
  experiment.RecordOutcome(phase, variantID, success, cost) -> update Beta params
  |
  writeIterationLog() -> includes experiment_id, variant_id
```

### Integration Points

| Point | File | What Changes |
|---|---|---|
| Config section | `config_types.go` | Add `ExperimentConfig` struct and field |
| Defaults | `config_defaults.go` | Add experiment defaults |
| Validation | `config.go` | Add experiment config validation |
| Arm assignment | `process.go` | Select variant in `setupBeadContext()` |
| Prompt shaping | `context_types.go` | Add experiment fields to `Context` |
| Iteration log | `logger.go` | Add experiment fields to `IterationLog` |
| Result capture | `runtypes/types.go` | Add experiment fields to `IterationResult` |
| Log writing | `logging.go` | Map experiment fields |
| Router override | `callbacks.go` | Apply variant model/tier override |

---

## Test Strategy

### Test Levels

1. **Unit Tests** (`internal/experiment/`): Pure bandit math, state persistence, loading/validation, convergence. Highest value — no external dependencies.
2. **Unit Tests** (`internal/config/`): ExperimentConfig defaults, validation, YAML deser. Existing pattern.
3. **Integration Tests** (`internal/runner/`): Variant selection wired into setupBeadContext, scoring after invocation, experiment fields in IterationLog. Existing mock patterns.
4. **CLI Tests** (`cmd/gromit/`): `gromit experiments` with fixture state files. Table-driven.

### Mocking Strategy

- **Bandit math**: No mocks — pure functions with known inputs/outputs
- **State I/O**: `t.TempDir()` for file read/write
- **Runner integration**: Mock `experiment.Manager` interface for runner wiring; real Manager for end-to-end bandit behavior
- **Experiment files**: Inline YAML strings or fixture files in `testdata/`

### Key Test Cases

- Thompson sampling distribution matches expected probabilities
- Convergence detection at threshold; no false convergence below min sample size
- Valid/invalid YAML loading with clear error messages
- State persistence round-trip fidelity
- Variant overrides: model, template, budget, gate — each in isolation
- Tool-call cap scoring (failure when exceeded)
- force_variant bypasses bandit
- No experiment for phase = unchanged default behavior
- JSONL contains experiment_id/variant_id when active; absent when not
- Default success criteria per phase produce correct pass/fail

---

## Implementation Tasks

### Task 1: Define core experiment types

**Files:**
- Create: `internal/experiment/experiment.go`
- Test: `internal/experiment/experiment_test.go`

**What to Do:**
Define the foundational types for the experiment system. `Experiment` struct with ID, Phase, Description, Created, Control, Variants, SuccessCriteria, ForceVariant fields. `Variant` struct with ID, Template, Budget (MaxChars, LearningCapChars), Model, ToolCallCap, Gate (MinFilesChanged), Flags fields. `Manager` struct holding a map of phase -> *Experiment and a `stateDir` path. Add `NewManager(experiments []*Experiment, stateDir string) *Manager` constructor. Add `ExperimentForPhase(phase string) *Experiment` accessor returning nil when no experiment targets the phase. Define `ValidPhases` set: build, validate, review, refactor, analyze, learn.

**Acceptance Criteria:**
- `Experiment` and `Variant` structs have all fields from the spec with YAML tags
- `NewManager` constructs a Manager; `ExperimentForPhase` returns the correct experiment or nil
- Types compile and are usable from other packages

**Dependencies:** None

---

### Task 2: Implement Thompson sampling bandit

**Files:**
- Create: `internal/experiment/bandit.go`
- Test: `internal/experiment/bandit_test.go`

**What to Do:**
Implement the Thompson sampling arm selection. `BanditState` struct per experiment holding per-variant `ArmState` (Successes, Failures int; TotalCost float64; SampleCount int). `SelectVariant(state *BanditState, forceVariant string) string` function: if forceVariant is set, return it; otherwise draw from Beta(successes+1, failures+1) for each arm using `math/rand`, return the arm with the highest sample. `RecordOutcome(state *BanditState, variantID string, success bool, cost float64)` updates the arm's counters. `IsConverged(state *BanditState, minSamples int, confidenceThreshold float64) (bool, string)` uses Monte Carlo simulation (10,000 draws) to estimate probability that the best arm outperforms all others; returns true + winner ID when probability exceeds threshold and all arms have >= minSamples.

**Acceptance Criteria:**
- `SelectVariant` with known Beta params produces expected distribution (statistical test with tolerance over 1000 draws)
- `RecordOutcome` correctly increments successes/failures and accumulates cost
- `IsConverged` returns false below min sample size; returns true with correct winner when one arm dominates

**Dependencies:** Task 1

---

### Task 3: Implement experiment YAML loader with validation

**Files:**
- Create: `internal/experiment/loader.go`
- Test: `internal/experiment/loader_test.go`

**What to Do:**
Implement `LoadExperiments(dir string) ([]*Experiment, error)` that globs `*.yaml` files from the directory, unmarshals each into an `Experiment`, and validates: phase must be in `ValidPhases`, control must be present (empty struct is valid), each variant must have a unique ID, variant fields must be well-formed (template path non-empty if set, budget values non-negative, tool_call_cap non-negative). Return all experiments on success, or a descriptive error on the first validation failure including the filename and field. Handle empty directory gracefully (return empty slice, nil error).

**Acceptance Criteria:**
- Valid YAML files produce correct `Experiment` structs with all fields populated
- Unknown phase, missing control, duplicate variant IDs, negative budget values each produce a clear error naming the file and problem
- Empty directory returns empty slice with no error

**Dependencies:** Task 1

---

### Task 4: Implement bandit state persistence

**Files:**
- Create: `internal/experiment/state.go`
- Test: `internal/experiment/state_test.go`

**What to Do:**
Implement `LoadState(stateDir string, experimentID string) (*BanditState, error)` and `SaveState(stateDir string, experimentID string, state *BanditState) error`. State files are JSON at `<stateDir>/<experimentID>.json`. `LoadState` returns a fresh zero-state when the file doesn't exist (not an error). `SaveState` creates the stateDir if needed, writes atomically (write to temp file, rename). Add `InitializeState(experiment *Experiment) *BanditState` that creates a BanditState with ArmState entries for control plus all variants.

**Acceptance Criteria:**
- Write state, read it back — round-trip fidelity for all fields
- Missing state file returns fresh zero-state without error
- State directory is created if it doesn't exist

**Dependencies:** Task 2

---

### Task 5: Add ExperimentConfig to config system

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_normalize.go`
- Test: `internal/config/config_test.go` (add cases)

**What to Do:**
Add `ExperimentConfig` struct with fields: `Enabled bool` (yaml:"enabled"), `MinSampleSize int` (yaml:"min_sample_size"), `ConfidenceThreshold float64` (yaml:"confidence_threshold"), `ExperimentsDir string` (yaml:"experiments_dir"). Add `Experiment ExperimentConfig` field to `Config` struct (yaml:"experiment"). In `SetDefaults()`: when Enabled, default MinSampleSize to 20 if zero, ConfidenceThreshold to 0.95 if zero, ExperimentsDir to ".gromit/experiments" if empty. In `Validate()`: reject MinSampleSize < 1, ConfidenceThreshold outside (0, 1], empty ExperimentsDir when Enabled. In `NormalizeNilFields()`: no action needed (no slice/map fields).

**Acceptance Criteria:**
- `SetDefaults()` applies correct defaults when Enabled is true and fields are zero
- `Validate()` rejects invalid configurations with descriptive errors
- YAML deserialization populates all fields correctly

**Dependencies:** None

---

### Task 6: Add experiment fields to logging and result types

**Files:**
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/runner/logging.go`
- Test: `internal/runner/logging_test.go` (add cases)

**What to Do:**
Add `ExperimentID string` (json:"experiment_id,omitempty") and `VariantID string` (json:"variant_id,omitempty") to `IterationLog` in logger.go near existing diagnostic fields. Add matching `ExperimentID string` and `VariantID string` to `IterationResult` in runtypes/types.go. In `logging.go`'s log entry construction, map `result.ExperimentID` and `result.VariantID` to the log struct fields. Fields are additive — zero-value empty strings are omitted via omitempty.

**Acceptance Criteria:**
- `IterationLog` and `IterationResult` have both experiment fields with correct JSON tags
- `writeIterationLog` maps experiment fields from result to log entry
- Existing tests pass unchanged (fields are additive, zero-value is empty string)

**Dependencies:** None

---

### Task 7: Wire experiment manager into runner

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/constructor_with_deps.go`
- Modify: `internal/runner/process.go`
- Test: `internal/runner/process_test.go` (add cases)

**What to Do:**
Add `experimentMgr *experiment.Manager` field to Runner struct. In `newRunnerWithDepsImpl`, when `cfg.Experiment.Enabled`, call `experiment.LoadExperiments(cfg.Experiment.ExperimentsDir)` and `experiment.NewManager(experiments, stateDir)` where stateDir is `filepath.Join(cfg.Experiment.ExperimentsDir, "state")`. Store on Runner. When disabled or no experiments found, leave nil.

In `setupBeadContext()`, after tier/model selection, if `r.experimentMgr != nil`: call `r.experimentMgr.ExperimentForPhase("build")` — if non-nil, call `r.experimentMgr.SelectVariant(experiment)` to get variant ID. Store experiment ID and variant ID on `bc.Result.ExperimentID` and `bc.Result.VariantID`. Apply variant overrides: if variant has Model, override `bc.Result.Model`; if variant has Budget, override renderer budget params; if variant has Gate with MinFilesChanged and bc doesn't meet threshold, set a skip flag.

**Acceptance Criteria:**
- Runner with experiment config loads experiments and creates manager during construction
- `setupBeadContext` selects a variant and stores experiment/variant IDs on the result
- Variant model override changes the model used for the phase; nil experiment manager is a no-op

**Dependencies:** Tasks 1-5

---

### Task 8: Wire variant scoring after invocation

**Files:**
- Modify: `internal/runner/callbacks.go`
- Test: `internal/runner/callbacks_test.go` (add cases)

**What to Do:**
In `makeInvokeFn`, after the invocation completes and result is captured, if `r.experimentMgr != nil` and `bc.Result.ExperimentID != ""`: determine success using the phase's default success criteria (build: validation passed; validate: inherent pass/fail; review: ReviewResult.Passed; refactor: validation passes post-refactor; analyze: bead progresses; learn: non-empty learning). Call `r.experimentMgr.RecordOutcome(bc.Result.ExperimentID, bc.Result.VariantID, success, result.CostUSD)`. For tool-call cap enforcement: if variant has ToolCallCap > 0 and result's tool_call_count exceeds it, override success to false before recording.

**Acceptance Criteria:**
- Successful build invocation records success=true for the active variant
- Failed invocation records success=false
- Tool-call cap exceeded forces success=false regardless of phase outcome

**Dependencies:** Tasks 4, 7

---

### Task 9: Add variant template and budget overrides to prompt rendering

**Files:**
- Modify: `internal/prompt/context_types.go`
- Modify: `internal/prompt/render_methods.go`
- Test: `internal/prompt/prompt_test.go` (add cases)

**What to Do:**
Add `ExperimentID string` and `VariantID string` fields to `Context` in context_types.go. These are informational — templates can use `{{.ExperimentID}}` for traceability but don't need to branch on them.

For template overrides: the runner passes an alternate template path to the renderer. Add a `TemplateOverride string` field to the render method's input (or to Context). When non-empty, `render()` uses this path instead of the default template for the phase. This is a minimal change — `render()` already takes a template name and loads from disk.

For budget overrides: the runner calls existing `Renderer` setters (`SetBudgetMaxChars`, `SetBudgetLearningCapChars` — add these if they don't exist, following the `SetSkipBuildLearnings` pattern) before rendering. Restore original values after.

**Acceptance Criteria:**
- `Context` has experiment fields accessible in templates
- Template override renders from the specified alternate path
- Budget override changes the effective budget for that single render call

**Dependencies:** Task 1

---

### Task 10: Implement convergence reporting

**Files:**
- Modify: `internal/experiment/bandit.go` (if not already complete from Task 2)
- Create: `internal/experiment/report.go`
- Test: `internal/experiment/report_test.go`

**What to Do:**
Implement `GenerateReport(experiments []*Experiment, stateDir string) ([]ExperimentReport, error)` that loads state for each experiment and builds an `ExperimentReport` struct: ExperimentID, Phase, Description, Converged bool, Winner string, Variants []VariantReport. `VariantReport` has: ID, SampleCount, Successes, Failures, SuccessRate float64, AvgCost float64, BanditWeight float64 (current Thompson sampling probability). BanditWeight is computed by drawing 10,000 samples and counting win frequency per arm.

Also implement `FormatReport(reports []ExperimentReport) string` for human-readable text output and `FormatReportJSON(reports []ExperimentReport) ([]byte, error)` for machine-readable output.

**Acceptance Criteria:**
- Report correctly computes success rates and average costs from bandit state
- Converged experiments show the winner; non-converged show current weights
- Text and JSON formats are both well-formed

**Dependencies:** Tasks 2, 4

---

### Task 11: Add `gromit experiments` CLI command

**Files:**
- Create: `cmd/gromit/experiments.go`
- Test: `cmd/gromit/experiments_test.go`

**What to Do:**
Add a `experiments` subcommand to the CLI. It loads config, calls `experiment.LoadExperiments`, calls `experiment.GenerateReport`, and outputs the report. Support `--json` flag for machine-readable output. When no experiments exist, print "No active experiments." When experiments are converged, print a summary with the recommended winner.

Follow the existing CLI command pattern (see `cmd/gromit/stats.go` as reference).

**Acceptance Criteria:**
- `gromit experiments` displays per-variant sample size, success rate, avg cost, bandit weight, convergence status
- `--json` flag produces valid JSON output
- Empty experiments directory produces a clean "no experiments" message

**Dependencies:** Tasks 3, 10

---

### Task 12: Add experiment section to gromit.yaml and converged stderr output

**Files:**
- Modify: `gromit.yaml`
- Modify: `internal/runner/runner.go` (Run method)

**What to Do:**
Add a commented-out `experiment:` section to gromit.yaml with all fields documented:
```yaml
# experiment:
#   enabled: false
#   min_sample_size: 20        # Minimum observations before convergence check
#   confidence_threshold: 0.95  # Probability threshold for declaring a winner
#   experiments_dir: .gromit/experiments
```

In `Runner.Run()`, after experiments are loaded, check each for convergence. If any experiment is converged, emit a summary line to stderr: `[experiment] "<id>" converged: variant "<winner>" outperforms (N samples, p=X.XX)`.

**Acceptance Criteria:**
- gromit.yaml has the documented experiment section (commented out)
- Converged experiments emit a summary to stderr during `gromit run`

**Dependencies:** Tasks 5, 7

---

### Task 13: End-to-end verification

**Files:**
- No new files — verification only

**What to Do:**
Run `go test ./...`, `go vet ./...`, `go build ./...` to confirm all quality gates pass. Create a sample experiment YAML in a temp directory, run the loader, verify it parses. Run the bandit through 100 iterations with a rigged success rate and verify convergence. Verify `gromit experiments` command works with fixture data.

**Acceptance Criteria:**
- All tests pass, no vet warnings, clean build
- Sample experiment file loads and validates correctly
- Bandit converges correctly with sufficient data

**Dependencies:** All previous tasks

---

## Notes

- The existing `skipBuildLearnings` toggle in `prompt.go` is a precedent for experiment-driven prompt shaping — the A/B framework generalizes this pattern.
- The existing `.gromit/experiment.json` is a manual tracking file that predates this framework. It can coexist — the new framework uses `.gromit/experiments/` (plural, directory of YAML files) which is a different path.
- Phase success criteria use existing signals that are already computed (validation pass/fail, review result, etc.). No new success-detection logic is needed — just mapping existing results to bandit outcomes.
- The `force_variant` option is critical for debugging — it bypasses the bandit and always selects the specified variant, making experiment behavior deterministic and testable.
- State files use atomic writes (temp file + rename) to prevent corruption from concurrent runs or crashes mid-write.
