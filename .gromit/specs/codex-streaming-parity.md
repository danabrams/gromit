---
id: codex-streaming-parity
source_ideas: []
created: 2026-02-12
epic: provider-ecosystem
---

# Codex Streaming Parity

## Specification

Make `CodexProvider` a first-class provider by implementing structured event streaming, validation, and scope-checking — achieving feature parity with `ClaudeProvider`.

### StreamRun with `--json` Events

When `CodexProvider.StreamRun` is called with a non-nil `EventHandler`, it invokes `codex exec --json` instead of plain text capture. The `--json` flag causes Codex CLI to emit JSONL events to stdout. These events are parsed line-by-line and normalized into Gromit's existing `logger.StreamEvent` format before being passed to the `EventHandler`. The downstream stream logger does not need to know which provider emitted the event.

**Codex → Gromit event mapping:**

| Codex Event | Gromit StreamEvent | Notes |
|---|---|---|
| `thread.started` | `type: "system"` | Session initialization |
| `item.completed` with `type: "agent_message"` | `type: "assistant"` with text content block | Agent text response |
| `item.started` with `type: "command_execution"` | Tool call: `ToolEvent{ToolName: "Bash"}` | Command being executed; `FilePath` = command string |
| `item.started` with `type: "file_change"` | Tool call: `ToolEvent{ToolName: "Write"}` | File modification; `FilePath` from change path |
| `item.started` with `type: "mcp_tool_call"` | Tool call: `ToolEvent{ToolName: <tool>}` | MCP tool invocation |
| `turn.completed` with `usage` | `type: "result"` with `InputTokens`, `OutputTokens` | Token usage data |
| `turn.completed` with `status: "failed"` and `UsageLimitExceeded` | Rate limit event | Detected via `codexErrorInfo.type` |

When `EventHandler` is nil (e.g., in `Run`), the `--json` flag is not added and behavior is unchanged (plain text capture).

**Text extraction:** The final result text is extracted from `item.completed` events with `type: "agent_message"`. The last such event's `text` field becomes `Result.Output`.

**Real-time streaming to terminal:** Agent message text is written to the `output` writer as it arrives (via `item.agentMessage.delta` events if available, otherwise from `item.completed` agent messages).

### RunValidation

`CodexProvider.RunValidation` follows the same prompt-based pattern as Claude: construct a prompt listing the validation commands, run it via `codex exec`, and check the output for `VALIDATION_PASSED` / `VALIDATION_FAILED` markers. The validation prompt template is identical — it's provider-agnostic.

This requires Codex to run in a mode that can execute bash commands (e.g., `--full-auto` or `--dangerously-bypass-approvals-and-sandbox`). The approval mode is controlled by the existing `flags` field in `CodexProvider` configuration.

### Provider Interface Promotion

`IsValidationPassed` and `IsScopeTooLarge` are promoted from `ClaudeProvider`-only methods to the `Provider` interface. The marker-checking logic (scanning output for `VALIDATION_PASSED` and `SCOPE_TOO_LARGE` text) is provider-agnostic and moves to a shared helper in the `provider` package. The runner is updated to call these methods through the `Provider` interface instead of importing the `claude` package directly.

### IsUsageLimitError Enhancement

`CodexProvider.IsUsageLimitError` gains structured detection: when `--json` streaming is active, rate limit errors are detected from `turn.completed` events with `codexErrorInfo.type == "UsageLimitExceeded"` rather than string-matching output text. The string-matching fallback remains for non-streaming runs.

## Acceptance Criteria

