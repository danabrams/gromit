package tdd

import (
	"testing"
)

func TestClassifyTouchedFilesSeparatesTestFromImpl(t *testing.T) {
	paths := []string{
		"internal/foo/bar.go",
		"internal/foo/bar_test.go",
		"internal/baz/qux.go",
	}

	testFiles, implFiles := ClassifyTouchedFiles(paths)

	if len(testFiles) != 1 {
		t.Fatalf("expected 1 test file, got %d", len(testFiles))
	}
	if testFiles[0] != "internal/foo/bar_test.go" {
		t.Fatalf("expected bar_test.go, got %s", testFiles[0])
	}
	if len(implFiles) != 2 {
		t.Fatalf("expected 2 impl files, got %d", len(implFiles))
	}
	if implFiles[0] != "internal/foo/bar.go" {
		t.Fatalf("expected bar.go first, got %s", implFiles[0])
	}
	if implFiles[1] != "internal/baz/qux.go" {
		t.Fatalf("expected qux.go second, got %s", implFiles[1])
	}
}
