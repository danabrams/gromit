# Spec 0004g — Auto-Generate Remediation Spec on Accept

## spec_id
0004g-remediation-spec-on-accept

## Depends on
0004b-review-outcome-recording-and-cli

## Vision

When a reviewer accepts a run with non-blocking review findings, those findings disappear. The distill flow only fires on rework outcomes. This means legitimate cleanup items — test gaps, code style issues, minor refactors — are lost unless the reviewer manually creates follow-up work. In practice they don't, and the debt accumulates.

Specs are the unit of work in gromit-next. A remediation spec is just a regular spec that flows through the normal exec/review pipeline. Auto-generating one on accept closes the tracking gap without inventing a new concept.

## Summary

When `review record --outcome accepted` is called and the run has non-blocking review findings, automatically generate a `<spec-id>-remediation.md` spec file in the specs directory. The remediation spec contains the findings grouped by category with file references, acceptance criteria derived from the `suggested_fix` fields, and validation commands. The spec is immediately visible in `spec list` as `ready` and can be executed through the normal pipeline.

## Goals

### Primary
- Auto-generate a remediation spec from non-blocking review findings when a run is accepted
- Remediation spec follows the standard spec format and is executable by gromit-next
- Print the remediation spec path to stdout so the reviewer knows it was created

### Secondary
- Include enough context in the remediation spec (file paths, line numbers, descriptions) that the executor doesn't need to re-read the original review

## Non-goals
- Generating remediation specs for rework outcomes (those go through distill)
- Filtering or prioritizing findings — all non-blocking findings are included
- Making remediation specs block the original spec's completion
- Deduplicating findings across categories — duplicate findings across `architecture_drift` and `spec_alignment` may produce redundant acceptance criteria; the executor should treat each criterion as idempotent

## Architecture

### Integration point

The remediation logic lives **outside** `reviewRecord`, called from the cobra `RunE` closure in `newReviewRecordCmd()` after `reviewRecord` returns successfully. This avoids changing `reviewRecord`'s signature or responsibility. The cobra closure has access to the command's flags and stdout.

Flow in the `RunE` closure:
```go
// Read flags
specsDir, _ := cmd.Flags().GetString("specs-dir")
project, _ := cmd.Flags().GetString("project")

// Resolve specsDir from project config if not explicitly provided
// (same pattern as exec_complete.go lines 38-56)
if specsDir == "" && project != "" {
    resolver := workspace.NewEnvResolver()
    root, _ := resolver.Resolve()
    if root != "" {
        projectDir, _ := ResolveProjectConfigPath(root, project)
        if cfg, err := LoadProjectConfig(projectDir); err == nil {
            specsDir = cfg.SpecsDir
            if specsDir == "" && cfg.RepoPath != "" {
                specsDir = filepath.Join(cfg.RepoPath, "docs", "specs")
            }
        }
    }
}

// Existing: record the outcome
if err := reviewRecord(runID, storeDir, outcome, summary, overrideReason); err != nil {
    return err
}

// NEW: generate remediation spec if accepted with findings
if outcome == "accepted" {
    if specsDir == "" {
        fmt.Fprintf(cmd.ErrOrStderr(), "Warning: no --specs-dir or --project provided, skipping remediation spec generation\n")
    } else {
        remediationPath, err := maybeGenerateRemediationSpec(runID, storeDir, specsDir)
        if err != nil {
            fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not generate remediation spec: %v\n", err)
        } else if remediationPath != "" {
            fmt.Fprintf(cmd.OutOrStdout(), "Created remediation spec: %s\n", remediationPath)
        }
    }
}
```

New flags on `review record`:
- `--specs-dir` — override specs directory
- `--project` — project name for resolving specs-dir from project config. Resolution follows the same pattern as `exec complete`: calls `workspace.NewEnvResolver().Resolve()` to find the workspace root, then `ResolveProjectConfigPath(root, project)` and `LoadProjectConfig(projectDir)` to get `cfg.SpecsDir` (falling back to `cfg.RepoPath + "/docs/specs"`)

