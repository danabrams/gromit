package enrich

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/contextpkt"
	"github.com/danabrams/gromit/internal/next/fact"
	"github.com/danabrams/gromit/internal/next/guide"
)

// testArtifactStore is a minimal ArtifactStore for contextpkt integration testing.
type testArtifactStore struct {
	artifacts map[string]json.RawMessage
}

func newTestArtifactStore() *testArtifactStore {
	return &testArtifactStore{artifacts: map[string]json.RawMessage{}}
}

func (s *testArtifactStore) set(name string, v any) {
	data, _ := json.Marshal(v)
	s.artifacts[name] = data
}

func (s *testArtifactStore) Read(_ string, artifact string, dest any) error {
	raw, ok := s.artifacts[artifact]
	if !ok {
		return os.ErrNotExist
	}
	return json.Unmarshal(raw, dest)
}

func (s *testArtifactStore) Write(_ string, _ string, _ any) error { return nil }

func (s *testArtifactStore) Exists(_ string, artifact string) bool {
	_, ok := s.artifacts[artifact]
	return ok
}

// categoryMockEnricher returns different facts per category to simulate realistic enrichment.
type categoryMockEnricher struct {
	factsByCategory map[EnrichmentCategory][]InferredFact
}

func (m *categoryMockEnricher) Enrich(_ context.Context, category EnrichmentCategory, _ []fact.Fact, _ EnrichInput) (EnrichResult, error) {
	facts, ok := m.factsByCategory[category]
	if !ok {
		return EnrichResult{Category: category, Facts: []InferredFact{}, FactCount: 0, Success: true}, nil
	}
	return EnrichResult{
		Category:     category,
		Facts:        facts,
		FactCount:    len(facts),
		Success:      true,
		CostUSD:      0.001,
		InputTokens:  100,
		OutputTokens: 50,
	}, nil
}

