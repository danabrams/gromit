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

### 2026-02-12 | Runner and Provider Invocation Pattern | patterns
*Related to: gromit-5pvp, gromit-82qx, gromit-vabo, gromit-juyb, gromit-2zju, gromit-557p, gromit-3gdz, gromit-gibz, gromit-sx84*

Runner methods use feature-flag gating (e.g., IsAutoPushEnabled()) and context.WithTimeout for subprocess calls. Failure handling uses mode field (warn vs stop) for non-critical operations. Follow the executeClaudeInvocation pattern: extract tier selection, use provider.StreamRun(), handle UsageLimitError by escalating to the next tier via TierToModel mapping. Capture the selected provider's Name() immediately after selection for use in dependent operations. Standard tiers: validation uses phase="validate" tier="low"; reviews use selectTier(bead) or "high" for opus builds/thorough reviews. Use selectReviewTier() helper which delegates to selectTier() for non-opus and returns "high" for opus builds.

*Note: Run() sequencing invariant, nil-safe checks, router/tier selection, escalation ownership, and skippedBeads pattern promoted to RULES.md.*

### 2026-02-11 | Prompt Template Structure | conventions
*Related to: gromit-rpne*

Prompt templates in .gromit/templates/ use explicit section headers (##) and preserve exact whitespace/structure when updating. Template files follow a consistent structure: context section at top, then Guidelines, then preserved sections like 'Avoiding Sibling Overlap' and ATDD blocks. When modifying sections, maintain blank lines between sections and ensure downstream blocks remain unchanged. Acceptance tests for template changes must match the exact content being added, including specific phrases and subsection structure.

### 2026-02-16 | Provider Contract Fixtures | patterns
*Related to: gromit-d7j9*

Contract tests consume canonical provider fixtures under test/fixtures/ using scenario-driven naming: `{provider}[_stream]_{outcome}.{format}`. Fixtures (codex_success.txt, codex_failure.txt, codex_stream_success.jsonl, codex_stream_failure.jsonl, claude_stream_success.jsonl) must include brief provenance comments describing the source and refresh workflow. Payloads should be minimal but realistic—Codex plain-text fixtures show output structure (touched/tests lines), JSONL fixtures emit `{"type":"assistant",...}` and `{"type":"result",...}` events. Fixture environment variables (CODEX_FIXTURE, CLAUDE_FIXTURE) point fake CLIs to fixture paths. Test assertions verify output matches canonical payloads, enabling both roundtrip validation and contract evolution tracking. Provenance comments facilitate fixture refresh workflow without manual intervention.

---

## Provisional

*Seen once - may be specific to one task.*

*No provisional learnings.*

---

## Archived

*Moved to LEARNINGS_ARCHIVE.md to reduce prompt context overhead.*

