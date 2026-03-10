# Spec-Level Review Prompt Template (v2)

You are executing a **spec-level review** of the bead diff described by the current task. Treat the cumulative diff as the authoritative body of work that will ship, and answer every question through the specificity of this bead’s title, description, and files. Do not invent behavior that is not represented in the diff.

Every finding must tie to the **cumulative diff** (all files together) rather than a single excerpt, so cross-file effects, instrumentation changes, and contract gaps are surfaced before downstream merges.

## Review Mission

For each dimension below you should consider both the new code and how it integrates with the existing surface area touched by the diff. Highlight what the change does, call out any missing contributors, and surface the most critical risks to launch.

## Review Dimensions

### Correctness
Does the diff do what the spec promises? Validate invariants, control-flow guards, branching coverage, concurrency safety, and data integrity (including allows/denies for identifiers and filesystem paths). Treat off-by-one loops, missing nil checks, and unchecked context cancelations as correctness gaps.

### Security
Look for new credential exposure, authorization bypasses, injection lints, or dependency misuse. Ask whether the diff introduces sensitive data in logs, allows attacker-provided paths past allowlists, or fails to validate input before executing privileged subprocesses.

### Error handling
Confirm that every failure path logs actionable diagnostics, wraps root causes, cleans up resources, and drives the shared metrics persist epilogue (including sentinel attribution rows). Check retry logic, cancel propagation, and whether goroutines that write to subprocess stdin surface write errors before closing.

### Test coverage
Verify that the diff adds the necessary tests for its new behavior (unit, integration, or telemetry). Identify missing branches, unstubbed dependencies, or architecture contracts that lack parity tests (e.g., telemetry emitters, persist epilogue guards, manifest-derived benchmarks, reducer contracts for attribution). If no tests exist, explain what kind of test would cover the risk.

### Code quality
Look for duplication, unclear naming, dead branches, or overly clever code that obscures intent. Confirm that normalization helpers (`NormalizeNilFields` or `normalizeNilFields`) are used when cross-package types change, and that synchronous flows do not rely on global state.

### Architectural fit
Validate that the change obeys the architecture rules: context must flow end-to-end, allowlist-validated identifiers only, single schema writers for externally consumed artifacts, manifest-only benchmark execution, single telemetry persist epilogue for every exit, and reducer/telemetry contracts for attribution or stream-event paths. Call out violations or confirm compatibility with process contracts (telemetry gating, beads for usage, decomposition rules, etc.).

## Scope Classification Rules

When populating `scope` for a finding, describe the subsystem touched by the issue, not just a file name. Use connective phrases such as:

- `prompt rendering` for changes centered on prompt/fragment files or rendering helpers.
- `cmd/gromit/review_spec_validation` for CLI-level validation logic.
- `runner/context-guard` when the issue spans runner control flow or context propagation.
- `telemetry/persist-epilogue` when calling out usage attribution or metrics.

If an issue crosses components, choose a short descriptor that reflects the dominant effect (e.g., `system telemetry` for architecture-level data-quality gaps). Keep `scope` to 3–4 words, use repo-relative paths when more precision is required, and avoid generic labels like “misc”.

## Severity Examples

Use severity to describe how urgently the issue must be fixed:

- **Critical** – Blocks launch or data quality: e.g., a missing metrics persist epilogue on every exit, an unknown-attribution row reaching SPC, or telemetry failing to mark a data_quality_invalid run.
- **High** – Violates architecture contracts or security rules: e.g., a subprocess launched with `runCmd` after accepting user input, missing allowlist validation, or a reducer contract without parity tests.
- **Medium** – Behavior drains confidence but may be mitigated: e.g., missing validation for non-critical branches, insufficient tests for a complex new helper, or missing nil normalization after a type change.
- **Low** – Style, readability, or duplication issues that do not block correctness but should be cleaned up before merge.

## Output Format

Return a single JSON object describing your findings. Do not wrap it in markdown or additional text. Cite **every** blocking issue as `"verdict": "issue"` with the proper `severity`. If no issues exist, still emit at least one `"pass"` finding explaining why the spec is ready.

Each finding must explain why it matters, mention the architecture or process guard it affects, and list touched files in `affected_files` (repository-relative paths).

## Output Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["findings", "summary"],
  "properties": {
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["verdict", "severity", "category", "scope", "description", "affected_files"],
        "properties": {
          "verdict": {
            "type": "string",
            "enum": ["issue", "pass"]
          },
          "severity": {
            "type": "string",
            "enum": ["critical", "high", "medium", "low"]
          },
          "category": {
            "type": "string",
            "enum": ["correctness", "security", "error_handling", "test_coverage", "code_quality", "architecture"]
          },
          "scope": {
            "type": "string",
            "minLength": 1
          },
          "description": {
            "type": "string",
            "minLength": 1
          },
          "affected_files": {
            "type": "array",
            "items": {
              "type": "string"
            },
            "minItems": 1
          }
        }
      }
    },
    "summary": {
      "type": "string",
      "minLength": 1
    }
  }
}
```

Treat this schema as normative: the final JSON must conform exactly, and every issue should map to one of the six `category` values above. The `summary` should be a crisp two-sentence overview of the diff’s health and any next steps.
