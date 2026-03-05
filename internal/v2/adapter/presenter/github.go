package presenter

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/danabrams/gromit/internal/v2/presentation"
)

// commandRunner describes the subset of os/exec.CommandContext that presenter relies on.
type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// GitHubPresenter uses the GitHub CLI to create pull requests.
type GitHubPresenter struct {
	runner commandRunner
	ghCmd  string
}

// NewGitHubPresenter returns a presenter backed by gh CLI using the provided runner.
// If runner is nil, the default command runner is used.
func NewGitHubPresenter(runner commandRunner) *GitHubPresenter {
	if runner == nil {
		runner = defaultCommandRunner{}
	}
	return &GitHubPresenter{runner: runner, ghCmd: "gh"}
}

// PresentSummary creates a pull request for the provided spec summary.
func (g *GitHubPresenter) PresentSummary(ctx context.Context, specID string, summary presentation.PresentationSummary) error {
	if g.runner == nil {
		g.runner = defaultCommandRunner{}
	}

	head := strings.TrimSpace(summary.SpecBranch)
	if head == "" {
		head = presentation.SpecBranchName(summary.SpecName)
	}
	base := strings.TrimSpace(summary.IntegrationBranch)
	if base == "" {
		base = presentation.DefaultIntegrationBranch()
	}
	if head == "" {
		return fmt.Errorf("spec branch required")
	}
	if base == "" {
		return fmt.Errorf("integration branch required")
	}

	title := strings.TrimSpace(summary.SpecName)
	if title == "" {
		title = strings.TrimSpace(specID)
	}
	if title == "" {
		title = "spec"
	}

	body := presentation.RenderPRBody(summary)
	args := []string{
		"pr",
		"create",
		"--head", head,
		"--base", base,
		"--title", title,
		"--body", body,
	}
	if _, err := g.runner.Run(ctx, g.ghCmd, args...); err != nil {
		return fmt.Errorf("create pr: %w", err)
	}
	return nil
}

// defaultCommandRunner executes commands using os/exec.
type defaultCommandRunner struct{}

func (defaultCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
