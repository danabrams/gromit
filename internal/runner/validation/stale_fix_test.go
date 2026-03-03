package validation

import "testing"

func TestStaleFixDetectorDetectsRepeatedNoOp(t *testing.T) {
	detector := NewStaleFixDetector()
	snapshot := StaleFixSnapshot{
		ChangedFiles:      []string{"a.go", "b.go"},
		ErrorCategories:   []string{"test_failure"},
		ChangedFilesKnown: true,
	}

	first := detector.RecordAttempt(snapshot)
	if first.StaleFixDetected {
		t.Fatalf("unexpected stale detection on first attempt")
	}

	second := detector.RecordAttempt(snapshot)
	if !second.StaleFixDetected {
		t.Fatalf("expected stale detection when files and categories repeat")
	}
	if !second.ChangedFilesMatch {
		t.Fatalf("expected changed files to match")
	}
	if !second.ErrorCategoriesMatch {
		t.Fatalf("expected error categories to match")
	}

	snapshot.ErrorCategories = []string{"lint_error"}
	third := detector.RecordAttempt(snapshot)
	if third.StaleFixDetected {
		t.Fatalf("expected detection to reset when category changes")
	}
	if !third.ChangedFilesMatch {
		t.Fatalf("expected changed files to still match")
	}
	if third.ErrorCategoriesMatch {
		t.Fatalf("error categories should differ after update")
	}
}
