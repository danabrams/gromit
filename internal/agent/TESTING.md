# Agent Package Test Coverage

## Test Files

- **agent_test.go**: Tests for Agent interface, cliAgent implementation, and Launch functionality
- **resolve_test.go**: Tests for Resolve() function and agent resolution logic
- **picker_test.go**: Tests for interactive agent picker

## Resolve() Function Test Coverage

All acceptance criteria for the Resolve() function are covered by tests in `resolve_test.go`:

### Priority Resolution Tests

✅ **Flag override has highest priority** (`TestResolveWithPriorityFlagOverride`)
- Flag override beats phase config
- Flag override beats default
- Flag override with custom agent
- Flag override with unknown agent returns error

✅ **Phase config used when no flag override** (`TestResolveWithPhaseConfigPriority`)
- Refine phase uses configured agent
- Plan phase uses configured agent
- Review phase uses configured agent
- Explore phase uses configured agent
- Phase config uses custom agent definition

✅ **Defaults to "claude" when no config** (`TestResolveDefaultsToClaudeWhenNoConfig`)
- All phases default to claude
- Nil config defaults to claude

✅ **Complete priority chain** (`TestResolveCompletePriorityChain`)
- Verifies: flag > phase > default

### Error Handling Tests

✅ **Unknown agent returns error** (`TestResolveUnknownAgent`)
✅ **Empty agent name returns error** (`TestResolveEmptyAgentName`)
✅ **Invalid phase config agent** (`TestResolveInvalidPhaseConfigAgent`)

### Picker Tests

✅ **chooseAgent triggers picker** (`TestResolveWithPickerChoosesAgent`)
- Multiple choice scenarios
- Invalid choices return error
- Picker output verification

✅ **Picker shows default marker** (`TestResolvePickerShowsDefaultMarker`)

### Preset Merging Tests

✅ **Claude preset inherits from cfg.Claude** (`TestResolveClaudePresetUsesClaudeConfig`)
- Uses Claude.Binary
- Uses Claude.Flags
- Uses FileRef delivery

✅ **Custom definitions override presets** (`TestResolveCustomDefinitionOverridesPreset`)
- Custom claude overrides built-in
- Custom codex overrides built-in

✅ **Built-in presets exist** (`TestResolveBuiltInPresetsExist`)
- Claude, Codex, Gemini presets

## Missing Test Coverage

❌ **agents.prompt config field** - NOT IMPLEMENTED YET

The spec mentions `agents.prompt: true` config option that should automatically trigger the picker on every interactive phase. This feature is not yet implemented:

**Required implementation:**
1. Add `Prompt bool` field to `config.AgentsConfig` struct
2. Update `Resolve()` to check `cfg.Agents.Prompt`
3. Priority: `flag override > chooseAgent || cfg.Agents.Prompt > phase config > default`

**Required tests:**
- `agents.prompt: true` triggers picker (without chooseAgent flag)
- Flag override beats `agents.prompt: true`
- `agents.prompt: false` uses normal resolution
- chooseAgent has same priority as agents.prompt

## Test Organization

Tests follow Go conventions:
- Source file `xxx.go` has tests in `xxx_test.go`
- `agent.go` → `agent_test.go`
- `resolve.go` → `resolve_test.go`
- `picker.go` → `picker_test.go` (when it exists)

All tests use table-driven test patterns where appropriate and follow the project's testing conventions.
