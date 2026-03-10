# Gromit Next — Spec 0001 Verification Plan

**Goal:** Confirm the implementation satisfies every user story and design principle from the spec.

**Run all tests:** `go test ./internal/next/... -v`
**Run all lints:** `go vet ./internal/next/... && go vet ./cmd/gromit-next/...`

---

## Foundation Packages

These packages underpin all user stories and must be tested independently.

### `fact` package

| Test | Verifies |
|------|----------|
| `TestCategory_String` | String representation of declared/observed/inferred categories |
| `TestNewFact` | Fact constructor sets category, source, and content correctly |
| `TestCategory_JSONRoundTrip` | Category serializes as string in JSON and round-trips correctly |

### `artifact` package

| Test | Verifies |
|------|----------|
| `TestJSONStore_WriteAndRead` | Round-trip write then read returns identical artifact |
| `TestJSONStore_Exists` | Exists returns true after write, false before |
| `TestJSONStore_WritesCorrectPath` | Artifact file lands at expected filesystem path |

### `workspace` package

| Test | Verifies |
|------|----------|
| `TestEnvResolver_GROMIT_HOME` | `GROMIT_HOME` env var takes precedence |
| `TestEnvResolver_XDG` | Falls back to `XDG_DATA_HOME` when `GROMIT_HOME` is unset |
| `TestEnvResolver_Default` | Falls back to `~/.local/share/gromit` when both are unset |
| `TestRoot_ProjectsDir` | Root exposes a `projects/` subdirectory path |
| `TestRoot_ProjectCell` | Root can resolve a named project cell path |

### `doctrine` package

| Test | Verifies |
|------|----------|
| `TestFSStore_SaveAndLoad` | Round-trip save then load returns identical rules |
| `TestFSStore_LoadEmpty` | Loading from empty directory returns empty slice, no error |
| `TestRule_SourceAlwaysDeclared` | Every Rule's source category is `declared` |

### `architecture` package

| Test | Verifies |
|------|----------|
| `TestModule_String` | Module struct fields |
| `TestDependency_Directions` | Dependency from/to relationship |
| `TestArchitecture_AddModule` | Module addition |
| `TestArchitecture_AddDependency` | Dependency addition |
| `TestNew_InitializesEmptySlices` | Nil-safe initialization |

### `validation` package

| Test | Verifies |
|------|----------|
| `TestCommand_String` | Command struct fields and kind |
| `TestKind_String` | String representation of test/lint/build kinds |
| `TestCommandSet_Add` | Adding commands to set |
| `TestCommandSet_ByKind` | Filtering commands by kind |
| `TestNewCommandSet_InitializesEmpty` | Nil-safe initialization |

### `sourcemap` package

| Test | Verifies |
|------|----------|
| `TestBuildFromFacts` | Source map construction from file-tree facts |

### `infer` package

| Test | Verifies |
|------|----------|
| `TestStubInferrer_ReturnsEmpty` | Stub returns empty inferred facts |
| `TestStubInferrer_ImplementsInferrer` | Compile-time interface check |

---

## Verification by User Story

### Story 1: Attach a project

**What to verify:**
- `gromit-next project attach /path/to/repo --name foo` creates a project cell at `$GROMIT_HOME/projects/foo/`
- `project.json` contains correct name, absolute repo path, and timestamp
- Subdirectories exist: `artifacts/`, `doctrine/`, `provenance/`, `guide/`
- No files are written to the target repo
- Attaching to a non-git directory fails with a clear error
- Attaching a duplicate name fails with a clear error
- Attaching two different repos with different names succeeds (isolation)

**Tests:**
| Test | Package | Verifies |
|------|---------|----------|
| `TestFSStore_CreateAndGet` | `projectcell` | Cell creation and retrieval |
| `TestFSStore_CreateDuplicate` | `projectcell` | Duplicate rejection |
| `TestFSStore_CreateNonGitRepo` | `projectcell` | Git validation |
| `TestFSStore_CreateBuildsDirectoryStructure` | `projectcell` | Subdirectory scaffold |
| `TestFSStore_List` | `projectcell` | Multi-project listing |
| `TestFSStore_Delete` | `projectcell` | Clean removal |

**Manual check:** Build the binary first: `go build -o gromit-next ./cmd/gromit-next/`. After `project attach`, run `ls -R $GROMIT_HOME/projects/foo/` and confirm structure. Run `git status` in the target repo and confirm no changes.

---

### Story 2: Inspect a project

**What to verify:**
- `gromit-next project inspect foo` produces artifacts in `artifacts/` subdirectory
- Phase 1 (extraction) produces observed facts from: file tree, go.mod, Makefile, CI config
- Phase 2 (inference) produces inferred facts: architecture summary, glossary, risk areas, invariants
- Every fact is tagged with its source category (declared/observed/inferred)
- Provenance is recorded for each artifact with git SHA and timestamp
- Re-inspection with same repo HEAD skips work (freshness check)
- Re-inspection with new commits updates artifacts

