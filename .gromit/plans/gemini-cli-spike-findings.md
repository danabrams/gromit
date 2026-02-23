---
id: gemini-cli-spike-findings
source_spec: gemini-cli-spike
created: 2026-02-23
---

# Gemini CLI Spike Findings

Date: 2026-02-23  
Environment: `/home/dabrams/gromit` (headless shell)  
Live invocation path used: `npm start --prefix /home/dabrams/gemini-cli -- --approval-mode yolo ...`

## Executive Summary

- Live Gemini non-interactive invocations were successfully executed and captured on 2026-02-23.
- `stdin` and inline `-p` prompt delivery both worked reliably for one-shot output.
- `stream-json` and `json` outputs were captured from real runs and now back fixture candidates.
- Invalid model handling is now observed: non-zero exit with `ModelNotFoundError` and `404` in stderr.
- Remaining gap: auth/rate-limit/transport-disconnect signatures are still not captured from live failures.

## Evidence Index

- Command ledger: `.gromit/plans/fixtures/gemini/commands.log`
- Prompt delivery artifacts: `.gromit/plans/fixtures/gemini/prompt-delivery/*`
- Stream fixture candidate (live): `.gromit/plans/fixtures/gemini/stream-json-success.jsonl`
- JSON fixture candidate (live): `.gromit/plans/fixtures/gemini/json-success.json`
- Model artifacts: `.gromit/plans/fixtures/gemini/models/*`
- Error artifacts: `.gromit/plans/fixtures/gemini/errors/*`
- Permissions notes: `.gromit/plans/fixtures/gemini/permissions/permissions-notes.md`
- Workdir notes: `.gromit/plans/fixtures/gemini/workdir/workdir-notes.md`

## 1. Prompt Delivery Modes

### Commands Run

- Inline: `... -p "Respond with exactly: READY" --output-format json`
- Stdin: `printf 'Return exactly PIPE_OK\n' | ... --output-format json`
- Prompt-file ref: `... -p "@.gromit/plans/fixtures/gemini/prompt-delivery/prompt-file-input.txt" --output-format json`

### Observed Output

- Inline and stdin succeeded (exit `0`) and returned expected responses (`READY`, `PIPE_OK`).
- `@file` mode exited `0` but showed unstable behavior in this setup (large multi-turn/tool-heavy run and empty response in captured output).

### Implementation Implications

- **Recommended prompt delivery mode:** stdin for provider default path.
- Keep inline `-p` as a fallback for short diagnostic calls.
- Treat `-p @file` as non-default/provisional due observed instability and cost risk in this run.

## 2. Streaming JSON (`--output-format stream-json`)

### Commands Run

- `... -p "Return exactly STREAM_OK" --output-format stream-json`

### Observed Output

Live JSONL events captured with this shape:
- `{"type":"init", ...}`
- `{"type":"message", "role":"user", ...}`
- `{"type":"message", "role":"assistant", "content":"STREAM_OK", "delta":true}`
- `{"type":"result", "status":"success", "stats":{...}}`

### Implementation Implications

- Parse line-by-line JSONL using `type` as primary discriminator.
- Assistant text can be read from `message` events where `role=assistant`.
- Final usage/timing is available in `result.stats` (`input_tokens`, `output_tokens`, `total_tokens`, `duration_ms`).

## 3. JSON Output (`--output-format json`)

### Commands Run

- `... -p "Respond with exactly: READY" --output-format json`

### Observed Output

Live object shape captured:
- `session_id`
- `response`
- `stats.models.<model>.api.{totalRequests,totalErrors,totalLatencyMs}`
- `stats.models.<model>.tokens.{input,prompt,candidates,total,cached,thoughts,tool}`
- `stats.tools.*`
- `stats.files.*`

### Implementation Implications

- Parse output text from `response`.
- Parse token metrics from nested `stats.models.*.tokens`.
- No direct `cost` field observed in this output mode.

