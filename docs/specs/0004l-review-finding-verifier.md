# Spec 0004l — Review Finding Verifier

## spec_id
0004l-review-finding-verifier

## Depends on
0003c-review-graceful-degradation

## Vision

The review stage generates findings by invoking the reviewer with a diff summary and the spec. For files in the current diff the reviewer has fresh evidence — it just read the changed code. For files not in the diff the reviewer has no fresh context; it reasons from prior prompt context that may be many cycles stale. This causes hallucination: the reviewer flags a bug that was already fixed because its context still contains the old version of the file.

In run `run-f183a076113ff186`, the reviewer flagged `validate.go` as still having a bare `"Acceptance Criteria"` marker — a bug that was fixed in cycle 2. The fix was already committed. Because the finding was a blocking error, it triggered a replan. The planner faithfully generated a repair task. The executor ran it, found nothing to do, and the next review cycle produced the same finding again. This loop ran for 18 consecutive replans across 8 cycles, wasting $4.69 in API costs before the run was abandoned.

The root cause is not reviewer quality — it is reviewer access. A reviewer cannot reliably flag code it has not read. Out-of-diff blocking errors are the only class of finding that creates thrash loops, because they block each cycle without the executor ever having a chance to address them (the code is already correct). This spec closes that gap with a targeted verifier: a lightweight haiku invocation that reads the actual file and confirms or discards each out-of-diff blocking error before it is allowed to block.

## Summary

After `ReviewStage.Run` receives findings from `Runner.Run`, for each blocking `Finding` (severity meets the replan threshold) whose `File` field is not present in the current diff, spawn a lightweight verifier invocation. The verifier reads the relevant lines from the actual file on disk, compares them to the finding's description, and returns one of three dispositions: `confirmed`, `downgraded`, or `fixed`. Confirmed findings pass through unchanged and are marked `VerifiedTrue`. Downgraded findings have their severity reduced to `SeverityWarning` and never block. Fixed findings are suppressed entirely. Every decision is appended to a per-run audit log. Findings about files in the current diff skip verification entirely — the reviewer just read them.

## Goals

### Primary
- Suppress out-of-diff blocking errors that refer to code that has already been fixed
- Downgrade out-of-diff blocking errors that are real but less severe than claimed
- Preserve all in-diff findings unchanged (no false negatives from over-verification)
- Produce an audit log entry for every verifier decision so systematic verifier errors are detectable

### Secondary
- Use haiku (`"low"` tier) for the verifier to keep per-verification cost minimal
- Run verifications in parallel (one goroutine per eligible finding) to bound latency
- Reuse the existing `llmadapter.FallbackAdapter` wiring pattern
- Emit a `review_finding_verified` event per decision for run-level observability

## Non-goals
- Verifying warning or suggestion findings (they do not block cycles; expand later if warranted)
- Verifying in-diff findings (the reviewer has fresh context; verification would add latency for no gain)
- Automatically rewriting findings or suggesting fixes based on verifier output
- Changing the facet review prompts or `Runner` internals
- Replacing the existing `filterContractContradictions` filter — the verifier runs before that filter, on the raw `BlockingFindings` slice

## Architecture

All new code is localized to two places: a new file `internal/next/review/verifier.go` in the `review` package, and modifications to `internal/next/specloop/stages/review.go` to wire the verifier into the blocking-findings path.

### `FilesInDiff(diffText string) map[string]bool` (`internal/next/review/verifier.go`)

A pure function that extracts the set of modified file paths from a unified diff string. A line beginning with `+++ b/` introduces a modified file; strip the `b/` prefix and normalize to a relative path. Returns an empty map when `diffText` is empty or contains no `+++ b/` lines. Used by `ReviewStage.Run` to determine which findings are eligible for verification.

### `VerifierDisposition` (`internal/next/review/verifier.go`)

```go
type VerifierDisposition string

const (
    DispositionConfirmed   VerifierDisposition = "confirmed"
    DispositionDowngraded  VerifierDisposition = "downgraded"
    DispositionFixed       VerifierDisposition = "fixed"
)
```

### `VerifierResult` (`internal/next/review/verifier.go`)

```go
type VerifierResult struct {
    Finding     Finding
    Disposition VerifierDisposition
    Reason      string
    FileExcerpt string // lines shown to the verifier
}
```

`NormalizeNilFields` is a no-op (no slice/map fields).

### `FindingVerifier` interface (`internal/next/review/verifier.go`)

```go
type FindingVerifier interface {
    Verify(ctx context.Context, f Finding, workDir string) (VerifierResult, error)
}
```

The interface allows a test double to be injected into `ReviewStage` without a live LLM.

### `LLMFindingVerifier` (`internal/next/review/verifier.go`)

The production implementation. Uses an `llmadapter.Completer` at the `"low"` tier (haiku). For a given `Finding`:

