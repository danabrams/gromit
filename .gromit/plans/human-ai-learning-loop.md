---
id: human-ai-learning-loop
source_spec: human-ai-learning-loop
created: 2026-03-01
decomposed: false
---

# Human-AI Learning Loop Implementation Plan

**Goal:** Extend `gromit retro` with a Workmanship Report section that surfaces codebase friction areas from accumulated learnings, presents improvement options, and confirms whether past interventions resolved friction.

**Architecture:** Programmatic friction clustering (area extraction from learning content, evidence gathering) feeds into the retro prompt template as a new section. The LLM generates the Workmanship Report with options per friction area. Friction history is persisted for the confirmation loop.

**Tech Stack:** Go, existing retro/learnings/logger packages

**Spec:** `.gromit/specs/human-ai-learning-loop.md`

---

## Architecture

**Overview:**
Add friction clustering as a programmatic pre-processing step in `internal/retro/`, feed clustered friction data into the retro prompt template as a new section, and let the LLM generate the Workmanship Report with options. Store friction history for the confirmation loop.

**Key Components:**

1. **`internal/retro/friction.go`** — Friction detection and clustering logic. Extracts codebase areas from learning content (Go package paths, file paths, module names), clusters learnings by area, gathers evidence per cluster (count, timespan, category distribution).

2. **`internal/retro/workmanship.go`** — Workmanship Report types and history management. FrictionCluster struct, WorkmanshipHistory for confirmation loop, JSON persistence.

3. **Extended `TemplateContext`** — New fields for friction clusters and previous friction history.

4. **Extended retro prompt template** — New "Workmanship Report" section with LLM instructions for option generation.

5. **Extended structured output** — New `workmanship` key in JSON output for friction areas and options.

**Integration Points:**
- `Retro.Run()` calls friction clustering after loading learnings, before rendering the template
- Friction clusters added to `TemplateContext` alongside existing data
- LLM output parsing extended to handle workmanship proposals
- History file updated after each retro run

**Data Flow:**
```
Learnings (confirmed + provisional)
    ↓ extractArea() — parse content for package/file references
    ↓ clusterByArea() — group learnings with same area
    ↓ gatherEvidence() — count, timespan, rework correlation
    ↓
FrictionClusters → TemplateContext → Prompt Template → LLM Analysis
                                                          ↓
                                    Workmanship Report (options per friction area)
                                                          ↓
                                    workmanship_history.json (for confirmation loop)
```

**Files to Modify:**
- `internal/retro/retro.go` — Add friction clustering call in `Run()`, extend `TemplateContext`
- `internal/retro/proposals.go` — Add workmanship proposal types and parsing
- `.gromit/templates/PROMPT_retro.md` — Add Workmanship Report section

**Files to Create:**
- `internal/retro/friction.go` — Area extraction, clustering, evidence gathering
- `internal/retro/friction_test.go` — Tests for friction detection
- `internal/retro/workmanship.go` — History types, load/save
- `internal/retro/workmanship_test.go` — Tests for history management

**Tradeoffs:**
- **Area extraction from content vs. adding Area field to Learning struct**: Chose content parsing to avoid modifying the learning data model and migration complexity. Extraction is heuristic-based but avoids structural changes.
- **LLM-generated options vs. programmatic options**: Chose LLM generation because the LLM has codebase context and can propose creative refactoring strategies.
- **History in JSON file vs. in LEARNINGS.md**: Chose separate JSON file to keep concerns separated.
- **Friction clustering threshold**: Configurable minimum (default 2+ learnings in same area) to filter noise.

## Test Strategy

**Test Levels:**

1. **Unit Tests**: Core friction detection logic — area extraction, clustering, evidence gathering, history load/save
2. **Integration Tests**: End-to-end flow through `Retro.Run()` with friction data in template context, proposal parsing of workmanship output
3. **Manual Testing**: Run `gromit retro` on a real codebase with accumulated learnings to verify report quality

