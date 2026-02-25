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

// TestExpectedOutputsOrTitle_PrefersProvidedOutputs ensures outputs are returned
// when provided.
func TestExpectedOutputsOrTitle_PrefersProvidedOutputs(t *testing.T) {
	outputs := ExpectedOutputsOrTitle([]string{"one", "two"}, "fallback")
	if len(outputs) != 2 || outputs[0] != "one" || outputs[1] != "two" {
		t.Fatalf("ExpectedOutputsOrTitle([]string{\"one\", \"two\"}, \"fallback\") = %v, want [one two]", outputs)
	}
}

// TestExpectedOutputsOrTitle_FallsBackToTrimmedTitle ensures title is used
// when outputs are empty.
func TestExpectedOutputsOrTitle_FallsBackToTrimmedTitle(t *testing.T) {
	outputs := ExpectedOutputsOrTitle(nil, "  Important work  ")
	if len(outputs) != 1 || outputs[0] != "Important work" {
		t.Fatalf("ExpectedOutputsOrTitle(nil, \"  Important work  \") = %v, want [Important work]", outputs)
	}
}

// TestExpectedOutputsOrTitle_ReturnsEmptyWhenTitleIsBlank ensures empty slice
// is returned when title is blank.
func TestExpectedOutputsOrTitle_ReturnsEmptyWhenTitleIsBlank(t *testing.T) {
	outputs := ExpectedOutputsOrTitle(nil, "   ")
	if len(outputs) != 0 {
		t.Fatalf("ExpectedOutputsOrTitle(nil, \"   \") = %v, want []", outputs)
	}
}
