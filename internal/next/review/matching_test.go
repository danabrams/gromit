package review

import "testing"

func TestMatchFindings_ExactMatch(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}

	labeled := LabelDispositions(current, prior)
	if len(labeled) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(labeled))
	}
	if labeled[0].Disposition != DispositionPreExisting {
		t.Errorf("expected pre-existing, got %q", labeled[0].Disposition)
	}
}

func TestMatchFindings_SubstringMatch(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty; line shifted to 45"},
	}

	labeled := LabelDispositions(current, prior)
	if labeled[0].Disposition != DispositionPreExisting {
		t.Errorf("substring match should yield pre-existing, got %q", labeled[0].Disposition)
	}
}

func TestMatchFindings_DifferentFile_IsNew(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "router.go", Description: "nil pointer if commands list is empty"},
	}

	labeled := LabelDispositions(current, prior)
	if labeled[0].Disposition != DispositionNew {
		t.Errorf("different file should be new, got %q", labeled[0].Disposition)
	}
}

func TestMatchFindings_DifferentDescription_IsNew(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "handler.go", Description: "duplicated logic in refund calculation"},
	}

	labeled := LabelDispositions(current, prior)
	if labeled[0].Disposition != DispositionNew {
		t.Errorf("different description should be new, got %q", labeled[0].Disposition)
	}
}

func TestMatchFindings_NoPrior_AllNew(t *testing.T) {
	current := []Finding{
		{File: "handler.go", Description: "missing error check"},
		{File: "router.go", Description: "unused variable"},
	}

	labeled := LabelDispositions(current, nil)
	for i, f := range labeled {
		if f.Disposition != DispositionNew {
			t.Errorf("finding[%d]: expected new with no prior, got %q", i, f.Disposition)
		}
	}
}

func TestMatchFindings_ReverseSubstringMatch(t *testing.T) {
	// Short description in prior matches longer current description (reverse substring)
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer"},
	}
	current := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}

	labeled := LabelDispositions(current, prior)
	if labeled[0].Disposition != DispositionPreExisting {
		t.Errorf("reverse substring match should yield pre-existing, got %q", labeled[0].Disposition)
	}
}

func TestMatchFindings_LineNumberDifference_StillMatches(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Line: 10, Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "handler.go", Line: 42, Description: "nil pointer if commands list is empty"},
	}

	labeled := LabelDispositions(current, prior)
	if labeled[0].Disposition != DispositionPreExisting {
		t.Errorf("same file+description but different line should still match as pre-existing, got %q", labeled[0].Disposition)
	}
}

func TestLabelDispositions_AllPreexisting(t *testing.T) {
	prior := []Finding{
		{File: "handler.go", Description: "missing error check"},
		{File: "router.go", Description: "unused variable"},
	}
	current := []Finding{
		{File: "handler.go", Description: "missing error check"},
		{File: "router.go", Description: "unused variable"},
	}

	labeled := LabelDispositions(current, prior)
	for i, f := range labeled {
		if f.Disposition != DispositionPreExisting {
			t.Errorf("finding[%d]: expected pre-existing, got %q", i, f.Disposition)
		}
	}
}

func TestFilterNewBlockingFindings_OnlyNewAboveThreshold(t *testing.T) {
	// Compose LabelDispositions + FilterBlockingFindings to get only new+blocking
	prior := []Finding{
		{File: "handler.go", Description: "nil pointer if commands list is empty"},
	}
	current := []Finding{
		{File: "handler.go", Severity: SeverityError, Description: "nil pointer if commands list is empty"},     // pre-existing
		{File: "router.go", Severity: SeverityError, Description: "missing authorization check"},                // new + blocking
		{File: "router.go", Severity: SeverityInfo, Description: "consider renaming variable"},                  // new but info (never blocks)
		{File: "service.go", Severity: SeveritySuggestion, Description: "could use early return pattern here"}, // new but below threshold
	}

	labeled := LabelDispositions(current, prior)
	// Filter to only blocking at warning threshold
	blocking := FilterBlockingFindings(labeled, SeverityWarning)
	// Then keep only new
	var newBlocking []Finding
	for _, f := range blocking {
		if f.Disposition == DispositionNew {
			newBlocking = append(newBlocking, f)
		}
	}

	if len(newBlocking) != 1 {
		t.Fatalf("expected 1 new+blocking finding, got %d", len(newBlocking))
	}
	if newBlocking[0].Description != "missing authorization check" {
		t.Errorf("expected 'missing authorization check', got %q", newBlocking[0].Description)
	}
}
