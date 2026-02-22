package runner

import (
	"bufio"
	"os"
	"testing"
)

const fileSizeLimit = 550

func countSourceLines(t *testing.T, filename string) int {
	t.Helper()
	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("open %s: %v", filename, err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		n++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning %s: %v", filename, err)
	}
	return n
}

func TestProcessMethodologyFileSizeLimit(t *testing.T) {
	n := countSourceLines(t, "process_methodology.go")
	if n > fileSizeLimit {
		t.Errorf("process_methodology.go has %d lines, want ≤ %d; extract ATDD red-phase logic to process_methodology_atdd.go", n, fileSizeLimit)
	}
}

func TestConstructorFileSizeLimit(t *testing.T) {
	n := countSourceLines(t, "constructor.go")
	if n > fileSizeLimit {
		t.Errorf("constructor.go has %d lines, want ≤ %d; extract adapter types into constructor_adapters.go", n, fileSizeLimit)
	}
}
