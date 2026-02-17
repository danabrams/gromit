package runtypes

import (
	"testing"
)

func TestTruncateOutput_ShortOutputUnchanged(t *testing.T) {
	input := "some short output\nline two\n"
	result := TruncateOutput(input)
	if result != input {
		t.Errorf("expected short output unchanged, got %q", result)
	}
}
