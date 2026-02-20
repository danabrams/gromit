package runner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
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

// getHead calls the injectable gitHeadFn, falling back to getGitHead if unset.
func (r *Runner) getHead() (string, error) {
	if r.gitHeadFn != nil {
		return r.gitHeadFn()
	}
	return getGitHead()
}

// getDiff calls the injectable gitDiffFn, falling back to getGitDiff if unset.
func (r *Runner) getDiff(fromCommit string) (string, error) {
	if r.gitDiffFn != nil {
		return r.gitDiffFn(fromCommit)
	}
	return getGitDiff(fromCommit)
}

// resetHard updates working tree and index to the provided commit.
func (r *Runner) resetHard(commit string) error {
	_, stderr, exitCode, err := r.runArgv(context.Background(), "git", []string{"reset", "--hard", commit}, ".")
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("git reset failed: %s", strings.TrimSpace(stderr))
	}
	return nil
}

// hasNewPackages returns true if any package in the list is not in the runner's
// touched packages map. Returns true if the map is nil or empty.
func (r *Runner) hasNewPackages(packages []string) bool {
	if len(r.touchedPackages) == 0 {
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

var nonInteractiveEnv = []string{
	"GIT_TERMINAL_PROMPT=0",
	"CI=1",
	"NONINTERACTIVE=1",
	"TERM=dumb",
}

const execFailureExitCode = -1

func prepareCommand(cmd *exec.Cmd, workDir string) {
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(nil)
	env := append(os.Environ(), nonInteractiveEnv...)
	env = append(env, validationGoCacheEnv(workDir)...)
	cmd.Env = env
}

func validationGoCacheEnv(workDir string) []string {
	root := strings.TrimSpace(workDir)
	if root == "" {
		root = "."
	}
	if absRoot, err := filepath.Abs(root); err == nil {
		root = absRoot
	}

	buildCache := filepath.Join(root, ".gromit", "tmp", "go-build-cache")
	modCache := filepath.Join(root, ".gromit", "tmp", "go-mod-cache")
	goPath := filepath.Join(root, ".gromit", "tmp", "go-path")

	for _, dir := range []string{buildCache, modCache, goPath} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
	}

	env := []string{
		"GOCACHE=" + buildCache,
		"GOMODCACHE=" + modCache,
		"GOPATH=" + goPath,
	}
	return env
}

func runCommand(cmd *exec.Cmd) (string, string, int, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout.String(), stderr.String(), exitErr.ExitCode(), nil
		}
		return stdout.String(), stderr.String(), execFailureExitCode, err
	}
	return stdout.String(), stderr.String(), 0, nil
}

// defaultCmdRunner executes a shell command and returns stdout, stderr, exit code.
// Non-zero exit codes are returned as exitCode (not as an error).
func defaultCmdRunner(ctx context.Context, command string, workDir string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	prepareCommand(cmd, workDir)
	return runCommand(cmd)
}

// defaultArgvRunner executes a program with explicit args and returns stdout, stderr, exit code.
// Non-zero exit codes are returned as exitCode (not as an error).
func defaultArgvRunner(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	prepareCommand(cmd, workDir)
	return runCommand(cmd)
}

// runCmd calls the injectable cmdRunnerFn, falling back to defaultCmdRunner if unset.
func (r *Runner) runCmd(ctx context.Context, command string, workDir string) (string, string, int, error) {
	if r.cmdRunnerFn != nil {
		return r.cmdRunnerFn(ctx, command, workDir)
	}
	return defaultCmdRunner(ctx, command, workDir)
}

// runArgv calls the injectable argvRunnerFn, falling back to defaultArgvRunner if unset.
func (r *Runner) runArgv(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
	if r.argvRunnerFn != nil {
		return r.argvRunnerFn(ctx, program, args, workDir)
	}
	return defaultArgvRunner(ctx, program, args, workDir)
}

// isInteractiveStdin reports whether stdin appears to be attached to a TTY.
// statFn is injected for testability.
func isInteractiveStdin(statFn func() (os.FileInfo, error)) bool {
	if statFn == nil {
		return false
	}

	info, err := statFn()
	if err != nil || info == nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

func parseYesNoResponse(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))
	return normalized == "y" || normalized == "yes"
}

