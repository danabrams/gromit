package review

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDiffProvider_Interface(t *testing.T) {
	var dp DiffProvider = &fakeDiffProvider{diff: "some diff"}
	diff, err := dp.Diff("main")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff != "some diff" {
		t.Errorf("diff = %q, want %q", diff, "some diff")
	}
}

type fakeDiffProvider struct {
	diff string
	err  error
}

func (f *fakeDiffProvider) Diff(baseBranch string) (string, error) {
	return f.diff, f.err
}

func TestGitDiffProvider_Diff_ReturnsRealDiff(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Initialize repo and configure user.
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")

	// Create initial commit on main.
	filePath := dir + "/hello.txt"
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "hello.txt")
	run("git", "commit", "-m", "initial")
	run("git", "branch", "-M", "main")

	// Create feature branch with a modification.
	run("git", "checkout", "-b", "feature")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "hello.txt")
	run("git", "commit", "-m", "modify hello")

	// Run Diff.
	provider := &GitDiffProvider{WorkDir: dir}
	diff, err := provider.Diff("main")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(diff, "hello world") {
		t.Errorf("diff should contain new content 'hello world', got:\n%s", diff)
	}
	if !strings.Contains(diff, "a/hello.txt") {
		t.Errorf("diff should reference hello.txt, got:\n%s", diff)
	}
}

// TestGitDiffProvider_Diff_UncommittedChanges verifies that uncommitted working
// tree changes are captured by git diff. This mirrors the noopGitOps scenario
// where the repo is cp -a'd into a temp dir and the executor modifies files
// without committing.
func TestGitDiffProvider_Diff_UncommittedChanges(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Initialize repo with a commit on main.
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	filePath := dir + "/hello.txt"
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "hello.txt")
	run("git", "commit", "-m", "initial")
	run("git", "branch", "-M", "main")

	// Modify file WITHOUT committing (simulates noopGitOps cp -a scenario).
	if err := os.WriteFile(filePath, []byte("hello changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	provider := &GitDiffProvider{WorkDir: dir}
	diff, err := provider.Diff("main")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff for uncommitted changes")
	}
	if !strings.Contains(diff, "hello changed") {
		t.Errorf("diff should contain uncommitted content 'hello changed', got:\n%s", diff)
	}
}
