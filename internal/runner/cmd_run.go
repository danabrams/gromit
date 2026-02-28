package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/procutil"
)

var nonInteractiveEnv = []string{
	"GIT_TERMINAL_PROMPT=0",
	"CI=1",
	"NONINTERACTIVE=1",
	"TERM=dumb",
}

const execFailureExitCode = -1
const runnerProcessCapacityWait = 1500 * time.Millisecond

func prepareCommand(cmd *exec.Cmd, workDir string) {
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(nil)
	env := append(procutil.SubprocessEnv(), nonInteractiveEnv...)
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

	return []string{
		"GOCACHE=" + buildCache,
		"GOMODCACHE=" + modCache,
		"GOPATH=" + goPath,
	}
}

func runCommand(ctx context.Context, cmd *exec.Cmd) (string, string, int, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if waitErr := procutil.WaitForProcessCapacity(ctx, runnerProcessCapacityWait); waitErr != nil {
		return "", "", execFailureExitCode, fmt.Errorf("waiting for process capacity: %w", waitErr)
	}
	if err := cmd.Start(); err != nil {
		return "", "", execFailureExitCode, err
	}
	defer procutil.ReapProcessTree(cmd)
	err := cmd.Wait()
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
	procutil.SetProcessGroupKill(cmd)
	prepareCommand(cmd, workDir)
	return runCommand(ctx, cmd)
}

// getGitDiff returns the full diff from fromCommit to the current working tree.
func getGitDiff(ctx context.Context, fromCommit string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", fromCommit)
	procutil.SetProcessGroupKill(cmd)
	stdout, stderr, exitCode, err := runCommand(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("git diff: exit code %d: %s", exitCode, strings.TrimSpace(stderr))
	}
	return stdout, nil
}
