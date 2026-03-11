package contextpkt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mockArtifactStore struct {
	artifacts map[string]json.RawMessage
}

func newMockArtifactStore() *mockArtifactStore {
	return &mockArtifactStore{artifacts: map[string]json.RawMessage{}}
}

func (m *mockArtifactStore) setArtifact(name string, v any) {
	data, _ := json.Marshal(v)
	m.artifacts[name] = data
}

func (m *mockArtifactStore) Read(cellPath string, artifact string, dest any) error {
	raw, ok := m.artifacts[artifact]
	if !ok {
		return fmt.Errorf("mock: artifact %q not found", artifact)
	}
	return json.Unmarshal(raw, dest)
}
func (m *mockArtifactStore) Write(cellPath string, artifact string, src any) error { return nil }
func (m *mockArtifactStore) Exists(cellPath string, artifact string) bool {
	_, ok := m.artifacts[artifact]
	return ok
}

func TestCompiler_ProjectLevel(t *testing.T) {
	store := newMockArtifactStore()
	store.setArtifact("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "internal/auth", "description": "Auth module", "language": "go"},
		},
	})
	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "arch-001", "summary": "Use hexagonal architecture", "scope": "architecture"},
		},
	})
	store.setArtifact("glossary", map[string]any{
		"terms": []map[string]any{
			{"term": "module", "definition": "A logical boundary"},
		},
	})
	store.setArtifact("validation", map[string]any{
		"rules": []map[string]any{
			{"id": "v1", "check": "lint passes"},
		},
	})

	cell := Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	packet, err := compiler.Compile(context.Background(), cell, LevelProject, CompileOpts{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(packet.Sections) == 0 {
		t.Fatal("expected at least one section in the packet")
	}

	sectionNames := make(map[string]bool)
	for _, s := range packet.Sections {
		sectionNames[s.Name] = true
	}
	for _, want := range []string{"architecture", "doctrine", "glossary", "validation"} {
		if !sectionNames[want] {
			t.Errorf("missing section %q in packet", want)
		}
	}
	if sectionNames["spec-text"] {
		t.Error("project level should NOT include spec-text section")
	}
	if packet.TokenCount == 0 {
		t.Error("token count should be populated")
	}
}

func TestCompiler_TokenBudget(t *testing.T) {
	store := newMockArtifactStore()
	store.setArtifact("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "internal/auth", "description": "Auth module with a long description that takes up tokens", "language": "go"},
			{"name": "internal/billing", "description": "Billing module with another long description", "language": "go"},
		},
	})
	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "Rule one", "scope": "all"},
			{"id": "r2", "summary": "Rule two", "scope": "all"},
		},
	})

	cell := Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	packet, err := compiler.Compile(context.Background(), cell, LevelProject, CompileOpts{TokenBudget: 50})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if packet.TokenCount > 50 {
		t.Errorf("packet token count %d exceeds budget 50", packet.TokenCount)
	}
}

