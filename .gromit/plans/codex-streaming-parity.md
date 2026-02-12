---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T15:44:11-05:00"
id: codex-streaming-parity
source_spec: codex-streaming-parity
---

# Codex Streaming Parity Implementation Plan

**Goal:** Make CodexProvider a first-class provider by implementing structured event streaming, validation, and scope-checking — achieving feature parity with ClaudeProvider.

**Architecture:** Extract provider-agnostic output-matching logic to shared helpers in the provider package, promote IsValidationPassed/IsScopeTooLarge to the Provider interface, implement Codex --json JSONL event parsing that normalizes to Gromit's StreamEvent format, add RunValidation using the same prompt pattern as Claude, and refactor the runner to use provider-level abstractions instead of importing the claude package directly.

**Tech Stack:** Go, Codex CLI (`codex exec --json`), existing Gromit provider/logger/runner packages

**Spec:** `.gromit/specs/codex-streaming-parity.md`

---

## Architecture

**Key Components:**

1. **Shared output helpers** (`internal/provider/helpers.go`): Provider-agnostic functions — `IsValidationPassed`, `IsScopeTooLarge`, `GetScopeTooLargeBreakdown`, `BuildValidationPrompt`, `ValidateCommands`. Pure text-matching, no provider-specific logic.

2. **Enhanced Provider interface** (`internal/provider/provider.go`): Adds `IsValidationPassed(*Result) bool` and `IsScopeTooLarge(*Result) (bool, string)`.

3. **Codex JSONL event parser** (`internal/provider/codex.go`): Structs for Codex event types. A `processCodexStream` function reading JSONL line-by-line, mapping to `StreamEvent` format, calling handlers, extracting result text and token usage.

4. **CodexProvider.StreamRun update**: When `EventHandler` is non-nil, adds `--json` flag, uses pipe-based stdout with line-by-line parsing. When nil, unchanged.

5. **CodexProvider.RunValidation**: Uses `BuildValidationPrompt` shared helper, runs via existing `Run` method.

6. **Runner refactor**: `executeClaudeInvocation` returns `*provider.Result`. All `claude.IsValidationPassed` / `claude.IsScopeTooLarge` / `claude.GetScopeTooLargeBreakdown` calls replaced with `provider` package equivalents.

**Codex → Gromit Event Mapping:**

| Codex Event | Gromit StreamEvent |
|---|---|
| `thread.started` | `type: "system"` |
| `item.completed` with `agent_message` | `type: "assistant"` with text content block |
| `item.started` with `command_execution` | `type: "assistant"` with tool_use block (ToolName: "Bash") |
| `item.started` with `file_change` | `type: "assistant"` with tool_use block (ToolName: "Write") |
| `item.started` with `mcp_tool_call` | `type: "assistant"` with tool_use block (ToolName: <tool>) |
| `turn.completed` with `usage` | `type: "result"` with InputTokens/OutputTokens |
| `turn.completed` with `UsageLimitExceeded` | `type: "error"`, subtype: `"rate_limit"` |

**Data Flow:**
```
StreamRun(handler != nil)
  → --json added to args
  → codex exec --json emits JSONL to stdout
  → processCodexStream reads line-by-line via bufio.Scanner
  → each line parsed as codexEvent struct
  → mapped to logger.StreamEvent
  → EventHandler(JSON-serialized StreamEvent)  → ParseAndLogEvent → StreamStats
  → ToolCallHandler(ToolEvent)                 → heartbeat channel
  → agent_message text → output writer         → terminal display
  → turn.completed usage → result fields
  → last agent_message text → Result.Output
```

## Test Strategy

**Unit Tests:**
- Shared helpers: parity with claude package equivalents, edge cases (nil result, empty output, mid-line markers)
- Event parsing: each of 7 event type mappings, malformed JSON handling, empty lines
- BuildValidationPrompt: command formatting, rejection of invalid commands

**Integration Tests (mock binary):**
- StreamRun with mock binary emitting controlled JSONL — verify EventHandler/ToolCallHandler called correctly
- StreamRun with nil handler — verify no --json flag, backward compatible
- RunValidation with mock binary echoing prompt content — verify prompt format

**Regression:**
- All existing provider and runner tests pass unchanged

**Mocking:**
- Mock bash scripts for CLI integration tests (existing pattern in codex_test.go)
- Direct JSON string input for unit-level parsing tests
- Existing runner mock infrastructure unchanged

---

## Implementation Tasks

### Task 1: Create shared output-matching helpers in provider package