1. Read lines `max(0, f.Line-5)` to `min(EOF, f.Line+10)` from `filepath.Join(workDir, f.File)`. If the file cannot be read, return `DispositionConfirmed` with reason `"file unreadable — retaining finding"` (fail safe: do not suppress a finding we cannot inspect).
2. Build the verifier prompt from a fixed template (see Verifier prompt, below).
3. Call the completer with the prompt.
4. Parse the first word of the response as the disposition. If parsing fails, return `DispositionConfirmed` with reason `"parse error — retaining finding"` (fail safe).
5. Return a `VerifierResult` with the disposition, the one-sentence reason, and the file excerpt shown.

### Verifier prompt template

```
You are verifying a code review finding against the actual current source code.

Finding: [severity] [file]:[line] — [description]

Current file contents at that location (lines [N-5] to [N+10]):
[file excerpt]

Is this finding still valid?
- confirmed: the described problem is present in this code
- downgraded: a real issue exists but it is less severe than "error"
- fixed: the code no longer has this problem

Return exactly one word (confirmed / downgraded / fixed) followed by a single sentence explaining why.
```

### `VerifyBlockingFindings` (`internal/next/review/verifier.go`)

```go
func VerifyBlockingFindings(
    ctx context.Context,
    findings []Finding,
    diffFiles map[string]bool,
    verifier FindingVerifier,
    workDir string,
) (kept []Finding, results []VerifierResult)
```

Iterates over `findings`. For each finding:
- If the file is in `diffFiles` (basename match: `diffFiles[filepath.Base(f.File)]` or `diffFiles[f.File]`): append to `kept` unchanged, no `VerifierResult` emitted.
- Otherwise: call `verifier.Verify` concurrently. On `confirmed`: append to `kept`. On `downgraded`: set `f.Severity = SeverityWarning`, append to `kept`. On `fixed`: do not append to `kept`.

All goroutines run in parallel; results are collected via a channel. Returns `kept` in stable order (same ordering as input, minus suppressed findings). `results` contains one `VerifierResult` per verified finding (in-diff findings produce no result entry).

### `ReviewStageConfig` changes (`internal/next/specloop/stages/review.go`)

Add two fields:

```go
type ReviewStageConfig struct {
    // ... existing fields ...
    Verifier  review.FindingVerifier  // nil disables verification (backward compatible)
    WorkDir   string                  // needed for file reads during verification
}
```

When `cfg.Verifier` is nil, the verification step is a no-op and all blocking findings pass through unchanged.

### Wiring in `ReviewStage.Run`

After `result.BlockingFindings` is computed (by `Runner.Run`) and before `filterContractContradictions`, when `cfg.Verifier != nil`:

1. Compute `diffFiles := review.FilesInDiff(diffSummary)`.
2. Call `review.VerifyBlockingFindings(ctx, result.BlockingFindings, diffFiles, cfg.Verifier, cfg.WorkDir)`.
3. Overwrite `result.BlockingFindings` with `kept`.
4. Recompute `result.HasBlockingFindings = len(result.BlockingFindings) > 0`.
5. Emit one `review_finding_verified` event per `VerifierResult` (see Events, below).
6. Append each `VerifierResult` to the verifier audit log (see Audit log, below).

### `ReviewFindingVerifiedEvent` (`internal/next/runstore/events.go`)

```go
type ReviewFindingVerifiedEvent struct {
    BaseEvent
    File        string `json:"file"`
    Line        int    `json:"line"`
    Severity    string `json:"severity"`
    Description string `json:"description"`
    Disposition string `json:"disposition"`
    Reason      string `json:"reason"`
}
```

Emitted once per verified finding. Type string: `"review_finding_verified"`.

### Audit log (`internal/next/specloop/stages/review.go`)

When `cfg.EvidenceDir != ""`, after verification, write (append) each `VerifierResult` as a JSON line to `<evidenceDir>/verifier-audit.jsonl`. Each line contains: finding fields (`file`, `line`, `severity`, `description`), `disposition`, `reason`, and `file_excerpt`. The file is created on first write; writes are best-effort (errors are logged, not fatal).

### Wiring in `stage_provider.go`

In `RealStageProvider.BuildStages`, when `p.claudeProvider != nil`, construct an `LLMFindingVerifier` using a `"low"`-tier `FallbackAdapter`:

```go
verifierAdapter := llmadapter.NewFallbackAdapter(
    router, "verify",
    llmadapter.Config{Tier: "low", OnCost: costCallback, OnInvocation: invocationCallback},
    "low",
)
verifier := review.NewLLMFindingVerifier(verifierAdapter)
```

Pass `verifier` and `WorkDir` to `ReviewStageConfig`. When `p.claudeProvider == nil`, leave `Verifier` nil (noop path).

## Acceptance Criteria

1. `FilesInDiff` returns a map containing `"validate.go"` when the diff text contains a line `+++ b/internal/next/specloop/stages/validate.go`; returns an empty map for an empty diff string.

2. `VerifyBlockingFindings` passes in-diff findings through to `kept` without calling the verifier; `results` contains no entry for those findings.

3. `VerifyBlockingFindings` calls the verifier for out-of-diff findings; a `confirmed` response leaves the finding in `kept` with severity unchanged.

4. `VerifyBlockingFindings` calls the verifier for an out-of-diff finding; a `fixed` response removes the finding from `kept`.