func promptYesNo(input io.Reader, output io.Writer, question string) (bool, error) {
	if output != nil {
		if _, err := fmt.Fprint(output, question); err != nil {
			return false, err
		}
	}
	if input == nil {
		return false, nil
	}
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil {
		return false, err
	}
	return parseYesNoResponse(line), nil
}

// extractExpectedFiles parses a bead description for file creation patterns
// like "Create internal/runner/adapters.go" and returns the file paths.
// This enables deterministic precheck rejection for beads that describe
// creating files that don't yet exist.
var fileCreationPattern = regexp.MustCompile(`(?:^|\n)\s*\d*\.?\s*Create\s+((?:internal|cmd|pkg|test)/\S+\.go)`)
var fileDeletionPattern = regexp.MustCompile(`(?:^|\n)\s*\d*\.?\s*(?:Delete|Remove)\s+((?:internal|cmd|pkg|test)/\S+)`)
var filePathPattern = regexp.MustCompile(`(?:internal|cmd|pkg|test)/[^\s,;:()]+`)
var buildTagPattern = regexp.MustCompile(`//go:build\s+([A-Za-z0-9_!&|() -]+)`)
var testNamePattern = regexp.MustCompile(`\bTest[A-Za-z0-9_]+\b`)

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

// extractDeletedFiles parses a bead description for file deletion patterns.
func extractDeletedFiles(description string) []string {
	matches := fileDeletionPattern.FindAllStringSubmatch(description, -1)
	if len(matches) == 0 {
		return nil
	}
	files := make([]string, 0, len(matches))
	for _, m := range matches {
		files = append(files, strings.TrimRight(m[1], ".,"))
	}
	return files
}

func extractFilePaths(description string) []string {
	raw := filePathPattern.FindAllString(description, -1)
	if len(raw) == 0 {
		return nil
	}
	paths := make([]string, 0, len(raw))
	for _, p := range raw {
		trimmed := strings.TrimRight(p, ".,;:)")
		paths = append(paths, trimmed)
	}
	return slices.Compact(paths)
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

func anyFileExists(paths []string) bool {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func filesMissingBuildTag(paths []string, tag string) []string {
	var missing []string
	wantPrefix := "//go:build " + strings.TrimSpace(tag)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			missing = append(missing, p)
			continue
		}
		lines := strings.Split(string(data), "\n")
		firstNonEmpty := ""
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			firstNonEmpty = strings.TrimSpace(line)
			break
		}
		if !strings.HasPrefix(firstNonEmpty, wantPrefix) {
			missing = append(missing, p)
		}
	}
	return missing
}

func filesStillContainTestNames(paths []string, testNames []string) []string {
	var present []string
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := string(data)
		for _, name := range testNames {
			if strings.Contains(content, "func "+name+"(") {
				present = append(present, name)
			}
		}
	}
	return slices.Compact(present)
}

// deterministicPrecheckReason performs lightweight deterministic checks to
// reject obvious false-positive auto-closes for structural/refactor tasks.
func deterministicPrecheckReason(description string) string {
	created := extractExpectedFiles(description)
	if len(created) > 0 && anyFileMissing(created) {
		return "description mentions files to create that don't exist"
	}

	deleted := extractDeletedFiles(description)
	if len(deleted) > 0 && anyFileExists(deleted) {
		return "description mentions files to delete that still exist"
	}

	m := buildTagPattern.FindStringSubmatch(description)
	if len(m) > 1 {
		tag := strings.TrimSpace(m[1])
		paths := extractFilePaths(description)
		if len(paths) > 0 {
			if missing := filesMissingBuildTag(paths, tag); len(missing) > 0 {
				return fmt.Sprintf("description requires //go:build %s but tag is missing in at least one target file", tag)
			}
		}
	}

	lower := strings.ToLower(description)
	if strings.Contains(lower, "delete") || strings.Contains(lower, "remove") {
		testNames := slices.Compact(testNamePattern.FindAllString(description, -1))
		paths := extractFilePaths(description)
		if len(testNames) > 0 && len(paths) > 0 {
			if present := filesStillContainTestNames(paths, testNames); len(present) > 0 {
				return "description says tests should be deleted, but named tests are still present"
			}
		}
	}

	return ""
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
