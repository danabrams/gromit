# Spec-Level Review Prompt Template (v2)

You are performing a **spec-level review** of the cumulative diff for the current bead. Treat the diff as the authoritative set of changes that will be shipped, and evaluate every line of that diff through the lenses described below. Always assume the context provided by the bead title and description; never invent missing facts.

## Review Focus
Evaluate the cumulative diff for each of the following dimensions:

1. **Correctness** – Does the new spec behave as intended? Are invariants preserved? Do control flows handle edge cases, branching logic, and preconditions safely?
2. **Security** – Identify any new sensitive data exposure, authentication/authorization gaps, injection risks, or other OWASP-style weaknesses introduced by the change.
3. **Error handling** – Are failure paths, retries, and diagnostics surfaced consistently (including logging, wrapping, and cleanup)? Are panic/exception paths covered?
4. **Test coverage** – Does the diff add or require tests? What gaps remain? Are critical branches or regression risks left unverified?
5. **Code quality** – Look for clarity, naming, duplication, dead code, or overly complex constructs that reduce maintainability.
6. **Architectural fit** – Does the change respect the architecture contracts (context propagation, telemetry persistence, strict writers, single schema owners, etc.)? Does it align with documented caps/flows and maintain the expected ownership boundaries?

When evaluating, reason about the **cumulative diff** (all files changed together) rather than isolated snippets. Look for systemic issues that only materialize after the entire change is considered.

## Output Format
Return a JSON object containing the overall assessment described below. Do not wrap the JSON in markdown or additional text.

```json
{
  "findings": [
    {
      "verdict": "issue" | "pass",
      "severity": "critical" | "high" | "medium" | "low",
      "category": "correctness" | "security" | "error_handling" | "test_coverage" | "code_quality" | "architecture",
      "scope": "Short descriptor of the subsystem or area under review (e.g., \"prompt rendering\" or \"cmd/gromit/review_spec_validation\").",
      "description": "Describe the finding, cite the relevant diff surface, and explain why it matters.",
      "affected_files": ["list", "of", "changed", "files", "impacted"]
    }
  ],
  "summary": "Two-sentence overview of the health of this spec-level diff and next steps (e.g., blockers, confidence, follow-up work)."
}
```

### Guidelines for findings
- Report **every blocking issue** as a finding with `verdict`: `issue` and `severity` set to the appropriate level. Use `category` to tie the issue to one of the six review dimensions above. Provide an actionable description and list the files touched.
- If no problems exist, emit at least one `verdict": "pass"` finding that summarises why the spec is ready and which areas were exercised.
- Mention both **positive behaviors** (tests added, architectural alignments) and **risks** (missing checks, unknown attribution) in separate findings when helpful.
- Be explicit about architectural fit: call out adherence or violations of the architecture rules (context/threading contracts, single schema writers, telemetry epilogue, etc.).
- Keep `affected_files` relative to the repository root.

Treat this prompt as the **final spec-level gate** before acceptance: the JSON you return should drive go/no-go decisions, so be precise, concise, and comprehensive.
