package inspect

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
)

type mockExtractor struct {
	name  string
	facts []fact.Fact
	err   error
}

func (m *mockExtractor) Name() string { return m.name }
func (m *mockExtractor) Extract(repoPath string) ([]fact.Fact, error) {
	return m.facts, m.err
}

type mockInferrer struct {
	facts []fact.Fact
	err   error
}

func (m *mockInferrer) Infer(ctx context.Context, observed []fact.Fact) ([]fact.Fact, error) {
	return m.facts, m.err
}

func TestResult_NormalizeNilFields(t *testing.T) {
	r := Result{}
	if r.Observed != nil || r.Inferred != nil {
		t.Fatal("expected nil slices before NormalizeNilFields")
	}
	r.NormalizeNilFields()
	if r.Observed == nil {
		t.Error("Observed should be non-nil after NormalizeNilFields")
	}
	if len(r.Observed) != 0 {
		t.Errorf("Observed should be empty, got %d", len(r.Observed))
	}
	if r.Inferred == nil {
		t.Error("Inferred should be non-nil after NormalizeNilFields")
	}
	if len(r.Inferred) != 0 {
		t.Errorf("Inferred should be empty, got %d", len(r.Inferred))
	}
}

func TestDefaultInspector_Inspect(t *testing.T) {
	cell := Cell{
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

func TestDefaultInspector_ExtractorError(t *testing.T) {
	cell := Cell{
		Name:     "test",
		RepoPath: t.TempDir(),
		CellPath: t.TempDir(),
	}

	inspector := NewInspector(
		[]Extractor{&mockExtractor{name: "bad-extractor", err: errors.New("disk read failed")}},
		&mockInferrer{},
	)

	_, err := inspector.Inspect(context.Background(), cell)
	if err == nil {
		t.Fatal("expected error from failing extractor, got nil")
	}
	if !strings.Contains(err.Error(), "bad-extractor") {
		t.Errorf("error %q should contain extractor name %q", err.Error(), "bad-extractor")
	}
}

func TestDefaultInspector_InferrerError(t *testing.T) {
	cell := Cell{
		Name:     "test",
		RepoPath: t.TempDir(),
		CellPath: t.TempDir(),
	}

	observed := []fact.Fact{fact.New("f1", fact.Observed, "go.mod exists", "gomod")}

	inspector := NewInspector(
		[]Extractor{&mockExtractor{name: "gomod", facts: observed}},
		&mockInferrer{err: errors.New("LLM timeout")},
	)

	_, err := inspector.Inspect(context.Background(), cell)
	if err == nil {
		t.Fatal("expected error from failing inferrer, got nil")
	}
	if !strings.Contains(err.Error(), "inferrer") {
		t.Errorf("error %q should contain %q", err.Error(), "inferrer")
	}
}
