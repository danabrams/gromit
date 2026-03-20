# Spec 0005a — Counterexample Scenario Synthesis

## spec_id
counterexample-scenario-synthesis

## Depends on
None (establishes pipeline stage ordering used by 0005b, 0005c)

## Vision
The scenario surface today is only as strong as the spec author's foresight. Many bugs live in the missing examples — boundaries, negations, error paths, and stale-state edges that the original scenarios never mentioned. Counterexample synthesis turns disciplined expansion of the existing scenarios into a pipeline stage, so the implementation must satisfy harder cases from the start rather than passing on optimistic happy-path coverage alone.

## Summary
A new configurable pipeline stage, SynthesizeCounterexamples, runs after WriteContracts and before Execute. It reads the spec's existing scenarios and acceptance criteria, then uses an LLM to synthesize counterexample scenarios across boundary, negation, error-path, state-edge, and persistence categories. Synthesized scenarios are tagged `origin: synthesized`, subject to a configurable per-scenario cap, and flow through the existing contract and test infrastructure with equal weight.

## Goals
### Primary
- Automatically expand the scenario surface with edge cases the spec author didn't write
- Integrate seamlessly with existing contract verification and scenario test writing stages
- Keep cost predictable via per-scenario cap and configurable model tier

### Secondary
- Provide traceability so reviewers can distinguish authored vs. synthesized scenarios
- Make the stage easy to disable for simple specs where it adds no value

## Non-goals
- Open-ended test generation unrelated to existing scenarios (out of scope)
- Mutation testing or code-level coverage analysis (deferred per vision triage)
- Modifying the behavior of existing WriteContracts or WriteScenarioTests stages — this stage feeds into them, not replaces them

## Architecture

### Pipeline Integration

```
Existing flow:
  Plan → WriteContracts → Execute → WriteScenarioTests → Validate → Review → Accept

New flow:
  Plan → WriteContracts → SynthesizeCounterexamples → Execute → WriteScenarioTests → Validate → Review → Accept
```

> **Note:** This diagram is simplified and omits the init, compile, finalize, and evidence stages.

### New Stage: SynthesizeCounterexamples

Lives in `internal/next/specloop/stages/synthesize_counterexamples.go`. Implements the existing `Stage` interface. `Name()` returns `"synthesize_counterexamples"`.

```go
type SynthesizedScenario struct {
    Name             string   // short descriptive name
    Description      string   // what this scenario tests
    Steps            []string // given/when/then steps
    ExpectedBehavior string   // what correct behavior looks like
    Origin           string   // always "synthesized"
    SourceScenario   string   // name of the authored scenario it was derived from
    Category         string   // "boundary" | "negation" | "error_path" | "state_edge" | "persistence"
}
```

### LLM Prompt Structure
- Receives: original scenarios, acceptance criteria, spec summary
- Instructed to consider 5 categories per scenario: boundary, negation, error_path, state_edge, persistence
- Must output structured scenarios in the same format as authored ones
- Capped at N per source scenario (default 3)

### Configuration

```yaml
counterexamples:
  enabled: true          # default: true
  max_per_scenario: 3    # default: 3
  model_tier: medium     # default: medium (resolves to project's medium-tier model)
```

### Integration Points
- Synthesized scenarios are written to the evidence artifacts (`scenario-contracts.yaml`, `scenario-test-manifest.json`) with `origin: synthesized` tag and passed to downstream stages via those files
- WriteScenarioTests sees them identically to authored scenarios
- Contract verification treats them identically
- Evidence bundler records them in `scenario-contracts.yaml` with origin metadata
- Review facets can see the tag but don't treat them differently

### Key Decisions
The stage does NOT modify the spec file. Counterexamples exist only in the run context and evidence artifacts. The spec remains the author's source of truth.

## Acceptance Criteria