## 4. Token and Cost Handling

### Commands Run

- JSON and stream-json successful runs above.

### Observed Output

- Token counts are present in both modes.
- Direct monetary cost field was **not** observed in captured live outputs.

### Implementation Implications

- Prefer provider token fields.
- Compute cost from configured pricing when needed.
- If pricing config is absent, keep `CostUSD=0` without hard-failing.

## 5. Model Selection

### Commands Run

- Valid: `... --model gemini-2.5-flash -p "Respond with exactly VALID_MODEL_OK" --output-format json`
- Invalid: `... --model invalid-model-does-not-exist -p "ping" --output-format json`

### Observed Output

- Valid model succeeded (exit `0`) and returned `VALID_MODEL_OK`.
- Invalid model failed (exit `1`) with stderr including `ModelNotFoundError` and `code: 404` / `NOT_FOUND` details.

### Implementation Implications

- Model mapping should stay config-driven.
- Add explicit model-invalid classification path keyed on `ModelNotFoundError` / `NOT_FOUND` signatures.

## 6. Exit Codes

### Commands Run

- Real Gemini runs (success + invalid model)
- Prior shell trigger captures for generic `1`, `42`, `53`

### Observed Output

- Success path: `0`
- Invalid model path: `1`
- Gemini-native semantics for `42` and `53` still not directly observed in live Gemini failures.

### Implementation Implications

- Preserve exact process exit code in result metadata/logging.
- Do not hardcode Gemini-specific meaning for `42`/`53` yet.

## 7. Error Classification Patterns

### Commands Run

- Invalid-model run (live)
- Historical missing-binary runs

### Observed Output

- Setup failure pattern remains valid in some environments: `command not found: gemini`.
- Live invalid-model pattern captured: `ModelNotFoundError` + `404`/`NOT_FOUND` details.

### Implementation Implications

- Immediate categories to support:
  - setup/binary-missing (`command not found: gemini`)
  - model-invalid (`ModelNotFoundError`, `NOT_FOUND`)
  - fallback `other`
- Auth/rate-limit/transport categories remain conservative placeholders until captured.

## 8. Permission Model

### Commands Run

- Prior shell permission checks (`/root` write, no-exec dir)

### Observed Output

- OS-level permission denials are captured as expected in stderr fixtures.
- Gemini-specific approval-flag behavior remains lightly sampled.

### Implementation Implications

- Surface permission-denied stderr verbatim and treat as non-retryable by default.

## 9. Working Directory (CWD)

### Commands Run

- Prior `pwd`/`cd /tmp`/relative path checks

### Observed Output

- Relative paths are CWD-dependent; absolute paths stay reliable.

### Implementation Implications

- Set working directory explicitly for provider runs.
- Use absolute paths for prompt/artifact files when crossing directories.

## Provider-Oriented Conclusions

- `GeminiProvider` is implementable now with stdin-first delivery, live-backed json/stream parsers, and config-driven model mapping.
- Token extraction can be implemented now from observed `stats` fields.
- Cost should be config-derived unless future Gemini outputs expose a stable direct cost field.
- Classifier can ship now with setup + model-invalid + fallback buckets; auth/rate/transport should remain conservative until captured.

## Limitations and Follow-up

- Still missing live captures for: auth failures, quota/rate-limit failures, transport disconnects, and Gemini-native 42/53 semantics.
- `-p @file` behavior in this run was unstable and should not be the default provider path.
- Existing follow-up bead for gap closure remains relevant: `gromit-sb4mt`.

## Version/Auth Caveats

- Findings are tied to the 2026-02-23 execution context and Gemini CLI build used via local source invocation.
- Non-triggerable cases in this run: auth-denied, quota/rate-limit, and transport-disconnect failures.
- Re-run against the exact production-installed Gemini binary and auth mode is still recommended before declaring full parity.

## Scope Confirmation

- This artifact is research-only documentation. No production Go provider code was changed in this task.
