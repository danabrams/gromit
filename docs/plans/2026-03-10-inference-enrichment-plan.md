# Spec 0001a — Inference Enrichment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add optional LLM-based inference enrichment to the Spec 0001 project memory system, with per-project configuration, multi-provider support, cost tracking, and reviewable inferred facts.

**Architecture:** Enrichment runs multiple focused LLM passes (one per category) against observed facts. Inferred facts are stored separately in `inferred/` within the project cell. Guide rendering and context compilation optionally include clearly marked inferred content. Direct provider calls (no router) with configurable model and reasoning level.

**Tech Stack:** Go, Cobra CLI, JSON artifact storage, existing `provider.Provider` interface.

**Design doc:** `docs/plans/2026-03-10-inference-enrichment-design.md`
**Verification plan:** `docs/plans/2026-03-10-inference-enrichment-verification-plan.md`

---

## Phase 1: Inferred Fact Model and Storage

### Task 1: Inferred fact type

**Files:**
- Create: `internal/next/enrich/fact.go`
- Test: `internal/next/enrich/fact_test.go`

**Step 1: Write the failing test**

```go
package enrich

import (
	"encoding/json"
	"testing"
)

func TestInferredFactStatus_String(t *testing.T) {
	tests := []struct {
		s    Status
		want string
	}{
		{StatusProposed, "proposed"},
		{StatusAccepted, "accepted"},
		{StatusRejected, "rejected"},
		{StatusSuperseded, "superseded"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestInferredFactStatus_JSONRoundTrip(t *testing.T) {
	f := InferredFact{
		FactID:   "abc123",
		Category: CategoryComponentBoundary,
		Status:   StatusProposed,
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var f2 InferredFact
	if err := json.Unmarshal(data, &f2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if f2.Status != StatusProposed {
		t.Errorf("Status = %v, want proposed", f2.Status)
	}
	if f2.Category != CategoryComponentBoundary {
		t.Errorf("Category = %q, want %q", f2.Category, CategoryComponentBoundary)
	}
}

func TestInferredFact_ContentHash(t *testing.T) {
	f1 := InferredFact{
		Category:  CategoryComponentBoundary,
		Statement: "payments-api uses hexagonal architecture",
	}
	f2 := InferredFact{
		Category:  CategoryComponentBoundary,
		Statement: "payments-api uses hexagonal architecture",
	}
	f3 := InferredFact{
		Category:  CategoryGlossaryTerm,
		Statement: "payments-api uses hexagonal architecture",
	}

	id1 := f1.ComputeID()
	id2 := f2.ComputeID()
	id3 := f3.ComputeID()

	if id1 != id2 {
		t.Errorf("same content should produce same ID: %q != %q", id1, id2)
	}
	if id1 == id3 {
		t.Error("different category should produce different ID")
	}
	if id1 == "" {
		t.Error("ID should not be empty")
	}
}

func TestAllCategories(t *testing.T) {
	cats := AllCategories()
	if len(cats) != 8 {
		t.Errorf("expected 8 categories, got %d", len(cats))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/next/enrich/ -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package enrich

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// EnrichmentCategory identifies the type of enrichment.
type EnrichmentCategory string

const (
	CategoryComponentBoundary      EnrichmentCategory = "component_boundary"
	CategoryComponentResponsibility EnrichmentCategory = "component_responsibility"
	CategoryEntrypoint             EnrichmentCategory = "entrypoint"
	CategoryRiskyArea              EnrichmentCategory = "risky_area"
	CategoryIntegrationPoint       EnrichmentCategory = "integration_point"
	CategoryGlossaryTerm           EnrichmentCategory = "glossary_term"
	CategoryValidationSurface      EnrichmentCategory = "likely_validation_surface"
	CategoryOwnershipBoundary      EnrichmentCategory = "likely_ownership_boundary"
)

func AllCategories() []EnrichmentCategory {
	return []EnrichmentCategory{
		CategoryComponentBoundary,
		CategoryComponentResponsibility,
		CategoryEntrypoint,
		CategoryRiskyArea,
		CategoryIntegrationPoint,
		CategoryGlossaryTerm,
		CategoryValidationSurface,
		CategoryOwnershipBoundary,
	}
}

// Status tracks the review state of an inferred fact.
type Status int

const (
	StatusProposed Status = iota
	StatusAccepted
	StatusRejected
	StatusSuperseded
)

func (s Status) String() string {
	switch s {
	case StatusProposed:
		return "proposed"
	case StatusAccepted:
		return "accepted"
	case StatusRejected:
		return "rejected"
	case StatusSuperseded:
		return "superseded"
	default:
		return "unknown"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "proposed":
		*s = StatusProposed
	case "accepted":
		*s = StatusAccepted
	case "rejected":
		*s = StatusRejected
	case "superseded":
		*s = StatusSuperseded
	default:
		return fmt.Errorf("unknown status: %q", str)
	}
	return nil
}

// InferredFact represents a candidate fact proposed by an LLM.
type InferredFact struct {
	FactID         string             `json:"fact_id"`
	SourceType     string             `json:"source_type"` // always "inferred"
	Category       EnrichmentCategory `json:"category"`
	Statement      string             `json:"statement"`
	Rationale      string             `json:"rationale"`
	EvidenceRefs   []string           `json:"evidence_refs"`
	Confidence     string             `json:"confidence"`
	Scope          string             `json:"scope"`
	Status         Status             `json:"status"`
	CreatedAt      time.Time          `json:"created_at"`
	InferenceRunID string             `json:"inference_run_id"`
}

// NormalizeNilFields maps nil slices to empty values.
func (f *InferredFact) NormalizeNilFields() {
	if f.EvidenceRefs == nil {
		f.EvidenceRefs = []string{}
	}
}

// ComputeID produces a content hash from category + statement.
func (f *InferredFact) ComputeID() string {
	h := sha256.New()
	h.Write([]byte(string(f.Category)))
	h.Write([]byte(f.Statement))
	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/next/enrich/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/next/enrich/
git commit -m "feat(next): add inferred fact type with status, categories, and content hash"
```

