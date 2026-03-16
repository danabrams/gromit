package calc

import "testing"

func TestSubtract(t *testing.T) {
	if got := Subtract(5, 3); got != 2 {
		t.Errorf("Subtract(5, 3) = %d, want 2", got)
	}
	if got := Subtract(0, 0); got != 0 {
		t.Errorf("Subtract(0, 0) = %d, want 0", got)
	}
	if got := Subtract(3, 5); got != -2 {
		t.Errorf("Subtract(3, 5) = %d, want -2", got)
	}
}
