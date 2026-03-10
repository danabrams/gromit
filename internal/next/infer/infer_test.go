package infer

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
)

func TestStubInferrer_ReturnsEmpty(t *testing.T) {
	stub := NewStubInferrer()
	observed := []fact.Fact{
		fact.New("f1", fact.Observed, "go.mod exists", "gomod"),
	}

	inferred, err := stub.Infer(context.Background(), observed)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if len(inferred) != 0 {
		t.Errorf("expected empty inferred facts, got %d", len(inferred))
	}
}

func TestStubInferrer_ImplementsInferrer(t *testing.T) {
	var _ Inferrer = (*StubInferrer)(nil)
}
