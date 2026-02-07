---
created: 2026-02-06T00:00:00Z
decomposed: true
decomposed_at: "2026-02-06T22:11:58-05:00"
id: interactive-session-flags
source_spec: interactive-session-flags
---

# Pass claude.flags to Interactive Sessions — Implementation Plan

**Goal:** Make `gromit refine` and `gromit plan` respect `claude.flags` and `claude.binary` from `gromit.yaml`, matching the behavior of non-interactive sessions.

**Architecture:** Insert `cfg.Claude.Flags` and `cfg.Claude.Binary` into the `exec.Command` calls in both `refine.go` and `plan.go`, with a `cfg == nil` guard defaulting to `"claude"` and empty flags.

**Tech Stack:** Go

**Spec:** `.gromit/specs/interactive-session-flags.md`

---

## Architecture

**Overview:**
Both `refine.go` and `plan.go` hardcode `exec.Command("claude", "--append-system-prompt", ...)`. The fix is to build an args slice that prepends `cfg.Claude.Flags` before the existing arguments, and use `cfg.Claude.Binary` instead of the hardcoded `"claude"` string.

**Integration Points:**
- Both files already import `config` and call `loadConfig()` with a `cfg == nil` guard
- The `ClaudeConfig.Flags` field already exists and is populated from `gromit.yaml`
- `decompose.go` already passes flags via `claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, ...)` — this is the reference pattern

**Change pattern (identical in both files):**
```go
// Before
claudeCmd := exec.Command("claude", "--append-system-prompt", systemPrompt, "Begin...")

// After
binary := "claude"
var flags []string
if cfg != nil {
    binary = cfg.Claude.Binary
    flags = cfg.Claude.Flags
}
args := append(flags, "--append-system-prompt", systemPrompt, "Begin...")
claudeCmd := exec.Command(binary, args...)
```

**Tradeoffs:**
- Also uses `cfg.Claude.Binary` (not just flags): consistency with `decompose.go`, and the binary field defaults to `"claude"` when config exists
- Inline construction over shared helper: only two call sites, a helper would be over-engineering

## Test Strategy

**Manual Testing:**
1. Run `gromit refine` / `gromit plan` with `claude.flags: ["--dangerously-skip-permissions"]` — Claude should launch without permission prompts
2. Run with `claude.flags: []` — behavior unchanged
3. Run without `gromit.yaml` — should still work with defaults
4. Set `claude.binary` to a custom path — should use that path

**Build Verification:**
- `go build ./cmd/gromit` compiles without errors
- `go test ./...` passes (no regressions)

## Implementation Tasks

### Task 1: Add config flags and binary to refine.go and plan.go

**Files:**
- Modify: `cmd/gromit/refine.go`
- Modify: `cmd/gromit/plan.go`

**What to Do:**
In both files, replace the hardcoded `exec.Command("claude", "--append-system-prompt", ...)` with a version that:
1. Resolves the binary from `cfg.Claude.Binary` (defaulting to `"claude"` when `cfg == nil`)
2. Prepends `cfg.Claude.Flags` before the existing `--append-system-prompt` and message arguments
3. Handles the `cfg == nil` case with empty flags

**Acceptance Criteria:**
- `gromit refine` uses `cfg.Claude.Binary` and passes `cfg.Claude.Flags` to the Claude CLI invocation
- `gromit plan` uses `cfg.Claude.Binary` and passes `cfg.Claude.Flags` to the Claude CLI invocation
- When `cfg == nil` (no config), both commands default to binary `"claude"` with no extra flags

**Dependencies:** None

**Notes:** The pattern in both files is identical. Follow the same `args` construction style used in `claude.go:61-65`.

---

## Notes

- This is a minimal, low-risk change — 4 lines of new code per file, no new imports needed
- The `cfg.Claude.Binary` default is `"claude"` (set in `config.go:152`), so it's always populated when config exists
- `normalizeNilFields` in `config.go:129-131` ensures `Flags` is never nil when config is loaded
