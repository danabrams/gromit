# TDD Fresh Context Benchmark (gpt-5.1-codex-mini, bug-fixed)

Date: 2026-02-25
Manifest: `tdd-fresh-only-mini`
Model: gpt-5.1-codex-mini (all tiers)
Pricing: $0.25/M input, $1.50/M output (but API-reported cost used below, which includes reasoning token overhead)

## Context

Previous fresh-context run (2026-02-24) scored 2/5 due to two bugs:
1. `BuildTierDefault` hardcoded to "medium" instead of "low"
2. Red phase timeout too short (3m) for mini models

Both fixed in commit `3d8dbf0e`. This run validates the fixes.

## Per-Bead Results

| Bead | Complexity | Success | Duration | Input Tokens | Output Tokens | API Cost |
|------|-----------|---------|----------|-------------|--------------|----------|
| gromit-7rj2f | low | PASS | 28m19s | 3,105,305 | 80,320 | $6.56 |
| gromit-980iu | low | PASS | 6m32s | 3,225,684 | 38,677 | $6.19 |
| gromit-dh34r | medium | PASS | 5m04s | 2,214,374 | 28,262 | $4.27 |
| gromit-cflw8 | high | PASS | 11m31s | 5,410,610 | 68,085 | $10.42 |
| gromit-50ggy | high | FAIL (timeout) | 6m+ | 0 | 0 | $0.00 |
| **Total** | | **4/5** | **69m33s** | **13,955,973** | **215,344** | **$27.44** |

Quality: 0.80 average (1.0 on all passing beads, 0.0 on timeout)
First-pass rate: 0.80

## Mode Comparison (all gpt-5.1-codex-mini)

| Mode | Success | Elapsed | Total Cost | Quality | Cost/Successful Bead |
|------|---------|---------|-----------|---------|---------------------|
| tdd_shared_context | 5/5 | 19m37s | $2.06 | 1.00 | $0.41 |
| single_pass | 5/5 | 22m30s | $2.69 | 1.00 | $0.54 |
| tdd_fresh_context (buggy) | 2/5 | 42m13s | $1.46 | 0.40 | $0.73 |
| tdd_fresh_context (fixed) | 4/5 | 69m33s | $27.44 | 0.80 | $6.86 |

## Cost Discrepancy Analysis

At published mini pricing ($0.25/M in, $1.50/M out), 14M input + 215K output = $3.81.
API-reported cost: $27.44. The 7x gap is likely reasoning token overhead from gpt-5.1-codex-mini's
internal chain-of-thought, which is billed but not exposed in input/output token counts.

## Root Cause: Why Fresh Context Is 13x More Expensive

### 1. Context bloat across TDD cycles (PRIMARY)

`AssembleRedHandoff()` in `internal/runner/tdd/assembly.go` reads complete test and impl file
contents via `readFiles()` and passes them to templates verbatim. Each cycle re-sends everything:

- gromit-980iu cycle 1 red: 7.5KB prompt
- gromit-980iu cycle 3 red: 78KB prompt (full bead_test.go with 50+ test cases)
- gromit-cflw8 cycle 3 red: 65KB prompt

### 2. Codex agentic execution per phase

Each TDD phase invocation is a full agentic session (file reads, command execution, git ops).
Output tokens (38K-80K per bead) reflect tool-call overhead, not just code generation.

### 3. Multiple red/green cycles

MaxTDDCycles defaults to 10. Even successful beads went through 3-4 cycles with ballooning prompts.

## Prompt Dump Observations

Green phase prompts ARE being generated (bug fix confirmed):
- `/tmp/gromit-green-prompt-gromit-7rj2f-1771981211843.md` (26KB)
- Proper TDD discipline instructions present
- Test/impl file separation correct

Red phase prompts grow unbounded:
- Cycle 1: ~7.5KB (base prompt + bead description)
- Cycle 2+: 18-78KB (accumulated file contents)

## Recommendations

1. **Use tdd_shared_context as default mode** -- $0.41/bead, 100% success, fastest
2. **Default all beads to low tier** -- mini handles all tested complexities, escalation handles failures
3. **Fix fresh context prompt bloat before re-benchmarking** -- truncate file contents in red handoff, add cycle-aware summarization (backlog item filed)

## Data Files

- Results JSON: `.gromit/benchmarks/results/tdd-fresh-only-mini/20260225T015913Z.json`
- Per-bead logs: `.gromit/benchmarks/logs/tdd_fresh_context.jsonl`
- Prompt dumps: `/tmp/gromit-{red,green}-prompt-gromit-*-177198*.md`
- Previous mini results: `.gromit/reports/benchmark-cost-efficiency-all-low-2026-02-25.md`
