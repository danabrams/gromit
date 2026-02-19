---
id: prompt-ab-framework
source_ideas: []
created: 2026-02-19
---

# Prompt A/B Testing Framework

## Specification

A continuous experimentation framework that enables iterative A/B testing of prompt variants across Gromit's phases to minimize cost while maintaining build quality.

Each phase in the Gromit loop (build, validate, review, refactor, analyze, learn) is treated as an independent optimization problem. Experiments define one or more variants for a specific phase, where a variant is a bundle of changes to any combination of: prompt template text, budget parameters, model selection, tool-call budget, and gate conditions (e.g., skip refactor unless N files changed).

Experiments are defined as individual YAML files in `.gromit/experiments/`. Each experiment targets a single phase and defines a control (the current behavior) plus one or more challenger variants. A multi-armed bandit (Thompson sampling) allocates traffic across variants, automatically shifting invocations toward better-performing variants as data accumulates.

Each phase has default success criteria derived from existing log signals:

| Phase | Default Success Signal |
|---|---|
| Build | Validation passes (ideally first-pass) |
| Validate | Pass/fail inherent |
| Review | `ReviewResult.Passed` |
| Refactor | Validation still passes post-refactor |
| Analyze | Bead progresses on next attempt |
| Learn | Learning extracted (non-empty) |

Experiments can override these defaults when needed.

Tool-call budgets are soft-enforced: the variant's cap is communicated to the model in the prompt, and tool-call events are monitored from stream-JSON. If the model exceeds the budget, the invocation is scored negatively for that variant (counted as a failure for bandit purposes) but the process is not killed.

The bandit tracks cost and success per variant. When an experiment reaches statistical significance (configurable minimum sample size and confidence threshold), it is flagged as **converged** with a summary showing cost savings and success rate comparison. Promotion is manual — the human reviews results, updates the template/config to adopt the winning variant as the new default, and archives the experiment file.

### Experiment Lifecycle

1. **Define**: Create a YAML file in `.gromit/experiments/` specifying phase, control, and challenger variants
2. **Run**: The bandit allocates traffic during normal `gromit run` execution
3. **Converge**: Framework detects statistical significance and flags the experiment
4. **Promote**: Human reviews results, adopts the winner into the default config/templates
5. **Archive**: Experiment file is removed or moved to an archive directory

### Variant Levers

A variant can modify any combination of:

- **`template`**: Path to an alternative prompt template file (e.g., `.gromit/templates/PROMPT_build_v2.md`)
- **`budget`**: Override `max_chars` and/or `learning_cap_chars` for prompt budget shaping
- **`model`**: Override the model used for this phase (e.g., haiku instead of sonnet for review)
- **`tool_call_cap`**: Maximum tool calls before the variant is scored as a failure
- **`gate`**: Conditions under which the phase is skipped entirely (e.g., `min_files_changed: 5` for refactor)
- **`flags`**: Additional CLI flags passed to the provider

### Bandit Mechanics

- Uses Thompson sampling with Beta(successes+1, failures+1) priors
- Each phase maintains its own bandit state, stored in `.gromit/experiments/state/`
- Minimum sample size (default: 20) before a variant can be considered for convergence
- Convergence threshold: 95% probability that the best variant outperforms all others
- A `force_variant` option allows pinning a specific variant for debugging
- Cost is tracked per-variant as a secondary metric alongside success rate — the convergence report shows both

### Observability

- Every invocation logs which experiment/variant was active for that phase in the iteration JSONL (`experiment_id`, `variant_id`)
- `gromit experiments` CLI command shows status of all active experiments: sample sizes, current bandit weights, cost per variant, success rates, convergence status
- Converged experiments emit a summary to stderr during `gromit run`

## Acceptance Criteria

- Experiment YAML files in `.gromit/experiments/` are loaded at run start and validated (unknown phases, missing control, malformed variants are rejected with clear errors)
- Each phase invocation selects a variant via Thompson sampling from the active experiment for that phase (or uses default behavior if no experiment targets that phase)
- Variant selection is logged per-invocation in iteration JSONL with `experiment_id` and `variant_id` fields
- Tool-call caps specified in a variant are injected into the rendered prompt and tool-call count is compared against the cap when scoring the variant
- Bandit state persists across runs in `.gromit/experiments/state/` and correctly updates Beta distribution parameters on each observation
- Default success criteria per phase produce correct pass/fail scores without requiring experiment-level overrides
- An experiment is flagged as converged when the winning variant exceeds the confidence threshold with sufficient sample size
- `gromit experiments` CLI command displays active experiments with per-variant sample size, success rate, average cost, bandit weight, and convergence status
- Experiments with `force_variant` bypass the bandit and always select the specified variant
- A variant that changes only `model` correctly overrides the model for that phase while leaving all other config intact
- A variant that changes only `gate` conditions correctly skips the phase when conditions are not met and runs it when they are

