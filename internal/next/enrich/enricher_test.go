package enrich

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/fact"
)

func TestMockEnricher_ImplementsInterface(t *testing.T) {
	var _ CategoryEnricher = (*MockEnricher)(nil)
}

func TestMockEnricher_ReturnsConfiguredFacts(t *testing.T) {
	mock := &MockEnricher{
		Facts: []InferredFact{
			{FactID: "test-1", Category: CategoryEntrypoint, Statement: "main.go"},
		},
	}

	result, err := mock.Enrich(context.Background(), CategoryEntrypoint, []fact.Fact{}, EnrichInput{})
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if result.FactCount != 1 {
		t.Errorf("FactCount = %d, want 1", result.FactCount)
	}
	if len(result.Facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(result.Facts))
	}
}
