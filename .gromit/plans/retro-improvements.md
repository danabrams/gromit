---
created: 2026-02-06T00:00:00Z
decomposed: true
decomposed_at: "2026-02-06T22:31:05-05:00"
id: retro-improvements
source_spec: retro-improvements
---

# Retro Improvements Implementation Plan

**Goal:** Complete the retro improvements by removing dead apply code and surfacing bead comments in the analysis prompt.

**Architecture:** Most of the spec is already implemented (enrichment, closed-bead filtering, Claude Code launch, interactive.go deletion). The remaining work is cleanup: remove dead `applyChanges`/`apply` param, update the prompt template to show comments, and fix test signatures.

**Tech Stack:** Go, Go templates

**Spec:** `.gromit/specs/retro-improvements.md`

---

## Architecture

### Current State (Already Implemented)

- `enrichBeadStats()` in `retro.go` calls `bd show` and `bd comments` for each stuck bead
- `BeadStats` struct has `Status`, `CloseReason`, `Comments` fields
- Closed beads filtered out before prompt rendering in `Run()`
- `LaunchClaudeCode()` launches interactive Claude Code session with analysis
- `runRetro()` in `main.go` uses Claude Code by default, `--non-interactive` writes to file
- `interactive.go` already deleted

### Remaining Changes

1. **Remove dead code in `retro.go`**: The `applyChanges()` method and `apply bool` parameter on `Run()` are dead — `runRetro()` always passes `false`. The `--non-interactive` file-writing is handled directly in `main.go`. Remove both.

2. **Update prompt template**: The stuck beads table shows failure counts but not comments. Add a per-bead comments section so Claude has context for root cause analysis.

3. **Update tests**: Three tests call `Run(ctx, false)` — update to `Run(ctx)`.

## Test Strategy

- Update existing `retro_test.go` tests to match new `Run()` signature (drop `apply` param)
- Existing enrichment and filtering tests are unaffected (they don't call `Run()`)
- Manual verification: run `gromit retro --non-interactive` to confirm template renders comments
- Compilation check: `go build ./cmd/gromit` confirms no broken call sites

## Implementation Tasks

### Task 1: Remove dead apply code from retro.go

**Files:**
- Modify: `internal/retro/retro.go`

**What to Do:**
- Remove the `apply bool` parameter from `Run()` method signature (line 66). Change `func (r *Retro) Run(ctx context.Context, apply bool)` to `func (r *Retro) Run(ctx context.Context)`.
- Remove the `if apply && claudeResult.Success` block (lines 136-141) that calls `applyChanges`.
- Remove the `applyChanges()` method entirely (lines 234-243).
- Remove the `"path/filepath"` import if it becomes unused after removing `applyChanges` (it's still used in `Run()` for `logsDir` so it stays).

**Acceptance Criteria:**
- `Run()` takes only `ctx context.Context` parameter
- `applyChanges` method no longer exists
- `go build ./...` succeeds

**Dependencies:** None

### Task 2: Update runRetro caller in main.go

**Files:**
- Modify: `cmd/gromit/main.go`

**What to Do:**
- Update the `r.Run(ctx, false)` call on line 202 to `r.Run(ctx)`.

**Acceptance Criteria:**
- `runRetro` calls `r.Run(ctx)` without the `apply` parameter
- `go build ./cmd/gromit` succeeds

**Dependencies:** Task 1

### Task 3: Update retro tests

**Files:**
- Modify: `internal/retro/retro_test.go`

**What to Do:**
- Update `TestRunNilReceiver`: change `r.Run(context.Background(), false)` to `r.Run(context.Background())`
- Update `TestRunNilClaudeClient`: same change
- Update `TestRunNilLearningsFile`: same change

**Acceptance Criteria:**
- All three tests updated to new signature
- `go test ./internal/retro/...` passes

**Dependencies:** Task 1

### Task 4: Add comments to retro prompt template

**Files:**
- Modify: `.gromit/templates/PROMPT_retro.md`

**What to Do:**
After the stuck beads table (the `{{- range $id, $stats := .BeadStats }}` block), add a section that lists comments for each bead that has them. Use Go template syntax to iterate over `BeadStats` and render comments:

```
{{- range $id, $stats := .BeadStats }}
{{- if $stats.Comments }}

**{{ $stats.BeadID }}** ({{ $stats.BeadTitle }}) comments:
{{- range $stats.Comments }}
- {{ . }}
{{- end }}
{{- end }}
{{- end }}
```

This gives Claude visibility into what's been said about each stuck bead, enabling better root cause analysis instead of blind guessing.

**Acceptance Criteria:**
- Template renders bead comments when present
- Template renders cleanly when beads have no comments
- Existing table format unchanged

**Dependencies:** None

---

## Notes

- The `proposals.go` file is intentionally kept — it's still used by the `--non-interactive` path conceptually (the analysis output contains structured proposals that could be parsed).
- `LaunchClaudeCode` hardcodes `"claude"` as the binary. There's a separate open bead (ralph-runner-ziid) for using `cfg.Claude.Binary` and `cfg.Claude.Flags` across commands — that's out of scope for this spec.
- Tasks 1-3 are tightly coupled (signature change). Task 4 is independent and can be done in parallel.
