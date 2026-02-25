# provenance: deterministic schema notes distilled from Gemini spike artifacts under this fixture directory.
# refresh: update only when fixture schemas change; keep section headings stable for contract tests.
# Gemini Schema Notes

## Token and Cost Observations

- Command evidence is logged in `commands.log` with `# token-cost` tags.
- `gemini` is not installed in this environment, so runtime JSON fields could not be directly observed.
- Parser planning note: probe for `input_tokens`, `output_tokens`, and `cost` fields when Gemini JSON output is available.

## Model Observations

- Valid-model attempt artifact: `models/valid-model.stderr.txt` (`command not found` in this environment).
- Invalid-model attempt artifact: `models/invalid-model.stderr.txt` (`command not found` in this environment).
- Matching stdout captures are retained for completeness under `models/valid-model.stdout.txt` and `models/invalid-model.stdout.txt`.

## Prompt Mode Comparison

- Inline `-p` (short): see `prompt-delivery/inline-small.stderr.txt` and `prompt-delivery/inline-small.exit.txt`.
- Inline `-p` (large): see `prompt-delivery/inline-large.stderr.txt` and `prompt-delivery/inline-large.exit.txt`.
- Stdin pipe: see `prompt-delivery/stdin-pipe.stderr.txt` and `prompt-delivery/stdin-pipe.exit.txt`.
- `@file` via `-p "@path"`: see `prompt-delivery/prompt-file-ref.stderr.txt`, `prompt-delivery/prompt-file-ref.exit.txt`, and `prompt-delivery/prompt-file-input.txt`.
- Inline -p, stdin pipe, and @file modes were all exercised as command-form variants.

## Stream-JSON Schema

- Fixture file: `stream-json-success.jsonl`
- Event/object keys observed in fixture: `type`, `role`, `content`, `delta`, `finish_reason`, `status`, `stats.input_tokens`, `stats.output_tokens`, `stats.total_tokens`, `stats.duration_ms`, `stats.tool_calls`.
- Representative event types in fixture: `init`, `message`, `result`.
- Parser planning note: tokenize assistant/user contributions from `message` events and record usage totals from the final `result.stats` object (Gemini stream-json fixtures do not expose `usage.*` directly).

## JSON Schema

- Fixture file: `json-success.json`
- Object keys observed in fixture: `output`, `usage.input_tokens`, `usage.output_tokens`, `cost.total`, `model`, `finish_reason`, `stats`.
- Stats structure:
- `stats.models` contains a mapping by model id.
- `stats.models.<model>.tokens` exposes `input`, `prompt`, `candidates`, `total`, `cached`, `thoughts`, `tool`.
- `models/<model>.tokens` (alias for `stats.models.<model>.tokens`) carries the observed counts for comparison to provider stats.
- `stats.tools` surfaces aggregated call/timing totals for any observed tool activity.
