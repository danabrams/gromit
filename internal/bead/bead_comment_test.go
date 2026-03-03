package bead

import (
	"os"
	"strings"
	"testing"
)

func TestProcutilVarComment(t *testing.T) {
	data, err := os.ReadFile("bead.go")
	if err != nil {
		t.Fatalf("failed to read bead.go: %v", err)
	}

	const expected = "// ClientDeps centralizes the procutil helpers that Client uses for subprocess management."
	if !strings.Contains(string(data), expected) {
		t.Fatalf("missing ClientDeps comment %q", expected)
	}
}
