# Spec 0005e — Regression Corpus

## spec_id
regression-corpus

## Depends on
spec-0005a, spec-0005c, spec-0005d

## Vision
Generic validation and review operate on abstract rules. They catch classes of problems but have no memory of specific failures that actually happened. A regression corpus grounds the system in real history — escaped bugs, near-misses, adversarial catches, and manually discovered defects become entries that future runs must prove they don't repeat. Unlike imagined test catalogs, a regression corpus has already proved its relevance. Every entry represents a failure someone cared about enough to record, making it the highest-signal regression surface the system can accumulate.

## Summary
A project accumulates a regression corpus of real failures — sourced from promoted proposals (0005d), adversarial review catches, and manual entries. Each entry is either a natural language assertion (verified by LLM evaluation) or an executable test snippet (run via a configurable command). A new pipeline stage, ReplayRegression, runs after Validate and replays corpus entries whose package tags or freeform tags match the current run's diff. A standalone CLI command allows on-demand replay against the current codebase. The corpus is language-agnostic — executable entries specify their runner command in configuration.

## Goals
### Primary
- Accumulate a living corpus of real escaped bugs, near-misses, and review catches
- Replay relevant corpus entries automatically during pipeline runs
- Support both natural language assertions (LLM-verified) and executable test snippets (runner-verified)
- Language-agnostic execution via configurable runner commands
- Match corpus entries to runs by package tags and freeform tags

### Secondary
- Provide a CLI for manual corpus entry and on-demand replay
- Integrate with 0005d's guardrail promotion as an entry source
- Record replay results in evidence for traceability

## Non-goals
- Auto-generating corpus entries from code analysis (entries come from real failures, not static analysis)
- Universal replay of all entries on every run (matching keeps replay targeted)
- Flaky-test management or retry logic for executable entries (out of scope)
- Cross-project corpus sharing (deferred)

## Architecture

### Corpus Store

Lives in `cellPath/regression-corpus.json`. Language-agnostic.

```go
package regressioncorpus

type Entry struct {
    ID               string    `json:"id"`
    Title            string    `json:"title"`
    Description      string    `json:"description"`
    Kind             string    `json:"kind"`              // "assertion" or "executable"
    Assertion        string    `json:"assertion,omitempty"`        // natural language assertion (kind=assertion)
    TestSnippet      string    `json:"test_snippet,omitempty"`    // code to execute (kind=executable)
    Runner           string    `json:"runner,omitempty"`          // command to run snippet, e.g. "go test" or "pytest"
    PackageTags      []string  `json:"package_tags"`              // package/directory paths for matching
    Tags             []string  `json:"tags"`                      // freeform tags for cross-cutting concerns
    Severity         string    `json:"severity"`                  // critical, high, medium, low
    Source           string    `json:"source"`                    // "promoted:<proposal-id>", "manual", "adversarial:<finding-id>"
    Status           string    `json:"status"`                    // active, superseded
    SourceRunID      string    `json:"source_run_id,omitempty"`
    SourceSpecID     string    `json:"source_spec_id,omitempty"`
    CreatedAt        time.Time `json:"created_at"`
    SupersededBy     string    `json:"superseded_by,omitempty"`
}

type Store struct{}

func (s *Store) Load(dir string) ([]Entry, error)
func (s *Store) Save(dir string, entries []Entry) error
```

Entry IDs computed from `(title, description)` via SHA-256, first 8 hex characters, prefixed with `rc-`.

### Entry Sources

1. **Promoted proposals (0005d)** — when a proposal is accepted and its content describes a specific reproducible failure, the triage CLI can promote it to the regression corpus via `review proposals accept --to-corpus`
2. **Manual entry** — `regression add` CLI command for bugs discovered outside the pipeline
3. **Adversarial findings** — blocking adversarial findings that triggered replan can be added via the triage CLI

### Matching Algorithm

When the ReplayRegression stage runs, it selects entries to replay:

1. **Package match** — if any of the entry's `package_tags` overlap with packages touched in the diff, the entry is selected
2. **Tag match** — if any of the entry's freeform `tags` appear in the spec content, acceptance criteria, or scenario text, the entry is selected
3. Only `active` entries are considered
4. Matching is deterministic — no LLM calls

### Replay Mechanisms

**Natural language assertions (kind=assertion):**
- The assertion text is evaluated by an LLM against the current codebase state (diff + relevant file contents)
- The LLM returns pass/fail with a rationale
- Model tier: configurable, defaults to project's medium tier

**Executable test snippets (kind=executable):**
- The snippet is written to a temp file and executed via the configured `runner` command
- Exit code 0 = pass, non-zero = fail
- Output captured for evidence
- Runner command is per-entry, allowing mixed languages in one corpus

### Pipeline Stage: ReplayRegression

Runs after Validate, before Review. Position:

```
Execute → WriteScenarioTests → Validate → ReplayRegression → Review → AdversarialReview → Accept
```

Lives in `internal/next/specloop/stages/replay_regression.go`. Implements the existing `Stage` interface.

**Behavior:**
1. Load corpus, filter to active entries
2. Match entries against current run's diff (package tags) and spec content (freeform tags)
3. If no entries match, return `Continue` immediately
4. Replay matched entries (assertions via LLM, executables via runner)
5. Collect results
6. If any entry fails: return `ReplanFrom` with failure context
7. If all pass: return `Continue`
8. Write `regression-replay.json` to evidence directory

### CLI Commands

**`regression add --title "..." --description "..." --kind assertion|executable [--assertion "..."] [--snippet-file path] [--runner "go test"] [--packages pkg1,pkg2] [--tags tag1,tag2] [--severity high]`**
- Adds a manual entry to the corpus
- `--kind assertion` requires `--assertion`
- `--kind executable` requires `--snippet-file` and `--runner`

**`regression list [--tag tag] [--package pkg] [--status active|superseded]`**
- Lists corpus entries with filters

**`regression show <entry-id>`**
- Displays full entry detail

**`regression replay [--package pkg] [--tag tag] [--all]`**
- On-demand replay against current codebase
- Defaults to entries matching current git diff; `--all` replays everything
- Reports pass/fail per entry

**`regression remove <entry-id> --reason "..."`**
- Supersedes an entry with a reason

### Configuration

```yaml
regression:
  enabled: true           # default: true
  model_tier: medium      # default: medium — for LLM-evaluated assertions
  default_runner: "go test"  # default runner for executable entries without explicit runner
```

### Evidence

`regression-replay.json` records:

```go
type ReplayResult struct {
    RunID       string         `json:"run_id"`
    Matched     int            `json:"matched"`
    Passed      int            `json:"passed"`
    Failed      int            `json:"failed"`
    Skipped     int            `json:"skipped"`
    Entries     []EntryResult  `json:"entries"`
}

type EntryResult struct {
    EntryID     string `json:"entry_id"`
    Title       string `json:"title"`
    Kind        string `json:"kind"`
    Status      string `json:"status"`      // passed, failed, skipped, error
    Output      string `json:"output"`      // runner output or LLM rationale
    DurationMs  int64  `json:"duration_ms"`
}
```

## Acceptance Criteria

1. When `regression.enabled` is true (default), the ReplayRegression stage runs after Validate and before Review
2. When `regression.enabled` is false, the stage is skipped
3. Corpus entries with `kind: assertion` are evaluated by an LLM against the current codebase state
4. Corpus entries with `kind: executable` are run via their configured `runner` command; exit code determines pass/fail
5. The runner command is per-entry and language-agnostic — not hardcoded to Go
6. A configurable `default_runner` is used for executable entries that don't specify their own runner
7. Entry matching uses package tag overlap with the diff and freeform tag overlap with spec content — no LLM calls for matching
8. Only `active` entries are replayed; `superseded` entries are excluded
9. If any matched entry fails, the stage returns `ReplanFrom` with failure context
10. If no entries match the current run, the stage returns `Continue` immediately with no LLM or runner invocations
11. Replay results are recorded in `regression-replay.json` in the evidence directory
12. `regression add` creates a manual corpus entry with title, description, kind, tags, packages, and severity
13. `regression replay` runs on-demand replay against the current codebase with optional filters
14. `regression list` and `regression show` provide corpus browsability
15. `regression remove` supersedes an entry with a recorded reason
16. Entries from 0005d's promotion pipeline can be added to the corpus via `review proposals accept --to-corpus`
17. Model tier for LLM-evaluated assertions is configurable, defaulting to project's medium tier
18. All existing tests continue to pass