**Files:**
- Create: `internal/provider/helpers.go`
- Create: `internal/provider/helpers_test.go`

**What to Do:**
Extract provider-agnostic output-matching logic from the `claude` package into shared functions in the `provider` package:

- `IsValidationPassed(result *Result) bool` — checks `result.Success && strings.Contains(result.Output, "VALIDATION_PASSED")`. Same logic as `claude.IsValidationPassed` but works on `provider.Result`.
- `IsScopeTooLarge(result *Result) (bool, string)` — finds `SCOPE_TOO_LARGE:` at start of line using `findStartOfLineMarker`, extracts explanation. Port `claude.IsScopeTooLarge` logic.
- `GetScopeTooLargeBreakdown(result *Result) string` — extracts full breakdown content. Port `claude.GetScopeTooLargeBreakdown` logic.
- `findStartOfLineMarker(output string) int` — helper to find marker at line start. Port from claude package.
- `ValidateCommands(commands []string) error` — validates command list (non-empty, no newlines, length limit). Port from `claude.ValidateCommands`.
- `BuildValidationPrompt(commands []string, workDir string) (string, error)` — constructs the validation prompt with numbered commands in fenced code block. Extracted from `claude.Client.RunValidation` prompt construction.

**Acceptance Criteria:**
- Shared helper functions produce identical results to claude package equivalents for the same inputs (verified by parity tests)
- BuildValidationPrompt formats numbered commands in a fenced code block matching the claude.RunValidation prompt format
- ValidateCommands rejects empty command lists and commands with newlines or exceeding 1024 chars

**Dependencies:** None

**Notes:**
The `findStartOfLineMarker` helper from `claude.go` (unexported) needs to be ported. The validation prompt text in `claude.go:173-186` should be replicated exactly.

---

### Task 2: Promote IsValidationPassed/IsScopeTooLarge to Provider interface

**Files:**
- Modify: `internal/provider/provider.go`
- Modify: `internal/provider/claude.go`
- Modify: `internal/provider/codex.go`

**What to Do:**
Add two methods to the Provider interface:
```go
IsValidationPassed(result *Result) bool
IsScopeTooLarge(result *Result) (bool, string)
```

Update `ClaudeProvider`:
- `IsValidationPassed` — change from delegating to `claude.IsValidationPassed(convertToClaudeResult(result))` to calling `IsValidationPassed(result)` shared helper directly. Remove the `convertToClaudeResult` call.
- `IsScopeTooLarge` — same change, delegate to shared helper.

Add `CodexProvider` implementations:
- `IsValidationPassed` — delegate to shared `IsValidationPassed(result)` helper
- `IsScopeTooLarge` — delegate to shared `IsScopeTooLarge(result)` helper

Both providers use identical logic via shared helpers since the marker detection is provider-agnostic.

**Acceptance Criteria:**
- Provider interface includes both methods and compile-time checks pass for both providers
- ClaudeProvider.IsValidationPassed/IsScopeTooLarge delegate to shared helpers (no longer import claude package for these)
- CodexProvider implements both methods using shared helpers

**Dependencies:** Task 1

**Notes:**
The existing tests in `claude_test.go` (TestClaudeProviderIsValidationPassedDelegation, TestClaudeProviderIsScopeTooLargeDelegation) and `claude_helper_methods_test.go` should continue passing. The acceptance test file has `//go:build acceptance` tag — these tests verify the same behavior.

---

