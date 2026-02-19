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

### 2026-02-19 | Prompt Context Budget Experiment Results | patterns
*Related to: experiment*

Experiment "Prompt context budget for sonnet builds" (2026-02-17 to 2026-02-19): Setting Renderer.SetMaxContextChars() to 30K for sonnet-tier builds. After 9 sonnet iterations (target was 30), avg sonnet cost dropped from $1.97 to $1.00, but was confounded by concurrent micro-decomposition improvement. One $4.49 outlier persisted. Success rate unchanged at 88-89%. First-pass rate jump (4% to 89%) was system-wide, not experiment-specific. Inconclusive — the context budget may help but the effect cannot be isolated from decomposition improvements.

### 2026-02-19 | Codex Cost Opacity and Token Reporting Gap | patterns
*Related to: gpt-5.3-codex Structural Cost Multiplier*

gpt-5.3-codex averages $22.77/iteration vs $2.46 for gpt-5.2-codex (9x cost multiplier). Root cause cannot be diagnosed because the codex provider reports 0 for both input_tokens and output_tokens in iteration_metrics.jsonl. The codex output parser must extract and populate token usage metrics, matching Claude's token-reporting path via `{"type":"result"}` events. Until token reporting is fixed and the cost differential explained, prefer gpt-5.2-codex for cost-sensitive routing.

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

### 2026-02-19 | normalizeNilFields Single Responsibility | conventions
*Related to: gromit-176m0*
normalizeNilFields() should remain a pure nil-to-empty-slice converter. Data transformations between fields (like AcceptanceCriteria to ExpectedOutputs mapping) belong in a separate resolution step to preserve single-responsibility and avoid surprising side effects during normalization.

---

## Archived

*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
