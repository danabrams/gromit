---
id: gemini-cli-spike-findings
source_spec: gemini-cli-spike
created: 2026-02-23
---

# Gemini CLI Spike Findings

Date: 2026-02-23  
Environment: `/home/dabrams/gromit` (headless shell)  
Primary caveat: `gemini` binary was not available for live provider-path execution in this environment (`command not found`).

## Executive Summary

- The evidence package is complete for command ledgering, fixture schema shape, permission/CWD shell behavior, and exit-code handling patterns.
- Live Gemini runtime behavior (auth failures, rate limits, transport disconnects, real stream/json payloads from the binary) remains partially unverified in this environment.
- Recommended prompt delivery mode for future `GeminiProvider`: stdin piping as default operational path, with `-p` retained as fallback for short prompts.

## Evidence Index

- Command ledger: `.gromit/plans/fixtures/gemini/commands.log`
- Prompt delivery artifacts: `.gromit/plans/fixtures/gemini/prompt-delivery/*`
- Stream fixture candidate: `.gromit/plans/fixtures/gemini/stream-json-success.jsonl`
- JSON fixture candidate: `.gromit/plans/fixtures/gemini/json-success.json`
- Model artifacts: `.gromit/plans/fixtures/gemini/models/*`
- Error artifacts: `.gromit/plans/fixtures/gemini/errors/*`
- Permissions notes: `.gromit/plans/fixtures/gemini/permissions/permissions-notes.md`
- Workdir notes: `.gromit/plans/fixtures/gemini/workdir/workdir-notes.md`

## 1. Prompt Delivery Modes

### Commands Run

- `gemini -p "Respond with the single word: ready"`
- `gemini -p "<large prompt omitted>"`
- `printf 'Return the token PIPE_OK.\n' | gemini`
- `gemini -p "@.gromit/plans/fixtures/gemini/prompt-delivery/prompt-file-input.txt"`

### Observed Output

- All four commands are logged in `commands.log` under `# prompt-delivery` and exited with code `127` in this environment.
- Corresponding stderr captures (`inline-small.stderr.txt`, `inline-large.stderr.txt`, `stdin-pipe.stderr.txt`, `prompt-file-ref.stderr.txt`) show `command not found: gemini`.
- Functional behavior differences between modes cannot be directly observed without a live binary.

### Implementation Implications

- **Recommended prompt delivery mode:** stdin piping for `GeminiProvider` primary path, because it avoids shell argument-size risks for large prompts and aligns with cross-provider robustness patterns.
- Keep `-p` compatibility for short prompts and diagnostics.
- Treat `@file` as explicitly unverified until live CLI rerun confirms expansion semantics.

## 2. Streaming JSON (`--output-format stream-json`)

### Commands Run

- Fixture generation command logged: `cat > .gromit/plans/fixtures/gemini/stream-json-success.jsonl`.

### Observed Output

- Fixture candidate `.gromit/plans/fixtures/gemini/stream-json-success.jsonl` contains JSONL events:
  - `message_start`
  - `content_delta`
  - `message_end`
- Usage/cost keys present in terminal event: `usage.input_tokens`, `usage.output_tokens`, `cost.total`.

### Implementation Implications

- `GeminiProvider.StreamRun` should parse per-line JSON and accumulate assistant text from `content_delta.delta.text`.
- Finalization logic should read usage/cost from terminating event when present.
- Because fixture is synthesized rather than captured from a live binary session, parser must remain tolerant to event-name variance.

## 3. JSON Output (`--output-format json`)

### Commands Run

- Fixture generation command logged: `cat > .gromit/plans/fixtures/gemini/json-success.json`.

### Observed Output

- Fixture candidate `.gromit/plans/fixtures/gemini/json-success.json` contains:
  - `output`
  - `usage.input_tokens`
  - `usage.output_tokens`
  - `cost.total`
  - `model`
  - `finish_reason`

### Implementation Implications

- `GeminiProvider.Run` should parse output text, usage tokens, and optional cost from JSON payload.
- Non-fatal parsing strategy should treat missing `cost` as allowed and continue with token-only accounting.

## 4. Token and Cost Handling

### Commands Run

- `gemini --output-format json --model gemini-2.0-flash -p token-check` (logged under `# token-cost`, exit `127`).

### Observed Output

- Live Gemini token/cost reporting was not observable due to missing binary.
- Schema fixtures and `schema-notes.md` encode expected keys (`input_tokens`, `output_tokens`, `cost`).

### Implementation Implications

- Token handling guidance:
  - Prefer provider-reported `input_tokens`, `output_tokens`, and `cached_input_tokens` when available.
  - If absent, leave counts at zero rather than inferring from text length.
- Cost handling guidance:
  - Prefer provider-reported total cost if present.
  - Otherwise compute from configured price table and parsed token counts.
  - If both unavailable, return `CostUSD=0` with no hard failure.

## 5. Model Selection

### Commands Run