**Key Test Cases:**
- `TestExtractArea_GoPackagePath`: Extracts "internal/runner" from learning content mentioning that package
- `TestExtractArea_FilePath`: Extracts area from file path references in content
- `TestExtractArea_CrossCutting`: Returns "cross-cutting" when no specific area detected
- `TestClusterByArea_GroupsCorrectly`: Groups learnings with same extracted area
- `TestClusterByArea_MinimumThreshold`: Excludes clusters with fewer than 2 learnings
- `TestClusterByArea_Empty`: Returns empty clusters for no learnings
- `TestGatherEvidence_CountAndTimespan`: Correct learning count and date range per cluster
- `TestWorkmanshipHistory_SaveLoad`: Round-trips history to/from JSON file
- `TestWorkmanshipHistory_ConfirmationLoop`: Compares current vs. previous friction, detects resolved areas
- `TestWorkmanshipHistory_EmptyFile`: Handles missing history file gracefully
- `TestTemplateContext_IncludesFriction`: Friction clusters appear in rendered template context
- `TestParseWorkmanshipProposals`: Parses LLM JSON output containing workmanship section
- `TestFrictionClusters_HealthyCodebase`: No clusters produces "healthy" signal

**Mocking Strategy:**
- No mocks for friction/clustering — pure functions on in-memory Learning slices
- Temp directories for history file I/O (consistent with existing retro tests)
- Canned JSON strings for LLM output parsing (consistent with existing proposal tests)

**Test Organization:**
- `internal/retro/friction_test.go` — area extraction and clustering tests
- `internal/retro/workmanship_test.go` — history load/save and confirmation loop tests

## Implementation Tasks

### Task 1: Area Extraction from Learning Content

**Files:**
- Create: `internal/retro/friction.go`
- Test: `internal/retro/friction_test.go`

**What to Do:**
Implement `extractArea(content string) string` that parses learning content for Go package paths (e.g., `internal/runner`, `cmd/gromit`) and file paths (e.g., `retro.go`, `pipeline.go`). Use regex to find common Go path patterns. When multiple paths are found, return the most frequently mentioned or the package-level path. When no path is found, return `"cross-cutting"`.

**Acceptance Criteria:**
- Extracts Go package paths like `internal/runner` from learning content
- Extracts file-level areas from paths like `cmd/gromit/review.go` → `cmd/gromit`
- Returns `"cross-cutting"` when no codebase area is detectable

**Dependencies:** None

### Task 2: Friction Clustering Logic

**Files:**
- Modify: `internal/retro/friction.go`
- Test: `internal/retro/friction_test.go`

**What to Do:**
Implement `FrictionCluster` struct with fields: Area (string), Learnings ([]Learning references), LearningCount (int), EarliestDate (time.Time), LatestDate (time.Time), Categories (map[string]int). Implement `clusterByArea(learnings []learnings.Learning, minClusterSize int) []FrictionCluster` that groups learnings by extracted area, computes evidence fields, and filters out clusters below the minimum size threshold. Sort clusters by learning count descending.

**Acceptance Criteria:**
- Groups learnings with the same extracted area into clusters
- Excludes clusters with fewer than `minClusterSize` learnings
- Returns empty slice when no learnings or no clusters meet threshold

**Dependencies:** Task 1

### Task 3: Workmanship History Types and Persistence

**Files:**
- Create: `internal/retro/workmanship.go`
- Test: `internal/retro/workmanship_test.go`

**What to Do:**
Define `WorkmanshipReport` struct (Date, FrictionAreas []FrictionCluster) and `WorkmanshipHistory` struct (Reports []WorkmanshipReport). Implement `LoadWorkmanshipHistory(path string) (*WorkmanshipHistory, error)` and `SaveWorkmanshipHistory(path string, history *WorkmanshipHistory) error` for JSON persistence. Implement `FindPreviousFriction(area string) *FrictionCluster` to look up a friction area from the most recent previous report. Handle missing/empty file gracefully (return empty history).

**Acceptance Criteria:**
- Round-trips WorkmanshipHistory to/from JSON file correctly
- Handles missing or empty history file without error
- FindPreviousFriction returns nil when area not found in previous reports

**Dependencies:** Task 2 (uses FrictionCluster type)

