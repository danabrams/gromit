package contextpkt_test

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/next/contextpkt"
)

// mockCompiler satisfies contextpkt.Compiler for testing.
type mockCompiler struct {
	packet contextpkt.Packet
	err    error
	called bool
	// capture call args for verification
	gotCell  contextpkt.Cell
	gotLevel contextpkt.Level
	gotOpts  contextpkt.CompileOpts
}

func (m *mockCompiler) Compile(ctx context.Context, cell contextpkt.Cell, level contextpkt.Level, opts contextpkt.CompileOpts) (contextpkt.Packet, error) {
	m.called = true
	m.gotCell = cell
	m.gotLevel = level
	m.gotOpts = opts
	return m.packet, m.err
}

func TestSpecCompilerAdapter_SingleSection(t *testing.T) {
	mc := &mockCompiler{
		packet: contextpkt.Packet{
			Sections: []contextpkt.Section{
				{Name: "architecture", Content: "module layout info"},
			},
		},
	}

	adapter := contextpkt.NewSpecCompilerAdapter(
		mc,
		contextpkt.Cell{Name: "root", CellPath: "/tmp/cell"},
		contextpkt.LevelSpec,
		contextpkt.CompileOpts{SpecPath: "specs/001.md"},
	)

	got, err := adapter.Compile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "## architecture\n\nmodule layout info\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSpecCompilerAdapter_MultipleSections(t *testing.T) {
	mc := &mockCompiler{
		packet: contextpkt.Packet{
			Sections: []contextpkt.Section{
				{Name: "architecture", Content: "arch content"},
				{Name: "doctrine", Content: "doctrine content"},
				{Name: "spec-text", Content: "the spec"},
			},
		},
	}

	adapter := contextpkt.NewSpecCompilerAdapter(
		mc,
		contextpkt.Cell{Name: "root", CellPath: "/tmp/cell"},
		contextpkt.LevelSpec,
		contextpkt.CompileOpts{SpecPath: "specs/001.md"},
	)

	got, err := adapter.Compile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "## architecture\n\narch content\n\n## doctrine\n\ndoctrine content\n\n## spec-text\n\nthe spec\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSpecCompilerAdapter_EmptySections(t *testing.T) {
	mc := &mockCompiler{
		packet: contextpkt.Packet{
			Sections: []contextpkt.Section{},
		},
	}

	adapter := contextpkt.NewSpecCompilerAdapter(
		mc,
		contextpkt.Cell{Name: "root", CellPath: "/tmp/cell"},
		contextpkt.LevelSpec,
		contextpkt.CompileOpts{SpecPath: "specs/001.md"},
	)

	got, err := adapter.Compile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestSpecCompilerAdapter_NilSections(t *testing.T) {
	mc := &mockCompiler{
		packet: contextpkt.Packet{
			Sections: nil,
		},
	}

	adapter := contextpkt.NewSpecCompilerAdapter(
		mc,
		contextpkt.Cell{Name: "root", CellPath: "/tmp/cell"},
		contextpkt.LevelSpec,
		contextpkt.CompileOpts{SpecPath: "specs/001.md"},
	)

	got, err := adapter.Compile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "" {
		t.Errorf("expected empty string for nil sections, got %q", got)
	}
}

func TestSpecCompilerAdapter_CompilerError(t *testing.T) {
	wantErr := errors.New("compile failed")
	mc := &mockCompiler{err: wantErr}

	adapter := contextpkt.NewSpecCompilerAdapter(
		mc,
		contextpkt.Cell{Name: "root", CellPath: "/tmp/cell"},
		contextpkt.LevelSpec,
		contextpkt.CompileOpts{SpecPath: "specs/001.md"},
	)

	_, err := adapter.Compile(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("got error %v, want %v", err, wantErr)
	}
}

func TestSpecCompilerAdapter_PassesConstructorArgs(t *testing.T) {
	mc := &mockCompiler{
		packet: contextpkt.Packet{Sections: []contextpkt.Section{}},
	}

	cell := contextpkt.Cell{Name: "mycell", CellPath: "/tmp/mycell"}
	level := contextpkt.LevelTask
	opts := contextpkt.CompileOpts{
		SpecPath:    "specs/002.md",
		TaskID:      "task-7",
		TokenBudget: 5000,
	}

	adapter := contextpkt.NewSpecCompilerAdapter(mc, cell, level, opts)
	_, err := adapter.Compile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mc.called {
		t.Fatal("expected compiler to be called")
	}
	if mc.gotCell != cell {
		t.Errorf("cell: got %+v, want %+v", mc.gotCell, cell)
	}
	if mc.gotLevel != level {
		t.Errorf("level: got %v, want %v", mc.gotLevel, level)
	}
	if mc.gotOpts != opts {
		t.Errorf("opts: got %+v, want %+v", mc.gotOpts, opts)
	}
}