5. `VerifyBlockingFindings` calls the verifier for an out-of-diff finding; a `downgraded` response keeps the finding in `kept` with `Severity` set to `SeverityWarning`.

6. `LLMFindingVerifier.Verify` returns `DispositionConfirmed` with a "file unreadable" reason when the file at `filepath.Join(workDir, f.File)` does not exist.

7. `LLMFindingVerifier.Verify` returns `DispositionConfirmed` with a "parse error" reason when the LLM response cannot be parsed as a known disposition word.

8. When `ReviewStageConfig.Verifier` is nil, `ReviewStage.Run` behaves identically to the pre-spec behavior: all blocking findings pass through unchanged, no verifier events are emitted.

9. When a finding is verified and `EvidenceDir` is non-empty, a JSON line is appended to `<evidenceDir>/verifier-audit.jsonl` containing `file`, `line`, `severity`, `description`, `disposition`, `reason`, and `file_excerpt` fields.

10. A `review_finding_verified` event is emitted to the event log for each out-of-diff finding that is verified, with `disposition` and `reason` set from the verifier response.

11. `ReviewResultEvent` (already emitted in `ReviewStage.Run`) reflects the post-verification `BlockingFindings` count — i.e., suppressed findings are not counted.

12. All existing `review_test.go`, `review_integration_test.go`, and `review_contract_filter_test.go` tests continue to pass (backward compatibility via nil `Verifier` field).

## Scenarios

### Scenario: verifier suppresses a false positive

**Given:** the reviewer returns one blocking error finding with `File: "internal/next/specloop/stages/validate.go"`, `Line: 42`, `Description: "specACMentionsPath still uses bare Acceptance Criteria marker"`
**And:** `validate.go` is NOT in the current diff (the fix was committed in a prior cycle)
**And:** the verifier reads lines 37–52 of `validate.go` and finds no bare marker
**When:** `ReviewStage.Run` completes
**Then:** `result.BlockingFindings` is empty
**And:** `result.HasBlockingFindings` is false
**And:** the stage returns `specloop.Continue` (no replan triggered)
**And:** a `review_finding_verified` event with `disposition: "fixed"` is appended to the event log
**And:** a JSON line with `disposition: "fixed"` is appended to `verifier-audit.jsonl`

### Scenario: verifier confirms a real bug on an untouched file

**Given:** the reviewer returns one blocking error finding with `File: "internal/next/review/runner.go"`, `Line: 120`, `Description: "concurrent map write without lock"`
**And:** `runner.go` is NOT in the current diff
**And:** the verifier reads lines 115–130 of `runner.go` and finds a map write outside the mutex
**When:** `ReviewStage.Run` completes
**Then:** `result.BlockingFindings` still contains the finding with `Severity: SeverityError`
**And:** the stage returns `specloop.ReplanFrom`
**And:** a `review_finding_verified` event with `disposition: "confirmed"` is emitted

### Scenario: verifier is skipped for in-diff files

**Given:** the reviewer returns one blocking error finding with `File: "internal/next/specloop/stages/validate.go"`
**And:** `validate.go` IS in the current diff (it was modified this cycle)
**When:** `VerifyBlockingFindings` is called
**Then:** the verifier is never invoked
**And:** the finding passes through to `kept` unchanged
**And:** `results` is empty

### Scenario: verifier is skipped for non-blocking findings

**Given:** the reviewer returns one finding with `Severity: SeverityWarning` on an out-of-diff file
**And:** `Threshold` is `SeverityError` (so the warning is not blocking)
**When:** `ReviewStage.Run` completes
**Then:** the verifier is never called for that warning finding
**And:** the finding appears in `result.AllFindings` but not in `result.BlockingFindings`

### Scenario: verifier downgrades an overclassified finding

**Given:** the reviewer returns one blocking error finding with `File: "cmd/gromit-next/stage_provider.go"`, `Line: 75`
**And:** `stage_provider.go` is NOT in the current diff
**And:** the verifier reads the relevant lines and returns `downgraded` — the issue is minor style, not a bug
**When:** `ReviewStage.Run` completes
**Then:** the finding appears in `kept` with `Severity: SeverityWarning`
**And:** `result.HasBlockingFindings` is false (warning does not meet `SeverityError` threshold)
**And:** a `review_finding_verified` event with `disposition: "downgraded"` is emitted

### Scenario: file is unreadable — fail safe retains finding

**Given:** the reviewer returns one blocking error finding with `File: "cmd/gromit-next/nonexistent.go"`
**And:** `nonexistent.go` is NOT in the current diff
**And:** the file does not exist on disk at `filepath.Join(workDir, f.File)`
**When:** `LLMFindingVerifier.Verify` is called
**Then:** it returns `DispositionConfirmed` with reason containing "file unreadable"
**And:** the finding is retained in `kept` with its original severity
**And:** the stage triggers a replan as it would have without the verifier

## Validation

### Automatic

```
go test ./internal/next/review/... -count=1 -timeout 60s
go test ./internal/next/specloop/stages/... -count=1 -timeout 60s
go test ./cmd/gromit-next/... -count=1 -timeout 60s
go vet ./...
go build ./...
```
