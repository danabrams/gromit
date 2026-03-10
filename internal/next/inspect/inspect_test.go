package inspect

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
	"github.com/danabrams/gromit/internal/next/projectcell"
)

type mockExtractor struct {
	name  string
	facts []fact.Fact
}

func (m *mockExtractor) Name() string                                 { return m.name }
func (m *mockExtractor) Extract(repoPath string) ([]fact.Fact, error) { return m.facts, nil }

type mockInferrer struct {
	facts []fact.Fact
}

func (m *mockInferrer) Infer(ctx context.Context, observed []fact.Fact) ([]fact.Fact, error) {
	return m.facts, nil
}

func TestDefaultInspector_Inspect(t *testing.T) {
	cell := projectcell.Cell{
		Name:     "test",
		RepoPath: t.TempDir(),
		CellPath: t.TempDir(),
	}

	observed := []fact.Fact{fact.New("f1", fact.Observed, "go.mod exists", "gomod")}
	inferred := []fact.Fact{fact.New("f2", fact.Inferred, "uses DDD", "llm")}

	inspector := NewInspector(
		[]Extractor{&mockExtractor{name: "gomod", facts: observed}},
		&mockInferrer{facts: inferred},
	)

	result, err := inspector.Inspect(context.Background(), cell)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(result.Observed) != 1 || len(result.Inferred) != 1 {
		t.Errorf("got %d observed, %d inferred; want 1, 1", len(result.Observed), len(result.Inferred))
	}
}
