package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/artifact"
	"github.com/danabrams/gromit/internal/next/contextpkt"
	"github.com/danabrams/gromit/internal/next/extract"
	"github.com/danabrams/gromit/internal/next/fact"
	"github.com/danabrams/gromit/internal/next/guide"
	"github.com/danabrams/gromit/internal/next/infer"
	"github.com/danabrams/gromit/internal/next/inspect"
	"github.com/danabrams/gromit/internal/next/provenance"
	"github.com/danabrams/gromit/internal/next/sourcemap"
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
	if result1.NewFactCount < 2 {
		t.Errorf("Run 1: expected at least 2 facts, got %d", result1.NewFactCount)
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

	// Verify all initial facts have StatusProposed.
	for _, f := range loadedFacts {
		if f.Status != StatusProposed {
			t.Errorf("expected initial fact %q status to be proposed, got %s", f.FactID, f.Status)
		}
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
		// Rejected facts that reappear in incoming are re-proposed per design doc:
		// only accepted is sticky, rejected facts get a second chance.
		if f.FactID == riskyAreaFactID && f.Status != StatusProposed {
			t.Errorf("rejected fact re-appearing should be proposed, got %v", f.Status)
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

func TestIntegration_MultiProjectIsolation(t *testing.T) {
	// --- Setup: two independent projects with separate cell paths ---
	cellPathA := t.TempDir()
	cellPathB := t.TempDir()

	os.MkdirAll(filepath.Join(cellPathA, "inferred", "runs"), 0o755)
	os.MkdirAll(filepath.Join(cellPathB, "inferred", "runs"), 0o755)

	factStore := NewFactStore()
	runStore := NewRunStore()

	// Project A: MVC architecture
	factA := InferredFact{
		Category:   CategoryEntrypoint,
		Statement:  "project-a uses MVC architecture",
		Rationale:  "observed controller/model/view directories",
		Confidence: "high",
		Scope:      "project",
	}
	mockA := &categoryMockEnricher{
		factsByCategory: map[EnrichmentCategory][]InferredFact{
			CategoryEntrypoint: {factA},
		},
	}

	// Project B: event sourcing
	factB := InferredFact{
		Category:   CategoryEntrypoint,
		Statement:  "project-b uses event sourcing",
		Rationale:  "observed event store and projections",
		Confidence: "high",
		Scope:      "project",
	}
	mockB := &categoryMockEnricher{
		factsByCategory: map[EnrichmentCategory][]InferredFact{
			CategoryEntrypoint: {factB},
		},
	}

	cfg := DefaultConfig()
	observedA := []fact.Fact{fact.New("obs-a", fact.Observed, "controllers/ exists", "file-tree")}
	observedB := []fact.Fact{fact.New("obs-b", fact.Observed, "events/ exists", "file-tree")}

	inputA := EnrichInput{ProjectName: "project-a", FileTree: []string{"controllers/", "models/", "views/"}}
	inputB := EnrichInput{ProjectName: "project-b", FileTree: []string{"events/", "projections/", "aggregates/"}}

	// --- Run enrichment for project A ---
	orchA := NewOrchestrator(mockA, factStore, runStore)
	resultA, err := orchA.Run(context.Background(), cellPathA, observedA, inputA, cfg)
	if err != nil {
		t.Fatalf("Project A enrichment failed: %v", err)
	}
	if resultA.NewFactCount < 1 {
		t.Errorf("Project A: expected at least 1 fact, got %d", resultA.NewFactCount)
	}

	// --- Run enrichment for project B ---
	orchB := NewOrchestrator(mockB, factStore, runStore)
	resultB, err := orchB.Run(context.Background(), cellPathB, observedB, inputB, cfg)
	if err != nil {
		t.Fatalf("Project B enrichment failed: %v", err)
	}
	if resultB.NewFactCount < 1 {
		t.Errorf("Project B: expected at least 1 fact, got %d", resultB.NewFactCount)
	}

	// --- Verify fact isolation: A's facts don't contain B's content ---
	factsA, err := factStore.LoadFacts(cellPathA)
	if err != nil {
		t.Fatalf("LoadFacts for project A: %v", err)
	}
	factsB, err := factStore.LoadFacts(cellPathB)
	if err != nil {
		t.Fatalf("LoadFacts for project B: %v", err)
	}

	for _, f := range factsA {
		if strings.Contains(f.Statement, "project-b") {
			t.Errorf("Project A facts should not contain project-b content, found: %s", f.Statement)
		}
	}
	for _, f := range factsB {
		if strings.Contains(f.Statement, "project-a") {
			t.Errorf("Project B facts should not contain project-a content, found: %s", f.Statement)
		}
	}

	// Verify expected content IS present in each project.
	foundAFact := false
	for _, f := range factsA {
		if f.Statement == "project-a uses MVC architecture" {
			foundAFact = true
		}
	}
	if !foundAFact {
		t.Error("Project A should contain its MVC architecture fact")
	}

	foundBFact := false
	for _, f := range factsB {
		if f.Statement == "project-b uses event sourcing" {
			foundBFact = true
		}
	}
	if !foundBFact {
		t.Error("Project B should contain its event sourcing fact")
	}

	// --- Guide rendering isolation ---
	renderer := guide.NewMarkdownRenderer()

	var inferredObsA []guide.InferredObservation
	for _, f := range factsA {
		inferredObsA = append(inferredObsA, guide.InferredObservation{
			Category:   string(f.Category),
			Statement:  f.Statement,
			Confidence: f.Confidence,
		})
	}
	var inferredObsB []guide.InferredObservation
	for _, f := range factsB {
		inferredObsB = append(inferredObsB, guide.InferredObservation{
			Category:   string(f.Category),
			Statement:  f.Statement,
			Confidence: f.Confidence,
		})
	}

	guideA, err := renderer.Render(guide.RenderInput{
		ProjectName:     "project-a",
		InferredFacts:   inferredObsA,
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Render guide A: %v", err)
	}
	guideB, err := renderer.Render(guide.RenderInput{
		ProjectName:     "project-b",
		InferredFacts:   inferredObsB,
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Render guide B: %v", err)
	}

	if strings.Contains(string(guideA), "event sourcing") {
		t.Error("Project A guide should not contain project B's event sourcing content")
	}
	if strings.Contains(string(guideB), "MVC architecture") {
		t.Error("Project B guide should not contain project A's MVC architecture content")
	}

	// --- Context compiler isolation ---
	artStoreA := newTestArtifactStore()
	artStoreA.set("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "controllers", "description": "MVC controllers", "language": "go"},
		},
	})

	artStoreB := newTestArtifactStore()
	artStoreB.set("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "events", "description": "Event store", "language": "go"},
		},
	})

	compilerA := contextpkt.NewCompiler(artStoreA)
	compilerB := contextpkt.NewCompiler(artStoreB)

	cellA := contextpkt.Cell{Name: "project-a", CellPath: cellPathA}
	cellB := contextpkt.Cell{Name: "project-b", CellPath: cellPathB}

	pktA, err := compilerA.Compile(context.Background(), cellA, contextpkt.LevelProject, contextpkt.CompileOpts{
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Compile project A context: %v", err)
	}
	pktB, err := compilerB.Compile(context.Background(), cellB, contextpkt.LevelProject, contextpkt.CompileOpts{
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Compile project B context: %v", err)
	}

	// Verify each packet's inferred section contains only its own facts.
	pktAStr := fmt.Sprintf("%v", pktA)
	pktBStr := fmt.Sprintf("%v", pktB)

	if strings.Contains(pktAStr, "event sourcing") {
		t.Error("Project A context packet should not contain project B's event sourcing content")
	}
	if strings.Contains(pktBStr, "MVC architecture") {
		t.Error("Project B context packet should not contain project A's MVC architecture content")
	}
}

func TestIntegration_StalenessExpiry(t *testing.T) {
	// --- Setup ---
	cellPath := t.TempDir()
	os.MkdirAll(filepath.Join(cellPath, "inferred", "runs"), 0o755)

	factStore := NewFactStore()
	runStore := NewRunStore()

	// Mock enricher returning two facts.
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

	// --- Run 1: initial enrichment to produce facts ---
	result1, err := orch.Run(context.Background(), cellPath, observed, input, cfg)
	if err != nil {
		t.Fatalf("Run 1 failed: %v", err)
	}
	if result1.NewFactCount < 2 {
		t.Fatalf("Run 1: expected at least 2 facts, got %d", result1.NewFactCount)
	}

	// --- Backdate all fact timestamps to 45 days ago ---
	loadedFacts, err := factStore.LoadFacts(cellPath)
	if err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	if len(loadedFacts) < 2 {
		t.Fatalf("expected at least 2 facts, got %d", len(loadedFacts))
	}

	fortyFiveDaysAgo := time.Now().Add(-45 * 24 * time.Hour)
	for i := range loadedFacts {
		loadedFacts[i].CreatedAt = fortyFiveDaysAgo
	}
	if err := factStore.SaveFacts(cellPath, loadedFacts); err != nil {
		t.Fatalf("SaveFacts (backdate): %v", err)
	}

	// --- Verify all facts are expired with a 30-day window ---
	backdatedFacts, err := factStore.LoadFacts(cellPath)
	if err != nil {
		t.Fatalf("LoadFacts (backdated): %v", err)
	}
	fresh := FilterExpired(backdatedFacts, 30)
	if len(fresh) != 0 {
		t.Errorf("FilterExpired(30 days) should return empty for 45-day-old facts, got %d", len(fresh))
	}

	// --- Guide rendering: caller filters expired facts before rendering ---
	// Since all facts are expired, the caller passes no inferred observations.
	renderer := guide.NewMarkdownRenderer()
	var inferredObs []guide.InferredObservation
	for _, f := range fresh {
		// fresh is empty, so this loop doesn't execute.
		inferredObs = append(inferredObs, guide.InferredObservation{
			Category:   string(f.Category),
			Statement:  f.Statement,
			Confidence: f.Confidence,
		})
	}

	guideOutput, err := renderer.Render(guide.RenderInput{
		ProjectName:     "test-project",
		InferredFacts:   inferredObs,
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Render with expired facts filtered: %v", err)
	}
	if strings.Contains(string(guideOutput), "[INFERRED]") {
		t.Error("guide should not contain [INFERRED] markers when all facts are expired and filtered out")
	}

	// --- Run 2: re-run enrichment to get fresh facts ---
	result2, err := orch.Run(context.Background(), cellPath, observed, input, cfg)
	if err != nil {
		t.Fatalf("Run 2 failed: %v", err)
	}
	if result2.NewFactCount < 2 {
		t.Errorf("Run 2: expected at least 2 facts, got %d", result2.NewFactCount)
	}

	// --- Verify new facts are fresh and not expired ---
	newFacts, err := factStore.LoadFacts(cellPath)
	if err != nil {
		t.Fatalf("LoadFacts (after re-run): %v", err)
	}

	freshAfterRerun := FilterExpired(newFacts, 30)
	if len(freshAfterRerun) == 0 {
		t.Error("FilterExpired(30 days) should return all facts after fresh enrichment run")
	}
	if len(freshAfterRerun) < 2 {
		t.Errorf("expected at least 2 fresh facts after re-run, got %d", len(freshAfterRerun))
	}
}

func TestIntegration_AcceptedFactSuperseded(t *testing.T) {
	// --- Setup ---
	cellPath := t.TempDir()
	os.MkdirAll(filepath.Join(cellPath, "inferred", "runs"), 0o755)

	factStore := NewFactStore()
	runStore := NewRunStore()

	// Mock 1: produces fact X ("payments-api uses hexagonal architecture").
	factX := InferredFact{
		Category:   CategoryEntrypoint,
		Statement:  "payments-api uses hexagonal architecture",
		Rationale:  "observed ports and adapters directories",
		Confidence: "high",
		Scope:      "project",
	}
	mock1 := &categoryMockEnricher{
		factsByCategory: map[EnrichmentCategory][]InferredFact{
			CategoryEntrypoint: {factX},
		},
	}

	observed := []fact.Fact{
		fact.New("obs-1", fact.Observed, "ports/ and adapters/ exist", "file-tree"),
	}
	input := EnrichInput{
		ProjectName: "payments-api",
		FileTree:    []string{"ports/", "adapters/", "domain/"},
	}
	cfg := DefaultConfig()

	// --- Run 1: initial enrichment producing fact X ---
	orch1 := NewOrchestrator(mock1, factStore, runStore)
	result1, err := orch1.Run(context.Background(), cellPath, observed, input, cfg)
	if err != nil {
		t.Fatalf("Run 1 failed: %v", err)
	}
	if result1.NewFactCount < 1 {
		t.Fatalf("Run 1: expected at least 1 fact, got %d", result1.NewFactCount)
	}

	// Find fact X's ID and accept it.
	loadedFacts, err := factStore.LoadFacts(cellPath)
	if err != nil {
		t.Fatalf("LoadFacts after Run 1: %v", err)
	}
	var factXID string
	for _, f := range loadedFacts {
		if f.Statement == "payments-api uses hexagonal architecture" {
			factXID = f.FactID
			break
		}
	}
	if factXID == "" {
		t.Fatalf("could not find fact X in loaded facts; got %d facts", len(loadedFacts))
	}

	// Accept fact X.
	if err := factStore.UpdateStatus(cellPath, factXID, StatusAccepted); err != nil {
		t.Fatalf("UpdateStatus(accepted): %v", err)
	}

	// --- Run 2: different mock that does NOT produce fact X ---
	factY := InferredFact{
		Category:   CategoryEntrypoint,
		Statement:  "payments-api uses layered architecture",
		Rationale:  "observed service/repository layers",
		Confidence: "medium",
		Scope:      "project",
	}
	mock2 := &categoryMockEnricher{
		factsByCategory: map[EnrichmentCategory][]InferredFact{
			CategoryEntrypoint: {factY},
		},
	}

	orch2 := NewOrchestrator(mock2, factStore, runStore)
	_, err = orch2.Run(context.Background(), cellPath, observed, input, cfg)
	if err != nil {
		t.Fatalf("Run 2 failed: %v", err)
	}

	// --- Verify: fact X should be superseded, new facts should be proposed ---
	finalFacts, err := factStore.LoadFacts(cellPath)
	if err != nil {
		t.Fatalf("LoadFacts after Run 2: %v", err)
	}

	var foundFactX, foundFactY bool
	for _, f := range finalFacts {
		if f.FactID == factXID {
			foundFactX = true
			if f.Status != StatusSuperseded {
				t.Errorf("accepted fact X should be superseded after re-run without it, got %v", f.Status)
			}
		}
		if f.Statement == "payments-api uses layered architecture" {
			foundFactY = true
			if f.Status != StatusProposed {
				t.Errorf("new fact Y should be proposed, got %v", f.Status)
			}
		}
	}
	if !foundFactX {
		t.Error("fact X should still be present (as superseded) in the final facts")
	}
	if !foundFactY {
		t.Error("fact Y should be present in the final facts")
	}
}

// TestIntegration_RefreshBeforeEnrich exercises the same logical flow as the
// --refresh flag on the enrich command: run the inspect pipeline to produce
// artifacts from a real git repo, then run enrichment on those artifacts.
func TestIntegration_RefreshBeforeEnrich(t *testing.T) {
	// --- Setup: create a real git repo with Go files ---
	repoPath := t.TempDir()

	// Write a Go source file into the repo.
	if err := os.WriteFile(filepath.Join(repoPath, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "util.go"), []byte("package main\n\nfunc helper() string { return \"ok\" }\n"), 0o644); err != nil {
		t.Fatalf("write util.go: %v", err)
	}

	// Initialize a git repo and commit the files so extractors work against
	// a real repository (some extractors may depend on git metadata).
	gitInit := exec.Command("git", "init")
	gitInit.Dir = repoPath
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = repoPath
	if out, err := gitAdd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	gitCommit := exec.Command("git", "-c", "user.email=test@test.com", "-c", "user.name=Test", "commit", "-m", "initial")
	gitCommit.Dir = repoPath
	if out, err := gitCommit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	// Create a cell directory with an EMPTY artifacts dir (no sourcemap yet).
	cellPath := t.TempDir()
	artifactsDir := filepath.Join(cellPath, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatalf("mkdir artifacts: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cellPath, "inferred", "runs"), 0o755); err != nil {
		t.Fatalf("mkdir inferred/runs: %v", err)
	}

	// Verify sourcemap does NOT exist yet.
	artStore := artifact.NewJSONStore()
	var smBefore sourcemap.SourceMap
	if err := artStore.Read(artifactsDir, "sourcemap", &smBefore); err == nil {
		t.Fatal("sourcemap should not exist before refresh")
	}

	// --- Phase 1: Run the inspect pipeline (same logic as runInspect) ---
	extractors := []inspect.Extractor{
		extract.NewFileTreeExtractor(),
		extract.NewGoModExtractor(),
		extract.NewValidationCommandsExtractor(),
	}
	inferrer := infer.NewStubInferrer()
	inspector := inspect.NewInspector(extractors, inferrer)

	inspCell := inspect.Cell{Name: "refresh-test", RepoPath: repoPath, CellPath: cellPath}
	result, err := inspector.Inspect(context.Background(), inspCell)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if len(result.Observed) == 0 {
		t.Fatal("inspect should produce at least one observed fact")
	}

	// Build and write the sourcemap artifact.
	sm := sourcemap.BuildFromFacts(result.Observed)
	if err := artStore.Write(artifactsDir, "sourcemap", sm); err != nil {
		t.Fatalf("write sourcemap: %v", err)
	}

	// Record provenance (mirrors runInspect behavior).
	tracker := provenance.NewFSTracker(filepath.Join(cellPath, "provenance", "provenance.json"))
	if err := tracker.Record(provenance.Record{
		Artifact: "sourcemap",
		GitSHA:   "abc123", // placeholder; real SHA not needed for this test
	}); err != nil {
		t.Fatalf("record provenance: %v", err)
	}

	// Verify sourcemap artifact was created with entries.
	var smAfter sourcemap.SourceMap
	if err := artStore.Read(artifactsDir, "sourcemap", &smAfter); err != nil {
		t.Fatalf("read sourcemap after refresh: %v", err)
	}
	if len(smAfter.Entries) == 0 {
		t.Fatal("sourcemap should have entries after inspect pipeline ran")
	}

	// Verify our Go files appear in the sourcemap.
	fileNames := map[string]bool{}
	for _, e := range smAfter.Entries {
		fileNames[e.Path] = true
	}
	if !fileNames["main.go"] {
		t.Error("sourcemap should contain main.go")
	}
	if !fileNames["util.go"] {
		t.Error("sourcemap should contain util.go")
	}

	// --- Phase 2: Run enrichment on the refreshed artifacts ---
	// Convert sourcemap entries to observed facts (same as sourcemapToFacts in enrich.go).
	observed := make([]fact.Fact, 0, len(smAfter.Entries))
	for _, e := range smAfter.Entries {
		data, err := json.Marshal(e)
		if err != nil {
			continue
		}
		observed = append(observed, fact.Fact{
			Category: fact.Observed,
			Content:  string(data),
			Source:   "file-tree",
		})
	}

	// Build EnrichInput from the refreshed artifacts.
	input := EnrichInput{
		ProjectName: "refresh-test",
	}
	for _, e := range smAfter.Entries {
		input.FileTree = append(input.FileTree, e.Path)
	}
	if input.FileTree == nil {
		input.FileTree = []string{}
	}

	// Read sourcemap JSON for the input field.
	if data, err := readArtifactJSONForTest(artStore, artifactsDir, "sourcemap"); err == nil {
		input.SourceMap = string(data)
	}

	// Mock enricher that produces facts for the refreshed data.
	entrypointFact := InferredFact{
		Category:   CategoryEntrypoint,
		Statement:  "main.go is the primary entrypoint for refresh-test",
		Rationale:  "observed main package in file tree",
		Confidence: "high",
		Scope:      "project",
	}
	mock := &categoryMockEnricher{
		factsByCategory: map[EnrichmentCategory][]InferredFact{
			CategoryEntrypoint: {entrypointFact},
		},
	}

	factStore := NewFactStore()
	runStore := NewRunStore()
	orch := NewOrchestrator(mock, factStore, runStore)
	cfg := DefaultConfig()

	enrichResult, err := orch.Run(context.Background(), cellPath, observed, input, cfg)
	if err != nil {
		t.Fatalf("enrichment after refresh failed: %v", err)
	}
	if enrichResult.NewFactCount == 0 {
		t.Error("enrichment should produce at least one fact from refreshed artifacts")
	}

	// Verify the enrichment facts were persisted.
	loadedFacts, err := factStore.LoadFacts(cellPath)
	if err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	foundRefreshFact := false
	for _, f := range loadedFacts {
		if f.Statement == "main.go is the primary entrypoint for refresh-test" {
			foundRefreshFact = true
			break
		}
	}
	if !foundRefreshFact {
		t.Error("enrichment should contain the entrypoint fact derived from refreshed artifacts")
	}

	// Verify run artifacts were created.
	runs, err := runStore.ListRuns(cellPath)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1 run, got %d", len(runs))
	}
}

// readArtifactJSONForTest reads an artifact file and returns its raw JSON bytes.
// This mirrors the readArtifactJSON helper in cmd/gromit-next/enrich.go.
func readArtifactJSONForTest(store *artifact.JSONStore, artifactsDir, name string) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := store.Read(artifactsDir, name, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
