---
id: agent-agnostic-llm-adapter
spec: debug-20260307-220935
created: 2026-03-07
decomposed: false
---

# Agent-Agnostic LLM Adapter Layer

Create separate `LLMProvider` implementations for Claude CLI and Codex CLI so the V2 routing layer can correctly invoke either provider without hardcoding CLI-specific flags.

## Architecture

The existing `LLMProvider` interface in `internal/v2/llmtypes/types.go` is already agnostic — it takes `{Prompt, Model, Dir}` and returns `{Success, Output, Tokens, CostUSD, Duration}`. The problem is there's only one implementation (`claudeAdapter`) that hardcodes Claude CLI flags. The fix is to add a `codexAdapter` and have `run2.go` select the right one.

```
llmtypes.LLMProvider (interface - unchanged)
├── claude.go   → claudeAdapter  (existing, rename from generic)
└── codex.go    → codexAdapter   (new, port from internal/provider/codex.go)
```

The V1 `CodexProvider` in `internal/provider/codex.go` is the reference implementation — it correctly builds `codex exec` args with `--json`, `--model`, `--skip-git-repo-check`, etc.

## Tasks

### Task 1: Create `codexAdapter` implementing `LLMProvider`
**Files:** `internal/v2/adapter/llm/codex.go`, `internal/v2/adapter/llm/codex_test.go`
**Size:** ~200 lines prod, ~150 lines test

Create `NewCodexAdapter(binary string, flags []string, timeout time.Duration, opts ...CodexOption) LLMProvider` that:

- `Invoke()`: builds args as `["exec", flags..., "--skip-git-repo-check", "--color", "never", "--model", model, "--json", "-"]`, pipes prompt to stdin, parses JSONL output for text+usage
- `StreamInvoke()`: same arg building but streams output, parses JSONL events for cost/tokens
- Port arg building logic from `internal/provider/codex.go:buildExecCommandArgs()`
- Port JSONL parsing from `internal/provider/codex.go:parseCodexOutputAndUsage()`
- Handle `--dangerously-bypass-approvals-and-sandbox` vs `--full-auto` mutual exclusivity
- Support `reasoning_effort` config via `CodexOption`
- Use `procutil` lifecycle: `WaitForProcessCapacity`, `SetProcessGroupKill`, `ReapProcessTree`
- Map codex result to `LLMInvokeResponse{Success, Output, Tokens, CostUSD, Duration}`

Reference: `internal/provider/codex.go` lines 148-527

### Task 2: Update `run2.go` to select adapter by provider type
**Files:** `cmd/gromit/run2.go`
**Size:** ~20 lines changed

In `buildRouter()`, instead of always using `NewClaudeAdapter`, check the provider config to determine which adapter to use:

```go
for name, def := range cfg.Providers {
    provBinary := binary
    if strings.TrimSpace(def.Binary) != "" {
        provBinary = strings.TrimSpace(def.Binary)
    }
    provFlags := flags
    if len(def.Flags) > 0 {
        provFlags = append([]string(nil), def.Flags...)
    }

    switch {
    case isCodexBinary(provBinary):
        var opts []llm.CodexOption
        if len(def.ReasoningEffort) > 0 {
            opts = append(opts, llm.WithReasoningEffort(def.ReasoningEffort))
        }
        providers[name] = llm.NewCodexAdapter(provBinary, provFlags, timeout, opts...)
    default:
        providers[name] = llm.NewClaudeAdapter(provBinary, provFlags, timeout)
    }
}
```

Detection: check if binary name contains "codex", or use `prompt_delivery` field as the discriminator (if `prompt_delivery == "prompt_file_arg"` → codex-style, if `"stdin"` → claude-style). Prefer explicit: add a `type` field to `ProviderDef` config, or infer from binary name.

### Task 3: Remove triage pin from gromit.yaml
**Files:** `gromit.yaml`
**Size:** 1 line

Remove the `triage: claude` stopgap from `phase_preferences` once the codex adapter is working. Triage should be routable to any provider.

### Task 4: Add CODEX_HOME env handling
**Files:** `internal/v2/adapter/llm/codex.go`
**Size:** ~20 lines

