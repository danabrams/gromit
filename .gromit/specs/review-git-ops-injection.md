---
id: review-git-ops-injection
source_ideas: []
created: 2026-02-19
epic: codebase-health
---

# Review Git Ops Injection and Bead ID Literal Matching

## Specification

Improve review scope and diff helper behavior so git subprocess calls used by review logic are injectable in tests and bead IDs are matched as literal strings.

This refinement applies to the git helper flow currently used by `gromit review` in `cmd/gromit/review.go`, specifically:
- `findFirstCommitForBead`
- `getCommitTimestamp`
- `getGitDiffForReview`
- `getGitDiffStatForReview`
- `getGitHeadForReview`

Behavioral requirements:
- The above helpers must execute git through injected subprocess dependencies rather than direct `exec.Command` calls.
- Production behavior must remain unchanged: same command semantics, same returned values, and same error surfaces for callers.
- Bead ID lookup must treat the bead ID as a literal search term in git log grep matching, not a regex pattern.
- Existing guardrails that reject flag-like user input (for example values starting with `-`) must remain in effect.

The change is focused on testability and correctness of commit lookup semantics, without expanding scope to repository-wide subprocess abstraction work.

## Acceptance Criteria

- The five review git helper functions no longer call `exec.Command` directly and instead use injected command execution dependencies.
- Unit tests can deterministically stub git command invocation for these helpers without launching real git subprocesses.
- `findFirstCommitForBead` uses git grep literal mode (`--fixed-strings`) when searching by bead ID.
- A bead ID containing regex metacharacters is matched literally (not interpreted as regex) in the commit lookup path.
- Existing validation behavior for invalid refs/IDs (empty or flag-like values) remains covered by tests and unchanged for callers.

## Decisions

1. **Keep scope local to review helper functions**  
   The spec limits changes to `cmd/gromit/review.go` helper paths to address the reported bug and testability gap quickly. A broader shared git abstraction can be considered separately.

2. **Preserve current runtime behavior while changing wiring**  
   The intent is dependency injection for test control, not user-facing behavior changes. Command outputs, error propagation style, and review flow semantics stay stable.

3. **Treat bead IDs as literals in git grep**  
   Bead IDs are identifiers, not regex inputs. Literal matching avoids false positives/negatives and prevents regex interpretation surprises.

## Research & Context

### Current State

`cmd/gromit/review.go` currently shells out directly in each target helper:
- `findFirstCommitForBead` runs `git log --all --format=%H --grep <beadID>`.
- `getCommitTimestamp` runs `git log -1 --format=%at <commit> --`.
- `runGitDiffForReview` (used by `getGitDiffForReview` and `getGitDiffStatForReview`) runs `git diff ...`.
- `getGitHeadForReview` runs `git rev-parse HEAD`.

These direct command constructions make helper-level tests rely on real subprocess execution unless higher-level integration paths are used.

### Existing Pattern Alignment

The codebase already contains injectable subprocess patterns (for example command/run function injection in `internal/agent/agent.go`). This refinement aligns review helpers with that testability pattern.

### Risk/Compatibility Notes

- `--fixed-strings` changes matching mode only for bead ID grep terms; this is intended to enforce literal identifier handling.
- Input validation checks already present in review helpers should remain intact to preserve command safety constraints.
