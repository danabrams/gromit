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

// TestGitDiffProvider_NewFilesVisibleInDiff verifies that new (untracked) files
// created in a git worktree appear in the diff output. This simulates the
// executor creating brand-new files in an ephemeral worktree without staging.
func TestGitDiffProvider_NewFilesVisibleInDiff(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	gitEnv := append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	runOutput := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	// Initialize repo with an initial commit.
	run(repoDir, "git", "init")
	if err := os.WriteFile(repoDir+"/README.md", []byte("# project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repoDir, "git", "add", "README.md")
	run(repoDir, "git", "commit", "-m", "initial")

	// Detect the initial branch name (main or master).
	baseBranch := runOutput(repoDir, "git", "branch", "--show-current")

	// Create a worktree on a new branch.
	run(repoDir, "git", "worktree", "add", "-b", "feature/new-files", worktreeDir)
	t.Cleanup(func() {
		_ = exec.Command("git", "worktree", "remove", "--force", worktreeDir).Run()
	})

	// Create a NEW untracked file in the worktree (do NOT git add).
	newFileDir := worktreeDir + "/internal/contract"
	if err := os.MkdirAll(newFileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	newFileContent := "package contract\n\ntype Scenario struct {\n\tName string\n}\n"
	if err := os.WriteFile(newFileDir+"/types.go", []byte(newFileContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Call Diff — the untracked file should appear.
	provider := &GitDiffProvider{WorkDir: worktreeDir}
	diff, err := provider.Diff(baseBranch)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff for new untracked file")
	}
	if !strings.Contains(diff, "internal/contract/types.go") {
		t.Errorf("diff should reference new file path, got:\n%s", diff)
	}
	if !strings.Contains(diff, "package contract") {
		t.Errorf("diff should contain new file content 'package contract', got:\n%s", diff)
	}
	if !strings.Contains(diff, "Scenario") {
		t.Errorf("diff should contain new file content 'Scenario', got:\n%s", diff)
	}
}

// TestGitDiffProvider_ModifiedAndNewFilesBothVisible verifies that both modified
// existing files and brand-new untracked files appear in the diff output when
// working in a git worktree.
func TestGitDiffProvider_ModifiedAndNewFilesBothVisible(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	gitEnv := append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	runOutput := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	// Initialize repo with an existing file.
	run(repoDir, "git", "init")
	if err := os.WriteFile(repoDir+"/existing.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repoDir, "git", "add", "existing.go")
	run(repoDir, "git", "commit", "-m", "initial")

	baseBranch := runOutput(repoDir, "git", "branch", "--show-current")

	// Create worktree.
	run(repoDir, "git", "worktree", "add", "-b", "feature/mixed-changes", worktreeDir)
	t.Cleanup(func() {
		_ = exec.Command("git", "worktree", "remove", "--force", worktreeDir).Run()
	})

	// Modify the existing file in the worktree.
	if err := os.WriteFile(worktreeDir+"/existing.go", []byte("package main\n\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a brand-new untracked file in the worktree.
	if err := os.WriteFile(worktreeDir+"/newfile.go", []byte("package main\n\nfunc World() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	provider := &GitDiffProvider{WorkDir: worktreeDir}
	diff, err := provider.Diff(baseBranch)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}

	// Verify the modification appears.
	if !strings.Contains(diff, "existing.go") {
		t.Errorf("diff should reference modified file existing.go, got:\n%s", diff)
	}
	if !strings.Contains(diff, "func Hello()") {
		t.Errorf("diff should contain modified content 'func Hello()', got:\n%s", diff)
	}

	// Verify the new file appears.
	if !strings.Contains(diff, "newfile.go") {
		t.Errorf("diff should reference new file newfile.go, got:\n%s", diff)
	}
	if !strings.Contains(diff, "func World()") {
		t.Errorf("diff should contain new file content 'func World()', got:\n%s", diff)
	}
}

// TestGitDiffProvider_GitignoredDirDoesNotBlockDiff verifies that a .gitignore'd
// directory in the worktree does not cause Diff to fail.
func TestGitDiffProvider_GitignoredDirDoesNotBlockDiff(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	gitEnv := append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	runOutput := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	// Initialize repo with a .gitignore that excludes .gromit-next/.
	run(repoDir, "git", "init")
	if err := os.WriteFile(repoDir+"/.gitignore", []byte(".gromit-next/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoDir+"/main.go", []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repoDir, "git", "add", ".")
	run(repoDir, "git", "commit", "-m", "initial")

	baseBranch := runOutput(repoDir, "git", "branch", "--show-current")

	// Create worktree.
	run(repoDir, "git", "worktree", "add", "-b", "feature/ignored-dir", worktreeDir)
	t.Cleanup(func() {
		_ = exec.Command("git", "worktree", "remove", "--force", worktreeDir).Run()
	})

	// Create .gromit-next/ directory with files (this is gitignored).
	ignoredDir := worktreeDir + "/.gromit-next/runs"
	if err := os.MkdirAll(ignoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignoredDir+"/run.json", []byte(`{"status":"running"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Also make a real tracked change.
	if err := os.WriteFile(worktreeDir+"/main.go", []byte("package main\n\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Diff should succeed despite the gitignored directory.
	provider := &GitDiffProvider{WorkDir: worktreeDir}
	diff, err := provider.Diff(baseBranch)
	if err != nil {
		t.Fatalf("Diff should not fail with gitignored dir present: %v", err)
	}
	if !strings.Contains(diff, "func Hello()") {
		t.Errorf("diff should contain tracked changes, got:\n%s", diff)
	}
	if strings.Contains(diff, ".gromit-next") {
		t.Errorf("diff should not contain gitignored files, got:\n%s", diff)
	}
}

// TestGitDiffProvider_EmptyWorktreeNoDiff verifies that a worktree with no
// changes produces an empty diff.
func TestGitDiffProvider_EmptyWorktreeNoDiff(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	gitEnv := append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	runOutput := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	// Initialize repo with a commit.
	run(repoDir, "git", "init")
	if err := os.WriteFile(repoDir+"/file.txt", []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repoDir, "git", "add", "file.txt")
	run(repoDir, "git", "commit", "-m", "initial")

	baseBranch := runOutput(repoDir, "git", "branch", "--show-current")

	// Create worktree — make NO changes.
	run(repoDir, "git", "worktree", "add", "-b", "feature/no-changes", worktreeDir)
	t.Cleanup(func() {
		_ = exec.Command("git", "worktree", "remove", "--force", worktreeDir).Run()
	})

	provider := &GitDiffProvider{WorkDir: worktreeDir}
	diff, err := provider.Diff(baseBranch)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff for unchanged worktree, got:\n%s", diff)
	}
}
