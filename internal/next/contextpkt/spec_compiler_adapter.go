package contextpkt

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/next/specloop/stages"
)

// Compile-time interface check: SpecCompilerAdapter must satisfy stages.SpecCompiler.
var _ stages.SpecCompiler = (*SpecCompilerAdapter)(nil)

// SpecCompilerAdapter adapts the contextpkt.Compiler interface to the
// stages.SpecCompiler interface by capturing cell, level, and opts at
// construction time. It renders compiled Packet sections as readable
// markdown text.
type SpecCompilerAdapter struct {
	compiler Compiler
	cell     Cell
	level    Level
	opts     CompileOpts
}

// NewSpecCompilerAdapter creates a SpecCompilerAdapter that will call
// compiler.Compile with the given cell, level, and opts on each invocation.
func NewSpecCompilerAdapter(compiler Compiler, cell Cell, level Level, opts CompileOpts) *SpecCompilerAdapter {
	return &SpecCompilerAdapter{
		compiler: compiler,
		cell:     cell,
		level:    level,
		opts:     opts,
	}
}

// Compile calls the underlying Compiler and renders the resulting Packet
// sections as markdown text. Each section is formatted as:
//
//	## {Name}\n\n{Content}\n\n
func (a *SpecCompilerAdapter) Compile(ctx context.Context) (string, error) {
	pkt, err := a.compiler.Compile(ctx, a.cell, a.level, a.opts)
	if err != nil {
		return "", fmt.Errorf("spec compiler adapter: %w", err)
	}

	var sb strings.Builder
	for _, s := range pkt.Sections {
		fmt.Fprintf(&sb, "## %s\n\n%s\n\n", s.Name, s.Content)
	}
	return sb.String(), nil
}
