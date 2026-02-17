package runtypes

import (
	"strings"
	"testing"
)

func TestTruncateOutput_ShortOutputUnchanged(t *testing.T) {
	input := "some short output\nline two\n"
	result := TruncateOutput(input)
	if result != input {
		t.Errorf("expected short output unchanged, got %q", result)
	}
}

func TestTruncateOutput_LongOutputKeepsTail(t *testing.T) {
	// Build input larger than 50KB
	line := "FAIL: TestSomething/case_one --- some error message here\n"
	var builder []byte
	for len(builder) < 60*1024 {
		builder = append(builder, line...)
	}
	input := string(builder)

	result := TruncateOutput(input)

	if len(result) >= len(input) {
		t.Error("expected result to be shorter than input")
	}
	if len(result) > maxOutputBytes+200 { // small allowance for marker
		t.Errorf("result length %d exceeds cap (marker included)", len(result))
	}
	// The tail of the original should be preserved
	tail := input[len(input)-1000:]
	if !strings.Contains(result, tail) {
		t.Error("expected result to contain the tail of the original output")
	}
}