1. When `counterexamples.enabled` is true (default), the SynthesizeCounterexamples stage runs after WriteContracts and before Execute
2. When `counterexamples.enabled` is false, the stage is skipped and the pipeline behaves identically to today
3. The stage reads all existing scenarios from the run context and produces counterexample scenarios for each
4. Each synthesized scenario includes `origin: synthesized`, the source scenario name, and a category tag
5. The number of synthesized scenarios per source scenario does not exceed `max_per_scenario` (default 3)
6. Synthesized scenarios flow through contract verification identically to authored scenarios
7. Synthesized scenarios flow through WriteScenarioTests identically to authored scenarios
8. A counterexample contract failure triggers replan, same as any other contract failure
9. The model tier used for synthesis is configurable and defaults to the project's medium tier
10. Synthesized scenarios appear in evidence artifacts (`scenario-contracts.yaml`, `scenario-test-manifest.json`) with their origin metadata
11. The spec file on disk is never modified by this stage
12. All existing tests continue to pass

## Scenarios

### Scenario: Happy path — counterexamples synthesized and verified
**Given:** A spec with 3 authored scenarios and default config (`enabled: true`, `max_per_scenario: 3`)
**When:** The pipeline runs through SynthesizeCounterexamples
**Then:** Up to 9 synthesized scenarios are appended to the run context, each tagged `origin: synthesized` with source scenario name and category. The pipeline continues to Execute with the expanded scenario list.

### Scenario: Counterexample contract failure triggers replan
**Given:** A spec with 2 authored scenarios, synthesis produces 4 counterexamples, one of which describes behavior the implementation doesn't handle
**When:** The implementation runs and contract verification evaluates all 6 scenarios
**Then:** The failing counterexample contract triggers a replan with failure context, identical to how an authored contract failure would behave

### Scenario: Stage disabled via config
**Given:** A spec with `counterexamples.enabled: false`
**When:** The pipeline reaches the SynthesizeCounterexamples stage
**Then:** The stage returns immediately with no synthesized scenarios, no LLM invocation, and no change to the run context

### Scenario: Per-scenario cap respected
**Given:** A spec with 1 authored scenario and `max_per_scenario: 2`
**When:** The LLM is prompted to synthesize counterexamples for that scenario
**Then:** The LLM prompt instructs it to return exactly 2 counterexamples (the cap is passed in the prompt, so the LLM produces at most N rather than generating extras to be filtered)

### Scenario: LLM invocation failure degrades gracefully
**Given:** A spec with 2 authored scenarios and default config
**When:** The LLM invocation fails due to a network error or timeout during SynthesizeCounterexamples
**Then:** The stage logs the error, produces zero synthesized scenarios, and the pipeline continues to Execute with only the authored scenarios (graceful degradation, not a blocking failure)

### Scenario: LLM returns malformed output
**Given:** A spec with 2 authored scenarios and default config
**When:** The LLM returns output that cannot be parsed into valid SynthesizedScenario structs
**Then:** The stage logs a warning with the parse error, produces zero synthesized scenarios, and the pipeline continues to Execute with only the authored scenarios

### Scenario: Evidence artifacts include origin metadata
**Given:** A completed run with 2 authored and 4 synthesized scenarios
**When:** Evidence is bundled at finalization
**Then:** `scenario-contracts.yaml` and `scenario-test-manifest.json` include all 6 scenarios, with synthesized ones carrying `origin: synthesized`, `source_scenario`, and `category` fields

### Scenario: WriteScenarioTests treats synthesized scenarios identically to authored ones
**Given:** A completed SynthesizeCounterexamples stage that produced 3 synthesized scenarios alongside 2 authored scenarios
**When:** The WriteScenarioTests stage runs
**Then:** Tests are written for all 5 scenarios using the same test-writing logic; synthesized scenarios receive scenario tests identical in structure to authored ones

### Scenario: Model tier configuration is respected
**Given:** A project config with `counterexamples.model_tier: low` and a spec with 2 authored scenarios
**When:** The SynthesizeCounterexamples stage invokes the LLM
**Then:** The LLM invocation uses the project's low-tier model, not the default medium tier

### Scenario: Spec file on disk is not modified
**Given:** A spec file with 3 authored scenarios and default config
**When:** The SynthesizeCounterexamples stage runs and produces synthesized scenarios
**Then:** The spec file on disk has identical content (byte-for-byte) before and after the stage runs; all synthesized scenarios exist only in evidence artifacts

## Validation

```
go test ./internal/next/specloop/stages/...
go test ./internal/next/...
go vet ./...
```
