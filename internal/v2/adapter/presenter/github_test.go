package presenter

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/v2/presentation"
	v2review "github.com/danabrams/gromit/internal/v2/review"
)

func TestGitHubPresenterCreatesPR(t *testing.T) {
	t.Parallel()

	runner := &spyCommandRunner{}
	presenter := NewGitHubPresenter(runner)

	summary := presentation.PresentationSummary{
		SpecName:          "spec-success",
		SpecBranch:        "gromit/spec-spec-success",
		IntegrationBranch: "main",
		Success:           true,
		AcceptanceResults: []presentation.AcceptanceResult{
			{
				Title:       "Acceptance tests",
				Description: "All green",
			},
		},
		OutOfScopeFindings: []v2review.Finding{},
	}

	if err := presenter.PresentSummary(context.Background(), "spec-success", summary); err != nil {
		t.Fatalf("present summary: %v", err)
	}

	if runner.name != "gh" {
		t.Fatalf("unexpected command %q", runner.name)
	}
	if !assertArgMatches(runner.args, "--head", summary.SpecBranch) {
		t.Fatalf("head flag missing or incorrect: %v", runner.args)
	}
	if !assertArgMatches(runner.args, "--base", summary.IntegrationBranch) {
		t.Fatalf("base flag missing or incorrect: %v", runner.args)
	}
	if !assertArgMatches(runner.args, "--title", summary.SpecName) {
		t.Fatalf("title flag missing or incorrect: %v", runner.args)
	}
	if runner.bodyContent == "" || !strings.Contains(runner.bodyContent, "Acceptance tests") {
		t.Fatalf("body missing acceptance details: %q", runner.bodyContent)
	}
}

func TestGitHubPresenterUsesBodyFile(t *testing.T) {
	t.Parallel()

	runner := &spyCommandRunner{}
	presenter := NewGitHubPresenter(runner)

	summary := presentation.PresentationSummary{
		SpecName:          "spec-team",
		SpecBranch:        "gromit/spec-team",
		IntegrationBranch: "main",
		Success:           true,
		AcceptanceResults: []presentation.AcceptanceResult{
			{
				Title:       "Acceptance tests",
				Description: "All green",
			},
		},
	}

	if err := presenter.PresentSummary(context.Background(), "spec-team", summary); err != nil {
		t.Fatalf("present summary: %v", err)
	}

	if runner.bodyFile == "" {
		t.Fatalf("expected --body-file arg: %v", runner.args)
	}
	if !strings.Contains(runner.bodyContent, "Acceptance tests") {
		t.Fatalf("body file missing acceptance details: %q", runner.bodyContent)
	}
	expected := presentation.RenderPRBody(summary)
	if runner.bodyContent != expected {
		t.Fatalf("unexpected body file contents\nexpected:\n%q\nactual:\n%q", expected, runner.bodyContent)
	}
}

func assertArgMatches(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

type spyCommandRunner struct {
	name        string
	args        []string
	bodyFile    string
	bodyContent string
}

func (s *spyCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	s.name = name
	s.args = append([]string(nil), args...)
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--body-file" {
			s.bodyFile = args[i+1]
			data, err := os.ReadFile(s.bodyFile)
			if err != nil {
				return "", err
			}
			s.bodyContent = string(data)
		}
	}
	return "", nil
}
