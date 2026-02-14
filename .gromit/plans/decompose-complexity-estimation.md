---
id: decompose-complexity-estimation
source_spec: decompose-complexity-estimation
created: 2026-02-14
decomposed: false
---

# Pre-Build Complexity Estimation via File Count

**Goal:** Add a file-count estimate to decompose output so the runner can route high-scope beads to stronger models before they fail.

**Architecture:** Extend `beadDef` with `estimated_files`, update the decompose skill to instruct Claude to output the field, auto-apply `complexity:high` and `estimated-files:N` labels during bead creation, and carry the estimate through `IterationResult` to `IterationLog` for calibration.

**Tech Stack:** Go

**Spec:** `.gromit/specs/decompose-complexity-estimation.md`

---

## Architecture

**Overview:**
The decompose agent already understands bead scope. We ask it to output a number it already knows — how many files will this bead touch — and use that to auto-apply complexity labels that feed into existing model selection routing.

**Key Components:**
1. **`internal/pipeline/decompose.go`**: Extended `beadDef` struct and label logic in the creation loop
2. **`skills/gromit-decompose/SKILL.md`**: Updated field list, output example, and description guidelines
3. **`internal/logger/logger.go`**: `IterationLog` with new `EstimatedFiles` field
4. **`internal/runner/runtypes/types.go`**: `IterationResult` with new `EstimatedFiles` field
5. **`internal/runner/runner.go`**: Label parsing in `setupBeadContext` or `processBead`, log population in `writeIterationLog`

**Integration Points:**
- Decompose creates beads with `complexity:high` and `estimated-files:N` labels via existing `CreateWithDepsAndDescription()`
- `SelectModel()` in `config.go:484` already routes `complexity:high` to opus — no changes needed
- Runner parses `estimated-files:N` from `bead.Labels` and writes to iteration log
- Retro analysis can compare `estimated_files` in logs against actual diff file count (already available via `parseDiffFiles()`)

**Data Flow:**
```
Claude JSON output → beadDef.EstimatedFiles
  → labels: ["complexity:high", "estimated-files:7"]
  → bd create with labels
  → bd ready returns bead with labels
  → setupBeadContext parses estimated-files:N → IterationResult.EstimatedFiles
  → writeIterationLog → IterationLog.EstimatedFiles → JSONL
```

**Files to Modify:**
- `internal/pipeline/decompose.go` — Add `EstimatedFiles` to `beadDef`, add label logic
- `skills/gromit-decompose/SKILL.md` — Add `estimated_files` to field list, example, guidelines
- `internal/logger/logger.go` — Add `EstimatedFiles` to `IterationLog`
- `internal/runner/runtypes/types.go` — Add `EstimatedFiles` to `IterationResult`
- `internal/runner/runner.go` — Parse label, populate result, write to log
- `internal/runner/process.go` — Parse `estimated-files:N` label in `setupBeadContext`

**Files to Create:** None.

**Tradeoffs:**
- **Label vs description encoding**: Labels — structured, queryable, consistent with `spec:` and `complexity:` patterns
- **Threshold 5 not 3**: Matches CLAUDE.md "soft file limit of 4-5", avoids over-escalation of routine 4-file changes (interface+impl+mock+test)
- **Route to opus, not auto-split**: Leverages existing `complexity:high -> opus` routing without new infrastructure

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Cover each changed component — JSON parsing, label logic, log serialization, label parsing
2. **No integration tests needed**: All changes wire through existing infrastructure already tested

**Key Test Cases:**

Decompose (`internal/pipeline/decompose_test.go`):
- `beadDef` with `estimated_files: 7` parses correctly from JSON
- `beadDef` without `estimated_files` parses correctly (backward compat, defaults to 0)
- Bead creation adds `complexity:high` label when `EstimatedFiles > 5`
- Bead creation does NOT add `complexity:high` when `EstimatedFiles <= 5`
- Bead creation adds `estimated-files:N` label when `EstimatedFiles > 0`
- Bead creation does NOT add `estimated-files:0` label when `EstimatedFiles == 0`

Logger (`internal/logger/logger_test.go`):
- `IterationLog` with `EstimatedFiles` serializes to JSON with `estimated_files` key
- `IterationLog` with `EstimatedFiles == 0` omits the field (omitempty)

Runner (`internal/runner/`):
- `estimated-files:7` label populates `IterationResult.EstimatedFiles = 7`
- No `estimated-files:` label results in `EstimatedFiles = 0`
- Malformed `estimated-files:abc` label results in `EstimatedFiles = 0`

**Mocking Strategy:**
- Decompose tests use existing `mockBeadClient` / `mockClaudeClient` patterns
- Runner tests use existing mock patterns for bead and logger

**Test Organization:**
- Tests live alongside implementation in existing `_test.go` files
- No new test files needed

## Implementation Tasks

### Task 1: Add `estimated_files` to beadDef and apply labels during bead creation