func TestCompiler_SpecLevel(t *testing.T) {
	store := newMockArtifactStore()
	store.setArtifact("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "internal/auth", "description": "Auth module", "language": "go"},
		},
	})
	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "Use hexagonal architecture", "scope": "architecture"},
		},
	})

	cell := Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	packet, err := compiler.Compile(context.Background(), cell, LevelSpec, CompileOpts{
		SpecPath: "specs/001-auth-redesign.md",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(packet.Sections) == 0 {
		t.Fatal("expected at least one section")
	}

	sectionNames := make(map[string]bool)
	for _, s := range packet.Sections {
		sectionNames[s.Name] = true
	}
	if !sectionNames["architecture"] {
		t.Error("spec level should include architecture section")
	}
	if !sectionNames["doctrine"] {
		t.Error("spec level should include doctrine section")
	}
	if !sectionNames["spec-text"] {
		t.Error("spec level should include spec-text section")
	}
	if sectionNames["glossary"] {
		t.Error("spec level should NOT include glossary section")
	}
	if sectionNames["validation"] {
		t.Error("spec level should NOT include validation section")
	}
}

func TestCompiler_TaskLevel(t *testing.T) {
	store := newMockArtifactStore()
	store.setArtifact("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "internal/auth", "description": "Auth module", "language": "go"},
		},
	})
	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "TDD required", "scope": "testing"},
		},
	})

	cell := Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	packet, err := compiler.Compile(context.Background(), cell, LevelTask, CompileOpts{
		SpecPath: "specs/001-auth-redesign.md",
		TaskID:   "task-3",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(packet.Sections) == 0 {
		t.Fatal("expected at least one section")
	}

	sectionNames := make(map[string]bool)
	for _, s := range packet.Sections {
		sectionNames[s.Name] = true
	}
	if !sectionNames["doctrine"] {
		t.Error("task level should include doctrine section")
	}
	if !sectionNames["spec-text"] {
		t.Error("task level should include spec-text section")
	}
	if !sectionNames["proof-requirements"] {
		t.Error("task level should include proof-requirements section")
	}
	if sectionNames["architecture"] {
		t.Error("task level should NOT include architecture section")
	}
}

func TestCompiler_SpecLevelMissingSpecPath(t *testing.T) {
	store := newMockArtifactStore()
	cell := Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	_, err := compiler.Compile(context.Background(), cell, LevelSpec, CompileOpts{})
	if err == nil {
		t.Error("expected error when spec path is empty for spec level")
	}
}

func TestCompiler_TaskLevelMissingTaskID(t *testing.T) {
	store := newMockArtifactStore()
	cell := Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	_, err := compiler.Compile(context.Background(), cell, LevelTask, CompileOpts{
		SpecPath: "specs/001.md",
	})
	if err == nil {
		t.Error("expected error when task ID is empty for task level")
	}
}

func TestPacket_NormalizeNilFields(t *testing.T) {
	p := Packet{Level: LevelProject}
	if p.Sections != nil {
		t.Fatal("precondition: Sections should be nil")
	}
	p.NormalizeNilFields()
	if p.Sections == nil {
		t.Error("NormalizeNilFields should set Sections to empty slice")
	}
	if len(p.Sections) != 0 {
		t.Error("NormalizeNilFields should set Sections to empty (not populated) slice")
	}
}

func TestCompiler_ProjectLevelWithInferred(t *testing.T) {
	store := newMockArtifactStore()

	store.setArtifact("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "api", "description": "API layer", "language": "go"},
		},
	})
	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "simplicity", "scope": "all"},
		},
	})

	cellPath := t.TempDir()
	inferredDir := filepath.Join(cellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)
	recent := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	factsJSON := fmt.Sprintf(`[{"fact_id":"f1","category":"entrypoint","statement":"main.go is the entrypoint","confidence":"high","status":"proposed","created_at":%q}]`, recent)
	os.WriteFile(filepath.Join(inferredDir, "facts.json"), []byte(factsJSON), 0o644)

	compiler := NewCompiler(store)
	cell := Cell{Name: "test", CellPath: cellPath}
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

	store.setArtifact("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "api", "description": "API layer", "language": "go"},
		},
	})
	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "simplicity", "scope": "all"},
		},
	})

	cellPath := t.TempDir()
	inferredDir := filepath.Join(cellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)
	recent := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	factsJSON := fmt.Sprintf(`[{"fact_id":"f1","category":"entrypoint","statement":"main.go","confidence":"high","status":"proposed","created_at":%q}]`, recent)
	os.WriteFile(filepath.Join(inferredDir, "facts.json"), []byte(factsJSON), 0o644)

	compiler := NewCompiler(store)
	cell := Cell{Name: "test", CellPath: cellPath}
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
	store := newMockArtifactStore()

	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "TDD required", "scope": "testing"},
		},
	})

	cellPath := t.TempDir()
	inferredDir := filepath.Join(cellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)
	recent := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	factsJSON := fmt.Sprintf(`[{"fact_id":"f1","category":"entrypoint","statement":"main.go","confidence":"high","status":"proposed","created_at":%q}]`, recent)
	os.WriteFile(filepath.Join(inferredDir, "facts.json"), []byte(factsJSON), 0o644)

	compiler := NewCompiler(store)
	cell := Cell{Name: "test", CellPath: cellPath}
	pkt, err := compiler.Compile(context.Background(), cell, LevelTask, CompileOpts{
		SpecPath:        "specs/001-test.md",
		TaskID:          "task-1",
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

func TestCompiler_InferredExcludesRejectedFacts(t *testing.T) {
	store := newMockArtifactStore()
	store.setArtifact("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "api", "description": "API layer", "language": "go"},
		},
	})
	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "simplicity", "scope": "all"},
		},
	})

	cellPath := t.TempDir()
	inferredDir := filepath.Join(cellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)
	recent := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	// All facts are rejected — section should be absent.
	factsJSON := fmt.Sprintf(`[
		{"fact_id":"f1","category":"entrypoint","statement":"main.go is the entrypoint","confidence":"high","status":"rejected","created_at":%q},
		{"fact_id":"f2","category":"pattern","statement":"uses MVC","confidence":"medium","status":"rejected","created_at":%q}
	]`, recent, recent)
	os.WriteFile(filepath.Join(inferredDir, "facts.json"), []byte(factsJSON), 0o644)

	compiler := NewCompiler(store)
	cell := Cell{Name: "test", CellPath: cellPath}
	pkt, err := compiler.Compile(context.Background(), cell, LevelProject, CompileOpts{
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for _, s := range pkt.Sections {
		if strings.Contains(s.Name, "inferred") {
			t.Error("rejected facts should be excluded; inferred section should not appear")
		}
	}
}

func TestCompiler_InferredExcludesSupersededFacts(t *testing.T) {
	store := newMockArtifactStore()
	store.setArtifact("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "api", "description": "API layer", "language": "go"},
		},
	})
	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "simplicity", "scope": "all"},
		},
	})

	cellPath := t.TempDir()
	inferredDir := filepath.Join(cellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)
	recent := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	// One superseded, one active — only the active fact should appear.
	factsJSON := fmt.Sprintf(`[
		{"fact_id":"f1","category":"entrypoint","statement":"old entrypoint","confidence":"high","status":"superseded","created_at":%q},
		{"fact_id":"f2","category":"entrypoint","statement":"new entrypoint","confidence":"high","status":"accepted","created_at":%q}
	]`, recent, recent)
	os.WriteFile(filepath.Join(inferredDir, "facts.json"), []byte(factsJSON), 0o644)

	compiler := NewCompiler(store)
	cell := Cell{Name: "test", CellPath: cellPath}
	pkt, err := compiler.Compile(context.Background(), cell, LevelProject, CompileOpts{
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	var inferredSection Section
	found := false
	for _, s := range pkt.Sections {
		if s.Name == "inferred-observations" {
			inferredSection = s
			found = true
		}
	}
	if !found {
		t.Fatal("expected inferred section with one active fact")
	}
	if strings.Contains(inferredSection.Content, "old entrypoint") {
		t.Error("superseded fact should be excluded from content")
	}
	if !strings.Contains(inferredSection.Content, "new entrypoint") {
		t.Error("confirmed fact should be included in content")
	}
	if len(inferredSection.Facts) != 1 {
		t.Errorf("expected 1 fact ref, got %d", len(inferredSection.Facts))
	}
}

func TestCompiler_InferredExcludesExpiredFacts(t *testing.T) {
	store := newMockArtifactStore()
	store.setArtifact("architecture", map[string]any{
		"modules": []map[string]any{
			{"name": "api", "description": "API layer", "language": "go"},
		},
	})
	store.setArtifact("doctrine", map[string]any{
		"rules": []map[string]any{
			{"id": "r1", "summary": "simplicity", "scope": "all"},
		},
	})

	cellPath := t.TempDir()
	inferredDir := filepath.Join(cellPath, "inferred")
	os.MkdirAll(inferredDir, 0o755)
	expired := time.Now().Add(-45 * 24 * time.Hour).Format(time.RFC3339)
	recent := time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339)
	// One expired (45 days old), one recent — only recent should appear.
	factsJSON := fmt.Sprintf(`[
		{"fact_id":"f1","category":"entrypoint","statement":"stale fact","confidence":"high","status":"proposed","created_at":%q},
		{"fact_id":"f2","category":"pattern","statement":"fresh fact","confidence":"high","status":"proposed","created_at":%q}
	]`, expired, recent)
	os.WriteFile(filepath.Join(inferredDir, "facts.json"), []byte(factsJSON), 0o644)

	compiler := NewCompiler(store)
	cell := Cell{Name: "test", CellPath: cellPath}
	pkt, err := compiler.Compile(context.Background(), cell, LevelProject, CompileOpts{
		IncludeInferred: true,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	var inferredSection Section
	found := false
	for _, s := range pkt.Sections {
		if s.Name == "inferred-observations" {
			inferredSection = s
			found = true
		}
	}
	if !found {
		t.Fatal("expected inferred section with one fresh fact")
	}
	if strings.Contains(inferredSection.Content, "stale fact") {
		t.Error("expired fact (45 days old) should be excluded")
	}
	if !strings.Contains(inferredSection.Content, "fresh fact") {
		t.Error("recent fact should be included")
	}
	if len(inferredSection.Facts) != 1 {
		t.Errorf("expected 1 fact ref, got %d", len(inferredSection.Facts))
	}
}

func TestTrimToBudget(t *testing.T) {
	sections := []Section{
		{Name: "a", Content: "aaaaaaaaaaaaaaaaaaaa", TokenEstimate: 5}, // 20 chars = 5 tokens
		{Name: "b", Content: "bbbbbbbbbbbbbbbbbbbb", TokenEstimate: 5}, // 20 chars = 5 tokens
		{Name: "c", Content: "cccccccccccccccccccc", TokenEstimate: 5}, // 20 chars = 5 tokens
	}
	// Total is 15 tokens; set budget to 8 so at least one section is truncated.
	budget := 8
	result := trimToBudget(sections, budget)

	// Total tokens in result should equal budget.
	totalTokens := 0
	for _, s := range result {
		totalTokens += s.TokenEstimate
	}
	if totalTokens != budget {
		t.Errorf("total tokens = %d, want %d", totalTokens, budget)
	}

	// At least one section should have been truncated (shorter content than original).
	truncated := false
	origByName := map[string]string{}
	for _, s := range sections {
		origByName[s.Name] = s.Content
	}
	for _, s := range result {
		if len(s.Content) < len(origByName[s.Name]) {
			truncated = true
			break
		}
	}
	if !truncated {
		t.Error("expected at least one section to be truncated")
	}
}