### Task 4: Confirmation Loop — Friction Resolution Detection

**Files:**
- Modify: `internal/retro/workmanship.go`
- Test: `internal/retro/workmanship_test.go`

**What to Do:**
Implement `CompareFriction(current []FrictionCluster, previous *WorkmanshipReport) []FrictionResolution` that compares current friction clusters against the most recent previous report. For each previously-identified friction area, determine if it's resolved (no longer appears or learning count decreased), persisting (still present with similar or higher count), or new (not in previous report). `FrictionResolution` struct: Area, Status (resolved/persisting/new), PreviousCount, CurrentCount.

**Acceptance Criteria:**
- Detects resolved friction areas (present before, absent or reduced now)
- Detects persisting friction areas (still present with similar or higher count)
- Handles case where no previous report exists (all areas are "new")

**Dependencies:** Task 3

### Task 5: Extend TemplateContext and Retro.Run() Integration

**Files:**
- Modify: `internal/retro/retro.go`
- Modify: `internal/retro/prompt_helpers.go` (token estimation)
- Test: `internal/retro/friction_test.go` (integration test)

**What to Do:**
Add `FrictionClusters []FrictionCluster` and `FrictionResolutions []FrictionResolution` fields to `TemplateContext`. In `Retro.Run()`, after loading learnings, call `clusterByArea()` on combined confirmed + recent provisional learnings. Load workmanship history, compute friction resolutions via `CompareFriction()`. Populate new TemplateContext fields. After retro completes, save updated workmanship history with current friction clusters.

**Acceptance Criteria:**
- FrictionClusters populated in TemplateContext when learnings have area clusters
- FrictionResolutions populated when previous workmanship history exists
- Workmanship history file updated after each retro run

**Dependencies:** Tasks 2, 4

### Task 6: Workmanship Report Prompt Template Section

**Files:**
- Modify: `.gromit/templates/PROMPT_retro.md`

**What to Do:**
Add a new "Section 8: Workmanship Report" after the existing task section. Template should render friction clusters with evidence (area, learning count, timespan, categories), friction resolutions from previous retros, and instruct the LLM to: generate 2-4 options per friction area with investment estimates and risk of deferral; report "codebase is healthy" when no friction clusters exist; note resolved friction areas as confirmation of successful interventions.

**Acceptance Criteria:**
- Template renders friction clusters with evidence data
- Template includes instructions for LLM option generation
- Template handles empty friction clusters (healthy codebase message)

**Dependencies:** Task 5

### Task 7: Parse Workmanship Proposals from LLM Output

**Files:**
- Modify: `internal/retro/proposals.go`
- Test: existing proposal test patterns

**What to Do:**
Add `WorkmanshipProposal` struct: Area (string), Options ([]FrictionOption), Resolution (string, if previously identified). Add `FrictionOption` struct: Title (string), Investment (string), Impact (string), Risk (string). Extend the JSON output parsing in proposals.go to extract the `workmanship` key from LLM output. Add `Workmanship []WorkmanshipProposal` field to the retro Result struct.

**Acceptance Criteria:**
- Parses `workmanship` array from LLM JSON output into WorkmanshipProposal structs
- Each proposal contains area, options with investment/impact/risk, and optional resolution status
- Gracefully handles missing `workmanship` key (backward compatible with older LLM output)

**Dependencies:** Task 6

---

## Notes

- **Area extraction is heuristic**: The initial implementation uses regex to find Go paths in learning content. This may miss some areas or miscategorize. The LLM's analysis in the Workmanship Report can compensate for extraction imprecision by reasoning about learning content directly.
- **Minimum cluster size**: Default of 2 keeps noise low. A single learning about an area isn't friction — it's an observation. Friction requires repeated observations.
- **Decision capture (spec from selected option)**: The spec mentions that selected options become specs. This is a UX concern handled in the interactive retro session — when the human selects an option, the interactive session creates a spec. This doesn't require new plumbing; the existing interactive flow can handle it via conversation.
- **Token budget**: The Workmanship Report section adds to the retro prompt size. Keep the friction data concise (summaries, not full learning text) to stay within budget.
