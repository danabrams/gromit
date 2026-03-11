package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type fakeCompiler struct {
	content string
	err     error
}

func (f *fakeCompiler) Compile(ctx context.Context) (string, error) {
	return f.content, f.err
}

// Verify CompileStage satisfies the Stage interface.
var _ specloop.Stage = (*CompileStage)(nil)

func TestCompileStage_WritesSpecPacket(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")

	// Create run dir
	runDir := store.RunDir(rs.RunID)
	os.MkdirAll(runDir, 0o755)

	compiler := &fakeCompiler{content: "compiled content"}
	stage := NewCompileStage(compiler, store, nil)

	if stage.Name() != "compile" {
		t.Fatalf("expected name 'compile', got %q", stage.Name())
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	packetPath := filepath.Join(runDir, "spec-packet.md")
	data, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatalf("spec-packet.md not written: %v", err)
	}
	if string(data) != "compiled content" {
		t.Fatalf("expected 'compiled content', got %q", string(data))
	}
}
