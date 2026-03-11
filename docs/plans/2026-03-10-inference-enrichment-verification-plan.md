# Spec 0001a — Inference Enrichment Verification Plan

**Goal:** Confirm the implementation satisfies every user story, acceptance criterion, and design constraint from the spec and design doc.

**Run all tests:** `go test ./internal/next/... -v`
**Run all lints:** `go vet ./internal/next/... && go vet ./cmd/gromit-next/...`

---

## Foundation Packages

### `enrich` package — Inferred Fact Model

| Test | Verifies |
|------|----------|
| `TestInferredFactStatus_String` | String representation of proposed/accepted/rejected/superseded |
| `TestInferredFactStatus_JSONRoundTrip` | Status serializes as string in JSON and round-trips correctly |
| `TestInferredFact_ContentHash` | Same content produces same ID; different category produces different ID |
| `TestAllCategories` | All 8 enrichment categories are defined |

### `enrich` package — Configuration

| Test | Verifies |
|------|----------|
| `TestConfig_Defaults` | Default config: claude/sonnet/medium/30 days |
| `TestConfig_SaveAndLoad` | Config round-trips through JSON file |
| `TestConfig_LoadMissing` | Missing config file returns defaults |
| `TestConfig_LoadCorrupted` | Corrupted config file returns error |

### `enrich` package — Fact Store

| Test | Verifies |
|------|----------|
| `TestFactStore_SaveAndLoad` | Facts round-trip through `inferred/facts.json` |
| `TestFactStore_LoadEmpty` | Empty/missing facts file returns empty slice |
| `TestFactStore_MergeStatuses` | Accepted facts retain status; missing accepted facts become superseded |
| `TestFactStore_UpdateStatus` | Individual fact status can be updated |

### `enrich` package — Run Store

| Test | Verifies |
|------|----------|
| `TestRunStore_SaveAndLoad` | Run artifacts round-trip through run directory |
| `TestRunStore_ListRuns` | Multiple runs can be listed |
| `TestRunStore_SavesSummary` | Human-readable summary.md is generated |

### `enrich` package — Staleness

| Test | Verifies |
|------|----------|
| `TestStaleness_FreshFact` | Recent fact is not expired |
| `TestStaleness_ExpiredFact` | Old fact is expired |
| `TestStaleness_FilterExpired` | Expired facts are removed from list |
| `TestStaleness_ObservedFactsFreshness` | SHA mismatch produces warning |

---

## Verification by User Story

### Story 1: Enrich project understanding

**What to verify:**
- `gromit-next project enrich <project>` runs enrichment passes and produces inferred facts
- Facts are stored in `inferred/facts.json` within the project cell
- Run artifacts are stored in `inferred/runs/<run-id>/`
- Each fact has: fact_id, category, statement, rationale, evidence_refs, confidence, scope, status
- Cost and token data are captured per run

**Tests:**

| Test | Package | Verifies |
|------|---------|----------|
| `TestOrchestrator_RunAll` | `enrich` | Full enrichment flow produces facts |
| `TestOrchestrator_PartialFailure` | `enrich` | Failed categories don't block successful ones |
| `TestRunStore_SaveAndLoad` | `enrich` | Run artifacts persist correctly |
| `TestRunStore_SavesSummary` | `enrich` | Cost/token summary is generated |

**Manual check:** Run `gromit-next project enrich <project>`, then inspect `$GROMIT_HOME/projects/<name>/inferred/facts.json` and `inferred/runs/`.

---

### Story 2: Keep inference separate

**What to verify:**
- Inferred facts are stored in `inferred/`, not in `artifacts/`
- No canonical artifact (`architecture.json`, `sourcemap.json`, `doctrine.json`, `validation.json`, `glossary.json`) is modified by enrichment
- Every fact has `source_type: "inferred"`

**Tests:**

