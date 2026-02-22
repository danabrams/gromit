package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

	return []string{
		"GOCACHE=" + buildCache,
		"GOMODCACHE=" + modCache,
		"GOPATH=" + goPath,
	}
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

// getGitDiff returns the full diff from fromCommit to the current working tree.
func getGitDiff(fromCommit string) (string, error) {
	cmd := exec.Command("git", "diff", fromCommit)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}
