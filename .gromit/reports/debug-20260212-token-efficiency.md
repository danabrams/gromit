# Token/Time Efficiency Investigation

**Date:** 2026-02-12
**Data source:** 210 iterations across 46 run logs, 156 unique beads
**Total time:** 35.3 hours | **Total cost:** $130.91

---

## Executive Summary

The biggest efficiency wins are structural, not prompt-level. Validation through Claude CLI (instead of direct shell execution) wastes 100+ Claude invocations. The ATDD lifecycle fires up to 11 Claude invocations per bead. Validation retries alone account for 23% of total cost.

---

## Ranked Savings Opportunities

### 1. Run validation commands as shell commands, not Claude invocations

**Savings: ~$10-15, ~3-5 hours**

**Evidence:** `RunValidation` in `internal/claude/claude.go:158` spawns a full Claude CLI process to execute `go test ./...`, `go vet ./...`, `go build ./...`. The prompt is just "run these commands and report VALIDATION_PASSED or VALIDATION_FAILED." Claude starts up (~5-10s), reads the prompt, then runs the exact same shell commands that could run directly.

Validation fires at:
- After every successful build (84 times)
- During ATDD verify-tests-fail phase (before implementation)
- After refactoring (ATDD/TDD beads)
- After review fixes
- During validation retry loops (21 times with `validation_retried: true`)

Conservative estimate: 120+ Claude invocations just to run shell commands. Each takes 60-180s through Claude vs 10-20s direct.

**Fix:** Run `go test ./...`, `go vet ./...`, `go build ./...` directly via `exec.Command`. Parse exit codes. If any command fails, capture stderr. No Claude needed. Only fall back to Claude for interpreting complex failures (the existing failure analysis path already handles this).

---

### 2. Validation retry loop is the single biggest cost amplifier

**Savings: ~$15-20 (currently $30.12 = 23% of total cost)**

**Evidence:** 21 iterations (10%) had `validation_retried: true`. When validation fails, the build model (usually sonnet at $0.40-1.40/invocation) is re-invoked to fix it, then validation runs again. The most expensive single iteration was $5.34 (gromit-mz3m, validation retry with 122 tool calls).

Top offenders:
- gromit-mz3m: 3 failed iterations, $10.21 total, all validation-retried
- gromit-6tyj: 1 failed iteration, $3.79, validation-retried
- gromit-66dz: $3.22, validation-retried

