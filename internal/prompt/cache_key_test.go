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
