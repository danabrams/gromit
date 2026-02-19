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

### 2026-02-19 | Codex Cost Opacity and Token Reporting Gap | patterns
*Related to: gpt-5.3-codex Structural Cost Multiplier*

gpt-5.3-codex averages $22.77/iteration vs $2.46 for gpt-5.2-codex (9x cost multiplier). Root cause cannot be diagnosed because the codex provider reports 0 for both input_tokens and output_tokens in iteration_metrics.jsonl. The codex output parser must extract and populate token usage metrics, matching Claude's token-reporting path via `{"type":"result"}` events. Until token reporting is fixed and the cost differential explained, prefer gpt-5.2-codex for cost-sensitive routing.

### 2026-02-19 | Runner Sub-Package Isolation via runtypes | architecture
*Related to: code-review*
*Promoted to RULES.md Architecture section.*

The runner sub-package split maintains clean isolation: no sub-package imports another sub-package, all production files under 500 lines, facade files under 1000 lines. All cross-cutting types live in runtypes/, which serves as the dependency-inversion boundary. Type aliases in the parent runner package maintain backward compatibility.

### 2026-02-19 | Agent Resolver Adapter Duplication | patterns
*Related to: code-review*

Agent resolver adapters (cliAgentResolver, agentResolverAdapter, exploreAgentResolver) are copy-pasted across cmd/gromit files — any interface change requires updating 3+ places.

### 2026-02-19 | Source-Text-Reading Test Anti-Pattern | gotchas
*Related to: code-review*

Source-text-reading test pattern (os.ReadFile + strings.Contains on .go files) has become widespread in *_agent_test.go files — these tests are fragile to refactoring and should be replaced before they multiply further.

### 2026-02-19 | Temp File Pattern for CLI Prompts | conventions
Interactive commands must write large prompts to temp files before passing to Claude CLI to avoid OS ARG_MAX limits. debug.go correctly follows this pattern (writes to .gromit/tmp/); retro.go does not and passes prompt text directly as a CLI argument. New interactive commands should follow the debug.go pattern.

### 2026-02-19 | Debug Command Model Flag Override | gotchas
*Related to: code-review*

The debug command's --model flag defaults to opus so the model override block always executes for the Claude agent, silently discarding any resolved agent configuration.

### 2026-02-19 | SetDefaults Zero-Value Sentinel Limitation | gotchas
Config SetDefaults() uses `if field == 0` guards for integer fields, which prevents users from intentionally setting zero. This is problematic for fields where zero is meaningful (e.g., PushTimeout where 0 should disable the timeout per PushTimeoutDuration() docs). Fields needing a disable-via-zero semantic should use *int or a sentinel like -1.

---

## Archived

*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