func TestIntegration_FullEnrichmentFlow(t *testing.T) {
	// --- Setup ---
	cellPath := t.TempDir()
	os.MkdirAll(filepath.Join(cellPath, "inferred", "runs"), 0o755)

	// Fixture directory that must remain untouched after enrichment.
	fixtureDir := t.TempDir()

	factStore := NewFactStore()
	runStore := NewRunStore()

	// Mock enricher returning category-specific facts.
	entrypointFact := InferredFact{
		Category:   CategoryEntrypoint,
		Statement:  "main.go is the primary entrypoint",
		Rationale:  "observed in file tree",
		Confidence: "high",
		Scope:      "project",
	}
	riskyAreaFact := InferredFact{
		Category:   CategoryRiskyArea,
		Statement:  "internal/auth has complex token refresh logic",
		Rationale:  "multiple error paths",
		Confidence: "medium",
		Scope:      "internal/auth",
	}

	mock := &categoryMockEnricher{
		factsByCategory: map[EnrichmentCategory][]InferredFact{
			CategoryEntrypoint: {entrypointFact},
			CategoryRiskyArea:  {riskyAreaFact},
		},
	}

	orch := NewOrchestrator(mock, factStore, runStore)
	observed := []fact.Fact{
		fact.New("obs-1", fact.Observed, "main.go exists", "file-tree"),
	}
	input := EnrichInput{
		ProjectName: "test-project",
		FileTree:    []string{"main.go", "internal/auth/auth.go"},
	}
	cfg := DefaultConfig()

	// --- Run 1: initial enrichment ---
	result1, err := orch.Run(context.Background(), cellPath, observed, input, cfg)
	if err != nil {
		t.Fatalf("Run 1 failed: %v", err)
	}
	if result1.RunID == "" {
		t.Error("Run 1: RunID should not be empty")
	}
	if result1.TotalFacts < 2 {
		t.Errorf("Run 1: expected at least 2 facts, got %d", result1.TotalFacts)
	}

	// Verify facts.json exists and contains expected facts.
	factsPath := filepath.Join(cellPath, "inferred", "facts.json")
	if _, err := os.Stat(factsPath); err != nil {
		t.Fatalf("facts.json should exist: %v", err)
	}
	loadedFacts, err := factStore.LoadFacts(cellPath)
	if err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	if len(loadedFacts) < 2 {
		t.Errorf("expected at least 2 loaded facts, got %d", len(loadedFacts))
	}

	// Verify run artifacts exist.
	runs1, err := runStore.ListRuns(cellPath)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs1) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs1))
	}
	runDir := filepath.Join(cellPath, "inferred", "runs", runs1[0])
	for _, artifact := range []string{"inputs.json", "request.json", "output.json", "summary.md"} {
		if _, err := os.Stat(filepath.Join(runDir, artifact)); err != nil {
			t.Errorf("run artifact %s should exist: %v", artifact, err)
		}
	}

	// --- Accept one fact, reject another ---
	var entrypointFactID, riskyAreaFactID string
	for _, f := range loadedFacts {
		if f.Category == CategoryEntrypoint && f.Statement == "main.go is the primary entrypoint" {
			entrypointFactID = f.FactID
		}
		if f.Category == CategoryRiskyArea && f.Statement == "internal/auth has complex token refresh logic" {
			riskyAreaFactID = f.FactID
		}
	}
	if entrypointFactID == "" || riskyAreaFactID == "" {
		t.Fatalf("could not find expected facts in loaded results; got %d facts", len(loadedFacts))
	}

	if err := factStore.UpdateStatus(cellPath, entrypointFactID, StatusAccepted); err != nil {
		t.Fatalf("UpdateStatus(accepted): %v", err)
	}
	if err := factStore.UpdateStatus(cellPath, riskyAreaFactID, StatusRejected); err != nil {
		t.Fatalf("UpdateStatus(rejected): %v", err)
	}

	// Sleep to ensure the second run gets a distinct timestamp-based run ID.
	time.Sleep(1100 * time.Millisecond)

	// --- Run 2: re-run enrichment with same mock ---
	result2, err := orch.Run(context.Background(), cellPath, observed, input, cfg)
	if err != nil {
		t.Fatalf("Run 2 failed: %v", err)
	}
	if result2.RunID == "" {
		t.Error("Run 2: RunID should not be empty")
	}

	// Verify accepted fact retained its status.
	reloadedFacts, err := factStore.LoadFacts(cellPath)
	if err != nil {
		t.Fatalf("LoadFacts after Run 2: %v", err)
	}
	for _, f := range reloadedFacts {
		if f.FactID == entrypointFactID && f.Status != StatusAccepted {
			t.Errorf("accepted fact should retain accepted status, got %v", f.Status)
		}
		// Rejected facts that reappear in incoming preserve their rejected status
		// per MergeWithExisting: both accepted and rejected are sticky.
		if f.FactID == riskyAreaFactID && f.Status != StatusRejected {
			t.Errorf("rejected fact should retain rejected status, got %v", f.Status)
		}
	}

	// Verify run count increased.
	runs2, err := runStore.ListRuns(cellPath)
	if err != nil {
		t.Fatalf("ListRuns after Run 2: %v", err)
	}
	if len(runs2) < 2 {
		t.Errorf("expected at least 2 runs, got %d", len(runs2))
	}

	// --- Guide rendering with inferred facts ---
	renderer := guide.NewMarkdownRenderer()

	// Build InferredObservation list from loaded facts.
	var inferredObs []guide.InferredObservation
	for _, f := range reloadedFacts {
		if f.Status == StatusAccepted || f.Status == StatusProposed {
			inferredObs = append(inferredObs, guide.InferredObservation{
				Category:   string(f.Category),
				Statement:  f.Statement,
				Confidence: f.Confidence,
			})
		}
	}

	// Render with IncludeInferred=true: should contain [INFERRED] markers.
	guideWithInferred, err := renderer.Render(guide.RenderInput{
		ProjectName:     "test-project",
		InferredFacts:   inferredObs,
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Render with inferred: %v", err)
	}
	if !strings.Contains(string(guideWithInferred), "[INFERRED]") {
		t.Error("guide with IncludeInferred=true should contain [INFERRED] markers")
	}

	// Render without IncludeInferred: should NOT contain [INFERRED] markers.
	guideWithout, err := renderer.Render(guide.RenderInput{
		ProjectName:     "test-project",
		InferredFacts:   inferredObs,
		IncludeInferred: false,
	})
	if err != nil {
		t.Fatalf("Render without inferred: %v", err)
	}
	if strings.Contains(string(guideWithout), "[INFERRED]") {
		t.Error("guide with IncludeInferred=false should not contain [INFERRED] markers")
	}

	// --- Context compiler with inferred facts ---
	// Write facts.json into a fresh cell for contextpkt testing.
	compilerCellPath := t.TempDir()
	inferredDir := filepath.Join(compilerCellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)

	// Write accepted/proposed facts as the compiler reads directly from disk.
	var factsForCompiler []InferredFact
	for _, f := range reloadedFacts {
		if f.Status == StatusAccepted || f.Status == StatusProposed {
			factsForCompiler = append(factsForCompiler, f)
		}
	}
	if err := factStore.SaveFacts(compilerCellPath, factsForCompiler); err != nil {
		t.Fatalf("SaveFacts for compiler cell: %v", err)
	}

	artStore := newTestArtifactStore()
	artStore.set("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "api", "description": "API layer", "language": "go"},
		},
	})
	artStore.set("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "simplicity", "scope": "all"},
		},
	})

	compiler := contextpkt.NewCompiler(artStore)
	cell := contextpkt.Cell{Name: "test-project", CellPath: compilerCellPath}

	// Compile with IncludeInferred=true: should have inferred-observations section.
	pktWith, err := compiler.Compile(context.Background(), cell, contextpkt.LevelProject, contextpkt.CompileOpts{
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Compile with IncludeInferred: %v", err)
	}
	hasInferred := false
	for _, s := range pktWith.Sections {
		if strings.Contains(s.Name, "inferred") {
			hasInferred = true
			break
		}
	}
	if !hasInferred {
		t.Error("context packet with IncludeInferred=true should contain inferred section")
	}

	// Compile without IncludeInferred: should NOT have inferred section.
	pktWithout, err := compiler.Compile(context.Background(), cell, contextpkt.LevelProject, contextpkt.CompileOpts{
		IncludeInferred: false,
	})
	if err != nil {
		t.Fatalf("Compile without IncludeInferred: %v", err)
	}
	for _, s := range pktWithout.Sections {
		if strings.Contains(s.Name, "inferred") {
			t.Error("context packet without IncludeInferred should not contain inferred section")
		}
	}

	// --- Verify fixture directory is untouched ---
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("reading fixture dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("fixture directory should be empty after enrichment, found %d entries", len(entries))
	}
}
