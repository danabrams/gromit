package gate

import (
	"testing"
)

func TestSatisfactionTier_Gen0SkipsCheck(t *testing.T) {
	if got := satisfactionTier(0); got != "" {
		t.Errorf("satisfactionTier(0) = %q, want %q", got, "")
	}
}

func TestSatisfactionTier_Gen1ReturnsLow(t *testing.T) {
	if got := satisfactionTier(1); got != "low" {
		t.Errorf("satisfactionTier(1) = %q, want %q", got, "low")
	}
}

func TestSatisfactionTier_Gen2ReturnsMedium(t *testing.T) {
	if got := satisfactionTier(2); got != "medium" {
		t.Errorf("satisfactionTier(2) = %q, want %q", got, "medium")
	}
}

func TestSatisfactionTier_Gen3ReturnsHigh(t *testing.T) {
	if got := satisfactionTier(3); got != "high" {
		t.Errorf("satisfactionTier(3) = %q, want %q", got, "high")
	}
}

func TestSatisfactionTier_Gen5ReturnsHigh(t *testing.T) {
	if got := satisfactionTier(5); got != "high" {
		t.Errorf("satisfactionTier(5) = %q, want %q", got, "high")
	}
}

func TestIsStructuralBead_RefactorTitle(t *testing.T) {
	if !isStructuralBead("Refactor debug command", "") {
		t.Error("expected true for refactor title")
	}
}

func TestIsStructuralBead_TestTitle(t *testing.T) {
	if !isStructuralBead("Add test coverage for router", "") {
		t.Error("expected true for test coverage title")
	}
}

func TestIsStructuralBead_ReorganizeDescription(t *testing.T) {
	if !isStructuralBead("Clean up", "reorganize the debug package") {
		t.Error("expected true for reorganize in description")
	}
}

func TestIsStructuralBead_NormalBead(t *testing.T) {
	if isStructuralBead("Implement debug command entry point", "") {
		t.Error("expected false for normal bead")
	}
}

func TestIsStructuralBead_ExtractTitle(t *testing.T) {
	if !isStructuralBead("Extract validation logic into helper", "") {
		t.Error("expected true for extract title")
	}
}

func TestIsStructuralBead_MoveTitle(t *testing.T) {
	if !isStructuralBead("Move types to shared package", "") {
		t.Error("expected true for move title")
	}
}

func TestIsStructuralBead_RenameTitle(t *testing.T) {
	if !isStructuralBead("Rename adapter methods for consistency", "") {
		t.Error("expected true for rename title")
	}
}