---

### Task 2: Enrichment configuration

**Files:**
- Create: `internal/next/enrich/config.go`
- Test: `internal/next/enrich/config_test.go`

**Step 1: Write the failing test**

```go
package enrich

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Provider != "claude" {
		t.Errorf("Provider = %q, want claude", cfg.Provider)
	}
	if cfg.Model != "sonnet" {
		t.Errorf("Model = %q, want sonnet", cfg.Model)
	}
	if cfg.Reasoning != "medium" {
		t.Errorf("Reasoning = %q, want medium", cfg.Reasoning)
	}
	if cfg.StalenessExpiryDays != 30 {
		t.Errorf("StalenessExpiryDays = %d, want 30", cfg.StalenessExpiryDays)
	}
}

func TestConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Model = "opus"

	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded.Model != "opus" {
		t.Errorf("Model = %q, want opus", loaded.Model)
	}
}

func TestConfig_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Provider != "claude" {
		t.Errorf("should return defaults when file missing, got Provider=%q", cfg.Provider)
	}
}

func TestConfig_LoadCorrupted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "enrichment.json"), []byte("{invalid"), 0o644)
	_, err := LoadConfig(dir)
	if err == nil {
		t.Error("expected error for corrupted config")
	}
}
```

**Step 2: Write minimal implementation**

```go
package enrich

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds per-project enrichment configuration.
type Config struct {
	Provider            string `json:"provider"`
	Model               string `json:"model"`
	Reasoning           string `json:"reasoning"`
	StalenessExpiryDays int    `json:"staleness_expiry_days"`
}

func DefaultConfig() Config {
	return Config{
		Provider:            "claude",
		Model:               "sonnet",
		Reasoning:           "medium",
		StalenessExpiryDays: 30,
	}
}

func SaveConfig(cellPath string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cellPath, "enrichment.json"), data, 0o644)
}

func LoadConfig(cellPath string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(cellPath, "enrichment.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
```

**Step 3: Commit**

```bash
git add internal/next/enrich/
git commit -m "feat(next): add per-project enrichment configuration"
```

---

### Task 3: Inferred fact store

**Files:**
- Create: `internal/next/enrich/store.go`
- Test: `internal/next/enrich/store_test.go`

**Step 1: Write the failing test**

```go
package enrich

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFactStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred"), 0o755)
	store := NewFactStore()

	facts := []InferredFact{
		{
			FactID:    "abc123",
			Category:  CategoryEntrypoint,
			Statement: "main.go is the primary entrypoint",
			Status:    StatusProposed,
			CreatedAt: time.Now(),
		},
	}

	if err := store.SaveFacts(dir, facts); err != nil {
		t.Fatalf("SaveFacts: %v", err)
	}

	loaded, err := store.LoadFacts(dir)
	if err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(loaded))
	}
	if loaded[0].Statement != "main.go is the primary entrypoint" {
		t.Errorf("Statement = %q", loaded[0].Statement)
	}
}

func TestFactStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred"), 0o755)
	store := NewFactStore()

	facts, err := store.LoadFacts(dir)
	if err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(facts))
	}
}

func TestFactStore_MergeStatuses(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred"), 0o755)
	store := NewFactStore()

	// Save initial facts with accepted status
	existing := []InferredFact{
		{FactID: "abc", Status: StatusAccepted, Category: CategoryEntrypoint, Statement: "main"},
		{FactID: "def", Status: StatusRejected, Category: CategoryRiskyArea, Statement: "risky"},
	}
	store.SaveFacts(dir, existing)

	// New enrichment produces overlapping facts
	incoming := []InferredFact{
		{FactID: "abc", Status: StatusProposed, Category: CategoryEntrypoint, Statement: "main"},
		{FactID: "ghi", Status: StatusProposed, Category: CategoryGlossaryTerm, Statement: "term"},
	}

	merged := store.MergeWithExisting(existing, incoming)

	// abc should retain accepted status
	for _, f := range merged {
		if f.FactID == "abc" && f.Status != StatusAccepted {
			t.Errorf("abc should retain accepted status, got %v", f.Status)
		}
		// def should be superseded (not in incoming)
		if f.FactID == "def" && f.Status != StatusSuperseded {
			t.Errorf("def should be superseded, got %v", f.Status)
		}
		// ghi should be proposed
		if f.FactID == "ghi" && f.Status != StatusProposed {
			t.Errorf("ghi should be proposed, got %v", f.Status)
		}
	}
}

func TestFactStore_UpdateStatus(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred"), 0o755)
	store := NewFactStore()

	facts := []InferredFact{
		{FactID: "abc", Status: StatusProposed},
		{FactID: "def", Status: StatusProposed},
	}
	store.SaveFacts(dir, facts)

	if err := store.UpdateStatus(dir, "abc", StatusAccepted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	loaded, _ := store.LoadFacts(dir)
	for _, f := range loaded {
		if f.FactID == "abc" && f.Status != StatusAccepted {
			t.Errorf("abc should be accepted, got %v", f.Status)
		}
	}
}
```

