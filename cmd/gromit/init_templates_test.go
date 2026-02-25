package main

import (
	"bufio"
	"os"
	"testing"
)

// TestInitGoLogicFileIsShort verifies that init.go stays under 500 lines (template constants moved to separate file)
func TestInitGoLogicFileIsShort(t *testing.T) {
	file, err := os.Open("init.go")
	if err != nil {
		t.Fatalf("failed to open init.go: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading init.go: %v", err)
	}

	const maxLines = 500
	if lineCount > maxLines {
		t.Errorf("init.go has %d lines, expected <= %d lines (templates should be extracted)", lineCount, maxLines)
	}
}
