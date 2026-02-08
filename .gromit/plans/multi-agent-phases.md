---
id: multi-agent-phases
source_spec: multi-agent-phases
created: 2026-02-08
decomposed: true
---

# Multi-Agent Interactive Phases Implementation Plan

**Goal:** Make interactive phases (refine, plan, review) agent-agnostic so users can choose which CLI agent runs each phase.

**Architecture:** New `internal/agent` package defines an `Agent` interface and handles resolution, presets, and launch. Config gains an `agents:` section for definitions and per-phase assignments. The three interactive commands delegate to `agent.Resolve()` + `agent.Launch()` instead of hardcoding Claude.

**Tech Stack:** Go, cobra (CLI flags), gopkg.in/yaml.v3 (config parsing)

**Spec:** `.gromit/specs/multi-agent-phases.md`

---

## Architecture

### New Package: `internal/agent`

**`agent.go`** — Core interface and implementation:

```go
// Agent is the contract for launching an interactive CLI agent.
type Agent interface {
    Name() string
    Launch(promptPath string) error
}

// PromptDelivery controls how the agent receives the prompt.
type PromptDelivery string

const (
    FileRef       PromptDelivery = "file_ref"        // "Read and follow ... in <path>"
    PromptFileArg PromptDelivery = "prompt_file_arg"  // --flag <path>
    Stdin         PromptDelivery = "stdin"             // pipe file content to stdin
)

// cliAgent implements Agent for CLI-based tools.
type cliAgent struct {
    name           string
    binary         string
    flags          []string
    promptDelivery PromptDelivery
    promptFlag     string
    extraArgs      []string
}
```

`cliAgent.Launch()` builds `exec.Command` based on `promptDelivery`:
- `file_ref`: args = `flags + extraArgs + "Read and follow instructions in <path>"`
- `prompt_file_arg`: args = `flags + extraArgs + promptFlag + path`
- `stdin`: args = `flags + extraArgs`, pipe file content to process stdin

All modes connect stdin/stdout/stderr to the terminal. `*exec.ExitError` is treated as graceful exit (matching current behavior).

**`presets.go`** — Built-in agent presets:
- `claude`: binary from `cfg.Claude.Binary`, flags from `cfg.Claude.Flags`, delivery `file_ref`
- `codex`: binary `"codex"`, delivery `prompt_file_arg`, prompt flag `"--prompt"`
- `gemini`: binary `"gemini"`, delivery `prompt_file_arg`, prompt flag `"--prompt"`

Presets are functions that return a `Definition` (the config struct), not an `Agent`. User config overrides merge on top of presets: user-specified fields win, missing fields fall back to preset defaults.

**`resolve.go`** — Agent resolution:

```go
func Resolve(cfg *config.Config, phase string, flagOverride string, chooseAgent bool) (Agent, error)
```

Resolution priority:
1. `--agent <name>` flag → use that agent directly
2. `--choose-agent` flag OR `agents.prompt: true` → show picker, use result
3. `agents.phases.<phase>` config → use configured default
4. Fall back to `"claude"`

Returns error if the resolved agent name is not found in definitions or presets.

**`picker.go`** — Interactive agent picker:

```go
func Pick(agents []string, defaultAgent string, r io.Reader, w io.Writer) (string, error)
```

Renders numbered list, reads choice. Uses `io.Reader`/`io.Writer` for testability.

### Config Additions

New structs in `internal/config/config.go`:

```go
type AgentsConfig struct {
    Definitions map[string]AgentDefinition `yaml:"definitions"`
    Phases      PhasesConfig               `yaml:"phases"`
    Prompt      bool                       `yaml:"prompt"`
}

type AgentDefinition struct {
    Binary         string   `yaml:"binary"`
    Flags          []string `yaml:"flags"`
    PromptDelivery string   `yaml:"prompt_delivery"`
    PromptFlag     string   `yaml:"prompt_flag"`
}

type PhasesConfig struct {
    Refine  string `yaml:"refine"`
    Plan    string `yaml:"plan"`
    Review  string `yaml:"review"`
    Explore string `yaml:"explore"`
}
```

Add `Agents AgentsConfig` to `Config` struct. `SetDefaults` initializes phases to `"claude"` and definitions to empty map. `NormalizeNilFields` initializes nil maps/slices.

### Command Changes

Each interactive command (`refine.go`, `plan.go`, `review.go`) gains:
- `--agent <name>` string flag
- `--choose-agent` bool flag

The Claude launch boilerplate (resolve binary, build args, exec.Command) is replaced with:

```go
resolved, err := agent.Resolve(cfg, "refine", agentFlag, chooseAgentFlag)
if err != nil { return err }
if err := resolved.Launch(promptPath); err != nil { return err }
```

The prompt building and temp file creation remain unchanged. Post-exit artifact detection remains unchanged.

---

## Test Strategy

### Unit Tests