**Step 2: Write minimal implementation**

The `FactStore` manages `inferred/facts.json` — save, load, merge statuses across re-enrichment, and update individual fact statuses. `MergeWithExisting` preserves accepted statuses for matching IDs and marks missing accepted facts as superseded.

**Step 3: Commit**

```bash
git add internal/next/enrich/
git commit -m "feat(next): add inferred fact store with merge and status update"
```

---

### Task 4: Run artifact storage

**Files:**
- Create: `internal/next/enrich/run.go`
- Test: `internal/next/enrich/run_test.go`

**Step 1: Write the failing test**

```go
package enrich

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)
	store := NewRunStore()

	run := EnrichmentRun{
		RunID:     "run-001",
		Timestamp: time.Now(),
		Provider:  "claude",
		Model:     "sonnet",
		Reasoning: "medium",
		Inputs:    EnrichInput{ProjectName: "test-project", FileTree: []string{"main.go", "go.mod"}},
		Request:   RunRequest{Categories: AllCategories()},
		Results:   []CategoryResult{},
		CostUSD:   0.05,
		InputTokens:  1000,
		OutputTokens: 500,
	}

	if err := store.SaveRun(dir, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	loaded, err := store.LoadRun(dir, "run-001")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if loaded.RunID != "run-001" {
		t.Errorf("RunID = %q, want run-001", loaded.RunID)
	}
	if loaded.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want 0.05", loaded.CostUSD)
	}

	// Verify inputs.json was written
	inputsPath := filepath.Join(dir, "inferred", "runs", "run-001", "inputs.json")
	inputsData, err := os.ReadFile(inputsPath)
	if err != nil {
		t.Fatalf("inputs.json not written: %v", err)
	}
	if len(inputsData) == 0 {
		t.Error("inputs.json should not be empty")
	}
}

func TestRunStore_ListRuns(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)
	store := NewRunStore()

	for _, id := range []string{"run-001", "run-002"} {
		store.SaveRun(dir, EnrichmentRun{RunID: id, Timestamp: time.Now()})
	}

	runs, err := store.ListRuns(dir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}
}

func TestRunStore_SavesSummary(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)
	store := NewRunStore()

	run := EnrichmentRun{
		RunID:     "run-001",
		Timestamp: time.Now(),
		Provider:  "claude",
		Model:     "sonnet",
		CostUSD:   0.12,
		InputTokens:  5000,
		OutputTokens: 2000,
		Results: []CategoryResult{
			{Category: CategoryEntrypoint, Success: true, FactCount: 3},
			{Category: CategoryRiskyArea, Success: false, Error: "timeout"},
		},
	}

	store.SaveRun(dir, run)

	summaryPath := filepath.Join(dir, "inferred", "runs", "run-001", "summary.md")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("summary.md not written: %v", err)
	}
	summary := string(data)
	if len(summary) == 0 {
		t.Error("summary.md should not be empty")
	}
}
```

**Step 2: Write minimal implementation**

The `RunStore` manages `inferred/runs/<run-id>/` directories. Each run directory contains:
- `inputs.json` — what the LLM saw (the `EnrichInput` data)
- `request.json` — what categories were requested
- `output.json` — full run data including per-category results and cost
- `summary.md` — human-readable summary with provider, model, cost, token counts, success/failure per category

Types needed:

```go
type EnrichmentRun struct {
	RunID        string           `json:"run_id"`
	Timestamp    time.Time        `json:"timestamp"`
	Provider     string           `json:"provider"`
	Model        string           `json:"model"`
	Reasoning    string           `json:"reasoning"`
	Inputs       EnrichInput      `json:"inputs"`
	Request      RunRequest       `json:"request"`
	Results      []CategoryResult `json:"results"`
	CostUSD      float64          `json:"cost_usd"`
	InputTokens  int              `json:"input_tokens"`
	OutputTokens int              `json:"output_tokens"`
}

type RunRequest struct {
	Categories []EnrichmentCategory `json:"categories"`
}

type CategoryResult struct {
	Category     EnrichmentCategory `json:"category"`
	Success      bool               `json:"success"`
	FactCount    int                `json:"fact_count"`
	Error        string             `json:"error,omitempty"`
	CostUSD      float64            `json:"cost_usd"`
	InputTokens  int                `json:"input_tokens"`
	OutputTokens int                `json:"output_tokens"`
}
```