If neither flag is provided, skip remediation generation with a warning to stderr. The accept itself still succeeds.

### New file: `cmd/gromit-next/review_remediation.go`

```go
// maybeGenerateRemediationSpec reads review.json from the run's evidence,
// extracts non-blocking findings, and generates a remediation spec if any exist.
// Returns the written file path, or empty string if no findings / no spec generated.
func maybeGenerateRemediationSpec(runID, storeDir, specsDir string) (string, error)

// generateRemediationSpec produces a remediation spec markdown string
// from non-blocking review findings. Pure function — no I/O.
func generateRemediationSpec(input RemediationInput) string

// RemediationInput holds the data needed to generate a remediation spec.
type RemediationInput struct {
    SpecID   string
    RunID    string
    Findings map[string][]RemediationFinding // category -> findings
}

// RemediationFinding contains the fields needed for remediation generation.
// This is a subset of the on-disk review.json finding structure.
type RemediationFinding struct {
    Severity     string `json:"severity"`
    File         string `json:"file"`
    Line         int    `json:"line"`
    Description  string `json:"description"`
    SuggestedFix string `json:"suggested_fix"`
}
```

### Obtaining the spec ID

`maybeGenerateRemediationSpec` loads the run from the store via `runstore.NewStore(storeDir).Get(runID)` to retrieve `RunState.SpecID`. This is used for the remediation spec's filename (`<spec-id>-remediation.md`), `spec_id` field, and `Depends on` field.

### Reading review.json

`maybeGenerateRemediationSpec` reads `review.json` from the run's evidence directory (`store.RunEvidenceDir(runID)`). The file is a JSON object where:
- Top-level keys that map to arrays are finding categories (e.g., `architecture_drift`, `spec_alignment`)
- Top-level keys that map to non-array values are metadata (e.g., `diff_unavailable: bool`) and are skipped
- Each finding object has fields including: `facet`, `severity`, `file`, `line`, `description`, `suggested_fix`, `cycle`, `disposition`

**Note:** The existing `reviewpacket.ReviewFindingJSON` type (in `internal/next/reviewpacket/types.go`) only has a `Message` field and cannot be reused here. The `RemediationFinding` struct defined above extracts the subset of fields needed for remediation generation using its own JSON unmarshalling.

If `review.json` is missing or cannot be parsed, return an error (the caller prints a warning and continues).

### Finding filtering

Include findings where:
- The top-level key maps to an array (skip non-array values like `diff_unavailable`)
- `severity` is `warning`, `info`, or `suggestion` (explicitly exclude `error` — error-severity findings indicate blocking issues that should have been resolved during the run itself, not deferred to remediation)
- `description` is non-empty

### Remediation spec template

The `generateRemediationSpec` function produces a spec with:
- **spec_id**: `<parent-spec-id>-remediation`
- **Depends on**: `<parent-spec-id>`
- **Summary**: "Cleanup items from the {spec-id} review: {count} findings across {categories}."
- **Goals**: Fix all non-blocking review findings
- **Non-goals**: No behavior changes
- **Architecture**: One subsection per unique file referenced in findings, listing the changes needed (from `suggested_fix` if present, otherwise from `description`)
- **Acceptance Criteria**: One criterion per finding — text is `suggested_fix` if non-empty, otherwise `description`. Prefixed with the file path for context.
- **Validation**: Default to `go test ./... -count=1` and `go vet ./...`

### Spec ID convention

The remediation spec ID is `<parent-spec-id>-remediation`. If a file already exists at that path, overwrite it (the latest accept has the most current findings).

## Acceptance Criteria

