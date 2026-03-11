package executor

import (
	"testing"
)

type fakeGit struct {
	files []string
}

func (f *fakeGit) DiffFiles(workDir string) ([]string, error) {
	return f.files, nil
}

func TestInspectChanges_ReturnsModifiedFiles(t *testing.T) {
	git := &fakeGit{files: []string{"pkg/parser/parser.go", "pkg/parser/parser_test.go"}}
	files, err := InspectChanges(git, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d", len(files))
	}
}

func TestInspectChanges_EmptyDiff(t *testing.T) {
	git := &fakeGit{files: []string{}}
	files, err := InspectChanges(git, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("want 0 files, got %d", len(files))
	}
}
