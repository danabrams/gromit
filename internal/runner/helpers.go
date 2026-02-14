package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
)

// getGitHead returns the current git HEAD commit
func getGitHead() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// getGitDiffStat returns the git diff --stat output from a given commit to the
// current working tree state (including both committed and uncommitted changes).
func getGitDiffStat(fromCommit string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat", fromCommit)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat: %w", err)
	}
	return string(out), nil
}

// getGitDiff returns the full diff from fromCommit to the current working tree
func getGitDiff(fromCommit string) (string, error) {
	cmd := exec.Command("git", "diff", fromCommit)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// getDiff calls the injectable gitDiffFn, falling back to getGitDiff if unset.
func (r *Runner) getDiff(fromCommit string) (string, error) {
	if r.gitDiffFn != nil {
		return r.gitDiffFn(fromCommit)
	}
	return getGitDiff(fromCommit)
}

// hasNewPackages returns true if any package in the list is not in the runner's
// touched packages map. Returns true if the map is nil or empty.
func (r *Runner) hasNewPackages(packages []string) bool {
	if r.touchedPackages == nil || len(r.touchedPackages) == 0 {
		return len(packages) > 0
	}
	for _, pkg := range packages {
		if !r.touchedPackages[pkg] {
			return true
		}
	}
	return false
}

// updateTouchedPackages adds the given packages to the runner's touched packages map.
func (r *Runner) updateTouchedPackages(packages []string) {
	if r.touchedPackages == nil {
		r.touchedPackages = make(map[string]bool)
	}
	for _, pkg := range packages {
		r.touchedPackages[pkg] = true
	}
}

// defaultCmdRunner executes a shell command and returns stdout, stderr, exit code.
// Non-zero exit codes are returned as exitCode (not as an error).
func defaultCmdRunner(ctx context.Context, command string, workDir string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout.String(), stderr.String(), exitErr.ExitCode(), nil
		}
		return stdout.String(), stderr.String(), -1, err
	}
	return stdout.String(), stderr.String(), 0, nil
}

// runCmd calls the injectable cmdRunnerFn, falling back to defaultCmdRunner if unset.
func (r *Runner) runCmd(ctx context.Context, command string, workDir string) (string, string, int, error) {
	if r.cmdRunnerFn != nil {
		return r.cmdRunnerFn(ctx, command, workDir)
	}
	return defaultCmdRunner(ctx, command, workDir)
}

// extractExpectedFiles parses a bead description for file creation patterns
// like "Create internal/runner/adapters.go" and returns the file paths.
// This enables deterministic precheck rejection for beads that describe
// creating files that don't yet exist.
var fileCreationPattern = regexp.MustCompile(`(?:^|\n)\s*\d*\.?\s*Create\s+((?:internal|cmd|pkg|test)/\S+\.go)`)

func extractExpectedFiles(description string) []string {
	matches := fileCreationPattern.FindAllStringSubmatch(description, -1)
	if len(matches) == 0 {
		return nil
	}
	files := make([]string, 0, len(matches))
	for _, m := range matches {
		files = append(files, m[1])
	}
	return files
}

// checkExpectedOutputs checks if expected files exist and returns a summary
func checkExpectedOutputs(expectedOutputs []string) string {
	if len(expectedOutputs) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "\nExpected outputs:")

	for _, path := range expectedOutputs {
		_, err := os.Stat(path)
		if err == nil {
			lines = append(lines, fmt.Sprintf("  ✓ %s (exists)", path))
		} else if os.IsNotExist(err) {
			lines = append(lines, fmt.Sprintf("  ✗ %s (not found)", path))
		} else {
			lines = append(lines, fmt.Sprintf("  ? %s (error: %v)", path, err))
		}
	}

	return strings.Join(lines, "\n")
}

// anyFileMissing returns true if any of the given paths don't exist on disk.
func anyFileMissing(paths []string) bool {
	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return true
		}
	}
	return false
}

// showPartialProgress displays git diff and expected outputs on failure
func (r *Runner) showPartialProgress(b *bead.Bead, startCommit string) {
	if r == nil || b == nil {
		return
	}
	// Always show git diff summary
	diffStat, err := getGitDiffStat(startCommit)
	if err != nil {
		r.log("Warning: could not get git diff: %v", err)
	} else if strings.TrimSpace(diffStat) != "" {
		r.log("\nChanges detected:")
		r.log("%s", diffStat)
	} else {
		r.log("\nNo git changes detected - Claude may not have completed any work.")
	}

	// Show expected outputs if specified
	if len(b.ExpectedOutputs) > 0 {
		summary := checkExpectedOutputs(b.ExpectedOutputs)
		r.log("%s", summary)
	}
}
