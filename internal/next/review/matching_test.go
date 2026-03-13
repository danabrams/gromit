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
