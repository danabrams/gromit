package stages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// SpecCompiler compiles a spec packet from project context.
type SpecCompiler interface {
	Compile(ctx context.Context) (string, error)
}

// CompileStage compiles the spec packet and writes it to the run directory.
type CompileStage struct {
	compiler SpecCompiler
	store    *runstore.Store
}

// NewCompileStage creates a new CompileStage.
func NewCompileStage(compiler SpecCompiler, store *runstore.Store) *CompileStage {
	return &CompileStage{compiler: compiler, store: store}
}

// Name returns the stage name.
func (s *CompileStage) Name() string { return "compile" }

// Run compiles the spec packet and writes it to spec-packet.md in the run dir.
func (s *CompileStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	content, err := s.compiler.Compile(ctx)
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("compile spec packet: %w", err)
	}

	runDir := s.store.RunDir(rs.RunID)
	packetPath := filepath.Join(runDir, "spec-packet.md")
	if err := os.WriteFile(packetPath, []byte(content), 0o644); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write spec packet: %w", err)
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}
