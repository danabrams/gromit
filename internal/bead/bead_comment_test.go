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

    const expected = `// procutil helpers are declared as vars so tests can replace them.`
    if !strings.Contains(string(data), expected) {
        t.Fatalf("missing procutil var comment %q", expected)
    }
}
