---
id: andon-autonomous-run-loop
source_ideas: []
created: 2026-02-15
---

# Andon-Style Autonomous Run Loop for Bead Failures

## Specification

Gromit run-loop behavior adopts an Andon-style failure policy optimized for high autonomy while preserving safety and output quality.

The target operating mode is:
- 80% of tasks complete without escalation.
- Up to 2 reasonable assumptions are allowed for ambiguous intent before escalation.
- Completion requires passing tests, lint, and build.
- Focused recovery work is capped at 15 minutes before stop-the-line escalation.

The run-loop executes this decision cycle on each step:
1. Parse intent and constraints.
2. Define success criteria and assumptions.
3. Execute the smallest reversible action.
4. Validate with quality gates.
5. Continue, self-heal, recover, or stop-the-line based on failure classification and thresholds.

Failure handling uses four escalation levels:
- L1 Quick Fix: bounded, safe self-heal attempts.
- L2 Focused Recovery: time-boxed diagnostics and deterministic repair sequence.
- L3 Stop Line: halt state-changing actions and prepare an escalation packet.
- L4 Human Decision: user chooses one of three options with explicit tradeoffs.

Failure classes are:
- Transient (timeouts, lock contention, flaky subprocess/network).
- Workflow (git/bd ordering or state mismatch).
- Quality Gate (tests, lint, build failures).
- Intent Ambiguity (unclear requirements that affect correctness).
- Data/State Integrity (suspected corruption or unsafe state divergence).

Hard-stop actions always require explicit user approval and trigger immediate escalation if needed:
- Bulk deletion (`rm -rf`) outside clearly scoped temporary directories.
- Irreversible schema or data migrations.
- Secrets or credential changes.

## Acceptance Criteria

- The spec defines a 4-level Andon escalation ladder with explicit L1 and L2 bounds and L3/L4 escalation behavior.
- The spec states autonomy and ambiguity targets: 80% autonomy and at most 2 assumptions before escalation when risk remains.
- The spec requires tests, lint, and build as mandatory completion gates.
- The spec defines a 15-minute L2 timebox and escalation when that budget is exhausted.
- The spec lists the three hard-stop action classes requiring explicit user approval.
- The spec includes immediate stop-line triggers for integrity risk, unresolved ambiguity after assumption budget, and unclear post-recovery quality-gate failure.
- The spec defines a standard escalation packet format that includes failed command, exact error, attempted recovery, state snapshot, risk, and three options with tradeoffs.
- The spec defines completion workflow steps including `git pull --rebase`, `bd sync`, `git push`, and up-to-date verification.

## Decisions

1. **Andon ladder over unbounded retries** A staged response (quick fix, focused recovery, stop line, human decision) maximizes autonomous throughput while preventing silent failure loops.

2. **Bounded autonomy with explicit thresholds** Numeric limits (2 assumptions, 2 retries/2 minutes, 15-minute recovery) reduce indecision and make escalation behavior predictable.

3. **Safety-first hard stops for destructive or sensitive actions** Bulk deletion, irreversible migrations, and credential changes are never executed autonomously without approval.

4. **Quality gates are mandatory, not best-effort** Tests, lint, and build are required before considering work complete to keep autonomy aligned with production-quality output.

5. **Escalation always includes 3 options with tradeoffs** This preserves user control at stop-line events while minimizing decision latency.

6. **Failure-class routing before action selection** Categorizing failures first prevents applying retry patterns to integrity or ambiguity problems that require escalation.

## Research & Context

### Current State

- Operational workflow expectations are documented in `AGENTS.md`, including end-of-session requirements to sync and push.
- Existing specs in `.gromit/specs/` follow a consistent structure: frontmatter, Specification, Acceptance Criteria, Decisions, and Research & Context.
- Gromit already uses structured loop control, failure handling, and escalation concepts in runner internals such as `internal/runner/runner.go` and `internal/runner/escalation/`.

### Failure-Class Thresholds and Actions

- Transient:
  - L1: retry up to 2 times or 2 minutes.
  - L2: diagnostics and deterministic repair for up to 15 minutes.
  - Escalate when repeated after L2 or integrity risk emerges.

- Workflow:
  - L1: one deterministic repair sequence.
  - L2: full state reconciliation once end-to-end.
  - Escalate if safe next action is unclear.

- Quality Gate:
  - L1: one obvious local fix attempt.
  - L2: isolate and fix root cause within 15 minutes.
  - Escalate for unclear systemic or cascading failures.

- Intent Ambiguity:
  - L1: use up to 2 assumptions.
  - Escalate if ambiguity persists and affects behavior/architecture.

- Data/State Integrity:
  - Immediate L3 stop line when credible integrity risk is detected.

### Command Templates

L1 Quick Fix:

```bash
bd sync
git status
# rerun the failed command
```

L2 Focused Recovery:

```bash
date
git status
git branch --show-current
bd sync
bd ready
# if an issue is active:
bd show <id>
bd update <id> --status in_progress

# quality gates
make test
make lint
make build
```

L3 Stop Line:
- Halt state-changing commands.
- Assemble escalation packet.
- Request L4 decision with 3 options and tradeoffs.

### Escalation Packet Template

```text
Incident: <short title>
Failed step: <command>
Error: <exact stderr/stdout excerpt>
Class: <Transient|Workflow|Quality|Intent|Data>
L1 attempts: <what + result>
L2 attempts: <what + result>
Current state:
- git status: <summary>
- branch: <name>
- bd state: <ready/in_progress details>
Risk: <low|med|high> + why

Options:
1) <Option A> - tradeoff
2) <Option B> - tradeoff
3) <Option C> - tradeoff
Recommended: <one line>
```

### Completion Protocol

1. File follow-up issues for remaining work.
2. Run tests, lint, and build.
3. Update issue status (`bd close` or `bd update`).
4. Run push sequence:

```bash
git pull --rebase
bd sync
git push
git status
```

5. Verify branch is up to date with origin.
6. Provide handoff context.

### Reliability Metrics

Track weekly:
- Autonomy completion rate.
- First-pass success rate.
- Mean time to recovery.
- Escalation rate by failure class.
- Recurrence rate for repeated failures.
- Intent mismatch defects found post-merge.