**Step 3: Commit**

```bash
git add internal/next/enrich/
git commit -m "feat(next): add enrichment run storage with cost tracking and summary"
```

---

## Phase 2: Enrichment Engine

### Task 5: Category enricher interface and mock

**Depends on:** Task 1

**Files:**
- Create: `internal/next/enrich/enricher.go`
- Test: `internal/next/enrich/enricher_test.go`

**Step 1: Write the failing test**

```go
package enrich

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
)

func TestMockEnricher_ImplementsInterface(t *testing.T) {
	var _ CategoryEnricher = (*MockEnricher)(nil)
}

func TestMockEnricher_ReturnsConfiguredFacts(t *testing.T) {
	mock := &MockEnricher{
		Facts: []InferredFact{
			{FactID: "test-1", Category: CategoryEntrypoint, Statement: "main.go"},
		},
	}

	result, err := mock.Enrich(context.Background(), CategoryEntrypoint, []fact.Fact{}, EnrichInput{})
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if result.FactCount != 1 {
		t.Errorf("FactCount = %d, want 1", result.FactCount)
	}
	if len(result.Facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(result.Facts))
	}
}
```

**Step 2: Write minimal implementation**

```go
package enrich

import (
	"context"

	"github.com/danabrams/gromit/internal/next/fact"
	"github.com/danabrams/gromit/internal/provider"
)

// EnrichInput holds the context provided to an enrichment pass.
type EnrichInput struct {
	ProjectName  string
	FileTree     []string
	Architecture string // JSON string of architecture artifact
	Doctrine     string // JSON string of doctrine artifact
	SourceMap    string // JSON string of sourcemap artifact
	Validation   string // JSON string of validation artifact
	Glossary     string // JSON string of glossary artifact
}

// EnrichResult holds the output of a single category enrichment pass.
type EnrichResult struct {
	Category     EnrichmentCategory
	Facts        []InferredFact
	FactCount    int
	Success      bool
	Error        string
	CostUSD      float64
	InputTokens  int
	OutputTokens int
}

// CategoryEnricher runs a single enrichment pass for a specific category.
type CategoryEnricher interface {
	Enrich(ctx context.Context, category EnrichmentCategory, observed []fact.Fact, input EnrichInput) (EnrichResult, error)
}

// LLMEnricher uses a provider.Provider to run enrichment.
type LLMEnricher struct {
	provider  provider.Provider
	model     string
	reasoning string
}

func NewLLMEnricher(p provider.Provider, model, reasoning string) *LLMEnricher {
	return &LLMEnricher{provider: p, model: model, reasoning: reasoning}
}

// MockEnricher returns preconfigured results for testing.
type MockEnricher struct {
	Facts []InferredFact
	Err   error
}

func (m *MockEnricher) Enrich(ctx context.Context, category EnrichmentCategory, observed []fact.Fact, input EnrichInput) (EnrichResult, error) {
	if m.Err != nil {
		return EnrichResult{Category: category, Success: false, Error: m.Err.Error()}, m.Err
	}
	return EnrichResult{
		Category:  category,
		Facts:     m.Facts,
		FactCount: len(m.Facts),
		Success:   true,
	}, nil
}
```

**Step 3: Commit**

```bash
git add internal/next/enrich/
git commit -m "feat(next): add category enricher interface with mock implementation"
```

---

### Task 6: LLM enricher implementation

**Depends on:** Task 5

**Files:**
- Modify: `internal/next/enrich/enricher.go`
- Create: `internal/next/enrich/prompts.go`
- Test: `internal/next/enrich/enricher_integration_test.go`

**Step 1: Write the failing test**

Test that `LLMEnricher.Enrich` calls the provider with a well-formed prompt containing the category name and observed facts, parses the JSON response into `InferredFact` structs, and captures cost/token data from the provider `Result`.

Use a fake provider that validates the prompt and returns canned JSON.

**Step 2: Write implementation**

- `prompts.go`: Per-category prompt templates. Each category has a function that builds a prompt from `EnrichInput` + observed facts. Prompts instruct the LLM to return structured JSON with `statement`, `rationale`, `evidence_refs`, `confidence`, `scope` fields.
- `enricher.go`: `LLMEnricher.Enrich` builds the prompt, converts the model name to a provider tier via `provider.TierFromLegacyModel(model)` (e.g., "sonnet" becomes "medium"), and calls `provider.Run()` with that tier. Reasoning effort is NOT a parameter to `provider.Run()` -- it is passed to providers that support it (e.g., Codex) via provider-specific configuration, and stored in `EnrichmentRun` for provenance. The method parses the JSON response, assigns content-hash IDs, and populates `EnrichResult` with cost/token data from `provider.Result`.

**Step 3: Commit**

```bash
git add internal/next/enrich/
git commit -m "feat(next): add LLM enricher with per-category prompts and cost tracking"
```

---

### Task 7: Enrichment orchestrator