**`internal/agent/agent_test.go`:**
- `TestLaunch_FileRef`: Verify file_ref delivery builds correct args
- `TestLaunch_PromptFileArg`: Verify prompt_file_arg delivery builds correct args
- `TestLaunch_Stdin`: Verify stdin delivery pipes prompt content
- `TestResolve_FlagOverride`: `--agent codex` returns codex regardless of config
- `TestResolve_PhaseDefault`: No flag, config says `refine: gemini`, returns gemini
- `TestResolve_NoAgentsSection`: No agents config returns claude from `cfg.Claude`
- `TestResolve_UnknownAgent`: `--agent nonexistent` returns error
- `TestResolve_ChooseAgent`: With chooseAgent=true, invokes picker
- `TestResolve_PromptConfig`: `agents.prompt: true` triggers picker
- `TestPresetMerge_ClaudeUsesClaudeConfig`: Claude preset inherits from cfg.Claude
- `TestPresetMerge_UserOverride`: User-defined fields override preset defaults

**`internal/agent/picker_test.go`:**
- `TestPick_ValidChoice`: User enters "2", returns second agent
- `TestPick_DefaultOnEmpty`: Enter with no input returns default
- `TestPick_InvalidInput`: Non-numeric or out-of-range re-prompts
- `TestPick_SingleAgent`: Only one agent, skips picker and returns it

**`internal/config/config_test.go` (additions):**
- `TestSetDefaults_AgentsConfig`: Verify phases default to "claude"
- `TestNormalizeNilFields_Agents`: Verify nil definitions map initialized
- `TestLoad_WithAgentsSection`: Verify YAML parsing of agents config

### Mocking Strategy

- `Launch()` tests use Go's `exec_test` pattern (test helper process) to verify command construction without launching real agents
- `Resolve()` tests that involve the picker accept a picker function parameter or use dependency injection
- `Pick()` tests use `strings.Reader`/`bytes.Buffer` — no mocking needed
- Command-level integration can be tested by passing mock `Agent` implementations

### Test Organization

- `internal/agent/agent_test.go` — Launch, Resolve, preset tests
- `internal/agent/picker_test.go` — Picker tests
- `internal/config/config_test.go` — Config struct additions (existing file)

---

## Implementation Tasks

### Task 1: Add agent config structs

**Files:**
- Modify: `internal/config/config.go`

**What to Do:**
Add `AgentsConfig`, `AgentDefinition`, and `PhasesConfig` structs. Add `Agents AgentsConfig` field to the `Config` struct. Update `SetDefaults()` to set phase defaults to `"claude"`. Update `NormalizeNilFields()` to initialize nil `Definitions` map and nil `Flags` slices within definitions.

**Acceptance Criteria:**
- `AgentsConfig`, `AgentDefinition`, `PhasesConfig` structs exist with correct YAML tags
- `SetDefaults` initializes all phase fields to `"claude"` when empty
- `NormalizeNilFields` initializes nil `Definitions` map to empty map
- Existing configs without `agents:` section load and behave identically (backward-compatible)

**Dependencies:** None

### Task 2: Add agent config tests

**Files:**
- Modify: `internal/config/config_test.go`

**What to Do:**
Add tests for the new agent config structs. Test SetDefaults populates phase defaults, NormalizeNilFields handles nil maps, and YAML parsing correctly loads agent definitions.

**Acceptance Criteria:**
- Tests verify SetDefaults sets all phases to "claude"
- Tests verify NormalizeNilFields initializes nil Definitions map
- Tests verify YAML with `agents:` section parses correctly into structs

**Dependencies:** Task 1

### Task 3: Create agent interface, types, and Launch implementation

**Files:**
- Create: `internal/agent/agent.go`

**What to Do:**
Define the `Agent` interface (`Name() string`, `Launch(promptPath string) error`), `PromptDelivery` type with constants (`FileRef`, `PromptFileArg`, `Stdin`), and `cliAgent` struct implementing `Agent`. Implement `Launch()` with the three prompt delivery modes: file_ref constructs a "Read and follow instructions in <path>" message as positional arg, prompt_file_arg passes `promptFlag + path` as flag args, stdin pipes the prompt file content. All modes connect stdin/stdout/stderr to the terminal and treat `*exec.ExitError` as non-error. Export a `New(name, binary string, flags []string, delivery PromptDelivery, promptFlag string, extraArgs []string) Agent` constructor.

**Acceptance Criteria:**
- `Agent` interface is exported with `Name()` and `Launch()` methods
- `cliAgent` correctly builds exec.Command for all three prompt delivery modes
- `*exec.ExitError` is swallowed (treated as graceful exit)

**Dependencies:** None

### Task 4: Add built-in presets and Resolve function

**Files:**
- Create: `internal/agent/resolve.go`

**What to Do:**
Implement built-in preset definitions for `claude`, `codex`, and `gemini`. The `claude` preset reads binary/flags from `cfg.Claude.Binary`/`cfg.Claude.Flags` and uses `file_ref` delivery. Implement `Resolve(cfg *config.Config, phase string, flagOverride string, chooseAgent bool, r io.Reader, w io.Writer) (Agent, error)` that follows the resolution priority: flag override → picker (if chooseAgent or agents.prompt) → phase config → "claude" default. Merge user-defined agent definitions with preset defaults (user fields win, missing fields fall back to preset). Return error for unknown agent names.