**Files:**
- Modify: `internal/pipeline/decompose.go`
- Test: `internal/pipeline/decompose_test.go`

**What to Do:**
Add `EstimatedFiles int` field with `json:"estimated_files,omitempty"` tag to the `beadDef` struct (line 17-23). In the bead creation loop (line 86-126), after building the initial `spec:<name>` label (line 91), add conditional logic: if `def.EstimatedFiles > highComplexityFileThreshold` (const = 5), append `complexity:high` to labels; if `def.EstimatedFiles > 0`, append `fmt.Sprintf("estimated-files:%d", def.EstimatedFiles)` to labels. Also update `newCreatedBeadFromDef` (line 182-189) to propagate the new labels to `CreatedBead` so review mode shows accurate labels. Update the prompt template (line 156) to mention `estimated_files` in the field list instruction.

**Acceptance Criteria:**
- `beadDef` includes `EstimatedFiles int` with `json:"estimated_files,omitempty"` tag
- `Decompose()` adds `complexity:high` label when `EstimatedFiles > 5`
- `Decompose()` adds `estimated-files:N` label when `EstimatedFiles > 0` and does not add it when `EstimatedFiles == 0`

**Dependencies:** None

**Notes:** The `highComplexityFileThreshold` const should be package-level. Backward compat is automatic via `omitempty` — existing JSON without the field parses to 0. The `newCreatedBeadFromDef` function currently hardcodes only `spec:<name>` — it needs to accept the full label slice or be updated to also apply complexity labels for review mode accuracy.

### Task 2: Update decompose skill with `estimated_files` field

**Files:**
- Modify: `skills/gromit-decompose/SKILL.md`

**What to Do:**
Add `estimated_files` to the Bead Definition Fields section (after `depends_on_index`, line 72): `estimated_files`: Integer count of files this bead will create or modify (include test files, mock files, and any touched interfaces). Add the field to the Output Format JSON example (lines 104-139) — each example bead should include a realistic `estimated_files` value. Add to the Description Guidelines (line 80-84): "Include a realistic file count in `estimated_files` — count every file that will have at least one line changed, including test files, mock files, and interface files." Update the "Labels and Metadata" section (line 149-156) to note that `complexity:high` is now auto-applied when `estimated_files > 5`.

**Acceptance Criteria:**
- `estimated_files` appears in the Bead Definition Fields list with description
- Output Format example includes `estimated_files` in all sample beads
- Description guidelines mention including realistic file count

**Dependencies:** None

**Notes:** This task is independent of Task 1 — the skill file and the Go code can be updated in parallel.

### Task 3: Add `EstimatedFiles` to IterationLog and IterationResult, wire in runner

**Files:**
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/process.go`
- Test: `internal/logger/logger_test.go`, `internal/runner/runner_test.go` or `internal/runner/process_test.go`

**What to Do:**
Add `EstimatedFiles int` with `json:"estimated_files,omitempty"` to `IterationLog` (logger.go, after line 41). Add `EstimatedFiles int` to `IterationResult` (runtypes/types.go, after line 73). In `setupBeadContext` (process.go, line 55-69) or in `processBead` (runner.go, line 823), parse the `estimated-files:N` label from `b.Labels` using a helper like `parseEstimatedFilesLabel(labels []string) int` that scans for the `estimated-files:` prefix, parses the integer, and returns 0 on missing/malformed values. Store the result in `bc.Result.EstimatedFiles`. In `writeIterationLog` (runner.go, line 793-819), add `EstimatedFiles: result.EstimatedFiles` to the `IterationLog` struct literal.

**Acceptance Criteria:**
- `IterationLog` includes `EstimatedFiles int` with `json:"estimated_files,omitempty"` tag
- `IterationResult` includes `EstimatedFiles int` field
- Runner parses `estimated-files:N` label from bead labels and populates `IterationResult.EstimatedFiles`
- `writeIterationLog` writes `EstimatedFiles` to the JSONL output

**Dependencies:** Task 1 (labels must be applied during decompose for the runner to read them)

**Notes:** The `parseEstimatedFilesLabel` helper should use `strings.TrimPrefix` + `strconv.Atoi` and return 0 on any error. This is a simple utility — no need for a separate package. Place it in `process.go` or `runner.go` alongside `setupBeadContext`.

---

## Notes

- **No changes to `SelectModel()` or `SelectTier()`** — the existing `complexity:high -> opus` label routing handles everything.
- **Backward compatible** — `omitempty` on all new fields means old data parses cleanly and new fields are absent from old logs.
- **Experiment-ready** — the `estimated-files:N` label enables retro analysis of prediction accuracy before committing to the threshold value of 5.
- **The `newCreatedBeadFromDef` function** (decompose.go:182) currently hardcodes only `spec:<name>` in the labels. Task 1 should update it to use the same label set built in the creation loop, or accept labels as a parameter, so review mode (`--review`) shows accurate labels including `complexity:high` and `estimated-files:N`.
