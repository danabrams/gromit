package review

import "testing"

func TestClassifierCopiesOutOfScopeAffectedFiles(t *testing.T) {
	t.Parallel()

	finding := Finding{
		Title:         "description drift",
		Description:   "doc drift",
		InScope:       false,
		AffectedFiles: []string{"docs/README.md"},
	}

	classifier := NewClassifier(nil)
	result := classifier.Classify(nil, []Finding{finding})

	if len(result.OutOfScope) != 1 {
		t.Fatalf("expected 1 out-of-scope finding, got %d", len(result.OutOfScope))
	}

	finding.AffectedFiles[0] = "docs/UPDATED.md"
	if result.OutOfScope[0].AffectedFiles[0] != "docs/README.md" {
		t.Fatalf("out-of-scope affected files mutated = %v", result.OutOfScope[0].AffectedFiles)
	}
}
