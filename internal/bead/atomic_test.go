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
