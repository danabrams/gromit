# Tier Demotion Experiment — 2026-02-20

## Hypothesis

Mechanical and trivial beads succeed on haiku at >=80% first-pass rate, with >=40% cost reduction versus the medium-tier baseline.

## Baseline (snapshot at experiment start)

| Metric | Value | Source |
|--------|-------|--------|
| Avg cost per bead | $1.72 | experiment.json baseline |
| Avg cost (30-window) | $1.75 | process_trend.json |
| Avg duration (30-window) | 313,010 ms | process_trend.json |
| First-pass success rate (30-window) | 93.3% | process_trend.json |
| Escalation rate (30-window) | 0% | process_trend.json |
| Failure rate (30-window) | 6.7% | process_trend.json |
| Timeout failure rate (30-window) | 3.3% | process_trend.json |
| Total iterations at start | 605 | process_trend.json |

## What changed

26 beads demoted. All tagged with notes containing `tier-demotion-2026-02-20`.

### P0 → P4 (haiku) — 8 beads

| Bead | Title | Category |
|------|-------|----------|
| gromit-ywkv | Delete explore_pipeline_adapter_test.go | Trivial: file deletion |
| gromit-y5ee | Document lane commands and fixture refresh workflow | Trivial: documentation |
| gromit-hw51 | Update acceptance template for shaped context | Trivial: single template edit |
| gromit-ddcq | Final verification and stability sweep | Mechanical: verification + docs |
| gromit-el7d | Add lane-aware timing and runtime budget visibility | Mechanical: bash scripts |
| gromit-n1fp | Add ATDD rules subset extraction | Mechanical: text filtering |
| gromit-xed7 | Add large-context budget regression coverage | Mechanical: test with fixtures |
| gromit-y1mv | Validate optimization runtime and correctness | Mechanical: timing validation |

### P0 → P1 (sonnet) — 3 beads

| Bead | Title | Category |
|------|-------|----------|
| gromit-4i8u | Add real-CLI smoke lane behind build tag and env gates | Moderate: test infra |
| gromit-l6wv | Wire ATDD shaping into acceptance rendering | Moderate: callbacks + observability |
| gromit-rmv6 | Migrate Codex acceptance/contract tests to canonical fake | Moderate: multi-file migration |

### P1 → P4 (haiku) — 15 beads

| Bead | Title | Category | TDD? |
|------|-------|----------|------|
| gromit-mekuu | Migrate codex_preflight.go to runArgv | Mechanical: find-replace | no |
| gromit-1fuo5 | Migrate spec_orchestrator.go to runArgv | Mechanical: find-replace | no |
| gromit-uag2b | Migrate lifecycle.go to runArgv | Mechanical: find-replace | no |
| gromit-odihv | Wire ArgvRunner into Deps and SpecOrchestrator | Mechanical: field threading | no |
| gromit-y2p5f | Add runArgv method and argvRunnerFn field | Mechanical: field + method | no |
| gromit-8edk.1 | Refactor execute fn to take escalation flag | Mechanical: signature change | yes |
| gromit-8edk.2 | Wire escalation value at all invocation sites | Mechanical: parameter threading | yes |
| gromit-f4q0 | Add compile-time interface checks | Mechanical: var _ I = (*T)(nil) | no |
| gromit-zhn6i | Update subprocess audit test for dual paths | Test-only: extend test cases | no |
| gromit-rkpn | Validate suite-wide test runtime <5s | Test-only: threshold assertion | no |
| gromit-sirx | Add t.Parallel() to top 5 packages | Test-only: boilerplate | no |
| gromit-9dtn | Add CircuitBreakerConfig to config and gromit.yaml | Mechanical: config struct | no |
| gromit-wnxk | Wire validation duration into IterationResult | Mechanical: field plumbing | no |
| gromit-51t64 | Report coverage results to IterationResult | Mechanical: field plumbing | yes |
| gromit-teea | Remove stray scratch/debug markdown files | Trivial: file cleanup | no |

### Kept at original tier (not changed)

**P0 (opus) — 3 beads:** gromit-5t4s (ATDD context shaper), gromit-10kg (fake Codex harness, in progress), gromit-pskg (Andon escalation, in progress)

**P1 (sonnet) — remaining ~23 beads:** Spec-level ATDD orchestration, CoverageTracker state machine, CircuitBreaker algorithm, token budget policy, shared-state audit, debug runbook wiring, etc.

## How to compare

After these beads have run (or a meaningful subset), compare:

### 1. Per-bead cost for demoted beads

```bash
# Find iteration logs for demoted beads and compare cost
grep -E "gromit-(ywkv|y5ee|hw51|ddcq|el7d|n1fp|xed7|y1mv|mekuu|1fuo5|uag2b|odihv|y2p5f|8edk|f4q0|zhn6i|rkpn|sirx|9dtn|wnxk|51t64|teea)" .gromit/metrics/iteration_metrics.jsonl
```

**Target:** Avg cost <$0.50 per bead (vs $1.72 baseline) for P4 beads.

### 2. First-pass success rate for demoted beads

**Target:** >=80% first-pass success rate on haiku for mechanical beads. Compare against 93.3% baseline (which was mostly medium-tier).

### 3. Escalation frequency

How often did haiku fail and escalate to sonnet/opus? Some escalation is expected and acceptable — the question is whether the net cost (haiku attempt + escalation) is still cheaper than always using sonnet.

**Target:** Escalation rate <25%. Net cost including escalations still <70% of baseline.

### 4. Quality signal

Did any demoted beads produce code that passed validation but was caught in review as low quality? Check review feedback for demoted bead IDs.

### 5. TDD-specific check

The 3 TDD beads (gromit-8edk.1, gromit-8edk.2, gromit-51t64) are the most interesting. Did haiku handle red-green-refactor for mechanical work? Compare their success rate and cycle count against TDD beads that stayed at sonnet.

## Decision criteria

| Outcome | Action |
|---------|--------|
| >=80% success, >=40% cost reduction | Confirm heuristic, write complexity-based-routing spec into code |
| 60-80% success or 20-40% cost reduction | Tighten heuristics, keep only trivial beads on haiku |
| <60% success or <20% cost reduction | Revert demotions, haiku not viable beyond test-only beads |

## Related artifacts

- Spec: `.gromit/specs/complexity-based-routing.md`
- Existing experiment: `experiment.json` (low-tier routing for test-only beads)
- Process trend: `.gromit/metrics/process_trend.json`
- Iteration logs: `.gromit/metrics/iteration_metrics.jsonl`