| Test | Package | Verifies |
|------|---------|----------|
| Integration test | `enrich` | Facts written to `inferred/`, artifacts unchanged |
| `TestInferredFactStatus_JSONRoundTrip` | `enrich` | source_type field present |

**Manual check:** Run enrichment, then `ls $GROMIT_HOME/projects/<name>/artifacts/` — files should be unchanged. Compare timestamps before/after.

---

### Story 3: Review inferred facts

**What to verify:**
- `review-inferred` lists all inferred facts with IDs, categories, statements, and statuses
- `accept-inferred --fact <id>` changes status to accepted
- `reject-inferred --fact <id>` changes status to rejected
- Status changes persist across commands

**Tests:**

| Test | Package | Verifies |
|------|---------|----------|
| `TestFactStore_UpdateStatus` | `enrich` | Status update persists |
| `TestFactStore_MergeStatuses` | `enrich` | Status preserved across re-enrichment |

**Manual check:** Run `review-inferred`, accept a fact, run `review-inferred` again — status should show `accepted`.

---

### Story 4: Use inferred observations in the guide

**What to verify:**
- Default guide excludes inferred content
- `--include-inferred` adds clearly labeled inferred sections
- Inferred sections are grouped by category
- Every inferred section contains `[INFERRED]` marker
- Confidence is shown

**Tests:**

| Test | Package | Verifies |
|------|---------|----------|
| `TestMarkdownRenderer_InferredSections` | `guide` | Inferred sections render with markers |
| `TestMarkdownRenderer_NoInferredByDefault` | `guide` | Default excludes inferred |

**Manual check:** Render guide with and without `--include-inferred`. Diff the two outputs. Inferred version should have additional sections, all marked.

---

### Story 5: Keep default packets conservative

**What to verify:**
- Default context packets exclude inferred facts
- `--include-inferred` adds inferred sections with provenance
- Inferred facts are filtered by packet scope

**Tests:**

| Test | Package | Verifies |
|------|---------|----------|
| `TestCompiler_ProjectLevelWithInferred` | `contextpkt` | Inferred sections included when requested |
| `TestCompiler_ProjectLevelDefaultExcludesInferred` | `contextpkt` | Default excludes inferred |
| `TestCompiler_TaskLevelInferredScopedToTask` | `contextpkt` | Task packet only includes task-relevant inferred facts |

---

## Verification by Acceptance Criteria

### AC1: Optional enrichment
- Enrichment does not affect deterministic inspect
- Test: run inspect, enrich, inspect again — observed artifacts should be identical

### AC2: Separation of truth layers
- `inferred/facts.json` is separate from `artifacts/`
- Test: `TestFactStore_SaveAndLoad` verifies path

### AC3: Provenance
- Every fact includes rationale, confidence, run ID
- Run artifacts include request.json, inputs.json, output.json, summary.md
- Test: `TestRunStore_SaveAndLoad`, `TestRunStore_SavesSummary`

### AC4: Guide support
- Default guide excludes inferred; `--include-inferred` includes with markers
- Test: `TestMarkdownRenderer_InferredSections`, `TestMarkdownRenderer_NoInferredByDefault`

### AC5: Context compiler support
- Default packets exclude inferred; `--include-inferred` includes with scope filtering
- Test: `TestCompiler_ProjectLevelWithInferred`, `TestCompiler_ProjectLevelDefaultExcludesInferred`

### AC6: Scope discipline
- Task-level packets only include task-relevant inferred facts
- Test: `TestCompiler_TaskLevelInferredScopedToTask`

### AC7: Multi-project isolation
- Inferred facts from project A never appear in project B
- Test: multi-project isolation integration test

### AC8: Reviewability
- Facts can be inspected, accepted, rejected
- Test: `TestFactStore_UpdateStatus`

### AC9: Zero repo pollution
- Enrichment writes nothing to the target repo
- Test: integration test verifies `git status --porcelain` is empty after enrichment

