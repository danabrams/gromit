# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-07 | Status File Management | patterns
*Related to: nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2*

Status struct fields require backward-compatible changes (omitempty for new optional fields). Use ReadStatus()/IsProcessAlive() for state + liveness checks. Return nil,nil for missing optional files (not an error). StatusWriter handles both lifecycle states and preserves completed iteration count on shutdown. Round-trip tests verify serialization fidelity. Stale resource cleanup integrates into status reporting via process liveness checks. Process utilities (IsProcessAlive) are co-located with Status in status.go. Test file I/O uses t.TempDir() for isolation.

### 2026-02-07 | Methodology Label Activation | patterns
*Related to: ralph-runner-4a3f, ralph-runner-nzue*

Methodologies use label-based activation ("methodology:true"/"false") with global config fallback via bead.IsMethodologyActive(). When active, replace the build prompt with a specialized RenderXXXBuild method. Check parent labels before adding globally-active methodology labels to sub-beads to avoid duplicates. Order methodology checks carefully for precedence when multiple methodologies are active.

### 2026-02-11 | Prompt Template Structure | conventions
*Related to: gromit-rpne*

Prompt templates in .gromit/templates/ use explicit section headers (##) and preserve exact whitespace/structure when updating. Template files follow a consistent structure: context section at top, then Guidelines, then preserved sections like 'Avoiding Sibling Overlap' and ATDD blocks. When modifying sections, maintain blank lines between sections and ensure downstream blocks remain unchanged. Acceptance tests for template changes must match the exact content being added, including specific phrases and subsection structure.

### 2026-02-16 | Provider Contract Fixtures | patterns
*Related to: gromit-d7j9*

Contract tests consume canonical provider fixtures under test/fixtures/ using scenario-driven naming: `{provider}[_stream]_{outcome}.{format}`. Fixtures (codex_success.txt, codex_failure.txt, codex_stream_success.jsonl, codex_stream_failure.jsonl, claude_stream_success.jsonl) must include brief provenance comments describing the source and refresh workflow. Payloads should be minimal but realistic—Codex plain-text fixtures show output structure (touched/tests lines), JSONL fixtures emit `{"type":"assistant",...}` and `{"type":"result",...}` events. Fixture environment variables (CODEX_FIXTURE, CLAUDE_FIXTURE) point fake CLIs to fixture paths. Test assertions verify output matches canonical payloads, enabling both roundtrip validation and contract evolution tracking. Provenance comments facilitate fixture refresh workflow without manual intervention.

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-20 | gpt-5.3-codex Cost Parity with 5.2 | patterns
*Related to: Codex Cost Opacity (archived), token reporting fix (gromit-h5ktw)*

gpt-5.3-codex and gpt-5.2-codex have comparable per-token pricing (5.3 may be slightly cheaper). Early retro data showing a 3x-9x cost multiplier was based on ~100 iterations with miscalculated cost metrics. With token reporting fixed (82/82 iterations reporting non-zero tokens on Feb 19), actual costs are comparable. Model selection between 5.2 and 5.3 should be based on capability and success rate, not cost.

### 2026-02-19 | Agent Resolver Adapter Duplication | patterns
*Related to: code-review*

Agent resolver adapters (cliAgentResolver, agentResolverAdapter, exploreAgentResolver) are copy-pasted across cmd/gromit files — any interface change requires updating 3+ places.

### 2026-02-19 | Debug Command Model Flag Override | gotchas
*Related to: code-review*

The debug command's --model flag defaults to opus so the model override block always executes for the Claude agent, silently discarding any resolved agent configuration.

### 2026-02-19 | SetDefaults Zero-Value Sentinel Limitation | gotchas
Config SetDefaults() uses `if field == 0` guards for integer fields, which prevents users from intentionally setting zero. This is problematic for fields where zero is meaningful (e.g., PushTimeout where 0 should disable the timeout per PushTimeoutDuration() docs). Fields needing a disable-via-zero semantic should use *int or a sentinel like -1.

### 2026-02-19 | Define Shared Helpers Before Wiring Call Sites | conventions
*Related to: gromit-jmqps*

When adding fallback logic (like title-as-expected-output), define it once as a shared helper and wire all call sites through it from the start. The expected_outputs wiring shows the same pattern implemented 5 different ways across packages (expectedOutputsOrTitle helper in 2 packages, inline in 3 others), creating maintenance burden.

### 2026-02-20 | Runner argv injection propagates through spec orchestration | conventions
*Related to: successful bead*

Threading `r.argvRunnerFn` into `SpecOrchestrator` construction keeps acceptance-test orchestration on the injected runner path instead of default behavior. Ensure all spec orchestrator creation paths copy that runner function so git/test argv commands remain testable and environment-consistent end-to-end.

### 2026-02-20 | Client.run uses RunFn as the only test hook before subprocess | patterns
*Related to: successful bead*

`(*Client).run()` should check only `RunFn` first for override execution; when `RunFn` is nil it must execute the existing subprocess flow (`exec.Command` + `c.Dir` + `Output` + `*exec.ExitError` stderr wrapping) unchanged. This keeps injectable tests isolated while preventing accidental divergence in real `bd` invocation behavior.

---

## Archived

*No longer relevant or superseded.*