**Acceptance Criteria:**
- Claude preset uses `cfg.Claude.Binary` and `cfg.Claude.Flags` for backward compatibility
- Resolution priority is: `--agent` flag > picker > `agents.phases` config > `"claude"` default
- User-defined definition fields override preset defaults; missing fields use preset
- Unknown agent name returns a clear error

**Dependencies:** Task 1, Task 3

### Task 5: Add interactive agent picker

**Files:**
- Create: `internal/agent/picker.go`

**What to Do:**
Implement `Pick(agents []string, defaultAgent string, r io.Reader, w io.Writer) (string, error)`. Render a numbered list of agents, marking the default with "(default)". Read user choice from `r`. Empty input selects the default. Invalid input (non-numeric, out of range) re-prompts. If only one agent exists, return it without prompting.

**Acceptance Criteria:**
- Picker renders numbered agent list with default marked
- Empty input returns default agent
- Invalid input re-prompts (does not error)

**Dependencies:** None

### Task 6: Add agent package tests

**Files:**
- Create: `internal/agent/agent_test.go`

**What to Do:**
Test `Launch()` for all three prompt delivery modes using Go's test helper process pattern (`TestHelperProcess`). Test `Resolve()` with various config combinations: flag override, phase default, no agents section, unknown agent error, chooseAgent triggering picker, agents.prompt triggering picker. Test preset merging: claude uses cfg.Claude settings, user overrides merge correctly.

**Acceptance Criteria:**
- Launch tests verify correct command construction for file_ref, prompt_file_arg, and stdin modes
- Resolve tests cover all priority levels and error cases
- Preset merge tests verify claude inherits from cfg.Claude and user overrides take precedence

**Dependencies:** Task 3, Task 4

### Task 7: Add picker tests

**Files:**
- Create: `internal/agent/picker_test.go`

**What to Do:**
Test `Pick()` with string-based I/O. Cover: valid numeric choice, empty input returns default, invalid input handling, single-agent skip.

**Acceptance Criteria:**
- All picker behaviors tested with deterministic string I/O
- No real TTY required

**Dependencies:** Task 5

### Task 8: Wire agent selection into refine command

**Files:**
- Modify: `cmd/gromit/refine.go`

**What to Do:**
Add `--agent` (string) and `--choose-agent` (bool) flags to the refine cobra command. After writing the prompt to the temp file, call `agent.Resolve(cfg, "refine", agentFlag, chooseAgentFlag, os.Stdin, os.Stdout)` to get the agent, then call `agent.Launch(promptPath)`. Remove the inline Claude binary/flags resolution and exec.Command construction. Keep prompt building and post-exit artifact detection unchanged.

**Acceptance Criteria:**
- `--agent <name>` flag selects the specified agent
- `--choose-agent` flag shows the interactive picker
- Default behavior (no flags, no agents config) launches claude identically to current behavior
- Post-exit spec detection still works

**Dependencies:** Task 4, Task 5

### Task 9: Wire agent selection into plan command

**Files:**
- Modify: `cmd/gromit/plan.go`

**What to Do:**
Same treatment as refine: add `--agent` and `--choose-agent` flags, replace inline Claude launch with `agent.Resolve()` + `agent.Launch()`. Keep prompt building and post-exit plan detection unchanged.

**Acceptance Criteria:**
- `--agent <name>` and `--choose-agent` flags work
- Default behavior unchanged
- Post-exit plan file detection still works

**Dependencies:** Task 4, Task 5

### Task 10: Wire agent selection into review command

**Files:**
- Modify: `cmd/gromit/review.go`

**What to Do:**
Same treatment for the **interactive review path only** (`runReviewInteractive`). Add `--agent` and `--choose-agent` flags to the review cobra command. Replace the interactive Claude launch with `agent.Resolve()` + `agent.Launch()`. The non-interactive path (`runReviewNonInteractive`) remains unchanged — it uses `claude.Client` for automated review with structured output. Remove `buildReviewArgs` helper (no longer needed).

**Acceptance Criteria:**
- Interactive review uses agent resolution and launch
- Non-interactive review (`--non-interactive`) is completely unaffected
- `--agent` and `--choose-agent` flags work
- Default behavior unchanged

**Dependencies:** Task 4, Task 5

---

## Notes

- The `debug.go` command also follows the interactive Claude launch pattern and could benefit from agent support, but it's not in the spec's scope. It can be added later with the same pattern.
- The `retro.go` LaunchClaudeCode function has a different invocation pattern (no temp file, hardcoded `"claude"` binary). It's an automated phase and out of scope.
- The `codex` and `gemini` preset details (exact flags, prompt delivery) are best-guess defaults. Users can override via config if the defaults don't match their installed version.
- The `explore` phase doesn't exist yet. The config supports `agents.phases.explore` for future use, but no command wiring is needed now.
