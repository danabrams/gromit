# Configurable Pipeline Timeout

**Date:** 2026-02-15
**Status:** Approved

## Problem

The `claudeClientAdapter` in `cmd/gromit/adapters.go` hardcodes a 30-minute timeout for all pipeline invocations (decompose, review). The runner's build invocations already respect `claude.timeout` and per-model overrides, but the pipeline adapter ignores configuration entirely.

## Design

Add a `pipeline_timeout` field to the `claude:` config section:

```yaml
claude:
  timeout: 900              # build invocations (existing)
  pipeline_timeout: 1800    # pipeline phases: decompose, review (new)
```

**Default: 1800 seconds (30 minutes)** — preserves current behavior.

## Changes

### 1. Config (`internal/config/config.go`)

Add `PipelineTimeout int` to `ClaudeConfig`. Default to 1800 in `SetDefaults()`.

### 2. Adapter (`cmd/gromit/adapters.go`)

Add a `Timeout time.Duration` field to `claudeClientAdapter`. Use it in the `Run` method instead of `30*time.Minute`.

### 3. Wiring (`cmd/gromit/decompose.go`, `cmd/gromit/review.go`)

Pass `time.Duration(cfg.Claude.PipelineTimeout) * time.Second` when constructing the adapter.

## What stays the same

- Runner timeout system (`TimeoutsForModel`, stall timeouts, bead timeouts)
- `ClaudeClient` interface signature: `Run(prompt, model)`
- Meaning of `claude.timeout` (build invocations only)
- Review's existing `timeout` field (different semantic, used elsewhere)