**Tests:**
| Test | Package | Verifies |
|------|---------|----------|
| `TestFileTreeExtractor_Extract` | `extract` | File inventory extraction |
| `TestFileTreeExtractor_Name` | `extract` | Extractor identity |
| `TestGoModExtractor_Extract` | `extract` | Module/dependency extraction |
| `TestGoModExtractor_NoGoMod` | `extract` | Graceful handling when go.mod is missing |
| `TestGoModExtractor_Name` | `extract` | Extractor identity |
| `TestValidationCommandsExtractor_Makefile` | `extract` | Makefile target extraction |
| `TestValidationCommandsExtractor_CIWorkflow` | `extract` | CI workflow step extraction |
| `TestValidationCommandsExtractor_NoFiles` | `extract` | Graceful handling when no config files exist |
| `TestValidationCommandsExtractor_Name` | `extract` | Extractor identity |
| `TestDefaultInspector_Inspect` | `inspect` | Extract-then-infer pipeline |
| `TestFSTracker_RecordAndCheck` | `provenance` | Provenance recording |
| `TestFSTracker_IsFresh` | `provenance` | Artifact freshness |
| `TestFSTracker_CheckMissing` | `provenance` | Returns error for unknown artifact |

**Extractor coverage matrix:**

| Source File | Extractor | Facts Produced |
|-------------|-----------|----------------|
| File system | `file-tree` | File paths, languages, line counts |
| `go.mod` | `go-module` | Module path, Go version, dependencies |
| `Makefile` | `validation-commands` | Build/test/lint targets |
| `.github/workflows/*.yml` | `validation-commands` | CI run steps |

**Manual check:** Run `gromit-next project inspect foo`, then `cat $GROMIT_HOME/projects/foo/artifacts/sourcemap.json` and verify it reflects the actual repo. Check `provenance/provenance.json` for SHA and timestamps.

---

### Story 3: Get an agent guide

**What to verify:**
- `gromit-next project guide foo` produces `guide/agent-guide.md` in the project cell
- The guide contains structured sections with consistent headings
- Sections with no data are omitted (not rendered empty)
- The guide is parseable by an LLM agent (structured markdown, no ambiguous formatting)
- The guide includes: project overview, architecture, source map, validation, risky areas, invariants, glossary, doctrine

**Tests:**
| Test | Package | Verifies |
|------|---------|----------|
| `TestMarkdownRenderer_Render` | `guide` | Section rendering from input |
| `TestMarkdownRenderer_OmitsEmptySections` | `guide` | Empty section omission |

**Section checklist:**

| Section | Source Artifact | Required? |
|---------|----------------|-----------|
| Project Overview | `project.json` + inferred | Yes (always present) |
| Architecture | `architecture.json` | If data exists |
| Source Map | `sourcemap.json` | If data exists |
| Validation | `validation.json` | If data exists |
| Risky Areas | `risks.json` | If data exists |
| Invariants | `risks.json` | If data exists |
| Glossary | `glossary.json` | If data exists |
| Doctrine | `doctrine/rules.json` | If data exists |

**Manual check:** Run `gromit-next project guide foo`, open the generated markdown, and confirm it accurately describes the repo. Feed the guide to an LLM and ask it to summarize the project — it should produce an accurate summary.

---

### Story 4: Compile context

**What to verify:**
- `gromit-next context build foo --level project` produces a project-level packet
- `gromit-next context build foo --level spec --spec path/to/spec.md` produces a spec-level packet
- `gromit-next context build foo --level task --spec path --task id` produces a task-level packet
- Each level selects relevant facts independently (not cumulative)
- Token budget is respected when set
- Packet includes section names, content, token estimates, and fact references
- Missing required flags (spec path for spec level, task ID for task level) produce clear errors

**Tests:**
| Test | Package | Verifies |
|------|---------|----------|
| `TestCompiler_ProjectLevel` | `contextpkt` | Project packet sections |
| `TestCompiler_SpecLevel` | `contextpkt` | Spec-scoped relevance selection — must assert presence of `spec-text` section |
| `TestCompiler_TaskLevel` | `contextpkt` | Task-scoped relevance + proof requirements — must assert presence of `proof-requirements` section |
| `TestCompiler_TokenBudget` | `contextpkt` | Budget enforcement |
| `TestCompiler_SpecLevelMissingSpecPath` | `contextpkt` | Returns error when spec path is empty |
| `TestCompiler_TaskLevelMissingTaskID` | `contextpkt` | Returns error when task ID is empty |

**Level content matrix:**

| Section | Project | Spec | Task |
|---------|---------|------|------|
| Architecture (full) | Yes | - | - |
| Architecture (scoped) | - | Yes | Yes |
| Doctrine | Yes | Relevant subset | Relevant subset |
| Glossary | Yes | Relevant terms | Relevant terms |
| Validation commands | Yes | Scoped commands | Task-specific commands |
| Spec text | - | Yes | Yes |
| Task scope / proof requirements | - | - | Yes |
| Risks (scoped) | - | Yes | Yes |

**Manual check:** Compile a project-level context, then a task-level context for the same project. The task-level packet should be smaller and more focused.

