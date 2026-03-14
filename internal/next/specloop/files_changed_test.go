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
	// First call: capture baseline.
	if _, err := detect(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Second call: no changes since baseline — expect empty delta.
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

	detect := GitFilesChanged()
	// First call: capture baseline (file is clean at this point).
	if _, err := detect(dir); err != nil {
		t.Fatal(err)
	}

	// Agent modifies the committed file.
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second call: compute delta.
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

	detect := GitFilesChanged()
	// First call: capture baseline.
	if _, err := detect(dir); err != nil {
		t.Fatal(err)
	}

	// Agent creates a new untracked file.
	if err := os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second call: compute delta.
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

	detect := GitFilesChanged()
	// First call: capture baseline.
	if _, err := detect(dir); err != nil {
		t.Fatal(err)
	}

	// Agent modifies existing + adds new.
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "added.go"), []byte("package foo"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second call: compute delta.
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

	detect := GitFilesChanged()
	// First call: capture baseline.
	if _, err := detect(dir); err != nil {
		t.Fatal(err)
	}

	// Agent stages a new file.
	newFile := filepath.Join(dir, "staged.go")
	if err := os.WriteFile(newFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "staged.go")

	// Second call: compute delta.
	files, err := detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should appear exactly once.
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

func TestGitFilesChanged_StatefulClosure_DetectsContentChange(t *testing.T) {
	dir := initGitRepo(t)

	// initial.txt already exists with content "hello" (from initGitRepo).
	// Simulate a pre-existing dirty state: modify it before our baseline.
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("dirty-before-task"), 0644); err != nil {
		t.Fatal(err)
	}

	detect := GitFilesChanged()

	// First call: capture baseline. File is already dirty vs HEAD.
	first, err := detect(dir)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if len(first) != 0 {
		t.Fatalf("first call should return empty baseline marker, got %v", first)
	}

	// Agent modifies initial.txt further (still dirty vs HEAD, but now different from baseline).
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("modified-by-agent"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second call: compute delta from baseline.
	second, err := detect(dir)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected 1 changed file, got %v", second)
	}
	if second[0] != "initial.txt" {
		t.Fatalf("expected initial.txt in delta, got %v", second)
	}
}

func TestGitFilesChanged_StatefulClosure_ResetsAfterSecondCall(t *testing.T) {
	dir := initGitRepo(t)

	detect := GitFilesChanged()

	// Task 1: first call (baseline), second call (delta)
	_, _ = detect(dir)
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("task1-change"), 0644); err != nil {
		t.Fatal(err)
	}
	delta1, err := detect(dir)
	if err != nil {
		t.Fatalf("task1 second call error: %v", err)
	}
	if len(delta1) != 1 || delta1[0] != "initial.txt" {
		t.Fatalf("task1: expected [initial.txt], got %v", delta1)
	}

	// Task 2: closure should reset and treat next call as a new baseline.
	first2, err := detect(dir)
	if err != nil {
		t.Fatalf("task2 first call error: %v", err)
	}
	if len(first2) != 0 {
		t.Fatalf("task2 first call should return empty (new baseline), got %v", first2)
	}

	// Agent modifies another file during task 2.
	if err := os.WriteFile(filepath.Join(dir, "new-task2.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	delta2, err := detect(dir)
	if err != nil {
		t.Fatalf("task2 second call error: %v", err)
	}
	if len(delta2) != 1 || delta2[0] != "new-task2.go" {
		t.Fatalf("task2: expected [new-task2.go], got %v", delta2)
	}
}

func TestGitFilesChanged_StatefulClosure_DetectsDeletedFile(t *testing.T) {
	dir := initGitRepo(t)

	detect := GitFilesChanged()

	// First call: capture baseline (initial.txt exists with "hello")
	_, _ = detect(dir)

	// Agent deletes initial.txt
	if err := os.Remove(filepath.Join(dir, "initial.txt")); err != nil {
		t.Fatal(err)
	}

	// Second call: should detect deletion
	delta, err := detect(dir)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	found := false
	for _, f := range delta {
		if f == "initial.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected initial.txt (deleted) in delta, got %v", delta)
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
