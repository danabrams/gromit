package specloop

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

func TestGitFilesChanged_NonGitDir_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	detect := GitFilesChanged()
	files, err := detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected empty list for non-git dir, got %v", files)
	}
}

func TestGitFilesChanged_CleanRepo_ReturnsEmpty(t *testing.T) {
	dir := initGitRepo(t)
	detect := GitFilesChanged()
	files, err := detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected empty list for clean repo, got %v", files)
	}
}

func TestGitFilesChanged_ModifiedFile(t *testing.T) {
	dir := initGitRepo(t)

	// Modify committed file
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	detect := GitFilesChanged()
	files, err := detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0] != "initial.txt" {
		t.Fatalf("expected [initial.txt], got %v", files)
	}
}

func TestGitFilesChanged_NewUntrackedFile(t *testing.T) {
	dir := initGitRepo(t)

	// Create new untracked file
	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	detect := GitFilesChanged()
	files, err := detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0] != "new.go" {
		t.Fatalf("expected [new.go], got %v", files)
	}
}

func TestGitFilesChanged_MixedChanges(t *testing.T) {
	dir := initGitRepo(t)

	// Modify existing + add new
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "added.go"), []byte("package foo"), 0644); err != nil {
		t.Fatal(err)
	}

	detect := GitFilesChanged()
	files, err := detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Strings(files)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %v", files)
	}
	if files[0] != "added.go" || files[1] != "initial.txt" {
		t.Fatalf("expected [added.go initial.txt], got %v", files)
	}
}

func TestGitFilesChanged_NoDuplicates(t *testing.T) {
	dir := initGitRepo(t)

	// Stage a new file (it will appear in diff --name-only HEAD and possibly in ls-files)
	newFile := filepath.Join(dir, "staged.go")
	if err := os.WriteFile(newFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "staged.go")

	detect := GitFilesChanged()
	files, err := detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should appear exactly once
	count := 0
	for _, f := range files {
		if f == "staged.go" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected staged.go exactly once, got %d times in %v", count, files)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"\n", nil},
		{"a.go\nb.go\n", []string{"a.go", "b.go"}},
		{"  a.go  \n  b.go  \n", []string{"a.go", "b.go"}},
	}
	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitLines(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

// initGitRepo creates a temp git repo with one committed file.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial")
	return dir
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
