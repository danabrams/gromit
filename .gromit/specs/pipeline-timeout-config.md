---
id: pipeline-timeout-config
source_ideas: []
created: 2026-02-15
epic: run-loop-reliability
---

# Configurable Pipeline Invocation Timeout

## Specification

The `claudeClientAdapter` in `cmd/gromit/adapters.go` hardcodes a 30-minute context timeout for all pipeline invocations (decompose, review). The runner's build invocations already respect `claude.timeout` and per-model overrides via `TimeoutsForModel()`, but the pipeline adapter ignores configuration entirely.

A new `pipeline_timeout` field in the `claude:` config section controls how long pipeline phases may run:

```yaml
claude:
  pipeline_timeout: 1800  # seconds for pipeline phases: decompose, review (default: 1800)
```

**Behavior:**
- The `claudeClientAdapter` struct gains a `Timeout time.Duration` field.
- The `Run` method uses `a.Timeout` instead of the hardcoded `30*time.Minute`.
- Both construction sites (decompose in `cmd/gromit/decompose.go` and review in `cmd/gromit/review.go`) pass `time.Duration(cfg.Claude.PipelineTimeout) * time.Second` when creating the adapter.

## Acceptance Criteria

- `ClaudeConfig` has a `PipelineTimeout int` field that deserializes from `pipeline_timeout` in YAML.
- `SetDefaults()` sets `PipelineTimeout` to 1800 when the field is zero.
- Loading a YAML file with `claude.pipeline_timeout: 2400` produces `cfg.Claude.PipelineTimeout == 2400`.
- `claudeClientAdapter` uses its `Timeout` field, not a hardcoded duration.
- The decompose and review construction sites pass the configured pipeline timeout to the adapter.

## Decisions

1. **Single field, not per-phase.** Decompose and review are both pipeline orchestration calls with similar runtime profiles. One knob covers both. If review needs its own override later, `review.timeout` already exists for that purpose.

2. **Default preserves current behavior.** 1800 seconds (30 minutes) matches the existing hardcoded value, so users who don't touch the config see no change.

3. **Field lives in `claude:` section.** Pipeline invocations are Claude CLI calls, so the timeout belongs alongside the other Claude timeout knobs (`timeout`, `stall_timeout`, `bead_timeout`, `analysis_timeout`).

## Research & Context

### Current State

The adapter in `cmd/gromit/adapters.go` (lines 12-31) wraps `claude.Client` for the `pipeline.ClaudeClient` interface. Its `Run` method creates a context with `30*time.Minute` hardcoded.

Two files construct this adapter:
- `cmd/gromit/decompose.go:166` — decompose pipeline
- `cmd/gromit/review.go:381-383` — review pipeline

The `ClaudeConfig` struct in `internal/config/config.go:103-112` already has `Timeout`, `StallTimeout`, `StallTimeoutActive`, `BeadTimeout`, and `AnalysisTimeout` fields with defaults in `SetDefaults()` (lines 301-315).

The runner's own adapters in `internal/runner/adapters.go` already use `cfg.Claude.TimeoutsForModel()` — that code path is unaffected.

### Key Files

- `cmd/gromit/adapters.go` — adapter struct and Run method (the hardcoded timeout)
- `cmd/gromit/decompose.go` — decompose construction site (line 166)
- `cmd/gromit/review.go` — review construction site (lines 381-383)
- `internal/config/config.go` — ClaudeConfig struct (lines 103-112) and SetDefaults (lines 301-315)
- `internal/config/config_test.go` — existing timeout default/load tests
- `gromit.yaml` — reference config, `claude:` section (lines 119-141)