1. When `review record --outcome accepted` is called and `review.json` contains non-blocking findings, a remediation spec is written to `<specs-dir>/<spec-id>-remediation.md`
2. When `review record --outcome accepted` is called and `review.json` has no non-blocking findings (or only non-array metadata), no remediation spec is generated
3. When the outcome is `rework_implementation_gap` or `rework_vision_change`, no remediation spec is generated regardless of findings
4. The remediation spec follows the standard gromit-next spec format with sections: spec_id, Depends on, Summary, Goals, Non-goals, Architecture, Acceptance Criteria, Validation
5. Each non-blocking finding produces one acceptance criterion in the remediation spec, using `suggested_fix` if non-empty, otherwise `description`
6. The remediation spec's `Depends on` field references the parent spec ID
7. The path to the generated remediation spec is printed to stdout
8. If the specs directory cannot be resolved (no `--specs-dir` and no `--project` flag), the accept still succeeds and a warning is printed to stderr
9. If a remediation spec already exists at the target path, it is overwritten
10. The `review record` command accepts `--specs-dir` and `--project` flags for resolving the specs directory
11. If `review.json` is missing or cannot be parsed, the accept still succeeds and a warning is printed to stderr
12. Findings with empty `description` are skipped
13. `generateRemediationSpec` is a pure function — it takes `RemediationInput` and returns a string, with no I/O
14. All existing `review record` tests continue to pass

## Scenarios

### Scenario: accepted run with findings generates remediation spec
**Given:** A terminal run for spec `0004f-contract-specificity-validation` with `review.json` containing 9 `architecture_drift` findings (severity: warning/info/suggestion) and 8 `spec_alignment` findings
**When:** `review record --run <run-id> --outcome accepted --summary "All proven" --specs-dir docs/specs` is called
**Then:** `docs/specs/0004f-contract-specificity-validation-remediation.md` is created with 17 acceptance criteria (one per finding), spec_id is `0004f-contract-specificity-validation-remediation`, `Depends on` is `0004f-contract-specificity-validation`, and stdout contains the file path

### Scenario: accepted run with no findings skips remediation
**Given:** A terminal run for spec `clean-feature` with `review.json` containing only `{"diff_unavailable": false}` (no finding arrays)
**When:** `review record --run <run-id> --outcome accepted --summary "Clean run" --specs-dir docs/specs` is called
**Then:** No remediation spec file is written, stdout shows only the accept confirmation

### Scenario: rework outcome does not generate remediation
**Given:** A terminal run with non-blocking findings in `review.json`
**When:** `review record --run <run-id> --outcome rework_implementation_gap --summary "Needs fixes" --specs-dir docs/specs` is called
**Then:** No remediation spec is generated

### Scenario: missing specs-dir warns but does not fail
**Given:** A terminal run with findings, no `--specs-dir` or `--project` flag provided
**When:** `review record --run <run-id> --outcome accepted --summary "Done"` is called
**Then:** The accept succeeds, `review-outcome.json` is written, and stderr contains a warning about skipping remediation spec generation

### Scenario: finding with empty suggested_fix uses description
**Given:** `review.json` has one finding with `"suggested_fix": ""` and `"description": "regexp compiled inside function body"`
**When:** The remediation spec is generated
**Then:** The acceptance criterion for that finding uses the `description` text, not an empty string

### Scenario: review.json missing does not fail the accept
**Given:** A terminal run whose evidence directory has no `review.json` file
**When:** `review record --run <run-id> --outcome accepted --summary "Done" --specs-dir docs/specs` is called
**Then:** The accept succeeds, `review-outcome.json` is written, stderr warns about missing review.json, no remediation spec is generated

### Scenario: malformed review.json does not fail the accept
**Given:** A terminal run whose evidence directory has a `review.json` containing invalid JSON (e.g., `{bad json`)
**When:** `review record --run <run-id> --outcome accepted --summary "Done" --specs-dir docs/specs` is called
**Then:** The accept succeeds, `review-outcome.json` is written, stderr warns about unparseable review.json, no remediation spec is generated

### Scenario: existing remediation spec is overwritten
**Given:** `docs/specs/my-spec-remediation.md` already exists with 3 acceptance criteria from a prior run
**When:** A new accept generates a remediation spec with 5 acceptance criteria for the same spec
**Then:** The file is overwritten with the new 5-criteria spec

## Validation

### Automatic
- `go test ./cmd/gromit-next/... -count=1 -timeout 120s`
- `go vet ./...`