### Task 3: Update runner to use provider-level abstractions

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/runner.go`

**What to Do:**

**In `process.go`:**
- Change `executeClaudeInvocation` return type from `*claude.Result` to `*provider.Result`. Remove the `claude.Result` conversion block (lines 242-252). Return `providerResult` directly.
- Change the heartbeat channel type from `chan claude.ToolEvent` (line 188) to `chan provider.ToolEvent`. Update the `onToolCall` closure to use `provider.ToolEvent`.
- Remove the handler type conversion dance (lines 195-222). The handlers are already `provider.EventHandler` / `provider.ToolCallHandler` types — pass them directly to `p.StreamRun`.
- Change `handleScopeTooLarge` parameter from `*claude.Result` to `*provider.Result`. Replace `claude.GetScopeTooLargeBreakdown(claudeResult)` (line 307) with `provider.GetScopeTooLargeBreakdown(result)`.
- Replace `claude.IsValidationPassed(valResult)` calls at lines 876, 1044, 1092, 1293 with `provider.IsValidationPassed(valResult)`. This requires changing `runDirectValidationCheck` return type from `*claude.Result` to `*provider.Result`.
- Update `runDirectValidationCheck` (line 913) to return `*provider.Result`. Change the result construction to use `provider.Result` fields.

**In `runner.go`:**
- Update `processBead` and related functions to use `*provider.Result` instead of `*claude.Result` where `executeClaudeInvocation` results are used (line 825 and subsequent usage).
- Replace `claude.IsScopeTooLarge(claudeResult)` at line 868 with `provider.IsScopeTooLarge(result)`.
- Replace `claude.IsValidationPassed(valResult)` at line 2112 with `provider.IsValidationPassed(valResult)`.
- Remove or reduce `claude` package imports from both files where no longer needed.

**Acceptance Criteria:**
- `executeClaudeInvocation` returns `*provider.Result` and no caller converts to `*claude.Result`
- All `claude.IsValidationPassed` / `claude.IsScopeTooLarge` / `claude.GetScopeTooLargeBreakdown` calls in runner replaced with `provider` package equivalents
- All existing runner tests pass without modification

**Dependencies:** Task 1, Task 2

**Notes:**
The `runDirectValidationCheck` function runs validation commands directly via `exec.Command` (not through a provider). It currently returns `*claude.Result` for backward compat. Changing it to return `*provider.Result` is straightforward since the fields are identical. The `startHeartbeat` function signature may also need updating if it takes `chan claude.ToolEvent` — check and update as needed.

---

### Task 4: Implement Codex StreamRun with --json event streaming

**Files:**
- Modify: `internal/provider/codex.go`
- Modify: `internal/provider/codex_test.go`

**What to Do:**

**Add Codex event types (unexported structs in codex.go):**
```go
type codexEvent struct {
    Type string          `json:"type"`    // thread.started, turn.started, turn.completed, item.started, item.completed
    Item *codexItem      `json:"item,omitempty"`
    Usage *codexUsage    `json:"usage,omitempty"`
    Status string        `json:"status,omitempty"`
    ErrorInfo *codexErrorInfo `json:"codexErrorInfo,omitempty"`
}

type codexItem struct {
    Type    string `json:"type"`    // agent_message, command_execution, file_change, mcp_tool_call
    Text    string `json:"text,omitempty"`
    Command string `json:"command,omitempty"`
    Path    string `json:"path,omitempty"`
    ToolName string `json:"tool_name,omitempty"`
}

type codexUsage struct {
    InputTokens       int `json:"input_tokens"`
    CachedInputTokens int `json:"cached_input_tokens"`
    OutputTokens      int `json:"output_tokens"`
}