- `gemini --model gemini-2.0-flash -p ping`
- `gemini --model invalid-model-does-not-exist -p ping`

### Observed Output

- Both attempts exited `127` with `command not found: gemini` (`models/valid-model.stderr.txt`, `models/invalid-model.stderr.txt`).
- Invalid-model-specific runtime signature remains unobserved.

### Implementation Implications

- Keep model mapping fully config-driven for `GeminiProvider`.
- Classifier should reserve a model-invalid category, but it cannot yet rely on stable Gemini-native stderr text from this spike.

## 6. Exit Codes

### Commands Run

- Shell trigger attempts from `exit-codes-notes.md`:
  - `sh -c 'echo ok'` -> `0`
  - `sh -c 'echo intentional trigger for exit code 1 >&2; exit 1'` -> `1`
  - `sh -c 'echo intentional trigger for exit code 42 >&2; exit 42'` -> `42`
  - `sh -c 'echo intentional trigger for exit code 53 >&2; exit 53'` -> `53`

### Observed Output

- Exit stderr fixtures exist and are non-empty for `1`, `42`, and `53` in `.gromit/plans/fixtures/gemini/errors/`.
- These observations validate handling paths for process exit status plumbing, not Gemini-specific semantic meanings.

### Implementation Implications

- `GeminiProvider` should preserve exact process exit codes in result metadata/logging.
- Avoid hardcoding Gemini semantic mapping for `42`/`53` until live Gemini triggers are collected.

## 7. Error Classification Patterns

### Commands Run

- Captured error fixture: `.gromit/plans/fixtures/gemini/errors/command-missing.stderr.txt`
- Shell-triggered stderr fixtures for generic failure categories: `exit-1.stderr.txt`, `exit-42.stderr.txt`, `exit-53.stderr.txt`

### Observed Output

- Reliable observed pattern in this environment: `command not found: gemini`.
- Auth, rate-limit, and transport-disconnect Gemini-native patterns are non-triggerable in this run.

### Implementation Implications

- Initial classifier guidance:
  - `command not found: gemini` -> treat as environment/setup failure.
  - Retain placeholder matching buckets for `auth`, `rate_limited`, and `transport_disconnect` with conservative fallback to `other`.
- Version/auth caveats: classifier strings for live Gemini errors must be finalized only after rerun with installed/authenticated CLI.

## 8. Permission Model

### Commands Run

- `touch /root/gromit-permissions-check`
- `d=$(mktemp -d); chmod 000 "$d"; ls "$d"`

### Observed Output

- Permission-denied errors captured in:
  - `.gromit/plans/fixtures/gemini/permissions/root-write.stderr.txt`
  - `.gromit/plans/fixtures/gemini/permissions/no-exec-dir.stderr.txt`
- No Gemini-specific interactive approval prompt behavior was observable in headless shell execution.

### Implementation Implications

- Permission model conclusion: non-interactive provider runs must assume OS/container policy enforcement first; Gemini approval UX flags remain to be validated live.
- `GeminiProvider` should surface permission-denied stderr verbatim and categorize as non-retryable unless clear transient signal exists.

## 9. Working Directory (CWD)

### Commands Run

- `pwd`
- `cd /tmp && pwd && ls /home/dabrams/gromit/.gromit/plans/fixtures/gemini/preflight.md`
- `d=$(mktemp -d); cd "$d" && pwd && ls preflight.md`

### Observed Output

- Initial CWD: `/home/dabrams/gromit`.
- After `cd /tmp`, absolute project path remained readable.
- Relative lookup failed from unrelated directory (`No such file or directory`).

### Implementation Implications

- CWD guidance for `GeminiProvider`:
  - Set command working directory explicitly to the target workspace/worktree.
  - Use absolute paths when referencing generated prompt files or artifacts outside current CWD.
  - Do not assume relative path stability across subprocess launches.

## Provider-Oriented Conclusions

- `GeminiProvider` should launch with explicit working directory, stdin-first prompt delivery, and tolerant schema parsing.
- Token/cost extraction should prefer provider fields and gracefully degrade to config-based or zero-cost fallback.
- Error classification must ship with setup-failure detection now, and keep auth/rate-limit/transport buckets behind conservative matching until live capture updates.
- Fixture references above are sufficient to begin parser scaffolding and tests, but a live binary rerun is required before declaring production-grade parity.

## Limitations and Follow-up

- Non-triggerable conditions in this spike: Gemini-native auth failures, rate limits, transport disconnects, invalid-model runtime payloads, and actual permission flags/approval behavior.
- Version/auth caveats:
  - Findings are bound to this run date and environment where `gemini` command execution was unavailable.
  - Re-run against installed Gemini CLI and authenticated session is required to lock final schemas and classifiers.
- Existing follow-up bead already captures this gap: `gromit-sb4mt`.

## Scope Confirmation

- This artifact is research-only documentation. No production Go provider code was changed in this task.