Port `ResolveCodexHomePath()` / `prepareCodexEnv()` from `internal/provider/codex.go` or call them directly. The codex adapter needs to set CODEX_HOME in the subprocess environment to avoid temp-dir issues.

### Task 5: Add transient retry logic to codex adapter
**Files:** `internal/v2/adapter/llm/codex.go`
**Size:** ~30 lines

Port the bounded retry from V1 (`codexTransientRetryMax=2`, backoff 250ms/750ms/1500ms) for transient failures (transport disconnect, rate limit). The claude adapter doesn't retry internally — this is codex-specific behavior needed for reliability.

### Task 6: Fix `plan.go` to accept injected provider
**Files:** `internal/v2/adapter/llm/plan.go`
**Size:** ~10 lines changed

`NewPlanLLMAdapter()` hardcodes `cfg.Claude` and calls `NewClaudeAdapter` directly (line 38). Refactor to accept an `LLMProvider` parameter instead of constructing one internally. This makes it provider-agnostic and consistent with how stages receive providers.

## Call Sites Audit

| Location | What it does | Status |
|----------|-------------|--------|
| `cmd/gromit/run2.go:310` | `buildRouter()` — creates all providers | **Active bug**: codex gets claude flags |
| `internal/v2/adapter/llm/plan.go:38` | `NewPlanLLMAdapter()` — plan stage provider | **Latent**: hardcodes `cfg.Claude`, plan pinned to claude |
| `internal/v2/adapter/llm/claude_test.go` | Test-only | OK |

## Dependencies

- Task 1 must complete before Task 2 (need the adapter to wire it)
- Task 4 and 5 can be done as part of Task 1 or separately
- Task 3 depends on Task 1+2 being verified working
- Task 6 is independent

## Testing Strategy

### Unit Tests — codexAdapter (Task 1)

Use fake script binaries (same pattern as `claude_test.go`) that echo expected output.

| Test | Verifies |
|------|----------|
| `TestCodexAdapter_Invoke_BuildsCorrectArgs` | Args include `exec`, `--json`, `--model`, `--skip-git-repo-check`; do NOT include `-p` or `--output-format` |
| `TestCodexAdapter_Invoke_ParsesJSONLOutput` | Extracts text response, input/output tokens, cost from codex JSONL |
| `TestCodexAdapter_Invoke_HandlesFailure` | Non-zero exit → `Success: false`, stderr captured |
| `TestCodexAdapter_Invoke_Timeout` | Context timeout → error, process reaped |
| `TestCodexAdapter_Invoke_BypassApprovalsExcludesFullAuto` | `--dangerously-bypass-approvals-and-sandbox` in flags → `--full-auto` omitted |
| `TestCodexAdapter_Invoke_ReasoningEffort` | Reasoning effort config → `-c model_reasoning_effort=X` in args |
| `TestCodexAdapter_StreamInvoke_StreamsOutput` | Output writer receives streaming content |
| `TestCodexAdapter_Invoke_SetsCodexHomeEnv` | Subprocess env includes resolved CODEX_HOME |
| `TestCodexAdapter_Invoke_RetriesTransient` | Transport disconnect → retried up to 2x with backoff |

### Unit Tests — buildRouter selection (Task 2)

| Test | Verifies |
|------|----------|
| `TestBuildRouter_SelectsClaudeAdapterForClaudeBinary` | Provider with `binary: claude` → `claudeAdapter` type |
| `TestBuildRouter_SelectsCodexAdapterForCodexBinary` | Provider with `binary: codex` → `codexAdapter` type |
| `TestBuildRouter_PassesReasoningEffortToCodex` | `reasoning_effort` config forwarded to codex adapter |

### Contract Tests — flag isolation

| Test | Verifies |
|------|----------|
| `TestClaudeAdapter_NeverIncludesExecSubcommand` | Claude args never start with `exec` |
| `TestCodexAdapter_NeverIncludesDashP` | Codex args never contain `-p` |
| `TestCodexAdapter_NeverIncludesOutputFormat` | Codex args never contain `--output-format` |

### Integration Verification

After Tasks 1-3 land:
1. Remove `triage: claude` pin from gromit.yaml
2. Run `gromit run2` with a spec that triggers triage
3. Verify codex provider handles triage without the `--profile` error
4. Verify cost/token attribution flows correctly from codex JSONL
