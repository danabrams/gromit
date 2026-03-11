package enrich

import (
	"encoding/json"
	"testing"
)

func TestInferredFactStatus_String(t *testing.T) {
	tests := []struct {
		s    Status
		want string
	}{
		{StatusProposed, "proposed"},
		{StatusAccepted, "accepted"},
		{StatusRejected, "rejected"},
		{StatusSuperseded, "superseded"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestInferredFactStatus_JSONRoundTrip(t *testing.T) {
	f := InferredFact{
		FactID:   "abc123",
		Category: CategoryComponentBoundary,
		Status:   StatusProposed,
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var f2 InferredFact
	if err := json.Unmarshal(data, &f2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if f2.Status != StatusProposed {
		t.Errorf("Status = %v, want proposed", f2.Status)
	}
	if f2.Category != CategoryComponentBoundary {
		t.Errorf("Category = %q, want %q", f2.Category, CategoryComponentBoundary)
	}
}

func TestInferredFact_ContentHash(t *testing.T) {
	f1 := InferredFact{
		Category:  CategoryComponentBoundary,
		Statement: "payments-api uses hexagonal architecture",
	}
	f2 := InferredFact{
		Category:  CategoryComponentBoundary,
		Statement: "payments-api uses hexagonal architecture",
	}
	f3 := InferredFact{
		Category:  CategoryGlossaryTerm,
		Statement: "payments-api uses hexagonal architecture",
	}

	id1 := f1.ComputeID()
	id2 := f2.ComputeID()
	id3 := f3.ComputeID()

	if id1 != id2 {
		t.Errorf("same content should produce same ID: %q != %q", id1, id2)
	}
	if id1 == id3 {
		t.Error("different category should produce different ID")
	}
	if id1 == "" {
		t.Error("ID should not be empty")
	}
}

func TestAllCategories(t *testing.T) {
	cats := AllCategories()
	if len(cats) != 8 {
		t.Errorf("expected 8 categories, got %d", len(cats))
	}
}
