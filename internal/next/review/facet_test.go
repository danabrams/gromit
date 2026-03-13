package review

import "testing"

func TestRegistry_DefaultFacets(t *testing.T) {
	reg := NewRegistry()

	sa, ok := reg.Get("spec_alignment")
	if !ok {
		t.Fatal("registry missing spec_alignment")
	}
	if sa.DefaultTier != "high" {
		t.Errorf("spec_alignment.DefaultTier = %q, want %q", sa.DefaultTier, "high")
	}

	cq, ok := reg.Get("code_quality")
	if !ok {
		t.Fatal("registry missing code_quality")
	}
	if cq.DefaultTier != "medium" {
		t.Errorf("code_quality.DefaultTier = %q, want %q", cq.DefaultTier, "medium")
	}
}

func TestRegistry_AdditionalFacets(t *testing.T) {
	reg := NewRegistry()
	for _, name := range []string{"logic_gaps", "test_coverage", "architecture_drift"} {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("registry missing additional facet %q", name)
		}
	}
}

func TestRegistry_ListNames(t *testing.T) {
	reg := NewRegistry()
	names := reg.ListNames()
	if len(names) < 5 {
		t.Errorf("expected at least 5 facets, got %d", len(names))
	}
}

func TestRegistry_UnknownFacet(t *testing.T) {
	reg := NewRegistry()
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("Get(\"nonexistent\") should return false")
	}
}

func TestRegistry_Select_AllValid(t *testing.T) {
	reg := NewRegistry()
	defs, err := reg.Select([]string{"spec_alignment", "code_quality"})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if len(defs) != 2 {
		t.Errorf("expected 2 facet defs, got %d", len(defs))
	}
}

func TestRegistry_Select_UnknownFacet(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Select([]string{"spec_alignment", "does_not_exist"})
	if err == nil {
		t.Fatal("expected error for unknown facet")
	}
}

func TestRegistry_Select_Empty(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Select([]string{})
	if err == nil {
		t.Fatal("expected error for empty facet list")
	}
}

func TestFacetDef_HasPromptTemplate(t *testing.T) {
	reg := NewRegistry()
	for _, name := range reg.ListNames() {
		facet, _ := reg.Get(name)
		if facet.PromptTemplate == "" {
			t.Errorf("facet %q has empty PromptTemplate", name)
		}
	}
}
