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

### 2026-02-19 | gpt-5.3-codex Structural Cost Multiplier | patterns

gpt-5.3-codex averages $22.77/iteration vs $2.46 for gpt-5.2-codex (9x) despite similar per-token pricing. Top 4 most expensive iterations in the last 30 were all 5.3-codex ($55, $42, $42, $22). Token counts are reported as 0 for all codex iterations, so root cause (prompt size vs output verbosity vs retry loops) cannot be diagnosed. Until token reporting is fixed and the cause identified, prefer gpt-5.2-codex for cost-sensitive routing.

### 2026-02-19 | Codex Token Reporting Must Parse Output Metrics | patterns
*Related to: gpt-5.3-codex Structural Cost Multiplier*

Codex provider runs must parse token usage from the provider output and populate `input_tokens` and `output_tokens` in `iteration_metrics.jsonl`, matching Claude’s token-reporting path. Leaving both values at zero hides root causes for cost anomalies (e.g., 9x cost difference between gpt-5.3-codex and gpt-5.2-codex), so parse and surface token metrics at the same stage Codex output is consumed.

### 2026-02-18 | Documentation Test Enforcement for RULES.md | conventions
Documentation tests in bead_sizing_docs_test.go enforce that RULES.md stays in sync with implemented behavior. Any changes to file sizing rules must update both the code AND the corresponding RULES.md documentation section.

### 2026-02-19 | Agent Resolver Adapter Duplication | patterns
*Related to: code-review*

Agent resolver adapters (cliAgentResolver, agentResolverAdapter, exploreAgentResolver) are copy-pasted across cmd/gromit files — any interface change requires updating 3+ places.

### 2026-02-19 | Go Interface Nil Check Gotcha in Pipeline | gotchas
*Related to: code-review*

The pipeline.validateReviewDeps function uses Go interface nil checks which do not catch typed nil pointers — this is a general Go gotcha worth documenting.

### 2026-02-19 | Source-Text-Reading Test Anti-Pattern | gotchas
*Related to: code-review*

Source-text-reading test pattern (os.ReadFile + strings.Contains on .go files) has become widespread in *_agent_test.go files — these tests are fragile to refactoring and should be replaced before they multiply further.

### 2026-02-19 | Runner Sub-Package Split Quality | patterns
*Related to: code-review*

The runner sub-package split is well-executed: no sub-package imports another sub-package, all production files under 500 lines, facade files under 1000 lines, and type aliases maintain backward compatibility.

### 2026-02-19 | Acceptance Test Line Budget Utilization | patterns
*Related to: code-review*

The acceptance test line budget is at 61.5% utilization (3,688 of 6,000 lines) — healthy headroom.

### 2026-02-19 | Debug Command Model Flag Override | gotchas
*Related to: code-review*

The debug command's --model flag defaults to opus so the model override block always executes for the Claude agent, silently discarding any resolved agent configuration.

### 2026-02-19 | Factory Functions Must Propagate Errors | gotchas

Constructor/factory functions that call other functions which can fail (e.g., backlog.NewFile) must return (*T, error) and propagate errors to callers. Returning nil on error without changing the signature leaves callers unable to detect failure. The createRefinePipeline function returned nil on backlog.NewFile failure, but was called at line 61 without nil checks, causing a panic when dereferencing. Solution: change signature to return error and check it before using the result.

---

## Archived

*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*
