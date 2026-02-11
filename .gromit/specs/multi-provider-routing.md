---
id: multi-provider-routing
source_ideas: []
created: 2026-02-11
---

# Multi-Provider Routing for the Build Loop

## Specification

Gromit currently uses Claude CLI as the sole LLM backend for all automated phases (build, validate, analyze, decompose, scope check, precheck, review). This feature adds the ability to route automated work across multiple LLM providers — primarily Claude and OpenAI's Codex CLI — using a combination of task-type preferences, ratio-based balancing, and automatic fallback on usage-limit errors.

### Why

LLM subscription plans have usage caps that reset periodically. When one provider's cap is exhausted, the entire Gromit loop halts. By supporting multiple providers, Gromit can:

1. Spread usage across providers to stay within per-provider caps
2. Automatically fall back to an available provider when one hits its limit
3. Let users assign provider preferences per phase (e.g., "Claude for builds, either for validation")

### Provider Definitions

Providers are defined in `gromit.yaml` under a new `providers` section. Each provider specifies a CLI binary, flags, prompt delivery method, and a model tier mapping.

```yaml
providers:
  claude:
    binary: claude
    flags: ["--no-input"]
    prompt_delivery: stdin        # stdin | file_ref | prompt_file_arg
    models:
      high: opus
      medium: sonnet
      low: haiku
  openai:
    binary: codex
    flags: []
    prompt_delivery: prompt_file_arg
    prompt_flag: "--prompt"
    models:
      high: o3
      medium: gpt-4o
      low: gpt-4o-mini
```

Each provider maps three abstract tiers (`high`, `medium`, `low`) to its own model names. The existing priority-based selection (P0→high, P1→medium, P2→low) and label overrides (`complexity:high`→high tier) resolve to an abstract tier first, then the tier is mapped to a provider-specific model name.

When no `providers` section exists, Gromit falls back to the existing `claude` config section — fully backward-compatible.

### Model Tier Mapping

The current model selection flow changes from:

```
priority/labels → model name (e.g., "opus")
```

To:

```
priority/labels → abstract tier (high/medium/low) → provider → concrete model name
```

The `models` config section changes to use tiers:

```yaml
models:
  p0: high       # P0 beads use the "high" tier
  p1: medium     # P1 beads use the "medium" tier
  p2: low        # P2 beads use the "low" tier
  validation: low
  labels:
    "complexity:high": high
    "complexity:low": low
```

For backward compatibility, if `models.p0` contains a known model name (opus, sonnet, haiku, o3, gpt-4o, etc.) instead of a tier name, Gromit treats it as a direct Claude model name and uses the legacy single-provider path.

### Routing Strategy

Three layers determine which provider handles each invocation, evaluated in order:

**Layer 1: Phase preferences** — Certain phases can be pinned to a specific provider:

```yaml
routing:
  phase_preferences:
    build: claude          # prefer claude for code generation
    validate: any          # any provider
    analyze: any
    scope_check: any
    precheck: any
    decompose: claude      # prefer claude for decomposition
    review: claude         # prefer claude for reviews
```

`any` means the ratio balancer (layer 2) decides. A named provider means "use this provider unless it's unavailable."

**Layer 2: Ratio balancing** — For `any` phases, and as a tiebreaker when the preferred provider is available, Gromit tracks invocation counts and routes to maintain a target ratio:

```yaml
routing:
  ratio:
    claude: 60
    openai: 40
```

Gromit tracks cumulative invocation counts per provider in `.gromit/state.json` and routes the next invocation to whichever provider is furthest below its target ratio. This is a soft target — phase preferences can override it.

**Layer 3: Automatic fallback** — When a provider returns a usage-limit error, Gromit marks it as unavailable for a cooldown period and routes all subsequent work to the next available provider:

```yaml
routing:
  fallback:
    enabled: true
    cooldown: 30m         # how long to avoid a limited provider
```

Usage-limit detection works by inspecting the exit code and stderr/stdout of the provider CLI for known error patterns (e.g., "usage limit", "rate limit", "quota exceeded"). Each provider implementation defines its own error patterns.

### Provider Interface

A new `internal/provider` package defines the abstraction:

```go
// Provider executes LLM invocations via a CLI tool
type Provider interface {
    Name() string
    Run(ctx context.Context, prompt string, tier string) (*Result, error)
    StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
              handler EventHandler, onToolCall ToolCallHandler) (*Result, error)
    RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*Result, error)
    IsUsageLimitError(result *Result, err error) bool
}
```

Key differences from the current `ClaudeClient` interface:
- Takes a **tier** (high/medium/low) instead of a model name — the provider maps to its own models internally
- Adds `IsUsageLimitError()` to detect provider-specific rate/usage limit errors
- `Result` is provider-agnostic (same fields as `claude.Result` but in the `provider` package)

Two concrete implementations:
- `ClaudeProvider` — wraps the existing `claude.Client` logic
- `CodexProvider` — invokes `codex` CLI with appropriate flags and prompt delivery

### Router

A new `internal/provider/router.go` handles the routing logic:

```go
type Router struct {
    providers    map[string]Provider
    preferences  map[string]string     // phase → provider name or "any"
    ratio        map[string]int        // provider → target percentage
    counts       map[string]int        // provider → invocation count (persisted in state.json)
    unavailable  map[string]time.Time  // provider → when it became unavailable
    cooldown     time.Duration
}

// Select picks the provider for a given phase and tier
func (r *Router) Select(phase string, tier string) (Provider, string) { ... }

// MarkUnavailable records a usage-limit hit for a provider
func (r *Router) MarkUnavailable(name string) { ... }
```

The `Select` method:
1. Checks phase preference — if a specific provider is preferred and available, use it
2. If `any` or preferred provider is unavailable, pick the available provider furthest below its target ratio
3. If all providers are unavailable, return an error (the loop should halt)

### Integration with Runner

The runner currently holds a `ClaudeClient` interface. This changes to hold a `Router`:

```go
type Runner struct {
    router  *provider.Router   // replaces: claude ClaudeClient
    // ...
}
```

Every invocation site in the runner changes from:

```go
result, err := r.claude.Run(ctx, prompt, model)
```

To:

```go
p, modelName := r.router.Select(phase, tier)
result, err := p.Run(ctx, prompt, tier)
if err != nil && p.IsUsageLimitError(result, err) {
    r.router.MarkUnavailable(p.Name())
    // retry with fallback provider
    p, modelName = r.router.Select(phase, tier)
    result, err = p.Run(ctx, prompt, tier)
}
```

### Stream-JSON Compatibility

The current `StreamRun` parses Claude's `--output-format stream-json` event format. Codex CLI has a different output format. The provider abstraction handles this:

- `ClaudeProvider.StreamRun()` — uses `--output-format stream-json` and parses Claude's JSON events
- `CodexProvider.StreamRun()` — uses Codex's native output format and adapts it to the common `Result` struct

The `EventHandler` and `ToolCallHandler` callbacks become provider-specific internals. The runner only sees the `Result` at the end.

### Validation Compatibility

The current `RunValidation` sends a structured prompt asking Claude to run commands and look for `VALIDATION_PASSED`. This is LLM-agnostic in principle — it's just a prompt convention. Both Claude and Codex should be able to follow the instructions. The `IsValidationPassed()` check remains the same (string match on output).

### State Persistence

Provider routing state is persisted in `.gromit/state.json`:

```json
{
  "provider_counts": {
    "claude": 15,
    "openai": 10
  },
  "provider_unavailable_until": {
    "claude": "2026-02-11T15:30:00Z"
  }
}
```

Counts reset when a new `gromit run` session starts (or optionally accumulate across sessions — TBD).

### Escalation Chain

The existing escalation chain (`haiku→sonnet→opus`) becomes tier-based:

```yaml
escalation:
  enabled: true
  chain: [low, medium, high]    # abstract tiers
  max_retries_per_model: 1
  max_retries_per_bead: 3
```

Escalation moves up tiers within the selected provider. If escalation exhausts all tiers AND the provider hits a usage limit, the router falls back to the other provider starting at the same tier.

For backward compatibility, string values "haiku"/"sonnet"/"opus" in the chain are mapped to low/medium/high.

## Acceptance Criteria