**Depends on:** Tasks 2, 3, 4, 6

**Files:**
- Create: `internal/next/enrich/orchestrator.go`
- Test: `internal/next/enrich/orchestrator_test.go`

**Step 1: Write the failing test**

```go
package enrich

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
)

func TestOrchestrator_RunAll(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)

	mock := &MockEnricher{
		Facts: []InferredFact{
			{Category: CategoryEntrypoint, Statement: "main.go is the entrypoint"},
		},
	}

	orch := NewOrchestrator(mock, NewFactStore(), NewRunStore())

	observed := []fact.Fact{
		fact.New("f1", fact.Observed, "main.go exists", "file-tree"),
	}

	result, err := orch.Run(context.Background(), dir, observed, EnrichInput{ProjectName: "test"}, Config{
		Provider: "claude", Model: "sonnet", Reasoning: "medium", StalenessExpiryDays: 30,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.TotalFacts == 0 {
		t.Error("expected at least 1 fact")
	}
	if result.RunID == "" {
		t.Error("RunID should not be empty")
	}
}

func TestOrchestrator_PartialFailure(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)

	// Use a mock that fails for specific categories
	failingMock := &CategorySelectiveMock{
		failCategories: map[EnrichmentCategory]bool{
			CategoryOwnershipBoundary: true,
		},
		defaultFacts: []InferredFact{
			{Statement: "test fact"},
		},
	}

	orch := NewOrchestrator(failingMock, NewFactStore(), NewRunStore())
	result, err := orch.Run(context.Background(), dir, []fact.Fact{}, EnrichInput{}, DefaultConfig())
	if err != nil {
		t.Fatalf("Run should not error on partial failure: %v", err)
	}
	if len(result.FailedCategories) != 1 {
		t.Errorf("expected 1 failed category, got %d", len(result.FailedCategories))
	}
	if result.TotalFacts == 0 {
		t.Error("successful categories should still produce facts")
	}
}

func TestOrchestrator_MergesStatusesOnRerun(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inferred", "runs"), 0o755)

	factStore := NewFactStore()
	// Pre-populate with an accepted fact
	factStore.SaveFacts(dir, []InferredFact{
		{FactID: "existing-id", Status: StatusAccepted, Category: CategoryEntrypoint, Statement: "main.go"},
	})

	mock := &MockEnricher{
		Facts: []InferredFact{
			{Category: CategoryEntrypoint, Statement: "main.go"},
		},
	}

	orch := NewOrchestrator(mock, factStore, NewRunStore())
	orch.Run(context.Background(), dir, []fact.Fact{}, EnrichInput{}, DefaultConfig())

	// Verify the accepted status was preserved
	loaded, _ := factStore.LoadFacts(dir)
	for _, f := range loaded {
		if f.Statement == "main.go" && f.Status != StatusAccepted {
			t.Errorf("accepted fact should preserve status, got %v", f.Status)
		}
	}
}

func TestOrchestrator_NoObservedFacts(t *testing.T) {
	dir := t.TempDir()
	// No artifacts directory, no observed facts
	mock := &MockEnricher{
		Facts: []InferredFact{
			{Category: CategoryEntrypoint, Statement: "main.go"},
		},
	}

	orch := NewOrchestrator(mock, NewFactStore(), NewRunStore())
	_, err := orch.Run(context.Background(), dir, []fact.Fact{}, EnrichInput{}, DefaultConfig())
	if err == nil {
		t.Error("expected error when observed facts are empty and no artifacts directory exists")
	}
}

func TestOrchestrator_DryRun(t *testing.T) {
	dir := t.TempDir()

	mock := &MockEnricher{
		Facts: []InferredFact{
			{Category: CategoryEntrypoint, Statement: "main.go"},
		},
	}

	orch := NewOrchestrator(mock, NewFactStore(), NewRunStore())
	result, err := orch.DryRun(context.Background(), dir, []fact.Fact{}, EnrichInput{}, DefaultConfig())
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if result.TotalFacts == 0 {
		t.Error("DryRun should still produce facts")
	}

	// Verify nothing was written
	if _, err := os.Stat(filepath.Join(dir, "inferred", "facts.json")); err == nil {
		t.Error("DryRun should not write facts.json")
	}
}
```

**Step 2: Write implementation**

The `Orchestrator` runs all category passes (via the `CategoryEnricher`), collects results, merges with existing statuses, saves facts and run artifacts. `DryRun` does the same but skips persistence.

Key behavior:
- Returns an error when observed facts are empty and no artifacts directory exists (no-observed-facts guard)
- Runs all 8 categories (can be parallelized in future, sequential for now)
- Aggregates cost/token counts across all passes
- Calls `FactStore.MergeWithExisting` to preserve accepted statuses
- Saves run artifacts via `RunStore`
- Returns `OrchestratorResult` with total facts, failed categories, run ID, aggregate cost

**Step 3: Commit**

```bash
git add internal/next/enrich/
git commit -m "feat(next): add enrichment orchestrator with merge, dry-run, and partial failure"
```

---

## Phase 3: Guide and Context Integration

