package prompt

import "testing"

func TestStaticPreambleCacheKey_StableAcrossSectionOrder(t *testing.T) {
	sectionsA := map[string]string{
		"rules":    "# Rules\nAlways test",
		"template": "PROMPT_build.md",
		"spec":     "# Spec\nBehavior",
	}
	sectionsB := map[string]string{
		"spec":     "# Spec\nBehavior",
		"template": "PROMPT_build.md",
		"rules":    "# Rules\nAlways test",
	}

	keyA := StaticPreambleCacheKey("build", sectionsA)
	keyB := StaticPreambleCacheKey("build", sectionsB)

	if keyA == "" || keyB == "" {
		t.Fatalf("expected non-empty keys, got keyA=%q keyB=%q", keyA, keyB)
	}
	if keyA != keyB {
		t.Fatalf("expected keys to match for equivalent static preambles, got %q vs %q", keyA, keyB)
	}
}

func TestStaticPreambleCacheKeyWithExclusions_IgnoresDynamicSections(t *testing.T) {
	exclude := map[string]struct{}{"bead_title": {}}
	sectionsA := map[string]string{
		"rules":      "# Rules\nAlways test",
		"template":   "PROMPT_build.md",
		"bead_title": "Task A",
	}
	sectionsB := map[string]string{
		"rules":      "# Rules\nAlways test",
		"template":   "PROMPT_build.md",
		"bead_title": "Task B",
	}

	keyA := StaticPreambleCacheKeyWithExclusions("build", sectionsA, exclude)
	keyB := StaticPreambleCacheKeyWithExclusions("build", sectionsB, exclude)

	if keyA != keyB {
		t.Fatalf("expected excluded dynamic section changes to not affect key, got %q vs %q", keyA, keyB)
	}
}

func TestCanonicalizeStaticPreamble_StableAndRepeatable(t *testing.T) {
	sections := map[string]string{
		"template": "PROMPT_build.md",
		"rules":    "rules",
		"spec":     "spec",
	}

	first := CanonicalizeStaticPreamble(sections, nil)
	second := CanonicalizeStaticPreamble(sections, nil)

	if first == "" {
		t.Fatal("expected canonical serialization to be non-empty")
	}
	if first != second {
		t.Fatalf("expected repeatable canonical serialization, got %q then %q", first, second)
	}
	want := "rules=rules\nspec=spec\ntemplate=PROMPT_build.md"
	if first != want {
		t.Fatalf("canonical serialization mismatch\nwant: %q\n got: %q", want, first)
	}
}