## Decisions

1. **Per-phase variant assignment** Variants are assigned independently per phase, not per-bead or per-run. This treats each phase as a separate optimization problem, enabling faster convergence since phases have different cost profiles and success signals. The tradeoff is that cross-phase interactions (e.g., a minimal build prompt causing more review findings) won't be captured within a single experiment — but these can be detected by monitoring bead-level success rates across experiments.

2. **Multi-armed bandit over simple random split** Thompson sampling automatically shifts traffic toward winning variants, reducing the cost of running experiments (fewer iterations wasted on losing variants). A simple 50/50 split would be easier to implement but slower to converge and more expensive overall — exactly the opposite of the framework's goal.

3. **Soft tool-call caps via prompt + monitoring** Rather than killing the Claude process at the cap (risking incomplete work) or relying purely on prompt instructions (unenforceable), the framework injects the cap into the prompt and monitors actual tool-call count from stream-JSON. Exceeding the cap scores the variant as a failure. This lets the model attempt to comply while providing a hard metric for evaluation.

4. **Manual promotion** The framework presents evidence (cost savings, success rates, statistical significance) but does not automatically rewrite templates or config. Prompt changes can have subtle quality effects that metrics don't fully capture, and a human should review before making changes permanent. Automated promotion could be added later if trust in the metrics grows.

5. **Separate experiment files** Experiments are defined in individual YAML files rather than in `gromit.yaml`. Experiments are transient by nature — defined, run, converged, archived — and mixing them with stable config would be noisy. The main config only needs `experiments.enabled: true` and global defaults (min sample size, confidence threshold).

6. **Default success criteria per phase** Each phase has built-in success criteria derived from existing log signals, so spinning up a new experiment requires only defining the variant — not the success metric. Custom criteria can be specified as overrides for unusual experiments. This keeps experiment creation lightweight, encouraging more experimentation.

## Research & Context

### Current State

**Prompt rendering** (`internal/prompt/prompt.go`): Templates are rendered via Go `text/template` with a `Context` struct. `ShapeContextForBudget()` in `budget.go` trims context to fit `prompt.budget.max_chars`. Variant template and budget overrides would hook into this rendering pipeline.

**Phase orchestration** (`internal/runner/process.go`, `process_methodology.go`): Each phase invocation flows through distinct functions (`executeClaudeInvocation`, `runValidationWithRecoveryForStage`, `runRefactorAndPostChecks`, etc.). Variant selection would inject at the point where each phase builds its invocation parameters.

**Tool-call monitoring** (`internal/claude/claude.go`): Tool events are already parsed from stream-JSON via `ToolCallHandler` callbacks, and `tool_call_count` is logged per invocation. The infrastructure for counting tool calls exists — the framework needs to add comparison against a variant's cap.

**Cost/token logging** (`internal/logger/logger.go`): `IterationLog` already captures `cost_usd`, `input_tokens`, `output_tokens` per invocation. The framework adds `experiment_id` and `variant_id` fields to enable per-variant cost aggregation.

**Configuration** (`internal/config/config.go`): The config system supports ~30 top-level sections. A new `ExperimentsConfig` section would hold global settings (enabled, min sample size, confidence threshold). Individual experiment files would be loaded separately from `.gromit/experiments/`.

**Existing metrics** (`.gromit/metrics/`): `iteration_metrics.jsonl` and `process_trend.json` provide rolling-window analytics. Experiment results would be a parallel analytics surface, not a replacement for existing metrics.

### Experiment File Format (Example)

```yaml
id: build-minimal-prompt
phase: build
description: "Test whether a stripped-down build prompt reduces cost without hurting first-pass success"
created: 2026-02-19

control:
  # Empty means "use current defaults for everything"

variants:
  - id: minimal
    template: .gromit/templates/PROMPT_build_minimal.md
    budget:
      max_chars: 10000
      learning_cap_chars: 0

  - id: minimal-with-learnings
    template: .gromit/templates/PROMPT_build_minimal.md
    budget:
      max_chars: 12000
      learning_cap_chars: 1000

success_criteria: null  # Use phase default (validation passes)
```

```yaml
id: review-model-downgrade
phase: review
description: "Test whether haiku produces equivalent review quality at lower cost"
created: 2026-02-19

control: {}

variants:
  - id: haiku-review
    model: haiku

  - id: haiku-with-tool-cap
    model: haiku
    tool_call_cap: 15
```

```yaml
id: skip-refactor-small-changes
phase: refactor
description: "Test whether skipping refactor for small changesets affects quality"
created: 2026-02-19

control: {}

variants:
  - id: skip-under-5-files
    gate:
      min_files_changed: 5
```
