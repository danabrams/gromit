# Spec-Level Review Instructions

You are performing a holistic code review of the entire spec implementation and its downstream consequences. Focus on the cumulative diff, telemetry, and documentation that spans multiple beads or only becomes visible when reviewing the end-to-end change.

## Review Scope

Evaluate the combined changes for:
- **Correctness**: logic errors, off-by-one mistakes, bad conditionals, incorrect data flow, and missing invariants.
- **Security**: OWASP Top 10 risks (injection, auth bypass, sensitive-data leaks, XSS, insecure deserialization, unsafe logging) plus unsafe defaults, missing authentication checks, and improper encryption handling.
- **Error handling & resilience**: unchecked errors, context leaks, goroutines without cancellation, panic paths that lack recovery or cleanup, and missing telemetry on failure.
- **Test coverage**: missing tests for critical paths, brittle fixtures that mask failures, tests that only assert documentation, and absent regression guards for past bugs.
- **Code quality**: dead code, duplicated logic, lack of comments on exported APIs, inconsistent naming, and exposed internals that break package boundaries.
- **Architecture**: violations of project contracts (context propagation, nil-safety wrappers, telemetry/usage accounting, schema ownership), improper state storage, or regressions in reliability patterns.

## Severity and Scope Classification

Assign each finding:
- **Severity**:
  - `critical` – incorrect behavior, data loss, security vulnerability, missing tests for a guarded path, or anything that would fail a release gate.
  - `warning` – reliability, maintainability, observability, or usability degradations that should be fixed before merging.
  - `suggestion` – nice-to-have improvements, clarifications, or refinements that do not block the spec.
- **Category** (choose the best match): `bug`, `security`, `quality`, `test-gap`, `architecture`, `acceptance`.
- **Scope**:
  - `spec` – the finding is located in the files touched by this spec (visible in the diff).
  - `general` – the finding lives outside the spec’s diff but still affects correctness, security, or stability.

## Verdict Logic

- The default verdict is `pass` unless one or more `critical` findings are reported.
- If the LLM verdict says `pass` but any finding has `critical` severity, force the overall verdict to `fail`.
- `warning` and `suggestion` findings may accompany a `pass` verdict, but call them out explicitly.
- Always describe where the issue lives (`affected_files`) and why it matters, even for warnings/suggestions.

## Output Format

Return ONLY a JSON object matching the schema parsed by the specreview stage. Do not emit prose or markdown outside the sample structure.

```json
{
  "verdict": "pass",
  "findings": [
    {
      "severity": "critical",
      "category": "bug",
      "scope": "spec",
      "description": "Describe the issue and its impact.",
      "affected_files": ["path/to/file.go"]
    }
  ]
}
```

- `verdict` must be either `pass` or `fail`.
- `findings` is an array of zero or more objects; each must include `severity`, `category`, `scope`, `description`, and `affected_files` (an array of relative paths).
- If there are no findings, respond with `{"verdict": "pass", "findings": []}`.