### Task 8: Guide renderer — inferred sections

**Depends on:** Task 1

**Files:**
- Modify: `internal/next/guide/guide.go`
- Test: `internal/next/guide/guide_test.go` (add tests)

**Step 1: Write the failing test**

```go
func TestMarkdownRenderer_InferredSections(t *testing.T) {
	r := NewMarkdownRenderer()
	input := RenderInput{
		ProjectName: "test-project",
		InferredFacts: []InferredObservation{
			{Category: "component_boundary", Statement: "API layer is separate from storage", Confidence: "high"},
			{Category: "risky_area", Statement: "No error handling in webhook handler", Confidence: "medium"},
			{Category: "glossary_term", Statement: "Bead: a unit of work in the pipeline", Confidence: "high"},
		},
		IncludeInferred: true,
	}

	out, err := r.Render(input)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)

	// Inferred sections should be present and labeled
	if !strings.Contains(s, "[INFERRED]") {
		t.Error("inferred content should be marked with [INFERRED]")
	}
	if !strings.Contains(s, "Inferred Component Structure") {
		t.Error("expected inferred component structure section")
	}
	if !strings.Contains(s, "Inferred Risky Areas") {
		t.Error("expected inferred risky areas section")
	}
}

func TestMarkdownRenderer_NoInferredByDefault(t *testing.T) {
	r := NewMarkdownRenderer()
	input := RenderInput{
		ProjectName: "test-project",
		InferredFacts: []InferredObservation{
			{Category: "entrypoint", Statement: "main.go", Confidence: "high"},
		},
		IncludeInferred: false,
	}

	out, _ := r.Render(input)
	if strings.Contains(string(out), "[INFERRED]") {
		t.Error("inferred content should not appear when IncludeInferred is false")
	}
}
```

**Step 2: Write implementation**

Add the `InferredObservation` type to the guide package:

```go
type InferredObservation struct {
    Category   string `json:"category"`
    Statement  string `json:"statement"`
    Confidence string `json:"confidence"`
}
```

Add `InferredFacts []InferredObservation` and `IncludeInferred bool` fields to `RenderInput`. When `IncludeInferred` is true, render additional sections grouped by category with `[INFERRED]` markers and confidence levels. Sections appear after canonical sections.

Update the `NormalizeNilFields()` method on `RenderInput` to handle the new slice field:

```go
if r.InferredFacts == nil {
    r.InferredFacts = []InferredObservation{}
}
```

**Step 3: Commit**

```bash
git add internal/next/guide/
git commit -m "feat(next): add inferred sections to guide renderer with INFERRED markers"
```

---

### Task 9: Context compiler — inferred fact inclusion

**Depends on:** Task 1

**Files:**
- Modify: `internal/next/contextpkt/context.go`
- Test: `internal/next/contextpkt/context_test.go` (add tests)

**Step 1: Write the failing test**

```go
func TestCompiler_ProjectLevelWithInferred(t *testing.T) {
	store := newMockArtifactStore()

	// Set up a cell with architecture and doctrine artifacts
	cellPath := t.TempDir()
	store.WriteArtifact(cellPath, "architecture", []byte(`{"components":["api"]}`))
	store.WriteArtifact(cellPath, "doctrine", []byte(`{"principles":["simplicity"]}`))

	// Write inferred/facts.json
	inferredDir := filepath.Join(cellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)
	factsJSON := `[{"fact_id":"f1","category":"entrypoint","statement":"main.go is the entrypoint","confidence":"high","status":"proposed"}]`
	os.WriteFile(filepath.Join(inferredDir, "facts.json"), []byte(factsJSON), 0o644)

	compiler := NewCompiler(store)
	cell := Cell{Path: cellPath}
	pkt, err := compiler.Compile(context.Background(), cell, LevelProject, CompileOpts{
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	hasInferred := false
	for _, s := range pkt.Sections {
		if strings.Contains(s.Name, "inferred") {
			hasInferred = true
		}
	}
	if !hasInferred {
		t.Error("expected inferred section in packet when IncludeInferred is true")
	}
}

func TestCompiler_ProjectLevelDefaultExcludesInferred(t *testing.T) {
	store := newMockArtifactStore()

	cellPath := t.TempDir()
	store.WriteArtifact(cellPath, "architecture", []byte(`{"components":["api"]}`))
	store.WriteArtifact(cellPath, "doctrine", []byte(`{"principles":["simplicity"]}`))

	inferredDir := filepath.Join(cellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)
	factsJSON := `[{"fact_id":"f1","category":"entrypoint","statement":"main.go","confidence":"high","status":"proposed"}]`
	os.WriteFile(filepath.Join(inferredDir, "facts.json"), []byte(factsJSON), 0o644)

	compiler := NewCompiler(store)
	cell := Cell{Path: cellPath}
	pkt, err := compiler.Compile(context.Background(), cell, LevelProject, CompileOpts{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	for _, s := range pkt.Sections {
		if strings.Contains(s.Name, "inferred") {
			t.Error("default CompileOpts should not include inferred sections")
		}
	}
}

func TestCompiler_TaskLevelInferredPresent(t *testing.T) {
	// Initial implementation includes all inferred facts regardless of scope.
	// This test verifies that inferred sections are present at task level
	// when IncludeInferred is true.
	store := newMockArtifactStore()

	cellPath := t.TempDir()
	store.WriteArtifact(cellPath, "architecture", []byte(`{"components":["api"]}`))

	inferredDir := filepath.Join(cellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)
	factsJSON := `[{"fact_id":"f1","category":"entrypoint","statement":"main.go","confidence":"high","status":"proposed"}]`
	os.WriteFile(filepath.Join(inferredDir, "facts.json"), []byte(factsJSON), 0o644)

	compiler := NewCompiler(store)
	cell := Cell{Path: cellPath}
	pkt, err := compiler.Compile(context.Background(), cell, LevelTask, CompileOpts{
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	hasInferred := false
	for _, s := range pkt.Sections {
		if strings.Contains(s.Name, "inferred") {
			hasInferred = true
		}
	}
	if !hasInferred {
		t.Error("expected inferred section in task-level packet when IncludeInferred is true")
	}
}
```

