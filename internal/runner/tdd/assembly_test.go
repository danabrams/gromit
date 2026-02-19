package tdd

import (
	"fmt"
	"testing"
)

func fakeReadFile(contents map[string]string) ReadFileFn {
	return func(path string) (string, error) {
		content, ok := contents[path]
		if !ok {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return content, nil
	}
}

func fakeGetDiff() GetDiffFn {
	return func() (string, error) {
		return "", nil
	}
}

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

func TestAssembleRedHandoffFirstCycleReturnsEmptyMapsAndSpecExcerpt(t *testing.T) {
	state := CycleState{
		CycleNumber:  1,
		MaxCycles:    10,
		Remaining:    []string{"users can log in with valid credentials"},
		TouchedFiles: []string{},
	}

	readFile := fakeReadFile(map[string]string{})
	getDiff := fakeGetDiff()

	handoff, err := AssembleRedHandoff(state, readFile, getDiff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if handoff.SpecExcerpt != "users can log in with valid credentials" {
		t.Fatalf("expected spec excerpt from Remaining[0], got %q", handoff.SpecExcerpt)
	}
	if len(handoff.TestFiles) != 0 {
		t.Fatalf("expected empty test files on first cycle, got %d", len(handoff.TestFiles))
	}
	if len(handoff.ImplFiles) != 0 {
		t.Fatalf("expected empty impl files on first cycle, got %d", len(handoff.ImplFiles))
	}
}