### AC10: Staleness expiry
- Facts older than 30 days excluded from guide/context even with `--include-inferred`
- Test: `TestStaleness_FilterExpired`, staleness integration test

---

## Edge Cases

| Test | Setup | Expected Behavior |
|------|-------|-------------------|
| No observed facts | Run enrich before inspect | Error: "No observed facts found" |
| Stale observed facts | Inspect, commit changes, enrich | Warning about SHA mismatch, enrichment proceeds |
| Expired inferred facts | Backdate facts 45 days | Guide/context excludes expired facts, warns |
| Partial enrichment failure | Mock one category to fail | Other categories succeed, failure reported |
| Corrupted facts.json | Write invalid JSON to facts.json | Clear parse error |
| Accept then supersede | Accept a fact, re-enrich without it | Fact marked superseded |
| Dry run | `--dry-run` flag | Facts produced to stdout, nothing written |
| Missing inferred directory | First enrichment run | Directory created automatically |

---

## Integration Tests

### Full enrichment flow (`TestIntegration_FullEnrichmentFlow`)

1. Create temp workspace with fixture repo
2. Run deterministic inspect
3. Run enrichment with mock enricher
4. Verify `inferred/facts.json` exists with expected facts
5. Verify `inferred/runs/<run-id>/` has all artifacts
6. Accept one fact, reject another
7. Re-run enrichment
8. Verify accepted retained, rejected re-proposed as proposed
9. Render guide with/without `--include-inferred`
10. Compile context with/without `--include-inferred`
11. Verify no repo writes

### Multi-project isolation (`TestIntegration_MultiProjectIsolation`)

1. Create two fixture repos, enrich both
2. Verify project A's inferred facts absent from project B's guide and packets

### Staleness expiry (`TestIntegration_StalenessExpiry`)

1. Enrich, backdate facts to 45 days
2. Guide with `--include-inferred` excludes expired facts
3. Re-enrich, verify fresh facts appear

---

## Cost Tracking Verification

| Check | Method |
|-------|--------|
| Per-category cost captured | Inspect `CategoryResult.CostUSD` in run output.json |
| Per-category tokens captured | Inspect `CategoryResult.InputTokens`, `OutputTokens` |
| Aggregate cost in run summary | Inspect `EnrichmentRun.CostUSD` |
| summary.md includes cost | Read summary.md, verify cost line present |

---

## Design Principle Verification

### 1. Interfaces and contracts
Every cross-package boundary uses an interface. `CategoryEnricher` is the key new interface.

```bash
go vet ./internal/next/enrich/...
```

### 2. Knowledge categories are explicit
Every inferred fact has a non-empty `category` field from the fixed set.

### 3. No repo writes
Before and after enrichment, `git status --porcelain` in the target repo returns empty output.

### 4. Deterministic before probabilistic
Running inspect before and after enrichment produces identical observed artifacts.

### 5. Separation of slow and fast layers
Inferred facts in `inferred/`, declared/observed facts in `artifacts/` and `doctrine/`.

---

## CI Checklist

Before merging, all of these must pass:

- [ ] `go test ./internal/next/... -v` — all unit tests pass
- [ ] `go test ./internal/next/enrich/ -run TestIntegration -v` — enrichment integration tests pass
- [ ] `go vet ./internal/next/...` — no vet warnings
- [ ] `go vet ./cmd/gromit-next/...` — no vet warnings
- [ ] `go test -race ./internal/next/...` — no data races
- [ ] `go mod tidy` then `git diff --exit-code go.mod go.sum` — module files clean
- [ ] `go build ./cmd/gromit-next/` — binary builds
- [ ] Manual: enrich a real repo, review inferred facts, accept/reject — all work
- [ ] Manual: guide renders with and without `--include-inferred` correctly
- [ ] Manual: context packets include/exclude inferred facts correctly
- [ ] Manual: no files written to target repo
- [ ] Manual: two projects enriched, fully isolated