**Step 2: Write implementation**

Add `IncludeInferred bool` to `CompileOpts`. When true, load `inferred/facts.json` from the cell, filter by packet scope (project=all, spec=spec-relevant, task=task-relevant), and add an `inferred-observations` section with `[INFERRED]` content and `category: "inferred"` on all fact refs.

**Step 3: Commit**

```bash
git add internal/next/contextpkt/
git commit -m "feat(next): add opt-in inferred fact inclusion to context compiler"
```

---

## Phase 4: Staleness and Provenance

### Task 10: Staleness checking for enrichment

**Depends on:** Tasks 3, 4

**Files:**
- Create: `internal/next/enrich/staleness.go`
- Test: `internal/next/enrich/staleness_test.go`

**Step 1: Write the failing test**

```go
package enrich

import (
	"testing"
	"time"
)

func TestStaleness_FreshFact(t *testing.T) {
	f := InferredFact{CreatedAt: time.Now()}
	if IsExpired(f, 30) {
		t.Error("fact created now should not be expired with 30-day window")
	}
}

func TestStaleness_ExpiredFact(t *testing.T) {
	f := InferredFact{CreatedAt: time.Now().Add(-45 * 24 * time.Hour)}
	if !IsExpired(f, 30) {
		t.Error("fact created 45 days ago should be expired with 30-day window")
	}
}

func TestStaleness_FilterExpired(t *testing.T) {
	facts := []InferredFact{
		{FactID: "fresh", CreatedAt: time.Now()},
		{FactID: "stale", CreatedAt: time.Now().Add(-60 * 24 * time.Hour)},
	}
	filtered := FilterExpired(facts, 30)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 non-expired fact, got %d", len(filtered))
	}
	if filtered[0].FactID != "fresh" {
		t.Errorf("expected fresh fact, got %q", filtered[0].FactID)
	}
}

func TestStaleness_ObservedFactsFreshness(t *testing.T) {
	// Test that CheckObservedFreshness returns warning when SHA mismatch
	warning := CheckObservedFreshness("abc123", "def456")
	if warning == "" {
		t.Error("expected staleness warning for SHA mismatch")
	}
	warning = CheckObservedFreshness("abc123", "abc123")
	if warning != "" {
		t.Error("expected no warning for matching SHA")
	}
}
```

**Step 2: Write implementation**

Simple functions: `IsExpired(fact, days)`, `FilterExpired(facts, days)`, `CheckObservedFreshness(provenanceSHA, headSHA)`.

**Step 3: Commit**

```bash
git add internal/next/enrich/
git commit -m "feat(next): add staleness checking and expiry for inferred facts"
```

---

## Phase 5: CLI Wiring

### Task 11: `gromit-next project enrich` command

**Depends on:** Tasks 2, 7, 10

**Files:**
- Modify: `cmd/gromit-next/project.go` (or create `cmd/gromit-next/enrich.go`)

**Step 1: Write implementation**

Wire up the `enrich` subcommand:
- Load project cell
- Load enrichment config (with CLI flag overrides for `--provider`, `--model`, `--reasoning`)
- Load observed facts from artifacts
- Check provenance freshness, warn if stale
- If `--refresh`, re-run inspect first
- If `--dry-run`, use `Orchestrator.DryRun` and output to stdout
- Otherwise, run `Orchestrator.Run`
- Print summary: facts produced, cost, tokens, any failed categories

Flags: `--provider`, `--model`, `--reasoning`, `--refresh`, `--dry-run`

**Step 2: Commit**

```bash
git add cmd/gromit-next/
git commit -m "feat(next): add gromit-next project enrich CLI command"
```

---

### Task 12: `gromit-next project review-inferred` command

**Depends on:** Task 3

**Files:**
- Modify: `cmd/gromit-next/project.go` (or create `cmd/gromit-next/review.go`)

**Step 1: Write implementation**

Wire up the `review-inferred` subcommand:
- Load inferred facts from cell
- Filter expired facts (with warning)
- Print facts grouped by category, showing: fact_id, statement, confidence, status
- Tabular or list output