type codexErrorInfo struct {
    Type string `json:"type"`  // UsageLimitExceeded, HttpConnectionFailed, etc.
}
```

**Implement `processCodexStream` function:**
- Takes `io.Reader` (stdout pipe), `io.Writer` (output), `EventHandler`, `ToolCallHandler`
- Returns `(resultText string, inputTokens int, outputTokens int, usageLimitDetected bool)`
- Uses `bufio.Scanner` with 1MB buffer (matching claude's pattern)
- For each line: parse as `codexEvent`, map to `logger.StreamEvent`, marshal to JSON, call `EventHandler`
- Event mapping logic per the table in the architecture section
- For `item.completed` with `agent_message`: write text to output writer, track as result text
- For tool events: construct `ToolEvent` and call `ToolCallHandler`
- For `turn.completed` with usage: capture token counts
- For `turn.completed` with `UsageLimitExceeded`: set usageLimitDetected flag, emit rate_limit error event

**Update `StreamRun`:**
- When `handler` is non-nil: add `"--json"` to command args (insert before model/prompt args), use `cmd.StdoutPipe()` instead of direct stdout capture, call `processCodexStream` on the pipe, set Result fields from returned values
- When `handler` is nil: behavior unchanged (existing plain text capture)
- If `usageLimitDetected`, include "UsageLimitExceeded" in the output so `IsUsageLimitError` string-matching picks it up

**Update existing tests and add new ones:**
- Update `TestCodexProviderStreamRunEventHandlerIsNoop` — this test verifies handler is NOT called when Codex has no stream-json. With the new behavior, handler IS called when non-nil and --json is active. Update test to verify handler IS called with JSONL mock binary.
- Add `TestCodexStreamRunWithJsonEvents` — mock binary emitting JSONL, verify EventHandler called for each event
- Add `TestCodexStreamRunNilHandlerNoJsonFlag` — verify --json NOT added when handler is nil
- Add `TestCodexProcessCodexStream*` — unit tests for each event type mapping
- Add `TestCodexStreamRunToolCallHandler` — verify ToolCallHandler called for tool events
- Add `TestCodexStreamRunTokenUsage` — verify token counts extracted from turn.completed
- Add `TestCodexStreamRunUsageLimitDetection` — verify UsageLimitExceeded detection

**Acceptance Criteria:**
- StreamRun with non-nil EventHandler adds `--json` flag and calls EventHandler for each parsed JSONL event normalized to StreamEvent format
- ToolCallHandler called for `command_execution`, `file_change`, and `mcp_tool_call` events with correct ToolEvent fields (ToolName, FilePath)
- Token usage extracted from `turn.completed` events and `UsageLimitExceeded` detected via structured `codexErrorInfo` field

**Dependencies:** None (can proceed in parallel with Tasks 1-3)

**Notes:**
The existing `TestCodexProviderStreamRunEventHandlerIsNoop` and `TestCodexProviderStreamRunToolCallHandlerIsNoop` tests assert handlers are NOT called. These must be updated since the new behavior calls handlers when non-nil (with --json active). The mock binary needs to emit valid JSONL for these tests to work. Consider having two mock binary variants: one that emits plain text (for nil handler) and one that emits JSONL (for non-nil handler).

---

### Task 5: Implement CodexProvider.RunValidation

**Files:**
- Modify: `internal/provider/codex.go`
- Modify: `internal/provider/codex_test.go`

**What to Do:**
Replace the current `RunValidation` stub (returns "not implemented" error) with a working implementation:

```go
func (cp *CodexProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error) {
    if cp == nil {
        return nil, fmt.Errorf("codex provider is nil")
    }
    prompt, err := BuildValidationPrompt(commands, workDir)
    if err != nil {
        return nil, err
    }
    return cp.Run(ctx, prompt, tier)
}
```

This uses `BuildValidationPrompt` from the shared helpers (Task 1) and runs via the existing `Run` method. The VALIDATION_PASSED/VALIDATION_FAILED markers in the output are detected by `IsValidationPassed` (from Task 2).

**Update tests:**
- Update `TestCodexProviderRunValidationIsNotImplemented` — rename and change to verify it now works: constructs prompt, runs via mock binary, result contains expected markers
- Add `TestCodexRunValidationPassedDetection` — mock binary that echoes "VALIDATION_PASSED", verify IsValidationPassed returns true
- Add `TestCodexRunValidationFailedDetection` — mock binary that echoes "VALIDATION_FAILED", verify IsValidationPassed returns false
- Add `TestCodexRunValidationInvalidCommands` — empty commands list returns error before invocation

**Acceptance Criteria:**
- RunValidation constructs validation prompt using shared BuildValidationPrompt and runs it via Run
- Result contains VALIDATION_PASSED or VALIDATION_FAILED marker detectable by IsValidationPassed
- Invalid commands (empty list, commands with newlines) return error before invocation

**Dependencies:** Task 1 (for BuildValidationPrompt and ValidateCommands)

**Notes:**
The existing test `TestCodexProviderRunValidationIsNotImplemented` will need to be significantly rewritten since RunValidation now works. The Codex CLI must be running in a mode that can execute bash commands (`--full-auto` or equivalent) — this is controlled by the existing `flags` field in CodexProvider configuration, not by RunValidation itself.

---

## Notes

- **Parallel work possible**: Tasks 1-3 form a dependency chain (helpers → interface → runner). Task 4 (StreamRun) is independent and can proceed in parallel. Task 5 depends only on Task 1.
- **Backward compatibility**: The runner refactor (Task 3) is the riskiest change since it touches multiple call sites. All existing tests must pass. The key insight is that `provider.Result` and `claude.Result` have identical field sets, so the refactor is mechanical.
- **Test updates**: Task 4 will break two existing tests (`TestCodexProviderStreamRunEventHandlerIsNoop`, `TestCodexProviderStreamRunToolCallHandlerIsNoop`) that assert handlers are NOT called. These tests reflect the old behavior where Codex had no event streaming. They must be updated to reflect the new --json streaming behavior.
- **ClaudeClient interface in runner**: The `ClaudeClient` interface in `internal/runner/interfaces.go` is related but not part of this spec. It may become removable after the runner fully migrates to provider-level abstractions, but that's a separate cleanup tracked by bead `gromit-914c`.
- **Codex event schema**: The event types defined in Task 4 are based on the Codex CLI documentation and may need adjustment when tested against real Codex CLI output. The `codex app-server generate-json-schema` command can generate the full schema for verification.
