package bead

import "testing"

func TestIsAtomic_AtomicTrueLabel(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID:     "test-bead",
		Labels: []string{"atomic:true"},
	}

	got := IsAtomic(bead, 0, 5)
	if !got {
		t.Errorf("IsAtomic() = %v, want true for atomic:true label", got)
	}
}

func TestIsAtomic_DepthAtMaxDepth(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID: "test-bead",
	}

	got := IsAtomic(bead, 5, 5)
	if !got {
		t.Errorf("IsAtomic(depth=5, maxDepth=5) = %v, want true (at max depth)", got)
	}
}

func TestIsAtomic_DepthBeyondMaxDepth(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID: "test-bead",
	}

	got := IsAtomic(bead, 6, 5)
	if !got {
		t.Errorf("IsAtomic(depth=6, maxDepth=5) = %v, want true (beyond max depth)", got)
	}
}

func TestIsAtomic_SingleFileTarget(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID:               "test-bead",
		ExpectedOutputs: []string{"internal/foo/bar.go"},
	}

	got := IsAtomic(bead, 0, 5)
	if !got {
		t.Errorf("IsAtomic() = %v, want true for single expected output (single-target heuristic)", got)
	}
}

func TestIsAtomic_MultipleFileTargets(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID:               "test-bead",
		ExpectedOutputs: []string{"internal/foo/bar.go", "internal/foo/bar_test.go"},
	}

	got := IsAtomic(bead, 0, 5)
	if got {
		t.Errorf("IsAtomic() = %v, want false for multiple expected outputs", got)
	}
}

func TestIsAtomic_NoTargets(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID: "test-bead",
	}

	got := IsAtomic(bead, 0, 3)
	if got {
		t.Errorf("IsAtomic(depth=0, maxDepth=3) = %v, want false for no targets and low depth", got)
	}
}

func TestIsAtomic_NilBead(t *testing.T) {
	t.Parallel()

	got := IsAtomic(nil, 0, 5)
	if got {
		t.Errorf("IsAtomic(nil) = %v, want false for nil bead", got)
	}
}

func TestIsAtomic_AtomicFalseLabel(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID:               "test-bead",
		Labels:           []string{"atomic:false"},
		ExpectedOutputs: []string{"internal/foo/bar.go"},
	}

	got := IsAtomic(bead, 0, 5)
	if !got {
		t.Errorf("IsAtomic() = %v, want true (single-target takes precedence over atomic:false label)", got)
	}
}

func TestIsAtomic_MultipleFilesAtMaxDepth(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID:               "test-bead",
		ExpectedOutputs: []string{"file1.go", "file2.go", "file3.go"},
	}

	got := IsAtomic(bead, 5, 5)
	if !got {
		t.Errorf("IsAtomic(depth=5, maxDepth=5) = %v, want true (at max depth)", got)
	}
}

func TestIsAtomic_LabelOverridesDepth(t *testing.T) {
	t.Parallel()

	bead := &Bead{
		ID:     "test-bead",
		Labels: []string{"atomic:true"},
	}

	got := IsAtomic(bead, 0, 10)
	if !got {
		t.Errorf("IsAtomic(atomic:true label, depth=0) = %v, want true (label overrides depth)", got)
	}
}

func TestIsAtomic_AllClassificationPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		bead      *Bead
		depth     int
		maxDepth  int
		wantAtomic bool
		reason    string
	}{
		{
			name: "atomic:true label makes atomic",
			bead: &Bead{
				ID:              "test-1",
				Labels:          []string{"atomic:true"},
				ExpectedOutputs: []string{"a.go", "b.go"},
			},
			depth:      0,
			maxDepth:   5,
			wantAtomic: true,
			reason:     "atomic:true label",
		},
		{
			name: "at max depth makes atomic",
			bead: &Bead{
				ID:              "test-2",
				ExpectedOutputs: []string{"a.go", "b.go", "c.go"},
			},
			depth:      5,
			maxDepth:   5,
			wantAtomic: true,
			reason:     "at max decomposition depth",
		},
		{
			name: "beyond max depth makes atomic",
			bead: &Bead{
				ID:              "test-3",
				ExpectedOutputs: []string{"a.go", "b.go", "c.go"},
			},
			depth:      6,
			maxDepth:   5,
			wantAtomic: true,
			reason:     "beyond max decomposition depth",
		},
		{
			name: "single expected output makes atomic",
			bead: &Bead{
				ID:              "test-4",
				ExpectedOutputs: []string{"single.go"},
			},
			depth:      0,
			maxDepth:   5,
			wantAtomic: true,
			reason:     "single-target heuristic (one expected output)",
		},
		{
			name: "multiple outputs at low depth is not atomic",
			bead: &Bead{
				ID:              "test-5",
				ExpectedOutputs: []string{"a.go", "b.go"},
			},
			depth:      0,
			maxDepth:   5,
			wantAtomic: false,
			reason:     "multiple targets at low depth",
		},
		{
			name: "no outputs at low depth is not atomic",
			bead: &Bead{
				ID: "test-6",
			},
			depth:      0,
			maxDepth:   5,
			wantAtomic: false,
			reason:     "no targets at low depth",
		},
		{
			name: "atomic:true overrides multiple outputs",
			bead: &Bead{
				ID:              "test-7",
				Labels:          []string{"atomic:true"},
				ExpectedOutputs: []string{"a.go", "b.go", "c.go"},
			},
			depth:      0,
			maxDepth:   5,
			wantAtomic: true,
			reason:     "atomic:true label takes precedence",
		},
		{
			name: "single output at low depth",
			bead: &Bead{
				ID:              "test-8",
				Labels:          []string{"no-atomic"},
				ExpectedOutputs: []string{"target.go"},
			},
			depth:      1,
			maxDepth:   3,
			wantAtomic: true,
			reason:     "single-target heuristic",
		},
		{
			name: "nil bead is not atomic",
			bead: nil,
			depth: 0,
			maxDepth: 5,
			wantAtomic: false,
			reason: "nil bead",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsAtomic(tt.bead, tt.depth, tt.maxDepth)
			if got != tt.wantAtomic {
				t.Errorf("IsAtomic() = %v, want %v (reason: %s)", got, tt.wantAtomic, tt.reason)
			}
		})
	}
}
