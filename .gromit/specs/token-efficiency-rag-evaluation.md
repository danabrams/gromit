---
id: token-efficiency-rag-evaluation
source_ideas: []
created: 2026-02-23
---

# Token Efficiency RAG Evaluation (Phase 4)

## Specification

Run a bounded, evidence-driven experiment to determine whether retrieval-assisted code discovery reduces token spend and latency without harming solution quality.

This spec is evaluation-first. It does not assume RAG adoption.

## Problem

Codebase discovery turns often consume large token budgets through repeated grep/read loops over low-signal files. RAG could reduce discovery overhead, but can also introduce retrieval errors, stale context, and new maintenance burden.

## Experiment Scope

### In Scope

1. Build a minimal local retrieval pipeline for code discovery queries
2. Compare retrieval workflow against baseline grep/read workflow
3. Decide adoption based on explicit gates

### Out of Scope

1. Full production search platform
2. Replacing deterministic code-reading tools
3. Using retrieval results without source file/line attribution

## Changes

### A. Experimental Retrieval Tool

Add an experimental tool interface that returns:
- Top-K candidate snippets
- File path and line-range metadata
- Retrieval score/confidence metadata

Tool use is limited to discovery/query phases in the experiment.

### B. Indexing Strategy

Create a local index with deterministic refresh rules:
- Initial build from tracked project files
- Incremental updates on file changes
- Explicit staleness marker when index is outdated

### C. Workflow Policy

Define strict policy for model usage:
- Use retrieval as entry-point guidance, not authoritative truth
- Require source verification by opening cited files/lines before edits
- Fall back to baseline tools when retrieval confidence is low

### D. Evaluation Harness

Run fixed scenario set with paired comparisons:
- Baseline path (grep/read only)
- Retrieval-assisted path

Capture token/cost/latency/quality metrics for each scenario.

## Acceptance Criteria

1. Retrieval tool returns attributed snippets with file and line metadata
2. Index staleness is detectable and surfaced
3. Experiment report includes paired baseline vs retrieval results
4. Adoption decision is made using gates below

## Adoption Gates

RAG advances beyond experiment only if all gates pass:
1. Median discovery-phase input tokens reduced by >=20%
2. Median discovery-phase latency reduced by >=15%
3. No material drop in task success rate
4. No increase in mislocated-edits or wrong-file edits beyond threshold
5. Operational overhead (index rebuild time/storage) remains acceptable

If any gate fails, retain baseline workflow and close experiment with findings.

## Risks and Mitigations

1. Risk: Hallucinated or weakly relevant retrieval results
- Mitigation: Mandatory source verification before edits

2. Risk: Stale index leads to wrong guidance
- Mitigation: Incremental refresh + staleness signaling + fallback path

3. Risk: Added complexity outweighs savings
- Mitigation: hard adoption gates and explicit no-adopt outcome

## Decisions

1. RAG is optional and evidence-gated, not pre-committed.
2. Retrieval supports discovery; deterministic tools remain primary for verification.
3. Experiment output must be reproducible on a fixed workload set.

## Dependencies

1. Metrics from `token-efficiency-foundation`
2. Routing/caching controls from `token-efficiency-cache-and-tiering` where relevant
3. Existing file-read/open tooling for attribution verification

## Related Specs

1. `token-efficiency-foundation`
2. `token-efficiency-cache-and-tiering`
