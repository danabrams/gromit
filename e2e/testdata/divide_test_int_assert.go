package calc

import "testing"

func TestDivide(t *testing.T) {
	result := Divide(10, 3)
	if result != 3 {
		t.Fatalf("got %d", result)
	}
}
