package validator

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Check represents a single validation check to execute.
type Check struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Type    string `json:"type"`
}

// CheckResult captures the outcome of running a single check.
type CheckResult struct {
	Name     string        `json:"name"`
	Pass     bool          `json:"pass"`
	Output   string        `json:"output"`
	Duration time.Duration `json:"duration"`
	Type     string        `json:"type"`
}

// Runner executes validation checks as shell commands.
type Runner struct {
	// KnownGaps contains known validation gaps for targeted validation prompts.
	// When non-empty, this text is included in validation guidance sections.
	KnownGaps string
}

// NewRunner creates a new Runner.
func NewRunner() *Runner { return &Runner{} }

// RunCheck executes a single check command in the given working directory.
// A non-zero exit code results in Pass=false but no error return;
// errors are reserved for failures to start the command.
func (r *Runner) RunCheck(ctx context.Context, c Check, workDir string) (CheckResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, "sh", "-c", c.Command)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			// Infrastructure failure (command not found, etc.) — propagate as error
			return CheckResult{}, fmt.Errorf("exec check %s: %w", c.Name, err)
		}
	}
	pass := err == nil
	// For lint checks, non-empty stdout also means failure (e.g. gofmt -l
	// exits 0 but lists files that need formatting).
	if pass && c.Type == "lint" && len(strings.TrimSpace(string(out))) > 0 {
		pass = false
	}
	return CheckResult{
		Name:     c.Name,
		Pass:     pass,
		Output:   string(out),
		Duration: time.Since(start),
		Type:     c.Type,
	}, nil
}