- `CodexProvider.StreamRun` with a non-nil `EventHandler` invokes `codex exec --json` and parses JSONL events into `StreamEvent` format, calling the handler for each event
- `ToolCallHandler` is invoked for `command_execution`, `file_change`, and `mcp_tool_call` item events with correct `ToolEvent` fields
- Token usage (`InputTokens`, `OutputTokens`) is extracted from `turn.completed` events and available in stream stats
- `CodexProvider.RunValidation` constructs a validation prompt, runs it, and correctly detects `VALIDATION_PASSED` / `VALIDATION_FAILED` markers in output
- `Provider` interface includes `IsValidationPassed(*Result) bool` and `IsScopeTooLarge(*Result) (bool, string)`, and both `ClaudeProvider` and `CodexProvider` implement them
- Runner code references `Provider.IsValidationPassed` / `Provider.IsScopeTooLarge` instead of `claude.IsValidationPassed` / `claude.IsScopeTooLarge`
- Rate limit events from Codex (`UsageLimitExceeded` in error info) are detected and surfaced through `IsUsageLimitError`

## Decisions

1. **Normalize to common event model, not provider-aware logging.** Codex JSONL events are converted to Gromit's `StreamEvent` format inside `CodexProvider`. The stream logger and stats tracker remain provider-agnostic. This keeps the event pipeline simple and avoids a proliferation of provider-specific parsing in the logger.

2. **`--json` flag is conditional on EventHandler presence.** When `EventHandler` is nil (plain `Run` calls, or streaming without logging), the `--json` flag is omitted and Codex runs in its default text mode. This avoids parsing overhead when structured events aren't needed and maintains backward compatibility.

3. **Validation uses the same prompt pattern as Claude.** Rather than building a Codex-specific validation mechanism, we reuse the identical prompt structure (numbered commands → VALIDATION_PASSED/FAILED markers). The prompt template should be extracted from `claude.RunValidation` into a shared location.

4. **Promote IsValidationPassed/IsScopeTooLarge to Provider interface.** These are output-text-matching functions, not Claude-specific. Moving them to the interface and having both providers delegate to shared helpers eliminates the runner's direct dependency on the `claude` package for these checks.

5. **Event parsing lives in `provider/codex.go`, not in a separate package.** Unlike Claude which has a separate `internal/claude` package with its own `processStreamJSON`, the Codex event parser lives directly in the provider implementation. This is simpler — the Codex CLI doesn't have enough complexity to warrant a dedicated package yet.

## Research & Context

### Current State

- **Provider interface** (`internal/provider/provider.go`): Defines `Provider` with `Run`, `StreamRun`, `RunValidation`, `IsUsageLimitError`, `Name`, `ModelForTier`
- **ClaudeProvider** (`internal/provider/claude.go`): Full implementation wrapping `internal/claude` client. Has extra `IsValidationPassed` and `IsScopeTooLarge` methods not in the interface
- **CodexProvider** (`internal/provider/codex.go`): Partial implementation — `StreamRun` is plain text capture with no-op handlers, `RunValidation` returns "not implemented" error
- **Stream logging** (`internal/logger/stream.go`): `StreamEvent` struct, `ParseAndLogEvent`, `StreamStats` — all designed around Claude's stream-json format but the event types are generic enough to support Codex events
- **Runner** (`internal/runner/process.go`, `runner.go`): Calls `claude.IsValidationPassed()` and `claude.IsScopeTooLarge()` directly instead of through the provider interface

### Codex CLI Capabilities

- `codex exec --json` outputs JSONL events to stdout with types: `thread.started`, `turn.started`, `turn.completed`, `item.started`, `item.completed`, and delta events
- Item types include: `agent_message`, `command_execution`, `file_change`, `mcp_tool_call`, `web_search`, `reasoning`, `plan`
- `turn.completed` includes `usage: {input_tokens, cached_input_tokens, output_tokens}`
- Error events include structured `codexErrorInfo` with types like `UsageLimitExceeded`, `HttpConnectionFailed`
- Schema can be generated with `codex app-server generate-json-schema`

### Sources

- [Codex CLI Reference](https://developers.openai.com/codex/cli/reference/)
- [Codex Non-interactive Mode](https://developers.openai.com/codex/noninteractive/)
- [Codex App Server Protocol](https://developers.openai.com/codex/app-server/)
- [GitHub Issue #2288 - JSON output](https://github.com/openai/codex/issues/2288)
