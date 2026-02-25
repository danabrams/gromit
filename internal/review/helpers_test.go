package review

import "testing"

// TestBuildReviewBeadLabels_PrependsSingleLabel ensures BuildReviewBeadLabels
// always starts with the from-review label.
func TestBuildReviewBeadLabels_PrependsSingleLabel(t *testing.T) {
	labels := BuildReviewBeadLabels([]string{"bug"})
	if len(labels) != 2 {
		t.Fatalf("got %d labels, want 2", len(labels))
	}
	if labels[0] != "from-review" {
		t.Fatalf("first label = %q, want 'from-review'", labels[0])
	}
	if labels[1] != "bug" {
		t.Fatalf("second label = %q, want 'bug'", labels[1])
	}
}

// TestBuildReviewBeadLabels_DeduplicatesExistingLabel ensures we don't append
// a second from-review label.
func TestBuildReviewBeadLabels_DeduplicatesExistingLabel(t *testing.T) {
	labels := BuildReviewBeadLabels([]string{"from-review", "bug"})
	if len(labels) != 2 {
		t.Fatalf("got %d labels, want 2", len(labels))
	}
	if labels[0] != "from-review" {
		t.Fatalf("first label = %q, want 'from-review'", labels[0])
	}
	if labels[1] != "bug" {
		t.Fatalf("second label = %q, want 'bug'", labels[1])
	}
}

// TestBuildReviewBeadLabels_PreservesOriginalLabelsOrder ensures incoming
// labels retain their relative ordering.
func TestBuildReviewBeadLabels_PreservesOriginalLabelsOrder(t *testing.T) {
	labels := BuildReviewBeadLabels([]string{"bug", "enhancement", "docs"})
	if len(labels) != 4 {
		t.Fatalf("got %d labels, want 4", len(labels))
	}
	expected := []string{"from-review", "bug", "enhancement", "docs"}
	for i, label := range expected {
		if labels[i] != label {
			t.Fatalf("labels[%d] = %q, want %q", i, labels[i], label)
		}
	}
}
