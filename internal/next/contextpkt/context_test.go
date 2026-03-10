package contextpkt

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/next/architecture"
	"github.com/danabrams/gromit/internal/next/doctrine"
)

type mockArtifactStore struct {
	arch     architecture.Architecture
	doctrine doctrine.Doctrine
}

func (m *mockArtifactStore) Read(cellPath string, artifact string, dest any) error {
	switch d := dest.(type) {
	case *architecture.Architecture:
		*d = m.arch
	case *doctrine.Doctrine:
		*d = m.doctrine
	default:
		return fmt.Errorf("mock: unsupported type %T", dest)
	}
	return nil
}
func (m *mockArtifactStore) Write(cellPath string, artifact string, src any) error { return nil }
func (m *mockArtifactStore) Exists(cellPath string, artifact string) bool          { return true }

func TestCompiler_ProjectLevel(t *testing.T) {
	store := &mockArtifactStore{
		arch: architecture.Architecture{
			Modules: []architecture.Module{
				{Name: "internal/auth", Description: "Auth module", Language: "go"},
			},
		},
		doctrine: doctrine.Doctrine{
			Rules: []doctrine.Rule{
				{ID: "arch-001", Summary: "Use hexagonal architecture", Scope: "architecture"},
			},
		},
	}

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
	store := &mockArtifactStore{
		arch: architecture.Architecture{
			Modules: []architecture.Module{
				{Name: "internal/auth", Description: "Auth module with a long description that takes up tokens", Language: "go"},
				{Name: "internal/billing", Description: "Billing module with another long description", Language: "go"},
			},
		},
		doctrine: doctrine.Doctrine{
			Rules: []doctrine.Rule{
				{ID: "r1", Summary: "Rule one", Scope: "all"},
				{ID: "r2", Summary: "Rule two", Scope: "all"},
			},
		},
	}

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
	store := &mockArtifactStore{
		arch: architecture.Architecture{
			Modules: []architecture.Module{
				{Name: "internal/auth", Description: "Auth module", Language: "go"},
			},
		},
		doctrine: doctrine.Doctrine{
			Rules: []doctrine.Rule{
				{ID: "r1", Summary: "Use hexagonal architecture", Scope: "architecture"},
			},
		},
	}

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
	store := &mockArtifactStore{
		arch: architecture.Architecture{
			Modules: []architecture.Module{
				{Name: "internal/auth", Description: "Auth module", Language: "go"},
			},
		},
		doctrine: doctrine.Doctrine{
			Rules: []doctrine.Rule{
				{ID: "r1", Summary: "TDD required", Scope: "testing"},
			},
		},
	}

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
	store := &mockArtifactStore{}
	cell := Cell{Name: "test", CellPath: t.TempDir()}
	compiler := NewCompiler(store)

	_, err := compiler.Compile(context.Background(), cell, LevelSpec, CompileOpts{})
	if err == nil {
		t.Error("expected error when spec path is empty for spec level")
	}
}

func TestCompiler_TaskLevelMissingTaskID(t *testing.T) {
	store := &mockArtifactStore{}
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
