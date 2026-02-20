---
id: gemini-cli-spike
source_ideas: []
created: 2026-02-20
---

# Gemini CLI Spike

## Specification

Verify the Gemini CLI invocation contract before building a full `GeminiProvider` implementation. This spike empirically tests prompt delivery, streaming output format, token/cost reporting, error patterns, tool permissions, and working directory behavior.

The Gemini CLI (`gemini`) is Google's official terminal agent. It supports non-interactive execution via `-p`, model selection via `-m`, and structured output via `--output-format json|stream-json`. The spike must confirm these documented behaviors match actual behavior and capture the exact schemas Gromit needs to parse.

### Areas to Verify

1. **Prompt delivery modes**
   - Inline: `gemini -m <model> -p "short prompt"` — confirm basic invocation works.
   - Large prompt via -p: test with a 50KB+ prompt string to find ARG_MAX limits.
   - Stdin pipe: `cat prompt.md | gemini -m <model>` — confirm non-interactive stdin works.
   - File inclusion: `gemini -m <model> -p "@prompt.md"` — does `@path` syntax include file contents?
   - Determine which delivery mode Gromit should use for large prompt files.

2. **Streaming JSONL format (`--output-format stream-json`)**
   - Run: `gemini -m gemini-3-flash --output-format stream-json -p "simple coding task"`
   - Capture and document every event type: `init`, `message`, `tool_use`, `tool_result`, `error`, `result`.
   - Record exact field names, types, and nesting for each event type.
   - Determine how assistant text is accumulated from `message` events (chunked vs complete).
   - Verify ordering: does `result` always appear last?

3. **JSON output format (`--output-format json`)**
   - Run: `gemini -m gemini-3-flash --output-format json -p "simple task"`
   - Capture the `response`, `stats`, and `error` fields.
   - Document the exact `stats` schema (token counts, latency, cost if present).

4. **Token and cost reporting**
   - Does the `result` event (streaming) or `stats` object (JSON) include `input_tokens` and `output_tokens`?
   - Does Gemini report cost directly, or must Gromit calculate from token counts + configured pricing?
   - Are cached input tokens reported separately?

5. **Model selection**
   - Verify `-m gemini-3-flash`, `-m gemini-3-pro`, `-m gemini-3.1-pro` all work.
   - What happens with an invalid model name? Document the error format.

6. **Exit codes**
   - Confirm: 0=success, 1=general error, 42=input error, 53=turn limit exceeded.
   - Trigger each exit code and capture stderr output.

7. **Error patterns for classification**
   - **Auth failure**: Use an invalid API key or no credentials. Capture stderr patterns.
   - **Rate limit**: Attempt rapid sequential requests to trigger 429. Capture stderr patterns.
   - **Transport**: Force a disconnect if possible. Capture stderr patterns.
   - Document string patterns for `classifyGeminiError()` implementation.

8. **Tool permissions**
   - What flags control tool execution permissions? Is there an equivalent to Claude's `--dangerously-skip-permissions`?
   - Can file write and shell execution be enabled non-interactively?
   - What is the default permission behavior in headless mode?

9. **Working directory**
   - Does `gemini` respect the current working directory for file operations?
   - Can the working directory be overridden via a flag?
   - Test: run from `/tmp`, ask to read a file in the project directory — does it work?

### Output

Write findings to `.gromit/plans/gemini-cli-spike-findings.md` documenting:
- Verified invocation format with exact flags
- JSONL event schema with example payloads
- JSON result schema with example payload
- Token/cost reporting details
- Recommended prompt delivery mode for Gromit
- Error patterns for classification
- Tool permission flags
- Working directory behavior
- Any unexpected behaviors or limitations

## Acceptance Criteria

- Spike findings document exists with verified schemas for all 9 areas.
- At least one complete JSONL stream capture is saved as a test fixture candidate.
- At least one complete JSON result capture is saved as a test fixture candidate.
- Recommended prompt delivery mode is identified with rationale.
- Error classification patterns are documented with real stderr samples.
- No production code is written — this is research only.

## Execution Order

- Sequence position: 1
- Dependencies: none (Gemini CLI must be installed)
- Unblocks: `gemini-provider-adapter`

## Decisions

1. **Spike before implementation** — Gemini CLI documentation is sparse on exact schemas. Empirical verification prevents building against assumptions.

2. **Follow Codex spike precedent** — The Codex integration used the same spike-first approach (`codex-cli-spike-findings.md`), which caught several documentation-vs-reality gaps.

3. **Capture fixture candidates** — Real CLI output becomes the basis for contract test fixtures, ensuring provider tests match actual behavior.

## Research & Context

### Gemini CLI Basics

- Binary: `gemini`
- Non-interactive: `-p "prompt"` flag
- Model selection: `-m <model>` flag
- Structured output: `--output-format json|stream-json`
- Documented event types (stream-json): `init`, `message`, `tool_use`, `tool_result`, `error`, `result`
- Documented exit codes: 0, 1, 42, 53

### Gemini 3 Models (Feb 2026)

| Model | Tier Target | Input $/1M | Output $/1M |
|-------|------------|------------|-------------|
| gemini-3.1-pro | high | $2.00 | $12.00 |
| gemini-3-pro | high/medium | $2.00 | $12.00 |
| gemini-3-flash | medium/low | $0.50 | $3.00 |

### Existing Provider Patterns

- `ClaudeProvider`: wraps `claude -p --model <m> --output-format stream-json --verbose`
- `CodexProvider`: wraps `codex --json` or `codex exec --json`
- Both parse JSONL events, extract text from message events, capture cost from result events
- Both use FnField pattern for test injection

### Out of Scope

- Writing the `GeminiProvider` implementation (that's the next spec).
- Changing routing configuration.
- Performance benchmarking Gemini vs other providers.