**Fix:** Two-part fix:
1. With direct validation (item #1), failures surface in <20s instead of 60-180s
2. For common validation failures (missing imports, unused variables, formatting), run `go fmt` and `goimports` automatically before re-invoking Claude. Many validation fixes are trivial — a `go vet` failure from an unused import doesn't need sonnet

---

### 3. ATDD lifecycle has too many Claude invocations (up to 11 per bead)

**Savings: ~5-8 minutes per ATDD bead (with fix #1 applied)**

**Evidence:** With `atdd: true` and `tdd: true` in gromit.yaml, a successful bead lifecycle is:

| Phase | Model | ~Duration | Notes |
|---|---|---|---|
| 1. Precheck | haiku | 23s | Checks if criteria met |
| 2. Acceptance tests | build model | 300-800s | Writes tests |
| 3. Verify tests fail | **haiku via Claude** | 60-120s | Runs go test — wasteful |
| 4. Build | build model | 300-800s | Implements code |
| 5. Validation | **haiku via Claude** | 60-120s | Runs go test — wasteful |
| 6. (if val fails) Fix + re-validate | build model + haiku | 300-600s | Re-invokes build model |
| 7. Refactor | build model | 200-600s | Often a no-op |
| 8. Re-validate after refactor | **haiku via Claude** | 60-120s | Runs go test — wasteful |
| 9. Success learning | haiku | 30s | Extracts learning |
| 10. Review | review model | 120-300s | Reviews diff |

Steps 3, 5, 8 are pure waste (Claude running shell commands). Step 7 is often a no-op. Step 9 produces mostly noise.

**Fix:** With direct validation (fix #1), steps 3, 5, 8 drop from 60-120s to 10-20s each. That's 150-300s saved per bead.

---

### 4. ATDD false positives waste ~13 iterations (~$5-10, ~2.7 hours)

**Savings: ~$5-10, ~2.7 hours**

**Evidence:** 13 iterations (6.2%) failed with "acceptance tests passed before implementation." This means ATDD wrote tests that didn't actually test new behavior. The recovery loop retries up to 3 times (with diff analysis), each costing a full Claude invocation.

Worst case: gromit-davm had 3 iterations of ATDD false positives before being auto-closed by precheck.

Error from logs:
```
"error": "acceptance tests passed before implementation after retry - tests may not be covering new behavior"
```

**Fix:**
- Add a `--skip-atdd` label override for beads that modify existing code (vs. adding new code)
- Track ATDD false positive rate per bead type and auto-disable for categories that consistently false-positive
- Consider making the verify-tests-fail step use direct shell execution (fix #1) to at least make the false positive detection faster

---

### 5. Refactor phase adds 2+ invocations per ATDD/TDD bead for marginal value

**Savings: ~3-5 minutes per bead, ~$2-5 total**

**Evidence:** Every ATDD/TDD bead runs a refactor phase (`process.go:1008-1065`). This spawns a build-tier Claude invocation to "improve code quality without changing behavior," then re-validates. In practice:
- Many refactors produce no changes (Claude says "code is already clean")
- When refactors do change code, they sometimes break validation (triggering revert + retry — `handleRefactorValidationFailure`)
- The refactor prompt includes the full CLAUDE.MD + RULES.MD + learnings boilerplate (~5,000 tokens)

**Fix:** Make refactor conditional:
- Skip refactor for beads that touched ≤2 files (low complexity)
- Skip refactor for haiku-tier beads (already simple tasks)
- Add a config flag `refactor: { enabled: true, min_files_changed: 3 }` to gate it

---

### 6. Success learning extraction produces mostly archived noise

**Savings: ~30s per bead, ~$1-2 total**

**Evidence:** LEARNINGS.md has 990 lines. The Archived section (lines 66-990) contains ~130 entries, of which ~90% were archived as "generic engineering advice" during retro. The LLM filter (learnings.NewLLMFilter) now helps, but each extraction still spawns a Claude haiku invocation.

Currently active learnings (Confirmed + Provisional): ~11 entries worth keeping.
Archived entries: ~130 entries — 92% noise rate.

**Fix:**
- Reduce extraction frequency: only extract learnings when a bead fails or when a bead touches a new package (novel territory)
- Skip extraction for haiku-tier beads (simple tasks rarely produce novel insights)
- The LLM filter is the right approach long-term; just reduce invocation frequency

---

### 7. Prompt context repeated 3x per ATDD bead (~15,000 tokens of boilerplate)

**Savings: ~15,000 input tokens per ATDD bead**

**Evidence:** Three prompts include the full boilerplate (CLAUDE.MD ~100 lines + RULES.MD ~52 lines + confirmed learnings ~7 entries + recent learnings):
- `PROMPT_acceptance_tests.md`
- `PROMPT_atdd_build.md` (or `PROMPT_tdd_build.md`)
- `PROMPT_refactor.md`

Each boilerplate block is ~5,000 tokens. That's ~15,000 tokens repeated per ATDD bead.

Templates that are lean (no CLAUDE.MD/learnings): validate, analyze, learn, scope, precheck, decompose. These are well-designed.

**Fix:**
- Consider whether RULES.MD is sufficient for acceptance tests and refactor phases (skip CLAUDE.MD for these)
- The CLAUDE.MD content is mostly architecture documentation — useful for the build phase but not for refactoring or writing tests
- Learnings could be filtered by relevance to the specific bead (e.g., only show learnings from the same package)

---

### 8. Precheck is efficient — keep as-is

**Evidence:** 44 precheck skips at avg 23s each = 1,004s total. Each precheck that passes saves a full build iteration (avg 789s). Net savings: 44 × 789s - 1,004s = ~33,700s saved = ~9.4 hours.

**Verdict:** Precheck is the most cost-effective phase. No changes needed.

---

## Summary Table

| # | Opportunity | Time Saved | Cost Saved | Difficulty |
|---|---|---|---|---|
| 1 | Direct shell validation | 3-5h | $10-15 | Medium |
| 2 | Smarter validation retry | 2-3h | $15-20 | Medium |
| 3 | Reduce ATDD invocations | 2-3h | $5-8 | Easy (with #1) |
| 4 | Fix ATDD false positives | 2.7h | $5-10 | Hard |
| 5 | Conditional refactor phase | 1-2h | $2-5 | Easy |
| 6 | Reduce learning extraction | 0.5h | $1-2 | Easy |
| 7 | Trim prompt boilerplate | — | ~$2-3 | Easy |

**Total potential savings: ~$40-63 (30-48% of current cost), ~12-19 hours (34-54% of current time)**

---

## Appendix: Data

### Key Metrics
- Success rate: 71.0% (149/210)
- Precheck skip rate: 21.0% (44/210) — efficient
- ATDD false positive rate: 6.2% (13/210) — wasteful
- Validation retry rate: 10.0% (21/210) — expensive
- Timeout rate: 7.1% (15/210) — invocation timeouts
- Average successful build: 789s (13.2 min)
- Average failed build: 756s (12.6 min) — fails are nearly as expensive as successes
- Failed build total time: 46,086s (12.8h) — 36% of total time for 0% output

### Cost Distribution
- Total: $130.91
- Successful iterations: $92.82 (70.9%)
- Failed iterations: $38.09 (29.1%)
- Validation-retried iterations: $30.12 (23.0%)

### Top 5 Most Expensive Single Iterations
1. gromit-mz3m: $5.34 (failed, validation retry, 122 tool calls)
2. gromit-bu86: $4.12 (success, 69 tool calls)
3. gromit-mz3m: $3.83 (failed, validation retry, 99 tool calls)
4. gromit-6tyj: $3.79 (failed, validation retry, 125 tool calls)
5. gromit-u1gm: $3.75 (success, 71 tool calls)
