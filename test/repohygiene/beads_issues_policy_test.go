package repohygiene

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckBeadsIssuesPolicyRejectsMixedSemanticAndNormalizationChanges(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	scriptPath := filepath.Join(repoRoot, "scripts", "check_beads_issues_policy.sh")

	repoDir := t.TempDir()
	runCmd(t, repoDir, "git", "init")
	runCmd(t, repoDir, "git", "config", "user.email", "test@example.com")
	runCmd(t, repoDir, "git", "config", "user.name", "Test User")
	if err := os.MkdirAll(filepath.Join(repoDir, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	headNonCanonical := "{ \"title\": \"Second\", \"id\": \"b\" }\n{ \"title\": \"First\", \"id\": \"a\" }\n"
	writeFile(t, filepath.Join(repoDir, ".beads", "issues.jsonl"), headNonCanonical)
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "seed")

	stagedCanonicalWithSemanticChange := "{\"id\":\"a\",\"title\":\"First updated\"}\n{\"id\":\"b\",\"title\":\"Second\"}\n"
	writeFile(t, filepath.Join(repoDir, ".beads", "issues.jsonl"), stagedCanonicalWithSemanticChange)
	runCmd(t, repoDir, "git", "add", ".beads/issues.jsonl")

	cmd := exec.Command(scriptPath)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected policy script to fail, output: %s", output)
	}
	if !strings.Contains(string(output), "split normalization from semantic issue edits") {
		t.Fatalf("expected mixed-change guidance in output, got: %s", output)
	}
}

func TestPreCommitRunsBeadsIssuesPolicyCheck(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	preCommitPath := filepath.Join(repoRoot, ".githooks", "pre-commit")

	content, err := os.ReadFile(preCommitPath)
	if err != nil {
		t.Fatalf("read pre-commit hook: %v", err)
	}
	if !strings.Contains(string(content), "./scripts/check_beads_issues_policy.sh") {
		t.Fatalf("expected pre-commit to run beads issues policy script; hook content:\n%s", string(content))
	}
}

func TestMakefileDefinesBeadsIssuesPolicyGuard(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	makefilePath := filepath.Join(repoRoot, "Makefile")

	content, err := os.ReadFile(makefilePath)
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(content)
	if !strings.Contains(makefile, "beads-issues-policy-guard:") {
		t.Fatalf("expected Makefile target beads-issues-policy-guard, got:\n%s", makefile)
	}
	if !strings.Contains(makefile, "./scripts/check_beads_issues_policy.sh") {
		t.Fatalf("expected Makefile to invoke beads issues policy script, got:\n%s", makefile)
	}
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s %s\n%s", name, strings.Join(args, " "), output)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