---

### Story 5: Keep projects isolated

**What to verify:**
- Two attached projects have separate cell directories
- Inspection of project A does not affect project B's artifacts
- Context compilation for project A does not include project B's facts
- Deleting project A does not affect project B

**Tests:**
| Test | Package | Verifies |
|------|---------|----------|
| `TestFSStore_List` | `projectcell` | Multiple projects coexist |
| `TestFSStore_Delete` | `projectcell` | Deletion is scoped |
| Integration test | `integration_test.go` | Full flow with multiple projects |

**Manual check:** Attach two repos. Inspect both. Verify `$GROMIT_HOME/projects/` contains two separate directories with independent artifacts.

---

### Edge Cases

These tests verify graceful behavior under abnormal conditions.

| Test | Setup | Expected Behavior |
|------|-------|-------------------|
| Corrupted JSON (project) | Write invalid JSON to `project.json` | Command produces clear parse error |
| Corrupted JSON (artifact) | Write invalid JSON to an artifact file | Command produces clear parse error |
| Missing subdirectories | Delete `artifacts/` after creation | `inspect` produces clear error about missing directory |
| Nonexistent path | `gromit-next project attach /nonexistent --name foo` | Fails with clear "path does not exist" error |
| Empty repo | Attach a git repo with zero commits | `inspect` handles gracefully (no panic, clear message) |

---

## Verification by Design Principle

### 1. Interfaces and contracts

**Check:** Every cross-package import uses an interface, not a concrete type.

```bash
# General hygiene check
go vet ./internal/next/...
```

Manual review: check that each package's exported API exposes interfaces and constructor functions, not concrete struct types. Verify that cross-package imports depend on interfaces.

### 2. Knowledge categories are explicit

**Check:** Every `Fact` in every artifact file has a non-empty `category` field.

```bash
# After running inspect, check all artifact files
for f in $GROMIT_HOME/projects/foo/artifacts/*.json; do jq '.. | .category? // empty' "$f"; done | sort -u
```

Expected output: only `"declared"`, `"observed"`, `"inferred"`.

### 3. Relevance before budgeting

**Check:** Compile a spec-level context with no token budget. Verify it includes only spec-relevant facts (not the full project). Then compile with a budget and verify it's a subset of the unbounded result.

### 4. No repo writes

**Check:** Before and after every command, run `git status` in the target repo. There should be zero changes.

```bash
cd /path/to/target/repo && git status --porcelain
# Expected: empty output
```

### 5. Isolation by default

**Check:** Covered by Story 5 tests above.

### 6. Deterministic before probabilistic

**Check:** Run `project inspect` twice on the same repo without changes. The observed facts should be byte-identical. Inferred facts may vary (LLM non-determinism) but should be structurally consistent.

```bash
# Copy artifact before re-inspect
cp $GROMIT_HOME/projects/foo/artifacts/sourcemap.json /tmp/sourcemap-before.json
# Run inspect again
gromit-next project inspect foo
# Diff after
diff /tmp/sourcemap-before.json $GROMIT_HOME/projects/foo/artifacts/sourcemap.json
# Expected: no differences for observed artifacts
```

---

## End-to-End Integration Test

The integration test (`internal/next/integration_test.go`) runs the complete flow:

1. Create temp workspace via `GROMIT_HOME` env var
2. Create a fixture git repo with known files (go.mod, Makefile, main.go, internal/ structure)
3. Attach the fixture repo as a project
4. Run inspect with a stub inferrer (no real LLM calls)
5. Render the agent guide
6. Compile context at all three levels
7. Run inspect again without repo changes, verify artifacts are not regenerated (provenance freshness check)
8. Attach a second fixture repo, inspect it, verify first project's artifacts are unchanged (isolation)
9. Assert:
   - All artifact files exist in the project cell
   - Provenance records exist for each artifact
   - Agent guide contains expected section headings
   - Project packet contains architecture and doctrine sections
   - Spec packet contains spec text section
   - Task packet contains proof requirements section
   - No files were written to the fixture repo
   - Re-inspect without changes does not regenerate artifacts (freshness)
   - Second project's inspection does not modify first project's artifacts (isolation)

**Run:** `go test ./internal/next/ -run TestIntegration -v`

---

## CI Checklist

Before merging, all of these must pass:

- [ ] `go test ./internal/next/... -v` — all unit tests pass
- [ ] `go test ./internal/next/ -run TestIntegration -v` — integration test passes
- [ ] `go vet ./internal/next/...` — no vet warnings
- [ ] `go vet ./cmd/gromit-next/...` — no vet warnings
- [ ] `go test -race ./internal/next/...` — no data races
- [ ] `go mod tidy` then `git diff --exit-code go.mod go.sum` — module files are clean
- [ ] `go build ./cmd/gromit-next/` — binary builds
- [ ] Manual: attach a real repo, inspect, render guide, compile context — all produce reasonable output
- [ ] Manual: verify no files written to target repo
- [ ] Manual: verify two projects are fully isolated
