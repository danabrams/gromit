package contextpkt

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
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

func TestTrimToBudget(t *testing.T) {
	sections := []Section{
		{Name: "a", Content: "aaaaaaaaaaaaaaaaaaaa", TokenEstimate: 5},  // 20 chars = 5 tokens
		{Name: "b", Content: "bbbbbbbbbbbbbbbbbbbb", TokenEstimate: 5},  // 20 chars = 5 tokens
		{Name: "c", Content: "cccccccccccccccccccc", TokenEstimate: 5},  // 20 chars = 5 tokens
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