- Users can define multiple providers in `gromit.yaml` under `providers` with binary, flags, prompt_delivery, and model tier mappings
- When no `providers` section exists, Gromit uses the existing `claude` config section identically to current behavior (full backward compatibility)
- Phase preferences in `routing.phase_preferences` control which provider handles each phase type
- Ratio-based balancing in `routing.ratio` spreads invocations across providers when phase preference is `any`
- When a provider hits a usage limit (detected via exit code and output patterns), Gromit automatically marks it unavailable and routes to the next available provider
- After the configured cooldown period, Gromit re-enables the unavailable provider
- Provider invocation counts are tracked in `.gromit/state.json` for ratio balancing
- Escalation chain works with abstract tiers (low→medium→high) instead of provider-specific model names
- The `models` config section supports both legacy model names (backward compat) and abstract tier names

## Decisions

1. **CLI tools, not APIs.** Providers are invoked as CLI subprocesses, consistent with the existing architecture and the multi-agent-phases spec. This avoids managing API keys, HTTP clients, and auth flows in Gromit. CLI tools handle their own authentication.

2. **Abstract tier mapping instead of direct model names.** The priority→tier→model indirection means the router doesn't need to know provider-specific model names. Adding a new provider is just defining its tier mapping. Label overrides like `complexity:high` map to the `high` tier, which each provider resolves independently.

3. **Ratio balancing is soft, not strict.** Phase preferences take precedence over ratio targets. The ratio only breaks ties and guides `any`-phase routing. This prevents the ratio from overriding domain-specific routing decisions.

4. **Usage-limit detection is provider-specific.** Each provider implementation defines its own error patterns. Claude returns specific exit codes and error messages; Codex returns different ones. The `IsUsageLimitError()` method encapsulates this per provider.

5. **Router replaces ClaudeClient in the runner.** Rather than adding a parallel abstraction, the router subsumes the `ClaudeClient` interface. This avoids split-brain issues where some invocations go through the router and others bypass it.

6. **Cooldown-based recovery, not token counting.** Rather than trying to estimate remaining token budgets (which is fragile and provider-specific), Gromit uses a simple time-based cooldown. After hitting a limit, it avoids that provider for the configured duration. Users can tune the cooldown to match their plan's reset interval.

7. **Stream format is an internal concern.** The runner should not need to know whether it's reading Claude's stream-json or Codex's output format. Each provider's `StreamRun()` handles its own format parsing and returns a common `Result`. The `EventHandler` is Claude-specific and can be kept internal to `ClaudeProvider`, with a no-op or equivalent for other providers.

## Research & Context

### Current Architecture

- `internal/claude/claude.go` — The sole LLM client. Invokes `exec.CommandContext` with `-p`, `--model`, and flags. Three methods: `Run()`, `StreamRun()`, `RunValidation()`. Output parsing is Claude-specific (stream-json event format with assistant/result event types).
- `internal/runner/interfaces.go` — `ClaudeClient` interface with `Run`, `StreamRun`, `RunValidation` methods. Runner depends on this interface, enabling substitution.
- `internal/runner/runner.go` — Runner holds `claude ClaudeClient` field. LLM invocation sites: `executeClaudeInvocation()` (build), `checkScope()`, `runPrecheck()`, `decomposeBeadIfNeeded()`, validation in `validateBead()`.
- `internal/config/config.go` — `ClaudeConfig` struct has binary, timeout, flags, per-model timeouts. `ModelsConfig` maps P0/P1/P2 to model names. `SelectModel()` uses priority + label overrides.
- `internal/agent/` — Already supports claude, codex, gemini as agent types with different prompt delivery methods. But this is only for interactive phases (refine, plan, review, explore), not the automated build loop.

### Codex CLI Interface

OpenAI's Codex CLI uses:
- `codex --prompt <file>` for non-interactive execution
- `--model <name>` for model selection
- Stdout/stderr for output
- Different exit codes and error messages than Claude CLI

The prompt delivery and output format differences are why each provider needs its own implementation rather than a purely config-driven approach.

### Related Specs

- `multi-agent-phases` — Covers agent selection for interactive phases. This spec extends the concept to automated phases with routing logic. The two specs are complementary: multi-agent-phases handles interactive TTY sessions, this spec handles the automated build loop.