**Step 2: Commit**

```bash
git add cmd/gromit-next/
git commit -m "feat(next): add gromit-next project review-inferred CLI command"
```

---

### Task 13: `gromit-next project accept-inferred` and `reject-inferred` commands

**Depends on:** Task 3

**Files:**
- Modify: `cmd/gromit-next/project.go`

**Step 1: Write implementation**

Wire up accept/reject subcommands:
- Load facts, find by `--fact` ID
- Call `FactStore.UpdateStatus` with `StatusAccepted` or `StatusRejected`
- Print confirmation

**Step 2: Commit**

```bash
git add cmd/gromit-next/
git commit -m "feat(next): add accept-inferred and reject-inferred CLI commands"
```

---

### Task 14: Update `guide` and `context build` commands for `--include-inferred`

**Depends on:** Tasks 8, 9

**Files:**
- Modify: `cmd/gromit-next/project.go` (guide command)
- Modify: `cmd/gromit-next/context.go` (context build command)

**Step 1: Write implementation**

Add `--include-inferred` flag to both commands. When set:
- Load inferred facts from cell
- Filter expired facts
- Pass to guide renderer or context compiler with `IncludeInferred: true`

**Step 2: Commit**

```bash
git add cmd/gromit-next/
git commit -m "feat(next): add --include-inferred flag to guide and context build commands"
```

---

## Phase 6: Integration Testing

### Task 15: Integration test — full enrichment flow

**Depends on:** All previous tasks

**Files:**
- Create: `internal/next/enrich/integration_test.go`

**Step 1: Write the test**

End-to-end test using mock enricher:
1. Create temp workspace with fixture repo
2. Run deterministic inspect (reuse existing extractors)
3. Run enrichment with mock enricher
4. Verify `inferred/facts.json` contains expected facts
5. Verify `inferred/runs/<run-id>/` exists with inputs.json, request.json, output.json, summary.md
6. Accept one fact, reject another
7. Re-run enrichment
8. Verify accepted fact retained status, rejected fact re-proposed
9. Verify run count increased
10. Render guide with `--include-inferred` and verify `[INFERRED]` markers
11. Render guide without flag and verify no inferred content
12. Compile project-level context with inferred and verify scoped inclusion
13. Compile project-level context without flag and verify exclusion
14. Verify no writes to fixture repo

**Step 2: Commit**

```bash
git add internal/next/enrich/
git commit -m "test(next): add integration test for full enrichment flow"
```

---

### Task 16: Integration test — multi-project isolation

**Depends on:** Task 15

**Files:**
- Modify: `internal/next/enrich/integration_test.go`

**Step 1: Write the test**

1. Create two fixture repos
2. Enrich both projects
3. Verify project A's inferred facts don't appear in project B's guide
4. Verify project A's inferred facts don't appear in project B's context packets

**Step 2: Commit**

```bash
git add internal/next/enrich/
git commit -m "test(next): add multi-project isolation test for enrichment"
```

---

### Task 17: Integration test — staleness and expiry

**Depends on:** Task 15

**Files:**
- Modify: `internal/next/enrich/integration_test.go`

**Step 1: Write the test**

1. Enrich a project
2. Manually backdate fact timestamps to 45 days ago
3. Render guide with `--include-inferred` — verify no inferred sections
4. Verify staleness warning is produced
5. Re-enrich — verify fresh facts appear

**Step 2: Commit**

```bash
git add internal/next/enrich/
git commit -m "test(next): add staleness expiry integration test for enrichment"
```

---

## Dependency Graph

```
Task 1 (fact type)
├── Task 2 (config) [independent]
├── Task 3 (fact store) [depends on 1]
├── Task 4 (run store) [depends on 1]
├── Task 5 (enricher interface) [depends on 1]
│   └── Task 6 (LLM enricher) [depends on 5]
├── Task 8 (guide inferred) [depends on 1]
└── Task 9 (context inferred) [depends on 1]

Task 7 (orchestrator) [depends on 2, 3, 4, 6]

Task 10 (staleness) [depends on 3, 4]

Task 11 (enrich CLI) [depends on 2, 7, 10]
Task 12 (review CLI) [depends on 3]
Task 13 (accept/reject CLI) [depends on 3]
Task 14 (guide/context CLI flags) [depends on 8, 9]

Task 15 (integration test) [depends on all]
Task 16 (isolation test) [depends on 15]
Task 17 (staleness test) [depends on 15]
```

## Parallelization Opportunities

- **Batch 1:** Tasks 1 and 2 are fully independent — run in parallel
- **Batch 2:** After Task 1: Tasks 3, 4, 5, 8, 9 can all run in parallel
- **Batch 3:** After Task 5: Task 6
- **Batch 4:** After Tasks 3, 4, 6: Task 7; After Tasks 3, 4: Task 10
- **Batch 5:** Tasks 11, 12, 13, 14 can partially parallelize (12, 13 need only Task 3 and can run in parallel with each other; 14 needs 8, 9)
- **Batch 6:** Tasks 15, 16, 17 are sequential