## Scenarios

### Scenario: Regression entry catches repeated bug during pipeline run

**Given:** An active corpus entry with title "Parser returns nil on empty input", kind `assertion`, assertion "The Parse function in internal/parser returns a non-nil error when given empty string input", package tags `["internal/parser"]`
**When:** A run produces a diff touching `internal/parser/parse.go` and the ReplayRegression stage runs
**Then:** The entry matches via package tag. The LLM evaluates the assertion against the current code and diff. If the parser silently returns nil on empty input, the entry fails and the stage returns `ReplanFrom` with the failure context.

### Scenario: Executable regression test runs via configured runner

**Given:** An active corpus entry with kind `executable`, runner `pytest`, test snippet testing that a Python serializer round-trips correctly, package tags `["serializers/"]`
**When:** A run touches files in `serializers/` and ReplayRegression runs
**Then:** The snippet is written to a temp file and executed via `pytest`. Exit code 0 means pass. The output is captured in `regression-replay.json`.

### Scenario: No matching entries — stage passes immediately

**Given:** A corpus with 5 active entries, none of whose package tags or freeform tags match the current run's diff or spec
**When:** The ReplayRegression stage runs
**Then:** The stage returns `Continue` immediately with no LLM or runner invocations. `regression-replay.json` records `matched: 0`.

### Scenario: Manual entry added via CLI

**Given:** A developer discovers a bug where the CLI omits a warning field in JSON output
**When:** They run `regression add --title "CLI JSON output missing warning field" --description "When warnings are present, gromit run output JSON should include a warnings array" --kind assertion --assertion "The RunOutput struct serializes warnings as a non-nil JSON array when warnings are present" --packages "cmd/gromit,internal/runner" --tags "json,output,cli" --severity high`
**Then:** A new entry appears in `cellPath/regression-corpus.json` with source `manual` and status `active`

### Scenario: Promoted proposal becomes corpus entry

**Given:** An `automated-distillation-proposals.json` contains a `counterexample_seed` proposal about nil map handling that was a real failure
**When:** The reviewer runs `review proposals accept <proposal-id> --to-corpus --packages "internal/config" --tags "nil,maps"`
**Then:** A new corpus entry is created with source `promoted:<proposal-id>`, the proposal's description as the entry description, and the specified tags

### Scenario: On-demand replay via CLI

**Given:** A corpus with 10 active entries, 3 of which have package tag `internal/review`
**When:** The developer runs `regression replay --package internal/review`
**Then:** Only the 3 matching entries are replayed. Results are printed per-entry with pass/fail status and output.

### Scenario: Superseded entry excluded from replay

**Given:** A corpus entry with status `superseded` whose package tags match the current run's diff
**When:** The ReplayRegression stage runs
**Then:** The superseded entry is not replayed and does not appear in matched entries

### Scenario: Stage disabled via config

**Given:** A project with `regression.enabled: false`
**When:** The pipeline reaches the ReplayRegression stage
**Then:** The stage is skipped entirely — no corpus loading, no matching, no replay

### Scenario: Mixed-language corpus in single project

**Given:** A corpus with 3 entries: one with runner `go test`, one with runner `pytest`, one with kind `assertion` (LLM-evaluated)
**When:** All three match the current run and ReplayRegression runs
**Then:** Each entry is replayed via its own mechanism — Go test via `go test`, Python test via `pytest`, assertion via LLM. All three results appear in `regression-replay.json`.

## Validation

```
go test ./internal/next/regressioncorpus/...
go test ./internal/next/specloop/stages/...
go test ./internal/next/...
go test ./cmd/gromit-next/...
go vet ./...
```
