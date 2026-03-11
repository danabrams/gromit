package inspect

import (
	"context"

	"github.com/danabrams/gromit/internal/next/fact"
)

// Cell contains the information the inspector needs about a project cell.
type Cell struct {
	Name     string
	RepoPath string
	CellPath string
}

// Result holds the output of a full repo inspection pass.
type Result struct {
	Observed []fact.Fact
	Inferred []fact.Fact
}

// NormalizeNilFields maps nil slices/maps to empty values.
// See CLAUDE.md nil-field normalization visibility convention.
func (r *Result) NormalizeNilFields() {
	if r.Observed == nil {
		r.Observed = []fact.Fact{}
	}
	if r.Inferred == nil {
		r.Inferred = []fact.Fact{}
	}
}

// Extractor produces observed facts from a repository path.
type Extractor interface {
	Name() string
	Extract(repoPath string) ([]fact.Fact, error)
}

// Inferrer derives inferred facts from a set of observed facts.
type Inferrer interface {
	Infer(ctx context.Context, observed []fact.Fact) ([]fact.Fact, error)
}

// Inspector performs repo inspection and writes artifacts to a project cell.
type Inspector interface {
	Inspect(ctx context.Context, cell Cell) (Result, error)
}
