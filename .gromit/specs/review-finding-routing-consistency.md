---
id: review-finding-routing-consistency
source_ideas: []
created: 2026-03-01
epic: workflow-reliability
---

# Review Finding Routing Consistency

## Specification

`gromit review` must persist review findings consistently across interactive and non-interactive modes.

Today, non-interactive review parses structured JSON and persists both `beads_to_create` and `backlog_items`, while interactive review launches a session without a deterministic ingestion/apply step. This causes review findings to be inconsistently tracked and can leave findings in backlog-only form when users expect bd beads.

Required behavior:

- Both interactive and non-interactive review flows must use a shared review-result ingestion/apply path.
- `beads_to_create` must create tracker issues (bd beads) with expected labels and expected outputs.
- `backlog_items` must be persisted to an explicit backlog sink with clear labeling.
- Review completion output must report what was persisted by artifact type and include created IDs.
- If structured output is missing or invalid in a mode that promises ingestion, fail closed with a clear error and no false success message.

## Acceptance Criteria

- Interactive review completion can ingest structured JSON findings and persist:
  - all `beads_to_create` as bd issues
  - all `backlog_items` via backlog writer
- Non-interactive behavior remains correct and uses the same shared apply helper as interactive mode.
- For a mixed result (both arrays populated), final summary output reports exact counts and created IDs for both sinks.
- Invalid or malformed review JSON in interactive ingestion path returns a typed error and does not print a successful completion summary.
- Regression tests prove parity of routing semantics between interactive and non-interactive flows.

## Decisions

1. **Shared apply helper is mandatory**
   Parsing and persistence logic must live in one place to prevent mode drift.

2. **Fail closed on ingestion promise**
   If the command path claims it will ingest results, malformed/missing JSON is a hard failure.

3. **Explicit artifact accounting in UX**
   Completion output must distinguish tracker issues vs backlog entries and include identifiers.

## Research & Context

- Investigation report: [debug-20260301-123211.md](/home/dabrams/gromit/.gromit/reports/debug-20260301-123211.md)
- Plan draft: [fix-review-finding-routing.md](/home/dabrams/gromit/.gromit/plans/fix-review-finding-routing.md)
- Relevant code paths:
  - [review.go](/home/dabrams/gromit/cmd/gromit/review.go)
  - [pipeline.go](/home/dabrams/gromit/internal/pipeline/pipeline.go)
